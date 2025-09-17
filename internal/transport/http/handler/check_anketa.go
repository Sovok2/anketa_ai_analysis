package handler

import (
	"context"
	"encoding/json"
	"errors"
	stdhttp "net/http"
	"strings"
	"time"

	DTO_http "anketa_ai_analysis/internal/DTO/http"
	"anketa_ai_analysis/internal/service/anketa"
)

// форматы ошибок
type errorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}

// фабрика хендлера: пробрасываем зависимость на сервис аналитики
func NewAnalysisHandler(svc anketa.Analysis) stdhttp.HandlerFunc {
	return func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		// только JSON
		if ct := r.Header.Get("Content-Type"); ct != "" && !strings.Contains(ct, "application/json") {
			writeJSON(w, stdhttp.StatusUnsupportedMediaType, errorResponse{
				Error:   "unsupported_media_type",
				Details: "Content-Type must be application/json",
			})
			return
		}

		// парсим тело
		var req DTO_http.Request
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&req); err != nil {
			writeJSON(w, stdhttp.StatusBadRequest, errorResponse{
				Error:   "invalid_request",
				Details: err.Error(),
			})
			return
		}

		// валидация (минимальная)
		if len(req.Answers) == 0 {
			writeJSON(w, stdhttp.StatusBadRequest, errorResponse{
				Error:   "validation_error",
				Details: "answers must contain at least one element",
			})
			return
		}
		if strings.TrimSpace(req.Answers[0].QuestionText) == "" {
			writeJSON(w, stdhttp.StatusBadRequest, errorResponse{
				Error:   "validation_error",
				Details: "question_text is required",
			})
			return
		}

		// контекст с таймаутом на запрос к LLM
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
		defer cancel()

		// дергаем твою аналитику
		// NB: интерфейс из предыдущего шага: Analysis(ctx context.Context) (DTO_llm.Response, error)
		out, err := svc.Analysis(ctx, req)
		if err != nil {
			writeJSON(w, statusFromError(err), errorResponse{
				Error:   "analysis_failed",
				Details: err.Error(),
			})
			return
		}

		// успех — отдаем как есть (DTO_llm.Response должен иметь json-теги)
		writeJSON(w, stdhttp.StatusOK, out)
	}
}

// маппинг ошибок в HTTP-коды (упрощённо)
func statusFromError(err error) int {
	if err == nil {
		return stdhttp.StatusOK
	}
	// примеры для расширения:
	var timeout interface{ Timeout() bool }
	if errors.As(err, &timeout) && timeout.Timeout() {
		return stdhttp.StatusGatewayTimeout
	}
	// по умолчанию — 500
	return stdhttp.StatusInternalServerError
}

func writeJSON(w stdhttp.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
