#!/usr/bin/env bash
set -euo pipefail

DB_URL="${NEXUS_DATABASE_URL:-postgres://postgres:postgres@postgres:5432/nexus?sslmode=disable}"
HTTP_PORT="${NEXUS_HTTP_PORT:-8080}"

echo "Waiting for postgres..."
bash scripts/db/wait-for-db.sh "$DB_URL"

echo "Waiting for nexus-gateway /readyz..."
for i in {1..60}; do
  if curl -fsS "http://localhost:${HTTP_PORT}/readyz" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

API_KEY="$(
python3 - <<'PY'
import secrets
print(secrets.token_hex(32))
PY
)"

API_KEY_HASH="$(
API_KEY="$API_KEY" python3 - <<'PY'
import hashlib, os
k = os.environ["API_KEY"]
print(hashlib.sha256(k.encode()).hexdigest())
PY
)"

export API_KEY

echo "Seeding org=demo, rotating api key hash..."

docker compose exec -T postgres psql "$DB_URL" <<SQL
\set ON_ERROR_STOP on

WITH ins AS (
  INSERT INTO orgs(name)
  SELECT 'demo'
  WHERE NOT EXISTS (SELECT 1 FROM orgs WHERE name='demo')
  RETURNING id
)
SELECT id AS org_id FROM ins
UNION ALL
SELECT id AS org_id FROM orgs WHERE name='demo'
LIMIT 1
\gset

-- Keep demo org deterministic: remove non-core tools (cascades policies/audit for those tools).
DELETE FROM tools
WHERE org_id=:'org_id'::uuid
  AND name NOT IN ('echo', 'transfer');

DELETE FROM org_api_keys WHERE org_id=:'org_id'::uuid AND name='demo-key';
INSERT INTO org_api_keys(org_id, api_key_hash, name)
VALUES (:'org_id'::uuid, '${API_KEY_HASH}', 'demo-key');

DELETE FROM org_api_key_scopes
WHERE api_key_id IN (
  SELECT id FROM org_api_keys WHERE org_id=:'org_id'::uuid AND name='demo-key'
);
INSERT INTO org_api_key_scopes(api_key_id, scope)
SELECT id, 'admin:secrets'
FROM org_api_keys
WHERE org_id=:'org_id'::uuid AND name='demo-key';

WITH upsert_echo AS (
  INSERT INTO tools(org_id, name, kind, description, method, url, input_schema_json, output_schema_json, action_type, classification, risk_level, enabled)
  VALUES (
    :'org_id'::uuid,
    'echo',
    'http',
    'demo echo tool',
    'POST',
    'http://mock-tools:8081/echo',
    '{"type":"object"}'::jsonb,
    NULL,
    'read',
    'internal',
    1,
    true
  )
  ON CONFLICT (org_id, name) DO UPDATE SET
    kind=EXCLUDED.kind,
    description=EXCLUDED.description,
    method=EXCLUDED.method,
    url=EXCLUDED.url,
    input_schema_json=EXCLUDED.input_schema_json,
    output_schema_json=EXCLUDED.output_schema_json,
    action_type=EXCLUDED.action_type,
    classification=EXCLUDED.classification,
    sensitivity='low',
    risk_level=EXCLUDED.risk_level,
    enabled=EXCLUDED.enabled
  RETURNING id
),
upsert_transfer AS (
  INSERT INTO tools(org_id, name, kind, description, method, url, input_schema_json, output_schema_json, action_type, classification, sensitivity, risk_level, enabled)
  VALUES (
    :'org_id'::uuid,
    'transfer',
    'http',
    'demo transfer tool',
    'POST',
    'http://mock-tools:8081/transfer',
    '{"type":"object","properties":{"amount":{"type":"number"}},"required":["amount"],"additionalProperties":true}'::jsonb,
    NULL,
    'write',
    'external',
    'high',
    3,
    true
  )
  ON CONFLICT (org_id, name) DO UPDATE SET
    kind=EXCLUDED.kind,
    description=EXCLUDED.description,
    method=EXCLUDED.method,
    url=EXCLUDED.url,
    input_schema_json=EXCLUDED.input_schema_json,
    output_schema_json=EXCLUDED.output_schema_json,
    action_type=EXCLUDED.action_type,
    classification=EXCLUDED.classification,
    sensitivity=EXCLUDED.sensitivity,
    risk_level=EXCLUDED.risk_level,
    enabled=EXCLUDED.enabled
  RETURNING id
)
SELECT 1;
SQL

echo "Seeding policies for transfer..."
docker compose exec -T postgres psql "$DB_URL" <<SQL
\set ON_ERROR_STOP on

WITH ins AS (
  INSERT INTO orgs(name)
  SELECT 'demo'
  WHERE NOT EXISTS (SELECT 1 FROM orgs WHERE name='demo')
  RETURNING id
)
SELECT id AS org_id FROM ins
UNION ALL
SELECT id AS org_id FROM orgs WHERE name='demo'
LIMIT 1
\gset

SELECT id AS transfer_tool_id FROM tools WHERE org_id=:'org_id'::uuid AND name='transfer' LIMIT 1
\gset

-- Keep only transfer policies in demo baseline.
DELETE FROM policies
WHERE org_id=:'org_id'::uuid
  AND tool_id <> :'transfer_tool_id'::uuid;

DELETE FROM policies WHERE org_id=:'org_id'::uuid AND tool_id=:'transfer_tool_id'::uuid;

INSERT INTO policies(org_id, tool_id, effect, priority, conditions_json, limits_json, reason_template, enabled)
VALUES
  (
    :'org_id'::uuid,
    :'transfer_tool_id'::uuid,
    'deny',
    5,
    '{"all":[{"path":"tool.classification","op":"eq","value":"external"},{"path":"context.dlp.credit_card.count","op":"gt","value":0}]}'::jsonb,
    '{}'::jsonb,
    'Denied: card data cannot be sent to external tools',
    true
  ),
  (
    :'org_id'::uuid,
    :'transfer_tool_id'::uuid,
    'deny',
    10,
    '{"path":"input.amount","op":"gt","value":1000}'::jsonb,
    '{}'::jsonb,
    'Denied because amount too high',
    true
  ),
  (
    :'org_id'::uuid,
    :'transfer_tool_id'::uuid,
    'allow',
    20,
    '{"all":[{"path":"input.amount","op":"lte","value":1000},{"path":"context.user_id","op":"exists"}]}'::jsonb,
    '{"require_idempotency":true}'::jsonb,
    'Allowed',
    true
  );
SQL

echo "NEXUS_DEMO_API_KEY=$API_KEY"
