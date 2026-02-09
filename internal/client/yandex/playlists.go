package yandex

import (
	"context"
	"fmt"
)

// PublicPlaylist is a minimal representation of a public playlist.
type PublicPlaylist struct {
	ID    string
	Title string
	Owner string
	// TrackCount is an optional hint about how many tracks are in the playlist
	TrackCount int
}

// GetPublicPlaylists should return a list of public playlists available without
// user authentication. This is a placeholder implementation and may be expanded
// as we identify reliable public endpoints.
func (c *APIClient) GetPublicPlaylists(ctx context.Context, limit int) ([]PublicPlaylist, error) {
	// Without an OAuth token, there is no stable public endpoint known to fetch
	// playlists reliably. Return a not-implemented error to indicate the need
	// for additional API surface or configuration.
	return nil, fmt.Errorf("GetPublicPlaylists not implemented without OAuth token")
}

// GetPlaylistTracks should fetch tracks for a given public playlist id.
// This is a placeholder and will be implemented once a public endpoint is found.
func (c *APIClient) GetPlaylistTracks(ctx context.Context, playlistID string, limit int) ([]Track, error) {
	return nil, fmt.Errorf("GetPlaylistTracks not implemented without OAuth token")
}
