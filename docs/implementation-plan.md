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

## Phase 3: Provider and API Core — Planned

### `pkg/openai` (OpenAI-Compatible Models)
1. Canonical request/response structs for chat/completions (incl. streaming chunks).
2. Validation/normalization helpers; mappers to provider-specific payloads.
3. Compatibility tests with representative payload fixtures.

### `internal/provider` (Abstraction + Adapters)
1. Provider interface for non-streaming and streaming calls.
2. Adapter scaffolds for Claude, OpenAI, Gemini, Bedrock, Vertex AI.
3. Standardized provider error taxonomy (auth, quota, timeout, transient, policy).
4. Retry/fallback contracts; adapter tests with mocked provider clients.

### `internal/api` proxy endpoints
1. `/v1/chat/completions` (OpenAI-compatible) plus minimal admin endpoints.
2. Middleware order: auth -> request id -> rate/quota precheck -> policy -> handler -> logging.
3. Streaming response flow via SSE/chunk forwarding.
4. Consistent error envelope and HTTP status mapping.

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
