# Photo2 → CoAI 功能迁移规划文档

> 目标：将 photo2 (电商图片处理 AI 工具) 的全部 15 大功能迁移到 CoAI 平台，并以**多 AI 供应商可配置**的方式实现。

---

## 1. 两项目架构对比

| 维度 | photo2 (迁移源) | coai (迁移目标) |
|------|----------------|----------------|
| **后端语言** | Python 3.11 + FastAPI | Go 1.20 + Gin |
| **前端框架** | React 18 + Ant Design 5 | React 18 + Radix UI + Tailwind |
| **状态管理** | 自定义 useImageTask Hook | Redux Toolkit |
| **路由** | SPA 单页 (HomePage) | 多页面路由 (react-router) |
| **数据库** | 无 (JSON 文件持久化) | MySQL / SQLite + Redis |
| **AI 供应商** | 单一 (dreamina CLI subprocess) | 20+ 适配器 (OpenAI/Claude/Azure/MJ...) |
| **渠道管理** | 无 | Channel 优先级/权重/分组/重试/密钥池 |
| **任务队列** | Python Thread | Channel Ticker 轮转 + 已有视频 Job 轮询 |
| **文件存储** | 本地 storage/ 目录 | 本地 + 已有图片代理存储 |
| **计费/配额** | 无 | 完整计费系统 (token/times/non-billing) |
| **用户体系** | 无 | 完整鉴权 (JWT/API Key/订阅/分组) |

---

## 2. 架构设计：新模块划分

### 2.1 目录结构

```
coai/                                    # 现有项目
├── addition/
│   └── photo/                  [新增]   # 电商图片处理核心模块
│       ├── router.go                   # API 路由注册
│       ├── types.go                    # 数据结构 + FeatureType 枚举
│       ├── handler.go                  # HTTP 请求处理
│       ├── processor.go                # 15 功能分发 + 并行调度
│       ├── prompts.go                  # 提示词模板管理
│       ├── local.go                    # 本地图像处理 (裁剪/Logo)
│       ├── upload.go                   # 图片上传 + 文件校验
│       ├── task.go                     # 任务 CRUD + 状态轮询
│       └── download.go                 # 单文件/ZIP 下载
├── adapter/
│   ├── common/
│   │   └── interface.go       [扩展]   # 新增 ImageEditFactory 等接口
│   └── dreamina/              [新增]   # 即梦云 API 适配器
│       ├── chat.go                     # 实现 Factory 接口 (复用聊天)
│       ├── image.go                    # 图片处理 (image2image/upscale/outpaint)
│       ├── video.go                    # 视频生成 (multimodal2video)
│       ├── struct.go                   # 数据结构
│       └── types.go                    # 请求/响应类型
├── config/
│   └── prompts.json           [新增]   # 15 功能提示词模板
├── storage/                   [新增]
│   ├── uploads/                        # 用户上传图片
│   └── results/                        # AI 处理结果
├── app/src/
│   ├── routes/
│   │   └── Photo.tsx          [新增]   # 图片处理主页面
│   ├── components/photo/
│   │   ├── UploadPanel.tsx    [新增]   # 上传面板
│   │   ├── FeaturePanel.tsx   [新增]   # 功能面板
│   │   └── TaskTable.tsx      [新增]   # 任务表格
│   ├── api/
│   │   └── photo.ts           [新增]   # API 封装
│   └── hooks/
│       └── usePhotoTask.ts    [新增]   # 状态管理 Hook
```

### 2.2 核心接口设计

#### 扩展适配器通用接口 (`adapter/common/interface.go`)

```go
// 新增：图片编辑接口
type ImageEditFactory interface {
    CreateImageEditRequest(props *ImageEditProps, hook globals.Hook) error
}

// 新增：图片放大接口
type ImageUpscaleFactory interface {
    CreateImageUpscaleRequest(props *ImageUpscaleProps, hook globals.Hook) error
}

// 新增：画布扩展接口
type ImageOutpaintFactory interface {
    CreateImageOutpaintRequest(props *ImageOutpaintProps, hook globals.Hook) error
}

// 新增：图生视频接口
type ImageToVideoFactory interface {
    CreateImageToVideoRequest(props *ImageToVideoProps, hook globals.Hook) error
}
```

#### 新增请求属性类型 (`adapter/common/types.go` 扩展)

```go
type ImageEditProps struct {
    RequestProps
    Model         string   `json:"model"`
    OriginalModel string   `json:"-"`
    Images        []string `json:"images"`       // base64 数组
    Prompt        string   `json:"prompt"`
    Strength      *float32 `json:"strength,omitempty"`
    User          string   `json:"-"`
}

type ImageToVideoProps struct {
    RequestProps
    Model         string   `json:"model"`
    OriginalModel string   `json:"-"`
    Images        []string `json:"images"`
    Prompt        string   `json:"prompt"`
    Duration      int      `json:"duration,omitempty"`
}
```

---

## 3. 多 AI 供应商配置方案

### 3.1 渠道类型注册

```go
// globals/constant.go 新增
const (
    DreaminaChannelType = "dreamina"   // 即梦云 API
    JimengChannelType   = "jimeng"     // 即梦 CLI
)

// adapter/adapter.go 新增映射
var imageProcessorFactories = map[string]ImageEditFactoryCreator{
    DreaminaChannelType: dreamina.NewImageProcessorFromConfig,
}
```

### 3.2 config.yaml 配置示例

```yaml
channel:
  # 现有 Chat 渠道保持不变...

  # 新增：即梦图片处理渠道
  - id: 10
    name: "即梦图片处理"
    type: "dreamina"
    models: ["dreamina-v2", "dreamina-v2-turbo", "dreamina-video"]
    endpoint: "https://dreamina-api.bytedance.com"
    secret: "ak-xxx|sk-xxx"
    priority: 1
    state: true
```

### 3.3 功能-渠道映射

每个功能在 prompts.json 中指定默认 `channel_type`，前端可运行时覆盖：

```json
{
  "features": {
    "white_bg": {
      "system_prompt": "product on pure white background...",
      "channel_type": "dreamina",
      "model": "dreamina-v2"
    },
    "detail_image": {
      "channel_type": "local"
    }
  }
}
```

---

## 4. 数据库表设计

### photo_images

```sql
CREATE TABLE photo_images (
    id          VARCHAR(12) PRIMARY KEY,
    user_id     BIGINT NOT NULL,
    filename    VARCHAR(255) NOT NULL,
    size        BIGINT NOT NULL,
    width       INT DEFAULT 0,
    height      INT DEFAULT 0,
    url         VARCHAR(512) NOT NULL,
    file_path   VARCHAR(512) NOT NULL,
    folder_name VARCHAR(255) DEFAULT '',
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_user (user_id),
    INDEX idx_folder (folder_name)
);
```

### photo_tasks

```sql
CREATE TABLE photo_tasks (
    task_id          VARCHAR(12) PRIMARY KEY,
    user_id          BIGINT NOT NULL,
    feature          VARCHAR(32) NOT NULL,
    status           VARCHAR(16) DEFAULT 'pending',
    image_ids        TEXT NOT NULL,
    result_urls      TEXT DEFAULT '[]',
    error_message    TEXT DEFAULT '',
    progress         INT DEFAULT 0,
    params           TEXT DEFAULT '{}',
    total_images     INT DEFAULT 0,
    processed_images INT DEFAULT 0,
    total_videos     INT DEFAULT 0,
    processed_videos INT DEFAULT 0,
    submit_ids       TEXT DEFAULT '[]',
    source_filenames TEXT DEFAULT '[]',
    source_paths     TEXT DEFAULT '[]',
    folder_name      VARCHAR(255) DEFAULT '',
    created_at       DATETIME DEFAULT CURRENT_TIMESTAMP,
    completed_at     DATETIME,
    INDEX idx_user (user_id),
    INDEX idx_status (status)
);
```

---

## 5. API 路由

```
POST   /api/addition/photo/upload              # 上传图片 (multipart, 多文件)
POST   /api/addition/photo/upload/folder       # 文件夹批量上传
GET    /api/addition/photo/images              # 图片列表 (query: limit, offset)
GET    /api/addition/photo/images/{id}         # 图片信息
DELETE /api/addition/photo/images/{id}         # 删除图片

POST   /api/addition/photo/process             # 统一处理入口
GET    /api/addition/photo/tasks               # 任务列表
GET    /api/addition/photo/tasks/{id}          # 任务状态
DELETE /api/addition/photo/tasks/{id}          # 删除任务
POST   /api/addition/photo/tasks/{id}/retry    # 智能重试

GET    /api/addition/photo/prompts             # 提示词配置
GET    /api/addition/photo/download/file       # 单文件下载 (query: url)
GET    /api/addition/photo/download/zip        # ZIP 批量下载 (query: urls)
```

---

## 6. 前端页面布局

```
┌────────────────────────────────────────────────────┐
│ NavBar (coai 现有)                      [用户菜单]  │
├──────────┬─────────────────┬───────────────────────┤
│ 上传面板  │   功能面板       │   任务表格             │
│ (320px)  │   (flex 1)      │   (flex 1)            │
│          │                 │                       │
│ 📤 拖拽  │ ⚪ 白底图 [✓]   │ Tabs: 当前处理|历史   │
│   上传   │ 🏞️ 场景图 [ ]   │ ┌──────┬────┬──────┐  │
│          │ 🧹 擦除   [✓]   │ │任务# │状态│ 进度 │  │
│ 📁 文件夹│ 🎨 换色   [ ]   │ ├──────┼────┼──────┤  │
│   上传   │ ...             │ │abc123│进行│ 45%  │  │
│          │ [开始处理] 按钮  │ │def456│完成│ 100% │  │
│ [缩略图] │                 │ └──────┴────┴──────┘  │
│ [缩略图] │  参数 Modal      │ 展开行: 错误+结果预览  │
└──────────┴─────────────────┴───────────────────────┘
```

---

## 7. Phase 详细拆分

### Phase 1：常量定义 + 目录骨架

#### P1.1 — 注册渠道类型常量
- **文件**：`globals/constant.go`
- **内容**：新增 `DreaminaChannelType = "dreamina"`
- **验证**：`go build` 通过

#### P1.2 — 定义 FeatureType 枚举
- **文件**：`addition/photo/types.go` (新建)
- **内容**：
  - `FeatureType` 常量 (15个：white_bg/scene_gen/image_erase/color_change/marketing/image_translate/hd_upscale/model_image/material_change/instruction_gen/detail_image/logo_custom/production_flow/resize/video_gen)
  - `TaskStatus` 常量 (pending/processing/success/failed)
  - `ImageInfo` struct
  - `TaskInfo` struct (含所有字段：task_id/feature/status/image_ids/result_urls/error_message/progress/...)
  - `ProcessRequest` struct
  - `UploadResponse` struct
- **验证**：编译通过

#### P1.3 — 注册路由骨架
- **文件**：`addition/photo/router.go` (新建)
- **内容**：`func Register(app *gin.RouterGroup)` 注册全部路由（handler 暂为 stub 返回 501）
- **文件**：`addition/router.go` (修改)
- **内容**：添加 `photo.Register(app)` 调用
- **文件**：`main.go` (检查)
- **验证**：`POST /api/addition/photo/process` 返回 501

#### P1.4 — 数据库迁移
- **文件**：`connection/db_migration.go` (修改)
- **内容**：添加 `photo_images` 和 `photo_tasks` 建表 SQL
- **验证**：启动后表自动创建

#### P1.5 — 添加 prompts.json
- **文件**：`config/prompts.json` (新建)
- **内容**：从 photo2 `backend/config/prompts.json` 完整迁移，每个 feature 增加 `channel_type` 字段
- **验证**：JSON 格式正确，`go` 可读取

---

### Phase 2：提示词系统

#### P2.1 — 实现 prompts 加载器
- **文件**：`addition/photo/prompts.go` (新建)
- **内容**：
  - `loadPromptsConfig()` — 读取 `config/prompts.json` 到内存
  - `getSystemPrompt(feature, kwargs)` — 占位符替换（先 defaults 再 kwargs）
  - `getTemplates(feature)` — 返回模板列表
  - `getOptions(feature, key)` — 返回选项（colors/languages/sizes）
  - 使用 `sync.RWMutex` 保证并发安全，支持热重载

#### P2.2 — 实现 GET /prompts API
- **文件**：`addition/photo/handler.go` (新建或扩展)
- **内容**：
  - `GetPromptsAPI(c *gin.Context)` — 返回完整配置给前端
  - 已在 P1.3 注册路由，此处实现 handler
- **验证**：`curl /api/addition/photo/prompts` 返回完整 prompts.json 内容

---

### Phase 3：图片上传模块

#### P3.1 — 实现上传核心逻辑
- **文件**：`addition/photo/upload.go` (新建)
- **内容**：
  - `validateFileFormat(filename)` — 检查扩展名 (png/jpg/jpeg/webp/bmp/tiff)
  - `validateFileSize(size)` — 检查 ≤ 50MB
  - `generateFilename(original)` — UUID 重命名
  - `saveUploadFile(file, userID)` — 保存到 `storage/uploads/`，写数据库
  - `deleteImageFile(id)` — 删除文件 + 数据库记录

#### P3.2 — 实现上传 API
- **文件**：`addition/photo/handler.go`
- **内容**：
  - `UploadImagesAPI(c)` — 多文件上传，返回 `[]ImageInfo`
  - `UploadFolderAPI(c)` — 文件夹上传，复用 UploadImagesAPI
  - `ListImagesAPI(c)` — 按用户列出图片 (分页)
  - `GetImageAPI(c)` — 单图片信息
  - `DeleteImageAPI(c)` — 删除图片
- **验证**：curl 上传图片 → 文件出现在 `storage/uploads/` → 数据库有记录

#### P3.3 — 前端 UploadPanel 组件
- **文件**：`app/src/components/photo/UploadPanel.tsx` (新建)
- **内容**：
  - 拖拽区域 (HTML5 Drag & Drop + input[type=file])
  - 文件夹选择按钮 (`webkitdirectory`)
  - 缩略图网格 (选中高亮 + 删除按钮)
  - 全选/取消/清空 操作栏
  - Props: `images, selectedIds, uploading, onUpload, onUploadFolder, onToggleSelect, onSelectAll, onClearSelection, onRemove, onClearAll`
- **验证**：前端可上传图片，显示缩略图，可多选

---

### Phase 4：适配器层 — Dreamina

#### P4.1 — 扩展通用接口
- **文件**：`adapter/common/interface.go` (修改)
- **内容**：添加 `ImageEditFactory` / `ImageUpscaleFactory` / `ImageOutpaintFactory` / `ImageToVideoFactory` 接口
- **文件**：`adapter/common/types.go` (修改)
- **内容**：添加 `ImageEditProps` / `ImageUpscaleProps` / `ImageOutpaintProps` / `ImageToVideoProps` struct

#### P4.2 — 实现 Dreamina 适配器（数据结构）
- **文件**：`adapter/dreamina/types.go` (新建)
- **内容**：
  - `ImageRequest` struct (image2image 请求体)
  - `ImageResponse` struct (响应体)
  - `UpscaleRequest` / `UpscaleResponse`
  - `VideoRequest` / `VideoJob` (multimodal2video)
  - `SubmitResult` (submit_id, gen_status)

#### P4.3 — 实现 Dreamina 适配器（核心逻辑）
- **文件**：`adapter/dreamina/struct.go` (新建)
- **内容**：
  - `ImageProcessor` struct (含 endpoint, secret)
  - `NewImageProcessorFromConfig(conf)` — 工厂函数
  - `getEndpoint()` — 组装 API URL

#### P4.4 — 实现 Dreamina 图片处理
- **文件**：`adapter/dreamina/image.go` (新建)
- **内容**：
  - `CreateImageEditRequest(props, hook)` — 图生图/编辑
    - 构造请求体 (images base64 + prompt)
    - POST 到 `/v1/image2image`
    - 提交 → 轮询 gen_status → 下载结果
    - 重试机制（复用 coai 现有 `adapter/request.go` 的 `NewChatRequest` 模式）
  - `CreateImageUpscaleRequest(props, hook)` — 超清放大
    - POST 到 `/v1/image_upscale`
  - `CreateImageOutpaintRequest(props, hook)` — 画布扩展
    - 实际上是 image2image + expand prompt

#### P4.5 — 实现 Dreamina 视频生成
- **文件**：`adapter/dreamina/video.go` (新建)
- **内容**：
  - `CreateImageToVideoRequest(props, hook)` — multimodal2video
    - 1-9 张参考图上传
    - prompt 可选
    - 提交 → 轮询 (每 10s, 最大 900s) → 下载 .mp4
    - 进度回调 `hook({Content: progress})`
  - 参考 coai 现有 `adapter/openai/videos.go` 的轮询模式

#### P4.6 — 注册 Dreamina 到适配器工厂
- **文件**：`adapter/adapter.go` (修改)
- **内容**：添加 `imageProcessorFactories` 映射
  ```go
  var imageProcessorFactories = map[string]ImageEditFactoryCreator{
      globals.DreaminaChannelType: dreamina.NewImageProcessorFromConfig,
  }
  ```

#### P4.7 — 实现渠道调用入口
- **文件**：`adapter/request.go` (修改)
- **内容**：
  - `NewImageEditRequest(conf, props, hook)` — 类似 `NewChatRequest`，含重试
  - `NewImageToVideoRequest(conf, props, hook)` — 视频生成 + 重试
  - 复用现有错误处理和重试逻辑

#### P4.8 — 扩展 channel worker
- **文件**：`channel/worker.go` (修改)
- **内容**：
  - `NewImageEditRequestWithChannel(group, props, hook)` — 通过 channel ticker 获取渠道
  - 复用现有 `ConduitInstance.GetTicker()` 模式

---

### Phase 5：本地图像处理

#### P5.1 — 智能中心裁剪
- **文件**：`addition/photo/local.go` (新建)
- **内容**：
  - `smartCropCenter(inputPath, outputPath, targetW, targetH)` — 中心裁剪
    ```go
    import "github.com/disintegration/imaging"
    // 计算中心区域 → imaging.Crop() → 保存 PNG
    ```
  - 默认裁剪尺寸 800x800

#### P5.2 — Logo 叠加
- **文件**：`addition/photo/local.go` (同上)
- **内容**：
  - `compositeLogo(basePath, logoPath, outputPath, position)`
    - 缩放 logo 到基准图 1/4 宽度
    - 5 个位置：top-left/top-right/bottom-left/bottom-right/center
    - 使用 `imaging.Paste()` 叠加

#### P5.3 — 画布扩展（本地版）
- **文件**：`addition/photo/local.go` (同上)
- **内容**：
  - `resizeCanvas(inputPath, outputPath, targetRatio)` — 纯白背景扩展
    - 解析比例字符串 (1:1, 16:9, 4:3 等)
    - 新建画布 → 居中粘贴原图
  - 注：AI 版的 outpaint 效果更好，此仅为 fallback

#### P5.4 — 图片工具函数
- **文件**：`addition/photo/local.go` (同上)
- **内容**：
  - `resizeToMax(inputPath, maxSize)` — 限制最大尺寸 (默认 2048px)
  - `convertToPNG(inputPath)` — 格式转换
  - `getImageDimensions(path)` — 获取宽高

---

### Phase 6：处理核心

#### P6.1 — 功能处理函数骨架
- **文件**：`addition/photo/processor.go` (新建)
- **内容**：每个功能一个处理函数，统一签名：
  ```go
  type ProcessFunc func(
      ctx context.Context,
      taskID string,
      imagePaths []string,
      params map[string]interface{},
      channelOverride string,  // "" = 使用默认
      userID int64,
  ) (resultURLs []string, submitIDs []string, err error)
  ```

#### P6.2 — 实现 AI 依赖功能 (13个)
- **文件**：`addition/photo/processor.go`
- **内容**：
  - `processWhiteBg` — 读取图片 → base64 → dreamina image2image (白底 prompt) → 保存结果
  - `processSceneGen` — 读取图片 → 替换 prompt 占位符 → dreamina image2image
  - `processImageErase` — image2image + erase prompt
  - `processColorChange` — image2image + color prompt
  - `processMarketing` — image2image + selling_point prompt
  - `processImageTranslate` — image2image + lang prompt
  - `processHdUpscale` — dreamina upscale (2k)
  - `processModelImage` — image2image + model prompt (可选 model_image)
  - `processMaterialChange` — image2image + material prompt
  - `processInstructionGen` — image2image + user prompt
  - `processProductionFlow` — image2image + flow prompt
  - `processResize` — image2image + ratio prompt (每个比例一个结果)
  - `processVideoGen` — multimodal2video (1-9 ref images + prompt)

#### P6.3 — 实现本地功能 (2个)
- **文件**：`addition/photo/processor.go`
- **内容**：
  - `processDetailImage` — 调用 `smartCropCenter()`
  - `processLogoCustom` — 解析 logo_image_id → 获取路径 → `compositeLogo()`

#### P6.4 — 并行处理调度
- **文件**：`addition/photo/processor.go`
- **内容**：
  - `parallelProcess(items, processFn, maxWorkers=4, taskID, progressFn)`
    ```go
    func parallelProcess(
        items []string,
        fn func(string) (string, error),
        maxWorkers int,
        taskID string,
    ) []string {
        sem := make(chan struct{}, maxWorkers)
        results := make([]string, len(items))
        var wg sync.WaitGroup
        var completed int32
        for i, item := range items {
            wg.Add(1)
            go func(idx int, it string) {
                defer wg.Done()
                sem <- struct{}{}
                defer func() { <-sem }()
                res, err := fn(it)
                if err == nil { results[idx] = res }
                atomic.AddInt32(&completed, 1)
                updateTaskProgress(taskID, int(completed), len(items))
            }(i, item)
        }
        wg.Wait()
        return filterEmpty(results)
    }
    ```
  - `updateTaskProgress(taskID, completed, total)` — 更新数据库进度

#### P6.5 — 统一处理入口 Handler
- **文件**：`addition/photo/handler.go`
- **内容**：
  - `ProcessAPI(c *gin.Context)` — 核心入口
    1. 解析 `ProcessRequest` (image_ids, features[], params, channel_override)
    2. 校验图片存在
    3. 创建 Task (status=pending) → 返回 task_id
    4. `go func()` 异步执行：
       - 遍历 features，每个 feature 创建子任务
       - 调用对应 `processXxx()` 函数
       - 保存 submit_ids
       - 更新 task 状态 (success/failed)
       - 保存 result_urls

---

### Phase 7：任务管理系统

#### P7.1 — 任务 CRUD
- **文件**：`addition/photo/task.go` (新建)
- **内容**：
  - `createTask(feature, imageIDs, params, userID)` — INSERT
  - `getTask(taskID)` — SELECT
  - `listTasks(userID, limit)` — SELECT (按时间倒序)
  - `deleteTask(taskID)` — DELETE
  - `updateTaskStatus(taskID, status, resultURLs, errorMsg)` — UPDATE
  - `updateTaskProgress(taskID, completed, total)` — UPDATE progress
  - `saveSubmitIDs(taskID, ids)` — UPDATE submit_ids

#### P7.2 — 任务 API
- **文件**：`addition/photo/handler.go`
- **内容**：
  - `ListTasksAPI(c)` — 返回用户任务列表
  - `GetTaskAPI(c)` — 返回单个任务状态
  - `DeleteTaskAPI(c)` — 删除任务

#### P7.3 — 智能重试
- **文件**：`addition/photo/task.go`
- **内容**：
  - `retryTask(taskID)`
    1. 查询 task.submit_ids
    2. 如果有 submit_ids → 逐个检查 gen_status → 已完成的直接下载
    3. 如果没 submit_ids 或未完成 → 重新提交 AI 请求
    4. 优先从 task.source_paths 找源文件 → 其次按文件名搜索
  - `RetryTaskAPI(c)` — HTTP handler

#### P7.4 — 前端任务轮询 Hook
- **文件**：`app/src/hooks/usePhotoTask.ts` (新建)
- **内容**：
  - 智能轮询策略：
    - 图片任务：30 秒间隔
    - 视频任务：前 3 分钟不轮询，之后 30 秒
    - 超时：15 分钟
  - 页面加载时自动恢复进行中任务的轮询

#### P7.5 — 前端 TaskTable 组件
- **文件**：`app/src/components/photo/TaskTable.tsx` (新建)
- **内容**：
  - 双 Tab：当前处理 / 历史记录
  - 列：任务编号、功能、状态、进度条、图片进度、视频进度、时间、操作
  - 可展开行：错误信息 + 源文件名 + 结果预览 (图片/视频 + 下载按钮)
  - 操作按钮：重试 (失败任务)、手动刷新 (进行中任务)、删除

---

### Phase 8：下载功能

#### P8.1 — 单文件下载
- **文件**：`addition/photo/download.go` (新建)
- **内容**：
  - `DownloadFileAPI(c)` — query param `url` → 读取文件 → `c.File()` 或 `c.Data()` 带 `Content-Disposition: attachment`
  - MIME 识别：mp4→video/mp4, png→image/png, jpg→image/jpeg

#### P8.2 — ZIP 批量下载
- **文件**：`addition/photo/download.go` (同上)
- **内容**：
  - `DownloadZipAPI(c)` — query param `urls` (逗号分隔) → 打包成 ZIP → 流式返回
  - 使用 `archive/zip` 标准库

#### P8.3 — 前端下载对接
- **文件**：`app/src/api/photo.ts` (新建)
- **内容**：
  - `downloadFile(url)` — axios blob → URL.createObjectURL → 触发下载
  - `downloadZip(urls)` — 同上

---

### Phase 9：前端集成

#### P9.1 — 创建 API 层
- **文件**：`app/src/api/photo.ts` (新建)
- **内容**：
  - `uploadImages(files, folderName)` → POST multipart
  - `uploadFolder(files, folderName)` → POST multipart
  - `listImages()` → GET
  - `deleteImage(id)` → DELETE
  - `submitProcess(imageIds, features, params)` → POST
  - `getTask(taskId)` → GET
  - `listTasks()` → GET
  - `retryTask(taskId)` → POST
  - `getPrompts()` → GET

#### P9.2 — 创建 FeaturePanel 组件
- **文件**：`app/src/components/photo/FeaturePanel.tsx` (新建)
- **内容**：
  - 15 个功能按钮 (两排，7+8)，支持多选
  - 无参数功能 → 直接可加入处理队列
  - 有参数功能 → Dialog (Modal)：
    - 快捷模板 Tag (从 prompts.json 加载)
    - 文本输入 (prompt / selling_point)
    - 下拉选择 (color / language / size / position)
    - 参考图 ID 输入 (model_image / material_image / logo_image)
  - AI 供应商选择器 (Dropdown，来自可用渠道)
  - "开始处理" 按钮

#### P9.3 — 创建 Photo 主页面
- **文件**：`app/src/routes/Photo.tsx` (新建)
- **内容**：
  - 三栏布局 (使用 Tailwind Flexbox)
  - 集成 UploadPanel + FeaturePanel + TaskTable
  - 空状态提示
  - 使用 `usePhotoTask` Hook

#### P9.4 — 路由注册 + 导航入口
- **文件**：`app/src/router.tsx` (修改)
- **内容**：添加 `/photo` 路由 (需登录)
- **文件**：`app/src/components/app/MenuBar.tsx` (修改)
- **内容**：添加 "📸 图片处理" 菜单项

---

## 8. 实施顺序 & 依赖关系

```
Phase 1 (骨架) ──────────────────────────────────────────────────┐
    │                                                             │
    ├── Phase 2 (提示词系统) ─────────────────────────────────────┤
    │                                                             │
    ├── Phase 3 (图片上传) ──────┐                                │
    │                             ├── Phase 9.2 (FeaturePanel) ───┤
    ├── Phase 4 (Dreamina适配器) ─┤                                ├── Phase 9.3 (Photo页面)
    │                             ├── Phase 6 (处理核心) ─────────┤
    ├── Phase 5 (本地处理) ──────┘                                │
    │                                                             │
    ├── Phase 7 (任务管理) ───────────────────────────────────────┤
    │                                                             │
    └── Phase 8 (下载) ──────────────────────────────────────────┘

注：Phase 2-5 可并行开发，Phase 6 依赖 Phase 4+5，Phase 9 依赖 Phase 3+6+7+8
```

---

## 9. 第三方 Go 依赖

```go
// go.mod 新增
require (
    github.com/disintegration/imaging v1.6.2    // 图片缩放/裁剪/合成
    github.com/google/uuid v1.6.0               // UUID 生成
    golang.org/x/image v0.18.0                  // 官方扩展图片库
)
```

---

## 10. 验收标准

- [ ] P1: `go build` 通过，路由骨架可访问
- [ ] P2: `GET /prompts` 返回完整提示词配置，占位符替换正确
- [ ] P3: 拖拽/文件夹上传成功，文件存入 storage/uploads/，数据库有记录
- [ ] P4: Dreamina 适配器 image2image 调用成功，返回处理结果
- [ ] P5: 中心裁剪 800x800、Logo 叠加 5 位置均正确
- [ ] P6: 所有 15 功能可单独执行，并行处理 4 路同时进行
- [ ] P7: 任务创建→轮询→状态更新→重试全链路正常
- [ ] P8: 单文件下载 + ZIP 打包下载正常
- [ ] P9: 前端完整：上传→选功能→处理→轮询→结果展示 全流程可用
