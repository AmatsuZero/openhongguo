package sdk

import (
	"context"
	"fmt"

	"github.com/beilunyang/openhongguo/internal/drm"
	"github.com/beilunyang/openhongguo/internal/protocol"
	"github.com/beilunyang/openhongguo/internal/protocol/sign"
	"github.com/beilunyang/openhongguo/sdk/model"
)

// PlaybackOptions is reserved for representation or surface selection.
// Phase 2 resolves the service model as shipped; the caller picks a stream.
type PlaybackOptions struct{}

// ResolvePlayback loads the anonymous media model for one episode.
//
// The request is the signed video_model call the shipped client sends. The
// response is the client's TTVideoEngine model. A protected model comes back
// together with ErrDRMProtected; pass it to ContentKey for the key. The
// model carries the wrapped key material (Playback.KeyMaterial), never the
// content key itself, per-segment check data, or decrypted media.
func (c *Client) ResolvePlayback(ctx context.Context, episodeID string, _ PlaybackOptions) (*model.Playback, error) {
	if !validID(episodeID) {
		return nil, fmt.Errorf("invalid episode ID")
	}
	p, err := c.ensureSearchGuest(ctx)
	if err != nil {
		return nil, err
	}
	body, err := protocol.PlaybackBody(episodeID)
	if err != nil {
		return nil, err
	}
	ticket := c.clock().UnixMilli()
	rawQuery, headers := sign.SignPOST(protocol.PathPlayback, protocol.DeviceQueryPairs(c.identity(p)), ticket, p.DeviceID, protocol.VersionName, body, nil)
	raw, _, err := c.doRaw(ctx, "POST", c.apiOrigin+protocol.PathPlayback+"?"+rawQuery, anonymousHeaders(c, p, ticket, headers), body)
	if err != nil {
		return nil, err
	}
	playback, err := protocol.ParsePlayback(raw, episodeID)
	if err != nil {
		return nil, err
	}
	if playback.Protected {
		return playback, ErrDRMProtected
	}
	return playback, nil
}

// ContentKey recovers the content key of a protected playback model, the one
// ResolvePlayback returns alongside ErrDRMProtected. It is local computation:
// the key is unwrapped from the model's KeyMaterial the way the shipped
// player does, with no further request. Every stream of the model shares it
// (each quality of an episode carries the same spade_a and kid, observed
// 2026-10-24).
//
// A model that is not encrypted gives ErrNotProtected; key material this
// version cannot unwrap gives ErrDRMProtected.
func ContentKey(pb *model.Playback) (model.ContentKey, error) {
	if pb == nil {
		return model.ContentKey{}, fmt.Errorf("playback model is missing")
	}
	if !pb.Protected {
		return model.ContentKey{}, ErrNotProtected
	}
	k, err := drm.Derive(pb.KeyMaterial)
	if err != nil {
		return model.ContentKey{}, fmt.Errorf("%w: %v", ErrDRMProtected, err)
	}
	return model.ContentKey{Hex: k.Hex, Version: k.Version}, nil
}
