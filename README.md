# az

Micro authorization service — pure RBAC, kept deliberately minimal. The sibling of [BulwarkAuth](https://github.com/latebit-io/bulwarkauth): BulwarkAuth answers *who are you*, az answers *what can you do*.

- **Resource types** declare what exists and the actions on it (`document`: `read`, `write`)
- **Roles** grant `resource:action` permissions
- **Assignments** bind subjects (opaque keys, e.g. account emails) to roles per tenant
- **Check API**: a policy decision point — `POST /api/check` with subject, action and resource returns `{allow, reason}`
- **Multi-tenant** everywhere, with a `default` tenant out of the box

Stack: Go, Echo v5, PostgreSQL (pgx), slog. No ORM, no policy language — policies are data. Four tables.

## Quick start

```bash
cp .env.example .env
docker-compose up
```

Define a policy and check it:

```bash
# a resource type with actions
curl -X POST localhost:8080/api/resources -H 'Content-Type: application/json' \
  -d '{"key":"document","actions":["read","write"]}'

# a role granting permissions
curl -X POST localhost:8080/api/roles -H 'Content-Type: application/json' \
  -d '{"key":"editor","name":"Editor","permissions":[{"resource":"document","action":"write"}]}'

# assign it
curl -X POST localhost:8080/api/assignments -H 'Content-Type: application/json' \
  -d '{"subject":"alice@example.com","role":"editor"}'

# check
curl -X POST localhost:8080/api/check -H 'Content-Type: application/json' \
  -d '{"subject":"alice@example.com","action":"write","resource":"document"}'
# {"allow":true,"reason":"role 'editor' grants document:write"}
```

An empty `tenantId` means the `default` tenant; pass `tenantId` in any body to scope to another tenant. More examples in `http/az.http.example`.

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
| Health | `GET /health` |

Referential integrity is enforced: grants are validated against resource types at write time, deleting a role cascades its assignments, and deleting a resource type still referenced by a role returns `409`.

`POST /api/subjects/roles` returns `{"roles": [...]}` for a subject — the payload BulwarkAuth embeds as the JWT `roles` claim at token issuance.

## Configuration

All configuration is via environment variables (a `.env` file is loaded when present):

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | HTTP port |
| `DB_CONNECTION` | `postgres://az:az@localhost:5432/az?sslmode=disable` | PostgreSQL connection string |
| `REQUESTS_PER_SECOND` | `20` | Rate limit |
| `API_KEY_ENABLED` | `false` | Require `X-AZ-API-KEY` header on all endpoints |
| `API_KEY` | — | The API key when enabled |
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
