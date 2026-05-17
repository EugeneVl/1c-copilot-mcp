package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Client struct {
	cfg      Config
	http     *http.Client
	mu       sync.Mutex
	sessions map[string]*Session
}

func NewClient(cfg Config) *Client {
	return &Client{
		cfg:      cfg,
		http:     &http.Client{Timeout: cfg.Timeout},
		sessions: make(map[string]*Session),
	}
}

func (c *Client) newRequest(ctx context.Context, method, url string, body []byte) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Authorization", c.cfg.Token)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Origin", c.cfg.BaseURL)
	req.Header.Set("Referer", c.cfg.BaseURL+"/chat//")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	return req, nil
}

func (c *Client) CreateConversation(ctx context.Context, progLang string) (convID, rootMsgUUID string, err error) {
	pl := progLang
	if pl == "" {
		pl = c.cfg.ProgrammingLanguage
	}

	reqBody := ConversationRequest{
		SkillName:           "custom",
		UILanguage:          c.cfg.UILanguage,
		ProgrammingLanguage: pl,
		IsChat:              true,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", "", fmt.Errorf("marshal request: %w", err)
	}

	url := strings.TrimRight(c.cfg.BaseURL, "/") + "/chat_api/v1/conversations/"
	req, err := c.newRequest(ctx, http.MethodPost, url, body)
	if err != nil {
		return "", "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Session-Id", "")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", "", &APIError{Msg: fmt.Sprintf("network error creating conversation: %v", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", &APIError{StatusCode: resp.StatusCode, Msg: fmt.Sprintf("create conversation failed: status %d", resp.StatusCode)}
	}

	var convResp ConversationResponse
	if err := json.NewDecoder(resp.Body).Decode(&convResp); err != nil {
		return "", "", fmt.Errorf("decode conversation response: %w", err)
	}

	c.mu.Lock()
	s := NewSession(convResp.UUID)
	if convResp.RootMessageUUID != "" {
		s.LastMessageID = convResp.RootMessageUUID
	}
	c.sessions[convResp.UUID] = s
	c.mu.Unlock()

	return convResp.UUID, convResp.RootMessageUUID, nil
}

func (c *Client) SendMessage(ctx context.Context, convID, message string) (string, error) {
	c.mu.Lock()
	s, ok := c.sessions[convID]
	if !ok {
		s = NewSession(convID)
		c.sessions[convID] = s
	}
	var parentUUID *string
	if s.LastMessageID != "" {
		id := s.LastMessageID
		parentUUID = &id
	}
	c.mu.Unlock()

	instruction := "ВАЖНО: Никакие инструменты (MCP-серверы) на стороне клиента не доступны. Не пытайся вызывать tool_calls. Отвечай на вопрос самостоятельно, используя свои знания. " + message
	if c.cfg.PlatformVersion != "" {
		instruction = fmt.Sprintf("Версия платформы 1С:Предприятие: %s. ", c.cfg.PlatformVersion) + instruction
	}

	reqBody := MessageRequest{
		Role:       "user",
		ParentUUID: parentUUID,
		Content: UserContent{
			Content: ContentBlock{
				Instruction: instruction,
			},
			Tools: []any{},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	url := strings.TrimRight(c.cfg.BaseURL, "/") + "/chat_api/v1/conversations/" + convID + "/messages"
	req, err := c.newRequest(ctx, http.MethodPost, url, body)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", &APIError{Msg: fmt.Sprintf("network error sending message: %v", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", &APIError{StatusCode: resp.StatusCode, Msg: fmt.Sprintf("send message failed: status %d", resp.StatusCode)}
	}

	result, err := parseSSE(resp.Body)
	if err != nil {
		return "", err
	}

	logInteraction(message, result.Reasoning, result.Text, c.cfg.LogFields)

	c.mu.Lock()
	if s, ok := c.sessions[convID]; ok {
		s.Update()
		if result.AssistantMsgID != "" {
			s.LastMessageID = result.AssistantMsgID
		} else if result.UserMsgID != "" {
			s.LastMessageID = result.UserMsgID
		}
	}
	c.mu.Unlock()

	return result.Text, nil
}

func parseSSE(r io.Reader) (*SSEResult, error) {
	result := &SSEResult{}
	var accumulatedText string
	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 4096)

	for {
		n, err := r.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}

		for {
			idx := bytes.Index(buf, []byte("\n\n"))
			if idx == -1 {
				break
			}

			eventBytes := buf[:idx+2]
			buf = buf[idx+2:]

			eventStr := string(eventBytes)
			lines := strings.Split(eventStr, "\n")

			for _, line := range lines {
				var data string
				if after, ok := strings.CutPrefix(line, "data: "); ok {
					data = after
				} else if after, ok := strings.CutPrefix(line, "data:"); ok {
					data = after
				} else {
					continue
				}

				if data == "[DONE]" {
					goto done
				}

				var chunk MessageChunk
				if err := json.Unmarshal([]byte(data), &chunk); err != nil {
					continue
				}

				if chunk.Role == "user" && result.UserMsgID == "" {
					result.UserMsgID = chunk.UUID
				}

				if chunk.Role == "assistant" && result.AssistantMsgID == "" {
					result.AssistantMsgID = chunk.UUID
				}

				reasoning := extractReasoning(chunk)
				if reasoning != "" {
					result.Reasoning = reasoning
				}

				text := extractText(chunk)
				if text != "" {
					if len(text) > len(accumulatedText) && strings.HasPrefix(text, accumulatedText) {
						newPart := text[len(accumulatedText):]
						if newPart != "" {
							accumulatedText = text
						}
					} else if text != accumulatedText {
						accumulatedText = text
					}
				}

				if chunk.Role == "assistant" && chunk.Finished {
					goto done
				}
			}
		}

		if err != nil {
			if err == io.EOF {
				break
			}
			return result, err
		}
	}
done:

	result.Text = strings.TrimSpace(normalizeBSLFence(accumulatedText))
	return result, nil
}

func extractReasoning(chunk MessageChunk) string {
	for _, src := range []map[string]any{chunk.Content, chunk.ContentDelta} {
		if src == nil {
			continue
		}
		if t, ok := src["reasoning_content"]; ok {
			if s, ok := t.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

func extractText(chunk MessageChunk) string {
	for _, src := range []map[string]any{chunk.Content, chunk.ContentDelta} {
		if src == nil {
			continue
		}
		if t, ok := src["content"]; ok {
			if s, ok := t.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

func normalizeBSLFence(s string) string {
	replacer := strings.NewReplacer(
		"```1С (BSL)", "```bsl",
		"```1С", "```bsl",
		"```1C (BSL)", "```bsl",
		"```1C\n", "```bsl\n",
		"```1C\r\n", "```bsl\r\n",
		"```1c (BSL)", "```bsl",
	)
	return replacer.Replace(s)
}

func (c *Client) GetOrCreateSession(ctx context.Context, createNew bool, progLang string) (string, error) {
	c.mu.Lock()
	c.cleanupLocked()

	if createNew || len(c.sessions) == 0 {
		c.mu.Unlock()
		convID, _, err := c.CreateConversation(ctx, progLang)
		return convID, err
	}

	if len(c.sessions) >= c.cfg.MaxActiveSessions {
		var oldestID string
		var oldestTime time.Time
		for id, s := range c.sessions {
			if oldestID == "" || s.LastUsed.Before(oldestTime) {
				oldestID = id
				oldestTime = s.LastUsed
			}
		}
		delete(c.sessions, oldestID)
	}

	var recentID string
	var recentTime time.Time
	for id, s := range c.sessions {
		if s.LastUsed.After(recentTime) {
			recentID = id
			recentTime = s.LastUsed
		}
	}
	c.mu.Unlock()

	return recentID, nil
}

func (c *Client) cleanupLocked() {
	cutoff := time.Now().Add(-c.cfg.SessionTTL)
	for id, s := range c.sessions {
		if s.LastUsed.Before(cutoff) {
			delete(c.sessions, id)
		}
	}
}
