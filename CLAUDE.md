# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

You are a staff engineer who loves to write the best tests, you try not to mock and if possible use real services.

## Project Overview

az is a minimal multi-tenant RBAC authorization service — the sibling of BulwarkAuth (authentication). Resource types declare actions, roles grant `resource:action` permissions, assignments bind subjects to roles, and `POST /api/check` decides. Policies are rows; integrity is foreign keys.

## Architecture

Three layers, manual dependency injection in `cmd/az/main.go`:

- `api/<domain>/` — Echo v5 handlers + routes; errors as RFC 7807 problem details (`api/problem`)
- `internal/<domain>/` — service interface + `Default*Service`, repository interface + `Postgres*Repository`
- `internal/db/` — pgx pool + embedded SQL migrations, run at startup

Rules that must hold:

- Repositories resolve their querier with `utils.QuerierFrom(ctx, pool)`; multi-statement writes go through `TxManager.WithTransaction`
- Typed domain errors mapped in handlers via `errors.As`; constraint violations (23505/23503) mapped at the repository
- Integrity is DB-enforced, never service-side lookups: invalid grants, referenced deletes, and cascades are all foreign keys
- `tenantID` is the first argument of service/repository methods that take scalar keys; write methods that persist a full entity carry it in the entity struct instead (`Create(ctx, role)`) — do not flag this as a convention violation. Empty tenantId in requests means `"default"`
- Handlers resolve the tenant with `auth.EffectiveTenant(c, request.TenantID)` — a tenant-scoped api key overrides any body tenantId; the bootstrap key (BOOTSTRAP_API_KEY) is root and the only key that manages api keys
- Ids are app-generated UUIDv7 (`uuid.NewV7()` in repository Create), never `gen_random_uuid()`
- Always use range for loops when possible

## Development Commands

```bash
go build -o az ./cmd/az             # build
go run ./cmd/az                     # run (needs postgres; .env loaded if present)
docker-compose up                   # run with postgres
go test -p 1 ./...                  # unit tests (embedded PostgreSQL, real DB, no mocks)
go test -run TestName ./...         # single test
go test ./test/integration/... -v   # black-box integration (needs a running service)
```

Unit tests: `utils.RunTestMain` in each package's TestMain boots one embedded postgres; `utils.NewTestPool(t)` gives an isolated migrated database per test. Keep `-p 1` — each package boots its own server. Integration tests need `REQUESTS_PER_SECOND=1000` on the service; `AZ_BASE_URI` overrides the target.

Config is env vars only — the table lives in README.md.

## Where deeper knowledge lives

Decision history, schema rationale, and gotchas are in the demarkus soul under `/az/` (ADRs, journal) — recall on demand rather than duplicated here.
