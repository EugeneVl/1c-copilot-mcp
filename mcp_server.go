package main

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"golang.org/x/text/unicode/norm"
)

type MCPServer struct {
	client *Client
	srv    *server.MCPServer
}

func NewMCPServer(cfg Config) *MCPServer {
	client := NewClient(cfg)

	srv := server.NewMCPServer(
		"MCP_1copilot",
		"0.1.0",
		server.WithToolCapabilities(false),
		server.WithRecovery(),
	)

	s := &MCPServer{client: client, srv: srv}
	s.registerTools()
	return s
}

func (s *MCPServer) registerTools() {
	for i := range AllSkills {
		skill := &AllSkills[i]
		opts := []mcp.ToolOption{
			mcp.WithDescription(skill.Description),
		}
		for _, p := range skill.Parameters {
			propOpts := []mcp.PropertyOption{
				mcp.Description(p.Description),
			}
			if p.Required {
				propOpts = append(propOpts, mcp.Required())
			}
			if len(p.Enum) > 0 {
				propOpts = append(propOpts, mcp.Enum(p.Enum...))
			}
			switch p.Type {
			case ParamString, ParamEnum:
				opts = append(opts, mcp.WithString(p.Name, propOpts...))
			case ParamInteger:
				opts = append(opts, mcp.WithNumber(p.Name, propOpts...))
			case ParamBoolean:
				opts = append(opts, mcp.WithBoolean(p.Name, propOpts...))
			}
		}
		tool := mcp.NewTool(skill.Name, opts...)
		s.srv.AddTool(tool, s.handleSkill(skill))
	}
}

func (s *MCPServer) handleSkill(skill *Skill) func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := make(map[string]string)
		for _, p := range skill.Parameters {
			if p.Type == ParamInteger {
				args[p.Name] = fmt.Sprintf("%d", getIntArg(req, p.Name, 0))
			} else {
				args[p.Name] = getStringArg(req, p.Name, "")
			}
		}

		if skill.SupportsFileInput {
			code := args["code"]
			filePath := args["file_path"]
			startLine := getIntArg(req, "start_line", 0)
			endLine := getIntArg(req, "end_line", 0)

			if code == "" && filePath != "" {
				resolved, err := ReadCodeFromFile(filePath, startLine, endLine)
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("Ошибка чтения файла: %v", err)), nil
				}
				args["code"] = resolved
			} else if code == "" && filePath == "" {
				return mcp.NewToolResultError("Ошибка: требуется указать code или file_path"), nil
			}
		}

		instruction := skill.Prompt(args)
		if instruction == "" {
			return mcp.NewToolResultError("Ошибка: не удалось сформировать запрос"), nil
		}

		createNew := getBoolArg(req, "create_new_session", false)
		progLang := getStringArg(req, "programming_language", "")

		convID, err := s.client.GetOrCreateSession(ctx, createNew, progLang)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Ошибка при обращении к 1С.ai: %v", err)), nil
		}

		start := time.Now()
		answer, err := s.client.SendMessage(ctx, convID, instruction)
		elapsed := time.Since(start)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Ошибка при обращении к 1С.ai: %v", err)), nil
		}

		clean := sanitizeText(answer)
		preface := fmt.Sprintf("Ответ от %s", skill.Name)
		return mcp.NewToolResultText(fmt.Sprintf("%s\n\n%s\n\n⏱ %s · Сессия: %s", preface, clean, elapsed.Round(time.Millisecond), convID)), nil
	}
}

func (s *MCPServer) Run() error {
	return server.ServeStdio(s.srv)
}

func getStringArg(req mcp.CallToolRequest, key, def string) string {
	args := req.GetArguments()
	if v, ok := args[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return def
}

func getIntArg(req mcp.CallToolRequest, key string, def int) int {
	args := req.GetArguments()
	if v, ok := args[key]; ok {
		switch n := v.(type) {
		case float64:
			return int(n)
		case int:
			return n
		}
	}
	return def
}

func getBoolArg(req mcp.CallToolRequest, key string, def bool) bool {
	args := req.GetArguments()
	if v, ok := args[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return def
}

func sanitizeText(s string) string {
	if s == "" {
		return s
	}

	s = norm.NFKC.String(s)
	s = normalizeBSLFence(s)

	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r == '\n' || r == '\r' || r == '\t' || (r >= 32 && !unicode.In(r, unicode.Cf)) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
