from datetime import datetime, timezone

from fastapi import APIRouter, Depends, HTTPException, status
from pydantic import BaseModel
from sqlalchemy.orm import Session

from app import audit
from app.db import get_db
from app.models import Source
from app.schemas import ORMModel
from app.security import CurrentUser, require_role

router = APIRouter(prefix="/v1/sources", tags=["sources"])


class SourceIn(BaseModel):
    name: str
    vendor: str
    product: str
    observer_type: str
    binding_peer_ip: str | None = None
    binding_listener: str | None = None
    parser_id: str | None = None
    timezone: str = "UTC"
    enabled: bool = True


class SourceOut(ORMModel):
    log_source_id: str
    name: str
    vendor: str
    product: str
    observer_type: str
    binding_peer_ip: str | None
    binding_listener: str | None
    parser_id: str | None
    timezone: str
    enabled: bool
    first_seen: datetime
    last_seen: datetime
    status: str


@router.get("", response_model=list[SourceOut])
def list_sources(
    db: Session = Depends(get_db),
    _: CurrentUser = Depends(require_role("admin", "engineer", "analyst", "auditor")),
):
    return db.query(Source).order_by(Source.name).all()


@router.post("", response_model=SourceOut, status_code=status.HTTP_201_CREATED)
def create_source(
    body: SourceIn,
    db: Session = Depends(get_db),
    user: CurrentUser = Depends(require_role("admin", "engineer")),
):
    now = datetime.now(timezone.utc)
    src = Source(**body.model_dump(), first_seen=now, last_seen=now, status="active")
    db.add(src)
    db.flush()
    audit.log(
        db,
        user.username,
        "create_source",
        src.log_source_id,
        before=None,
        after=body.model_dump(),
    )
    db.commit()
    db.refresh(src)
    return src


@router.put("/{log_source_id}", response_model=SourceOut)
def update_source(
    log_source_id: str,
    body: SourceIn,
    db: Session = Depends(get_db),
    user: CurrentUser = Depends(require_role("admin", "engineer")),
):
    src = db.get(Source, log_source_id)
    if src is None:
        raise HTTPException(status.HTTP_404_NOT_FOUND, "source not found")
    before = {c.name: getattr(src, c.name) for c in Source.__table__.columns}
    for k, v in body.model_dump().items():
        setattr(src, k, v)
    db.flush()
    after = {c.name: getattr(src, c.name) for c in Source.__table__.columns}
    audit.log(
        db,
        user.username,
        "update_source",
        log_source_id,
        before=_jsonable(before),
        after=_jsonable(after),
    )
    db.commit()
    db.refresh(src)
    return src


def _jsonable(d: dict) -> dict:
    return {k: (v.isoformat() if hasattr(v, "isoformat") else v) for k, v in d.items()}
