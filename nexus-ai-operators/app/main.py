from __future__ import annotations

from collections.abc import AsyncIterator
from contextlib import asynccontextmanager

from fastapi import FastAPI

from app.adapters.nexus_saas_client import NexusSaaSClient
from app.api.routes import router
from app.core.config import settings
from app.core.logging import configure_logging
from app.services.engine import OperatorEngine

configure_logging()


@asynccontextmanager
async def lifespan(application: FastAPI) -> AsyncIterator[None]:
    client = NexusSaaSClient(
        base_url=settings.saas_base_url,
        api_key=settings.saas_api_key,
        timeout_seconds=settings.saas_timeout_seconds,
    )
    engine = OperatorEngine(settings=settings, client=client)
    application.state.settings = settings
    application.state.engine = engine
    await engine.start()
    yield
    await engine.stop()


app = FastAPI(title='nexus-ai-operators', version='0.1.0', lifespan=lifespan)
app.include_router(router)
