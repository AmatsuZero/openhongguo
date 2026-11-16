# OpenHuangguoCore Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 在独立仓库 `AmatsuZero/openhuangguo-swift` 实现黄果 Go SDK + gomobile Core（OpenHuangguoCore）+ Swift/Kotlin 绑定壳 + 独立发版流水线，不动 hongguo。

**Architecture:** 参考 HGhub（`AmatsuZero/study-materials-fork`）的 edge 信封接口与媒体算法，仿 hongguo SDK 形态。三模块：SDK（业务接口/transport/edge/token/媒体纯函数）、Core（gomobile 按名调用）、gen-huangguo（类型生成，schema 构建期复制改造）。所有复杂数据经 `Start`/callback 的 JSON 通道，gomobile 只导简单类型 + interface。

**Tech Stack:** Go 1.26（SDK 可用更低但 Core 因 gomobile tool 用 1.26）、gomobile/gobind、标准库 net/http + crypto/aes、Swift 5.9 SwiftPM、Kotlin/Gradle、GitHub Actions。

**设计依据:** `docs/plans/2026-11-16-openhuangguo-core-design.md`（已通过 codex review）。

---

## 执行前提（动手前确认，涉及外部/共享状态）

1. **创建独立仓库** `AmatsuZero/openhuangguo-swift`（`gh repo create`，需用户授权）。所有实现代码落此仓库，**不写入当前 fork**。
2. 当前 fork 只保留两份设计/计划文档（已 commit，未推送）。是否推送 fork 由用户单独决定。
3. 参考源 `/tmp/study-materials-fork` 为临时克隆；执行中若丢失，重新 `git clone --depth 1 https://github.com/AmatsuZero/study-materials-fork /tmp/study-materials-fork`。

> 下列所有路径相对于**独立仓库根** `openhuangguo-swift/`。文中 `repo` 指该仓库根。

## 全局约定

- Module path：
  - SDK：`github.com/AmatsuZero/openhuangguo-swift/bindings/huangguo/sdk`
  - Core：`github.com/AmatsuZero/openhuangguo-swift/bindings/huangguo/core`
  - gen：`github.com/AmatsuZero/openhuangguo-swift/bindings/huangguo/gen-huangguo`
- 提交粒度：每个 Task 末尾一次 commit；TDD（先红后绿）；`gofmt -l` 为空才提交。
- 测试命令在各自 module 目录执行；本地不做 xcframework 编译/打包/发布（走 CI `hg-xcf.yml`）。
- 每个 Go 源码文件顶部注释用中文说明职责（对齐 hongguo 风格）。

---

### Task 0: 仓库骨架与 module 初始化

**Files:**
- Create: `repo/go.mod`（可选，根聚合；若不需要可省，三 module 各自独立）
- Create: `repo/bindings/huangguo/sdk/go.mod`
- Create: `repo/bindings/huangguo/core/go.mod`
- Create: `repo/bindings/huangguo/gen-huangguo/go.mod`
- Create: `repo/.gitignore`

**Step 1: 创建仓库并初始化三 module**

```bash
gh repo create AmatsuZero/openhuangguo-swift --private  # 或 --public，用户定
cd openhuangguo-swift
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
```

**Step 2: 初始化 core 的 go.sum（gobind tool）**

```bash
cd repo/bindings/huangguo/core && go mod tidy
```
Expected: 生成 go.sum，含 golang.org/x/mobile 等；`tool gobind` 解析成功。

**Step 3: 验证三 module 可独立构建**

```bash
cd repo/bindings/huangguo/sdk && go build ./...
cd repo/bindings/huangguo/core && go build ./...   # 此时 core 还没有 .go，可先放一个占位 package
```
Expected: 无编译错误。

**Step 4: Commit**

```bash
cd repo && git add . && git commit -m "chore: init modules for huangguo core"
```

---

### Task 1: SDK model 具名类型

**Files:**
- Create: `repo/bindings/huangguo/sdk/model/model.go`
- Create: `repo/bindings/huangguo/sdk/model/model_test.go`

**Step 1: 写失败测试（JSON 反序列化对齐 src/types.ts）**

```go
package model

import (
    "encoding/json"
    "testing"
)

func TestVideoCardJSON(t *testing.T) {
    raw := `{"id":1,"title":"t","cover":"c","tags":["a"],"score":9,
      "episode_count":2,"total_episodes":2,"hot":100,"like_count":3,
      "collect_count":4,"comment_count":5,"is_finished":true,"is_original":1,
      "author":{"id":7,"name":"n"}}`
    var v VideoCard
    if err := json.Unmarshal([]byte(raw), &v); err != nil { t.Fatal(err) }
    if v.ID != 1 || v.Author == nil || v.Author.ID != 7 { t.Fatalf("bad: %+v", v) }
}

func TestPagedInstancesAndNestedStructs(t *testing.T) {
    // 分页具名类型 + Detail.counts/author + PlayInfo.streams 具名结构
    var d Detail
    if err := json.Unmarshal([]byte(`{"id":1,"counts":{"like":2},"author":{"id":3},
      "episodes":[]}`), &d); err != nil { t.Fatal(err) }
    if d.Counts.Like != 2 || d.Author.ID != 3 { t.Fatalf("bad: %+v", d) }
    var p PlayInfo
    if err := json.Unmarshal([]byte(`{"id":1,"streams":{"main":"http://m"}}`), &p); err != nil { t.Fatal(err) }
    if p.Streams == nil || p.Streams.Main != "http://m" { t.Fatalf("bad: %+v", p) }
}
```

**Step 2: 运行确认失败**

```bash
cd repo/bindings/huangguo/sdk && go test ./model/ -run 'TestVideoCardJSON|TestPagedInstances' -v
```
Expected: FAIL（类型未定义）。

**Step 3: 实现具名类型（对照 /tmp/study-materials-fork/src/types.ts）**

`model.go` 要点（字段 JSON tag 与 TS 完全一致；嵌套/分页必须具名，供 schema AST 解析）：

```go
package model

type Author struct {
    ID        int64  `json:"id"`
    Name      string `json:"name"`
    Avatar    string `json:"avatar,omitempty"`
    Followers int    `json:"followers,omitempty"`
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

type Pagination struct { Page, Pages, Size, Total int `json:"page,..." /* 各自 tag */` }
// 用完整字段：
type Pagination struct {
    Page  int `json:"page"`
    Pages int `json:"pages"`
    Size  int `json:"size"`
    Total int `json:"total"`
}

// 分页具名实例（避免泛型，供 schema 解析）
type VideoCardPage struct { Items []VideoCard `json:"items"`; Pagination Pagination `json:"pagination"` }
type PostPage      struct { Items []Post      `json:"items"`; Pagination Pagination `json:"pagination"` }
type CommentPage   struct { EP int `json:"ep"`; Revision string `json:"revision,omitempty"`; Reset bool `json:"reset,omitempty"`; Items []Comment `json:"items"`; Pagination Pagination `json:"pagination"` }
type SearchResult  struct { Items []VideoCard `json:"items"`; Pagination Pagination `json:"pagination"`; Keyword string `json:"keyword"` }

type Episode struct { EpNum int `json:"ep_num"`; Title string `json:"title"`; Duration int `json:"duration"`; Status int `json:"status"` }

type DetailCounts struct { Collect, Comment, Hot, Like int64 /* 各自 tag */ }
// 完整：
type DetailCounts struct {
    Collect int64 `json:"collect"`
    Comment int64 `json:"comment"`
    Hot     int64 `json:"hot"`
    Like    int64 `json:"like"`
}
type Detail struct {
    ID int64 `json:"id"`; Title, Description, Cover string `json:"title,..."`
    // 全部字段按 types.ts，嵌套用具名：
    Tags        []string          `json:"tags"`
    TagLinks    []TagLink         `json:"tag_links,omitempty"`
    Score       float64           `json:"score"`
    EpisodeCount, TotalEpisodes int `json:"episode_count,..."`
    Episodes    []Episode         `json:"episodes"`
    Counts      DetailCounts      `json:"counts"`
    Author      Author            `json:"author"`
    Interaction *Interaction      `json:"interaction,omitempty"`
    VideoURL    string            `json:"video_url,omitempty"`
    VideoURLHevc string           `json:"video_url_hevc,omitempty"`
    IsFinished  bool              `json:"is_finished"`
    IsOriginal  int               `json:"is_original"`
    VideoType   int               `json:"video_type,omitempty"`
    Duration    int               `json:"duration,omitempty"`
    CreatedAt   string            `json:"created_at,omitempty"`
}
type TagLink struct { Name, URL string }
type Interaction struct { Collected, Liked bool; LikedEP int `json:"liked_ep,omitempty"` }

type Streams struct { Main, Hevc, Preview string /* 各自 tag */ }
type PlayInfo struct {
    ID int64; EpNum int; Duration int; Title string
    VideoURL string `json:"video_url"`
    VideoURLHevc string `json:"video_url_hevc,omitempty"`
    Streams *Streams `json:"streams,omitempty"`
}

type RankMetrics struct { Collect, Comment, Like, Play int64 }
type RankItem struct { /* 按 types.ts；Metrics RankMetrics `json:"metrics"` */ }

type Comment struct { /* 按 types.ts；ReplyList []Comment `json:"reply_list,omitempty"` */ }
type Post struct { /* 按 types.ts，Counts 具名 */ }
type User struct { /* 按 types.ts，Interactions 具名，LikeVideoEpisode 具名 */ }
```

> 实现时把所有 `/* 各自 tag */` 与省略字段按 `types.ts` 补全，不得遗漏；用 `omitempty` 对应 TS 的可选字段；指针对应可空。

**Step 4: 运行确认通过**

```bash
cd repo/bindings/huangguo/sdk && go test ./model/ -v
```
Expected: PASS。

**Step 5: Commit**

```bash
git add bindings/huangguo/sdk/model && git commit -m "feat(sdk): add named model types aligned with HGhub"
```

---

### Task 2: SDK transport（代理 + 轻守卫 + 重定向）

**Files:**
- Create: `repo/bindings/huangguo/sdk/transport.go`
- Create: `repo/bindings/huangguo/sdk/transport_test.go`

**Step 1: 写失败测试**

```go
package sdk

import (
    "context"
    "net/http"
    "net/http/httptest"
    "testing"
)

func TestTransportSendsBearerHeader(t *testing.T) {
    // 自建 httptest server，transport 必须能发送 Authorization（不同于 hongguo）
    var got string
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        got = r.Header.Get("Authorization"); w.Write([]byte(`{"status":1,"data":{}}`))
    }))
    defer srv.Close()
    tr := NewTransport(TransportConfig{})
    req := &Request{Method: "POST", URL: srv.URL, Headers: map[string]string{"Authorization": "Bearer tok"}, Body: []byte(`{}`)}
    if _, err := tr.Do(context.Background(), req); err != nil { t.Fatal(err) }
    if got != "Bearer tok" { t.Fatalf("authorization not sent: %q", got) }
}

func TestTransportRejectsUserInfoAndPrivateTarget(t *testing.T) {
    tr := NewTransport(TransportConfig{})
    for _, u := range []string{"https://user:pass@example.com/", "http://127.0.0.1:1/", "https://evil.example:443/"} {
        if _, err := tr.Do(context.Background(), &Request{Method:"GET", URL:u}); err == nil {
            t.Fatalf("expected reject: %s", u)
        }
    }
}

func TestProxyEnvPriorityAndExplicitDirect(t *testing.T) {
    // 显式 ProxyURL > HGHUB_PROXY > HTTPS_PROXY；空字符串 = 强制直连
    c := NewTransport(TransportConfig{})
    c.resolveProxy("https://api.example.com") // 断言按优先级返回
}
```

**Step 2: 运行确认失败**

```bash
cd repo/bindings/huangguo/sdk && go test . -run 'TestTransport' -v
```
Expected: FAIL（NewTransport/Request 未定义）。

**Step 3: 实现 transport**

`transport.go` 要点：

```go
package sdk

// Request/Response 是本 SDK 内部 HTTP 抽象（不直接 gomobile 绑定）。
type Request struct {
    Method string
    URL    string
    Headers map[string]string
    Body   []byte
    // NoRedirect 为 true 时禁止自动跳转
    NoRedirect bool
}
type Response struct {
    StatusCode int
    Headers map[string]string
    Body []byte
}
type TransportConfig struct {
    ProxyURL string // 显式代理；"" 且未设 EnvDirect 时走环境回退；特殊值 "direct" 表示强制直连
}
type Transport struct { cfg TransportConfig; client *http.Client }

func NewTransport(cfg TransportConfig) *Transport {
    // http.Client{Timeout:30s, CheckRedirect: 见下}
    // Redirect: 默认禁止跳转（edge 用）；媒体可在 Request 层放开有限跳转
}

// resolveProxy 按优先级：显式 cfg.ProxyURL > HGHUB_PROXY > HTTPS_PROXY/ALL_PROXY/HTTP_PROXY；
// 环境变量回退遵循 NO_PROXY；显式代理忽略 NO_PROXY；"direct" 强制直连。
// 首版只 http/https，不支持 socks。
func (t *Transport) resolveProxy(target string) (*url.URL, error) { /* ... */ }

func (t *Transport) Do(ctx context.Context, req *Request) (*Response, error) {
    // 1. 校验目标 URL：禁 user:pass、禁内网/回环/保留地址（媒体请求的目标同样校验）
    // 2. 代理 URL 与目标 URL 分开校验：代理地址不套用目标的"禁内网"规则
    // 3. 构造 http.Request（header 原样带上，含 Authorization）
    // 4. client.Do；错误里代理凭据必须脱敏（redactProxy）
    // 5. 读 body：API 8MB 上限；媒体由调用方设更大上限（见 FetchSegment）
    // 6. 返回 Response；重定向策略见 client.CheckRedirect
}
```

- 禁内网判定用 `net.ParseIP` + `ip.IsLoopback()/IsPrivate()/IsLinkLocalUnicast()/IsUnspecified()`；域名 `localhost`/`*.local`/`*.internal` 拒绝。
- `redactProxy` 只回显 scheme://host:port，隐藏 userinfo/path/query。
- 媒体跳转最多 5 跳、逐跳校验、禁 HTTPS→HTTP 降级：在 `CheckRedirect` 或媒体专用 do 循环实现。

**Step 4: 运行确认通过**

```bash
cd repo/bindings/huangguo/sdk && go test . -run 'TestTransport' -v && go vet ./...
```
Expected: PASS。

**Step 5: Commit**

```bash
git add bindings/huangguo/sdk/transport.go bindings/huangguo/sdk/transport_test.go && git commit -m "feat(sdk): transport with proxy, guards and redirect policy"
```

---

### Task 3: SDK edge 信封与响应解析

**Files:**
- Create: `repo/bindings/huangguo/sdk/edge.go`
- Create: `repo/bindings/huangguo/sdk/edge_test.go`

**Step 1: 写失败测试（精确响应语义）**

```go
package sdk

func TestEdgeEnvelopeEncoding(t *testing.T) {
    // 无 body：body/content_type 均为空串；有 body：base64(JSON)+application/json
    // 断言发给 /api/edge/request 的 JSON 信封字段
}

func TestEdgeResponseSemantics(t *testing.T) {
    cases := []struct{ body string; wantOK bool; wantData string }{
        {``, true, `null`},                       // 空串成功
        {`null`, true, `null`},                   // JSON null 成功
        {`{}`, false, ``},                        // 裸 {} 失败（无 status）
        {`{"status":1,"data":{}}`, true, `{}`},   // data:{} 成功
        {`{"status":0,"msg":"bad"}`, false, ``},  // status!=1 失败
    }
    // 对每个 case 起 httptest，调用 edge，断言结果
}
```

**Step 2: 运行确认失败**

```bash
cd repo/bindings/huangguo/sdk && go test . -run 'TestEdge' -v
```
Expected: FAIL（edge 未定义）。

**Step 3: 实现 edge**

```go
package sdk

type edgeEnvelope struct {
    Method      string `json:"method"`
    Path        string `json:"path"`
    Body        string `json:"body"`
    ContentType string `json:"content_type"`
}

type CodeError struct { Code int; Message, Name, Kind string }
func (e *CodeError) Error() string { return e.Message }

// edge 把 method/path/body 封成信封 POST 到 base/api/edge/request。
// retry 由调用方 client.do 统一处理；这里只做单次请求 + 响应解析。
func (c *Client) edge(ctx context.Context, method, path string, body any, token string) ([]byte, error) {
    enc := ""
    ctype := ""
    if body != nil {
        b, _ := json.Marshal(body)
        enc = base64.StdEncoding.EncodeToString(b)
        ctype = "application/json"
    }
    payload, _ := json.Marshal(edgeEnvelope{method, path, enc, ctype})
    headers := map[string]string{"Accept": "application/json", "Referer": c.base + "/"}
    if token != "" { headers["Authorization"] = "Bearer " + token }
    req := &Request{Method:"POST", URL: c.base + "/api/edge/request", Headers: headers, Body: payload}
    resp, err := c.transport.Do(ctx, req) // 禁止重定向
    if err != nil { return nil, err }
    return parseEdgeResponse(resp.Body)
}

func parseEdgeResponse(raw []byte) ([]byte, error) {
    s := strings.TrimSpace(string(raw))
    if s == "" || s == "null" { return []byte("null"), nil }
    var v struct { Status *int `json:"status"`; Data json.RawMessage `json:"data"`; Msg string `json:"msg"` }
    if err := json.Unmarshal(raw, &v); err != nil { return nil, err }
    if v.Status != nil && *v.Status == 1 {
        if len(v.Data) == 0 { return []byte("null"), nil }
        return v.Data, nil
    }
    if v.Msg != "" { return nil, &CodeError{Message: v.Msg} }
    return nil, &CodeError{Message: "接口返回失败"}
}
```

**Step 4: 运行确认通过**

```bash
cd repo/bindings/huangguo/sdk && go test . -run 'TestEdge' -v && go vet ./...
```
Expected: PASS。

**Step 5: Commit**

```bash
git add bindings/huangguo/sdk/edge.go bindings/huangguo/sdk/edge_test.go && git commit -m "feat(sdk): edge envelope and exact response semantics"
```

---

### Task 4: SDK TokenStore、凭据信任与 client 基础

**Files:**
- Create: `repo/bindings/huangguo/sdk/store.go`
- Create: `repo/bindings/huangguo/sdk/client.go`
- Create: `repo/bindings/huangguo/sdk/store_test.go`
- Create: `repo/bindings/huangguo/sdk/client_test.go`

**Step 1: 写失败测试**

```go
package sdk

func TestSessionOriginBindingAndGeneration(t *testing.T) {
    m := NewMemStore()
    m.Save(Session{Origin:"https://huangguoai.com", Token:"tok", Generation:1})
    // 换 origin 读取应为空；generation 递增后旧快照失效
}
func TestSetBaseClearsTokenAndCredentialTrust(t *testing.T) {
    // setBase 到镜像域名后 token 被清；镜像不能接收密码/Bearer
}
func TestCredentialTrust(t *testing.T) {
    // 默认 huangguoai.com/.ai 放行凭据；evil.example 拒绝密码/Bearer
}
```

**Step 2: 运行确认失败**

```bash
cd repo/bindings/huangguo/sdk && go test . -run 'TestSession|TestSetBase|TestCredentialTrust' -v
```
Expected: FAIL。

**Step 3: 实现 store + client 基础**

`store.go`：
```go
type Session struct { Origin, Token string; Generation int64; User json.RawMessage }
type TokenStore interface {
    Load() (Session, error)
    Save(Session) error
    Clear() error
}
type MemStore struct { mu sync.Mutex; s Session }
// Load/Save/Clear 实现；Clear 立即内存失效
// FileStore：0600 + tmp+rename 原子写；Clear 删文件失败必须返回错误
```

`client.go`：
```go
type Client struct {
    transport Transport // 用接口便于测试注入
    store TokenStore
    base string
    credTrust map[string]bool // 默认 huangguoai.com / .ai
    clock func() time.Time
    logger Logger
    retryAttempts int
    mu sync.Mutex
}
func NewClient(opts Options) (*Client, error) {
    // 必填 transport/store；base 规范化；默认 credential trust
}
func normalizeBase(raw string) (string, error) // HTTPS 纯 origin，无 userinfo/path/query/fragment，禁内网
func (c *Client) allowedCredentialTarget() bool { return c.credTrust[hostOf(c.base)] }
func (c *Client) SetBase(raw string) error { // 规范化；mu 锁；generation++；store.Clear() }
func (c *Client) do(ctx, method, path, body, needCred) ([]byte, error) {
    // 重试循环照搬 hongguo client.go:122-174；判定按 HGhub（无 body GET 可带 token、播放 POST 可重试、408/425/429/5xx）
    // needCred 且 !allowedCredentialTarget → 拒绝（镜像不收密码/Bearer）
    // 取 token：原子读 origin+generation 快照；generation 变化时禁止提交登录结果
}
```

- generation 失效：登录响应提交前再次校验当前 generation 与发起时一致，不一致丢弃。
- 失败语义：`Clear` 持久化失败返回错误，不谎称跨重启成功（见设计 #6）。

**Step 4: 运行确认通过**

```bash
cd repo/bindings/huangguo/sdk && go test . -run 'TestSession|TestSetBase|TestCredentialTrust' -v && go vet ./...
```
Expected: PASS。

**Step 5: Commit**

```bash
git add bindings/huangguo/sdk/store.go bindings/huangguo/sdk/client.go bindings/huangguo/sdk/*_test.go && git commit -m "feat(sdk): token store, credential trust and client base"
```

---

### Task 5: SDK 全部业务接口（约 20 个）

**Files:**
- Create: `repo/bindings/huangguo/sdk/api.go`
- Create: `repo/bindings/huangguo/sdk/api_test.go`

**Step 1: 写失败测试（用 mock transport 回放 HGhub fixture）**

```go
package sdk

func TestLoginAndMe(t *testing.T) {
    // mock transport 返回 {"status":1,"data":{"token":"t","user":{...}}}
    // 断言 Login 存 token、Me 带 Bearer
}
func TestPlayFallbackAndStreamsNormalization(t *testing.T) {
    // 新接口 POST /episodes/{ep}/playback 返回 streams.main → 回填 VideoURL
    // 新接口无地址时回退 GET /play?ep=...&page_texts=0（对齐 api.rs:473-528）
}
func TestAllBusinessEndpoints(t *testing.T) {
    // 逐个断言 method/path/参数：feed/category/rank/search/posts/post/video/
    // comments/commentSend/like/collect/follow/postLike/postCollect/postComments
}
```

**Step 2: 运行确认失败**

```bash
cd repo/bindings/huangguo/sdk && go test . -run 'TestLogin|TestPlay|TestAllBusiness' -v
```
Expected: FAIL。

**Step 3: 实现 api.go（方法表对照 /tmp/study-materials-fork/src-tauri/src/api.rs）**

```go
// 每个方法：构造 path（含 query，用 url.QueryEscape）+ 调 c.do
// method/path 对照（来自 api.rs）：
// login: POST /api/account/loginByPassword {username,password}
// register: POST /api/account/registerByPassword {username,password,confirm_password,email?}
// logout: POST /api/account/logout
// me: GET /api/account/me
// feed: GET /api/videos?page=&size=
// category: GET /api/videos/category/{cate}?page=&size=
// rank: GET /api/ranks/{kind}  (kind 限定 recommend|potential|hot)
// search: GET /api/search?q=<escaped>&page=&size=
// posts: GET /api/posts?page=&size=
// post: GET /api/posts/{id}
// video: GET /api/videos/{id}
// play: 见下（双接口回退）
// comments: GET /api/videos/{id}/comments?ep=&page=&size=
// commentSend: POST /api/videos/{id}/comments[?ep=] {text,reply_id?}
// like: POST|DELETE /api/videos/{id}/like?ep=
// collect: POST|DELETE /api/videos/{id}/collect
// follow: POST|DELETE /api/users/{uid}/follow
// postLike: POST|DELETE /api/posts/{id}/like
// postCollect: POST|DELETE /api/posts/{id}/collect
// postComments: GET /api/posts/{id}/comments?page=&size=
```

`Play` 实现要点（照 `api.rs:499-528`）：
```go
func (c *Client) Play(ctx, id int64, ep int) (*model.PlayInfo, error) {
    // 1. POST /api/videos/{id}/episodes/{ep}/playback body {}
    // 2. normalizeStreams：streams.main/hevc 回填到 VideoURL/VideoURLHevc（若原字段空）
    // 3. 若 video_url 非空 → 返回
    // 4. 否则 GET /api/videos/{id}/play?ep={ep}&page_texts=0，normalize 后返回
    // 5. 两者都失败 → 返回新接口结果/错误
}
```

`like/collect/follow` 等写操作 `needCred=true`，禁止盲重试；`commentSend` 同理。

**Step 4: 运行确认通过**

```bash
cd repo/bindings/huangguo/sdk && go test . -v && go vet ./...
```
Expected: PASS。

**Step 5: Commit**

```bash
git add bindings/huangguo/sdk/api.go bindings/huangguo/sdk/api_test.go && git commit -m "feat(sdk): all business endpoints with play fallback"
```

---

### Task 6: SDK 媒体纯函数

**Files:**
- Create: `repo/bindings/huangguo/sdk/media.go`
- Create: `repo/bindings/huangguo/sdk/media_test.go`

**Step 1: 写失败测试（对齐 media.rs fixture）**

```go
package sdk

func TestDecryptCoverPlainPassthroughAndCipher(t *testing.T) {
    // 明文 JPEG/PNG 直接透传；密文用 key/IV 解密后去 padding
}
func TestRewritePlaylistFullBehavior(t *testing.T) {
    in := "\ufeff#EXTM3U\n#EXT-X-CONTENT-STEERING:URI=\"x\"\n" +
      "#EXT-X-DATERANGE:ID=\"a\",X-ASSET-LIST=\"b\",DURATION=1\n" +
      "#EXT-X-KEY:METHOD=AES-128,URI=\"key.bin\"\nseg-1.ts\nsub/list.m3u8\n"
    got := RewritePlaylist("https://cdn.example/master.m3u8", "hg://localhost", "hg://localhost", in)
    // 断言：CONTENT-STEERING 删除；DATERANGE 去 URI/X-ASSET-LIST 保留 DURATION；
    // KEY URI 改写为本地协议；分片/子清单 URL 绝对化后改写
}
func TestFetchSegmentRange(t *testing.T) {
    // mock server 返回 206 + Content-Range；断言 SegmentResult.Headers 带 Content-Range
}
```

**Step 2: 运行确认失败**

```bash
cd repo/bindings/huangguo/sdk && go test . -run 'TestDecrypt|TestRewrite|TestFetchSegment' -v
```
Expected: FAIL。

**Step 3: 实现 media.go（算法照 /tmp/study-materials-fork/src-tauri/src/media.rs）**

```go
package sdk

const imgKey = "f5d965df75336270"
const imgIV  = "97b60394abc2fbe1"

// DecryptCover：明文图片（JPEG/PNG/GIF/WEBP magic）直接返回；否则 AES-128-CBC 解密 + 条件去 padding。
func DecryptCover(data []byte) ([]byte, error) { /* aes.NewCipher(imgKey); CBC decrypt; stripPadding */ }

// RewritePlaylist：upstreamURL 作相对 URI 解析基准；改写 URI 为 localScheme://host/seg?u=<pct-encoded abs>
// 行为矩阵：BOM 去除；CONTENT-STEERING 删除；DATERANGE 过滤 URI/X-ASSET-LIST 保留其余；
// 含 URI=" 的属性行（KEY/MAP/MEDIA/SESSION-KEY/PART/PRELOAD-HINT）绝对化+改写；
// 分片行绝对化+改写；子清单（.m3u8）同样改写（递归由宿主按调用处理）。
func RewritePlaylist(upstreamURL, localScheme, host, text string) string { /* ... */ }

// SegmentResult 仅 SDK 内部使用；经 core 的 Start/callback JSON 传出。
type SegmentResult struct {
    Status  int               `json:"status"`
    Headers map[string]string `json:"headers"` // Content-Range / Accept-Ranges / Content-Length
    Body    []byte            `json:"body"`
}
// FetchSegment：独立大上限（如 32MB），支持 Range 请求头，返回 206 状态与响应头。
func (c *Client) FetchSegment(ctx context.Context, url string, reqHeader map[string]string) (SegmentResult, error) { /* ... */ }
```

- AES CBC：`cipher.NewCBCEncrypter` 的逆——手写块解密 + XOR prev（照 media.rs:56-75）。
- `imageKind` 检测复刻（media.rs:87-104）。
- 属性解析 `hlsAttributes`/`mapUriAttributes` 复刻（media.rs:148-190），注意带逗号引号值。
- 媒体目标校验复用 transport 的守卫；分片上限独立于 API 8MB。

**Step 4: 运行确认通过**

```bash
cd repo/bindings/huangguo/sdk && go test . -run 'TestDecrypt|TestRewrite|TestFetchSegment' -v && go vet ./...
```
Expected: PASS。

**Step 5: Commit**

```bash
git add bindings/huangguo/sdk/media.go bindings/huangguo/sdk/media_test.go && git commit -m "feat(sdk): pure-function media tools aligned with HGhub"
```

---

### Task 7: gen-huangguo 类型生成器（schema 构建期复制改造）

**Files:**
- Create: `repo/bindings/huangguo/gen-huangguo/schema/` （从 hongguo `bindings/internal/schema` 复制）
- Create: `repo/bindings/huangguo/gen-huangguo/main.go`
- Create: `repo/bindings/huangguo/gen-huangguo/main_test.go`

**Step 1: 复制并改造 schema**

```bash
cp -R /Users/daubert/Github/openhongguo/bindings/internal/schema repo/bindings/huangguo/gen-huangguo/schema
# 改 schema 包内 import 路径前缀（若有）为本仓库；改 QueryTypes 为可配置
```

**Step 2: 写失败测试（黄果 QueryTypes + 具名类型可解析）**

```go
package main
func TestGenerateHuangguoTypes(t *testing.T) {
    s, err := schema.LoadWithQueryTypes("../sdk", []string{"VideoCardPage","PostPage","CommentPage","SearchResult"})
    if err != nil { t.Fatal(err) }
    swift := swiftTypes(s); kotlin := kotlinTypes(s)
    if !strings.Contains(swift, "struct VideoCard") { t.Fatal("missing swift VideoCard") }
    if !strings.Contains(kotlin, "data class VideoCard") { t.Fatal("missing kotlin VideoCard") }
}
```

**Step 3: 改造 schema 支持自定义 QueryTypes**

```go
// schema 包：新增 LoadWithQueryTypes(root string, queries []string) (*Schema, error)
// 原 Load 保留（内部转调默认 QueryTypes）。把 schema.go:23-26 的 QueryTypes 改为可传入。
// 具名类型已由 Task 1 保证（Paged<T> 实例化、嵌套具名），AST 可解析。
```

**Step 4: 实现 gen main（输出 Swift/Kotlin）**

```go
// 照 hongguo bindings/mobile/gen 的 swift()/kotlin() 生成逻辑复制改造：
// - 输出路径：bindings/huangguo/ios/Sources/OpenHuangguo/Types.swift
//             bindings/huangguo/android/src/main/kotlin/dev/openhuangguo/Types.kt
// - 属性名 JSON→驼峰；零值缺省规则照搬
// - 查询类型用黄果具名分页类型
```

**Step 5: 运行确认通过 + 生成门禁**

```bash
cd repo/bindings/huangguo/gen-huangguo && go test ./... -v
cd repo && go run ./bindings/huangguo/gen-huangguo
# 断言生成文件与预期一致（git diff 检查 / 快照测试）
```
Expected: PASS；生成文件存在且内容正确。

**Step 6: Commit**

```bash
git add bindings/huangguo/gen-huangguo && git commit -m "feat(gen): huangguo type generator with configurable query types"
```

---

### Task 8: Core gomobile 入口（按名调用 + token 桥接）

**Files:**
- Create: `repo/bindings/huangguo/core/core.go`
- Create: `repo/bindings/huangguo/core/dispatch/dispatch.go`
- Create: `repo/bindings/huangguo/core/core_test.go`
- Create: `repo/bindings/huangguo/core/dispatch/dispatch_test.go`

**Step 1: 写失败测试**

```go
package core
func TestStartReturnsResultEnvelope(t *testing.T) {
    // 用 mock SDK client / dispatch，断言 callback.Done 收到 {"result":...} 或 {"error":{code,...}}
}
func TestTokenBridgeJSON(t *testing.T) {
    c.SetTokenJSON(`{"origin":"https://huangguoai.com","token":"t","generation":1}`)
    got, _ := c.GetTokenJSON(); // 断言 roundtrip
}
```

**Step 2: 运行确认失败**

```bash
cd repo/bindings/huangguo/core && go test ./... -v
```
Expected: FAIL。

**Step 3: 实现 core.go（照 hongguo bindings/mobile/core/core.go:37-102）**

```go
package core

type Logger interface { Log(line string) }
type Callback interface { Done(resultJSON string) }

type Client struct { c *sdk.Client }

// 构造器只收 configJSON + logger，不注入 Go interface。
func NewClient(configJSON string, logger Logger) (*Client, error) {
    var cfg Config // base/proxy/credTrustOrigins
    json.Unmarshal([]byte(configJSON), &cfg)
    store := sdk.NewMemStore() // 默认内存；原生层用 SetTokenJSON 桥接持久化
    c, err := sdk.NewClient(sdk.Options{Store: store, Base: cfg.Base, ProxyURL: cfg.ProxyURL, ...})
    return &Client{c: c}, err
}

// token 桥接：单返回值 + error，gomobile 可绑定。
func (c *Client) SetTokenJSON(jsonStr string) error { /* 解析 Session，store.Save */ }
func (c *Client) GetTokenJSON() (string, error)     { /* store.Load → JSON */ }

func (c *Client) Start(method, argsJSON string, callback Callback) *Call {
    // goroutine: dispatch.Call(ctx, c.c, method, argsJSON) → callback.Done(envelope(v, err))
}
type Call struct{ cancel context.CancelFunc }
func (c *Call) Cancel() { c.cancel() }
// envelope/jsonError/logf 照搬 hongguo
```

**Step 4: 实现 dispatch 方法表（照 hongguo dispatch.go:198-280 模式）**

```go
// 约 20 个方法：login/register/me/logout/feed/category/rank/search/posts/post/video/play/
// comments/commentSend/like/collect/follow/postLike/postCollect/postComments/setBase/
// decryptCover/rewritePlaylist/fetchSegment
// 参数解析 arg/args2/oneString 照搬；错误码映射换 huangguo 哨兵错误；
// 媒体方法返回 SegmentResult → 由 envelope 序列化为 JSON（不直接绑定结构体）。
```

**Step 5: 运行确认通过**

```bash
cd repo/bindings/huangguo/core && go test ./... -v && go vet ./...
```
Expected: PASS。

**Step 6: Commit**

```bash
git add bindings/huangguo/core && git commit -m "feat(core): gomobile entry with JSON token bridge and dispatch"
```

---

### Task 9: 移动端壳（iOS Swift + Android Kotlin）

**Files:**
- Create: `repo/bindings/huangguo/ios/Sources/OpenHuangguo/Client.swift`
- Create: `repo/bindings/huangguo/ios/Sources/OpenHuangguo/PlayerResourceLoader.swift`
- Create: `repo/bindings/huangguo/ios/Package.swift`（本地引用 build.sh 产物，用于开发）
- Create: `repo/bindings/huangguo/android/src/main/kotlin/dev/openhuangguo/Client.kt`

**Step 1: Swift 壳（callback→async + 播放器资源加载层）**

```swift
// Client.swift：封装 OpenHuangguoCoreClient 的 Start/callback 为 async 方法
// SetTokenJSON/GetTokenJSON 接 Keychain（定义协议 TokenStoring，默认 Keychain 实现）
// PlayerResourceLoader.swift：AVAssetResourceLoaderDelegate
//   - 把 hg://localhost/seg?u= 还原为上游 URL
//   - 调 Core 的 rewritePlaylist/fetchSegment/decryptCover（经 JSON）
//   - 处理 Range：把 AVPlayer 的 requests 转成 reqHeader，回传 Content-Range
//   - 子清单递归：识别 .m3u8 分片，再次调用 rewritePlaylist
```

**Step 2: Kotlin 壳**

```kotlin
// Client.kt：封装 Start/callback 为 suspend/回调；SetTokenJSON/GetTokenJSON 接 Android Keystore
// 播放器层用 Android MediaDataSource 实现 Range/解密/子清单
```

**Step 3: 本地验证（不需 xcframework）**

```bash
# Swift 壳语法检查（需 Xcode；CI 覆盖）
cd repo/bindings/huangguo/ios && swift build   # 需 build.sh 先生成 xcframework，本地可跳过
# Kotlin 编译由 Gradle/CI 覆盖
```
Expected: 本地若无工具链跳过，CI 覆盖。

**Step 4: Commit**

```bash
git add bindings/huangguo/ios bindings/huangguo/android && git commit -m "feat(bindings): iOS and Android shells with player resource loading"
```

---

### Task 10: 构建脚本 + 发版流水线 + 独立 Swift 包

**Files:**
- Create: `repo/bindings/huangguo/build.sh`（复制 hongguo build.sh 改造）
- Create: `repo/.github/workflows/hg-xcf.yml`
- Create: `repo/Package.swift`（标准根包，指向本仓库 release asset）
- Create: `repo/Sources/OpenHuangguo/`（从 bindings/huangguo/ios 壳源码复制）

**Step 1: build.sh 改造**

```bash
cp /Users/daubert/Github/openhongguo/bindings/mobile/build.sh repo/bindings/huangguo/build.sh
# 改造点：
# - core="$here/core" → SDK/core 模块路径
# - 产物名 OpenhongguoCore.xcframework → OpenHuangguoCore.xcframework
# - aar openhongguo-release.aar → huangguo-release.aar
# - javapkg dev.openhongguo → dev.openhuangguo
# - 只保留 android + ios target
# - macOS 部署目标沿用 v12（不跟随上游降 v11）
```

**Step 2: Package.swift（标准根包）**

```swift
// swift-tools-version:5.9
// name: OpenHuangguo, platforms iOS13/macOS12
// product .library(name:"OpenHuangguo", targets:["OpenHuangguo"])
// binaryTarget name:"OpenHuangguoCore", url:"https://github.com/AmatsuZero/openhuangguo-swift/releases/download/hg-xcf-v.../OpenHuangguoCore.xcframework.zip", checksum:...
// target OpenHuangguo 依赖 OpenHuangguoCore, path Sources/OpenHuangguo
```
> 注意：checksum/url 由 `hg-xcf.yml` 回填（在本独立仓库内，改的是自己的 Package.swift，不动 hongguo）。

**Step 3: hg-xcf.yml（独立发版）**

```yaml
# 触发：push tags ['hg-xcf-v*'] + workflow_dispatch
# jobs:
#   - checkout 本仓库
#   - setup-go 1.26 + gomobile init
#   - 编译 macOS slice 校验（照 xcf.yml）
#   - build.sh ios → OpenHuangguoCore.xcframework
#   - zip + shasum -a 256
#   - 回填本仓库 Package.swift 的 binaryTarget url/checksum（脚本仿 fix_xcf_binarytarget.py，但只匹配 OpenHuangguoCore.zip）
#   - commit Package.swift 到本仓库 main（git checkout -B main origin/main 在本仓库内，不碰 hongguo）
#   - 打标签 hg-xcf-v*、publish release（softprops/action-gh-release），asset = OpenHuangguoCore.xcframework.zip
```

> 关键：本 workflow 只在 `openhuangguo-swift` 仓库运行，用该仓库 GITHUB_TOKEN，其 release 事件不会触发 hongguo 的 `release.yml`。

**Step 4: CI（本仓库自检）**

```yaml
# .github/workflows/ci.yml（本仓库）：
#   - go: 逐 module gofmt/vet/test + go run gen-huangguo 生成一致性
#   - ios: build.sh ios + swift test
#   - android: build.sh android + gradle test
#   - 发布门禁：以上全绿才允许 hg-xcf-v* 发版
```

**Step 5: 本地确认脚本不破坏 hongguo**

```bash
# 确认本仓库所有路径都不指向 hongguo 的 Package.swift / fix_xcf_binarytarget.py / xcf.yml
grep -R 'openhongguo' repo/ || echo "OK: no hongguo refs"
```

**Step 6: Commit**

```bash
git add .github bindings/huangguo/build.sh Package.swift Sources && git commit -m "build: standalone build, release workflow and Swift package"
```

---

### Task 11: fixture 契约测试与发布前验证

**Files:**
- Create: `repo/bindings/huangguo/sdk/testdata/*.json`（HGhub 真实响应样本）
- Create: `repo/bindings/huangguo/sdk/contract_test.go`

**Step 1: 准备 fixture**

```bash
# 从 HGhub 的 testdata / 实际响应采集：feed/video/play/comments/rank/search/posts/me
# 放到 bindings/huangguo/sdk/testdata/，脱敏 token/userinfo
```

**Step 2: 写契约测试**

```go
func TestFixturesParse(t *testing.T) {
    // 每个 fixture 用 c.do 的解析路径跑通，断言关键字段路径与 JSON 类型存在
    // 对照 src/types.ts 字段
}
```

**Step 3: 运行全量验证**

```bash
cd repo/bindings/huangguo/sdk && go test ./... -v
cd repo/bindings/huangguo/core && go test ./... -v
cd repo/bindings/huangguo/gen-huangguo && go test ./... -v
# 生成一致性：go run ./bindings/huangguo/gen-huangguo && git diff --exit-code
```
Expected: 全绿；生成文件无 drift。

**Step 4: Commit**

```bash
git add . && git commit -m "test: HGhub fixtures and full validation gate"
```

---

## 完成后交接

全部 Task 通过后：
1. 本地 `go test ./...` / `go vet` / gofmt 全绿；生成文件无 drift。
2. xcframework 编译/打包/发版由 CI（`hg-xcf.yml`）执行，本地不做。
3. 发版标签顺序（独立仓库）：先把含改动的 main 推到 `openhuangguo-swift`，再打 `hg-xcf-v*` 触发构建与 release；消费者用 `Package.swift` 的 binaryTarget 指向 release。
4. 若需推送当前 fork 的设计/计划文档，单独请求用户授权（按惯例发版/推送单独确认）。
