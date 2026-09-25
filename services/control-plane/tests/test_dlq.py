def test_ingest_list_and_replay(client, admin_headers, monkeypatch):
    published = []
    monkeypatch.setattr(
        "app.routers.dlq._publish_replay",
        lambda event_id, raw_ref: published.append((event_id, raw_ref)),
    )
    ingested = client.post(
        "/v1/dlq",
        json={
            "event_id": "evt-bad-1",
            "raw_ref": {
                "segment_id": "seg-1",
                "offset": 0,
                "length": 10,
                "sha256": "abc",
            },
            "reason": "schema_violation",
            "parser_id": "paloalto.panos.traffic",
        },
    )
    assert ingested.status_code == 201, ingested.text

    listed = client.get("/v1/dlq", headers=admin_headers).json()
    assert any(e["event_id"] == "evt-bad-1" for e in listed)

    filtered = client.get(
        "/v1/dlq?reason=schema_violation", headers=admin_headers
    ).json()
    assert len(filtered) == 1

    limited = client.get("/v1/dlq?limit=1", headers=admin_headers)
    assert limited.status_code == 200
    assert len(limited.json()) == 1
    assert client.get("/v1/dlq?limit=0", headers=admin_headers).status_code == 422

    replay = client.post(
        "/v1/dlq/replay",
        json={"event_ids": ["evt-bad-1", "does-not-exist"]},
        headers=admin_headers,
    )
    assert replay.status_code == 200
    body = replay.json()
    assert body["resolved"] == ["evt-bad-1"]
    assert body["not_found"] == ["does-not-exist"]
    assert published == [("evt-bad-1", {"segment_id": "seg-1", "offset": 0, "length": 10, "sha256": "abc"})]

    resolved_only = client.get("/v1/dlq?resolved=true", headers=admin_headers).json()
    assert any(e["event_id"] == "evt-bad-1" for e in resolved_only)


def test_replay_keeps_event_open_when_kafka_rejects(client, admin_headers, monkeypatch):
    client.post(
        "/v1/dlq",
        json={
            "event_id": "evt-retry-1",
            "raw_ref": {"segment_id": "seg-1", "offset": 0, "length": 10, "sha256": "abc"},
            "reason": "schema_violation",
        },
    )
    monkeypatch.setattr(
        "app.routers.dlq._publish_replay",
        lambda *_: (_ for _ in ()).throw(RuntimeError("Kafka unavailable")),
    )

    response = client.post(
        "/v1/dlq/replay", json={"event_ids": ["evt-retry-1"]}, headers=admin_headers
    )
    assert response.status_code == 503
    rows = client.get("/v1/dlq?resolved=false", headers=admin_headers).json()
    assert any(row["event_id"] == "evt-retry-1" for row in rows)
