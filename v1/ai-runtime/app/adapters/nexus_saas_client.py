from __future__ import annotations

import asyncio
import logging
from typing import Any

import httpx


logger = logging.getLogger(__name__)


class NexusSaaSClient:
    def __init__(self, base_url: str, api_key: str, timeout_seconds: float) -> None:
        self._base_url = base_url.rstrip("/")
        self._api_key = api_key
        self._timeout = timeout_seconds
        self._client = httpx.AsyncClient(base_url=self._base_url, timeout=self._timeout)

    async def close(self) -> None:
        await self._client.aclose()

    async def get_assistant_context(self, org_id: str) -> dict[str, Any]:
        return await self._request("GET", f"/internal/assistant/context/{org_id}")

    async def _request(self, method: str, path: str) -> dict[str, Any]:
        headers = {"X-NEXUS-SAAS-KEY": self._api_key}

        attempts = 3
        for attempt in range(1, attempts + 1):
            try:
                response = await self._client.request(method, path, headers=headers)
                if response.status_code >= 500 and attempt < attempts:
                    await asyncio.sleep(0.25 * (2 ** (attempt - 1)))
                    continue
                response.raise_for_status()
                return response.json() if response.content else {}
            except (httpx.TimeoutException, httpx.TransportError) as exc:
                if attempt >= attempts:
                    raise
                logger.warning("saas_request_retry transport_error=%s attempt=%s path=%s", type(exc).__name__, attempt, path)
                await asyncio.sleep(0.25 * (2 ** (attempt - 1)))
        raise RuntimeError("unreachable")
