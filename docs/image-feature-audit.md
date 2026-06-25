# 图片处理功能审计与改进计划

> 交付物：本文档批准后将写入仓库 `docs/image-feature-audit.md`，作为团队协作与后续跟踪的依据。
> 范围：**全部图片链路**（Photo 页 + 聊天内文生图 + `/v1/images` API + Midjourney/DALLE + 即梦官方 API + 管理后台/计费/数据库/存储）。
> 排序原则：**先稳后美**（生产健壮性 → 链路与计费正确性 → 可观测 → UI/UX → i18n → 后台/配置 → 多渠道补齐）。

---

## Context / 背景

CoAI.Dev 已经接入了**即梦官方 API（`adapter/jimengapi`）**作为当前主力图片能力，并新建了独立的 **Photo 图片处理页**（`/photo`，电商场景：白底图/场景图/擦除/换色/高清/视频等 15 个功能）。核心业务流程（上传→选功能→生成→看结果）已跑通，即梦官方 API 的文生图/图生图/超清/扩图/视频均已通过 live smoke test。

但通过对前后端、配置、管理后台、数据库的系统审计，发现图片链路在**生产健壮性、计费正确性、可观测性、前端体验、国际化**等方面存在系统性缺口，直接影响上线后的稳定性、可运营性和用户体验。本计划把这些缺口拆成可独立执行、可验证的 phase。

> 现有进度文档 `findings.md` / `progress.md` / `task_plan.md` 已记录即梦集成历程，本文档不重复，只聚焦「还差什么」。

---

## 一、现状总览（能力矩阵）

| 链路 | 适配器/文件 | 状态 | 备注 |
|---|---|---|---|
| 即梦官方 API（文生图/图生图/超清/扩图/inpaint/提取/视频） | `adapter/jimengapi/*` | ✅ 较完整 | 已 live 验证；签名/轮询/退避健全 |
| 即梦 CLI | `adapter/jimeng/*` | ⚠️ 淘汰中 | 子进程调用，仅做 video 后备；无 context 取消、临时文件用 `/tmp` |
| Midjourney（imagine/U/V/R） | `adapter/midjourney/*` | ⚠️ 有缺陷 | 固定 30min 超时、依赖 webhook、无重试、Redis TTL 固定 60min |
| OpenAI DALLE / GPT-Image | `adapter/openai/image.go` | ⚠️ 最小实现 | 硬编码 `n=1`、尺寸写死、无异步、安全过滤报错混淆 |
| DashScope（通义万相）文生图 | `adapter/dashscope/*` | ❌ 未实现 | 仅文本对话 |
| Photo 页前端 | `app/src/routes/Photo.tsx`、`components/photo/*` | ⚠️ 功能通但体验粗糙 | 无暗色/无移动端/文案硬编码/无大图预览/无取消 |

---

## 二、问题清单（按领域，标严重程度 🔴高 / 🟠中 / 🟢低）

### A. 生产健壮性（最优先止血）
- 🔴 **本地存储无清理、无配额**：`storage/results` 永久增长，无磁盘检查/大小限制/TTL，生产会磁盘爆满。路径硬编码 `adapter/jimengapi/image.go:20`（`resultDir = "storage/results"`）。
- 🔴 **24h 临时 URL 失效无感知**：即梦返回的是 24h 临时 URL，下载到本地后前端/历史仍可能引用到原始临时链接；缺统一「本地化 URL + 长期可访问」保证，历史会话图片会 broken（`findings.md:45`）。
- 🔴 **无单用户并发/任务队列控制**：同一用户可并发提交大量生成任务（最坏 6×6×N），易自我 DDoS（`channel/worker.go:181`、`middleware/throttle.go` 无图片专用限流）。
- 🟠 **超时策略各自为政、不可配**：即梦 10min、Midjourney 30min、CLI 15min，用户/管理员均不可配。
- 🟠 **CLI / Midjourney 轮询无退避**：固定 10s / webhook 依赖，网络抖动即卡死或浪费配额。
- 🟢 **文件名碰撞风险**：MD5 前 10 位 + 时间戳，高并发下理论可重名（`adapter/jimengapi/image.go:101`）。

### B. 后端架构与链路一致性
- 🔴 **缺统一的图片模型识别**：仅有 `IsJimengImageGenerationModel`（`globals/variables.go`），只覆盖文生图；无统一 `IsImageGenerationModel()` / `GetImageModelCapability()`，导致链路判断散落。
- 🔴 **`/v1/images/generations` 与聊天链路对即梦不友好**：仍走 chat buffer 提 Markdown 图片（`manager/images.go:209-298`），未按模型类型选「图片工厂 vs 聊天工厂」；OpenAI 兼容参数（scale/seed/n/size）支持不全。
- 🔴 **聊天内选图片模型不自动切换图片流程**：`manager/chat.go` 缺 `IsImageGenerationModel` 分支。
- 🟠 **DALLE 不支持批量**：硬编码 `n=1`、尺寸写死（`adapter/openai/image.go:29-34`），与 OpenAI spec 不符。
- 🟠 **即梦 4.6/4.0 的 scale 类型差异（int vs float）需手工处理**（`adapter/jimengapi/validation.go`）。
- 🟢 **`adapter/adapter.go` 残留 dreamina/旧注释**，无迁移说明。

### C. 计费、数据库与可观测
- 🔴 **不支持「按张计费」**：计费仅 `token/times/non-billing`，即梦官方按张收费（约 0.22 元/图）无法表达；批量生成是否按 N 倍扣费逻辑不清（`manager/images.go:241,286`、`app/src/admin/charge.ts`）。当前 config 里即梦全部 `non-billing`，运营即亏损。
- 🔴 **缺图片生成历史/任务主表**：聊天与 `/v1/images` 的图片任务无独立记录；火山 `task_id` 塞在 `conversation.task_id`（VARCHAR255、无索引），元数据丢失。`record` 表的 `input_tokens/output_tokens` 对图片无意义，缺 `image_count` 等字段（`connection/database.go:324-352`）。
- 🟠 **缺火山 API 错误追踪字段**（`request_id`/`code`/`message`），售后排查困难。
- 🟠 **管理后台无图片用量统计**：`Record.tsx` 无法按「张数」查看图片生成记录。
- 注：Photo 页已有 `photo_images` / `photo_tasks` 两表（`connection/database.go:379-430`），结构尚可，但与「聊天/API 图片任务」割裂。

### D. 前端 UI/UX（Photo 页 `components/photo/*`，已核实）
- 🔴 **完全无暗色模式适配**：大量硬编码 `bg-white` / `bg-gray-50` / `bg-gray-200` / `border-gray-300` / `text-gray-500`（`Photo.tsx:18,28,36`、`TaskTable.tsx:53,59,...`、`UploadPanel.tsx`），暗色下背景刺眼、对比失效。应换语义色 `bg-background`/`border-border`/`text-muted-foreground`。
- 🔴 **无移动端响应式**：三栏固定 `w-80 + flex-1 + flex-1`（`Photo.tsx:16-44`），`<1024px` 不可用；应在窄屏改 Tab/堆叠。
- 🔴 **结果无大图预览**：缩略图 `w-40 h-24` 无点击放大弹窗（对比 `components/markdown/Image.tsx` 已有 Dialog/复制/打开）。
- 🟠 **进行中任务无法取消**：`TaskTable.tsx` 只有刷新，无停止/取消（`usePhotoTask.ts`）。
- 🟠 **无批量下载**：`api/photo.ts` 已有 `getDownloadZipUrl()` 但 UI 未用。
- 🟠 **上传失败无 toast**：`usePhotoTask.ts:59` 仅 `console.error`，用户无感知；无单文件上传进度。
- 🟠 **无粘贴上传**：对比 `FileProvider.tsx` 已支持剪贴板。
- 🟢 缩略图过小（`h-20`）、拖拽反馈弱、无「重新生成/收藏/分享」、任务列表无筛选搜索。

### E. 管理后台与配置
- 🔴 **缺图片存储配置项**：无 `storage`/`blob`/云存储（S3/R2/MinIO）配置，全写死本地。
- 🔴 **缺并发限制配置**：无法表达官方 1–2 并发。
- 🟠 **`ChannelEditor.tsx` 无 jimeng-api 专属帮助**：`AK|SK`（竖线分隔）格式无前端校验/提示。
- 🟠 **`config.example.yaml` 未同步即梦/存储示例**，新部署者无模板。
- 🟢 **模型市场无计费/尺寸/文档链接展示**（`routes/admin/Market.tsx`）。

### F. 国际化
- 🔴 **Photo 页约 95% 文案硬编码中文**：`FeaturePanel.tsx`/`UploadPanel.tsx`/`TaskTable.tsx` 的功能名、按钮、状态、错误、空状态全写死，`cn.json`/`en.json` 缺失对应 key，无法切英文。

---

## 三、分阶段执行计划（先稳后美）

> 每个 phase 可独立交付与验证。标注 `[后端]`/`[前端]`/`[DB]`/`[配置]`。建议顺序执行，Phase 1–3 为「稳」，Phase 4–7 为「美/全」。

### Phase 1 — 生产健壮性止血 🔴
**目标**：上线不爆盘、不失效、不被自己打挂。
1. `[配置][后端]` 抽出存储配置：新增 `storage.result_dir` / `storage.max_size` / `storage.ttl_hours`，替换 `adapter/jimengapi/image.go:20` 的硬编码；`config.yaml`/`config.example.yaml` 同步示例。
2. `[后端]` 实现结果文件清理：后台定时任务按 TTL/总容量清理 `storage/results`、`storage/uploads`（参考现有定时任务/启动钩子位置）。
3. `[后端]` 统一「下载即梦 24h URL → 本地长期 URL」并确保前端/历史只存本地 URL，杜绝引用临时链接（核对 `image.go` 存储后返回的是本地路径）。
4. `[后端]` 单用户并发限制：在 `manager/images.go`/`channel/worker.go` 入口加「每用户在途图片任务数」上限（可配），超限排队或拒绝。
5. `[后端]` 文件名加随机后缀/UUID，消除碰撞（`image.go:101`）。

**验收**：磁盘占用稳定（清理任务可见日志）；历史会话图片 24h 后仍可访问；并发压测同一用户被限流；生成文件名唯一。

### Phase 2 — 后端链路统一与计费正确性 🔴
**目标**：聊天/API/Photo 三条入口走同一套「图片模型」判定与计费。
1. `[后端]` 新增 `globals` 统一函数：`IsImageGenerationModel(model)`、`GetImageModelCapability(model)`，替换散落的即梦判断。
2. `[后端]` `manager/chat.go` 增图片模型分支：聊天中选图片模型自动走图片生成流程。
3. `[后端]` `manager/images.go` 按模型类型路由到「图片工厂 vs 聊天工厂」，补齐 OpenAI 兼容参数（n/size/scale/seed）。
4. `[计费]` 新增「按张计费」：扩展 `charge` 类型（如 `times-billing` 支持按 `image_count` 倍率，或新增 `image-billing`），`CollectQuota` 按实际生成张数扣费；`app/src/admin/charge.ts` 同步。
5. `[后端]` DALLE 支持 `n` 与尺寸参数，安全过滤返回明确 safety error（`adapter/openai/image.go`）。

**验收**：聊天内选即梦模型直接出图；`/v1/images` 用即梦工厂返回多图；生成 3 张按 3 张扣费（账单核对）；DALLE 多图与尺寸生效。

### Phase 3 — 数据库与可观测 🟠 ✅ 已完成
**目标**：图片任务可记录、可统计、可排查。
1. `[DB]` ✅ 新增 `image_generation` 观测表（`connection/database.go` `CreateImageGenerationTable`）：记录 `user_id/username/source/model/channel/channel_name/image_count/quota/duration/status/task_id/request_id/code/message/created_at`，建 user/model/status/source/created_at 索引。新表用 `CREATE TABLE IF NOT EXISTS`（fresh 与既有库均幂等），无需 ALTER 迁移。
2. `[后端]` ✅ 三入口成功/失败均落库：`manager/images.go`（API：DALLE/聊天工厂 + 即梦工厂两路径）、`manager/chat.go`（聊天内图片模型分支，含错误早退路径）、`addition/photo/processor.go`（Photo 流水线完成处）。统一经 `manager.RecordImageOutcome` / `recordImageGeneration`，火山 `code/request_id` 由 `parseJimengError` 从错误文本提取（TDD：`manager/image_record_test.go`）。
3. `[前端]` ✅ 管理后台 `Record.tsx` 增「图片用量」Tab：今日/本月出图张数、图片消费、成功/失败数、本月 Top 模型；列表支持按来源(chat/api/photo)与状态(success/failed)筛选，展示张数/计费/状态/`request_id`/错误信息。新增后端 `/admin/image-record/list`、`/admin/image-record/stats`（`admin/image_record.go`）+ 中英 i18n。

**验收**：每次生成可在后台查到记录与张数；失败带火山 `request_id` 可排查。

> 说明：观测落库为「尽力而为」（DB 失败仅告警，不阻断出图）；图片专用元数据与通用 `record` 表解耦，避免污染 token 语义。Photo 入口的 `quota` 暂记 0（Photo 计费在后续阶段细化），张数与成败已可观测。

### Phase 4 — 前端体验打磨：暗色 + 移动端 + 预览 🔴 ✅ 已完成
**目标**：Photo 页在暗色/移动端可用，结果可放大。
1. `[前端]` ✅ 硬编码颜色全量替换为语义色：`Photo.tsx`、`UploadPanel.tsx`、`TaskTable.tsx`（`bg-white→bg-card`、`bg-gray-50→bg-muted/20`、`bg-gray-200→bg-muted`、`text-gray-*→text-muted-foreground`、`border-gray-300→border-input`、`text-red-500→text-destructive`、`bg-red-500 text-white→bg-destructive text-destructive-foreground`、`text-white→text-primary-foreground`）。`FeaturePanel.tsx` 原已基本语义化（仅保留成功态绿色）。
2. `[前端]` ✅ 响应式：`Photo.tsx` 由固定三栏改为 `flex-col lg:flex-row`；`<lg` 顶部出现「图片/功能/任务」Tab 栏（带数量角标），单栏全宽切换；`lg+` 恢复 `w-80 + flex-1 + flex-1` 三栏。
3. `[前端]` ✅ 结果大图预览弹窗：`TaskTable.tsx` 新增 `ResultPreview`（复用 `Dialog` + `useClipboard` + `openWindow`），缩略图 hover 显示预览图标，点击弹大图/视频，支持复制链接 / 新窗口打开 / 下载；缩略图由 `h-24` 增至 `h-28`。
4. `[前端]` ✅ 进行中任务「取消」按钮：后端无 cancel 接口，按计划用「前端停轮询 + 删除任务记录」(`deleteAction` → `DELETE /photo/tasks/:id` + `stopPolling`)；active 任务显示「取消」，history 任务显示「删除」。
5. `[前端]` ✅ 批量下载：结果数 >1 时展开区出现「打包下载」按钮，接入已有 `getDownloadZipUrl()`。

**验收**：暗色模式无白底刺眼；手机宽度可正常上传/生成/看结果；点图弹大图；可取消、可打包下载。

> 说明：i18n（硬编码中文抽取）属 Phase 6，本阶段保留中文文案不动。前端 `tsc --noEmit` 通过。

### Phase 5 — 上传体验与反馈 🟠 ✅ 已完成
1. `[前端]` ✅ 上传失败 `toast.error` + 成功 `toast.success`（`usePhotoTask.ts`）；上传进度：`api/photo.ts` 的 `uploadImages` 由 `fetch` 改 `XMLHttpRequest`（保留字面量 URL 与鉴权头不变）以拿到 `upload.onprogress`，`UploadPanel` 顶部显示 `Progress` 进度条与百分比。
2. `[前端]` ✅ 全局 `Ctrl+V` 粘贴上传：`UploadPanel` 挂 window `paste` 监听，仅对 `kind==='file'` 的图片生效（纯文本粘贴到输入框不受影响）。
3. `[前端]` ✅ 拖拽增强（拖入时 `ring-2 + scale + 文案/图标变色「松开即可上传」`）、缩略图 `h-20→h-24`、上传中显示 `Skeleton` 占位 + 校验失败（格式/超 50MB）逐项 `toast` 提示。

**验收**：上传失败有可见提示；可 Ctrl+V 粘贴图；拖拽反馈明显。

> 说明：批量上传用单请求 FormData，进度为整批百分比（非逐文件）；前端 `tsc --noEmit` 与 `vite build` 均通过。

### Phase 6 — 国际化 🔴（与 Phase 4/5 可并行）✅ 已完成
1. `[前端]` ✅ Photo 页全部可见文案抽到 `resources/i18n/{cn,en}.json` 新增 `photo.*` 命名空间（`tabs`/`upload`/`feature`/`task`/`features`/`status`/`errors`）：`Photo.tsx`、`UploadPanel.tsx`、`FeaturePanel.tsx`（含 17 功能名、对话框全部 Label/占位/按钮）、`TaskTable.tsx`、`usePhotoTask.ts`（toast）全部改 `t()`。`FEATURES`/`STATUS` 等 map 去掉硬编码 label，仅留 key/图标，运行时 `t('photo.features.<key>')` 取值。
2. `[前端]` ✅ `friendlyError(raw, t)` 改为返回 `photo.errors.*` i18n key 文案（检测用的中文/英文 `includes` 模式保留——它们匹配后端报错，非显示文本）。

**验收**：切换语言后 Photo 页全英文/全中文无残留硬编码。

> 说明：构建时 i18n 插件已据 cn 自动为 ja/ru/tw 补全 `photo.*`（en 由人工撰写保留）。残留源码 CJK 仅两类非显示用途：① FeaturePanel `category` 默认值（发往后端的参数值）② friendlyError 的后端错误匹配模式。`tsc --noEmit` 与 `vite build` 均通过。

### Phase 7 — 管理后台/配置 + 多渠道补齐 🟠🟢 ✅ 核心完成（7.3/7.4 明确延后）
1. `[配置][前端]` ✅ `ChannelEditor.tsx` 为 jimeng-api 增 `AK|SK` **软校验**：`akSkInvalidLines()` 逐行校验「恰一个竖线、两侧非空」，非法行用 `text-destructive` 内联提示并报行号，合法时显示格式帮助文案（不阻断保存，仅引导）；中英 i18n（`admin.channels.jimeng-aksk-hint/-invalid`）。帮助文本（`channel.ts` 的 jimeng-api `description`/`format=<access-key>|<secret-key>`）原已完整。`config.example.yaml` 补即梦渠道说明（AK|SK、后台维护、转存逻辑）。
2. `[配置]` ✅ 并发（P1 已做 `image.max_concurrent_tasks_per_user`）+ 超时可配：新增 `image.task_timeout_minutes` / `image.poll_interval_seconds` → `globals.ImageTaskTimeout()/ImagePollInterval()`（<=0 回退默认），替换即梦 `image.go`/`process.go` 硬编码的 `10min/10s`；`config.example` 同步注释。
3. `[前端]` ⏸ **延后**：模型市场展示计费/尺寸/文档链接需给 Model schema 跨栈加字段（后端结构体 + DB + API + 前端 reducer），属 🟢 最低优先且高改动面，无集成测试支撑下不做。
4. `[后端]` ⏸ **延后**：Midjourney 健壮化 / DALLE 异步 / DashScope 通义万相接入——体量大、依赖各渠道实环境联调，单独立项更稳妥。

**验收**：后台填错 AK|SK 有提示 ✅；example 可直接照抄部署 ✅；超时/并发可配 ✅。（MJ 健壮化随 7.4 延后）

> 说明：本阶段聚焦低风险高价值的配置/校验项；`go build`/`go vet`、前端 `tsc`/`vite build` 均通过。7.3/7.4 延后理由与前序阶段一致（无集成测试的大改动不在「先稳」窗口内强推）。

---

## 四、验证方式（端到端）

- **后端**：`go build ./...` 通过；对即梦/DALLE 跑现有 `adapter/jimengapi/live_smoke_test.go` 风格的冒烟；用 `curl` 打 `/v1/images/generations` 验证多图 + 计费扣减。
- **存储/清理**：本地塞满 `storage/results` 后触发清理任务，观察日志与磁盘回收；隔 24h（或调短 TTL）验证历史图片仍可访问。
- **计费**：生成 N 张前后对比用户额度，确认按张扣费；后台 Record 出现对应记录。
- **前端**：`cd app && pnpm build`；浏览器在亮/暗两种主题、桌面/移动两种宽度下走完「上传→选功能→生成→大图预览→下载→取消」；切换中英文确认无硬编码残留。
- **回归**：确认聊天内文生图、`/v1/images` API、Photo 页三条入口都能出图且互不影响。

---

## 五、落地动作（批准后执行）

1. 将本文档写入 `docs/image-feature-audit.md`。
2. 按 Phase 1 → 7 顺序实施；每个 phase 独立提交、独立验证。
3. （可选）每个 phase 完成后在文档勾选状态，作为团队跟踪看板。
