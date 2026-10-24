package sdk

import "context"

// Request and Response carry only the wire fields the anonymous protocol sends.
type Request struct {
	Method  string
	URL     string
	Headers map[string]string
	Body    []byte
}

type Response struct {
	StatusCode int
	Headers    map[string]string
	Body       []byte
	// Cookies are name/value pairs from Set-Cookie. They are not logged.
	Cookies map[string]string
}

type Transport interface {
	Do(ctx context.Context, req *Request) (*Response, error)
}
