#!/usr/bin/env bash
set -euo pipefail

service_image_ref="${SERVICE_IMAGE_REF:?SERVICE_IMAGE_REF is required}"
tag="${service_image_ref##*:}"
commit="${tag#sha-}"

curl -fsS "https://billing-service.example.com/api/ping" | jq -e \
  --arg image "${service_image_ref}" \
  --arg tag "${tag}" \
  --arg commit "${commit}" \
  '.image == $image and .tag == $tag and .commit == $commit'
