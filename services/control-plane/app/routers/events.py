"""Raw bytes + traceability for one event.

Resolving event_id -> RawRef normally means a Presto query against
vault.ulpf.raw_index (written by internal/sink/vaultindex — see
docs/QUERIES.md Q4). That lookup isn't wired here yet since it depends on
Presto being reachable; both endpoints accept the ref explicitly via query
params in the meantime (exactly what a Presto-backed lookup would hand this
endpoint next) — the console's traceability viewer is the caller, and it
gets the ref from its own query result, so this is a real, usable API
today, not a stub.
"""

from fastapi import APIRouter, Depends, HTTPException, status
from pydantic import BaseModel

from app.config import settings
from app.security import CurrentUser, require_role
from app.ulpfctl import UlpfctlError, vault_prove, vault_read

router = APIRouter(prefix="/v1/events", tags=["events"])


class RawResponse(BaseModel):
    event_id: str
    sha256_verified: bool
    length: int
    raw_base64: str


class TraceResponse(BaseModel):
    event_id: str
    raw: RawResponse
    merkle_proof: dict


@router.get("/{event_id}/raw", response_model=RawResponse)
def get_raw(
    event_id: str,
    segment_id: str,
    offset: int,
    length: int,
    sha256: str,
    _: CurrentUser = Depends(require_role("admin", "engineer", "analyst", "auditor")),
):
    try:
        result = vault_read(segment_id, offset, length, sha256, settings)
    except UlpfctlError as e:
        raise HTTPException(
            status.HTTP_502_BAD_GATEWAY,
            f"raw bytes retrieval/verification failed: {e.output}",
        ) from e
    return RawResponse(event_id=event_id, **result)


@router.get("/{event_id}/trace", response_model=TraceResponse)
def get_trace(
    event_id: str,
    segment_id: str,
    offset: int,
    length: int,
    sha256: str,
    _: CurrentUser = Depends(require_role("admin", "engineer", "analyst", "auditor")),
):
    """Raw bytes + Merkle inclusion proof — the two pieces of evidence the
    console's Traceability page's 'Verify now' button needs, in one call.
    Field-offset maps (byte-range per UES field within the raw text) are
    computed by the parse-engine's dissect/regex operators at parse time in
    Phase 3/4, not re-derived here — a Phase 9 follow-up wires that
    metadata through the pipeline into this response.
    """
    try:
        raw_result = vault_read(segment_id, offset, length, sha256, settings)
        proof = vault_prove(segment_id, offset, length, sha256, settings)
    except UlpfctlError as e:
        raise HTTPException(
            status.HTTP_502_BAD_GATEWAY, f"trace retrieval failed: {e.output}"
        ) from e

    return TraceResponse(
        event_id=event_id,
        raw=RawResponse(event_id=event_id, **raw_result),
        merkle_proof=proof,
    )
