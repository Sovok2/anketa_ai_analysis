package llm

type Response struct {
	DetailedReport string `json:"detailed_report" jsonschema_description:"Детальный отчет об анализе ответов студента"`
	Resume         string `json:"resume" jsonschema_description:"Краткое итоговое резюме по студенту"`
	Category       string `json:"category" jsonschema_description:"Категория к которой отностится студент"`
}
