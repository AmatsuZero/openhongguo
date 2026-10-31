package sdk

import (
	"context"
	"fmt"
	"mime"
	"regexp"
	"strings"
	"time"

	"github.com/beilunyang/openhongguo/internal/protocol"
	"github.com/beilunyang/openhongguo/sdk/model"
)

const comicHTMLLimit = 4 << 20
const officialComicOrigin = "https://hongguoduanju.com"

var comicBookID = regexp.MustCompile(`^[0-9]{4,25}$`)

// GetComicReader 加载官方 Web 页当前公开的漫画正文。
// 不发送游客或账号凭据，不注册设备；仅返回网页明确公开的章节。
// NextEpisodeID 不表示后续章节可由 Web 访问，也不表示付费状态。
func (c *Client) GetComicReader(ctx context.Context, bookID string) (*model.ComicReader, error) {
	if !comicBookID.MatchString(bookID) {
		return nil, fmt.Errorf("%w: invalid comic book ID", ErrParamInvalid)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	response, err := c.transport.Do(ctx, &Request{
		Method: "GET", URL: c.webOrigin + "/comic/" + bookID,
		Headers:    map[string]string{"Accept": "text/html", "User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"},
		NoRedirect: true, MaxResponseBytes: comicHTMLLimit,
	})
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("%w: comic page request failed", ErrTransport)
	}
	if response == nil {
		return nil, fmt.Errorf("%w: empty comic page response", ErrTransport)
	}
	if response.StatusCode >= 300 && response.StatusCode < 400 {
		return nil, fmt.Errorf("%w: comic page redirect refused", ErrContentUnavailable)
	}
	if response.StatusCode == 404 {
		return nil, ErrContentUnavailable
	}
	if response.StatusCode == 429 {
		return nil, ErrRateLimited
	}
	if response.StatusCode == 401 || response.StatusCode == 403 {
		return nil, ErrAuthenticationRequired
	}
	if response.StatusCode != 200 {
		return nil, &APIError{HTTPStatus: response.StatusCode, Message: "comic page request failed"}
	}
	var contentType string
	for name, value := range response.Headers {
		if strings.EqualFold(name, "Content-Type") {
			contentType = value
			break
		}
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != "text/html" {
		return nil, fmt.Errorf("%w: invalid comic page content type", ErrTransport)
	}
	if len(response.Body) > comicHTMLLimit {
		return nil, fmt.Errorf("%w: comic page too large", ErrTransport)
	}
	raw, err := decodeBody(response)
	if err != nil {
		return nil, err
	}
	if len(raw) > comicHTMLLimit {
		return nil, fmt.Errorf("%w: decoded comic page too large", ErrTransport)
	}
	reader, err := protocol.ParseComicReader(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrContentUnavailable, err)
	}
	if reader.BookID != bookID {
		return nil, fmt.Errorf("%w: comic book ID mismatch", ErrContentUnavailable)
	}
	reader.SourceURL = officialComicOrigin + "/comic/" + bookID
	return reader, nil
}
