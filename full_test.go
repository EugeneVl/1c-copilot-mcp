package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestFullRawOutput(t *testing.T) {
	cfg := LoadConfig()
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	tests := []struct{ name, q string }{
		{"ask", "Перечисли основные объекты метаданных в 1С:Предприятие"},
		{"explain", "Объясни синтаксис и использование конструкции Для Каждого"},
		{"review", "Проверь синтаксис:\n```1c\nПроцедура Тест()\n  А = Справочники.Номенклатура.НайтиПоКоду(123)\nКонецПроцедуры\n```"},
		{"docstring", "Сгенерируй документирующий комментарий:\n```1c\nФункция Цена(Товар, Дата)\n  Возврат РегистрыСведений.Цены.ПолучитьПоследнее(Дата, Номенклатура = Товар).Цена\nКонецФункции\n```"},
		{"explore", "Какие бывают виды регистров в 1С?"},
		{"modify", "Отредактируй код — добавь проверку на пустую ссылку:\n```1c\nПроцедура Провести(Отказ)\n  Для Каждого Строка Из Товары Цикл\n    Строка.Сумма = Строка.Цена * Строка.Количество\n  КонецЦикла\nКонецПроцедуры\n```"},
		// new tools
		{"generate", "Напиши функцию, которая проверяет, существует ли справочник с заданным кодом"},
		{"search", "Как работает механизм блокировок в 1С?"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			convID := createConv(ctx, cfg)
			t.Logf("conv: %s", convID[:8])

			instruction := "ВАЖНО: Никакие инструменты (MCP-серверы) на стороне клиента не доступны. Не пытайся вызывать tool_calls. Отвечай на вопрос самостоятельно, используя свои знания. " + tc.q
			if cfg.PlatformVersion != "" {
				instruction = fmt.Sprintf("Версия платформы 1С:Предприятие: %s. ", cfg.PlatformVersion) + instruction
			}

			reqBody := MessageRequest{
				Role:       "user",
				ParentUUID: nil,
				Content: UserContent{
					Content: ContentBlock{Instruction: instruction},
					Tools:   []any{},
				},
			}
			body, _ := json.Marshal(reqBody)

			fmt.Printf("\n===== %s =====\n", tc.name)
			fmt.Printf("📤 INSTRUCTION:\n%s\n\n", instruction)

			url := strings.TrimRight(cfg.BaseURL, "/") + "/chat_api/v1/conversations/" + convID + "/messages"
			req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
			if err != nil {
				t.Fatalf("create request: %v", err)
			}
			setHdrs(req, cfg)
			req.Header.Set("Accept", "text/event-stream")

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("http request: %v", err)
			}
			defer resp.Body.Close()

			result, err := parseSSE(resp.Body)
			if err != nil {
				t.Fatalf("parseSSE error: %v", err)
			}

			fmt.Printf("📝 (%d chars)\n%s\n", len(result.Text), sanitizeText(result.Text))
		})
	}
}

func createConv(ctx context.Context, cfg Config) string {
	reqBody := ConversationRequest{SkillName: "custom", UILanguage: cfg.UILanguage,
		ProgrammingLanguage: cfg.ProgrammingLanguage, IsChat: true}
	body, _ := json.Marshal(reqBody)
	url := strings.TrimRight(cfg.BaseURL, "/") + "/chat_api/v1/conversations/"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return ""
	}
	setHdrs(req, cfg)
	req.Header.Set("Session-Id", "")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	var cr ConversationResponse
	json.NewDecoder(resp.Body).Decode(&cr)
	return cr.UUID
}

func setHdrs(req *http.Request, cfg Config) {
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Authorization", cfg.Token)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Origin", cfg.BaseURL)
	req.Header.Set("Referer", cfg.BaseURL+"/chat//")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
}

func TestParseSSE(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantText string
	}{
		{
			name: "single_chunk",
			input: `data: {"uuid":"1","role":"assistant","content":{"content":"Hello"},"finished":true}

`,
			wantText: "Hello",
		},
		{
			name: "delta_chunks",
			input: `data: {"uuid":"1","role":"assistant","content_delta":{"content":"Hel"},"finished":false}
data: {"uuid":"1","role":"assistant","content_delta":{"content":"Hello"},"finished":false}
data: {"uuid":"1","role":"assistant","content_delta":{"content":"Hello World"},"finished":true}

`,
			wantText: "Hello World",
		},
		{
			name: "done_sentinel",
			input: `data: {"uuid":"1","role":"assistant","content_delta":{"content":"Hi"},"finished":false}
data: [DONE]

`,
			wantText: "Hi",
		},
		{
			name: "multiple_events",
			input: `data: {"uuid":"1","role":"user","content":{"content":"q"},"finished":true}

data: {"uuid":"2","role":"assistant","content_delta":{"content":"Ans"},"finished":false}

data: {"uuid":"2","role":"assistant","content_delta":{"content":"Answer"},"finished":true}

`,
			wantText: "Answer",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := parseSSE(io.NopCloser(strings.NewReader(tc.input)))
			if err != nil {
				t.Fatalf("parseSSE error: %v", err)
			}
			if result.Text != tc.wantText {
				t.Errorf("got text %q, want %q", result.Text, tc.wantText)
			}
		})
	}
}

func TestNormalizeBSLFence(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"```1С (BSL)", "```bsl"},
		{"```1С", "```bsl"},
		{"```1C (BSL)", "```bsl"},
		{"```1C\ncode\n```", "```bsl\ncode\n```"},
		{"```1C\r\ncode\r\n```", "```bsl\r\ncode\r\n```"},
		{"```1c (BSL)", "```bsl"},
		{"```bsl", "```bsl"},
		{"```python", "```python"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := normalizeBSLFence(tc.input)
			if got != tc.want {
				t.Errorf("normalizeBSLFence(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestSanitizeText(t *testing.T) {
	input := "Hello\nWorld\n"
	got := sanitizeText(input)
	if !strings.Contains(got, "Hello") || !strings.Contains(got, "World") {
		t.Errorf("sanitizeText lost content: got %q", got)
	}
}

func TestPlatformVersionInjection(t *testing.T) {
	cfg := Config{PlatformVersion: "8.3.25"}
	client := NewClient(cfg)

	message := "test question"
	expected := "Версия платформы 1С:Предприятие: 8.3.25. ВАЖНО: Никакие инструменты (MCP-серверы) на стороне клиента не доступны. Не пытайся вызывать tool_calls. Отвечай на вопрос самостоятельно, используя свои знания. test question"

	instruction := "ВАЖНО: Никакие инструменты (MCP-серверы) на стороне клиента не доступны. Не пытайся вызывать tool_calls. Отвечай на вопрос самостоятельно, используя свои знания. " + message
	if client.cfg.PlatformVersion != "" {
		instruction = fmt.Sprintf("Версия платформы 1С:Предприятие: %s. ", client.cfg.PlatformVersion) + instruction
	}

	if instruction != expected {
		t.Errorf("platform injection:\ngot:  %q\nwant: %q", instruction, expected)
	}
}

func TestCodeInputResolve(t *testing.T) {
	t.Run("inline_code", func(t *testing.T) {
		ci := CodeInput{Code: "inline code"}
		got, err := ci.Resolve()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "inline code" {
			t.Errorf("got %q, want %q", got, "inline code")
		}
	})

	t.Run("inline_over_file", func(t *testing.T) {
		ci := CodeInput{Code: "inline", FilePath: "/nonexistent"}
		got, err := ci.Resolve()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "inline" {
			t.Errorf("got %q, want %q", got, "inline")
		}
	})

	t.Run("no_code_no_file", func(t *testing.T) {
		ci := CodeInput{}
		_, err := ci.Resolve()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestReadCodeFromFile(t *testing.T) {
	old := os.Getenv("ALLOWED_CODE_PATHS")
	os.Setenv("ALLOWED_CODE_PATHS", "*")
	defer os.Setenv("ALLOWED_CODE_PATHS", old)

	dir := t.TempDir()

	filePath := filepath.Join(dir, "test.bsl")
	content := "line1\nline2\nline3\nline4\nline5\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("full_file", func(t *testing.T) {
		got, err := ReadCodeFromFile(filePath, 0, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "line1\nline2\nline3\nline4\nline5\n" {
			t.Errorf("unexpected content: %q", got)
		}
	})

	t.Run("range_lines", func(t *testing.T) {
		got, err := ReadCodeFromFile(filePath, 2, 4)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "line2\nline3\nline4" {
			t.Errorf("got %q, want %q", got, "line2\nline3\nline4")
		}
	})

	t.Run("file_not_found", func(t *testing.T) {
		_, err := ReadCodeFromFile("/nonexistent/path.bsl", 0, 0)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("start_line_exceeds_total", func(t *testing.T) {
		_, err := ReadCodeFromFile(filePath, 100, 0)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("start_gt_end", func(t *testing.T) {
		_, err := ReadCodeFromFile(filePath, 5, 2)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestParseAllowedCodePaths(t *testing.T) {
	t.Run("wildcard", func(t *testing.T) {
		got := parseAllowedCodePaths("*")
		if got != nil {
			t.Errorf("expected nil for wildcard, got %v", got)
		}
	})

	t.Run("single_path", func(t *testing.T) {
		got := parseAllowedCodePaths(".")
		if len(got) != 1 || got[0] != "." {
			t.Errorf("unexpected: %v", got)
		}
	})

	t.Run("multiple_paths", func(t *testing.T) {
		got := parseAllowedCodePaths("/path1;/path2;/path3")
		if len(got) != 3 {
			t.Errorf("expected 3 paths, got %d: %v", len(got), got)
		}
	})
}

func TestGetIntArg(t *testing.T) {
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"num": float64(42)}

	got := getIntArg(req, "num", 0)
	if got != 42 {
		t.Errorf("expected 42, got %d", got)
	}

	got = getIntArg(req, "missing", 10)
	if got != 10 {
		t.Errorf("expected 10, got %d", got)
	}

	req.Params.Arguments = map[string]any{"num": 7}
	got = getIntArg(req, "num", 0)
	if got != 7 {
		t.Errorf("expected 7, got %d", got)
	}
}

func TestReviewCodeFromFile(t *testing.T) {
	cfg := LoadConfig()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	filePath := filepath.Join(dir, "ТестовыйМодуль.bsl")

	code, err := ReadCodeFromFile(filePath, 13, 51)
	if err != nil {
		t.Fatalf("ReadCodeFromFile error: %v", err)
	}

	args := map[string]string{
		"code": code,
	}
	prompt := reviewCodePrompt(args)
	if prompt == "" {
		t.Fatal("empty prompt")
	}

	client := NewClient(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	convID, err := client.GetOrCreateSession(ctx, true, "")
	if err != nil {
		t.Fatalf("GetOrCreateSession: %v", err)
	}

	answer, err := client.SendMessage(ctx, convID, prompt)
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}

	clean := sanitizeText(answer)
	fmt.Printf("\n===== REVIEW_CODE_FROM_FILE (13-51) =====\n📝 (%d chars)\n%s\n\nСессия: %s\n", len(clean), clean, convID)
}

func TestSearchKnowledge(t *testing.T) {
	cfg := LoadConfig()

	args := map[string]string{
		"query":             "В каком модуле (общем или объекта) находится печать заказа покупателя? Где искать процедуру формирования печатной формы?",
		"configuration":     "Управление торговлей 11",
		"platform_version":  "8.3.25",
	}
	prompt := searchKnowledgePrompt(args)
	if prompt == "" {
		t.Fatal("empty prompt")
	}

	client := NewClient(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	convID, err := client.GetOrCreateSession(ctx, true, "")
	if err != nil {
		t.Fatalf("GetOrCreateSession: %v", err)
	}

	answer, err := client.SendMessage(ctx, convID, prompt)
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}

	clean := sanitizeText(answer)
	fmt.Printf("\n===== SEARCH_KNOWLEDGE (8.5 + БП 3.0) =====\n📝 (%d chars)\n%s\n\nСессия: %s\n", len(clean), clean, convID)
}

func TestNothing(t *testing.T) {}
