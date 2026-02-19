package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds application settings sourced from environment variables.
type Config struct {
	TelegramToken string
	YandexToken   string
	LogLevel      string
	// MetricsAddr is the listen address for the metrics HTTP server (e.g. ":9090"). Empty disables metrics server.
	MetricsAddr string
}

// Load reads configuration from the environment.
func Load() (Config, error) {
	cfg := Config{
		TelegramToken: strings.TrimSpace(os.Getenv("TELEGRAM_TOKEN")),
		YandexToken:   strings.TrimSpace(os.Getenv("YANDEX_TOKEN")),
		LogLevel:      strings.TrimSpace(os.Getenv("LOG_LEVEL")),
		MetricsAddr:   strings.TrimSpace(os.Getenv("METRICS_ADDR")),
	}

	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}
	if cfg.MetricsAddr == "" {
		if p := os.Getenv("METRICS_PORT"); p != "" {
			if _, err := strconv.Atoi(p); err == nil {
				cfg.MetricsAddr = ":" + p
			}
		}
	}
	if os.Getenv("METRICS_DISABLE") == "1" {
		cfg.MetricsAddr = ""
	} else if cfg.MetricsAddr == "" {
		cfg.MetricsAddr = ":9090"
	}

	if cfg.TelegramToken == "" {
		return cfg, fmt.Errorf("TELEGRAM_TOKEN is not set")
	}

	return cfg, nil
}

