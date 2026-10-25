# openhongguo-react-native

这是红果短剧（Hongguo）非官方 SDK 的 React Native 版本。它能浏览短剧目录、搜索，并取得剧集的播放地址。

所有请求都以匿名设备的身份发出，不需要登录。原生层是 gomobile 编译的 Go SDK，通过 Expo Modules 接入，所以行为和 Go 版完全一致，接口和 Node.js 版相同。

> 本项目与红果短剧及其运营方没有任何关联，也没有获得授权或认可。“红果”和“Hongguo”是权利人的商标，这里只用来说明兼容对象。

## 快速上手

下面的代码搜索一部剧，并取得第 1 集的播放地址。

```ts
import { createClient } from 'openhongguo-react-native';

const client = await createClient();
const page = await client.search('全球死考');
const episodes = await client.listEpisodes(page.items[0].id);
const { playback } = await client.resolvePlayback(episodes.items[0].id);
console.log(playback.streams.map((s) => `${s.definition} ${s.url}`));
```

首次请求时，SDK 自动注册一台匿名设备，并保存下来，以后的请求都用这台设备。

## 要求

- **Expo Modules**：Expo 应用可以直接使用。不用 Expo 的 React Native 应用，先运行 `npx install-expo-modules` 接入 Expo Modules。
- **自定义原生代码**：包里带有原生代码，不能在 Expo Go 里运行，需要用 `npx expo run:android` 或开发构建（development build）。
- **系统版本**：Android 5.0（API 21）及以上，iOS 15.1 及以上。

## 构建与安装

这个包还没有发布到 npm，需要从源码构建。构建需要 Go 1.26 或更高版本和 gomobile，环境要求见 [bindings/mobile/README.md](../mobile/README.md#构建)。

```sh
../mobile/build.sh react-native   # 编译 Go core，放进 android/ 和 ios/
npm install
npm pack
```

`npm pack` 生成 `openhongguo-react-native-0.1.0.tgz`，在你的 App 里安装它。

```sh
npm install /path/to/openhongguo-react-native-0.1.0.tgz
```

安装后重新生成原生工程（`npx expo prebuild`），或者直接运行 `npx expo run:android`。

包的体积主要是 Go 运行时：Android 每种 CPU 架构约 8 MB，iOS 真机部分约 9 MB。Android 应用按架构拆分 APK（比如 `-PreactNativeArchitectures=arm64-v8a`）后，只会带上一份。

## 接口

接口与 Node.js 版 `openhongguo` 完全相同，见 [bindings/node/README.md](../node/README.md#接口)。所有方法都返回 Promise，失败时抛出 `OpenhongguoError`；翻页用 `discoverAll` 和 `searchAll`（`for await`）。返回值的字段名是下划线风格，比如 `has_more`、`cover_url`，与 Go SDK 的 JSON 一致。

和 Node.js 版不同的地方如下。

| 设置或方法 | 说明 |
|---|---|
| `profilePath` | 设备档案的路径，替代 Node.js 版的 `profile`。默认在不参与系统备份的目录里：Android 是 `noBackupFilesDir`，iOS 是标记为不备份的 Application Support。这样设备档案不会随备份迁移到别的手机 |
| `log` | 接收诊断日志的函数 |
| `client.release()` | 释放原生端的客户端，之后的调用都会失败 |

不支持 Node.js 版的 `transport` 和 `timeoutMs`：请求由 Go 的 `net/http` 直接发送，单个请求超时 30 秒。

## 开发

```sh
npm install   # 只安装 typescript
npm test      # 编译到 build/，再用假的原生模块测试客户端
```

`src/types.ts` 由 `go run ./bindings/node/tsgen` 生成，与 Node.js 版的 `types.d.ts` 相同，请勿手工修改。仓库根目录的 `go test ./...` 会检查它是否过时。

## 许可证

本包采用 Apache License 2.0 许可证，见仓库根目录的 `LICENSE`。gomobile 生成的代码来自 Go 项目，采用 BSD 许可证。
