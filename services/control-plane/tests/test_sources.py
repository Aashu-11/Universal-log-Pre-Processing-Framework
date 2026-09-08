def test_create_and_list_sources(client, admin_headers):
    resp = client.post(
        "/v1/sources",
        json={
            "name": "edge-fw-01",
            "vendor": "paloalto",
            "product": "panos",
            "observer_type": "firewall",
            "binding_peer_ip": "10.1.1.1",
            "binding_listener": "syslog-tcp",
            "parser_id": "paloalto.panos.traffic",
        },
        headers=admin_headers,
    )
    assert resp.status_code == 201, resp.text
    created = resp.json()
    assert created["status"] == "active"
    assert created["log_source_id"]

    listed = client.get("/v1/sources", headers=admin_headers).json()
    assert any(s["name"] == "edge-fw-01" for s in listed)


def test_update_source_writes_audit_log(client, admin_headers, db_session):
    from app.models import AuditLog

    created = client.post(
        "/v1/sources",
        json={
            "name": "fw-a",
            "vendor": "cisco",
            "product": "asa",
            "observer_type": "firewall",
        },
        headers=admin_headers,
    ).json()

    updated = client.put(
        f"/v1/sources/{created['log_source_id']}",
        json={
            "name": "fw-a-renamed",
            "vendor": "cisco",
            "product": "asa",
            "observer_type": "firewall",
        },
        headers=admin_headers,
    )
    assert updated.status_code == 200
    assert updated.json()["name"] == "fw-a-renamed"

    entries = (
        db_session.query(AuditLog).filter(AuditLog.action == "update_source").all()
    )
    assert len(entries) == 1
    assert entries[0].before["name"] == "fw-a"
    assert entries[0].after["name"] == "fw-a-renamed"


def test_update_missing_source_404(client, admin_headers):
    resp = client.put(
        "/v1/sources/does-not-exist",
        json={"name": "x", "vendor": "v", "product": "p", "observer_type": "firewall"},
        headers=admin_headers,
    )
    assert resp.status_code == 404
