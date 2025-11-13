package llm

import (
	"anketa_ai_analysis/internal/config/llm/prompt"
)

var PromptMap = map[string]string{
	"Девиантное поведение":              prompt.DiviantPrompt,
	"Индекс толерантности":              prompt.ToleranceIndexPrompt,
	"Адаптированность студентов в вузе": prompt.AdaptabilityMonitoring,
	"Суицидальный риск":                 prompt.SuicideRisk,
	"Шкала самоуважения Розенберга":     prompt.Rosenberg,
	"Опросник Г. Айзенка":               prompt.Aizek,
}
