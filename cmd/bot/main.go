package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/joho/godotenv"
	"go.uber.org/zap"

	"ym-bot/internal/client/yandex"
	"ym-bot/internal/config"
	"ym-bot/internal/metrics"
	"ym-bot/internal/services/music"
	"ym-bot/internal/transport/telegram"
	"ym-bot/internal/utils"
)

func main() {
	// Load .env when running locally; ignored if file is absent.
	_ = godotenv.Load()

	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	logger, err := utils.NewLogger(cfg.LogLevel)
	if err != nil {
		log.Fatalf("logger: %v", err)
	}
	defer logger.Sync() // best-effort flush

	if cfg.TelegramToken == "" {
		logger.Fatal("TELEGRAM_TOKEN is required")
	}

	m := metrics.New()
	if cfg.MetricsAddr != "" {
		mux := http.NewServeMux()
		mux.Handle("/metrics", m.Handler())
		srv := &http.Server{Addr: cfg.MetricsAddr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
		go func() {
			logger.Info("metrics server listening", zap.String("addr", cfg.MetricsAddr))
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.Warn("metrics server stopped", zap.Error(err))
			}
		}()
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = srv.Shutdown(shutdownCtx)
		}()
	}

	httpClient := &http.Client{Timeout: 20 * time.Second}
	ymClient := yandex.NewClient(httpClient, cfg.YandexToken, logger)
	musicService := music.NewService(ymClient, logger)

	bot, err := telegram.NewBot(cfg.TelegramToken, musicService, logger, m)
	if err != nil {
		logger.Fatal("telegram init failed", zap.Error(err))
	}

	logger.Info("bot is starting")
	if err := bot.Start(ctx); err != nil {
		logger.Fatal("bot stopped with error", zap.Error(err))
	}
}

