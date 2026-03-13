"""Integration tests for POST /v1/assistant/query."""
from __future__ import annotations

import pytest
from httpx import ASGITransport, AsyncClient
from unittest.mock import AsyncMock

from tests.integration.conftest import auth_headers, build_app, make_mock_client, make_settings


@pytest.mark.asyncio
async def test_assistant_query_returns_summary_with_operator_state(
    client: AsyncClient,
) -> None:
    """A valid request should return the current operator summary, tables, and actions."""
    payload = {"org_id": "org-123", "query": "What is the operator status?"}

    response = await client.post(
        "/v1/assistant/query",
        json=payload,
        headers=auth_headers(),
    )

    assert response.status_code == 200
    body = response.json()

    # summary must include the echoed query (fallback format: "... | Query: ...")
    assert "What is the operator status?" in body["summary"]

    # tables section
    assert isinstance(body["tables"], list)
    assert len(body["tables"]) == 1
    table = body["tables"][0]
    assert table["title"] == "Operator State"
    assert table["columns"] == ["field", "value"]
    field_names = [row["field"] for row in table["rows"]]
    assert "cursor" in field_names
    assert "last_action_at" in field_names

    # actions section
    assert isinstance(body["actions"], list)
    assert len(body["actions"]) == 1
    action = body["actions"][0]
    assert action["action_type"] == "operator.tick"
    assert action["payload"]["endpoint"] == "/v1/internal/tick"


@pytest.mark.asyncio
async def test_assistant_query_requires_valid_operator_key(
    client: AsyncClient,
) -> None:
    """An omitted key should return 401 when operator_internal_key is configured."""
    payload = {"org_id": "org-123", "query": "hello"}

    response = await client.post("/v1/assistant/query", json=payload)

    assert response.status_code == 401
    assert response.json()["detail"] == "invalid operator key"


@pytest.mark.asyncio
async def test_assistant_query_with_invalid_key_returns_401(
    client: AsyncClient,
) -> None:
    """A wrong key value should be rejected."""
    payload = {"org_id": "org-123", "query": "hello"}

    response = await client.post(
        "/v1/assistant/query",
        json=payload,
        headers=auth_headers("wrong-key-value"),
    )

    assert response.status_code == 401
    assert response.json()["detail"] == "invalid operator key"


@pytest.mark.asyncio
async def test_assistant_query_with_optional_actor_field(
    client: AsyncClient,
) -> None:
    """The optional 'actor' field should be accepted without error."""
    payload = {"org_id": "org-456", "query": "status", "actor": "admin@acme.com"}

    response = await client.post(
        "/v1/assistant/query",
        json=payload,
        headers=auth_headers(),
    )

    assert response.status_code == 200
    body = response.json()
    assert "status" in body["summary"]


@pytest.mark.asyncio
async def test_assistant_query_rate_limit_exceeded() -> None:
    rate_key = "rate-limit-key"
    app = build_app(
        test_settings=make_settings(
            OPERATOR_ASSISTANT_RATE_LIMIT_PER_MIN=1,
            OPERATOR_INTERNAL_KEY=rate_key,
        ),
        mock_client=make_mock_client(),
        start_engine_loop=False,
    )
    transport = ASGITransport(app=app)
    payload = {"org_id": "org-rate", "query": "status?"}

    async with AsyncClient(transport=transport, base_url="http://testserver") as client:
        first = await client.post("/v1/assistant/query", json=payload, headers=auth_headers(rate_key))
        second = await client.post("/v1/assistant/query", json=payload, headers=auth_headers(rate_key))

    assert first.status_code == 200
    assert second.status_code == 429
    assert second.json()["detail"] == "rate limit exceeded"


@pytest.mark.asyncio
async def test_assistant_query_emits_prompt_runtime_metrics(client: AsyncClient) -> None:
    payload = {"org_id": "org-metrics", "query": "status?"}

    response = await client.post(
        "/v1/assistant/query",
        json=payload,
        headers=auth_headers(),
    )
    assert response.status_code == 200

    metrics = await client.get("/metrics")
    assert metrics.status_code == 200
    assert "nexus_ai_prompt_requests_total" in metrics.text
    assert "nexus_ai_prompt_latency_seconds" in metrics.text


@pytest.mark.asyncio
async def test_assistant_query_includes_tenant_aware_snapshot_fields() -> None:
    saas_client = AsyncMock()
    saas_client.get_assistant_context.return_value = {
        "tenant": {
            "plan_code": "growth",
            "status": "active",
            "hard_limits": {"tools_max": 75, "run_rpm": 1200, "audit_retention_days": 90},
        },
        "billing": {
            "billing_status": "active",
            "usage_period": "2026-03",
            "usage": {"api_calls": 12, "events_ingested": 4, "incidents_opened": 1, "actions_executed": 2},
        },
        "incidents": {
            "open_count": 1,
            "items": [{"severity": "HIGH", "status": "open", "title": "Deny spike for admin@acme.com"}],
        },
        "actions": {"active_count": 1, "items": [{"action_type": "throttle_tenant_rpm", "status": "active", "scope_type": "tenant"}]},
        "proposals": {"pending_count": 1, "items": [{"status": "pending", "rationale": "Tighten egress review"}]},
        "events": {"recent_count": 1, "items": [{"event_type": "incident.opened", "summary": "Deny spike for admin@acme.com"}]},
    }
    app = build_app(
        test_settings=make_settings(OPERATOR_INTERNAL_KEY="tenant-aware-key"),
        mock_client=make_mock_client(),
        mock_saas_client=saas_client,
        start_engine_loop=False,
    )
    transport = ASGITransport(app=app)

    async with AsyncClient(transport=transport, base_url="http://testserver") as client:
        response = await client.post(
            "/v1/assistant/query",
            json={"org_id": "org-tenant", "query": "What changed?"},
            headers=auth_headers("tenant-aware-key"),
        )

    assert response.status_code == 200
    body = response.json()
    fields = {row["field"]: row["value"] for row in body["tables"][0]["rows"]}
    assert fields["plan_code"] == "growth"
    assert fields["tenant_status"] == "active"
    assert fields["open_incidents"] == "1"
    assert "admin@acme.com" not in body["summary"]
    saas_client.get_assistant_context.assert_awaited_once_with("org-tenant")
