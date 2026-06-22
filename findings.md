# Findings and Decisions

## 2026-06-22 Jimeng API Phase 1 Implementation Findings

- Volcengine console provides two credential families on the same key-management page. Access Key is the AK/SK pair used for signed requests and is the correct credential for the official Jimeng Visual API. API Key is a Bearer-token style product/data-plane credential and is not used by the current `jimeng-api` adapter.
- The local project runtime config path is `config/config.yaml`, not the root `config.yaml`. This file is gitignored, so writing AK/SK there does not add the secret to git status.
- The Jimeng official channel was configured in `config/config.yaml` as `type=jimeng-api`, model `jimeng-seedream-4.6`, endpoint `https://visual.volcengineapi.com`, secret format `AK|SK`, with a `non-billing` charge rule so non-admin authenticated users do not hit the "price of the model is not set" guard.
- The official Jimeng 4.6 image API does not expose an OpenAI-style `n` parameter. It may infer multiple outputs from prompt intent, but the deterministic and cost-safe CoAI implementation should create N images by submitting N separate `force_single=true` tasks.
- For phase 1, `jimeng-seedream-4.6` maps to `req_key=jimeng_seedream46_cvtob`, endpoint `https://visual.volcengineapi.com`, `Region=cn-north-1`, and `Service=cv`.
- `/v1/images/generations` supports Jimeng `n` by looping through the common image-generation channel path and returning every generated image in `data[]`.
- Conversation use is now backend-capable when the selected model is a Jimeng image-generation model: the chat manager bypasses normal text chat and calls the image-generation channel path, returning Markdown image output. The current chat UI still has no dedicated image-count option, so chat defaults to one image.
- The Photo page now has a global `生成数量` option that submits `image_count` to the backend. The backend applies it only to generative AI features; deterministic operations such as HD upscale, resize, video, and local processing are not repeated.
- Dirty-tree cleanup safely removed `.DS_Store` files and ignored future `.DS_Store` / `chat.bak*` noise. The existing `chat.bak.*` executable backups were not deleted.
- Full Go tests now pass after aligning the pre-existing Photo prompt test to the current `jimeng` prompt configuration. The only remaining build noise is a non-fatal third-party `github.com/chai2010/webp` C warning.

## 2026-06-22 Jimeng API Phase 2 Findings

- `jimeng-seedream-4.0` is now registered beside `jimeng-seedream-4.6` across backend model constants, admin channel metadata, and the local runtime Jimeng API channel configuration.
- The Jimeng API registry now carries per-model request semantics instead of assuming all Seedream models share the same request schema.
- Current registry entries:
  - `jimeng-seedream-4.6` → `req_key=jimeng_seedream46_cvtob`, max 14 input images, prompt limit 800 runes, output count cap 6, scale integer `[1,100]`, default `50`.
  - `jimeng-seedream-4.0` → `req_key=jimeng_t2i_v40`, max 10 input images, prompt limit 800 runes, output count cap 6, scale float `[0,1]`, default `0.5`.
- Request building now validates locally before submitting to Volcengine: prompt, masks, `n`, image URL count/format, JPEG/PNG URL extensions when present, data URL rejection, `size` area, paired `width/height`, and aspect ratio.
- `/v1/images/generations` now accepts Jimeng-specific controls through the OpenAI-compatible route: `size` can be `WIDTHxHEIGHT` or an integer area, while `width`, `height`, `scale`, `min_ratio`, `max_ratio`, `force_single`, `images`, and `image_urls` are passed through to the official adapter.
- Mask inputs are intentionally rejected for the current generation models. Inpainting/removal should use a separate capability builder because Jimeng inpainting has a different two-image source/mask contract.
- Chat still defaults Jimeng image generation to one image. Multiple-image selection is available in `/v1/images/generations` via `n` and in the Photo page via the existing `生成数量` option; chat needs a separate UI/settings pass if image count should be user-selectable there.
- A live Jimeng smoke test was not run during phase 2 to avoid consuming paid Volcengine quota without explicit approval.

## 2026-06-22 Jimeng API Phase 3 Findings

- Before Phase 3, the official `jimeng-api` adapter implemented only `ImageGenerationFactory`. The Photo page's edit/upscale/outpaint features call `ImageEditFactory`/`ImageUpscaleFactory`/`ImageOutpaintFactory`, which were registered only for the CLI `jimeng` and custom `dreamina` adapters — so Photo editing never used the official Volcengine API.
- The single `ImageGenerator` struct now also implements the edit/upscale/outpaint interfaces. `NewImageProcessorFromConfig` returns it as an `ImageEditFactory`, and the adapter layer type-asserts it to the upscale/outpaint interfaces, mirroring how `dreamina` is consumed.
- Image edit reuses the seedream 4.6/4.0 generate specs (4.x is a unified generate/edit model); editing is just generation with input images plus `force_single=true`.
- Input images are classified per entry: `http(s)` URLs go to `image_urls`, everything else is treated as base64 and sent in `binary_data_base64` (with any `data:` URI prefix stripped). This lets Photo local files and `/v1` URL inputs share one path. The exact `binary_data_base64` field name for seedream edit is assumed from the standard Volcengine visual contract and still needs one live confirmation.
- Super-resolution (`jimeng-superres` → `jimeng_i2i_seed3_tilesr_cvtob`) normalizes Photo's 2k/4k/8k to the official 4k (default) / 8k and sends detail `scale=50`.
- Outpaint (`jimeng-outpaint` → `jimeng_img2img_seed3_painting_edit`) decodes the input image dimensions and derives expand-only directional fractions in `[0,1]` from the requested target ratio; a target ratio equal to the source is rejected with a clear error instead of submitting a no-op.
- Edit/upscale/outpaint emit results as `utils.GetImageMarkdown(storedURL)` to match the dreamina convention the Photo processor already stores.
- `config/prompts.json` was deliberately left pointing at the CLI `jimeng-v2`. Routing Photo through the official API is a one-line-per-feature config change the user makes after verifying prompts and running a live call.
- Verification stayed local (Go vet/test/build + frontend tsc all pass); no live API call was made to avoid consuming paid Volcengine quota.

## 2026-06-22 Jimeng API Phase 3 Live Smoke Test + prompts.json Switch

- Switched `config/prompts.json` to the official models (backed up to `config/prompts.json.bak.before-jimeng-api`): edit-class features (white_bg, scene_gen, image_erase, color_change, marketing, image_translate, model_image, material_change, instruction_gen, production_flow) → `jimeng-seedream-4.6`; `hd_upscale` → `jimeng-superres`; `resize` → `jimeng-outpaint`; all with `channel_type: jimeng-api`. `video_gen` stays on CLI `jimeng-video`; `detail_image`/`logo_custom` stay `local`.
- Live smoke test (guarded test `TestLiveSmoke`, `JIMENG_LIVE=1`, AK/SK from env) ran against the real `https://visual.volcengineapi.com`:
  - **Text-to-image PASS** — signing + submit + poll + result + 24h-URL download to `storage/results` all worked.
  - **Image edit PASS** — confirmed `binary_data_base64` IS the correct input field for seedream edit. A first attempt with an 8×8 image returned `code=50207 "binary data width or height too small"` (the API decoded our base64, only rejecting the tiny size); a 512×512 input then succeeded.
- Bug found and fixed via the live run: Volcengine TOS image URLs end in `.image` (template suffix), so `storeImageURL` was saving results as `*.image`, which browsers won't render. `storeImageURL` now keeps only known image extensions (`.png/.jpg/.jpeg/.webp/.gif`) and falls back to `.png`. Verified live: the edit result saved as `.png`.
- Updated the `addition/photo` `TestGetChannelType` expectation from `jimeng` to `jimeng-api` for `white_bg` to match the switched config.
- Cost note: the smoke test consumed a small number of paid generations (one text-to-image + one edit succeeded; one edit attempt failed pre-charge on size validation).
- The guarded `live_smoke_test.go` is kept for future re-validation; it skips unless `JIMENG_LIVE=1`.

## Requirements
- Analyze the current project carefully.
- Summarize project structure and implemented functionality.
- Produce a Markdown document useful for later bug fixing and secondary development.

## Research Findings
- Project root contains Go backend files (`go.mod`, `main.go`) and multiple domain directories such as `adapter`, `admin`, `channel`, `db`, `manager`, `middleware`, and `utils`.
- The `app/` directory is a separate frontend package with `package.json` and `pnpm-lock.yaml`.
- Root includes deployment/runtime assets such as `Dockerfile`, `docker-compose*.yaml`, `nginx.conf`, `config.example.yaml`, and `config.yaml`.
- README identifies the product as CoAI.Dev, an AIGC commercial solution combining chat UI, model/channel distribution, OpenAI-compatible API relay, billing/subscription, admin dashboard, file parsing, web search, image generation, and PWA support.
- Backend stack: Go 1.20, Gin, Redis, MySQL, Viper, JWT, Gorilla WebSocket, Logrus, file/document helper libraries, and multiple model provider SDKs/adapters.
- Frontend stack: React 18, Vite, TypeScript, Redux Toolkit, Radix UI, Tailwind CSS, Tremor, i18next, Markdown/Mermaid/KaTeX rendering, PWA tooling, and optional Tauri desktop support.
- `main.go` bootstraps config, admin singleton, channel manager, optional CLI commands, Gin engine/middleware, static routes, API routers, CORS origin parsing, and the configured server port.
- API routes are grouped under `/api` when `serve_static=true`; in separated frontend/backend mode they register at the root path.
- Runtime dependencies from config include MySQL, Redis, JWT `secret`, `serve_static`, server port, mail settings, backend URL, and search endpoint/query count.
- Backend domains visible from file layout:
  - `auth`: login/register/account/quota/API key/payment/redeem/subscription/user-facing auth services.
  - `admin`: dashboard/admin APIs for users, stats, market, records, payment, redeem, invitation, logs, and settings.
  - `manager`: chat, completions, images, videos, relay, usage, connection, broadcast, and conversation persistence/share/mask APIs.
  - `channel`: provider/channel configuration, load balancing, billing rules, plans, system model settings, and channel health workers.
  - `adapter`: concrete upstream model/provider adapters for OpenAI, Azure, Claude, Gemini/PaLM, Midjourney, SparkDesk, ZhipuAI, Hunyuan, DashScope, Dify, Coze, etc.
  - `addition`: auxiliary deprecated/extra generation modules, article generation, web search, and card endpoints.
- Frontend domains visible from file layout:
  - `routes`: page-level views such as Home, Auth, Admin, Wallet, Model, Generation, Article, Sharing, Account, and NotFound.
  - `api`: typed request wrappers for auth, v1, quota, generation, sharing, file, record, broadcast, plugin, and other server APIs.
  - `store`: Redux slices for auth, chat, settings, record, quota, subscription, package, sharing, and UI/menu state.
  - `components`: reusable chat/Markdown/file/theme/editor/message UI primitives.
  - `conf`: runtime/static model, storage, API, environment, bootstrap, subscription, and version config.
- Storage initialization:
  - `middleware.RegisterMiddleware` initializes database and Redis, then injects them into Gin context.
  - MySQL is the default when `mysql.host` exists; otherwise it falls back to SQLite at `./db/chatnio.db`.
  - Tables are created in code for auth, conversation, mask, sharing, package, quota, subscription, API keys, invitation, redeem, broadcast, record, and payment.
  - Migrations currently alter quota precision, add `auth.is_banned`, and add `conversation.task_id`.
- Request/middleware flow:
  - Gin engine uses CORS, built-in DB/Redis injection, Redis-backed rate limiting, and token/API-key authentication.
  - `/admin` routes require admin privileges in `AuthMiddleware`.
  - `Authorization: Bearer sk-*` is treated as API key auth; other bearer tokens are web user tokens.
- Backend route groups:
  - User/auth: `/login`, `/register`, `/state`, `/userinfo`, `/apikey`, `/quota`, `/subscription`, `/subscribe`, `/invite`, `/redeem`.
  - Admin: `/admin/*` for config, channels, charge rules, plans, users, records, analytics, payment, invitations, redeem codes, market, logs, and license.
  - Chat/web UI: `/chat` WebSocket plus `/conversation/*`, `/broadcast/*`.
  - OpenAI-compatible relay: `/v1/models`, `/v1/market`, `/v1/charge`, `/v1/plans`, `/v1/chat/completions`, `/v1/images/generations`, `/v1/videos`.
  - Provider callbacks/extras: `/mj/notify`, `/card`, `/generation/*`, `/article/*`, `/attachments/:hash`.
- Chat flow:
  - Frontend `ConnectionStack` connects to `${websocketEndpoint}/chat` and sends first-frame `{token, id}`.
  - Server `ChatAPI` authenticates the token/API key, loads or creates a conversation, and processes message types `chat`, `stop`, `restart`, `share`, `mask`, `edit`, and `remove`.
  - `ChatHandler` checks quota/subscription, applies web search and message cleanup, calls channel request logic, streams chunks to the client, records analytics, collects quota, and saves final response.
- Relay flow:
  - `/v1/chat/completions` only accepts API-key agent auth.
  - Relay supports stream and non-stream modes and converts responses to OpenAI-compatible JSON/SSE chunks.
  - Models prefixed with `web-` trigger web search wrapping; models suffixed with `-official` suppress quota field in response.
- Channel flow:
  - Channel config is loaded from Viper key `channel`, supports provider type, priority, weight, model list, retry count, secret(s), endpoint, model mapper, state, group, and proxy.
  - `Manager.Load` derives supported models and per-model preflight channel sequences.
  - `Ticker` chooses channels by descending priority; same-priority channels are selected by weight.
  - Adapter dispatch maps channel type to provider factories such as OpenAI, Azure, Claude, Palm/Gemini, Midjourney, SparkDesk, ZhipuAI, DashScope/Qwen, Dify, Coze, etc.
- Frontend runtime:
  - `conf/bootstrap.ts` derives REST and WebSocket endpoints from `VITE_BACKEND_ENDPOINT` or `/api`, configures Axios authorization, and synchronizes site info.
  - `router.tsx` defines pages for chat home, model market, wallet, account, auth, share, generation, article, and admin subpages.
  - Redux slices manage global info, auth, chat, quota, packages, subscriptions, API state, sharing, settings, records, avatars, and menu state.
- Deployment/build:
  - Root Dockerfile builds Go backend and React frontend in separate stages, then copies `/chat`, config template, templates, article template, and `app/dist` into a final Alpine image.
  - `docker-compose.yaml` starts MySQL, Redis, and the app, exposes `8000:8094`, and mounts `./config`, `./logs`, and `./storage`.
- 2026-06-17 Pro module diagnosis:
  - Screenshot shows `/admin/license` with all module cards marked as bought.
  - `admin/license.go` does not validate module-specific licenses. It only checks whether any `subscription` row has `enterprise = TRUE AND expired_at > NOW()`, then sets `Bought: hasPro` for `coai-pro`, `afdian`, `paypal`, `stripe`, and `digital`.
  - `app/src/components/admin/ProGate.tsx` only checks whether any returned module has `bought=true`; it does not check a specific module id.
  - Left-menu `Pro` badges in `app/src/components/admin/MenuBar.tsx` are only visual labels; they do not imply feature activation.
  - Implemented gated pages are limited to admin warmup, admin record, and admin payment list routes. License cards for PayPal/Stripe/Afdian/Digital do not themselves wire feature implementations.
  - Current payment frontend calls `/payment/create` and `/payment/check/:order`, but no matching local Go routes were found; existing `auth/payment.go` payment logic delegates to Deeptrain when `auth.use_deeptrain` is enabled, otherwise purchase returns `cannot find payment provider`.
- 2026-06-17 PayPal implementation:
  - PayPal top-up now uses the existing frontend contract: `POST /payment/create` creates a PayPal order and `GET /payment/check/:order` captures/checks it after buyer approval.
  - PayPal is enabled through `system.payment.paypal` only when `enabled=true`, `client_id` is set, and `secret` is set.
  - The public `/info` payload now returns `payment: ["paypal"]` when PayPal is configured, allowing the wallet to show the PayPal payment button.
  - Wallet top-up keeps the existing conversion rule: `amount = quota * 0.1`, so selecting 10 credits charges `1.00` in the configured PayPal currency.
  - Payment completion is idempotent: local order state is updated with `state = FALSE` guard inside a transaction before quota is credited.
  - Current scope is quota top-up only. Subscription purchase and module-specific PayPal licensing are still separate follow-up work.

## Technical Decisions
| Decision | Rationale |
|----------|-----------|
| Start from README, entry point, config, routers, and package manifests | These files usually define the product purpose, runtime model, external dependencies, and module boundaries. |

## Issues Encountered
| Issue | Resolution |
|-------|------------|
| Planning skill catchup script is absent in this installation | Continue with manually created planning files and log the missing script. |
| Pro license page shows modules as bought but module functionality may still be unavailable | Root cause appears to be placeholder/global `hasPro` authorization plus incomplete module-specific backend wiring, especially payment creation/check routes. |
| PayPal backend could not be compiled in this shell | `go`/`gofmt` are not available in PATH; frontend type/build checks passed, but Go verification remains pending. |

## Resources
- `/Users/wuzhixuan/code/project/coai/README.md`
- `/Users/wuzhixuan/code/project/coai/README_zh-CN.md`
- `/Users/wuzhixuan/code/project/coai/main.go`
- `/Users/wuzhixuan/code/project/coai/go.mod`
- `/Users/wuzhixuan/code/project/coai/app/package.json`
- `/Users/wuzhixuan/code/project/coai/config.example.yaml`
- `/Users/wuzhixuan/code/project/coai/utils/bootstrap.go`
- `/Users/wuzhixuan/code/project/coai/utils/config.go`
- `/Users/wuzhixuan/code/project/coai/connection/database.go`
- `/Users/wuzhixuan/code/project/coai/middleware/*.go`
- `/Users/wuzhixuan/code/project/coai/auth/router.go`
- `/Users/wuzhixuan/code/project/coai/admin/router.go`
- `/Users/wuzhixuan/code/project/coai/manager/router.go`
- `/Users/wuzhixuan/code/project/coai/manager/manager.go`
- `/Users/wuzhixuan/code/project/coai/manager/chat.go`
- `/Users/wuzhixuan/code/project/coai/manager/chat_completions.go`
- `/Users/wuzhixuan/code/project/coai/channel/*.go`
- `/Users/wuzhixuan/code/project/coai/adapter/adapter.go`
- `/Users/wuzhixuan/code/project/coai/app/src/router.tsx`
- `/Users/wuzhixuan/code/project/coai/app/src/conf/bootstrap.ts`
- `/Users/wuzhixuan/code/project/coai/app/src/api/connection.ts`
- `/Users/wuzhixuan/code/project/coai/Dockerfile`
- `/Users/wuzhixuan/code/project/coai/docker-compose.yaml`

## Visual/Browser Findings
- Not applicable yet.

## 2026-06-22 Jimeng API Documentation Research

### Supplied page 1817045: Jimeng Image Generation 4.0
- Official title: `即梦AI-图片生成4.0-接口文档`.
- The API unifies text-to-image, image editing, and multi-image composition.
- Input supports 0-10 JPEG/PNG images, each up to 15 MB and 4096 x 4096, with aspect ratio in `[1/3, 3]`.
- Output can contain multiple related images; maximum output count is `15 - input image count`.
- Official guidance recommends `force_single` when latency or price is sensitive.
- Base endpoint is `https://visual.volcengineapi.com`, method `POST`, content type `application/json`.
- Async submission uses query `Action=CVSync2AsyncSubmitTask` and `Version=2022-08-31`.
- Volcengine V4 request signing uses fixed `Region=cn-north-1` and `Service=cv` for this API.
- Business identifier is `req_key=jimeng_t2i_v40`.
- Source: https://www.volcengine.com/docs/85621/1817045?lang=zh

### Research Issues
- The first Jina fetch of supplied page `2533614` did not produce a file before the combined command timed out. Next attempt will fetch it separately or use the official page/web index instead of repeating the same combined request.

### Full 4.0 request lifecycle details
- Submission body: required `req_key` and `prompt`; optional `image_urls`, `size`, `width`, `height`, `scale`, `force_single`, `min_ratio`, `max_ratio`, callback/watermark/AIGC metadata options.
- `prompt` supports Chinese/English and is recommended to stay within 800 characters.
- Default `size` is `2048*2048` area (`4194304`); supported area is from `1024*1024` through `4096*4096`. Explicit `width` and `height` must be supplied together and take precedence.
- Default `scale=0.5`, allowed `[0,1]`; it trades prompt influence against reference-image influence.
- Submission success returns `code=10000` and `data.task_id`.
- Polling uses `Action=CVSync2AsyncGetResult`, the same version, `req_key`, and `task_id`; optional serialized `req_json` selects URL return, watermark, and implicit AIGC metadata.
- Poll response supplies `binary_data_base64` or 24-hour `image_urls`, plus status `in_queue`, `generating`, `done`, `not_found`, or `expired`. Tasks may expire after 12 hours.
- Callback mode is also supported through a public `callback_url`.
- Retry policy from official business errors: retry output-image copyright/risk failures (`50511`, `50519`) and 429 limit errors (`50429`, `50430`); do not blindly retry input-content failures or internal failures.

### Supplied page 2533614: official Jimeng documentation inventory
- Page title is `产品动态`, but its navigation exposes the current full Jimeng documentation tree.
- Image capabilities currently listed include Image Generation 4.6 and 4.0, POD/material extraction, product extraction, interactive inpainting, intelligent super-resolution, text-to-image 3.0/3.1, image-to-image 3.0 smart reference, and intelligent outpainting.
- Video and agent capabilities are separate families: Video Generation 3.0/3.0 Pro, motion imitation, OmniHuman 1.5, video translation 2.0, Motion Imitation 2.0, and Xiaoyunque video agents.
- The product notice was updated 2026-06-10 and says Xiaoyunque API free trial and free concurrency are now zero; this notice does not itself say the same about Jimeng image APIs.
- For this project's image/chat goal, deep research should prioritize authentication/quick start, pricing, Image Generation 4.6/4.0, inpainting, super-resolution, outpainting, and material/product extraction. Video/agent documents should remain indexed as future scope rather than driving the initial adapter.

### Product onboarding and pricing
- A Volcengine account must be registered, authenticated, and the selected Jimeng/Visual capability explicitly enabled in the Intelligent Visual console.
- Credentials are a Volcengine AccessKey pair: `AccessKeyID` (AK) and `AccessKeySecret` (SK). Official quick-start warns about main-account key risk; project implementation should use a least-privilege IAM sub-account where possible and never expose SK to the browser.
- Official SDK examples are available for Python, Go, PHP, and other languages through the AI platform quick-access documentation; this Go project should either use the official Go signing/client implementation or an equivalent reviewed V4 signer.
- Image pricing page updated 2026-03-31: Image Generation 4.0, 4.6, POD extraction, and product extraction are listed at CNY 0.22 per successfully generated image; inpainting is CNY 0.20 per successful call; super-resolution is CNY 0.40 per successful call. The control-panel price is authoritative.
- Free status is documented as 200 trial calls with concurrency 1; paid default concurrency is 1 or 2 depending on the console. Only successful image generation is charged.
- 4.0/4.6 may generate multiple images and charge per generated image; this reinforces using `force_single=true` as the project's safe/default interactive-chat behavior.

### Image Generation 4.6 compared with 4.0
- 4.6 is positioned for portrait retouching, graphic design/text replacement, and image stylization; it is based on Seedream 4.0.
- The async endpoint, action/version, signing region/service, task/poll/callback lifecycle, 1K-4K size range, prompt length guidance, URL lifetime, and task expiration semantics match the 4.0 pattern.
- 4.6 uses `req_key=jimeng_seedream46_cvtob`; 4.0 uses `req_key=jimeng_t2i_v40`.
- 4.6 accepts up to 14 input images vs 10 for 4.0; both cap total input+output images at 15. Official guidance recommends no more than 6 input/output images for 4.6 quality/stability.
- Important schema incompatibility: 4.6 defines `scale` as integer `[1,100]` with default `50`; 4.0 defines float `[0,1]` with default `0.5`. The adapter needs per-model normalization rather than sharing raw request structs blindly.
- 4.6 and 4.0 are priced identically in the current official table. Recommendation for new general editing/chat use: default to 4.6 when enabled, retain 4.0 as a selectable compatibility/model option.

### Authentication and SDK/HTTP integration
- Every request carries public parameters; recommended header signing uses `X-Date` and `Authorization`, optionally `X-Content-Sha256`, with signed headers normally including `content-type;host;x-content-sha256;x-date`.
- Authorization format is `HMAC-SHA256 Credential={AK}/{YYYYMMDD}/{Region}/{Service}/request, SignedHeaders={...}, Signature={...}`.
- Signature construction is canonical request -> string to sign -> HMAC key derivation by date/region/service/`request` -> final HMAC-SHA256 signature. Jimeng fixes region/service to `cn-north-1`/`cv`.
- Official guidance prefers SDK use to reduce signature bugs. The Visual SDK supports the exact `CVSync2AsyncSubmitTask` and `CVSync2AsyncGetResult` action family.
- Official Go SDK repository/example is `github.com/volcengine/volc-sdk-golang/tree/main/example/visual`; project dependency choice must be checked against the existing `go.mod` and current user-authored Jimeng code before adding anything.
- The generic official HTTP example warns that it is reference code and should be adapted/updated, not copied verbatim. AK/SK and signing must remain backend-only.
- Recommended configuration surface: channel-level `access_key`, `secret_key`, endpoint override, model/req-key mapping, default output mode, polling timeout/interval, and optional callback base URL. Credentials should reuse the current encrypted/secret channel storage mechanism rather than frontend environment variables.

### Image capability matrix beyond 4.0/4.6
- POD material extraction: `req_key=i2i_material_extraction`; one JPEG/PNG input; prompt field `image_edit_prompt` with four prescribed task families (pattern, packaging, logo, texture); optional `lora_weight`, 1024-4096 width/height, seed.
- Product extraction: `req_key=jimeng_i2i_extract_tiled_images`; one input; documented field `edit_prompt` with six prescribed product categories (full outfit, shoes, bag, sofa, daily goods, jewelry); 1024-4096 width/height, seed. The official example inconsistently names the field `image_edit_prompt`; implementation must test and preserve a compatibility alias rather than assuming the table/example typo.
- Inpainting/removal: `req_key=jimeng_image2image_dream_inpaint`; exactly two same-sized images (source then single-channel grayscale mask); mask black `0` means preserve and white `255` means redraw; prompt `删除` performs removal; max 4.7 MB and 4096 square; default seed 101.
- Intelligent super-resolution: `req_key=jimeng_i2i_seed3_tilesr_cvtob`; one input; target `resolution` is `4k` (default) or `8k`; detail-generation `scale` integer `[0,100]`, default 50; max input 4.7 MB and 4096 square.
- Text-to-image 3.0/3.1: `jimeng_t2i_v30` / `jimeng_t2i_v31`; optional prompt expansion `use_pre_llm=true`, seed, explicit paired width/height; default 1328 square; supported area 512-square through 2048-square and aspect ratio 1:3 to 3:1. These remain useful compatibility models but 4.6 should lead new general use.
- Image-to-image 3.0 smart reference: `req_key=jimeng_i2i_v30`; one input; natural-language editing, `scale` float `[0,1]` default 0.5, seed and dimensions; max input 4.7 MB / 4096 square / aspect <=3; explicit output dimensions are normalized to nearby multiples of 16 and output remains within 512-1536.
- Outpainting: `req_key=jimeng_img2img_seed3_painting_edit`; one image for proportional/aspect/directional expansion or two same-sized canvas+mask images for canvas expansion. Directional `top/bottom/left/right` values are each `[0,1]`; prompt is optional; callback/watermark/AIGC metadata are supported.
- All capabilities above share the same signed Visual endpoint and async submit/poll protocol, so the project should implement one Volcengine Visual transport plus per-capability request builders/validators.

### Current repository image architecture
- The working tree already contains a substantial uncommitted photo migration: `addition/photo`, its frontend route/components/hooks/API, database changes, generic image adapter interfaces, and `docs/photo-migration-plan.md`. This is user-owned in-progress work, not clean baseline.
- `adapter/jimeng` is currently a CLI/subprocess adapter. It saves base64 inputs to temp files and invokes commands such as `jimeng image2image`, `image_upscale`, `query_result`, and `multimodal2video`; its channel `secret` is interpreted as a CLI executable path. It does not call the official Volcengine Jimeng API.
- `adapter/dreamina` is currently an HTTP adapter for a custom Bearer-token service exposing `/v1/image2image`, `/v1/image_upscale`, and `/v1/query_result`. It is not compatible with official Volcengine V4 AK/SK signing or the Visual API action/body schema.
- The project already has generic `ImageEditFactory`, `ImageUpscaleFactory`, `ImageOutpaintFactory`, and `ImageToVideoFactory` interfaces, plus channel retry/priority routing for their request props. These are the correct seam for photo-tool integration.
- Current image factory registration maps `dreamina` to the custom HTTP adapter and `jimeng` to CLI. A new official API provider must not silently reuse either credential contract. Recommended type: `jimeng-api` or redefine `dreamina` only through an explicit migration with backward compatibility.
- Generic `ImageEditProps` lacks official API controls such as capability, multiple outputs, width/height/size, per-model scale semantics, force_single, mask role, seed, task ID, callback options, and AIGC/watermark metadata. The implementation guide should propose capability-specific option structs or an extensible options object rather than continuing to overload only prompt/model.
- Current photo flow already invokes channel-routed image edit/upscale/outpaint operations, making it suitable for official Jimeng API reuse after transport/props expansion.

### Chat and OpenAI image route integration gaps
- `POST /v1/images/generations` currently converts the prompt into a normal chat message and calls `channel.NewChatRequestWithCache`; it then extracts Markdown/base64 images from the chat buffer. It does not call the project's `ImageEditFactory` path.
- WebSocket chat's `createChatTask` has a special branch for video models, but otherwise always uses the normal chat channel factory. A Jimeng provider registered only in `imageProcessorFactories` therefore cannot be selected in conversation today.
- `channelFactories` does not register either `dreamina` or `jimeng`, while `imageProcessorFactories` does. This is the precise reason the current photo tool can use them but chat/OpenAI image generation cannot.
- Existing OpenAI relay input only models text-to-image fields `model`, `prompt`, and `n`; it has no image-edit/reference-image input surface. Conversation messages can contain image URLs in OpenAI-compatible arrays, but `transformContent` flattens them to plain URL text; the Web UI's file action embeds uploaded file text blocks rather than native image references.
- Recommended unification: introduce an `ImageGenerationFactory`/capability router that both `/v1/images/generations` and the WebSocket image-model branch call; preserve existing photo edit/upscale/outpaint interfaces as specialized operations over the same official Jimeng client.
- Chat UX should expose Jimeng as an image-capable model only when an enabled Jimeng API channel advertises that model. Selecting it routes the user's prompt and optional uploaded images to image generation/editing and streams progress plus final Markdown image(s) into the conversation. It should not attempt to make Jimeng a text-chat LLM.
- Generated 24-hour Volcengine URLs must be downloaded into existing `storage/results` (or configured object storage) before being persisted to a conversation/task; otherwise old conversations will break after URL expiry.

### Current local configuration state
- `config/config.yaml` currently has an enabled `jimeng` channel named `即梦CLI` with models `jimeng-v2` and `jimeng-video`; its endpoint is empty and the code interprets `secret` as CLI path, confirming this is CLI-based integration.
- Root `config.yaml` additionally contains a disabled `dreamina` channel named `即梦API` with models `dreamina-v2` and `dreamina-video`, but the code behind `dreamina` expects a custom Bearer endpoint rather than official Volcengine AK/SK signing.
- Recommendation: introduce explicit official Volcengine channel type/config (for example `jimeng-api`) and new model names such as `jimeng-seedream-4.6`, `jimeng-seedream-4.0`, `jimeng-inpaint`, `jimeng-superres`, and `jimeng-outpaint`; do not overload `jimeng-v2` until the CLI behavior is intentionally retired.
- Model market already supports the `image-generation` tag and the channel manager exposes configured models automatically through `/v1/models`, so the project mainly needs backend routing and admin channel metadata, not a wholly new model registry.

## 2026-06-18 Pro Feature Regression Recheck
- Git baseline is `main` at `3048a493eedcfe75de4d59afd5139847ad3195ed` with no local branches or tags besides `origin/main`.
- The repository baseline `HEAD` contains `app/src/routes/admin/License.tsx`, but that page is a placeholder: it hardcodes `data = { domain: "", digest: "" }`, shows all module cards with `bought={false}`, and only displays the Pro-required toast. It does not call a backend license API.
- In `HEAD`, `admin/license.go`, `admin/record.go`, `admin/payment.go`, `auth/paypal.go`, `auth/epay.go`, `manager/record.go`, `app/src/components/admin/ProGate.tsx`, `app/src/routes/admin/Record.tsx`, `Payment.tsx`, and `Warmup.tsx` do not exist. These are current working-tree additions, not committed baseline enterprise code.
- In `HEAD`, frontend `app/src/payment/request.ts` already calls `/payment/create` and `/payment/check/:order`, but no matching backend route or handler exists in `auth/router.go`. Local payment relies on `auth/payment.go`, which returns `cannot find payment provider` when `auth.use_deeptrain` is disabled.
- Current `admin/license.go` implements a local synthetic license response by querying `subscription` rows where `enterprise = TRUE AND expired_at > NOW()`. It then marks every module (`coai-pro`, `afdian`, `paypal`, `stripe`, `digital`) as bought based on this single boolean. This is not module-specific entitlement verification.
- Current `ProGate.tsx` authorizes a page when any returned module has `bought=true`; it does not require the specific module that backs the page.
- Current runtime `/info` exposes `payment:["epay"]`; `/v1/models` exposes only `deepseek-v4-pro`; `/v1/charge` has one token-billing rule for `deepseek-v4-pro`.
- Anonymous `/admin/license` returns 401, so license verification must be checked with an authenticated admin session.
- Binary backup comparison:
  - `chat.bak.20260617231909` already contains `/admin/license`, `/admin/record/list`, and `/admin/payment/view`, but not `/payment/create`, `/payment/check`, EasyPay callbacks, or `INSERT INTO record`.
  - Later backups progressively include PayPal and EasyPay routes.
  - Only the current `chat` binary contains `INSERT INTO record` and `api chat completion`, matching the latest usage-record fix.
- Conclusion: the observed breakage is not caused by a lost Pro subscription alone. The local baseline code did not contain complete Pro module implementations; current visible Pro pages are a local/secondary-development layer, and several missing backend paths have been filled during this session.

## 2026-06-18 Usage Record Closed-Loop Verification
- Database probe against the configured remote MySQL showed:
  - `auth_total=2`
  - `admin_users=1`
  - `subscription_total=1`
  - `enterprise_active=1`
  - `record_total=0` before verification
  - `payment_total=5`, all pending (`epay=3`, `paypal=2`)
- The valid enterprise subscription explains why the current local `/admin/license` gate marks Pro modules as bought.
- A first WebSocket chat probe using `deepseek-v4-pro` returned `empty response` and did not create a record. This proved the record path was not triggered because the upstream response buffer was empty.
- Direct upstream protocol probe showed the configured endpoint `101.34.212.83:8080` accepts both OpenAI-compatible `/v1/chat/completions` and Claude-compatible `/v1/messages`.
- Claude streaming responses from that endpoint start with `content_block_delta` events whose delta type is `thinking_delta` and whose payload field is `thinking`; the existing Claude adapter only parsed `delta.text`, so reasoning-only initial chunks were swallowed.
- Fixed `adapter/claude` to parse Claude `thinking_delta` chunks and wrap them as `<think>...</think>`, to close the thinking block before text chunks, and to return `no response` when a stream produces no non-empty chunks.
- After rebuilding and restarting the backend, the same WebSocket chat verification succeeded:
  - `record_before=0`
  - `record_after=1`
  - `record_delta=1`
  - latest record: `model=deepseek-v4-pro`, `channel_name=deepseek`, `input_tokens=8`, `output_tokens=15`, `quota=0.023`
- This confirms that the admin usage-record page should now have data after a successful chat with the configured model.
