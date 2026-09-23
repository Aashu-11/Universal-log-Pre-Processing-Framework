"""The Phase 8 onboarding pipeline: mine templates, infer field types/names,
draft a Parser + Mapping YAML, and immediately execute the draft against the
real sample via the compiled Go engine — every number this endpoint returns
(coverage %, per-field success rate) comes from actually running the
candidate parser, never a guess.
"""

import tempfile
from pathlib import Path

import httpx
from fastapi import APIRouter, HTTPException, status
from fastapi.responses import Response
from pydantic import BaseModel

from app.config import settings
from app.draftgen import generate_draft
from app.importers import import_cef_mapping, import_logstash_grok
from app.templates import mine_templates
from app.logkramactl import LogKramactlError, lint, run_parser

router = APIRouter(prefix="/v1/onboarding", tags=["onboarding"])


class AnalyzeRequest(BaseModel):
    sample_lines: list[str]
    vendor: str
    product: str
    event_class: str = "generic"
    parser_id: str | None = None
    sim_th: float = 0.4
    depth: int = 4


class FieldOut(BaseModel):
    name: str
    inferred_type: str
    confidence: float
    ues_path: str | None
    mapped: bool


class SampleComparison(BaseModel):
    raw: str
    status: str
    extracted_field_count: int
    mapped: dict | None = None
    unmapped: dict[str, str] | None = None


class AnalyzeResponse(BaseModel):
    parser_id: str
    shape: str
    template_coverage_pct: float
    template_count: int
    top_templates: list[dict]
    fields: list[FieldOut]
    parser_yaml: str
    mapping_yaml: str
    parse_success_rate: float
    per_field_success_rate: dict[str, float]
    sample_comparison: list[SampleComparison]
    lint_ok: bool
    lint_output: str


def _parser_id_from(
    vendor: str, product: str, event_class: str, override: str | None
) -> str:
    return override or f"{vendor}.{product}.{event_class}"


def _dominant_literal(sample_lines: list[str]) -> str:
    """A short literal substring common to the sample, used as the draft's
    match.any anchor when the caller doesn't supply one — picked from the
    single most common non-trivial "word" across the sample.
    """
    from collections import Counter

    counter: Counter[str] = Counter()
    for line in sample_lines[:50]:
        for tok in line.split():
            if len(tok) >= 4:
                counter[tok] += 1
    if not counter:
        return ""
    return counter.most_common(1)[0][0]


@router.post("/analyze", response_model=AnalyzeResponse)
def analyze(body: AnalyzeRequest):
    if not body.sample_lines:
        raise HTTPException(
            status.HTTP_400_BAD_REQUEST, "sample_lines must not be empty"
        )

    parser_id = _parser_id_from(
        body.vendor, body.product, body.event_class, body.parser_id
    )
    mined = mine_templates(body.sample_lines, sim_th=body.sim_th, depth=body.depth)
    match_literal = _dominant_literal(body.sample_lines)

    draft = generate_draft(
        body.sample_lines,
        parser_id,
        body.vendor,
        body.product,
        body.event_class,
        settings.repo_root,
        match_literal,
    )

    with tempfile.TemporaryDirectory() as tmp:
        parser_path = Path(tmp) / "candidate.yaml"
        mapping_path = Path(tmp) / "candidate.mapping.yaml"
        parser_path.write_text(draft.parser_yaml, encoding="utf-8")
        mapping_path.write_text(draft.mapping_yaml, encoding="utf-8")

        try:
            lint_ok, lint_output = lint(str(parser_path), settings.repo_root)
        except LogKramactlError as e:
            raise HTTPException(
                status.HTTP_502_BAD_GATEWAY, f"logkramactl unavailable: {e}"
            ) from e

        results = []
        if lint_ok:
            try:
                results = run_parser(
                    str(parser_path),
                    str(mapping_path),
                    body.sample_lines,
                    settings.repo_root,
                )
            except LogKramactlError as e:
                raise HTTPException(
                    status.HTTP_502_BAD_GATEWAY, f"parser run failed: {e.output}"
                ) from e

    ok_count = sum(1 for r in results if r.get("status") == "ok")
    parse_success_rate = round(ok_count / len(results), 3) if results else 0.0

    field_success: dict[str, int] = {p.name: 0 for p in draft.fields}
    for r in results:
        for name in field_success:
            if r.get("fields", {}).get(name):
                field_success[name] += 1
    per_field_success_rate = {
        name: round(count / len(results), 3) if results else 0.0
        for name, count in field_success.items()
    }

    fields_out = [
        FieldOut(
            name=p.name,
            inferred_type=p.inferred_type,
            confidence=p.confidence,
            ues_path=p.ues_path,
            mapped=p.ues_path is not None,
        )
        for p in draft.fields
    ]

    sample_comparison = [
        SampleComparison(
            raw=r["raw"],
            status=r.get("status", ""),
            extracted_field_count=len(r.get("fields", {})),
            mapped=r.get("mapped"),
            unmapped=r.get("unmapped"),
        )
        for r in results[:5]
    ]

    top_templates = [
        {"template": c.template, "size": c.size, "coverage_pct": c.coverage_pct}
        for c in mined.clusters[:5]
    ]

    return AnalyzeResponse(
        parser_id=parser_id,
        shape=draft.shape,
        template_coverage_pct=mined.total_coverage_pct,
        template_count=len(mined.clusters),
        top_templates=top_templates,
        fields=fields_out,
        parser_yaml=draft.parser_yaml,
        mapping_yaml=draft.mapping_yaml,
        parse_success_rate=parse_success_rate,
        per_field_success_rate=per_field_success_rate,
        sample_comparison=sample_comparison,
        lint_ok=lint_ok,
        lint_output=lint_output,
    )


class TestRequest(BaseModel):
    parser_yaml: str
    mapping_yaml: str | None = None
    sample_lines: list[str]


class TestResponse(BaseModel):
    parse_success_rate: float
    results: list[dict]


@router.post("/test", response_model=TestResponse)
def test_draft(body: TestRequest):
    """Re-runs an edited draft (from the console's Parser Workbench) against
    the sample — the debounced live re-parse loop."""
    with tempfile.TemporaryDirectory() as tmp:
        parser_path = Path(tmp) / "candidate.yaml"
        parser_path.write_text(body.parser_yaml, encoding="utf-8")
        mapping_path = None
        if body.mapping_yaml:
            mapping_path = Path(tmp) / "candidate.mapping.yaml"
            mapping_path.write_text(body.mapping_yaml, encoding="utf-8")

        try:
            results = run_parser(
                str(parser_path),
                str(mapping_path) if mapping_path else None,
                body.sample_lines,
                settings.repo_root,
            )
        except LogKramactlError as e:
            raise HTTPException(
                status.HTTP_502_BAD_GATEWAY, f"parser run failed: {e.output}"
            ) from e

    ok_count = sum(1 for r in results if r.get("status") == "ok")
    rate = round(ok_count / len(results), 3) if results else 0.0
    return TestResponse(parse_success_rate=rate, results=results)


class PublishProxyRequest(BaseModel):
    parser_yaml: str
    mapping_yaml: str
    changelog: str | None = None


@router.post("/publish")
def publish_proxy(body: PublishProxyRequest, authorization: str | None = None):
    """Forwards to the control plane's real publish endpoint — onboarding
    doesn't own the registry, the control plane does; this just saves the
    console an extra round trip during the onboarding flow.
    """
    headers = {"Authorization": authorization} if authorization else {}
    try:
        resp = httpx.post(
            f"{settings.control_plane_url}/v1/parsers",
            json=body.model_dump(),
            headers=headers,
            timeout=30,
        )
    except httpx.HTTPError as e:
        raise HTTPException(
            status.HTTP_502_BAD_GATEWAY, f"control plane unreachable: {e}"
        ) from e
    return Response(
        content=resp.content,
        status_code=resp.status_code,
        media_type=resp.headers.get("content-type"),
    )


class ImportGrokRequest(BaseModel):
    grok_filter: str
    parser_id: str
    vendor: str
    product: str


class ImportGrokResponse(BaseModel):
    parser_yaml: str
    mapping_yaml: str
    unsupported_patterns: list[str]


@router.post("/import/grok", response_model=ImportGrokResponse)
def import_grok(body: ImportGrokRequest):
    parser_yaml, mapping_yaml, unsupported = import_logstash_grok(
        body.grok_filter, body.parser_id, body.vendor, body.product
    )
    return ImportGrokResponse(
        parser_yaml=parser_yaml,
        mapping_yaml=mapping_yaml,
        unsupported_patterns=unsupported,
    )


class ImportCEFRequest(BaseModel):
    field_map: dict[str, str]
    parser_id: str
    vendor: str
    product: str


class ImportCEFResponse(BaseModel):
    parser_yaml: str
    mapping_yaml: str


@router.post("/import/cef", response_model=ImportCEFResponse)
def import_cef(body: ImportCEFRequest):
    parser_yaml, mapping_yaml = import_cef_mapping(
        body.field_map, body.parser_id, body.vendor, body.product
    )
    return ImportCEFResponse(parser_yaml=parser_yaml, mapping_yaml=mapping_yaml)


class ShapesRequest(BaseModel):
    parser_yaml: str
    mapping_yaml: str
    sample_line: str


@router.post("/shapes")
def shapes(body: ShapesRequest):
    """PS requirement (g): renders one real parsed+mapped event as
    UES/ECS/OCSF/CEF side by side — see app/logkramactl.py's run_shapes, which
    shells to `logkramactl parser run --shapes`.
    """
    from app.logkramactl import run_shapes

    with tempfile.TemporaryDirectory() as tmp:
        parser_path = Path(tmp) / "candidate.yaml"
        mapping_path = Path(tmp) / "candidate.mapping.yaml"
        parser_path.write_text(body.parser_yaml, encoding="utf-8")
        mapping_path.write_text(body.mapping_yaml, encoding="utf-8")
        try:
            result = run_shapes(
                str(parser_path),
                str(mapping_path),
                body.sample_line,
                settings.repo_root,
            )
        except LogKramactlError as e:
            raise HTTPException(
                status.HTTP_502_BAD_GATEWAY, f"shapes run failed: {e.output}"
            ) from e
    return result
