package yandex

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"
)

const (
	apiBase   = "https://api.music.yandex.net"
	userAgent = "ym-bot/0.1 (+github.com/ndrewnee/go-yandex-music compatible)"
)

// Track represents a minimal subset of Yandex Music track fields.
type Track struct {
	ID              string
	Title           string
	Artists         []string
	DurationSeconds int
	CoverURL        string
	AlbumTitle      string
}

// Client describes operations the service layer relies on.
type Client interface {
	SearchTracks(ctx context.Context, query string, limit, offset int) ([]Track, error)
	GetTrack(ctx context.Context, id string) (Track, error)
	GetDownloadURL(ctx context.Context, id string) (string, error)
	DownloadToFile(ctx context.Context, downloadURL, destPath string) error
}

// HTTPClient wraps the stdlib client for easier testing.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// APIClient implements Client against Yandex Music HTTP endpoints.
type APIClient struct {
	httpClient HTTPClient
	token      string
	logger     *zap.Logger
}

// NewClient builds a Yandex Music API client.
func NewClient(httpClient HTTPClient, token string, logger *zap.Logger) *APIClient {
	if logger == nil {
		logger = zap.NewNop()
	}

	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}

	return &APIClient{
		httpClient: httpClient,
		token:      token,
		logger:     logger,
	}
}

// SearchTracks queries Yandex Music search API for tracks.
func (c *APIClient) SearchTracks(ctx context.Context, query string, limit, offset int) ([]Track, error) {
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("query is empty")
	}
	if limit <= 0 {
		limit = 10
	}
	if offset < 0 {
		offset = 0
	}

	page := offset / limit

	u, _ := url.Parse(apiBase + "/search")
	q := u.Query()
	q.Set("text", query)
	q.Set("type", "track")
	q.Set("page", fmt.Sprintf("%d", page))
	q.Set("nococrrect", "true")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	c.attachHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return nil, fmt.Errorf("search failed: status=%d body=%s", resp.StatusCode, string(body))
	}

	var payload searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode search response: %w", err)
	}

	tracks := make([]Track, 0, len(payload.Result.Tracks.Results))
	for i, t := range payload.Result.Tracks.Results {
		if i >= limit {
			break
		}
		tracks = append(tracks, mapTrack(t))
	}

	return tracks, nil
}

// GetTrack fetches detailed track metadata by id.
func (c *APIClient) GetTrack(ctx context.Context, id string) (Track, error) {
	if id == "" {
		return Track{}, fmt.Errorf("track id is empty")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/tracks/%s", apiBase, id), nil)
	if err != nil {
		return Track{}, err
	}
	c.attachHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Track{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return Track{}, fmt.Errorf("get track failed: status=%d body=%s", resp.StatusCode, string(body))
	}

	var payload trackResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return Track{}, fmt.Errorf("decode track response: %w", err)
	}

	if len(payload.Result) == 0 {
		return Track{}, fmt.Errorf("track not found")
	}

	return mapTrack(payload.Result[0]), nil
}

// GetDownloadURL resolves a track id to a downloadable URL.
// Official clients perform an extra redirect/URL signing step; for the purposes
// of this demo we reuse the same pattern used by community clients.
func (c *APIClient) GetDownloadURL(ctx context.Context, id string) (string, error) {
	if id == "" {
		return "", fmt.Errorf("track id is empty")
	}

	// Request all available formats and pick the first (usually mp3).
	u := fmt.Sprintf("%s/tracks/%s/download-info", apiBase, id)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	c.attachHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return "", fmt.Errorf("download-info failed: status=%d body=%s", resp.StatusCode, string(body))
	}

	var payload downloadInfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", fmt.Errorf("decode download-info: %w", err)
	}

	if len(payload.Result) == 0 {
		return "", fmt.Errorf("download url not found")
	}

	info := pickDownloadInfo(payload.Result)
	if info.URL == "" {
		return "", fmt.Errorf("download url not found")
	}

	// Resolve final downloadable URL (handles downloadInfoUrl indirection).
	finalURL, err := c.resolveDownloadInfoURL(ctx, info.URL, id)
	if err != nil {
		return "", err
	}
	return finalURL, nil
}

// DownloadToFile streams the content into destPath.
func (c *APIClient) DownloadToFile(ctx context.Context, downloadURL, destPath string) error {
	if downloadURL == "" {
		return fmt.Errorf("download url is empty")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return err
	}
	c.attachHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return fmt.Errorf("download failed: status=%d body=%s", resp.StatusCode, string(body))
	}

	tmpDir := filepath.Dir(destPath)
	if err := ensureDir(tmpDir); err != nil {
		return err
	}

	out, err := createFile(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func (c *APIClient) attachHeaders(req *http.Request) {
	req.Header.Set("User-Agent", userAgent)
	if c.token != "" {
		req.Header.Set("Authorization", "OAuth "+c.token)
	}
}

// resolveDownloadInfoURL fetches downloadInfoUrl and extracts the final audio URL.
// Some deployments return JSON {"src": "...mp3"}, some redirect, others return XML
// with host/path/ts/s which needs to be combined into a final URL.
func (c *APIClient) resolveDownloadInfoURL(ctx context.Context, infoURL, trackID string) (string, error) {
	if infoURL == "" {
		return "", fmt.Errorf("download info url is empty")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, infoURL, nil)
	if err != nil {
		return "", err
	}
	c.attachHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// First, try JSON payload with "src".
	var payload struct {
		Src string `json:"src"`
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
	if len(body) > 0 {
		if err := json.Unmarshal(body, &payload); err == nil && payload.Src != "" {
			return payload.Src, nil
		}
	}

	// Try XML response with host/path/ts/s.
	xmlURL, xmlErr := parseDownloadInfoXML(body, trackID)
	if xmlErr == nil && xmlURL != "" {
		return xmlURL, nil
	}

	// If not JSON, but redirect is provided.
	if loc := resp.Header.Get("Location"); loc != "" {
		return loc, nil
	}
	if resp.Request != nil && resp.Request.URL != nil {
		return resp.Request.URL.String(), nil
	}

	return "", fmt.Errorf("cannot resolve download url: status=%d", resp.StatusCode)
}

// parseDownloadInfoXML builds the final mp3 URL from XML payload.
func parseDownloadInfoXML(data []byte, trackID string) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("empty xml body")
	}

	type xmlInfo struct {
		Host   string `xml:"host"`
		Path   string `xml:"path"`
		TS     string `xml:"ts"`
		S      string `xml:"s"`
		Region string `xml:"region"`
	}

	var info xmlInfo
	if err := xml.Unmarshal(data, &info); err != nil {
		return "", err
	}
	if info.Host == "" || info.Path == "" || info.TS == "" || info.S == "" {
		return "", fmt.Errorf("incomplete xml fields")
	}

	final := fmt.Sprintf("https://%s/get-mp3/%s/%s%s", info.Host, info.S, info.TS, info.Path)
	if trackID != "" {
		final = final + "?track-id=" + trackID
	}
	return final, nil
}

// pickDownloadInfo chooses the best available download info (prefer mp3).
func pickDownloadInfo(items []downloadInfoDTO) downloadInfoDTO {
	if len(items) == 0 {
		return downloadInfoDTO{}
	}
	for _, i := range items {
		if strings.EqualFold(i.Codec, "mp3") {
			return i
		}
	}
	// fallback to first entry
	return items[0]
}

// mapTrack converts API model to internal Track.
func mapTrack(t trackDTO) Track {
	artists := make([]string, 0, len(t.Artists))
	for _, a := range t.Artists {
		if a.Name != "" {
			artists = append(artists, a.Name)
		}
	}

	cover := ""
	if t.CoverURI != "" {
		cover = "https://" + strings.ReplaceAll(t.CoverURI, "%%", "200x200")
	}

	return Track{
		ID:              t.ID.String(),
		Title:           t.Title,
		Artists:         artists,
		DurationSeconds: t.DurationMs / 1000,
		CoverURL:        cover,
		AlbumTitle:      t.Albums.Title(),
	}
}

