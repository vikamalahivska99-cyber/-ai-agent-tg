package config

import (
	"fmt"
	"os"
)

// Config зберігає базові налаштування бота.
type Config struct {
	BotToken string
	AnalysisMode string

	// Ollama settings (used when AnalysisMode == "ollama")
	OllamaURL   string
	OllamaModel string
}

// Load читає конфігурацію зі змінних середовища.
// Обов'язкові змінні:
//   - TELEGRAM_BOT_TOKEN
func Load() (*Config, error) {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN is not set")
	}

	mode := os.Getenv("ANALYSIS_MODE")
	if mode == "" {
		mode = "mock"
	}

	ollamaURL := os.Getenv("OLLAMA_URL")
	if ollamaURL == "" {
		// 127.0.0.1 краще працює на Windows ніж localhost (уникаємо IPv6)
		ollamaURL = "http://127.0.0.1:11434"
	}
	ollamaModel := os.Getenv("OLLAMA_MODEL")
	if ollamaModel == "" {
		ollamaModel = "llava"
	}

	return &Config{
		BotToken:      token,
		AnalysisMode:  mode,
		OllamaURL:     ollamaURL,
		OllamaModel:   ollamaModel,
	}, nil
}

