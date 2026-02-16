package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"bugreportbot/internal/analysis"
	"bugreportbot/internal/config"
	"bugreportbot/internal/telegram"
)

func main() {
	// Завантажуємо .env з кореня проєкту (якщо є), щоб ANALYSIS_MODE=ollama тощо працювали
	_ = godotenv.Load()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	botAPI, err := tgbotapi.NewBotAPI(cfg.BotToken)
	if err != nil {
		log.Fatalf("failed to create telegram bot api: %v", err)
	}

	log.Printf("authorized on account: @%s", botAPI.Self.UserName)

	var analyzer analysis.Analyzer
	switch cfg.AnalysisMode {
	case "ollama":
		log.Printf("analysis mode: ollama (url=%s model=%s)", cfg.OllamaURL, cfg.OllamaModel)
		if err := analysis.CheckOllamaReachable(cfg.OllamaURL); err != nil {
			log.Printf("WARNING: %v", err)
			log.Printf("Start Ollama (open the app or run: ollama serve), then send a photo again. Until then you will get sample templates.")
		} else {
			log.Printf("Ollama is reachable; AI analysis enabled.")
		}
		analyzer = analysis.NewOllamaAnalyzer(cfg.OllamaURL, cfg.OllamaModel)
	case "mock":
		fallthrough
	default:
		log.Printf("analysis mode: mock")
		analyzer = analysis.NewMockAnalyzer()
	}

	bot := telegram.NewBot(botAPI, analyzer)

	if err := bot.Run(ctx); err != nil && err != context.Canceled {
		log.Fatalf("bot stopped with error: %v", err)
	}
}

