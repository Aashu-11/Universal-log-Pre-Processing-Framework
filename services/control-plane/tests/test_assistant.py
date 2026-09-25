from app.routers.assistant import _deterministic_assist


def test_deterministic_assistant_identifies_syslog_and_network_fields():
    result = _deterministic_assist(
        "<134>1 2026-09-23T12:00:00Z fw action=blocked src=10.0.0.8 dst=198.51.100.4 alert=malware"
    )

    assert result.mode == "deterministic"
    assert any("syslog" in hint for hint in result.parser_hints)
    assert "event.action" in result.field_hints
    assert "src_ip / src_port" in result.field_hints
    assert "dst_ip / dst_port" in result.field_hints
    assert "threat.category" in result.field_hints


def test_assistant_endpoint_returns_local_analysis(client, admin_headers):
    response = client.post(
        "/v1/assistant/analyze",
        json={"log_line": '{"action":"deny","src":"10.0.0.8"}'},
        headers=admin_headers,
    )

    assert response.status_code == 200, response.text
    body = response.json()
    assert body["mode"] == "deterministic"
    assert body["parser_hints"]
    assert body["next_steps"]
