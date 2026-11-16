# OpenHuangguoCore Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 在独立仓库 `AmatsuZero/openhuangguo-swift` 实现黄果 Go SDK + gomobile Core（OpenHuangguoCore）+ Swift/Kotlin 绑定壳 + 独立发版流水线，不动 hongguo。

**Architecture:** 参考 HGhub（`AmatsuZero/study-materials-fork`）的 edge 信封接口与媒体算法，仿 hongguo SDK 形态。三模块：SDK（业务接口/transport/edge/token/媒体纯函数）、Core（gomobile 按名调用）、gen-huangguo（类型生成，schema 构建期复制改造）。所有复杂数据经 `Start`/callback 的 JSON 通道，gomobile 只导简单类型 + interface。

**Tech Stack:** Go 1.26（SDK 用 1.22 亦可，Core 因 gomobile tool 用 1.26）、gomobile/gobind、标准库 net/http + crypto/aes、Swift 5.9 SwiftPM、Kotlin/Gradle + Media3、GitHub Actions。

**设计依据:** `docs/plans/2026-11-16-openhuangguo-core-design.md`（已通过 codex review）。本计划已纳入实现计划 review 的 13 条修订。

> **关于代码示例**：本计划的 Go 片段给出**接口签名与关键逻辑**，字段细节以"按 types.ts"标注处需照参考文件逐一补全。凡给出完整 struct 的，字段 tag 均已写全、可编译；不得以占位符提交。

---

## 执行前提（涉及外部/共享状态，需用户授权）

1. **创建独立仓库** `AmatsuZero/openhuangguo-swift`（`gh repo create`）。所有实现代码落此仓库，**不写入当前 fork**。
2. 当前 fork 只保留两份设计/计划文档（已 commit，未推送）。是否推送 fork 由用户单独决定。
3. 参考源 `/tmp/study-materials-fork` 为临时克隆；丢失时 `git clone --depth 1 https://github.com/AmatsuZero/study-materials-fork /tmp/study-materials-fork`。
4. **AGhub 没有 JSON fixture testdata**（已核实：`tests/` 只有浏览器测试页）。业务响应 fixture 由我们在 Task 1 按 `src/types.ts` 构造 + 后续从真实接口采集脱敏，不从 HGhub 复制。

> 下列路径相对**独立仓库根**（文中 `repo`）。

## 全局约定

- Module path：
  - SDK：`github.com/AmatsuZero/openhuangguo-swift/bindings/huangguo/sdk`
  - Core：`github.com/AmatsuZero/openhuangguo-swift/bindings/huangguo/core`
  - gen：`github.com/AmatsuZero/openhuangguo-swift/bindings/huangguo/gen-huangguo`
- **本地开发可用临时 go.work（不入库、不进 CI）；CI 一律 `GOWORK=off` 逐 module 跑。**
- 提交粒度：每 Task 末尾一次 commit；TDD（先红后绿）；`gofmt -l` 为空才提交。
- 本地不做 xcframework 编译/打包/发布（走 CI）。
- 关键类型先定义、后被依赖（Transport/Options/Client 在 Task 1 定死，避免后续 Task 引用未定义类型）。

## 任务依赖顺序

```
Task 0 骨架/Makefile/.gitignore/README
  └─ Task 1 SDK 公共契约(Transport/Options/Logger/Client 骨架/fixture 构造)
       ├─ Task 2 model 具名类型
       ├─ Task 3 store + 凭据信任 + SetBase
       ├─ Task 4 edge 信封 + 响应解析
       │    └─ Task 5 全部业务接口 + play 回退（用 Task1 fixture）
       └─ Task 6 媒体纯函数
  └─ Task 7 gen-huangguo（schema 复制改造）
       └─ Task 8 core gomobile 入口 + dispatch
            └─ Task 9 移动端壳（iOS/Android 完整工程）
  └─ Task 10 构建脚本 + hg-xcf.yml + Package.swift + 回填脚本
  └─ Task 11 全量验证门禁
```

---

### Task 0: 仓库骨架、Makefile、忽略规则、README

**Files:**
- Create: `repo/bindings/huangguo/sdk/go.mod`
- Create: `repo/bindings/huangguo/core/go.mod`
- Create: `repo/bindings/huangguo/gen-huangguo/go.mod`
- Create: `repo/Makefile`
- Create: `repo/.gitignore`
- Create: `repo/README.md`
- （不建根 go.mod——三子 module 各自独立，逐 module 构建；临时 go.work 不入库）

**Step 1: clone/init、创建目录、初始化三个 module**

```bash
gh repo create AmatsuZero/openhuangguo-swift   # 授权后
git clone https://github.com/AmatsuZero/openhuangguo-swift.git && cd openhuangguo-swift
mkdir -p bindings/huangguo/{sdk/model,core/dispatch,gen-huangguo,ios/Sources/OpenHuangguo,android/src/main/kotlin/dev/openhuangguo} Sources/OpenHuangguo .github/workflows scripts
cat > bindings/huangguo/sdk/go.mod <<'EOF'
module github.com/AmatsuZero/openhuangguo-swift/bindings/huangguo/sdk

go 1.22
EOF
cat > bindings/huangguo/core/go.mod <<'EOF'
module github.com/AmatsuZero/openhuangguo-swift/bindings/huangguo/core

go 1.26.0

require github.com/AmatsuZero/openhuangguo-swift/bindings/huangguo/sdk v0.0.0

replace github.com/AmatsuZero/openhuangguo-swift/bindings/huangguo/sdk => ../sdk

tool golang.org/x/mobile/cmd/gobind
EOF
cat > bindings/huangguo/gen-huangguo/go.mod <<'EOF'
module github.com/AmatsuZero/openhuangguo-swift/bindings/huangguo/gen-huangguo

go 1.22
EOF
(cd bindings/huangguo/core && go mod tidy)   # 生成独立 go.sum，锁定 gomobile 版本
```

**Step 2: .gitignore（忽略 workspace 与产物）**

```gitignore
go.work
go.work.sum
*.a
*.aar
*.xcframework
*.xcframework.zip
build/
bin/
.openhongguo-build/     # 若借用
.gradle/
**/build/
```

**Step 3: Makefile（逐 module 入口）**

```makefile
MODULES := bindings/huangguo/sdk bindings/huangguo/core bindings/huangguo/gen-huangguo

.PHONY: fmt vet test gen-check all
fmt:
	@for m in $(MODULES); do (cd $$m && gofmt -w $$(find . -name '*.go')); done
vet:
	@for m in $(MODULES); do (cd $$m && GOWORK=off go vet ./...) || exit 1; done
test:
	@for m in $(MODULES); do (cd $$m && GOWORK=off go test ./...) || exit 1; done
gen-check:
	cd bindings/huangguo/gen-huangguo && GOWORK=off go run . --repo-root ../../..
	git diff --exit-code -- bindings/huangguo/ios/Sources/OpenHuangguo/Types.swift bindings/huangguo/android/src/main/kotlin/dev/openhuangguo/Types.kt
all: fmt vet test gen-check
```

**Step 4: README 骨架**

说明：工具链（Go 1.26 / gomobile / Xcode / Android SDK）、生成命令 `make gen-check`、包消费方式（SwiftPM 指向本仓库 `Package.swift` binaryTarget）、**token 持久化失败边界**（Clear 持久化失败不保证跨重启退出，宿主须视为硬错误）、本地不打包走 CI。

**Step 5: Commit**

```bash
git add . && git commit -m "chore: repo skeleton, Makefile, gitignore, README"
```

---

### Task 1: SDK 公共契约 + fixture 构造

**Files:**
- Create: `repo/bindings/huangguo/sdk/transport.go`（接口 + HTTPTransport）
- Create: `repo/bindings/huangguo/sdk/client.go`（Options/Logger/Client 骨架/NewClient）
- Create: `repo/bindings/huangguo/sdk/transport_test.go`
- Create: `repo/bindings/huangguo/sdk/testdata/feed.json`（按 types.ts 构造的首批 fixture）

**Step 1: 定义公共契约（先写类型，后面 Task 都依赖；本 Task 结束时 sdk 包须能独立编译）**

```go
// transport.go
package sdk

import (
    "context"
    "net/http"
)

// Transport 是内部 HTTP 抽象（不 gomobile 绑定）。测试注入 fake 实现。
type Transport interface {
    Do(ctx context.Context, req *Request) (*Response, error)
}

type Request struct {
    Method string
    URL    string
    Headers map[string]string
    Body   []byte
    MaxResponseBytes int64 // 媒体设大值，API 用默认 8MB
}
type Response struct {
    StatusCode int
    Headers    map[string]string
    Body       []byte
}

// TransportConfig 由 configJSON 构造。Proxy: nil=走环境；指向 ""=强制直连；指向 URL=显式代理。
type TransportConfig struct { Proxy *string }

// HTTPTransport 是 Transport 的真实实现，持有可注入的 *http.Client（测试塞 fake RoundTripper）。
type HTTPTransport struct {
    client *http.Client
    cfg    TransportConfig
}
func NewHTTPTransport(cfg TransportConfig) *HTTPTransport {
    return &HTTPTransport{client: &http.Client{Timeout: 30 * time.Second}, cfg: cfg}
}
// Do 的完整实现（代理解析/目标守卫/重定向）在 Task 4 补齐；本 Task 先给能编译的最小 Do
// （含目标守卫，Bearer 原样放行），测试即可验证。
func (t *HTTPTransport) Do(ctx context.Context, req *Request) (*Response, error) { /* Task4 完善 */ }
```

```go
// store.go（本 Task 只放接口与 Session 定义；MemStore/FileStore 实现在 Task 3）
package sdk
import "encoding/json"
type Session struct {
    Origin     string          `json:"origin"`
    Token      string          `json:"token"`
    Generation int64           `json:"generation"`
    User       json.RawMessage `json:"user"`
}
type TokenStore interface {
    Load() (Session, error)
    Save(Session) error
    Clear() error
}
```

**Step 2: Options/Logger/Client 骨架（client.go，本 Task 完整可编译）**

```go
// client.go
package sdk
import "sync"

type Logger interface { Printf(format string, args ...any) }

type Options struct {
    Transport Transport  // 必填
    Store     TokenStore // 必填
    Base      string
    Proxy     *string    // nil=走环境；指向 ""=强制直连；指向 URL=显式代理
    CredTrust map[string]bool
    Logger    Logger
    RetryAttempts int
}

type Client struct {
    mu            sync.Mutex      // 保护 base / snapshot 读取
    transport     Transport
    store         TokenStore
    base          string
    credTrust     map[string]bool
    logger        Logger
    retryAttempts int
}

// NewClient：校验 Transport/Store 必填；base 规范化（normalizeBase 在 Task 3 实现，本 Task 先内联最小校验）
// 默认 credTrust = {https://huangguoai.com:true, https://huangguoai.ai:true}；不发请求。
func NewClient(o Options) (*Client, error) { /* ... */ }

// snapshot：原子读 {origin, token, generation} + 当前 base，供 edge 全程使用同一份（Task 4 使用）。
// 定义放在这里，保证 Task 1 编译闭合。
func (c *Client) snapshot() (Session, string) {
    c.mu.Lock(); base := c.base; c.mu.Unlock()
    s, _ := c.store.Load()
    return s, base
}
```

> **本 Task 完成标志**：`GOWORK=off go build ./...` 通过（HTTPTransport/TransportConfig/Session/TokenStore/Client/snapshot 均已定义）；Task 3 只**新增** MemStore/FileStore/normalizeBase/SetBase，不改这些签名。

**Step 3: 写失败测试（Bearer 用注入 fake，不用回环 server）**

```go
type fakeTransport struct { gotAuth string; resp *Response }
func (f *fakeTransport) Do(ctx context.Context, r *Request) (*Response, error) {
    f.gotAuth = r.Headers["Authorization"]
    return f.resp, nil
}
func TestTransportAllowsBearer(t *testing.T) {
    // 用 fakeTransport 注入，断言 Authorization 头原样送出不报错（区别于 hongguo 的拒绝）
}
func TestTargetGuards(t *testing.T) {
    // 独立测试：https://u:p@example.com/ 与 http://127.0.0.1:1/ 被 HTTPTransport 拒绝
    // （用真实 HTTPTransport，不发网络：校验在 Do 开头目标检查阶段）
}
```

**Step 4: 构造 fixture（按 types.ts，不依赖 HGhub）**

```bash
# testdata/feed.json：{"status":1,"data":{"items":[{...VideoCard 全字段...}],"pagination":{...}}}
# 每个可选字段至少覆盖"存在"与"缺失"两种，供 Task 5/11 校验字段解析
```

**Step 5: 验证**

```bash
cd repo/bindings/huangguo/sdk && GOWORK=off go test ./... -v && GOWORK=off go vet ./...
```
Expected: PASS（transport 守卫测试通过；TokenStore 接口已声明故编译过）。

**Step 6: Commit**

```bash
git commit -m "feat(sdk): transport interface, client options and initial fixtures"
```

---

### Task 2: model 具名类型（完整可编译代码，无占位）

**Files:**
- Create: `repo/bindings/huangguo/sdk/model/model.go`
- Create: `repo/bindings/huangguo/sdk/model/model_test.go`

**Step 1: 写失败测试（完整解码 + 序列化键名断言）**

```go
package model

import (
    "encoding/json"
    "strings"
    "testing"
)

func TestVideoCardRoundTrip(t *testing.T) {
    raw := `{"id":1,"title":"t","cover":"c","description":"d","tags":["a"],"score":9,
      "episode_count":2,"total_episodes":3,"hot":100,"like_count":3,"collect_count":4,
      "comment_count":5,"is_finished":true,"is_original":1,"author":{"id":7,"name":"n","followers":8}}`
    var v VideoCard
    if err := json.Unmarshal([]byte(raw), &v); err != nil { t.Fatal(err) }
    if v.ID != 1 || v.Author == nil || v.Author.ID != 7 || v.Author.Followers != 8 { t.Fatalf("bad: %+v", v) }
    out, _ := json.Marshal(v)
    for _, key := range []string{`"episode_count"`, `"total_episodes"`, `"is_finished"`, `"like_count"`} {
        if !strings.Contains(string(out), key) { t.Fatalf("missing key %s in %s", key, out) }
    }
}

func TestDetailAndPlayInfoNested(t *testing.T) {
    raw := `{"id":1,"title":"t","description":"d","cover":"c","tags":[],
      "score":9,"episode_count":1,"total_episodes":1,"episodes":[],
      "counts":{"collect":1,"comment":2,"hot":3,"like":4},
      "author":{"id":3,"name":"n","is_following":true,"following":5,"like_count":6},
      "is_finished":true,"is_original":0,"video_url":""}`
    var d Detail
    if err := json.Unmarshal([]byte(raw), &d); err != nil { t.Fatal(err) }
    if d.Counts.Like != 4 || d.Author.ID != 3 || !d.Author.IsFollowing { t.Fatalf("bad: %+v", d) }
    var p PlayInfo
    if err := json.Unmarshal([]byte(`{"id":1,"ep_num":2,"video_url":"http://m","streams":{"main":"http://m"}}`), &p); err != nil { t.Fatal(err) }
    if p.EpNum != 2 || p.Streams == nil || p.Streams.Main != "http://m" { t.Fatalf("bad: %+v", p) }
}
```

**Step 2: 运行确认失败**

```bash
cd repo/bindings/huangguo/sdk && GOWORK=off go test ./model/ -v
```
Expected: FAIL（类型未定义）。

**Step 3: 实现完整具名类型（每字段独立 tag，无简写、无重复定义）**

`model.go`（对照 `/tmp/study-materials-fork/src/types.ts` 逐字段补全，以下为可直接编译的骨架，标注处按 types.ts 补齐）：

```go
package model

type Author struct {
    ID        int64  `json:"id"`
    Name      string `json:"name"`
    Avatar    string `json:"avatar,omitempty"`
    Followers int    `json:"followers,omitempty"`
}

// DetailAuthor 是详情里的作者（TS 的 Author & {is_following?...}），单独具名
type DetailAuthor struct {
    ID          int64  `json:"id"`
    Name        string `json:"name"`
    Avatar      string `json:"avatar,omitempty"`
    IsFollowing bool   `json:"is_following,omitempty"`
    Following   int    `json:"following,omitempty"`
    LikeCount   int    `json:"like_count,omitempty"`
}

type VideoCard struct {
    ID            int64    `json:"id"`
    Title         string   `json:"title"`
    Cover         string   `json:"cover"`
    Description   string   `json:"description,omitempty"`
    Tags          []string `json:"tags"`
    Score         float64  `json:"score"`
    EpisodeCount  int      `json:"episode_count"`
    TotalEpisodes int      `json:"total_episodes"`
    Duration      string   `json:"duration,omitempty"`
    Hot           int64    `json:"hot"`
    LikeCount     int64    `json:"like_count"`
    CollectCount  int64    `json:"collect_count"`
    CommentCount  int64    `json:"comment_count"`
    IsFinished    bool     `json:"is_finished"`
    IsOriginal    int      `json:"is_original"`
    CreatedAt     string   `json:"created_at,omitempty"`
    LastEpAt      *int64   `json:"last_ep_at,omitempty"`
    NewAt         *int64   `json:"new_at,omitempty"`
    Author        *Author  `json:"author,omitempty"`
}

type Pagination struct {
    Page  int `json:"page"`
    Pages int `json:"pages"`
    Size  int `json:"size"`
    Total int `json:"total"`
}
// 分页具名实例（避免泛型，供 schema AST 解析）
type VideoCardPage struct {
    Items      []VideoCard `json:"items"`
    Pagination Pagination  `json:"pagination"`
}
type PostPage struct {
    Items      []Post     `json:"items"`
    Pagination Pagination `json:"pagination"`
}
type CommentPage struct {
    EP         int        `json:"ep"`
    Revision   string     `json:"revision,omitempty"`
    Reset      bool       `json:"reset,omitempty"`
    Items      []Comment  `json:"items"`
    Pagination Pagination `json:"pagination"`
}
type SearchResult struct {
    Items      []VideoCard `json:"items"`
    Pagination Pagination  `json:"pagination"`
    Keyword    string      `json:"keyword"`
}

type Episode struct {
    EpNum    int    `json:"ep_num"`
    Title    string `json:"title"`
    Duration int    `json:"duration"`
    Status   int    `json:"status"`
}
type TagLink struct {
    Name string `json:"name"`
    URL  string `json:"url"`
}
type DetailCounts struct {
    Collect int64 `json:"collect"`
    Comment int64 `json:"comment"`
    Hot     int64 `json:"hot"`
    Like    int64 `json:"like"`
}
type Interaction struct {
    Collected bool `json:"collected"`
    Liked     bool `json:"liked"`
    LikedEP   int  `json:"liked_ep,omitempty"`
}
type Detail struct {
    ID            int64         `json:"id"`
    Title         string        `json:"title"`
    Description   string        `json:"description"`
    Cover         string        `json:"cover"`
    Tags          []string      `json:"tags"`
    TagLinks      []TagLink     `json:"tag_links,omitempty"`
    Score         float64       `json:"score"`
    EpisodeCount  int           `json:"episode_count"`
    TotalEpisodes int           `json:"total_episodes"`
    Episodes      []Episode     `json:"episodes"`
    Counts        DetailCounts  `json:"counts"`
    Author        DetailAuthor  `json:"author"`
    Interaction   *Interaction  `json:"interaction,omitempty"`
    VideoURL      string        `json:"video_url,omitempty"`
    VideoURLHevc  string        `json:"video_url_hevc,omitempty"`
    IsFinished    bool          `json:"is_finished"`
    IsOriginal    int           `json:"is_original"`
    VideoType     int           `json:"video_type,omitempty"`
    Duration      int           `json:"duration,omitempty"`
    CreatedAt     string        `json:"created_at,omitempty"`
}

type Streams struct {
    Main    string `json:"main,omitempty"`
    Hevc    string `json:"hevc,omitempty"`
    Preview string `json:"preview,omitempty"`
}
type PlayInfo struct {
    ID           int64    `json:"id"`
    EpNum        int      `json:"ep_num"`
    Duration     int      `json:"duration"`
    Title        string   `json:"title"`
    VideoURL     string   `json:"video_url"`
    VideoURLHevc string   `json:"video_url_hevc,omitempty"`
    Streams      *Streams `json:"streams,omitempty"`
}

type RankMetrics struct {
    Collect int64 `json:"collect"`
    Comment int64 `json:"comment"`
    Like    int64 `json:"like"`
    Play    int64 `json:"play"`
}
type RankItem struct {
    Rank         int64       `json:"rank"`
    VideoID      int64       `json:"video_id"`
    Title        string      `json:"title"`
    Cover        string      `json:"cover"`
    Tags         []string    `json:"tags"`
    Score        float64     `json:"score"`
    CateName     string      `json:"cate_name"`
    EpisodeCount int         `json:"episode_count"`
    IsFinished   bool        `json:"is_finished"`
    MetricLabel  string      `json:"metric_label"`
    MetricValue  int64       `json:"metric_value"`
    Metrics      RankMetrics `json:"metrics"`
}

type Comment struct {
    ID         int64      `json:"id"`
    UserID     int64      `json:"user_id"`
    Nickname   string     `json:"nickname"`
    Avatar     string     `json:"avatar"`
    AvatarColor string    `json:"avatar_color"`
    Text       string     `json:"text"`
    Time       string     `json:"time"`
    CreatedAt  string     `json:"created_at,omitempty"`
    LikeCount  int64      `json:"like_count"`
    Liked      bool       `json:"liked"`
    ReplyCount int        `json:"reply_count"`
    ReplyID    int64      `json:"reply_id"`
    ReplyList  []Comment  `json:"reply_list,omitempty"`
    IsPinned   bool       `json:"is_pinned,omitempty"`
    IPRegion   string     `json:"ip_region,omitempty"`
}

type PostAuthor struct {
    ID          int64  `json:"id"`
    Name        string `json:"name"`
    Avatar      string `json:"avatar,omitempty"`
    IsFollowing bool   `json:"is_following,omitempty"`
}
type PostCounts struct {
    Like    int64 `json:"like"`
    Comment int64 `json:"comment"`
    Collect int64 `json:"collect"`
    Share   int64 `json:"share,omitempty"`
    Hot     int64 `json:"hot,omitempty"`
}
type Post struct {
    ID          int64      `json:"id"`
    Title       string     `json:"title"`
    Desc        string     `json:"desc,omitempty"`
    Content     string     `json:"content,omitempty"`
    Cover       string     `json:"cover"`
    Tags        []string   `json:"tags"`
    Hot         int64      `json:"hot"`
    VideoURL    string     `json:"video_url,omitempty"`
    VideoID     int64      `json:"video_id,omitempty"`
    Author      *PostAuthor `json:"author,omitempty"`
    Counts      *PostCounts `json:"counts,omitempty"`
    CreatedAt   string     `json:"created_at,omitempty"`
    PublishTime string     `json:"publish_time,omitempty"`
}

type LikeVideoEpisode struct {
    VideoID int64 `json:"video_id"`
    EP      int   `json:"ep"`
}
type UserInteractions struct {
    LikeVideoIDs     []int64           `json:"like_video_ids"`
    FavoriteVideoIDs []int64           `json:"favorite_video_ids"`
    FollowUserIDs    []int64           `json:"follow_user_ids"`
    LikeVideoEpisodes []LikeVideoEpisode `json:"like_video_episodes"`
}
type User struct {
    ID          int64            `json:"id"`
    Username    string           `json:"username"`
    Nickname    string           `json:"nickname"`
    Avatar      string           `json:"avatar"`
    AvatarColor string           `json:"avatar_color"`
    Sign        string           `json:"sign,omitempty"`
    Desc        string           `json:"desc,omitempty"`
    UserType    string           `json:"user_type,omitempty"`
    Counts      map[string]int64 `json:"counts,omitempty"`
    Interactions *UserInteractions `json:"interactions,omitempty"`
}
```

**Step 4: 运行确认通过**

```bash
cd repo/bindings/huangguo/sdk && GOWORK=off go test ./model/ -v && GOWORK=off go vet ./...
```
Expected: PASS。

**Step 5: Commit**

```bash
git commit -m "feat(sdk): complete named model types aligned with types.ts"
```

---

### Task 3: store、凭据信任、SetBase

**Files:**
- Create: `repo/bindings/huangguo/sdk/store.go`（Task1 声明的接口，这里实现）
- Create: `repo/bindings/huangguo/sdk/client_base.go`（base 规范化/凭据信任/SetBase/do 取快照）
- Create: `repo/bindings/huangguo/sdk/store_test.go`

**Step 1: 写失败测试**

```go
package sdk
func TestSessionOriginBindingAndGeneration(t *testing.T) {
    m := NewMemStore()
    m.Save(Session{Origin:"https://huangguoai.com", Token:"tok", Generation:1, User: json.RawMessage(`{}`)})
    if _, err := m.Load(); err != nil { t.Fatal(err) }
    // 换 origin 读应拒绝；Clear 后 Load 为空；generation 递增
}
func TestNormalizeBaseAndCredentialTrust(t *testing.T) {
    for _, raw := range []string{"http://huangguoai.com", "https://u:p@huangguoai.com", "https://huangguoai.com/x", "http://127.0.0.1"} {
        if _, err := normalizeBase(raw); err == nil { t.Fatalf("should reject %s", raw) }
    }
    c, _ := NewClient(Options{Transport: &fakeTransport{}, Store: NewMemStore(), Base:"https://huangguoai.ai"})
    if !c.allowedCredentialTarget() { t.Fatal("huangguoai.ai should be trusted") }
}
func TestSetBaseClearsToken(t *testing.T) { /* ... */ }
```

**Step 2: 运行确认失败**

```bash
cd repo/bindings/huangguo/sdk && GOWORK=off go test . -run 'TestSession|TestNormalizeBase|TestSetBase' -v
```
Expected: FAIL。

**Step 3: 实现**

`store.go`：
```go
type Session struct {
    Origin     string          `json:"origin"`      // 显式小写 tag，GetTokenJSON 输出稳定
    Token      string          `json:"token"`
    Generation int64           `json:"generation"`
    User       json.RawMessage `json:"user"`
}
// TokenStore 接口已在 Task1 client.go 声明；这里给 MemStore / FileStore
type MemStore struct { mu sync.Mutex; s Session }
func NewMemStore() *MemStore { ... }
func (m *MemStore) Load() (Session, error) { /* mu.Lock; return copy */ }
func (m *MemStore) Save(s Session) error  { /* mu.Lock; m.s = s */ }
func (m *MemStore) Clear() error          { /* mu.Lock; m.s = Session{} */ }
// FileStore：0600 + tmp+rename 原子写；Clear 删文件失败必须返回错误
```

`client_base.go`：
```go
// normalizeBase：HTTPS 纯 origin，无 userinfo/path/query/fragment，禁内网/回环
func normalizeBase(raw string) (string, error) { /* url.Parse + 校验 + net.ParseIP 守卫 */ }

// allowedCredentialTarget 用规范化后的完整 origin（含端口）比较信任名单，不用 hostOf
func (c *Client) allowedCredentialTarget() bool {
    return c.credTrust[c.base] // c.base 是规范化完整 origin
}

// SetBase：mu 锁；generation++；store.Clear()；返回 Clear 结果（持久化失败不谎称成功）
func (c *Client) SetBase(raw string) error {
    b, err := normalizeBase(raw); if err != nil { return err }
    c.mu.Lock(); defer c.mu.Unlock()
    c.base = b
    s, _ := c.store.Load(); s.Generation++; s.Origin = ""; s.Token = ""; s.User = nil
    return c.store.Save(s)
}

// 说明：snapshot() 已在 Task 1 定义（返回 (Session, string)，含当前 base），本 Task 不再重复定义。
// allowedCredentialTarget 改为接受显式 origin 参数，避免读取可变的 c.base（见 Task 4 用法）。
func (c *Client) allowedCredentialTarget(origin string) bool {
    return c.credTrust[origin] // origin 为规范化完整 origin（含端口）
}
```

- 凭据检查在 `edge` 里对**所有携带密码或非空 Bearer** 的请求做（不限 needCred 分支）；用快照里的 origin 判断，不用 c.base。

**Step 4: 运行确认通过**

```bash
cd repo/bindings/huangguo/sdk && GOWORK=off go test . -run 'TestSession|TestNormalizeBase|TestSetBase' -v && GOWORK=off go vet ./...
```
Expected: PASS。

**Step 5: Commit**

```bash
git commit -m "feat(sdk): token stores, credential trust and origin-bound base switching"
```

---

### Task 4: edge 信封与响应解析（含 HTTP 状态处理）

**Files:**
- Create: `repo/bindings/huangguo/sdk/edge.go`
- Create: `repo/bindings/huangguo/sdk/edge_test.go`

**Step 1: 写失败测试（HTTP 状态 × body 组合）**

```go
package sdk
func TestEdgeHTTPStatusBeforeEnvelope(t *testing.T) {
    cases := []struct{ status int; body string; wantErr bool }{
        {200, `{"status":1,"data":{}}`, false},
        {200, ``, false},                    // 空 body 成功
        {200, `null`, false},                // null 成功
        {200, `{}`, true},                   // 裸 {} 失败
        {401, ``, true},                     // 401 即使空 body 也失败
        {500, ``, true},                     // 500 失败（且可重试分类）
        {200, `{"status":0,"msg":"bad"}`, true},
    }
    // 用 fakeTransport 返回对应 Response，断言 edge 结果
}
func TestEdgeEnvelopeFields(t *testing.T) {
    // 用 HTTPTransport + 注入 fake RoundTripper（不走网络、不用回环）：
    //   捕获 *http.Request，断言外层 Content-Type: application/json、Authorization: Bearer ...
    //   解码请求体为 edgeEnvelope，断言 method/path、无 body 时两空串、有 body 时 base64+application/json
    // 这同时证明真实 HTTPTransport 放行 Bearer（fakeTransport 无法证明这点）
}
```

**Step 2: 运行确认失败**

```bash
cd repo/bindings/huangguo/sdk && GOWORK=off go test . -run 'TestEdge' -v
```
Expected: FAIL。

**Step 3: 实现 edge（先处理 HTTP 状态，再解析信封）**

```go
type edgeEnvelope struct {
    Method string `json:"method"`
    Path   string `json:"path"`
    Body   string `json:"body"`
    ContentType string `json:"content_type"`
}
type CodeError struct { Code int; Message, Name, Kind string }
func (e *CodeError) Error() string { return e.Message }

func (c *Client) edge(ctx context.Context, method, path string, body any, needCred bool) ([]byte, error) {
    // 1. 一次取快照：s, base := c.snapshot()（token + 当前 base 同一份，全程用它）
    s, base := c.snapshot()
    // 2. 凭据检查用快照 base：若 needCred 或 s.Token!="" → !allowedCredentialTarget(base) 则拒绝
    //    （镜像不收密码/Bearer；远端 URL 也用 base 拼，不用 c.base）
    // 3. 编码信封（marshal 错误必须传播）
    enc, ctype := "", ""
    if body != nil {
        b, err := json.Marshal(body); if err != nil { return nil, err }
        enc = base64.StdEncoding.EncodeToString(b); ctype = "application/json"
    }
    payload, err := json.Marshal(edgeEnvelope{method, path, enc, ctype}); if err != nil { return nil, err }
    // 4. headers：Accept、Content-Type: application/json（外层）、Referer: base+"/"
    //    Bearer 来自快照 s.Token；URL = base+"/api/edge/request"（全程同一 base）
    // 5. resp := c.transport.Do(ctx, &Request{Method:"POST", URL: base+"/api/edge/request", ...})
    // 6. 先判 HTTP 状态：401/403→ErrAuth；4xx→错误；5xx/408/425/429→可重试错误；非 2xx→错误
    // 7. 提交登录结果前再校验 generation 未变（s.Generation==当前），否则丢弃
    // 8. 再 parseEdgeResponse(resp.Body)
}

func parseEdgeResponse(raw []byte) ([]byte, error) {
    s := strings.TrimSpace(string(raw))
    if s == "" || s == "null" { return []byte("null"), nil }
    var v struct {
        Status *int `json:"status"`
        Data json.RawMessage `json:"data"`
        Msg string `json:"msg"`
    }
    if err := json.Unmarshal(raw, &v); err != nil { return nil, err }
    if v.Status != nil && *v.Status == 1 {
        if len(v.Data) == 0 { return []byte("null"), nil }
        return v.Data, nil
    }
    if v.Msg != "" { return nil, &CodeError{Message: v.Msg} }
    return nil, &CodeError{Message: "接口返回失败"}
}
```

- 外层 `Content-Type: application/json` 必设；信封内 `content_type` 描述的是 body 的媒体类型，二者不同。

**Step 4: 运行确认通过**

```bash
cd repo/bindings/huangguo/sdk && GOWORK=off go test . -run 'TestEdge' -v && GOWORK=off go vet ./...
```
Expected: PASS。

**Step 5: Commit**

```bash
git commit -m "feat(sdk): edge envelope with HTTP status handling and exact response semantics"
```

---

### Task 5: 全部业务接口 + play 回退

**Files:**
- Create: `repo/bindings/huangguo/sdk/api.go`
- Create: `repo/bindings/huangguo/sdk/api_test.go`
- Modify: `repo/bindings/huangguo/sdk/testdata/*.json`（补齐各接口 fixture）

**Step 1: 写失败测试（调用具名方法，验证模型字段而非裸字节）**

```go
package sdk
func TestFeedParsesFixture(t *testing.T) {
    fake := &fakeTransport{resp: &Response{StatusCode:200, Body: mustRead("testdata/feed.json")}}
    c, _ := NewClient(Options{Transport: fake, Store: NewMemStore(), Base:"https://huangguoai.com"})
    page, err := c.Feed(ctx, 1, 20)
    if err != nil { t.Fatal(err) }
    if len(page.Items) == 0 || page.Pagination.Total == 0 { t.Fatalf("fields lost: %+v", page) }
}
func TestPlayFallbackAndStreams(t *testing.T) {
    // 新接口 streams.main → 回填 VideoURL；无地址时回退旧 GET /play
}
func TestAllEndpointsPaths(t *testing.T) {
    // fakeTransport 记录 req.URL/method 断言 path（对照 api.rs）
}
```

**Step 2: 运行确认失败**

```bash
cd repo/bindings/huangguo/sdk && GOWORK=off go test . -run 'TestFeed|TestPlay|TestAllEndpoints' -v
```
Expected: FAIL。

**Step 3: 实现 api.go（方法表对照 api.rs:314-626）**

方法→`method/path`（用 `url.QueryEscape` 编码 query）：
```
Login  POST /api/account/loginByPassword      {username,password}
Register POST /api/account/registerByPassword  {username,password,confirm_password,email?}
Logout POST /api/account/logout
Me     GET  /api/account/me
Feed   GET  /api/videos?page=&size=
Category GET /api/videos/category/{cate}?page=&size=
Rank   GET  /api/ranks/{kind}    (kind 限 recommend|potential|hot，否则 hot)
Search GET  /api/search?q=<escaped>&page=&size=
Posts  GET  /api/posts?page=&size=
Post   GET  /api/posts/{id}
Video  GET  /api/videos/{id}
Play   见下
Comments GET /api/videos/{id}/comments?ep=&page=&size=
CommentSend POST /api/videos/{id}/comments[?ep=]  {text,reply_id?}
Like   POST|DELETE /api/videos/{id}/like?ep=
Collect POST|DELETE /api/videos/{id}/collect
Follow POST|DELETE /api/users/{uid}/follow
PostLike POST|DELETE /api/posts/{id}/like
PostCollect POST|DELETE /api/posts/{id}/collect
PostComments GET /api/posts/{id}/comments?page=&size=
```
`SetBase` 已在 Task 3 实现。

`Play`（照 `api.rs:499-528`）：
```go
func (c *Client) Play(ctx context.Context, id int64, ep int) (*model.PlayInfo, error) {
    // 1. POST /api/videos/{id}/episodes/{ep}/playback  body {}
    // 2. normalizeStreams：streams.main/hevc 回填 VideoURL/VideoURLHevc（原字段空时）
    // 3. video_url 非空→返回；否则 GET /api/videos/{id}/play?ep={ep}&page_texts=0，normalize 后返回
    // 4. 两者都失败→返回新接口结果/错误
}
```
写操作（login/register/commentSend/like/collect/follow/postLike/postCollect）`needCred=true`（发送密码或 Bearer 时凭据信任检查在 edge 统一做），禁止盲重试。

**Step 4: 运行确认通过**

```bash
cd repo/bindings/huangguo/sdk && GOWORK=off go test ./... -v && GOWORK=off go vet ./...
```
Expected: PASS。

**Step 5: Commit**

```bash
git commit -m "feat(sdk): all business endpoints, play fallback and fixtures"
```

---

### Task 6: 媒体纯函数

**Files:**
- Create: `repo/bindings/huangguo/sdk/media.go`
- Create: `repo/bindings/huangguo/sdk/media_test.go`

**Step 1: 写失败测试（对齐 media.rs）**

```go
package sdk
func TestDecryptCover(t *testing.T) {
    // 明文 JPEG/PNG 直接透传；密文解密 + 去 padding；无效 padding 保留原文
}
func TestRewritePlaylist(t *testing.T) {
    in := "\ufeff#EXTM3U\n#EXT-X-CONTENT-STEERING:URI=\"x\"\n#EXT-X-DATERANGE:ID=\"a\",X-ASSET-LIST=\"b\",DURATION=1\n#EXT-X-KEY:METHOD=AES-128,URI=\"key.bin\"\nseg-1.ts\nsub/list.m3u8\n"
    // 注意 localScheme="hg", host="localhost"（不是 "hg://localhost" 两次）
    got := RewritePlaylist("https://cdn.example/master.m3u8", "hg", "localhost", in)
    // 断言：STEERING 删除；DATERANGE 去 URI/X-ASSET-LIST 保留 DURATION；
    // KEY URI / 分片 / 子清单 改写为 hg://localhost/seg?u=<pct>
}
func TestFetchSegmentRange(t *testing.T) {
    // fakeTransport 返回 206 + Content-Range；断言 SegmentResult.Headers 含 Content-Range
    // Body 含 0x00/0xff 往返
}
```

**Step 2: 运行确认失败**

```bash
cd repo/bindings/huangguo/sdk && GOWORK=off go test . -run 'TestDecrypt|TestRewrite|TestFetchSegment' -v
```
Expected: FAIL。

**Step 3: 实现（算法照 media.rs）**

```go
package sdk

const imgKey = "f5d965df75336270"
const imgIV  = "97b60394abc2fbe1"

func imageKind(buf []byte) string { /* JPEG/PNG/GIF/WEBP magic，照 media.rs:87-104 */ }

// DecryptCover：明文图片直接返回；否则 AES-128-CBC 解密 + 条件去 padding（无效 padding 原样保留）
func DecryptCover(data []byte) ([]byte, error) { /* cipher.NewCBCDecrypter + XOR prev 链 */ }

// RewritePlaylist：upstreamURL 作相对 URI 解析基准；localScheme+host 拼本地协议前缀。
// 行为矩阵照 media.rs:210-260：BOM；CONTENT-STEERING 删除；DATERANGE 过滤 URI/X-ASSET-LIST；
// 含 URI=" 的属性行绝对化+改写；分片行绝对化+改写；子清单行同样改写。
func RewritePlaylist(upstreamURL, localScheme, host, text string) string { /* ... */ }

// SegmentResult 仅 SDK 内部，经 core 的 Start/callback JSON 传出；Body []byte 经 JSON 为 base64 字符串
type SegmentResult struct {
    Status  int               `json:"status"`
    Headers map[string]string `json:"headers"` // Content-Range / Accept-Ranges / Content-Length
    Body    []byte            `json:"body"`
}
// FetchSegment：用共享 transport；MaxResponseBytes 设大值（如 32MB）；透传 Range 请求头；返回 206 状态与响应头
func (c *Client) FetchSegment(ctx context.Context, url string, reqHeader map[string]string) (SegmentResult, error) { /* ... */ }
```

- AES CBC：照 `media.rs:56-75` 手写块解密 + XOR 前一块。
- 属性解析 `hlsAttributes`/`mapUriAttributes` 复刻 `media.rs:148-190`（带逗号引号值）。
- 子清单识别不只看 `.m3u8`：宿主播放器层还要按 MIME / `#EXTM3U` 内容判断（照 `media.rs:322`）。

**Step 4: 运行确认通过**

```bash
cd repo/bindings/huangguo/sdk && GOWORK=off go test . -run 'TestDecrypt|TestRewrite|TestFetchSegment' -v && GOWORK=off go vet ./...
```
Expected: PASS。

**Step 5: Commit**

```bash
git commit -m "feat(sdk): pure media tools (decrypt, rewrite, segment with Range)"
```

---

### Task 7: gen-huangguo（schema 复制改造）

**Files:**
- Create: `repo/bindings/huangguo/gen-huangguo/schema/` （复制 hongguo `bindings/internal/schema`）
- Create: `repo/bindings/huangguo/gen-huangguo/main.go`
- Create: `repo/bindings/huangguo/gen-huangguo/main_test.go`

**关键事实（codex #5）**：
- `schema.Load(root)` 期望 `root/sdk/model`（响应模型）和 `root/sdk`（请求参数类型 QueryTypes）。我们的结构 `bindings/huangguo/sdk/model` → **root 应为 `bindings/huangguo`**（`Load` 内部拼 `sdk/model` 和 `sdk` 正好命中）。
- **黄果业务方法参数是原始值 `(id,ep,page,size)`，没有 Options/Query 结构体** → 黄果的 `Queries` 应为空。不要再传分页类型当 QueryTypes（那会生成请求编码类型且改变字段规则）。
- 跨 module 用 `go run . --repo-root <path>`，不能在仓库根 `go run ./bindings/...`。

**Step 1: 复制 schema 并改造 loader**

```bash
cp -R /Users/daubert/Github/openhongguo/bindings/internal/schema repo/bindings/huangguo/gen-huangguo/schema
```
改造 `schema/schema.go`：
```go
// Load(root) 保留；新增 LoadModelOnly(root) 只解析 root/sdk/model 的响应模型，不查 QueryTypes
func LoadModelOnly(root string) (*Schema, error) {
    // 复用 parseDir(filepath.Join(root,"sdk","model")) + convert + consts
    // s.Queries 留空（黄果无请求 DTO）
}
```

**Step 2: 写失败测试**

```go
package main
func TestGenerateHuangguoTypes(t *testing.T) {
    // 测试在 gen 模块目录 bindings/huangguo/gen-huangguo 运行：
    //   schema 的 huangguo 根 = <repo>/bindings/huangguo = ..（LoadModelOnly 内部拼 <root>/sdk/model）
    //   注意：仓库根才是 ../../..，那是 main.go 的 --repo-root 传值，不是这里的参数
    huangguoRoot := ".."
    s, err := schema.LoadModelOnly(huangguoRoot) // 内部拼 <root>/sdk/model = bindings/huangguo/sdk/model
    if err != nil { t.Fatal(err) }
    swift := swiftTypes(s); kotlin := kotlinTypes(s)
    for _, want := range []string{"struct VideoCard", "struct Detail", "struct PlayInfo"} {
        if !strings.Contains(swift, want) { t.Fatalf("missing %s", want) }
    }
    if !strings.Contains(kotlin, "data class VideoCard") { t.Fatal("missing Kotlin VideoCard") }
}
```

**Step 3: 实现 generator（照 hongguo bindings/mobile/gen 的 swift()/kotlin()）**

```go
// main.go
// flag: --repo-root（默认 "."，指仓库根 repo/）；
//   schema.LoadModelOnly(filepath.Join(repoRoot, "bindings", "huangguo"))
// 输出路径：
//   <repoRoot>/bindings/huangguo/ios/Sources/OpenHuangguo/Types.swift
//   <repoRoot>/bindings/huangguo/android/src/main/kotlin/dev/openhuangguo/Types.kt
// 生成规则照搬：JSON 名→驼峰；零值缺省；指针可空；枚举具名。
// 先 os.MkdirAll 输出目录再写文件。
```

**Step 4: 验证生成一致性**

```bash
cd repo/bindings/huangguo/gen-huangguo && GOWORK=off go test ./... -v
cd repo/bindings/huangguo/gen-huangguo && GOWORK=off go run . --repo-root ../../..
cd repo && git diff --exit-code -- bindings/huangguo/ios/Sources/OpenHuangguo/Types.swift bindings/huangguo/android/src/main/kotlin/dev/openhuangguo/Types.kt
```
Expected: PASS；生成文件已提交且无 drift。

**Step 5: Commit**

```bash
git add bindings/huangguo/gen-huangguo bindings/huangguo/ios/Sources/OpenHuangguo/Types.swift bindings/huangguo/android/src/main/kotlin/dev/openhuangguo/Types.kt
git commit -m "feat(gen): huangguo type generator with model-only schema loader"
```

---

### Task 8: core gomobile 入口 + dispatch

**Files:**
- Create: `repo/bindings/huangguo/core/core.go`
- Create: `repo/bindings/huangguo/core/dispatch/dispatch.go`
- Create: `repo/bindings/huangguo/core/core_test.go`
- Create: `repo/bindings/huangguo/core/dispatch/dispatch_test.go`

**Step 1: 写失败测试**

```go
package core
func TestStartEnvelope(t *testing.T) {
    // 调 Start("feed", "[1,20]", cb)，断言 cb 收到 {"result":...}；错误时 {"error":{code,...}}
}
func TestTokenBridgeRoundTrip(t *testing.T) {
    c.SetTokenJSON(`{"origin":"https://huangguoai.com","token":"t","generation":1}`)
    got, _ := c.GetTokenJSON()
    // 断言 origin/token/generation 字段小写且值一致
}
```

**Step 2: 运行确认失败**

```bash
cd repo/bindings/huangguo/core && GOWORK=off go test ./... -v
```
Expected: FAIL。

**Step 3: 实现 core.go（照 hongguo core.go:37-102）**

```go
package core

type Logger interface { Log(line string) }
type Callback interface { Done(resultJSON string) }

type Client struct {
    c     *sdk.Client
    store sdk.TokenStore // 持有 store 引用（类型来自 SDK），供桥接访问；不通过构造器注入接口给外部
}

// 构造器只收 configJSON + logger（gomobile 可绑定）
func NewClient(configJSON string, logger Logger) (*Client, error) {
    var cfg struct {
        Base string `json:"base"`
        Proxy *string `json:"proxy"`  // nil 缺省走环境；"" 强制直连
        CredTrust map[string]bool `json:"credTrustOrigins"`
    }
    if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil { return nil, jsonError(err) }
    store := sdk.NewMemStore() // 默认内存；原生层用 SetTokenJSON 回写/恢复
    c, err := sdk.NewClient(sdk.Options{Transport: sdk.NewHTTPTransport(sdk.TransportConfig{Proxy: cfg.Proxy}), Store: store, Base: cfg.Base, CredTrust: cfg.CredTrust})
    return &Client{c: c, store: store}, err
}

// token 桥接：单返回值 + error，gomobile 可绑定。
// SetTokenJSON 仅用于**启动恢复**：把原生持久化快照灌入内存 store，不得在会话中调用覆盖
// SDK 正在管理的 generation（否则旧快照会覆盖新状态）。登录/登出/切 base 的状态变化走 dispatch 方法。
func (cl *Client) SetTokenJSON(s string) error {
    var sess sdk.Session
    if err := json.Unmarshal([]byte(s), &sess); err != nil { return err }
    return cl.store.Save(sess)
}
func (cl *Client) GetTokenJSON() (string, error) {
    sess, err := cl.store.Load(); if err != nil { return "", err }
    b, err := json.Marshal(sess); return string(b), err
}

// 持久化时序（避免 callback 结束后无法传播错误）：
//   原生 async API 的流程 = Start(method) → callback 收到 result/error →
//     若为登录/注册/登出/SetBase 且成功 → 调 GetTokenJSON 写 Keychain/Keystore →
//     **持久化完成后**才 resolve 原生 Promise/Future；写失败则 reject 并把错误返回调用方。
//   绝不在 callback 已完成（Promise 已 resolve）后再做持久化——那样失败无法回传。
//   启动时先 SetTokenJSON 恢复。Clear/Save 失败一律视为硬错误上报，不静默成功。

func (cl *Client) Start(method, argsJSON string, cb Callback) *Call {
    ctx, cancel := context.WithCancel(context.Background())
    go func() { defer cancel(); v, err := dispatch.Call(ctx, cl.c, method, argsJSON); cb.Done(envelope(v, err)) }()
    return &Call{cancel: cancel}
}
type Call struct{ cancel context.CancelFunc }
func (c *Call) Cancel() { c.cancel() }
// envelope/jsonError/logf 照搬 hongguo core.go:79-102
```

**Step 4: 实现 dispatch 方法表（照 hongguo dispatch.go:198-280）**

```go
// 方法表：login/register/me/logout/feed/category/rank/search/posts/post/video/play/
// comments/commentSend/like/collect/follow/postLike/postCollect/postComments/setBase/
// decryptCover/rewritePlaylist/fetchSegment
// - 参数解析 arg/args2/oneString 照搬
// - 错误码映射换 huangguo 哨兵错误（ErrAuth/ErrCode/ErrTransport/ErrInvalidBase/...）
// - 媒体方法返回 SegmentResult / string → 由 envelope 序列化 JSON（不直接绑定结构）
// - Body []byte 经 JSON 自动 base64，可承载 0x00/0xff
```

**Step 5: 验证**

```bash
cd repo/bindings/huangguo/core && GOWORK=off go test ./... -v && GOWORK=off go vet ./...
```
Expected: PASS。补往返测试：Swift/Kotlin 侧解码含 `0x00/0xff`、206、响应头的 JSON（在 Task 9 壳测试里）。

**Step 6: Commit**

```bash
git commit -m "feat(core): gomobile entry, token JSON bridge and dispatch table"
```

---

### Task 9: 移动端壳（iOS + Android 完整工程）

**Files:**
- Create: `repo/bindings/huangguo/ios/Sources/OpenHuangguo/Client.swift`
- Create: `repo/bindings/huangguo/ios/Sources/OpenHuangguo/PlayerResourceLoader.swift`
- Create: `repo/bindings/huangguo/ios/OpenhuangguoCore.xcframework（由 build.sh 生成，不提交）`
- Create: `repo/bindings/huangguo/ios/Package.swift`（本地开发用）
- Create: `repo/bindings/huangguo/android/settings.gradle.kts`
- Create: `repo/bindings/huangguo/android/build.gradle.kts`
- Create: `repo/bindings/huangguo/android/gradle/wrapper/*`（gradlew + jar + properties）
- Create: `repo/bindings/huangguo/android/src/main/AndroidManifest.xml`
- Create: `repo/bindings/huangguo/android/src/main/kotlin/dev/openhuangguo/Client.kt`
- Create: `repo/bindings/huangguo/android/src/main/kotlin/dev/openhuangguo/PlayerDataSourceFactory.kt`
- Create: `repo/bindings/huangguo/android/src/test/kotlin/dev/openhuangguo/ClientTest.kt`

**Step 1: iOS 壳（swift build 由 CI 覆盖，本地可选）**

```swift
// Client.swift：封装 Start/callback 为 async；SetTokenJSON/GetTokenJSON 接 Keychain（定义 TokenStoring 协议）
// 登录/注册/登出/SetBase 的 async 包装：callback 成功后先 GetTokenJSON 写 Keychain，写入成功才 resolve，
// 写失败则 throw（对应 core 层的"持久化完成后再结束"约定）
// PlayerResourceLoader.swift：AVAssetResourceLoaderDelegate
//   - hg://localhost/seg?u= 还原上游 URL
//   - 调 Core 的 decryptCover/rewritePlaylist/fetchSegment（经 JSON，解码 base64 body）
//   - Range：AVPlayer 的 requests 转 reqHeader，回传 Content-Range + status 206
//   - 子清单：按扩展名 OR MIME OR #EXTM3U 内容判断，递归 rewritePlaylist
```

**Step 2: Android 完整工程（codex #8：build.sh 依赖这些文件）**

```kotlin
// Client.kt：封装 Start/callback 为 suspend；token 桥接接 Android Keystore
// PlayerDataSourceFactory.kt：用 Media3 DataSource.Factory 扩展（不是 MediaDataSource——后者是单数据源字节读取，
//   不负责逐 URI 的 HLS 加载）。实现主清单→子清单→KEY→分片→seek 验证。
// 包含 Range 请求与 206 响应头处理。
```
Gradle wrapper / Manifest / settings.gradle.kts 参照 `bindings/mobile/android/` 改造包名与产物名。

**Step 3: 原生测试**

```
- Swift：取消测试 + TokenStore Keychain 往返 + 播放器链路最小验证（CI 需 Xcode/macOS）
- Kotlin：ClientTest.kt 覆盖取消、token 桥接、播放器 seek（CI Gradle）
```

**Step 4: Commit**

```bash
git commit -m "feat(bindings): iOS and Android shells with full Gradle project and player loading"
```

---

### Task 10: 构建脚本 + 发版 workflow + Package.swift + 回填脚本

**Files:**
- Create: `repo/bindings/huangguo/build.sh`
- Create: `repo/scripts/fix_huangguo_binarytarget.py`
- Create: `repo/.github/workflows/hg-xcf.yml`
- Create: `repo/.github/workflows/ci.yml`
- Create: `repo/Package.swift`
- Create/Update: `repo/Sources/OpenHuangguo/`（从 ios 壳源码复制，作为发布产物）

**Step 1: build.sh（改造 hongguo build.sh）**

```
- core 路径指向本仓库 sdk/core 模块
- 产物 OpenHuangguoCore.xcframework / huangguo-release.aar
- javapkg dev.openhuangguo；只保留 android + ios
- macOS 目标 v12（不随上游降 v11）
```

**Step 2: 回填脚本（codex #7：资产名一致 + 精确替换）**

```python
# scripts/fix_huangguo_binarytarget.py <tag> <sha256>
# 1. 只匹配 name == "OpenHuangguoCore" 的 binaryTarget 块
# 2. URL = .../releases/download/<tag>/OpenHuangguoCore.xcframework.zip（与实际发布资产名一致）
# 3. checksum 断言在目标块内命中恰好一次；全局其它 binaryTarget 不受影响
# 4. 用含第二个 binaryTarget 的 fixture 单测验证"其他目标不变"
NEW_URL = ".../OpenHuangguoCore.xcframework.zip"
# 禁止简单全局 checksum 替换
```

**Step 3: 发版 tag 时序（codex #6：tag 必须含回填后的 Package.swift）**

```
1. 先把含改动的 main 推到 openhuangguo-swift
2. 打 hg-xcf-v* 标签 → 触发 hg-xcf.yml：编译 → 校验 → zip+checksum → 回填本仓库 Package.swift → publish release（资产 OpenHuangguoCore.xcframework.zip）
3. 回填提交后，在最新 main 上打**标准语义版本 tag** `v0.1.0`（纯 semver，SwiftPM 可解析）——消费者用 `.package(url: ..., from: "0.1.0")` 解析到含正确 URL/checksum 的 Package.swift。用 `hg-v0.1.0` 这类非纯 semver tag 会导致 SwiftPM 无法按版本解析
4. `hg-xcf-v*` 仅作二进制资产 tag，不作为 SwiftPM 版本；只有 `vX.Y.Z` 是消费者版本（与 hongguo 的 `xcf-v*`/`v*` 分工一致）
```

**Step 4: hg-xcf.yml（隔离）**

```yaml
# push tags ['hg-xcf-v*'] + workflow_dispatch；只在本仓库运行，用本仓库 GITHUB_TOKEN
# jobs: build（gomobile init + build.sh ios + macOS slice 校验 + zip + shasum）
#       →回填 Package.swift → commit 到本仓库 main → publish release
# 不写任何 hongguo 文件；不触发 hongguo 的 release.yml
```

**Step 5: 验证门禁接通（codex #8：跨 workflow 不能用 needs，改用可复用 workflow）**

不同 workflow 的 jobs 不能直接 `needs` 关联。方案：抽出 `.github/workflows/verify.yml`（`on: workflow_call` + `push`/`PR`），包含全部验证 jobs；`ci.yml` 与 `hg-xcf.yml` 都通过 `uses: ./.github/workflows/verify.yml` 调用它。

```yaml
# verify.yml（workflow_call）：
#   jobs.go:        GOWORK=off 逐 module fmt/vet/test + gen-check
#   jobs.ios:       build.sh ios + swift test；验证对象含实际发布的根目录 Sources/OpenHuangguo
#   jobs.android:   build.sh android + gradle test + 模拟器/设备任务跑播放器 seek 验证
#                   （不是仅 gradle test；Media3 播放器链路需真实渲染环境）
#   jobs.player-e2e: 主清单→子清单→KEY→分片→seek 最小验证（macOS 模拟器 + Android 模拟器）
# hg-xcf.yml：
#   jobs.verify:  uses: ./.github/workflows/verify.yml
#   jobs.release: needs: verify  （同 workflow 内 needs 成立）
# 断言：只有 verify 全部 jobs 全绿，hg-xcf-v* 的 release job 才运行
```

**Step 6: 本地确认不破坏 hongguo**

```bash
cd repo && grep -R 'openhongguo\|beilunyang' . --exclude-dir=.git || echo "OK: no hongguo refs"
```

**Step 7: Commit**

```bash
git commit -m "build: standalone build, release workflow, Package.swift and precise backfill script"
```

---

### Task 11: 全量验证门禁

**Step 1: 运行全部本地检查**

```bash
cd repo
make fmt && make vet && make test && make gen-check
# 生成一致性：git diff --exit-code（含 Task7 的 Types.swift/Types.kt）
```

**Step 2: 发布前清单（codex #13 闭环）**

```
- 黄果独立 Go 检查全绿
- 生成文件无 drift
- gomobile 双端构建（CI）
- Swift/Kotlin 壳编译 + 取消测试 + 播放器链路最小验证（CI）
- token 持久化失败边界测试（Clear/Save 失败返回错误，不谎称成功）
- SegmentResult 含 0x00/0xff、206、响应头的 JSON 往返测试
```

**Step 3: Commit（若验证过程有修复）**

```bash
git commit -am "test: full validation gate fixes" || echo "no changes"
```

---

## 完成后交接

1. 本地 `make all` 全绿；生成文件无 drift。
2. xcframework 编译/打包/发版由 CI（`hg-xcf.yml`）执行，本地不做。
3. 发版时序（独立仓库）：推 main → 打 `hg-xcf-v*` 触发构建+release → 回填后打标准 semver tag `vX.Y.Z` 供 SwiftPM 消费。
4. 若需推送当前 fork 的设计/计划文档，单独请求用户授权（发版/推送单独确认）。
5. 整个实现**不触碰 hongguo 任何文件**；完成后用 `git -C 当前fork status` 确认 fork 除文档外无改动。
