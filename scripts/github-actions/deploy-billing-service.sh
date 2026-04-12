#!/usr/bin/env bash
set -euo pipefail

target_host="${STACK_TARGET_HOST:?STACK_TARGET_HOST is required}"
artifact_path="${BILLING_SERVICE_BINARY_ARTIFACT:?BILLING_SERVICE_BINARY_ARTIFACT is required}"
image_ref="${BILLING_SERVICE_IMAGE_REF:-${IMAGE_REF:-}}"
database_url="${DATABASE_URL:?DATABASE_URL is required}"
internal_service_token="${INTERNAL_SERVICE_TOKEN:?INTERNAL_SERVICE_TOKEN is required}"
playbooks_repo_url="${PLAYBOOKS_REPO_URL:-https://github.com/x-evor/playbooks.git}"
playbooks_repo_ref="${PLAYBOOKS_REPO_REF:-c0f1a1c2ee00e4131db2484c8cc00b2bc4dc1263}"

if [[ ! -f "${artifact_path}" ]]; then
  echo "binary artifact not found: ${artifact_path}" >&2
  exit 1
fi

if [[ -z "${image_ref}" ]]; then
  echo "BILLING_SERVICE_IMAGE_REF or IMAGE_REF is required" >&2
  exit 1
fi

workdir="$(mktemp -d)"
trap 'rm -rf "${workdir}"' EXIT

git clone --depth 1 "${playbooks_repo_url}" "${workdir}/playbooks"
git -C "${workdir}/playbooks" fetch --depth 1 origin "${playbooks_repo_ref}"
git -C "${workdir}/playbooks" checkout --detach FETCH_HEAD

export ANSIBLE_HOST_KEY_CHECKING=false
export BILLING_SERVICE_BINARY_ARTIFACT="$(cd "$(dirname "${artifact_path}")" && pwd)/$(basename "${artifact_path}")"
export BILLING_SERVICE_IMAGE_REF="${image_ref}"
export DATABASE_URL="${database_url}"
export INTERNAL_SERVICE_TOKEN="${internal_service_token}"

cd "${workdir}/playbooks"
ansible-playbook -i inventory.ini deploy_billing_service.yml --limit "${target_host}"
