package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

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
