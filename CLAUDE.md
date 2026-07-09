# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

You are a staff engineer who loves to write the best tests, you try not to mock and if possible use real services.

## Project Overview

az is an API-based, developer-focused authorization (authz) service — pure RBAC, kept deliberately minimal. It is the sibling of BulwarkAuth (authentication): BulwarkAuth answers "who are you", az answers "what can you do". Multi-tenant, PostgreSQL-backed, exposes a check API (policy decision point) plus management APIs for resource types, roles, and role assignments.

## Key Architecture

Three layers, manual dependency injection wired in `cmd/az/main.go`:

- `api/` — Echo v5 HTTP layer, one package per domain (`<domain>_handlers.go` + `<domain>_routes.go`). Request DTOs; errors returned as RFC 7807 problem details (`api/problem`).
- `internal/` — business logic, one package per domain. Each domain: service interface + `Default*Service` impl, repository interface + `Postgres*Repository` impl.
- `cmd/az/` — entrypoint (`main.go`) and env config (`config.go`).

Domains:
- `internal/tenants` — tenant registry, `"default"` tenant auto-created at startup
- `internal/resources` — resource types (key + actions)
- `internal/roles` — roles (permission grants `resource:action`) and role assignments (subject → role); subject keys are opaque (BulwarkAuth accounts map via email)
- `internal/check` — the decision engine (PDP)
- `internal/db` — pgx pool + embedded SQL migrations (run at startup)
- `internal/utils` — QuerierFrom/TxManager, key validation, embedded-postgres test util

Patterns:
- Repositories resolve their querier with `utils.QuerierFrom(ctx, pool)` so the same methods work inside and outside `TxManager.WithTransaction`.
- Typed domain errors (`*NotFoundError`, `*DuplicateError`, `*ReferencedError`, ...) mapped in handlers via `errors.As` to problem details. Duplicates via unique constraints (pg error 23505), cascades and blocks via FKs (23503).
- Multi-tenant everywhere: `tenantID` is the first argument of service/repository methods; empty tenantId in requests means `"default"`.
- Cross-package integrity without import cycles: small interfaces defined at the consumer (`resources.ReferenceChecker`), implemented by repositories elsewhere, wired in main.

## Decision Engine

`POST /api/check` with `{tenantId, subject, action, resource}` → `{allow, reason}`:

1. Resolve resource type; unknown type/action → deny (200 with allow:false, never an error).
2. Subject's role assignments joined against role permission grants (jsonb containment, GIN indexed) — match → allow.
3. Otherwise deny.

## Coding Standards

- Always use range for loops when possible.

## Development Commands

### Building and Running
```bash
go build -o az ./cmd/az     # build
go run ./cmd/az             # run
docker-compose up           # run with postgres
./az -version               # print version
```

### Testing
```bash
go test ./...               # unit tests (embedded PostgreSQL, real DB, no mocks)
go test -cover ./...        # with coverage
go test -run TestName ./... # single test
go test ./test/integration/... -v   # integration (needs docker-compose up)
```

Unit tests use `fergusstrange/embedded-postgres` via `utils.RunTestMain` (package TestMain) and `utils.NewTestPool(t)` (unique database per test, migrations applied). Run packages serially in CI (`-p 1`) — each package boots its own postgres.

### Mocks
Mocks via mockery (`.mockery.yaml`, inpackage) — only for external boundaries; prefer real services.

## Configuration

All config via env vars (godotenv loads `.env`):

| Variable | Default | Description |
|---|---|---|
| PORT | 8080 | HTTP port |
| DB_CONNECTION | postgres://az:az@localhost:5432/az?sslmode=disable | PostgreSQL connection |
| REQUESTS_PER_SECOND | 20 | Rate limit |
| API_KEY_ENABLED / API_KEY | false / — | KeyAuth on header X-AZ-API-KEY |
| CORS_ENABLED / ALLOWED_WEB_ORIGINS / DOMAIN | false / [] / — | CORS |
| DECISION_LOG_ENABLED | true | slog line per check decision |
| DEFAULT_TENANT_ID | default | Default tenant |

## Relationship to BulwarkAuth

- Subject keys are opaque; the convention for BulwarkAuth accounts is the account email.
- `POST /api/subjects/roles` returns a subject's role keys — intended for BulwarkAuth to embed the `roles` claim in access tokens at issuance.
- Stack mirrors BulwarkAuth flavour: layered architecture, repository/service patterns, RFC 7807 errors, real-service testing. Differences: PostgreSQL instead of MongoDB, Echo v5 instead of v4.
