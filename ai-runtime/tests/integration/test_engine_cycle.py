"""Integration tests for the full OperatorEngine cycle.

These tests instantiate a real OperatorEngine with mocked NexusCoreClient
methods and verify the complete pipeline:
  events -> observe -> compute signal -> score -> apply action -> open incident -> create proposal
"""
from __future__ import annotations

from unittest.mock import call

import pytest

from app.services.engine import OperatorEngine
from tests.integration.conftest import make_mock_client, make_settings


def _high_risk_events(count: int = 20, deny_count: int = 18) -> dict:
    """Build a nexus-core-style response with a high-risk deny ratio."""
    return {
        "items": [
            {
                "id": i + 1,
                "event_type": "tool.call.completed",
                "created_at": "2026-01-15T12:00:00Z",
                "payload": {"decision": "deny" if i < deny_count else "allow"},
            }
            for i in range(count)
        ],
        "next_cursor": count,
    }


def _low_risk_events(count: int = 20, deny_count: int = 1) -> dict:
    """Build a nexus-core-style response with a low deny ratio."""
    return {
        "items": [
            {
                "id": i + 1,
                "event_type": "tool.call.completed",
                "created_at": "2026-01-15T12:00:00Z",
                "payload": {"decision": "deny" if i < deny_count else "allow"},
            }
            for i in range(count)
        ],
        "next_cursor": count,
    }


def _empty_events() -> dict:
    return {"items": [], "next_cursor": 0}


# ---------------------------------------------------------------------------
# Full high-risk cycle
# ---------------------------------------------------------------------------

@pytest.mark.asyncio
async def test_full_cycle_high_risk_applies_action_incident_proposal() -> None:
    """When deny ratio exceeds threshold the engine must apply action,
    open an incident, and create a policy proposal."""
    mock = make_mock_client()
    mock.list_events.return_value = _high_risk_events(count=20, deny_count=18)

    settings = make_settings(
        OPERATOR_MIN_EVENTS_FOR_SIGNAL=5,
        OPERATOR_DENY_RATIO_THRESHOLD=0.35,
        OPERATOR_ACTION_COOLDOWN_SECONDS=0,
    )
    engine = OperatorEngine(settings=settings, client=mock)
    await engine.tick_once()

    # All three downstream calls must have been made exactly once.
    mock.apply_action.assert_awaited_once()
    mock.create_incident.assert_awaited_once()
    mock.create_policy_proposal.assert_awaited_once()

    # Verify the action payload structure
    action_payload = mock.apply_action.call_args[0][0]
    assert action_payload["scope_type"] == "tenant"
    assert action_payload["action_type"] == "throttle_tenant_rpm"
    assert action_payload["ttl_seconds"] == settings.action_ttl_seconds
    assert "evidence_refs" in action_payload
    assert len(action_payload["evidence_refs"]) > 0

    # Verify incident payload
    incident_payload = mock.create_incident.call_args[0][0]
    assert incident_payload["severity"] in ("CRIT", "HIGH", "MED")
    assert "deny ratio" in incident_payload["summary"].lower() or "deny_ratio" in incident_payload["summary"].lower()
    assert "evidence_refs" in incident_payload

    # Verify proposal payload
    proposal_payload = mock.create_policy_proposal.call_args[0][0]
    assert proposal_payload["status"] == "pending"
    assert "diff" in proposal_payload
    assert "rationale" in proposal_payload

    # Engine state should be updated
    assert engine.state.cursor == 20
    assert engine.state.last_action_at > 0
    assert "High-risk" in engine.state.latest_summary


@pytest.mark.asyncio
async def test_full_cycle_high_risk_severity_is_crit_for_very_high_deny_ratio() -> None:
    """When deny ratio >= 0.8, severity must be CRIT."""
    mock = make_mock_client()
    mock.list_events.return_value = _high_risk_events(count=20, deny_count=18)

    settings = make_settings(
        OPERATOR_MIN_EVENTS_FOR_SIGNAL=5,
        OPERATOR_DENY_RATIO_THRESHOLD=0.35,
        OPERATOR_ACTION_COOLDOWN_SECONDS=0,
    )
    engine = OperatorEngine(settings=settings, client=mock)
    await engine.tick_once()

    incident_payload = mock.create_incident.call_args[0][0]
    # 18/20 = 0.90 => CRIT
    assert incident_payload["severity"] == "CRIT"


@pytest.mark.asyncio
async def test_full_cycle_high_risk_severity_is_high_for_moderate_deny_ratio() -> None:
    """When 0.6 <= deny ratio < 0.8, severity must be HIGH."""
    mock = make_mock_client()
    # 14/20 = 0.70 -> HIGH
    mock.list_events.return_value = _high_risk_events(count=20, deny_count=14)

    settings = make_settings(
        OPERATOR_MIN_EVENTS_FOR_SIGNAL=5,
        OPERATOR_DENY_RATIO_THRESHOLD=0.35,
        OPERATOR_ACTION_COOLDOWN_SECONDS=0,
    )
    engine = OperatorEngine(settings=settings, client=mock)
    await engine.tick_once()

    incident_payload = mock.create_incident.call_args[0][0]
    assert incident_payload["severity"] == "HIGH"


# ---------------------------------------------------------------------------
# Low-risk cycle -- no actions
# ---------------------------------------------------------------------------

@pytest.mark.asyncio
async def test_low_risk_signal_does_not_apply_action() -> None:
    """When deny ratio is below threshold, no action/incident/proposal should be created."""
    mock = make_mock_client()
    mock.list_events.return_value = _low_risk_events(count=20, deny_count=1)

    settings = make_settings(
        OPERATOR_MIN_EVENTS_FOR_SIGNAL=5,
        OPERATOR_DENY_RATIO_THRESHOLD=0.35,
        OPERATOR_ACTION_COOLDOWN_SECONDS=0,
    )
    engine = OperatorEngine(settings=settings, client=mock)
    await engine.tick_once()

    mock.apply_action.assert_not_awaited()
    mock.create_incident.assert_not_awaited()
    mock.create_policy_proposal.assert_not_awaited()

    assert engine.state.cursor == 20
    assert "Processed 20 events" in engine.state.latest_summary


# ---------------------------------------------------------------------------
# Empty events -- no-op
# ---------------------------------------------------------------------------

@pytest.mark.asyncio
async def test_empty_events_no_action() -> None:
    """An empty event list should be a safe no-op."""
    mock = make_mock_client()
    mock.list_events.return_value = _empty_events()

    settings = make_settings()
    engine = OperatorEngine(settings=settings, client=mock)
    await engine.tick_once()

    mock.apply_action.assert_not_awaited()
    mock.create_incident.assert_not_awaited()
    mock.create_policy_proposal.assert_not_awaited()
    assert engine.state.cursor == 0


# ---------------------------------------------------------------------------
# Cooldown suppresses repeated actions
# ---------------------------------------------------------------------------

@pytest.mark.asyncio
async def test_cooldown_prevents_second_action() -> None:
    """After a high-risk action, a second tick within cooldown should skip actions."""
    mock = make_mock_client()
    mock.list_events.return_value = _high_risk_events(count=20, deny_count=18)

    settings = make_settings(
        OPERATOR_MIN_EVENTS_FOR_SIGNAL=5,
        OPERATOR_DENY_RATIO_THRESHOLD=0.35,
        # Very long cooldown so the second tick is always suppressed
        OPERATOR_ACTION_COOLDOWN_SECONDS=999999,
    )
    engine = OperatorEngine(settings=settings, client=mock)

    # First tick applies actions
    await engine.tick_once()
    assert mock.apply_action.await_count == 1
    assert mock.create_incident.await_count == 1
    assert mock.create_policy_proposal.await_count == 1

    # Second tick with same high-risk data -- cooldown should suppress
    mock.list_events.return_value = _high_risk_events(count=20, deny_count=18)
    await engine.tick_once()

    # Counts should not have increased
    assert mock.apply_action.await_count == 1
    assert mock.create_incident.await_count == 1
    assert mock.create_policy_proposal.await_count == 1


# ---------------------------------------------------------------------------
# Cursor advances across ticks
# ---------------------------------------------------------------------------

@pytest.mark.asyncio
async def test_cursor_advances_across_multiple_ticks() -> None:
    """Cursor returned by nexus-core should be persisted and sent in the next request."""
    mock = make_mock_client()

    # First call returns cursor 50, second returns cursor 100, third empty
    mock.list_events.side_effect = [
        {"items": [], "next_cursor": 50},
        {"items": [], "next_cursor": 100},
        {"items": [], "next_cursor": 100},
    ]

    settings = make_settings()
    engine = OperatorEngine(settings=settings, client=mock)

    await engine.tick_once()
    assert engine.state.cursor == 50

    await engine.tick_once()
    assert engine.state.cursor == 100

    # Verify the second call used cursor=50
    calls = mock.list_events.call_args_list
    assert calls[0] == call(0, settings.poll_batch_size)
    assert calls[1] == call(50, settings.poll_batch_size)


# ---------------------------------------------------------------------------
# Evidence refs are capped at 20
# ---------------------------------------------------------------------------

@pytest.mark.asyncio
async def test_evidence_refs_capped_at_20() -> None:
    """compute_signal caps evidence_refs at 20. The engine should pass them through."""
    mock = make_mock_client()
    # 30 deny events -> 30 potential evidence_refs, but observer caps at 20
    mock.list_events.return_value = _high_risk_events(count=30, deny_count=30)

    settings = make_settings(
        OPERATOR_MIN_EVENTS_FOR_SIGNAL=5,
        OPERATOR_DENY_RATIO_THRESHOLD=0.35,
        OPERATOR_ACTION_COOLDOWN_SECONDS=0,
    )
    engine = OperatorEngine(settings=settings, client=mock)
    await engine.tick_once()

    action_payload = mock.apply_action.call_args[0][0]
    assert len(action_payload["evidence_refs"]) == 20


# ---------------------------------------------------------------------------
# Engine start / stop lifecycle
# ---------------------------------------------------------------------------

@pytest.mark.asyncio
async def test_engine_start_stop_lifecycle() -> None:
    """start() should create the background task; stop() should tear it down."""
    mock = make_mock_client()
    settings = make_settings(OPERATOR_POLL_INTERVAL_SECONDS=3600)
    engine = OperatorEngine(settings=settings, client=mock)

    await engine.start()
    assert engine._task is not None
    assert not engine._task.done()

    await engine.stop()
    assert engine._task is None
    mock.close.assert_awaited_once()
