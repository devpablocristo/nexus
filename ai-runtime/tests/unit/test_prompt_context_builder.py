from app.services.engine import EngineState
from app.services.prompt_context_builder import PromptContextBuilder


def test_prompt_context_builder_redacts_sensitive_values() -> None:
    builder = PromptContextBuilder(max_chars=1200)

    context = builder.build(
        prompt_id="assistant_system",
        org_id="org-123",
        actor="alice@example.com",
        query="Check sk_test_123456789 and token aaa.bbb.ccc for +1 (555) 010-0101",
        engine_state=EngineState(
            cursor=42,
            last_action_at=0,
            latest_summary="Latest contact alice@example.com used sk_live_secret and jwt aaa.bbb.ccc",
        ),
    )

    assert context.actor == "[REDACTED]"
    assert "[REDACTED]" in context.query
    assert "[REDACTED]" in context.latest_summary
    assert "alice@example.com" not in context.block
    assert "aaa.bbb.ccc" not in context.block


def test_prompt_context_builder_includes_tenant_snapshot_context() -> None:
    builder = PromptContextBuilder(max_chars=1600)

    context = builder.build(
        prompt_id="assistant_system",
        org_id="org-123",
        actor=None,
        query="Show my posture",
        engine_state=EngineState(cursor=7, last_action_at=0, latest_summary="Processed recent events."),
        assistant_context={
            "tenant": {
                "plan_code": "growth",
                "status": "active",
                "hard_limits": {"tools_max": 75, "run_rpm": 1200, "audit_retention_days": 90},
            },
            "billing": {
                "billing_status": "active",
                "usage_period": "2026-03",
                "usage": {"api_calls": 12, "events_ingested": 4, "incidents_opened": 1, "actions_executed": 2},
            },
            "incidents": {
                "open_count": 1,
                "items": [{"severity": "HIGH", "status": "open", "title": "Deny spike for admin@acme.com"}],
            },
            "actions": {"active_count": 1, "items": [{"action_type": "throttle_tenant_rpm", "status": "active", "scope_type": "tenant"}]},
            "proposals": {"pending_count": 1, "items": [{"status": "pending", "rationale": "Tighten egress review"}]},
            "events": {"recent_count": 1, "items": [{"event_type": "incident.opened", "summary": "Deny spike for admin@acme.com"}]},
        },
    )

    assert context.assistant_overview["plan_code"] == "growth"
    assert context.assistant_overview["open_incidents"] == "1"
    assert "tenant_snapshot:" in context.block
    assert "billing_snapshot:" in context.block
    assert "open_incidents:" in context.block
    assert "admin@acme.com" not in context.block
