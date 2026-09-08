"""DLQ inspection + replay. Rows here are populated by a Kafka consumer
bridging the ulpf.dlq topic into this table — not yet wired end-to-end
since it needs a reachable Kafka broker to test against (see
docs/DECISIONS.md). POST /v1/dlq lets a bridge (or a test) insert one row
directly in the meantime; the read/replay endpoints are fully real either
way.
"""

from datetime import datetime

from fastapi import APIRouter, Depends, status
from pydantic import BaseModel
from sqlalchemy.orm import Session

from app import audit
from app.db import get_db
from app.models import DLQEvent
from app.schemas import ORMModel
from app.security import CurrentUser, require_role

router = APIRouter(prefix="/v1/dlq", tags=["dlq"])


class DLQEventIn(BaseModel):
    event_id: str
    raw_ref: dict
    reason: str
    parser_id: str | None = None


class DLQEventOut(ORMModel):
    event_id: str
    raw_ref: dict
    reason: str
    parser_id: str | None
    occurred_at: datetime
    resolved: bool


@router.post("", response_model=DLQEventOut, status_code=status.HTTP_201_CREATED)
def ingest_dlq_event(body: DLQEventIn, db: Session = Depends(get_db)):
    row = DLQEvent(**body.model_dump())
    db.merge(row)
    db.commit()
    return db.get(DLQEvent, body.event_id)


@router.get("", response_model=list[DLQEventOut])
def list_dlq(
    reason: str | None = None,
    parser_id: str | None = None,
    resolved: bool | None = None,
    db: Session = Depends(get_db),
    _: CurrentUser = Depends(require_role("admin", "engineer", "analyst", "auditor")),
):
    q = db.query(DLQEvent)
    if reason:
        q = q.filter(DLQEvent.reason == reason)
    if parser_id:
        q = q.filter(DLQEvent.parser_id == parser_id)
    if resolved is not None:
        q = q.filter(DLQEvent.resolved == resolved)
    return q.order_by(DLQEvent.occurred_at.desc()).all()


class ReplayRequest(BaseModel):
    event_ids: list[str]


class ReplayResponse(BaseModel):
    resolved: list[str]
    not_found: list[str]


@router.post("/replay", response_model=ReplayResponse)
def replay(
    body: ReplayRequest,
    db: Session = Depends(get_db),
    user: CurrentUser = Depends(require_role("admin", "engineer")),
):
    """Marks the given DLQ events resolved. Actually re-driving them through
    the pipeline (re-publishing their raw_ref to ulpf.raw.refs) is the
    natural next step once a parser fix is published — that publish call is
    a one-line Kafka produce once a broker is reachable to test against;
    marking-resolved is what's verifiable without one, so that's what's
    implemented and tested now.
    """
    resolved, not_found = [], []
    for event_id in body.event_ids:
        row = db.get(DLQEvent, event_id)
        if row is None:
            not_found.append(event_id)
            continue
        row.resolved = True
        resolved.append(event_id)
    audit.log(
        db,
        user.username,
        "dlq_replay",
        ",".join(resolved),
        after={"resolved": resolved},
    )
    db.commit()
    return ReplayResponse(resolved=resolved, not_found=not_found)
