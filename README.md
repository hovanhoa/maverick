# Maverick

An AI Gateway platform that centralizes, secures, and observes LLM usage
across an engineering organization. Maverick sits between your teams' tools
(Claude Code, Cursor, CI bots, internal apps) and the LLM providers they call,
giving you one place for authentication, governance, usage tracking, and
multi-provider routing.

## Why This Project Exists

Teams adopt AI tools quickly - IDE extensions, CI bots, internal apps. Without
a shared gateway, usage becomes fragmented, hard to govern, and hard to
optimize for cost. Maverick provides a single control plane and data plane for
LLM access so an organization can:

- **Enforce security and policy controls** - centralized request/response filtering and redaction
- **Track usage and cost transparently** - token-level metering across all providers
- **Manage quotas and permissions** - per-team monthly token budgets and model allowlists
- **Route traffic across providers** - one OpenAI-compatible endpoint in front of multiple backends
- **Maintain audit trails** - structured logs, per-request logs, and traces for every call

## Quick Start

### Prerequisites
- Go 1.25+
- PostgreSQL 14+
- Redis 7+
- Node.js 18+ (for the web console)

### Setup

```bash
cp deployment/.env.example deployment/.env
make docker-up    # Start PostgreSQL + Redis
make run          # Start the API on :8080

# First-time only: seed the first OWNER account + API key
make seed
```

The API is available at `http://localhost:8080` (GraphQL at `/graphql`,
health at `/ping`, metrics at `:9090/metrics`). To run the web console:

```bash
make web-install
make web-dev      # http://localhost:5173
```

## What's Implemented

### Access Control
- API-key and username/password authentication
- Role-based access control (`OWNER`/`ADMIN`/`MEMBER`), scoped per-team
- Account avatar upload/serving
- Team management, with a per-team model allowlist

### LLM Proxy
- `POST /v1/chat/completions`, OpenAI-compatible request/response shape,
  including SSE streaming
- Model routing via `"provider/model"` (e.g. `anthropic/claude-3-5-sonnet-20241022`)
- Live provider adapters: **Anthropic**, **OpenAI**, **Gemini** (API-key based).
  Bedrock and Vertex AI are scaffolded but not yet implemented (they need AWS
  SigV4 / GCP OAuth2, not just an API key)
- A web-based playground for trying the proxy interactively

### Governance & Metering
- Per-team, calendar-month token quota with pre-call reservation and post-call reconciliation
- Policy chain (prompt length limits, blocked patterns, sensitive-data redaction) evaluated before every provider call
- Usage/cost tracking per team, with a web dashboard
- Per-request audit log (request logs page in the web console)

### Observability
- Structured logs with request id, account, team, provider, model, latency, status
- OpenTelemetry tracing across the request lifecycle and provider/storage calls
- Prometheus metrics, including quota-denial and stream-duration counters
- Header/body redaction so API keys and raw prompts never land in logs verbatim

See [docs/implementation-plan.md](docs/implementation-plan.md) for the full
phase-by-phase history and design rationale behind each of these.

## Project Structure

```
.
├── cmd/
│   ├── api/              # Service entrypoint: connects DB/Redis, runs migrations, starts HTTP
│   └── seed/             # One-off CLI to seed the first OWNER account + API key
├── internal/
│   ├── api/              # GraphQL resolvers (account, team, apikey, usage, requestlog)
│   ├── authz/            # RBAC: Authorizer + Principal wiring
│   ├── db/               # Repository layer (Postgres via Squirrel) + migrations
│   ├── http/             # Gin HTTP service: routes, auth middleware, login, avatar
│   ├── model/            # gqlgen-generated models + hand-written domain types
│   ├── policy/           # Pre-call policy chain (length limits, redaction, blocklists)
│   ├── provider/         # Provider abstraction + adapters (anthropic/openai/gemini/bedrock/vertexai)
│   ├── proxy/            # POST /v1/chat/completions handler
│   ├── quota/            # Per-team monthly token budget enforcement
│   ├── schema/            # GraphQL schema (.graphqls), source of truth for codegen
│   └── usage/             # Cost calculation + usage_event persistence
├── pkg/
│   ├── openai/           # OpenAI-compatible request/response types
│   ├── core/             # Shared framework: auth, log, apm, encoding, http
│   └── driver/           # SQLStore (pgx) / KVStore (go-redis) interfaces + implementations
├── web/                  # React/TypeScript management console
├── deployment/           # Docker Compose + env config
├── scripts/              # Codegen scripts + k6 load test
└── docs/                 # Implementation plan / design notes
```

## Development

```bash
make lint             # fmt + vet
make test             # Run all tests
make test-race        # Tests with race detector
make build             # Build to ./bin/api
make generate          # Regenerate GraphQL code (gqlgen + openapi)
make docker-down
make docker-logs
```

Run a single test:
```bash
go test ./internal/db/... -run TestMigrate
```

### Web console

```bash
make web-install
make web-dev      # Development with hot reload, :5173
make web-build    # Production build
make web-lint     # Type-check
```

## Architecture

```
HTTP Request -> Gin middleware (CORS, logging, Sentry)
             -> /graphql (management API) or /v1/chat/completions (LLM proxy)
             -> Auth middleware (API key or session) -> RBAC resolver checks
             -> internal/api or internal/proxy -> internal/db / provider adapters
             -> PostgreSQL (accounts, teams, usage, request logs)
                Redis (quota counters)
                Anthropic / OpenAI / Gemini (live) / Bedrock / Vertex AI (scaffolded)
```

## Configuration

Key environment variables (see [deployment/.env.example](deployment/.env.example)):

- `ENVIRONMENT` - `dev`/`staging`/`production` (affects logging format and code paths)
- `DB_HOST`/`DB_PORT`/`DB_USER`/`DB_PASS`/`DB_NAME` - PostgreSQL connection
- `REDIS_HOST`/`REDIS_PORT`/`REDIS_PASS` - Redis connection
- `ANTHROPIC_API_KEY`/`OPENAI_API_KEY`/`GEMINI_API_KEY` - optional; a provider is only registered when its key is set

## Testing

Test DB utilities live in `internal/db/testdb/`. Integration tests use real
PostgreSQL/Redis connections, not mocks - this is intentional to catch real
migration/driver issues.

## Project Status

Early production (MVP). Phases 1-6 (accounts/teams/RBAC, the LLM proxy,
governance/metering, observability, and the login/avatar/playground/usage
console) are done - see [docs/implementation-plan.md](docs/implementation-plan.md)
for what's next.
