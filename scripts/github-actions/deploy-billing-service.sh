#!/usr/bin/env bash
set -euo pipefail

test -n "${IMAGE_REF:?IMAGE_REF is required}"
ansible-playbook -i inventory playbooks/deploy_billing_service.yml
