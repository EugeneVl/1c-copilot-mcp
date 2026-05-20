package main

import (
	"fmt"
	"time"
)

type ConversationRequest struct {
	SkillName           string `json:"skill_name"`
	UILanguage          string `json:"ui_language"`
	ProgrammingLanguage string `json:"programming_language"`
	IsChat              bool   `json:"is_chat"`
}

type ConversationResponse struct {
	UUID            string `json:"uuid"`
	RootMessageUUID string `json:"root_message_uuid"`
}

type ContentBlock struct {
	Instruction string `json:"instruction"`
}

type UserContent struct {
	Content ContentBlock `json:"content"`
	Tools   []any        `json:"tools"`
}

type MessageRequest struct {
	Role       string      `json:"role"`
	ParentUUID *string     `json:"parent_uuid"`
	Content    UserContent `json:"content"`
}

type MessageChunk struct {
	UUID         string         `json:"uuid"`
	Role         string         `json:"role"`
	Content      map[string]any `json:"content"`
	ContentDelta map[string]any `json:"content_delta"`
	ParentUUID   string         `json:"parent_uuid"`
	Finished     bool           `json:"finished"`
}

type SSEResult struct {
	Text           string
	Reasoning      string
	AssistantMsgID string
	UserMsgID      string
}

type Session struct {
	ID            string
	CreatedAt     time.Time
	LastUsed      time.Time
	MsgCount      int
	LastMessageID string
}

func NewSession(id string) *Session {
	now := time.Now()
	return &Session{
		ID:        id,
		CreatedAt: now,
		LastUsed:  now,
	}
}

func (s *Session) Update() {
	s.LastUsed = time.Now()
	s.MsgCount++
}

type APIError struct {
	Msg        string
	StatusCode int
}

func (e *APIError) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("API error (status %d): %s", e.StatusCode, e.Msg)
	}
	return e.Msg
}
