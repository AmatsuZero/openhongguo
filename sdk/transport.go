package sdk

import "context"

// Request 和 Response 只承载匿名协议实际发送的线上字段。
type Request struct {
	Method string
	// URL 是绝对地址，包含查询参数。
	URL     string
	Headers map[string]string
	Body    []byte
	// NoRedirect 禁止自动跟随重定向，公开 Web 读取使用它。
	NoRedirect bool
	// MaxResponseBytes 是原始响应体的大小上限；零值沿用 Transport 默认值。
	MaxResponseBytes int64
}

// Response 是 Transport 收到的响应。Body 是原始载荷，可能经过 gzip 压缩，
// 由客户端负责解码。
type Response struct {
	StatusCode int
	Headers    map[string]string
	Body       []byte
	// Cookies 是来自 Set-Cookie 的名值对，不会写入日志。
	Cookies map[string]string
}

// Transport 发送一个请求。只有收不到响应时才返回错误；
// HTTP 错误状态码和其他响应一样，作为 Response 返回。
// 请求 URL 的查询参数里带有设备 id，所以记录日志的 Transport 应将其脱敏，
// 客户端自己的日志也是这样做的。
type Transport interface {
	Do(ctx context.Context, req *Request) (*Response, error)
}
