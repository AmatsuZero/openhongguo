package protocol

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/beilunyang/openhongguo/sdk/model"
)

var comicRouterAssignment = regexp.MustCompile(`\b_ROUTER_DATA\s*=`)
var comicImageHost = regexp.MustCompile(`^p[0-9]+-novel-sign\.byteimg\.com$`)
var comicIntegerID = regexp.MustCompile(`^[0-9]+$`)

// comicID preserves decimal IDs without floating-point conversion, but never
// turns malformed JSON objects, arrays or booleans into chapter identifiers.
type comicID string

func (id *comicID) UnmarshalJSON(raw []byte) error {
	raw = bytes.TrimSpace(raw)
	if bytes.Equal(raw, []byte("null")) {
		*id = ""
		return nil
	}
	if len(raw) > 0 && raw[0] == '"' {
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return err
		}
		*id = comicID(strings.TrimSpace(value))
		return nil
	}
	if !comicIntegerID.Match(raw) {
		return fmt.Errorf("comic ID must be a string or unsigned decimal integer")
	}
	*id = comicID(raw)
	return nil
}

func (id comicID) String() string { return string(id) }

// ParseComicReader 只读取官方 Web 页内的 JSON，不执行脚本。
// 公开数量为零时不返回图片；任何公开章节损坏都会显式失败。
func ParseComicReader(raw []byte) (*model.ComicReader, error) {
	match := comicRouterAssignment.FindIndex(raw)
	if match == nil {
		return nil, fmt.Errorf("comic page has no public reader data")
	}
	var router struct {
		LoaderData map[string]json.RawMessage `json:"loaderData"`
	}
	if err := json.NewDecoder(bytes.NewReader(raw[match[1]:])).Decode(&router); err != nil {
		return nil, fmt.Errorf("invalid comic router data: %w", err)
	}
	var page struct {
		BookID    comicID         `json:"book_id"`
		Title     string          `json:"title"`
		CanRead   *int            `json:"can_read_episode_cnt"`
		NextID    comicID         `json:"next_vid"`
		Content   json.RawMessage `json:"content"`
		IsSuccess *bool           `json:"isSuccess"`
	}
	data, ok := router.LoaderData["comic_(id)/page"]
	if !ok {
		return nil, fmt.Errorf("comic page has no public reader loader")
	}
	if err := json.Unmarshal(data, &page); err != nil {
		return nil, fmt.Errorf("invalid comic loader: %w", err)
	}
	if page.IsSuccess != nil && !*page.IsSuccess {
		return nil, fmt.Errorf("comic page reports reader unavailable")
	}
	if page.CanRead == nil || *page.CanRead < 0 || *page.CanRead > 10000 || page.BookID.String() == "" || strings.TrimSpace(page.Title) == "" {
		return nil, fmt.Errorf("comic loader has invalid identity or access count")
	}
	reader := &model.ComicReader{BookID: page.BookID.String(), Title: strings.TrimSpace(page.Title), CanReadEpisodeCount: *page.CanRead, Chapters: []model.ComicChapter{}, NextEpisodeID: page.NextID.String()}
	if *page.CanRead == 0 {
		return reader, nil
	}
	var encoded string
	if err := json.Unmarshal(page.Content, &encoded); err != nil || strings.TrimSpace(encoded) == "" {
		return nil, fmt.Errorf("comic page has no valid public body")
	}
	var content struct {
		Chapters []struct {
			ID       comicID `json:"item_id"`
			Pictures []struct {
				URL    string `json:"pic_url"`
				Width  int    `json:"width"`
				Height int    `json:"height"`
			} `json:"pic_list"`
		} `json:"chapter_info_list"`
	}
	if err := json.Unmarshal([]byte(encoded), &content); err != nil {
		return nil, fmt.Errorf("invalid comic content: %w", err)
	}
	if len(content.Chapters) < *page.CanRead {
		return nil, fmt.Errorf("comic public chapter data is incomplete")
	}
	seen := map[string]bool{}
	for index, ch := range content.Chapters[:*page.CanRead] {
		id := ch.ID.String()
		if id == "" || seen[id] || len(ch.Pictures) == 0 {
			return nil, fmt.Errorf("invalid comic chapter or missing images")
		}
		seen[id] = true
		chapter := model.ComicChapter{ID: id, Number: index + 1, Pages: make([]model.ComicPage, 0, len(ch.Pictures))}
		for number, picture := range ch.Pictures {
			if !comicImageAllowed(picture.URL) || picture.Width <= 0 || picture.Height <= 0 || picture.Width > 20000 || picture.Height > 20000 {
				return nil, fmt.Errorf("invalid comic image URL or dimensions")
			}
			chapter.Pages = append(chapter.Pages, model.ComicPage{ID: fmt.Sprintf("%s-%d", id, number+1), URL: picture.URL, Width: picture.Width, Height: picture.Height})
		}
		reader.Chapters = append(reader.Chapters, chapter)
	}
	return reader, nil
}

func comicImageAllowed(address string) bool {
	u, err := url.Parse(address)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Fragment != "" || !comicImageHost.MatchString(strings.ToLower(u.Hostname())) || (u.Port() != "" && u.Port() != "443") {
		return false
	}
	return strings.HasPrefix(u.EscapedPath(), "/novel-pic/")
}
