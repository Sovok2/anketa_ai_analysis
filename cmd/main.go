package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/fatih/color"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/joho/godotenv"

	// хендлер
	"anketa_ai_analysis/internal/transport/http/handler"

	// сервис аналитики
	"anketa_ai_analysis/internal/service/anketa"
)

func main() {
	color.Cyan("🚀 Starting Anketa Analysis Service...")

	// Устанавливаем прокси
	proxy := "socks5://127.0.0.1:1080"
	os.Setenv("ALL_PROXY", proxy)
	log.Printf("🌐 Proxy set: %s", proxy)

	// Загружаем .env
	color.Yellow("📦 Loading .env file...")
	if err := godotenv.Load(); err != nil {
		log.Fatalf("❌ Error loading .env file: %v", err)
	}
	color.Green("✅ .env loaded successfully")

	// Читаем конфигурацию
	modelName := os.Getenv("DEEPSEEK_CHAT")
	provider := os.Getenv("DEEPSEEK")
	port := os.Getenv("PORT")

	color.Blue("🔧 Configuration:")
	log.Printf("   MODEL_NAME: %s", modelName)
	log.Printf("   PROVIDER:   %s", provider)
	log.Printf("   PORT:       %s", port)

	// Инициализируем сервис
	color.Yellow("🔌 Initializing analysis service...")
	svc := anketa.NewAnalysis(modelName, provider)
	color.Green("✅ Service initialized")

	// Настраиваем роутер
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Logger)
	r.Use(middleware.Timeout(60 * time.Second))

	// Хендлер
	r.Post("/analysis", handler.NewAnalysisHandler(svc))

	// Healthcheck
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// Запуск сервера
	addr := ":" + port
	color.Magenta("🌐 Server starting on http://localhost%s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("❌ Server failed to start: %v", err)
	}
}
