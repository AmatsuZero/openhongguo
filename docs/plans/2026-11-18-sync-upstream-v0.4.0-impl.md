# 同步上游 v0.3.1 + v0.4.0 实现计划

日期：2026-11-18
对应设计：`docs/plans/2026-11-18-sync-upstream-v0.4.0-design.md`
状态：已纳入 codex review（第一轮 9 条、第二轮 5 条已闭合，第三轮 1 P1 本版修订）。

## 全局约定

- **上游**指 `v0.4.0` tag（已 fetch，含中间 `v0.3.1`）。上游改动用 `git show v0.4.0:<path>` / `git diff v0.3.0 v0.3.1 -- <path>` 读取，逐文件手工合并，**禁止 `git checkout` 目录/整文件覆盖 fork 版本**。
- **提交与授权**（用户已确认，codex P1-3）：每个 Task 在本地 commit 一次（TDD、便于回滚），**绝不 push、绝不打标签**。全部 Task 完成 + 本地门禁全绿后汇报，等用户**单独授权**才推送与发版。
- **执行前核对当前 HEAD 与工作树状态**（不写死 commit 号，避免状态说明过期，codex 第二轮 P3）；未跟踪 `.codebuddy/` 是本地工具配置，与同步无关，不清理、不提交。
- **可编译单元原则**（codex 第二/三轮 P1）：因 Go 包内符号互相依赖，按"每次提交后**全仓** `go test ./...` 可通过"聚合文件，而不是按 v0.3.1/v0.4.0 版本或功能点拆分。依赖关系已用 `git grep` 核实：
  - `internal/protocol`：`parse.go` 的 `ParseWorksTab` 使用 `WorksTab`（定义在 `request.go:433`）→ 同批。
  - 跨包：`request.go` 把 `UserWorksBody` 改为三参，唯一调用点 `sdk/person.go` 必须同批更新，否则中间提交全仓编译失败（codex 第三轮 P1）。
  - `sdk`：`client.go` 的 `playbackSigner` 被 `media.go` 使用；`remote_signer.go` 的 `newRemoteSigner` 被 `client.go` 调用 → 同批。
  - 因此 **internal/protocol 与 sdk 合并为一次代码提交**（见下方 Task 1）。

## Task 0 — 前置核对

- 确认 `v0.4.0`、`v0.3.1` tag 存在；核对当前 HEAD、工作树状态（`.codebuddy/` 未跟踪属预期）。
- 不建 worktree（同步任务，直接叠加在当前 HEAD），本 Task 不提交。

## Task 1 — internal/protocol + sdk 一起（单次代码提交，全仓可编译）

`request.go` 的 `UserWorksBody` 改三参、`sdk/person.go` 是唯一调用点，两包强耦合，必须在一次提交内一起改（codex 第三轮 P1）。TDD 顺序：先合入测试断言，再合入实现，最后 `go test ./...` 全绿再提交。

**internal/protocol 侧**：

- 取上游 v0.3.1 版本（fork 与 v0.3.0 一致）：`internal/protocol/parse.go`、`cursor.go`、`sign/aes.go`。
- 新增（取上游原文）：`playback_v5.go`、`playback_url.go`、`sign/playback_v5.go`、`sign/playback_v5_aes.go`、`sign/playback_v5_constants.go`、`sign/playback_v5_test.go`。
- **冲突文件手工合并**：`internal/protocol/request.go`（合入 `WorksTab`、`UserWorksTabsBody`、三参 `UserWorksBody`、`SignedCellChangePairs` 改动，**保留** fork 的 `FeedContinuation.Limit`/`CellChangeLimit()`/`limit` 逻辑）；`internal/protocol/cursor.go`（**保留** fork 的 `feedCursor.Limit` 编解码）；`internal/protocol/parse_test.go`（fork 有 `Limit=30` 往返断言，不得整文件替换）。
- 新增/更新 testdata：`person-tabs.json`、`person-tabs-works.json`、`short-rank-actors.json`（取上游）。

**sdk 侧**：

- `sdk/client.go`（冲突）：保留 fork 独有 `webOrigin`、`durations`（弹幕）、`resetter` 并发换设备语义、`WebOrigin` 默认值；合入 **v0.3.1 已引入**的 `playbackSigner *sign.PlaybackV5Signer` 及 `sign.NewPlaybackV5Signer(opts.Clock().UnixMilli())` 初始化（**不是 v0.4.0**）；合入 v0.4.0 的 `SignerURL` 校验 + `newRemoteSigner` 包装、`doOnce` 中 `ErrSignerUnavailable` 原样返回。
- `sdk/media.go`、`sdk/person.go`、`sdk/model/catalog.go`（fork 与 v0.3.0 一致）：取上游 v0.3.1（`ResolvePlayback` V5 优先 + 空结果重签一次 300ms + 404/405 回退 + `playbackRouteUnavailable`；person 改用三参 `UserWorksBody` + `ParseWorksTab`）。
- `sdk/options.go`（冲突）：合入 v0.3.1 注释更新 + v0.4.0 的 `SignerURL`/`SignerToken`；**保留** fork 的 `WebOrigin`。
- `sdk/catalog_test.go`（冲突）：合入上游 V5/person 断言与夹具，**保留** fork 的 `TestDiscoverPageSizeUsesLimit` 等。
- 新增 `sdk/remote_signer.go`、`sdk/remote_signer_test.go`（取上游原文）；`sdk/errors.go` 合入 `ErrSignerUnavailable` 语义扩展（错误值 fork 已有）。
- **远程签名覆盖范围**（codex 第一轮 P2-3）：装饰器仅拦带 `x-medusa` 请求；V5 回退旧接口时旧接口删除 `x-medusa`，回退请求仍用内置 Argus/Ladon 签名。「不回退」仅指签名服务自身失败。

- 验证：`go test ./...` 全绿后一次提交（此时 root 全仓可编译）。

## Task 2 — contract 层（冲突文件，三层重试定责）

- `internal/contract/run.go`（冲突）：合入上游对 contract 的其余改动；处理换设备重试——
  - **唯一换设备责任方 = contract 手动循环，SDK 自动换设备关闭**（codex 第二轮 P2-1 定责）：在 contract 创建 client 的 `sdk.Options` 中加 `DeviceResets: sdk.Int(0)`，使 `withFreshDevice` 直接 `return call(ctx)`（`sdk/reset.go:35`）。保留 fork 的 `playSeries` 中 `ErrAnonymousLimited` → `ResetDevice` 手动循环（最多 2 次）。理由：上游 v0.3.1 的 contract 本就无手动循环、依赖 SDK；fork 的手动循环是 v0.3.0 补丁，二者并存会无界重试。关闭 SDK 自动换设备后责任单一，且 production 调用者（CLI、hongguo-cli）不受影响。
  - **总预算**：最多 2 次换设备、9 次播放请求（初始 1 + 每次换设备后 1，共 3 次 `ResolvePlayback`；每次内部 2 次 V5 + 1 次旧接口）。
- `internal/contract/contract_test.go`：合入上游用例，保留 fork 用例；**加断言**锁定持续 `ErrAnonymousLimited` 时请求次数 ≤ 9、换设备 ≤ 2。
- 验证：`go test ./internal/contract/...`（live contract 定时任务属历史 run，区分对待）。

## Task 3 — dispatch 配置透传（冲突文件，含陷阱）

- `bindings/internal/dispatch/dispatch.go`：新增 `SignerURL`/`SignerToken` 字段与 `Apply` 透传；**保留** fork 的 `WebOrigin` 字段与 `getComicReader` 方法；**不得**带回上游的 `DeviceResets *int` 及 `opts.DeviceResets = cfg.DeviceResets`（fork binding 层已删该透传，core 层不受影响）。
- `bindings/internal/dispatch/dispatch_test.go`：合入上游 signer 用例，保留 fork 的 `webOrigin` 等用例。
- 验证：`go test ./bindings/internal/...`。

## Task 4 — bindings 各语言 signerUrl/signerToken 透传

逐语言合入上游 v0.4.0 的客户端配置字段与类型（配置字段，非 V5 类型）：

- node：`index.js`、`index.d.ts`、`types.d.ts`
- react-native：`src/client.ts`、`src/types.ts`
- mobile：`Client.kt`、`Client.swift`
- **flutter：`lib/src/client.dart`**（上游增加 `signerUrl`/`signerToken` 构造参数与 JSON 透传，**`make gen` 不生成此文件，必须手工合入**，codex 第二轮 P1-2）
- `cmd/ohg/main.go`：CLI 读 `OHG_SIGNER_URL`/`OHG_SIGNER_TOKEN`。
- **各语言 Types 文件含 fork 独有内容（漫画类型、PageSize），必须保留**（codex 核对确认）。
- 版本类文件（`package.json`/`package-lock.json`/`pubspec.yaml`/`build.gradle*`/podspec/`Package.swift`）只取 signer 相关语义，**版本号与 AmatsuZero 分发地址保持 fork 现状**。
- 验证：`make gen` 后审阅生成 diff；`make test-node`、`make test-react-native`。

## Task 5 — 文档与 changelog

- `README.md`：合入「选择签名方式」章节、Options 表 `SignerURL/SignerToken` 行、错误码说明；URL 按 fork 规则（仓库/CI/Release 用 AmatsuZero，Go module/Android Maven/作者版权保留 beilunyang）。
- **死链处理**（codex 第一轮 P2-6）：上游正文链接 `signer/README.md`，但本同步排除 `signer/`；替换为上游固定 URL 或 Docker Hub 说明，全局检查其它 `signer/` 引用。
- `docs/cli.md`、`docs/development.md`、`docs/types.md` 合入上游更新；flutter `CHANGELOG.md` 与各 binding README 合入 0.3.1/0.4.0 条目，版本表述对齐 fork。
- 验证：人工核对 URL 引用规则 + 无死链。

## Task 6 — 本地验证门禁（对应现有 CI，codex 第一轮 P2-4、第二轮 P2-2）

`make gen` 是**写入生成文件**而非 drift 校验；drift 由 Go 生成物测试检查，生成后须审阅 diff。全部通过后汇报：

1. `make gen` 后审阅生成 diff（仅预期变化）
2. 根目录 `go vet ./...`、`go test ./...`
3. **嵌套 module `bindings/mobile/core`**（根目录命令不覆盖，CI 单独跑，codex 第二轮 P2-2）：在该目录执行 `go vet ./...`、`go test ./...`
4. `make test-node`、`make test-react-native`
5. wasm 侧 `go vet`/`go test`（对照根 `ci.yml` 的 wasm job）
6. Swift 壳 `swift build` + `swift test`；Kotlin `compileDebugKotlin`/`test`（本地实编）
7. Flutter `flutter analyze` + `flutter test`（本机可用时；否则注明留给 CI）
8. `gofmt -l` 无输出（ci.yml 门禁）
- 明确留给 CI 的部分在汇报中说明。仓库无 benchmark 门禁（codex 核对确认）。

## Task 7 — 发版（需用户单独授权，授权后才执行）

- 推 `xcf-v*` 标签触发 `.github/workflows/xcf.yml`：**仅**构建并发布 `OpenhongguoCore.xcframework`（该 workflow 只跑 `bindings/mobile/build.sh ios`，**不含 AAR**），**不本地打包**。AAR 另行按 fork 既有路径验收。
- 推送后用 `gh run -R AmatsuZero/openhongguo` 盯本次 run，全绿再汇报；临时 infra 故障（runner 容量）等待重试，不削弱门禁。
- semver 标签与 Package.swift 回填按 fork 既有发版流程处理。

## 不纳入本计划

- `signer/` 目录整套、`.github/workflows/signer.yml`、Docker Hub 推送。
- 上游 `Package.swift` 的版本/URL。
- 上游 `.github/workflows/ci.yml`、`contract.yml`、`release.yml`：经核对 v0.3.1→v0.4.0 **无实际增量**（stat 为空），保持 fork 现状。
- CrawlerBot hongguo-cli 的 `makeClient()` 改动（在下游仓库；本仓库只保证 binding 暴露配置字段）。
