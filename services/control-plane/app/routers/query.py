"""Read-only SQL proxy to Presto. The allowlist check runs BEFORE any
connection attempt — that's the security-relevant part and it's fully
testable without a live Presto server; only the actual query execution
needs one reachable.
"""

import re

import prestodb
from prestodb.exceptions import PrestoQueryError
from fastapi import APIRouter, Depends, HTTPException, status
from pydantic import BaseModel

from app.config import settings
from app.security import CurrentUser, require_role

router = APIRouter(prefix="/v1/query", tags=["query"])

# Only these three statement types may reach Presto. Checked against the
# first non-whitespace, non-comment token — "DROP TABLE" or "DELETE FROM"
# never gets far enough to open a connection, let alone execute.
_ALLOWED_PREFIXES = ("select", "explain", "show")
_LEADING_COMMENT = re.compile(r"^\s*(--[^\n]*\n|/\*.*?\*/\s*)*", re.DOTALL)
# Presto's client protocol sends the statement as a single request body, not
# a multi-statement script — a trailing ';' (the normal end-of-statement
# habit from any CLI) is not part of the grammar and Presto rejects it with
# a plain syntax error. Every hand-typed and saved query is written with one,
# so stripping it here (rather than asking every query author to remember
# not to) is what actually makes "paste a query, hit Run" work.
_TRAILING_SEMICOLON = re.compile(r"\s*;\s*$")


class QueryRequest(BaseModel):
    sql: str
    catalog: str = "lake"
    schema_: str = "logkrama"


class QueryResponse(BaseModel):
    columns: list[str]
    rows: list[list]
    row_count: int


def is_statement_allowed(sql: str) -> bool:
    stripped = _LEADING_COMMENT.sub("", sql).strip().lower()
    return any(stripped.startswith(p) for p in _ALLOWED_PREFIXES)


def normalize_sql(sql: str) -> str:
    return _TRAILING_SEMICOLON.sub("", sql.strip())


@router.post("", response_model=QueryResponse)
def run_query(
    body: QueryRequest,
    _: CurrentUser = Depends(require_role("admin", "engineer", "analyst", "auditor")),
):
    sql = normalize_sql(body.sql)

    if not is_statement_allowed(sql):
        raise HTTPException(
            status.HTTP_403_FORBIDDEN,
            "only SELECT, EXPLAIN and SHOW statements are permitted through this endpoint",
        )

    conn = prestodb.dbapi.connect(
        host=settings.presto_host,
        port=settings.presto_port,
        user=settings.presto_user,
        catalog=body.catalog,
        schema=body.schema_,
    )
    try:
        cur = conn.cursor()
        cur.execute(sql)
        rows = cur.fetchall()
        columns = [c[0] for c in (cur.description or [])]
    except PrestoQueryError as e:
        # Presto was reached and it parsed/ran the request — this is a bad
        # or unsupported query (syntax error, unknown table/schema, type
        # mismatch), not an infrastructure problem. 400, not 502, so the
        # console doesn't tell the user Presto is down when it responded
        # correctly by rejecting a bad statement.
        raise HTTPException(status.HTTP_400_BAD_REQUEST, f"query error: {e}") from e
    except Exception as e:  # transport/connection failure — Presto genuinely unreachable
        raise HTTPException(
            status.HTTP_502_BAD_GATEWAY, f"presto unreachable: {e}"
        ) from e
    finally:
        conn.close()

    return QueryResponse(
        columns=columns, rows=[list(r) for r in rows], row_count=len(rows)
    )
