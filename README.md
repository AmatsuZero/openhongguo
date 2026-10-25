# openhongguo

openhongguo 是红果短剧（Hongguo）的非官方开源客户端，包含 Go SDK 和命令行工具 `ohg`。

它能浏览短剧目录、搜索、播放和下载。所有请求都以匿名设备的身份发出，不需要登录。

> 本项目与红果短剧及其运营方没有任何关联，也没有获得授权或认可。“红果”和“Hongguo”是权利人的商标，这里只用来说明兼容对象。

## 快速上手

下面三条命令，演示了最常见的用法：搜索一部剧，然后播放。

```sh
go build -o bin/ohg ./cmd/ohg
bin/ohg search 全球死考
bin/ohg play 全球死考第一季
```

第一条命令编译 CLI，第二条搜索剧名，第三条播放第 1 集。

## 安装

编译需要 Go 1.22 或更高版本。

```sh
go build -o bin/ohg ./cmd/ohg
```

运行时还需要一些外部程序，用途如下。

| 用途 | 需要的程序 |
|---|---|
| 播放 | `mpv`（推荐）、`ffplay` 或 `vlc` 三者之一；macOS 上都没有时，改用系统的 `open` |
| 解密加密剧集 | `mpv` 和 `ffplay` 自带解密；用 `vlc` 或 `open` 播放，以及下载时，需要 `ffmpeg` |
| HTTPS | 系统的 CA 证书；精简版 Linux 镜像需要安装 `ca-certificates` |

macOS 用 Homebrew 安装。

```sh
brew install mpv ffmpeg
```

Debian 和 Ubuntu 用 apt 安装。

```sh
apt install mpv ffmpeg ca-certificates
```

## 用法

下文假定 `ohg` 已经在 `PATH` 里。完整参数见 `ohg --help`，下面按用途分组介绍。

### 搜索与播放

```sh
ohg search 全球死考                    # 搜索，输出 ID、剧名和集数
ohg play 全球死考第一季                # 按剧名播放第 1 集，取最匹配的结果
ohg play 全球死考 --pick               # 列出搜索结果，选一部再播
ohg play 7675400266065136665 --episode 5 --autoplay
ohg continue                          # 续播最近看的剧
ohg download 全球死考第一季 --episodes 1-20 --jobs 4
ohg history                           # 观看记录
```

播放时，剧集不写入磁盘，边下边播。不同播放器的行为略有不同。

- `mpv` 和 `ffplay` 直接解密，可以拖动进度。
- `vlc` 读取 `ffmpeg` 解密后的数据流，不能拖动进度。
- `open` 和 `xdg-open` 先把剧集解密到临时文件，播完后删除。

`--autoplay` 会在一集播完后自动播下一集，并提前取好下一集的地址。播完最后一集、提前关掉播放器，或者按 Ctrl-C，都会停止连播。匿名身份不能播的集会被跳过；连续 3 集不能播，就停止连播。

在 mpv 里，按 `>` 切到下一集，按 `<` 回到上一集。

主 CDN 地址播放失败时，会自动换成备用地址，重试一次。

“播完”和“中途关掉”的判断方法，取决于播放器。

- mpv：由脚本报告结束原因。
- ffplay：看最后的播放位置是否接近结尾。
- 其他播放器：退出即视为播完。

### 观看记录与续播

每次播放都会留下一条记录：剧名、集数、是否看完、停在第几秒。记录存放在配置目录的 `history.json` 里，文件权限为 0600。

- `play <剧> --resume`：没看完的集，从停下的位置往回 3 秒接着播；已经看完的，播下一集。
- `continue [play 的参数]`：续播最近看的一部剧，相当于 `play <最近的剧> --resume`。
- 使用 `--url`、`--json`、`--download` 或 `--preview` 时，不写记录。

### 下载

`download` 把每一集存成 `<目录>/第NNN集.mp4`，目录名默认是剧名。

下载时，先写入 `.part` 文件，下载完成后再改名。所以目录里的 `.mp4` 文件一定是完整的。已经存在的集会被跳过，中断后重新运行同一条命令，就能接着下载。

只下载一集时，也可以用 `play <剧> --episode N --download PATH`。

### 浏览与发现

```sh
ohg discover --channel 电影            # 频道首页，默认是“推荐”
ohg discover --channel 漫剧 --all      # 沿着游标翻页，跨页去重
ohg search 末世 --all --pages 3        # 搜索结果翻页
ohg search 奇幻 --filter 真人剧,影视剧   # 只要某些类型，只用于“综合”分类
ohg info 7690184933037329432           # 剧的元数据
ohg info 7675400266065136665 7683373426689461273  # 一次读取多部剧
ohg episodes 7690184933037329432       # 集列表，不能播的集标为 locked
ohg seasons 7675400266065136665        # 多季剧的全部季
ohg related 7690184933037329432        # 推荐的剧
ohg suggest 末世                        # 搜索联想词
ohg hot                                # 热搜
ohg new                                # 上新日历
ohg new --date 20261026                # 指定日期的上新
ohg person 3358342267023107            # 演员主页和作品
ohg cues                               # 搜索框里的提示词
```

几条命令的补充说明如下。

- `search --filter` 可选的类型有：真人剧、漫剧、AI剧、影视剧和未看过。
- `info` 输出类型、状态、角标、热度和标签。改编自小说的剧，会多一行 `novel`。一次给多个 ID 时，每 10 部发一个请求。
- `seasons` 列出全部季，包括已经预告、但还没有剧集的季。
- `related` 在匿名身份下，返回通用的“更多好剧”，不针对这部剧。
- `hot` 输出“猜你想搜”、短剧热搜榜、漫剧热搜榜和热点话题榜。每个榜只有页面上显示的那几条。
- `new` 输出当天已经上线和即将上线的剧，附上线时间、热度或预约人数。输出里的 `dates` 是前后 7 天中可选的日期；用 `--cursor` 翻页。
- `person` 的 ID 取自演员榜的第二列，用 `--cursor` 翻看更多作品。
- `cues` 是搜索框里滚动的提示词，比如“书名 / 主角 / 作者”，不是热搜。

### 排行榜与筛选

```sh
ohg rank --shorts                      # 默认是“全部”内容的“推荐榜”
ohg rank --shorts --content human --board human_new_rank  # 真人剧的新剧榜
ohg rank --shorts --cursor <上一页的 cursor>
ohg filter --panel --genre comic_series                    # 筛选面板
ohg filter --genre comic_series --filter category_dim_theme=cate_7 --filter sort=hot_score
ohg filter --cursor <上一页的 cursor>
```

`rank --shorts` 先打印内容类型和榜单目录，再按页面顺序列出剧。目录里列出了每种内容可选的榜单。翻页的游标里带着内容和榜单；`filter` 的游标里带着筛选条件。

`filter --panel` 列出每个筛选维度的 KEY 和可选值，维度随体裁不同。上面第二条 `filter` 命令，筛选的是“漫剧 · 玄幻 · 最高热度”。

排行榜的名次以页面顺序为准，有几点需要注意。

- 服务端会插入“平台推荐”的推广位，标为 `promoted`。推广位照抄下一行的名次。
- “推荐榜”按个人兴趣排序，名次会重复。
- 剧集榜翻页时，游标里带着已经展示过的剧，服务端据此去重，所以页与页之间不重复。

演员榜（`--content ranklist_celebrity`，包括剧星榜和星光榜）列出的是演员，不是剧。每一行依次是：用户 ID、名字、人气或星光值、作品数等副标题，以及名次变化（up、down、same 或 new）。用户 ID 可以直接传给 `ohg person`。演员榜按 offset 和 session 翻页。

### 小说与听书

下面这些命令返回的是书（小说和听书），不是短剧。`play` 和 `download` 不接受这些 ID。

```sh
ohg category 68                        # 分类的标签页：全部、男生、女生
ohg landing 68 --filter sort_by=76 --filter book_status=0
ohg landing 68 --tab 1 --filter sub_category=515 --filter word_type=1 --cursor 10
ohg rank                               # 书城榜单
ohg rank --groups                      # 书城榜单的类型
ohg rank --activity <id> --list <id>   # 活动榜单
```

- `landing` 列出分类下的书，可以按排序方式、完结状态、频道、子题材和字数筛选，用 `--cursor` 翻页。
- `rank` 每次请求都换一批书，名次只在当前这一页内有意义。
- 活动没有开放时，活动榜单没有条目。

## 配置

CLI 通过环境变量配置，一般都不需要设置。

| 环境变量 | 作用 |
|---|---|
| `OHG_CONFIG_DIR` | 配置目录，保存匿名设备 `anonymous.json` 和观看记录 `history.json` |
| `OHG_CENC_KEY` | 手动指定单集的内容密钥，通常不需要 |
| `OHG_API_ORIGIN` | 读取类接口的地址 |
| `OHG_REGISTER_ORIGIN` | 设备注册接口的地址 |
| `OHG_DISTRIBUTION_ORIGIN` | 短剧筛选（`/reading/distribution/`）和活动榜接口的地址 |

配置目录默认是 `<用户配置目录>/openhongguo`，macOS 上就是 `~/Library/Application Support/openhongguo`。首次运行时，如果发现旧名称的目录 `…/hgdrama`，会把其中的两个文件拷贝过来。

接口地址的默认值和 App 保持一致。

- `OHG_API_ORIGIN` 默认是 `api5-normal-sinfonlinea.fqnovel.com`。也可以改成 `https://reading.snssdk.com`，但它不提供短剧筛选和活动榜。
- `OHG_REGISTER_ORIGIN` 默认是 `log5-applog-lq.fqnovel.com`，也可以改成 `https://log.snssdk.com`。
- `OHG_DISTRIBUTION_ORIGIN` 默认与 `OHG_API_ORIGIN` 的默认值相同。把 `OHG_API_ORIGIN` 改成 `reading.snssdk.com` 之后，短剧筛选和活动榜仍然走这个地址。

请求失败时，CLI 会自动处理。

- 遇到限流或网络错误，自动退避重试，默认 3 次。
- 服务端要求等待超过 10 秒时，直接报错，并给出需要等待的时间。
- 设备被服务端拒绝时，提示并注册一个新的匿名设备。查询类命令和 `download` 会自动重放一次请求；`play` 不重放，请用 `ohg continue` 接着播。

## 匿名设备

`ohg device show` 查看当前的匿名设备，`ohg device reset` 重新生成一个。

匿名设备在注册请求、每个请求的设备参数和 User-Agent 里，都自称同一台手机。默认机型是小米 14（`23127PN0CC`，Android 14），渠道是小米商店（`xiaomi_8662_64`）。机型记录在 `anonymous.json` 的 `device` 字段里，所以同一个设备 ID 永远对应同一个机型。

## SDK

CLI 的全部功能都来自 `sdk` 包，Go 程序可以直接调用。

### 指定机型

`sdk.Options.Device`（类型为 `model.Device`）可以换成别的机型，没有填写的字段取默认值。

```go
client, err := sdk.NewClient(sdk.Options{
	Transport: transport, ProfileStore: store,
	Device: model.Device{Model: "21051182C", OSVersion: "11", OSAPI: 30, Build: "RKQ1.200826.002",
		Resolution: "1600*2524", DPI: 360, Pad: true}, // 小米平板 5
})
```

这个选项只影响新注册的设备。已经注册的档案沿用注册时的机型，`device reset` 之前的档案也是如此。要换机型，需要先重置设备。

### 取得内容密钥

加密剧集（`Playback.Protected`）的内容密钥，用 `sdk.ContentKey` 取得。

对于加密剧集，`ResolvePlayback` 会同时返回播放模型和 `ErrDRMProtected`。把这个播放模型传给 `sdk.ContentKey` 即可。

```go
pb, err := client.ResolvePlayback(ctx, episodeID, sdk.PlaybackOptions{})
if errors.Is(err, sdk.ErrDRMProtected) {
	key, err := sdk.ContentKey(pb) // key.Hex 可直接交给 ffmpeg 的 -decryption_key，或播放器的 ClearKey
	...
}
```

`sdk.ContentKey` 只做本地计算，不发请求，也不依赖 ffmpeg。同一集的各个清晰度共用这把密钥，加密方式为 cenc-aes-ctr。

## 接口契约检查

契约检查用来发现服务端的改版。`testdata/protocol/` 里的响应样本，就是解析器依赖的契约：样本里的每个字段路径和 JSON 类型，真实接口都必须仍然提供。

检查按 CLI 的调用顺序请求真实接口，逐个与样本比对。覆盖的接口如下：

- 发现、搜索、剧集详情和播放模型；
- 联想词、推荐、多季、批量详情、热搜和分类；
- 书城榜单、短剧筛选面板与结果、短剧榜；
- 演员榜、演员主页、上新和搜索提示词。

比对之后，再验证结果确实可用：搜得到剧，剧集编号连续，播放模型有流，恢复出的密钥能用 ffmpeg 解出一帧，各个列表不为空，短剧榜的前几名确实是视频。活动榜不在检查范围内，因为活动没有开放时，它本来就没有条目。

运行检查的命令如下。

```sh
go test -tags live -run TestLiveContract -v ./internal/contract
```

检查可以用下面的环境变量调整。

| 环境变量 | 作用 |
|---|---|
| `OHG_CONFIG_DIR` | 复用已有的匿名设备；不设置时，每次注册一个新设备，并顺带检查注册响应 |
| `OHG_CONTRACT_QUERY` | 搜索词，默认是 `全球死考` |
| `OHG_CONTRACT_MOVIE` | 额外检查一部分段（DASH）电影，填剧的 ID，对照 `dynamic-playback.json` |
| `OHG_CONTRACT_OUT` | 把报告和真实响应的结构写到这个目录，方便与样本对比；结构只有路径和类型，不含任何值 |

不带 `-tags live` 时，`go test ./...` 不访问网络。`.github/workflows/contract.yml` 每天运行一次检查，并把结构文件作为 artifact 上传。

服务端改版时，检查会列出漂移的字段，比如 `data.video_data.video_list[].title: missing (fixture has string)`。修改解析器时，请同步更新对应的样本。样本只保留解析器真正读取的字段，这样无关字段的变化不会造成误报。

## 平台

各平台的测试情况如下。

- macOS：实测了 mpv 和 ffplay 播放。
- Linux：单元测试通过，实测了下载和解密。
- Windows：只做了交叉编译。`internal/media` 的测试用 `/bin/sh` 脚本模拟播放器，不能在 Windows 上运行。

## 目录结构

- `sdk/`：客户端 SDK，提供搜索、发现、剧集和播放模型
- `internal/cliapp/`：命令行
- `internal/media/`：调用播放器，以及用 ffmpeg 解密和下载
- `internal/drm/`：从播放模型的密钥材料恢复内容密钥，通过 `sdk.ContentKey` 对外提供
- `internal/history/`：观看记录
- `internal/contract/`：接口契约检查
- `docs/rea/`：协议与解密的逆向分析记录
  - `playback-contract.md`：播放模型的请求和响应契约
  - `playback-decrypt.md`：内容密钥的恢复与验证
  - `reverse-engineering-backlog.md`：R1～R14 能力清单、逐项结论与待办
  - `video-model-enums.json`：枚举值的证据，包括系列状态、付费、内容类型、平台和信息行

## 许可证

本项目采用 Apache License 2.0 许可证，全文见 [LICENSE](LICENSE)。Copyright 2026 beilunyang。
