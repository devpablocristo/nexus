from __future__ import annotations

from dataclasses import dataclass
import logging
import random
import re
import time

from app.adapters.nexus_saas_client import NexusSaaSClient
from app.core.config import Settings
from app.core.metrics import (
    PROMPT_FALLBACKS,
    PROMPT_GUARDRAIL_VIOLATIONS,
    PROMPT_LATENCY,
    PROMPT_REQUESTS,
    PROMPT_TOKENS,
)
from app.services.engine import EngineState
from app.services.llm_client import LLMClient, LLMResult
from app.services.prompt_context_builder import PromptContext, PromptContextBuilder
from app.services.prompt_registry import PromptRegistry


logger = logging.getLogger(__name__)

EXECUTION_CLAIM_PATTERNS = (
    re.compile(
        r"\b(?:i|we)\b(?:\s+\w+){0,3}\s+(?:executed|ran|applied|approved|rejected|opened|closed|deleted|disabled|enabled|rolled\s+back)\b",
        re.IGNORECASE,
    ),
    re.compile(r"\b(?:action|incident|proposal)\s+(?:was|were)\s+(?:applied|approved|rejected|opened|closed)\b", re.IGNORECASE),
)
BYPASS_PATTERNS = (
    re.compile(r"\bbypass(?:ed|ing)?\b", re.IGNORECASE),
    re.compile(
        r"\b(?:ignore|disabl(?:e|ed)|skip)\b.{0,40}\b(?:policy|approval|dlp|rate limit|egress|audit|enforcement)\b",
        re.IGNORECASE,
    ),
)


@dataclass(frozen=True)
class PromptRuntimeResult:
    summary: str
    backend: str
    prompt_id: str
    prompt_version: str
    fallback_used: bool
    fallback_reason: str | None
    guardrail_violations: tuple[str, ...]
    context: PromptContext


class PromptRuntime:
    def __init__(
        self,
        settings: Settings,
        llm_client: LLMClient | None = None,
        registry: PromptRegistry | None = None,
        context_builder: PromptContextBuilder | None = None,
        saas_client: NexusSaaSClient | None = None,
    ) -> None:
        self._settings = settings
        self._llm_client = llm_client or LLMClient(settings)
        self._registry = registry or PromptRegistry()
        self._context_builder = context_builder or PromptContextBuilder(settings.ai_prompt_max_context_chars)
        self._saas_client = saas_client

    async def generate_summary(
        self,
        prompt_id: str,
        org_id: str,
        actor: str | None,
        query: str,
        engine_state: EngineState,
        request_id: str,
    ) -> PromptRuntimeResult:
        prompt = self._registry.get(prompt_id, self._default_version(prompt_id))
        assistant_context = None
        if prompt.prompt_id == "assistant_system" and self._saas_client is not None:
            assistant_context = await self._load_assistant_context(org_id, request_id)
        context = self._context_builder.build(
            prompt.prompt_id,
            org_id,
            actor,
            query,
            engine_state,
            assistant_context=assistant_context,
        )
        system_prompt = f"{prompt.body}\n\n## Safe Runtime Context\n{context.block}"

        started_at = time.perf_counter()
        llm_result = await self._llm_client.query(
            system_prompt=system_prompt,
            user_message=context.query,
            prompt_id=prompt.prompt_id,
            prompt_version=prompt.version,
            max_output_tokens=self._settings.ai_prompt_max_output_tokens,
        )
        duration = time.perf_counter() - started_at

        labels = {
            "backend": llm_result.backend,
            "prompt_id": prompt.prompt_id,
            "prompt_version": prompt.version,
        }
        PROMPT_REQUESTS.labels(**labels).inc()
        PROMPT_LATENCY.labels(**labels).observe(duration)
        if llm_result.usage.input_tokens > 0:
            PROMPT_TOKENS.labels(direction="input", **labels).inc(llm_result.usage.input_tokens)
        if llm_result.usage.output_tokens > 0:
            PROMPT_TOKENS.labels(direction="output", **labels).inc(llm_result.usage.output_tokens)

        summary, violations = self._apply_guardrails(llm_result, context)
        fallback_used = llm_result.fallback_used or bool(violations)
        fallback_reason = llm_result.fallback_reason
        if violations:
            fallback_reason = f"guardrail:{violations[0]}"
            for violation in violations:
                PROMPT_GUARDRAIL_VIOLATIONS.labels(rule=violation, **labels).inc()

        if fallback_used:
            PROMPT_FALLBACKS.labels(reason=fallback_reason or "unspecified", **labels).inc()

        self._log_request(
            request_id=request_id,
            org_id=context.org_id,
            actor=context.actor,
            llm_result=llm_result,
            prompt_id=prompt.prompt_id,
            prompt_version=prompt.version,
            fallback_used=fallback_used,
            fallback_reason=fallback_reason,
            guardrail_violations=violations,
        )

        return PromptRuntimeResult(
            summary=summary,
            backend=llm_result.backend,
            prompt_id=prompt.prompt_id,
            prompt_version=prompt.version,
            fallback_used=fallback_used,
            fallback_reason=fallback_reason,
            guardrail_violations=violations,
            context=context,
        )

    def _default_version(self, prompt_id: str) -> str:
        mapping = {
            "assistant_system": self._settings.assistant_prompt_version,
            "diagnosis_system": self._settings.diagnosis_prompt_version,
            "comms_system": self._settings.comms_prompt_version,
            "executive_qa_system": self._settings.executive_qa_prompt_version,
        }
        return mapping.get(prompt_id, "v1")

    def _apply_guardrails(
        self,
        llm_result: LLMResult,
        context: PromptContext,
    ) -> tuple[str, tuple[str, ...]]:
        if llm_result.fallback_used:
            return self._deterministic_summary(context), ()

        candidate = llm_result.text.strip()
        violations: list[str] = []
        if not candidate:
            violations.append("empty_output")
        if any(pattern.search(candidate) for pattern in EXECUTION_CLAIM_PATTERNS):
            violations.append("execution_claim")
        if any(pattern.search(candidate) for pattern in BYPASS_PATTERNS):
            violations.append("bypass_suggestion")
        if violations:
            return self._deterministic_summary(context), tuple(dict.fromkeys(violations))

        return self._trim_summary(candidate), ()

    @staticmethod
    def _trim_summary(summary: str) -> str:
        if len(summary) <= 900:
            return summary
        return summary[:888].rstrip() + " ..."

    @staticmethod
    def _deterministic_summary(context: PromptContext) -> str:
        latest = context.latest_summary or "No operator activity recorded yet."
        return (
            "Assistant operating in deterministic fallback mode. "
            f"Latest operator status: {latest}. "
            f"Question: {context.query or 'No question provided.'}"
        )

    def _log_request(
        self,
        *,
        request_id: str,
        org_id: str,
        actor: str | None,
        llm_result: LLMResult,
        prompt_id: str,
        prompt_version: str,
        fallback_used: bool,
        fallback_reason: str | None,
        guardrail_violations: tuple[str, ...],
    ) -> None:
        should_log_success = random.random() <= self._settings.ai_prompt_observability_sample_rate
        if not fallback_used and not guardrail_violations and not should_log_success:
            return

        logger.info(
            "ai_prompt_request_completed",
            extra={
                "request_id": request_id,
                "org_id": org_id,
                "actor": actor or "unknown",
                "backend": llm_result.backend,
                "prompt_id": prompt_id,
                "prompt_version": prompt_version,
                "fallback_used": fallback_used,
                "fallback_reason": fallback_reason,
                "guardrail_violations": list(guardrail_violations),
            },
        )

    async def _load_assistant_context(self, org_id: str, request_id: str) -> dict[str, object] | None:
        saas_client = self._saas_client
        if saas_client is None:
            return None
        try:
            return await saas_client.get_assistant_context(org_id)
        except Exception as exc:  # noqa: BLE001
            logger.warning(
                "assistant_context_fetch_failed",
                extra={
                    "request_id": request_id,
                    "org_id": org_id,
                    "error": str(exc),
                },
            )
            return None
