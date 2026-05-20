package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func ReadCodeFromFile(path string, startLine, endLine int) (string, error) {
	absPath, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", fmt.Errorf("invalid path %q: %w", path, err)
	}

	cfg := LoadConfig()
	if !isPathAllowed(absPath, cfg.AllowedCodePaths) {
		return "", fmt.Errorf("access denied: %s is outside allowed code paths", absPath)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("file not found: %s", absPath)
		}
		return "", fmt.Errorf("cannot access %s: %w", absPath, err)
	}

	if info.Size() > 10*1024*1024 {
		return "", fmt.Errorf("file too large: %d bytes (max 10MB)", info.Size())
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		return "", fmt.Errorf("cannot read %s: %w", absPath, err)
	}

	lines := strings.Split(string(content), "\n")
	totalLines := len(lines)

	if startLine < 0 {
		startLine = 0
	}
	if endLine < 0 {
		endLine = 0
	}

	if startLine > totalLines {
		return "", fmt.Errorf("start_line %d exceeds total lines %d", startLine, totalLines)
	}
	if startLine > endLine && endLine != 0 {
		return "", fmt.Errorf("start_line %d is greater than end_line %d", startLine, endLine)
	}

	from := 0
	if startLine > 0 {
		from = startLine - 1
	}
	to := totalLines
	if endLine > 0 && endLine <= totalLines {
		to = endLine
	}
	if endLine > totalLines {
		to = totalLines
	}

	selected := lines[from:to]
	return strings.Join(selected, "\n"), nil
}

func isPathAllowed(absPath string, allowedPaths []string) bool {
	if len(allowedPaths) == 0 {
		return true
	}
	evalPath, err := filepath.EvalSymlinks(absPath)
	if err == nil {
		absPath = evalPath
	}
	absPathLower := strings.ToLower(absPath)
	for _, p := range allowedPaths {
		absAllowed, err := filepath.Abs(filepath.Clean(p))
		if err != nil {
			continue
		}
		evalAllowed, err := filepath.EvalSymlinks(absAllowed)
		if err == nil {
			absAllowed = evalAllowed
		}
		if strings.HasPrefix(absPathLower, strings.ToLower(absAllowed)) {
			return true
		}
	}
	return false
}
