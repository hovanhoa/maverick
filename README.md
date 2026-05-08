# go-ai-gateway

Internal AI Gateway platform written in Go to centralize, secure, and observe AI/LLM usage across the engineering organization.

## Why This Project Exists

Teams are adopting AI tools quickly (Claude Code, Cursor, IDE extensions, CI bots, internal apps). Without a shared gateway, usage becomes fragmented, difficult to govern, and hard to optimize.

`go-ai-gateway` provides a single control plane and data plane for AI access so the organization can:

- enforce security and policy controls,
- track usage and cost transparently,
- manage quotas and permissions per team/member,
- route traffic across multiple providers,
- support future internal AI agents and automation.

## High-Level Objectives

1. Centralize AI usage behind one gateway
2. Track token and cost usage
3. Apply security and prompt policies
4. Manage team/member quotas
5. Route requests to multiple providers
6. Support internal AI agents and workflows
7. Prevent confidential data leakage
8. Build reusable AI infrastructure

## Supported Provider Direction

- Anthropic Claude
- Amazon Bedrock
- Google Vertex AI
- OpenAI
- Local/self-hosted models (future)

The architecture is provider-agnostic to avoid vendor lock-in.

## Core Platform Capabilities

- Authentication and RBAC
- API key management
- Team/user quotas
- Token usage tracking
- Cost monitoring
- Request logging
- Audit logs
- Model routing
- Rate limiting
- Retry and fallback handling
- Prompt filtering
- Sensitive data detection
- Internal system prompts
- OpenAI-compatible APIs
- Streaming response support

## MVP Scope (Phase 1)

This repository starts with a focused MVP:

- API gateway service
- Authentication
- Provider abstraction layer
- Claude + Bedrock + Vertex AI integration
- Usage tracking
- Quota management
- Request logging
- Basic policy enforcement
- Streaming support

### Out of Scope for MVP

- Complex multi-agent orchestration
- Heavy Kubernetes platform architecture
- Large-scale workflow engine
- Advanced vector database architecture
- Enterprise microservice decomposition

## Architecture

```text
Users & Tools
  - Claude Code
  - Cursor
  - VSCode extensions
  - CI/CD pipelines
  - Internal applications
          |
          v
go-ai-gateway
  - auth
  - policies
  - quotas
  - observability
  - routing
  - governance
  - logging
          |
          v
Providers
  - Claude
  - Bedrock
  - Vertex AI
  - OpenAI
  - local models (future)
```

## Technical Stack

- Go (Golang)
- PostgreSQL
- Redis
- REST APIs (OpenAI-compatible where possible)
- OpenTelemetry
- Docker
- Optional messaging: NATS/Kafka

## Design Principles

1. Lightweight and easy to operate
2. Provider-agnostic
3. Security-first
4. Extensible architecture
5. Minimal infrastructure complexity
6. Ready for future AI agents and automation

## Intended Users

- Frontend engineers
- Backend engineers
- QA engineers
- Product managers
- Designers
- DevOps/platform engineers
- Internal AI agents and automation workflows

## Suggested Repository Roadmap

Recommended package/module boundaries as implementation starts:

- `cmd/gateway` - service entrypoint
- `internal/api` - HTTP handlers and routing
- `internal/auth` - authn/authz and RBAC
- `internal/policy` - prompt and data policy checks
- `internal/provider` - provider abstraction + adapters
- `internal/quota` - usage limits and enforcement
- `internal/usage` - token/cost metering
- `internal/observability` - logs, traces, metrics
- `internal/storage` - PostgreSQL/Redis repositories
- `pkg/openai` - OpenAI-compatible request/response models

## Security and Governance Expectations

- Treat prompts and responses as potentially sensitive
- Minimize storage of raw payloads where possible
- Redact secrets/PII in logs and traces
- Maintain auditable access trails
- Enforce least privilege via RBAC
- Apply quota and rate policies before provider calls

## Future Direction

After MVP stabilization, expand toward:

- AI Agent runtime
- Jira integration
- GitLab/GitHub integration
- Playwright/test generation agents
- Documentation agents
- Knowledge/RAG system
- Internal prompt registry
- Workflow automation
- Organization-wide AI analytics dashboard

## Project Status

Early-stage scaffold and planning phase.

Initial focus is to establish the MVP gateway foundation with clean interfaces and strong observability/security defaults.
