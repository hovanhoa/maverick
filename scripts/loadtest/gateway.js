// k6 load test for the gateway's own request path (auth, RBAC, DataLoaders,
// quota/policy prechecks, structured logging/metrics). It deliberately does
// NOT exercise a real provider call: /v1/chat/completions is hit with an
// unconfigured provider name, which the proxy rejects with a 400 before
// ever dispatching upstream (see internal/proxy/proxy.go's resolve step).
// That's enough to load-test everything this gateway is actually
// responsible for - auth, quota reservation, policy evaluation, DB reads,
// observability - without incurring real LLM API cost or being bottlenecked
// by a third party's latency.
//
// Usage:
//   1. Get an API key: `make seed` (prints one for a fresh OWNER account),
//      or use any existing account's key.
//   2. Optionally get a team id via the management GraphQL API
//      (`mutation { createTeam(name: "loadtest") { id } }`) if you want the
//      GraphQL scenario to hit a real team instead of just erroring on a
//      missing one - the script tolerates either.
//   3. Run:
//        BASE_URL=http://localhost:8080 API_KEY=llmgw_... k6 run scripts/loadtest/gateway.js
//      Tunable via env vars: VUS (default 10), DURATION (default 30s).
//
// See scripts/loadtest/README.md for how to read the results and what
// good/concerning numbers look like for this gateway.

import http from 'k6/http';
import { check } from 'k6';

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const API_KEY = __ENV.API_KEY;
const TEAM_ID = __ENV.TEAM_ID || 'team_does_not_exist';

if (!API_KEY) {
  throw new Error('API_KEY env var is required - see the header comment in this file for how to get one');
}

export const options = {
  vus: Number(__ENV.VUS || 10),
  duration: __ENV.DURATION || '30s',
  thresholds: {
    // The gateway's own overhead (everything except an actual provider
    // call) should stay well under a second even under load; tune this
    // based on what a real run against your deployment shows.
    http_req_duration: ['p(95)<1000'],
    // Not http_req_failed: the chat-completions request is *expected* to
    // come back 400 (see the check below), which k6 would otherwise count
    // as a failed request. `checks` is the right signal here - did each
    // response match what this script actually expects.
    checks: ['rate>0.99'],
  },
};

const authHeaders = {
  headers: {
    Authorization: `Bearer ${API_KEY}`,
    'Content-Type': 'application/json',
  },
};

export default function () {
  // 1. A cheap GraphQL read - exercises auth, RBAC, and a DB round trip.
  const teamQuery = JSON.stringify({
    query: `query($id: ID!) { team(id: $id) { id name } }`,
    variables: { id: TEAM_ID },
  });
  const teamRes = http.post(`${BASE_URL}/graphql/query`, teamQuery, authHeaders);
  check(teamRes, { 'graphql: got a response': (r) => r.status === 200 });

  // 2. The proxy's full auth -> request-id -> validate -> resolve pipeline,
  // rejected at the "unknown provider" step before any upstream call.
  const chatBody = JSON.stringify({
    model: 'loadtest-unconfigured-provider/does-not-matter',
    messages: [{ role: 'user', content: 'load test ping' }],
  });
  const chatRes = http.post(`${BASE_URL}/v1/chat/completions`, chatBody, authHeaders);
  check(chatRes, { 'chat completions: rejected as expected': (r) => r.status === 400 });
}
