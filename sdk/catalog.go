package sdk

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/beilunyang/openhongguo/internal/protocol"
	"github.com/beilunyang/openhongguo/internal/protocol/sign"
	"github.com/beilunyang/openhongguo/sdk/model"
)

// Page is one anonymous result page.
type Page[T any] struct {
	Items   []T    `json:"items"`
	Cursor  string `json:"cursor,omitempty"`
	HasMore bool   `json:"has_more"`
}

// DiscoverOptions selects a discovery page.
// Channel is a bookmall name. An empty channel is 推荐.
// Cursor is the token from the previous page.
type DiscoverOptions struct {
	Cursor  string
	Channel string
}

// SearchOptions selects a search page.
// Tab is 综合, 剧集, 视频, 讨论, 小说, 用户, or 听书. An empty tab is 综合.
// Cursor is the next offset.
type SearchOptions struct {
	Cursor   string
	PageSize int
	Tab      string
	// Filters narrows the 综合 tab to SearchFilter* values; several widen
	// each other. A cursor does not carry them, so pass the same filters
	// with it.
	Filters []string
}

// Search filters of the 综合 tab, as the app's filter panel sends them in
// selected_items (observed 2026-10-24). The two comic filters return
// content_type 1004 alike; the service tells them apart.
const (
	SearchFilterShortPlay = "content_type_short_play"       // 真人剧
	SearchFilterComic     = "content_type_dynamic_comic"    // 漫剧
	SearchFilterAIComic   = "content_type_ai_dynamic_comic" // AI剧
	SearchFilterFilm      = "content_type_long_video"       // 影视剧
	SearchFilterUnwatched = "search_range_unread"           // 未看过
)

// EpisodeOptions selects an episode page. A cursor is not part of phase 1.
type EpisodeOptions struct {
	Cursor   string
	PageSize int
}

// Discover loads one anonymous bookmall channel.
// The first page is the tab preload plus the first unlimited cell/change page.
// A cursor loads only the next cell/change page.
func (c *Client) Discover(ctx context.Context, opts DiscoverOptions) (*Page[model.SeriesSummary], error) {
	channel, err := protocol.ResolveDiscoverChannel(opts.Channel)
	if err != nil {
		return nil, err
	}
	p, err := c.ensureSearchGuest(ctx)
	if err != nil {
		return nil, err
	}
	id := c.identity(p)
	if opts.Cursor != "" {
		next, err := protocol.DecodeFeedCursor(opts.Cursor)
		if err != nil {
			return nil, err
		}
		if next.TabType != strconvTab(channel.ResponseTab) {
			return nil, fmt.Errorf("discover cursor is not verified")
		}
		more, err := c.cellChange(ctx, p, id, next)
		if err != nil {
			return nil, err
		}
		return finishDiscover(more.Items, next, more)
	}
	ticket := c.clock().UnixMilli()
	rawQuery, headers := sign.SignGET(protocol.SignedDiscoverPairsIn(id, channel.RequestTab, channel.BottomTab), ticket, p.DeviceID, protocol.VersionName, nil)
	raw, _, err := c.doRaw(ctx, "GET", c.apiOrigin+protocol.PathDiscover+"?"+rawQuery, anonymousHeaders(c, p, ticket, headers), nil)
	if err != nil {
		if errors.Is(err, ErrEndOfList) {
			return &Page[model.SeriesSummary]{}, nil
		}
		return nil, err
	}
	parsed, err := protocol.ParseDiscoverTab(raw, channel.ResponseTab)
	if err != nil {
		return nil, err
	}
	if next, ok := protocol.DiscoverContinuationTab(raw, channel.ResponseTab); ok {
		next.Offset = "0"
		more, err := c.cellChange(ctx, p, id, next)
		if err != nil {
			return nil, err
		}
		parsed.Items = mergeSeries(parsed.Items, more.Items)
		return finishDiscover(parsed.Items, next, more)
	}
	if len(parsed.Items) == 0 {
		return nil, ErrAnonymousLimited
	}
	return &Page[model.SeriesSummary]{Items: parsed.Items, HasMore: parsed.HasMore}, nil
}

func (c *Client) cellChange(ctx context.Context, p *DeviceProfile, id protocol.SearchIdentity, next protocol.FeedContinuation) (*protocol.CatalogPage, error) {
	ticket := c.clock().UnixMilli()
	changeQuery, changeHeaders := sign.SignGET(protocol.SignedCellChangePairs(id, next), ticket, p.DeviceID, protocol.VersionName, nil)
	moreRaw, _, err := c.doRaw(ctx, "GET", c.apiOrigin+protocol.PathCellChange+"?"+changeQuery, anonymousHeaders(c, p, ticket, changeHeaders), nil)
	if err != nil {
		return nil, err
	}
	return protocol.ParseCellChange(moreRaw)
}

func finishDiscover(items []model.SeriesSummary, next protocol.FeedContinuation, more *protocol.CatalogPage) (*Page[model.SeriesSummary], error) {
	if len(items) == 0 {
		return nil, ErrAnonymousLimited
	}
	page := &Page[model.SeriesSummary]{Items: items, HasMore: more.HasMore}
	// An empty change page does not advertise a further offset. The preload
	// rows, when present, are still returned.
	if !more.HasMore || more.NextOffset <= 0 || len(more.Items) == 0 {
		return page, nil
	}
	if more.SessionID != "" {
		next.SessionID = more.SessionID
	}
	next.Offset = strconv.Itoa(more.NextOffset)
	cursor, err := protocol.EncodeFeedCursor(next)
	if err != nil {
		return page, nil
	}
	page.Cursor = cursor
	return page, nil
}

func strconvTab(n int) string {
	return strconv.Itoa(n)
}

func mergeSeries(base, extra []model.SeriesSummary) []model.SeriesSummary {
	seen := make(map[string]bool, len(base)+len(extra))
	out := make([]model.SeriesSummary, 0, len(base)+len(extra))
	for _, item := range append(base, extra...) {
		if item.ID == "" || seen[item.ID] {
			continue
		}
		seen[item.ID] = true
		out = append(out, item)
	}
	return out
}

// Search loads anonymous short-play search results for query.
func (c *Client) Search(ctx context.Context, query string, opts SearchOptions) (*Page[model.SeriesSummary], error) {
	if query == "" {
		return nil, fmt.Errorf("missing search query")
	}
	tab, err := protocol.ResolveSearchTab(opts.Tab)
	if err != nil {
		return nil, err
	}
	offset, err := protocol.SearchOffset(opts.Cursor)
	if err != nil {
		return nil, err
	}
	selected, err := searchFilters(opts.Filters, tab)
	if err != nil {
		return nil, err
	}
	p, err := c.ensureSearchGuest(ctx)
	if err != nil {
		return nil, err
	}
	count := opts.PageSize
	if count <= 0 {
		count = 20
	}
	pairs := protocol.SignedSearchPairsFor(c.identity(p), query, count, offset, tab)
	if selected != "" {
		pairs = append(pairs, [2]string{"selected_items", selected})
	}
	ticket := c.clock().UnixMilli()
	rawQuery, headers := sign.SignGET(pairs, ticket, p.DeviceID, protocol.VersionName, nil)
	raw, _, err := c.doRaw(ctx, "GET", c.apiOrigin+protocol.PathSearch+"?"+rawQuery, anonymousHeaders(c, p, ticket, headers), nil)
	if err != nil {
		if errors.Is(err, ErrEndOfList) {
			return &Page[model.SeriesSummary]{}, nil
		}
		return nil, err
	}
	tabType, err := strconv.Atoi(tab)
	if err != nil {
		return nil, err
	}
	parsed, err := protocol.ParseSearchTab(raw, tabType)
	if err != nil {
		return nil, err
	}
	if len(parsed.Items) == 0 {
		return nil, ErrAnonymousLimited
	}
	page := &Page[model.SeriesSummary]{Items: parsed.Items, HasMore: parsed.HasMore}
	if parsed.HasMore && parsed.NextOffset > offset {
		page.Cursor = strconv.Itoa(parsed.NextOffset)
	}
	return page, nil
}

// searchFilters joins filters into the selected_items value. They were only
// seen on the 综合 tab, so other tabs refuse them rather than ignore them.
func searchFilters(filters []string, tab string) (string, error) {
	if len(filters) == 0 {
		return "", nil
	}
	if tab != protocol.SearchTabShortPlay {
		return "", fmt.Errorf("search filters apply to the 综合 tab only")
	}
	seen := map[string]bool{}
	var out []string
	for _, f := range filters {
		switch f {
		case SearchFilterShortPlay, SearchFilterComic, SearchFilterAIComic, SearchFilterFilm, SearchFilterUnwatched:
		default:
			return "", fmt.Errorf("unknown search filter %q", f)
		}
		if !seen[f] {
			seen[f] = true
			out = append(out, f)
		}
	}
	return strings.Join(out, ","), nil
}

// GetSeries loads one anonymous series.
func (c *Client) GetSeries(ctx context.Context, seriesID string) (*model.Series, error) {
	series, _, err := c.seriesDetail(ctx, seriesID)
	return series, err
}

// seriesBatchSize is how many series one batch request asks for. The service
// took 20 and refused 30 with 100001 (observed 2026-10-24); ten keeps a
// response under about 1 MB.
const seriesBatchSize = 10

// GetSeriesBatch loads several series with their episodes, in request order.
// IDs the service does not know are left out; repeated IDs are asked once.
//
// The player's multi_video_detail read answers each series in the same shape
// as GetSeries; larger lists go in requests of seriesBatchSize.
func (c *Client) GetSeriesBatch(ctx context.Context, seriesIDs []string) ([]model.SeriesDetail, error) {
	var ids []string
	seen := map[string]bool{}
	for _, id := range seriesIDs {
		if !validID(id) {
			return nil, fmt.Errorf("invalid series ID %q", id)
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	found := make(map[string]model.SeriesDetail, len(ids))
	for start := 0; start < len(ids); start += seriesBatchSize {
		chunk := ids[start:min(start+seriesBatchSize, len(ids))]
		body, err := protocol.SeriesBatchBody(chunk)
		if err != nil {
			return nil, err
		}
		p, err := c.ensureSearchGuest(ctx)
		if err != nil {
			return nil, err
		}
		ticket := c.clock().UnixMilli()
		rawQuery, headers := sign.SignPOST(protocol.PathSeriesBatch, protocol.DeviceQueryPairs(c.identity(p)), ticket, p.DeviceID, protocol.VersionName, body, nil)
		raw, _, err := c.doRaw(ctx, "POST", c.apiOrigin+protocol.PathSeriesBatch+"?"+rawQuery, anonymousHeaders(c, p, ticket, headers), body)
		if err != nil {
			return nil, err
		}
		got, err := protocol.ParseSeriesBatch(raw)
		if err != nil {
			return nil, err
		}
		for id, d := range got {
			found[id] = d
		}
	}
	out := make([]model.SeriesDetail, 0, len(found))
	for _, id := range ids {
		if d, ok := found[id]; ok {
			out = append(out, d)
		}
	}
	return out, nil
}

// ListEpisodes loads the anonymous episode list. Numbers start at 1.
func (c *Client) ListEpisodes(ctx context.Context, seriesID string, opts EpisodeOptions) (*Page[model.Episode], error) {
	if opts.Cursor != "" || opts.PageSize < 0 {
		return nil, fmt.Errorf("episode pagination is not verified")
	}
	_, episodes, err := c.seriesDetail(ctx, seriesID)
	if err != nil {
		return nil, err
	}
	if len(episodes) == 0 {
		return nil, ErrAnonymousLimited
	}
	return &Page[model.Episode]{Items: episodes}, nil
}

func (c *Client) seriesDetail(ctx context.Context, seriesID string) (*model.Series, []model.Episode, error) {
	if !validID(seriesID) {
		return nil, nil, fmt.Errorf("invalid series ID")
	}
	p, err := c.ensureSearchGuest(ctx)
	if err != nil {
		return nil, nil, err
	}
	body, err := protocol.SeriesBody(seriesID)
	if err != nil {
		return nil, nil, err
	}
	ticket := c.clock().UnixMilli()
	rawQuery, headers := sign.SignPOST(protocol.PathSeries, protocol.DeviceQueryPairs(c.identity(p)), ticket, p.DeviceID, protocol.VersionName, body, nil)
	raw, _, err := c.doRaw(ctx, "POST", c.apiOrigin+protocol.PathSeries+"?"+rawQuery, anonymousHeaders(c, p, ticket, headers), body)
	if err != nil {
		return nil, nil, err
	}
	return protocol.ParseSeries(raw)
}

func anonymousHeaders(c *Client, p *DeviceProfile, ticket int64, headers map[string]string) map[string]string {
	if headers == nil {
		headers = map[string]string{}
	}
	headers["User-Agent"] = protocol.UserAgent(c.deviceOf(p))
	headers["x-ss-dp"] = protocol.AID
	headers["lc"] = "101"
	headers["x-reading-request"] = protocol.ReadingRequestID(ticket, c.randomInt31())
	if p != nil && p.TTReq != "" {
		headers["Cookie"] = guestCookie(p)
	}
	return headers
}

func validID(id string) bool {
	if len(id) < 4 || len(id) > 25 {
		return false
	}
	for _, c := range id {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// Pager walks a cursor-paged list and drops rows already returned by an
// earlier page. The service repeats titles across pages (observed on 小说,
// 漫剧 and 真人剧), so callers that follow cursors need this filter.
type Pager struct {
	fetch func(ctx context.Context, cursor string) (*Page[model.SeriesSummary], error)
	seen  map[string]struct{}
	next  string
	done  bool
	// Pages counts the pages fetched so far.
	Pages int
}

// NewPager builds a deduplicating pager over any page fetch, so a caller that
// only holds an interface (the CLI, gomobile) gets the same rule.
func NewPager(cursor string, fetch func(ctx context.Context, cursor string) (*Page[model.SeriesSummary], error)) *Pager {
	return &Pager{seen: map[string]struct{}{}, next: cursor, fetch: fetch}
}

// DiscoverPager walks one bookmall channel, starting from opts.Cursor.
func (c *Client) DiscoverPager(opts DiscoverOptions) *Pager {
	return NewPager(opts.Cursor, func(ctx context.Context, cursor string) (*Page[model.SeriesSummary], error) {
		o := opts
		o.Cursor = cursor
		return c.Discover(ctx, o)
	})
}

// SearchPager walks one search tab, starting from opts.Cursor.
func (c *Client) SearchPager(query string, opts SearchOptions) *Pager {
	return NewPager(opts.Cursor, func(ctx context.Context, cursor string) (*Page[model.SeriesSummary], error) {
		o := opts
		o.Cursor = cursor
		return c.Search(ctx, query, o)
	})
}

// Next loads the next page and returns the rows not seen before. more reports
// whether another page exists; a page that only repeats known rows is skipped
// so callers do not have to special-case it.
func (p *Pager) Next(ctx context.Context) (items []model.SeriesSummary, more bool, err error) {
	for !p.done {
		page, err := p.fetch(ctx, p.next)
		if err != nil {
			return nil, false, err
		}
		p.Pages++
		p.next = page.Cursor
		p.done = !page.HasMore || page.Cursor == ""
		fresh := make([]model.SeriesSummary, 0, len(page.Items))
		for _, item := range page.Items {
			if _, ok := p.seen[item.ID]; ok {
				continue
			}
			p.seen[item.ID] = struct{}{}
			fresh = append(fresh, item)
		}
		if len(fresh) > 0 || p.done {
			return fresh, !p.done, nil
		}
	}
	return nil, false, nil
}

// Seen reports how many distinct rows the pager has returned.
func (p *Pager) Seen() int { return len(p.seen) }

// AlbumOptions is reserved for album surface selection.
type AlbumOptions struct{}

// GetAlbum loads one album (multi-season collection) with its seasons.
//
// The path and request class are observations from the APK:
// GetVideoAlbumDetailRequest carries albumId, seriesIds and
// needVideoDetailInfo; the response lists the seasons of the collection.
func (c *Client) GetAlbum(ctx context.Context, albumID string, _ AlbumOptions) (*model.Album, error) {
	if !validID(albumID) {
		return nil, fmt.Errorf("invalid album ID")
	}
	p, err := c.ensureSearchGuest(ctx)
	if err != nil {
		return nil, err
	}
	body, err := protocol.AlbumBody(albumID)
	if err != nil {
		return nil, err
	}
	ticket := c.clock().UnixMilli()
	rawQuery, headers := sign.SignPOST(protocol.PathAlbum, protocol.DeviceQueryPairs(c.identity(p)), ticket, p.DeviceID, protocol.VersionName, body, nil)
	raw, _, err := c.doRaw(ctx, "POST", c.apiOrigin+protocol.PathAlbum+"?"+rawQuery, anonymousHeaders(c, p, ticket, headers), body)
	if err != nil {
		return nil, err
	}
	return protocol.ParseAlbum(raw, albumID)
}

// Suggest returns search suggestions for a prefix.
//
// The path comes from the shipped sug page: GET
// /reading/bookapi/search/suggest/v:version/. The response shape is read
// permissively because the list name differs between app versions.
func (c *Client) Suggest(ctx context.Context, prefix string) ([]model.Suggestion, error) {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return nil, fmt.Errorf("missing search prefix")
	}
	p, err := c.ensureSearchGuest(ctx)
	if err != nil {
		return nil, err
	}
	ticket := c.clock().UnixMilli()
	rawQuery, headers := sign.SignGET(protocol.SignedSuggestPairs(c.identity(p), prefix), ticket, p.DeviceID, protocol.VersionName, nil)
	raw, _, err := c.doRaw(ctx, "GET", c.apiOrigin+protocol.PathSuggest+"?"+rawQuery, anonymousHeaders(c, p, ticket, headers), nil)
	if err != nil {
		return nil, err
	}
	return protocol.ParseSuggest(raw)
}

// SeriesRecommend loads the related-video rows for a series.
//
// The surface has no dedicated RPC: the client reuses the generic plan feed
// (GET /reading/bookapi/plan/v:version/) with from=select_panel_recommend and
// scene=10, then takes the single cell whose show_type is 160. Anonymously the
// service answers a generic 更多好剧 cell (show_type 401) whose rows do not
// follow the series and change per call, and no session id.
func (c *Client) SeriesRecommend(ctx context.Context, seriesID string) (*model.Recommend, error) {
	if !validID(seriesID) {
		return nil, fmt.Errorf("invalid series ID")
	}
	p, err := c.ensureSearchGuest(ctx)
	if err != nil {
		return nil, err
	}
	ticket := c.clock().UnixMilli()
	session := randomUUID()
	rawQuery, headers := sign.SignGET(protocol.SignedPlanPairs(c.identity(p), seriesID, session), ticket, p.DeviceID, protocol.VersionName, nil)
	raw, _, err := c.doRaw(ctx, "GET", c.apiOrigin+protocol.PathPlan+"?"+rawQuery, anonymousHeaders(c, p, ticket, headers), nil)
	if err != nil {
		return nil, err
	}
	cells, err := protocol.ParsePlanCell(raw)
	if err != nil {
		return nil, err
	}

	for _, cell := range cells {
		if cell.ShowType == model.ShowTypeDetailRelatedVideo && len(cell.Items) > 0 {
			return &model.Recommend{CellTitle: cell.CellName, SessionID: cell.SessionID, Items: cell.Items}, nil
		}
	}
	// The anonymous surface answers with its own cell type (observed: 401
	// "更多好剧") rather than 160, so any populated cell is a usable answer.
	for _, cell := range cells {
		if len(cell.Items) > 0 {
			return &model.Recommend{CellTitle: cell.CellName, SessionID: cell.SessionID, Items: cell.Items}, nil
		}
	}
	return nil, ErrContentUnavailable
}

// Categories loads a category's frame: its 全部/男生/女生 tabs and the default
// tab. The live service sends no rows here (observed 2026-10-24); the list
// and its filters come from CategoryLanding.
//
// Path and request class come from the APK: GetNewCategoryFrontPageRequest is
// sent to /reading/bookapi/new_category/front/v:version/.
func (c *Client) Categories(ctx context.Context, categoryID string) (*model.CategoryPage, error) {
	if strings.TrimSpace(categoryID) == "" {
		return nil, fmt.Errorf("missing category ID")
	}
	const offset = 0
	p, err := c.ensureSearchGuest(ctx)
	if err != nil {
		return nil, err
	}
	ticket := c.clock().UnixMilli()
	rawQuery, headers := sign.SignGET(protocol.SignedCategoryPairs(c.identity(p), categoryID, offset, 20), ticket, p.DeviceID, protocol.VersionName, nil)
	raw, _, err := c.doRaw(ctx, "GET", c.apiOrigin+protocol.PathCategory+"?"+rawQuery, anonymousHeaders(c, p, ticket, headers), nil)
	if err != nil {
		return nil, err
	}
	return protocol.ParseCategoryFront(raw)
}

// LandingOptions selects a category landing page.
//
// Tab is a tab value from Categories: 8 (全部, the default), 1 (男生), or 0
// (女生). Filters maps a filter group key from CategoryPage.Filters
// (book_status, sort_by, sub_category, word_type) to one of its option
// values. Cursor is NextOffset from the previous page.
type LandingOptions struct {
	Tab     string
	Filters map[string]string
	Cursor  string
}

// CategoryLanding loads one page of a category's landing list with its filter
// panel. The rows are books (novels), not short dramas.
func (c *Client) CategoryLanding(ctx context.Context, categoryID string, opts LandingOptions) (*model.CategoryPage, error) {
	if strings.TrimSpace(categoryID) == "" {
		return nil, fmt.Errorf("missing category ID")
	}
	gender := ""
	switch opts.Tab {
	case "", "8":
	case "1", "0":
		// The tab is sent as query_gender; tab_type has no effect (observed).
		gender = opts.Tab
	default:
		return nil, fmt.Errorf("unknown category tab %q; use 8, 1, or 0", opts.Tab)
	}
	for key := range opts.Filters {
		if !slices.ContainsFunc(protocol.LandingFilterParams, func(kv [2]string) bool { return kv[0] == key }) {
			return nil, fmt.Errorf("unknown category filter %q", key)
		}
	}
	offset, err := protocol.SearchOffset(opts.Cursor)
	if err != nil {
		return nil, err
	}
	p, err := c.ensureSearchGuest(ctx)
	if err != nil {
		return nil, err
	}
	ticket := c.clock().UnixMilli()
	rawQuery, headers := sign.SignGET(protocol.SignedCategoryLandingPairs(c.identity(p), categoryID, gender, opts.Filters, offset, 20), ticket, p.DeviceID, protocol.VersionName, nil)
	raw, _, err := c.doRaw(ctx, "GET", c.apiOrigin+protocol.PathCategoryLanding+"?"+rawQuery, anonymousHeaders(c, p, ticket, headers), nil)
	if err != nil {
		return nil, err
	}
	return protocol.ParseCategoryLanding(raw)
}

// Ranking loads one activity ranking page.
//
// The activity_api host is separate from the reading host and accepts a plain
// JSON POST without the signed device query.
//
// The activity_api family lives on DistributionOrigin (observed: the reading
// host answers 404). The shipped client hard-codes the 星粉榜 activity and list ids;
// pass empty strings for those defaults.
func (c *Client) Ranking(ctx context.Context, activityID, rankListID string) (*model.Ranking, error) {
	if activityID == "" {
		activityID = protocol.DefaultRankActivity
	}
	if rankListID == "" {
		rankListID = protocol.DefaultRankList
	}
	body, err := protocol.RankActivityBody(activityID, rankListID, 0, 50)
	if err != nil {
		return nil, err
	}
	// The activity host takes a plain JSON POST: adding the signed device query
	// makes it answer HTTP 400 (observed both ways).
	headers := map[string]string{"Content-Type": "application/json", "User-Agent": protocol.UserAgent(c.device)}
	raw, _, err := c.doRaw(ctx, "POST", c.distOrigin+protocol.PathRankActivity, headers, body)
	if err != nil {
		return nil, err
	}
	return protocol.ParseRanking(raw)
}

// RankRows loads one page of a bookmall ranking cell (the 经典 channel's
// ranklist entry). The rows are books, not short dramas. The request asks for
// a fresh batch (change_type=1), so every call answers a different set
// (observed 2026-10-24: three calls, three different 1st places) and Rank is
// only the row's position on this page, not a standing on a stable list.
func (c *Client) RankRows(ctx context.Context, cellID string, offset int) (*model.Ranking, error) {
	if cellID == "" {
		cellID = protocol.DefaultRankCell
	}
	if offset < 0 {
		offset = 0
	}
	p, err := c.ensureSearchGuest(ctx)
	if err != nil {
		return nil, err
	}
	ticket := c.clock().UnixMilli()
	rawQuery, headers := sign.SignGET(protocol.SignedRankCellPairs(c.identity(p), cellID, offset, 20), ticket, p.DeviceID, protocol.VersionName, nil)
	raw, _, err := c.doRaw(ctx, "GET", c.apiOrigin+protocol.PathRankCells+"?"+rawQuery, anonymousHeaders(c, p, ticket, headers), nil)
	if err != nil {
		return nil, err
	}
	ranking, err := protocol.ParseRankRows(raw, offset)
	if err != nil {
		return nil, err
	}
	// An empty first page means the cell is gone; an empty later page is
	// only the end of the list, as with the other paged reads.
	if len(ranking.Items) == 0 && !ranking.HasMore && offset == 0 {
		return nil, ErrContentUnavailable
	}
	return ranking, nil
}

// Seasons lists the seasons of a multi-season title.
//
// The detail page's 系列剧 row lists every season, upcoming ones included. A
// title without other seasons gets 101116 there, and when the row cannot be
// read this falls back to the album endpoint (which refuses anonymous reads
// with 101001 on every sampled album) and then to search: search the base
// title, read the candidates' details, and keep those that share the
// collection id (or the same base title when the service did not publish
// one). Results are ordered by season index.
func (c *Client) Seasons(ctx context.Context, seriesID string) (*model.Album, error) {
	series, err := c.GetSeries(ctx, seriesID)
	if err != nil {
		return nil, err
	}
	if seasons, err := c.seriesSeasons(ctx, series.ID); err == nil && len(seasons) > 0 {
		for i := range seasons {
			if seasons[i].SeriesID == series.ID {
				seasons[i].Status = series.Status
			}
		}
		sortSeasons(seasons)
		return &model.Album{
			ID: series.RelatedAlbumID, Title: seasonBaseTitle(series.Title),
			SeasonCount: len(seasons), Seasons: seasons,
		}, nil
	} else if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if series.RelatedAlbumID != "" {
		if album, err := c.GetAlbum(ctx, series.RelatedAlbumID, AlbumOptions{}); err == nil && len(album.Seasons) > 0 {
			return album, nil
		}
	}
	base := seasonBaseTitle(series.Title)
	out := &model.Album{ID: series.RelatedAlbumID, Title: base}
	if !series.IsMultiSeason && series.RelatedAlbumID == "" {
		// A single-season title only shares its name with unrelated entries;
		// searching would turn look-alikes into fake seasons.
		out.Seasons = []model.Season{{
			SeriesID: series.ID, Title: series.Title, Index: 1,
			CoverURL: series.CoverURL, EpisodeCount: series.EpisodeCount, Status: series.Status,
		}}
		out.SeasonCount = 1
		return out, nil
	}
	out.Seasons = append(out.Seasons, model.Season{
		SeriesID: series.ID, Title: series.Title, Index: series.SeasonIndex,
		CoverURL: series.CoverURL, EpisodeCount: series.EpisodeCount, Status: series.Status,
	})
	// A long exact title only surfaces the queried season, so the sibling
	// lookup searches a short prefix and then keeps rows whose base matches.
	query := base
	if runes := []rune(base); len(runes) > 6 {
		query = string(runes[:6])
	}
	page, err := c.Search(ctx, query, SearchOptions{PageSize: 20})
	if err != nil && !errors.Is(err, ErrAnonymousLimited) {
		return out, err
	}
	// Up to ten candidates whose base title matches are read in one batch.
	var candidates []string
	if page != nil {
		for _, item := range page.Items {
			if item.ID == series.ID || len(candidates) >= seriesBatchSize {
				continue
			}
			candBase := seasonBaseTitle(item.Title)
			if candBase != base && !strings.HasPrefix(candBase, base) && !strings.HasPrefix(base, candBase) {
				continue
			}
			candidates = append(candidates, item.ID)
		}
	}
	if len(candidates) > 0 {
		details, err := c.GetSeriesBatch(ctx, candidates)
		if err != nil && ctx.Err() != nil {
			return nil, ctx.Err()
		}
		for _, d := range details {
			detail := d.Series
			if series.RelatedAlbumID != "" && detail.RelatedAlbumID != series.RelatedAlbumID {
				continue
			}
			out.Seasons = append(out.Seasons, model.Season{
				SeriesID: detail.ID, Title: detail.Title, Index: detail.SeasonIndex,
				CoverURL: detail.CoverURL, EpisodeCount: detail.EpisodeCount, Status: detail.Status,
			})
		}
	}
	sortSeasons(out.Seasons)
	out.SeasonCount = len(out.Seasons)
	return out, nil
}

// seriesSeasons reads the detail page's 系列剧 row: the plan feed with
// from=single_col_same_ip_series.
func (c *Client) seriesSeasons(ctx context.Context, seriesID string) ([]model.Season, error) {
	p, err := c.ensureSearchGuest(ctx)
	if err != nil {
		return nil, err
	}
	ticket := c.clock().UnixMilli()
	rawQuery, headers := sign.SignGET(protocol.SignedSeasonPairs(c.identity(p), seriesID, randomUUID()), ticket, p.DeviceID, protocol.VersionName, nil)
	raw, _, err := c.doRaw(ctx, "GET", c.apiOrigin+protocol.PathPlan+"?"+rawQuery, anonymousHeaders(c, p, ticket, headers), nil)
	if err != nil {
		return nil, err
	}
	return protocol.ParseSeasons(raw)
}

func sortSeasons(seasons []model.Season) {
	sort.SliceStable(seasons, func(i, j int) bool {
		if seasons[i].Index != seasons[j].Index {
			return seasons[i].Index < seasons[j].Index
		}
		return seasons[i].SeriesID < seasons[j].SeriesID
	})
}

// seasonSuffix matches the trailing season marker of a title.
var seasonSuffix = regexp.MustCompile(`(?s)\s*(第\s*[0-9一二三四五六七八九十百]+\s*[季部期]|[0-9]+)\s*$`)

// seasonBaseTitle strips a trailing season marker ("…第二季", "…3") so titles of
// sibling seasons share one searchable base.
func seasonBaseTitle(title string) string {
	t := strings.TrimSpace(title)
	if t == "" {
		return t
	}
	if loc := seasonSuffix.FindStringIndex(t); loc != nil && loc[0] > 0 {
		t = strings.TrimSpace(t[:loc[0]])
	}
	return t
}

// RankGroups lists the ranking channels of the 经典 bookmall tab.
//
// The tab serves one ranking cell whose rank_with_category_data names every
// ranking; the ids are the rank_algo values the cell/change call expects.
func (c *Client) RankGroups(ctx context.Context) ([]model.RankList, error) {
	channel, err := protocol.ResolveDiscoverChannel("经典")
	if err != nil {
		return nil, err
	}
	p, err := c.ensureSearchGuest(ctx)
	if err != nil {
		return nil, err
	}
	id := c.identity(p)
	ticket := c.clock().UnixMilli()
	rawQuery, headers := sign.SignGET(protocol.SignedDiscoverPairsIn(id, channel.RequestTab, channel.BottomTab), ticket, p.DeviceID, protocol.VersionName, nil)
	raw, _, err := c.doRaw(ctx, "GET", c.apiOrigin+protocol.PathDiscover+"?"+rawQuery, anonymousHeaders(c, p, ticket, headers), nil)
	if err != nil {
		return nil, err
	}
	groups := protocol.ParseRankGroups(raw)
	if len(groups) == 0 {
		return nil, ErrContentUnavailable
	}
	return groups, nil
}

// SearchCues loads the search box's rotating hints.
//
// Path and request fields come from the shipped client:
// GET /reading/bookapi/search/cue/v:version/ with cue_type, limit,
// search_source and tab_name. The response is not hot-search data: it carries
// placeholder hints such as "书名" / "主角" / "作者" / "书籍类型".
func (c *Client) SearchCues(ctx context.Context) ([]string, error) {
	p, err := c.ensureSearchGuest(ctx)
	if err != nil {
		return nil, err
	}
	ticket := c.clock().UnixMilli()
	rawQuery, headers := sign.SignGET(protocol.SignedCuePairs(c.identity(p), 1, 20), ticket, p.DeviceID, protocol.VersionName, nil)
	raw, _, err := c.doRaw(ctx, "GET", c.apiOrigin+protocol.PathCue+"?"+rawQuery, anonymousHeaders(c, p, ticket, headers), nil)
	if err != nil {
		return nil, err
	}
	return protocol.ParseCue(raw)
}

// HotSearch loads the search page before a query: 猜你想搜 and the hot lists
// (短剧热搜榜, 漫剧热搜榜, 热点话题榜, and sometimes 短剧新剧榜).
//
// The page is the plan feed with from=search_input_page; its rows arrive in a
// Lynx cell's lynx_data (captured 2026-10-24). Each list holds the few rows
// the page shows.
func (c *Client) HotSearch(ctx context.Context) (*model.HotSearch, error) {
	p, err := c.ensureSearchGuest(ctx)
	if err != nil {
		return nil, err
	}
	ticket := c.clock().UnixMilli()
	rawQuery, headers := sign.SignGET(protocol.SignedHotSearchPairs(c.identity(p)), ticket, p.DeviceID, protocol.VersionName, nil)
	raw, _, err := c.doRaw(ctx, "GET", c.apiOrigin+protocol.PathPlan+"?"+rawQuery, anonymousHeaders(c, p, ticket, headers), nil)
	if err != nil {
		return nil, err
	}
	hot, err := protocol.ParseHotSearch(raw)
	if err != nil {
		return nil, err
	}
	if len(hot.Boards) == 0 && len(hot.Guesses) == 0 {
		return nil, ErrContentUnavailable
	}
	return hot, nil
}

// NewReleaseQuery picks a day of the 上新 calendar: Date is YYYYMMDD (empty
// is today) and Cursor the previous page's Cursor, which carries its day.
type NewReleaseQuery struct {
	Date   string
	Cursor string
}

// NewReleases loads one page of the 上新 calendar: the titles out on a day
// and those announced for it, with the days the calendar offers.
//
// The page is /reading/user/subscribe/list/ with tab_type 5 and
// target_date (captured 2026-10-24); an anonymous device gets the public
// calendar, not a list of its own reservations.
func (c *Client) NewReleases(ctx context.Context, q NewReleaseQuery) (*model.NewReleasePage, error) {
	cur, err := protocol.DecodeShortCursor(q.Cursor)
	if err != nil {
		return nil, err
	}
	date := q.Date
	switch {
	case date != "" && !protocol.ValidDate(date):
		return nil, fmt.Errorf("date must be YYYYMMDD")
	case q.Cursor != "" && date != "" && date != cur.Date:
		return nil, fmt.Errorf("the cursor belongs to %s, not %s", cur.Date, date)
	case q.Cursor != "":
		date = cur.Date
	}
	p, err := c.ensureSearchGuest(ctx)
	if err != nil {
		return nil, err
	}
	ticket := c.clock().UnixMilli()
	rawQuery, headers := sign.SignGET(protocol.SignedNewReleasePairs(c.identity(p), date, cur), ticket, p.DeviceID, protocol.VersionName, nil)
	raw, _, err := c.doRaw(ctx, "GET", c.apiOrigin+protocol.PathSubscribeList+"?"+rawQuery, anonymousHeaders(c, p, ticket, headers), nil)
	if err != nil {
		return nil, err
	}
	res, err := protocol.ParseNewReleases(raw)
	if err != nil {
		return nil, err
	}
	page := res.NewReleasePage
	if page.Date == "" {
		page.Date = date
	}
	if res.HasMore && len(res.Items) > 0 && res.NextOffset > cur.Offset {
		next := protocol.ShortCursor{Offset: res.NextOffset, Session: res.SessionID, Date: page.Date}
		if next.Session == "" {
			next.Session = cur.Session
		}
		if page.Cursor, err = protocol.EncodeShortCursor(next); err != nil {
			return nil, err
		}
	}
	return &page, nil
}

// Person loads an actor's or a character's page. userID is the account id
// (RankItem.UserID on the actor ranking), not the actor id.
func (c *Client) Person(ctx context.Context, userID string) (*model.Person, error) {
	if !validID(userID) {
		return nil, fmt.Errorf("invalid user ID")
	}
	p, err := c.ensureSearchGuest(ctx)
	if err != nil {
		return nil, err
	}
	ticket := c.clock().UnixMilli()
	rawQuery, headers := sign.SignGET(protocol.SignedProfilePairs(c.identity(p), userID), ticket, p.DeviceID, protocol.VersionName, nil)
	raw, _, err := c.doRaw(ctx, "GET", c.apiOrigin+protocol.PathProfile+"?"+rawQuery, anonymousHeaders(c, p, ticket, headers), nil)
	if err != nil {
		return nil, err
	}
	person, err := protocol.ParseProfile(raw)
	if err != nil {
		return nil, err
	}
	if person.Name == "" {
		return nil, ErrContentUnavailable
	}
	return person, nil
}

// PersonWorks loads one page of the series a person appears in (作品 ›
// 剧作 on their page). cursor is the previous page's Cursor.
func (c *Client) PersonWorks(ctx context.Context, userID, cursor string) (*model.WorkPage, error) {
	if !validID(userID) {
		return nil, fmt.Errorf("invalid user ID")
	}
	cur, err := protocol.DecodeWorksCursor(cursor)
	if err != nil {
		return nil, err
	}
	if cursor != "" && cur.UserID != userID {
		return nil, fmt.Errorf("the cursor belongs to another person")
	}
	body, err := protocol.UserWorksBody(userID, cur.Next)
	if err != nil {
		return nil, err
	}
	p, err := c.ensureSearchGuest(ctx)
	if err != nil {
		return nil, err
	}
	path := fmt.Sprintf(protocol.PathUserWorks, userID)
	ticket := c.clock().UnixMilli()
	rawQuery, headers := sign.SignPOST(path, protocol.DeviceQueryPairs(c.identity(p)), ticket, p.DeviceID, protocol.VersionName, body, nil)
	raw, _, err := c.doRaw(ctx, "POST", c.apiOrigin+path+"?"+rawQuery, anonymousHeaders(c, p, ticket, headers), body)
	if err != nil {
		return nil, err
	}
	res, err := protocol.ParseUserWorks(raw)
	if err != nil {
		return nil, err
	}
	page := &model.WorkPage{Items: res.Items, HasMore: res.HasMore}
	if res.HasMore && res.Cursor != "" && len(res.Items) > 0 {
		if page.Cursor, err = protocol.EncodeWorksCursor(protocol.WorksCursor{UserID: userID, Next: res.Cursor}); err != nil {
			return nil, err
		}
	}
	return page, nil
}

// ShortFilterQuery selects short dramas on the filter page (筛选).
//
// Selections maps a FilterRow.Key to the chosen option Values; a missing row
// is 全部. Cursor is the previous page's Cursor.
type ShortFilterQuery struct {
	Selections map[string][]string
	Cursor     string
}

// ShortFilterPanel loads the selector rows of the short-drama filter page for
// a genre: short_play (真人剧), comic_series (漫剧), ai_series (AI剧), or empty
// for all. The rows and their options differ by genre.
func (c *Client) ShortFilterPanel(ctx context.Context, genre string) ([]model.FilterRow, error) {
	scene, err := shortFilterScene(genre)
	if err != nil {
		return nil, err
	}
	body, err := protocol.ShortFilterBody(scene, nil, true, protocol.ShortCursor{})
	if err != nil {
		return nil, err
	}
	res, err := c.shortFilter(ctx, body)
	if err != nil {
		return nil, err
	}
	if len(res.Rows) == 0 {
		return nil, ErrContentUnavailable
	}
	return res.Rows, nil
}

// ShortFilter loads one page of short dramas matching q.
//
// The service lives on DistributionOrigin. Each page's Cursor carries the
// series already shown; the service skips them, so pages do not repeat
// (observed 2026-10-24).
func (c *Client) ShortFilter(ctx context.Context, q ShortFilterQuery) (*model.ShortFilterPage, error) {
	for key, values := range q.Selections {
		if !slices.Contains(protocol.ShortFilterKeys, key) {
			return nil, fmt.Errorf("unknown short filter row %q", key)
		}
		for _, v := range values {
			if strings.TrimSpace(v) == "" {
				return nil, fmt.Errorf("empty value for short filter row %q", key)
			}
		}
	}
	cur, err := protocol.DecodeShortCursor(q.Cursor)
	if err != nil {
		return nil, err
	}
	// A cursor resumes its own query; a different query cannot continue it.
	selections := q.Selections
	if q.Cursor != "" {
		if len(selections) == 0 {
			selections = cur.Selections
		} else if !sameSelections(selections, cur.Selections) {
			return nil, fmt.Errorf("the cursor continues a different filter; drop the selections or the cursor")
		}
	}
	genre := ""
	if g := selections["genre"]; len(g) == 1 {
		genre = g[0]
	}
	scene, err := shortFilterScene(genre)
	if err != nil {
		return nil, err
	}
	body, err := protocol.ShortFilterBody(scene, selections, false, cur)
	if err != nil {
		return nil, err
	}
	res, err := c.shortFilter(ctx, body)
	if err != nil {
		return nil, err
	}
	if len(res.Items) == 0 && cur.Offset == 0 {
		return nil, ErrContentUnavailable
	}
	page := &model.ShortFilterPage{Items: res.Items, HasMore: res.HasMore}
	ids := make([]string, 0, len(res.Items))
	for _, it := range res.Items {
		ids = append(ids, it.ID)
	}
	cur.Selections = selections
	page.Cursor, err = nextShortCursor(cur, res.HasMore, res.NextOffset, res.SessionID, ids)
	return page, err
}

// sameSelections compares two filter queries, ignoring empty rows.
func sameSelections(a, b map[string][]string) bool {
	norm := func(m map[string][]string) map[string]string {
		out := map[string]string{}
		for k, v := range m {
			if len(v) > 0 {
				out[k] = strings.Join(v, ",")
			}
		}
		return out
	}
	na, nb := norm(a), norm(b)
	if len(na) != len(nb) {
		return false
	}
	for k, v := range na {
		if nb[k] != v {
			return false
		}
	}
	return true
}

// shortFilterScene maps a genre to req_scene. 漫剧 and AI剧 have scenes of
// their own; 真人剧 is the default scene with the genre selected (a
// short_play scene is ignored by the service, observed).
func shortFilterScene(genre string) (string, error) {
	switch genre {
	case "", "short_play":
		return "default", nil
	case "comic_series", "ai_series":
		return genre, nil
	}
	return "", fmt.Errorf("unknown short-drama genre %q; use short_play, comic_series, or ai_series", genre)
}

func (c *Client) shortFilter(ctx context.Context, body []byte) (*protocol.ShortFilterResult, error) {
	p, err := c.ensureSearchGuest(ctx)
	if err != nil {
		return nil, err
	}
	ticket := c.clock().UnixMilli()
	rawQuery, headers := sign.SignPOST(protocol.PathShortFilter, protocol.DeviceQueryPairs(c.identity(p)), ticket, p.DeviceID, protocol.VersionName, body, nil)
	raw, _, err := c.doRaw(ctx, "POST", c.distOrigin+protocol.PathShortFilter+"?"+rawQuery, anonymousHeaders(c, p, ticket, headers), body)
	if err != nil {
		return nil, err
	}
	return protocol.ParseShortFilter(raw)
}

// ShortRankQuery selects a short-drama ranking. Content is a RankContent ID
// (all, human, comic_series_rank, ai_playlet, series_album; empty is all) and
// Board one of its RankBoard IDs (empty is the service's default for the
// content). Cursor is the previous page's Cursor.
type ShortRankQuery struct {
	Content string
	Board   string
	Cursor  string
}

// ShortRank loads one page of a short-drama ranking (排行榜).
//
// The ranking page is cell/change on one bookmall cell: every page also
// carries the page's catalog of contents and their boards. Rows come in page
// order; RankItem.Position is that order, and a Promoted row is one the
// service inserts (平台推荐). Each Cursor carries the series already shown,
// which the service skips (observed 2026-10-24).
func (c *Client) ShortRank(ctx context.Context, q ShortRankQuery) (*model.ShortRanking, error) {
	cur, err := protocol.DecodeShortCursor(q.Cursor)
	if err != nil {
		return nil, err
	}
	content, board := strings.TrimSpace(q.Content), strings.TrimSpace(q.Board)
	if q.Cursor != "" {
		// A cursor resumes its own list; a different list cannot continue it.
		if (content != "" && content != cur.Content) || (board != "" && board != cur.Board) {
			return nil, fmt.Errorf("the cursor continues a different ranking; drop --content/--board or the cursor")
		}
		content, board = cur.Content, cur.Board
	}
	if content == "" {
		content = "all"
	}
	p, err := c.ensureSearchGuest(ctx)
	if err != nil {
		return nil, err
	}
	ticket := c.clock().UnixMilli()
	rawQuery, headers := sign.SignGET(protocol.SignedShortRankPairs(c.identity(p), content, board, cur), ticket, p.DeviceID, protocol.VersionName, nil)
	raw, _, err := c.doRaw(ctx, "GET", c.apiOrigin+protocol.PathRankCells+"?"+rawQuery, anonymousHeaders(c, p, ticket, headers), nil)
	if err != nil {
		return nil, err
	}
	res, err := protocol.ParseShortRank(raw)
	if err != nil {
		return nil, err
	}
	if len(res.Items) == 0 && cur.Offset == 0 {
		return nil, ErrContentUnavailable
	}
	out := &model.ShortRanking{Contents: res.Contents, Version: res.Version, Items: res.Items, HasMore: res.HasMore}
	// The catalog marks what the service answered, which is the default
	// board when none was asked for.
	for _, c := range res.Contents {
		if !c.Selected {
			continue
		}
		out.Content = c.ID
		for _, b := range c.Boards {
			if b.Selected {
				out.Board = b.ID
			}
		}
	}
	if out.Content == "" {
		out.Content, out.Board = content, board
	}
	// Positions continue across pages.
	for i := range out.Items {
		out.Items[i].Position += len(cur.Seen)
	}
	ids := make([]string, 0, len(res.Items))
	for _, it := range res.Items {
		id := it.SeriesID
		if id == "" {
			id = it.ActorID
		}
		ids = append(ids, id)
	}
	// Later pages ask for the list this page answered.
	cur.Content, cur.Board = out.Content, out.Board
	out.Cursor, err = nextShortCursor(cur, res.HasMore, res.NextOffset, res.SessionID, ids)
	return out, err
}

// nextShortCursor extends cur with one page's series, or returns "" at the
// end of the list.
func nextShortCursor(cur protocol.ShortCursor, more bool, offset int, session string, ids []string) (string, error) {
	if !more || len(ids) == 0 || offset <= cur.Offset {
		return "", nil
	}
	next := cur
	next.Offset, next.Seen, next.Session = offset, append(append([]string{}, cur.Seen...), ids...), session
	return protocol.EncodeShortCursor(next)
}
