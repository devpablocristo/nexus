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
