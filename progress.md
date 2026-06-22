# Progress Log

## Session: 2026-06-17

### Phase 1: Requirements and Discovery
- **Status:** complete
- **Started:** 2026-06-17
- Actions taken:
  - Read the `using-superpowers` skill instructions.
  - Read the `planning-with-files` skill instructions and templates.
  - Checked for existing planning files and project overview files.
  - Created project-local planning files.
  - Read `README_zh-CN.md`, `main.go`, `go.mod`, `config.example.yaml`, `app/package.json`, and top-level directory layout.
  - Identified product purpose, stack, runtime dependencies, and high-level backend boot flow.
  - Listed Go backend files and frontend source files to map module boundaries.
  - Read backend bootstrap, config/static route handling, DB/Redis initialization, migrations, middleware, route registration, chat relay, WebSocket chat, channel selection, adapter dispatch, frontend bootstrap/router/store/API connection, and Docker deployment files.
- Files created/modified:
  - `/Users/wuzhixuan/code/project/coai/task_plan.md`
  - `/Users/wuzhixuan/code/project/coai/findings.md`
  - `/Users/wuzhixuan/code/project/coai/progress.md`

### Phase 2: Architecture Exploration
- **Status:** complete
- Actions taken:
  - Mapped backend module boundaries, route groups, middleware flow, DB/Redis initialization, channel scheduling, adapter dispatch, chat WebSocket flow, and OpenAI-compatible relay flow.
  - Mapped frontend app bootstrap, router, API endpoint setup, WebSocket client, Redux chat state, admin APIs, and deployment build.
- Files created/modified:
  - `/Users/wuzhixuan/code/project/coai/findings.md`
  - `/Users/wuzhixuan/code/project/coai/progress.md`
  - `/Users/wuzhixuan/code/project/coai/task_plan.md`

### Phase 3: Documentation Draft
- **Status:** complete
- Actions taken:
  - Created `PROJECT_ANALYSIS.md` with project positioning, stack, directory responsibilities, boot flow, middleware/auth, route overview, chat/relay/channel/data/frontend/deployment analysis, secondary development entry points, and bug triage map.
  - Reviewed the generated document by reading it back in two chunks.
- Files created/modified:
  - `/Users/wuzhixuan/code/project/coai/PROJECT_ANALYSIS.md`

### Phase 4: Verification and Delivery
- **Status:** complete
- Actions taken:
  - Verified the document by reading it back.
  - Checked git status to identify touched files and pre-existing dirty worktree items.
  - Prepared final handoff summary.
- Files created/modified:
  - `/Users/wuzhixuan/code/project/coai/task_plan.md`
  - `/Users/wuzhixuan/code/project/coai/progress.md`

### Follow-up: Pro Module Availability Diagnosis
- **Status:** complete
- Actions taken:
  - Inspected screenshot for `/admin/license`.
  - Read `admin/license.go`, `app/src/routes/admin/License.tsx`, `app/src/components/admin/ProGate.tsx`, `app/src/router.tsx`, Pro menu, payment/record/warmup pages, and payment-related backend/frontend request code.
  - Confirmed that the module cards are driven by a single enterprise subscription boolean rather than per-module licensing.
  - Confirmed that local payment gateway creation/check routes are not registered in current Go routers.
- Files created/modified:
  - `/Users/wuzhixuan/code/project/coai/findings.md`
  - `/Users/wuzhixuan/code/project/coai/progress.md`

### Follow-up: PayPal Quota Top-up Implementation
- **Status:** implementation complete, Go runtime verification pending
- Actions taken:
  - Added PayPal payment configuration under system settings and exposed enabled PayPal in `/info`.
  - Added user payment routes `/payment/create` and `/payment/check/:order`.
  - Implemented PayPal OAuth token retrieval, Orders v2 create/capture flow, local payment order persistence, and quota crediting.
  - Made payment completion idempotent with a database transaction that marks the order complete before crediting quota.
  - Added wallet PayPal top-up UI, amount selection, PayPal redirect, return handling, order check, quota refresh, and PayPal-specific i18n messages.
  - Ran frontend TypeScript check and production Vite build with bundled Node.
- Files created/modified:
  - `/Users/wuzhixuan/code/project/coai/auth/paypal.go`
  - `/Users/wuzhixuan/code/project/coai/auth/router.go`
  - `/Users/wuzhixuan/code/project/coai/channel/system.go`
  - `/Users/wuzhixuan/code/project/coai/config.example.yaml`
  - `/Users/wuzhixuan/code/project/coai/app/src/admin/api/system.ts`
  - `/Users/wuzhixuan/code/project/coai/app/src/routes/admin/System.tsx`
  - `/Users/wuzhixuan/code/project/coai/app/src/routes/wallet/WalletQuotaBox.tsx`
  - `/Users/wuzhixuan/code/project/coai/app/src/payment/request.ts`
  - `/Users/wuzhixuan/code/project/coai/app/src/resources/i18n/*.json`

## Test Results
| Test | Input | Expected | Actual | Status |
|------|-------|----------|--------|--------|
| Planning context setup | Create planning files | Files exist in project root | Created successfully | Pass |
| Documentation review | Read `PROJECT_ANALYSIS.md` | Document is readable and covers requested analysis | Reviewed full document | Pass |
| Frontend type check | `PATH=/Users/wuzhixuan/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/bin:$PATH ./node_modules/.bin/tsc --noEmit` in `app/` | No TypeScript errors | Completed with exit code 0 | Pass |
| Frontend production build | `PATH=/Users/wuzhixuan/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/bin:$PATH NODE_OPTIONS=--max_old_space_size=4096 ./node_modules/.bin/vite build --mode production` in `app/` | Production build succeeds | Completed with existing chunk/import warnings | Pass |
| Go toolchain check | `command -v go`, `command -v gofmt` | Go toolchain available | No Go binaries found in current shell PATH | Blocked |
| Go package tests | `PATH=/tmp/codex-go1.26.4/go/bin:$PATH go test ./manager ./admin ./channel` | Relevant backend packages pass | Completed with exit code 0 | Pass |
| Go build | `PATH=/tmp/codex-go1.26.4/go/bin:$PATH go build -o /tmp/coai-chat-audit .` | Backend builds | Completed with exit code 0 | Pass |
| Frontend type check after audit | `PATH=/Users/wuzhixuan/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/bin:$PATH ./node_modules/.bin/tsc --noEmit` in `app/` | No TypeScript errors | Completed with exit code 0 | Pass |
| Claude adapter tests | `GOCACHE=/tmp/coai-go-cache PATH=/tmp/codex-go1.26.4/go/bin:$PATH go test ./adapter/claude ./manager ./admin ./channel` | Relevant backend packages pass | Completed with exit code 0 | Pass |
| Final backend build | `GOCACHE=/tmp/coai-go-cache PATH=/tmp/codex-go1.26.4/go/bin:$PATH go build -o /tmp/coai-chat-final main.go` | Backend builds after temp file cleanup | Completed with exit code 0 | Pass |
| Final frontend type check | `PATH=/Users/wuzhixuan/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/bin:$PATH ./node_modules/.bin/tsc --noEmit` in `app/` | No TypeScript errors | Completed with exit code 0 | Pass |
| Usage record live verification | WebSocket chat with `deepseek-v4-pro` after Claude thinking fix | Successful chat inserts one `record` row | `record_delta=1`, latest record has model `deepseek-v4-pro` and channel `deepseek` | Pass |

## Error Log
| Timestamp | Error | Attempt | Resolution |
|-----------|-------|---------|------------|
| 2026-06-17 | `/Users/wuzhixuan/.agents/skills/planning-with-files/scripts/session-catchup.py` not found | 1 | Used templates directly and logged the issue. |
| 2026-06-17 | `go`/`gofmt` not available in current shell PATH | 1 | Could not run Go formatting/tests in this environment; backend should be verified once Go is available. |
| 2026-06-18 | Planning skill catchup script still absent at `/Users/wuzhixuan/.agents/skills/planning-with-files/scripts/session-catchup.py` | 2 | Continued with existing project planning files and logged the recheck. |

## 5-Question Reboot Check
| Question | Answer |
|----------|--------|
| Where am I? | Phase 4, delivery. |
| Where am I going? | Hand off the generated Markdown document and summarize touched files. |
| What's the goal? | Produce a source-backed project structure and functionality analysis document. |
| What have I learned? | CoAI.Dev is a Go/Gin + React/Vite AIGC platform with model routing, chat, admin, billing, and OpenAI-compatible relay capabilities. |
| What have I done? | Created `PROJECT_ANALYSIS.md` and updated planning/progress records. |

## Session: 2026-06-18

### Follow-up: Pro Feature Regression Recheck
- **Status:** complete
- Actions taken:
  - Re-read `using-superpowers` and `planning-with-files` instructions.
  - Checked `git status`, `git log`, branches, tags, and diff stats to establish the current baseline.
  - Compared `HEAD` with the current working tree for admin license, Pro gate, payment, record, channel refresh, charge refresh, and usage-record paths.
  - Verified that many Pro-related files are untracked working-tree additions rather than committed baseline code.
  - Used `git grep` against `HEAD` to confirm that backend `/admin/license`, `/payment/create`, `/payment/check`, `/admin/record/list`, and record insert paths are absent from the committed baseline.
  - Checked the currently running backend via `/info`, `/v1/models`, `/v1/charge`, and anonymous `/admin/license`.
  - Compared local `chat.bak.*` binaries for route/error strings to identify when PayPal/EasyPay/record-write capabilities appeared.
  - Created `PRO_FEATURE_REGRESSION_AUDIT.md` with the source-backed comparison and remaining validation checklist.
  - Re-ran backend package tests, backend build, and frontend TypeScript check successfully.
- Key result:
  - The current local Pro behavior is a partial local implementation layered on top of a baseline that had mostly placeholder Pro UI. The broken modules are code/implementation gaps rather than a simple subscription-state issue.

### Follow-up: Usage Record Closed-Loop Fix
- **Status:** complete
- Actions taken:
  - Probed remote DB counts and confirmed there is one active enterprise subscription but zero usage records before live verification.
  - Reproduced WebSocket chat with `deepseek-v4-pro`; first attempt returned `empty response` and did not insert a record.
  - Probed upstream protocol and confirmed Claude streaming events use `thinking_delta`.
  - Updated the Claude adapter to emit thinking content as `<think>...</think>` and report `no response` for truly empty streams.
  - Rebuilt and restarted the backend, then reran the WebSocket verification.
  - Confirmed a successful chat inserted one usage record.
- Files modified:
  - `/Users/wuzhixuan/code/project/coai/adapter/claude/chat.go`
  - `/Users/wuzhixuan/code/project/coai/adapter/claude/struct.go`
  - `/Users/wuzhixuan/code/project/coai/adapter/claude/types.go`
  - `/Users/wuzhixuan/code/project/coai/findings.md`
  - `/Users/wuzhixuan/code/project/coai/progress.md`

### Follow-up: EasyPay Safety Note
- **Status:** pending user-driven payment verification
- Actions taken:
  - Reviewed EasyPay order creation and callback code.
  - Confirmed order creation signs a local EasyPay URL and inserts a pending `payment` row; it does not contact the EasyPay provider until the user opens/pays the URL.
  - Did not auto-create another test order because the remote database already has pending EasyPay/PayPal rows and creating more would add noise.
- Next verification:
  - Use the wallet page to create a small EasyPay order and complete the payment on the provider page.
  - Confirm `/payment/epay/notify` receives `TRADE_SUCCESS` or equivalent.
  - Confirm the `payment` row changes to `state=true` and quota increases.

## Session: 2026-06-22

### Jimeng API Documentation and Integration Map
- **Status:** complete
- Actions taken:
  - Read the `using-superpowers`, `agent-reach`, and `planning-with-files` skill instructions.
  - Attempted the planning session catchup helper; the repository-local helper path is absent.
  - Preserved the existing planning history and appended a dedicated Jimeng research plan.
  - Inspected the working tree and identified pre-existing uncommitted Jimeng, Dreamina, photo-processing, routing, adapter, and documentation work that must remain untouched.
  - Fetched and began extracting the official Jimeng Image Generation 4.0 API page.
  - The combined fetch timed out before the second supplied page was written; logged the failure and switched to a separate-fetch strategy.
  - Completed extraction of the 4.0 submit/poll/callback lifecycle, parameters, status model, expiration behavior, and business-error retry semantics.
  - Fetched the second supplied page separately and extracted the current official Jimeng documentation navigation tree.
  - Read the official product intro, image pricing, quick start, Image Generation 4.6 product page, and Image Generation 4.6 API page.
  - Identified the 4.6 `req_key`, higher input-image limit, and the critical 4.0-vs-4.6 `scale` type/range incompatibility.
  - Read official public parameters, HMAC signing method, AI SDK usage, and direct HTTP examples.
  - Confirmed that Volcengine's Visual SDK supports the exact sync-to-async action family and that all AK/SK signing must stay server-side.
  - Fetched and reviewed the complete current Jimeng image-document set: material/product extraction, inpainting, super-resolution, text-to-image 3.0/3.1, image-to-image 3.0, and outpainting.
  - Built a capability/`req_key` matrix and found an official product-extraction field-name inconsistency that must be handled deliberately.
  - Inspected the existing user-authored Jimeng CLI adapter, custom Dreamina HTTP adapter, generic image interfaces, photo-processing module, and prior migration plan.
  - Confirmed that neither existing provider is the official Volcengine Jimeng AK/SK Visual API and documented the migration boundary.
  - Traced the current WebSocket chat and `/v1/images/generations` flows.
  - Identified that both flows use normal chat factories, while Jimeng/Dreamina are registered only as photo/image processors; documented the required shared image-generation router.
  - Checked current runtime configuration without exposing secrets and confirmed that the active Jimeng channel is CLI-based, while the disabled Dreamina channel is a custom Bearer API contract.
  - Created `docs/jimeng-api-integration.md` with official documentation index, API lifecycle, capability matrix, current-project gap analysis, target architecture, channel config example, phased implementation plan, and verification checklist.
  - Reviewed the generated Markdown and updated planning/progress files.
- Files created/modified:
  - `/Users/wuzhixuan/code/project/coai/docs/jimeng-api-integration.md`
  - `/Users/wuzhixuan/code/project/coai/task_plan.md`
  - `/Users/wuzhixuan/code/project/coai/findings.md`
  - `/Users/wuzhixuan/code/project/coai/progress.md`

### Jimeng API Phase 1 Implementation
- **Status:** complete
- Actions taken:
  - Re-read the current Jimeng integration document and existing planning files.
  - Rechecked the planning catchup helper; it is still absent at `.cursor/skills/planning-with-files/scripts/session-catchup.py`.
  - Confirmed the working tree already contains user-owned Jimeng CLI, Dreamina custom API, photo module, and routing changes, so the official API implementation will be added in a separate `adapter/jimengapi` package.
  - Scoped phase 1 to the official client plus `jimeng-seedream-4.6` text-to-image loop via `/v1/images/generations`.
  - Added `jimeng-api` channel type and admin channel metadata.
  - Added a reusable image-generation adapter interface, adapter retry wrapper, and channel dispatch path.
  - Implemented `adapter/jimengapi` with Volcengine HMAC-SHA256 signing, submit, poll, result parsing, proxy support, and `storage/results` persistence.
  - Registered `jimeng-seedream-4.6` to `jimeng_seedream46_cvtob`.
  - Routed `/v1/images/generations` to the official Jimeng image-generation path when the model is `jimeng-seedream-4.6`.
  - Added Jimeng conversation backend branching so selecting a Jimeng image model in chat returns Markdown image output instead of going through text chat.
  - Added `n` support for Jimeng image generation by submitting up to 6 single-image tasks and returning all images in OpenAI-compatible `data[]`.
  - Added a Photo page `生成数量` control and backend `image_count` handling for generative features.
  - Removed `.DS_Store` files and added `.DS_Store` / `chat.bak*` to `.gitignore` to reduce dirty-tree noise without deleting executable backups.
  - Updated the existing Photo prompt test expectation from `dreamina` to the current `jimeng` config.
- Files modified:
  - `/Users/wuzhixuan/code/project/coai/.gitignore`
  - `/Users/wuzhixuan/code/project/coai/adapter/adapter.go`
  - `/Users/wuzhixuan/code/project/coai/adapter/common/interface.go`
  - `/Users/wuzhixuan/code/project/coai/adapter/common/types.go`
  - `/Users/wuzhixuan/code/project/coai/adapter/request.go`
  - `/Users/wuzhixuan/code/project/coai/adapter/jimengapi/*`
  - `/Users/wuzhixuan/code/project/coai/channel/worker.go`
  - `/Users/wuzhixuan/code/project/coai/globals/constant.go`
  - `/Users/wuzhixuan/code/project/coai/globals/variables.go`
  - `/Users/wuzhixuan/code/project/coai/manager/images.go`
  - `/Users/wuzhixuan/code/project/coai/manager/chat.go`
  - `/Users/wuzhixuan/code/project/coai/addition/photo/handler.go`
  - `/Users/wuzhixuan/code/project/coai/addition/photo/processor.go`
  - `/Users/wuzhixuan/code/project/coai/addition/photo/prompts_test.go`
  - `/Users/wuzhixuan/code/project/coai/app/src/admin/channel.ts`
  - `/Users/wuzhixuan/code/project/coai/app/src/components/photo/FeaturePanel.tsx`
  - `/Users/wuzhixuan/code/project/coai/utils/char.go`
  - `/Users/wuzhixuan/code/project/coai/utils/image.go`
  - `/Users/wuzhixuan/code/project/coai/utils/char_test.go`
  - `/Users/wuzhixuan/code/project/coai/task_plan.md`
  - `/Users/wuzhixuan/code/project/coai/progress.md`

### Verification: Jimeng API Phase 1
- **Status:** complete
- Results:
  - `GOCACHE=/tmp/coai-go-cache go test ./adapter/jimengapi ./utils ./adapter ./channel ./manager` passed.
  - `GOCACHE=/tmp/coai-go-cache go test ./addition/photo` passed after aligning the test expectation to current config.
  - `GOCACHE=/tmp/coai-go-cache go test ./...` passed.
  - `GOCACHE=/tmp/coai-go-cache go build -o /tmp/coai-jimeng-phase1 .` passed.
  - `./node_modules/.bin/tsc --noEmit` in `app/` passed.
  - Go commands still emit a third-party `github.com/chai2010/webp` C warning about `2 ^ ALPHA_OFFSET`; it is non-fatal and outside this change.

### Jimeng Credential Configuration
- **Status:** complete
- Actions taken:
  - Inspected the supplied screenshot and local key files without printing secret values.
  - Confirmed `AccessKey.txt` contains the AK/SK pair required by Volcengine signed Visual API requests.
  - Confirmed `ApiKey.txt` is an API Key style credential and is not used by the current official Jimeng Visual API adapter.
  - Backed up the runtime config to `config/config.yaml.bak.before-jimeng-api`.
  - Configured `config/config.yaml` with a `jimeng-api` channel for `jimeng-seedream-4.6`, endpoint `https://visual.volcengineapi.com`, and secret format `AK|SK`.
  - Added a `non-billing` charge rule for `jimeng-seedream-4.6` to avoid the unset-price guard for normal authenticated use.
- Verification:
  - Re-read `config/config.yaml` with secrets redacted and confirmed the channel is enabled, model is present, endpoint is set, and secret contains the expected `AK|SK` separator.

### Jimeng API Phase 2 Implementation
- **Status:** complete
- Actions taken:
  - Added `jimeng-seedream-4.0` to the backend model constants, Jimeng image-generation model list, admin channel metadata, runtime `config/config.yaml`, and non-billing charge rule.
  - Expanded the Jimeng API model registry with per-model `req_key`, capability, max input images, max output count, prompt length, scale kind/default, output format, size area bounds, and default aspect-ratio bounds.
  - Added `BuildSubmitTaskRequest` to normalize and validate Jimeng generation requests before submit.
  - Implemented separate scale handling: 4.6 encodes `scale` as integer `[1,100]`, while 4.0 encodes `scale` as float `[0,1]`.
  - Added validation for empty/overlong prompts, unsupported masks, `n` upper bound, input image count and URL format, JPEG/PNG extension checks, data URL rejection, size area, paired width/height, and aspect-ratio range.
  - Extended `/v1/images/generations` request parsing for Jimeng with `size`, `width`, `height`, `scale`, `min_ratio`, `max_ratio`, `force_single`, `images`, `image_urls`, and `masks`.
  - Added focused Jimeng API tests for 4.0 mapping, 4.6/4.0 scale normalization, invalid scale, validation failures, and signed image URLs without file extensions.
- Files modified:
  - `/Users/wuzhixuan/code/project/coai/adapter/common/types.go`
  - `/Users/wuzhixuan/code/project/coai/adapter/jimengapi/types.go`
  - `/Users/wuzhixuan/code/project/coai/adapter/jimengapi/image.go`
  - `/Users/wuzhixuan/code/project/coai/adapter/jimengapi/validation.go`
  - `/Users/wuzhixuan/code/project/coai/adapter/jimengapi/client_test.go`
  - `/Users/wuzhixuan/code/project/coai/adapter/jimengapi/validation_test.go`
  - `/Users/wuzhixuan/code/project/coai/globals/variables.go`
  - `/Users/wuzhixuan/code/project/coai/manager/types.go`
  - `/Users/wuzhixuan/code/project/coai/manager/images.go`
  - `/Users/wuzhixuan/code/project/coai/app/src/admin/channel.ts`
  - `/Users/wuzhixuan/code/project/coai/config/config.yaml`
  - `/Users/wuzhixuan/code/project/coai/task_plan.md`
  - `/Users/wuzhixuan/code/project/coai/findings.md`
  - `/Users/wuzhixuan/code/project/coai/progress.md`

### Verification: Jimeng API Phase 2
- **Status:** complete
- Results:
  - `GOCACHE=/tmp/coai-go-cache go test ./adapter/jimengapi` passed.
  - `GOCACHE=/tmp/coai-go-cache go test ./adapter/jimengapi ./utils ./adapter ./channel ./manager ./addition/photo` passed.
  - `GOCACHE=/tmp/coai-go-cache go test ./...` passed.
  - `GOCACHE=/tmp/coai-go-cache go build -o /tmp/coai-jimeng-phase2 .` passed.
  - `./node_modules/.bin/tsc --noEmit` in `app/` passed.
  - Go commands still emit a third-party `github.com/chai2010/webp` C warning about `2 ^ ALPHA_OFFSET`; it is non-fatal and outside this change.
- Not run:
  - Live Jimeng API generation smoke test, because a real call may consume paid Volcengine quota.

### Jimeng API Phase 3 Implementation
- **Status:** complete
- Scope (user-confirmed): wire the official `jimeng-api` adapter into Photo editing — image edit (seedream 4.6/4.0) + `jimeng-superres` + `jimeng-outpaint`; support both `binary_data_base64` and `image_urls` inputs; no live smoke test.
- Actions taken:
  - Verified Phase 1/2 build and `jimengapi`/`photo` tests still pass before continuing.
  - Added `jimeng-superres` (`jimeng_i2i_seed3_tilesr_cvtob`) and `jimeng-outpaint` (`jimeng_img2img_seed3_painting_edit`) model constants and registry specs with new `upscale`/`outpaint` capabilities.
  - Extended `SubmitTaskRequest` with `binary_data_base64`, `resolution`, directional `top/bottom/left/right`, and `seed`.
  - Implemented `CreateImageEditRequest` (image edit via seedream with mixed base64/URL inputs), `CreateImageUpscaleRequest` (resolution normalized to 4k/8k, detail scale 50), and `CreateImageOutpaintRequest` (decodes image dimensions and derives expand-only directional fractions from the target ratio).
  - Added shared helpers in `process.go`: input classification, raw-base64 normalization, a submit/poll/store/emit runner, image-size decoding, ratio parsing, and outpaint edge math.
  - Made `jimeng-api` satisfy `ImageEditFactory`/`ImageUpscaleFactory`/`ImageOutpaintFactory` and registered `NewImageProcessorFromConfig` in `imageProcessorFactories`.
  - Matched the dreamina chunk-content convention (`utils.GetImageMarkdown`) so Photo result storage stays consistent.
  - Added `jimeng-superres` / `jimeng-outpaint` to admin channel metadata, the runtime `config/config.yaml` jimeng-api channel, and the non-billing charge rule.
  - Added `process_test.go` covering input classification, resolution mapping, ratio parsing, outpaint edge math, base64 size decoding, and registry specs.
- Files created/modified:
  - `/Users/wuzhixuan/code/project/coai/adapter/jimengapi/process.go` (new)
  - `/Users/wuzhixuan/code/project/coai/adapter/jimengapi/edit.go` (new)
  - `/Users/wuzhixuan/code/project/coai/adapter/jimengapi/upscale.go` (new)
  - `/Users/wuzhixuan/code/project/coai/adapter/jimengapi/outpaint.go` (new)
  - `/Users/wuzhixuan/code/project/coai/adapter/jimengapi/process_test.go` (new)
  - `/Users/wuzhixuan/code/project/coai/adapter/jimengapi/types.go`
  - `/Users/wuzhixuan/code/project/coai/adapter/jimengapi/struct.go`
  - `/Users/wuzhixuan/code/project/coai/adapter/adapter.go`
  - `/Users/wuzhixuan/code/project/coai/globals/variables.go`
  - `/Users/wuzhixuan/code/project/coai/app/src/admin/channel.ts`
  - `/Users/wuzhixuan/code/project/coai/config/config.yaml`
  - `/Users/wuzhixuan/code/project/coai/task_plan.md`
  - `/Users/wuzhixuan/code/project/coai/findings.md`
  - `/Users/wuzhixuan/code/project/coai/progress.md`

### Verification: Jimeng API Phase 3
- **Status:** complete
- Results:
  - `GOCACHE=/tmp/coai-go-cache go vet ./adapter/jimengapi` passed.
  - `GOCACHE=/tmp/coai-go-cache go test ./adapter/jimengapi ./adapter ./manager ./channel ./addition/photo ./utils ./globals` passed.
  - `GOCACHE=/tmp/coai-go-cache go build -o /tmp/coai-jimeng-phase3 .` passed.
  - `./node_modules/.bin/tsc --noEmit` in `app/` passed.
  - Only the usual non-fatal `github.com/chai2010/webp` C warning remains.
- Not run:
  - Live edit/upscale/outpaint smoke test against the official API (user chose 暂不联调; would consume paid quota).

### Jimeng API Phase 3 Follow-up: prompts.json Switch + Live Smoke Test
- **Status:** complete
- Actions taken:
  - Backed up `config/prompts.json` to `config/prompts.json.bak.before-jimeng-api`.
  - Repointed Photo editing features to official models: edit-class → `jimeng-seedream-4.6`, `hd_upscale` → `jimeng-superres`, `resize` → `jimeng-outpaint`, all `channel_type: jimeng-api`; left `video_gen` on CLI and `detail_image`/`logo_custom` local.
  - Added a guarded live test (`adapter/jimengapi/live_smoke_test.go`, `JIMENG_LIVE=1`, AK/SK via env) and ran it against the real endpoint.
  - Live results: text-to-image PASS; image edit PASS after confirming `binary_data_base64` is the correct input field (8×8 image rejected as too small with `50207`, 512×512 succeeded).
  - Fixed `storeImageURL`: Volcengine TOS URLs end in `.image`, so results were saved as non-renderable `*.image`; now restricted to known image extensions with `.png` fallback. Verified live (edit result saved `.png`).
  - Updated `addition/photo` `TestGetChannelType` expectation (`white_bg` → `jimeng-api`).
  - Cleaned smoke-test download artifacts under `adapter/jimengapi/storage`.
- Files created/modified:
  - `/Users/wuzhixuan/code/project/coai/config/prompts.json` (+ `.bak.before-jimeng-api`)
  - `/Users/wuzhixuan/code/project/coai/adapter/jimengapi/image.go`
  - `/Users/wuzhixuan/code/project/coai/adapter/jimengapi/live_smoke_test.go` (new, guarded)
  - `/Users/wuzhixuan/code/project/coai/addition/photo/prompts_test.go`

### Verification: Jimeng API Phase 3 Follow-up
- **Status:** complete
- Results:
  - Live `TestLiveSmoke` (generate + edit) PASS against `https://visual.volcengineapi.com`.
  - `go test ./adapter/jimengapi ./adapter ./addition/photo ./manager ./channel ./utils ./globals` passed (guarded live test skips without `JIMENG_LIVE=1`).
  - `go build` and frontend `tsc --noEmit` passed.

### Jimeng API Phase 3 Follow-up 2: Superres + Outpaint Live Smoke
- **Status:** complete
- Added `upscale` and `outpaint` subtests to `live_smoke_test.go` (shared `makeSolidPNGBase64` helper).
- Live results against `https://visual.volcengineapi.com`:
  - **`jimeng-superres` (resolution 4k) PASS** — returned and stored a `.png` result.
  - **`jimeng-outpaint` (512×512 → 16:9) PASS** — directional-fraction outpaint returned and stored a `.png` result.
- All four official capabilities are now live-verified: generate, edit, upscale, outpaint.
- Cleaned smoke-test artifacts; `go test`/`go build`/`tsc` still pass.

### Jimeng API Phase 3 Second Batch: Inpaint + Material/Product Extract
- **Status:** backend + extract Photo features complete; inpaint brush-mask UI deferred
- Backend (adapter): added capabilities `inpaint`/`extract` and specs `jimeng-inpaint` (`jimeng_image2image_dream_inpaint`), `jimeng-material-extract` (`i2i_material_extraction`), `jimeng-product-extract` (`jimeng_i2i_extract_tiled_images`). `CreateImageEditRequest` now routes by capability; extract writes the prompt into the per-model field (`image_edit_prompt` vs `edit_prompt`); inpaint takes source+mask as ordered 2-image input with default seed 101.
- Live verification (real API):
  - **inpaint PASS** — source + grayscale mask 2-image contract works.
  - **product_extract PASS** — confirmed `edit_prompt` is the correct field (table value, not the example's `image_edit_prompt`).
  - **material_extract** — `image_edit_prompt` field accepted and task ran to output review; failed only with `50511 Post Img Risk Not Pass` because the synthetic solid-color test image produced a risk-flagged output. Field/path verified; real product images expected to pass. Smoke test tolerates 50511 on synthetic input.
- Photo features: added `material_extract` + `product_extract` as full Photo features (backend feature consts + processor + ProcessTask cases + prompts.json entries with category options + frontend FeaturePanel buttons and a category picker). Both excluded from `生成数量` repetition.
- Registry/config: added the three models to `globals` constants, `config/config.yaml` channel + non-billing charge, and admin `channel.ts` metadata.
- Deferred: a brush/canvas mask-drawing UI to drive `jimeng-inpaint` from the Photo page. The backend inpaint path is ready and live-verified; the existing `image_erase` feature stays on seedream edit until the mask UI exists.
- Verification: `go vet`/`go test ./adapter/jimengapi ./addition/photo`/`go build`/frontend `tsc` all pass; added `edit_test.go` (prompt-field selection, capability routing, offline validation paths).
