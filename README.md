# az

Micro authorization service — RBAC + ABAC in the style of permit.io. The sibling of [BulwarkAuth](https://github.com/latebit-io/bulwarkauth): BulwarkAuth answers *who are you*, az answers *what can you do*.

- **RBAC**: resource types with actions, roles granting `resource:action` permissions, per-tenant role assignments
- **ABAC**: attributes on subjects and resource instances, condition sets (subject sets / resource sets) with and/or condition trees, rules granting a permission from a subject set to a resource set
- **Check API**: a policy decision point — `POST /api/check` with subject, action and resource returns `{allow, reason}`
- **Multi-tenant** everywhere, with a `default` tenant out of the box

Stack: Go, Echo v5, PostgreSQL (pgx), slog. No ORM, no policy language — policies are data.

## Quick start

```bash
cp .env.example .env
docker-compose up
```

Define a policy and check it:

```bash
# a resource type with actions
curl -X POST localhost:8080/api/resources -H 'Content-Type: application/json' \
  -d '{"key":"document","name":"Document","actions":["read","write"]}'

# a role granting permissions
curl -X POST localhost:8080/api/roles -H 'Content-Type: application/json' \
  -d '{"key":"editor","name":"Editor","permissions":[{"resource":"document","action":"write"}]}'

# assign it
curl -X POST localhost:8080/api/assignments -H 'Content-Type: application/json' \
  -d '{"subject":"alice@example.com","role":"editor"}'

# check
curl -X POST localhost:8080/api/check -H 'Content-Type: application/json' \
  -d '{"subject":{"key":"alice@example.com"},"action":"write","resource":{"type":"document"}}'
# {"allow":true,"reason":"role 'editor' grants document:write"}
```

An empty `tenantId` means the `default` tenant; pass `tenantId` in any body to scope to another tenant. More examples in `http/az.http.example`.

## How a check decides

1. Resolve the resource type. Unknown type or action → deny (denies are `200 {allow:false}`, never errors).
2. **RBAC**: if any role assigned to the subject grants `resource:action` → allow.
3. **ABAC**: for each rule on `resource:action`, merge stored attributes with inline ones (inline wins, shallow) and evaluate the rule's subject set against the subject's attributes and its resource set against the resource's. First match → allow.
4. Otherwise deny.

Condition operators: `equals, not-equals, in, not-in, gt, gte, lt, lte, contains`, combined with `allOf`/`anyOf` (nesting up to 10 levels). Evaluation is fail-closed: a missing attribute makes a leaf false for every operator.

```json
{"allOf": [
  {"attribute": "department", "operator": "equals", "value": "engineering"},
  {"anyOf": [
    {"attribute": "level", "operator": "gte", "value": 5},
    {"attribute": "tags", "operator": "contains", "value": "admin"}
  ]}
]}
```

## API

All endpoints take JSON bodies; errors are RFC 7807 problem details.

| Area | Endpoints |
|---|---|
| Resource types | `POST /api/resources` · `POST /api/resources/get` · `POST /api/resources/list` · `PUT /api/resources` · `PUT /api/resources/delete` |
| Resource instances | `POST /api/resources/instances` (+`/get`, `/list`) · `PUT /api/resources/instances` · `PUT /api/resources/instances/delete` |
| Subjects | `POST /api/subjects` (+`/get`, `/list`) · `POST /api/subjects/roles` · `PUT /api/subjects` · `PUT /api/subjects/delete` |
| Roles | `POST /api/roles` (+`/get`, `/list`) · `PUT /api/roles` · `PUT /api/roles/delete` |
| Assignments | `POST /api/assignments` · `POST /api/assignments/list` · `PUT /api/assignments/delete` |
| Condition sets | `POST /api/conditionsets` (+`/get`, `/list`) · `PUT /api/conditionsets` · `PUT /api/conditionsets/delete` |
| Rules | `POST /api/conditionsets/rules` (+`/list`) · `PUT /api/conditionsets/rules/delete` |
| Check | `POST /api/check` · `POST /api/check/bulk` |
| Health | `GET /health` |

Referential integrity is enforced: grants are validated against resource types at write time, deleting a role cascades its assignments, and deleting a resource type or condition set that is still referenced returns `409`.

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
