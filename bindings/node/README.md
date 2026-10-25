# openhongguo（Node.js）

openhongguo 是红果短剧（Hongguo）的非官方 Node.js SDK。它能浏览短剧目录、搜索，并取得剧集的播放地址。

所有请求都以匿名设备的身份发出，不需要登录。SDK 的核心是 Go SDK 编译成的 WebAssembly，所以 Node 版和 Go 版的行为完全一致。

> 本项目与红果短剧及其运营方没有任何关联，也没有获得授权或认可。“红果”和“Hongguo”是权利人的商标，这里只用来说明兼容对象。

## 快速上手

下面的代码搜索一部剧，并取得第 1 集的播放地址。

```js
import { createClient } from 'openhongguo';

const client = await createClient();
const page = await client.search('全球死考');
const episodes = await client.listEpisodes(page.items[0].id);
const { playback } = await client.resolvePlayback(episodes.items[0].id);
console.log(playback.streams.map((s) => `${s.definition} ${s.url}`));
```

首次请求时，SDK 自动注册一台匿名设备，并保存下来，以后的请求都用这台设备。

## 安装

需要 Node.js 20 或更高版本。

这个包还没有发布到 npm，需要从源码构建。构建需要 Go 1.22 或更高版本。

```sh
cd bindings/node
npm run build
npm pack
```

`npm pack` 生成 `openhongguo-0.1.0.tgz`，在你的项目里安装它即可。

```sh
npm install /path/to/openhongguo-0.1.0.tgz
```

这个包只支持 ES 模块（`import`）。

## 接口

所有方法都返回 Promise，失败时抛出 `OpenhongguoError`。

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

各方法的参数和返回值，见包里的类型定义 `index.d.ts` 和 `types.d.ts`。类型定义由 Go 源码生成，字段说明和 Go SDK 的文档一致。

### 翻页

带游标的列表，可以用 `discoverAll` 和 `searchAll` 逐部遍历。服务端会在不同页里重复返回同一部剧，这两个方法会自动去掉重复。

```js
for await (const series of client.searchAll('末世', {}, { maxPages: 3 })) {
  console.log(series.id, series.title);
}
```

### 搜索筛选

`SearchFilter` 提供了“综合”分类的筛选项。

```js
import { SearchFilter } from 'openhongguo';

await client.search('奇幻', { filters: [SearchFilter.ShortPlay, SearchFilter.Film] });
```

可选的筛选项有：`ShortPlay`（真人剧）、`Comic`（漫剧）、`AIComic`（AI剧）、`Film`（影视剧）和 `Unwatched`（未看过）。

### 加密剧集

`resolvePlayback` 返回 `{ playback, contentKey }`。对于加密剧集（`playback.protected` 为 `true`），`contentKey.key` 是内容密钥，加密方式为 cenc-aes-ctr。

```js
const { playback, contentKey } = await client.resolvePlayback(episodeId);
if (playback.protected) {
  // 例如：ffmpeg -decryption_key <key> -i <url> -c copy out.mp4
  console.log(contentKey.key);
}
```

密钥推导失败时，没有 `contentKey`，`contentKeyError` 会说明原因，播放模型照常返回。

## 字段命名

返回值的字段名，与 Go SDK 输出的 JSON 完全相同，采用下划线风格，比如 `has_more`、`cover_url`、`episode_count`。这样它们和 CLI 的 `--json` 输出也一致。

传入的查询参数采用驼峰风格，比如 `pageSize`、`cursor`。

## 设置

`createClient(options)` 的所有设置都是可选的。

| 设置 | 作用 |
|---|---|
| `profile` | 设备档案的位置：文件路径，或自定义的存储对象 |
| `transport` | 自定义请求的发送方式 |
| `timeoutMs` | 默认发送方式的超时，单位毫秒，默认 30000 |
| `retryAttempts` | 可重试的失败最多尝试几次，默认 3 |
| `device` | 新注册设备自称的机型，未填的字段取默认值（小米 14） |
| `log` | 接收诊断日志的函数，日志里的 URL 已经脱敏 |
| `apiOrigin`、`registerOrigin`、`distributionOrigin` | 覆盖接口地址，默认和 App 一致 |

### 设备档案

设备档案默认保存在 `<用户配置目录>/openhongguo/anonymous.json`，和 CLI 共用同一台设备。macOS 上就是 `~/Library/Application Support/openhongguo/anonymous.json`。设置环境变量 `OHG_CONFIG_DIR`，可以改变这个目录。

也可以只保存在内存里，进程退出后丢弃。

```js
import { createClient, memoryProfileStore } from 'openhongguo';

const client = await createClient({ profile: memoryProfileStore() });
```

自定义存储需要实现 `load()`、`save(json)` 和 `reset()` 三个异步方法。`load()` 返回 `null`，表示还没有档案。

### 自定义请求

`transport` 接收一个请求对象，返回一个响应对象。响应体必须是原始字节，不能自动解压，因为 SDK 会根据 `Content-Encoding` 自己处理。

```js
const client = await createClient({
  transport: async ({ method, url, headers, body }) => {
    // ……发送请求……
    return { status, headers, cookies, body }; // body 是 Uint8Array
  },
});
```

只有在没有收到响应时，`transport` 才应该抛出错误。HTTP 错误状态码要正常返回，交给 SDK 判断。

## 错误处理

所有错误都是 `OpenhongguoError`，带有三个属性。

- `code`：错误码，比如 `RATE_LIMITED`、`CONTENT_UNAVAILABLE`、`DEVICE_REJECTED`。
- `retryable`：原样重试是否可能成功。
- `retryAfterMs`：服务端建议的等待时间，单位毫秒；0 表示没有建议。

```js
import { OpenhongguoError } from 'openhongguo';

try {
  await client.getSeries(seriesId);
} catch (e) {
  if (e instanceof OpenhongguoError && e.code === 'CONTENT_UNAVAILABLE') {
    console.log('这部剧已下架');
  }
}
```

全部错误码见 `index.d.ts` 的 `ErrorCode`，它们与 Go SDK 的哨兵错误一一对应。

## 实现原理

`dist/openhongguo.wasm` 是 Go SDK 编译成的 WebAssembly，桥接代码在 `bindings/node/wasm`。方法表在 `bindings/internal/dispatch`，和 Android、iOS 版共用。

- 请求由 JS 发送：Go 把请求交给 `transport` 回调，默认用 `node:https`。
- 设备档案由 JS 读写：Go 通过 `profile` 的三个方法读写 JSON。
- SDK 的每个方法都在 Go 的 goroutine 里运行，结果以 JSON 返回给 JS。

wasm 只在第一次调用 `createClient` 时加载一次，耗时约几百毫秒。同一进程里的多个客户端共用它。

## 开发

在本目录下运行以下命令。

```sh
npm run build   # 编译 wasm，复制 wasm_exec.js，重新生成 types.d.ts
npm test        # 用 testdata/protocol 的样本回放，不访问网络
```

`types.d.ts` 由 `go run ./bindings/node/tsgen` 生成，请勿手工修改。仓库根目录的 `go test ./...` 会检查它是否过时。

## 许可证

本包采用 Apache License 2.0 许可证。`dist/wasm_exec.js` 来自 Go 项目，采用 BSD 许可证，全文见 `GO_LICENSE`。
