package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
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
		log.Printf("Error loading .env file: %v", err)
	}
	color.Green("✅ .env loaded successfully")

	// Читаем конфигурацию
	modelName := os.Getenv("MODEL")
	provider := os.Getenv("PROVIDER")
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
	r.Use(middleware.Timeout(5 * time.Minute))

	// Хендлер
	r.Post("/api/analysis", handler.NewAnalysisHandler(svc))

	// Healthcheck
	r.Get("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// Создаем сервер с настройками
	addr := ":" + port
	server := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	// Канал для graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Запускаем сервер в горутине
	go func() {
		color.Magenta("🌐 Server starting on http://localhost%s", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("❌ Server failed to start: %v", err)
		}
	}()

	// Ждем сигнал завершения
	<-stop
	color.Yellow("\n🛑 Received shutdown signal...")

	// Graceful shutdown с таймаутом
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("❌ Server shutdown error: %v", err)
	} else {
		color.Green("✅ Server stopped gracefully")
	}
}
