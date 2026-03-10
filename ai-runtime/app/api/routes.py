from __future__ import annotations

import threading
import time
from uuid import uuid4
from typing import Any, cast

from fastapi import APIRouter, Depends, Header, HTTPException, Request
from fastapi.responses import PlainTextResponse
from prometheus_client import CONTENT_TYPE_LATEST, generate_latest
from pydantic import BaseModel, Field

from app.core.config import Settings
from app.services.engine import OperatorEngine
from app.services.prompt_runtime import PromptRuntime

router = APIRouter()


class AssistantQueryRequest(BaseModel):
    org_id: str
    query: str
    actor: str | None = None


class AssistantQueryResponse(BaseModel):
    summary: str
    tables: list[dict[str, Any]] = Field(default_factory=list)
    actions: list[dict[str, Any]] = Field(default_factory=list)


class AssistantRateLimiter:
    def __init__(self) -> None:
        self._lock = threading.Lock()
        self._windows: dict[str, tuple[int, int]] = {}

    def allow(self, key: str, limit_per_min: int) -> bool:
        now_window = int(time.time() // 60)
        with self._lock:
            # Keep only current/previous minute to avoid unbounded growth.
            stale = [k for k, (window, _) in self._windows.items() if now_window-window > 1]
            for k in stale:
                del self._windows[k]

            window, count = self._windows.get(key, (now_window, 0))
            if window != now_window:
                window, count = now_window, 0
            if count >= limit_per_min:
                self._windows[key] = (window, count)
                return False
            self._windows[key] = (window, count + 1)
            return True


assistant_rate_limiter = AssistantRateLimiter()


def _settings(request: Request) -> Settings:
    return cast(Settings, request.app.state.settings)


def _engine(request: Request) -> OperatorEngine:
    return cast(OperatorEngine, request.app.state.engine)


def _prompt_runtime(request: Request) -> PromptRuntime:
    runtime = getattr(request.app.state, 'prompt_runtime', None)
    if runtime is None:
        runtime = PromptRuntime(_settings(request))
        request.app.state.prompt_runtime = runtime
    return cast(PromptRuntime, runtime)


async def verify_operator_key(
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
    if not assistant_rate_limiter.allow(payload.org_id, settings.assistant_rate_limit_per_min):
        raise HTTPException(status_code=429, detail='rate limit exceeded')

    runtime = _prompt_runtime(request)
    request_id = request.headers.get('X-Request-ID') or str(uuid4())
    result = await runtime.generate_summary(
        prompt_id='assistant_system',
        org_id=payload.org_id,
        actor=payload.actor,
        query=payload.query,
        engine_state=engine.state,
        request_id=request_id,
    )

    return AssistantQueryResponse(
        summary=result.summary,
        tables=[
            {
                'title': 'Operator State',
                'columns': ['field', 'value'],
                'rows': [
                    {'field': 'cursor', 'value': str(result.context.cursor)},
                    {'field': 'last_action_at', 'value': result.context.last_action_at},
                    {'field': 'prompt_id', 'value': result.prompt_id},
                    {'field': 'prompt_version', 'value': result.prompt_version},
                    {'field': 'backend', 'value': result.backend},
                ]
                + [
                    {'field': field_name, 'value': value}
                    for field_name, value in result.context.assistant_overview.items()
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
