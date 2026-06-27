# grsai 多模型渠道接入设计

日期：2026-06-27
状态：已确认，待实现

## 目标

接入 grsai 平台（https://grsai.com）作为一个新的 AI 图片/视频生成渠道，与现有火山即梦（`jimeng-api`）并列存在。接入后：

- grsai 的生图模型（nano-banana / nano-banana-2 / gpt-image）和视频模型（veo）作为新模型可用。
- `addition/photo` 现有的 17 个功能**代码零改动**，仅通过 `config.yaml` 新增渠道 + `prompts.json` 把某功能指到 grsai 模型即可启用。
- 借 grsai 的 veo 补齐视频能力（即梦当前仅 `jimeng-video` 一个视频模型）。

**非目标**：不新增前端功能入口；不下线即梦；不改动 photo handler/processor/任务表结构。

## 关键决策

1. **接入方式**：新增 `adapter/grsai`，注册进 adapter 工厂表，复用 `channel → adapter` 调度。不在 photo 模块内直接调用外部 API。
2. **模型命名**：使用裸名 `nano-banana`、`nano-banana-2`、`gpt-image`、`veo`（与即梦模型名不冲突，prompts.json 直接引用，无需 mapper）。
3. **reply 模式**：统一 `replyType:"async"`，提交后轮询 `GET /v1/api/result?id=...` 取结果。不使用 stream（避免长连接，对齐即梦 submit/poll 模式）。
4. **结果落地**：下载结果 URL 到 `storage/results`，`hook` 回推本地公开 URL，与即梦行为完全一致。

## 现状参照（镜像模板）

grsai adapter 完整镜像 `adapter/jimengapi` 的结构与约定：

- `adapter/jimengapi/struct.go` — 一个 `ImageGenerator` struct 同时实现 5 个图片处理接口，`From*Config` 构造器。
- `adapter/jimengapi/image.go` — 生成/编辑：提交→轮询→`storeResults`→`hook(&globals.Chunk{Content: GetImageMarkdown(url)})`。
- `adapter/jimengapi/video.go` — 视频：更长轮询超时（20min），下载落地为 .mp4，`hook` 回推裸 URL。
- `adapter/jimengapi/client.go` — HTTP + 退避轮询。
- `adapter/jimengapi/*_test.go` — 表驱动单测 + 环境变量门控的 live smoke test。

接口与 props 定义见 `adapter/common/interface.go`、`adapter/common/types.go`：
- `ImageGenerationFactory.CreateImageGenerationRequest(*ImageGenerationProps, hook)`
- `ImageEditFactory.CreateImageEditRequest(*ImageEditProps, hook)`
- `ImageUpscaleFactory.CreateImageUpscaleRequest(*ImageUpscaleProps, hook)`
- `ImageOutpaintFactory.CreateImageOutpaintRequest(*ImageOutpaintProps, hook)`
- `ImageToVideoFactory.CreateImageToVideoRequest(*ImageToVideoProps, hook)`

工厂注册见 `adapter/adapter.go`：`imageProcessorFactories`（ImageEdit 系，含按接口断言的 upscale/outpaint/video）、`imageGenerationFactories`（生成系）。
渠道调度见 `channel/worker.go`：`NewImageEditRequestWithChannel` 等按 `OriginalModel` 经 `ticker` 路由到匹配渠道。

## grsai API 规格

认证：Header `Authorization: Bearer <sk-...>`，`Content-Type: application/json`。
双 Host：海外 `https://grsaiapi.com`、国内直连 `https://grsai.dakka.com.cn`（由渠道 `endpoint` 决定）。

**重要（实测确认 2026-06-27，用真实 key）**：grsai 实际有**两套并存的接口面**，按模型分流，本接入两套都支持（`ModelSpec.Surface` 字段分发）：

**SurfaceA** — nano-banana 系：
| 能力 | 方法 | 路径 | 响应 |
|---|---|---|---|
| 提交 | POST | `/v1/api/generate` | 扁平 JSON `{"id","status":"running"}` |
| 结果 | **GET** | `/v1/api/result?id=<id>` | `{"id","status","progress","results":[{"url"}],"error"}` |

**SurfaceB** — gpt-image-2 / veo3-* 系：
| 能力 | 方法 | 路径 | 响应 |
|---|---|---|---|
| 提交（图） | POST | `/v1/draw/completions` | **SSE 流** `data: {"id","status","progress",...}`（读首帧取 id） |
| 提交（视频） | POST | `/v1/video/veo` | 同上 SSE |
| 结果 | POST | `/v1/draw/result` body `{"id"}` | **包装** `{"code":0,"data":{"id","status","progress","url","results","error","failure_reason"},"msg"}` |

公共：认证 `Authorization: Bearer`；状态枚举 `running / succeeded / failed / violation`（terminal：后三者）。

**真实模型名（实测，替换早期错误假设）**：
- SurfaceA：`nano-banana`、`nano-banana-2`（生图）
- SurfaceB：`gpt-image-2`（生图；旧 `sora-image` 已下架，官方提示用 gpt-image-2 代替）、`veo3.1-fast`、`veo3.1-pro`（视频；旧 `veo3-*` 已被 google 下架，提示用 veo3.1）
- ❌ `gpt-image`、`veo`、`veo3`、`veo3-fast`、`veo3.1` 均报 "model not found"，不可用。

**结果传递（重要）**：SurfaceB 的结果**来自 SSE 流本身**——提交后流持续推送 `data:` 进度帧，最终帧带 `status:succeeded` + `results[].url`。`/v1/draw/result` 轮询不可靠（长期 running），故 adapter 全程消费流直到终态帧（`streamB`，ctx 控时、客户端不设整体 Timeout 以支持长视频）。SurfaceA 仍是提交 + GET 轮询。

## 请求映射

| photo 调用 | props | grsai body |
|---|---|---|
| 文生图 | `ImageGenerationProps{Prompt, Width/Height/Size}` | `{model, prompt, aspectRatio, imageSize, replyType:"async"}` |
| 图生图/换色/场景/擦除 | `ImageEditProps{Images[], Prompt}` | `{model, prompt, images[](base64或URL), aspectRatio, imageSize, replyType:"async"}` |
| 超分 | `ImageUpscaleProps{Image, ResolutionType(2k/4k)}` | `{model, images:[image], imageSize: 2K/4K, replyType:"async"}` |
| 扩图 | `ImageOutpaintProps{Image, TargetRatio, Prompt}` | `{model, images:[image], aspectRatio: TargetRatio, prompt, replyType:"async"}` |
| 图生视频 | `ImageToVideoProps{Images[], Prompt, Duration}` | `{model:veo3-*, prompt, images[], replyType:"async"}`（SurfaceB） |

图片输入分类复用即梦思路：`http(s)://` 走 URL，其余按 base64（去 data: 前缀）。

## 错误处理

- 结果 `status ∈ {failed, violation}` → 返回错误，交由 `channel` 的 ticker 多渠道降级（同功能可再配即梦兜底）。
- 提交非 2xx / 缺 `id` → 返回错误。
- 轮询超时 → 返回超时错误（图片用 `globals.ImageTaskTimeout()`，视频用 20min）。
- 结果下载落地失败 → 视频回退源 URL（对齐即梦）；图片返回错误。

## 配置

`config.example.yaml` 增加示例渠道：

```yaml
- name: grsai 多模型渠道
  type: grsai
  endpoint: https://grsaiapi.com   # 或 https://grsai.dakka.com.cn
  secret: sk-xxxxxxxx              # grsai API Key（单 key，非 AK/SK）
  models:
    - nano-banana
    - nano-banana-2
    - gpt-image
    - veo
  state: true
  weight: 1
```

> 注意：grsai 用单一 Bearer key，**不**使用即梦的 `SplitRandomSecret(2)`（AK|SK）格式。

## 测试策略

- 表驱动单测（mock HTTP，不打真实 API）：模型注册表查询、各能力 body 构造、async/result 响应解析、图片输入分类、错误分支。
- 环境变量门控的 live smoke test（对齐 `jimengapi/live_smoke_test.go`）：设置 `GRSAI_API_KEY` 才跑，真实打一次 nano-banana 生图。
- 回归：`go build ./...` + `go test ./adapter/grsai/...`。

## 验收标准

1. `go build ./...` 通过。
2. `adapter/grsai` 单测通过。
3. 配好 grsai 渠道 + 把某 photo 功能（如 white_bg）在 prompts.json 指到 `nano-banana` 后，该功能端到端跑通并出图。
4. 配一个 veo 模型并发起视频生成任务能产出本地 .mp4。
5. 即梦渠道与现有功能不受影响。
