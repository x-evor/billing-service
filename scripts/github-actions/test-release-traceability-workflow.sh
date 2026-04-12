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

if not any("SERVICE_IMAGE_REF: ${{ needs.build.outputs.service_image_ref }}" in line for line in validate_block):
    raise SystemExit("validate job must consume needs.build.outputs.service_image_ref")
PY
