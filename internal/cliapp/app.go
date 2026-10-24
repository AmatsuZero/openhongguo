package cliapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/beilunyang/openhongguo/internal/history"
	"github.com/beilunyang/openhongguo/internal/media"
	"github.com/beilunyang/openhongguo/sdk"
	"github.com/beilunyang/openhongguo/sdk/model"
)

const (
	ExitOK          = 0
	ExitUsage       = 2
	ExitUnavailable = 3
	ExitFailure     = 1
)

// Service is the anonymous catalog and playback surface the CLI calls.
type Service interface {
	Discover(context.Context, sdk.DiscoverOptions) (*sdk.Page[model.SeriesSummary], error)
	Search(context.Context, string, sdk.SearchOptions) (*sdk.Page[model.SeriesSummary], error)
	GetSeries(context.Context, string) (*model.Series, error)
	GetSeriesBatch(context.Context, []string) ([]model.SeriesDetail, error)
	GetAlbum(context.Context, string, sdk.AlbumOptions) (*model.Album, error)
	Seasons(context.Context, string) (*model.Album, error)
	Suggest(context.Context, string) ([]model.Suggestion, error)
	SearchCues(context.Context) ([]string, error)
	HotSearch(context.Context) (*model.HotSearch, error)
	NewReleases(context.Context, sdk.NewReleaseQuery) (*model.NewReleasePage, error)
	Person(context.Context, string) (*model.Person, error)
	PersonWorks(context.Context, string, string) (*model.WorkPage, error)
	SeriesRecommend(context.Context, string) (*model.Recommend, error)
	ShortFilterPanel(context.Context, string) ([]model.FilterRow, error)
	ShortFilter(context.Context, sdk.ShortFilterQuery) (*model.ShortFilterPage, error)
	Ranking(context.Context, string, string) (*model.Ranking, error)
	RankRows(context.Context, string, int) (*model.Ranking, error)
	RankGroups(context.Context) ([]model.RankList, error)
	ShortRank(context.Context, sdk.ShortRankQuery) (*model.ShortRanking, error)
	Categories(context.Context, string) (*model.CategoryPage, error)
	CategoryLanding(context.Context, string, sdk.LandingOptions) (*model.CategoryPage, error)
	ListEpisodes(context.Context, string, sdk.EpisodeOptions) (*sdk.Page[model.Episode], error)
	ResolvePlayback(context.Context, string, sdk.PlaybackOptions) (*model.Playback, error)
}

// Media plays a resolved stream with an external player, saves it, or
// rewrites a protected stream into a clear file with a caller-supplied key.
type Media interface {
	Open(ctx context.Context, url, player string) error
	Save(ctx context.Context, url, path string) (int64, error)
	Decrypt(ctx context.Context, url, audio, key, path string, limitSeconds int) (int64, error)
	// Play plays a stream without writing a file, decrypting it with
	// req.Key when set, and reports where playback stopped and whether the
	// viewer asked for another episode. It returns media.ErrStopped when the
	// player is closed before the end and media.ErrNeedsFile when the player
	// can only open a decrypted file.
	Play(ctx context.Context, req media.Request) (media.Result, error)
}

// ProfileStore is the local anonymous profile.
type ProfileStore interface {
	Load(context.Context) (*sdk.DeviceProfile, error)
	Reset(context.Context) error
}

// WatchHistory remembers the last episode played per series.
type WatchHistory interface {
	Get(ctx context.Context, seriesID string) (history.Entry, bool, error)
	Record(ctx context.Context, e history.Entry) error
	List(ctx context.Context) ([]history.Entry, error)
	Clear(ctx context.Context) error
}

// DeviceResetter re-registers the anonymous device after the service rejects
// it (BaseResp 263175/263176).
type DeviceResetter interface {
	ResetDevice(context.Context) error
}

// Dependencies wires the CLI to the shipped SDK. History is optional; without
// it playback is not remembered and --resume is unavailable.
type Dependencies struct {
	Service  Service
	Profiles ProfileStore
	Media    Media
	History  WatchHistory
	// DeviceReset re-registers the anonymous device after a rejection; when
	// nil the CLI reports the failure instead of replaying.
	DeviceReset DeviceResetter
	// Stdin answers the --pick prompt.
	Stdin          io.Reader
	Stdout, Stderr io.Writer
}

// Run executes one anonymous CLI invocation.
func Run(ctx context.Context, args []string, d Dependencies) int {
	if d.Stdout == nil {
		d.Stdout = io.Discard
	}
	if d.Stderr == nil {
		d.Stderr = io.Discard
	}
	if err := rejectAccountFlags(args); err != nil {
		fmt.Fprintln(d.Stderr, "ohg:", err)
		return ExitUsage
	}
	err := dispatch(ctx, args, d)
	if errors.Is(err, sdk.ErrDeviceRejected) && d.DeviceReset != nil {
		err = recoverDevice(ctx, args, d, err)
	}
	if err == nil {
		return ExitOK
	}
	fmt.Fprintln(d.Stderr, "ohg:", strings.TrimPrefix(err.Error(), "openhongguo: "))
	if errors.Is(err, errUsage) {
		return ExitUsage
	}
	if errors.Is(err, sdk.ErrSignerUnavailable) || errors.Is(err, sdk.ErrDRMProtected) || errors.Is(err, sdk.ErrAuthenticationRequired) || errors.Is(err, sdk.ErrAnonymousLimited) {
		return ExitUnavailable
	}
	return ExitFailure
}

// recoverDevice registers a new anonymous device after the service rejected
// the current one. A command that is safe to run twice is replayed; playback
// is not, because it would start over from its first episode.
func recoverDevice(ctx context.Context, args []string, d Dependencies, err error) error {
	fmt.Fprintln(d.Stderr, "ohg: the service rejected this anonymous device; registering a new one")
	if resetErr := d.DeviceReset.ResetDevice(ctx); resetErr != nil {
		return fmt.Errorf("%w (registering a new device failed: %v)", err, resetErr)
	}
	if len(args) > 0 && replaySafe[args[0]] {
		return dispatch(ctx, args, d)
	}
	return fmt.Errorf("%w; a new device is registered, run `ohg continue` to pick up where playback stopped", err)
}

// replaySafe lists the commands that only read, plus download, which skips
// the episodes already saved.
var replaySafe = map[string]bool{
	"discover": true, "search": true, "info": true, "episodes": true,
	"suggest": true, "related": true, "rank": true, "category": true,
	"landing": true, "seasons": true, "album": true, "download": true,
	"filter": true,
}

var errUsage = errors.New("invalid arguments")

func usage(format string, args ...any) error {
	return fmt.Errorf("%w: %s", errUsage, fmt.Sprintf(format, args...))
}

func rejectAccountFlags(args []string) error {
	for _, arg := range args {
		name := strings.TrimLeft(arg, "-")
		name = strings.SplitN(name, "=", 2)[0]
		switch strings.ToLower(name) {
		case "login", "password", "token", "cookie", "session", "username":
			return usage("account login is not available")
		}
	}
	return nil
}

func dispatch(ctx context.Context, args []string, d Dependencies) error {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprintln(d.Stdout, helpText)
		return nil
	}
	switch args[0] {
	case "discover":
		return cmdDiscover(ctx, args[1:], d)
	case "search":
		return cmdSearch(ctx, args[1:], d)
	case "info":
		return cmdInfo(ctx, args[1:], d)
	case "episodes":
		return cmdEpisodes(ctx, args[1:], d)
	case "play":
		return cmdPlay(ctx, args[1:], d)
	case "landing":
		return cmdLanding(ctx, args[1:], d)
	case "category":
		return cmdCategory(ctx, args[1:], d)
	case "rank":
		return cmdRank(ctx, args[1:], d)
	case "filter":
		return cmdFilter(ctx, args[1:], d)
	case "related":
		return cmdRelated(ctx, args[1:], d)
	case "hot":
		return cmdHot(ctx, args[1:], d)
	case "new":
		return cmdNew(ctx, args[1:], d)
	case "person":
		return cmdPerson(ctx, args[1:], d)
	case "cues":
		return cmdCues(ctx, args[1:], d)
	case "suggest":
		return cmdSuggest(ctx, args[1:], d)
	case "seasons":
		return cmdSeasons(ctx, args[1:], d)
	case "album":
		return cmdAlbum(ctx, args[1:], d)
	case "device":
		return cmdDevice(ctx, args[1:], d)
	case "history":
		return cmdHistory(ctx, args[1:], d)
	case "continue":
		return cmdContinue(ctx, args[1:], d)
	case "download":
		return cmdDownload(ctx, args[1:], d)
	default:
		return usage("unknown command %q", args[0])
	}
}

const helpText = `ohg — anonymous short-drama catalog and playback

  ohg discover [--channel 推荐|找剧|首页真人剧|首页漫剧|电影|听书|小说|社区|漫画|大视听|经典|知识|漫剧|真人剧]
               [--cursor TOKEN] [--all [--pages N]] [--json]
  ohg search <keyword> [--tab 综合|剧集|视频|讨论|小说|用户|听书] [--page-size N] [--cursor OFFSET]
              [--filter 真人剧|漫剧|AI剧|影视剧|未看过]... [--all [--pages N]] [--json]
  ohg info <series-id>... [--json]
  ohg episodes <series-id> [--json]
  ohg album <album-id> [--json]
  ohg seasons <series-id> [--json]
  ohg rank --shorts [--content ID] [--board ID] [--cursor C] [--json]
  ohg rank [--groups] [--cell ID] [--offset N] [--json]
  ohg rank --activity ID [--list ID] [--json]
  ohg filter --panel [--genre short_play|comic_series|ai_series] [--json]
  ohg filter [--genre G] [--filter KEY=VALUE]... [--cursor C] [--json]
  ohg related <series-id> [--json]
  ohg suggest <prefix> [--json]
  ohg hot [--json]
  ohg new [--date YYYYMMDD] [--cursor C] [--json]
  ohg person <user-id> [--cursor C] [--json]
  ohg cues [--json]
  ohg category <category-id> [--json]
  ohg landing <category-id> [--tab 8|1|0] [--filter KEY=VALUE]... [--cursor OFFSET] [--json]
  ohg play <series-id|title> [--pick] [--episode N | --resume] [--autoplay]
                          [--quality best|worst|360p|480p|540p|720p|1080p]
                          [--player auto|mpv|ffplay|vlc|open|xdg-open] [--url] [--json]
                          [--download PATH] [--key HEX] [--preview SECONDS] [--no-decrypt]
  ohg continue [play flags]
  ohg download <series-id|title> [--pick] [--episodes 1-10,12] [--dir PATH]
                          [--jobs N] [--quality Q]
  ohg history [--json]
  ohg history clear
  ohg device show
  ohg device reset

Every command is anonymous. There is no login.

play and download take a series ID or a title. A title is searched and the
best match is used; --pick lists the matches to choose from.

play resolves the service media model and streams it without writing a
file: ffplay and mpv decrypt a cenc-aes-ctr stream themselves, so seeking
works; vlc reads an ffmpeg-decrypted pipe; with open/xdg-open the episode is
decrypted to a temp file first. The content key is recovered from the
model's wrapped key material, or taken from --key HEX / OHG_CENC_KEY.
ffmpeg is required for decryption outside ffplay and mpv.
--download PATH saves one episode; --preview SECONDS plays or saves only the
leading seconds; --no-decrypt reports the protected model and stops.

--autoplay continues with the next episode when one ends, until the last
episode, until the player is closed early, or until Ctrl-C. In mpv, > and <
switch to the next and previous episode at any time.
Playback is remembered per series, including where an episode was left.
--resume continues from there, or from the episode after the last one
watched to the end; continue resumes the most recent series.

download saves every listed episode (default: all) as 第NNN集.mp4 under
--dir (default: the series title), --jobs at a time (default 3). Episodes
already saved are skipped, so an interrupted download can be run again.`

type flags struct {
	json      bool
	url       bool
	cursor    string
	channel   string
	tab       string
	quality   string
	player    string
	download  string
	key       string
	noDecrypt bool
	autoplay  bool
	resume    bool
	pick      bool
	filters   []string
	episodes  string
	dir       string
	jobs      int
	all       bool
	pages     int
	activity  string
	list      string
	cell      string
	offset    int
	// offsetSet records an explicit --offset, which single-page reads refuse.
	offsetSet bool
	groups    bool
	shorts    bool
	panel     bool
	content   string
	board     string
	date      string
	genre     string
	// episodeSet records an explicit --episode, which --resume excludes.
	episodeSet bool
	preview    int
	episode    int
	pageSize   int
	rest       []string
}

func parseFlags(args []string) (flags, error) {
	f := flags{pageSize: 20, episode: 1, jobs: 3}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			f.rest = append(f.rest, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(a, "--") {
			f.rest = append(f.rest, a)
			continue
		}
		name, val, has := strings.Cut(strings.TrimPrefix(a, "--"), "=")
		switch name {
		case "json":
			if has {
				return flags{}, usage("--json takes no value")
			}
			f.json = true
		case "url":
			if has {
				return flags{}, usage("--url takes no value")
			}
			f.url = true
		case "episode":
			if !has {
				if i+1 >= len(args) {
					return flags{}, usage("--episode needs a value")
				}
				i++
				val = args[i]
			}
			n, err := strconv.Atoi(val)
			if err != nil || n < 1 {
				return flags{}, usage("episode must be a positive integer")
			}
			f.episode = n
			f.episodeSet = true
		case "quality":
			if !has {
				if i+1 >= len(args) {
					return flags{}, usage("--quality needs a value")
				}
				i++
				val = args[i]
			}
			f.quality = strings.TrimSpace(val)
		case "player":
			if !has {
				if i+1 >= len(args) {
					return flags{}, usage("--player needs a value")
				}
				i++
				val = args[i]
			}
			f.player = strings.TrimSpace(val)
		case "download":
			if !has {
				if i+1 >= len(args) {
					return flags{}, usage("--download needs a path")
				}
				i++
				val = args[i]
			}
			if strings.TrimSpace(val) == "" {
				return flags{}, usage("--download needs a path")
			}
			f.download = val
		case "preview":
			if !has {
				if i+1 >= len(args) {
					return flags{}, usage("--preview needs a duration in seconds")
				}
				i++
				val = args[i]
			}
			n, err := strconv.Atoi(val)
			if err != nil || n < 1 {
				return flags{}, usage("preview must be a positive number of seconds")
			}
			f.preview = n
		case "filter":
			if !has {
				if i+1 >= len(args) {
					return flags{}, usage("--filter needs key=value")
				}
				i++
				val = args[i]
			}
			f.filters = append(f.filters, val)
		case "panel":
			if has {
				return flags{}, usage("--panel takes no value")
			}
			f.panel = true
		case "content", "board", "genre", "date":
			if !has {
				if i+1 >= len(args) {
					return flags{}, usage("--%s needs a value", name)
				}
				i++
				val = args[i]
			}
			if strings.TrimSpace(val) == "" {
				return flags{}, usage("--%s needs a value", name)
			}
			switch name {
			case "content":
				f.content = val
			case "board":
				f.board = val
			case "date":
				f.date = val
			default:
				f.genre = val
			}
		case "pick":
			if has {
				return flags{}, usage("--pick takes no value")
			}
			f.pick = true
		case "episodes", "dir":
			if !has {
				if i+1 >= len(args) {
					return flags{}, usage("--%s needs a value", name)
				}
				i++
				val = args[i]
			}
			if strings.TrimSpace(val) == "" {
				return flags{}, usage("--%s needs a value", name)
			}
			if name == "episodes" {
				f.episodes = val
			} else {
				f.dir = val
			}
		case "jobs":
			if !has {
				if i+1 >= len(args) {
					return flags{}, usage("--jobs needs a value")
				}
				i++
				val = args[i]
			}
			n, err := strconv.Atoi(val)
			if err != nil || n < 1 || n > 16 {
				return flags{}, usage("jobs must be between 1 and 16")
			}
			f.jobs = n
		case "resume":
			if has {
				return flags{}, usage("--resume takes no value")
			}
			f.resume = true
		case "autoplay":
			if has {
				return flags{}, usage("--autoplay takes no value")
			}
			f.autoplay = true
		case "no-decrypt":
			if has {
				return flags{}, usage("--no-decrypt takes no value")
			}
			f.noDecrypt = true
		case "key":
			if !has {
				if i+1 >= len(args) {
					return flags{}, usage("--key needs a hex content key")
				}
				i++
				val = args[i]
			}
			f.key = strings.TrimSpace(val)
			if f.key == "" {
				return flags{}, usage("--key needs a hex content key")
			}
		case "cursor":
			if !has {
				if i+1 >= len(args) {
					return flags{}, usage("--cursor needs a value")
				}
				i++
				val = args[i]
			}
			f.cursor = val
		case "channel":
			if !has {
				if i+1 >= len(args) {
					return flags{}, usage("--channel needs a value")
				}
				i++
				val = args[i]
			}
			f.channel = val
		case "tab":
			if !has {
				if i+1 >= len(args) {
					return flags{}, usage("--tab needs a value")
				}
				i++
				val = args[i]
			}
			f.tab = val
		case "shorts":
			if has {
				return flags{}, usage("--shorts takes no value")
			}
			f.shorts = true
		case "groups":
			if has {
				return flags{}, usage("--groups takes no value")
			}
			f.groups = true
		case "cell":
			if !has {
				if i+1 >= len(args) {
					return flags{}, usage("--cell needs an id")
				}
				i++
				val = args[i]
			}
			f.cell = val
		case "offset":
			if !has {
				if i+1 >= len(args) {
					return flags{}, usage("--offset needs a number")
				}
				i++
				val = args[i]
			}
			n, err := strconv.Atoi(val)
			if err != nil || n < 0 {
				return flags{}, usage("offset must be a non-negative integer")
			}
			f.offset, f.offsetSet = n, true
		case "activity":
			if !has {
				if i+1 >= len(args) {
					return flags{}, usage("--activity needs an id")
				}
				i++
				val = args[i]
			}
			f.activity = val
		case "list":
			if !has {
				if i+1 >= len(args) {
					return flags{}, usage("--list needs an id")
				}
				i++
				val = args[i]
			}
			f.list = val
		case "all":
			if has {
				return flags{}, usage("--all takes no value")
			}
			f.all = true
		case "pages":
			if !has {
				if i+1 >= len(args) {
					return flags{}, usage("--pages needs a value")
				}
				i++
				val = args[i]
			}
			n, err := strconv.Atoi(val)
			if err != nil || n < 1 {
				return flags{}, usage("pages must be a positive integer")
			}
			f.pages = n
		case "page-size":
			if !has {
				if i+1 >= len(args) {
					return flags{}, usage("--page-size needs a value")
				}
				i++
				val = args[i]
			}
			n, err := strconv.Atoi(val)
			if err != nil || n < 0 {
				return flags{}, usage("page-size must be a non-negative integer")
			}
			f.pageSize = n
		default:
			return flags{}, usage("unknown flag --%s", name)
		}
	}
	return f, nil
}

func requireService(d Dependencies) error {
	if d.Service == nil {
		return errors.New("service is not configured")
	}
	return nil
}

func cmdDiscover(ctx context.Context, args []string, d Dependencies) error {
	if err := requireService(d); err != nil {
		return err
	}
	f, err := parseFlags(args)
	if err != nil {
		return err
	}
	if len(f.rest) != 0 {
		return usage("discover takes no arguments")
	}
	if f.all {
		return runAllPages(ctx, d, f, func(ctx context.Context, cursor string) (*sdk.Page[model.SeriesSummary], error) {
			return d.Service.Discover(ctx, sdk.DiscoverOptions{Cursor: cursor, Channel: f.channel})
		})
	}
	page, err := d.Service.Discover(ctx, sdk.DiscoverOptions{Cursor: f.cursor, Channel: f.channel})
	if err != nil {
		return usageFrom(err)
	}
	return printValue(d.Stdout, page, f.json)
}

func usageFrom(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	if strings.HasPrefix(msg, "unknown discover channel") || strings.HasPrefix(msg, "unknown search tab") || strings.HasPrefix(msg, "search filters apply") || strings.HasSuffix(msg, "cursor is not verified") {
		return usage("%s", msg)
	}
	return err
}

func cmdSearch(ctx context.Context, args []string, d Dependencies) error {
	if err := requireService(d); err != nil {
		return err
	}
	f, err := parseFlags(args)
	if err != nil {
		return err
	}
	if len(f.rest) != 1 || strings.TrimSpace(f.rest[0]) == "" {
		return usage("search needs one keyword")
	}
	filters, err := searchFilters(f.filters)
	if err != nil {
		return err
	}
	if f.all {
		return runAllPages(ctx, d, f, func(ctx context.Context, cursor string) (*sdk.Page[model.SeriesSummary], error) {
			return d.Service.Search(ctx, f.rest[0], sdk.SearchOptions{Cursor: cursor, PageSize: f.pageSize, Tab: f.tab, Filters: filters})
		})
	}
	page, err := d.Service.Search(ctx, f.rest[0], sdk.SearchOptions{Cursor: f.cursor, PageSize: f.pageSize, Tab: f.tab, Filters: filters})
	if err != nil {
		return usageFrom(err)
	}
	return printValue(d.Stdout, page, f.json)
}

// searchFilterNames are the labels of the app's search filter panel.
var searchFilterNames = map[string]string{
	"真人剧": sdk.SearchFilterShortPlay,
	"漫剧":  sdk.SearchFilterComic,
	"AI剧": sdk.SearchFilterAIComic,
	"影视剧": sdk.SearchFilterFilm,
	"未看过": sdk.SearchFilterUnwatched,
}

// searchFilters reads --filter labels; each may hold several, comma-separated.
func searchFilters(values []string) ([]string, error) {
	var out []string
	for _, v := range values {
		for _, name := range strings.Split(v, ",") {
			name = strings.TrimSpace(name)
			filter, ok := searchFilterNames[name]
			if !ok {
				return nil, usage("unknown search filter %q (真人剧|漫剧|AI剧|影视剧|未看过)", name)
			}
			out = append(out, filter)
		}
	}
	return out, nil
}

func cmdInfo(ctx context.Context, args []string, d Dependencies) error {
	if err := requireService(d); err != nil {
		return err
	}
	f, err := parseFlags(args)
	if err != nil {
		return err
	}
	if len(f.rest) == 0 {
		return usage("info needs a series ID")
	}
	ids := make([]string, 0, len(f.rest))
	for _, arg := range f.rest {
		id, err := validateID(arg)
		if err != nil {
			return err
		}
		ids = append(ids, id)
	}
	if len(ids) == 1 {
		series, err := d.Service.GetSeries(ctx, ids[0])
		if err != nil {
			return err
		}
		return printValue(d.Stdout, series, f.json)
	}
	// Several IDs are read in one batch; unknown ones are left out.
	details, err := d.Service.GetSeriesBatch(ctx, ids)
	if err != nil {
		return err
	}
	list := make([]*model.Series, 0, len(details))
	for _, dt := range details {
		list = append(list, dt.Series)
	}
	if f.json {
		return printJSON(d.Stdout, list)
	}
	for i, series := range list {
		if i > 0 {
			fmt.Fprintln(d.Stdout)
		}
		if err := printValue(d.Stdout, series, false); err != nil {
			return err
		}
	}
	if len(list) < len(ids) {
		fmt.Fprintf(d.Stderr, "ohg: %d of %d series not found\n", len(ids)-len(list), len(ids))
	}
	return nil
}

func cmdEpisodes(ctx context.Context, args []string, d Dependencies) error {
	if err := requireService(d); err != nil {
		return err
	}
	f, err := parseFlags(args)
	if err != nil {
		return err
	}
	if len(f.rest) != 1 {
		return usage("episodes needs one series ID")
	}
	id, err := validateID(f.rest[0])
	if err != nil {
		return err
	}
	page, err := d.Service.ListEpisodes(ctx, id, sdk.EpisodeOptions{Cursor: f.cursor})
	if err != nil {
		return err
	}
	return printValue(d.Stdout, page, f.json)
}

func findEpisode(items []model.Episode, number int) (model.Episode, bool) {
	for _, e := range items {
		if e.Number == number {
			return e, true
		}
	}
	return model.Episode{}, false
}

// drmError names the protection scheme so the caller can tell a protected
// stream from a missing one. It never includes keys or per-segment data.
func drmError(p *model.Playback) error {
	scheme := ""
	for _, s := range p.Streams {
		if s.Encryption != "" {
			scheme = s.Encryption
			break
		}
	}
	if scheme == "" {
		return fmt.Errorf("%w: pass --key HEX, set OHG_CENC_KEY, or leave decryption enabled", sdk.ErrDRMProtected)
	}
	return fmt.Errorf("%w (%s): pass --key HEX, set OHG_CENC_KEY, or leave decryption enabled", sdk.ErrDRMProtected, scheme)
}

func cmdDevice(ctx context.Context, args []string, d Dependencies) error {
	if len(args) != 1 {
		return usage("device needs show or reset")
	}
	if d.Profiles == nil {
		return errors.New("profile store is not configured")
	}
	switch args[0] {
	case "show":
		p, err := d.Profiles.Load(ctx)
		if err != nil {
			return err
		}
		return printValue(d.Stdout, p, false)
	case "reset":
		return d.Profiles.Reset(ctx)
	default:
		return usage("unknown device command %q", args[0])
	}
}

func cmdHistory(ctx context.Context, args []string, d Dependencies) error {
	if d.History == nil {
		return errors.New("watch history is not configured")
	}
	if len(args) == 1 && args[0] == "clear" {
		return d.History.Clear(ctx)
	}
	f, err := parseFlags(args)
	if err != nil {
		return err
	}
	if len(f.rest) != 0 {
		return usage("history takes no arguments other than clear")
	}
	entries, err := d.History.List(ctx)
	if err != nil {
		return err
	}
	return printValue(d.Stdout, entries, f.json)
}

// choices lists ranking contents as id=name, marking the selected one.
func choices(contents []model.RankContent, selected string) string {
	parts := make([]string, 0, len(contents))
	for _, c := range contents {
		mark := ""
		if c.ID == selected {
			mark = "*"
		}
		parts = append(parts, c.ID+"="+c.Name+mark)
	}
	return strings.Join(parts, " ")
}

func boardChoices(boards []model.RankBoard, selected string) string {
	parts := make([]string, 0, len(boards))
	for _, b := range boards {
		mark := ""
		if b.ID == selected {
			mark = "*"
		}
		parts = append(parts, b.ID+"="+b.Name+mark)
	}
	return strings.Join(parts, " ")
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// streamName labels one representation for text output.
func streamName(s model.Stream) string {
	for _, name := range []string{s.Definition, s.Quality, s.Codec} {
		if name != "" {
			return name
		}
	}
	return "stream"
}

func validateID(raw string) (string, error) {
	if raw == "" {
		return "", usage("missing series ID")
	}
	if strings.Contains(raw, "://") {
		return "", usage("share links are not supported; use a series ID")
	}
	for _, r := range raw {
		if r < '0' || r > '9' {
			return "", usage("series ID must be numeric")
		}
	}
	if len(raw) < 4 || len(raw) > 25 {
		return "", usage("series ID must be 4-25 digits")
	}
	return raw, nil
}

func printValue(w io.Writer, v any, asJSON bool) error {
	if asJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(jsonView(v))
	}
	switch x := v.(type) {
	case *sdk.Page[model.SeriesSummary]:
		for _, s := range x.Items {
			fmt.Fprintf(w, "%s\t%s\t%d\n", s.ID, s.Title, s.EpisodeCount)
		}
		if x.Cursor != "" {
			fmt.Fprintf(w, "cursor\t%s\n", x.Cursor)
		}
	case *sdk.Page[model.Episode]:
		for _, e := range x.Items {
			state := ""
			if !e.Playable {
				state = "\tlocked"
			}
			fmt.Fprintf(w, "%d\t%s\t%s\t%d%s\n", e.Number, e.ID, e.Title, int(e.Duration.Seconds()), state)
		}
	case *model.ShortRanking:
		for _, c := range x.Contents {
			if c.ID == x.Content {
				fmt.Fprintf(w, "content\t%s\n", choices(x.Contents, c.ID))
				fmt.Fprintf(w, "board\t%s\n", boardChoices(c.Boards, x.Board))
			}
		}
		for _, item := range x.Items {
			if item.ActorID != "" {
				// An actor row: name, 人气, the details line, and the move.
				move := ""
				if item.Change != nil {
					move = "\t" + item.Change.Kind.String()
					if item.Change.Places > 0 {
						move += fmt.Sprintf(" %d", item.Change.Places)
					}
				}
				// The user ID is what ohg person takes; the actor ID only
				// stands in when the row has none.
				id := item.UserID
				if id == "" {
					id = item.ActorID
				}
				fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s%s\n", item.Position, id, item.Actor, item.Score, strings.Join(item.Details, " · "), move)
				continue
			}
			// Position is the order on the page; the service's rank repeats
			// around promoted rows.
			mark := ""
			if item.Promoted {
				mark = "\tpromoted"
			}
			fmt.Fprintf(w, "%d\t%s\t%s\t%d\t%s%s\n", item.Position, item.SeriesID, item.Title, item.EpisodeCount, item.Score, mark)
		}
		if x.Cursor != "" {
			fmt.Fprintf(w, "cursor\t%s\n", x.Cursor)
		}
	case *model.Ranking:
		if x.ListName != "" {
			fmt.Fprintf(w, "list\t%s\tstage=%d\tcan_vote=%t\n", x.ListName, x.ActivityStage, x.CanVote)
		}
		for _, l := range x.Lists {
			fmt.Fprintf(w, "rank_list\t%s\t%s\ttype=%d\tsize=%d\n", l.ID, l.Name, l.Type, l.Size)
		}
		for _, item := range x.Items {
			fmt.Fprintf(w, "%d\t%s\t%d\t%s\n", item.Rank, item.Title, item.Votes, item.Actor)
		}
	case []model.FilterRow:
		for _, r := range x {
			parts := make([]string, 0, len(r.Options))
			for _, o := range r.Options {
				parts = append(parts, o.Name+"="+o.Value)
			}
			fmt.Fprintf(w, "%s\t%s\t%s\n", r.Key, r.Name, strings.Join(parts, " "))
		}
	case *model.ShortFilterPage:
		for _, item := range x.Items {
			fmt.Fprintf(w, "%s\t%s\t%d\n", item.ID, item.Title, item.EpisodeCount)
		}
		if x.Cursor != "" {
			fmt.Fprintf(w, "cursor\t%s\n", x.Cursor)
		}
	case *model.Recommend:
		if x.CellTitle != "" {
			fmt.Fprintf(w, "cell\t%s\n", x.CellTitle)
		}
		for _, item := range x.Items {
			fmt.Fprintf(w, "%s\t%s\t%d\n", item.ID, item.Title, item.EpisodeCount)
		}
	case *model.CategoryPage:
		if x.CategoryName != "" {
			fmt.Fprintf(w, "category\t%s\t%s\n", x.CategoryID, x.CategoryName)
		}
		if len(x.Tabs) > 0 {
			parts := make([]string, 0, len(x.Tabs))
			for _, t := range x.Tabs {
				parts = append(parts, t.Name+"="+t.Value)
			}
			fmt.Fprintf(w, "tabs\t%s\tdefault=%d\n", strings.Join(parts, " "), x.DefaultTab)
		}
		for _, group := range x.Filters {
			parts := make([]string, 0, len(group.Options))
			for _, o := range group.Options {
				parts = append(parts, o.Name+"="+o.Value)
			}
			fmt.Fprintf(w, "filter\t%s\t%s\n", group.Key, strings.Join(parts, " "))
		}
		for _, item := range x.Items {
			fmt.Fprintf(w, "%s\t%s\t%s\t%d\n", item.ID, item.Title, item.Author, item.Words)
		}
		if x.HasMore {
			fmt.Fprintf(w, "cursor\t%d\n", x.NextOffset)
		}
	case *model.Album:
		fmt.Fprintf(w, "%s\t%s\t%d seasons\n", x.ID, x.Title, x.SeasonCount)
		for _, season := range x.Seasons {
			fmt.Fprintf(w, "season\t%d\t%s\t%s\t%d\t%s\n",
				season.Index, season.SeriesID, season.Title, season.EpisodeCount, season.Status)
		}
	case *model.Series:
		fmt.Fprintf(w, "%s\t%s\t%d\n%s\n", x.ID, x.Title, x.EpisodeCount, oneLine(x.Description))
		if x.CoverURL != "" {
			fmt.Fprintf(w, "cover\t%s\n", x.CoverURL)
		}
		fmt.Fprintf(w, "kind\t%s\t%s\n", x.ContentType, x.Platform)
		fmt.Fprintf(w, "status\t%s\tpay\t%s\n", x.Status, x.PayType)
		if x.RightText != "" {
			fmt.Fprintf(w, "badge\t%s\n", x.RightText)
		}
		if x.Heat > 0 || x.PlayCount > 0 || x.FollowCount > 0 || x.DiggCount > 0 {
			fmt.Fprintf(w, "counters\theat=%d\tplay=%d\tfollow=%d\tdigg=%d\n",
				x.Heat, x.PlayCount, x.FollowCount, x.DiggCount)
		}
		if !x.CreatedAt.IsZero() {
			fmt.Fprintf(w, "created\t%s\n", x.CreatedAt.Format("2006-01-02"))
		}
		for _, row := range x.Rows {
			fmt.Fprintf(w, "row\t%s\t%s\n", row.Kind, row.Text)
		}
		if n := x.Novel; n != nil {
			fmt.Fprintf(w, "novel\t%s\t%s\t%s\t%s\n", n.BookID, n.Title, n.Status, strings.Join(n.Tags, ","))
		}
	case *model.Playback:
		fmt.Fprintf(w, "episode\t%s\n", x.EpisodeID)
		if x.VideoID != "" {
			fmt.Fprintf(w, "video\t%s\n", x.VideoID)
		}
		if x.DurationSeconds > 0 {
			fmt.Fprintf(w, "duration\t%d\n", x.DurationSeconds)
		}
		fmt.Fprintf(w, "protected\t%t\n", x.Protected)
		for _, s := range x.Streams {
			fmt.Fprintf(w, "%s\t%s\t%d\t%t\t%s\n", streamName(s), s.Codec, s.Size, s.Protected, s.URL)
		}
	case *sdk.DeviceProfile:
		fmt.Fprintf(w, "device_id=%s\ninstall_id=%s\n", x.DeviceID, x.InstallID)
	case []history.Entry:
		for _, e := range x {
			state := "watching"
			if e.Finished {
				state = "finished"
			}
			fmt.Fprintf(w, "%s\t%s\t%d/%d\t%s\t%s\n", e.SeriesID, e.Title, e.Episode, e.Episodes, state,
				e.UpdatedAt.Local().Format("2006-01-02 15:04"))
		}
	default:
		return fmt.Errorf("unsupported output type %T", v)
	}
	return nil
}

func jsonView(v any) any {
	switch x := v.(type) {
	case *sdk.Page[model.Episode]:
		items := make([]map[string]any, 0, len(x.Items))
		for _, e := range x.Items {
			items = append(items, map[string]any{
				"id": e.ID, "series_id": e.SeriesID, "number": e.Number, "title": e.Title,
				"duration_seconds": int(e.Duration.Seconds()), "playable": e.Playable,
				"vertical": e.Vertical, "is_new": e.IsNew, "cover_url": e.CoverURL,
				"content_type": e.ContentType.String(), "comment_count": e.CommentCount,
				"digg_count": e.DiggCount,
			})
		}
		return map[string]any{"items": items, "has_more": x.HasMore}
	case *sdk.DeviceProfile:
		return map[string]any{"device_id": x.DeviceID, "install_id": x.InstallID}
	default:
		return v
	}
}

// runAllPages follows cursors and prints every distinct row, in order.
func runAllPages(ctx context.Context, d Dependencies, f flags, fetch func(context.Context, string) (*sdk.Page[model.SeriesSummary], error)) error {
	pager := sdk.NewPager(f.cursor, fetch)
	limit := f.pages
	if limit == 0 {
		limit = 10
	}
	items := make([]model.SeriesSummary, 0, 32)
	more := false
	for i := 0; i < limit; i++ {
		fresh, hasMore, err := pager.Next(ctx)
		if err != nil {
			return usageFrom(err)
		}
		items = append(items, fresh...)
		more = hasMore
		if !hasMore {
			break
		}
	}
	page := &sdk.Page[model.SeriesSummary]{Items: items, HasMore: more}
	return printValue(d.Stdout, page, f.json)
}

func cmdAlbum(ctx context.Context, args []string, d Dependencies) error {
	if err := requireService(d); err != nil {
		return err
	}
	f, err := parseFlags(args)
	if err != nil {
		return err
	}
	if len(f.rest) != 1 {
		return usage("album needs one album ID")
	}
	id, err := validateID(f.rest[0])
	if err != nil {
		return err
	}
	album, err := d.Service.GetAlbum(ctx, id, sdk.AlbumOptions{})
	if err != nil {
		return err
	}
	return printValue(d.Stdout, album, f.json)
}

func cmdSuggest(ctx context.Context, args []string, d Dependencies) error {
	if err := requireService(d); err != nil {
		return err
	}
	f, err := parseFlags(args)
	if err != nil {
		return err
	}
	if len(f.rest) != 1 || strings.TrimSpace(f.rest[0]) == "" {
		return usage("suggest needs one prefix")
	}
	words, err := d.Service.Suggest(ctx, f.rest[0])
	if err != nil {
		return err
	}
	if f.json {
		return printJSON(d.Stdout, words)
	}
	for _, w := range words {
		if w.Keyword != "" && w.Keyword != w.Text {
			fmt.Fprintf(d.Stdout, "%s\t%s\n", w.Text, w.Keyword)
			continue
		}
		fmt.Fprintln(d.Stdout, w.Text)
	}
	return nil
}

func printJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func cmdCategory(ctx context.Context, args []string, d Dependencies) error {
	if err := requireService(d); err != nil {
		return err
	}
	f, err := parseFlags(args)
	if err != nil {
		return err
	}
	if len(f.rest) != 1 {
		return usage("category needs one category ID")
	}
	page, err := d.Service.Categories(ctx, f.rest[0])
	if err != nil {
		return err
	}
	return printValue(d.Stdout, page, f.json)
}

func cmdLanding(ctx context.Context, args []string, d Dependencies) error {
	if err := requireService(d); err != nil {
		return err
	}
	f, err := parseFlags(args)
	if err != nil {
		return err
	}
	if len(f.rest) != 1 {
		return usage("landing needs one category ID")
	}
	filters := map[string]string{}
	for _, kv := range f.filters {
		key, value, ok := strings.Cut(kv, "=")
		if !ok || key == "" || value == "" {
			return usage("--filter takes key=value, got %q", kv)
		}
		filters[key] = value
	}
	page, err := d.Service.CategoryLanding(ctx, f.rest[0], sdk.LandingOptions{Tab: f.tab, Filters: filters, Cursor: f.cursor})
	if err != nil {
		return err
	}
	return printValue(d.Stdout, page, f.json)
}

func cmdRelated(ctx context.Context, args []string, d Dependencies) error {
	if err := requireService(d); err != nil {
		return err
	}
	f, err := parseFlags(args)
	if err != nil {
		return err
	}
	if len(f.rest) != 1 {
		return usage("related needs one series ID")
	}
	id, err := validateID(f.rest[0])
	if err != nil {
		return err
	}
	rec, err := d.Service.SeriesRecommend(ctx, id)
	if err != nil {
		return err
	}
	return printValue(d.Stdout, rec, f.json)
}

func cmdRank(ctx context.Context, args []string, d Dependencies) error {
	if err := requireService(d); err != nil {
		return err
	}
	f, err := parseFlags(args)
	if err != nil {
		return err
	}
	if len(f.rest) != 0 {
		return usage("rank takes no arguments")
	}
	var ranking *model.Ranking
	var err2 error
	if f.shorts {
		if f.offsetSet {
			return usage("rank --shorts pages with --cursor, not --offset")
		}
		page, err := d.Service.ShortRank(ctx, sdk.ShortRankQuery{Content: f.content, Board: f.board, Cursor: f.cursor})
		if err != nil {
			return err
		}
		return printValue(d.Stdout, page, f.json)
	}
	if f.content != "" || f.board != "" {
		return usage("--content and --board need --shorts")
	}
	if f.groups {
		groups, err := d.Service.RankGroups(ctx)
		if err != nil {
			return err
		}
		ranking := &model.Ranking{Lists: groups, Items: []model.RankItem{}}
		return printValue(d.Stdout, ranking, f.json)
	}
	if f.cell != "" || f.activity == "" {
		ranking, err2 = d.Service.RankRows(ctx, f.cell, f.offset)
	} else {
		ranking, err2 = d.Service.Ranking(ctx, f.activity, f.list)
	}
	if err2 != nil {
		return err2
	}
	return printValue(d.Stdout, ranking, f.json)
}

func cmdSeasons(ctx context.Context, args []string, d Dependencies) error {
	if err := requireService(d); err != nil {
		return err
	}
	f, err := parseFlags(args)
	if err != nil {
		return err
	}
	if len(f.rest) != 1 {
		return usage("seasons needs one series ID")
	}
	id, err := validateID(f.rest[0])
	if err != nil {
		return err
	}
	album, err := d.Service.Seasons(ctx, id)
	if err != nil && album == nil {
		return err
	}
	if err := printValue(d.Stdout, album, f.json); err != nil {
		return err
	}
	if err != nil {
		return err
	}
	return nil
}

func cmdCues(ctx context.Context, args []string, d Dependencies) error {
	if err := requireService(d); err != nil {
		return err
	}
	f, err := parseFlags(args)
	if err != nil {
		return err
	}
	if len(f.rest) != 0 {
		return usage("cues takes no arguments")
	}
	words, err := d.Service.SearchCues(ctx)
	if err != nil {
		return err
	}
	if f.json {
		return printJSON(d.Stdout, words)
	}
	for _, w := range words {
		fmt.Fprintln(d.Stdout, w)
	}
	return nil
}

// cmdNew prints one day of the 上新 calendar: the day and the days on offer,
// then series ID, title, local publish time, online or upcoming, heat or
// reservations, and episodes.
func cmdNew(ctx context.Context, args []string, d Dependencies) error {
	if err := requireService(d); err != nil {
		return err
	}
	f, err := parseFlags(args)
	if err != nil {
		return err
	}
	if len(f.rest) != 0 {
		return usage("new takes no arguments")
	}
	page, err := d.Service.NewReleases(ctx, sdk.NewReleaseQuery{Date: f.date, Cursor: f.cursor})
	if err != nil {
		if msg := err.Error(); strings.HasPrefix(msg, "date must be") || strings.HasPrefix(msg, "the cursor belongs") || strings.HasSuffix(msg, "cursor is not valid") {
			return usage("%s", msg)
		}
		return err
	}
	if f.json {
		return printJSON(d.Stdout, page)
	}
	fmt.Fprintf(d.Stdout, "date\t%s\n", page.Date)
	if len(page.Dates) > 0 {
		fmt.Fprintf(d.Stdout, "dates\t%s\n", strings.Join(page.Dates, " "))
	}
	for _, r := range page.Items {
		state, when := "upcoming", ""
		if r.Online {
			state = "online"
		}
		if !r.PublishAt.IsZero() {
			when = r.PublishAt.Local().Format("01-02 15:04")
		}
		fmt.Fprintf(d.Stdout, "%s\t%s\t%s\t%s\t%s\t%d\n", r.SeriesID, r.Title, when, state, r.Heat, r.EpisodeCount)
	}
	if page.Cursor != "" {
		fmt.Fprintf(d.Stdout, "cursor\t%s\n", page.Cursor)
	}
	return nil
}

// cmdPerson prints a person's page and the first page of their works; with
// --cursor it prints only the next page of works.
func cmdPerson(ctx context.Context, args []string, d Dependencies) error {
	if err := requireService(d); err != nil {
		return err
	}
	f, err := parseFlags(args)
	if err != nil {
		return err
	}
	if len(f.rest) != 1 {
		return usage("person needs one user ID")
	}
	userID := f.rest[0]
	var person *model.Person
	if f.cursor == "" {
		if person, err = d.Service.Person(ctx, userID); err != nil {
			return usageFromID(err)
		}
	}
	works, err := d.Service.PersonWorks(ctx, userID, f.cursor)
	if err != nil {
		return usageFromID(err)
	}
	if f.json {
		return printJSON(d.Stdout, struct {
			Person *model.Person   `json:"person,omitempty"`
			Works  *model.WorkPage `json:"works"`
		}{person, works})
	}
	if person != nil {
		fmt.Fprintf(d.Stdout, "%s\t%s\t%s\n", person.UserID, person.Name, strings.Join(append(append([]string{}, person.Tags...), person.Labels...), " · "))
		if person.Description != "" {
			fmt.Fprintln(d.Stdout, person.Description)
		}
		fmt.Fprintf(d.Stdout, "followers\t%d\tlikes\t%d\n", person.Followers, person.Likes)
		fmt.Fprintln(d.Stdout, "# 作品")
	}
	for _, w := range works.Items {
		fmt.Fprintf(d.Stdout, "%s\t%s\t%d\t%s\n", w.SeriesID, w.Title, w.EpisodeCount, strings.Join(w.Tags, " · "))
	}
	if works.Cursor != "" {
		fmt.Fprintf(d.Stdout, "cursor\t%s\n", works.Cursor)
	}
	return nil
}

// usageFromID turns a malformed ID or cursor into a usage error.
func usageFromID(err error) error {
	if msg := err.Error(); strings.HasPrefix(msg, "invalid user ID") || strings.HasPrefix(msg, "the cursor belongs") || strings.HasSuffix(msg, "cursor is not valid") {
		return usage("%s", msg)
	}
	return err
}

// cmdHot prints the hot lists: each list's name, then position, series ID,
// title, and the list's figure; topics print their search query.
func cmdHot(ctx context.Context, args []string, d Dependencies) error {
	if err := requireService(d); err != nil {
		return err
	}
	f, err := parseFlags(args)
	if err != nil {
		return err
	}
	if len(f.rest) != 0 {
		return usage("hot takes no arguments")
	}
	hot, err := d.Service.HotSearch(ctx)
	if err != nil {
		return err
	}
	if f.json {
		return printJSON(d.Stdout, hot)
	}
	printItems := func(items []model.HotItem) {
		for _, it := range items {
			score := it.SearchScore
			if score == "" {
				score = it.Heat
			}
			fmt.Fprintf(d.Stdout, "%d\t%s\t%s\t%s\n", it.Position, it.SeriesID, it.Title, score)
		}
	}
	if len(hot.Guesses) > 0 {
		fmt.Fprintln(d.Stdout, "# 猜你想搜")
		printItems(hot.Guesses)
	}
	for _, b := range hot.Boards {
		fmt.Fprintf(d.Stdout, "# %s\n", b.Name)
		printItems(b.Items)
		for i, t := range b.Topics {
			fmt.Fprintf(d.Stdout, "%d\t%s\t%s\n", i+1, t.Query, t.Heat)
		}
	}
	return nil
}

func cmdFilter(ctx context.Context, args []string, d Dependencies) error {
	if err := requireService(d); err != nil {
		return err
	}
	f, err := parseFlags(args)
	if err != nil {
		return err
	}
	if len(f.rest) != 0 {
		return usage("filter takes no arguments; select with --genre and --filter KEY=VALUE")
	}
	if f.panel {
		if len(f.filters) > 0 || f.cursor != "" {
			return usage("--panel lists the filter rows; it takes only --genre")
		}
		rows, err := d.Service.ShortFilterPanel(ctx, f.genre)
		if err != nil {
			return err
		}
		return printValue(d.Stdout, rows, f.json)
	}
	selections := map[string][]string{}
	if f.genre != "" {
		selections["genre"] = []string{f.genre}
	}
	for _, kv := range f.filters {
		key, value, ok := strings.Cut(kv, "=")
		if !ok || key == "" || value == "" {
			return usage("--filter takes KEY=VALUE, got %q", kv)
		}
		selections[key] = append(selections[key], value)
	}
	page, err := d.Service.ShortFilter(ctx, sdk.ShortFilterQuery{Selections: selections, Cursor: f.cursor})
	if err != nil {
		return err
	}
	return printValue(d.Stdout, page, f.json)
}
