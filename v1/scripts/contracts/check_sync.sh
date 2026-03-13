#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

pairs=(
  "pkgs/contracts/openapi.nexus-core.snapshot.yaml|data-plane/docs/openapi.yaml"
  "pkgs/contracts/openapi.nexus-saas.snapshot.yaml|control-plane/docs/openapi.yaml"
  "pkgs/contracts/openapi.nexus-core.snapshot.yaml|tower/public/downloads/nexus-core.openapi.yaml"
  "pkgs/contracts/openapi.nexus-saas.snapshot.yaml|tower/public/downloads/nexus-saas.openapi.yaml"
  "docs/postman/nexus-core.postman_collection.json|tower/public/downloads/nexus-core.postman_collection.json"
  "docs/postman/nexus-saas.postman_collection.json|tower/public/downloads/nexus-saas.postman_collection.json"
)

status=0

for pair in "${pairs[@]}"; do
  src_rel="${pair%%|*}"
  dst_rel="${pair##*|}"
  src="${ROOT_DIR}/${src_rel}"
  dst="${ROOT_DIR}/${dst_rel}"

  if [[ ! -f "${src}" ]]; then
    echo "[FAIL] missing ${src_rel}"
    status=1
    continue
  fi
  if [[ ! -f "${dst}" ]]; then
    echo "[FAIL] missing ${dst_rel}"
    status=1
    continue
  fi

  if cmp -s "${src}" "${dst}"; then
    echo "[PASS] ${src_rel} == ${dst_rel}"
    continue
  fi

  echo "[FAIL] drift detected: ${src_rel} != ${dst_rel}"
  diff -u "${src}" "${dst}" || true
  status=1
done

exit "${status}"
