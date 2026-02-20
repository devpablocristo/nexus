from app.domain.models import Event
from app.services.observer import compute_signal


def test_compute_signal_high_risk() -> None:
    events = [
        Event(id=i, event_type='tool.call.completed', created_at='2026-01-01T00:00:00Z', payload={'decision': 'deny' if i < 8 else 'allow'})
        for i in range(10)
    ]

    signal = compute_signal(events, min_events_for_signal=5, deny_ratio_threshold=0.5)

    assert signal.total_events == 10
    assert signal.denied_events == 8
    assert signal.high_risk is True
    assert signal.deny_ratio >= 0.8
