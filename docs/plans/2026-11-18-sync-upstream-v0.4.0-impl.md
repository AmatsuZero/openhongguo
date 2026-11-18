# 同步上游 v0.3.1 + v0.4.0 实现计划

日期：2026-11-18
对应设计：`docs/plans/2026-11-18-sync-upstream-v0.4.0-design.md`
状态：已纳入 codex review（第一轮 REQUEST CHANGES，本版逐条修订）。

约定：上游指 `v0.4.0` tag（已 fetch 到本地，含中间 `v0.3.1`）。所有上游改动通过 `git show v0.4.0:<path>` / `git diff v0.3.0 v0.3.1 -- <path>` 读取，逐文件手工合并，**禁止 `git checkout` 目录/整文件覆盖 fork 版本**。

提交与授权（用户已确认，解决 codex P1-3）：**每个 Task 在本地 commit 一次**（便于 TDD 节奏与回滚），但**绝不 `git push`、绝不打标签**。全部 Task 完成 + 本地门禁全绿后汇报，等用户**单独授权**才推送与发版。当前状态：HEAD=`bee17c9`（比 origin/main 超前 1 个文档 commit）、工作树有未跟踪的 `.codebuddy/`——该目录是本地工具配置，**与同步无关，不要清理、不要提交**。同步代码直接叠加在此 HEAD 之上。

Task 依赖顺序原则（解决 codex P1-2）：先底层请求函数 → 再依赖它们的 client/person → 再装饰 Transport → 最后 UI 配置。每个 Task 结束时其涉及的包必须能编译、测试通过；互相依赖的文件归入同一 Task。

## Task 0 — 前置核对

- 确认 `v0.4.0`、`v0.3.1` tag 存在；确认 HEAD 含文档 commit、`.codebuddy/` 未跟踪属预期（见上）。
- 不建 worktree（同步任务，直接叠加在 main 当前 HEAD），不在本 Task 提交。
- 产出：状态确认，无代码改动。

## Task 1 — V5 播放协议 core + 解析层（v0.3.1 新增/一致文件）

一次完成 V5 所需的所有底层，保证后续 Task 可编译：

- 新增 `internal/protocol/playback_v5.go`、`playback_url.go`、`internal/protocol/sign/playback_v5.go`、`playback_v5_aes.go`、`playback_v5_constants.go`、`playback_v5_test.go`（取上游原文，无 fork 独有改动）。
- `internal/protocol/parse.go`：fork 与 v0.3.0 一致，取上游 v0.3.1。
- `internal/protocol/sign/aes.go`：取上游（小改）。
- 新增 `testdata/protocol/person-tabs.json`、`person-tabs-works.json`；更新 `short-rank-actors.json`（取上游）。
- 验证：`go test ./internal/protocol/...`。

## Task 2 — 请求层与游标（冲突文件，person 的前置依赖）

必须先于 person/client：person.go 依赖本 Task 的 `UserWorksTabsBody`、新版 `UserWorksBody(userID, cur, *tab)`。

- `internal/protocol/request.go`：合入上游 v0.3.1 对请求参数的改动 + 新增 `UserWorksTabsBody`、改 `UserWorksBody` 签名；**保留** fork 的 `FeedContinuation.Limit` 字段、`CellChangeLimit()`、`limit` 取值逻辑。
- `internal/protocol/cursor.go`：合入上游改动，保留 fork 的 `feedCursor.Limit` 编解码。
- `internal/protocol/parse_test.go`：**冲突文件**（codex P2-1）——fork 增加了游标 `Limit=30` 往返断言，不得整文件替换；合入上游 v0.3.1 用例并保留 fork 断言。
- 验证：`go test ./internal/protocol/...`；确认 fork 的 PageSize/Limit 回归测试仍通过。

## Task 3 — 解析重试与分页测试（冲突文件）

- `sdk/catalog_test.go`：**冲突文件**（codex P1-1，原计划遗漏）——上游把播放/person 断言改为 V5、更新 person tabs 夹具；fork 有 `TestDiscoverPageSizeUsesLimit` 等用例。手工合并两侧，保留 fork 测试。
- 验证：`go test ./sdk/...`（此时 client 尚未合 V5，若测试依赖 V5 断言则本 Task 与 Task 4 合并提交——见下）。

## Task 4 — client / person / media / catalog 的 V5 接线（冲突文件，可编译单元）

把相互依赖的 SDK 文件作为一个可编译单元（解决 codex P1-2、P1-1）：

- `sdk/client.go`（冲突）：保留 fork 独有 `webOrigin`、`durations`（弹幕）、`resetter` 并发换设备语义、`WebOrigin` 默认值；合入 **v0.3.1 已引入**的 `playbackSigner *sign.PlaybackV5Signer` 字段及 `sign.NewPlaybackV5Signer(opts.Clock().UnixMilli())` 初始化（**不是 v0.4.0**）；保留 v0.4.0 的 `SignerURL` 字段校验 + `newRemoteSigner` 包装（包装函数在 Task 5 新增，本 Task 与之同批提交以保证可编译）、`doOnce` 中 `ErrSignerUnavailable` 原样返回。
- `sdk/media.go`（fork 与 v0.3.0 一致）：取上游 v0.3.1 的 `ResolvePlayback`（V5 优先 + 空结果重签重试一次 300ms + 404/405 回退旧接口 + `playbackRouteUnavailable`）。
- `sdk/person.go`（fork 与 v0.3.0 一致）：取上游 v0.3.1（依赖 Task 2 的新函数）。
- `sdk/model/catalog.go`（一致）：取上游 v0.3.1。
- `sdk/options.go`（冲突）：合入 v0.3.1 注释更新 + v0.4.0 的 `SignerURL`/`SignerToken` 字段；保留 fork 的 `WebOrigin`。
- 验证：`go test ./sdk/...`（含 Task 3 合并后的 catalog_test.go）。

## Task 5 — 远程签名 core（v0.4.0 新增文件）

- 新增 `sdk/remote_signer.go`、`sdk/remote_signer_test.go`（取上游原文）。
- `sdk/errors.go`：合入 `ErrSignerUnavailable` 语义扩展（注释；错误值 fork 已有）。
- 与 Task 4 的 client 包装同批提交以保证可编译（见 Task 4）。
- 验证：`go test ./sdk/...`，含 `remote_signer_test.go`。
- **覆盖范围限制**（codex P2-3）：远程签名装饰器仅拦截带 `x-medusa` 的请求；V5 回退旧播放接口时，旧接口会删除 `x-medusa`，故回退请求仍用内置 Argus/Ladon 签名，不受 `SignerURL` 覆盖。「不回退内置签名」仅指签名服务自身失败时，不概括播放路由回退。

## Task 6 — contract 层（冲突文件，含三层重试预算）

- `internal/contract/run.go`（冲突）：合入上游改动，**保留** fork 的 `playSeries` 中 `ErrAnonymousLimited` → `ResetDevice` 换设备循环（最多 2 次）。
- **三层重试预算**（codex P2-2）：一次 contract 检查的重试来自三层——① `ResolvePlayback` 内 V5 重签 + 旧接口回退（每次尝试 2 次 V5 + 1 次旧接口）；② SDK `withFreshDevice` 默认自动换设备最多 2 次（`sdk/reset.go:10`）；③ fork contract 手动换设备最多 2 次。持续 `ErrAnonymousLimited` 时理论上最多 8 次换设备、27 次播放请求。合入后**明确唯一换设备责任方与总预算**，避免无界重试；加测试断言请求次数在预期范围。
- `internal/contract/contract_test.go`：取上游新增用例，保留 fork 用例。
- 验证：`go test ./internal/contract/...`（live contract 定时任务属历史 run，区分对待）。

## Task 7 — dispatch 配置透传（冲突文件，含陷阱）

- `bindings/internal/dispatch/dispatch.go`：新增 `SignerURL`/`SignerToken` 字段与 `Apply` 透传；**保留** fork 的 `WebOrigin` 字段与 `getComicReader` 方法；**不得**带回上游的 `DeviceResets *int` 及 `opts.DeviceResets = cfg.DeviceResets`（fork binding 层已删该透传，core 层不受影响）。
- `bindings/internal/dispatch/dispatch_test.go`：合入上游 signer 用例，保留 fork 用例（含 `webOrigin` 相关，见 codex 核对）。
- 验证：`go test ./bindings/internal/...`。

## Task 8 — bindings 各语言 signerUrl/signerToken 透传

- 逐语言合入上游 v0.4.0 的 client 配置字段与类型（配置字段，非 V5 类型）：node `index.js`/`index.d.ts`/`types.d.ts`、react-native `src/client.ts`/`src/types.ts`、mobile `Client.kt`/`Client.swift`、`cmd/ohg/main.go`（CLI 读 `OHG_SIGNER_URL`/`OHG_SIGNER_TOKEN`）。
- **各语言 Types 文件含 fork 独有内容（漫画类型、PageSize），必须保留**（codex 核对确认）。
- 版本类文件（`package.json`/`package-lock.json`/`pubspec.yaml`/`build.gradle*`/podspec/`Package.swift`）只取 signer 相关语义，**版本号与 AmatsuZero 分发地址保持 fork 现状**。
- 验证：`make gen`（生成文件后**审阅 diff**，见 Task 11）；`make test-node`、`make test-react-native`。

## Task 9 — 文档与 changelog

- `README.md`：合入「选择签名方式」章节、Options 表 `SignerURL/SignerToken` 行、错误码说明；URL 按 fork 规则处理（仓库/CI/Release 用 AmatsuZero，Go module/Android Maven/作者版权保留 beilunyang）。
- **死链处理**（codex P2-6）：上游「选择签名方式」正文链接到 `signer/README.md`，但本同步排除整个 `signer/`；把该链接改为上游固定 URL 或 Docker Hub 说明，并全局检查其它 `signer/` 引用，不留死链。
- `docs/cli.md`、`docs/development.md`、`docs/types.md` 合入上游更新；flutter `CHANGELOG.md` 与各 binding README 合入 0.3.1/0.4.0 条目，版本表述对齐 fork。
- 验证：人工核对 URL 引用规则 + 无死链。

## Task 10 — 本地验证门禁（对应现有 CI，codex P2-4）

`make gen` 是**写入生成文件**而非 drift 校验；drift 由 Go 生成物测试检查。全部通过后再汇报：

1. `make gen` 后审阅生成 diff（确认仅预期变化）
2. `go vet ./...`
3. `go test ./...`
4. `make test-node`、`make test-react-native`
5. wasm 侧 `go vet`/`go test`（对照根 `ci.yml` 的 wasm job）
6. Swift 壳 `swift build` + `swift test`；Kotlin `compileDebugKotlin`/`test`（本地实编）
7. Flutter `flutter analyze` + `flutter test`（若本机可用；否则注明留给 CI）
8. `gofmt -l` 无输出
- 明确留给 CI 的部分并在汇报中说明。仓库无 benchmark 门禁（codex 核对确认）。

## Task 11 — 发版（需用户单独授权，授权后才执行）

- 推 `xcf-v*` 标签触发 `.github/workflows/xcf.yml`：**仅**构建并发布 `OpenhongguoCore.xcframework`（codex P2-5，该 workflow 不含 AAR）。不在本地打包。
- AAR 不在本 workflow 产物内，另行按 fork 既有路径构建/验收（如需要）。
- 推送后用 `gh run -R AmatsuZero/openhongguo` 盯本次 run，全绿再汇报；遇临时 infra 故障（runner 容量）等待重试，不削弱门禁。
- semver 标签与 Package.swift 回填按 fork 既有发版流程处理。

## 不纳入本计划

- `signer/` 目录整套、`.github/workflows/signer.yml`、Docker Hub 推送。
- 上游 `Package.swift` 的版本/URL。
- 上游 `.github/workflows/ci.yml`、`contract.yml`、`release.yml`：经核对 v0.3.1→v0.4.0 **无实际增量**（stat 为空），保持 fork 现状。
- CrawlerBot hongguo-cli 的 `makeClient()` 改动（在下游仓库；本仓库只保证 binding 暴露配置字段）。
