from __future__ import annotations

from typing import Any, cast

from fastapi import APIRouter, Depends, Header, HTTPException, Request
from fastapi.responses import PlainTextResponse
from prometheus_client import CONTENT_TYPE_LATEST, generate_latest
from pydantic import BaseModel, Field

from app.core.config import Settings
from app.services.engine import OperatorEngine
from app.services.llm_client import LLMClient

router = APIRouter()


class AssistantQueryRequest(BaseModel):
    org_id: str
    query: str
    actor: str | None = None


class AssistantQueryResponse(BaseModel):
    summary: str
    tables: list[dict[str, Any]] = Field(default_factory=list)
    actions: list[dict[str, Any]] = Field(default_factory=list)


def _settings(request: Request) -> Settings:
    return cast(Settings, request.app.state.settings)


def _engine(request: Request) -> OperatorEngine:
    return cast(OperatorEngine, request.app.state.engine)


def verify_operator_key(
    request: Request,
    x_operator_key: str | None = Header(default=None, alias='X-Operator-Key'),
) -> None:
    settings = _settings(request)
    if settings.operator_internal_key and x_operator_key != settings.operator_internal_key:
        raise HTTPException(status_code=401, detail='invalid operator key')


@router.get('/healthz')
async def healthz() -> dict[str, bool]:
    return {'ok': True}


@router.get('/readyz')
async def readyz(request: Request) -> dict[str, bool]:
    engine = _engine(request)
    if engine._task is None or engine._task.done():
        raise HTTPException(status_code=503, detail='engine loop not running')
    return {'ok': True}


@router.get('/metrics')
async def metrics() -> PlainTextResponse:
    data = generate_latest()
    return PlainTextResponse(data.decode('utf-8'), media_type=CONTENT_TYPE_LATEST)


@router.post('/v1/internal/tick')
async def tick_once(request: Request, _: None = Depends(verify_operator_key)) -> dict[str, str]:
    engine = _engine(request)
    await engine.tick_once()
    return {'status': 'ok'}


@router.post('/v1/assistant/query', response_model=AssistantQueryResponse)
async def assistant_query(
    request: Request,
    payload: AssistantQueryRequest,
    _: None = Depends(verify_operator_key),
) -> AssistantQueryResponse:
    engine = _engine(request)
    settings = _settings(request)
    state = engine.state

    llm = LLMClient(settings)

    if llm.is_configured:
        system_prompt = (
            "You are the Nexus Operator assistant. You have access to the current operator "
            "state and should answer the user's question concisely.\n\n"
            "## Operator State\n"
            f"- cursor: {state.cursor}\n"
            f"- last_action_at: {state.last_action_at}\n"
            f"- latest_summary: {state.latest_summary}\n"
        )
        summary = await llm.query(system_prompt, payload.query)
    else:
        summary = f"{state.latest_summary} | query={payload.query}"

    return AssistantQueryResponse(
        summary=summary,
        tables=[
            {
                'title': 'Operator State',
                'columns': ['field', 'value'],
                'rows': [
                    {'field': 'cursor', 'value': str(state.cursor)},
                    {'field': 'last_action_at', 'value': str(state.last_action_at)},
                ],
            }
        ],
        actions=[
            {
                'label': 'Run operator tick now',
                'action_type': 'operator.tick',
                'payload': {'endpoint': '/v1/internal/tick'},
            }
        ],
    )
