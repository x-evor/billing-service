#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
script_path="${repo_root}/scripts/github-actions/validate-release-traceability.sh"
tmpdir="$(mktemp -d)"
trap 'rm -rf "${tmpdir}"' EXIT

service_image_ref="ghcr.io/x-evor/billing-service:sha-0123456789abcdef0123456789abcdef01234567"

run_success_case() {
  local response_path="${tmpdir}/success.json"
  cat > "${response_path}" <<'EOF'
{"image":"ghcr.io/x-evor/billing-service:sha-0123456789abcdef0123456789abcdef01234567","tag":"sha-0123456789abcdef0123456789abcdef01234567","commit":"0123456789abcdef0123456789abcdef01234567","version":"0123456789abcdef0123456789abcdef01234567","status":"ok"}
EOF

  SERVICE_IMAGE_REF="${service_image_ref}" \
  RUNTIME_PING_URL="file://${response_path}" \
  bash "${script_path}"
}

run_failure_case() {
  local name="$1"
  local response_path="$2"

  if SERVICE_IMAGE_REF="${service_image_ref}" \
    RUNTIME_PING_URL="file://${response_path}" \
    bash "${script_path}"; then
    echo "expected ${name} to fail" >&2
    exit 1
  fi
}

cat > "${tmpdir}/empty.json" <<'EOF'
{"image":"","tag":"","commit":"","version":"","status":"ok"}
EOF

cat > "${tmpdir}/mismatch.json" <<'EOF'
{"image":"ghcr.io/x-evor/billing-service:sha-fedcba98765432100123456789abcdef01234567","tag":"sha-fedcba98765432100123456789abcdef01234567","commit":"fedcba98765432100123456789abcdef01234567","version":"fedcba98765432100123456789abcdef01234567","status":"ok"}
EOF

run_success_case
run_failure_case "empty runtime metadata" "${tmpdir}/empty.json"
run_failure_case "mismatched runtime metadata" "${tmpdir}/mismatch.json"
