# 同步上游 v0.3.1 + v0.4.0 实现计划

日期：2026-11-18
对应设计：`docs/plans/2026-11-18-sync-upstream-v0.4.0-design.md`
状态：待 codex 审查。

约定：上游指 `upstream/v0.4.0` tag（已 fetch）。所有上游改动通过 `git show v0.4.0:<path>` / `git diff v0.3.0 v0.3.1 -- <path>` 读取，逐文件手工合并，**禁止 `git checkout` 目录/整文件覆盖 fork 版本**。每个 Task 一次 commit，TDD：先加/改测试再改实现。

## Task 0 — 前置核对与分支

- 确认 main 干净、HEAD 与 origin/main 一致；确认 `upstream/v0.4.0` tag 存在。
- 从 main 开始（直接在 main 逐 Task commit，不建 worktree——本任务是同步，不是隔离功能开发）。
- 产出：无代码改动，仅状态确认。

## Task 1 — V5 播放协议 core（v0.3.1 新增文件）

- 新增 `internal/protocol/playback_v5.go`、`internal/protocol/playback_url.go`（取上游原文，无 fork 独有改动）。
- 新增 `internal/protocol/sign/playback_v5.go`、`playback_v5_aes.go`、`playback_v5_constants.go`、`playback_v5_test.go`（取上游原文）。
- 新增 `testdata/protocol/person-tabs.json`、`person-tabs-works.json`；更新 `short-rank-actors.json`（取上游）。
- 验证：`go test ./internal/protocol/...`。

## Task 2 — 解析层与 person/search 修复（v0.3.1 修改文件，fork 一致可直接取）

- `internal/protocol/parse.go`、`parse_test.go`、`sdk/person.go`、`sdk/model/catalog.go`：fork 相对 v0.3.0 一致，直接取上游 v0.3.1 版本。
- `internal/protocol/sign/aes.go`：取上游（小改）。
- 验证：`go test ./internal/protocol/... ./sdk/...`。

## Task 3 — 请求层与游标（冲突文件）

- `internal/protocol/request.go`：合入上游 v0.3.1 对 `SignedCellChangePairs`/请求参数的改动，**保留** fork 的 `FeedContinuation.Limit` 字段、`CellChangeLimit()` 与 `limit` 取值逻辑（`git diff v0.3.0 HEAD -- internal/protocol/request.go` 为 fork 侧差异）。
- `internal/protocol/cursor.go`：合入上游改动，保留 fork 的 `feedCursor.Limit` 编解码。
- 验证：`go test ./internal/protocol/...`；确认 `TestDiscoverPageSizeUsesLimit` 等 fork 测试仍通过。

## Task 4 — sdk/client.go 与 options.go（冲突文件，含 V5 接线）

- `sdk/options.go`：合入上游 v0.3.1 对 `Device`/`DeviceResets` 注释的更新 + v0.4.0 的 `SignerURL`/`SignerToken` 字段；**保留** fork 的 `WebOrigin` 字段。
- `sdk/client.go`：三处要合并——
  1. fork 独有：`webOrigin`、`durations`（弹幕）、`resetter` 并发换设备语义、`WebOrigin` 默认值；
  2. v0.3.1：`ResolvePlayback` 走 V5（`sdk/media.go` 在 Task 2/5 处理，但 client 需持有 `playbackSigner *sign.PlaybackV5Signer` 字段，上游在 v0.4.0 的 client.go 引入该字段——实际由 v0.3.1 的 media.go 首次使用，合并时以 `ResolvePlayback` 能编译为准确认字段归属）；
  3. v0.4.0：`SignerURL` 校验 + `newRemoteSigner` 包装 Transport、`doOnce` 中 `ErrSignerUnavailable` 原样返回。
- 注意：上游 v0.4.0 的 `playbackSigner` 初始化用 `sign.NewPlaybackV5Signer(opts.Clock().UnixMilli())`；确认 fork 的 `opts.Clock` 默认值语义一致。
- 验证：`go test ./sdk/...`。

## Task 5 — media.go ResolvePlayback 的 V5 改写（冲突文件）

- fork 相对 v0.3.0 一致，但 Task 4 若已在 client 引入 `playbackSigner`，这里直接取上游 v0.3.1 的 `ResolvePlayback` 实现（V5 优先 + 重签重试 + 404/405 回退旧接口 + `playbackRouteUnavailable`）。
- 确认上层 `withFreshDevice` 与 fork 的 `resetter` 并发语义兼容（上游 v0.3.1 的 media.go 与 fork 一致，可直接取）。
- 验证：契约测试覆盖 V5 成功 / V5 空重签 / 404 回退 / 换设备四条路径。

## Task 6 — 远程签名 core（v0.4.0 新增文件）

- 新增 `sdk/remote_signer.go`、`sdk/remote_signer_test.go`（取上游原文）。
- `sdk/errors.go`：合入 `ErrSignerUnavailable` 语义扩展（注释更新；错误值本身 fork 已有）。
- 确认 `client.go` 的 `ErrSignerUnavailable` 透传在 Task 4 已完成。
- 验证：`go test ./sdk/...`，含 `remote_signer_test.go` 全部用例。

## Task 7 — contract 层（冲突文件）

- `internal/contract/run.go`：合入上游对 contract 的改动，**保留** fork 的 `playSeries` 中 `ErrAnonymousLimited` → `ResetDevice` 换设备重试循环（最多 2 次），使「V5 内部重签 → 旧接口回退 → contract 换设备」三层语义并存。
- `internal/contract/contract_test.go`：取上游新增用例，保留 fork 已有用例。
- 验证：`go test ./internal/contract/...`（live contract 定时器属历史 run，区分对待）。

## Task 8 — dispatch 配置透传（冲突文件，含陷阱）

- `bindings/internal/dispatch/dispatch.go`：新增 `SignerURL`/`SignerToken` 字段与 `Apply` 透传；**保留** fork 的 `WebOrigin` 字段与 `getComicReader` 方法；**不得**带回上游的 `DeviceResets *int` 字段与 `opts.DeviceResets = cfg.DeviceResets`（fork binding 层已删除该透传，core 层 `Options.DeviceResets` 不受影响）。
- `bindings/internal/dispatch/dispatch_test.go`：合入上游 signer 配置用例，保留 fork 用例。
- 验证：`go test ./bindings/internal/...`。

## Task 9 — bindings 各语言 signerUrl/signerToken 透传

逐语言合入上游 v0.4.0 的 client 配置字段与类型（配置字段，非 V5 类型）：

- `bindings/node/index.js`、`index.d.ts`、`package.json`、`types.d.ts`
- `bindings/react-native/src/client.ts`、`types.ts`、`package.json`、`package-lock.json`
- `bindings/mobile/android/src/main/kotlin/dev/openhongguo/Client.kt`、`Types.kt`
- `bindings/mobile/ios/Sources/Openhongguo/Client.swift`、`Types.swift`
- `bindings/flutter/lib/src/client.dart`、`types.dart`、`pubspec.yaml`
- `cmd/ohg/main.go`：CLI 读 `OHG_SIGNER_URL`/`OHG_SIGNER_TOKEN` 环境变量。
- 版本类文件（`package.json`/`pubspec.yaml`/`build.gradle*`/podspec/`Package.swift`）只取 signer 相关语义，**版本号与 AmatsuZero 分发地址保持 fork 现状**，不取上游版本号。
- 验证：`make gen`（生成物 drift 校验）；`make test-node`、`make test-react-native`。

## Task 10 — 文档与 changelog

- `README.md`：合入「选择签名方式」章节、Options 表 `SignerURL/SignerToken` 行、错误码说明；URL 按 fork 规则处理（仓库/CI/Release 用 AmatsuZero，Go module/Android Maven/作者版权保留 beilunyang）。
- `docs/cli.md`、`docs/development.md`、`docs/types.md`：合入上游对应更新。
- flutter `CHANGELOG.md` 及各 binding README：合入 0.3.1/0.4.0 条目，版本表述对齐 fork。
- 验证：人工核对 URL 引用规则。

## Task 11 — 本地验证门禁

1. `make gen`
2. `go test ./...`
3. `make test-node`、`make test-react-native`
4. Swift 壳 `swift build`、Kotlin `compileDebugKotlin`（本地实编）
5. `gofmt -l` 无输出
- 全部通过后汇报，等待用户单独授权发版。

## Task 12 — 发版（需用户单独授权，授权后才执行）

- 推 `xcf-v*` 标签触发 `.github/workflows/xcf.yml`，不在本地打包 xcframework/AAR。
- 推送后用 `gh run -R AmatsuZero/openhongguo` 盯本次 run，全绿再汇报；遇临时 infra 故障等待重试，不削弱门禁。
- semver 标签与 Package.swift 回填按 fork 既有发版流程处理。

## 不纳入本计划

- `signer/` 目录整套、`.github/workflows/signer.yml`、`signer.yml` 推 Docker Hub。
- 上游 `Package.swift` 的版本/URL。
- CrawlerBot hongguo-cli 的 `makeClient()` 改动（在下游仓库，本仓库只保证 binding 暴露配置字段）。
