def test_login_success(client):
    resp = client.post(
        "/v1/auth/login", json={"username": "admin", "password": "admin"}
    )
    assert resp.status_code == 200
    body = resp.json()
    assert body["role"] == "admin"
    assert body["access_token"]


def test_login_bad_password(client):
    resp = client.post(
        "/v1/auth/login", json={"username": "admin", "password": "wrong"}
    )
    assert resp.status_code == 401


def test_protected_route_requires_token(client):
    resp = client.get("/v1/sources")
    assert resp.status_code == 401


def test_protected_route_rejects_bad_token(client):
    resp = client.get(
        "/v1/sources", headers={"Authorization": "Bearer not-a-real-token"}
    )
    assert resp.status_code == 401


def test_auditor_cannot_create_source(client, db_session):
    from app.models import User
    from app.security import hash_password

    db_session.add(
        User(username="auditor1", hashed_password=hash_password("pw"), role="auditor")
    )
    db_session.commit()
    token = client.post(
        "/v1/auth/login", json={"username": "auditor1", "password": "pw"}
    ).json()["access_token"]

    resp = client.post(
        "/v1/sources",
        json={"name": "x", "vendor": "v", "product": "p", "observer_type": "firewall"},
        headers={"Authorization": f"Bearer {token}"},
    )
    assert resp.status_code == 403
