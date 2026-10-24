// Package httpadapter is the default net/http transport for anonymous reads.
package httpadapter

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/beilunyang/openhongguo/internal/protocol"
	"github.com/beilunyang/openhongguo/sdk"
)

// Transport performs anonymous HTTP requests. It does not keep a cookie jar.
type Transport struct {
	client *http.Client
	logger sdk.Logger
}

func New() (*Transport, error) {
	return &Transport{client: &http.Client{Timeout: 30 * time.Second}}, nil
}

func (t *Transport) SetLogger(logger sdk.Logger) { t.logger = logger }

func (t *Transport) Do(ctx context.Context, req *sdk.Request) (*sdk.Response, error) {
	if req == nil {
		return nil, errors.New("http adapter: nil request")
	}
	for name, value := range req.Headers {
		if strings.EqualFold(name, "Authorization") || (strings.EqualFold(name, "Cookie") && !guestCookieAllowed(value)) {
			return nil, fmt.Errorf("%w: credential header refused", sdk.ErrAuthenticationRequired)
		}
	}
	method := req.Method
	if method == "" {
		method = http.MethodGet
	}
	hreq, err := http.NewRequestWithContext(ctx, method, req.URL, bytes.NewReader(req.Body))
	if err != nil {
		return nil, err
	}
	for k, v := range req.Headers {
		hreq.Header.Set(k, v)
	}
	resp, err := t.client.Do(hreq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if t.logger != nil {
		t.logger.Printf("HTTP %s %s -> %d", method, protocol.RedactedURL(req.URL), resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	headers := map[string]string{}
	for k, values := range resp.Header {
		if strings.EqualFold(k, "Set-Cookie") || strings.EqualFold(k, "Cookie") {
			continue
		}
		if len(values) > 0 {
			headers[k] = values[0]
		}
	}
	cookies := map[string]string{}
	for _, cookie := range resp.Cookies() {
		if cookie.Name != "" {
			cookies[cookie.Name] = cookie.Value
		}
	}
	return &sdk.Response{StatusCode: resp.StatusCode, Headers: headers, Body: body, Cookies: cookies}, nil
}

// guestCookieAllowed accepts only the anonymous cookie built from this installation.
// Account cookies, including a session cookie, stay refused.
func guestCookieAllowed(value string) bool {
	lower := strings.ToLower(value)
	if strings.Contains(lower, "session") || strings.Contains(lower, "odin_tt") {
		return false
	}
	return strings.Contains(value, "passport_csrf_token=;") &&
		strings.Contains(value, "install_id=") &&
		strings.Contains(value, "ttreq=")
}
