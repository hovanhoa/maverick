# Maverick Implementation Plan

This document tracks the feature-by-feature roadmap. Status markers (`Done` /
`In progress` / `Planned`) reflect the actual state of the codebase, not the
original aspirational plan — update them as work lands.

## Phase 0: Foundation Setup — Done

### 1) `cmd/api` (Service Entrypoint) — Done
- Bootstrap sequence: connect Redis -> connect PostgreSQL -> run migrations -> start HTTP service (see [cmd/api/main.go](../cmd/api/main.go)).
- Gin HTTP service with `GET /ping`, `POST /graphql`, `:9090/metrics` (see [internal/http/](../internal/http/)).

### 2) `internal/db` (PostgreSQL/Redis) — Done for Account/Team/API key
- Schema so far: `account`, `team` — JSONB payload pattern (`id`, `<entity>` JSONB, `created_at`, `updated_at`); `api_key` — a normal relational table (`id`, `account_id`, `key_hash`, `prefix`, `created_at`, `revoked_at`).
- Migration system tracks state in `migrations_state` table (see [internal/db/migrations/](../internal/db/migrations/)).
- Repository layer built with Masterminds Squirrel (see [internal/db/account.go](../internal/db/account.go), [internal/db/team.go](../internal/db/team.go), [internal/db/apikey.go](../internal/db/apikey.go)).
- Still needed: `quota`, `usage_event`, `audit_log` tables (Phase 3/4).

## Phase 1: Account, Team, Role & Access Control — Done

Goal: every dev in the org gets an `Account` (optionally under a `Team`), with
a `Role` that governs what they can manage, and a self-issued API key that
authenticates their calls (both to the management GraphQL API and, from
Phase 2 onward, to the LLM proxy itself).

### 1a) Account & Team CRUD — Done
- GraphQL schema: [internal/schema/account.graphqls](../internal/schema/account.graphqls), [internal/schema/team.graphqls](../internal/schema/team.graphqls).
- Resolvers + db layer: [internal/api/account.go](../internal/api/account.go), [internal/api/team.go](../internal/api/team.go), [internal/db/account.go](../internal/db/account.go), [internal/db/team.go](../internal/db/team.go).
- Paginated list queries for both, with DataLoaders to avoid N+1.

### 1b) Role model — Done
1. Added `enum Role { OWNER ADMIN MEMBER }` and `role: Role!` on `Account` in [account.graphqls](../internal/schema/account.graphqls). Reused the gqlgen-generated `model.Role` type as the generic auth framework's `Role` type parameter.
2. `createAccount(..., role: Role)` (default `MEMBER`); `updateAccount(..., role: Role)` — see [internal/api/account.go](../internal/api/account.go), [internal/db/account.go](../internal/db/account.go).
3. Business rule enforced in the resolver layer (`requireRole` in [internal/api/authz.go](../internal/api/authz.go)): only `OWNER`/`ADMIN` can change another account's role, create an account with an elevated role, or delete accounts.
4. Team creator is auto-assigned `OWNER` for that team (`createTeam` in [internal/api/team.go](../internal/api/team.go)).
5. **Revised during the Phase 4 correctness pass** (see "RBAC: strict per-team scoping" under Phase 4) — the original decision here was "role is global on Account, not per-team," meaning any `OWNER`/`ADMIN` could manage *any* team or account, not just their own. That turned out to be a real authorization gap, not an intentional simplification, and was tightened to scope `OWNER`/`ADMIN` to the team they belong to. `Role` is still a single field on `Account` (unchanged), but holding it is no longer sufficient on its own for team/account-scoped operations - see the Phase 4 section for the full rationale and what's still intentionally unscoped.

### 1c) API key issuance — Done
1. New table `api_key(id, account_id, key_hash, prefix, created_at, revoked_at)` — stores a SHA-256 hash, never the plaintext secret (see [migrations](../internal/db/migrations/00001_api_key_table.up.sql)).
2. New schema [apikey.graphqls](../internal/schema/apikey.graphqls):
   - `createApiKey(accountId: ID!): ApiKeySecret!` — returns the plaintext key once, on creation only.
   - `apiKeys(accountId: ID!): [ApiKey!]!` — metadata only (id, prefix, createdAt, revokedAt).
   - `revokeApiKey(id: ID!): Boolean!`
3. db layer: [internal/db/apikey.go](../internal/db/apikey.go) — `CreateAPIKey`, `GetAccountByAPIKeyHash`, `ListAPIKeysByAccount`, `RevokeAPIKey`, `HashAPIKey`.
4. Decision: API key is the auth mechanism for both the management API (Phase 1) and the LLM proxy (Phase 2+) — no separate JWT/SSO flow for MVP.

### 1d) Wire RBAC into the request path — Done
1. Defined concrete `type Identity string` (`ACCOUNT`) in [internal/model/identity.go](../internal/model/identity.go), reusing the gqlgen-generated `model.Role` (`OWNER`/`ADMIN`/`MEMBER`) as the `Role` type parameter, on top of the existing generic framework in [pkg/core/auth/](../pkg/core/auth/) and [pkg/core/http/auth_middleware.go](../pkg/core/http/auth_middleware.go).
2. Implemented `Authorizer[Identity, Role]` in [internal/authz/authorizer.go](../internal/authz/authorizer.go): `GetPrincipalFromToken` treats the token as an API key -> hash -> lookup account -> build `Principal{ID, Roles: [account.Role], OrgID: account.TeamID}`. `GetPrincipalFromEmail` is a no-op (returns `nil, nil`) — the email-fallback branch doesn't apply to this project.
3. `AuthMiddleware` + `RequireAuth` are registered ahead of the `/graphql` route in [internal/http/service.go](../internal/http/service.go).
4. Since all GraphQL operations share one HTTP route, per-operation role checks live in the resolvers rather than as HTTP middleware: `requireRole`/`requireSelfOrRole` in [internal/api/authz.go](../internal/api/authz.go), applied to `deleteAccount`, `updateAccount` (when changing `role`), `createTeam`/`updateTeam`/`deleteTeam`, and `createApiKey`/`revokeApiKey` (when targeting another account).
5. Bootstrap: [cmd/seed](../cmd/seed/main.go) is a one-off CLI that seeds the first `OWNER` account + API key (logging the plaintext key once) so there's a first admin able to create everyone else. It's a no-op if an `OWNER` account already exists.

### 1e) Tests — Done
- [internal/db/apikey_test.go](../internal/db/apikey_test.go): hash/lookup/revoke correctness; revoked key fails auth.
- [internal/api/account_test.go](../internal/api/account_test.go), [team_test.go](../internal/api/team_test.go), [apikey_test.go](../internal/api/apikey_test.go): role-change/delete/create-team/API-key operations denied for non-OWNER/ADMIN callers, allowed for OWNER/ADMIN.
- [internal/authz/authorizer_test.go](../internal/authz/authorizer_test.go) and [internal/http/service_test.go](../internal/http/service_test.go): Authorizer and end-to-end middleware integration — valid key -> correct Principal/role; missing/invalid/revoked key -> 401.

## Phase 2: Virtual API Keys for the Proxy — Done (data model + CRUD; enforcement lands in Phase 3)

Reuses the `api_key` mechanism from 1c, scoped to the actual LLM proxy path
(not just the management API):
1. Devs configure their Cursor/agent with their personal or team API key pointed at the gateway base URL - no new mechanism needed, this is exactly the API key from Phase 1c.
2. Key -> account/team/role resolution reused from Phase 1's `Authorizer` as-is.
3. Per-team model allowlist (which providers/models a team's keys may call) - decided **per-team**, not per-key, matching the 1-account-to-0-or-1-team model:
   - `Team.modelAllowlist: [String!]!` - entries are `"provider:model"` (e.g. `"anthropic:*"`, `"openai:gpt-4o"`); empty means unrestricted (no allowlist configured yet). See [internal/schema/team.graphqls](../internal/schema/team.graphqls).
   - `updateTeamModelAllowlist(teamId, allowlist)` mutation - requires OWNER/ADMIN, same rule as other team management mutations.
   - `isModelAllowed(teamId, provider, model): Boolean!` query - a preview/test helper; pure matching logic lives in `(*model.Team).IsModelAllowed` ([internal/model/team.go](../internal/model/team.go)) so Phase 3's proxy path can call the same function to actually enforce it.
   - Decision: this phase only builds the data model and management API. There is no proxy endpoint yet to enforce against (`/v1/chat/completions` is Phase 3) - actual enforcement wires in when that endpoint exists.

## Phase 3: Provider and API Core — Done (Claude/OpenAI/Gemini live; Bedrock/Vertex scaffolded)

Scope decision going in: build full, tested adapters for the three API-key-based
providers (Claude, OpenAI, Gemini) and scaffold Bedrock/Vertex AI as
not-yet-implemented (they need AWS SigV4 and GCP OAuth2 respectively, a
materially bigger lift than an API key) - see [internal/provider/bedrock](../internal/provider/bedrock/adapter.go)
and [internal/provider/vertexai](../internal/provider/vertexai/adapter.go).

### `pkg/openai` (OpenAI-Compatible Models) - Done
1. Canonical request/response structs for chat/completions, incl. streaming chunks ([types.go](../pkg/openai/types.go)).
2. `(*ChatCompletionRequest).Validate()` checks model/messages/roles/temperature/top_p/max_tokens ([validate.go](../pkg/openai/validate.go)).
3. OpenAI-compatible error envelope (`ErrorResponse`/`ErrorType`) so clients written against OpenAI's SDKs work unmodified ([error.go](../pkg/openai/error.go)).
4. Tests: JSON round-trips and validation edge cases.

### `internal/provider` (Abstraction + Adapters) - Done
1. `Provider` interface (`ChatCompletion` + `StreamChatCompletion`) and a `Registry` keyed by provider name ([provider.go](../internal/provider/provider.go)).
2. Adapters: [anthropic](../internal/provider/anthropic/adapter.go) (Messages API), [openai](../internal/provider/openai/adapter.go) (near pass-through, since the canonical types mirror OpenAI's wire format), [gemini](../internal/provider/gemini/adapter.go) (generateContent/streamGenerateContent, role/system-instruction mapping). [bedrock](../internal/provider/bedrock/adapter.go) and [vertexai](../internal/provider/vertexai/adapter.go) are scaffolds that satisfy `Provider` but return a clear "not implemented" error.
3. Standardized error taxonomy - `ErrorKind` (auth/quota/timeout/transient/policy/invalid_request/unknown) wrapped in `*provider.Error` ([error.go](../internal/provider/error.go)).
4. `provider.WithRetry` retries only `ErrorKindTransient` failures with exponential backoff ([retry.go](../internal/provider/retry.go)).
5. Tests: each real adapter is tested against an `httptest` mock server (request mapping, response mapping, streaming chunk forwarding, and HTTP-status-to-ErrorKind mapping) - no live provider credentials needed to run the suite.

### `internal/proxy` + `internal/http` proxy endpoint - Done
1. `POST /v1/chat/completions` (OpenAI-compatible), registered in [internal/http/service.go](../internal/http/service.go) behind the same API-key `AuthMiddleware`/`RequireAuth` as the management API (Phase 1/2's `Authorizer`, reused as-is).
2. Model routing: the request's `model` field is `"provider/model"` (e.g. `"anthropic/claude-3-5-sonnet-20241022"`); [internal/proxy/proxy.go](../internal/proxy/proxy.go) splits it, resolves the provider from the `Registry`, and strips the prefix before calling the adapter.
3. Phase 2's model allowlist is enforced here: if the caller's account has a team, `team.IsModelAllowed(provider, model)` gates the call (a `policy`-kind error, mapped to 403) before any upstream request is made. No team means unrestricted, per the Phase 2 decision.
4. Streaming response flow via real SSE: chunks are forwarded as `data: {...}\n\n`, terminated with `data: [DONE]\n\n`, flushed per chunk.
5. Consistent error envelope: `proxy.ErrorResponseFor` maps `provider.ErrorKind` to both an HTTP status and an OpenAI-compatible `ErrorResponse` body.
6. Request-id propagation, quota prechecks, and content policy hooks landed in Phase 4 below.
7. Provider registry is built once in [cmd/api/providers.go](../cmd/api/providers.go) from optional `ANTHROPIC_API_KEY`/`OPENAI_API_KEY`/`GEMINI_API_KEY` env vars - a provider is simply absent (not present-but-erroring) when unconfigured.
8. Tests: full HTTP-level integration tests (auth required, non-streaming success with provider-prefix stripping, invalid model format, team-allowlist blocking, and real SSE streaming through the router) using a stub `Provider` - no live provider calls.

## Phase 4: Governance, Limits, and Metering — Done

### `internal/quota`
1. Scoped to exactly one quota dimension - **per-team, calendar-month token budget** - rather than the full team/account/key/model/window matrix the plan sketched. This composes with the Phase 2 per-team model allowlist and the `Team.MonthlyTokenBudget *int` field mirrors that phase's `nil = unlimited` convention. Other dimensions can reuse the same `windowKey`-per-counter pattern later if needed.
2. Pre-call reservation via `Checker.Reserve` (atomic `KVStore.GetAndSet` against a Redis counter keyed `quota:<teamID>:<yyyy-mm>`); post-call `Checker.Reconcile` adjusts the reservation to actual usage (delta-based, floored at 0) once the provider responds, or fully releases it (`actual=0`) on any failure/deny before dispatch. `Reconcile` takes the same `budget` and the exact `window` string `Reserve` returned, so it's a no-op for unlimited teams (mirroring `Reserve`) and always adjusts the calendar-month counter the reservation actually landed in, even if the call straddles a month boundary.
3. Pre-call token estimate (`quota.EstimateTokens`, in [internal/quota/estimate.go](../internal/quota/estimate.go)): prompt chars/4 heuristic + `max_tokens` or a 1000-token default - deliberately rough, corrected by `Reconcile` once real usage is known.
4. `updateTeamQuota(teamId, monthlyTokenBudget, clearMonthlyTokenBudget)` GraphQL mutation (owner/admin only, same `requireRole` pattern as the model allowlist mutations) is the admin API for quota policy updates; no separate REST endpoint.
5. Known limitation, documented in code: streaming calls only reserve-and-release-on-error - none of the three live provider adapters reliably surface a final usage figure over SSE, so a successful stream keeps its upfront estimate reserved rather than reconciling to a real number.
6. `pkg/driver/redis`'s `GetAndSet` (part of the pre-existing driver interface, but with no real caller before this phase) had a latent bug: its `WATCH` never actually guarded anything, because the write ran via `t.Set` directly instead of through `t.TxPipelined` - so two concurrent callers could both read the same stale counter value and both be admitted, silently defeating the reservation. Fixed to send the write through `TxPipelined` (so `WATCH`+`EXEC` actually detects the conflict) with a short jittered-backoff retry loop on `redis.TxFailedErr`. Verified with a throwaway 100-goroutine concurrent-increment script against the real dev Redis: before the fix this was capable of losing updates; after, updates are never lost (a handful of callers can still get a hard error under extreme same-key contention, which is the correct trade-off for optimistic locking, not a correctness bug).

### `internal/usage`
1. `usage_event` table (migration `00002_usage_event_table`): request id, account id (nullable), team id (nullable), provider, model, prompt/completion/total tokens, cost, timestamp - append-only, durable record of every completed non-streaming call. Both FKs are `ON DELETE SET NULL`, not `CASCADE` or left as the default `NO ACTION`: usage_event is a billing/audit trail, so deleting an account or team should orphan its past usage rows (keeping the historical record) rather than erasing that history or blocking the delete outright with a raw FK-violation error.
2. `internal/usage/pricing.go` provides `CalculateCost(provider, model, promptTokens, completionTokens)` against a small hardcoded per-model $/1M-token table (Anthropic/OpenAI/Gemini's current flagship + mini/haiku models); unknown provider/model pairs cost $0 rather than erroring - illustrative/approximate pricing, not billing-grade.
3. `teamUsage(teamId, since)` GraphQL query returns a `UsageSummary` (request count, prompt/completion/total tokens, cost) aggregated via `SumTeamUsage`'s `COALESCE(SUM(...), 0)` query; defaults to the current calendar month when `since` is omitted.
4. Metering never blocks or fails the user-facing response: `RecordUsageEvent` errors are logged and swallowed in `proxy.recordUsage`, consistent with `usage.CalculateCost`'s $0 fallback philosophy.

### `internal/policy`
1. `policy.Chain` runs an ordered list of `Rule`s before every provider call; the first `Deny` short-circuits (releasing any quota reservation already made), `Redact` decisions accumulate against a working copy of the request, and rules never mutate the caller's original request in place.
2. Baseline rules ([internal/policy/rules.go](../internal/policy/rules.go)): `MaxPromptLength` (deny over a char budget), `BlockedPatterns` (case-insensitive substring deny), `SensitiveDataRedaction` (regex-based redaction of gateway/OpenAI-shaped API keys and card-like numbers to `[REDACTED]`) - intentionally simple pattern/length checks illustrating the hook structure, not a production moderation system.
3. `policy.DefaultChain(...)` (wired in [cmd/api/main.go](../cmd/api/main.go)) composes `MaxPromptLength` + `BlockedPatterns` + `SensitiveDataRedaction` in that order.
4. Every non-Allow decision is logged with only `request_id`, `action`, and `reason_code` (via `proxy.logPolicyDecision`) - raw request content is never logged, per the plan's requirement.

### Wiring (`internal/proxy`, `internal/http`)
1. Request order inside `proxy.Handler.prepare`: validate → resolve provider/model → team allowlist → quota reserve → policy evaluate → dispatch (cheapest checks first, so a policy or quota rejection never reaches a provider).
2. `internal/http/request_id.go` adds a request-id middleware ahead of the chat route (`X-Request-Id` echoed if the client sent one, otherwise generated); the id flows through to policy-decision logging and the persisted `usage_event` row.
3. Tests: `internal/quota`, `internal/usage`, `internal/policy` each have full unit coverage in isolation (memkv-backed for quota); `internal/proxy` adds integration-level tests exercising quota-exceeded blocking, quota reconciliation to actual usage, policy-deny blocking + quota release, policy redaction reaching the provider, and usage-event persistence, on top of the existing Phase 3 routing/allowlist/retry tests.

### RBAC: strict per-team scoping — Done
A correctness pass on Phase 4 (adding `teamUsage`, which exposes cost/spend data) surfaced that the *existing* Phase 1-3 authorization design let this go further than intended: `requireRole(ctx, OWNER, ADMIN)` alone was used to gate every team- and account-scoped mutation (`updateTeam`, `deleteTeam`, `updateTeamModelAllowlist`, `updateAccount`'s role change, `deleteAccount`, API key issuance/revocation for another account), and it only checks *that the caller holds the role somewhere*, never *for which team*. An `OWNER` of Team A could rename, delete, or reconfigure Team B, or manage Team B's accounts and API keys, purely by holding the role - and the newly-added `teamUsage`/`getTeam`/`isModelAllowed` reads had no check at all.

1. Fixed by adding `requireTeamMember`/`requireTeamRole`/`requireSelfOrTeamRole` in [internal/api/authz.go](../internal/api/authz.go), built on `Principal.BelongsToOrg` (already present in `pkg/core/auth`, just never wired up by any resolver). `requireTeamMember` checks `principal.OrgID == teamID`; `requireTeamRole` additionally requires an `OWNER`/`ADMIN` role.
2. Applied to: `getTeam`/`isModelAllowed`/`teamUsage` (read - any team member) and `updateTeam`/`deleteTeam`/`updateTeamModelAllowlist`/`updateTeamQuota` (write - `OWNER`/`ADMIN` of *that* team) in [internal/api/team.go](../internal/api/team.go)/[internal/api/usage.go](../internal/api/usage.go); and, for the same reason, `createAccount` (elevated role), `updateAccount` (role change), `deleteAccount` in [internal/api/account.go](../internal/api/account.go), and `createAPIKey`/`listAPIKeys`/`revokeAPIKey` in [internal/api/apikey.go](../internal/api/apikey.go) - all now scoped by the *target* account's current team, falling back to a plain role check only when the target has no team (mirrors `createTeam`'s bootstrap-time behavior, where there's no team yet to scope against).
3. `createTeam` is intentionally unchanged (`requireRole` only) - there's no existing team to scope a *creation* against.
4. **Closed in a follow-up pass** (was previously deferred - see below): `listTeams`/`getAccount`/`listAccounts` no longer return platform-wide data to any authenticated caller.
   - `listTeams` returns only the team the caller belongs to (0 or 1 today - an account belongs to at most one team), never every team on the platform.
   - `getAccount` is readable by the account itself, or by any member of the same team (read access within a team isn't role-gated, only membership) - an unaffiliated target (no team) stays readable by anyone, since there's no boundary to enforce.
   - `listAccounts` requires membership in an explicitly-given `teamId`; omitting it defaults to the caller's own team's roster, or - for an unaffiliated caller with no team roster to default to - just their own account (not an empty page that omits even themselves).
   - There is still no "platform admin" concept (deliberately, per the RBAC redesign above), so an Owner/Admin only ever sees their own team's roster now, not a directory of every team/account. The web console's Teams/Accounts pages needed no code changes for this (they don't assume more than one team), only a copy update ([AccountsPage.tsx](../web/src/pages/AccountsPage.tsx)) since "every account" was no longer accurate. Verified live and via Playwright: two unrelated Owners each see only their own team/roster.
   - Separately fixed a real bug this surfaced during review: `updateAccount` ran *no* authorization check at all when the caller wasn't changing role (only email/username/team) - any authenticated account, even from a different team, could edit any other account's profile. Now requires self (for email/username only) or `OWNER`/`ADMIN` of the target's team (for anything else, including changing your *own* team membership, which is a privileged action, not a cosmetic self-edit). Verified live against a real server.
5. Test impact: this reverses a Phase 1 design decision (§1b.5) that a large share of the existing Phase 1-3 test suite was written against - `testResolver`'s default principal is deliberately unaffiliated with any team ("RBAC-exempt", see its doc comment in [internal/api/account_test.go](../internal/api/account_test.go)), and several `_AllowedForAdmin`-style tests had to be updated to scope their test principal to the team under test via `asPrincipal(ctx, accountID, role, teamID)`. New `_DeniedForAdminOfAnotherTeam`/`_DeniedForNonMember` tests assert the actual cross-team denial, not just that the old tests still pass. Verified live end-to-end too: two real accounts, each creating and owning their own team via the real GraphQL API, confirmed same-team access allowed and cross-team access denied (`forbidden: not a member of this team`) for `team`, `updateTeamQuota`, and `teamUsage`.

## Phase 5: Observability and Operational Hardening — Done

Before implementing, audited what `pkg/core/http`/`pkg/core/apm` already provided (this project sits on a shared Go framework with existing Sentry/Prometheus wiring) rather than building parallel infrastructure - most of this phase turned out to be *wiring this gateway into instrumentation that already existed*, plus closing two concrete gaps (request id never reached logs; request/response bodies, including raw LLM prompts and completions, were logged verbatim).

1. **Structured logging (request id, team, account, model, provider, latency, status)**:
   - `internal/http/observability.go` (renamed from `request_id.go`, now used on *both* the GraphQL and chat routers, not just chat): assigns/propagates the request id and attaches `requestId`/`accountId`/`teamId` to every request's canonical log line via `RequestExtraFieldsContextKey`.
   - `internal/proxy/proxy.go`'s new `logCompletion` emits one `"chat_completion"` log line per proxy call with exactly the fields this item asks for (request id, account, team, provider, model, latency, status), on both the success and every failure path of `ChatCompletion`/`StreamChatCompletion`.
2. **OpenTelemetry tracing across request lifecycle and provider/storage calls**: request-lifecycle tracing already existed (`sentrygin` middleware in `pkg/core/http/service.go`, Sentry bridged to the OTel tracer provider in `pkg/core/apm/sentry.go`), and so did storage-call tracing (`pkg/driver/postgres`'s `TracingRunner`, `pkg/driver/redis`'s tracing hook) - both pre-existing, unrelated to this phase. The one real gap was **provider calls**, which used a plain uninstrumented `http.Client`. Closed by switching `cmd/api/providers.go` to `pkg/core/http.NewInstrumentedClient(providerName, ...)`, which gives every provider call a Sentry/OTel span for free.
   - **Caught by an independent review pass, fixed before this landed**: `NewInstrumentedClient`'s transport unconditionally `io.ReadAll(resp.Body)`'d the *entire* response before returning from `RoundTrip`, discarding any read error. That's fine for the JSON-API callers this helper originally existed for, but wiring the 3 live LLM adapters through it (this phase, above) would have silently broken `StreamChatCompletion` for all of them: each adapter's `pumpStream` reads `resp.Body` incrementally *after* `client.Do()` returns (see `internal/provider/anthropic/adapter.go`'s `StreamChatCompletion`), so a transport that buffers the whole body first means `Do()` doesn't return until the entire generation has finished streaming - no incremental delivery at all - and any stream that outlived the client's `Timeout` would have its read error silently discarded, turning a real timeout into a truncated-but-"successful" response everywhere (metrics, logs, and the client's own SSE output). Fixed by detecting `Content-Type: text/event-stream` and passing that body straight through unbuffered/unlogged; regression-tested with a test server that blocks on writing its second chunk until the client has already read the first (`TestNewInstrumentedClient_StreamsSSEResponseWithoutBuffering`), which would hang/timeout against the old code.
3. **Metrics (throughput, latency, errors, quota denials, provider failures, stream duration)**: generic throughput/latency/error metrics for this gateway's own routes already existed (`core_http_request_count`/`core_http_request_duration_seconds`, labeled by method/path/status). `NewInstrumentedClient` (see above) adds the same for *provider* calls (`core_http_client_request_total`/`_duration_seconds`, labeled by `external_service`=provider name), closing "provider failures" without new code. New in this phase: `internal/proxy/metrics.go`'s `llmgateway_quota_denied_total` (labeled `team_id` - bounded, server-generated) and `llmgateway_proxy_stream_duration_seconds` (labeled `provider`/`status` only - **not** `model`, which the same review pass flagged as an unbounded/caller-controlled label: `modelName` is whatever the caller put after the `/` in `model: "provider/model"`, unchecked against any known list when a team's allowlist is empty, so it isn't safe as a Prometheus label).
4. **Redaction layer for logs**: found and fixed a real, live leak - the shared canonical request logger (`pkg/core/http/middleware.go`'s `RequestLogger`) wrote `Authorization`/`Cookie` header values to logs verbatim on *every* request, meaning every API key used against this gateway was in plaintext in the logs. Fixed generically (`redactHeaders`, replaces the value with `"[REDACTED]"`) since this is shared framework code other consumers rely on too. Separately, `RequestLogger`/`NewInstrumentedClient` both log full JSON request/response bodies by default, which for this gateway means raw LLM prompts and completions - added `WithBodyLogDropper` (suppresses body fields for a route, keeps the rest of the canonical line) and `WithoutBodyLogging` (same idea for `NewInstrumentedClient`), and wired both for `/v1/chat/completions` and the provider clients respectively.
5. **Load tests; tune retries/timeouts/pools**: `scripts/loadtest/gateway.js` (k6) load-tests this gateway's own overhead (auth, RBAC, quota/policy prechecks, DB reads) without calling a real provider - see [scripts/loadtest/README.md](../scripts/loadtest/README.md) for usage and what to look at if numbers regress. Verified locally (5 VUs / 8s against the dev stack: p95 ≈2.5ms, 0% check failures). Pool/timeout tuning was scoped conservatively: `pkg/driver/postgres`/`pkg/driver/redis` are shared framework code used beyond this project, so rather than guess new pool-size numbers with no load-test evidence behind them, only the two *unambiguously missing* safety nets were added - a Postgres connect timeout + max connection lifetime, and a Redis max connection age - both previously unset (unbounded), which risks silently stale connections behind a load balancer. Retry backoff (`internal/provider/retry.go`) was already tuned in Phase 3/4 and left as-is.
6. **Unrelated bug found via `go test -race` while verifying this phase, fixed anyway**: `pkg/core/http`'s `Service.Start()` assigns a default `healthFn` ~500ms after the server starts accepting connections, and `GracefulStop` reassigns `healthFn`/`isStopping` from the shutdown path - both racing, unsynchronized, against the `/healthz` handler reading them on every request. Pre-existing, unrelated to any project phase, but a real data race nonetheless; fixed with a `sync.RWMutex` guarding both fields (`Service.getHealthFn`/`setHealthFn`/`setStopping` in [pkg/core/http/service.go](../pkg/core/http/service.go)), which also required changing `Router()`/`Handler()` from value to pointer receivers (`go vet`'s `copylocks` check correctly flagged copying a struct that now contains a mutex). Verified with `go test -race ./pkg/core/http/...`.

## Phase 6: Login, Avatars, Request Log, Playground, Usage Dashboard — Done

### Username/password login — Done
1. `account.password_hash` column (migration `00005_account_password`), set via `db.HashPassword`/`SetAccountPassword`/`SetRandomAccountPassword` (bcrypt, mirrors the API-key convention of never persisting the plaintext) — see [internal/db/password.go](../internal/db/password.go).
2. `POST /login` (not a GraphQL mutation — the whole `/graphql` route already requires an API key, which is unusable for the one request that must work before the caller has a key): verifies username/password via `db.VerifyAccountPassword`, then mints and returns a fresh API key exactly like `createApiKey` would. Old keys from earlier logins remain valid until explicitly revoked. See [internal/http/login.go](../internal/http/login.go).
3. `VerifyAccountPassword` intentionally collapses "no such username," "no password set," and "wrong password" into one `nil, nil` result so the caller can't distinguish them — avoids username enumeration.
4. Web console: [auth.tsx](../web/src/lib/auth.tsx) now offers a username/password form as the default login path, falling back to pasting a raw API key.

### Account avatars — Done
1. `account_avatar(account_id, content_type, data, updated_at)` table (migration `00006_account_avatar_table`); stores image bytes base64-encoded as `TEXT` rather than `BYTEA` (see the migration's doc comment). Capped at `db.MaxAvatarBytes` (2 MiB) to keep the table bounded. See [internal/db/avatar.go](../internal/db/avatar.go).
2. Plain REST, not GraphQL, since it moves raw image bytes: `GET /accounts/:id/avatar` (deliberately unauthenticated — an `<img>` tag can't attach an `Authorization` header, and a profile picture isn't sensitive), `POST`/`DELETE /accounts/:id/avatar` (self-service only, no OWNER/ADMIN-on-behalf-of path, unlike `updateAccount`). See [internal/http/avatar.go](../internal/http/avatar.go), registered in [internal/http/service.go](../internal/http/service.go).

### Request log audit trail — Done
1. `request_log` table (migration `00003_request_log_table`) — one row per proxy call (request id, account, team, provider, model, status, timing), persisted by `proxy.recordRequestLog` alongside the existing `usage_event` write. See [internal/db/request_log.go](../internal/db/request_log.go), [internal/proxy/proxy.go](../internal/proxy/proxy.go).
2. Three GraphQL queries at different authorization tiers, mirroring the `usage` queries' tiering: `myRequestLogs` (any authenticated caller, own history only), `teamRequestLogs` (OWNER/ADMIN of that team — stricter than `teamUsage`'s member-readable tier, since this exposes every member's raw prompt/response content, not just aggregate numbers), `globalRequestLogs` (platform OWNER/ADMIN only). See [internal/api/requestlog.go](../internal/api/requestlog.go), [internal/schema/requestlog.graphqls](../internal/schema/requestlog.graphqls).
3. `api_key.last_used_at` column added in the same wave (migration `00004_api_key_last_used`) so a team's API Keys page can show staleness, not just creation date.

### Playground — Done
- [PlaygroundPage.tsx](../web/src/pages/PlaygroundPage.tsx): a web-console page that calls the caller's own gateway (`POST /v1/chat/completions`) with their session key, for interactively trying providers/models and seeing the request/response shape without curl.

### Usage dashboard — Done
- [UsagePage.tsx](../web/src/pages/UsagePage.tsx) + [usage.ts](../web/src/lib/usage.ts): charts (via [Chart.tsx](../web/src/components/ui/Chart.tsx)) over the `teamUsageByAccount`/`teamUsageByModel`/`teamUsageDaily` queries added in Phase 4's usage work, previously only reachable via raw GraphQL.

### Rebrand — Done
- Web console renamed to **Maverick**; new favicon/logo/manifest set under [web/public/](../web/public/), unused theme-toggle component and dead theme lib removed.

## MVP Exit Criteria

- Auth (API key) is enforced on all management and inference endpoints.
- Role-based access control governs account/team management operations.
- Claude, OpenAI, Gemini, Bedrock, and Vertex integrations are operational.
- OpenAI-compatible endpoint supports both non-streaming and streaming flows.
- Policy checks run before provider calls.
- Quota enforcement blocks overages correctly.
- Usage and cost events are persisted and queryable.
- Request and audit logs are available with redaction.
- Core traces and metrics are available for operational visibility.
