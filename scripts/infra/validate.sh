#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
INFRA_DIR="${ROOT_DIR}/infra"
TERRAFORM_IMAGE="${TERRAFORM_IMAGE:-hashicorp/terraform:1.11.4}"
WORK_DIR="$(mktemp -d)"
HOST_UID="$(id -u)"
HOST_GID="$(id -g)"

cleanup() {
  rm -rf "${WORK_DIR}"
}
trap cleanup EXIT

cp -R "${INFRA_DIR}/." "${WORK_DIR}/"

python3 - "${INFRA_DIR}" <<'PY'
from pathlib import Path
import sys

infra_dir = Path(sys.argv[1])
db_main = (infra_dir / "modules" / "database" / "main.tf").read_text()
root_vars = (infra_dir / "variables.tf").read_text()
cdn_vars = (infra_dir / "modules" / "cdn" / "variables.tf").read_text()

required_db_markers = [
    "deletion_protection          = true",
    "skip_final_snapshot          = false",
    "copy_tags_to_snapshot        = true",
]
for marker in required_db_markers:
    if db_main.count(marker) < 2:
        raise SystemExit(f"missing required RDS guardrail: {marker}")

if 'variable "tower_force_destroy"' not in root_vars or 'default     = false' not in root_vars:
    raise SystemExit("tower_force_destroy must exist with default false")

if 'variable "force_destroy_bucket"' not in cdn_vars or 'default = false' not in cdn_vars:
    raise SystemExit("cdn force_destroy_bucket must default to false")
PY

python3 - "${WORK_DIR}/main.tf" <<'PY'
from pathlib import Path
import re
import sys

path = Path(sys.argv[1])
text = path.read_text()
updated, count = re.subn(
    r'(?ms)\n\s*backend\s+"s3"\s*\{.*?^\s*\}\n',
    "\n",
    text,
    count=1,
)
if count != 1:
    raise SystemExit("failed to strip backend block from infra/main.tf")
path.write_text(updated)
PY

docker run --rm --user "${HOST_UID}:${HOST_GID}" -v "${WORK_DIR}:/workspace" -w /workspace "${TERRAFORM_IMAGE}" init -backend=false -reconfigure >/tmp/nexus-tf-init.log
docker run --rm --user "${HOST_UID}:${HOST_GID}" -v "${WORK_DIR}:/workspace" -w /workspace "${TERRAFORM_IMAGE}" validate >/tmp/nexus-tf-validate.log
docker run --rm \
  --user "${HOST_UID}:${HOST_GID}" \
  -e AWS_ACCESS_KEY_ID=dummy \
  -e AWS_SECRET_ACCESS_KEY=dummy \
  -e AWS_SESSION_TOKEN=dummy \
  -e AWS_EC2_METADATA_DISABLED=true \
  -v "${WORK_DIR}:/workspace" \
  -w /workspace \
  "${TERRAFORM_IMAGE}" \
  plan \
  -var-file=terraform.tfvars.example \
  -var=aws_skip_credentials_validation=true \
  -var=aws_skip_requesting_account_id=true \
  -var=aws_skip_metadata_api_check=true \
  -var=aws_skip_region_validation=true \
  -input=false \
  -lock=false \
  >/tmp/nexus-tf-plan.log

cat /tmp/nexus-tf-validate.log
tail -n 40 /tmp/nexus-tf-plan.log
