package model

// ComicReader 是官方 Web 漫画页公开的正文。它只包含当前网页返回的
// 可读章节；NextEpisodeID 只是网页给出的后续标识，不代表该章节可读。
type ComicReader struct {
	BookID              string         `json:"book_id"`
	Title               string         `json:"title"`
	CanReadEpisodeCount int            `json:"can_read_episode_cnt"`
	Chapters            []ComicChapter `json:"chapters"`
	NextEpisodeID       string         `json:"next_episode_id,omitempty"`
	SourceURL           string         `json:"source_url,omitempty"`
}

// ComicChapter 是一话公开漫画正文，Number 从 1 开始。
type ComicChapter struct {
	ID     string      `json:"id"`
	Number int         `json:"number"`
	Pages  []ComicPage `json:"pages"`
}

// ComicPage 是一张漫画长图。URL 是官方网页返回的签名地址，调用方应按
// 自身的安全边界代理或短期使用，不应把它持久化为稳定地址。
type ComicPage struct {
	ID     string `json:"id"`
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}
