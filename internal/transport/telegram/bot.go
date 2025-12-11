package telegram

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"

	"ym-bot/internal/services/music"
)

const (
	callbackPrefix = "download:"
	searchLimit    = 10
)

// Bot wraps Telegram API interactions.
type Bot struct {
	api          *tgbotapi.BotAPI
	musicService *music.Service
	logger       *zap.Logger
}

// NewBot constructs a bot instance with inline mode enabled.
func NewBot(token string, musicService *music.Service, logger *zap.Logger) (*Bot, error) {
	if musicService == nil {
		return nil, fmt.Errorf("music service is nil")
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}
	api.Debug = false

	return &Bot{
		api:          api,
		musicService: musicService,
		logger:       logger,
	}, nil
}

// Start begins long polling and handles incoming updates.
func (b *Bot) Start(ctx context.Context) error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 10

	updates := b.api.GetUpdatesChan(u)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case update := <-updates:
			if update.InlineQuery != nil {
				go b.handleInlineQuery(ctx, update.InlineQuery)
			} else if update.CallbackQuery != nil {
				go b.handleCallback(ctx, update.CallbackQuery)
			}
		}
	}
}

func (b *Bot) handleInlineQuery(ctx context.Context, q *tgbotapi.InlineQuery) {
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()

	query := strings.TrimSpace(q.Query)
	if query == "" {
		return
	}

	offset := 0
	if q.Offset != "" {
		if v, err := strconv.Atoi(q.Offset); err == nil && v >= 0 {
			offset = v
		}
	}

	tracks, err := b.musicService.Search(ctx, query, searchLimit, offset)
	if err != nil {
		b.logger.Warn("search failed", zap.String("query", query), zap.Error(err))
		return
	}

	results := make([]interface{}, 0, len(tracks))
	for _, track := range tracks {
		// Fetch meta + direct url; Telegram will send audio directly from URL.
		meta, url, err := b.musicService.StreamURL(ctx, track.ID)
		if err != nil || url == "" {
			b.logger.Debug("skip track: no direct url", zap.String("trackID", track.ID), zap.Error(err))
			continue
		}

		audio := tgbotapi.NewInlineQueryResultAudio(meta.ID, url, meta.Title)
		audio.Performer = meta.ArtistsString()
		audio.Caption = fmt.Sprintf("%s — %s", meta.Title, meta.ArtistsString())
		results = append(results, audio)
	}

	ans := tgbotapi.InlineConfig{
		InlineQueryID: q.ID,
		IsPersonal:    true,
		CacheTime:     0,
		Results:       results,
		NextOffset:    strconv.Itoa(offset + len(results)),
	}

	if _, err := b.api.Request(ans); err != nil {
		b.logger.Warn("answer inline failed", zap.String("query", query), zap.Error(err))
	}
}

func (b *Bot) handleCallback(ctx context.Context, cb *tgbotapi.CallbackQuery) {
	if cb.Data == "" || !strings.HasPrefix(cb.Data, callbackPrefix) {
		return
	}

	trackID := strings.TrimPrefix(cb.Data, callbackPrefix)

	var chatID int64
	if cb.Message != nil && cb.Message.Chat != nil {
		chatID = cb.Message.Chat.ID
	} else {
		// Inline keyboard callbacks may omit message; fall back to sender.
		chatID = cb.From.ID
	}

	// Immediately acknowledge to avoid Telegram timeout.
	ack := tgbotapi.NewCallback(cb.ID, "Готовим ваш трек…")
	if _, err := b.api.Request(ack); err != nil {
		b.logger.Warn("callback ack failed", zap.Error(err))
	}

	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	meta, path, err := b.musicService.DownloadTrack(ctx, trackID)
	if err != nil {
		b.logger.Warn("download failed", zap.String("trackID", trackID), zap.Error(err))
		b.sendAlert(cb, "Не удалось скачать трек :(")
		return
	}
	defer os.RemoveAll(filepath.Dir(path))

	audio := tgbotapi.NewAudio(chatID, tgbotapi.FilePath(path))
	audio.Duration = meta.DurationSeconds
	audio.Performer = meta.ArtistsString()
	audio.Title = meta.Title
	audio.Caption = fmt.Sprintf("%s — %s", meta.Title, meta.ArtistsString())

	if _, err := b.api.Send(audio); err != nil {
		b.logger.Warn("send audio failed", zap.String("trackID", trackID), zap.Error(err))
		b.sendAlert(cb, "Не удалось отправить аудио :(")
		return
	}
}

func (b *Bot) sendAlert(cb *tgbotapi.CallbackQuery, text string) {
	alert := tgbotapi.NewCallbackWithAlert(cb.ID, text)
	if _, err := b.api.Request(alert); err != nil {
		b.logger.Warn("callback alert failed", zap.Error(err))
	}
}
