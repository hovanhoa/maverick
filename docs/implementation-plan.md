# Go AI Gateway Implementation Plan

This document captures a feature-by-feature, step-by-step implementation plan based on the suggested repository roadmap.

## Phase 0: Foundation Setup

### 1) `cmd/gateway` (Service Entrypoint)
1. Define runtime configuration structure (env vars + defaults) for DB, Redis, providers, auth, and observability.
2. Build bootstrap sequence: config -> logger -> telemetry -> storage -> services -> HTTP server.
3. Add graceful shutdown and health/readiness endpoints.
4. Add dependency wiring using interfaces (avoid tight coupling to concrete implementations).
5. Add startup validation for required settings and provider credential shape.

### 2) `internal/storage` (PostgreSQL/Redis)
1. Design initial schema: users, teams, api_keys, quotas, usage_events, audit_logs, requests.
2. Create migration strategy with versioned migrations.
3. Define repository interfaces first, then implement PostgreSQL and Redis backends.
4. Implement connection pooling, retry policy, and timeout handling.
5. Add Redis-backed patterns for rate limiting counters, window tracking, and idempotency keys.
6. Add tests for repository behavior and migration sanity.

## Phase 1: Security and Access Control

### 3) `internal/auth` (Authentication + Authorization + RBAC)
1. Decide MVP authentication mode (API key first; optional JWT later).
2. Define identity model (user/team/service account) and role model (admin/member/read-only).
3. Implement authentication middleware to verify credentials and attach principal to request context.
4. Implement authorization checks based on role + resource scope.
5. Add API key lifecycle flows: create, rotate, revoke.
6. Emit audit logs for authentication failures, denied access, and key lifecycle operations.
7. Add unit/integration tests for role allow/deny matrix.

### 4) `internal/policy` (Prompt/Data Policy Checks)
1. Define policy engine interfaces with pre-request, pre-log, and pre-response hooks.
2. Implement baseline policies:
   - blocked content patterns
   - sensitive data detection rules
   - prompt size/format guardrails
3. Add configurable policy actions: allow, redact, deny.
4. Ensure policy checks run before provider invocation.
5. Add policy decision logging with reason codes (without exposing sensitive raw content).
6. Add tests for pass/deny/redact coverage.

## Phase 2: Provider and API Core

### 5) `pkg/openai` (OpenAI-Compatible Models)
1. Define canonical request/response structs for chat/completions (including streaming chunks).
2. Add validation and normalization helpers.
3. Add mapper helpers between canonical models and provider-specific payloads.
4. Add compatibility tests with representative payload fixtures.

### 6) `internal/provider` (Abstraction + Adapters)
1. Define provider interface for non-streaming and streaming calls.
2. Implement adapter scaffolds for Claude, Bedrock, and Vertex AI.
3. Map canonical request/response to each provider format.
4. Standardize provider error taxonomy (auth, quota, timeout, transient, policy).
5. Implement retry/fallback contracts for MVP-safe behavior.
6. Add adapter tests with mocked provider clients/responses.

### 7) `internal/api` (HTTP Handlers + Routing)
1. Define primary endpoints (`/v1/chat/completions`, health, and minimal admin endpoints).
2. Build middleware order: auth -> request id -> rate/quota precheck -> policy -> handler -> logging.
3. Implement request parsing, validation, and OpenAI-compatible response shape.
4. Add streaming response flow using SSE/chunk forwarding.
5. Ensure consistent error envelope and HTTP status mapping.
6. Add integration tests from HTTP input through provider mocks.

## Phase 3: Governance, Limits, and Metering

### 8) `internal/quota` (Usage Limits + Enforcement)
1. Define quota dimensions (team/user/key/model/time window).
2. Implement pre-call quota checks with reservation semantics.
3. Implement post-call reconciliation with actual usage.
4. Use Redis for fast counters and PostgreSQL for durable records.
5. Add admin APIs for quota policy updates.
6. Add tests for edge cases and concurrent access.

### 9) `internal/usage` (Token/Cost Metering)
1. Define metering event schema: request id, principal, model, token counts, unit cost.
2. Implement provider-specific token/cost extraction and normalization.
3. Persist immutable usage events.
4. Add aggregation queries for daily totals by team/user/model/provider.
5. Expose basic usage reporting endpoints.
6. Validate metering accuracy with fixtures and edge-case tests.

## Phase 4: Observability and Operational Hardening

### 10) `internal/observability` (Logs, Traces, Metrics)
1. Define structured logging fields: request id, team, model, provider, latency, status.
2. Add OpenTelemetry tracing across request lifecycle and provider/storage calls.
3. Add metrics for throughput, latency, errors, quota denials, provider failures, and stream duration.
4. Add redaction layer for logs/traces to avoid secrets/PII leakage.
5. Build baseline dashboard/alerts for latency spikes and error-rate changes.
6. Run load tests and tune retries, timeouts, and connection pools.

## Recommended Delivery Order

1. `cmd/gateway` foundation + config + health checks
2. `internal/storage` + migrations
3. `internal/auth`
4. `pkg/openai` canonical models
5. `internal/provider` with one provider first, then expand
6. `internal/api` baseline non-streaming endpoint
7. `internal/policy` baseline checks
8. `internal/quota` enforcement
9. `internal/usage` metering and reporting
10. `internal/observability` deep instrumentation
11. End-to-end streaming support hardening
12. Stabilization: testing, security review, and docs

## MVP Exit Criteria

- Auth is enforced on all inference endpoints.
- Claude, Bedrock, and Vertex integrations are operational.
- OpenAI-compatible endpoint supports both non-stream and streaming flows.
- Policy checks run before provider calls.
- Quota enforcement blocks overages correctly.
- Usage and cost events are persisted and queryable.
- Request and audit logs are available with redaction.
- Core traces and metrics are available for operational visibility.
