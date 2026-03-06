from __future__ import annotations

from dataclasses import dataclass
from datetime import UTC, datetime
import re

from app.services.engine import EngineState


EMAIL_RE = re.compile(r"\b[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}\b", re.IGNORECASE)
PHONE_RE = re.compile(r"\+?\d[\d\s().-]{7,}\d")
JWT_RE = re.compile(r"\b[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\b")
SECRET_RE = re.compile(r"\b(?:sk|pk|rk|nxk|whsec)_[A-Za-z0-9_-]{8,}\b")
MULTISPACE_RE = re.compile(r"\s+")


@dataclass(frozen=True)
class PromptContext:
    prompt_id: str
    org_id: str
    actor: str | None
    query: str
    latest_summary: str
    cursor: int
    last_action_at: str
    block: str


class PromptContextBuilder:
    def __init__(self, max_chars: int) -> None:
        self._max_chars = max_chars

    def build(
        self,
        prompt_id: str,
        org_id: str,
        actor: str | None,
        query: str,
        engine_state: EngineState,
    ) -> PromptContext:
        safe_org_id = self._trim(self._redact(org_id), 128)
        safe_actor = self._trim(self._redact(actor or ""), 128) or None
        safe_query = self._trim(self._redact(query), 600)
        safe_summary = self._trim(self._redact(engine_state.latest_summary), 800)
        safe_last_action_at = self._format_ts(engine_state.last_action_at)

        lines = [
            "flow: " + prompt_id,
            "org_id: " + safe_org_id,
            "actor: " + (safe_actor or "unknown"),
            "cursor: " + str(engine_state.cursor),
            "last_action_at: " + safe_last_action_at,
            "latest_summary: " + (safe_summary or "No operator activity recorded yet."),
        ]
        block = "\n".join(lines)
        if len(block) > self._max_chars:
            overflow = len(block) - self._max_chars
            trimmed_summary = self._trim(
                safe_summary,
                max(80, len(safe_summary) - overflow - 16),
            )
            lines[-1] = "latest_summary: " + (trimmed_summary or "No operator activity recorded yet.")
            block = "\n".join(lines)
            if len(block) > self._max_chars:
                block = self._trim(block, self._max_chars)

        return PromptContext(
            prompt_id=prompt_id,
            org_id=safe_org_id,
            actor=safe_actor,
            query=safe_query,
            latest_summary=safe_summary,
            cursor=engine_state.cursor,
            last_action_at=safe_last_action_at,
            block=block,
        )

    @staticmethod
    def _format_ts(epoch_seconds: float) -> str:
        if epoch_seconds <= 0:
            return "never"
        return datetime.fromtimestamp(epoch_seconds, tz=UTC).isoformat()

    @staticmethod
    def _redact(value: str) -> str:
        redacted = value
        for pattern in (EMAIL_RE, PHONE_RE, JWT_RE, SECRET_RE):
            redacted = pattern.sub("[REDACTED]", redacted)
        return MULTISPACE_RE.sub(" ", redacted).strip()

    @staticmethod
    def _trim(value: str, limit: int) -> str:
        if limit <= 0:
            return ""
        if len(value) <= limit:
            return value
        return value[: max(0, limit - 12)].rstrip() + " [truncated]"
