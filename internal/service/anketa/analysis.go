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
	"anketa_ai_analysis/pkg/helper"
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
	const maxAttempts = 5

	// Инициализация модели с ретраями
	llmService := service_llm.NewInitModel(a.modelName, a.provider)
	var (
		g   *genkit.Genkit
		err error
	)

	// делаем анонимную функцию в аргумент
	g, err = helper.RunWithRetryInitModel(func() (*genkit.Genkit, error) {
		return llmService.Init(ctx)
	}, maxAttempts)

	if err != nil {
		color.Yellow("Не получилось инициализировать модель ИИ, переходим на резервную модель - deepseek")
		llmService := service_llm.NewInitModel("deepseek/deepseek-chat", "deepseek")
		g, err = helper.RunWithRetryInitModel(func() (*genkit.Genkit, error) {
			return llmService.Init(ctx)
		}, maxAttempts)

		if err != nil {
			color.Red("ВСЕ ПОПЫТКИ ИНИЦИАЛИЗИРОВАТЬ РЕЗЕРВНУЮ МОДЕЛЬ ПРОВАЛИЛИСЬ! ОШИБКА - %v", err)
			return response, fmt.Errorf("ERROR in INIT MODEL: %w", err)
		}

		// Переопределяем поля
		a.modelName = "deepseek/deepseek-chat"
		a.provider = "deepseek"
	}
	color.Green(fmt.Sprintf("Успешно определена ИИ модель - %s\nОтправляем запрос", a.modelName))

	// Ретраи запроса к модели (и парсинга ответа)
	userPrompt := a.buildUserPrompt(request)

	resp, genErr := helper.RunWithRetryGenerateResponse(
		ctx,
		func() (*ai.ModelResponse, error) {
			return genkit.Generate(
				ctx,
				g,
				ai.WithSystem(config_llm.Prompt),
				ai.WithPrompt(userPrompt),
				ai.WithModelName(a.modelName),
				ai.WithOutputType(DTO_llm.Response{}),
			)
		},
		maxAttempts,
	)

	if genErr != nil {
		return DTO_llm.Response{
			DetailedReport: "Произошла ошибка при анализе",
			Resume:         "Произошла ошибка при анализе",
		}, genErr
	}

	// Парсинг вывода
	if outErr := resp.Output(&response); outErr != nil {
		color.Red(fmt.Sprintf("Ошибка при парсинге ответа от модели - %v", outErr))
		return DTO_llm.Response{
			DetailedReport: "Произошла ошибка при парсинге ответов от ИИ",
			Resume:         "Произошла ошибка при парсинге ответов от ИИ",
		}, fmt.Errorf("parse failed: %w", outErr)
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
