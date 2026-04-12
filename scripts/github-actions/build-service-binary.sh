#!/usr/bin/env bash
set -euo pipefail

artifact_path="${BILLING_SERVICE_BINARY_ARTIFACT:?BILLING_SERVICE_BINARY_ARTIFACT is required}"
target_dir="$(dirname "${artifact_path}")"

mkdir -p "${target_dir}"

CGO_ENABLED="${CGO_ENABLED:-0}" \
GOOS="${GOOS:-linux}" \
GOARCH="${GOARCH:-amd64}" \
go build -buildvcs=false -o "${artifact_path}" ./cmd/billing-service

chmod 0755 "${artifact_path}"
