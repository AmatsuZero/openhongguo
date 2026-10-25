# openhongguo

openhongguo 是红果短剧（Hongguo）的非官方开源客户端，包含 Go SDK、命令行工具 `ohg`，以及 Node.js、Android 和 iOS 的 SDK。

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

播放需要 `mpv`、`ffplay` 或 `vlc`，下载需要 `ffmpeg`。详见[命令行文档](docs/cli.md#安装)。

## 命令行

常用命令如下。完整用法、环境变量和匿名设备的说明，见 [docs/cli.md](docs/cli.md)。

```sh
ohg search 全球死考                  # 搜索
ohg play 全球死考第一季              # 播放第 1 集
ohg continue                        # 续播最近看的剧
ohg download 全球死考 --episodes 1-10 # 下载前 10 集
ohg discover --channel 找剧          # 浏览书城频道
ohg rank --shorts                   # 短剧排行榜
```

## SDK

CLI 的全部功能都来自 `sdk` 包，Go 程序可以直接调用。

`adapters.NewClient` 用默认的网络请求实现创建客户端，并把匿名设备保存在指定的文件里。

```go
client, err := adapters.NewClient("anonymous.json", sdk.Options{})
page, err := client.Search(ctx, "全球死考", sdk.SearchOptions{})
```

`adapters` 下的三个子包也可以单独组合：`httptransport` 发送请求，`filestore` 和 `memstore` 分别把设备档案存在文件和内存里。

### 指定机型

`sdk.Options.Device`（类型为 `model.Device`）可以换成别的机型，没有填写的字段取默认值。

```go
client, err := adapters.NewClient("anonymous.json", sdk.Options{
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

## Node.js SDK

`bindings/node` 是 Node.js 版的 SDK（npm 包名 `openhongguo`）。它把 Go SDK 编译成 WebAssembly，接口和 Go 版一一对应，方法返回 Promise。

```js
import { createClient } from 'openhongguo';

const client = await createClient();
const page = await client.search('全球死考');
```

这个包还没有发布到 npm，构建和用法见 [bindings/node/README.md](bindings/node/README.md)。

## Android 与 iOS SDK

`bindings/mobile` 是 Android 和 iOS 版的 SDK。它用 gomobile 把 Go SDK 编译成原生库，再用 Kotlin 和 Swift 封装，接口和 Node.js 版一一对应。Kotlin 的方法是挂起函数，Swift 的方法是 `async throws`。

```kotlin
val client = OpenhongguoClient.create(context)
val page = client.search("全球死考")
```

```swift
let client = try OpenhongguoClient.create()
let page = try await client.search("全球死考")
```

两个库都还没有发布，构建和用法见 [bindings/mobile/README.md](bindings/mobile/README.md)。

## 开发

`go test ./...` 不访问网络。每日的接口契约检查，以及每次推送时运行的持续集成，见 [docs/development.md](docs/development.md)。

## 目录结构

- `sdk/`：客户端 SDK，提供搜索、发现、剧集和播放模型
- `adapters/`：SDK 的默认实现（网络请求、设备档案）和便捷构造函数 `NewClient`
- `internal/cliapp/`：命令行
- `internal/media/`：调用播放器，以及用 ffmpeg 解密和下载
- `internal/drm/`：从播放模型的密钥材料恢复内容密钥，通过 `sdk.ContentKey` 对外提供
- `internal/history/`：观看记录
- `internal/contract/`：接口契约检查
- `bindings/node/`：Node.js SDK（WebAssembly 封装）
- `bindings/mobile/`：Android 与 iOS SDK（gomobile 封装）
- `bindings/internal/`：各语言绑定共用的方法表、类型解析和测试样本服务
- `docs/cli.md`、`docs/development.md`：命令行文档，以及契约检查与持续集成的说明
- `docs/rea/`：协议与解密的逆向分析记录，见 [docs/rea/README.md](docs/rea/README.md)

## 许可证

本项目采用 Apache License 2.0 许可证，全文见 [LICENSE](LICENSE)。Copyright 2026 beilunyang。
