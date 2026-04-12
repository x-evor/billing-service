#!/usr/bin/env bash
set -euo pipefail

service_image_ref="${SERVICE_IMAGE_REF:?SERVICE_IMAGE_REF is required}"
target_host="${STACK_TARGET_HOST:?STACK_TARGET_HOST is required}"
ssh_target="${RUNTIME_SSH_TARGET:-root@${target_host}}"
runtime_ping_path="${RUNTIME_PING_PATH:-http://127.0.0.1:8081/api/ping}"
tag="${service_image_ref##*:}"
commit="${tag#sha-}"

ssh -o BatchMode=yes "${ssh_target}" "systemctl is-active billing-service >/dev/null"

runtime_payload="$(ssh -o BatchMode=yes "${ssh_target}" "curl -fsS ${runtime_ping_path}")"

jq -e \
  --arg image "${service_image_ref}" \
  --arg tag "${tag}" \
  --arg commit "${commit}" \
  '
  (.image | type == "string" and length > 0) and
  (.tag | type == "string" and length > 0) and
  (.commit | type == "string" and length > 0) and
  (.version | type == "string" and length > 0) and
  .image == $image and
  .tag == $tag and
  .commit == $commit and
  .version == $commit
  ' <<<"${runtime_payload}" >/dev/null
