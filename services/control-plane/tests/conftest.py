import os

import pytest
from fastapi.testclient import TestClient
from sqlalchemy import create_engine
from sqlalchemy.orm import sessionmaker
from sqlalchemy.pool import StaticPool

os.environ.setdefault("ULPF_DB_URL", "sqlite:///:memory:")
os.environ.setdefault("ULPF_JWT_SECRET", "test-secret")

from app.db import Base, get_db  # noqa: E402
from app.main import app  # noqa: E402
from app.security import ensure_seed_admin  # noqa: E402


@pytest.fixture()
def db_session():
    # StaticPool is required for a SQLite ":memory:" engine under test: without
    # it, each checked-out connection is a *separate* empty in-memory database
    # (SQLite only shares memory-db state within one connection), so a second
    # request from a different thread (FastAPI runs sync routes via a worker
    # threadpool) would see "no such table" even though setup just ran.
    engine = create_engine(
        "sqlite:///:memory:",
        connect_args={"check_same_thread": False},
        poolclass=StaticPool,
    )
    Base.metadata.create_all(bind=engine)
    TestingSessionLocal = sessionmaker(bind=engine, autoflush=False, autocommit=False)
    session = TestingSessionLocal()
    try:
        yield session
    finally:
        session.close()


@pytest.fixture()
def client(db_session):
    def override_get_db():
        try:
            yield db_session
        finally:
            pass

    app.dependency_overrides[get_db] = override_get_db
    ensure_seed_admin(db_session)
    db_session.commit()
    with TestClient(app) as c:
        yield c
    app.dependency_overrides.clear()


@pytest.fixture()
def admin_token(client):
    resp = client.post(
        "/v1/auth/login", json={"username": "admin", "password": "admin"}
    )
    assert resp.status_code == 200, resp.text
    return resp.json()["access_token"]


@pytest.fixture()
def admin_headers(admin_token):
    return {"Authorization": f"Bearer {admin_token}"}
