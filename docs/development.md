# 开发

这篇文档介绍 openhongguo 怎样保证兼容性：每日的接口契约检查，以及每次推送时运行的持续集成。

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

## 持续集成

`.github/workflows/ci.yml` 在每次推送和 pull request 时运行，不访问网络。各平台的绑定都用 `testdata/protocol` 的样本回放。

- Go：格式检查、`go vet` 和全部测试，同时检查生成的类型文件是否过时。
- Node.js：在 Node 20 和 22 上构建 wasm 并运行测试。
- Android：编译 aar，运行 Kotlin 单元测试，再在模拟器上运行设备测试。
- iOS：编译 xcframework，在 macOS 上运行 Swift 测试，并为 iOS 真机和模拟器编译。

只改动文档时不运行。
