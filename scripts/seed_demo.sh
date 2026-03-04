#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

COMPOSE_FILE="${NEXUS_COMPOSE_FILE:-docker-compose.yml}"
compose() { docker compose -f "$COMPOSE_FILE" "$@"; }

DB_URL_EXEC="${NEXUS_DATABASE_URL_EXEC:-postgres://postgres:postgres@localhost:5432/nexus?sslmode=disable}"
SAAS_DB_URL_EXEC="${NEXUS_SAAS_DATABASE_URL_EXEC:-postgres://postgres:postgres@localhost:5432/nexus_saas?sslmode=disable}"
HTTP_PORT="${NEXUS_HTTP_PORT:-8080}"
SAAS_HTTP_PORT="${NEXUS_SAAS_HTTP_PORT:-8082}"

echo "Waiting for postgres..."
bash scripts/db/wait-for-db.sh "$DB_URL_EXEC"

echo "Waiting for saas postgres..."
for i in {1..60}; do
  if compose exec -T postgres-saas psql "$SAAS_DB_URL_EXEC" -c "select 1" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

echo "Waiting for nexus-core /readyz..."
for i in {1..60}; do
  if curl -fsS "http://localhost:${HTTP_PORT}/readyz" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

echo "Waiting for nexus-saas /health..."
for i in {1..60}; do
  if curl -fsS "http://localhost:${SAAS_HTTP_PORT}/health" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

# Fixed keys for local dev.
# Keep these stable so local environments are deterministic.
API_KEY="nexus-core-local-key"
OPERATOR_API_KEY="nexus-ai-operators-local-key"

API_KEY_HASH="$(
API_KEY="$API_KEY" python3 - <<'PY'
import hashlib, os
k = os.environ["API_KEY"]
print(hashlib.sha256(k.encode()).hexdigest())
PY
)"

export API_KEY
OPERATOR_API_KEY_HASH="$(
OPERATOR_API_KEY="$OPERATOR_API_KEY" python3 - <<'PY'
import hashlib, os
k = os.environ["OPERATOR_API_KEY"]
print(hashlib.sha256(k.encode()).hexdigest())
PY
)"

echo "Seeding org=demo, rotating api key hash..."

compose exec -T postgres psql "$DB_URL_EXEC" <<SQL
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
SELECT id, scope
FROM org_api_keys
JOIN (VALUES
  ('tools:read'),
  ('tools:write'),
  ('policy:read'),
  ('policy:write'),
  ('egress:read'),
  ('egress:write'),
  ('audit:read'),
  ('gateway:run'),
  ('gateway:simulate'),
  ('mcp:read'),
  ('mcp:call'),
  ('a2a:call'),
  ('admin:secrets'),
  ('admin:console:read'),
  ('admin:console:write')
) AS scopes(scope) ON true
WHERE org_id=:'org_id'::uuid AND name='demo-key';

DELETE FROM org_api_keys WHERE org_id=:'org_id'::uuid AND name='operator-key';
INSERT INTO org_api_keys(org_id, api_key_hash, name)
VALUES (:'org_id'::uuid, '${OPERATOR_API_KEY_HASH}', 'operator-key');

DELETE FROM org_api_key_scopes
WHERE api_key_id IN (
  SELECT id FROM org_api_keys WHERE org_id=:'org_id'::uuid AND name='operator-key'
);
INSERT INTO org_api_key_scopes(api_key_id, scope)
SELECT id, scope
FROM org_api_keys
JOIN (VALUES
  ('audit:read'),
  ('admin:console:read'),
  ('admin:console:write')
) AS scopes(scope) ON true
WHERE org_id=:'org_id'::uuid AND name='operator-key';

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
compose exec -T postgres psql "$DB_URL_EXEC" <<SQL
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

DEMO_ORG_ID="$(
compose exec -T postgres psql "$DB_URL_EXEC" -At -c "SELECT id FROM orgs WHERE name='demo' LIMIT 1;"
)"
if [[ -z "$DEMO_ORG_ID" ]]; then
  echo "Seed verification failed: demo org not found in core DB" >&2
  exit 1
fi

echo "Seeding org + API keys in nexus-saas..."
compose exec -T postgres-saas psql "$SAAS_DB_URL_EXEC" <<SQL
\set ON_ERROR_STOP on

INSERT INTO orgs(id, name)
VALUES ('${DEMO_ORG_ID}'::uuid, 'demo')
ON CONFLICT (id) DO UPDATE SET name=EXCLUDED.name;

DELETE FROM org_api_keys WHERE org_id='${DEMO_ORG_ID}'::uuid AND name='demo-key';
INSERT INTO org_api_keys(id, org_id, api_key_hash, name)
VALUES ((md5(random()::text || clock_timestamp()::text)::uuid), '${DEMO_ORG_ID}'::uuid, '${API_KEY_HASH}', 'demo-key');

DELETE FROM org_api_key_scopes
WHERE api_key_id IN (
  SELECT id FROM org_api_keys WHERE org_id='${DEMO_ORG_ID}'::uuid AND name='demo-key'
);
INSERT INTO org_api_key_scopes(id, api_key_id, scope)
SELECT (md5(random()::text || clock_timestamp()::text)::uuid), id, scope
FROM org_api_keys
JOIN (VALUES
  ('tools:read'),
  ('tools:write'),
  ('policy:read'),
  ('policy:write'),
  ('egress:read'),
  ('egress:write'),
  ('audit:read'),
  ('gateway:run'),
  ('gateway:simulate'),
  ('mcp:read'),
  ('mcp:call'),
  ('a2a:call'),
  ('admin:secrets'),
  ('admin:console:read'),
  ('admin:console:write')
) AS scopes(scope) ON true
WHERE org_id='${DEMO_ORG_ID}'::uuid AND name='demo-key';

DELETE FROM org_api_keys WHERE org_id='${DEMO_ORG_ID}'::uuid AND name='operator-key';
INSERT INTO org_api_keys(id, org_id, api_key_hash, name)
VALUES ((md5(random()::text || clock_timestamp()::text)::uuid), '${DEMO_ORG_ID}'::uuid, '${OPERATOR_API_KEY_HASH}', 'operator-key');

DELETE FROM org_api_key_scopes
WHERE api_key_id IN (
  SELECT id FROM org_api_keys WHERE org_id='${DEMO_ORG_ID}'::uuid AND name='operator-key'
);
INSERT INTO org_api_key_scopes(id, api_key_id, scope)
SELECT (md5(random()::text || clock_timestamp()::text)::uuid), id, scope
FROM org_api_keys
JOIN (VALUES
  ('audit:read'),
  ('admin:console:read'),
  ('admin:console:write')
) AS scopes(scope) ON true
WHERE org_id='${DEMO_ORG_ID}'::uuid AND name='operator-key';
SQL

seeded_count="$(
compose exec -T postgres psql "$DB_URL_EXEC" -At -c "SELECT count(*) FROM org_api_keys WHERE api_key_hash IN ('${API_KEY_HASH}', '${OPERATOR_API_KEY_HASH}');"
)"
if [[ "$seeded_count" != "2" ]]; then
  echo "Seed verification failed: expected 2 local API keys, got ${seeded_count}" >&2
  exit 1
fi

saas_seeded_count="$(
compose exec -T postgres-saas psql "$SAAS_DB_URL_EXEC" -At -c "SELECT count(*) FROM org_api_keys WHERE api_key_hash IN ('${API_KEY_HASH}', '${OPERATOR_API_KEY_HASH}');"
)"
if [[ "$saas_seeded_count" != "2" ]]; then
  echo "Seed verification failed: expected 2 local API keys in saas DB, got ${saas_seeded_count}" >&2
  exit 1
fi

echo "Seeding demo alert rule..."
curl -sS -H "X-NEXUS-CORE-KEY: ${API_KEY}" -H "Content-Type: application/json" \
  -d '{"name":"high-deny-rate","metric":"deny_rate","threshold":0.3,"webhook_url":"http://mock-tools:8081/echo","window_seconds":300,"cooldown_seconds":600,"enabled":true}' \
  "http://localhost:${SAAS_HTTP_PORT}/v1/alert-rules" >/dev/null 2>&1 || true

echo "NEXUS_DEMO_API_KEY=$API_KEY"

echo "NEXUS_OPERATOR_API_KEY=$OPERATOR_API_KEY"
