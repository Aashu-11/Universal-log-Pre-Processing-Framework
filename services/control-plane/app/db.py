from collections.abc import Generator

from sqlalchemy import create_engine
from sqlalchemy.orm import DeclarativeBase, Session, sessionmaker

from app.config import settings

connect_args = (
    {"check_same_thread": False} if settings.db_url.startswith("sqlite") else {}
)
engine = create_engine(settings.db_url, connect_args=connect_args)
SessionLocal = sessionmaker(bind=engine, autoflush=False, autocommit=False)


class Base(DeclarativeBase):
    pass


def get_db() -> Generator[Session, None, None]:
    db = SessionLocal()
    try:
        yield db
    finally:
        db.close()


def init_db() -> None:
    """Create every table if it doesn't exist. A migration tool (Alembic)
    would be the production-grade choice once this schema needs to evolve
    without dropping data — see docs/DECISIONS.md. For this prototype, a
    fresh schema created directly from the models is both correct and far
    less setup, and `make demo` always starts from an empty database
    anyway.
    """
    Base.metadata.create_all(bind=engine)
