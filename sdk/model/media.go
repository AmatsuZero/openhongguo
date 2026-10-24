package model

import (
	"fmt"
	"sort"
	"strings"
)

// Playback is one resolved anonymous media model. It describes what the
// service offers; it never carries decryption keys or licence material.
type Playback struct {
	EpisodeID       string   `json:"episode_id"`
	VideoID         string   `json:"video_id,omitempty"`
	MediaType       string   `json:"media_type,omitempty"`
	DurationSeconds int      `json:"duration_seconds,omitempty"`
	ExpiresAt       int64    `json:"expires_at,omitempty"`
	Protected       bool     `json:"protected"`
	Streams         []Stream `json:"streams"`
	// AudioURL is the separate audio track of a segmented (DASH) title such as
	// a full-length film. A single-file representation leaves it empty.
	AudioURL string `json:"audio_url,omitempty"`
	// KeyMaterial is the service's wrapped content key (encrypt_info.spade_a).
	// It is opaque DRM material: it is never serialized, logged, or copied into
	// a stream, and only the decrypt path reads it.
	KeyMaterial string `json:"-"`
}

// ContentKey is the CENC (cenc-aes-ctr) content key of a protected
// playback model, as sdk.ContentKey recovers it.
type ContentKey struct {
	// Hex is the 16-byte AES key as lowercase hex, the form ffmpeg's
	// -decryption_key and a player's ClearKey setup take.
	Hex string `json:"key"`
	// Version is the scheme marker the player reads next to the key.
	Version string `json:"version,omitempty"`
}

// Stream is one representation from the media model.
// Protected marks a representation the shipped client decrypts internally;
// sdk.ContentKey recovers the key for it from the model's KeyMaterial.
type Stream struct {
	Definition string `json:"definition,omitempty"`
	Quality    string `json:"quality,omitempty"`
	Codec      string `json:"codec,omitempty"`
	Width      int    `json:"width,omitempty"`
	Height     int    `json:"height,omitempty"`
	Bitrate    int64  `json:"bitrate,omitempty"`
	Size       int64  `json:"size,omitempty"`
	URL        string `json:"url,omitempty"`
	BackupURL  string `json:"backup_url,omitempty"`
	HLS        bool   `json:"hls,omitempty"`
	Dash       bool   `json:"dash,omitempty"`
	Protected  bool   `json:"protected"`
	Encryption string `json:"encryption,omitempty"`
}

// Select picks one playable representation. An empty quality selects the
// highest resolution, then the highest bitrate. "worst" selects the lowest.
// Any other value matches the definition ("720p"), the quality label
// ("normal"), or the codec. A stream without a URL is never selected.
func (p *Playback) Select(quality string) (Stream, error) {
	if p == nil {
		return Stream{}, fmt.Errorf("playback model is missing")
	}
	usable := make([]Stream, 0, len(p.Streams))
	for _, s := range p.Streams {
		if strings.TrimSpace(s.URL) != "" {
			usable = append(usable, s)
		}
	}
	if len(usable) == 0 {
		return Stream{}, fmt.Errorf("playback model has no stream URL")
	}
	sort.SliceStable(usable, func(i, j int) bool {
		hi, hj := heightRank(usable[i]), heightRank(usable[j])
		if hi != hj {
			return hi < hj
		}
		return usable[i].Bitrate < usable[j].Bitrate
	})
	want := strings.ToLower(strings.TrimSpace(quality))
	switch want {
	case "", "best":
		return usable[len(usable)-1], nil
	case "worst":
		return usable[0], nil
	}
	for _, s := range usable {
		if strings.EqualFold(s.Definition, quality) || strings.EqualFold(s.Quality, quality) || strings.EqualFold(s.Codec, quality) {
			return s, nil
		}
	}
	return Stream{}, fmt.Errorf("quality %q is not offered (available: %s)", quality, strings.Join(definitions(usable), " "))
}

// heightRank orders representations. An explicit height wins; otherwise the
// leading number of the definition ("1080p", "720p") supplies it.
func heightRank(s Stream) int {
	if s.Height > 0 {
		return s.Height
	}
	digits := 0
	for _, r := range s.Definition {
		if r < '0' || r > '9' {
			break
		}
		digits = digits*10 + int(r-'0')
	}
	if digits > 0 {
		return digits
	}
	return s.Width
}

func definitions(streams []Stream) []string {
	out := make([]string, 0, len(streams))
	for _, s := range streams {
		switch {
		case s.Definition != "":
			out = append(out, s.Definition)
		case s.Quality != "":
			out = append(out, s.Quality)
		default:
			out = append(out, "unknown")
		}
	}
	return out
}
