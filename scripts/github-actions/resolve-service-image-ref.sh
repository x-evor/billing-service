#!/usr/bin/env bash
set -euo pipefail

full_sha="${GITHUB_SHA:?GITHUB_SHA is required}"
tag="sha-${full_sha}"
image_ref="ghcr.io/${GITHUB_REPOSITORY:?GITHUB_REPOSITORY is required}:${tag}"

printf 'service_image_ref=%s\n' "${image_ref}" >> "${GITHUB_OUTPUT}"
printf 'service_image_tag=%s\n' "${tag}" >> "${GITHUB_OUTPUT}"
printf 'service_image_commit=%s\n' "${full_sha}" >> "${GITHUB_OUTPUT}"
