# Integration Tests

Black-box tests against a running az service on `http://localhost:8080`.

## Run

```bash
docker-compose up -d          # postgres + az
go test ./test/integration/... -v
```

Each test creates a unique tenant, so the suite is repeatable against the same
database. When `API_KEY_ENABLED=true` on the service, export `API_KEY` before
running the tests. Set `AZ_BASE_URI` to test a service on another address.

The suite issues requests faster than the default rate limit — run the service
with `REQUESTS_PER_SECOND=1000` (CI does).

The suite uses the [az-client](https://github.com/latebit-io/az-client)
library.
