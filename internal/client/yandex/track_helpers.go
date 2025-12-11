package yandex

import "strings"

// ArtistsString renders joined artist names.
func (t Track) ArtistsString() string {
	if len(t.Artists) == 0 {
		return ""
	}
	return strings.Join(t.Artists, ", ")
}

