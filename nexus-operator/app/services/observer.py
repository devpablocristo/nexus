from __future__ import annotations

from app.domain.models import Event, Signal


def compute_signal(events: list[Event], min_events_for_signal: int, deny_ratio_threshold: float) -> Signal:
    if not events:
        return Signal()

    denied = 0
    evidence_refs: list[str] = []

    for event in events:
        if event.event_type != 'tool.call.completed':
            continue
        decision = str(event.payload.get('decision', '')).lower()
        if decision == 'deny':
            denied += 1
            evidence_refs.append(f"event:{event.id}")

    total_tool_events = sum(1 for event in events if event.event_type == 'tool.call.completed')
    deny_ratio = (denied / total_tool_events) if total_tool_events > 0 else 0.0
    high_risk = total_tool_events >= min_events_for_signal and deny_ratio >= deny_ratio_threshold

    return Signal(
        deny_ratio=deny_ratio,
        total_events=total_tool_events,
        denied_events=denied,
        high_risk=high_risk,
        evidence_refs=evidence_refs[:20],
    )
