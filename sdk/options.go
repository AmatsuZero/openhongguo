package sdk

import (
	"time"

	"github.com/beilunyang/openhongguo/sdk/model"
)

// Logger receives diagnostic lines. Implementations must tolerate redacted URLs only.
type Logger interface {
	Printf(format string, args ...any)
}

// Options supplies every platform dependency. Empty origins use the production hosts
// observed for anonymous reads. Tests override them with a local server.
type Options struct {
	Transport      Transport
	ProfileStore   ProfileStore
	Signer         Signer
	AuthProvider   AuthProvider
	Logger         Logger
	Clock          func() time.Time
	APIOrigin      string
	RegisterOrigin string
	// DistributionOrigin serves the /reading/distribution/ and activity_api
	// reads (the short-drama filter and the activity ranking), which
	// APIOrigin's default host does not. Empty uses the app's own API host.
	DistributionOrigin string
	// Device is the phone a new anonymous profile registers as and then
	// presents on every request; zero fields take model.DefaultDevice (a
	// Xiaomi 14 on Android 14, Xiaomi store channel). A profile keeps the
	// device it registered with, so changing this affects only profiles
	// registered afterwards, including after a device reset.
	Device model.Device
	// RetryAttempts caps how many times one request runs when the failure is
	// retryable (rate limit, transport, 5xx). 0 or 1 disables retrying.
	RetryAttempts int
}
