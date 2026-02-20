from __future__ import annotations

from app.domain.models import Signal


def score_severity(signal: Signal) -> str:
    if not signal.high_risk:
        return 'LOW'
    if signal.deny_ratio >= 0.8:
        return 'CRIT'
    if signal.deny_ratio >= 0.6:
        return 'HIGH'
    return 'MED'
