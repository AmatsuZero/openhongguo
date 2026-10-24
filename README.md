# openhongguo

红果短剧（Hongguo）的非官方开源客户端：匿名的短剧目录、播放与下载，包含 Go SDK 和命令行工具 `ohg`。所有请求都以匿名设备身份发出，没有登录。

> 本项目与红果短剧及其运营方没有任何关联，也未获其授权或认可。“红果”“Hongguo”是其各自权利人的商标，这里仅用于说明兼容对象。

## 安装

需要 Go 1.22+。

```sh
go build -o bin/ohg ./cmd/ohg
```

运行时依赖：

| 用途 | 需要 |
|---|---|
| 播放 | `mpv`（推荐）、`ffplay` 或 `vlc` 之一；macOS 上没有时会退回 `open` |
| 解密加密剧集 | `ffplay` 和 `mpv` 自带解密；用 `vlc`、`open` 播放或下载时需要 `ffmpeg` |
| HTTPS | 系统 CA 证书（精简 Linux 镜像要装 `ca-certificates`） |

macOS：`brew install mpv ffmpeg`。Debian/Ubuntu：`apt install mpv ffmpeg ca-certificates`。

## 用法

```sh
ohg search 全球死考  # 搜索，输出 ID、剧名、集数
ohg play 全球死考第一季  # 按剧名播放第 1 集（取最匹配的结果）
ohg play 全球死考 --pick  # 列出搜索结果，选一个
ohg play 7675400266065136665 --episode 5 --autoplay
ohg continue  # 续播最近看的剧，从上次停下的位置
ohg download 全球死考第一季 --episodes 1-20 --jobs 4
ohg history  # 观看记录
```

目录浏览与发现：

```sh
ohg discover --channel 电影            # 频道首页（默认 推荐）
ohg discover --channel 漫剧 --all      # 跟随游标翻页，跨页去重
ohg search 末世 --all --pages 3        # 搜索结果翻页
ohg search 奇幻 --filter 真人剧,影视剧   # 只要某些类型：真人剧/漫剧/AI剧/影视剧/未看过（仅综合 tab）
ohg info 7690184933037329432           # 剧的元数据：kind/status/badge/热度/标签，改编自小说时多一行 novel
ohg info 7675400266065136665 7683373426689461273  # 多个 ID 一次批量读取（每 10 部一个请求）
ohg episodes 7690184933037329432       # 集列表，不可播的集标 locked
ohg seasons 7675400266065136665        # 多季剧的全部季（含已预告、0 集的季）
ohg related 7690184933037329432        # 推荐的剧（匿名下是通用的“更多好剧”，不针对这部剧）
ohg suggest 末世                        # 搜索联想词
ohg hot   # 猜你想搜 + 短剧/漫剧热搜榜 + 热点话题榜（每榜只有页面显示的几条）
ohg new                    # 上新日历：今天已上线与待上线的剧（上线时间、热度或预约人数）
ohg new --date 20261026    # 指定日期（输出里的 dates 是可选的前后 7 天）；--cursor 翻页
ohg person 3358342267023107  # 演员/角色主页与作品列表（ID 取自演员榜的第二列）；--cursor 翻作品
ohg cues  # 搜索框提示词（“书名 / 主角 / 作者…”，不是热搜）
```

短剧排行榜与筛选：

```sh
ohg rank --shorts  # 默认「全部 → 推荐榜」；先打印内容类型和榜单目录，再按页面顺序列出剧
ohg rank --shorts --content human --board human_new_rank  # 真人剧新剧榜；目录里列出每种内容可选的榜单
ohg rank --shorts --cursor <上一页的 cursor>  # 翻页，游标里带着内容和榜单
ohg filter --panel --genre comic_series  # 筛选面板：每一维度的 KEY 与可选值（随体裁不同）
ohg filter --genre comic_series --filter category_dim_theme=cate_7 --filter sort=hot_score  # 漫剧·玄幻·最高热度
ohg filter --cursor <上一页的 cursor>  # 翻页，游标里带着筛选条件
```

下面这些拿到的是**书（小说 / 听书）**，不是短剧，`play` 和 `download` 会拒绝这些 ID：

```sh
ohg category 68                        # 分类的 tab（全部 / 男生 / 女生）
ohg landing 68 --filter sort_by=76 --filter book_status=0   # 分类列表，按排序、完结状态筛选
ohg landing 68 --tab 1 --filter sub_category=515 --filter word_type=1 --cursor 10   # 男生频道、子题材、字数、翻页
ohg rank                               # 书城榜单：每次请求换一批，名次只在本页内有意义
ohg rank --groups                      # 榜单类型（书）
ohg rank --activity <id> --list <id>   # 活动榜单（活动未开放时没有条目）
```

短剧排行榜的名次以页面顺序为准：服务端会插入「平台推荐」的推广位（标为 `promoted`），它照抄下一行的名次；「推荐榜」按个人兴趣排序，名次会重复。演员榜（`--content ranklist_celebrity`，剧星榜 / 星光榜）列的是演员：每行是用户 ID（可直接给 `ohg person`）、名字、人气或星光值、作品数等副标题和名次变化（up/down/same/new）。剧集榜翻页时，游标里带着已展示过的剧，服务端据此去重，页与页不重复；演员榜按 offset 和 session 翻页。

完整参数见 `ohg --help`。

### 播放

- 不落盘的流式播放。`mpv` 和 `ffplay` 直接解密，可以拖动进度；`vlc` 读取 `ffmpeg` 解密后的管道，不能拖动；`open`/`xdg-open` 先解密到临时文件，播完删除。
- `--autoplay`：一集播完自动下一集，同时预取下一集的地址。播到最后一集、提前关闭播放器、或 Ctrl-C 时停止。匿名不可播的集会跳过，连续 3 集不可播则停止。
- mpv 里按 `>` / `<` 随时切到下一集 / 上一集。
- 主 CDN 地址播放失败时会自动换备用地址重试一次。
- 怎么判断“播完”还是“关掉”：mpv 由脚本报告结束原因，ffplay 看最后的播放位置是否接近结尾，其它播放器以退出为播完。

### 观看记录与续播

每次播放都会记录剧名、集数、是否看完、停在第几秒，存放在配置目录的 `history.json`（权限 0600）。

- `play <剧> --resume`：未看完的集从停下的位置（回退 3 秒）接着播；已看完则播下一集。
- `continue [play 参数]`：续播最近一部剧，等于 `play <最近的剧> --resume`。
- `--url`、`--json`、`--download`、`--preview` 不写记录。

### 下载

`download` 把剧集存为 `<目录>/第NNN集.mp4`，目录默认是剧名。先写 `.part` 文件，完成后再改名，所以目录里的 `.mp4` 一定是完整的；已存在的集会跳过，中断后重跑同一条命令即可续下。单集也可以用 `play <剧> --episode N --download PATH`。

## 配置

| 环境变量 | 作用 |
|---|---|
| `OHG_CONFIG_DIR` | 配置目录，默认 `<用户配置目录>/openhongguo`（macOS 为 `~/Library/Application Support/openhongguo`），保存匿名设备 `anonymous.json` 和观看记录 `history.json`。首次运行时会从旧名称的目录 `…/hgdrama` 拷贝这两个文件 |
| `OHG_CENC_KEY` | 手动指定单集的内容密钥，通常不需要 |
| `OHG_API_ORIGIN` | 覆盖读接口的地址，默认 `api5-normal-sinfonlinea.fqnovel.com`（和 App 一致）；也可以用 `https://reading.snssdk.com`，但它不提供短剧筛选和活动榜 |
| `OHG_REGISTER_ORIGIN` | 覆盖设备注册的地址，默认 `log5-applog-lq.fqnovel.com`（和 App 一致）；`https://log.snssdk.com` 也可以 |
| `OHG_DISTRIBUTION_ORIGIN` | 覆盖 `/reading/distribution/`（短剧筛选）和活动榜接口的地址，默认同 `OHG_API_ORIGIN` 的默认值；把 `OHG_API_ORIGIN` 改成 `reading.snssdk.com` 时这组接口仍走这里 |
| — | 失败重试：限流 / 网络错误自动退避重试（默认 3 次）；服务端要求等待超过 10 秒时直接报错并给出等待时间。设备被服务端拒绝时会提示并注册新的匿名设备，查询类命令和 `download` 自动重放一次，`play` 不重放，改用 `ohg continue` 接着播 |

`ohg device show` 查看匿名设备，`device reset` 重新生成。

匿名设备在注册请求、每个请求的设备参数和 User-Agent 里自称同一台手机，默认是小米 14（`23127PN0CC`，Android 14，小米商店渠道 `xiaomi_8662_64`），并记在 `anonymous.json` 的 `device` 里：同一个设备 ID 永远对应同一个机型。SDK 调用方可以用 `sdk.Options.Device`（`model.Device`）换成别的机型，没填的字段取默认值：

```go
client, err := sdk.NewClient(sdk.Options{
	Transport: transport, ProfileStore: store,
	Device: model.Device{Model: "21051182C", OSVersion: "11", OSAPI: 30, Build: "RKQ1.200826.002",
		Resolution: "1600*2524", DPI: 360, Pad: true}, // 小米平板 5
})
```

它只决定**新注册**的设备；已注册的档案沿用注册时的机型（包括 `device reset` 之前的档案），要换机型需要重置设备。

加密剧集（`Playback.Protected`）的内容密钥用 `sdk.ContentKey` 取：`ResolvePlayback` 对加密剧集同时返回播放模型和 `ErrDRMProtected`，把这个模型传进去即可。它只做本地计算，不发请求，也不依赖 ffmpeg；同一集的各个清晰度共用这把密钥（cenc-aes-ctr）。

```go
pb, err := client.ResolvePlayback(ctx, episodeID, sdk.PlaybackOptions{})
if errors.Is(err, sdk.ErrDRMProtected) {
	key, err := sdk.ContentKey(pb) // key.Hex 可直接给 ffmpeg 的 -decryption_key 或播放器的 ClearKey
	...
}
```

## 接口契约检查

`testdata/protocol/` 里的响应样本就是解析器依赖的契约：样本中出现的每个字段路径和 JSON 类型，真实接口都必须还有。契约检查按 CLI 的调用顺序请求真实接口（发现 → 搜索 → 剧集详情 → 播放模型 → 联想词、推荐、多季、批量详情、热搜、分类、书城榜单、短剧筛选面板与结果、短剧榜、演员榜与演员主页、上新、搜索提示词），逐个和样本比对，再验证结果可用：搜得到剧、剧集编号连续、播放模型有流、恢复出的密钥能用 ffmpeg 解出一帧、各列表不为空、短剧榜的前几名确实是视频。活动榜不在其中：活动未开放时它本来就没有条目。

```sh
go test -tags live -run TestLiveContract -v ./internal/contract
```

| 环境变量 | 作用 |
|---|---|
| `OHG_CONFIG_DIR` | 复用已有的匿名设备；不设则每次注册一个新的，并同时检查注册响应 |
| `OHG_CONTRACT_QUERY` | 搜索词，默认 `全球死考` |
| `OHG_CONTRACT_MOVIE` | 再检查一部分段（DASH）电影，填剧 ID，对照 `dynamic-playback.json` |
| `OHG_CONTRACT_OUT` | 把报告和真实响应的结构（只有路径和类型，不含任何值）写到这个目录，便于和样本对比 |

不带 `-tags live` 时 `go test ./...` 不会访问网络。`.github/workflows/contract.yml` 每天跑一次，并把结构文件作为 artifact 上传。

服务端改版时，检查会列出类似 `data.video_data.video_list[].title: missing (fixture has string)` 的漂移项。改解析器时同步更新对应样本：样本里只保留解析器真正读取的字段，契约才不会因为无关字段的变动而误报。

## 平台

在 macOS（mpv、ffplay）上实测；Linux 上单元测试通过，并实测了下载和解密；Windows 只做了交叉编译。`internal/media` 的测试用 `/bin/sh` 脚本模拟播放器，不能在 Windows 上运行。

## 目录

- `sdk/`：客户端 SDK（搜索、发现、剧集、播放模型）
- `internal/cliapp/`：命令行
- `internal/media/`：调用播放器、ffmpeg 解密与下载
- `internal/drm/`：从播放模型中的密钥材料恢复内容密钥，经 `sdk.ContentKey` 对外提供
- `internal/history/`：观看记录
- `internal/contract/`：接口契约检查
- `docs/rea/`：协议与解密的逆向分析记录
  - `playback-contract.md`：播放模型的请求/响应契约
  - `playback-decrypt.md`：内容密钥的恢复与验证
  - `reverse-engineering-backlog.md`：R1–R14 能力清单、逐项结论与待办
  - `video-model-enums.json`：枚举值证据（系列状态、付费、内容类型、平台、信息行）
