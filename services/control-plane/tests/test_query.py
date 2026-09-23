from app.routers.query import is_statement_allowed, normalize_sql


def test_allowlist_permits_select_explain_show():
    assert is_statement_allowed("SELECT * FROM lake.logkrama.events")
    assert is_statement_allowed("  select 1")
    assert is_statement_allowed("EXPLAIN SELECT * FROM x")
    assert is_statement_allowed("SHOW CATALOGS")
    assert is_statement_allowed("-- a comment\nSELECT 1")


def test_allowlist_rejects_mutations():
    assert not is_statement_allowed("DROP TABLE lake.logkrama.events")
    assert not is_statement_allowed("DELETE FROM lake.logkrama.events")
    assert not is_statement_allowed("INSERT INTO lake.logkrama.events VALUES (1)")
    assert not is_statement_allowed("UPDATE lake.logkrama.events SET x=1")
    assert not is_statement_allowed("CREATE TABLE evil (x int)")


def test_query_endpoint_rejects_drop_table_without_even_trying_presto(
    client, admin_headers
):
    resp = client.post(
        "/v1/query",
        json={"sql": "DROP TABLE lake.logkrama.events"},
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


def test_normalize_sql_strips_trailing_semicolon_and_whitespace():
    # Regression: every saved query (and most hand-typed ones) ends with
    # a ';' out of CLI habit, but Presto's client protocol treats that as
    # a syntax error — it isn't a multi-statement script. Found live: this
    # broke every single saved query in the Explorer page at once.
    assert normalize_sql("SELECT 1;") == "SELECT 1"
    assert normalize_sql("SELECT 1 ;  \n") == "SELECT 1"
    assert normalize_sql("SELECT 1\nGROUP BY 1;\n") == "SELECT 1\nGROUP BY 1"


def test_normalize_sql_leaves_semicolon_free_query_untouched():
    assert normalize_sql("SELECT 1") == "SELECT 1"


def test_query_endpoint_rejects_drop_table_even_with_trailing_semicolon(
    client, admin_headers
):
    # normalize_sql runs before the allowlist check — confirms stripping
    # the semicolon doesn't accidentally let a disguised mutation through.
    resp = client.post(
        "/v1/query",
        json={"sql": "DROP TABLE lake.logkrama.events;"},
        headers=admin_headers,
    )
    assert resp.status_code == 403
