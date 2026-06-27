# 通用 OpenAI 兼容渠道：对话 + 图片处理统一桥接

- 日期：2026-06-28
- 状态：设计待评审
- 涉及目录：`adapter/openai/`、`adapter/adapter.go`、`config/`

## 1. 背景与问题

项目把「对话」和「图片处理」做成两条独立的派发路：

- 对话：`createChatRequest` → `channelFactories`。`OpenAIChannelType` 已注册，任何 OpenAI 兼容渠道（含中转站）填 `type: openai` + baseURL + key 即可对话，包括「对话里出图」（如 nano-banana 经 `/v1/chat/completions` 回 markdown 图）。
- 图片处理（白底图/场景图/换色/擦除/超分/扩图/图生视频）：`createImageGenerationRequest` / `createImageEditRequest` 等 → `imageGenerationFactories` / `imageProcessorFactories`。**这两张工厂表只注册了 `jimeng` / `jimeng-api` / `grsai`，没有 `openai`。**

后果：OpenAI 兼容渠道能对话出图，却无法驱动「图片处理功能」。任何新中转站想接图片处理，都得照 grsai 写一套私有 adapter。

经实测验证：

- **nycatai**（New API 面板）：`/v1/chat/completions` 通；`/v1/images/generations` 对 nano-banana 报「only imagen models supported」。
- **grsai**：同时提供私有面 `/v1/api/generate`（异步）与 OpenAI 兼容面 `/v1/chat/completions`、`/v1/images/generations`（`/v1/images/generations` 实测返回 `{"data":[{"url":...}]}`）。

两家唯一都支持、且天然支持「输入图」的端点是 **`/v1/chat/completions`（多模态）**。

## 2. 目标

> 在渠道里配置一个 `type: openai` 渠道（含 models 列表），**该渠道下的模型，无论用于对话还是图片处理，都能直接使用，无需为每个中转站新增 adapter。**

具体：

1. 写一次「通用 OpenAI 兼容媒体桥」，让 `type: openai` 渠道也实现图片处理全部接口（文生图、图生图/编辑、超分、扩图、图生视频）。
2. 之后接入任何 OpenAI 兼容站（nycatai / grsai / 云雾 兼容面 / …）只改配置，不写代码。
3. 对话路保持不变（已支持）。

## 3. 非目标（Out of scope）

- **私有非 OpenAI 接口**：如 grsai 的 `/v1/api/generate` 异步面、云雾 Seedance 的火山异步面。这类协议结构非 OpenAI，桥覆盖不了，仍需专属 adapter。
- **退役 grsai 现有自定义 adapter**：本次保留，后续可单独评估是否迁到 `type: openai`。
- **模型能力注册表**：不引入「每个模型登记能力/端点」的表（那等于每加模型就写代码，违背目标）。桥信任渠道配置的 models 列表。

## 4. 设计总览

新增一个**通用桥**：`adapter/openai` 下的 `Generator` 类型，内嵌现有 `ChatInstance`，实现全部图片/视频处理接口。每个操作统一走：

```
图片处理请求(props)
  → 构造多模态请求体(prompt + 输入图作为 image_url)
  → POST {endpoint}/v1/chat/completions (非流式)
  → 解析 choices[0].message.content，抠出图/视频(url / 内联base64 / markdown)
  → 落地存储(StoreImage / storeRemoteURL)
  → hook 回推(图: GetImageMarkdown; 视频: 同 grsai 现有格式)
```

回推格式 `globals.Chunk{Content: ...}` 与 grsai 现有图片处理一致，前端无需改动。

## 5. 组件

### 5.1 新文件 `adapter/openai/generator.go`

```go
type Generator struct {
    *ChatInstance
}

func newGenerator(conf globals.ChannelConfig) *Generator {
    return &Generator{ChatInstance: NewChatInstance(conf.GetEndpoint(), conf.GetRandomSecret())}
}

// 工厂入口
func NewImageGeneratorFromConfig(conf globals.ChannelConfig) adaptercommon.ImageGenerationFactory { return newGenerator(conf) }
func NewImageProcessorFromConfig(conf globals.ChannelConfig) adaptercommon.ImageEditFactory       { return newGenerator(conf) }

// 接口实现断言
var (
    _ adaptercommon.ImageGenerationFactory = (*Generator)(nil)
    _ adaptercommon.ImageEditFactory       = (*Generator)(nil)
    _ adaptercommon.ImageUpscaleFactory    = (*Generator)(nil)
    _ adaptercommon.ImageOutpaintFactory   = (*Generator)(nil)
    _ adaptercommon.ImageToVideoFactory    = (*Generator)(nil)
)
```

实现的方法：`CreateImageGenerationRequest`、`CreateImageEditRequest`、`CreateImageUpscaleRequest`、`CreateImageOutpaintRequest`、`CreateImageToVideoRequest`。

### 5.2 统一内部方法

```go
// 把 (prompt, 输入图, 期望媒体类型) 转成一次多模态 chat 调用并回推结果
func (g *Generator) runChatMedia(model, prompt string, images []string, kind mediaKind, hook globals.Hook) error
```

要点：

- **自建多模态消息**，不复用 `formatMessages`（后者用 `IsVisionModel(model)` 门控，nano-banana 等不在表里会被跳过，导致输入图丢失）。消息体形如：
  ```json
  {"role":"user","content":[
    {"type":"text","text":"<prompt>"},
    {"type":"image_url","image_url":{"url":"<图1>"}}
  ]}
  ```
  输入图支持 http(s) URL 与 base64（base64 需补 `data:image/...;base64,` 前缀；复用 grsai `classifyImages` 的规整思路）。
- 非流式 POST，复用 `ChatInstance.GetChatEndpoint` / `GetHeader` / `utils.Post`。
- 解析 `choices[0].message.content`：为字符串时直接用；为数组时拼接其中 text 段。

### 5.3 媒体抽取

- **图片**：`utils.ExtractImages(content, true)` 拿到 url 与内联 base64；逐个 `StoreImage` 落地；`hook(Chunk{Content: GetImageMarkdown(stored)})`。base64 直接 `GetBase64ImageMarkdown`。
- **视频**：从 content 中识别视频链接（markdown 链接或裸 URL，后缀 `.mp4/.mov/.webm`）；落地后 `hook(Chunk{Content: stored})`，与 grsai `CreateImageToVideoRequest` 输出一致。

### 5.4 注册（`adapter/adapter.go`）

```go
var imageGenerationFactories = map[string]adaptercommon.ImageGenerationFactoryCreator{
    globals.JimengAPIChannelType: jimengapi.NewImageGeneratorFromConfig,
    globals.GrsaiChannelType:     grsai.NewImageGeneratorFromConfig,
    globals.OpenAIChannelType:    openai.NewImageGeneratorFromConfig,  // 新增
}

var imageProcessorFactories = map[string]adaptercommon.ImageEditFactoryCreator{
    globals.JimengChannelType:    jimeng.NewCLIAdapterFromConfig,
    globals.JimengAPIChannelType: jimengapi.NewImageProcessorFromConfig,
    globals.GrsaiChannelType:     grsai.NewImageProcessorFromConfig,
    globals.OpenAIChannelType:    openai.NewImageProcessorFromConfig,  // 新增
}
```

`createImageToVideoRequest` 已对返回实例做 `ImageToVideoFactory` 断言，桥实现该接口即自动生效。

## 6. 各操作 → chat 调用映射

| 操作 | 入口接口 | prompt 构造 | 输入图 | 媒体类型 |
|---|---|---|---|---|
| 文生图 | `CreateImageGenerationRequest` | 原 prompt | 可空（props.Images） | image |
| 图生图/编辑（白底/换色/场景/擦除） | `CreateImageEditRequest` | 原 prompt | 必须 ≥1（props.Images） | image |
| 超分 | `CreateImageUpscaleRequest` | 「将图片高清放大到 {ResolutionType}，保持内容不变」 | props.Image | image |
| 扩图 | `CreateImageOutpaintRequest` | 「将画布扩展为 {TargetRatio} 比例，自然延展背景」+ 原 prompt | props.Image | image |
| 图生视频 | `CreateImageToVideoRequest` | 原 prompt | props.Images（首帧） | video |

> 超分/扩图：chat 出图模型不原生支持，属「提示词尽力而为」，结果不保证保真；抽取不到图则按 §7 明确报错。

## 7. 错误处理

- 抽取不到任何图/视频 → 返回明确 error（不回推空内容），保证 `CollectQuota` 在 `err != nil` 时不扣费。
- 内容被安全系统拒绝（content 含 `safety` 等）→ 复用 `CreateImage` 现有判断，返回明确错误而非把错误文本当图。
- 上游返回 OpenAI 错误体（`{"error":{"message":...}}`）→ 透传 message（复用 `hideRequestId` 去掉 request id）。
- 图生图缺输入图 → 直接报错「图片编辑需要至少 1 张输入图」（对齐 grsai）。

## 8. 配置与功能接线

### 8.1 渠道（`config/config.yaml`）

```yaml
- id: 13
  name: nycatai
  type: openai
  endpoint: https://api.nycatai.com/image
  secret: sk-xxx
  models: [nano-banana-2, nano-banana-pro, gpt-image-2]

- id: 14
  name: grsai-openai
  type: openai
  endpoint: https://grsaiapi.com
  secret: sk-xxx
  models: [nano-banana-2, gpt-image-2]
```

### 8.2 功能（`config/prompts.json`）

把需要走通用渠道的图片功能 `channel_type` 改为 `openai`、`model` 指向所配模型。例如 `video_gen` 若改用 nycatai（待其开通视频模型后）：`channel_type: openai`、`model: grok-video-15s`。

## 9. 边界与已知限制

- 仅覆盖**通过 OpenAI 接口返回媒体**的站点。私有异步面（grsai `/v1/api/generate`、云雾 Seedance）不在内。
- 视频：nycatai 的 grok-video 当前未被站点定价开通，调用会被网关拦截（`model_price_error`）；桥逻辑就绪，开通后即可用。
- 超分/扩图为尽力而为，非保真操作。
- 不实现 models 列表探测；模型有效性由配置与上游返回保证。

## 10. 测试

- **单元测试**：mock「chat 返回 markdown/base64 图」「返回视频 url」「返回 OpenAI error 体」「返回纯文本无媒体」四类响应，验证抽取、落地、回推、报错路径。
- **集成/冒烟**（可选，带真 key）：用 grsai、nycatai 各跑文生图、图生图，断言回推为本地存储的图片 markdown。
- 复用现有测试风格（参考 `adapter/grsai/image_test.go`、`live_smoke_test.go`）。

## 11. 后续（未来工作）

- 评估把 grsai 渠道迁到 `type: openai`，退役其自定义 adapter（图像部分）。
- 若需「真异步私有视频」（云雾 Seedance），再单独写专属 adapter，与本桥并存。
