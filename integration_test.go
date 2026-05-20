//go:build integration

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFullRawOutput(t *testing.T) {
	cfg := LoadConfig()
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	client := NewClient(cfg)

	tests := []struct{ name, q string }{
		{"ask", "Перечисли основные объекты метаданных в 1С:Предприятие"},
		{"explain", "Объясни синтаксис и использование конструкции Для Каждого"},
		{"review", "Проверь синтаксис:\n```1c\nПроцедура Тест()\n  А = Справочники.Номенклатура.НайтиПоКоду(123)\nКонецПроцедуры\n```"},
		{"docstring", "Сгенерируй документирующий комментарий:\n```1c\nФункция Цена(Товар, Дата)\n  Возврат РегистрыСведений.Цены.ПолучитьПоследнее(Дата, Номенклатура = Товар).Цена\nКонецФункции\n```"},
		{"explore", "Какие бывают виды регистров в 1С?"},
		{"modify", "Отредактируй код — добавь проверку на пустую ссылку:\n```1c\nПроцедура Провести(Отказ)\n  Для Каждого Строка Из Товары Цикл\n    Строка.Сумма = Строка.Цена * Строка.Количество\n  КонецЦикла\nКонецПроцедуры\n```"},
		{"generate", "Напиши функцию, которая проверяет, существует ли справочник с заданным кодом"},
		{"search", "Как работает механизм блокировок в 1С?"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			convID, _, err := client.CreateConversation(ctx, "")
			if err != nil {
				t.Fatalf("CreateConversation: %v", err)
			}
			t.Logf("conv: %s", convID[:8])

			fmt.Printf("\n===== %s =====\n", tc.name)
			fmt.Printf("📤 QUESTION:\n%s\n\n", tc.q)

			answer, err := client.SendMessage(ctx, convID, tc.q)
			if err != nil {
				t.Fatalf("SendMessage: %v", err)
			}

			fmt.Printf("📝 (%d chars)\n%s\n", len(answer), sanitizeText(answer))
		})
	}
}

func TestCheckCodeFromFile(t *testing.T) {
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
		"code": strings.ReplaceAll(code, "\r\n", "\n"),
	}
	prompt := checkCodePrompt(args)
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
	fmt.Printf("\n===== CHECK_CODE_FROM_FILE (13-51) =====\n📝 (%d chars)\n%s\n\nСессия: %s\n", len(clean), clean, convID)
}

func TestSearchKnowledge(t *testing.T) {
	cfg := LoadConfig()

	args := map[string]string{
		"query":            "В каком модуле (общем или объекта) находится печать заказа покупателя? Где искать процедуру формирования печатной формы?",
		"configuration":    "Управление торговлей 11",
		"platform_version": "8.3.25",
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
	fmt.Printf("\n===== SEARCH_KNOWLEDGE =====\n📝 (%d chars)\n%s\n\nСессия: %s\n", len(clean), clean, convID)
}
