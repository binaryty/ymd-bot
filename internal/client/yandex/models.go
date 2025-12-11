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
	ID         json.Number   `json:"id"`
	Title      string        `json:"title"`
	DurationMs int           `json:"durationMs"`
	Artists    []artistDTO   `json:"artists"`
	Albums     albumListDTO  `json:"albums"`
	CoverURI   string        `json:"coverUri"`
	StorageDir string        `json:"storageDir"`
	RealID     string        `json:"realId"`
	TrackShare string        `json:"trackShareUrl"`
	Type       string        `json:"type"`
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
	URL    string `json:"downloadInfoUrl"`
	Codec  string `json:"codec"`
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

