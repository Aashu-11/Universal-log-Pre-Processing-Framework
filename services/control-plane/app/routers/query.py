"""Read-only SQL proxy to Presto. The allowlist check runs BEFORE any
connection attempt — that's the security-relevant part and it's fully
testable without a live Presto server; only the actual query execution
needs one reachable.
"""

import re

import prestodb
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


class QueryRequest(BaseModel):
    sql: str
    catalog: str = "lake"
    schema_: str = "ulpf"


class QueryResponse(BaseModel):
    columns: list[str]
    rows: list[list]
    row_count: int


def is_statement_allowed(sql: str) -> bool:
    stripped = _LEADING_COMMENT.sub("", sql).strip().lower()
    return any(stripped.startswith(p) for p in _ALLOWED_PREFIXES)


@router.post("", response_model=QueryResponse)
def run_query(
    body: QueryRequest,
    _: CurrentUser = Depends(require_role("admin", "engineer", "analyst", "auditor")),
):
    if not is_statement_allowed(body.sql):
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
        cur.execute(body.sql)
        rows = cur.fetchall()
        columns = [c[0] for c in (cur.description or [])]
    except Exception as e:  # presto client raises various transport/query errors
        raise HTTPException(
            status.HTTP_502_BAD_GATEWAY, f"presto query failed: {e}"
        ) from e
    finally:
        conn.close()

    return QueryResponse(
        columns=columns, rows=[list(r) for r in rows], row_count=len(rows)
    )
