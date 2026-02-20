from __future__ import annotations

from fastapi import FastAPI

from app.adapters.nexus_core_client import NexusCoreClient
from app.api.routes import router
from app.core.config import settings
from app.core.logging import configure_logging
from app.services.engine import OperatorEngine

configure_logging()

app = FastAPI(title='nexus-operator', version='0.1.0')


@app.on_event('startup')
async def startup() -> None:
    client = NexusCoreClient(
        base_url=settings.core_base_url,
        api_key=settings.core_api_key,
        timeout_seconds=settings.core_timeout_seconds,
    )
    engine = OperatorEngine(settings=settings, client=client)
    app.state.settings = settings
    app.state.engine = engine
    await engine.start()


@app.on_event('shutdown')
async def shutdown() -> None:
    engine: OperatorEngine = app.state.engine
    await engine.stop()


app.include_router(router)
