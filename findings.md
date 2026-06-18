# Findings and Decisions

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
