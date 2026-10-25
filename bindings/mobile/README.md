# openhongguo（Android 与 iOS）

这里是红果短剧（Hongguo）非官方 SDK 的 Android 和 iOS 版本。它能浏览短剧目录、搜索，并取得剧集的播放地址。

所有请求都以匿名设备的身份发出，不需要登录。两个平台的核心都是用 gomobile 编译的 Go SDK，所以行为和 Go 版、Node.js 版完全一致。

> 本项目与红果短剧及其运营方没有任何关联，也没有获得授权或认可。“红果”和“Hongguo”是权利人的商标，这里只用来说明兼容对象。

## 快速上手

下面的代码搜索一部剧，并取得第 1 集的播放地址。

Android（Kotlin）：

```kotlin
import dev.openhongguo.OpenhongguoClient

val client = OpenhongguoClient.create(context)
val page = client.search("全球死考")
val episodes = client.listEpisodes(page.items.first().id)
val result = client.resolvePlayback(episodes.items.first().id)
result.playback.streams.forEach { println("${it.definition} ${it.url}") }
```

iOS（Swift）：

```swift
import Openhongguo

let client = try OpenhongguoClient.create()
let page = try await client.search("全球死考")
let episodes = try await client.listEpisodes(page.items[0].id)
let result = try await client.resolvePlayback(episodes.items[0].id)
for s in result.playback.streams { print(s.definition, s.url) }
```

首次请求时，SDK 自动注册一台匿名设备，并保存下来，以后的请求都用这台设备。

## 构建

两个平台的库都还没有发布，需要从源码构建。构建需要 Go 1.26 或更高版本（gomobile 的要求），以及 gomobile。

```sh
go install golang.org/x/mobile/cmd/gomobile@latest
gomobile init
```

然后在本目录下运行 `build.sh`。

```sh
./build.sh android   # 生成 android/build/outputs/aar/openhongguo-release.aar
./build.sh ios       # 生成 ios/OpenhongguoCore.xcframework
./build.sh           # 两者都构建
```

Android 还需要以下环境：

- 环境变量 `ANDROID_HOME` 指向 Android SDK；
- 装有 Android NDK，脚本默认使用 `$ANDROID_HOME/ndk` 下最新的版本，也可以用 `ANDROID_NDK_HOME` 指定；
- JDK 17 或更高版本，比如 Android Studio 自带的 JBR。

iOS 需要 Xcode。

## 安装

### Android

把 `openhongguo-release.aar` 复制到 App 模块的 `libs/` 目录，再加上依赖。

```kotlin
dependencies {
    implementation(files("libs/openhongguo-release.aar"))
    implementation("org.jetbrains.kotlinx:kotlinx-coroutines-core:1.10.2")
}
```

本地 aar 不会带上它的依赖，所以要自己加上 kotlinx-coroutines。

最低支持 Android 5.0（API 21）。aar 已经声明了网络权限，也带有混淆规则，不需要额外配置。

### iOS

在 Xcode 里选择 File > Add Package Dependencies，然后点 Add Local，选中 `ios/` 目录。或者在 `Package.swift` 里用路径引用。

```swift
.package(path: "../openhongguo/bindings/mobile/ios")
```

最低支持 iOS 13 和 macOS 11。

## 接口

两个平台的方法和 Node.js 版一一对应。Kotlin 的方法是挂起函数，Swift 的方法是 `async throws`。

| 方法 | 作用 |
|---|---|
| `discover(options)` | 书城频道首页，默认是“推荐” |
| `search(query, options)` | 搜索短剧 |
| `suggest(prefix)` | 搜索联想词 |
| `hotSearch()` | 搜索页的“猜你想搜”和热搜榜 |
| `searchCues()` | 搜索框里滚动的提示词 |
| `newReleases(query)` | 上新日历 |
| `getSeries(seriesId)` | 一部剧的详情 |
| `getSeriesBatch(seriesIds)` | 一次读取多部剧及其各集 |
| `listEpisodes(seriesId, options)` | 一部剧的分集列表 |
| `seasons(seriesId)` | 多季剧的全部季 |
| `seriesRecommend(seriesId)` | 一部剧的相关推荐 |
| `getAlbum(albumId)` | 一个合集及其各季 |
| `shortRank(query)` | 短剧排行榜 |
| `shortFilterPanel(genre)` | 短剧筛选面板 |
| `shortFilter(query)` | 按条件筛选短剧 |
| `ranking(activityId, rankListId)` | 活动榜单 |
| `rankRows(cellId, offset)` | 书城榜单 |
| `rankGroups()` | 书城榜单的类型 |
| `person(userId)` | 演员或角色主页 |
| `personWorks(userId, cursor)` | 演员的作品 |
| `categories(categoryId)` | 书籍分类的标签页 |
| `categoryLanding(categoryId, options)` | 书籍分类下的列表 |
| `resolvePlayback(episodeId)` | 一集的播放模型 |
| `resetDevice()` | 丢弃当前匿名设备，下次请求时重新注册 |

同一个客户端可以在多个线程里同时使用。

### 类型

返回值的类型在 `Types.kt` 和 `Types.swift` 里，由 Go 源码生成，字段说明和 Go SDK 的文档一致。

- **属性名**：JSON 字段名转成驼峰，比如 `cover_url` 变成 `coverUrl`，两个平台相同。
- **缺省值**：字段缺省时取零值（空字符串、0、空列表），不用可空类型，和 Go 的规则一致。只有 Go 里的指针字段可以为空，比如 `Series.novel`。
- **枚举**：`ContentType`、`SeriesStatus` 等枚举是包着整数的值类型，已知的取值是它的常量，比如 Kotlin 的 `ContentType.SHORT_PLAY`、Swift 的 `.shortPlay`。服务端新增的取值也能原样保留。
- **查询参数**：`SearchOptions` 等查询参数的字段全部可空，为空表示不传。

Swift 的 `Stream` 类型和 Foundation 的 `Stream` 重名。同时导入两者时，写成 `Openhongguo.Stream`。

### 翻页

带游标的列表，可以用 `discoverAll` 和 `searchAll` 逐部遍历。服务端会在不同页里重复返回同一部剧，这两个方法会自动去掉重复。Kotlin 返回 `Flow`，Swift 返回 `AsyncThrowingStream`。

```kotlin
client.searchAll("末世", maxPages = 3).collect { println("${it.id} ${it.title}") }
```

```swift
for try await series in client.searchAll("末世", maxPages: 3) {
    print(series.id, series.title)
}
```

### 加密剧集

`resolvePlayback` 返回播放模型和内容密钥。对于加密剧集（`playback.protected` 为真），`contentKey.key` 是内容密钥，加密方式为 cenc-aes-ctr。

密钥推导失败时，`contentKey` 为空，`contentKeyError` 会说明原因，播放模型照常返回。

### 取消

Kotlin 的协程或 Swift 的 Task 被取消时，进行中的请求随之取消。此时 Kotlin 抛出 `CancellationException`，Swift 抛出 `CancellationError`。

## 设置

`create` 的第二个参数是 `ClientOptions`，所有设置都是可选的。

| 设置 | 作用 |
|---|---|
| `apiOrigin`、`registerOrigin`、`distributionOrigin` | 覆盖接口地址，默认和 App 一致 |
| `retryAttempts` | 可重试的失败最多尝试几次，默认 3 |
| `device` | 新注册设备自称的机型，为空的字段取默认值（小米 14） |
| `logger` | 接收诊断日志的函数，日志里的 URL 已经脱敏 |

### 设备档案

设备档案的默认位置如下。两个位置都不参与系统备份，这样设备档案不会随备份迁移到别的手机上。

- Android：`noBackupFilesDir/openhongguo/anonymous.json`。
- iOS：`Application Support/openhongguo/anonymous.json`，目录标记为不备份。

也可以指定一个文件：Kotlin 用 `OpenhongguoClient.create(file, options)`，Swift 用 `OpenhongguoClient.create(profile: url, options: options)`。

## 错误处理

Kotlin 抛出 `OpenhongguoException`，Swift 抛出 `OpenhongguoError`，都带有三个属性。

- `code`：错误码，比如 `RATE_LIMITED`（Swift 是 `.rateLimited`）。
- `retryable`：原样重试是否可能成功。
- `retryAfterMs`：服务端建议的等待时间，单位毫秒；0 表示没有建议。

```kotlin
try {
    client.getSeries(seriesId)
} catch (e: OpenhongguoException) {
    if (e.code == ErrorCode.CONTENT_UNAVAILABLE) println("这部剧已下架")
}
```

```swift
do {
    _ = try await client.getSeries(seriesId)
} catch let e as OpenhongguoError where e.code == .contentUnavailable {
    print("这部剧已下架")
}
```

## 实现原理

- `core/`：gomobile 编译的 Go 包，是一个单独的 Go 模块，因为 gomobile 要求模块依赖 `golang.org/x/mobile`。它只导出一个按名字调用的入口：参数和结果都是 JSON，结果通过回调返回，不占用调用方的线程。
- `android/`：Kotlin 封装，Gradle 库工程。
- `ios/`：Swift 封装，Swift 包。
- `gen/`：根据 Go 源码生成 `Types.kt` 和 `Types.swift`。
- `fixtureserver/`：在本机回放 `testdata/protocol` 的样本，供测试使用。

请求由 Go 的 `net/http` 直接发送，设备档案由 Go 读写。三种语言的绑定共用 `bindings/internal/dispatch` 的方法表，所以新增 SDK 方法时，只需要在那里登记一次，再在各平台的客户端里加上对应的方法。

库的体积主要是 Go 运行时。Android 每种 CPU 架构约 8 MB，aar 共约 13 MB；App 按架构拆分 APK 后，每个 APK 只增加一份。iOS 真机部分约 9 MB。

## 开发

修改 `sdk` 或 `sdk/model` 的类型后，在仓库根目录重新生成类型。

```sh
go run ./bindings/mobile/gen
```

仓库根目录的 `go test ./...` 会检查生成的文件是否过时。

各部分的测试命令如下。

```sh
(cd core && go test ./...)               # Go 入口，回放样本
./build.sh android                       # 同时运行 Kotlin 单元测试
(cd ios && swift test)                   # 在 macOS 上运行，回放样本
```

Kotlin 的设备测试需要一台模拟器，并在电脑上运行样本服务。

```sh
go run ./bindings/mobile/fixtureserver -addr 127.0.0.1:8790   # 在仓库根目录
(cd android && ./gradlew connectedDebugAndroidTest)
```

## 许可证

本库采用 Apache License 2.0 许可证，见仓库根目录的 `LICENSE`。gomobile 生成的代码来自 Go 项目，采用 BSD 许可证。
