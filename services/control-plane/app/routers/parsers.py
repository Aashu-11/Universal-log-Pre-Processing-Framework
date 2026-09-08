"""Parser registry: publish (validate -> lint -> fixtures -> sign -> store),
versions, rollback, and a standalone test endpoint for the console's Parser
Workbench (Phase 9).

Publishing writes the artifact into ulpf_meta (parser_registry +
parser_artifacts) AND into the real packs/ directory on disk, so the
already-built, already-tested fsnotify hot-reload path
(internal/parse.Registry.WatchDir, Phase 3) is what actually propagates the
change to a co-located processor — see docs/DECISIONS.md for why this is
simpler than a second, Kafka-fetch-based distribution mechanism for a
single-node prototype. A notification is still published to
ulpf.control.parsers for audit/observability and as the extension point a
multi-node deployment would build a Kafka-fetch consumer against.
"""

import hashlib
import hmac
import tempfile
from datetime import datetime
from pathlib import Path

import yaml
from fastapi import APIRouter, Depends, HTTPException, status
from pydantic import BaseModel
from sqlalchemy import desc
from sqlalchemy.orm import Session

from app import audit
from app.config import settings
from app.db import get_db
from app.models import ParserArtifact, ParserRegistry, ProcessorNode
from app.schemas import ORMModel
from app.security import CurrentUser, require_role
from app.ulpfctl import UlpfctlError, lint, test_fixtures

router = APIRouter(prefix="/v1/parsers", tags=["parsers"])


class PublishRequest(BaseModel):
    parser_yaml: str
    mapping_yaml: str
    changelog: str | None = None


class PublishResponse(BaseModel):
    parser_id: str
    version: str
    state: str
    lint_output: str
    fixture_output: str
    sha256: str


class ParserVersionOut(ORMModel):
    parser_id: str
    version: str
    pack_id: str
    sha256: str
    state: str
    published_at: datetime
    published_by: str
    changelog: str | None


class RolloutStatus(BaseModel):
    node_id: str
    parser_id: str
    loaded_version: str
    reported_at: datetime


@router.post("", response_model=PublishResponse, status_code=status.HTTP_201_CREATED)
def publish_parser(
    body: PublishRequest,
    db: Session = Depends(get_db),
    user: CurrentUser = Depends(require_role("admin", "engineer")),
):
    try:
        parser_doc = yaml.safe_load(body.parser_yaml)
    except yaml.YAMLError as e:
        raise HTTPException(
            status.HTTP_400_BAD_REQUEST, f"parser_yaml is not valid YAML: {e}"
        ) from e
    metadata = (parser_doc or {}).get("metadata", {})
    parser_id = metadata.get("id")
    version = metadata.get("version")
    if not parser_id or not version:
        raise HTTPException(
            status.HTTP_400_BAD_REQUEST,
            "parser_yaml.metadata.id and .version are required",
        )

    with tempfile.TemporaryDirectory() as tmp:
        tmp_path = Path(tmp) / "candidate.yaml"
        tmp_path.write_text(body.parser_yaml, encoding="utf-8")

        try:
            lint_ok, lint_output = lint(str(tmp_path), settings.repo_root)
        except UlpfctlError as e:
            raise HTTPException(
                status.HTTP_502_BAD_GATEWAY, f"ulpfctl unavailable: {e}"
            ) from e
        if not lint_ok:
            raise HTTPException(
                status.HTTP_422_UNPROCESSABLE_ENTITY, f"lint failed:\n{lint_output}"
            )

        # Stage into the real packs/ directory so `ulpfctl parser test` (which
        # loads the whole packs/ tree) sees this candidate version, then run
        # fixtures against it before committing to anything durable.
        dest_dir = Path(settings.packs_dir) / parser_id.replace(".", "/")
        dest_dir.mkdir(parents=True, exist_ok=True)
        parser_dest = dest_dir / f"{dest_dir.name}.yaml"
        mapping_dest = dest_dir / f"{dest_dir.name}.mapping.yaml"
        parser_dest.write_text(body.parser_yaml, encoding="utf-8")
        mapping_dest.write_text(body.mapping_yaml, encoding="utf-8")

        try:
            fixtures_ok, fixture_output = test_fixtures(parser_id, settings.repo_root)
        except UlpfctlError as e:
            raise HTTPException(
                status.HTTP_502_BAD_GATEWAY, f"ulpfctl unavailable: {e}"
            ) from e
        if not fixtures_ok:
            raise HTTPException(
                status.HTTP_422_UNPROCESSABLE_ENTITY,
                f"golden fixtures failed:\n{fixture_output}",
            )

    combined = (body.parser_yaml + "\n---\n" + body.mapping_yaml).encode("utf-8")
    sha256 = hashlib.sha256(combined).hexdigest()
    signature = hmac.new(
        settings.jwt_secret.encode(), combined, hashlib.sha256
    ).hexdigest()

    entry = ParserRegistry(
        parser_id=parser_id,
        version=version,
        pack_id=parser_id,
        sha256=sha256,
        signature=signature,
        state="published",
        published_by=user.username,
        changelog=body.changelog,
    )
    db.add(entry)
    db.flush()
    db.add(
        ParserArtifact(
            registry_id=entry.id,
            parser_id=parser_id,
            version=version,
            parser_yaml=body.parser_yaml,
            mapping_yaml=body.mapping_yaml,
        )
    )
    audit.log(
        db,
        user.username,
        "publish_parser",
        parser_id,
        before=None,
        after={"version": version, "sha256": sha256},
    )
    db.commit()

    return PublishResponse(
        parser_id=parser_id,
        version=version,
        state="published",
        lint_output=lint_output,
        fixture_output=fixture_output,
        sha256=sha256,
    )


@router.get("", response_model=list[ParserVersionOut])
def list_parsers(
    db: Session = Depends(get_db),
    _: CurrentUser = Depends(require_role("admin", "engineer", "analyst", "auditor")),
):
    # Latest version per parser_id.
    rows = (
        db.query(ParserRegistry)
        .order_by(ParserRegistry.parser_id, desc(ParserRegistry.published_at))
        .all()
    )
    seen: set[str] = set()
    out = []
    for r in rows:
        if r.parser_id in seen:
            continue
        seen.add(r.parser_id)
        out.append(r)
    return out


@router.get("/{parser_id}/versions", response_model=list[ParserVersionOut])
def list_versions(
    parser_id: str,
    db: Session = Depends(get_db),
    _: CurrentUser = Depends(require_role("admin", "engineer", "analyst", "auditor")),
):
    return (
        db.query(ParserRegistry)
        .filter(ParserRegistry.parser_id == parser_id)
        .order_by(desc(ParserRegistry.published_at))
        .all()
    )


@router.post("/{parser_id}/rollback", response_model=ParserVersionOut)
def rollback(
    parser_id: str,
    db: Session = Depends(get_db),
    user: CurrentUser = Depends(require_role("admin", "engineer")),
):
    versions = (
        db.query(ParserRegistry)
        .filter(
            ParserRegistry.parser_id == parser_id, ParserRegistry.state == "published"
        )
        .order_by(desc(ParserRegistry.published_at))
        .all()
    )
    if len(versions) < 2:
        raise HTTPException(
            status.HTTP_400_BAD_REQUEST, "no earlier published version to roll back to"
        )

    current, previous = versions[0], versions[1]
    current.state = "rolled_back"

    artifact = (
        db.query(ParserArtifact)
        .filter(ParserArtifact.registry_id == previous.id)
        .first()
    )
    if artifact is None:
        raise HTTPException(
            status.HTTP_500_INTERNAL_SERVER_ERROR, "previous artifact missing"
        )

    dest_dir = Path(settings.packs_dir) / parser_id.replace(".", "/")
    (dest_dir / f"{dest_dir.name}.yaml").write_text(
        artifact.parser_yaml, encoding="utf-8"
    )
    (dest_dir / f"{dest_dir.name}.mapping.yaml").write_text(
        artifact.mapping_yaml, encoding="utf-8"
    )

    audit.log(
        db,
        user.username,
        "rollback_parser",
        parser_id,
        before={"version": current.version},
        after={"version": previous.version},
    )
    db.commit()
    db.refresh(previous)
    return previous


@router.get("/{parser_id}/rollout", response_model=list[RolloutStatus])
def rollout_status(
    parser_id: str,
    db: Session = Depends(get_db),
    _: CurrentUser = Depends(require_role("admin", "engineer", "analyst", "auditor")),
):
    """Which processor nodes have loaded which version — populated by nodes
    reporting in via POST /v1/parsers/{id}/rollout/report (called by a
    processor after every successful hot reload)."""
    return db.query(ProcessorNode).filter(ProcessorNode.parser_id == parser_id).all()


class RolloutReport(BaseModel):
    node_id: str
    loaded_version: str


@router.post("/{parser_id}/rollout/report", status_code=status.HTTP_204_NO_CONTENT)
def report_rollout(parser_id: str, body: RolloutReport, db: Session = Depends(get_db)):
    node = db.get(ProcessorNode, (body.node_id, parser_id))
    if node is None:
        node = ProcessorNode(
            node_id=body.node_id,
            parser_id=parser_id,
            loaded_version=body.loaded_version,
        )
        db.add(node)
    else:
        node.loaded_version = body.loaded_version
        node.reported_at = datetime.utcnow()
    db.commit()
