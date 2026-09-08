from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

from app.routers import onboarding

app = FastAPI(
    title="ULPF Onboarding Engine",
    description="Drain3 template mining, field type/name inference, and draft parser generation for unknown log sources.",
    version="1.0.0",
)

app.add_middleware(
    CORSMiddleware, allow_origins=["*"], allow_methods=["*"], allow_headers=["*"]
)


@app.get("/healthz")
def healthz() -> dict:
    return {"status": "ok"}


app.include_router(onboarding.router)
