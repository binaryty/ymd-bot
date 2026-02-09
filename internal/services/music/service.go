package music

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"

	"ym-bot/internal/client/yandex"
)

// Service orchestrates music search and download workflow.
type Service struct {
	client yandex.Client
	logger *zap.Logger

	mu               sync.Mutex
	chartCache       cachedTracks
	newReleasesCache cachedTracks
	genreCaches      map[string]genreCache
}

type cachedTracks struct {
	tracks    []yandex.Track
	expiresAt time.Time
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

// TopChart returns tracks from the cached chart or fetches them from Yandex Music.
func (s *Service) TopChart(ctx context.Context, limit int) ([]yandex.Track, error) {
	if limit <= 0 {
		limit = 50
	}

	now := time.Now()

	s.mu.Lock()
	if len(s.chartCache.tracks) > 0 && s.chartCache.expiresAt.After(now) {
		n := limit
		if n > len(s.chartCache.tracks) {
			n = len(s.chartCache.tracks)
		}
		cached := s.chartCache.tracks[:n]
		s.mu.Unlock()
		s.logger.Debug("top chart served from cache", zap.Int("count", len(cached)))
		return cached, nil
	}
	s.mu.Unlock()

	s.logger.Info("fetching top chart from API")
	tracks, err := s.client.GetTopChart(ctx, limit)
	if err != nil {
		s.logger.Error("get top chart failed", zap.Error(err))
		return nil, err
	}

	s.mu.Lock()
	s.chartCache = cachedTracks{
		tracks:    tracks,
		expiresAt: now.Add(1 * time.Hour),
	}
	s.mu.Unlock()

	return tracks, nil
}

// NewReleases returns tracks from the cached new releases or fetches them from Yandex Music.
func (s *Service) NewReleases(ctx context.Context, limit int) ([]yandex.Track, error) {
	if limit <= 0 {
		limit = 50
	}

	now := time.Now()

	s.mu.Lock()
	if len(s.newReleasesCache.tracks) > 0 && s.newReleasesCache.expiresAt.After(now) {
		n := limit
		if n > len(s.newReleasesCache.tracks) {
			n = len(s.newReleasesCache.tracks)
		}
		cached := s.newReleasesCache.tracks[:n]
		s.mu.Unlock()
		s.logger.Debug("new releases served from cache", zap.Int("count", len(cached)))
		return cached, nil
	}
	s.mu.Unlock()

	s.logger.Info("fetching new releases from API")
	tracks, err := s.client.GetNewReleases(ctx, limit)
	if err != nil {
		s.logger.Error("get new releases failed", zap.Error(err))
		return nil, err
	}

	s.mu.Lock()
	s.newReleasesCache = cachedTracks{
		tracks:    tracks,
		expiresAt: now.Add(1 * time.Hour),
	}
	s.mu.Unlock()

	return tracks, nil
}

// GenreCache holds cached genre tracks
type genreCache struct {
	tracks    []yandex.Track
	expiresAt time.Time
}

// GetGenreTracks returns tracks from the cached genre playlist or fetches them from Yandex Music.
func (s *Service) GetGenreTracks(ctx context.Context, genre string, limit int) ([]yandex.Track, error) {
	if limit <= 0 {
		limit = 50
	}

	now := time.Now()

	s.mu.Lock()
	if s.genreCaches == nil {
		s.genreCaches = make(map[string]genreCache)
	}
	if cache, ok := s.genreCaches[genre]; ok && len(cache.tracks) > 0 && cache.expiresAt.After(now) {
		n := limit
		if n > len(cache.tracks) {
			n = len(cache.tracks)
		}
		cached := cache.tracks[:n]
		s.mu.Unlock()
		s.logger.Debug("genre tracks served from cache", zap.String("genre", genre), zap.Int("count", len(cached)))
		return cached, nil
	}
	s.mu.Unlock()

	s.logger.Info("fetching genre tracks from API", zap.String("genre", genre))
	tracks, err := s.client.GetGenreTracks(ctx, genre, limit)
	if err != nil {
		s.logger.Error("get genre tracks failed", zap.String("genre", genre), zap.Error(err))
		return nil, err
	}

	s.mu.Lock()
	s.genreCaches[genre] = genreCache{
		tracks:    tracks,
		expiresAt: now.Add(1 * time.Hour),
	}
	s.mu.Unlock()

	return tracks, nil
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
