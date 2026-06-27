# grsai 多模型渠道接入 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 新增 `adapter/grsai` 渠道，接入 grsai 平台的 nano-banana / gpt-image 生图与 veo 视频，使 `addition/photo` 现有功能代码零改动即可通过 channel 配置切换到 grsai。

**Architecture:** 完整镜像 `adapter/jimengapi`：一个 `Generator` struct 同时实现 5 个图片处理接口（生成/编辑/超分/扩图/图生视频），提交后用 `replyType:"async"` + 轮询 `/v1/draw/result` 取结果，下载落地到 `storage/results`，经 `hook` 回推本地公开 URL。新增 channel type `grsai` 注册进 adapter 工厂表，复用 `channel → adapter` 调度与 `photo_tasks` 异步任务体系。

**Tech Stack:** Go，`github.com/goccy/go-json`，标准库 `net/http`；测试用标准 `testing` + `httptest`。

## Global Constraints

- 渠道 type 常量字符串：`grsai`（`globals.GrsaiChannelType`）。
- 认证：HTTP Header `Authorization: Bearer <key>`，key 取自 `conf.GetRandomSecret()`（单 key，**非** AK/SK）。
- reply 模式固定 `replyType:"async"`，结果统一轮询 `POST /v1/draw/result`，入参 `{"id": "<task id>"}`。
- 结果状态枚举：`running` / `succeeded` / `failed` / `violation`；终态为 `succeeded`/`failed`/`violation`。
- 模型裸名：`nano-banana`、`nano-banana-2`、`gpt-image`、`veo`。
- 落地目录 `globals.StorageResultDir`，公开 URL 用 `globals.ResultPublicURL(filename)`，图片回推用 `utils.GetImageMarkdown(url)`，视频回推裸 URL。
- 包名：`package grsai`；所有日志前缀 `[grsai]`。
- 不修改 `addition/photo`、`connection`、`channel` 任何文件；不改任务表结构。

---

## File Structure

| 文件 | 责任 |
|---|---|
| `globals/constant.go`（改） | 新增 `GrsaiChannelType = "grsai"` 常量 |
| `adapter/grsai/types.go`（建） | 模型注册表 `GetModelSpec` + 请求/响应结构 + 终态判断 |
| `adapter/grsai/client.go`（建） | `Generator` struct、HTTP 客户端、`postJSON`、`Submit`、`PollResult` |
| `adapter/grsai/struct.go`（建） | `From*Config` 构造器 + 5 个接口断言 |
| `adapter/grsai/store.go`（建） | 结果 URL 下载落地（图片/视频）、文件名生成 |
| `adapter/grsai/image.go`（建） | `CreateImageGenerationRequest` + `CreateImageEditRequest` |
| `adapter/grsai/edit_extra.go`（建） | `CreateImageUpscaleRequest` + `CreateImageOutpaintRequest` |
| `adapter/grsai/video.go`（建） | `CreateImageToVideoRequest`（veo） |
| `adapter/adapter.go`（改） | 在两个工厂表注册 grsai |
| `config.example.yaml`（改） | 增加 grsai 渠道示例 |
| `adapter/grsai/*_test.go`（建） | 表驱动单测 + 门控 live smoke test |

---

## Phase 0：核对 apifox 精确字段

### Task 0: 确认 gpt-image / veo / 结果查询的请求与响应字段

**Files:** 无代码改动，产出确认记录写入本计划末尾「字段确认」小节（或直接确认与下方假设一致）。

**Interfaces:**
- Produces: 确认 `/v1/draw/result` 入参与响应、`/v1/video/veo` 与 `/v1/draw/completions` 的 body 字段，供后续 Phase 使用。

- [ ] **Step 1: 抓取 apifox 结果查询页**

用 WebFetch 抓 `https://qmy27nhsd9.apifox.cn/452409577e0`，确认结果查询接口的请求体（是否就是 `{"id": "..."}`）与响应字段（`status/progress/results[].url/error`）。

- [ ] **Step 2: 抓取 gpt-image 与 veo 页**

在 apifox 站点（`qmy27nhsd9.apifox.cn`）找到 gpt-image、veo 两页，确认 body 字段。grsai.com 文档页为 JS 渲染、抓取不全，优先用 apifox。

- [ ] **Step 3: 核对下方假设**

确认本计划假设：nano-banana/gpt-image 都接受 `{model, prompt, images[], aspectRatio, imageSize, replyType}`；veo 接受 `{model, prompt, images[], aspectRatio, replyType}`。若字段不同，更新 Phase 2/3/4 中对应 body 构造代码（仅改字段名，不改结构）。

- [ ] **Step 4: 无需提交**（纯调研）

> 若 apifox 页无法访问，按本计划已确认的 nano-banana schema + 文档公开的结果 schema 实现；字段差异在联调（Phase 5）阶段用 live smoke test 校正。

---

## Phase 1：渠道骨架（常量 + 模型注册表 + HTTP 客户端 + 轮询）

### Task 1: 新增 channel type 常量

**Files:**
- Modify: `globals/constant.go`（在 `JimengAPIChannelType` 行附近）

**Interfaces:**
- Produces: `globals.GrsaiChannelType string = "grsai"`

- [ ] **Step 1: 加常量**

在 `globals/constant.go` 中 `JimengAPIChannelType = "jimeng-api" ...` 这一行下方新增：

```go
	GrsaiChannelType     = "grsai"      // grsai 多模型平台（nano-banana / gpt-image / veo）
```

- [ ] **Step 2: 编译验证**

Run: `go build ./globals/...`
Expected: 编译通过，无输出。

- [ ] **Step 3: Commit**

```bash
git add globals/constant.go
git commit -m "feat(grsai): add grsai channel type constant"
```

---

### Task 2: 模型注册表与请求/响应结构

**Files:**
- Create: `adapter/grsai/types.go`
- Test: `adapter/grsai/types_test.go`

**Interfaces:**
- Produces:
  - `type Capability int`，常量 `CapabilityGenerate`、`CapabilityVideo`
  - `type ModelSpec struct { Model, Path string; Capability Capability; MaxImages int }`
  - `func GetModelSpec(model string) (ModelSpec, bool)`
  - `type GenerateRequest struct{...}`、`type ResultRequest struct{ ID string }`
  - `type TaskResponse struct{ ID, Status string; Progress int; Results []TaskResult; Error string }`
  - `type TaskResult struct{ URL string }`
  - `func (r *TaskResponse) IsTerminal() bool`、`func (r *TaskResponse) IsSucceeded() bool`、`func (r *TaskResponse) ErrorMessage(def string) string`

- [ ] **Step 1: 写失败测试**

创建 `adapter/grsai/types_test.go`：

```go
package grsai

import "testing"

func TestGetModelSpec(t *testing.T) {
	cases := []struct {
		model string
		ok    bool
		path  string
		cap   Capability
	}{
		{"nano-banana", true, "/v1/api/generate", CapabilityGenerate},
		{"nano-banana-2", true, "/v1/api/generate", CapabilityGenerate},
		{"gpt-image", true, "/v1/draw/completions", CapabilityGenerate},
		{"veo", true, "/v1/video/veo", CapabilityVideo},
		{"unknown-model", false, "", CapabilityGenerate},
	}
	for _, c := range cases {
		spec, ok := GetModelSpec(c.model)
		if ok != c.ok {
			t.Fatalf("%s: ok=%v want %v", c.model, ok, c.ok)
		}
		if ok && (spec.Path != c.path || spec.Capability != c.cap) {
			t.Fatalf("%s: spec=%+v", c.model, spec)
		}
	}
}

func TestTaskResponseState(t *testing.T) {
	if !(&TaskResponse{Status: "succeeded"}).IsSucceeded() {
		t.Fatal("succeeded should be succeeded")
	}
	if !(&TaskResponse{Status: "failed"}).IsTerminal() {
		t.Fatal("failed should be terminal")
	}
	if (&TaskResponse{Status: "running"}).IsTerminal() {
		t.Fatal("running should not be terminal")
	}
	if got := (&TaskResponse{Status: "failed", Error: "boom"}).ErrorMessage("def"); got != "boom" {
		t.Fatalf("ErrorMessage=%q", got)
	}
	if got := (&TaskResponse{Status: "failed"}).ErrorMessage("def"); got != "def" {
		t.Fatalf("ErrorMessage fallback=%q", got)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./adapter/grsai/ -run TestGetModelSpec -v`
Expected: 编译失败（`undefined: GetModelSpec` 等）。

- [ ] **Step 3: 写实现**

创建 `adapter/grsai/types.go`：

```go
package grsai

// Capability 标识模型能力类别。
type Capability int

const (
	CapabilityGenerate Capability = iota // 文生图 / 图生图
	CapabilityVideo                      // 文/图生视频
)

// ModelSpec 描述一个 grsai 模型的调用方式。
type ModelSpec struct {
	Model      string
	Path       string // 提交接口相对路径，如 /v1/api/generate
	Capability Capability
	MaxImages  int // 允许的最大参考图数量，0 表示不限制
}

// modelSpecs 是 grsai 模型注册表。新增模型只需加一行。
var modelSpecs = map[string]ModelSpec{
	"nano-banana":   {Model: "nano-banana", Path: "/v1/api/generate", Capability: CapabilityGenerate, MaxImages: 6},
	"nano-banana-2": {Model: "nano-banana-2", Path: "/v1/api/generate", Capability: CapabilityGenerate, MaxImages: 6},
	"gpt-image":     {Model: "gpt-image", Path: "/v1/draw/completions", Capability: CapabilityGenerate, MaxImages: 6},
	"veo":           {Model: "veo", Path: "/v1/video/veo", Capability: CapabilityVideo, MaxImages: 1},
}

// GetModelSpec 按模型名查注册表。
func GetModelSpec(model string) (ModelSpec, bool) {
	spec, ok := modelSpecs[model]
	return spec, ok
}

// GenerateRequest 是 nano-banana / gpt-image / veo 的统一提交体。
// veo 不使用 ImageSize，其余字段通用。
type GenerateRequest struct {
	Model       string   `json:"model"`
	Prompt      string   `json:"prompt"`
	Images      []string `json:"images,omitempty"`
	AspectRatio string   `json:"aspectRatio,omitempty"`
	ImageSize   string   `json:"imageSize,omitempty"`
	ReplyType   string   `json:"replyType"`
}

// ResultRequest 是 /v1/draw/result 的入参。
type ResultRequest struct {
	ID string `json:"id"`
}

// TaskResult 单条结果。
type TaskResult struct {
	URL string `json:"url"`
}

// TaskResponse 是提交与结果查询的统一响应体。
type TaskResponse struct {
	ID       string       `json:"id"`
	Status   string       `json:"status"`
	Progress int          `json:"progress"`
	Results  []TaskResult `json:"results"`
	Error    string       `json:"error"`
}

// IsTerminal 终态：succeeded / failed / violation。
func (r *TaskResponse) IsTerminal() bool {
	switch r.Status {
	case "succeeded", "failed", "violation":
		return true
	default:
		return false
	}
}

// IsSucceeded 是否成功。
func (r *TaskResponse) IsSucceeded() bool {
	return r.Status == "succeeded"
}

// ErrorMessage 返回错误信息，缺失时回退到 def。
func (r *TaskResponse) ErrorMessage(def string) string {
	if r.Error != "" {
		return r.Error
	}
	if r.Status == "violation" {
		return "content violation"
	}
	return def
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./adapter/grsai/ -run 'TestGetModelSpec|TestTaskResponseState' -v`
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add adapter/grsai/types.go adapter/grsai/types_test.go
git commit -m "feat(grsai): add model registry and request/response types"
```

---

### Task 3: HTTP 客户端、提交与结果轮询

**Files:**
- Create: `adapter/grsai/client.go`
- Test: `adapter/grsai/client_test.go`

**Interfaces:**
- Consumes: `GenerateRequest`、`ResultRequest`、`TaskResponse`、`ModelSpec`（Task 2）
- Produces:
  - `type Generator struct { instance globals.ChannelConfig; endpoint, apiKey string }`
  - `func newGenerator(conf globals.ChannelConfig) *Generator`
  - `func (c *Generator) postJSON(ctx context.Context, path string, body interface{}, out *TaskResponse) error`
  - `func (c *Generator) Submit(ctx context.Context, spec ModelSpec, body GenerateRequest) (*TaskResponse, error)`
  - `func (c *Generator) PollResult(ctx context.Context, id string, maxWait, interval time.Duration) (*TaskResponse, error)`
  - `func (c *Generator) GetProxy() globals.ProxyConfig`

- [ ] **Step 1: 写失败测试（用 httptest 模拟 submit + poll）**

创建 `adapter/grsai/client_test.go`：

```go
package grsai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/goccy/go-json"
)

// fakeConfig 实现 globals.ChannelConfig，仅用于测试。
type fakeConfig struct {
	endpoint string
	secret   string
	proxy    globals.ProxyConfig
}

func (f fakeConfig) GetType() string                   { return "grsai" }
func (f fakeConfig) GetModelReflect(m string) string   { return m }
func (f fakeConfig) GetRetry() int                     { return 0 }
func (f fakeConfig) GetRandomSecret() string           { return f.secret }
func (f fakeConfig) SplitRandomSecret(n int) []string  { return []string{f.secret} }
func (f fakeConfig) GetEndpoint() string               { return f.endpoint }
func (f fakeConfig) ProcessError(err error) error      { return err }
func (f fakeConfig) GetId() int                        { return 1 }
func (f fakeConfig) GetProxy() globals.ProxyConfig     { return f.proxy }

func TestSubmitAndPoll(t *testing.T) {
	polls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer sk-test" {
			t.Errorf("auth header=%q", got)
		}
		switch r.URL.Path {
		case "/v1/api/generate":
			_ = json.NewEncoder(w).Encode(TaskResponse{ID: "task-1", Status: "running"})
		case "/v1/draw/result":
			polls++
			if polls < 2 {
				_ = json.NewEncoder(w).Encode(TaskResponse{ID: "task-1", Status: "running", Progress: 50})
				return
			}
			_ = json.NewEncoder(w).Encode(TaskResponse{
				ID: "task-1", Status: "succeeded", Progress: 100,
				Results: []TaskResult{{URL: "https://example.com/a.png"}},
			})
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	c := newGenerator(fakeConfig{endpoint: srv.URL, secret: "sk-test"})
	spec, _ := GetModelSpec("nano-banana")
	submit, err := c.Submit(context.Background(), spec, GenerateRequest{Model: "nano-banana", Prompt: "x", ReplyType: "async"})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if submit.ID != "task-1" {
		t.Fatalf("id=%q", submit.ID)
	}
	res, err := c.PollResult(context.Background(), submit.ID, 10*time.Second, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if !res.IsSucceeded() || len(res.Results) != 1 || res.Results[0].URL == "" {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestPollFailedReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(TaskResponse{ID: "t", Status: "failed", Error: "generate failed"})
	}))
	defer srv.Close()
	c := newGenerator(fakeConfig{endpoint: srv.URL, secret: "sk-test"})
	_, err := c.PollResult(context.Background(), "t", 5*time.Second, 50*time.Millisecond)
	if err == nil {
		t.Fatal("expected error on failed status")
	}
}
```

> 注意：测试文件需 `import "chat/globals"`（fakeConfig 引用了 `globals.ProxyConfig`）。在 import 块加上即可。

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./adapter/grsai/ -run TestSubmitAndPoll -v`
Expected: 编译失败（`undefined: newGenerator` 等）。

- [ ] **Step 3: 写实现**

创建 `adapter/grsai/client.go`：

```go
package grsai

import (
	"bytes"
	"chat/globals"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/goccy/go-json"
)

const defaultEndpoint = "https://grsaiapi.com"

// Generator 是 grsai 渠道的调用实例，同时实现多个图片处理接口。
type Generator struct {
	instance globals.ChannelConfig
	endpoint string
	apiKey   string
}

func newGenerator(conf globals.ChannelConfig) *Generator {
	endpoint := strings.TrimSpace(conf.GetEndpoint())
	if endpoint == "" {
		endpoint = defaultEndpoint
	}
	return &Generator{
		instance: conf,
		endpoint: strings.TrimRight(endpoint, "/"),
		apiKey:   strings.TrimSpace(conf.GetRandomSecret()),
	}
}

func (c *Generator) GetProxy() globals.ProxyConfig {
	return c.instance.GetProxy()
}

func (c *Generator) httpClient() *http.Client {
	client := &http.Client{
		Timeout:   globals.HttpMaxTimeout,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
	}
	config := c.GetProxy()
	if config.ProxyType == globals.HttpProxyType || config.ProxyType == globals.HttpsProxyType {
		if proxyURL, err := url.Parse(config.Proxy); err == nil {
			if config.Username != "" || config.Password != "" {
				proxyURL.User = url.UserPassword(config.Username, config.Password)
			}
			client.Transport = &http.Transport{
				Proxy:           http.ProxyURL(proxyURL),
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			}
		}
	}
	return client
}

// postJSON 发送 JSON POST 请求，带 Bearer 认证，解析响应到 out。
func (c *Generator) postJSON(ctx context.Context, path string, body interface{}, out *TaskResponse) error {
	if c.apiKey == "" {
		return fmt.Errorf("grsai requires an API key (secret)")
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+path, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("grsai invalid response (http %d): %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := out.ErrorMessage(strings.TrimSpace(string(raw)))
		return fmt.Errorf("grsai http %d: %s", resp.StatusCode, msg)
	}
	return nil
}

// Submit 提交任务（async），返回带任务 id 的响应。
func (c *Generator) Submit(ctx context.Context, spec ModelSpec, body GenerateRequest) (*TaskResponse, error) {
	body.ReplyType = "async"
	var resp TaskResponse
	if err := c.postJSON(ctx, spec.Path, body, &resp); err != nil {
		return nil, err
	}
	// async 提交通常返回 running + id；少数情况下可能直接返回终态。
	if resp.ID == "" && !resp.IsTerminal() {
		return &resp, fmt.Errorf("grsai submit failed: missing task id")
	}
	if resp.Status == "failed" || resp.Status == "violation" {
		return &resp, fmt.Errorf("grsai submit failed: %s", resp.ErrorMessage("unknown error"))
	}
	return &resp, nil
}

const (
	initialPollInterval = 2 * time.Second
	pollBackoffFactor   = 3 // 每次 ×3/2 退避
)

// PollResult 轮询 /v1/draw/result 直到终态或超时。
func (c *Generator) PollResult(ctx context.Context, id string, maxWait, interval time.Duration) (*TaskResponse, error) {
	if id == "" {
		return nil, fmt.Errorf("grsai poll: empty task id")
	}
	if maxWait <= 0 {
		maxWait = 10 * time.Minute
	}
	if interval <= 0 {
		interval = 10 * time.Second
	}
	maxInterval := interval
	cur := initialPollInterval
	if cur > maxInterval {
		cur = maxInterval
	}

	start := time.Now()
	deadline := start.Add(maxWait)
	for attempt := 1; ; attempt++ {
		var resp TaskResponse
		if err := c.postJSON(ctx, "/v1/draw/result", ResultRequest{ID: id}, &resp); err != nil {
			return nil, err
		}
		globals.Debug(fmt.Sprintf("[grsai] poll #%d id=%s status=%q progress=%d elapsed=%s",
			attempt, id, resp.Status, resp.Progress, time.Since(start).Truncate(time.Second)))

		if resp.IsTerminal() {
			if !resp.IsSucceeded() {
				return &resp, fmt.Errorf("[grsai] task failed (id=%s): %s", id, resp.ErrorMessage("unknown error"))
			}
			globals.Info(fmt.Sprintf("[grsai] task done (id=%s, polls=%d, elapsed=%s)",
				id, attempt, time.Since(start).Truncate(time.Second)))
			return &resp, nil
		}

		if time.Now().Add(cur).After(deadline) {
			return &resp, fmt.Errorf("[grsai] task timeout after %s (id=%s, status=%s)", maxWait, id, resp.Status)
		}
		timer := time.NewTimer(cur)
		select {
		case <-ctx.Done():
			timer.Stop()
			return &resp, ctx.Err()
		case <-timer.C:
		}
		if cur < maxInterval {
			cur = cur * pollBackoffFactor / 2
			if cur > maxInterval {
				cur = maxInterval
			}
		}
	}
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./adapter/grsai/ -run 'TestSubmitAndPoll|TestPollFailedReturnsError' -v`
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add adapter/grsai/client.go adapter/grsai/client_test.go
git commit -m "feat(grsai): add http client with submit and result polling"
```

---

### Task 4: 结果落地（下载图片/视频到 storage/results）

**Files:**
- Create: `adapter/grsai/store.go`
- Test: `adapter/grsai/store_test.go`

**Interfaces:**
- Consumes: `Generator`（Task 3）
- Produces:
  - `func (c *Generator) storeRemoteURL(remote string, imageExt bool) (string, error)` — 下载并落地，返回 `globals.ResultPublicURL(filename)`；`imageExt=true` 用图片扩展名回退 `.png`，否则视频回退 `.mp4`
  - `func resultFilename(source, ext string) string`

- [ ] **Step 1: 写失败测试**

创建 `adapter/grsai/store_test.go`：

```go
package grsai

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"chat/globals"
)

func TestStoreRemoteURL(t *testing.T) {
	globals.StorageResultDir = t.TempDir()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("\x89PNG\r\n\x1a\n-fake-bytes"))
	}))
	defer srv.Close()

	c := newGenerator(fakeConfig{endpoint: "https://grsaiapi.com", secret: "sk-test"})
	public, err := c.storeRemoteURL(srv.URL+"/x.png", true)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	if !strings.HasPrefix(public, globals.ResultPublicURL("")) && public == "" {
		t.Fatalf("public url=%q", public)
	}
	// 文件应真实落地
	entries, _ := os.ReadDir(globals.StorageResultDir)
	if len(entries) != 1 {
		t.Fatalf("expected 1 file, got %d", len(entries))
	}
}

func TestResultFilenameExt(t *testing.T) {
	if name := resultFilename("src", ""); !strings.HasSuffix(name, ".png") {
		t.Fatalf("default ext should be png: %s", name)
	}
	if name := resultFilename("src", ".mp4"); !strings.HasSuffix(name, ".mp4") {
		t.Fatalf("mp4 ext: %s", name)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./adapter/grsai/ -run 'TestStoreRemoteURL|TestResultFilenameExt' -v`
Expected: 编译失败（`undefined: storeRemoteURL`）。

- [ ] **Step 3: 写实现**

创建 `adapter/grsai/store.go`：

```go
package grsai

import (
	"chat/globals"
	"chat/utils"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

func resultFilename(source, ext string) string {
	if ext == "" || ext == "." {
		ext = ".png"
	}
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	hash := utils.Md5Encrypt(source)
	if len(hash) > 10 {
		hash = hash[:10]
	}
	return fmt.Sprintf("grsai_%d_%s_%s%s", time.Now().UnixNano(), hash, utils.GenerateChar(6), ext)
}

// storeRemoteURL 下载 remote 并落地到 storage/results，返回本地公开 URL。
// imageExt=true 时按图片扩展名回退 .png；否则按视频回退 .mp4。
func (c *Generator) storeRemoteURL(remote string, imageExt bool) (string, error) {
	remote = strings.TrimSpace(remote)
	if remote == "" {
		return "", fmt.Errorf("empty result url")
	}
	parsed, err := url.Parse(remote)
	if err != nil {
		return "", err
	}
	ext := strings.ToLower(path.Ext(parsed.Path))
	if imageExt {
		switch ext {
		case ".png", ".jpg", ".jpeg", ".webp", ".gif":
		default:
			ext = ".png"
		}
	} else {
		switch ext {
		case ".mp4", ".webm", ".mov":
		default:
			ext = ".mp4"
		}
	}

	if err := os.MkdirAll(globals.StorageResultDir, 0755); err != nil {
		return "", err
	}
	filename := resultFilename(remote, ext)
	savePath := filepath.Join(globals.StorageResultDir, filename)

	req, err := http.NewRequest(http.MethodGet, remote, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("download grsai result failed: http %d", resp.StatusCode)
	}

	file, err := os.Create(savePath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	if _, err := io.Copy(file, resp.Body); err != nil {
		_ = os.Remove(savePath)
		return "", err
	}
	return globals.ResultPublicURL(filename), nil
}
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./adapter/grsai/ -run 'TestStoreRemoteURL|TestResultFilenameExt' -v`
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add adapter/grsai/store.go adapter/grsai/store_test.go
git commit -m "feat(grsai): add result download and local storage"
```

---

## Phase 2：生图与图生图

### Task 5: 输入图片分类 + 生成/编辑请求

**Files:**
- Create: `adapter/grsai/image.go`
- Test: `adapter/grsai/image_test.go`

**Interfaces:**
- Consumes: `Generator`、`GenerateRequest`、`GetModelSpec`、`storeRemoteURL`
- Produces:
  - `func classifyImages(images []string) []string` — 去 `data:` 前缀，http(s) 与纯 base64 都原样进 `images[]`
  - `func ratioFromSize(w, h *int) string` — 由宽高推导 `aspectRatio`，无则返回 ""
  - `func (c *Generator) CreateImageGenerationRequest(props *adaptercommon.ImageGenerationProps, hook globals.Hook) error`
  - `func (c *Generator) CreateImageEditRequest(props *adaptercommon.ImageEditProps, hook globals.Hook) error`
  - `func (c *Generator) emitResults(res *TaskResponse, hook globals.Hook) error`

- [ ] **Step 1: 写失败测试**

创建 `adapter/grsai/image_test.go`：

```go
package grsai

import (
	"net/http"
	"net/http/httptest"
	"testing"

	adaptercommon "chat/adapter/common"
	"chat/globals"

	"github.com/goccy/go-json"
)

func TestClassifyImages(t *testing.T) {
	in := []string{"  ", "https://a.com/x.png", "data:image/png;base64,AAAA", "BBBB"}
	out := classifyImages(in)
	want := []string{"https://a.com/x.png", "AAAA", "BBBB"}
	if len(out) != len(want) {
		t.Fatalf("got %v", out)
	}
	for i := range want {
		if out[i] != want[i] {
			t.Fatalf("got %v want %v", out, want)
		}
	}
}

func TestCreateImageEditRequest(t *testing.T) {
	globals.StorageResultDir = t.TempDir()
	var gotBody GenerateRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/api/generate":
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			_ = json.NewEncoder(w).Encode(TaskResponse{ID: "t1", Status: "running"})
		case "/v1/draw/result":
			_ = json.NewEncoder(w).Encode(TaskResponse{ID: "t1", Status: "succeeded",
				Results: []TaskResult{{URL: r.Host}}}) // 用一个可下载地址替换见下
		}
	}))
	defer srv.Close()
	// 让结果 URL 指向同一 server 的下载分支
	dl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("img"))
	}))
	defer dl.Close()

	c := newGenerator(fakeConfig{endpoint: srv.URL, secret: "sk-test"})
	// 覆盖结果 URL：重启一个 handler 直接给 dl 地址
	srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/api/generate":
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			_ = json.NewEncoder(w).Encode(TaskResponse{ID: "t1", Status: "running"})
		case "/v1/draw/result":
			_ = json.NewEncoder(w).Encode(TaskResponse{ID: "t1", Status: "succeeded",
				Results: []TaskResult{{URL: dl.URL + "/a.png"}}})
		}
	})

	var emitted []string
	hook := func(ch *globals.Chunk) error { emitted = append(emitted, ch.Content); return nil }
	err := c.CreateImageEditRequest(&adaptercommon.ImageEditProps{
		Model:  "nano-banana",
		Images: []string{"data:image/png;base64,AAAA"},
		Prompt: "make it white bg",
	}, hook)
	if err != nil {
		t.Fatalf("edit: %v", err)
	}
	if gotBody.Model != "nano-banana" || gotBody.Prompt == "" || len(gotBody.Images) != 1 || gotBody.ReplyType != "async" {
		t.Fatalf("body=%+v", gotBody)
	}
	if len(emitted) != 1 {
		t.Fatalf("emitted=%v", emitted)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./adapter/grsai/ -run 'TestClassifyImages|TestCreateImageEditRequest' -v`
Expected: 编译失败（`undefined: classifyImages` / 方法未定义）。

- [ ] **Step 3: 写实现**

创建 `adapter/grsai/image.go`：

```go
package grsai

import (
	adaptercommon "chat/adapter/common"
	"chat/globals"
	"chat/utils"
	"context"
	"fmt"
	"strings"
)

// classifyImages 规整图片输入：去空白、剥离 data: 前缀，http(s) URL 与纯 base64 都进 images[]。
func classifyImages(images []string) []string {
	out := make([]string, 0, len(images))
	for _, img := range images {
		s := strings.TrimSpace(img)
		if s == "" {
			continue
		}
		if strings.HasPrefix(strings.ToLower(s), "data:") {
			if idx := strings.Index(s, ","); idx >= 0 {
				s = strings.TrimSpace(s[idx+1:])
			}
		}
		out = append(out, s)
	}
	return out
}

// ratioFromSize 由目标宽高推导 grsai 的 aspectRatio，无法推导时返回空串。
func ratioFromSize(w, h *int) string {
	if w == nil || h == nil || *w <= 0 || *h <= 0 {
		return ""
	}
	return fmt.Sprintf("%d:%d", *w, *h)
}

// emitResults 把结果下载落地后经 hook 回推（图片 Markdown）。
func (c *Generator) emitResults(res *TaskResponse, hook globals.Hook) error {
	if len(res.Results) == 0 {
		return fmt.Errorf("grsai task finished without result")
	}
	for _, item := range res.Results {
		stored, err := c.storeRemoteURL(item.URL, true)
		if err != nil {
			return err
		}
		if err := hook(&globals.Chunk{Content: utils.GetImageMarkdown(stored)}); err != nil {
			return err
		}
	}
	return nil
}

func (c *Generator) runGenerate(body GenerateRequest, spec ModelSpec, maxWait, interval int64, hook globals.Hook) error {
	ctx := context.Background()
	submit, err := c.Submit(ctx, spec, body)
	if err != nil {
		return err
	}
	// async 提交若直接返回成功结果，则无需轮询。
	if submit.IsSucceeded() && len(submit.Results) > 0 {
		return c.emitResults(submit, hook)
	}
	res, err := c.PollResult(ctx, submit.ID, globals.ImageTaskTimeout(), globals.ImagePollInterval())
	if err != nil {
		return err
	}
	return c.emitResults(res, hook)
}

// CreateImageGenerationRequest 文生图。
func (c *Generator) CreateImageGenerationRequest(props *adaptercommon.ImageGenerationProps, hook globals.Hook) error {
	spec, ok := GetModelSpec(props.Model)
	if !ok || spec.Capability != CapabilityGenerate {
		return fmt.Errorf("grsai 不支持的生图模型: %s", props.Model)
	}
	body := GenerateRequest{
		Model:       spec.Model,
		Prompt:      strings.TrimSpace(props.Prompt),
		Images:      classifyImages(props.Images),
		AspectRatio: ratioFromSize(props.Width, props.Height),
		ReplyType:   "async",
	}
	if spec.MaxImages > 0 && len(body.Images) > spec.MaxImages {
		body.Images = body.Images[:spec.MaxImages]
	}
	return c.runGenerate(body, spec, 0, 0, hook)
}

// CreateImageEditRequest 图生图 / 换色 / 场景 / 擦除等（带参考图）。
func (c *Generator) CreateImageEditRequest(props *adaptercommon.ImageEditProps, hook globals.Hook) error {
	spec, ok := GetModelSpec(props.Model)
	if !ok || spec.Capability != CapabilityGenerate {
		return fmt.Errorf("grsai 不支持的图片编辑模型: %s", props.Model)
	}
	images := classifyImages(props.Images)
	if len(images) == 0 {
		return fmt.Errorf("grsai 图片编辑需要至少 1 张输入图")
	}
	if spec.MaxImages > 0 && len(images) > spec.MaxImages {
		images = images[:spec.MaxImages]
	}
	body := GenerateRequest{
		Model:     spec.Model,
		Prompt:    strings.TrimSpace(props.Prompt),
		Images:    images,
		ReplyType: "async",
	}
	return c.runGenerate(body, spec, 0, 0, hook)
}
```

> `runGenerate` 的 `maxWait/interval int64` 参数当前未用（统一用 `globals.ImageTaskTimeout()/ImagePollInterval()`），保留签名以便视频复用时区分超时；若 lint 报未使用参数，删除这两个形参并在调用处去掉即可。为简洁起见可直接用无额外参数版本——见下方实现说明。

**实现说明（简化）**：把 `runGenerate` 改为不带 `maxWait/interval/spec` 多余参数的版本以避免未用参数：

```go
func (c *Generator) runGenerate(spec ModelSpec, body GenerateRequest, hook globals.Hook) error {
	ctx := context.Background()
	submit, err := c.Submit(ctx, spec, body)
	if err != nil {
		return err
	}
	if submit.IsSucceeded() && len(submit.Results) > 0 {
		return c.emitResults(submit, hook)
	}
	res, err := c.PollResult(ctx, submit.ID, globals.ImageTaskTimeout(), globals.ImagePollInterval())
	if err != nil {
		return err
	}
	return c.emitResults(res, hook)
}
```

并把两个 Create 方法末尾调用改为 `return c.runGenerate(spec, body, hook)`。

- [ ] **Step 4: 运行确认通过**

Run: `go test ./adapter/grsai/ -run 'TestClassifyImages|TestCreateImageEditRequest' -v`
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add adapter/grsai/image.go adapter/grsai/image_test.go
git commit -m "feat(grsai): implement image generation and edit"
```

---

## Phase 3：超分与扩图（用 generate 近似）

### Task 6: 超分与扩图

**Files:**
- Create: `adapter/grsai/edit_extra.go`
- Test: `adapter/grsai/edit_extra_test.go`

**Interfaces:**
- Consumes: `Generator`、`GenerateRequest`、`classifyImages`、`runGenerate`
- Produces:
  - `func (c *Generator) CreateImageUpscaleRequest(props *adaptercommon.ImageUpscaleProps, hook globals.Hook) error` — `ResolutionType`(2k/4k/8k) → `imageSize`(2K/4K)
  - `func (c *Generator) CreateImageOutpaintRequest(props *adaptercommon.ImageOutpaintProps, hook globals.Hook) error` — `TargetRatio` → `aspectRatio`
  - `func normalizeImageSize(res string) string`

- [ ] **Step 1: 写失败测试**

创建 `adapter/grsai/edit_extra_test.go`：

```go
package grsai

import "testing"

func TestNormalizeImageSize(t *testing.T) {
	cases := map[string]string{
		"2k": "2K", "2K": "2K", "4k": "4K", "8k": "4K", "": "2K", "garbage": "2K",
	}
	for in, want := range cases {
		if got := normalizeImageSize(in); got != want {
			t.Fatalf("normalizeImageSize(%q)=%q want %q", in, got, want)
		}
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./adapter/grsai/ -run TestNormalizeImageSize -v`
Expected: 编译失败（`undefined: normalizeImageSize`）。

- [ ] **Step 3: 写实现**

创建 `adapter/grsai/edit_extra.go`：

```go
package grsai

import (
	adaptercommon "chat/adapter/common"
	"chat/globals"
	"fmt"
	"strings"
)

// normalizeImageSize 把 photo 的分辨率档位映射到 grsai imageSize（grsai 上限 4K）。
func normalizeImageSize(res string) string {
	switch strings.ToLower(strings.TrimSpace(res)) {
	case "4k", "8k":
		return "4K"
	case "2k":
		return "2K"
	default:
		return "2K"
	}
}

// CreateImageUpscaleRequest 用 generate + imageSize 近似超分（grsai 无独立超分端点）。
func (c *Generator) CreateImageUpscaleRequest(props *adaptercommon.ImageUpscaleProps, hook globals.Hook) error {
	spec, ok := GetModelSpec(props.Model)
	if !ok || spec.Capability != CapabilityGenerate {
		return fmt.Errorf("grsai 不支持的超分模型: %s", props.Model)
	}
	images := classifyImages([]string{props.Image})
	if len(images) == 0 {
		return fmt.Errorf("grsai 超分需要 1 张输入图")
	}
	body := GenerateRequest{
		Model:     spec.Model,
		Prompt:    "upscale this image to higher resolution, keep all original content and details unchanged",
		Images:    images,
		ImageSize: normalizeImageSize(props.ResolutionType),
		ReplyType: "async",
	}
	return c.runGenerate(spec, body, hook)
}

// CreateImageOutpaintRequest 用 generate + aspectRatio 近似扩图（grsai 无独立扩图端点）。
func (c *Generator) CreateImageOutpaintRequest(props *adaptercommon.ImageOutpaintProps, hook globals.Hook) error {
	spec, ok := GetModelSpec(props.Model)
	if !ok || spec.Capability != CapabilityGenerate {
		return fmt.Errorf("grsai 不支持的扩图模型: %s", props.Model)
	}
	images := classifyImages([]string{props.Image})
	if len(images) == 0 {
		return fmt.Errorf("grsai 扩图需要 1 张输入图")
	}
	prompt := strings.TrimSpace(props.Prompt)
	if prompt == "" {
		prompt = "expand the canvas naturally, extend the background to fill the new area, do not crop the subject"
	}
	body := GenerateRequest{
		Model:       spec.Model,
		Prompt:      prompt,
		Images:      images,
		AspectRatio: strings.TrimSpace(props.TargetRatio),
		ReplyType:   "async",
	}
	return c.runGenerate(spec, body, hook)
}
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./adapter/grsai/ -run TestNormalizeImageSize -v`
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add adapter/grsai/edit_extra.go adapter/grsai/edit_extra_test.go
git commit -m "feat(grsai): approximate upscale and outpaint via generate"
```

---

## Phase 4：视频（veo）

### Task 7: 图/文生视频

**Files:**
- Create: `adapter/grsai/video.go`
- Test: `adapter/grsai/video_test.go`

**Interfaces:**
- Consumes: `Generator`、`GenerateRequest`、`classifyImages`、`storeRemoteURL`、`Submit`、`PollResult`
- Produces:
  - `func (c *Generator) CreateImageToVideoRequest(props *adaptercommon.ImageToVideoProps, hook globals.Hook) error`
  - `const videoPollMaxWait = 20 * time.Minute`

- [ ] **Step 1: 写失败测试**

创建 `adapter/grsai/video_test.go`：

```go
package grsai

import (
	"net/http"
	"net/http/httptest"
	"testing"

	adaptercommon "chat/adapter/common"
	"chat/globals"

	"github.com/goccy/go-json"
)

func TestCreateImageToVideoRequest(t *testing.T) {
	globals.StorageResultDir = t.TempDir()
	dl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("mp4-bytes"))
	}))
	defer dl.Close()

	var gotBody GenerateRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/video/veo":
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			_ = json.NewEncoder(w).Encode(TaskResponse{ID: "v1", Status: "running"})
		case "/v1/draw/result":
			_ = json.NewEncoder(w).Encode(TaskResponse{ID: "v1", Status: "succeeded",
				Results: []TaskResult{{URL: dl.URL + "/out.mp4"}}})
		}
	}))
	defer srv.Close()

	c := newGenerator(fakeConfig{endpoint: srv.URL, secret: "sk-test"})
	var emitted []string
	hook := func(ch *globals.Chunk) error { emitted = append(emitted, ch.Content); return nil }
	err := c.CreateImageToVideoRequest(&adaptercommon.ImageToVideoProps{
		Model:  "veo",
		Images: []string{"data:image/png;base64,AAAA"},
		Prompt: "make it move",
	}, hook)
	if err != nil {
		t.Fatalf("video: %v", err)
	}
	if gotBody.Model != "veo" || len(gotBody.Images) != 1 {
		t.Fatalf("body=%+v", gotBody)
	}
	if len(emitted) != 1 || emitted[0] == "" {
		t.Fatalf("emitted=%v", emitted)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./adapter/grsai/ -run TestCreateImageToVideoRequest -v`
Expected: 编译失败（方法未定义）。

- [ ] **Step 3: 写实现**

创建 `adapter/grsai/video.go`：

```go
package grsai

import (
	adaptercommon "chat/adapter/common"
	"chat/globals"
	"context"
	"fmt"
	"strings"
	"time"
)

// videoPollMaxWait 视频生成较慢，给足轮询时长。
const videoPollMaxWait = 20 * time.Minute

// CreateImageToVideoRequest 图/文生视频（veo）。
func (c *Generator) CreateImageToVideoRequest(props *adaptercommon.ImageToVideoProps, hook globals.Hook) error {
	spec, ok := GetModelSpec(props.Model)
	if !ok || spec.Capability != CapabilityVideo {
		return fmt.Errorf("grsai 不支持的视频模型: %s", props.Model)
	}
	images := classifyImages(props.Images)
	if spec.MaxImages > 0 && len(images) > spec.MaxImages {
		images = images[:spec.MaxImages]
	}
	body := GenerateRequest{
		Model:     spec.Model,
		Prompt:    strings.TrimSpace(props.Prompt),
		Images:    images,
		ReplyType: "async",
	}

	ctx := context.Background()
	submit, err := c.Submit(ctx, spec, body)
	if err != nil {
		return err
	}
	res := submit
	if !(submit.IsSucceeded() && len(submit.Results) > 0) {
		res, err = c.PollResult(ctx, submit.ID, videoPollMaxWait, 10*time.Second)
		if err != nil {
			return err
		}
	}
	if len(res.Results) == 0 || strings.TrimSpace(res.Results[0].URL) == "" {
		return fmt.Errorf("grsai 未返回视频结果 (id=%s)", submit.ID)
	}

	stored, err := c.storeRemoteURL(res.Results[0].URL, false)
	if err != nil {
		globals.Warn(fmt.Sprintf("[grsai] 视频结果落地失败，回退源地址 (id=%s): %s", submit.ID, err))
		stored = res.Results[0].URL
	}
	return hook(&globals.Chunk{Content: stored})
}
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./adapter/grsai/ -run TestCreateImageToVideoRequest -v`
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add adapter/grsai/video.go adapter/grsai/video_test.go
git commit -m "feat(grsai): implement veo image-to-video"
```

---

## Phase 5：接口断言、工厂注册、配置与联调

### Task 8: 构造器与接口断言

**Files:**
- Create: `adapter/grsai/struct.go`
- Test: `adapter/grsai/struct_test.go`

**Interfaces:**
- Consumes: `Generator`、`adaptercommon` 各接口
- Produces:
  - `func NewImageGeneratorFromConfig(conf globals.ChannelConfig) adaptercommon.ImageGenerationFactory`
  - `func NewImageProcessorFromConfig(conf globals.ChannelConfig) adaptercommon.ImageEditFactory`

- [ ] **Step 1: 写失败测试（编译期接口断言）**

创建 `adapter/grsai/struct_test.go`：

```go
package grsai

import (
	adaptercommon "chat/adapter/common"
	"testing"
)

func TestGeneratorImplementsInterfaces(t *testing.T) {
	var _ adaptercommon.ImageGenerationFactory = (*Generator)(nil)
	var _ adaptercommon.ImageEditFactory = (*Generator)(nil)
	var _ adaptercommon.ImageUpscaleFactory = (*Generator)(nil)
	var _ adaptercommon.ImageOutpaintFactory = (*Generator)(nil)
	var _ adaptercommon.ImageToVideoFactory = (*Generator)(nil)

	c := NewImageGeneratorFromConfig(fakeConfig{endpoint: "https://grsaiapi.com", secret: "k"})
	if c == nil {
		t.Fatal("nil generator factory")
	}
	if NewImageProcessorFromConfig(fakeConfig{secret: "k"}) == nil {
		t.Fatal("nil processor factory")
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./adapter/grsai/ -run TestGeneratorImplementsInterfaces -v`
Expected: 编译失败（`undefined: NewImageGeneratorFromConfig`）。

- [ ] **Step 3: 写实现**

创建 `adapter/grsai/struct.go`：

```go
package grsai

import (
	adaptercommon "chat/adapter/common"
	"chat/globals"
)

// NewImageGeneratorFromConfig 供生图工厂表使用。
func NewImageGeneratorFromConfig(conf globals.ChannelConfig) adaptercommon.ImageGenerationFactory {
	return newGenerator(conf)
}

// NewImageProcessorFromConfig 供图片处理工厂表使用；返回实例同时实现编辑/超分/扩图/视频接口，
// adapter 层按需断言。
func NewImageProcessorFromConfig(conf globals.ChannelConfig) adaptercommon.ImageEditFactory {
	return newGenerator(conf)
}

var (
	_ adaptercommon.ImageGenerationFactory = (*Generator)(nil)
	_ adaptercommon.ImageEditFactory       = (*Generator)(nil)
	_ adaptercommon.ImageUpscaleFactory    = (*Generator)(nil)
	_ adaptercommon.ImageOutpaintFactory   = (*Generator)(nil)
	_ adaptercommon.ImageToVideoFactory    = (*Generator)(nil)
)
```

- [ ] **Step 4: 运行确认通过 + 整包测试**

Run: `go test ./adapter/grsai/... -v`
Expected: 全部 PASS。

- [ ] **Step 5: Commit**

```bash
git add adapter/grsai/struct.go adapter/grsai/struct_test.go
git commit -m "feat(grsai): add config constructors and interface assertions"
```

---

### Task 9: 注册到 adapter 工厂表

**Files:**
- Modify: `adapter/adapter.go:52-59`

**Interfaces:**
- Consumes: `grsai.NewImageProcessorFromConfig`、`grsai.NewImageGeneratorFromConfig`、`globals.GrsaiChannelType`

- [ ] **Step 1: 加 import**

在 `adapter/adapter.go` 顶部 import 块中 `"chat/adapter/jimengapi"` 行下方新增：

```go
	"chat/adapter/grsai"
```

- [ ] **Step 2: 注册图片处理工厂**

在 `imageProcessorFactories` map（`adapter/adapter.go:52`）中 `globals.JimengAPIChannelType: jimengapi.NewImageProcessorFromConfig,` 行下方新增：

```go
	globals.GrsaiChannelType: grsai.NewImageProcessorFromConfig,
```

- [ ] **Step 3: 注册生图工厂**

在 `imageGenerationFactories` map（`adapter/adapter.go:57`）中 `globals.JimengAPIChannelType: jimengapi.NewImageGeneratorFromConfig,` 行下方新增：

```go
	globals.GrsaiChannelType: grsai.NewImageGeneratorFromConfig,
```

- [ ] **Step 4: 全量编译**

Run: `go build ./...`
Expected: 编译通过，无输出。

- [ ] **Step 5: Commit**

```bash
git add adapter/adapter.go
git commit -m "feat(grsai): register grsai in adapter factories"
```

---

### Task 10: 配置示例

**Files:**
- Modify: `config.example.yaml`（`channel:` 列表，参照即梦渠道条目附近）

**Interfaces:** 无代码接口。

- [ ] **Step 1: 加渠道示例**

在 `config.example.yaml` 的 `channel:` 列表中（火山即梦条目之后）新增：

```yaml
  - name: grsai 多模型渠道
    type: grsai
    endpoint: https://grsaiapi.com   # 国内直连可改 https://grsai.dakka.com.cn
    secret: sk-xxxxxxxxxxxxxxxx       # grsai API Key（单 key，非 AK|SK）
    models:
      - nano-banana
      - nano-banana-2
      - gpt-image
      - veo
    state: true
    weight: 1
```

- [ ] **Step 2: 校验 YAML 可解析**

Run: `go run ./cli 2>/dev/null || true`（或项目既有的配置校验方式；若无则用 `python3 -c "import yaml,sys; yaml.safe_load(open('config.example.yaml'))"`）
Expected: 无 YAML 解析错误。

- [ ] **Step 3: Commit**

```bash
git add config.example.yaml
git commit -m "docs(grsai): add grsai channel config example"
```

---

### Task 11: live smoke test（环境变量门控）

**Files:**
- Create: `adapter/grsai/live_smoke_test.go`

**Interfaces:**
- Consumes: `Generator`、`CreateImageGenerationRequest`

- [ ] **Step 1: 写门控 smoke test**

创建 `adapter/grsai/live_smoke_test.go`：

```go
package grsai

import (
	"os"
	"testing"

	adaptercommon "chat/adapter/common"
	"chat/globals"
)

// 仅当设置 GRSAI_API_KEY 时运行，真实打一次 nano-banana 生图。
// 运行：GRSAI_API_KEY=sk-xxx go test ./adapter/grsai/ -run TestLiveGenerate -v
func TestLiveGenerate(t *testing.T) {
	key := os.Getenv("GRSAI_API_KEY")
	if key == "" {
		t.Skip("set GRSAI_API_KEY to run live smoke test")
	}
	endpoint := os.Getenv("GRSAI_ENDPOINT")
	if endpoint == "" {
		endpoint = "https://grsaiapi.com"
	}
	globals.StorageResultDir = t.TempDir()

	c := newGenerator(fakeConfig{endpoint: endpoint, secret: key})
	var emitted []string
	hook := func(ch *globals.Chunk) error { emitted = append(emitted, ch.Content); return nil }
	err := c.CreateImageGenerationRequest(&adaptercommon.ImageGenerationProps{
		Model:  "nano-banana",
		Prompt: "a cute corgi poster, vivid colors",
	}, hook)
	if err != nil {
		t.Fatalf("live generate: %v", err)
	}
	if len(emitted) == 0 {
		t.Fatal("no image emitted")
	}
	t.Logf("emitted: %v", emitted)
}
```

- [ ] **Step 2: 默认跳过验证**

Run: `go test ./adapter/grsai/ -run TestLiveGenerate -v`
Expected: SKIP（未设置 key）。

- [ ] **Step 3: Commit**

```bash
git add adapter/grsai/live_smoke_test.go
git commit -m "test(grsai): add gated live smoke test"
```

---

### Task 12: 端到端联调（手动验收）

**Files:** 无代码改动。本地 `config.yaml`（不提交）。

- [ ] **Step 1: 配置真实渠道**

在本地 `config.yaml` 加入 Task 10 的 grsai 渠道，填入真实 `secret`，选好 `endpoint`。

- [ ] **Step 2: 把一个功能指到 grsai**

在 `config/prompts.json` 把 `white_bg` 的 `channel_type` 改为 `grsai`、`model` 改为 `nano-banana`（先备份原值）。

- [ ] **Step 3: 跑起来验证生图**

Run: `go build -o coai . && ./coai`（或项目既有启动方式）
操作：在 photo 页面对一张图执行「白底」功能，确认任务 `succeeded` 且 `storage/results` 下生成图片、前端可预览。

- [ ] **Step 4: 验证视频**

临时把某视频功能（或新增一个测试任务）模型指到 `veo`，发起视频任务，确认 `storage/results` 下生成 `.mp4` 且可下载。

- [ ] **Step 5: 回归即梦**

把 `prompts.json` 改回即梦模型，确认即梦功能不受影响。

- [ ] **Step 6: 还原并提交 prompts（如需）**

```bash
git add config/prompts.json
git commit -m "chore(grsai): wire white_bg to grsai nano-banana (optional)"
```

> 若联调阶段发现 grsai 实际字段与本计划假设不符（gpt-image/veo 尤其需注意），回到 Task 2（模型注册表/请求结构）与对应 Phase 的 body 构造处修正字段名后重跑该 Phase 单测。

---

## Self-Review 记录

- **Spec coverage**：grsai API 规格 → Task 2/3；请求映射 → Task 5/6/7；异步轮询 → Task 3；错误处理 → Task 3（terminal/violation）、Task 7（视频回退）；配置 → Task 10；测试策略 → 各 Task 单测 + Task 11；验收标准 → Task 9（build）、Task 8（单测）、Task 12（端到端）。
- **类型一致性**：`Generator`、`GenerateRequest`、`TaskResponse`、`GetModelSpec`、`runGenerate(spec, body, hook)`、`storeRemoteURL(url, imageExt)`、`classifyImages` 在各 Task 间签名一致。
- **占位扫描**：无 TBD/TODO；唯一「待核对」为 Phase 0 的 gpt-image/veo 字段，已用 nano-banana 已确认 schema 给出可运行默认实现，并在 Task 12 注明校正路径。
- **已知近似**：grsai 无独立超分/扩图端点，用 generate + imageSize/aspectRatio 近似（Task 6 注明）。
