from __future__ import annotations

from app.domain.models import Signal
from app.services.playbooks import proposal_from_signal


def build_proposal(signal: Signal) -> dict[str, object]:
    return proposal_from_signal(signal)
