from __future__ import annotations

from dataclasses import dataclass, field
from datetime import UTC, datetime
import re
from typing import Any

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
    assistant_overview: dict[str, str] = field(default_factory=dict)


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
        assistant_context: dict[str, Any] | None = None,
    ) -> PromptContext:
        safe_org_id = self._trim(self._redact(org_id), 128)
        safe_actor = self._trim(self._redact(actor or ""), 128) or None
        safe_query = self._trim(self._redact(query), 600)
        safe_summary = self._trim(self._redact(engine_state.latest_summary), 800)
        safe_last_action_at = self._format_ts(engine_state.last_action_at)
        assistant_lines, assistant_overview, safe_summary = self._assistant_context_lines(assistant_context, safe_summary)

        lines = [
            "flow: " + prompt_id,
            "org_id: " + safe_org_id,
            "actor: " + (safe_actor or "unknown"),
            "cursor: " + str(engine_state.cursor),
            "last_action_at: " + safe_last_action_at,
            "latest_summary: " + (safe_summary or "No operator activity recorded yet."),
        ]
        lines.extend(assistant_lines)
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
            assistant_overview=assistant_overview,
        )

    def _assistant_context_lines(
        self,
        assistant_context: dict[str, Any] | None,
        operator_summary: str,
    ) -> tuple[list[str], dict[str, str], str]:
        if not assistant_context:
            return [], {}, operator_summary

        lines: list[str] = []
        overview: dict[str, str] = {}

        tenant = self._dict(assistant_context.get("tenant"))
        billing = self._dict(assistant_context.get("billing"))
        incidents = self._dict(assistant_context.get("incidents"))
        actions = self._dict(assistant_context.get("actions"))
        proposals = self._dict(assistant_context.get("proposals"))
        events = self._dict(assistant_context.get("events"))

        plan_code = self._trim(self._redact(self._string(tenant.get("plan_code"))), 48)
        tenant_status = self._trim(self._redact(self._string(tenant.get("status"))), 48)
        billing_status = self._trim(self._redact(self._string(billing.get("billing_status"))), 48)
        overview["plan_code"] = plan_code or "unknown"
        overview["tenant_status"] = tenant_status or "unknown"
        overview["billing_status"] = billing_status or "unknown"

        hard_limits = self._dict(tenant.get("hard_limits"))
        lines.append(
            "tenant_snapshot: "
            + ", ".join(
                [
                    f"plan={plan_code or 'unknown'}",
                    f"status={tenant_status or 'unknown'}",
                    f"tools_max={self._string(hard_limits.get('tools_max')) or '0'}",
                    f"run_rpm={self._string(hard_limits.get('run_rpm')) or '0'}",
                    f"audit_retention_days={self._string(hard_limits.get('audit_retention_days')) or '0'}",
                ]
            )
        )

        usage = self._dict(billing.get("usage"))
        lines.append(
            "billing_snapshot: "
            + ", ".join(
                [
                    f"status={billing_status or 'unknown'}",
                    f"period={self._string(billing.get('usage_period')) or 'current'}",
                    f"api_calls={self._string(usage.get('api_calls')) or '0'}",
                    f"events_ingested={self._string(usage.get('events_ingested')) or '0'}",
                    f"incidents_opened={self._string(usage.get('incidents_opened')) or '0'}",
                    f"actions_executed={self._string(usage.get('actions_executed')) or '0'}",
                ]
            )
        )

        open_incidents = self._items(incidents.get("items"))
        active_actions = self._items(actions.get("items"))
        pending_proposals = self._items(proposals.get("items"))
        recent_events = self._items(events.get("items"))
        overview["open_incidents"] = self._string(incidents.get("open_count")) or str(len(open_incidents))
        overview["active_actions"] = self._string(actions.get("active_count")) or str(len(active_actions))
        overview["pending_proposals"] = self._string(proposals.get("pending_count")) or str(len(pending_proposals))
        overview["recent_events"] = self._string(events.get("recent_count")) or str(len(recent_events))

        if open_incidents:
            lines.append("open_incidents: " + self._join_items(open_incidents, ["severity", "status", "title"], 3))
        if active_actions:
            lines.append("active_actions: " + self._join_items(active_actions, ["action_type", "status", "scope_type", "scope_id"], 3))
        if pending_proposals:
            lines.append("pending_proposals: " + self._join_items(pending_proposals, ["status", "rationale"], 3))
        if recent_events:
            lines.append("recent_events: " + self._join_items(recent_events, ["event_type", "summary"], 5))

        combined_summary = self._trim(
            self._redact(
                f"{operator_summary} | tenant_status={overview['tenant_status']}, "
                f"billing_status={overview['billing_status']}, "
                f"open_incidents={overview['open_incidents']}, "
                f"active_actions={overview['active_actions']}, "
                f"pending_proposals={overview['pending_proposals']}"
            ),
            800,
        )

        return lines, overview, combined_summary

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

    def _join_items(self, items: list[dict[str, Any]], keys: list[str], limit: int) -> str:
        rendered: list[str] = []
        for item in items[:limit]:
            text = " | ".join(self._string(item.get(key)) for key in keys if self._string(item.get(key)))
            safe = self._trim(self._redact(text), 140)
            if safe:
                rendered.append(safe)
        return "; ".join(rendered)

    @staticmethod
    def _dict(value: Any) -> dict[str, Any]:
        if isinstance(value, dict):
            return value
        return {}

    @staticmethod
    def _items(value: Any) -> list[dict[str, Any]]:
        if not isinstance(value, list):
            return []
        return [item for item in value if isinstance(item, dict)]

    @staticmethod
    def _string(value: Any) -> str:
        if value is None:
            return ""
        return str(value).strip()
