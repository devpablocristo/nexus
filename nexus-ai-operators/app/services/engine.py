from __future__ import annotations

import asyncio
import logging
import time
from dataclasses import dataclass

from app.adapters.nexus_saas_client import NexusSaaSClient
from app.core.config import Settings
from app.core.metrics import ACTIONS_APPLIED, EVENTS_CONSUMED, INCIDENTS_OPENED, LAST_CURSOR, PROPOSALS_CREATED
from app.domain.models import Event
from app.services.observer import compute_signal
from app.services.playbooks import incident_from_signal, throttle_tenant_playbook
from app.services.policy_proposer import build_proposal
from app.services.risk_scorer import score_severity


logger = logging.getLogger(__name__)


@dataclass
class EngineState:
    cursor: int = 0
    last_action_at: float = 0.0
    latest_summary: str = 'Operator started. No events processed yet.'


class OperatorEngine:
    def __init__(self, settings: Settings, client: NexusSaaSClient) -> None:
        self.settings = settings
        self.client = client
        self.state = EngineState()
        self._task: asyncio.Task[None] | None = None
        self._stop = asyncio.Event()

    async def start(self) -> None:
        if self._task is not None:
            return
        self._stop.clear()
        self._task = asyncio.create_task(self._loop(), name='operator-engine-loop')

    async def stop(self) -> None:
        self._stop.set()
        if self._task is not None:
            await self._task
            self._task = None
        await self.client.close()

    async def tick_once(self) -> None:
        data = await self.client.list_events(self.state.cursor, self.settings.poll_batch_size)
        items = [Event.model_validate(item) for item in data.get('items', [])]
        next_cursor = int(data.get('next_cursor', self.state.cursor))
        self.state.cursor = next_cursor
        LAST_CURSOR.set(float(self.state.cursor))

        if not items:
            return

        EVENTS_CONSUMED.inc(len(items))
        signal = compute_signal(
            items,
            min_events_for_signal=self.settings.min_events_for_signal,
            deny_ratio_threshold=self.settings.deny_ratio_threshold,
        )

        self.state.latest_summary = (
            f"Processed {len(items)} events | tool_events={signal.total_events} "
            f"denied={signal.denied_events} deny_ratio={signal.deny_ratio:.2f}"
        )

        if not signal.high_risk:
            logger.info('operator_signal low_risk total=%s deny_ratio=%.2f', signal.total_events, signal.deny_ratio)
            return

        now = time.time()
        if now - self.state.last_action_at < self.settings.action_cooldown_seconds:
            logger.info('operator_signal high_risk but in cooldown')
            return

        severity = score_severity(signal)

        action_payload = throttle_tenant_playbook(signal, self.settings.action_ttl_seconds)
        await self.client.apply_action(action_payload)
        ACTIONS_APPLIED.inc()

        incident_payload = incident_from_signal(signal, severity)
        await self.client.create_incident(incident_payload)
        INCIDENTS_OPENED.inc()

        proposal_payload = build_proposal(signal)
        await self.client.create_policy_proposal(proposal_payload)
        PROPOSALS_CREATED.inc()

        self.state.last_action_at = now
        self.state.latest_summary = (
            f"High-risk signal handled: severity={severity}, deny_ratio={signal.deny_ratio:.2f}. "
            f"Applied temporary action + incident + proposal."
        )
        logger.warning('operator_response_applied severity=%s deny_ratio=%.2f', severity, signal.deny_ratio)

    async def _loop(self) -> None:
        while not self._stop.is_set():
            try:
                await self.tick_once()
            except Exception as exc:  # noqa: BLE001
                logger.exception('operator_tick_failed error=%s', exc)
            try:
                await asyncio.wait_for(self._stop.wait(), timeout=self.settings.poll_interval_seconds)
            except asyncio.TimeoutError:
                pass
