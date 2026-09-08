from app.routers.query import is_statement_allowed


def test_allowlist_permits_select_explain_show():
    assert is_statement_allowed("SELECT * FROM lake.ulpf.events")
    assert is_statement_allowed("  select 1")
    assert is_statement_allowed("EXPLAIN SELECT * FROM x")
    assert is_statement_allowed("SHOW CATALOGS")
    assert is_statement_allowed("-- a comment\nSELECT 1")


def test_allowlist_rejects_mutations():
    assert not is_statement_allowed("DROP TABLE lake.ulpf.events")
    assert not is_statement_allowed("DELETE FROM lake.ulpf.events")
    assert not is_statement_allowed("INSERT INTO lake.ulpf.events VALUES (1)")
    assert not is_statement_allowed("UPDATE lake.ulpf.events SET x=1")
    assert not is_statement_allowed("CREATE TABLE evil (x int)")


def test_query_endpoint_rejects_drop_table_without_even_trying_presto(
    client, admin_headers
):
    resp = client.post(
        "/v1/query",
        json={"sql": "DROP TABLE lake.ulpf.events"},
        headers=admin_headers,
    )
    assert resp.status_code == 403


def test_query_endpoint_accepts_select_shape(client, admin_headers):
    # No live Presto in this environment: the important assertion is that
    # the request gets *past* the allowlist (any failure here must be a
    # 502 "presto unreachable", never a 403).
    resp = client.post(
        "/v1/query",
        json={"sql": "SELECT 1"},
        headers=admin_headers,
    )
    assert resp.status_code != 403
