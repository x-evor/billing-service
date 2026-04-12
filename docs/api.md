# billing-service API and interfaces

This document describes the current `billing-service` task API plus the
upstream and downstream interfaces it depends on.

## Service endpoints

### `GET /api/ping`

Returns the runtime image identity exposed by the running container.

Example response:

```json
{
  "image": "registry.example.com/billing-service:sha-0123456789abcdef0123456789abcdef01234567",
  "tag": "sha-0123456789abcdef0123456789abcdef01234567",
  "commit": "0123456789abcdef0123456789abcdef01234567",
  "version": "0123456789abcdef0123456789abcdef01234567"
}
```

### `GET /healthz`

Returns service health derived from the most recent collect-and-rate execution.

Example response:

```json
{
  "status": "ok",
  "message": ""
}
```

### `GET /v1/status`

Returns the latest in-memory job result snapshot.

Key fields:

- `job`
- `started_at`
- `finished_at`
- `processed_samples`
- `written_minutes`
- `replayed_minutes`
- `status`
- `error`

### `POST /v1/jobs/collect-and-rate`

Triggers an immediate snapshot pull from `xray-exporter`, computes minute
deltas, rates chargeable bytes, and writes replay-safe facts into PostgreSQL.

Behavior:

- method must be `POST`
- returns `200` when the run completes without a fatal service error
- returns `503` when upstream fetch or persistence fails hard enough to mark the
  run unavailable

### `POST /v1/jobs/reconcile`

Triggers the same execution path as collect-and-rate, but records the job name
as `reconcile` for operational visibility.

## Upstream dependency

### `xray-exporter`

`billing-service` currently depends on a single exporter base URL and fetches:

- `GET /v1/snapshots/latest`

Minimum payload shape:

```json
{
  "collected_at": "2026-04-08T12:00:00Z",
  "node_id": "jp-xhttp-contabo.svc.plus",
  "env": "prod",
  "samples": [
    {
      "uuid": "uuid-1",
      "email": "user@example.com",
      "inbound_tag": "xhttp-premium",
      "uplink_bytes_total": 1024,
      "downlink_bytes_total": 2048
    }
  ]
}
```

Required fields:

- `collected_at`
- `node_id`
- `env`
- `samples[].uuid`
- `samples[].email`
- `samples[].inbound_tag`
- `samples[].uplink_bytes_total`
- `samples[].downlink_bytes_total`

### Target upstream contract

Current production behavior remains `GET /v1/snapshots/latest`, but the target
multi-node design should evolve to:

- HTTPS transport for remote exporter pulls
- source-specific authentication
- a windowed pull API that supports catch-up and pagination

Recommended target path:

- `GET /v1/snapshots/window?since=<RFC3339>&until=<RFC3339>&limit=<n>&cursor=<token>`

Target-state expectations:

- remote pulls use `https://` exporter base URLs
- TLS verification stays enabled
- each source can be authenticated independently
- responses can be replayed safely from source checkpoints without duplicate
  billing writes

## Downstream reads

User-facing reads do not go through `billing-service`. The read model is
`accounts.svc.plus`, backed by PostgreSQL.

Relevant downstream APIs:

- `GET /api/account/usage/summary`
- `GET /api/account/usage/buckets`
- `GET /api/account/billing/summary`

Read-path rules:

- `billing-service` does not expose user-facing usage or billing query APIs
- `accounts.svc.plus` reads PostgreSQL-backed usage and billing facts
- `console.svc.plus` queries `accounts.svc.plus`, not `billing-service`

## Configuration inputs

Runtime environment variables used by the current implementation:

- `IMAGE`
- `EXPORTER_BASE_URL`
- `DATABASE_URL`
- `LISTEN_ADDR`
- `COLLECT_INTERVAL`
- `DEFAULT_REGION`
- `SOURCE_REVISION`
- `PRICE_PER_BYTE`
- `INITIAL_INCLUDED_QUOTA_BYTES`
- `INITIAL_BALANCE`

`IMAGE` rule:

- it must contain the full image reference used to start the container
- `/api/ping` derives `image`, `tag`, `commit`, and `version` from this value
- when `IMAGE` is missing or malformed, runtime metadata fields should remain empty rather than fabricated

`DATABASE_URL` rule:

- it must point to the same `account` database that `accounts.svc.plus` uses
- on `jp-xhttp-contabo.svc.plus`, the current accounts containers use
  `DB_HOST=stunnel-client`, `DB_PORT=15432`, and `DB_NAME=account`
- `billing-service` should follow that same target so user-facing reads in
  `accounts.svc.plus` see the exact facts written by `billing-service`
