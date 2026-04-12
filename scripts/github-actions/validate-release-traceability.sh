#!/usr/bin/env bash
set -euo pipefail

service_image_ref="${SERVICE_IMAGE_REF:?SERVICE_IMAGE_REF is required}"
runtime_ping_url="${RUNTIME_PING_URL:?RUNTIME_PING_URL is required}"
tag="${service_image_ref##*:}"
commit="${tag#sha-}"

curl -fsS "${runtime_ping_url}" | jq -e \
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
  ' >/dev/null
