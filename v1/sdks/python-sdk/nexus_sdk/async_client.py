from __future__ import annotations

from typing import Any

import httpx

from nexus_sdk.types import (
    AgentSession,
    AlertRule,
    ApprovalItem,
    AuditEvent,
    NexusError,
    OrgCreated,
    Policy,
    RunResponse,
    SimulateResponse,
    Tool,
)


class AsyncNexusClient:
    """Async Nexus Gateway SDK client."""

    def __init__(
        self,
        base_url: str = "http://localhost:8080",
        api_key: str = "",
        *,
        actor: str = "",
        role: str = "",
        scopes: str = "",
        timeout: float = 30.0,
    ):
        headers: dict[str, str] = {"Content-Type": "application/json"}
        if api_key:
            headers["X-NEXUS-CORE-KEY"] = api_key
        if actor:
            headers["X-NEXUS-ACTOR"] = actor
        if role:
            headers["X-NEXUS-ROLE"] = role
        if scopes:
            headers["X-NEXUS-SCOPES"] = scopes
        self._client = httpx.AsyncClient(
            base_url=base_url, headers=headers, timeout=timeout
        )

    async def close(self) -> None:
        await self._client.aclose()

    async def __aenter__(self) -> AsyncNexusClient:
        return self

    async def __aexit__(self, *exc: Any) -> None:
        await self.close()

    async def _request(self, method: str, path: str, **kwargs: Any) -> Any:
        resp = await self._client.request(method, path, **kwargs)
        if resp.status_code >= 400:
            body = resp.json() if resp.headers.get("content-type", "").startswith("application/json") else {}
            raise NexusError(
                resp.status_code,
                body.get("error", {}).get("code", "") if isinstance(body.get("error"), dict) else body.get("code", ""),
                body.get("error", {}).get("message", "") if isinstance(body.get("error"), dict) else body.get("message", resp.text[:200]),
            )
        return resp.json()

    async def run(
        self,
        tool_name: str,
        input: dict[str, Any] | None = None,
        context: dict[str, Any] | None = None,
        *,
        idempotency_key: str | None = None,
        timeout_ms: int | None = None,
        request_id: str | None = None,
    ) -> RunResponse:
        body: dict[str, Any] = {
            "tool_name": tool_name,
            "input": input or {},
            "context": context or {},
        }
        if request_id:
            body["request_id"] = request_id
        headers: dict[str, str] = {}
        if idempotency_key:
            headers["Idempotency-Key"] = idempotency_key
        if timeout_ms is not None:
            headers["X-Timeout-Ms"] = str(timeout_ms)
        try:
            data = await self._request("POST", "/v1/run", json=body, headers=headers)
        except NexusError as e:
            if e.status in (403, 429):
                return RunResponse.from_dict({"decision": "deny", "status": "blocked", "tool_name": tool_name, "error": {"code": e.code, "message": e.error_message}})
            raise
        return RunResponse.from_dict(data)

    async def simulate(
        self,
        tool_name: str,
        input: dict[str, Any] | None = None,
        context: dict[str, Any] | None = None,
    ) -> SimulateResponse:
        body = {"tool_name": tool_name, "input": input or {}, "context": context or {}}
        try:
            data = await self._request("POST", "/v1/run/simulate", json=body)
        except NexusError as e:
            if e.status in (403, 200):
                return SimulateResponse.from_dict({"decision": "deny", "status": "blocked", "tool_name": tool_name, "reason": e.error_message})
            raise
        return SimulateResponse.from_dict(data)

    async def list_tools(self) -> list[Tool]:
        data = await self._request("GET", "/v1/tools")
        return [Tool.from_dict(t) for t in data.get("items", [])]

    async def get_tool(self, name: str) -> Tool:
        data = await self._request("GET", f"/v1/tools/{name}")
        return Tool.from_dict(data)

    async def query_audit(self, *, tool_name: str = "", decision: str = "", status: str = "", limit: int = 200) -> list[AuditEvent]:
        params: dict[str, Any] = {"limit": limit}
        if tool_name:
            params["tool_name"] = tool_name
        if decision:
            params["decision"] = decision
        if status:
            params["status"] = status
        data = await self._request("GET", "/v1/audit", params=params)
        return [AuditEvent.from_dict(e) for e in data.get("items", [])]

    async def list_policies(self, tool_name: str) -> list[Policy]:
        data = await self._request("GET", f"/v1/tools/{tool_name}/policies")
        return [Policy.from_dict(p) for p in data.get("items", [])]

    async def add_egress_rule(self, tool_name: str, host: str, enabled: bool = True) -> Any:
        return await self._request("POST", f"/v1/tools/{tool_name}/egress-rules", json={"host": host, "enabled": enabled})

    # -- Approvals --

    async def list_approvals(self, limit: int = 100) -> list[ApprovalItem]:
        data = await self._request("GET", "/v1/approvals", params={"limit": limit})
        return [ApprovalItem.from_dict(a) for a in data.get("items", [])]

    async def get_approval(self, approval_id: str) -> ApprovalItem:
        data = await self._request("GET", f"/v1/approvals/{approval_id}")
        return ApprovalItem.from_dict(data)

    async def approve(self, approval_id: str, decided_by: str = "") -> dict[str, Any]:
        return await self._request("POST", f"/v1/approvals/{approval_id}/approve", json={"decided_by": decided_by})

    async def reject(self, approval_id: str, decided_by: str = "") -> dict[str, Any]:
        return await self._request("POST", f"/v1/approvals/{approval_id}/reject", json={"decided_by": decided_by})

    # -- Alert Rules --

    async def list_alert_rules(self) -> list[AlertRule]:
        data = await self._request("GET", "/v1/alert-rules")
        return [AlertRule.from_dict(r) for r in data.get("items", [])]

    async def create_alert_rule(
        self,
        name: str,
        metric: str,
        threshold: float,
        webhook_url: str,
        *,
        window_seconds: int = 300,
        cooldown_seconds: int = 600,
        tool_name: str | None = None,
        enabled: bool = True,
    ) -> AlertRule:
        body: dict[str, Any] = {
            "name": name, "metric": metric, "threshold": threshold,
            "webhook_url": webhook_url, "window_seconds": window_seconds,
            "cooldown_seconds": cooldown_seconds, "enabled": enabled,
        }
        if tool_name:
            body["tool_name"] = tool_name
        data = await self._request("POST", "/v1/alert-rules", json=body)
        return AlertRule.from_dict(data)

    async def delete_alert_rule(self, rule_id: str) -> dict[str, Any]:
        return await self._request("DELETE", f"/v1/alert-rules/{rule_id}")

    # -- Sessions --

    async def get_session(self, session_id: str) -> AgentSession:
        data = await self._request("GET", f"/v1/sessions/{session_id}")
        return AgentSession.from_dict(data)

    # -- Orgs --

    async def create_org(self, name: str, scopes: list[str] | None = None) -> OrgCreated:
        body: dict[str, Any] = {"name": name}
        if scopes:
            body["scopes"] = scopes
        data = await self._request("POST", "/v1/orgs", json=body)
        return OrgCreated.from_dict(data)

    # -- Health --

    async def health(self) -> dict[str, Any]:
        return await self._request("GET", "/healthz")
