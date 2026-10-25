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

// Page 是一页匿名结果。
type Page[T any] struct {
	Items   []T    `json:"items"`
	Cursor  string `json:"cursor,omitempty"`
	HasMore bool   `json:"has_more"`
}

// DiscoverOptions 选择一个发现页。Channel 是书城频道名，空频道即推荐。
// Cursor 是上一页给出的令牌。
type DiscoverOptions struct {
	Cursor  string
	Channel string
}

// SearchOptions 选择一个搜索页。Tab 取值为综合、剧集、视频、讨论、小说、
// 用户或听书，空 tab 即综合。Cursor 是下一个偏移量。
type SearchOptions struct {
	Cursor   string
	PageSize int
	Tab      string
	// Filters 把综合 tab 收窄到 SearchFilter* 值；选多个时互相放宽。
	// 游标不携带这些筛选，所以要连同游标传入相同的筛选。
	Filters []string
}

// 综合 tab 的搜索筛选项，与 app 筛选面板在 selected_items
// 中发送的值一致（2026-10-24 实测）。两个漫剧类筛选都返回 content_type 1004，
// 由服务端区分二者。
const (
	SearchFilterShortPlay = "content_type_short_play"       // 真人剧
	SearchFilterComic     = "content_type_dynamic_comic"    // 漫剧
	SearchFilterAIComic   = "content_type_ai_dynamic_comic" // AI剧
	SearchFilterFilm      = "content_type_long_video"       // 影视剧
	SearchFilterUnwatched = "search_range_unread"           // 未看过
)

// EpisodeOptions 选择一个分集页。游标不在第 1 阶段的范围内。
type EpisodeOptions struct {
	Cursor   string
	PageSize int
}

// Discover 加载一个匿名的书城频道。首页由 tab 预加载和第一个不限量的
// cell/change 页组成。带游标时只加载下一个 cell/change 页。
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
	// 空的 change 页不会给出下一个偏移量。如有预加载行，仍然返回。
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

// Search 加载 query 的匿名短剧搜索结果。
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

// searchFilters 把 filters 拼成 selected_items 的值。
// 这些筛选只在综合 tab 上见过，所以其他 tab 会拒绝它们，而不是忽略。
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

// GetSeries 匿名加载一部剧。
func (c *Client) GetSeries(ctx context.Context, seriesID string) (*model.Series, error) {
	series, _, err := c.seriesDetail(ctx, seriesID)
	return series, err
}

// seriesBatchSize 是一次批量请求查询的剧数。服务端接受 20，
// 拒绝 30 并返回 100001（2026-10-24 实测）；取 10 可让响应保持在约 1 MB 以内。
const seriesBatchSize = 10

// GetSeriesBatch 加载多部剧及其各集，按请求顺序返回。
// 服务端不认识的 ID 会被略去；重复的 ID 只查询一次。
//
// 播放器的 multi_video_detail 读取接口为每部剧返回与 GetSeries 相同的结构；
// 更长的列表按 seriesBatchSize 分批请求。
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

// ListEpisodes 加载匿名的分集列表。集数从 1 开始。
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

// Pager 沿着游标翻页，并去掉之前的页已返回过的行。
// 服务端会跨页重复标题（在小说、漫剧和真人剧上实测），
// 所以跟随游标的调用方需要这个过滤。
type Pager struct {
	fetch func(ctx context.Context, cursor string) (*Page[model.SeriesSummary], error)
	seen  map[string]struct{}
	next  string
	done  bool
	// Pages 统计目前已拉取的页数。
	Pages int
}

// NewPager 基于任意翻页函数构建一个去重的 pager，让只持有接口的调用方（CLI、
// gomobile）也遵循同样的规则。
func NewPager(cursor string, fetch func(ctx context.Context, cursor string) (*Page[model.SeriesSummary], error)) *Pager {
	return &Pager{seen: map[string]struct{}{}, next: cursor, fetch: fetch}
}

// DiscoverPager 从 opts.Cursor 开始遍历一个书城频道。
func (c *Client) DiscoverPager(opts DiscoverOptions) *Pager {
	return NewPager(opts.Cursor, func(ctx context.Context, cursor string) (*Page[model.SeriesSummary], error) {
		o := opts
		o.Cursor = cursor
		return c.Discover(ctx, o)
	})
}

// SearchPager 从 opts.Cursor 开始遍历一个搜索 tab。
func (c *Client) SearchPager(query string, opts SearchOptions) *Pager {
	return NewPager(opts.Cursor, func(ctx context.Context, cursor string) (*Page[model.SeriesSummary], error) {
		o := opts
		o.Cursor = cursor
		return c.Search(ctx, query, o)
	})
}

// Next 加载下一页，返回之前没见过的行。more 表示是否还有下一页；
// 只含已知行的页会被跳过，调用方无需特殊处理。
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

// Seen 返回 pager 已返回的不同行数。
func (p *Pager) Seen() int { return len(p.seen) }

// AlbumOptions 留作选择合集界面之用。
type AlbumOptions struct{}

// GetAlbum 加载一个合集（多季合集）及其各季。
//
// 路径和请求类来自对 APK 的观察：GetVideoAlbumDetailRequest 携带 albumId、
// seriesIds 和 needVideoDetailInfo；响应列出该合集的各季。
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

// Suggest 返回某个前缀的搜索联想。
//
// 路径取自随 app 发布的 sug 页面：
// GET /reading/bookapi/search/suggest/v:version/。响应结构按宽松方式解析，
// 因为不同 app 版本的列表名不同。
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

// SeriesRecommend 加载一部剧的相关视频行。
//
// 该界面没有专门的 RPC：客户端复用通用的 plan feed（GET
// /reading/bookapi/plan/v:version/），带上 from=select_panel_recommend 和
// scene=10，再取 show_type 为 160 的那个 cell。匿名时，
// 服务端返回一个通用的更多好剧 cell（show_type 401），其中的行不随该剧而定，
// 每次调用都会变化，也不带 session id。
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
	// 匿名界面返回它自己的 cell 类型（实测：401 “更多好剧”），而不是 160，
	// 所以任何有内容的 cell 都是可用的结果。
	for _, cell := range cells {
		if len(cell.Items) > 0 {
			return &model.Recommend{CellTitle: cell.CellName, SessionID: cell.SessionID, Items: cell.Items}, nil
		}
	}
	return nil, ErrContentUnavailable
}

// Categories 加载一个分类的框架：全部/男生/女生 tab 和默认 tab。
// 线上服务在这里不返回行（2026-10-24 实测）；列表及其筛选来自 CategoryLanding。
//
// 路径和请求类取自 APK：GetNewCategoryFrontPageRequest 发往
// /reading/bookapi/new_category/front/v:version/。
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

// LandingOptions 选择一个分类落地页。
//
// Tab 是 Categories 给出的 tab 值：8（全部，默认）、1（男生）或 0（女生）。
// Filters 把 CategoryPage.Filters 中的筛选组键（book_status、sort_by、
// sub_category、word_type）映射到该组的某个选项值。
// Cursor 是上一页的 NextOffset。
type LandingOptions struct {
	Tab     string
	Filters map[string]string
	Cursor  string
}

// CategoryLanding 加载分类落地列表的一页及其筛选面板。这些行是书（小说），
// 不是短剧。
func (c *Client) CategoryLanding(ctx context.Context, categoryID string, opts LandingOptions) (*model.CategoryPage, error) {
	if strings.TrimSpace(categoryID) == "" {
		return nil, fmt.Errorf("missing category ID")
	}
	gender := ""
	switch opts.Tab {
	case "", "8":
	case "1", "0":
		// tab 以 query_gender 发送；tab_type 不起作用（实测）。
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

// Ranking 加载一个活动榜单页。
//
// activity_api 主机独立于 reading 主机，
// 接受不带签名设备查询参数的普通 JSON POST。
//
// activity_api 系列接口位于 DistributionOrigin（实测：reading 主机返回 404）。
// 随 app 发布的客户端写死了星粉榜的活动 id 和榜单 id；
// 传空字符串即使用这些默认值。
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
	// 活动主机接受普通 JSON POST：加上签名设备查询参数会让它返回 HTTP
	// 400（两种方式都实测过）。
	headers := map[string]string{"Content-Type": "application/json", "User-Agent": protocol.UserAgent(c.device)}
	raw, _, err := c.doRaw(ctx, "POST", c.distOrigin+protocol.PathRankActivity, headers, body)
	if err != nil {
		return nil, err
	}
	return protocol.ParseRanking(raw)
}

// RankRows 加载书城榜单 cell 的一页（经典频道的榜单入口）。这些行是书，
// 不是短剧。请求要的是新的一批（change_type=1），
// 所以每次调用返回的集合都不同
// （2026-10-24 实测：调用三次，第 1 名三次都不同）。Rank 只是该行在本页的位置，
// 而不是稳定榜单上的名次。
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
	// 首页为空说明该 cell 已不存在；后续页为空只表示列表到头了，
	// 与其他分页读取一致。
	if len(ranking.Items) == 0 && !ranking.HasMore && offset == 0 {
		return nil, ErrContentUnavailable
	}
	return ranking, nil
}

// Seasons 列出多季作品的各季。
//
// 详情页的系列剧行列出所有季，包括即将上线的季。
// 没有其他季的作品在那里会得到 101116。读不到该行时，
// 本方法先退回合集接口（在所有抽样的合集上，它都以 101001 拒绝匿名读取），
// 再退回搜索：搜索基础标题，读取候选项的详情，
// 保留合集 id 相同的那些（服务端没有发布合集 id 时，保留基础标题相同的那些）。
// 结果按季序号排序。
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
		// 单季作品只是与无关条目同名；去搜索会把相似条目误当成别的季。
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
	// 完整的长标题只会搜出被查询的那一季，
	// 所以查找同系列其他季时搜索一个短前缀，再保留基础标题匹配的行。
	query := base
	if runes := []rune(base); len(runes) > 6 {
		query = string(runes[:6])
	}
	page, err := c.Search(ctx, query, SearchOptions{PageSize: 20})
	if err != nil && !errors.Is(err, ErrAnonymousLimited) {
		return out, err
	}
	// 基础标题匹配的候选项最多取十个，一次批量读取。
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

// seriesSeasons 读取详情页的系列剧行，即带 from=single_col_same_ip_series 的
// plan feed。
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

// seasonSuffix 匹配标题末尾的季标记。
var seasonSuffix = regexp.MustCompile(`(?s)\s*(第\s*[0-9一二三四五六七八九十百]+\s*[季部期]|[0-9]+)\s*$`)

// seasonBaseTitle 去掉标题末尾的季标记（“…第二季”、“…3”），
// 让同系列各季的标题共享一个可搜索的基础标题。
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

// RankGroups 列出经典书城 tab 的各个榜单频道。
//
// 该 tab 提供一个榜单 cell，其 rank_with_category_data 列出所有榜单；
// 这些 id 就是 cell/change 调用所需的 rank_algo 值。
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

// SearchCues 加载搜索框轮播的提示词。
//
// 路径和请求字段取自随 app 发布的客户端：
// GET /reading/bookapi/search/cue/v:version/，带 cue_type、limit、
// search_source 和 tab_name。响应不是热搜数据，
// 而是“书名”/“主角”/“作者”/“书籍类型”这类占位提示。
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

// HotSearch 加载输入查询之前的搜索页：猜你想搜和各个热榜（短剧热搜榜、
// 漫剧热搜榜、热点话题榜，有时还有短剧新剧榜）。
//
// 该页是带 from=search_input_page 的 plan feed；
// 其中的行位于一个 Lynx cell 的 lynx_data 中（2026-10-24 抓包）。
// 每个榜单只含页面展示的那几行。
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

// NewReleaseQuery 选择上新日历中的一天：Date 格式为 YYYYMMDD（空即今天），
// Cursor 是上一页的 Cursor，它带有所在的日期。
type NewReleaseQuery struct {
	Date   string
	Cursor string
}

// NewReleases 加载上新日历的一页：某天上线的作品和预告在当天上线的作品，
// 以及日历提供的日期。
//
// 该页是 /reading/user/subscribe/list/，带 tab_type 5 和
// target_date（2026-10-24 抓包）；匿名设备拿到的是公开日历，
// 而不是它自己的预约列表。
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

// Person 加载一位演员或一个角色的主页。userID 是账号 id（演员榜上的
// RankItem.UserID），不是演员 id。
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

// PersonWorks 加载某人参演剧的一页（其主页上的作品 › 剧作）。
// cursor 是上一页的 Cursor。
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

// ShortFilterQuery 在筛选页（筛选）上挑选短剧。
//
// Selections 把 FilterRow.Key 映射到选中的选项 Values；缺省的行即全部。
// Cursor 是上一页的 Cursor。
type ShortFilterQuery struct {
	Selections map[string][]string
	Cursor     string
}

// ShortFilterPanel 加载某个类型的短剧筛选页选择行。
// 类型可为 short_play（真人剧）、comic_series（漫剧）、ai_series（AI剧），
// 空表示全部。不同类型的行和选项各不相同。
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

// ShortFilter 加载符合 q 的短剧的一页。
//
// 该服务位于 DistributionOrigin。每页的 Cursor 都带有已展示过的剧；
// 服务端会跳过它们，所以各页不会重复（2026-10-24 实测）。
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
	// 游标只能续接它自己的查询；不同的查询无法续接。
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

// sameSelections 比较两组筛选查询，忽略空行。
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

// shortFilterScene 把类型映射到 req_scene。漫剧和 AI剧有各自的 scene；
// 真人剧使用 default scene 并选中该类型
// （服务端会忽略 short_play scene，实测）。
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

// ShortRankQuery 选择一个短剧榜单。Content 是 RankContent ID（all、human、
// comic_series_rank、ai_playlet、series_album；空即 all），
// Board 是它的某个 RankBoard ID（空即服务端对该 content 的默认值）。
// Cursor 是上一页的 Cursor。
type ShortRankQuery struct {
	Content string
	Board   string
	Cursor  string
}

// ShortRank 加载短剧榜单（排行榜）的一页。
//
// 榜单页就是对某个书城 cell 调用 cell/change：
// 每页还带有该页的内容目录及各内容的榜单。行按页面顺序给出，
// RankItem.Position 就是这个顺序；Promoted 行是服务端插入的行（平台推荐）。
// 每个 Cursor 都带有已展示过的剧，服务端会跳过它们（2026-10-24 实测）。
func (c *Client) ShortRank(ctx context.Context, q ShortRankQuery) (*model.ShortRanking, error) {
	cur, err := protocol.DecodeShortCursor(q.Cursor)
	if err != nil {
		return nil, err
	}
	content, board := strings.TrimSpace(q.Content), strings.TrimSpace(q.Board)
	if q.Cursor != "" {
		// 游标只能续接它自己的榜单；不同的榜单无法续接。
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
	// 目录标出了服务端实际返回的内容；没有指定榜单时，这就是默认榜单。
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
	// 位置跨页连续编号。
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
	// 后续页请求本页返回的那个榜单。
	cur.Content, cur.Board = out.Content, out.Board
	out.Cursor, err = nextShortCursor(cur, res.HasMore, res.NextOffset, res.SessionID, ids)
	return out, err
}

// nextShortCursor 用一页的剧扩展 cur；到列表末尾时返回 ""。
func nextShortCursor(cur protocol.ShortCursor, more bool, offset int, session string, ids []string) (string, error) {
	if !more || len(ids) == 0 || offset <= cur.Offset {
		return "", nil
	}
	next := cur
	next.Offset, next.Seen, next.Session = offset, append(append([]string{}, cur.Seen...), ids...), session
	return protocol.EncodeShortCursor(next)
}
