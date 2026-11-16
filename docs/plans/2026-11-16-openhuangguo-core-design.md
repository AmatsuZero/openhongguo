# OpenHuangguoCore 设计

日期：2026-11-16
状态：已纳入 codex review（REQUEST CHANGES，2026-11-16）的全部阻塞项修订。

## 背景与目标

在 openhongguo 仓库内新增一个**独立**的黄果 SDK 及移动端绑定，命名为 OpenHuangguoCore。参考黄果客户端 HGhub（`AmatsuZero/study-materials-fork`）的接口，仿照现有 hongguo SDK 的形态实现。Swift 分发放**独立仓库** `AmatsuZero/openhuangguo-swift`。

两个硬约束（codex review 后进一步强化）：

1. **不动 hongguo**。上游 `beilunyang/openhongguo` 会定期更新，hongguo 部分的代码、产物、发版线、脚本保持原样，改动只通过 import 复用、不修改源文件。**不提交根 `go.work`**（见下），不改 `fix_xcf_binarytarget.py` / `release.yml` / `xcf.yml`。
2. **OpenHuangguoCore 与 OpenHongguoCore 完全独立**。独立的 xcframework/AAR、独立的构建脚本、独立的标签前缀与 release、独立的 Swift 分发仓库，互不回填、互不覆盖、不共用版本号。

## 范围

### 做

- Go SDK：HGhub **全部业务接口**，以强类型 model 暴露（codex 核对：无遗漏无多余）。
- gomobile Core（OpenHuangguoCore）：按名调用的单一入口，先打通 Swift / Kotlin。
- 纯函数式媒体工具：封面 AES 解密、HLS 播放列表改写、分片/KEY 拉取（首版行为矩阵见媒体节）。
- domain / proxy 设置能力，含凭据目标信任策略。

### 不做

- 应用层命令：线路发现与探测、更新检查、debug 日志、屏幕旋转。
- Node / Flutter / React-Native 绑定（后续再加）。
- 严格网络守卫（DNS pinning、逐跳重检）放进 Core——留给上层；但 transport 自身必须有明确的重定向策略（见下）。
- 有状态本地 HTTP 媒体代理 server（宿主 App 承担播放器资源加载层）。

## HGhub 接口事实

- 全部业务接口收敛到单一信封：`POST /api/edge/request`，body 为 `{method, path, body(base64 JSON), content_type}`。
  - 无 body：`body` 与 `content_type` 均为空字符串（`net.rs:486-497`）。
  - 有 body：标准 Base64(JSON)、`application/json`。
  - 成功响应可能是空串 / `null` / `{}`，均应视为成功（`net.rs:600-605`）。
- 登录态：`Authorization: Bearer <token>`；无 hongguo 的 SM3/AES/ProtoBuf 签名。
- 响应：`{status: 1, data, msg}`，`status != 1` 时取 `msg` 报错。
- 业务接口清单（约 20 个，对应 `api.rs` 的 20 个业务 command）：
  - 账号：`loginByPassword` / `registerByPassword` / `logout` / `me`
  - 列表：`/api/videos`、`/api/videos/category/{cate}`、`/api/ranks/{kind}`、`/api/search`、`/api/posts`、`/api/posts/{id}`
  - 详情/播放：`/api/videos/{id}`、`/api/videos/{id}/episodes/{ep}/playback`（新）、`/api/videos/{id}/play`（旧，兜底）
  - 互动：video comments / comment send、video like / collect、user follow、post like / collect / comments
- 数据形状：`src/types.ts`（VideoCard / Paged / Detail / PlayInfo / RankItem / Comment / Post / User 等）。注意 `Paged<T>` 是泛型，`Detail.counts/author`、`PlayInfo.streams` 是匿名嵌套结构——需在 Go 侧定义为具名类型（见 schema 节）。
- 媒体：封面 AES-128-CBC（key/IV 硬编码于 `media.rs`）、HLS URI 改写、分片/KEY 本地代理。
- 线路：固定 `https://huangguoai.com`、`https://huangguoai.ai`，镜像发布页 `huangguoai.pages.dev`。
- 重试语义（对齐 HGhub `net.rs:384-389, 515-516`）：仅无 body 的逻辑 GET（**可带 token**）、以及取播放地址的 POST 可重试；可重试状态含 408/425/429/5xx；401/403/4xx 其余不重试；登录/注册与所有写操作禁止盲重试。设计比初稿更宽松（允许带 token 的 GET 重试），以 HGhub 为准。

## 架构与 workspace 布局

**不提交根 `go.work`**（codex #1）。workspace 的 Go 1.26 要求会作用于原按 Go 1.22 运行的任务，且会统一依赖版本、漏纳现有 `bindings/mobile/core`——这些都触碰 hongguo。黄果模块用**自己的依赖声明 + 局部 `replace`** 开发；本地临时 go.work 仅作个人开发用，不入库、不进 CI。

```
openhongguo/
├── go.mod                               # 不变：github.com/beilunyang/openhongguo (go 1.22)
├── bindings/
│   ├── mobile/core/                     # 不变：OpenHongguoCore
│   └── huangguo/                        # 新增子树
│       ├── sdk/                         # module github.com/beilunyang/openhongguo/bindings/huangguo/sdk
│       │   ├── client.go  edge.go  auth.go  transport.go  media.go
│       │   └── model/                   # 具名类型，供 schema 生成
│       ├── core/                        # module github.com/beilunyang/openhongguo/bindings/huangguo/core → OpenHuangguoCore
│       │   ├── core.go
│       │   └── dispatch/
│       ├── gen-huangguo/                # 改造 schema 的独立生成器
│       ├── ios/  android/               # 移动端壳
│       └── build.sh                     # 独立构建脚本
└── .github/workflows/hg-xcf.yml         # 独立发版 workflow

独立仓库 AmatsuZero/openhuangguo-swift/
├── Package.swift                        # 标准包，独立版本标签
└── Sources/OpenHuangguo/                # Swift 壳源码
```

**Module path 必须保留 `github.com/beilunyang/openhongguo/...` 前缀**（codex #11）——这是复用 `bindings/internal/schema` 的硬性条件（Go internal 规则按 import 路径前缀判定）。不能改成 fork namespace。

依赖与 replace 归属（所有 replace 写在使用方 module 的 go.mod 里，下游不继承）：

- `huangguo/sdk` require `github.com/beilunyang/openhongguo v0.0.0` + 本地 replace 指向仓库根（为复用 schema/protocol 时需，按需）；go 版本独立锁定。
- `huangguo/core` require `huangguo/sdk` + 本地 replace，并自带 `tool golang.org/x/mobile/cmd/gobind` 与独立 `go.sum`。
- 黄果的 mobile 工具版本、go.sum 独立锁定，不跟随 hongguo。

## SDK 内核（`bindings/huangguo/sdk`）

- **client.go**：`Client` 持有 `transport` / `tokenStore` / `base` / `credTrustOrigins` / `clock` / `logger`。`NewClient(opts)` 校验依赖、定默认值（base = `https://huangguoai.com`），不发请求。业务方法统一 `ctx` 打头。重试循环、退避抖动、`sleepContext` 照搬 hongguo `client.go:122-174`，但重试判定改按 HGhub（见上）。
- **edge.go**：唯一线上出口。封 `{method,path,body,content_type}` POST 到 `{base}/api/edge/request`。解析契约须分别处理：外层 HTTP 状态、业务 status、无 body、`{}`、`null`。**首版禁止 edge 自动重定向**（`CheckRedirect` 返回 `http.ErrUseLastResponse`）——Go 默认会改 POST 方法/重发带密码的 body（`transport.go:56-62` 仅在 `NoRedirect` 时禁）；若要支持游客迁移，显式实现有限跳转 + 重新封装。
- **auth.go / token**：`TokenStore` 接口持久化 `{origin, token, generation}`：
  - **origin 绑定**：拒绝以另一 base 重启时误用旧 token（HGhub `session.rs:8-15,76-78`）。
  - **generation**：切 base / 退出登录递增；旧 generation 的登录响应禁止提交（对齐 HGhub `api.rs:335-338` 拒绝线路变化后的登录结果）。请求原子读取 origin/token 快照。
  - 明确 `Clear/Save` 失败如何反馈，避免退出后重启恢复旧 token。
  - SetBase 入口在 SDK、dispatch、Swift/Kotlin 壳三层都定义清楚。
- **凭据目标信任策略**（codex #4）：base 规范化为 HTTPS + 纯 origin（无 userinfo/path/query/fragment）；另设默认固定的**凭据信任名单**（`huangguoai.com` / `huangguoai.ai`）。镜像线路仅供游客浏览，不得因其被选中就向它发送登录密码或 Bearer。API 鉴权头不得自动附加到媒体请求（两者虽共用 transport，但 token 由 edge 显式注入）。
- **model/**：照 `src/types.ts` 建强类型，JSON tag 对齐 TS。`Paged<T>` 实例化为具名类型（如 `VideoCardPage`、`CommentPage`、`PostPage`），`Detail.counts/author`、`PlayInfo.streams` 定义为具名 struct——既满足 schema AST 解析，也便于生成。
- **transport.go**：自写轻量 transport（**不能复用 `httptransport`**，其 `Do()` 硬性拒绝 `Authorization`，`transport.go:44-48`）。支持：
  - 显式 `ProxyURL` + 环境变量回退。**`http.ProxyFromEnvironment` 只处理 `HTTP_PROXY`/`HTTPS_PROXY`/`NO_PROXY`（及小写）**，不含 `HGHUB_PROXY`/`ALL_PROXY`（codex #9）——额外变量自行读取，明确优先级、显式代理是否受 NO_PROXY 影响、是否支持 SOCKS、是否支持显式直连。
  - **目标 URL 与代理 URL 分开校验**：本地代理地址、代理认证不能被目标的"禁内网/禁 userinfo"规则误拒；错误信息脱敏代理凭据。
  - 轻守卫：拒目标 URL 内 user:pass、禁内网目标、日志/错误脱敏。
  - 响应读取 / 大小限制：API 响应沿用 8MB 限制；媒体资源单独定义上限（大分片不能套 API 限制，codex #8）。错误分类照搬 `httptransport`。

## 媒体工具（纯函数式，放 sdk）

对齐 hongguo 薄 Core：不起有状态本地 HTTP server。Core 暴露无状态纯函数；**宿主 App 承担播放器资源加载层**（URI 可逆映射、Range、取消、子清单递归、AVPlayer/Android 播放器适配）——codex #7。明确先用真实播放器完成"主清单→子清单→KEY→分片→seek"最小验证，再确认不需要本地 server。

- `DecryptCover([]byte) ([]byte, error)`：AES-128-CBC，key/IV 为 HGhub `media.rs:26-27` 硬编码常量；**明文图片直接透传**（HGhub `media.rs:106-118`），仅密文走解密；**条件去 padding**（`media.rs:77-84`，无效 padding 原样保留）。
- `RewritePlaylist(localScheme, host, text) string`：HLS 改写，增加显式的本地协议前缀/映射参数。首版行为矩阵与 fixture 对齐 HGhub `media.rs`：BOM 处理、带逗号的引号属性、`CONTENT-STEERING` 删除、`DATERANGE` 外部资源清理、KEY/MAP/MEDIA/SESSION-KEY/PART/PRELOAD-HINT 及分片行改写、**子清单再次识别并改写**（`media.rs:315-330`）。初稿把子清单递归列为"后续"属范围矛盾，首版纳入。
- `FetchSegment(ctx, url) (ctype string, body []byte, err error)`：用共享 transport 拉取分片/KEY，独立大小上限，支持取消。

## 绑定层

- **core/core.go**：照搬 `bindings/mobile/core/core.go:37-102` 的 gomobile 模板——`Client.Start`、`Call.Cancel`、`envelope/jsonError/logf`。**注意现有构造器耦合 `adapters.NewClient/profilePath`，不能只换 client 类型原样搬**（codex 结论）：huangguo 版构造器直接接收 `TokenStore` 与 config，不走 `adapters.NewClient`。configJSON 带 `base` / `proxy` / `credTrustOrigins`；token store 通过原生层注入。
- **core/dispatch/**：为 huangguo 重写方法表（约 20 个方法），照搬 `dispatch.go:198-280` 的参数解析、error envelope、`ErrorCodes` 结构；错误码映射换成 huangguo 哨兵错误；补 `setBase` 方法。
- **gen-huangguo/**（codex #2，改造 schema 复用）：**不能零改动复用**——`schema.go:23-26` 硬编码 hongguo `QueryTypes` 且 `schema.go:133` 要求全部存在；解析器跳过泛型声明、不支持匿名结构体字段。方案：在独立生成器进程设置导出的 `schema.QueryTypes` 为黄果查询类型，并把分页实例/嵌套对象定义为具名 Go 类型；不改 hongguo 源文件。增加最小 schema→Swift/Kotlin 生成验证作为门禁。输出 `Types.swift` / `Types.kt` 到 `bindings/huangguo/{ios,android}`。
- **移动端壳**：`bindings/huangguo/ios`（SwiftPM + `Client.swift` 封装 callback→async + 播放器资源加载适配）、`bindings/huangguo/android`（Gradle + `Client.kt`）。默认内存 TokenStore；移动端明确 Keychain/Keystore 接入契约。

## 构建与发版隔离

- **独立构建脚本** `bindings/huangguo/build.sh`：复制 `build.sh` 改造，产物改名 `OpenHuangguoCore.xcframework` / `huangguo-release.aar`、javapkg `dev.openhuangguo`，只保留 android + ios target。本地不做 xcframework 编译/打包/发布——走 `hg-xcf.yml`。
- **独立 workflow** `.github/workflows/hg-xcf.yml`：触发标签 `hg-xcf-v*`，编译 → macOS slice 校验 → zip+checksum → 发布 `OpenHuangguoCore` release。**刻意不回填、不 push 根 `Package.swift`**（避开 `xcf.yml:114` 的 `git checkout -B main origin/main` 覆盖坑与 `fix_xcf_binarytarget.py` 的全局 checksum 替换）。
- **独立 Swift 分发仓库** `AmatsuZero/openhuangguo-swift`：标准根 `Package.swift` + `Sources/OpenHuangguo` + 独立版本标签。彻底避开同包版本线绑定、`Package-Huangguo.swift` 无法被 SwiftPM 引用、现有 `release.yml:6` 监听全部 `release.published` 无标签过滤导致黄果 release 误触发 hongguo CLI 构建三个问题（codex #3/#12）。
- **CI 兼容**：现有 `ci.yml` 的 `git ls-files '*.go'` gofmt 门禁会扫到新文件（只是格式覆盖扩大，非隔离故障）；根 `go vet ./...` 不覆盖嵌套 module——新增 `huangguo-sdk` job（`working-directory: bindings/huangguo/sdk` 与 `core`）单独跑 vet/test。本地开发用临时（不入库）go.work 或逐 module 命令。

## 复用矩阵

| 档位 | 模块 | 复用方式 |
|---|---|---|
| 有条件复用 | `bindings/internal/schema` | 需设黄果 QueryTypes + 具名类型适配；保留 `github.com/beilunyang/openhongguo/...` 前缀 |
| 改造复用 | 工程骨架 | `Makefile` gen/test 目标、`build.sh` gomobile 流程、workflow 结构，复制改造；**根 go.work 不复用/不提交** |
| 改造复用 | `bindings/mobile/gen` | 逻辑通用，改输出路径 + 数据来源；只留 Swift/Kotlin |
| 改造复用 | `bindings/internal/dispatch` | 方法表重写，参数解析/envelope/ErrorCodes 照搬（`dispatch.go:198-280`） |
| 改造复用 | `bindings/mobile/core/core.go` | 异步调用/取消/JSON envelope 可照搬；**构造器不能原样搬**（耦合 adapters.NewClient） |
| 改造复用 | `adapters/memstore`/`filestore` | 存储思路借用，改成带 origin/generation 的 TokenStore；文件实现需权限/原子写/错误传播 |
| 必须重写 | transport | 因拒绝 Bearer 不能用 `httptransport`；响应读取/限制/脱敏照搬，约 100 行 |
| 必须重写 | SDK client/edge/auth | 协议不同；重试/脱敏机制照搬，补凭据信任/重定向/origin 绑定 |
| 必须重写 | model / 接口 | 照 `src/types.ts` 与 `api.rs` 机械翻译 |
| 跨语言复刻 | HGhub `media.rs` | Rust 实现无法直接复用，复刻算法与常量到 Go，含子清单递归 |

真正从零设计的部分几乎没有：绑定层是照搬骨架 + 改耦合点，SDK 层是把 Rust/TS 翻译成 Go + 复用重试/脱敏机制。无新范式要学。

## 测试

- SDK：`go test ./...`（huangguo/sdk、huangguo/core 各自 module），覆盖：
  - edge 信封编码/解析（无 body / 有 body / 空 / null / `{}`）、业务 status 错误、重试分类（含带 token GET、408/425/429）、重定向禁用。
  - TokenStore：origin 绑定、generation 失效、切域名清 token、文件权限/原子写。
  - 凭据信任：默认域名放行、镜像拒收密码/Bearer、媒体请求不带鉴权头。
  - proxy：显式 + 环境变量优先级、目标/代理分开校验、脱敏。
  - 媒体三函数：明文透传、条件 padding、BOM/属性/子清单递归，对齐 HGhub fixture。
- 契约：用 HGhub 真实响应样本做 fixture 回放，验证 model 解析（对照 `src/types.ts` 字段）。
- 生成门禁：最小 schema→Swift/Kotlin 生成验证，确认生成文件一致。
- 绑定：dispatch 方法表逐个单测；移动端壳编译 + 取消测试 + 播放器链路最小验证（codex #7）。
- 发布门禁（codex #13）：黄果独立 Go 检查（gofmt/vet/test）、生成文件一致性、gomobile 双端构建、Swift/Kotlin 壳编译与取消测试、播放器验证通过。

## 风险与开放问题

- **HGhub 接口稳定性**：上游 README 声明不保证接口持续可用；model 解析需容忍字段缺失（指针/零值）。
- **账号凭据安全**：token 默认内存 store，移动端接入 Keychain/Keystore；文件实现规定权限与原子写；日志/错误绝不输出 token 与代理凭据。
- **媒体宿主层**：播放器资源加载层由 App 实现，本仓库只提供纯函数；首版需真实播放器验证后再定是否长期不需要本地 server。
- **Swift 分发仓库**：独立仓库 `openhuangguo-swift` 的版本标签与二进制 release 对齐方式需在新仓库落地时确定；与 hongguo 根 `Package.swift` 并存无冲突（不同仓库）。
- **Go SDK 外部消费**：当前 module path 与 replace 主要服务仓库内构建；若需外部 `go get`，需嵌套模块标签方案 + 外部最小消费者验证（codex #11）。首版以仓库内/绑定消费为主。
