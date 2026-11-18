# 同步上游 v0.3.1 + v0.4.0 设计

日期：2026-11-18
状态：设计已与用户逐节确认；已纳入 codex review（第一轮 REQUEST CHANGES，本版逐条修订）。

## 背景与目标

fork（AmatsuZero/openhongguo）的 main 停在 v0.1.8，core 只移植了上游 v0.3.0 的 Argus/Ladon 播放签名。上游随后发布：

- **v0.3.1**：播放改走 V5 取址协议，修复「v0.3.0 取播放模型失败」；演员作品先取 tab 列表；搜索联想参数改 `q`。
- **v0.4.0**：新增 `SignerURL`/`SignerToken` 选项，可把请求签名交给独立的 `openhongguo-signer` HTTP 服务；签名服务不可用时返回 `SIGNER_UNAVAILABLE`，不回退内置签名。

目标：

1. **修复 fork 当前的播放取址失败**——合入 v0.3.1 的 V5 协议（这是线上 bug 修复，必须做）。
2. **预留远程签名能力**——合入 v0.4.0 的 core/binding 侧薄接口（`SignerURL` + Transport 装饰器），使内置签名失效时（App 升级改算法）可切换到外部 signer 服务。

## 范围界定（用户已确认：core 加 SignerURL，服务独立）

### 做

- **v0.3.1 全部 core 及 binding 改动**：V5 播放取址、V5 签名算法、person tabs、search `q`、相关 fixture 与契约测试。
- **v0.4.0 的 core/binding 侧**：`sdk/remote_signer.go`、`Options.SignerURL/SignerToken`、`client.go` 的 Transport 包装、`ErrSignerUnavailable` 语义扩展、各语言 binding 的 `signerUrl`/`signerToken` 配置透传、CLI 环境变量、文档与 changelog。

### 不做

- **signer 服务整套**（`signer/` 目录，约 4000 行 Java/unidbg/`libmetasec_ml.so`/Docker/Caddy/gradlew、`signer/compare/`）、`.github/workflows/signer.yml`。该服务与 SDK 正交，将来独立成模块或直接用上游 Docker Hub 镜像 `beilunyang/openhongguo-signer`，与本次 core 同步解耦。
- **上游 `Package.swift`**：保持指向 AmatsuZero 的 `xcf-v*` release，不取上游版本。
- 不动 fork 自有能力：comics 阅读器、Discover `PageSize`/`CellChangeLimit`、wasm 绑定、`Danmaku`、`WebOrigin`、`DeviceResets` 在 core 层的行为。

## 架构与数据流

两条正交改动线：

### 线 A：V5 播放取址（v0.3.1）

`internal/protocol/playback_v5.go` 新增 V5 取址请求构造，`internal/protocol/sign/playback_v5*.go` 新增 V5 签名。`sdk.ResolvePlayback`（`sdk/media.go`）改为 V5 优先：

1. 用 `PlaybackV5Pairs` + `PlaybackV5Signer` 签名，POST `PathPlaybackV5`；
2. 空结果（`ErrAnonymousLimited`）时重签重试一次（间隔 300ms）；
3. HTTP 404/405（路由不可用）或仍空时，回退旧 `PathPlayback` 的 Argus/Ladon 签名路径。

**重试是三层，不是两层**（codex P2-2）：除 V5 内部重签 + 旧接口回退外，SDK 的 `withFreshDevice`（`sdk/reset.go:10`，`defaultDeviceResets=2`）在 `ErrAnonymousLimited`/`ErrDeviceRejected` 时默认自动换设备最多 2 次，fork 的 `internal/contract/run.go` 在 `ResolvePlayback` 外又有一层手动 `ResetDevice` 循环（最多 2 次）。持续 `ErrAnonymousLimited` 时一次 contract 检查理论上最多 8 次换设备、27 次播放请求（每次尝试 2 次 V5 + 1 次旧接口）。合并后须明确定 唯一的换设备责任方与总预算，避免无界重试，并用测试锁定请求次数。

### 线 B：远程签名（v0.4.0，可选）

在 **Transport 层套装饰器** `remoteSigner`（`sdk/remote_signer.go`），不是替换签名算法：

```
请求构造 → sign 包算内置签名头（基线）
        → remoteSigner.Do 拦截：仅当请求带 x-medusa
            → POST signer 服务 {url, body_hex}，带 X-Signer-Token
            → 用返回签名头替换 x-gorgon/x-medusa/x-helios/x-khronos/x-argus/x-ladon
        → 下层 Transport 真正发出
```

关键性质：

- **默认不启用**：`SignerURL` 为空时不装 `remoteSigner`，链路与现状完全一致，零行为变化。
- **服务不可用即失败**：`remoteSigner` 的错误包 `ErrSignerUnavailable` 原样透传（不在 `client.go` 脱敏为 `ErrTransport`），**不回退内置签名**——内置失效时回退只会拿到空结果并触发换设备重试。
- **装饰器位置**保证对所有 binding 的 Transport（含 node 的 JS 回调 Transport）统一生效。
- **覆盖范围有限**（codex P2-3）：装饰器仅拦截带 `x-medusa` 的请求。V5 遇 404/405 或空响应回退旧播放接口时，旧接口会主动删除 `x-medusa`（`sign/playback.go`），此时即使设了 `SignerURL`，回退请求仍用内置 Argus/Ladon 签名。「不回退内置签名」仅指**签名服务自身失败**时，不能概括为播放路由回退也不走内置签名。
- 签名方式在创建客户端时确定，之后不可切换；只设 `SignerToken` 不设 `SignerURL` 仍用内置签名。

### hongguo-cli 接入点

`CrawlerBot/hongguo/swift/.../main.swift` 的 `makeClient()` 只读 origin 类环境变量。加读 `OHG_SIGNER_URL`/`OHG_SIGNER_TOKEN` 写进 `ClientOptions` 即可（此步在 CrawlerBot 仓库，属 fork 之外的下游改动，不在本仓库落地；本仓库只需保证 binding 暴露 `signerUrl`/`signerToken` 配置字段）。默认空 → 仍走内置签名。

## 逐文件合并策略

上游 v0.3.1 / v0.4.0 共触及约 60 个文件。原则：**逐文件手工合并，禁止目录级 checkout，禁止整文件覆盖 fork 版本**。

### 可直接取上游版本（fork 相对 v0.3.0 一致，codex 已用 `git diff v0.3.0 HEAD` 核实）

`sdk/media.go`、`sdk/person.go`、`internal/protocol/parse.go`（v0.3.1 改动）、全新文件如 `internal/protocol/playback_v5.go`、`internal/protocol/playback_url.go`、`internal/protocol/sign/playback_v5*.go`、`sdk/remote_signer.go`、`sdk/remote_signer_test.go`、testdata、各 changelog。

### 需手工合并（fork 有独有改动，共 8 个，见 impl 逐条）

`sdk/client.go`、`sdk/options.go`、`sdk/catalog_test.go`、`internal/protocol/request.go`、`internal/protocol/cursor.go`、`internal/protocol/parse_test.go`、`internal/contract/run.go`、`bindings/internal/dispatch/dispatch.go`。

其中 `parse_test.go` 易漏（codex P2-1）：fork 增加了游标 `Limit=30` 往返断言，不得整文件替换。`sdk/catalog_test.go`（codex P1-1）必须合入，否则上游 V5/person 断言不更新会导致 `go test ./sdk/...` 失败。

重点说明：

- `sdk/client.go`：fork 有 `webOrigin`、`durations`（弹幕）、`resetter` 并发换设备语义；上游 `playbackSigner *sign.PlaybackV5Signer` 字段与初始化是 **v0.3.1 引入**（不是 v0.4.0），v0.4.0 才加 remoteSigner 包装。需把两者都合入且保留 fork 独有字段。
- `bindings/internal/dispatch/dispatch.go`：**陷阱**——fork 相对 v0.3.0 删掉了 `DeviceResets *int` 字段与 `opts.DeviceResets = cfg.DeviceResets` 透传（core 层 `Options.DeviceResets` 仍在），上游 v0.4.0 的 Config 保留了它。合 signer 两字段时**不得把 `DeviceResets` 透传带回**（保持 fork binding 层现状），只加 `SignerURL`/`SignerToken`。
- `internal/contract/run.go`：保留 fork 的换设备重试循环，叠加上游对 V5/contract 的改动。
- `sdk/options.go` / `internal/protocol/request.go` / `cursor.go` / `catalog_test.go`：保留 fork 的 `WebOrigin`、`CellChangeLimit`/`Limit` 游标字段、PageSize 测试，叠加上游字段。

### README 引用规则

改 README 时仓库/CI/Release URL 用 AmatsuZero；Go module path、Android Maven 坐标、作者/版权保留 `beilunyang`（遵循既有 fork 规则）。上游「选择签名方式」章节正文链接到 `signer/README.md`，但本同步排除整个 `signer/` 目录，合入时须替换为可用链接（上游固定 URL 或 Docker Hub 说明），不留死链（codex P2-6）。

## 提交与授权边界

用户已确认：**每个 Task 在本地 commit 一次**（TDD 节奏、便于回滚），但**绝不 push、绝不打标签**。全部 Task 完成 + 本地门禁全绿后汇报，等用户**单独授权**才推送与发版。当前 HEAD 含文档 commit、工作树有未跟踪的 `.codebuddy/`（本地工具配置，与同步无关，不清理不提交）。

## 验证门禁（推送前本地全绿，对应现有 CI，codex P2-4）

`make gen` 是**写入生成文件**而非 drift 校验；drift 由 Go 生成物测试检查，生成后须审阅 diff。

1. `make gen` 后审阅生成 diff（仅预期变化）
2. `go vet ./...`、`go test ./...`
3. `make test-node`、`make test-react-native`
4. wasm 侧 `go vet`/`go test`（对照根 `ci.yml`）
5. Swift 壳 `swift build` + `swift test`、Kotlin `compileDebugKotlin`/`test`（本地实编）
6. Flutter `flutter analyze` + `flutter test`（本机可用时；否则注明留给 CI）
7. `gofmt -l` 无输出（ci.yml 门禁）
- 仓库无 benchmark 门禁（codex 核对确认）。留给 CI 的部分在汇报中说明。

## 发版（另行授权）

- 推 `xcf-v*` 标签触发 `.github/workflows/xcf.yml`：**仅**构建并发布 `OpenhongguoCore.xcframework`（该 workflow 只跑 `bindings/mobile/build.sh ios`，**不含 AAR**，codex P2-5），**不本地打包**。AAR 不在本 workflow 产物内，另行按 fork 既有路径验收。
- 推送后用 `gh run -R AmatsuZero/openhongguo` 盯本次流水线，区分本次 run 与历史定时任务，全绿再汇报；临时 infra 故障（runner 容量）不削弱门禁，等待重试。

## 风险与不做的事

- V5 协议是线上修复，若回退逻辑合错会导致播放仍失败或误回退——impl 阶段用契约测试锁定「V5 成功 / V5 空重签 / 404 回退 / 换设备」四条路径，并锁定三层重试的请求次数预算。
- `remoteSigner` 不回退内置签名是**有意设计**，不要"好心"加回退；但其覆盖范围仅限带 `x-medusa` 的请求，V5 回退旧接口不受覆盖。
- 不把 4000 行 signer 服务拖进 core，不引入 Java/unidbg 构建依赖。
- 上游 `.github/workflows/ci.yml`、`contract.yml`、`release.yml` 在 v0.3.1→v0.4.0 无实际增量（stat 为空），保持 fork 现状。
