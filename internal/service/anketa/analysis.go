package anketa

import (
	"context"
	"fmt"

	"github.com/fatih/color"
	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"

	DTO_http "anketa_ai_analysis/internal/DTO/http"
	DTO_llm "anketa_ai_analysis/internal/DTO/llm"
	config_llm "anketa_ai_analysis/internal/config/llm"
	service_llm "anketa_ai_analysis/internal/service/llm"
)

type analysis struct {
	modelName string
	provider  string
}

type Analysis interface {
	Analysis(ctx context.Context, request DTO_http.Request) (DTO_llm.Response, error)
}

func NewAnalysis(modelName string, provider string) Analysis {
	return &analysis{
		modelName: modelName,
		provider:  provider,
	}
}

func (a *analysis) Analysis(ctx context.Context, request DTO_http.Request) (DTO_llm.Response, error) {
	var response DTO_llm.Response

	// Инициализация модели
	llmService := service_llm.NewInitModel(a.modelName, a.provider)
	g, err := llmService.Init(ctx)
	if err != nil {
		return response, fmt.Errorf("failed to initialize model: %w", err)
	}

	color.Yellow(fmt.Sprintf("Определена модель - %s\nОтправляем запрос к модели", a.modelName))
	// Запрос к модели
	userPrompt := a.buildUserPrompt(request)
	resp, err := genkit.Generate(ctx, g,
		ai.WithSystem(config_llm.Prompt),
		ai.WithPrompt(userPrompt),
		ai.WithModelName(a.modelName),
		ai.WithOutputType(DTO_llm.Response{}),
	)

	if resp == nil {
		fmt.Println("resp is nil")
	}
	if resp.Usage == nil {
		fmt.Println("resp.Usage is null")
	}
	fmt.Printf("resp.Usage.InputTokens: %v\n", resp.Usage.InputTokens)
	fmt.Printf("resp.Usage.OutputTokens: %v\n", resp.Usage.OutputTokens)

	if err != nil {
		color.Red(fmt.Sprintf("Ошибка при работе с моделью - %v", err))
		return DTO_llm.Response{
			DetailedReport: "Произошла ошибка при анализе",
			Resume:         "Произошла ошибка при анализе",
		}, fmt.Errorf("generation failed: %w", err)
	}

	if err := resp.Output(&response); err != nil {
		color.Red(fmt.Sprintf("Ошибка при парсинге ответа от модели - %v", err))
		return DTO_llm.Response{
			DetailedReport: "Произошла ошибка при парсинге ответов от ИИ",
			Resume:         "Произошла ошибка при парсинге ответов от ИИ",
		}, fmt.Errorf("generation failed: %w", err)
	}

	color.Green("Ответ от ИИ был успешно получен!")
	return response, nil
}

func (a *analysis) buildUserPrompt(request DTO_http.Request) string {
	var userPrompt string

	for index, questionData := range request.Answers {
		data := fmt.Sprintf(
			"Вопрос - %d.\nТекст вопроса - %s\nОтвет студента - %s\nВремя ответа - %s\n",
			index+1,
			questionData.QuestionText,
			questionData.Answer,
			questionData.Time,
		)
		userPrompt += data // добавляем к строке
	}

	return userPrompt
}
