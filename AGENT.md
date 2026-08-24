# AGENT.md

This file provides guidance to AI agents (Claude, etc.) when working with code tasks in this repository.

## Quick Start for Agents

When assigned a task in this codebase:

1. **Understand the context** — Read CLAUDE.md first for architecture overview
2. **Locate relevant code** — Use `grep_search` to find existing patterns
3. **Follow the patterns** — This codebase has strong conventions (see below)
4. **Test thoroughly** — Run `make test` and `make test-race` before submitting
5. **Generate when needed** — Run `make generate` if you modify `.graphqls` schemas

## Code Organization Patterns

### Adding a New Feature

**GraphQL Schema First:**
- Edit `.graphqls` files in `internal/schema/`
- Run `make generate` to create resolver stubs and models
- Implement resolvers in `internal/api/`

**Database Layer:**
- Create migration in `internal/db/migrations/` (naming: `001_feature_name.sql`)
- Add repository methods in `internal/db/` using Masterminds Squirrel
- Use JSONB payload pattern for flexible fields (see `account` table)

**Service Layer:**
- Wire dependencies in `internal/api/resolver.go`
- Implement business logic in `internal/api/service_*.go`
- Use DataLoaders to prevent N+1 queries

### ID Generation

Always use the prefixed ID pattern:
```go
import "github.com/your-org/pkg/core/encoding"

id := encoding.NewRandomIdentifier("entity")  // produces: entity_abc123xyz
```

### Error Handling

Use `pkg/core/errors` for consistent error handling with stack traces:
```go
import "github.com/your-org/pkg/core/errors"

if err != nil {
    return errors.Wrap(err, "context about what failed")
}
```

### Logging

Use the centralized logger from `pkg/core/log`:
```go
import "github.com/your-org/pkg/core/log"

log.Info("event", "key", "value")
log.Error("failed to process", "error", err)
```

### Database Queries

Use Masterminds Squirrel with the SQLStore pattern:
```go
query := squirrel.Select("id", "data").
    From("accounts").
    Where(squirrel.Eq{"id": accountID})
    
rows, err := store.Query(ctx, query)
```

### Authentication & Authorization

- Token extraction: `pkg/core/http/auth_extract.go`
- Middleware: `pkg/core/http/auth_middleware.go`
- RBAC: `pkg/core/auth/`

## Common Tasks

### Add a New GraphQL Query

1. Update `internal/schema/*.graphqls`:
```graphql
type Query {
    getFeature(id: ID!): Feature
}

type Feature {
    id: ID!
    name: String!
}
```

2. Run `make generate` — creates resolver stub

3. Implement in `internal/api/resolver_feature.go`:
```go
func (r *queryResolver) GetFeature(ctx context.Context, id string) (*model.Feature, error) {
    // Implementation
}
```

### Add a Database Migration

1. Create file: `internal/db/migrations/NNN_description.sql`
2. Use migration interface tracking in `migrations_state` table
3. Test with `make test` — migrations run automatically in test setup

### Add a New HTTP Endpoint

1. Register route in `internal/http/service.go`:
```go
router.GET("/api/feature/:id", h.GetFeature)
```

2. Implement handler with proper error handling and logging

3. Return JSON responses via `pkg/core/http` utilities

### Add Tests

- Integration tests live alongside code: `*_test.go` files
- Use real PostgreSQL/Redis connections (see `internal/db/testdb/`)
- Run: `make test` and `make test-race`
- Test file pattern: `TestFeature`, `TestFeature_ErrorCase`

## Key Files to Know

| Path | Purpose |
|------|---------|
| `cmd/api/main.go` | Service bootstrap |
| `internal/http/service.go` | HTTP routing and Gin setup |
| `internal/api/resolver.go` | GraphQL dependency wiring |
| `internal/schema/` | GraphQL schema definitions |
| `internal/db/` | Database repository layer |
| `pkg/core/auth/` | JWT and RBAC utilities |
| `pkg/core/errors/` | Error handling with stacks |
| `pkg/core/log/` | Centralized logging |
| `internal/db/migrations/` | Database schema migrations |

## Development Workflow for Agents

```bash
# Start environment
make docker-up

# Run API
make run

# In another terminal, make changes then:
make generate    # If schema changed
make lint        # Format and vet
make test        # Run tests
make test-race   # Race condition detector
```

## Common Pitfalls to Avoid

- ❌ Don't use global state — pass dependencies explicitly
- ❌ Don't query without DataLoaders — causes N+1 problems
- ❌ Don't skip migrations — all schema changes need migrations
- ❌ Don't mix concerns — keep API, DB, and HTTP layers separate
- ❌ Don't hard-code IDs — always use ID generation utilities
- ✅ Do run `make test-race` before submitting
- ✅ Do include integration tests, not just unit tests
- ✅ Do use the logger, not fmt.Print
- ✅ Do wrap errors with context

## Questions or Stuck?

When uncertain:
1. Look for similar existing code (use grep_search)
2. Check the relevant test files for patterns
3. Review CLAUDE.md for architecture context
4. Examine migrations if schema is involved
5. Check `pkg/core/` for utility patterns

This codebase values explicit dependencies, integration testing, and clear separation of concerns.