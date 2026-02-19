package music

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"

	"ym-bot/internal/client/yandex"
)

// Service orchestrates music search and download workflow.
type Service struct {
	client yandex.Client
	logger *zap.Logger
}

// NewService constructs a music service instance.
func NewService(client yandex.Client, logger *zap.Logger) *Service {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Service{
		client: client,
		logger: logger.With(zap.String("component", "music.service")),
	}
}

// Search proxies query to Yandex Music with pagination support.
func (s *Service) Search(ctx context.Context, query string, limit, offset int) ([]yandex.Track, error) {
	s.logger.Debug("search", zap.String("query", query), zap.Int("limit", limit), zap.Int("offset", offset))
	tracks, err := s.client.SearchTracks(ctx, query, limit, offset)
	if err != nil {
		s.logger.Warn("search failed", zap.String("query", query), zap.Error(err))
		return nil, err
	}
	s.logger.Debug("search result", zap.String("query", query), zap.Int("count", len(tracks)))
	return tracks, nil
}

// StreamURL returns track meta and a direct URL for inline playback/download.
func (s *Service) StreamURL(ctx context.Context, id string) (yandex.Track, string, error) {
	s.logger.Debug("stream url", zap.String("track_id", id))
	meta, err := s.client.GetTrack(ctx, id)
	if err != nil {
		s.logger.Warn("get track failed", zap.String("track_id", id), zap.Error(err))
		return yandex.Track{}, "", fmt.Errorf("get track meta: %w", err)
	}

	downloadURL, err := s.client.GetDownloadURL(ctx, id)
	if err != nil {
		s.logger.Warn("get download url failed", zap.String("track_id", id), zap.Error(err))
		return yandex.Track{}, "", fmt.Errorf("get download url: %w", err)
	}

	s.logger.Debug("stream url resolved", zap.String("track_id", id), zap.String("title", meta.Title))
	return meta, downloadURL, nil
}

// DownloadTrack downloads the audio file for the given track id into a temp file.
// Returns track meta and local file path that caller must remove.
func (s *Service) DownloadTrack(ctx context.Context, id string) (yandex.Track, string, error) {
	s.logger.Info("download started", zap.String("track_id", id))

	meta, err := s.client.GetTrack(ctx, id)
	if err != nil {
		s.logger.Warn("get track failed", zap.String("track_id", id), zap.Error(err))
		return yandex.Track{}, "", fmt.Errorf("get track meta: %w", err)
	}

	downloadURL, err := s.client.GetDownloadURL(ctx, id)
	if err != nil {
		s.logger.Warn("get download url failed", zap.String("track_id", id), zap.Error(err))
		return yandex.Track{}, "", fmt.Errorf("get download url: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "ym-bot-*")
	if err != nil {
		s.logger.Error("temp dir creation failed", zap.String("track_id", id), zap.Error(err))
		return yandex.Track{}, "", fmt.Errorf("temp dir: %w", err)
	}

	filename := fmt.Sprintf("%s - %s.mp3", meta.ArtistsString(), meta.Title)
	dest := filepath.Join(tmpDir, filename)

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	if err := s.client.DownloadToFile(ctx, downloadURL, dest); err != nil {
		_ = os.RemoveAll(tmpDir)
		s.logger.Warn("download to file failed", zap.String("track_id", id), zap.String("title", meta.Title), zap.Error(err))
		return yandex.Track{}, "", fmt.Errorf("download: %w", err)
	}

	s.logger.Info("download completed", zap.String("track_id", id), zap.String("title", meta.Title), zap.String("artists", meta.ArtistsString()))
	return meta, dest, nil
}
