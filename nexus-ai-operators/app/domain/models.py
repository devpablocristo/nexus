from __future__ import annotations

from pydantic import BaseModel, Field


class Event(BaseModel):
    id: int
    event_type: str
    created_at: str
    payload: dict[str, object] = Field(default_factory=dict)


class Signal(BaseModel):
    deny_ratio: float = 0.0
    total_events: int = 0
    denied_events: int = 0
    high_risk: bool = False
    evidence_refs: list[str] = Field(default_factory=list)
