# go-ai-gateway

Internal AI Gateway platform written in Go to centralize, secure, and observe AI/LLM usage across the engineering organization.

## Why This Project Exists

Teams are adopting AI tools quickly (Claude Code, Cursor, IDE extensions, CI bots, internal apps). Without a shared gateway, usage becomes fragmented, difficult to govern, and hard to optimize.

`go-ai-gateway` provides a single control plane and data plane for AI access so the organization can:

- **Enforce security and policy controls** - centralized request/response filtering and validation
- **Track usage and cost transparently** - token-level metering across all providers
- **Manage quotas and permissions** - per team/member/model quota enforcement
- **Route traffic intelligently** - failover, load balancing, and provider selection
- **Support internal AI automation** - foundation for future agents and workflows
- **Maintain audit trails** - comprehensive logging and observability

## Quick Start

### Prerequisites
- Go 1.21+
- PostgreSQL 14+
- Redis 7+
- Node.js 18+ (for web UI)

### Setup

```bash
# Install dependencies
go mod download
cd web && npm install && cd ..

# Build the gateway
make build

# Start services (PostgreSQL and Redis required)
make dev
```

The gateway will be available at `http://localhost:8080` and the web UI at `http://localhost:5173`.

## High-Level Objectives

1. Centralize AI usage behind one gateway
2. Track token and cost usage per model/provider
3. Apply security, prompt, and data policies
4. Manage team/member/model quotas
5. Route requests intelligently across multiple providers
6. Support internal AI agents and workflows
7. Prevent confidential data leakage
8. Build reusable AI infrastructure

## Supported Providers

**Currently Implemented:**
- Anthropic Claude (via Bedrock)
- Amazon Bedrock
- Google Vertex AI

**Planned:**
- OpenAI
- Local/self-hosted models
- Azure OpenAI

The architecture is provider-agnostic to avoid vendor lock-in.

## Core Platform Capabilities

### Security & Access Control
- Authentication and RBAC via API keys
- Team and user management
- API key generation and revocation
- Request authentication and validation

### Usage & Billing
- Token-level usage tracking
- Cost monitoring per team/user/model
- Quota enforcement (tokens, requests, rate limits)
- Detailed usage analytics and reporting

### Observability
- Comprehensive request/response logging
- Distributed tracing with OpenTelemetry
- Audit logs for security and compliance
- Metrics collection and export

### Intelligence
- Model routing and provider selection
- Retry and fallback handling
- Prompt filtering and validation
- Sensitive data detection
- Internal system prompt injection

### Compatibility
- OpenAI-compatible REST API endpoints
- Streaming response support
- Multi-provider request abstraction

## Project Structure

```
.
├── cmd/
│   └── gateway/          # Main gateway service
├── internal/
│   ├── api/              # HTTP handlers and routing
│   ├── auth/             # Authentication and RBAC
│   ├── policy/           # Request/response policies
│   ├── provider/         # Provider adapters and abstraction
│   ├── quota/            # Quota management and enforcement
│   ├── usage/            # Token/cost metering
│   ├── observability/    # Logging, tracing, metrics
│   └── storage/          # PostgreSQL/Redis repositories
├── pkg/
│   ├── openai/           # OpenAI-compatible models
│   ├── core/             # Shared utilities and infrastructure
│   └── driver/           # Database drivers
├── web/                  # React/TypeScript dashboard UI
├── deployment/           # Docker and deployment configs
└── docs/                 # Architecture and design docs
```

## Development

### Build Commands

```bash
make dev          # Start development server
make build        # Build the gateway binary
make test         # Run all tests
make lint         # Run linters
make fmt          # Format code
```

### Testing

```bash
go test ./...
go test -v ./internal/provider/...
go test -bench ./...
```

### Database

PostgreSQL migrations are managed automatically on startup. Redis is used for caching and rate limiting.

### Web Dashboard

The web UI provides:
- Account management
- API key management
- Team and member management
- Usage and cost analytics
- Quota configuration

```bash
cd web
npm run dev      # Development with hot reload
npm run build    # Production build
```

## Architecture

```
┌─────────────────────────────────────────────────┐
│              Users & Tools                      │
│  • Claude Code / Cursor                         │
│  • VSCode extensions                            │
│  • CI/CD pipelines                              │
│  • Internal applications                        │
└──────────────────┬──────────────────────────────┘
                   │
                   v
   ┌───────────────────────────────────┐
   │    go-ai-gateway                  │
   ├───────────────────────────────────┤
   │  • Authentication & RBAC          │
   │  • Request validation & routing   │
   │  • Policy enforcement             │
   │  • Quota & rate limiting          │
   │  • Usage tracking & metering      │
   │  • Observability & logging        │
   └──────┬──────────────┬─────────────┘
          │              │
     ┌────v──────┐  ┌────v─────┐
     │PostgreSQL │  │  Redis    │
     │(Usage,    │  │(Cache,    │
     │Quotas)    │  │Rates)     │
     └───────────┘  └───────────┘
          │
          v
   ┌──────────────────┐
   │   Providers      │
   ├──────────────────┤
   │  • Anthropic     │
   │  • Bedrock       │
   │  • Vertex AI     │
   │  • OpenAI (soon) │
   └──────────────────┘
```

## Technical Stack

**Backend:**
- Go 1.21+ (type-safe, concurrent, fast)
- PostgreSQL 14+ (primary data store)
- Redis 7+ (caching, rate limiting)
- OpenTelemetry (distributed tracing, metrics)

**Frontend:**
- React 18 (UI framework)
- TypeScript (type safety)
- Tailwind CSS (styling)
- Vite (build tooling)

**Infrastructure:**
- Docker & Docker Compose
- REST APIs (OpenAI-compatible)
- OpenAPI/Swagger documentation
- GraphQL (internal APIs)

**Optional:**
- Kubernetes (deployment)
- Datadog/New Relic (APM)
- NATS/Kafka (event streaming)

## Design Principles

1. **Lightweight and easy to operate** - minimal dependencies, clear deployment story
2. **Provider-agnostic** - abstracted provider interface prevents vendor lock-in
3. **Security-first** - default deny, explicit allow, audit everything
4. **Extensible architecture** - clean interfaces for adding providers and policies
5. **Observable by default** - comprehensive logging, tracing, and metrics
6. **Production-ready** - error handling, retries, failover, graceful degradation
7. **Ready for future automation** - foundation for internal AI agents and workflows

## Intended Users

**Direct Users:**
- Frontend engineers
- Backend engineers
- QA engineers
- Product managers
- Designers

**Platform Operators:**
- DevOps/platform engineers
- Security teams
- Finance/billing teams

**Future:**
- Internal AI agents and automation workflows

## Configuration

Environment variables:

```bash
# Server
GATEWAY_PORT=8080
GATEWAY_ENV=development

# Database
DATABASE_URL=postgres://user:pass@localhost:5432/gateway
REDIS_URL=redis://localhost:6379

# Providers
AWS_REGION=us-west-2
AWS_ACCESS_KEY_ID=...
AWS_SECRET_ACCESS_KEY=...

GCP_PROJECT_ID=...
GCP_CREDENTIALS=...

# Observability
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317
LOG_LEVEL=info
```

See `.env.example` and `CLAUDE.md` for detailed configuration.

## Security & Compliance

### Data Handling
- Prompts and responses treated as sensitive
- Minimal storage of raw payloads (tokens + metadata only)
- Automatic secrets/PII redaction in logs and traces
- TLS for all external communications
- Database encryption at rest

### Access Control
- API key based authentication
- Role-based access control (RBAC)
- Least privilege enforcement
- API key scoping and rotation

### Audit & Monitoring
- Comprehensive audit logs
- Request/response tracing
- Rate limiting and quota enforcement
- Anomaly detection ready

### Compliance
- Ready for SOC 2, HIPAA, GDPR compliance
- Data residency controls
- Access audit trails
- User consent tracking

## Roadmap

### Phase 1 (Current) - MVP Gateway
- [x] Core gateway service
- [x] Authentication & API keys
- [x] Bedrock + Vertex AI support
- [x] Usage tracking & quotas
- [x] Web dashboard
- [ ] OpenAI integration
- [ ] Advanced policy engine

### Phase 2 - Enterprise Features
- [ ] Multi-workspace support
- [ ] Advanced analytics & reporting
- [ ] Custom policy builders
- [ ] Model fine-tuning support
- [ ] Webhook integrations

### Phase 3 - Automation & Intelligence
- [ ] AI Agent runtime
- [ ] Workflow automation engine
- [ ] Internal prompt registry
- [ ] Knowledge/RAG system
- [ ] GitHub/GitLab integration
- [ ] Jira integration

### Phase 4 - Scale
- [ ] Multi-region deployment
- [ ] Advanced load balancing
- [ ] Provider marketplace
- [ ] Self-service onboarding

## Contributing

Contributions welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

### Getting Help

- **Documentation**: See `docs/` for architecture and design docs
- **Issues**: File bugs and feature requests on GitHub
- **Architecture**: See `CLAUDE.md` for detailed system design

## Project Status

**Status**: Early Production (MVP Phase)

Current focus is establishing a stable, secure, and observable gateway foundation with clean provider abstraction and strong security/observability defaults.

See [CLAUDE.md](CLAUDE.md) for detailed architecture decisions and implementation notes.
