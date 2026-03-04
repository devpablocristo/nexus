from __future__ import annotations

import asyncio
import logging
from typing import Any

import httpx


logger = logging.getLogger(__name__)


class NexusCoreClient:
    def __init__(self, base_url: str, api_key: str, timeout_seconds: float) -> None:
        self._base_url = base_url.rstrip('/')
        self._api_key = api_key
        self._timeout = timeout_seconds
        self._client = httpx.AsyncClient(base_url=self._base_url, timeout=self._timeout)

    async def close(self) -> None:
        await self._client.aclose()

    async def list_events(self, cursor: int, limit: int = 100) -> dict[str, Any]:
        return await self._request('GET', f'/internal/operators/events?cursor={cursor}&limit={limit}')

    async def apply_action(self, payload: dict[str, Any]) -> dict[str, Any]:
        return await self._request('POST', '/internal/operators/actions/apply', json=payload)

    async def create_incident(self, payload: dict[str, Any]) -> dict[str, Any]:
        return await self._request('POST', '/internal/operators/incidents', json=payload)

    async def create_policy_proposal(self, payload: dict[str, Any]) -> dict[str, Any]:
        return await self._request('POST', '/internal/operators/policy-proposals', json=payload)

    async def _request(self, method: str, path: str, json: dict[str, Any] | None = None) -> dict[str, Any]:
        headers = {
            'X-NEXUS-AI-KEY': self._api_key,
            'Content-Type': 'application/json',
        }

        attempts = 3
        for attempt in range(1, attempts + 1):
            try:
                response = await self._client.request(method, path, headers=headers, json=json)
                if response.status_code >= 500 and attempt < attempts:
                    await asyncio.sleep(0.25 * (2 ** (attempt - 1)))
                    continue
                response.raise_for_status()
                return response.json() if response.content else {}
            except (httpx.TimeoutException, httpx.TransportError) as exc:
                if attempt >= attempts:
                    raise
                logger.warning('core_request_retry transport_error=%s attempt=%s path=%s', type(exc).__name__, attempt, path)
                await asyncio.sleep(0.25 * (2 ** (attempt - 1)))
        raise RuntimeError('unreachable')
