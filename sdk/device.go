package sdk

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"time"

	"github.com/beilunyang/openhongguo/sdk/model"
)

// DeviceProfile is a locally generated anonymous installation.
// Server-assigned ids stay empty until registration returns them.
// DeviceToken is never printed by the CLI.
type DeviceProfile struct {
	CDID           string    `json:"cdid"`
	OpenUDID       string    `json:"openudid"`
	ClientUDID     string    `json:"clientudid"`
	OAID           string    `json:"oaid_id"`
	ReqID          string    `json:"req_id"`
	DeviceID       string    `json:"device_id_str,omitempty"`
	InstallID      string    `json:"install_id_str,omitempty"`
	DeviceToken    string    `json:"device_token,omitempty"`
	TTReq          string    `json:"ttreq,omitempty"`
	StoreRegion    string    `json:"store_region,omitempty"`
	StoreRegionSrc string    `json:"store_region_src,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	// Device is the phone the profile registered as; every request presents
	// it. A profile without one takes the client's on first use.
	Device *model.Device `json:"device,omitempty"`
}

// ProfileStore persists one anonymous profile.
type ProfileStore interface {
	Load(ctx context.Context) (*DeviceProfile, error)
	Save(ctx context.Context, profile *DeviceProfile) error
	Reset(ctx context.Context) error
}

// NewAnonymousDeviceProfile fills identity fields from crypto-secure randomness.
// A nil entropy source uses crypto/rand. A nil clock uses time.Now.
func NewAnonymousDeviceProfile(entropy io.Reader, clock func() time.Time) (*DeviceProfile, error) {
	if entropy == nil {
		entropy = rand.Reader
	}
	if clock == nil {
		clock = time.Now
	}
	var raw [72]byte
	if _, err := io.ReadFull(entropy, raw[:]); err != nil {
		return nil, err
	}
	return &DeviceProfile{
		CDID:       uuid(raw[0:16]),
		OpenUDID:   hex.EncodeToString(raw[16:24]),
		ClientUDID: uuid(raw[24:40]),
		OAID:       uuid(raw[40:56]),
		ReqID:      uuid(raw[56:72]),
		CreatedAt:  clock().UTC(),
	}, nil
}

func uuid(b []byte) string {
	var id [16]byte
	copy(id[:], b)
	id[6] = id[6]&0x0f | 0x40
	id[8] = id[8]&0x3f | 0x80
	var s [36]byte
	hex.Encode(s[0:8], id[0:4])
	s[8] = '-'
	hex.Encode(s[9:13], id[4:6])
	s[13] = '-'
	hex.Encode(s[14:18], id[6:8])
	s[18] = '-'
	hex.Encode(s[19:23], id[8:10])
	s[23] = '-'
	hex.Encode(s[24:36], id[10:16])
	return string(s[:])
}
