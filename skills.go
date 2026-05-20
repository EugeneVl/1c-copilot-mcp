package main

import (
	"fmt"
	"strings"
)

type SkillParamType int

const (
	ParamString SkillParamType = iota
	ParamInteger
	ParamBoolean
	ParamEnum
)

type SkillParam struct {
	Name        string
	Required    bool
	Type        SkillParamType
	Enum        []string
	Description string
}

type Skill struct {
	Name              string
	Description       string
	Parameters        []SkillParam
	SupportsFileInput bool
	Prompt            func(args map[string]string) string
}

var AllSkills []Skill

func init() {
	AllSkills = []Skill{
		{
			Name:        "ask_1c_ai",
			Description: "Задать вопрос специализированному ИИ-ассистенту 1С.ai (1С:Напарник) по платформе 1С:Предприятие. Режимы: общий вопрос, развёрнутая консультация, глубокий экспертный разбор.",
			Parameters: []SkillParam{
				{Name: "question", Required: true, Type: ParamString, Description: "Вопрос для модели 1С.ai"},
				{Name: "consultation_type", Required: false, Type: ParamEnum, Enum: []string{"general", "consulting", "expertise"}, Description: "Тип консультации: general (обычный ответ), consulting (развёрнутая консультация), expertise (экспертный разбор)"},
				{Name: "create_new_session", Required: false, Type: ParamBoolean, Description: "Создать новую сессию (без контекста предыдущих вопросов)"},
			},
			SupportsFileInput: false,
			Prompt:            askPrompt,
		},
		{
			Name:        "explain_1c_syntax",
			Description: "Объяснить синтаксис, конструкции и объекты языка 1С:Предприятие.",
			Parameters: []SkillParam{
				{Name: "syntax_element", Required: true, Type: ParamString, Description: "Элемент синтаксиса или объект 1С для объяснения"},
				{Name: "context", Required: false, Type: ParamString, Description: "Дополнительный контекст использования"},
			},
			SupportsFileInput: false,
			Prompt:            explainPrompt,
		},
		{
			Name:        "check_1c_code",
			Description: "Комплексная проверка кода 1С: синтаксис, логика, производительность, архитектура, стиль оформления, стандарты и лучшие практики. Поддерживает inline-код или чтение из файла.",
			Parameters: []SkillParam{
				{Name: "code", Required: false, Type: ParamString, Description: "Inline-код 1С для проверки (обязателен, если нет file_path)"},
				{Name: "file_path", Required: false, Type: ParamString, Description: "Путь к файлу с кодом 1С (обязателен, если нет code)"},
				{Name: "start_line", Required: false, Type: ParamInteger, Description: "Начальная строка (1-indexed, 0 = с начала)"},
				{Name: "end_line", Required: false, Type: ParamInteger, Description: "Конечная строка (0 = до конца)"},
				{Name: "context", Required: false, Type: ParamString, Description: "Имя конфигурации или стандартов для учёта при проверке"},
			},
			SupportsFileInput: true,
			Prompt:            checkCodePrompt,
		},
		{
			Name:        "explore_1c_codebase",
			Description: "Исследовать базу кода 1С:Предприятие — найти объекты, связи и зависимости.",
			Parameters: []SkillParam{
				{Name: "query", Required: true, Type: ParamString, Description: "Что найти или исследовать"},
				{Name: "code", Required: false, Type: ParamString, Description: "Фрагмент кода 1С для анализа связей"},
			},
			SupportsFileInput: false,
			Prompt:            explorePrompt,
		},
		{
			Name:        "search_1c_knowledge",
			Description: "Поиск информации по ИТС, документации платформы 1С и типовым конфигурациям через знания модели.",
			Parameters: []SkillParam{
				{Name: "query", Required: true, Type: ParamString, Description: "Поисковый запрос"},
				{Name: "configuration", Required: false, Type: ParamString, Description: "Имя конфигурации для фокусировки ответа"},
				{Name: "platform_version", Required: false, Type: ParamString, Description: "Версия платформы для фокусировки ответа"},
			},
			SupportsFileInput: false,
			Prompt:            searchKnowledgePrompt,
		},
	}
}

var checkCodeTemplate = `Проведи комплексную проверку кода 1С. Для каждой найденной проблемы ОБЯЗАТЕЛЬНО указывай номер строки (начиная с 1) и имя процедуры/функции, где она обнаружена. НЕ выводи полный исправленный код целиком.

Проверь код по аспектам:

1. СИНТАКСИС: правильность конструкций языка, парные скобки (Если/КонецЕсли, Процедура/КонецПроцедуры), корректность обращения к методам и свойствам объектов, соответствие синтаксису встроенного языка 1С.

2. ЛОГИКА: корректность алгоритма, обработка граничных случаев (пустые значения, Null, Неопределено), потенциальные ошибки времени выполнения, корректность условий и циклов, работа с транзакциями.

3. ПРОИЗВОДИТЕЛЬНОСТЬ: эффективность запросов (индексы, временные таблицы, пакетные запросы вместо циклов), оптимальность обхода выборок (Выбрать/Выгрузить), работа с регистрами, потенциальные блокировки.

4. АРХИТЕКТУРА: разделение ответственности между модулями, правильность выбора объектов метаданных, связность и зацепление, масштабируемость.

5. СТИЛЬ ОФОРМЛЕНИЯ: именование переменных, процедур и функций (в 1С принят PascalCase — каждое слово с заглавной буквы), отступы и форматирование, длина строк, расположение операторов, наличие и качество комментариев, использование директив компиляции.

6. СТАНДАРТЫ РАЗРАБОТКИ: соответствие методическому пособию по разработке 1С, стандартам оформления типовых конфигураций, правильность организации модулей, корректность обработчиков событий, использование типовых механизмов платформы.

7. ЛУЧШИЕ ПРАКТИКИ: читаемость и понятность, соблюдение принципа DRY (отсутствие дублирования), модульность и переиспользуемость, поддерживаемость, следование идиоматичному стилю 1С.

Для каждой проблемы укажи номер строки, имя процедуры/функции и приоритет: КРИТИЧЕСКИЕ (приведут к ошибке), СЕРЬЁЗНЫЕ (потенциальные проблемы), РЕКОМЕНДАТЕЛЬНЫЕ (улучшения). Кратко опиши причину. Если исправление тривиально (1-2 строки) — напиши его в скобках после описания.

ЭКСПЕРТНЫЙ РАЗБОР: проведи глубокий экспертный разбор. Рассмотри проблемы с разных сторон: архитектурные последствия, влияние на производительность, риски при обновлении конфигурации, соответствие методическим рекомендациям 1С.

ИТОГОВЫЙ ВЕРДИКТ: дай общую оценку качества кода (хорошо/средне/плохо) с кратким обоснованием. Пиши по делу, без воды.`


func askPrompt(args map[string]string) string {
	question := args["question"]
	if question == "" {
		return ""
	}
	consultationType := args["consultation_type"]
	if consultationType == "" {
		consultationType = "general"
	}

	switch consultationType {
	case "consulting":
		return fmt.Sprintf("Дай развёрнутую консультацию по вопросу. Обоснуй ответ, приведи примеры из разработки на 1С:Предприятие, укажи альтернативные подходы с плюсами и минусами. Пиши по делу, без воды. Вопрос: %s", question)
	case "expertise":
		return fmt.Sprintf("Проведи глубокий экспертный разбор. Рассмотри проблему с разных сторон: архитектурные последствия, влияние на производительность, риски при обновлении конфигурации, соответствие методическим рекомендациям 1С. Дай итоговую рекомендацию с обоснованием. Пиши по делу, без воды. Вопрос: %s", question)
	default:
		return fmt.Sprintf("Ответь кратко — 2-3 предложения. Вопрос: %s", question)
	}
}

func explainPrompt(args map[string]string) string {
	element := args["syntax_element"]
	if element == "" {
		return ""
	}
	contextStr := args["context"]

	q := fmt.Sprintf("Объясни синтаксис и использование конструкции/объекта 1С:Предприятие: %s.\nОпиши кратко:\n1. Назначение\n2. Синтаксис (с примерами кода)\n3. Особенности и типичные ошибки\n\nЕсли вопрос простой — ответь в 2-3 предложения без примеров.", element)
	if contextStr != "" {
		q += fmt.Sprintf("\nКонтекст: %s", contextStr)
	}
	return q
}

func checkCodePrompt(args map[string]string) string {
	code := args["code"]
	if code == "" {
		return ""
	}

	var buf strings.Builder
	buf.WriteString(checkCodeTemplate)

	contextStr := args["context"]
	if contextStr != "" {
		fmt.Fprintf(&buf, "\n\nКонтекст: %s\nУчитывай стандарты конфигурации %s.", contextStr, contextStr)
	}

	buf.WriteString("\n\nКод 1С для анализа:\n```bsl\n")
	buf.WriteString(code)
	buf.WriteString("\n```")
	return buf.String()
}

func explorePrompt(args map[string]string) string {
	query := args["query"]
	if query == "" {
		return ""
	}
	code := args["code"]

	q := fmt.Sprintf("Ответь строго по запросу. Формат ответа — маркированный список. Каждый пункт: имя объекта, краткое описание (1 предложение). Без лишних пояснений.\n\nЗапрос: %s", query)
	if code != "" {
		q += fmt.Sprintf("\n\nКод для анализа:\n```bsl\n%s\n```", code)
	}
	return q
}

func searchKnowledgePrompt(args map[string]string) string {
	query := args["query"]
	if query == "" {
		return ""
	}
	config := args["configuration"]
	platformVersion := args["platform_version"]
	if platformVersion == "" {
		platformVersion = "8.3.25"
	}

	prompt := fmt.Sprintf(`Ответь на вопрос, используя все доступные знания:
- ИТС (Информационно-технологическое сопровождение 1С): документация, методические рекомендации, типовые решения
- Платформа 1С:Предприятие %s: механизмы, объекты, синтаксис встроенного языка, системные перечисления, ограничения
- Библиотека стандартных подсистем (БСП): описание общих модулей, процедур и функций БСП, их назначение, параметры, исходные тексты`, platformVersion)

	if config != "" {
		prompt += fmt.Sprintf("\n- Типовые конфигурации %s: объекты метаданных, модули, обработчики, типовые механизмы", config)
	}

	prompt += fmt.Sprintf(`
Вопрос: %s`, query)

	return prompt
}
