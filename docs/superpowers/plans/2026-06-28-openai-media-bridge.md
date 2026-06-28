# 通用 OpenAI 兼容媒体桥 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 `type: openai` 渠道实现图片处理全部接口，使「渠道里配置的模型，对话与图片处理都能用」，无需为每个 OpenAI 兼容中转站写 adapter。

**Architecture:** 新增 `adapter/openai/generator.go`，一个 `Generator` 内嵌现有 `ChatInstance`，把每个图片/视频操作统一转成一次多模态 `/v1/chat/completions` 调用，从响应文本里抠出图/视频并经 hook 回推；在 `adapter/adapter.go` 的两张图片工厂表注册 `openai`。

**Tech Stack:** Go；复用 `adapter/openai`（ChatInstance、ChatRequest/ChatResponse、Message/MessageContent/ImageUrl）与 `chat/utils`（Post、MapToStruct、ExtractImages、ExtractUrls、StoreImage、GetImageMarkdown）。

## Global Constraints

- 回推格式统一为 `globals.Chunk{Content: ...}`：图片用 `![image](...)` markdown；视频用落地后的裸 URL（对齐 `adapter/grsai/video.go` 的 `CreateImageToVideoRequest`）。
- 抽取不到媒体时返回 `error`，**不得** 回推空内容（保证 `CollectQuota` 在 `err != nil` 时不扣费）。
- 不引入「每模型能力注册表」；模型有效性由渠道配置与上游返回决定。
- 桥**自建多模态消息**，不复用 `formatMessages`（其 `IsVisionModel` 门控会丢掉输入图）。
- 所有新代码在 `package openai` / `package adapter` 内，遵循现有文件风格。

---

### Task 1: 纯函数 —— 输入图规整、消息构造、媒体抽取

**Files:**
- Create: `adapter/openai/generator.go`
- Test: `adapter/openai/generator_test.go`

**Interfaces:**
- Consumes: `chat/utils`（`ExtractImages`、`ExtractUrls`）、`chat/globals`（`User`）、本包 `Message`/`MessageContent`/`ImageUrl`（`adapter/openai/types.go`）。
- Produces（供 Task 2 使用）：
  - `func normalizeInputImage(s string) string`
  - `func normalizeImages(images []string) []string`
  - `func buildUserMessage(prompt string, images []string) Message`
  - `func extractImages(content string) []string`
  - `func extractVideoURLs(content string) []string`

- [ ] **Step 1: 写失败测试**

写入 `adapter/openai/generator_test.go`：

```go
package openai

import "testing"

func TestNormalizeInputImage(t *testing.T) {
	cases := map[string]string{
		"  https://x.com/a.png ": "https://x.com/a.png",
		"data:image/png;base64,AAA": "data:image/png;base64,AAA",
		"AAABBB": "data:image/png;base64,AAABBB",
		"   ": "",
	}
	for in, want := range cases {
		if got := normalizeInputImage(in); got != want {
			t.Fatalf("normalizeInputImage(%q)=%q want %q", in, got, want)
		}
	}
}

func TestBuildUserMessage(t *testing.T) {
	msg := buildUserMessage("画只猫", []string{"https://x.com/a.png", "  "})
	if msg.Role != "user" {
		t.Fatalf("role=%q", msg.Role)
	}
	if len(msg.Content) != 2 { // 1 text + 1 image（空白图被丢弃）
		t.Fatalf("content len=%d want 2", len(msg.Content))
	}
	if msg.Content[0].Type != "text" || msg.Content[0].Text == nil || *msg.Content[0].Text != "画只猫" {
		t.Fatalf("text block wrong: %+v", msg.Content[0])
	}
	if msg.Content[1].Type != "image_url" || msg.Content[1].ImageUrl == nil || msg.Content[1].ImageUrl.Url != "https://x.com/a.png" {
		t.Fatalf("image block wrong: %+v", msg.Content[1])
	}
}

func TestExtractImages(t *testing.T) {
	// base64 data URI（nano-banana 风格）
	b64 := "![image](data:image/jpeg;base64,/9j/AAAB+/=)"
	got := extractImages(b64)
	if len(got) != 1 || got[0] != "data:image/jpeg;base64,/9j/AAAB+/=" {
		t.Fatalf("base64 extract=%v", got)
	}
	// http 图片 URL
	httpc := "done ![image](https://file.x.com/a.png)"
	got = extractImages(httpc)
	if len(got) != 1 || got[0] != "https://file.x.com/a.png" {
		t.Fatalf("http extract=%v", got)
	}
	// 无图
	if got = extractImages("纯文本没有图"); len(got) != 0 {
		t.Fatalf("expected no images, got %v", got)
	}
}

func TestExtractVideoURLs(t *testing.T) {
	content := "生成完成 ![video](https://file.x.com/v.mp4) 另有 https://file.x.com/b.webm?sig=1"
	got := extractVideoURLs(content)
	if len(got) != 2 {
		t.Fatalf("video extract=%v want 2", got)
	}
	// 图片 URL 不应被当成视频
	if v := extractVideoURLs("![image](https://x.com/a.png)"); len(v) != 0 {
		t.Fatalf("png should not be video: %v", v)
	}
}
```

- [ ] **Step 2: 运行测试，确认失败**

Run: `cd /Users/wuzhixuan/code/project/coai && go test ./adapter/openai/ -run 'TestNormalizeInputImage|TestBuildUserMessage|TestExtractImages|TestExtractVideoURLs' -v`
Expected: 编译失败 / `undefined: normalizeInputImage` 等。

- [ ] **Step 3: 写最小实现**

写入 `adapter/openai/generator.go`：

```go
package openai

import (
	"chat/globals"
	"chat/utils"
	"strings"
)

// normalizeInputImage 把输入图规整为 OpenAI image_url 可接受的形式：
// http(s) 与 data: 原样返回；裸 base64 补 data:image/png;base64, 前缀；空白返回 ""。
func normalizeInputImage(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	low := strings.ToLower(s)
	if strings.HasPrefix(low, "http://") || strings.HasPrefix(low, "https://") || strings.HasPrefix(low, "data:") {
		return s
	}
	return "data:image/png;base64," + s
}

// normalizeImages 规整并丢弃空白项。
func normalizeImages(images []string) []string {
	out := make([]string, 0, len(images))
	for _, img := range images {
		if n := normalizeInputImage(img); n != "" {
			out = append(out, n)
		}
	}
	return out
}

// buildUserMessage 构造一条多模态 user 消息：text + 若干 image_url。
func buildUserMessage(prompt string, images []string) Message {
	p := prompt
	contents := MessageContents{{Type: "text", Text: &p}}
	for _, u := range normalizeImages(images) {
		url := u
		contents = append(contents, MessageContent{
			Type:     "image_url",
			ImageUrl: &ImageUrl{Url: url},
		})
	}
	return Message{Role: globals.User, Content: contents}
}

// extractImages 从响应文本抠出图片（http 图片 URL 与 base64 data URI）。
func extractImages(content string) []string {
	_, images := utils.ExtractImages(content, true)
	return images
}

// extractVideoURLs 从响应文本抠出视频链接（.mp4/.webm/.mov，允许带 query）。
func extractVideoURLs(content string) []string {
	var out []string
	for _, u := range utils.ExtractUrls(content) {
		clean := strings.TrimRight(u, ").,\"'")
		p := strings.ToLower(clean)
		if i := strings.IndexAny(p, "?#"); i >= 0 {
			p = p[:i]
		}
		if strings.HasSuffix(p, ".mp4") || strings.HasSuffix(p, ".webm") || strings.HasSuffix(p, ".mov") {
			out = append(out, clean)
		}
	}
	return out
}
```

- [ ] **Step 4: 运行测试，确认通过**

Run: `cd /Users/wuzhixuan/code/project/coai && go test ./adapter/openai/ -run 'TestNormalizeInputImage|TestBuildUserMessage|TestExtractImages|TestExtractVideoURLs' -v`
Expected: PASS（4 个测试）。

- [ ] **Step 5: 提交**

```bash
cd /Users/wuzhixuan/code/project/coai
git add adapter/openai/generator.go adapter/openai/generator_test.go
git commit -m "feat(openai): 媒体桥纯函数——输入图规整/消息构造/媒体抽取"
```

---

### Task 2: Generator 类型、工厂入口与 runChatMedia + 五个接口方法

**Files:**
- Modify: `adapter/openai/generator.go`
- Test: `adapter/openai/generator_test.go`（追加）

**Interfaces:**
- Consumes（Task 1 产出）：`buildUserMessage`、`normalizeImages`、`extractImages`、`extractVideoURLs`；本包 `ChatInstance`/`NewChatInstance`/`GetEndpoint`/`GetHeader`/`hideRequestId`（`adapter/openai/chat.go`）、`ChatRequest`/`ChatResponse`（`adapter/openai/types.go`）；`chat/utils`（`Post`、`MapToStruct`、`StoreImage`、`GetImageMarkdown`）；`adapter/common` 的 props 与接口。
- Produces（供 Task 3 使用）：
  - `func NewImageGeneratorFromConfig(conf globals.ChannelConfig) adaptercommon.ImageGenerationFactory`
  - `func NewImageProcessorFromConfig(conf globals.ChannelConfig) adaptercommon.ImageEditFactory`
  - `*Generator` 实现 `ImageGenerationFactory`/`ImageEditFactory`/`ImageUpscaleFactory`/`ImageOutpaintFactory`/`ImageToVideoFactory`。

- [ ] **Step 1: 写失败测试（接口符合性 + 编辑缺图报错）**

追加到 `adapter/openai/generator_test.go`：

```go
import (
	adaptercommon "chat/adapter/common"
	"strings"
	"testing"
)

func TestGeneratorImplementsInterfaces(t *testing.T) {
	var g interface{} = &Generator{}
	if _, ok := g.(adaptercommon.ImageGenerationFactory); !ok {
		t.Fatal("not ImageGenerationFactory")
	}
	if _, ok := g.(adaptercommon.ImageEditFactory); !ok {
		t.Fatal("not ImageEditFactory")
	}
	if _, ok := g.(adaptercommon.ImageUpscaleFactory); !ok {
		t.Fatal("not ImageUpscaleFactory")
	}
	if _, ok := g.(adaptercommon.ImageOutpaintFactory); !ok {
		t.Fatal("not ImageOutpaintFactory")
	}
	if _, ok := g.(adaptercommon.ImageToVideoFactory); !ok {
		t.Fatal("not ImageToVideoFactory")
	}
}

func TestImageEditRequiresInputImage(t *testing.T) {
	g := &Generator{ChatInstance: NewChatInstance("https://example.com", "sk-x")}
	err := g.CreateImageEditRequest(&adaptercommon.ImageEditProps{
		Model: "nano-banana-2", Prompt: "换白底", Images: []string{"  "},
	}, func(*globals.Chunk) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "至少 1 张") {
		t.Fatalf("want missing-image error, got %v", err)
	}
}
```

（`globals` 已在 Task 1 的测试文件导入；若编译报未导入，按提示补 `"chat/globals"`。）

- [ ] **Step 2: 运行测试，确认失败**

Run: `cd /Users/wuzhixuan/code/project/coai && go test ./adapter/openai/ -run 'TestGeneratorImplementsInterfaces|TestImageEditRequiresInputImage' -v`
Expected: 编译失败 / `undefined: Generator`。

- [ ] **Step 3: 追加实现到 `adapter/openai/generator.go`**

在文件末尾追加（并把 import 区补上 `adaptercommon "chat/adapter/common"` 和 `"fmt"`）：

```go
// Generator 通用 OpenAI 兼容媒体桥：把图片/视频操作统一走多模态 chat。
type Generator struct {
	*ChatInstance
}

func newGenerator(conf globals.ChannelConfig) *Generator {
	return &Generator{ChatInstance: NewChatInstance(conf.GetEndpoint(), conf.GetRandomSecret())}
}

// NewImageGeneratorFromConfig 供生图工厂表使用。
func NewImageGeneratorFromConfig(conf globals.ChannelConfig) adaptercommon.ImageGenerationFactory {
	return newGenerator(conf)
}

// NewImageProcessorFromConfig 供图片处理工厂表使用；同一实例实现编辑/超分/扩图/视频接口。
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

// runChatMedia 发一次多模态 /v1/chat/completions，从响应里抠出图/视频并回推。
func (g *Generator) runChatMedia(model, prompt string, images []string, wantVideo bool, proxy globals.ProxyConfig, hook globals.Hook) error {
	body := ChatRequest{
		Model:    model,
		Messages: []Message{buildUserMessage(prompt, images)},
		Stream:   false,
	}
	res, err := utils.Post(
		fmt.Sprintf("%s/v1/chat/completions", g.GetEndpoint()),
		g.GetHeader(), body, proxy,
	)
	if err != nil || res == nil {
		if err != nil {
			return fmt.Errorf("openai media request failed: %s", err.Error())
		}
		return fmt.Errorf("openai media request failed: empty response")
	}

	data := utils.MapToStruct[ChatResponse](res)
	if data == nil {
		return fmt.Errorf("openai media error: cannot parse response")
	}
	if data.Error.Message != "" {
		return fmt.Errorf(hideRequestId(data.Error.Message))
	}
	if len(data.Choices) == 0 {
		return fmt.Errorf("openai media error: empty choices")
	}
	content := data.Choices[0].Message.Content

	if wantVideo {
		urls := extractVideoURLs(content)
		if len(urls) == 0 {
			return fmt.Errorf("未从响应中解析到视频结果")
		}
		for _, u := range urls {
			if err := hook(&globals.Chunk{Content: utils.StoreImage(u)}); err != nil {
				return err
			}
		}
		return nil
	}

	imgs := extractImages(content)
	if len(imgs) == 0 {
		return fmt.Errorf("未从响应中解析到图片结果")
	}
	for _, img := range imgs {
		markdown := utils.GetImageMarkdown(img) // data: URI 直接内联
		if !strings.HasPrefix(img, "data:") {
			markdown = utils.GetImageMarkdown(utils.StoreImage(img)) // http 图落地后再回推
		}
		if err := hook(&globals.Chunk{Content: markdown}); err != nil {
			return err
		}
	}
	return nil
}

// CreateImageGenerationRequest 文生图（参考图可空）。
func (g *Generator) CreateImageGenerationRequest(props *adaptercommon.ImageGenerationProps, hook globals.Hook) error {
	return g.runChatMedia(props.Model, strings.TrimSpace(props.Prompt), props.Images, false, props.Proxy, hook)
}

// CreateImageEditRequest 图生图 / 换色 / 场景 / 擦除（需输入图）。
func (g *Generator) CreateImageEditRequest(props *adaptercommon.ImageEditProps, hook globals.Hook) error {
	if len(normalizeImages(props.Images)) == 0 {
		return fmt.Errorf("图片编辑需要至少 1 张输入图")
	}
	return g.runChatMedia(props.Model, strings.TrimSpace(props.Prompt), props.Images, false, props.Proxy, hook)
}

// CreateImageUpscaleRequest 超分（chat 尽力而为）。
func (g *Generator) CreateImageUpscaleRequest(props *adaptercommon.ImageUpscaleProps, hook globals.Hook) error {
	prompt := fmt.Sprintf("将这张图片高清放大到 %s 分辨率，保持画面内容与构图不变，仅提升清晰度与细节。", props.ResolutionType)
	return g.runChatMedia(props.Model, prompt, []string{props.Image}, false, props.Proxy, hook)
}

// CreateImageOutpaintRequest 扩图（chat 尽力而为）。
func (g *Generator) CreateImageOutpaintRequest(props *adaptercommon.ImageOutpaintProps, hook globals.Hook) error {
	prompt := strings.TrimSpace(fmt.Sprintf("将这张图片的画布扩展为 %s 比例，自然延展周围背景，保持主体不变。%s", props.TargetRatio, props.Prompt))
	return g.runChatMedia(props.Model, prompt, []string{props.Image}, false, props.Proxy, hook)
}

// CreateImageToVideoRequest 图/文生视频。
func (g *Generator) CreateImageToVideoRequest(props *adaptercommon.ImageToVideoProps, hook globals.Hook) error {
	return g.runChatMedia(props.Model, strings.TrimSpace(props.Prompt), props.Images, true, props.Proxy, hook)
}
```

- [ ] **Step 4: 运行测试，确认通过**

Run: `cd /Users/wuzhixuan/code/project/coai && go test ./adapter/openai/ -v`
Expected: 全部 PASS（含 Task 1 的 4 个 + 本任务 2 个）。

- [ ] **Step 5: 提交**

```bash
cd /Users/wuzhixuan/code/project/coai
git add adapter/openai/generator.go adapter/openai/generator_test.go
git commit -m "feat(openai): 媒体桥 Generator——五接口走多模态 chat，抠图/视频回推"
```

---

### Task 3: 在工厂表注册 openai + 注册测试 + 渠道配置示例

**Files:**
- Modify: `adapter/adapter.go:61-64`（`imageGenerationFactories`）、`adapter/adapter.go:55-59`（`imageProcessorFactories`）
- Test: `adapter/adapter_register_test.go`（新建）
- Modify（部署配置，手动）：`config/config.yaml`

**Interfaces:**
- Consumes（Task 2 产出）：`openai.NewImageGeneratorFromConfig`、`openai.NewImageProcessorFromConfig`。
- Produces：`imageGenerationFactories[globals.OpenAIChannelType]` 与 `imageProcessorFactories[globals.OpenAIChannelType]` 非空。

- [ ] **Step 1: 写失败测试**

新建 `adapter/adapter_register_test.go`：

```go
package adapter

import (
	"chat/globals"
	"testing"
)

func TestOpenAIRegisteredForImagePipelines(t *testing.T) {
	if imageGenerationFactories[globals.OpenAIChannelType] == nil {
		t.Fatal("openai 未注册到 imageGenerationFactories")
	}
	if imageProcessorFactories[globals.OpenAIChannelType] == nil {
		t.Fatal("openai 未注册到 imageProcessorFactories")
	}
}
```

- [ ] **Step 2: 运行测试，确认失败**

Run: `cd /Users/wuzhixuan/code/project/coai && go test ./adapter/ -run TestOpenAIRegisteredForImagePipelines -v`
Expected: FAIL（map 中无 openai，取到 nil）。

- [ ] **Step 3: 在 `adapter/adapter.go` 两张表各加一行**

`imageProcessorFactories`（约 55-59 行）加入：

```go
	globals.OpenAIChannelType: openai.NewImageProcessorFromConfig,
```

`imageGenerationFactories`（约 61-64 行）加入：

```go
	globals.OpenAIChannelType: openai.NewImageGeneratorFromConfig,
```

（`openai` 包已在 `adapter.go:18` 导入，无需改 import。）

- [ ] **Step 4: 运行测试，确认通过**

Run: `cd /Users/wuzhixuan/code/project/coai && go test ./adapter/ -run TestOpenAIRegisteredForImagePipelines -v && go build ./...`
Expected: PASS 且 `go build ./...` 无错误。

- [ ] **Step 5: 提交**

```bash
cd /Users/wuzhixuan/code/project/coai
git add adapter/adapter.go adapter/adapter_register_test.go
git commit -m "feat(adapter): 注册 openai 到图片生成/处理工厂表"
```

- [ ] **Step 6: 配置渠道（手动，部署时）**

在 `config/config.yaml` 的 `channel:` 数组追加（`secret` 换成自己的 key，`id` 取未占用值）：

```yaml
  - id: 13
    name: nycatai
    type: openai
    endpoint: https://api.nycatai.com/image
    secret: sk-你的KEY
    models:
      - nano-banana-2
      - nano-banana-pro
      - gpt-image-2
```

如需让某个图片功能走该渠道，在 `config/prompts.json` 把对应 feature 的 `channel_type` 改为 `openai`、`model` 指向上面所配模型。

---

### Task 4: 真实链路冒烟验证（手动，带 key）

**Files:**
- Test: `adapter/openai/live_smoke_test.go`（新建，参考 `adapter/grsai/live_smoke_test.go`）

**Interfaces:**
- Consumes：`Generator`、`CreateImageGenerationRequest`、`CreateImageEditRequest`。

- [ ] **Step 1: 写带环境门控的冒烟测试**

新建 `adapter/openai/live_smoke_test.go`：

```go
package openai

import (
	adaptercommon "chat/adapter/common"
	"chat/globals"
	"os"
	"strings"
	"testing"
)

// 设置 OPENAI_BRIDGE_ENDPOINT 与 OPENAI_BRIDGE_KEY 后运行：
// OPENAI_BRIDGE_ENDPOINT=https://api.nycatai.com/image OPENAI_BRIDGE_KEY=sk-xxx \
//   go test ./adapter/openai/ -run TestLiveBridge -v
func TestLiveBridgeImageGeneration(t *testing.T) {
	endpoint, key := os.Getenv("OPENAI_BRIDGE_ENDPOINT"), os.Getenv("OPENAI_BRIDGE_KEY")
	if endpoint == "" || key == "" {
		t.Skip("set OPENAI_BRIDGE_ENDPOINT and OPENAI_BRIDGE_KEY to run")
	}
	g := &Generator{ChatInstance: NewChatInstance(endpoint, key)}

	var got string
	err := g.CreateImageGenerationRequest(&adaptercommon.ImageGenerationProps{
		Model: "nano-banana-2", Prompt: "a red cup on white background",
	}, func(c *globals.Chunk) error { got += c.Content; return nil })
	if err != nil {
		t.Fatalf("generation failed: %v", err)
	}
	if !strings.Contains(got, "![image](") {
		t.Fatalf("expected image markdown, got: %.80s", got)
	}
}
```

- [ ] **Step 2: 运行（带 key）**

Run:
```bash
cd /Users/wuzhixuan/code/project/coai
OPENAI_BRIDGE_ENDPOINT=https://api.nycatai.com/image OPENAI_BRIDGE_KEY=sk-你的KEY \
  go test ./adapter/openai/ -run TestLiveBridgeImageGeneration -v
```
Expected: PASS（无 key 时 SKIP）。

- [ ] **Step 3: 提交**

```bash
cd /Users/wuzhixuan/code/project/coai
git add adapter/openai/live_smoke_test.go
git commit -m "test(openai): 媒体桥真实链路冒烟（环境门控）"
```

---

## Self-Review

- **Spec coverage:**
  - §5.1 新文件 Generator → Task 2 ✅
  - §5.2 runChatMedia + 自建多模态（不复用 formatMessages）→ Task 1（buildUserMessage）+ Task 2 ✅
  - §5.3 媒体抽取（图片 ExtractImages / 视频 ExtractUrls 过滤）→ Task 1 ✅
  - §5.4 两表注册 → Task 3 ✅
  - §6 操作映射（生成/编辑/超分/扩图/图生视频）→ Task 2 五方法 ✅
  - §7 错误处理（无媒体报错不扣费、上游 error 透传、缺图报错）→ Task 2 ✅
  - §8 配置与功能接线 → Task 3 Step 6 ✅
  - §10 测试（单元 + 冒烟）→ Task 1/2/3 单测 + Task 4 冒烟 ✅
- **Placeholder scan:** 无 TBD/TODO；每个代码步骤含完整代码。
- **Type consistency:** `Generator`、`runChatMedia(model, prompt, images, wantVideo, proxy, hook)`、`NewImageGeneratorFromConfig`/`NewImageProcessorFromConfig`、五个 `Create*Request` 方法名在各任务一致；复用的 `ChatResponse.Error.Message`、`hideRequestId`、`globals.Message.Content(string)`、`utils.StoreImage`/`GetImageMarkdown`/`ExtractImages`/`ExtractUrls` 均与现有源码签名一致。

> 注：§7 提到的 safety 拒绝场景，由 `runChatMedia` 中 `data.Error.Message != ""` 透传上游错误覆盖（中转站对违规内容通常返回 error 体而非正常 content），不再单列分支。
