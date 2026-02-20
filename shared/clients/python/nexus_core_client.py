from __future__ import annotations

from dataclasses import dataclass
from typing import Any

import httpx


@dataclass
class NexusCoreClient:
    base_url: str
    api_key: str
    scopes: list[str]
    actor: str = "operator/engine"
    timeout_seconds: float = 5.0

    def _headers(self, scopes: list[str] | None = None) -> dict[str, str]:
        resolved_scopes = scopes or self.scopes
        headers = {
            "X-NEXUS-GATEWAY-KEY": self.api_key,
            "X-NEXUS-ACTOR": self.actor,
            "X-NEXUS-SCOPES": ",".join(resolved_scopes),
            "Content-Type": "application/json",
        }
        return headers

    def _client(self) -> httpx.Client:
        return httpx.Client(base_url=self.base_url.rstrip("/"), timeout=self.timeout_seconds)

    def list_events(self, cursor: int, limit: int = 100) -> dict[str, Any]:
        with self._client() as client:
            r = client.get(
                f"/v1/events?cursor={cursor}&limit={limit}",
                headers=self._headers(["audit:read", "admin:console:read"]),
            )
            r.raise_for_status()
            return r.json()

    def apply_action(self, payload: dict[str, Any]) -> dict[str, Any]:
        with self._client() as client:
            r = client.post(
                "/v1/actions/apply",
                headers=self._headers(["admin:console:write"]),
                json=payload,
            )
            r.raise_for_status()
            return r.json()

    def create_incident(self, payload: dict[str, Any]) -> dict[str, Any]:
        with self._client() as client:
            r = client.post(
                "/v1/incidents",
                headers=self._headers(["admin:console:write"]),
                json=payload,
            )
            r.raise_for_status()
            return r.json()

    def create_policy_proposal(self, payload: dict[str, Any]) -> dict[str, Any]:
        with self._client() as client:
            r = client.post(
                "/v1/policy-proposals",
                headers=self._headers(["admin:console:write"]),
                json=payload,
            )
            r.raise_for_status()
            return r.json()
