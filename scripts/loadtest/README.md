# Load testing

`gateway.js` is a [k6](https://k6.io) script that load-tests the gateway's
own request path - auth, RBAC, quota/policy prechecks, DB reads, structured
logging/metrics - without calling a real LLM provider. That keeps a run
free (no per-token cost) and means results reflect this gateway's own
overhead rather than a provider's latency.

## Running it

```bash
# 1. Start the stack
make docker-up
make run   # in another terminal

# 2. Get an API key
make seed  # prints an API key for a fresh OWNER account

# 3. Run the load test
BASE_URL=http://localhost:8080 API_KEY=llmgw_... k6 run scripts/loadtest/gateway.js

# Tune load:
VUS=50 DURATION=60s BASE_URL=http://localhost:8080 API_KEY=llmgw_... k6 run scripts/loadtest/gateway.js
```

## Reading the results

k6 prints `http_req_duration` (p50/p90/p95/p99) and `http_req_failed` at
the end of the run. The script's default thresholds (p95 < 1s, failure
rate < 1%) are a starting point, not a guarantee - they exist so the run
exits non-zero if something regresses badly, not as a validated SLO.

If `http_req_duration` grows with `VUS` faster than expected, or
`http_req_failed` climbs, the next place to look is:

- `pkg/driver/postgres`'s pool (`MaxConns`/`MinIdleConns` in
  [postgres.go](../../pkg/driver/postgres/postgres.go)) - exhausted
  connections show up as growing query latency under load.
- Redis (`pkg/driver/redis`) - the quota checker's `GetAndSet` retries on
  contention (see `internal/quota/quota.go`); a spike in
  `client.GetAndSet(...): exceeded N retries` errors in logs means quota
  reservations for the *same team* are colliding faster than they can be
  serialized - expected at very high VUS against a single team, not
  otherwise.
- `:9090/metrics` - `core_http_request_duration_seconds` (this gateway's
  own routes) and `llmgateway_quota_denied_total`/
  `llmgateway_proxy_stream_duration_seconds` (Phase 4/5 additions) during
  the run.

This script intentionally doesn't call a real provider, so it can't tell
you anything about provider-side latency/rate limits - that's a separate
concern from what this gateway controls.
