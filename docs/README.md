# billing-service docs

This directory holds service-owned documentation for `billing-service`.

## Documents

- [architecture.md](architecture.md) - deployment topology, billing data flow,
  and current-vs-target architecture notes
- [api.md](api.md) - task endpoints, upstream snapshot contract, and downstream
  read-model boundaries
- [multi-node-https-plan.md](multi-node-https-plan.md) - target-state plan for
  evolving from a single exporter URL to secure multi-node HTTPS ingestion
- [../sql/billing-service-schema.sql](../sql/billing-service-schema.sql) -
  reference DDL for the accounting tables `billing-service` depends on

## Scope

These docs describe the `billing-service` role inside the Cloud Network Billing
& Control Plane.

- `billing-service` is the task-oriented write model
- `accounts.svc.plus` is the PostgreSQL-backed read model
- `console.svc.plus` is the presentation layer and does not query
  `billing-service` directly

System-wide contracts still live in
`github-org-cloud-neutral-toolkit/docs/architecture/network-billing-contracts.md`.

## Deployment Verification

Local or operator dry-run validation:

```bash
cd /Users/shenlan/workspaces/cloud-neutral-toolkit/playbooks
export DATABASE_URL=postgres://...
ANSIBLE_CONFIG=./ansible.cfg \
ansible-playbook -i ./inventory.ini -D -C ./deploy_billing_service.yml -l jp_xhttp_contabo_host
```

Notes:

- `DATABASE_URL` must be exported before running `deploy_billing_service.yml`
- on `jp-xhttp-contabo.svc.plus`, `DATABASE_URL` should reference the same
  `account` database used by `accounts.svc.plus`
- check mode may report predicted changes; the goal is to pass the preflight
  assertion and render a valid deployment plan
- GitHub Actions uses the `BILLING_SERVICE_DATABASE_URL` secret to satisfy the
  same precondition in the `deploy-billing-service` job
