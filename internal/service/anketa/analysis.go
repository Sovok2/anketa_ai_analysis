package anketa

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

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
	const maxAttempt = 5

	// Ретраи инициализации модели
	llmService := service_llm.NewInitModel(a.modelName, a.provider)
	var (
		g   *genkit.Genkit
		err error
	)
	for i := 1; i <= maxAttempt; i++ {
		if ctx.Err() != nil {
			return response, ctx.Err()
		}
		g, err = llmService.Init(ctx)
		if err == nil {
			break
		}

		// fallback после 2 неудачных попыток
		if i > 2 {
			color.Yellow(fmt.Sprintf("Переключаемся на резервную модель deepseek после %d неудачных попыток", i))
			a.modelName = "deepseek/deepseek-chat"
			llmService = service_llm.NewInitModel(a.modelName, "deepseek")
			g, err = llmService.Init(ctx)
			if err == nil {
				break // успешно инициализировали резервную модель
			}
		}

		if i == maxAttempt {
			return response, fmt.Errorf("failed to initialize model after %d attempts: %w", i, err)
		}
		time.Sleep(time.Duration(1<<uint(i-1)) * 200 * time.Millisecond) // экспоненциальная пауза
	}

	color.Yellow(fmt.Sprintf("Определена модель - %s\nОтправляем запрос к модели", a.modelName))

	// Ретраи запроса к модели (и парсинга ответа)
	userPrompt := a.buildUserPrompt(request)
	for i := 1; i <= maxAttempt; i++ {
		if ctx.Err() != nil {
			return response, ctx.Err()
		}

		resp, genErr := genkit.Generate(
			ctx,
			g,
			ai.WithSystem(config_llm.Prompt),
			ai.WithPrompt(userPrompt),
			ai.WithModelName(a.modelName),
			ai.WithOutputType(DTO_llm.Response{}),
		)

		if resp == nil {
			log.Printf("genkit.Generate returned nil resp (model=%s)", a.modelName)
		} else if resp.Usage == nil {
			log.Printf("token usage is nil (model=%s)", a.modelName)
		} else {
			log.Printf("usage in=%d out=%d", resp.Usage.InputTokens, resp.Usage.OutputTokens)
		}

		// Ошибка генерации или пустой ответ — ретраим
		if genErr != nil || resp == nil {
			if genErr != nil {
				color.Red(fmt.Sprintf("Ошибка при работе с моделью - %v", genErr))
			}
			if i == maxAttempt {
				return DTO_llm.Response{
						DetailedReport: "Произошла ошибка при анализе",
						Resume:         "Произошла ошибка при анализе",
					},
					fmt.Errorf("generation failed after %d attempts: %w", i, genErr)
			}
			time.Sleep(time.Duration(1<<uint(i-1)) * 200 * time.Millisecond)
			continue
		}

		// Парсинг вывода — тоже ретраим при ошибке
		if outErr := resp.Output(&response); outErr != nil {
			color.Red(fmt.Sprintf("Ошибка при парсинге ответа от модели - %v", outErr))
			if i == maxAttempt {
				return DTO_llm.Response{
						DetailedReport: "Произошла ошибка при парсинге ответов от ИИ",
						Resume:         "Произошла ошибка при парсинге ответов от ИИ",
					},
					fmt.Errorf("parse failed after %d attempts: %w", i, outErr)
			}
			time.Sleep(time.Duration(1<<uint(i-1)) * 200 * time.Millisecond)
			continue
		}

		color.Green("Ответ от ИИ был успешно получен!")
		return response, nil
	}

	return response, errors.New("unreachable")
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
