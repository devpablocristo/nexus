import pytest

from app.core.config import Settings
from app.services.engine import OperatorEngine


class FakeClient:
    def __init__(self) -> None:
        self.actions = 0
        self.incidents = 0
        self.proposals = 0

    async def list_events(self, cursor: int, limit: int = 100):
        return {
            'items': [
                {'id': i + 1, 'event_type': 'tool.call.completed', 'created_at': '2026-01-01T00:00:00Z', 'payload': {'decision': 'deny' if i < 8 else 'allow'}}
                for i in range(10)
            ],
            'next_cursor': cursor + 10,
        }

    async def apply_action(self, payload):
        self.actions += 1
        return {'ok': True, 'payload': payload}

    async def create_incident(self, payload):
        self.incidents += 1
        return {'ok': True}

    async def create_policy_proposal(self, payload):
        self.proposals += 1
        return {'ok': True}

    async def close(self):
        return None


@pytest.mark.asyncio
async def test_engine_tick_applies_playbook_on_high_risk() -> None:
    settings = Settings(
        OPERATOR_MIN_EVENTS_FOR_SIGNAL=5,
        OPERATOR_DENY_RATIO_THRESHOLD=0.5,
        OPERATOR_ACTION_COOLDOWN_SECONDS=1,
    )
    client = FakeClient()
    engine = OperatorEngine(settings=settings, client=client)

    await engine.tick_once()

    assert client.actions == 1
    assert client.incidents == 1
    assert client.proposals == 1
    assert engine.state.cursor == 10
