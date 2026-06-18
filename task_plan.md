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
