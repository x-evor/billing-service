#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
workflow_path="${repo_root}/.github/workflows/release-traceability.yml"

python3 - "${workflow_path}" <<'PY'
import sys
from pathlib import Path

workflow_path = Path(sys.argv[1])
lines = workflow_path.read_text().splitlines()

validate_start = None
validate_end = len(lines)
for index, line in enumerate(lines):
    if line.startswith("  validate:"):
        validate_start = index
        continue
    if validate_start is not None and index > validate_start and line.startswith("  ") and not line.startswith("    "):
        validate_end = index
        break

if validate_start is None:
    raise SystemExit("validate job not found")

validate_block = lines[validate_start:validate_end]
if not any(line.strip() == "- build" for line in validate_block):
    raise SystemExit("validate job must depend on build")
if not any(line.strip() == "- deploy" for line in validate_block):
    raise SystemExit("validate job must depend on deploy")

build_block = []
deploy_block = []
current_job = None

for line in lines:
    if line.startswith("  build:"):
        current_job = "build"
    elif line.startswith("  deploy:"):
        current_job = "deploy"
    elif line.startswith("  validate:"):
        current_job = "validate"
    elif line.startswith("  ") and not line.startswith("    "):
        current_job = None

    if current_job == "build":
        build_block.append(line)
    elif current_job == "deploy":
        deploy_block.append(line)

if not any("Upload billing-service binary artifact" in line for line in build_block):
    raise SystemExit("build job must upload the billing-service binary artifact")

if not any("Download billing-service binary artifact" in line for line in deploy_block):
    raise SystemExit("deploy job must download the billing-service binary artifact")

if not any("BILLING_SERVICE_IMAGE_REF: ${{ needs.build.outputs.service_image_ref }}" in line for line in deploy_block):
    raise SystemExit("deploy job must consume needs.build.outputs.service_image_ref")

if not any("SERVICE_IMAGE_REF: ${{ needs.build.outputs.service_image_ref }}" in line for line in validate_block):
    raise SystemExit("validate job must consume needs.build.outputs.service_image_ref")
PY
