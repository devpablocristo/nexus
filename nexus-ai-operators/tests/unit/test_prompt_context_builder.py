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
