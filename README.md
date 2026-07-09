# az

Micro authorization service — pure RBAC, kept deliberately minimal. The sibling of [BulwarkAuth](https://github.com/latebit-io/bulwarkauth): BulwarkAuth answers *who are you*, az answers *what can you do*.

- **Resource types** declare what exists and the actions on it (`document`: `read`, `write`)
- **Roles** (uuid id + unique name) grant `resource:action` permissions
- **Assignments** bind subjects (opaque keys, e.g. account emails) to roles by id, per tenant
- **Check API**: a policy decision point — `POST /api/check` with subject, action and resource returns `{allow, reason}`
- **Multi-tenant** everywhere, with a `default` tenant out of the box

Stack: Go, Echo v5, PostgreSQL (pgx), slog. No ORM, no policy language — policies are rows: `tenants`, `resource_types`, `resource_type_actions`, `roles`, `role_permissions`, `role_assignments`. Integrity is foreign keys, not application code.

## Quick start

```bash
cp .env.example .env
docker-compose up
```

Define a policy and check it:

```bash
# a resource type with actions
curl -X POST localhost:8080/api/resources -H 'Content-Type: application/json' \
  -d '{"name":"document","actions":["read","write"]}'

# a role granting permissions (response includes the generated id)
curl -X POST localhost:8080/api/roles -H 'Content-Type: application/json' \
  -d '{"name":"editor","permissions":[{"resource":"document","action":"write"}]}'
# {"id":"6a8f...","tenantId":"default","name":"editor",...}

# assign it by id
curl -X POST localhost:8080/api/assignments -H 'Content-Type: application/json' \
  -d '{"subject":"alice@example.com","roleId":"6a8f..."}'

# check
curl -X POST localhost:8080/api/check -H 'Content-Type: application/json' \
  -d '{"subject":"alice@example.com","action":"write","resource":"document"}'
# {"allow":true,"reason":"role 'editor' grants document:write"}
```

An empty `tenantId` means the `default` tenant; pass `tenantId` in any body to scope to another tenant. More examples in `http/az.http.example`.

## Authentication

Set `BOOTSTRAP_API_KEY` and every endpoint except `/health` requires a key in the `X-AZ-API-KEY` header (unset means auth is disabled — dev only). Two kinds of key, permit.io environment-key style:

- **Bootstrap key** (the env var): the root credential. Works on any tenant and is the only key that can manage api keys.
- **Tenant keys**: minted per tenant via the api. Full capability — manage policy, check — but locked to their tenant: the tenant comes from the key, any `tenantId` in the body is ignored.

```bash
# mint a tenant-scoped key (bootstrap key required; secret is shown once)
curl -X POST localhost:8080/api/apikeys \
  -H "X-AZ-API-KEY: $BOOTSTRAP_API_KEY" -H 'Content-Type: application/json' \
  -d '{"tenantId":"acme","name":"backend"}'
# {"id":"...","tenantId":"acme","name":"backend","prefix":"azk_1a2b3c4d","key":"azk_..."}
```

Keys are stored hashed (sha256 of the high-entropy token); only the `azk_` prefix is kept readable for lookup and log identification. Revoke with `PUT /api/apikeys/delete`.

## How a check decides

1. Resolve the resource type. Unknown type or action → deny (denies are `200 {allow:false}`, never errors).
2. If any role assigned to the subject grants `resource:action` → allow.
3. Otherwise deny.

## API

All endpoints take JSON bodies; errors are RFC 7807 problem details.

| Area | Endpoints |
|---|---|
| Resource types | `POST /api/resources` · `POST /api/resources/get` · `POST /api/resources/list` · `PUT /api/resources` · `PUT /api/resources/delete` |
| Roles | `POST /api/roles` (+`/get`, `/list`) · `PUT /api/roles` · `PUT /api/roles/delete` |
| Assignments | `POST /api/assignments` · `POST /api/assignments/list` · `PUT /api/assignments/delete` · `POST /api/subjects/roles` |
| Check | `POST /api/check` · `POST /api/check/bulk` |
| Api keys (all bootstrap key only) | `POST /api/apikeys` · `POST /api/apikeys/list` · `PUT /api/apikeys/delete` |
| Health | `GET /health` (unauthenticated) |

Referential integrity is foreign keys: a grant referencing an undeclared `resource:action` is rejected (`400`), deleting a role cascades its grants and assignments, and deleting a resource type (or removing a still-granted action) returns `409` while a role references it.

Resource types and roles are addressed by uuid `id` (returned on create); `name` is the unique per-tenant label. Permission grants and check requests reference resource types by name, so renaming a type never breaks existing grants. `POST /api/subjects/roles` returns `{"roles": [names...]}` for a subject — the payload BulwarkAuth embeds as the JWT `roles` claim at token issuance.

## Configuration

All configuration is via environment variables (a `.env` file is loaded when present):

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | HTTP port |
| `DB_CONNECTION` | `postgres://az:az@localhost:5432/az?sslmode=disable` | PostgreSQL connection string |
| `REQUESTS_PER_SECOND` | `20` | Rate limit |
| `BOOTSTRAP_API_KEY` | — | Root api key; when set, all endpoints except `/health` require a key |
| `CORS_ENABLED` | `false` | Enable CORS |
| `ALLOWED_WEB_ORIGINS` | — | Comma-separated CORS origins |
| `DOMAIN` | — | Appended to CORS origins as `https://<domain>` |
| `DECISION_LOG_ENABLED` | `true` | slog line per check decision |
| `DEFAULT_TENANT_ID` | `default` | Default tenant id |

Schema migrations run automatically at startup.

## Development

```bash
go run ./cmd/az                      # run (needs postgres)
go test -p 1 ./...                   # unit tests (embedded PostgreSQL — real DB, no mocks)
docker-compose up -d
go test ./test/integration/... -v    # black-box integration tests
```

## License

See [LICENSE](LICENSE).
