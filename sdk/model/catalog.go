package model

import "time"

// SeriesStatus is the publication state of a series, carried by
// data.video_data.series_status.
//
// Names come from the shipped enum com.dragon.read.rpc.model.SeriesStatus
// (SeriesUpdating, SeriesEnd, SeriesUpdateTaday, SeriesUpdateStop); values 0
// and 1 were also observed live.
type SeriesStatus int

const (
	// SeriesUpdating is an unfinished series ("更新至 N 集").
	SeriesUpdating SeriesStatus = 0
	// SeriesEnd is a completed series ("全 N 集").
	SeriesEnd SeriesStatus = 1
	// SeriesUpdatedToday is the enum's "updated today" state (inferred).
	SeriesUpdatedToday SeriesStatus = 3
	// SeriesUpdateStopped is the enum's "updates stopped" state (inferred).
	SeriesUpdateStopped SeriesStatus = 4
)

// String names the state for output.
func (s SeriesStatus) String() string {
	switch s {
	case SeriesUpdating:
		return "updating"
	case SeriesEnd:
		return "finished"
	case SeriesUpdatedToday:
		return "updated_today"
	case SeriesUpdateStopped:
		return "stopped"
	default:
		return "unknown"
	}
}

// PayType is pay_info.pay_type, from com.dragon.read.rpc.model.VideoPayType.
type PayType int

const (
	// PayFree is watchable without an account.
	PayFree PayType = 0
	// PayVip needs a subscription.
	PayVip PayType = 1
	// PaySingle needs a one-off purchase.
	PaySingle PayType = 2
)

// String names the pay type for output.
func (p PayType) String() string {
	switch p {
	case PayFree:
		return "free"
	case PayVip:
		return "vip"
	case PaySingle:
		return "single"
	default:
		return "unknown"
	}
}

// ContentType is video content_type, from
// com.dragon.read.rpc.model.VideoContentType. The shipped enum mixes small
// values with a 1000-based block; both are reproduced here.
type ContentType int

const (
	ContentUnknown         ContentType = 0
	ContentShortSeries     ContentType = 1
	ContentScenePlay       ContentType = 2
	ContentExpertRecommend ContentType = 3
	ContentPPTRecommend    ContentType = 4
	ContentMovie           ContentType = 5
	ContentTelePlay        ContentType = 6
	ContentShortPlay       ContentType = 7
	ContentDropMaterial    ContentType = 1000
	ContentPUGC            ContentType = 1001
	ContentUnrealShortPlay ContentType = 1002
	ContentAlbum           ContentType = 1003
	ContentMotionComic     ContentType = 1004
)

// String names the content type for output.
func (c ContentType) String() string {
	switch c {
	case ContentShortSeries:
		return "short_series"
	case ContentScenePlay:
		return "scene_play"
	case ContentExpertRecommend:
		return "expert_recommend"
	case ContentPPTRecommend:
		return "ppt_recommend"
	case ContentMovie:
		return "movie"
	case ContentTelePlay:
		return "teleplay"
	case ContentShortPlay:
		return "short_play"
	case ContentDropMaterial:
		return "drop_material"
	case ContentPUGC:
		return "pugc"
	case ContentUnrealShortPlay:
		return "unreal_short_play"
	case ContentAlbum:
		return "album"
	case ContentMotionComic:
		return "motion_comic"
	default:
		return "unknown"
	}
}

// PlatformType is video_platform, from
// com.dragon.read.rpc.model.VideoPlatformType.
type PlatformType int

const (
	PlatformUnknown   PlatformType = 0
	PlatformDouyin    PlatformType = 1
	PlatformUGCVideo  PlatformType = 2
	PlatformOuter     PlatformType = 3
	PlatformXigua     PlatformType = 4
	PlatformChameleon PlatformType = 5
	PlatformMixed     PlatformType = 1024
)

// String names the platform for output.
func (p PlatformType) String() string {
	switch p {
	case PlatformDouyin:
		return "douyin"
	case PlatformUGCVideo:
		return "ugc_video"
	case PlatformOuter:
		return "outer"
	case PlatformXigua:
		return "xigua"
	case PlatformChameleon:
		return "chameleon"
	case PlatformMixed:
		return "mixed"
	default:
		return "unknown"
	}
}

// InfoKind is secondary_infos[].data_type, from
// com.dragon.read.rpc.model.SecondaryInfoDataType.
type InfoKind int

const (
	// InfoSeasonOrder is the "第 N 季" row (observed with value 0).
	InfoSeasonOrder InfoKind = 0
	// InfoRecommendReason is a recommendation reason.
	InfoRecommendReason InfoKind = 1
	// InfoRankScore is a ranking position.
	InfoRankScore InfoKind = 2
	// InfoCategory is a category tag such as "末世" (observed with value 3).
	InfoCategory InfoKind = 3
	// InfoScore is a user score.
	InfoScore InfoKind = 4
	// InfoAuthorName is an author name.
	InfoAuthorName InfoKind = 8
	// InfoTags is a tag list.
	InfoTags InfoKind = 9
	// InfoCreationStatus is an ongoing/finished note.
	InfoCreationStatus InfoKind = 11
	// InfoTwoActors is the cast line "演员·A B" (observed with value 33).
	InfoTwoActors InfoKind = 33
	// InfoPlotTag is a plot tag.
	InfoPlotTag InfoKind = 40
)

// String names the row kind for output.
func (k InfoKind) String() string {
	switch k {
	case InfoSeasonOrder:
		return "season"
	case InfoRecommendReason:
		return "reason"
	case InfoRankScore:
		return "rank"
	case InfoCategory:
		return "category"
	case InfoScore:
		return "score"
	case InfoAuthorName:
		return "author"
	case InfoTags:
		return "tags"
	case InfoCreationStatus:
		return "creation"
	case InfoTwoActors:
		return "cast"
	case InfoPlotTag:
		return "plot"
	default:
		return "other"
	}
}

// InfoRow is one secondary_infos entry: a labelled line on the detail page.
type InfoRow struct {
	Kind   InfoKind `json:"kind"`
	Text   string   `json:"text"`
	Schema string   `json:"schema,omitempty"`
}

// FilterOption is one selectable value on a category page.
type FilterOption struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// FilterGroup is one filter dimension (status, sort, sub-category, length).
type FilterGroup struct {
	Key     string         `json:"key"`
	Options []FilterOption `json:"options"`
}

// CategoryPage is one category landing page with its filter options.
type CategoryPage struct {
	CategoryID   string          `json:"category_id,omitempty"`
	CategoryName string          `json:"category_name,omitempty"`
	Description  string          `json:"description,omitempty"`
	Filters      []FilterGroup   `json:"filters,omitempty"`
	Tabs         []FilterOption  `json:"tabs,omitempty"`
	DefaultTab   int             `json:"default_tab,omitempty"`
	Items        []SeriesSummary `json:"items"`
	HasMore      bool            `json:"has_more"`
	NextOffset   int             `json:"next_offset,omitempty"`
}

// SeriesSummary is a public catalog row. IDs and titles are the stable fields.
type SeriesSummary struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	CoverURL    string `json:"cover_url,omitempty"`
	// CoverExpiresAt is the signed expiry of CoverURL (x-expires); refresh the
	// cover from the API after it passes.
	CoverExpiresAt time.Time `json:"cover_expires_at,omitempty"`
	EpisodeCount   int       `json:"episode_count,omitempty"`
	// Author, Category and Words are filled by a category page row.
	Author   string `json:"author,omitempty"`
	Category string `json:"category,omitempty"`
	Words    int64  `json:"words,omitempty"`
}

// Series is the detail view of one anonymous series.
type Series struct {
	SeriesSummary
	// Status is the publication state; PayType is the paywall state.
	Status  SeriesStatus `json:"status,omitempty"`
	PayType PayType      `json:"pay_type,omitempty"`
	// ContentType and Platform describe what the title is and where it comes
	// from (short series, movie, motion comic, ...).
	ContentType ContentType  `json:"content_type,omitempty"`
	Platform    PlatformType `json:"platform,omitempty"`
	// Heat is the raw hot score; the app renders it as "红果热度值N万".
	Heat      int64 `json:"heat,omitempty"`
	PlayCount int64 `json:"play_count,omitempty"`
	// FollowCount and DiggCount are the counters shown next to the title.
	FollowCount int64 `json:"follow_count,omitempty"`
	DiggCount   int64 `json:"digg_count,omitempty"`
	// CreatedAt is the publishing time (create_time, unix seconds).
	CreatedAt time.Time `json:"created_at,omitempty"`
	// RightText is the badge next to the episode count ("更新至12集").
	RightText string `json:"right_text,omitempty"`
	// Subtitles are the short status lines under the title.
	Subtitles []string `json:"subtitles,omitempty"`
	// AgeGate is 0 for unrestricted titles.
	AgeGate int `json:"age_gate,omitempty"`
	// IsMultiSeason marks a title that belongs to a collection of seasons.
	IsMultiSeason bool `json:"is_multi_season,omitempty"`
	// SeasonIndex is the 1-based season number inside that collection.
	SeasonIndex int `json:"season_index,omitempty"`
	// RelatedAlbumID is the collection id shared by the seasons.
	RelatedAlbumID string `json:"related_album_id,omitempty"`
	// RelatedNovelBookID is the novel a series adapts, when published.
	RelatedNovelBookID string `json:"related_novel_book_id,omitempty"`
	// Novel is the source novel the detail page links (video_relate_book),
	// when the service names one.
	Novel *Novel `json:"novel,omitempty"`
	// Rows are the detail page's secondary info lines (season, category, cast).
	Rows []InfoRow `json:"rows,omitempty"`
}

// Novel is the novel a series is adapted from.
type Novel struct {
	BookID string `json:"book_id"`
	// Title is the novel's own name; ShortTitle the name it shares with the
	// series.
	Title          string      `json:"title"`
	ShortTitle     string      `json:"short_title,omitempty"`
	Status         NovelStatus `json:"status"`
	Tags           []string    `json:"tags,omitempty"`
	Reads          int64       `json:"reads,omitempty"`
	CoverURL       string      `json:"cover_url,omitempty"`
	CoverExpiresAt time.Time   `json:"cover_expires_at,omitempty"`
	// ShowAtEpisode is the episode where the player offers the novel.
	ShowAtEpisode int `json:"show_at_episode,omitempty"`
}

// NovelStatus is the client's NovelCreationStatus; -1 when not given.
type NovelStatus int

const (
	NovelFinished     NovelStatus = 0
	NovelOngoing      NovelStatus = 1
	NovelUpdatedToday NovelStatus = 3
)

// String names the status as the CLI prints it.
func (s NovelStatus) String() string {
	switch s {
	case NovelFinished:
		return "finished"
	case NovelOngoing:
		return "ongoing"
	case NovelUpdatedToday:
		return "updated today"
	}
	return "unknown"
}

// SeriesDetail is one series with its episodes, as a batch read returns it.
type SeriesDetail struct {
	Series   *Series   `json:"series"`
	Episodes []Episode `json:"episodes"`
}

// Episode is one publicly listed episode. Number is 1-based for callers.
type Episode struct {
	ID       string        `json:"id"`
	SeriesID string        `json:"series_id"`
	Number   int           `json:"number"`
	Title    string        `json:"title,omitempty"`
	Duration time.Duration `json:"-"`
	// Playable is false when the service forbids playing this episode
	// (video_list[].disable_play).
	Playable bool `json:"playable"`
	// Vertical marks a portrait-framed episode.
	Vertical bool `json:"vertical,omitempty"`
	// IsNew marks an episode published in the current update.
	IsNew bool `json:"is_new,omitempty"`
	// CoverURL is the episode still; CoverExpiresAt is its signed expiry.
	CoverURL       string    `json:"cover_url,omitempty"`
	CoverExpiresAt time.Time `json:"cover_expires_at,omitempty"`
	// ContentType is the same enum as Series.ContentType.
	ContentType ContentType `json:"content_type,omitempty"`
	// CommentCount and DiggCount are the episode counters.
	CommentCount int64 `json:"comment_count,omitempty"`
	DiggCount    int64 `json:"digg_count,omitempty"`
}

// Season is one entry of an album (multi-season collection).
type Season struct {
	SeriesID     string       `json:"series_id"`
	Title        string       `json:"title,omitempty"`
	Index        int          `json:"index,omitempty"`
	CoverURL     string       `json:"cover_url,omitempty"`
	EpisodeCount int          `json:"episode_count,omitempty"`
	Status       SeriesStatus `json:"status,omitempty"`
}

// Album is a collection of seasons, from /novel/player/album_detail/v1/.
type Album struct {
	ID             string    `json:"id"`
	Title          string    `json:"title,omitempty"`
	Description    string    `json:"description,omitempty"`
	CoverURL       string    `json:"cover_url,omitempty"`
	CoverExpiresAt time.Time `json:"cover_expires_at,omitempty"`
	SeasonCount    int       `json:"season_count,omitempty"`
	Seasons        []Season  `json:"seasons,omitempty"`
}

// Suggestion is one search suggestion. Text is what the search box shows;
// Keyword is what it submits (the content ID for a direct-hit row).
type Suggestion struct {
	Text    string `json:"text"`
	Keyword string `json:"keyword,omitempty"`
	Kind    int    `json:"kind,omitempty"`
}

// Recommend is the related-video cell of one series: the rows plus the cell
// title and the paging session the client keeps for follow-up requests.
type Recommend struct {
	CellTitle string          `json:"cell_title,omitempty"`
	SessionID string          `json:"session_id,omitempty"`
	Items     []SeriesSummary `json:"items"`
}

// ShowTypeDetailRelatedVideo is the plan cell type of the related-video tab
// (ShowType.DetailRelatedVideo value; the Kotlin ordinal is 58).
const ShowTypeDetailRelatedVideo = 160

// RankList is one ranking channel of an activity.
type RankList struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
	Type int    `json:"type,omitempty"`
	Size int    `json:"size,omitempty"`
}

// NewReleasePage is one day of the 上新 calendar.
type NewReleasePage struct {
	// Date is the day answered (YYYYMMDD); Dates are the days the calendar
	// offers, a week either side of today.
	Date    string       `json:"date,omitempty"`
	Dates   []string     `json:"dates,omitempty"`
	Items   []NewRelease `json:"items"`
	HasMore bool         `json:"has_more"`
	Cursor  string       `json:"cursor,omitempty"`
}

// NewRelease is one title of the 上新 calendar: out already (Online) or
// announced for PublishAt.
type NewRelease struct {
	SeriesID       string    `json:"series_id"`
	Title          string    `json:"title"`
	Description    string    `json:"description,omitempty"`
	CoverURL       string    `json:"cover_url,omitempty"`
	CoverExpiresAt time.Time `json:"cover_expires_at,omitempty"`
	EpisodeCount   int       `json:"episode_count,omitempty"`
	Online         bool      `json:"online"`
	PublishAt      time.Time `json:"publish_at,omitempty"`
	Categories     []string  `json:"categories,omitempty"`
	// Heat is "378万热度" for a title that is out and "159.7万人预约" for
	// one that is announced; Note is the announcement ("预计10月26日上线").
	Heat string `json:"heat,omitempty"`
	Note string `json:"note,omitempty"`
}

// Person is an actor's or a character's page.
type Person struct {
	UserID      string `json:"user_id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	AvatarURL   string `json:"avatar_url,omitempty"`
	// Tags are the page's titles ("女演员", "虚拟人物"); Labels the small
	// badges under the name ("水瓶座").
	Tags      []string `json:"tags,omitempty"`
	Labels    []string `json:"labels,omitempty"`
	Followers int64    `json:"followers,omitempty"`
	Likes     int64    `json:"likes,omitempty"`
}

// WorkPage is one page of a person's works (作品 › 剧作).
type WorkPage struct {
	Items   []Work `json:"items"`
	HasMore bool   `json:"has_more"`
	Cursor  string `json:"cursor,omitempty"`
}

// Work is one series a person appears in.
type Work struct {
	SeriesID       string    `json:"series_id"`
	Title          string    `json:"title"`
	Description    string    `json:"description,omitempty"`
	CoverURL       string    `json:"cover_url,omitempty"`
	CoverExpiresAt time.Time `json:"cover_expires_at,omitempty"`
	EpisodeCount   int       `json:"episode_count,omitempty"`
	// Tags are the card's lines, such as "爱情" and "70集".
	Tags  []string `json:"tags,omitempty"`
	Heat  int64    `json:"heat,omitempty"`
	Plays int64    `json:"plays,omitempty"`
}

// HotSearch is the search page before a query: the 猜你想搜 row and the hot
// lists under it.
type HotSearch struct {
	Guesses []HotItem  `json:"guesses,omitempty"`
	Boards  []HotBoard `json:"boards,omitempty"`
}

// HotBoard is one hot list. Title lists (短剧热搜榜, 漫剧热搜榜, 短剧新剧榜)
// fill Items; the 热点话题榜 fills Topics. Which lists appear varies by call.
type HotBoard struct {
	Name   string     `json:"name"`
	Items  []HotItem  `json:"items,omitempty"`
	Topics []HotTopic `json:"topics,omitempty"`
}

// HotItem is one title on a hot list, in list order.
type HotItem struct {
	Position       int         `json:"position"`
	SeriesID       string      `json:"series_id"`
	Title          string      `json:"title"`
	CoverURL       string      `json:"cover_url,omitempty"`
	CoverExpiresAt time.Time   `json:"cover_expires_at,omitempty"`
	EpisodeCount   int         `json:"episode_count,omitempty"`
	ContentType    ContentType `json:"content_type,omitempty"`
	// SearchScore is the list's own figure ("355万热搜值", sometimes with
	// "·第1季"); Heat is the title's overall heat ("4102万热度").
	SearchScore string `json:"search_score,omitempty"`
	Heat        string `json:"heat,omitempty"`
	// Tag is the corner badge, such as 新剧.
	Tag string `json:"tag,omitempty"`
}

// HotTopic is one entry of the 热点话题榜. Query is the search it opens.
type HotTopic struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Query string `json:"query,omitempty"`
	Heat  string `json:"heat,omitempty"`
}

// RankItem is one row of a ranking.
type RankItem struct {
	Rank     int    `json:"rank,omitempty"`
	Votes    int64  `json:"votes,omitempty"`
	Kind     int    `json:"kind,omitempty"`
	Actor    string `json:"actor,omitempty"`
	SeriesID string `json:"series_id,omitempty"`
	Title    string `json:"title,omitempty"`
	CoverURL string `json:"cover_url,omitempty"`
	// Score is the displayed heat ("2550万热度", "998万推荐", "平台推荐").
	Score string `json:"score,omitempty"`
	// EpisodeCount is filled by the short-drama ranking.
	EpisodeCount int `json:"episode_count,omitempty"`
	// Position is the row's 1-based place on a short-drama ranking page.
	// Rank is the service's own number, which repeats around a promoted row
	// and is personalised on the 推荐榜 lists.
	Position int `json:"position,omitempty"`
	// Promoted marks a row the service inserts into a short-drama ranking:
	// it reads "平台推荐" instead of a heat, and its rank copies the next
	// row's (observed).
	Promoted bool `json:"promoted,omitempty"`
	// ActorID, AvatarURL, Intro, and Details fill an actor row of the
	// short-drama ranking, whose Actor is the name and Score the 人气.
	// Intro lists works; Details holds lines such as "67部作品".
	ActorID string `json:"actor_id,omitempty"`
	// UserID is the actor's account: the id ActorProfile and ActorWorks take.
	UserID    string   `json:"user_id,omitempty"`
	AvatarURL string   `json:"avatar_url,omitempty"`
	Intro     string   `json:"intro,omitempty"`
	Details   []string `json:"details,omitempty"`
	// Change is the row's move since the previous ranking, when given.
	Change *RankChange `json:"change,omitempty"`
}

// RankChange is a ranking row's move (RankChangeType in the client).
type RankChange struct {
	Kind RankChangeKind `json:"kind"`
	// Places is how far the row moved; 0 for a new or unchanged row.
	Places int `json:"places,omitempty"`
}

// RankChangeKind values are the client's RankChangeType.
type RankChangeKind int

const (
	RankNew       RankChangeKind = 0
	RankUp        RankChangeKind = 1
	RankDown      RankChangeKind = 2
	RankUnchanged RankChangeKind = 3
)

// String is the arrow the CLI prints for the move.
func (k RankChangeKind) String() string {
	switch k {
	case RankNew:
		return "new"
	case RankUp:
		return "up"
	case RankDown:
		return "down"
	case RankUnchanged:
		return "same"
	}
	return "unknown"
}

// Ranking is one activity ranking page.
type Ranking struct {
	ListName      string     `json:"list_name,omitempty"`
	Lists         []RankList `json:"lists,omitempty"`
	Items         []RankItem `json:"items"`
	HasMore       bool       `json:"has_more"`
	NextOffset    int        `json:"next_offset,omitempty"`
	ActivityStage int        `json:"activity_stage,omitempty"`
	CanVote       bool       `json:"can_vote,omitempty"`
	SessionID     string     `json:"session_id,omitempty"`
}

// FilterRow is one selector row of the short-drama filter page. Key is the
// row's select_items key (genre, category_dim_theme, category_dim_role,
// category_dim_epoch, sort, gender, online_time, creation_status); an
// option's Value is what a query selects. The rows depend on the genre.
type FilterRow struct {
	Key     string         `json:"key"`
	Name    string         `json:"name"`
	Options []FilterOption `json:"options"`
}

// ShortFilterPage is one page of short dramas matching a filter.
type ShortFilterPage struct {
	Items   []SeriesSummary `json:"items"`
	HasMore bool            `json:"has_more"`
	// Cursor continues the list; it carries the series already shown, which
	// the service skips.
	Cursor string `json:"cursor,omitempty"`
}

// RankBoard is one list of a ranking content (推荐榜, 热播榜, ...).
type RankBoard struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Selected bool   `json:"selected,omitempty"`
}

// RankContent is one content of the short-drama ranking page (全部, 真人剧,
// 漫剧, AI剧, 系列剧, 演员) with its own lists.
type RankContent struct {
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	Selected bool        `json:"selected,omitempty"`
	Boards   []RankBoard `json:"boards"`
}

// ShortRanking is one page of a short-drama ranking list.
type ShortRanking struct {
	// Contents is the page's catalog: every content and its lists.
	Contents []RankContent `json:"contents"`
	// Content and Board are the IDs this page answers.
	Content string     `json:"content"`
	Board   string     `json:"board"`
	Version string     `json:"version,omitempty"`
	Items   []RankItem `json:"items"`
	HasMore bool       `json:"has_more"`
	Cursor  string     `json:"cursor,omitempty"`
}
