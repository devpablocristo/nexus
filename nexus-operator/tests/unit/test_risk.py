from app.domain.models import Signal
from app.services.risk_scorer import score_severity


def test_score_severity_levels() -> None:
    assert score_severity(Signal(high_risk=False)) == 'LOW'
    assert score_severity(Signal(high_risk=True, deny_ratio=0.61)) == 'HIGH'
    assert score_severity(Signal(high_risk=True, deny_ratio=0.85)) == 'CRIT'
