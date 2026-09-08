from datetime import date

from fastapi import APIRouter, Depends, HTTPException, status
from pydantic import BaseModel

from app.config import settings
from app.security import CurrentUser, require_role
from app.ulpfctl import UlpfctlError, vault_verify

router = APIRouter(prefix="/v1/integrity", tags=["integrity"])


class VerifyResponse(BaseModel):
    passed: bool
    output: str


@router.get("/verify", response_model=VerifyResponse)
def verify(
    from_: date | None = None,
    to: date | None = None,
    _: CurrentUser = Depends(require_role("admin", "engineer", "analyst", "auditor")),
):
    """Recomputes the Merkle chain for the given date range and reports
    PASS/FAIL — the exact same check `ulpfctl vault verify` runs, exposed
    over HTTP for the console's Reviewer Mode 'Prove it' button on
    requirement (a), lossless raw preservation.
    """
    today = date.today()
    f = (from_ or today).isoformat()
    t = (to or today).isoformat()
    try:
        ok, output = vault_verify(f, t, settings)
    except UlpfctlError as e:
        raise HTTPException(
            status.HTTP_502_BAD_GATEWAY, f"integrity verify unavailable: {e.output}"
        ) from e
    return VerifyResponse(passed=ok, output=output)
