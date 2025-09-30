package helper

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/fatih/color"
	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
)

func RunWithRetryInitModel(llmService func() (*genkit.Genkit, error), maxAttempts int) (*genkit.Genkit, error) {
	var g *genkit.Genkit
	var err error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		g, err = llmService()

		// когда нам прилетает ошибка, пытаемся еще раз на след итерации с задержкой
		if err != nil {
			color.Yellow(fmt.Sprintf("Попытка %d/%d не удалась: %v\n", attempt, maxAttempts, err))
			time.Sleep(2 * time.Second)
			continue
		}

		// логируем успех
		color.Green(fmt.Sprintf("Успешно получили результат %d/%d попытки\n", attempt, maxAttempts))
		return g, nil
	}
	return g, fmt.Errorf("После %d попыток не удалось получить результат функции. Ошибка - %v", maxAttempts, err)
}

func RunWithRetryGenerateResponse(
	ctx context.Context,
	llmService func() (*ai.ModelResponse, error),
	maxAttempts int,
) (*ai.ModelResponse, error) {
	var resp *ai.ModelResponse
	var err error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		// Проверяем контекст перед каждой попыткой
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		resp, err = llmService()

		// Логируем информацию о токенах, если ответ получен
		if resp != nil {
			if resp.Usage == nil {
				log.Printf("token usage is nil")
			} else {
				log.Printf("usage in=%d out=%d", resp.Usage.InputTokens, resp.Usage.OutputTokens)
			}
		} else {
			log.Printf("genkit.Generate returned nil resp")
		}

		// Если есть ошибка или пустой ответ - ретраим
		if err != nil || resp == nil {
			if err != nil {
				color.Red(fmt.Sprintf("Ошибка при работе с моделью - %v", err))
			}

			// На последней попытке возвращаем ошибку
			if attempt == maxAttempts {
				return nil, fmt.Errorf("generation failed after %d attempts: %w", attempt, err)
			}

			// Экспоненциальная задержка перед повтором
			delay := time.Duration(1<<uint(attempt-1)) * 200 * time.Millisecond
			color.Yellow(fmt.Sprintf("Попытка %d/%d не удалась, повтор через %v\n", attempt, maxAttempts, delay))
			time.Sleep(delay)
			continue
		}

		// Успех
		color.Green(fmt.Sprintf("Успешно получили результат %d/%d попытки\n", attempt, maxAttempts))
		return resp, nil
	}

	return nil, fmt.Errorf("После %d попыток не удалось получить результат функции. Ошибка - %v", maxAttempts, err)
}
