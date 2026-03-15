#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/common.sh
source "${SCRIPT_DIR}/../lib/common.sh"

require_cmd grep

shared_import='github.com/devpablocristo/nexus/v2/pkgs/go-pkg/handlers'

assert_contains() {
  local file="$1"
  local pattern="$2"
  if ! grep -Fq -- "${pattern}" "${file}"; then
    echo "missing pattern in ${file}: ${pattern}" >&2
    exit 1
  fi
}

assert_not_contains() {
  local file="$1"
  local pattern="$2"
  if grep -Eq -- "${pattern}" "${file}"; then
    echo "forbidden pattern in ${file}: ${pattern}" >&2
    exit 1
  fi
}

check_crud_handler() {
  local file="$1"
  local resource="$2"

  check_http_handler "${file}"
  assert_contains "${file}" "${shared_import}"
  assert_contains "${file}" "sharedhandlers.DecodeJSON("
  assert_contains "${file}" "sharedhandlers.WriteJSON("
  assert_contains "${file}" "sharedhandlers.ParseArchived("
  assert_contains "${file}" "POST /v1/${resource}"
  assert_contains "${file}" "GET /v1/${resource}"
  assert_contains "${file}" "GET /v1/${resource}/{id}"
  assert_contains "${file}" "PATCH /v1/${resource}/{id}"
  assert_contains "${file}" "DELETE /v1/${resource}/{id}"
  assert_contains "${file}" "POST /v1/${resource}/{id}/archive"
  assert_contains "${file}" "POST /v1/${resource}/{id}/restore"
  assert_contains "${file}" "w.Header().Set(\"Location\", \"/v1/${resource}/"
}

check_http_handler() {
  local file="$1"

  assert_contains "${file}" "${shared_import}"
  assert_contains "${file}" "sharedhandlers.WriteJSON("
  assert_not_contains "${file}" 'func (decodeJSON|writeJSON|parseOptionalBool|parseArchived|parseLimit|decodeApprovalJSON|writeApprovalJSON)\('
  assert_not_contains "${file}" 'internal/helpers'
  assert_not_contains "${file}" 'internal/httpx'
}

check_http_handler "${DATA_PLANE_DIR}/internal/action/handler.go"
check_http_handler "${DATA_PLANE_DIR}/internal/approval/handler.go"
check_http_handler "${DATA_PLANE_DIR}/internal/gateway/handler.go"
check_crud_handler "${CONTROL_PLANE_DIR}/internal/resources/handler.go" "resources"
check_crud_handler "${CONTROL_PLANE_DIR}/internal/policies/handler.go" "policies"
check_crud_handler "${CONTROL_WORKERS_DIR}/internal/incidents/handler.go" "incidents"
check_crud_handler "${CONTROL_WORKERS_DIR}/internal/alerts/handler.go" "alerts"
check_crud_handler "${DATA_PLANE_DIR}/internal/policy/handler.go" "policies"
