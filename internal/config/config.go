package config

import (
	"fmt"
	"os"
	"strings"
)

// Config holds application settings sourced from environment variables.
type Config struct {
	TelegramToken string
	YandexToken   string
	LogLevel      string
}

// Load reads configuration from the environment.
func Load() (Config, error) {
	cfg := Config{
		TelegramToken: strings.TrimSpace(os.Getenv("TELEGRAM_TOKEN")),
		YandexToken:   strings.TrimSpace(os.Getenv("YANDEX_TOKEN")),
		LogLevel:      strings.TrimSpace(os.Getenv("LOG_LEVEL")),
	}

	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}

	if cfg.TelegramToken == "" {
		return cfg, fmt.Errorf("TELEGRAM_TOKEN is not set")
	}

	return cfg, nil
}

