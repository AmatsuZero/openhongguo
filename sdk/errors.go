package sdk

import (
	"errors"
	"fmt"
	"time"

	"github.com/beilunyang/openhongguo/internal/protocol"
)

// Sentinel errors callers can match with errors.Is.
// Authentication and DRM stay distinct so a protected media response is never
// treated as a login failure, and neither path decrypts or unlocks content.
var (
	ErrInvalidOptions         = errors.New("openhongguo: invalid client options")
	ErrProfileNotFound        = errors.New("openhongguo: profile not found")
	ErrSignerUnavailable      = errors.New("openhongguo: signer unavailable")
	ErrAuthenticationRequired = errors.New("openhongguo: authentication required")
	ErrDRMProtected           = errors.New("openhongguo: DRM protected")
	// ErrNotProtected is a content key asked for a model that is not
	// encrypted.
	ErrNotProtected = errors.New("openhongguo: playback is not encrypted")
	ErrAnonymousLimited       = errors.New("openhongguo: anonymous response was empty")
	ErrRateLimited            = errors.New("openhongguo: rate limited")
	ErrTransport              = errors.New("openhongguo: transport failure")
	ErrSignatureRejected      = errors.New("openhongguo: signature rejected")
	// ErrContentUnavailable covers the content domain: removed, missing, or
	// not permitted for this caller (101001, 101104, 101109, 101115, ...).
	ErrContentUnavailable = errors.New("openhongguo: content unavailable")
	// ErrParamInvalid is a rejected request (100103, 4000, 101142).
	ErrParamInvalid = errors.New("openhongguo: invalid request")
	// ErrChallengeRequired means the service wants a captcha or verification
	// (111 HIT_VERIFY_CODE); an anonymous client cannot continue.
	ErrChallengeRequired = errors.New("openhongguo: verification required")
	// ErrDeviceRejected means the anonymous device or installation id was
	// refused (BaseResp 263175/263176) and must be re-registered.
	ErrDeviceRejected = errors.New("openhongguo: device rejected")
	// ErrEndOfList is 101 ALL_CONTENT_SHOWN: the paging surface is exhausted.
	ErrEndOfList = errors.New("openhongguo: no more content")
	// ErrAlbumUnavailable reports a collection the anonymous surface will not
	// serve (101001 on every sampled album) or a title without sibling seasons.
	ErrAlbumUnavailable = errors.New("openhongguo: album is not available anonymously")
)

// APIError is a service failure whose text never includes headers or URL queries.
type APIError struct {
	HTTPStatus int
	Code       string
	Message    string
}

func (e *APIError) Error() string {
	if e == nil {
		return "openhongguo: API error"
	}
	return fmt.Sprintf("openhongguo: API error (HTTP %d, code %s)", e.HTTPStatus, e.Code)
}

func (e *APIError) Is(target error) bool {
	if e == nil || target == nil {
		return false
	}
	switch target {
	case ErrRateLimited:
		return e.HTTPStatus == 429 || e.Code == "rate_limited"
	case ErrAuthenticationRequired:
		return e.HTTPStatus == 401 || e.HTTPStatus == 403 || e.Code == "auth_required" || e.Code == "401" || e.Code == "403"
	case ErrSignatureRejected:
		return e.Code == "signature_rejected"
	default:
		return false
	}
}

// CodeError is one service failure that carries the business code.
//
// The code space is not global: 100103 means "invalid parameter" in the book
// API and something else in the trade API, so Code and Kind stay separate and
// callers should branch on Kind.
type CodeError struct {
	Code       int
	Name       string
	Message    string
	Kind       protocol.ErrorKind
	HTTPStatus int
	Path       string
}

func (e *CodeError) Error() string {
	if e == nil {
		return "openhongguo: service error"
	}
	name := e.Name
	if name == "" {
		name = e.Kind.String()
	}
	return fmt.Sprintf("openhongguo: %s (code %d, %s)", e.Message, e.Code, name)
}

// Is lets callers match the sentinels above with errors.Is.
func (e *CodeError) Is(target error) bool {
	if e == nil || target == nil {
		return false
	}
	switch target {
	case ErrAuthenticationRequired:
		return e.Kind == protocol.KindAuth
	case ErrRateLimited:
		return e.Kind == protocol.KindRateLimited
	case ErrContentUnavailable:
		return e.Kind == protocol.KindContentGone
	case ErrParamInvalid:
		return e.Kind == protocol.KindParam
	case ErrChallengeRequired:
		return e.Kind == protocol.KindChallenge
	case ErrDeviceRejected:
		return e.Kind == protocol.KindDeviceRejected
	case ErrEndOfList:
		return e.Kind == protocol.KindEndOfList
	}
	return false
}

// Retryable reports whether the same request may succeed unchanged.
// Rate limits, transport failures and server errors are worth a retry; an
// auth, param, content or challenge failure is not.
func Retryable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrRateLimited) || errors.Is(err, ErrTransport) {
		return true
	}
	var api *APIError
	if errors.As(err, &api) {
		return api.HTTPStatus == 0 || api.HTTPStatus >= 500
	}
	var code *CodeError
	if errors.As(err, &code) {
		return code.Kind == protocol.KindRateLimited
	}
	return false
}

// RetryAfter suggests how long to wait before retrying err.
//
// A rate-limit response may carry the service's own ban window in its message
// (SvrBanApiModel); otherwise the caller's default applies.
func RetryAfter(err error, fallback time.Duration) time.Duration {
	var code *CodeError
	if errors.As(err, &code) {
		if minWait, maxWait, ok := protocol.BanWindow(code.Message); ok {
			if maxWait > minWait && minWait > 0 {
				return minWait + (maxWait-minWait)/2
			}
			return maxWait
		}
	}
	return fallback
}
