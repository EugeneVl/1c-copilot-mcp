package main

import "fmt"

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
			Description: "Проверить код 1С на синтаксические, логические ошибки, проблемы производительности и архитектурные просчёты. Поддерживает inline-код или чтение из файла.",
			Parameters: []SkillParam{
				{Name: "code", Required: false, Type: ParamString, Description: "Inline-код 1С для проверки (обязателен, если нет file_path)"},
				{Name: "file_path", Required: false, Type: ParamString, Description: "Путь к файлу с кодом 1С (обязателен, если нет code)"},
				{Name: "start_line", Required: false, Type: ParamInteger, Description: "Начальная строка (1-indexed, 0 = с начала)"},
				{Name: "end_line", Required: false, Type: ParamInteger, Description: "Конечная строка (0 = до конца)"},
			},
			SupportsFileInput: true,
			Prompt:            checkCodePrompt,
		},
		{
			Name:        "review_1c_code",
			Description: "Провести code review кода 1С — оценить качество оформления, соответствие стандартам и лучшим практикам. НЕ проверяет код на ошибки (для этого — check_1c_code).",
			Parameters: []SkillParam{
				{Name: "code", Required: false, Type: ParamString, Description: "Inline-код 1С для ревью (обязателен, если нет file_path)"},
				{Name: "file_path", Required: false, Type: ParamString, Description: "Путь к файлу с кодом 1С (обязателен, если нет code)"},
				{Name: "start_line", Required: false, Type: ParamInteger, Description: "Начальная строка (1-indexed, 0 = с начала)"},
				{Name: "end_line", Required: false, Type: ParamInteger, Description: "Конечная строка (0 = до конца)"},
				{Name: "context", Required: false, Type: ParamString, Description: "Контекст ревью (имя конфигурации, стандарты)"},
			},
			SupportsFileInput: true,
			Prompt:            reviewCodePrompt,
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
			Name:        "generate_docstring",
			Description: "Сгенерировать документирующий комментарий для процедуры или функции на языке 1С:Предприятие.",
			Parameters: []SkillParam{
				{Name: "code", Required: false, Type: ParamString, Description: "Inline-код 1С (обязателен, если нет file_path)"},
				{Name: "file_path", Required: false, Type: ParamString, Description: "Путь к файлу с кодом 1С (обязателен, если нет code)"},
				{Name: "start_line", Required: false, Type: ParamInteger, Description: "Начальная строка (1-indexed, 0 = с начала)"},
				{Name: "end_line", Required: false, Type: ParamInteger, Description: "Конечная строка (0 = до конца)"},
			},
			SupportsFileInput: true,
			Prompt:            docstringPrompt,
		},
		{
			Name:        "modify_1c_code",
			Description: "Отредактировать или отрефакторить код 1С по заданной инструкции. Поддерживает inline-код или чтение из файла.",
			Parameters: []SkillParam{
				{Name: "code", Required: false, Type: ParamString, Description: "Исходный inline-код 1С для редактирования (обязателен, если нет file_path)"},
				{Name: "file_path", Required: false, Type: ParamString, Description: "Путь к файлу с кодом 1С (обязателен, если нет code)"},
				{Name: "start_line", Required: false, Type: ParamInteger, Description: "Начальная строка (1-indexed, 0 = с начала)"},
				{Name: "end_line", Required: false, Type: ParamInteger, Description: "Конечная строка (0 = до конца)"},
				{Name: "instruction", Required: true, Type: ParamString, Description: "Инструкция: что изменить, как отрефакторить"},
			},
			SupportsFileInput: true,
			Prompt:            modifyCodePrompt,
		},
		{
			Name:        "generate_1c_code",
			Description: "Написать код 1С с нуля по описанию задачи. Для редактирования существующего кода используйте modify_1c_code.",
			Parameters: []SkillParam{
				{Name: "task", Required: true, Type: ParamString, Description: "Описание задачи (что нужно реализовать)"},
				{Name: "context", Required: false, Type: ParamString, Description: "Контекст: объекты метаданных, конфигурация, окружение"},
				{Name: "output_format", Required: false, Type: ParamEnum, Enum: []string{"full", "module", "procedure", "function"}, Description: "Формат вывода: full (по умолчанию), module, procedure, function"},
			},
			SupportsFileInput: false,
			Prompt:            generateCodePrompt,
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
		return fmt.Sprintf("Дай развёрнутую консультацию по вопросу. Обоснуй ответ, приведи примеры из разработки на 1С:Предприятие, укажи альтернативные подходы с плюсами и минусами. Вопрос: %s", question)
	case "expertise":
		return fmt.Sprintf("Проведи глубокий экспертный разбор. Рассмотри проблему с разных сторон: архитектурные последствия, влияние на производительность, риски при обновлении конфигурации, соответствие методическим рекомендациям 1С. Дай итоговую рекомендацию с обоснованием. Вопрос: %s", question)
	default:
		return question
	}
}

func explainPrompt(args map[string]string) string {
	element := args["syntax_element"]
	if element == "" {
		return ""
	}
	contextStr := args["context"]

	q := fmt.Sprintf("Объясни синтаксис и использование конструкции/объекта 1С:Предприятие: %s.\nОпиши:\n1. Назначение и область применения\n2. Синтаксис с примерами кода на встроенном языке\n3. Особенности использования (ограничения, нюансы)\n4. Типичные ошибки и как их избежать", element)
	if contextStr != "" {
		q += fmt.Sprintf("\nВ контексте: %s", contextStr)
	}
	return q
}

func checkCodePrompt(args map[string]string) string {
	code := args["code"]
	if code == "" {
		return ""
	}

	prompt := `Проведи комплексную проверку этого кода 1С по всем аспектам:

1. СИНТАКСИС: правильность конструкций языка, парные скобки (Если/КонецЕсли, Процедура/КонецПроцедуры), корректность обращения к методам и свойствам объектов, соответствие синтаксису встроенного языка 1С.

2. ЛОГИКА: корректность алгоритма, обработка граничных случаев (пустые значения, Null, Неопределено), потенциальные ошибки времени выполнения, корректность условий и циклов, работа с транзакциями.

3. ПРОИЗВОДИТЕЛЬНОСТЬ: эффективность запросов (индексы, временные таблицы, пакетные запросы вместо циклов), оптимальность обхода выборок (Выбрать/Выгрузить), работа с регистрами, потенциальные блокировки.

4. АРХИТЕКТУРА: разделение ответственности между модулями, правильность выбора объектов метаданных, связность и зацепление, масштабируемость.

По каждому аспекту перечисли проблемы по приоритету: КРИТИЧЕСКИЕ (приведут к ошибке), СЕРЬЁЗНЫЕ (потенциальные проблемы), РЕКОМЕНДАТЕЛЬНЫЕ (улучшения). Дай конкретные исправления для каждой проблемы.`

	return prompt + "\n\nКод 1С для анализа:\n```bsl\n" + code + "\n```"
}

func reviewCodePrompt(args map[string]string) string {
	code := args["code"]
	if code == "" {
		return ""
	}
	contextStr := args["context"]

	prompt := `Проведи полное ревью этого кода 1С по трём измерениям (НЕ проверяй синтаксические или логические ошибки — правильность кода не оценивается):

1. СТИЛЬ ОФОРМЛЕНИЯ: именование переменных, процедур и функций (в 1С принят PascalCase — каждое слово с заглавной буквы, например ИмяПеременной, ВыполнитьДействие), отступы и форматирование, длина строк, расположение операторов, наличие и качество комментариев, использование директив компиляции. Укажи, ЧТО нарушено и КАК должно быть.

2. СТАНДАРТЫ РАЗРАБОТКИ: соответствие методическому пособию по разработке 1С, стандартам оформления типовых конфигураций, правильность организации модулей (общие модули, модули объектов, модули менеджеров), корректность обработчиков событий, использование типовых механизмов платформы вместо велосипедов.

3. ЛУЧШИЕ ПРАКТИКИ: читаемость и понятность, соблюдение принципа DRY (отсутствие дублирования), модульность и переиспользуемость, поддерживаемость, следование идиоматичному стилю 1С.

Для каждого измерения укажи конкретные проблемы и рекомендации. Итоговый вердикт: общая оценка читаемости и поддерживаемости кода (хорошо/средне/плохо) с кратким обоснованием.`

	if contextStr != "" {
		prompt += fmt.Sprintf("\n\nКонтекст: %s\nУчитывай стандарты конфигурации %s.", contextStr, contextStr)
	}

	return prompt + "\n\nКод 1С для ревью:\n```bsl\n" + code + "\n```"
}

func explorePrompt(args map[string]string) string {
	query := args["query"]
	if query == "" {
		return ""
	}
	code := args["code"]

	q := fmt.Sprintf("Исследуй по запросу: %s\n\nОпиши найденные объекты метаданных, их взаимосвязи, структуру модулей, точки взаимодействия. Если запрос касается конкретного объекта конфигурации — опиши его подсистемы, реквизиты, табличные части, обработчики событий.", query)
	if code != "" {
		q += fmt.Sprintf("\n\nДополнительный код для анализа связей:\n```bsl\n%s\n```", code)
	}
	return q
}

func docstringPrompt(args map[string]string) string {
	code := args["code"]
	if code == "" {
		return ""
	}

	return fmt.Sprintf(`Сгенерируй документирующий комментарий для этого кода 1С. Комментарий должен включать:
1. Краткое описание назначения процедуры/функции
2. Описание каждого параметра: имя, тип, назначение, обязательность
3. Описание возвращаемого значения (для функций): тип и что возвращается
4. Пример использования (если уместно)

Используй стандартный для 1С формат: комментарий перед процедурой/функцией с //.

Код:
`+"```bsl"+`
%s
`+"```"+`

Верни ТОЛЬКО готовый комментарий в формате 1С, не добавляй пояснений от себя.`, code)
}

func modifyCodePrompt(args map[string]string) string {
	code := args["code"]
	instruction := args["instruction"]
	if code == "" {
		return ""
	}
	if instruction == "" {
		return ""
	}

	return fmt.Sprintf(`Отредактируй этот код 1С согласно инструкции.

ВАЖНЫЕ ПРАВИЛА:
1. Внеси ТОЛЬКО запрошенное изменение, не меняй остальную логику
2. Верни ПОЛНЫЙ код целиком (не только изменённый фрагмент) — я заменю им исходный файл
3. Сохрани оригинальное форматирование, отступы, комментарии (кроме затрагиваемых изменением)
4. Если изменение требует добавления новых переменных — объяви их в начале области видимости

Инструкция: %s

Код 1С:
`+"```bsl"+`
%s
`+"```"+`

Верни ТОЛЬКО исправленный код без пояснений (только код в блоке `+"```bsl"+`).`, instruction, code)
}

func generateCodePrompt(args map[string]string) string {
	task := args["task"]
	if task == "" {
		return ""
	}
	contextStr := args["context"]
	outputFormat := args["output_format"]
	if outputFormat == "" {
		outputFormat = "full"
	}

	contextPart := ""
	if contextStr != "" {
		contextPart = fmt.Sprintf("\n\nКонтекст: %s", contextStr)
	}

	switch outputFormat {
	case "module":
		return fmt.Sprintf("Напиши код программного модуля 1С согласно задаче. Включи все необходимые переменные уровня модуля, процедуры, функции, обработчики событий. Соблюдай стандарты оформления 1С. Не используй MCP-инструменты. Задача: %s%s", task, contextPart)
	case "procedure":
		return fmt.Sprintf("Напиши ТОЛЬКО процедуру 1С согласно задаче (одна процедура, без дополнительного кода). Добавь документирующий комментарий с описанием параметров. Соблюдай стандарты 1С. Задача: %s%s", task, contextPart)
	case "function":
		return fmt.Sprintf("Напиши ТОЛЬКО функцию 1С согласно задаче (одна функция с Возврат, без дополнительного кода). Добавь документирующий комментарий с описанием параметров и возвращаемого значения. Соблюдай стандарты 1С. Задача: %s%s", task, contextPart)
	default:
		return fmt.Sprintf("Напиши код 1С по описанию задачи. Создай полноценное решение: все необходимые процедуры, функции, переменные, обработчики событий. Соблюдай стандарты оформления 1С, добавляй документирующие комментарии к процедурам и функциям. Не используй MCP-инструменты — напиши код самостоятельно, используя свои знания.\n\nЗадача: %s%s\n\nВерни ТОЛЬКО готовый код в блоке ```bsl без пояснений.", task, contextPart)
	}
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

Для каждого блока информации укажи источник (ИТС / платформа / БСП / конфигурация).

Вопрос: %s`, query)

	return prompt
}
