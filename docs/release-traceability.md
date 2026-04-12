# Release Traceability Contract

This repository now treats `IMAGE` as the single runtime source of truth for
release identity.

## Runtime contract

- `IMAGE` must contain the full image reference used to start the container.
- `/api/ping` returns `image`, `tag`, `commit`, and `version`.
- `tag`, `commit`, and `version` are derived from `IMAGE`.
- If `IMAGE` is missing or malformed, the derived fields stay empty instead of
  being fabricated.

## Pipeline contract

- Build must produce `service_image_ref` only from the full `GITHUB_SHA`.
- Deploy must consume `service_image_ref` and pass it through as the runtime
  image identity.
- Validate must derive `tag` and `commit` from `service_image_ref` and compare
  them against `/api/ping`.

## External playbook alignment

The external `playbooks/deploy_billing_service.yml` playbook should accept
`IMAGE_REF` (or an equivalent full image reference variable), derive any
repo/tag helpers from it, and inject `IMAGE=<full image ref>` into the running
container environment.
