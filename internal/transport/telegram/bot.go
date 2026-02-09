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

	yandex "ym-bot/internal/client/yandex"
	"ym-bot/internal/metrics"
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
	metrics      *metrics.Metrics
}

// NewBot constructs a bot instance with inline mode enabled.
func NewBot(token string, musicService *music.Service, logger *zap.Logger, m *metrics.Metrics) (*Bot, error) {
	if musicService == nil {
		return nil, fmt.Errorf("music service is nil")
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	if m == nil {
		m = metrics.New()
	}

	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}
	api.Debug = false

	return &Bot{
		api:          api,
		musicService: musicService,
		logger:       logger.With(zap.String("component", "telegram.bot")),
		metrics:      m,
	}, nil
}

// Start begins long polling and handles incoming updates.
func (b *Bot) Start(ctx context.Context) error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 10
	updates := b.api.GetUpdatesChan(u)
	b.logger.Info("long polling started")

	for {
		select {
		case <-ctx.Done():
			b.logger.Info("shutting down", zap.Error(ctx.Err()))
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
		b.logger.Debug("inline query skipped: empty query", zap.String("inline_query_id", q.ID))
		return
	}

	normalized := strings.TrimSpace(strings.TrimLeft(strings.ToLower(query), "/"))

	// Handle special commands: тренды, new, and genres.
	if normalized == "тренды" || normalized == "new" {
		b.handleChartsInlineQuery(ctx, q, normalized)
		return
	}

	// Handle genre commands
	genres := map[string]bool{
		"rock": true, "рок": true,
		"trance": true, "транс": true,
		"pop": true, "поп": true,
		"hip-hop": true, "хип-хоп": true,
		"jazz": true, "джаз": true,
		"classical": true, "классика": true,
		"electronic": true, "электроника": true,
		"metal": true, "метал": true,
	}
	if genres[normalized] {
		// Map Russian names to English for API
		genreMap := map[string]string{
			"рок": "rock", "rock": "rock",
			"транс": "trance", "trance": "trance",
			"поп": "pop", "pop": "pop",
			"хип-хоп": "hip-hop", "hip-hop": "hip-hop",
			"джаз": "jazz", "jazz": "jazz",
			"классика": "classical", "classical": "classical",
			"электроника": "electronic", "electronic": "electronic",
			"метал": "metal", "metal": "metal",
		}
		b.handleGenreInlineQuery(ctx, q, genreMap[normalized])
		return
	}

	b.metrics.SearchesTotal.Inc()
	logFields := []zap.Field{
		zap.String("query", query),
		zap.String("inline_query_id", q.ID),
		zap.Int64("user_id", q.From.ID),
	}

	offset := 0
	if q.Offset != "" {
		if v, err := strconv.Atoi(q.Offset); err == nil && v >= 0 {
			offset = v
		}
	}

	tracks, err := b.musicService.Search(ctx, query, searchLimit, offset)
	if err != nil {
		b.metrics.SearchesFailed.Inc()
		b.logger.Warn("search failed", append(logFields, zap.Error(err))...)
		return
	}

	b.logger.Debug("search completed", append(logFields, zap.Int("tracks_found", len(tracks)), zap.Int("offset", offset))...)

	results := make([]interface{}, 0, len(tracks))
	for _, track := range tracks {
		meta, url, err := b.musicService.StreamURL(ctx, track.ID)
		if err != nil || url == "" {
			b.metrics.StreamURLFailed.Inc()
			b.logger.Debug("skip track: no direct url", zap.String("track_id", track.ID), zap.Error(err))
			continue
		}
		b.metrics.StreamURLRequests.Inc()

		audio := tgbotapi.NewInlineQueryResultAudio(meta.ID, url, meta.Title)
		audio.Performer = meta.ArtistsString()
		results = append(results, audio)
	}

	b.metrics.InlineResultsTotal.Add(float64(len(results)))

	ans := tgbotapi.InlineConfig{
		InlineQueryID: q.ID,
		IsPersonal:    true,
		CacheTime:     0,
		Results:       results,
		NextOffset:    strconv.Itoa(offset + len(results)),
	}

	if _, err := b.api.Request(ans); err != nil {
		b.metrics.SearchesFailed.Inc()
		b.logger.Warn("answer inline failed", append(logFields, zap.Error(err))...)
		return
	}
	b.logger.Info("inline query answered", append(logFields, zap.Int("results_count", len(results)))...)
}

// handleChartsInlineQuery serves inline queries for charts (/trending, /new).
func (b *Bot) handleChartsInlineQuery(ctx context.Context, q *tgbotapi.InlineQuery, featureType string) {
	logFields := []zap.Field{
		zap.String("feature_type", featureType),
		zap.String("inline_query_id", q.ID),
		zap.Int64("user_id", q.From.ID),
	}

	const chartsLimit = 10

	var (
		tracks []musicTrackWrapper
		err    error
	)

	switch featureType {
	case "тренды":
		b.logger.Info("inline charts request: тренды", logFields...)
		t, svcErr := b.musicService.TopChart(ctx, chartsLimit)
		err = svcErr
		tracks = wrapTracks(t)
	case "new":
		b.logger.Info("inline charts request: new", logFields...)
		t, svcErr := b.musicService.NewReleases(ctx, chartsLimit)
		err = svcErr
		tracks = wrapTracks(t)
	default:
		b.logger.Warn("unknown charts feature type", logFields...)
		return
	}

	if err != nil {
		b.logger.Error("charts request failed", append(logFields, zap.Error(err))...)
		// Do not send any results to avoid confusing the user; Telegram will show "no results".
		return
	}

	b.metrics.ChartsRequests.WithLabelValues(featureType).Inc()

	results := make([]interface{}, 0, len(tracks))
	for _, t := range tracks {
		// Reuse existing StreamURL logic to obtain direct URL.
		meta, url, err := b.musicService.StreamURL(ctx, t.ID)
		if err != nil || url == "" {
			b.metrics.StreamURLFailed.Inc()
			b.logger.Debug("skip chart track: no direct url", append(logFields, zap.String("track_id", t.ID), zap.Error(err))...)
			continue
		}
		b.metrics.StreamURLRequests.Inc()

		audio := tgbotapi.NewInlineQueryResultAudio(meta.ID, url, meta.Title)
		audio.Performer = meta.ArtistsString()
		results = append(results, audio)
	}

	if len(results) == 0 {
		b.logger.Info("charts inline query answered with empty results", logFields...)
		return
	}

	ans := tgbotapi.InlineConfig{
		InlineQueryID: q.ID,
		IsPersonal:    true,
		CacheTime:     60,
		Results:       results,
	}

	if _, err := b.api.Request(ans); err != nil {
		b.logger.Warn("answer charts inline failed", append(logFields, zap.Error(err))...)
		return
	}

	b.logger.Info("charts inline query answered", append(logFields, zap.Int("results_count", len(results)))...)
}

// handleGenreInlineQuery serves inline queries for genre playlists (/rock, /pop, etc.)
func (b *Bot) handleGenreInlineQuery(ctx context.Context, q *tgbotapi.InlineQuery, genre string) {
	logFields := []zap.Field{
		zap.String("genre", genre),
		zap.String("inline_query_id", q.ID),
		zap.Int64("user_id", q.From.ID),
	}

	const genreLimit = 10

	b.logger.Info("inline genre request", logFields...)
	tracks, err := b.musicService.GetGenreTracks(ctx, genre, genreLimit)
	if err != nil {
		b.logger.Error("genre request failed", append(logFields, zap.Error(err))...)
		return
	}

	b.metrics.GenreRequests.WithLabelValues(genre).Inc()

	results := make([]interface{}, 0, len(tracks))
	for _, t := range tracks {
		meta, url, err := b.musicService.StreamURL(ctx, t.ID)
		if err != nil || url == "" {
			b.metrics.StreamURLFailed.Inc()
			b.logger.Debug("skip genre track: no direct url", append(logFields, zap.String("track_id", t.ID), zap.Error(err))...)
			continue
		}
		b.metrics.StreamURLRequests.Inc()

		audio := tgbotapi.NewInlineQueryResultAudio(meta.ID, url, meta.Title)
		audio.Performer = meta.ArtistsString()
		results = append(results, audio)
	}

	if len(results) == 0 {
		b.logger.Info("genre inline query answered with empty results", logFields...)
		return
	}

	ans := tgbotapi.InlineConfig{
		InlineQueryID: q.ID,
		IsPersonal:    true,
		CacheTime:     60,
		Results:       results,
	}

	if _, err := b.api.Request(ans); err != nil {
		b.logger.Warn("answer genre inline failed", append(logFields, zap.Error(err))...)
		return
	}

	b.logger.Info("genre inline query answered", append(logFields, zap.Int("results_count", len(results)))...)
}

// musicTrackWrapper is a tiny helper to adapt tracks without importing yandex package here.
type musicTrackWrapper struct {
	ID string
}

func wrapTracks(ts []yandex.Track) []musicTrackWrapper {
	out := make([]musicTrackWrapper, 0, len(ts))
	for _, t := range ts {
		out = append(out, musicTrackWrapper{ID: t.ID})
	}
	return out
}

func (b *Bot) handleCallback(ctx context.Context, cb *tgbotapi.CallbackQuery) {
	if cb.Data == "" || !strings.HasPrefix(cb.Data, callbackPrefix) {
		return
	}

	trackID := strings.TrimPrefix(cb.Data, callbackPrefix)
	logFields := []zap.Field{
		zap.String("track_id", trackID),
		zap.String("callback_id", cb.ID),
		zap.Int64("user_id", cb.From.ID),
	}

	var chatID int64
	if cb.Message != nil && cb.Message.Chat != nil {
		chatID = cb.Message.Chat.ID
	} else {
		chatID = cb.From.ID
	}

	ack := tgbotapi.NewCallback(cb.ID, "Готовим ваш трек…")
	if _, err := b.api.Request(ack); err != nil {
		b.logger.Warn("callback ack failed", append(logFields, zap.Error(err))...)
	}

	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	b.logger.Info("download started", logFields...)
	meta, path, err := b.musicService.DownloadTrack(ctx, trackID)
	if err != nil {
		b.metrics.DownloadsFailed.Inc()
		b.logger.Warn("download failed", append(logFields, zap.Error(err))...)
		b.sendAlert(cb, "Не удалось скачать трек :(")
		return
	}
	defer os.RemoveAll(filepath.Dir(path))

	audio := tgbotapi.NewAudio(chatID, tgbotapi.FilePath(path))
	audio.Duration = meta.DurationSeconds
	audio.Performer = meta.ArtistsString()
	audio.Title = meta.Title

	if _, err := b.api.Send(audio); err != nil {
		b.metrics.DownloadsFailed.Inc()
		b.logger.Warn("send audio failed", append(logFields, zap.Error(err))...)
		b.sendAlert(cb, "Не удалось отправить аудио :(")
		return
	}

	b.metrics.DownloadsTotal.Inc()
	b.logger.Info("track sent",
		append(logFields,
			zap.String("title", meta.Title),
			zap.String("artists", meta.ArtistsString()),
			zap.Int("duration_sec", meta.DurationSeconds),
		)...)
}

func (b *Bot) sendAlert(cb *tgbotapi.CallbackQuery, text string) {
	alert := tgbotapi.NewCallbackWithAlert(cb.ID, text)
	if _, err := b.api.Request(alert); err != nil {
		b.logger.Warn("callback alert failed", zap.Error(err))
	}
}
