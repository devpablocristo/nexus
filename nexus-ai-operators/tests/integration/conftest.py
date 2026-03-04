"""Shared fixtures for nexus-ai-operators integration tests.

These tests exercise the full FastAPI application through httpx.AsyncClient
with the real ASGI transport -- no running server required. The management
client is replaced by an AsyncMock so tests never make real HTTP calls.
"""
from __future__ import annotations

from collections.abc import AsyncIterator
from contextlib import asynccontextmanager
from typing import Any
from unittest.mock import AsyncMock

import pytest
from fastapi import FastAPI
from httpx import ASGITransport, AsyncClient

from app.api.routes import router
from app.core.config import Settings
from app.services.engine import OperatorEngine


TEST_OPERATOR_KEY = "test-operator-key"


def make_settings(**overrides: Any) -> Settings:
    """Build a Settings instance with sensible test defaults."""
    defaults: dict[str, Any] = {
        "OPERATOR_APP_NAME": "nexus-ai-operators-test",
        "OPERATOR_ENV": "test",
        "OPERATOR_PORT": 9999,
        "NEXUS_SAAS_BASE_URL": "http://nexus-saas-fake:8082",
        "NEXUS_SAAS_API_KEY": "fake-api-key",
        "NEXUS_SAAS_TIMEOUT_SECONDS": 1.0,
        "OPERATOR_POLL_INTERVAL_SECONDS": 3600,
        "OPERATOR_POLL_BATCH_SIZE": 100,
        "OPERATOR_DENY_RATIO_THRESHOLD": 0.35,
        "OPERATOR_MIN_EVENTS_FOR_SIGNAL": 5,
        "OPERATOR_ACTION_COOLDOWN_SECONDS": 0,
        "OPERATOR_ACTION_TTL_SECONDS": 300,
        "OPERATOR_INTERNAL_KEY": TEST_OPERATOR_KEY,
        # Keep integration tests deterministic and offline-safe.
        "LLM_BACKEND": "fallback",
        "ANTHROPIC_API_KEY": "",
        "OLLAMA_BASE_URL": "http://localhost:11434",
        "OLLAMA_MODEL": "llama3.1:8b",
    }
    defaults.update(overrides)
    return Settings(**defaults)


def make_mock_client() -> AsyncMock:
    """Return an AsyncMock that quacks like the management API client."""
    client = AsyncMock()
    client.list_events.return_value = {"items": [], "next_cursor": 0}
    client.apply_action.return_value = {"ok": True}
    client.create_incident.return_value = {"ok": True}
    client.create_policy_proposal.return_value = {"ok": True}
    client.close.return_value = None
    return client


def build_app(
    test_settings: Settings | None = None,
    mock_client: AsyncMock | None = None,
    start_engine_loop: bool = False,
) -> FastAPI:
    """Create a FastAPI app wired with a mocked engine for testing.

    By default the background engine loop is *not* started so that tests
    control exactly when ``tick_once`` runs.  Pass ``start_engine_loop=True``
    to mimic production behaviour (needed for the ``/readyz`` endpoint).
    """
    _settings = test_settings or make_settings()
    _client = mock_client or make_mock_client()
    _engine = OperatorEngine(settings=_settings, client=_client)

    @asynccontextmanager
    async def lifespan(application: FastAPI) -> AsyncIterator[None]:
        if start_engine_loop:
            await _engine.start()
        yield
        if start_engine_loop:
            await _engine.stop()

    application = FastAPI(title="nexus-ai-operators-test", version="0.0.1-test", lifespan=lifespan)
    application.include_router(router)
    # Set state directly — ASGITransport does not trigger lifespan events
    application.state.settings = _settings
    application.state.engine = _engine
    return application


@pytest.fixture()
def mock_client() -> AsyncMock:
    return make_mock_client()


@pytest.fixture()
def test_settings() -> Settings:
    return make_settings()


@pytest.fixture()
async def client(test_settings: Settings, mock_client: AsyncMock) -> AsyncIterator[AsyncClient]:
    """Yield an httpx.AsyncClient wired to the test ASGI app.

    The engine background loop is **not** started, so ``/readyz`` will return
    503.  Use the ``client_with_engine_loop`` fixture when you need the loop.
    """
    app = build_app(test_settings=test_settings, mock_client=mock_client, start_engine_loop=False)
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://testserver") as ac:
        yield ac


@pytest.fixture()
async def client_with_engine_loop(
    test_settings: Settings, mock_client: AsyncMock
) -> AsyncIterator[AsyncClient]:
    """Like ``client`` but the engine background loop **is** started."""
    app = build_app(test_settings=test_settings, mock_client=mock_client, start_engine_loop=True)
    engine: OperatorEngine = app.state.engine
    await engine.start()
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://testserver") as ac:
        yield ac
    await engine.stop()


def auth_headers(key: str = TEST_OPERATOR_KEY) -> dict[str, str]:
    """Convenience helper to build the operator-key header."""
    return {"X-Operator-Key": key}
