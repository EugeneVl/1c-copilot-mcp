package main

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Token               string
	BaseURL             string
	Timeout             time.Duration
	UILanguage          string
	ProgrammingLanguage string
	PlatformVersion     string
	AllowedCodePaths    []string
	MaxActiveSessions   int
	SessionTTL          time.Duration
	LogFields           map[string]bool
}

func LoadConfig() Config {
	return Config{
		Token:               envOrDefault("ONEC_AI_TOKEN", ""),
		BaseURL:             envOrDefault("ONEC_AI_BASE_URL", "https://code.1c.ai"),
		Timeout:             time.Duration(envIntOrDefault("ONEC_AI_TIMEOUT", 30)) * time.Second,
		UILanguage:          envOrDefault("ONEC_AI_UI_LANGUAGE", "russian"),
		ProgrammingLanguage: envOrDefault("ONEC_AI_PROGRAMMING_LANGUAGE", "1C (BSL)"),
		PlatformVersion:     envOrDefault("ONEC_AI_PLATFORM_VERSION", "8.3.25"),
		AllowedCodePaths:    parseAllowedCodePaths(envOrDefault("ALLOWED_CODE_PATHS", ".")),
		MaxActiveSessions:   envIntOrDefault("MAX_ACTIVE_SESSIONS", 10),
		SessionTTL:          time.Duration(envIntOrDefault("SESSION_TTL", 3600)) * time.Second,
		LogFields:           parseLogFields(os.Getenv("ONEC_AI_LOG_FIELDS")),
	}
}

func parseAllowedCodePaths(s string) []string {
	if s == "*" {
		return nil
	}
	parts := strings.Split(s, ";")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	if len(result) == 0 {
		return []string{"."}
	}
	return result
}

func parseLogFields(s string) map[string]bool {
	fields := make(map[string]bool)
	for _, part := range strings.FieldsFunc(s, func(r rune) bool {
		return r == ' ' || r == ','
	}) {
		if part != "" {
			fields[strings.ToLower(part)] = true
		}
	}
	return fields
}

func envOrDefault(key, def string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return def
}

func envIntOrDefault(key string, def int) int {
	if val := os.Getenv(key); val != "" {
		if n, err := strconv.Atoi(val); err == nil {
			return n
		}
	}
	return def
}
