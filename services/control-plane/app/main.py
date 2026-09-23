from contextlib import asynccontextmanager

from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

from app.db import SessionLocal, init_db
from app.routers import (
    auth,
    dlq,
    events,
    integrity,
    parsers,
    query,
    reviewer,
    sources,
    stats,
)
from app.security import ensure_seed_admin


@asynccontextmanager
async def lifespan(_: FastAPI):
    init_db()
    db = SessionLocal()
    try:
        ensure_seed_admin(db)
    finally:
        db.close()
    yield


app = FastAPI(
    title="LOGKRAMA Control Plane",
    description="Parser registry, source inventory, integrity/traceability API, and the read-only Presto query proxy.",
    version="1.0.0",
    lifespan=lifespan,
)

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],  # console dev server; tighten for a real deployment
    allow_methods=["*"],
    allow_headers=["*"],
)


@app.get("/healthz")
def healthz() -> dict:
    return {"status": "ok"}


app.include_router(auth.router)
app.include_router(sources.router)
app.include_router(parsers.router)
app.include_router(events.router)
app.include_router(integrity.router)
app.include_router(dlq.router)
app.include_router(stats.router)
app.include_router(query.router)
app.include_router(reviewer.router)
