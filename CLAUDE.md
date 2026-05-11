# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Project Is

An AI Gateway platform that centralizes, secures, and observes LLM usage across an engineering organization. It provides centralized governance, policy enforcement, usage tracking, quota management, and multi-provider routing (Claude, Bedrock, Vertex AI, OpenAI).

**Stack:** Go, PostgreSQL, Redis, Gin, GraphQL (gqlgen), OpenTelemetry, Docker

## Commands

```bash
# Local dev setup
cp deployment/.env.example deployment/.env
make docker-up        # Start PostgreSQL + Redis
make run              # Start API on :8080

# Development
make lint             # fmt + vet
make test             # Run all tests
make test-race        # Tests with race detector
make build            # Build to ./bin/api
make generate         # Regenerate GraphQL code (gqlgen + openapi)

# Docker
make docker-down
make docker-logs
```

Run a single test:
```bash
go test ./internal/db/... -run TestMigrate
```

## Architecture

### Request Flow

```
HTTP Request → Gin middleware (CORS, logging, Sentry) → GraphQL endpoint
             → DataLoader middleware injects loaders into context
             → Resolver → internal/api/ service methods → internal/db/ queries
             → PostgreSQL (via driver.SQLStore) or Redis (via driver.KVStore)
```

### Key Layers

- **[cmd/api/main.go](cmd/api/main.go)** — Bootstrap: connects Redis → PostgreSQL → runs migrations → starts HTTP service
- **[internal/http/](internal/http/)** — Gin HTTP service with routes: `GET /ping`, `POST /graphql`, `:9090/metrics`
- **[internal/api/](internal/api/)** — GraphQL resolvers; `resolver.go` wires dependencies; DataLoaders prevent N+1
- **[internal/db/](internal/db/)** — Repository layer using Masterminds Squirrel for query building; `main.go` combines SQLStore + KVStore
- **[internal/schema/](internal/schema/)** — GraphQL schema definitions (`.graphqls` files); source of truth for code generation
- **[internal/db/migrations/](internal/db/migrations/)** — Migration system tracking state in `migrations_state` table
- **[pkg/driver/](pkg/driver/)** — Database driver interfaces: `SQLStore` (pgx/v5) and `KVStore` (go-redis); `memkv/` for tests
- **[pkg/core/](pkg/core/)** — Shared utilities: `auth/` (JWT/RBAC), `log/` (Zap), `errors/` (stack traces), `apm/` (Sentry/Prometheus), `encoding/` (ID generation)

### Data Storage Pattern

Accounts (and likely future entities) use a **JSONB payload** pattern: `account(id TEXT, data JSONB, created_at, updated_at)`. IDs are generated via `encoding.NewRandomIdentifier("account")` which produces prefixed random identifiers.

### Dependency Injection

Dependencies flow explicitly via structs, not globals:
```go
http.Service{Dependencies: http.Dependencies{DB: db, Clock: clock}}
api.Service{Dependencies: api.Dependencies{Database: database}}
```

### GraphQL Code Generation

Schema-first: edit `.graphqls` files in `internal/schema/`, then run `make generate`. Generated code goes to `internal/model/models_gen.go` and resolver stubs in `internal/api/`. Configuration: `internal/gqlgen.yml`.

## Environment

Key variables (see `deployment/.env.example`):
- `ENVIRONMENT` — `dev`/`staging`/`production` (affects logging format and code paths)
- `DB_*` — PostgreSQL connection
- `REDIS_*` — Redis connection

## Testing

Test DB utilities live in `internal/db/testdb/`. Integration tests use real PostgreSQL/Redis connections, not mocks — this is intentional to catch real migration/driver issues.
