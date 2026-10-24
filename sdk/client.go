package sdk

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/beilunyang/openhongguo/internal/protocol"

	"github.com/beilunyang/openhongguo/sdk/model"
)

const (
	defaultAPIOrigin      = "https://" + protocol.APIHost
	defaultRegisterOrigin = "https://" + protocol.RegisterHost
	// defaultDistributionOrigin serves /reading/distribution/ and activity_api.
	defaultDistributionOrigin = "https://" + protocol.DistributionHost
)

// Client performs anonymous catalog reads. It is safe for concurrent use
// when the injected transport and profile store are.
type Client struct {
	guestMu        sync.Mutex
	transport      Transport
	profileStore   ProfileStore
	signer         Signer
	authProvider   AuthProvider
	logger         Logger
	clock          func() time.Time
	apiOrigin      string
	registerOrigin string
	distOrigin     string
	// device is what a new profile registers as (Options.Device).
	device model.Device
	// retryAttempts is how many times one request may run (1 = no retry).
	retryAttempts int
}

func NewClient(opts Options) (*Client, error) {
	if opts.Transport == nil || opts.ProfileStore == nil {
		return nil, ErrInvalidOptions
	}
	if opts.AuthProvider == nil {
		opts.AuthProvider = AnonymousProvider{}
	}
	if opts.Signer == nil {
		opts.Signer = UnavailableSigner{}
	}
	if opts.Clock == nil {
		opts.Clock = time.Now
	}
	api := strings.TrimRight(opts.APIOrigin, "/")
	if api == "" {
		api = defaultAPIOrigin
	}
	reg := strings.TrimRight(opts.RegisterOrigin, "/")
	if reg == "" {
		reg = defaultRegisterOrigin
	}
	dist := strings.TrimRight(opts.DistributionOrigin, "/")
	if dist == "" {
		dist = defaultDistributionOrigin
	}
	attempts := opts.RetryAttempts
	if attempts <= 0 {
		attempts = 1
	}
	if err := opts.Device.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidOptions, err)
	}
	return &Client{
		device:        opts.Device.OrDefault(),
		retryAttempts: attempts,
		transport:     opts.Transport, profileStore: opts.ProfileStore,
		signer: opts.Signer, authProvider: opts.AuthProvider,
		logger: opts.Logger, clock: opts.Clock,
		apiOrigin: api, registerOrigin: reg, distOrigin: dist,
	}, nil
}

func (c *Client) do(ctx context.Context, method, origin, path string, query url.Values, headers map[string]string, body []byte) ([]byte, map[string]string, error) {
	u, err := url.Parse(origin + path)
	if err != nil {
		return nil, nil, err
	}
	if query != nil {
		u.RawQuery = query.Encode()
	}
	return c.doRaw(ctx, method, u.String(), headers, body)
}

// doRaw sends a URL whose query is already signed. It does not re-encode that query.
func (c *Client) doRaw(ctx context.Context, method, rawURL string, headers map[string]string, body []byte) ([]byte, map[string]string, error) {
	for attempt := 0; ; attempt++ {
		raw, cookies, err := c.doOnce(ctx, method, rawURL, headers, body, attempt)
		if err == nil || !retryableInLoop(err) || attempt+1 >= c.retryAttempts {
			return raw, cookies, err
		}
		wait := retryWait(err, attempt)
		if wait > maxRetryWait {
			// A ban window this long is the service asking the caller to
			// back off, not a blip worth sleeping through.
			return nil, nil, fmt.Errorf("%w; the service asks to wait about %s before retrying", err, wait.Round(time.Second))
		}
		if waitErr := sleepContext(ctx, wait); waitErr != nil {
			return nil, nil, waitErr
		}
	}
}

// maxRetryWait caps one in-place retry. The retry resends the same signed
// request, so the cap also keeps its signing time fresh.
const maxRetryWait = 10 * time.Second

// retryableInLoop marks the failures worth repeating unchanged.
func retryableInLoop(err error) bool { return Retryable(err) }

// retryWait is the backoff of this attempt, or the service's own ban window
// when the failure carried one.
func retryWait(err error, attempt int) time.Duration {
	backoff := time.Duration(300*(1<<uint(attempt))) * time.Millisecond
	if backoff > 5*time.Second {
		backoff = 5 * time.Second
	}
	// Spread concurrent callers over the window (0.8x .. 1.2x).
	if crand, e := randInt64(); e == nil {
		backoff = time.Duration(float64(backoff) * (0.8 + 0.4*float64(crand%100)/100))
	}
	return RetryAfter(err, backoff)
}

// sleepContext waits for d or until ctx is done.
func sleepContext(ctx context.Context, wait time.Duration) error {
	if wait <= 0 {
		return nil
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (c *Client) doOnce(ctx context.Context, method, rawURL string, headers map[string]string, body []byte, attempt int) ([]byte, map[string]string, error) {
	if err := refuseCredentialHeaders(headers); err != nil {
		return nil, nil, err
	}
	if headers == nil {
		headers = map[string]string{}
	}
	if _, ok := headers["Accept"]; !ok {
		headers["Accept"] = "application/json"
	}
	req := &Request{Method: method, URL: rawURL, Headers: headers, Body: body}
	if err := c.authProvider.Apply(ctx, req); err != nil {
		return nil, nil, err
	}
	if err := refuseCredentialHeaders(req.Headers); err != nil {
		return nil, nil, err
	}
	resp, err := c.transport.Do(ctx, req)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %s", ErrTransport, protocol.RedactedURL(req.URL))
	}
	if c.logger != nil {
		c.logger.Printf("HTTP %s %s -> %d", method, protocol.RedactedURL(req.URL), statusOf(resp))
	}
	raw, err := readAnonymous(resp)
	if err != nil {
		return nil, nil, err
	}
	return raw, resp.Cookies, nil
}

func statusOf(resp *Response) int {
	if resp == nil {
		return 0
	}
	return resp.StatusCode
}

func readAnonymous(resp *Response) ([]byte, error) {
	if resp == nil {
		return nil, fmt.Errorf("%w: nil response", ErrTransport)
	}
	switch resp.StatusCode {
	case 401, 403:
		return nil, ErrAuthenticationRequired
	case 429:
		return nil, ErrRateLimited
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &APIError{HTTPStatus: resp.StatusCode, Message: "HTTP request failed"}
	}
	body, err := decodeBody(resp)
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return nil, ErrAnonymousLimited
	}
	if code, message, ok := protocol.BusinessCode(body); ok && code != "0" {
		n, convErr := strconv.Atoi(code)
		if convErr != nil {
			if protocol.AuthCode(code, message) {
				return nil, ErrAuthenticationRequired
			}
			return nil, &CodeError{Message: message, Kind: protocol.Classify(0, message)}
		}
		kind := protocol.Classify(n, message)
		return nil, &CodeError{
			Code: n, Name: protocol.CodeName(n), Message: message, Kind: kind,
		}
	}
	if status := protocol.BaseStatus(body); protocol.DeviceRejected(status) {
		return nil, &CodeError{Code: status, Name: "DEVICE_REJECTED", Kind: protocol.KindDeviceRejected}
	}
	return body, nil
}

func decodeBody(resp *Response) ([]byte, error) {
	encoding := ""
	for k, v := range resp.Headers {
		if strings.EqualFold(k, "Content-Encoding") {
			encoding = strings.ToLower(v)
		}
	}
	var r io.Reader = bytes.NewReader(resp.Body)
	switch encoding {
	case "gzip":
		zr, err := gzip.NewReader(r)
		if err != nil {
			return nil, err
		}
		defer zr.Close()
		r = zr
	case "":
	default:
		return nil, fmt.Errorf("%w: unsupported content encoding", ErrTransport)
	}
	return io.ReadAll(io.LimitReader(r, 8<<20))
}

func refuseCredentialHeaders(headers map[string]string) error {
	for name := range headers {
		for _, banned := range protocol.CredentialHeaderNames {
			if strings.EqualFold(name, banned) {
				return fmt.Errorf("%w: credential header refused", ErrAuthenticationRequired)
			}
		}
	}
	return nil
}

func (c *Client) ensureGuest(ctx context.Context) (*DeviceProfile, error) {
	return c.loadOrRegister(ctx, false)
}

// ensureSearchGuest registers again when a saved guest has no ttreq.
// A register response that sets cookies but omits ttreq is an error.
// A register response with no cookies leaves the ticket empty so fixture
// calls can search without a guest cookie.
func (c *Client) ensureSearchGuest(ctx context.Context) (*DeviceProfile, error) {
	return c.loadOrRegister(ctx, true)
}

func (c *Client) loadOrRegister(ctx context.Context, needTicket bool) (*DeviceProfile, error) {
	c.guestMu.Lock()
	defer c.guestMu.Unlock()
	p, err := c.profileStore.Load(ctx)
	if err != nil && !errors.Is(err, ErrProfileNotFound) {
		return nil, err
	}
	if p != nil && p.Device == nil {
		// A profile from before devices were stored keeps the client's
		// device from now on, so its model does not change between runs.
		d := c.device
		p.Device = &d
		if err = c.profileStore.Save(ctx, p); err != nil {
			return nil, err
		}
	}
	if guestReady(p) && (!needTicket || p.TTReq != "") {
		return p, nil
	}
	if p == nil {
		p, err = NewAnonymousDeviceProfile(nil, c.clock)
		if err != nil {
			return nil, err
		}
		d := c.device
		p.Device = &d
		if err = c.profileStore.Save(ctx, p); err != nil {
			return nil, err
		}
	}
	body, err := protocol.BuildRegisterBody(protocol.RegisterIdentity{
		CDID: p.CDID, OpenUDID: p.OpenUDID, ClientUDID: p.ClientUDID, OAID: p.OAID, ReqID: p.ReqID,
		Device: *p.Device,
	}, c.clock())
	if err != nil {
		return nil, err
	}
	if len(body) < 6 || body[0] != 0x74 || body[1] != 0x63 {
		return nil, ErrSignerUnavailable
	}
	q := url.Values{}
	// bdinstall.a1.k adds req_id before the encrypted register body appends tt_data=a.
	q.Set("req_id", p.ReqID)
	q.Set("tt_data", "a")
	headers := map[string]string{
		"User-Agent":                protocol.UserAgent(*p.Device),
		"Content-Type":              "application/octet-stream;tt-data=a",
		"log-encode-type":           "gzip",
		"sdk-version":               "2",
		"passport-sdk-version":      "5051452",
		"x-vc-bdturing-sdk-version": "4.0.3.cn",
		"x-ss-dp":                   protocol.AID,
	}
	// X-SS-STUB is a portable MD5 of the body. It is sent and never logged.
	headers["X-SS-STUB"] = protocol.ContentMD5(body)
	raw, cookies, err := c.do(ctx, "POST", c.registerOrigin, protocol.PathRegister, q, headers, body)
	if err != nil {
		return nil, err
	}
	registered, err := protocol.ParseRegister(raw)
	if err != nil {
		return nil, err
	}
	p.DeviceID = registered.DeviceID
	p.InstallID = registered.InstallID
	p.DeviceToken = ""
	applyGuestCookies(p, cookies)
	if needTicket && len(cookies) > 0 && p.TTReq == "" {
		return nil, fmt.Errorf("%w: guest ticket missing", ErrAnonymousLimited)
	}
	if err := c.profileStore.Save(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

func guestReady(p *DeviceProfile) bool {
	return p != nil && p.DeviceID != "" && p.InstallID != "" && p.DeviceID != "0" && p.InstallID != "0"
}

func applyGuestCookies(p *DeviceProfile, cookies map[string]string) {
	if p == nil || len(cookies) == 0 {
		return
	}
	if v := cookies["ttreq"]; v != "" {
		p.TTReq = v
	}
	if v := cookies["store-region"]; v != "" {
		p.StoreRegion = v
	}
	if v := cookies["store-region-src"]; v != "" {
		p.StoreRegionSrc = v
	}
}

func guestCookie(p *DeviceProfile) string {
	return "passport_csrf_token=; passport_csrf_token_default=; store-region=" + p.StoreRegion +
		"; store-region-src=" + p.StoreRegionSrc + "; install_id=" + p.InstallID + "; ttreq=" + p.TTReq
}

func (c *Client) randomInt31() int32 {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 1
	}
	return int32(binary.BigEndian.Uint32(b[:]) & 0x7fffffff)
}

// randomUUID returns a v4 UUID string for the session_uuid query parameter.
func randomUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "00000000-0000-4000-8000-000000000000"
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// randInt64 returns a non-negative random value for jitter.
func randInt64() (int64, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0, err
	}
	v := int64(binary.BigEndian.Uint64(b[:]) & 0x7fffffffffffffff)
	return v, nil
}

// ResetDevice forgets the anonymous profile and registers a fresh device.
// Callers use it when a request reports ErrDeviceRejected (BaseResp 263175 or
// 263176).
func (c *Client) ResetDevice(ctx context.Context) error {
	if c.profileStore == nil {
		return ErrInvalidOptions
	}
	if err := c.profileStore.Reset(ctx); err != nil {
		return err
	}
	_, err := c.ensureGuest(ctx)
	return err
}

// identity is the device query identity of p: its registered ids and the
// device it registered as.
func (c *Client) identity(p *DeviceProfile) protocol.SearchIdentity {
	return protocol.SearchIdentity{
		DeviceID: p.DeviceID, InstallID: p.InstallID, CDID: p.CDID, OpenUDID: p.OpenUDID,
		Device: c.deviceOf(p),
	}
}

// deviceOf is the device p registered as, or the client's for a profile
// that names none.
func (c *Client) deviceOf(p *DeviceProfile) model.Device {
	if p != nil && p.Device != nil {
		return p.Device.OrDefault()
	}
	return c.device
}
