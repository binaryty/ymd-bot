package yandex

import (
	"fmt"
	"os"
	"path/filepath"

	"encoding/json"
)

type searchResponse struct {
	Result searchResult `json:"result"`
}

type searchResult struct {
	Tracks trackMatches `json:"tracks"`
}

type trackMatches struct {
	Results []trackDTO `json:"results"`
}

type trackResponse struct {
	Result []trackDTO `json:"result"`
}

type trackDTO struct {
	ID         json.Number  `json:"id"`
	Title      string       `json:"title"`
	DurationMs int          `json:"durationMs"`
	Artists    []artistDTO  `json:"artists"`
	Albums     albumListDTO `json:"albums"`
	CoverURI   string       `json:"coverUri"`
	StorageDir string       `json:"storageDir"`
	RealID     string       `json:"realId"`
	TrackShare string       `json:"trackShareUrl"`
	Type       string       `json:"type"`
}

type artistDTO struct {
	Name string `json:"name"`
}

type albumListDTO []albumDTO

func (a albumListDTO) Title() string {
	if len(a) == 0 {
		return ""
	}
	return a[0].Title
}

type albumDTO struct {
	Title string `json:"title"`
}

type downloadInfoResponse struct {
	Result []downloadInfoDTO `json:"result"`
}

type downloadInfoDTO struct {
	URL     string `json:"downloadInfoUrl"`
	Codec   string `json:"codec"`
	Bitrate int    `json:"bitrateInKbps"`
}

// ensureDir creates a directory if missing.
func ensureDir(dir string) error {
	if dir == "" || dir == "." {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

// createFile creates/truncates a file ensuring the parent directory exists.
func createFile(path string) (*os.File, error) {
	if err := ensureDir(filepath.Dir(path)); err != nil {
		return nil, fmt.Errorf("ensure dir: %w", err)
	}
	return os.Create(path) //nolint:gosec // destination controlled internally
}

// --- Chart and new releases DTOs ---

// chartResponse represents response shape for /landing3/chart endpoints.
type chartResponse struct {
	Result chartResult `json:"result"`
}

type chartResult struct {
	Chart chartBlock `json:"chart"`
}

type chartBlock struct {
	Tracks []chartTrackItem `json:"tracks"`
}

type chartTrackItem struct {
	Track trackDTO `json:"track"`
}

// newReleasesResponse represents response shape for /landing3/new-releases.
type newReleasesResponse struct {
	Result newReleasesResult `json:"result"`
}

type newReleasesResult struct {
	Entity []newReleaseEntity `json:"entity"`
}

// newReleaseEntity describes a single new release item (album-centric).
type newReleaseEntity struct {
	Album newReleaseAlbum `json:"album"`
}

type newReleaseAlbum struct {
	ID       json.Number `json:"id"`
	Title    string      `json:"title"`
	Artists  []artistDTO `json:"artists"`
	CoverURI string      `json:"coverUri"`
}

// --- Genre DTOs ---

// genreResponse represents response shape for /landing3/genre-* endpoints.
type genreResponse struct {
	Result genreResult `json:"result"`
}

type genreResult struct {
	Blocks []genreBlock `json:"blocks"`
}

type genreBlock struct {
	Entities []genreEntity `json:"entities"`
}

type genreEntity struct {
	Data genreEntityData `json:"data"`
}

type genreEntityData struct {
	Track trackDTO `json:"track"`
}
