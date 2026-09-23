"""JWT auth + RBAC. Four roles per CLAUDE.md: admin, engineer, analyst,
auditor (read-only). Every mutating endpoint requires admin or engineer;
auditor can read everything but write nothing — enforced by the
`require_role` dependency each router applies per-route.
"""

from datetime import datetime, timedelta, timezone

from fastapi import Depends, HTTPException, status
from fastapi.security import HTTPAuthorizationCredentials, HTTPBearer
from jose import JWTError, jwt
from passlib.context import CryptContext
from sqlalchemy.orm import Session

from app.config import settings
from app.models import User

pwd_context = CryptContext(schemes=["bcrypt"], deprecated="auto")
bearer_scheme = HTTPBearer(auto_error=False)

ROLES = ("admin", "engineer", "analyst", "auditor")


def hash_password(password: str) -> str:
    return pwd_context.hash(password)


def verify_password(password: str, hashed: str) -> bool:
    return pwd_context.verify(password, hashed)


def create_access_token(username: str, role: str) -> str:
    expire = datetime.now(timezone.utc) + timedelta(minutes=settings.jwt_expire_minutes)
    payload = {"sub": username, "role": role, "exp": expire}
    return jwt.encode(payload, settings.jwt_secret, algorithm=settings.jwt_algorithm)


class CurrentUser:
    def __init__(self, username: str, role: str):
        self.username = username
        self.role = role


def get_current_user(
    creds: HTTPAuthorizationCredentials | None = Depends(bearer_scheme),
) -> CurrentUser:
    if creds is None:
        raise HTTPException(status.HTTP_401_UNAUTHORIZED, "missing bearer token")
    try:
        payload = jwt.decode(
            creds.credentials, settings.jwt_secret, algorithms=[settings.jwt_algorithm]
        )
    except JWTError as e:
        raise HTTPException(status.HTTP_401_UNAUTHORIZED, "invalid token") from e
    username = payload.get("sub")
    role = payload.get("role")
    if not username or role not in ROLES:
        raise HTTPException(status.HTTP_401_UNAUTHORIZED, "malformed token")
    return CurrentUser(username=username, role=role)


def require_role(*allowed_roles: str):
    """FastAPI dependency factory: `Depends(require_role("admin", "engineer"))`."""

    def _dep(user: CurrentUser = Depends(get_current_user)) -> CurrentUser:
        if user.role not in allowed_roles:
            raise HTTPException(
                status.HTTP_403_FORBIDDEN,
                f"role {user.role!r} may not perform this action (requires one of {allowed_roles})",
            )
        return user

    return _dep


def ensure_seed_admin(db: Session) -> None:
    """Creates a default admin/admin account on first startup if the users
    table is empty, purely so `make demo` works with zero manual setup.
    Dev-only credential — a real deployment sets LOGKRAMA_ADMIN_PASSWORD (or
    just changes the password immediately after first login).
    """
    import os

    if db.query(User).count() > 0:
        return
    password = os.environ.get("LOGKRAMA_ADMIN_PASSWORD", "admin")
    db.add(
        User(username="admin", hashed_password=hash_password(password), role="admin")
    )
    db.commit()
