# OpenHuangguoCore 设计

日期：2026-11-16
状态：已纳入两轮 codex review（REQUEST CHANGES → 复核）。第二轮 13 条中 8 条已解决、其余在本版逐条收紧，见文末"复核处理记录"。

## 背景与目标

在 openhongguo 仓库内新增一个**独立**的黄果 SDK 及移动端绑定，命名为 OpenHuangguoCore。参考黄果客户端 HGhub（`AmatsuZero/study-materials-fork`）的接口，仿照现有 hongguo SDK 的形态实现。Go 源码、构建、二进制 release 全部在独立仓库 `AmatsuZero/openhuangguo-swift`（含标准 `Package.swift` 与 Swift 壳）。

两个硬约束：

1. **不动 hongguo**。上游 `beilunyang/openhongguo` 会定期更新，hongguo 的代码、产物、发版线、脚本保持原样，改动只通过 import 复用、不改源文件。**不提交根 `go.work`**，不改 `fix_xcf_binarytarget.py` / `release.yml` / `xcf.yml`。
2. **OpenHuangguoCore 与 OpenHongguoCore 完全独立**。独立的 xcframework/AAR、构建脚本、标签前缀、release **与 Swift 分发仓库**，互不回填、互不覆盖、不共用版本号，也不触发源仓库的 `release.published`。

## 范围

### 做

- Go SDK：HGhub **全部业务接口**，以强类型 model 暴露（codex 核对：无遗漏无多余）。
- gomobile Core（OpenHuangguoCore）：按名调用的单一入口，先打通 Swift / Kotlin。
- 纯函数式媒体工具：封面 AES 解密、HLS 播放列表改写、分片/KEY 拉取（首版行为矩阵见媒体节）。
- domain / proxy 设置能力，含凭据目标信任策略。

### 不做

- 应用层命令：线路发现与探测、更新检查、debug 日志、屏幕旋转。
- Node / Flutter / React-Native 绑定（后续再加）。
- 严格网络守卫（DNS pinning、逐跳重检）放进 Core——留给上层；transport 自身仍有明确的重定向策略。
- 有状态本地 HTTP 媒体代理 server（宿主 App 承担播放器资源加载层）。

## HGhub 接口事实

- 全部业务接口收敛到单一信封：`POST /api/edge/request`，body 为 `{method, path, body(base64 JSON), content_type}`。
  - 无 body：`body` 与 `content_type` 均为空字符串（`net.rs:486-497`）。
  - 有 body：标准 Base64(JSON)、`application/json`。
- 响应成功判定的**精确**语义（`net.rs:578-613`）：
  - HTTP 成功且响应体 trim 后为空 → 成功，返回 null（`Ok(Value::Null)`）。
  - 响应体是 JSON `null` → 成功（`value.is_null()`）。
  - 其余非空响应必须是对象且 `status == 1` → 取 `data`（`data` 可以是 `{}` 或 null）；**外层裸 `{}` 因无 status 视为失败**——不要把它和成功信封里的 `data: {}` 混淆。
  - `status != 1` → 取非空 `msg` 报错，否则报"接口返回失败"。
- 登录态：`Authorization: Bearer <token>`；无 hongguo 的 SM3/AES/ProtoBuf 签名。
- 业务接口清单（约 20 个，对应 `api.rs` 的 20 个业务 command）：
  - 账号：`loginByPassword` / `registerByPassword` / `logout` / `me`
  - 列表：`/api/videos`、`/api/videos/category/{cate}`、`/api/ranks/{kind}`、`/api/search`、`/api/posts`、`/api/posts/{id}`
  - 详情/播放：`/api/videos/{id}`、`/api/videos/{id}/episodes/{ep}/playback`（新）、`/api/videos/{id}/play`（旧，兜底）
  - 互动：video comments / comment send、video like / collect、user follow、post like / collect / comments
- 数据形状：`src/types.ts`（VideoCard / Paged / Detail / PlayInfo / RankItem / Comment / Post / User 等）。`Paged<T>` 是泛型，`Detail.counts/author`、`PlayInfo.streams` 是匿名嵌套结构——Go 侧定义为具名类型（见 schema 节）。
- 媒体：封面 AES-128-CBC（key/IV 硬编码于 `media.rs`）、HLS URI 改写、分片/KEY 本地代理。
- 线路：固定 `https://huangguoai.com`、`https://huangguoai.ai`，镜像发布页 `huangguoai.pages.dev`。
- 重试语义（对齐 `net.rs:384-389, 515-516`）：仅无 body 的逻辑 GET（**可带 token**）、以及取播放地址的 POST 可重试；可重试状态含 408/425/429/5xx；401/403/4xx 其余不重试；登录/注册与所有写操作禁止盲重试。

## 架构与 workspace 布局

**不提交根 `go.work`**。workspace 的 Go 1.26 要求会作用于原按 Go 1.22 运行的任务、统一依赖版本、漏纳现有 `bindings/mobile/core`——都触碰 hongguo。黄果模块用**自己的依赖声明 + 局部 `replace`** 开发；本地临时 go.work 仅个人开发用，不入库、不进 CI。

```
AmatsuZero/openhuangguo-swift/            # 独立分发仓库（Go 源码 + Swift 壳 + 二进制 release）
├── Package.swift                         # 标准根包，独立版本标签
├── Sources/OpenHuangguo/                 # Swift 壳源码 + 播放器资源加载层
├── go.mod                                # github.com/.../openhuangguo-swift（独立 module）
├── bindings/huangguo/
│   ├── sdk/                              # module <独立前缀>/sdk
│   │   ├── client.go edge.go auth.go transport.go media.go store.go
│   │   └── model/                        # 具名类型
│   ├── core/                             # module <独立前缀>/core → OpenHuangguoCore
│   │   ├── core.go dispatch/
│   │   └── go.mod                        # tool gobind + 独立 go.sum
│   ├── gen-huangguo/                     # 独立生成器（schema 依赖仅在此）
│   ├── ios/ android/                     # 移动端壳
│   └── build.sh
└── .github/workflows/hg-xcf.yml          # 构建并在本独立仓库发布二进制 release

openhongguo（源仓库）：仅保留 bindings/huangguo 的 Go 源码镜像/或迁入独立仓库后删除，不改任何 hongguo 文件。
```

**Module path 用独立前缀**（codex #11，已定）：模块放独立仓库 `github.com/AmatsuZero/openhuangguo-swift`，sdk 与 core 分别为 `.../bindings/huangguo/sdk`、`.../bindings/huangguo/core`。**不复用 hongguo 的 `bindings/internal/schema`**——把 schema 源码复制进 `gen-huangguo` 并改造（见绑定层），因此不受 `github.com/beilunyang/openhongguo/...` 前缀约束，仓库/模块彻底独立。这是唯一方案，不再二选一。

依赖与 replace 归属（写在使用方 module 的 go.mod，下游不继承）：

- `sdk`：运行时**不依赖 hongguo 模块**，无根模块 replace。
- `core`：require `sdk` + 本地 replace；自带 `tool golang.org/x/mobile/cmd/gobind` 与独立 `go.sum`。
- `gen-huangguo`：唯一允许依赖/复制 hongguo `bindings/internal/schema` 的地方（构建期依赖，不进 SDK 运行时）。
- 首版**不支持外部 `go get` SDK**；消费方式是通过绑定产物（xcframework/AAR）。若日后要支持，再设计嵌套模块标签 + 外部最小消费者验证。

## SDK 内核（`bindings/huangguo/sdk`）

- **client.go**：`Client` 持有 `transport` / `tokenStore` / `base` / `credTrustOrigins` / `clock` / `logger`。`NewClient(opts)` 校验依赖、定默认值（base = `https://huangguoai.com`），不发请求。业务方法统一 `ctx` 打头。重试循环、退避抖动、`sleepContext` 照搬 hongguo `client.go:122-174`，判定按 HGhub。
- **edge.go**：唯一线上出口。封 `{method,path,body,content_type}` POST 到 `{base}/api/edge/request`。响应按上面"精确语义"分别处理 HTTP 状态、业务 status、空串/null/裸 `{}`/`data:{}`。**edge 首版禁止自动重定向**（`CheckRedirect` 返回 `http.ErrUseLastResponse`）；游客迁移若日后支持，显式有限跳转 + 重新封装。
- **auth.go / store.go**：分两层，避免把内部接口与 gomobile 可绑定签名混为一谈（codex #13）：
  - **内部 `TokenStore` 接口**（Go SDK 内部用，返回值不受 gomobile 限制）：`Load() (Session, error)`，其中 `Session{Origin, Token string; Generation int64}`；`Save(Session) error`；`Clear() error`。
  - **gomobile 绑定边界**：Bindings 不直接暴露 `TokenStore` 接口（gomobile 绑定方法不支持多返回值，且 Go interface 由宿主实现有边界）。改为在 `core` 层提供一个**可绑定的 token 桥接类型**：`SetTokenJSON(json string) error` / `GetTokenJSON() (string, error)`（单返回值 + error，gomobile 可绑定），JSON 承载 `{origin,token,generation}`；原生层用它把 Keychain/Keystore 读写桥接到内部 store。宿主实现须**线程安全**（任意 Go goroutine 可调用）。
  - **origin 绑定**：拒绝以另一 base 重启时误用旧 token（HGhub `session.rs:8-15,76-78`）。
  - **generation**：切 base / 登出即递增，旧 generation 的登录响应禁止提交（对齐 `api.rs:335-338`）。请求原子读取 origin/token 快照。
  - **失败语义（明确跨重启保证的边界）**：`Clear` 无论持久化成功与否，立即清空内存 token（当前进程不再使用），失败返回错误供记录；登出/切域名不因 Clear 失败回滚。**若 Clear 持久化失败，磁盘仍存旧 token+旧 generation，重启会一并加载——故首版明确：持久化失败时不保证跨重启的退出生效**，须由宿主把 Clear 失败视为必须重试/上报的硬错误（移动端 Keychain 实现删除失败即报错，绝不静默成功）。不能声称"靠 origin+generation 兜底拒绝重启后的旧 token"——那只在 Clear 持久化成功时成立。
  - **文件实现**：权限 0600、tmp+rename 原子写、明确错误传播；Clear 通过写入"已撤销"标记或删除文件实现，删除失败必须返回错误。移动端默认内存 store，Keychain/Keystore 接入走上面的 token 桥接。
  - SetBase 入口在 SDK、dispatch、Swift/Kotlin 壳三层都定义清楚。
- **凭据目标信任策略**：base 规范化为 HTTPS + 纯 origin（无 userinfo/path/query/fragment）；默认固定**凭据信任名单**（`huangguoai.com` / `huangguoai.ai`）。镜像仅供游客浏览，不得向其发送登录密码或 Bearer。token 由 edge 显式注入，**媒体请求不自动带鉴权头**。
- **model/**：照 `src/types.ts` 建强类型，JSON tag 对齐 TS。`Paged<T>` 实例化为具名类型（`VideoCardPage`/`CommentPage`/`PostPage`），`Detail.counts/author`、`PlayInfo.streams` 为具名 struct——满足 AST 解析与生成。
- **transport.go**：自写轻量 transport（**不能复用 `httptransport`**，其拒绝 `Authorization`，`transport.go:44-48`）。支持：
  - **代理优先级（明确选择）**：显式 `ProxyURL` > `HGHUB_PROXY` > `HTTPS_PROXY`/`ALL_PROXY`/`HTTP_PROXY`；环境变量回退遵循 `NO_PROXY`，**显式代理不受 NO_PROXY 影响**（显式即强制）；`ProxyURL` 设空字符串表示强制直连（禁用环境回退）；首版只支持 HTTP/HTTPS 代理，不支持 SOCKS。
  - 标准库 `http.ProxyFromEnvironment` 只认 `HTTP_PROXY`/`HTTPS_PROXY`/`NO_PROXY`（及小写），不含 `HGHUB_PROXY`/`ALL_PROXY`（codex #9）——额外变量自行读取。
  - **目标 URL 与代理 URL 分开校验**：本地代理地址、代理认证不被目标"禁内网/禁 userinfo"规则误拒；错误脱敏代理凭据。
  - 轻守卫：拒目标 URL 内 user:pass、禁内网目标；API 响应沿用 8MB 上限，媒体资源独立上限。错误分类照搬 `httptransport`。
  - **媒体跳转策略**：媒体请求最多跟随 5 跳，每跳重新做目标检查，禁止 HTTPS→HTTP 降级；任一跳目标检查失败即终止。

## 媒体工具（纯函数式，放 sdk）

Go Core（xcframework/AAR 内的部分）只提供**无状态纯函数**；**播放器资源加载层是宿主 App 的职责，落在 `bindings/huangguo/{ios,android}` 壳里**（不在 Go Core 内，也不在独立 Swift 分发仓库的纯 Core 定义内）——职责边界至此明确。先用真实播放器完成"主清单→子清单→KEY→分片→seek"最小验证，再确认长期不需要本地 server。

- `DecryptCover([]byte) ([]byte, error)`：AES-128-CBC，key/IV 为 `media.rs:26-27` 硬编码常量；明文图片直接透传（`media.rs:106-118`），仅密文解密；条件去 padding（`media.rs:77-84`，无效 padding 原样保留）。
- `RewritePlaylist(upstreamURL, localScheme, host, text string) string`：显式保留**上游清单完整 URL**作为所有相对 URI 的解析基准（解决 codex #7 的基准不清）。URI 改写为可逆的本地协议映射（上游绝对 URL 放 query，如 `?u=`，宿主据此还原）。首版行为矩阵对齐 `media.rs`：BOM、带逗号的引号属性、`CONTENT-STEERING` 删除、`DATERANGE` 外部资源清理、KEY/MAP/MEDIA/SESSION-KEY/PART/PRELOAD-HINT 与分片行改写、**子清单再次识别并改写**（`media.rs:315-330`）。
- `FetchSegment(ctx, url string, reqHeader map[string]string) (SegmentResult, error)`：接受 Range 等请求头（用 `map[string]string` 而非 `http.Header`，便于绑定序列化）；返回结构 `SegmentResult{Status int; Headers map[string]string; Body []byte}`——**Headers 字段承载 Content-Range / Accept-Ranges / Content-Length 等响应头**，宿主资源加载层据此应答播放器（含 206 语义）。`map[string]string` 与结构体字段都是 gomobile 可序列化类型。独立大小上限，支持取消。Range 的宿主↔Core 契约即此签名与结构。

## 绑定层

- **core/core.go**：照搬 `bindings/mobile/core/core.go:37-102` 的 gomobile 模板——`Client.Start`、`Call.Cancel`、`envelope/jsonError/logf`。**构造器不能原样搬**（耦合 `adapters.NewClient/profilePath`）：huangguo 版直接接收 `TokenStore` 与 config，不走 `adapters.NewClient`。configJSON 带 `base`/`proxy`/`credTrustOrigins`；TokenStore 由原生层注入（签名见上）。
- **core/dispatch/**：为 huangguo 重写方法表（约 20 个方法），照搬 `dispatch.go:198-280` 的参数解析、error envelope、`ErrorCodes` 结构；错误码换 huangguo 哨兵错误；补 `setBase`。
- **gen-huangguo/**（codex #2）：**不能零改动复用** schema——`schema.go:23-26` 硬编码 hongguo `QueryTypes` 且 `:133` 要求全存在，跳过泛型、不支持匿名结构体字段。方案：把 schema 作为**构建期依赖复制进 gen-huangguo**（不进 SDK 运行时），设黄果 `QueryTypes`，分页实例/嵌套对象用具名 Go 类型；不改 hongguo 源文件。最小 schema→Swift/Kotlin 生成验证作为门禁。
- **移动端壳**：`bindings/huangguo/ios`（SwiftPM + `Client.swift` 封装 callback→async + **播放器资源加载层**：把 Core 的 URI 映射/Range/解密接到 AVPlayer）、`bindings/huangguo/android`（Gradle + `Client.kt`）。壳与独立 Swift 分发仓库的关系：壳源码复制到 `openhuangguo-swift/Sources/OpenHuangguo` 随标准 `Package.swift` 发布。

## 构建与发版隔离

- **独立构建脚本** `bindings/huangguo/build.sh`：复制 `build.sh` 改造，产物改名 `OpenHuangguoCore.xcframework` / `huangguo-release.aar`、javapkg `dev.openhuangguo`，只保留 android + ios。本地不做 xcframework 编译/打包/发布——走 `hg-xcf.yml`。
- **二进制 release 发到独立仓库**（codex #12）：`.github/workflows/hg-xcf.yml`（在独立仓库 `openhuangguo-swift` 内运行）由 `hg-xcf-v*` 标签触发，编译 → macOS slice 校验 → zip+checksum → **用独立仓库的 GITHUB_TOKEN 在本仓库发布 `OpenHuangguoCore` release**。源仓库的 `release.yml` 监听的是源仓库的 `release.published`，独立仓库的 release 事件不会触发它——这是"不串线"的具体机制。**黄果二进制 release 与 Swift 包在同仓库**，根 `Package.swift` 指向本仓库 release asset。
- **不回填、不 push 源仓库任何文件**：避开 `xcf.yml:114` 的 `git checkout -B main origin/main` 与 `fix_xcf_binarytarget.py` 的全局 checksum 替换。
- **CI**：独立仓库用自己的 workflow 跑黄果 Go 检查（gofmt/vet/test）、生成一致性、gomobile 双端构建、壳编译/取消测试、播放器验证。源仓库 `ci.yml` 的 `git ls-files '*.go'` 门禁若仍有黄果源码在源仓库会扫到（只是格式覆盖扩大，非隔离故障）；源码全部迁出源仓库则无此问题。

## 复用矩阵

| 档位 | 模块 | 复用方式 |
|---|---|---|
| 构建期复用 | hongguo `bindings/internal/schema` | 复制进 gen-huangguo 并设黄果 QueryTypes；**不进 SDK 运行时依赖**；module path 因此可独立 |
| 改造复用 | 工程骨架 | `Makefile` gen/test 目标、`build.sh` gomobile 流程、workflow 结构，复制改造；**根 go.work 不复用/不提交** |
| 改造复用 | `bindings/mobile/gen` | 逻辑通用，改输出路径 + 数据来源；只留 Swift/Kotlin |
| 改造复用 | `bindings/internal/dispatch` | 方法表重写，参数解析/envelope/ErrorCodes 照搬（`dispatch.go:198-280`） |
| 改造复用 | `bindings/mobile/core/core.go` | 异步调用/取消/JSON envelope 可照搬；**构造器不能原样搬** |
| 改造复用 | `adapters/memstore`/`filestore` | 存储思路借用，改成带 origin/generation 的 TokenStore；文件实现 0600/原子写/错误传播 |
| 必须重写 | transport | 拒绝 Bearer 不能用 `httptransport`；响应读取/限制/脱敏照搬，约 100 行 |
| 必须重写 | SDK client/edge/auth | 协议不同；重试/脱敏照搬，补凭据信任/重定向/origin 绑定 |
| 必须重写 | model / 接口 | 照 `src/types.ts` 与 `api.rs` 机械翻译 |
| 跨语言复刻 | HGhub `media.rs` | Rust 无法直接复用，复刻算法与常量到 Go，含子清单递归 |

## 测试

- SDK `go test ./...`（sdk、core 各自 module）：edge 信封编码/解析（无 body / 有 body / 空串 / null / 裸 `{}` 失败 / `data:{}` 成功）、业务 status 错误、重试分类（带 token GET、408/425/429）、重定向禁用；TokenStore origin 绑定/generation/切域名清 token/Clear-Save 失败语义/文件 0600 原子写；凭据信任（默认域名放行、镜像拒收密码与 Bearer、媒体不带鉴权）；代理优先级与 NO_PROXY/SOCKS 选择、目标与代理分开校验、脱敏；媒体三函数（明文透传、条件 padding、BOM/属性/危险标签/子清单递归、Range）对齐 HGhub fixture。
- 契约：HGhub 真实响应样本 fixture 回放，验证 model 解析（对照 `src/types.ts`）。
- 生成门禁：最小 schema→Swift/Kotlin 生成验证，确认生成文件一致。
- 绑定：dispatch 方法表逐个单测；壳编译 + 取消测试 + 真实播放器链路最小验证。
- 发布门禁：黄果独立 Go 检查、生成一致性、gomobile 双端构建、Swift/Kotlin 壳编译/取消/播放器验证全绿才发版。

## 风险与开放问题

- **HGhub 接口稳定性**：上游不保证接口持续可用；model 解析容忍字段缺失（指针/零值）。
- **账号凭据安全**：默认内存 store，移动端 Keychain/Keystore；文件 0600/原子写；日志与错误绝不输出 token 与代理凭据。
- **媒体宿主层**：由 App 实现，Go Core 只给纯函数；首版真实播放器验证后再定是否长期不需要本地 server。
- **Go SDK 外部消费**：首版只经绑定产物消费，不支持外部 `go get`；如需则另设嵌套模块标签与最小消费者验证。

## 复核处理记录（第二轮 codex review 13 条）

- 已解决：#1 go.work、#2 schema、#3 Swift 包入口、#4 凭据信任、#8 媒体行为矩阵。
- 本版收紧：
  - #5 补媒体重定向策略（最多 5 跳、逐跳检查、禁降级）。
  - #6 明确 Clear/Save 失败语义：generation 先递增；持久化 Clear 失败不保证跨重启退出，宿主须视为硬错误重试/上报；gomobile 可绑定 token 桥接用单返回值 JSON 签名。
  - #7 明确职责边界（纯函数在 Core、资源加载层在壳）、`RewritePlaylist` 保留上游完整 URL 作解析基准；`FetchSegment` 用 `SegmentResult{Status,Headers,Body}` 回传 Content-Range 等响应头（map[string]string + 结构体，gomobile 可序列化）。
  - #9 明确代理优先级、NO_PROXY 对显式代理、SOCKS、显式直连的**选择**。
  - #10 纠正响应语义：空串/null 成功，裸 `{}` 失败，`data:{}` 属成功信封。
  - #11 schema 依赖限定到 gen（构建期），SDK 无根模块依赖，core 不需补 replace；首版不支持外部 go get。
  - #12 明确二进制 release 也发布到独立仓库 `openhuangguo-swift`，源仓库 `release.yml` 不会收到其 `release.published`。
  - #13 内部 TokenStore 用多返回值 Go 接口，gomobile 边界用单返回值 `SetTokenJSON/GetTokenJSON` 桥接；错误/线程约定、文件 0600/原子写、发布门禁。
