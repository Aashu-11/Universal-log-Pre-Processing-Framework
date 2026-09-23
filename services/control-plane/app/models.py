"""SQLAlchemy ORM models for logkrama_meta — kept flat and query-friendly per
CLAUDE.md's Phase 7 note, since Presto's `meta` catalog (postgresql
connector) reads these tables directly, no view layer in between.
"""

import uuid
from datetime import datetime, timezone

from sqlalchemy import JSON, Boolean, DateTime, ForeignKey, Integer, String, Text
from sqlalchemy.orm import Mapped, mapped_column, relationship

from app.db import Base


def _now() -> datetime:
    return datetime.now(timezone.utc)


def _uuid() -> str:
    return str(uuid.uuid4())


class Source(Base):
    __tablename__ = "sources"

    log_source_id: Mapped[str] = mapped_column(String, primary_key=True, default=_uuid)
    name: Mapped[str] = mapped_column(String, nullable=False)
    vendor: Mapped[str] = mapped_column(String, nullable=False)
    product: Mapped[str] = mapped_column(String, nullable=False)
    observer_type: Mapped[str] = mapped_column(String, nullable=False)
    binding_peer_ip: Mapped[str | None] = mapped_column(String, nullable=True)
    binding_listener: Mapped[str | None] = mapped_column(String, nullable=True)
    parser_id: Mapped[str | None] = mapped_column(String, nullable=True)
    timezone: Mapped[str] = mapped_column(String, default="UTC")
    enabled: Mapped[bool] = mapped_column(Boolean, default=True)
    first_seen: Mapped[datetime] = mapped_column(DateTime(timezone=True), default=_now)
    last_seen: Mapped[datetime] = mapped_column(DateTime(timezone=True), default=_now)
    status: Mapped[str] = mapped_column(
        String, default="active"
    )  # active | unknown | disabled


class ParserRegistry(Base):
    __tablename__ = "parser_registry"

    id: Mapped[str] = mapped_column(String, primary_key=True, default=_uuid)
    parser_id: Mapped[str] = mapped_column(String, nullable=False, index=True)
    version: Mapped[str] = mapped_column(String, nullable=False)
    pack_id: Mapped[str] = mapped_column(String, nullable=False)
    sha256: Mapped[str] = mapped_column(String, nullable=False)
    signature: Mapped[str | None] = mapped_column(String, nullable=True)
    state: Mapped[str] = mapped_column(
        String, default="published"
    )  # draft | published | rolled_back
    published_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), default=_now
    )
    published_by: Mapped[str] = mapped_column(String, nullable=False)
    changelog: Mapped[str | None] = mapped_column(Text, nullable=True)

    artifact: Mapped["ParserArtifact"] = relationship(
        back_populates="registry_entry", uselist=False
    )


class ParserArtifact(Base):
    __tablename__ = "parser_artifacts"

    id: Mapped[str] = mapped_column(String, primary_key=True, default=_uuid)
    registry_id: Mapped[str] = mapped_column(
        ForeignKey("parser_registry.id"), nullable=False, unique=True
    )
    parser_id: Mapped[str] = mapped_column(String, nullable=False)
    version: Mapped[str] = mapped_column(String, nullable=False)
    parser_yaml: Mapped[str] = mapped_column(Text, nullable=False)
    mapping_yaml: Mapped[str] = mapped_column(Text, nullable=False)

    registry_entry: Mapped["ParserRegistry"] = relationship(back_populates="artifact")


class Route(Base):
    __tablename__ = "routes"

    id: Mapped[str] = mapped_column(String, primary_key=True, default=_uuid)
    expression: Mapped[str] = mapped_column(String, nullable=False)
    sinks: Mapped[list] = mapped_column(JSON, nullable=False, default=list)
    priority: Mapped[int] = mapped_column(Integer, default=100)
    enabled: Mapped[bool] = mapped_column(Boolean, default=True)


class AuditLog(Base):
    __tablename__ = "audit_log"

    id: Mapped[str] = mapped_column(String, primary_key=True, default=_uuid)
    actor: Mapped[str] = mapped_column(String, nullable=False)
    action: Mapped[str] = mapped_column(String, nullable=False)
    target: Mapped[str] = mapped_column(String, nullable=False)
    before: Mapped[dict | None] = mapped_column(JSON, nullable=True)
    after: Mapped[dict | None] = mapped_column(JSON, nullable=True)
    at: Mapped[datetime] = mapped_column(DateTime(timezone=True), default=_now)


class DLQEvent(Base):
    __tablename__ = "dlq_events"

    event_id: Mapped[str] = mapped_column(String, primary_key=True)
    raw_ref: Mapped[dict] = mapped_column(JSON, nullable=False)
    reason: Mapped[str] = mapped_column(String, nullable=False)
    parser_id: Mapped[str | None] = mapped_column(String, nullable=True)
    occurred_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), default=_now)
    resolved: Mapped[bool] = mapped_column(Boolean, default=False)


class ProcessorNode(Base):
    """Rollout status per processor node — which parser version each node
    has actually loaded, reported back after every hot reload (Phase 7's
    "rollout status per node" deliverable).
    """

    __tablename__ = "processor_nodes"

    node_id: Mapped[str] = mapped_column(String, primary_key=True)
    parser_id: Mapped[str] = mapped_column(String, primary_key=True)
    loaded_version: Mapped[str] = mapped_column(String, nullable=False)
    reported_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), default=_now)


class User(Base):
    __tablename__ = "users"

    username: Mapped[str] = mapped_column(String, primary_key=True)
    hashed_password: Mapped[str] = mapped_column(String, nullable=False)
    role: Mapped[str] = mapped_column(
        String, nullable=False
    )  # admin | engineer | analyst | auditor
