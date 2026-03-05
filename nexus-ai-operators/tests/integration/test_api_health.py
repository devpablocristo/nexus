"""Integration tests for health / readiness / metrics endpoints."""
from __future__ import annotations

import pytest
from httpx import AsyncClient


@pytest.mark.asyncio
async def test_healthz_returns_200(client: AsyncClient) -> None:
    response = await client.get("/healthz")

    assert response.status_code == 200
    body = response.json()
    assert body == {"ok": True}


@pytest.mark.asyncio
async def test_readyz_returns_200_when_engine_running(
    client_with_engine_loop: AsyncClient,
) -> None:
    """The engine loop task is started during lifespan, so readyz should succeed."""
    response = await client_with_engine_loop.get("/readyz")

    assert response.status_code == 200
    body = response.json()
    assert body == {"ok": True}


@pytest.mark.asyncio
async def test_readyz_returns_503_when_engine_loop_not_running(
    client: AsyncClient,
) -> None:
    """When the engine loop is not started, readyz should return 503."""
    response = await client.get("/readyz")

    assert response.status_code == 503
    body = response.json()
    assert body["detail"] == "engine loop not running"


@pytest.mark.asyncio
async def test_metrics_returns_prometheus_output(client: AsyncClient) -> None:
    response = await client.get("/metrics")

    assert response.status_code == 200
    assert "text/plain" in response.headers["content-type"]

    text = response.text
    # The prometheus_client always emits at least built-in process metrics.
    # Our app also registers custom counters/gauges.
    assert "nexus_operator_events_consumed_total" in text
    assert "nexus_operator_actions_applied_total" in text
    assert "nexus_operator_last_cursor" in text


@pytest.mark.asyncio
async def test_metrics_allows_unauthenticated_access(client: AsyncClient) -> None:
    response = await client.get("/metrics")
    assert response.status_code == 200
