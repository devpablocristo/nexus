from __future__ import annotations

from pathlib import Path

import pytest
import yaml

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
        assert user_message is not None
        assert prompt_id
        assert prompt_version
        assert max_output_tokens > 0
        return self._result


def make_settings() -> Settings:
    return Settings(
        LLM_BACKEND="fallback",
        NEXUS_AI_PROMPT_MAX_CONTEXT_CHARS=1600,
        NEXUS_AI_PROMPT_MAX_OUTPUT_TOKENS=256,
        NEXUS_AI_PROMPT_OBSERVABILITY_SAMPLE_RATE=1.0,
        NEXUS_AI_PROMPT_EVAL_MODE=True,
    )


def load_cases(filename: str) -> list[dict[str, object]]:
    path = Path(__file__).with_name(filename)
    raw = yaml.safe_load(path.read_text(encoding="utf-8"))
    return list(raw["cases"])


EVAL_CASES = load_cases("assistant_cases.yaml") + load_cases("diagnosis_cases.yaml")


@pytest.mark.asyncio
@pytest.mark.parametrize("case", EVAL_CASES, ids=[str(case["name"]) for case in EVAL_CASES])
async def test_prompt_runtime_eval_cases(case: dict[str, object]) -> None:
    llm_result = LLMResult(
        text=str(case.get("llm_text", "")),
        backend=str(case.get("backend", "fallback")),
        usage=LLMUsage(input_tokens=10, output_tokens=8),
        fallback_used="fallback_reason" in case,
        fallback_reason=str(case["fallback_reason"]) if "fallback_reason" in case else None,
    )
    runtime = PromptRuntime(settings=make_settings(), llm_client=FakeLLMClient(llm_result))

    result = await runtime.generate_summary(
        prompt_id=str(case["prompt_id"]),
        org_id="org-eval",
        actor="eval@example.com",
        query=str(case["query"]),
        engine_state=EngineState(latest_summary=str(case["latest_summary"])),
        request_id="eval-1",
    )

    assert str(case["expect_summary_contains"]).lower() in result.summary.lower()
    assert result.fallback_used is bool(case["expect_fallback_used"])
    assert list(result.guardrail_violations) == list(case["expect_guardrail"])
