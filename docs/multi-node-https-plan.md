# billing-service multi-node HTTPS ingestion plan

This document defines the target evolution path for `billing-service` from the
current single `EXPORTER_BASE_URL` pull model to a secure multi-node ingestion
model.

## Goal

Keep `billing-service` as the single billing write model, but let it ingest
snapshots from many remote `xray-exporter` instances over HTTPS without
assuming private-network reachability.

## Consistency budget

Target state does not require second-level strong consistency.

- minute-level sync drift is acceptable
- the system should be treated as eventually consistent across a short
  multi-minute window
- user-facing reads in `accounts.svc.plus` and `console.svc.plus` may lag the
  newest exporter counters briefly
- billing correctness matters more than immediate freshness

Operational meaning:

- collector retries may intentionally overlap prior windows
- delayed exporter delivery should be repaired by later collect or reconcile
  runs
- the write model must converge to the correct minute buckets and ledger state
  without double charging

## Why the current model is not enough

Today:

- `billing-service` accepts one `EXPORTER_BASE_URL`
- it fetches one `GET /v1/snapshots/latest` payload
- it assumes the latest snapshot is enough to advance billing state

This is fine for a single local exporter, but it is not enough for:

- multiple proxy nodes
- exporters reachable only over public or cross-region networks
- outage recovery where `latest` alone cannot prove whether intermediate
  windows were missed
- source-specific authentication and certificate validation

## Target design

### 1. Multi-source registry instead of one base URL

Target state replaces the single `EXPORTER_BASE_URL` dependency with a source
registry owned by `billing-service`.

Each configured source should define at least:

- `source_id`
- `node_id`
- `env`
- `base_url`
- `enabled`
- `auth_mode`
- `credential_ref`
- `ca_bundle_ref` or trusted issuer reference
- `server_name`
- `collect_interval`
- `request_timeout`

Rules:

- target `base_url` must be `https://...`
- `node_id` and `env` must match what the exporter emits
- one source maps to one exporter endpoint, even if several sources later share
  the same network path

### 2. HTTPS-only upstream interaction

Target state requires secure transport for remote exporter pulls.

Security rules:

- remote exporter pulls must use HTTPS
- certificate verification must stay enabled
- `billing-service` must not rely on insecure skip-verify mode
- prefer mTLS for service-to-service trust
- if mTLS is not yet available, use HTTPS plus a per-source bearer token
- credentials must be scoped per source, not shared globally across all nodes

Recommended trust order:

1. HTTPS + mTLS
2. HTTPS + bearer token + pinned CA / trusted issuer

### 3. Completeness-first pull contract

To make multi-node billing safe, the upstream contract must evolve from
`latest` to a windowed pull API.

Recommended target contract:

`GET /v1/snapshots/window?since=<RFC3339>&until=<RFC3339>&limit=<n>&cursor=<token>`

Response shape should include:

- `source_id`
- `node_id`
- `env`
- `window_start`
- `window_end`
- `items[]`
- `next_cursor`
- `has_more`
- `emitted_at`

Each item should still carry:

- `collected_at`
- `samples[].uuid`
- `samples[].email`
- `samples[].inbound_tag`
- `samples[].uplink_bytes_total`
- `samples[].downlink_bytes_total`

Why this matters:

- `latest` is enough for observability, but not enough to prove billing
  completeness
- windowed pagination lets `billing-service` resume from checkpoints and catch
  up after transient failures

### 4. Source checkpoints and replay safety

`billing-service` should track fetch progress per source, not globally.

Recommended source checkpoint fields:

- `source_id`
- `last_successful_until`
- `last_cursor`
- `last_attempted_at`
- `last_succeeded_at`
- `last_error`

Collection behavior:

- pull per source using that source's last successful checkpoint
- always overlap a small safety window during retries
- rely on idempotent minute-bucket writes so overlap does not double-charge
- expose source-level health in `/v1/status`
- treat short multi-minute lag as acceptable if replay convergence is preserved

### 5. Safe write semantics

Security alone is not enough; the write path must remain replay-safe.

Target write-path rules:

- billing facts remain keyed by `node_id`, `env`, `uuid`, `inbound_tag`, and
  bucket time
- re-fetching the same source window must not duplicate usage or ledger rows
- reconcile jobs must be able to replay a source or time range intentionally

## Recommended rollout

### Phase 1. Preserve current runtime

- keep `EXPORTER_BASE_URL` as legacy single-source mode
- keep `GET /v1/snapshots/latest` for current deployment compatibility

### Phase 2. Add source registry support

- introduce a multi-source config model
- let `billing-service` iterate sources internally
- keep single-source config as a compatibility shim

### Phase 3. Add HTTPS window API to exporter

- extend `xray-exporter` with a secure windowed snapshot API
- add source authentication and certificate validation requirements

### Phase 4. Dual-read migration

- let `billing-service` support both:
  - legacy single-source `latest`
  - target multi-source HTTPS window pulls
- compare source-level completeness and write counts during rollout

### Phase 5. Make multi-source HTTPS the default

- require HTTPS for remote exporter sources
- reserve plain HTTP for explicit same-host dev or local-only modes
- retire single global `EXPORTER_BASE_URL` as the primary production contract

## Non-goals

- exposing `billing-service` as a user-facing query API
- moving billing truth into Prometheus
- weakening TLS verification to simplify rollout
- making `accounts.svc.plus` call `billing-service` for runtime reads
