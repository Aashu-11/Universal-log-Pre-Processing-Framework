"""DLQ inspection + replay. Rows here are populated by a Kafka consumer
bridging the logkrama.dlq topic into this table — not yet wired end-to-end
since it needs a reachable Kafka broker to test against (see
docs/DECISIONS.md). POST /v1/dlq lets a bridge (or a test) insert one row
directly in the meantime; the read/replay endpoints are fully real either
way.
"""

from datetime import datetime, timezone
import json

from fastapi import APIRouter, Depends, HTTPException, Query, status
from kafka import KafkaProducer
from pydantic import BaseModel
from sqlalchemy.orm import Session

from app import audit
from app.config import settings
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
    limit: int | None = Query(default=None, ge=1, le=500),
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
    q = q.order_by(DLQEvent.occurred_at.desc())
    if limit is not None:
        q = q.limit(limit)
    return q.all()


class ReplayRequest(BaseModel):
    event_ids: list[str]


class ReplayResponse(BaseModel):
    resolved: list[str]
    not_found: list[str]


def _publish_replay(event_id: str, raw_ref: dict) -> None:
    """Return an immutable raw reference to the processor's normal ingress."""
    message = {
        "event_id": event_id,
        "ref": raw_ref,
        "listener_id": "dlq-replay",
        "peer_ip": "",
        "received_at": datetime.now(timezone.utc).isoformat(),
    }
    producer = KafkaProducer(
        bootstrap_servers=[broker.strip() for broker in settings.kafka_brokers.split(",")],
        value_serializer=lambda value: json.dumps(value).encode("utf-8"),
        acks="all",
        retries=3,
        request_timeout_ms=10_000,
    )
    try:
        producer.send(settings.kafka_topic_raw_refs, key=event_id.encode("utf-8"), value=message).get(timeout=15)
    finally:
        producer.close(timeout=5)


@router.post("/replay", response_model=ReplayResponse)
def replay(
    body: ReplayRequest,
    db: Session = Depends(get_db),
    user: CurrentUser = Depends(require_role("admin", "engineer")),
):
    """Marks the given DLQ events resolved. Actually re-driving them through
    the pipeline (re-publishing their raw_ref to logkrama.raw.refs) is the
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
        try:
            _publish_replay(row.event_id, row.raw_ref)
        except Exception as exc:
            # A record is resolved only after Kafka accepts the replay.
            raise HTTPException(
                status.HTTP_503_SERVICE_UNAVAILABLE,
                f"replay for {event_id} was not accepted by Kafka: {exc}",
            ) from exc
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
