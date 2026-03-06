from __future__ import annotations

import pytest

from app.core.config import Settings
from app.services.engine import EngineState
from app.services.llm_client import LLMResult, LLMUsage
from app.services.prompt_runtime import PromptRuntime


class FakeLLMClient:
    def __init__(self, result: LLMResult) -> None:
        self._result = result

    async def query(
        self,
        system_prompt: str,
        user_message: str,
        prompt_id: str,
        prompt_version: str,
        max_output_tokens: int,
    ) -> LLMResult:
        assert system_prompt
        assert user_message
        assert prompt_id
        assert prompt_version
        assert max_output_tokens > 0
        return self._result


class FakeSaaSClient:
    def __init__(self, payload: dict[str, object]) -> None:
        self.payload = payload
        self.calls: list[str] = []

    async def get_assistant_context(self, org_id: str) -> dict[str, object]:
        self.calls.append(org_id)
        return self.payload


def make_settings(**overrides: object) -> Settings:
    defaults: dict[str, object] = {
        "LLM_BACKEND": "fallback",
        "NEXUS_AI_PROMPT_MAX_CONTEXT_CHARS": 1600,
        "NEXUS_AI_PROMPT_MAX_OUTPUT_TOKENS": 256,
        "NEXUS_AI_PROMPT_OBSERVABILITY_SAMPLE_RATE": 1.0,
    }
    defaults.update(overrides)
    return Settings(**defaults)


@pytest.mark.asyncio
async def test_prompt_runtime_uses_deterministic_fallback_on_guardrail_violation() -> None:
    runtime = PromptRuntime(
        settings=make_settings(),
        llm_client=FakeLLMClient(
            LLMResult(
                text="We executed the mitigation and bypassed approval.",
                backend="anthropic",
                usage=LLMUsage(input_tokens=12, output_tokens=8),
                fallback_used=False,
            )
        ),
    )

    result = await runtime.generate_summary(
        prompt_id="assistant_system",
        org_id="org-123",
        actor="alice@example.com",
        query="Did we run anything already?",
        engine_state=EngineState(latest_summary="No actions recorded yet."),
        request_id="req-1",
    )

    assert result.fallback_used is True
    assert result.guardrail_violations == ("execution_claim", "bypass_suggestion")
    assert "deterministic fallback mode" in result.summary


@pytest.mark.asyncio
async def test_prompt_runtime_returns_safe_llm_summary_when_output_is_valid() -> None:
    runtime = PromptRuntime(
        settings=make_settings(),
        llm_client=FakeLLMClient(
            LLMResult(
                text="Current operator status is stable. No confirmed incidents are visible in the provided context.",
                backend="anthropic",
                usage=LLMUsage(input_tokens=20, output_tokens=14),
                fallback_used=False,
            )
        ),
    )

    result = await runtime.generate_summary(
        prompt_id="assistant_system",
        org_id="org-123",
        actor=None,
        query="What is the current status?",
        engine_state=EngineState(latest_summary="Processed 10 events with low deny ratio."),
        request_id="req-2",
    )

    assert result.fallback_used is False
    assert result.guardrail_violations == ()
    assert "stable" in result.summary


@pytest.mark.asyncio
async def test_prompt_runtime_fetches_saas_assistant_context() -> None:
    fake_saas_client = FakeSaaSClient(
        {
            "tenant": {"plan_code": "growth", "status": "active", "hard_limits": {"run_rpm": 1200}},
            "billing": {"billing_status": "active", "usage_period": "2026-03", "usage": {"api_calls": 12}},
            "incidents": {"open_count": 1, "items": [{"severity": "HIGH", "status": "open", "title": "Deny spike"}]},
            "actions": {"active_count": 1, "items": [{"action_type": "throttle_tenant_rpm", "status": "active", "scope_type": "tenant"}]},
            "proposals": {"pending_count": 1, "items": [{"status": "pending", "rationale": "Tighten egress"}]},
            "events": {"recent_count": 1, "items": [{"event_type": "incident.opened", "summary": "Deny spike"}]},
        }
    )
    runtime = PromptRuntime(
        settings=make_settings(),
        llm_client=FakeLLMClient(
            LLMResult(
                text="Tenant is active with one open incident and one active mitigation.",
                backend="anthropic",
                usage=LLMUsage(input_tokens=20, output_tokens=14),
                fallback_used=False,
            )
        ),
        saas_client=fake_saas_client,
    )

    result = await runtime.generate_summary(
        prompt_id="assistant_system",
        org_id="org-tenant",
        actor=None,
        query="What changed?",
        engine_state=EngineState(latest_summary="Processed 3 events."),
        request_id="req-3",
    )

    assert fake_saas_client.calls == ["org-tenant"]
    assert result.context.assistant_overview["tenant_status"] == "active"
    assert result.context.assistant_overview["open_incidents"] == "1"
