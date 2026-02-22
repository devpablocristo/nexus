#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

COMPOSE_FILE="${NEXUS_COMPOSE_FILE:-docker-compose.yml}"
compose() { docker compose -f "$COMPOSE_FILE" "$@"; }

DB_URL_EXEC="${NEXUS_DATABASE_URL_EXEC:-postgres://postgres:postgres@localhost:5432/nexus?sslmode=disable}"

echo "Waiting for services (postgres, redis, nexus-core, world-sim)..."
compose up -d --build --wait postgres redis nexus-core world-sim

if [[ "${WORLDSIM_SKIP_BASE_SEED:-0}" != "1" ]]; then
  echo "Running base demo seed (nexus-core/scripts/seed_demo.sh)..."
  (
    cd nexus-core
    NEXUS_COMPOSE_FILE="../${COMPOSE_FILE}" NEXUS_DATABASE_URL_EXEC="${DB_URL_EXEC}" bash scripts/seed_demo.sh >/dev/null
  )
fi

echo "Applying world-sim schema..."
compose exec -T postgres psql "$DB_URL_EXEC" < world-sim/migrations/0001_worldsim.up.sql

echo "Seeding world.observe/world.move tools + egress + policies..."
compose exec -T postgres psql "$DB_URL_EXEC" <<'SQL'
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

WITH upsert_observe AS (
  INSERT INTO tools(org_id, name, kind, description, method, url, input_schema_json, output_schema_json, action_type, classification, sensitivity, risk_level, enabled)
  VALUES (
    :'org_id'::uuid,
    'world.observe',
    'http',
    'WorldSim deterministic observe tool',
    'POST',
    'http://world-sim:8087/tools/world.observe',
    '{"type":"object","properties":{"org_id":{"type":"string"},"agent_id":{"type":"string"},"run_id":{"type":"string"},"step_id":{"type":"integer"},"request_id":{"type":"string"}},"required":["org_id","agent_id","run_id","step_id"],"additionalProperties":true}'::jsonb,
    '{"type":"object","properties":{"request_id":{"type":"string"},"status":{"type":"string"},"error":{"type":["object","null"]},"data":{"type":"object"}},"required":["request_id","status"],"additionalProperties":true}'::jsonb,
    'read',
    'internal',
    'low',
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
    sensitivity=EXCLUDED.sensitivity,
    risk_level=EXCLUDED.risk_level,
    enabled=EXCLUDED.enabled
  RETURNING id
),
upsert_move AS (
  INSERT INTO tools(org_id, name, kind, description, method, url, input_schema_json, output_schema_json, action_type, classification, sensitivity, risk_level, enabled)
  VALUES (
    :'org_id'::uuid,
    'world.move',
    'http',
    'WorldSim deterministic move tool',
    'POST',
    'http://world-sim:8087/tools/world.move',
    '{"type":"object","properties":{"org_id":{"type":"string"},"agent_id":{"type":"string"},"run_id":{"type":"string"},"step_id":{"type":"integer"},"request_id":{"type":"string"},"direction":{"type":"object","properties":{"x":{"type":"number"},"y":{"type":"number"}},"additionalProperties":false},"target":{"type":"object","properties":{"x":{"type":"number"},"y":{"type":"number"}},"additionalProperties":false},"speed":{"type":"number"}},"required":["org_id","agent_id","run_id","step_id"],"additionalProperties":true}'::jsonb,
    '{"type":"object","properties":{"request_id":{"type":"string"},"status":{"type":"string"},"error":{"type":["object","null"]},"data":{"type":"object"}},"required":["request_id","status"],"additionalProperties":true}'::jsonb,
    'write',
    'internal',
    'low',
    2,
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

INSERT INTO tool_egress_rules(org_id, tool_id, host, enabled)
SELECT :'org_id'::uuid, t.id, 'world-sim', true
FROM tools t
WHERE t.org_id = :'org_id'::uuid
  AND t.name IN ('world.observe', 'world.move')
ON CONFLICT (org_id, tool_id, host) DO UPDATE
SET enabled = EXCLUDED.enabled;

SELECT id AS world_move_tool_id
FROM tools
WHERE org_id = :'org_id'::uuid
  AND name = 'world.move'
LIMIT 1
\gset

DELETE FROM policies
WHERE org_id = :'org_id'::uuid
  AND tool_id = :'world_move_tool_id'::uuid;

INSERT INTO tenant_settings(org_id, plan_code, hard_limits_json, updated_by)
VALUES (
  :'org_id'::uuid,
  'enterprise',
  '{"tools_max":250,"run_rpm":5000,"audit_retention_days":365}'::jsonb,
  'worldsim-seed'
)
ON CONFLICT (org_id) DO UPDATE SET
  plan_code = EXCLUDED.plan_code,
  hard_limits_json = EXCLUDED.hard_limits_json,
  updated_by = EXCLUDED.updated_by;

INSERT INTO policies(org_id, tool_id, effect, priority, conditions_json, limits_json, reason_template, enabled)
VALUES
  (
    :'org_id'::uuid,
    :'world_move_tool_id'::uuid,
    'deny',
    5,
    '{"path":"input.agent_id","op":"eq","value":"agent-001"}'::jsonb,
    '{}'::jsonb,
    'Door jam policy: agent-001 temporarily denied',
    true
  ),
  (
    :'org_id'::uuid,
    :'world_move_tool_id'::uuid,
    'allow',
    20,
    '{}'::jsonb,
    '{"rate_limit":{"per_minute":5000}}'::jsonb,
    'World move allowed',
    true
  );
SQL

echo "WorldSim seed complete."
