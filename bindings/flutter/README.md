# openhongguo（Flutter）

这是红果短剧（Hongguo）非官方 SDK 的 Flutter 版本。它能浏览短剧目录、搜索，并取得剧集的播放地址。

所有请求都以匿名设备的身份发出，不需要登录。原生层是 gomobile 编译的 Go SDK，通过平台通道调用，所以行为和 Go 版完全一致。

> 本项目与红果短剧及其运营方没有任何关联，也没有获得授权或认可。“红果”和“Hongguo”是权利人的商标，这里只用来说明兼容对象。

## 快速上手

下面的代码搜索一部剧，并取得第 1 集的播放地址。

```dart
import 'package:openhongguo/openhongguo.dart';

final client = await OpenhongguoClient.create();
final page = await client.search('全球死考');
final episodes = await client.listEpisodes(page.items.first.id);
final result = await client.resolvePlayback(episodes.items.first.id);
for (final s in result.playback.streams) {
  print('${s.definition} ${s.url}');
}
```

首次请求时，SDK 自动注册一台匿名设备，并保存下来，以后的请求都用这台设备。

## 要求

支持 Android 7.0（API 24）及以上和 iOS 15 及以上，与 Flutter 自身的要求一致。不支持 Web 和桌面平台。

## 构建与安装

这个包还没有发布到 pub.dev，需要从源码构建。构建需要 Go 1.26 或更高版本和 gomobile，环境要求见 [bindings/mobile/README.md](../mobile/README.md#构建)。

```sh
../mobile/build.sh flutter   # 编译 Go core，放进 android/ 和 ios/
```

然后在 App 的 `pubspec.yaml` 里用路径引用。

```yaml
dependencies:
  openhongguo:
    path: ../openhongguo/bindings/flutter
```

包的体积主要是 Go 运行时：Android 每种 CPU 架构约 8 MB，iOS 真机部分约 9 MB。`flutter build apk --split-per-abi` 按架构拆分 APK 后，每个 APK 只带一份。

## 接口

方法和其他语言的版本一一对应，都返回 `Future`，失败时抛出 `OpenhongguoException`。

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
| `release()` | 释放原生端的客户端 |

### 类型

返回值的类型由 Go 源码生成，字段说明和 Go SDK 的文档一致。

- **属性名**：JSON 字段名转成驼峰，比如 `cover_url` 变成 `coverUrl`，和 Kotlin、Swift 版相同。
- **缺省值**：字段缺省时取零值（空字符串、0、空列表），不用可空类型，和 Go 的规则一致。只有 Go 里的指针字段可以为 null，比如 `Series.novel`。
- **枚举**：`ContentType`、`SeriesStatus` 等枚举是包着整数的类，已知的取值是它的静态常量，比如 `ContentType.shortPlay`。服务端新增的取值也能原样保留。和关键字重名的常量加上 `Value` 后缀，比如 `RankChangeKind.newValue`。
- **查询参数**：`SearchOptions` 等查询参数的字段全部可空，为空表示不传；`copyWith` 可以改动其中几项。
- **改名**：为了不和 Dart、Flutter 自带的类型重名，有两个类型换了名字。分页结果 Page 叫 `PagedResult`，以免和 Flutter widgets 的 `Page` 冲突；流 Stream 叫 `VideoStream`，以免和 `dart:core` 的 `Stream` 冲突。

### 翻页

带游标的列表，可以用 `discoverAll` 和 `searchAll` 逐部遍历，返回 `Stream`。服务端会在不同页里重复返回同一部剧，这两个方法会自动去掉重复。

```dart
await for (final series in client.searchAll('末世', const SearchOptions(), 3)) {
  print('${series.id} ${series.title}');
}
```

### 加密剧集

`resolvePlayback` 返回播放模型和内容密钥。对于加密剧集（`playback.protected` 为 true），`contentKey.key` 是内容密钥，加密方式为 cenc-aes-ctr。

密钥推导失败时，`contentKey` 为 null，`contentKeyError` 会说明原因，播放模型照常返回。

## 设置

`OpenhongguoClient.create` 的参数是 `ClientOptions`，所有设置都是可选的。

| 设置 | 作用 |
|---|---|
| `profilePath` | 设备档案的路径。默认在不参与系统备份的目录里：Android 是 `noBackupFilesDir`，iOS 是标记为不备份的 Application Support |
| `apiOrigin`、`registerOrigin`、`distributionOrigin` | 覆盖接口地址，默认和 App 一致 |
| `retryAttempts` | 可重试的失败最多尝试几次，默认 3 |
| `device` | 新注册设备自称的机型，为空的字段取默认值（小米 14） |
| `log` | 接收诊断日志的函数，日志里的 URL 已经脱敏 |

## 错误处理

所有错误都是 `OpenhongguoException`，带有三个属性。

- `code`：错误码，比如 `ErrorCode.rateLimited`。
- `retryable`：原样重试是否可能成功。
- `retryAfterMs`：服务端建议的等待时间，单位毫秒；0 表示没有建议。

```dart
try {
  await client.getSeries(seriesId);
} on OpenhongguoException catch (e) {
  if (e.code == ErrorCode.contentUnavailable) print('这部剧已下架');
}
```

## 开发

`lib/src/types.dart` 由 `go run ./bindings/mobile/gen` 生成，请勿手工修改。仓库根目录的 `go test ./...` 会检查它是否过时。

```sh
flutter test   # 用模拟的平台通道测试客户端
```

## 许可证

本包采用 Apache License 2.0 许可证，见仓库根目录的 `LICENSE`。gomobile 生成的代码来自 Go 项目，采用 BSD 许可证。
