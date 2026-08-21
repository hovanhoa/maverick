# Go AI Gateway Implementation Plan

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
5. Decision: role is **global on Account**, not per-team — matches the current 1-account-to-0-or-1-team model. Revisit if accounts ever need to belong to multiple teams.

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
6. Deferred to Phase 4 (doesn't exist yet, so not part of this phase's middleware chain): request-id propagation, rate/quota prechecks, and content policy hooks.
7. Provider registry is built once in [cmd/api/providers.go](../cmd/api/providers.go) from optional `ANTHROPIC_API_KEY`/`OPENAI_API_KEY`/`GEMINI_API_KEY` env vars - a provider is simply absent (not present-but-erroring) when unconfigured.
8. Tests: full HTTP-level integration tests (auth required, non-streaming success with provider-prefix stripping, invalid model format, team-allowlist blocking, and real SSE streaming through the router) using a stub `Provider` - no live provider calls.

## Phase 4: Governance, Limits, and Metering — Planned

### `internal/quota`
1. Quota dimensions (team/account/key/model/time window).
2. Pre-call quota checks with reservation semantics; post-call reconciliation.
3. Redis for fast counters, PostgreSQL for durable records.
4. Admin APIs for quota policy updates.

### `internal/usage`
1. Metering event schema: request id, principal, model, token counts, unit cost.
2. Provider-specific token/cost extraction and normalization.
3. Immutable usage events; aggregation queries by team/account/model/provider.
4. Basic usage reporting endpoints.

### `internal/policy`
1. Pre-request/pre-log/pre-response policy hooks.
2. Baseline policies: blocked content patterns, sensitive data detection, prompt size/format guardrails.
3. Configurable actions: allow, redact, deny — enforced before provider invocation.
4. Policy decision logging with reason codes (no raw sensitive content).

## Phase 5: Observability and Operational Hardening — Planned

1. Structured logging fields: request id, team, account, model, provider, latency, status.
2. OpenTelemetry tracing across request lifecycle and provider/storage calls (Sentry already wired for errors — see CLAUDE.md).
3. Metrics for throughput, latency, errors, quota denials, provider failures, stream duration.
4. Redaction layer for logs/traces to avoid secret/PII leakage.
5. Load tests; tune retries, timeouts, connection pools.

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
