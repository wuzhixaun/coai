# Photo（电商图片处理）开放 API

> 面向脚本 / Agent / ToB 集成。所有端点复用站点统一鉴权与计费，无需独立接入。

## 鉴权

所有端点走 `Authorization` 头，支持两种凭证：

- **API Key**（推荐用于程序/Agent）：`Authorization: Bearer sk-xxxxxxxx`
- Web 登录 token（前端会话）

API Key 在站点「账户/API」处创建。下文示例统一用 `sk-...`。

## 计费

- 出图按 **模型单价 × 张数** 从账户额度扣费，**仅成功产出计费，失败不扣**。
- 提交前会做额度预检，余额不足返回 `402`，不会空跑。
- 单价取自后台 charge 配置（与聊天/官方 API 出图同一套定价）。

## 基址

```
<host>/api/photo
```

---

## 1. 上传图片

```bash
curl -X POST <host>/api/photo/upload \
  -H "Authorization: Bearer sk-..." \
  -F "files=@product.jpg"
```

返回 `[{ id, filename, size, url, folder_name, created_at }]`。后续处理用返回的 `id`。

## 2. 贴链接抓图（直链或页面 og:image）

```bash
curl -X POST <host>/api/photo/fetch-url \
  -H "Authorization: Bearer sk-..." -H "Content-Type: application/json" \
  -d '{"url":"https://example.com/item/123"}'
```

返回单个 `{ id, url, ... }`。商品详情页若需登录态/JS 渲染可能抓不到，此时改用图片直链。

## 3. 列出 / 删除图片

```bash
curl <host>/api/photo/images -H "Authorization: Bearer sk-..."
curl -X DELETE <host>/api/photo/images/{id} -H "Authorization: Bearer sk-..."
```

## 4. 单/多功能处理

```bash
curl -X POST <host>/api/photo/process \
  -H "Authorization: Bearer sk-..." -H "Content-Type: application/json" \
  -d '{
    "image_ids": ["abc123"],
    "features": ["white_bg", "scene_gen"],
    "params": { "prompt": "简约棚拍场景" },
    "identity_id": "",        // 可选：应用商品/模特一致性身份
    "brand_kit_id": ""        // 可选：叠加品牌 Logo/主色
  }'
```

返回每个 feature 一个任务 `[{ task_id, feature, status, ... }]`。

**可用 features**：`white_bg, scene_gen, image_erase, color_change, marketing,
image_translate, hd_upscale, model_image, material_change, instruction_gen,
detail_image, logo_custom, production_flow, resize, material_extract,
product_extract, video_gen`。

部分 feature 的常用 `params`：
- `scene_gen / image_erase / model_image / instruction_gen`：`prompt`
- `color_change`：`target_color`；`marketing`：`selling_point`
- `image_translate`：`target_lang`；`resize`：`target_sizes`（如 `["1:1","4:5"]`）
- `material_extract / product_extract`：`category`；`video_gen`：`prompt`,`duration`
- 生成类支持 `image_count`（1–6）

## 5. 一键成套工作流（操作链编排）

列出预置模板：

```bash
curl <host>/api/photo/workflow/templates -H "Authorization: Bearer sk-..."
```

提交工作流（模板或自定义有序步骤，步骤间产物自动传递）：

```bash
curl -X POST <host>/api/photo/workflow \
  -H "Authorization: Bearer sk-..." -H "Content-Type: application/json" \
  -d '{
    "image_ids": ["abc123"],
    "template": "apparel_listing",
    "steps": [                         // 与 template 二选一；steps 即"配方"
      { "feature": "white_bg" },
      { "feature": "scene_gen", "params": { "prompt": "棚拍" } },
      { "feature": "marketing", "params": { "selling_point": "新品" } }
    ],
    "identity_id": "", "brand_kit_id": ""
  }'
```

返回一个聚合任务（`feature: "workflow"`），所有步骤产物汇总在该任务的 `result_urls`。

## 6. 任务

```bash
curl <host>/api/photo/tasks            -H "Authorization: Bearer sk-..."   # 列表
curl <host>/api/photo/tasks/{id}       -H "Authorization: Bearer sk-..."   # 轮询单个
curl -X POST   <host>/api/photo/tasks/{id}/retry  -H "Authorization: Bearer sk-..."  # 只重试失败项
curl -X DELETE <host>/api/photo/tasks/{id}        -H "Authorization: Bearer sk-..."
```

任务字段：`status(pending|processing|success|failed)`、`progress`、`result_urls`、
`item_status[]`（逐图状态：`{index, filename, status, urls, error}`）。
任一图失败 → 任务标 `failed` 但保留已成功产物（部分成功）。`retry` 只重跑失败的源图。

## 7. 一致性身份 / 品牌资产 / 配方（可选）

```bash
# 身份：type=product|model|brandkit；brandkit 用 ref_image_ids[0] 作 Logo、color 作主色
curl -X POST <host>/api/photo/identity -H "Authorization: Bearer sk-..." \
  -H "Content-Type: application/json" \
  -d '{"type":"product","name":"A款连衣裙","ref_image_ids":["abc"],"subject_prompt":"红色长裙"}'
curl <host>/api/photo/identity            -H "Authorization: Bearer sk-..."
curl -X DELETE <host>/api/photo/identity/{id} -H "Authorization: Bearer sk-..."

# 配方：保存命名工作流，后续以 steps 复跑
curl -X POST <host>/api/photo/recipe   -H "Authorization: Bearer sk-..." \
  -H "Content-Type: application/json" \
  -d '{"name":"上架三件套","steps":[{"feature":"white_bg"},{"feature":"scene_gen"}]}'
curl <host>/api/photo/recipe              -H "Authorization: Bearer sk-..."
curl -X DELETE <host>/api/photo/recipe/{id}   -H "Authorization: Bearer sk-..."
```

## 8. 下载

```bash
# 单文件（url 为任务 result_urls 中的项）
<host>/api/photo/download/file?url=/storage/results/xxx.png
# 打包，可选 prefix 自定义文件名前缀
<host>/api/photo/download/zip?urls=/storage/results/a.png,/storage/results/b.png&prefix=SKU001
```

## 典型 Agent 流程

1. `POST /upload` 或 `POST /fetch-url` 得到 `image_id`
2.（可选）`POST /identity` 建商品/模特身份
3. `POST /workflow`（带 `template` 或 `steps` + `identity_id`）提交成套出图
4. 轮询 `GET /tasks/{id}` 到 `success`
5. 读 `result_urls`，按需 `download/zip`

## 已知局限

- `fetch-url`：登录态/纯前端渲染的商品页抓不到主图，需直链。
- `video_gen`：仅图生视频（5s/10s），**暂不含数字人/口播**（底座未提供）。
- **暂不写 C2PA 内容凭证**（需引入 C2PA 签名库与证书，后续）。
- 重试暂不重新套用 identity（任务未持久化 identity_id）。
