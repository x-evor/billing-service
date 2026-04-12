#!/usr/bin/env bash
set -euo pipefail

docker build \
  --tag "${SERVICE_IMAGE_REF:?SERVICE_IMAGE_REF is required}" \
  --tag "${SERVICE_IMAGE_LATEST_REF:?SERVICE_IMAGE_LATEST_REF is required}" \
  .
