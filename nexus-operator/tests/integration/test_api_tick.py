"""Integration tests for POST /v1/internal/tick."""
from __future__ import annotations

from unittest.mock import AsyncMock

import pytest
from httpx import ASGITransport, AsyncClient

from tests.integration.conftest import (
    auth_headers,
    build_app,
    make_mock_client,
    make_settings,
)


@pytest.mark.asyncio
async def test_tick_requires_operator_key(client: AsyncClient) -> None:
    """Calling tick without the operator key header should return 401."""
    response = await client.post("/v1/internal/tick")

    assert response.status_code == 401
    assert response.json()["detail"] == "invalid operator key"


@pytest.mark.asyncio
async def test_tick_with_wrong_key_returns_401(client: AsyncClient) -> None:
    response = await client.post(
        "/v1/internal/tick",
        headers=auth_headers("definitely-wrong"),
    )

    assert response.status_code == 401


@pytest.mark.asyncio
async def test_tick_succeeds_with_mocked_core_empty_events(
    client: AsyncClient,
    mock_client: AsyncMock,
) -> None:
    """When nexus-core returns no events, tick should succeed and leave state untouched."""
    response = await client.post(
        "/v1/internal/tick",
        headers=auth_headers(),
    )

    assert response.status_code == 200
    assert response.json() == {"status": "ok"}
    mock_client.list_events.assert_awaited_once()


@pytest.mark.asyncio
async def test_tick_processes_events_from_mocked_core() -> None:
    """When nexus-core returns events, tick should drive the full engine pipeline."""
    mock = make_mock_client()
    mock.list_events.return_value = {
        "items": [
            {
                "id": i + 1,
                "event_type": "tool.call.completed",
                "created_at": "2026-01-15T12:00:00Z",
                "payload": {"decision": "deny" if i < 18 else "allow"},
            }
            for i in range(20)
        ],
        "next_cursor": 20,
    }

    settings = make_settings(
        OPERATOR_MIN_EVENTS_FOR_SIGNAL=5,
        OPERATOR_DENY_RATIO_THRESHOLD=0.35,
        OPERATOR_ACTION_COOLDOWN_SECONDS=0,
    )
    app = build_app(test_settings=settings, mock_client=mock)
    transport = ASGITransport(app=app)

    async with AsyncClient(transport=transport, base_url="http://testserver") as ac:
        response = await ac.post("/v1/internal/tick", headers=auth_headers())

    assert response.status_code == 200
    assert response.json() == {"status": "ok"}

    # The engine should have processed the high-risk signal
    mock.list_events.assert_awaited_once()
    mock.apply_action.assert_awaited_once()
    mock.create_incident.assert_awaited_once()
    mock.create_policy_proposal.assert_awaited_once()
