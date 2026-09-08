"""Shared Pydantic v2 models. Route-specific request/response models live in
their own router module for locality; only genuinely cross-cutting shapes
belong here.
"""

from datetime import datetime

from pydantic import BaseModel, ConfigDict


class ORMModel(BaseModel):
    """Base for response models read straight off a SQLAlchemy row."""

    model_config = ConfigDict(from_attributes=True)


class TokenResponse(BaseModel):
    access_token: str
    token_type: str = "bearer"
    role: str


class AuditEntry(ORMModel):
    id: str
    actor: str
    action: str
    target: str
    before: dict | None
    after: dict | None
    at: datetime
