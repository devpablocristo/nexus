from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any


class NexusError(Exception):
    def __init__(self, status: int, code: str, message: str):
        super().__init__(f"[{status}] {code}: {message}")
        self.status = status
        self.code = code
        self.error_message = message


@dataclass
class IdempotencyInfo:
    present: bool = False
    outcome: str = ""


@dataclass
class RunResponse:
    request_id: str
    decision: str
    tool_name: str
    status: str
    result: Any = None
    reason: str = ""
    error: Any = None
    latency_ms: int = 0
    idempotency: IdempotencyInfo | None = None

    @staticmethod
    def from_dict(d: dict[str, Any]) -> RunResponse:
        idem = None
        if d.get("idempotency"):
            idem = IdempotencyInfo(
                present=d["idempotency"].get("present", False),
                outcome=d["idempotency"].get("outcome", ""),
            )
        return RunResponse(
            request_id=d.get("request_id", ""),
            decision=d.get("decision", ""),
            tool_name=d.get("tool_name", ""),
            status=d.get("status", ""),
            result=d.get("result"),
            reason=d.get("reason", ""),
            error=d.get("error"),
            latency_ms=d.get("latency_ms", 0),
            idempotency=idem,
        )

    @property
    def allowed(self) -> bool:
        return self.decision == "allow"

    @property
    def denied(self) -> bool:
        return self.decision == "deny"


@dataclass
class SimulateResponse:
    request_id: str
    decision: str
    tool_name: str
    status: str
    reason: str = ""
    error: Any = None
    explain: dict[str, Any] = field(default_factory=dict)
    latency_ms: int = 0

    @staticmethod
    def from_dict(d: dict[str, Any]) -> SimulateResponse:
        return SimulateResponse(
            request_id=d.get("request_id", ""),
            decision=d.get("decision", ""),
            tool_name=d.get("tool_name", ""),
            status=d.get("status", ""),
            reason=d.get("reason", ""),
            error=d.get("error"),
            explain=d.get("explain", {}),
            latency_ms=d.get("latency_ms", 0),
        )

    @property
    def allowed(self) -> bool:
        return self.decision == "allow"


@dataclass
class Tool:
    id: str
    name: str
    kind: str
    method: str
    url: str
    action_type: str
    enabled: bool
    description: str = ""
    input_schema: dict[str, Any] = field(default_factory=dict)
    output_schema: dict[str, Any] = field(default_factory=dict)
    classification: str = ""
    sensitivity: str = ""
    risk_level: int = 0
    created_at: str = ""
    updated_at: str = ""

    @staticmethod
    def from_dict(d: dict[str, Any]) -> Tool:
        return Tool(
            id=d.get("id", ""),
            name=d.get("name", ""),
            kind=d.get("kind", ""),
            method=d.get("method", ""),
            url=d.get("url", ""),
            action_type=d.get("action_type", ""),
            enabled=d.get("enabled", False),
            description=d.get("description", ""),
            input_schema=d.get("input_schema", {}),
            output_schema=d.get("output_schema", {}),
            classification=d.get("classification", ""),
            sensitivity=d.get("sensitivity", ""),
            risk_level=d.get("risk_level", 0),
            created_at=d.get("created_at", ""),
            updated_at=d.get("updated_at", ""),
        )


@dataclass
class AuditEvent:
    request_id: str
    tool_name: str
    decision: str
    status: str
    reason: str = ""
    latency_ms: int = 0
    created_at: str = ""
    actor: str | None = None
    role: str | None = None
    error: Any = None
    input: Any = None
    context: Any = None
    output: Any = None
    dlp_summary: Any = None

    @staticmethod
    def from_dict(d: dict[str, Any]) -> AuditEvent:
        return AuditEvent(
            request_id=d.get("request_id", ""),
            tool_name=d.get("tool_name", ""),
            decision=d.get("decision", ""),
            status=d.get("status", ""),
            reason=d.get("reason", ""),
            latency_ms=d.get("latency_ms", 0),
            created_at=d.get("created_at", ""),
            actor=d.get("actor"),
            role=d.get("role"),
            error=d.get("error"),
            input=d.get("input"),
            context=d.get("context"),
            output=d.get("output"),
            dlp_summary=d.get("dlp_summary"),
        )


@dataclass
class Policy:
    id: str
    tool_id: str
    effect: str
    priority: int
    conditions: dict[str, Any] = field(default_factory=dict)
    limits: dict[str, Any] = field(default_factory=dict)
    reason_template: str = ""
    enabled: bool = True
    created_at: str = ""
    updated_at: str = ""

    @staticmethod
    def from_dict(d: dict[str, Any]) -> Policy:
        return Policy(
            id=d.get("id", ""),
            tool_id=d.get("tool_id", ""),
            effect=d.get("effect", ""),
            priority=d.get("priority", 0),
            conditions=d.get("conditions", {}),
            limits=d.get("limits", {}),
            reason_template=d.get("reason_template", ""),
            enabled=d.get("enabled", True),
            created_at=d.get("created_at", ""),
            updated_at=d.get("updated_at", ""),
        )


@dataclass
class ApprovalItem:
    id: str
    request_id: str
    tool_name: str
    status: str
    reason: str = ""
    actor: str | None = None
    role: str | None = None
    input_redacted: dict[str, Any] = field(default_factory=dict)
    context_redacted: dict[str, Any] = field(default_factory=dict)
    decided_by: str | None = None
    decided_at: str | None = None
    expires_at: str = ""
    created_at: str = ""

    @staticmethod
    def from_dict(d: dict[str, Any]) -> ApprovalItem:
        return ApprovalItem(
            id=d.get("id", ""),
            request_id=d.get("request_id", ""),
            tool_name=d.get("tool_name", ""),
            status=d.get("status", ""),
            reason=d.get("reason", ""),
            actor=d.get("actor"),
            role=d.get("role"),
            input_redacted=d.get("input_redacted", {}),
            context_redacted=d.get("context_redacted", {}),
            decided_by=d.get("decided_by"),
            decided_at=d.get("decided_at"),
            expires_at=d.get("expires_at", ""),
            created_at=d.get("created_at", ""),
        )


@dataclass
class AlertRule:
    id: str
    name: str
    metric: str
    threshold: float
    webhook_url: str
    enabled: bool = True
    window_seconds: int = 300
    cooldown_seconds: int = 600
    tool_name: str | None = None
    last_fired_at: str | None = None
    created_at: str = ""

    @staticmethod
    def from_dict(d: dict[str, Any]) -> AlertRule:
        return AlertRule(
            id=d.get("id", ""),
            name=d.get("name", ""),
            metric=d.get("metric", ""),
            threshold=d.get("threshold", 0.0),
            webhook_url=d.get("webhook_url", ""),
            enabled=d.get("enabled", True),
            window_seconds=d.get("window_seconds", 300),
            cooldown_seconds=d.get("cooldown_seconds", 600),
            tool_name=d.get("tool_name"),
            last_fired_at=d.get("last_fired_at"),
            created_at=d.get("created_at", ""),
        )


@dataclass
class AgentSession:
    id: str
    session_id: str
    total_calls: int = 0
    total_writes: int = 0
    total_denials: int = 0
    actor: str | None = None
    metadata: dict[str, Any] = field(default_factory=dict)
    created_at: str = ""
    last_call_at: str = ""

    @staticmethod
    def from_dict(d: dict[str, Any]) -> AgentSession:
        return AgentSession(
            id=d.get("id", ""),
            session_id=d.get("session_id", ""),
            total_calls=d.get("total_calls", 0),
            total_writes=d.get("total_writes", 0),
            total_denials=d.get("total_denials", 0),
            actor=d.get("actor"),
            metadata=d.get("metadata", {}),
            created_at=d.get("created_at", ""),
            last_call_at=d.get("last_call_at", ""),
        )


@dataclass
class OrgCreated:
    org_id: str
    api_key: str
    name: str

    @staticmethod
    def from_dict(d: dict[str, Any]) -> OrgCreated:
        return OrgCreated(
            org_id=d.get("org_id", ""),
            api_key=d.get("api_key", ""),
            name=d.get("name", ""),
        )
