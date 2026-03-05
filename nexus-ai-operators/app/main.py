from __future__ import annotations

from collections.abc import AsyncIterator
from contextlib import asynccontextmanager

from fastapi import FastAPI
from fastapi.responses import JSONResponse
from starlette.middleware.cors import CORSMiddleware
from starlette.types import ASGIApp, Message, Receive, Scope, Send

from app.adapters.nexus_core_client import NexusCoreClient
from app.api.routes import router
from app.core.config import settings
from app.core.logging import configure_logging
from app.services.engine import OperatorEngine

configure_logging()


class RequestTooLargeError(Exception):
    pass


class BodySizeLimitMiddleware:
    def __init__(self, app: ASGIApp, max_body_bytes: int) -> None:
        self.app = app
        self.max_body_bytes = max_body_bytes

    async def __call__(self, scope: Scope, receive: Receive, send: Send) -> None:
        if scope["type"] != "http":
            await self.app(scope, receive, send)
            return

        raw_content_length = next((v for (k, v) in scope["headers"] if k == b"content-length"), b"")
        if raw_content_length:
            try:
                if int(raw_content_length) > self.max_body_bytes:
                    response = JSONResponse({"detail": "request body too large"}, status_code=413)
                    await response(scope, receive, send)
                    return
            except ValueError:
                pass

        received = 0

        async def limited_receive() -> Message:
            nonlocal received
            message = await receive()
            if message["type"] == "http.request":
                received += len(message.get("body", b""))
                if received > self.max_body_bytes:
                    raise RequestTooLargeError
            return message

        try:
            await self.app(scope, limited_receive, send)
        except RequestTooLargeError:
            response = JSONResponse({"detail": "request body too large"}, status_code=413)
            await response(scope, receive, send)


@asynccontextmanager
async def lifespan(application: FastAPI) -> AsyncIterator[None]:
    client = NexusCoreClient(
        base_url=settings.core_base_url,
        api_key=settings.core_api_key,
        timeout_seconds=settings.core_timeout_seconds,
    )
    engine = OperatorEngine(settings=settings, client=client)
    application.state.settings = settings
    application.state.engine = engine
    await engine.start()
    yield
    await engine.stop()


app = FastAPI(title='nexus-ai-operators', version='0.1.0', lifespan=lifespan)
app.add_middleware(BodySizeLimitMiddleware, max_body_bytes=settings.max_body_bytes)
app.add_middleware(
    CORSMiddleware,
    allow_origins=[s.strip() for s in settings.cors_allowed_origins.split(",") if s.strip()],
    allow_credentials=False,
    allow_methods=["GET", "POST", "OPTIONS"],
    allow_headers=["Authorization", "Content-Type", "X-Operator-Key", "X-NEXUS-CORE-KEY", "X-NEXUS-AI-KEY"],
)
app.include_router(router)
