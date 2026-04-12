# Release Traceability Contract

This repository now treats `IMAGE` as the single runtime source of truth for
release identity.

## Runtime contract

- `IMAGE` must contain the full release image reference produced by the build
  job, even when the target host runs the service as a systemd binary instead
  of a container.
- `/api/ping` returns `image`, `tag`, `commit`, and `version`.
- `tag`, `commit`, and `version` are derived from `IMAGE`.
- If `IMAGE` is missing or malformed, the derived fields stay empty instead of
  being fabricated.

## Pipeline contract

- Build must produce `service_image_ref` only from the full `GITHUB_SHA`.
- Build must also produce the linux billing-service binary artifact consumed by
  deploy.
- Deploy must consume that build artifact directly and must not rebuild on the
  target host.
- Deploy must pass `service_image_ref` through as the runtime image identity.
- Validate must query `billing-service` on the deployment target at
  `http://127.0.0.1:8081/api/ping` over SSH.
- Validate must derive `tag` and `commit` from `service_image_ref` and compare
  them against `/api/ping`.
- Validate must fail when runtime `image`, `tag`, `commit`, or `version` is
  empty.
- Validate must compare `version` against the derived full commit, not a
  synthetic fallback.

## External playbook alignment

The external `playbooks/deploy_billing_service.yml` playbook should accept
`BILLING_SERVICE_BINARY_ARTIFACT` and `BILLING_SERVICE_IMAGE_REF`, deploy the
binary artifact to the target host without rebuilding it there, and inject
`IMAGE=<full image ref>` into `/etc/default/billing-service` for the runtime
served through `http://127.0.0.1:8081/api/ping`.

If `billing-service /api/ping` keeps returning empty runtime metadata, treat
that as a deployment contract failure: the runtime did not receive the full
`IMAGE` value and release traceability is broken.
