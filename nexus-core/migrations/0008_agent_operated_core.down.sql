DROP TRIGGER IF EXISTS trg_audit_to_operational_events ON audit_events;
DROP FUNCTION IF EXISTS emit_tool_call_operational_event();

DROP TABLE IF EXISTS policy_versions;
DROP TABLE IF EXISTS policy_proposals;
DROP TABLE IF EXISTS incidents;
DROP TABLE IF EXISTS actions;
DROP TABLE IF EXISTS operational_events;
