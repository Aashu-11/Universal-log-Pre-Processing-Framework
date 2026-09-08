from sqlalchemy.orm import Session

from app.models import AuditLog


def log(
    db: Session,
    actor: str,
    action: str,
    target: str,
    before: dict | None = None,
    after: dict | None = None,
) -> None:
    """Every mutating endpoint calls this — CLAUDE.md: 'Every mutating call
    writes to audit_log with before/after.' Caller commits alongside its own
    change so the audit row and the mutation are one transaction.
    """
    db.add(
        AuditLog(actor=actor, action=action, target=target, before=before, after=after)
    )
