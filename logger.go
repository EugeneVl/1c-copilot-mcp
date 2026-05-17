package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	logPath string
	logOnce sync.Once
)

func initLogger() {
	logOnce.Do(func() {
		exe, err := os.Executable()
		if err != nil {
			logPath = "1c-copilot.log"
		} else {
			logPath = filepath.Join(filepath.Dir(exe), "1c-copilot.log")
		}
	})
}

func logInteraction(prompt, reasoning, answer string, fields map[string]bool) {
	if !fields["prompt"] && !fields["reasoning"] && !fields["answer"] {
		return
	}

	initLogger()

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	now := time.Now().Format("2006-01-02 15:04:05")
	fmt.Fprintf(f, "════ %s ════\n", now)
	if prompt != "" && fields["prompt"] {
		fmt.Fprintf(f, "📤 PROMPT:\n%s\n\n", prompt)
	}
	if reasoning != "" && fields["reasoning"] {
		fmt.Fprintf(f, "💭 REASONING:\n%s\n\n", reasoning)
	}
	if answer != "" && fields["answer"] {
		fmt.Fprintf(f, "📝 ANSWER:\n%s\n\n", answer)
	}
}
