# Task Plan: Pro Feature Audit and Payment Integration

## Goal
Enable payment/record Pro-adjacent features and keep a source-backed audit of why current Pro modules differ from the remembered working state.

## Current Phase
Follow-up Audit

## Phases

### Phase 1: Discovery
- [x] Inspect existing payment, wallet, subscription, license, and system config code.
- [x] Confirm PayPal Orders API flow from official docs.
- [x] Identify local backend gaps.
- **Status:** complete

### Phase 2: Backend Implementation
- [x] Add PayPal config to system config.
- [x] Add `/payment/create` and `/payment/check/:order` routes.
- [x] Implement OAuth token, create order, show/capture order, payment table update, and quota credit.
- **Status:** complete

### Phase 3: Frontend Wiring
- [x] Add PayPal config fields to admin system settings.
- [x] Surface enabled PayPal in site info.
- [x] Add wallet PayPal top-up UI and return/check handling.
- **Status:** complete

### Phase 4: Verification
- [x] Run Go tests/build checks.
- [x] Run frontend type/build checks if feasible.
- [x] Update findings/progress and summarize result.
- **Status:** complete

### Follow-up Audit: Pro Feature Regression Recheck
- [x] Compare current working tree with `HEAD`.
- [x] Confirm which Pro files are baseline code vs local additions.
- [x] Verify runtime `/info`, `/v1/models`, `/v1/charge`, and `/admin/license` behavior.
- [x] Compare local `chat.bak.*` binaries for key route/record-write strings.
- [x] Write `PRO_FEATURE_REGRESSION_AUDIT.md`.
- [x] Re-run Go package tests, Go build, and frontend TypeScript check.
- **Status:** complete

## Decisions Made
| Decision | Rationale |
|----------|-----------|
| Start with quota top-ups only | Existing subscription purchase flow consumes internal quota; PayPal subscription orders require additional product/order semantics. |
| Use PayPal Orders v2 CAPTURE flow | Official PayPal docs recommend server-side Orders API for checkout and immediate capture after buyer approval. |
| Store PayPal order id in `payment.order_id` | Existing payment table has no separate external order id field; PayPal order id is unique and can be polled/captured. |
| Complete payment orders inside a DB transaction | Prevent duplicate quota credit when the PayPal return/check endpoint is refreshed or hit concurrently. |

## Errors Encountered
| Error | Attempt | Resolution |
|-------|---------|------------|
| Existing payment frontend calls `/payment/create` and `/payment/check/:order`, but local backend routes were absent | 1 | Implement those routes for PayPal. |
| `go`/`gofmt` are not available in the current shell PATH | 1 | Frontend checks were run with bundled Node; Go verification remains pending for an environment with Go installed. |
| Baseline Pro page looked like a license page but did not validate a license | 1 | Compared `HEAD:app/src/routes/admin/License.tsx` and current `admin/license.go`; documented the gap in `PRO_FEATURE_REGRESSION_AUDIT.md`. |

## Current Task: Jimeng API Documentation and Integration Map (2026-06-22)

### Goal
Read the supplied Volcengine Jimeng API documentation and the related official documentation set, map it to the current CoAI image-processing and chat/channel architecture, and create a source-backed Markdown implementation guide under `docs/`.

### Phase 1: Official Documentation Inventory
- [x] Read both supplied documentation pages in full.
- [x] Discover and classify related official Jimeng API documents.
- [x] Record authentication, endpoints, task lifecycle, parameters, responses, errors, quotas, and model capabilities.
- **Status:** complete

### Phase 2: Repository Architecture Mapping
- [x] Inspect existing `adapter/jimeng`, `adapter/dreamina`, photo-processing, image-generation, and chat/channel paths.
- [x] Identify reusable abstractions and implementation gaps.
- **Status:** complete

### Phase 3: Markdown Deliverable
- [x] Create a Jimeng API documentation index and CoAI integration design under `docs/`.
- [x] Include configuration, request flows, data models, API examples, phased implementation plan, and source links.
- **Status:** complete

### Phase 4: Verification
- [x] Verify every documented field and endpoint against official sources.
- [x] Check that proposed file paths and extension points exist in the current working tree.
- [x] Review Markdown completeness and update progress/findings.
- **Status:** complete

### Task Decisions
| Decision | Rationale |
|----------|-----------|
| Treat current uncommitted Jimeng/photo code as user-owned work | The working tree already contains substantial uncommitted and untracked implementation; this research task must not overwrite it. |
| Produce documentation and an implementation map before coding | The user explicitly requested full documentation research and a repository Markdown deliverable to guide later image/chat integration. |

### Task Errors
| Error | Attempt | Resolution |
|-------|---------|------------|
| Planning skill catchup helper is absent at the repository-local `.cursor` path | 1 | Reused existing project planning files and added this task as a new section. |

## Current Task: Jimeng API Phase 1 Implementation (2026-06-22)

### Goal
Add the official Volcengine Jimeng API client and first working `jimeng-seedream-4.6` single text-to-image loop, without replacing the existing CLI-based `jimeng` adapter or custom `dreamina` adapter.

### Phase 1: Official Client + Single Text-to-Image Loop
- [x] Add a dedicated `jimeng-api` channel type.
- [x] Add a reusable image-generation interface and channel dispatch path.
- [x] Implement official Visual API signing, submit, poll, and result parsing under `adapter/jimengapi/`.
- [x] Register `jimeng-seedream-4.6` → `jimeng_seedream46_cvtob`.
- [x] Route `/v1/images/generations` to the official image-generation path when the selected model is a Jimeng image model.
- [x] Persist returned image URLs to `storage/results` and return an OpenAI-compatible image URL response.
- [x] Add focused tests for signing/model mapping and run backend verification.
- **Status:** complete

### Phase 1b: Cleanup and Image Count Option
- [x] Remove `.DS_Store` files and ignore future `.DS_Store` / `chat.bak*` noise.
- [x] Support `n` for Jimeng `/v1/images/generations` by submitting up to 6 single-image tasks.
- [x] Return all generated image URLs in OpenAI-compatible `data[]`.
- [x] Add a backend Jimeng image-generation branch for chat conversations, defaulting to one image.
- [x] Add a Photo page `生成数量` option and pass `image_count` to the backend.
- [x] Apply `image_count` only to generative Photo features; deterministic/local/video features are not repeated.
- **Status:** complete

### Decisions
| Decision | Rationale |
|----------|-----------|
| Add `adapter/jimengapi` instead of modifying `adapter/jimeng` | Existing `jimeng` is a CLI/subprocess adapter; mixing it with AK/SK official API would make configuration ambiguous. |
| Use `Channel.Secret` format `AK|SK` for phase 1 | Existing channel config already supports multi-line secret pools and `SplitRandomSecret(2)`. |
| Start with `/v1/images/generations` | It gives the smallest external API closed loop before broader Photo and WebSocket chat integration. |
| Implement Jimeng `n` by repeated single-image tasks | The official 4.6 API has no explicit `n` field; repeated `force_single=true` tasks are more predictable for cost and output count. |
| Keep chat image count defaulted to 1 | The current chat UI has no image-model-specific settings panel; adding one should be a separate UI pass. |

### Errors
| Error | Attempt | Resolution |
|-------|---------|------------|
| Planning skill catchup helper is still absent at `.cursor/skills/planning-with-files/scripts/session-catchup.py` | 1 | Continued from existing `task_plan.md`, `findings.md`, and `progress.md`. |
| `gofmt` was accidentally run with `.gitignore` in the file list | 1 | Re-ran `gofmt` with only Go files; `.gitignore` was not modified by gofmt. |
| `addition/photo` tests expected `dreamina` while current prompts config returns `jimeng` | 1 | Updated the test expectation to match the current config. |

## Current Task: Jimeng API Phase 2 Implementation (2026-06-22)

### Goal
Upgrade the official Jimeng API adapter from a first 4.6-only loop into a model/capability registry with per-model request normalization and validation, starting with `jimeng-seedream-4.6` and `jimeng-seedream-4.0`.

### Phase 2: Model/Capability Registry and Validation
- [x] Add `jimeng-seedream-4.0` to backend/admin/runtime model surfaces.
- [x] Extend the Jimeng model registry with per-model scale kind, default scale, max input images, prompt length, and output count limits.
- [x] Normalize 4.6 integer scale `[1,100]` separately from 4.0 float scale `[0,1]`.
- [x] Validate prompt length, image URL count/format, `n`, size area, width/height pair, aspect ratio, and unsupported mask inputs before submit.
- [x] Add focused tests for 4.0 mapping, scale normalization, and validation failures.
- [x] Re-run Go/TS verification.
- **Status:** complete

### Phase 2 Decisions
| Decision | Rationale |
|----------|-----------|
| Keep `jimeng-seedream-4.6` as the default/lead model and add 4.0 as an explicit selectable model | Official docs position 4.6 as the newer general image-generation/editing path while 4.0 remains useful compatibility coverage. |
| Build request validation before submit instead of relying on provider errors | It catches scale-type mismatches, unsupported data URLs/masks, invalid dimensions, and over-large `n` locally with clearer errors. |
| Accept OpenAI image `size` as either `WIDTHxHEIGHT` or Jimeng area integer | This keeps `/v1/images/generations` friendly to existing clients while still exposing Jimeng's native `size` area control. |
| Do not run a live Jimeng generation automatically | Real API calls may consume paid quota, so phase 2 verification remains local unless the user explicitly asks for a smoke test. |

## Current Task: Jimeng API Phase 3 Implementation (2026-06-22)

### Goal
Make the official `jimeng-api` adapter serve the Photo page editing capabilities so image edit, super-resolution, and outpainting flow through the official Volcengine Visual API instead of the CLI `jimeng` / custom `dreamina` adapters.

### Scope (user-confirmed)
- Capabilities: image edit (seedream 4.6/4.0) + `jimeng-superres` + `jimeng-outpaint`. Inpaint / material-extract / product-extract are deferred (need frontend mask + live field verification).
- Image input: support both `binary_data_base64` (Photo local files) and `image_urls`.
- No live smoke test this phase (avoid consuming paid quota).

### Phase 3: Photo Editing via Official API
- [ ] Add `jimeng-superres` / `jimeng-outpaint` model constants and registry specs (req_key, capability, scale/resolution semantics).
- [ ] Add `binary_data_base64`, `resolution`, directional `top/bottom/left/right`, `seed` to the submit request struct.
- [ ] Implement `CreateImageEditRequest` (seedream image edit with base64/URL inputs).
- [ ] Implement `CreateImageUpscaleRequest` (`jimeng-superres`, resolution + detail scale).
- [ ] Implement `CreateImageOutpaintRequest` (`jimeng-outpaint`, target-ratio → directional expansion fractions from decoded image dimensions).
- [ ] Register `jimeng-api` in `imageProcessorFactories` so edit/upscale/outpaint lookups resolve it.
- [ ] Match dreamina's chunk-content convention (`utils.GetImageMarkdown`).
- [ ] Add the new models to admin channel metadata and the runtime `config/config.yaml` channel + non-billing charge rules.
- [x] Add `jimeng-superres` / `jimeng-outpaint` model constants and registry specs (req_key, capability, scale/resolution semantics).
- [x] Add `binary_data_base64`, `resolution`, directional `top/bottom/left/right`, `seed` to the submit request struct.
- [x] Implement `CreateImageEditRequest` (seedream image edit with base64/URL inputs).
- [x] Implement `CreateImageUpscaleRequest` (`jimeng-superres`, resolution + detail scale).
- [x] Implement `CreateImageOutpaintRequest` (`jimeng-outpaint`, target-ratio → directional expansion fractions from decoded image dimensions).
- [x] Register `jimeng-api` in `imageProcessorFactories`.
- [x] Match dreamina's chunk-content convention (`utils.GetImageMarkdown`).
- [x] Add the new models to admin channel metadata and the runtime `config/config.yaml` channel + non-billing charge rules.
- [x] Add focused tests for edit input classification, superres resolution mapping, and outpaint ratio math.
- [x] Re-run Go/TS verification.
- **Status:** complete

### Phase 3 Follow-up (deferred, user-controlled)
- `config/prompts.json` still points Photo features at the CLI `jimeng-v2`. To route Photo through the official API, set the feature `model` (and `channel_type: jimeng-api`): edit-class features → `jimeng-seedream-4.6`, `hd_upscale` → `jimeng-superres`, `resize` → `jimeng-outpaint`. Left untouched because flipping it changes runtime Photo behavior and no live smoke test was run this phase.
- The `binary_data_base64` input field name for seedream edit is assumed from the standard Volcengine visual contract; confirm with one live edit call before relying on it in production.

### Phase 3 Decisions
| Decision | Rationale |
|----------|-----------|
| Reuse the seedream 4.6/4.0 `generate` specs for image edit | Official 4.x is a unified text-to-image/edit/compose model; edit is generation with input images, so no separate edit model is needed. |
| Implement one `ImageGenerator` struct satisfying edit/upscale/outpaint interfaces | The struct already holds endpoint/AK/SK and submit/poll/store helpers; adding methods avoids duplicating the transport. |
| Compute outpaint directional fractions locally from decoded image dimensions | The official outpaint contract takes `top/bottom/left/right` in `[0,1]`; the Photo page only supplies a target ratio, so the adapter derives expand-only padding. |
| Defer inpaint / extract | Inpaint needs a frontend mask and extract has an unverified field-name inconsistency; both are higher-risk and out of this phase's no-mask scope. |
