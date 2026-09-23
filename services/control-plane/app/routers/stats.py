"""Pipeline/source statistics. Scrapes the collector's and processor's own
/metrics (Prometheus exposition format) directly rather than requiring
Prometheus itself to be up — useful during local dev, and Grafana panels in
Phase 9+ point at Prometheus directly for the full dashboard regardless.
"""

import re

import httpx
from fastapi import APIRouter, Depends
from pydantic import BaseModel
from sqlalchemy.orm import Session

from app.config import settings
from app.db import get_db
from app.models import Source
from app.schemas import ORMModel
from app.security import CurrentUser, require_role

router = APIRouter(prefix="/v1/stats", tags=["stats"])

_METRIC_LINE = re.compile(
    r"^([a-zA-Z_:][a-zA-Z0-9_:]*)(\{[^}]*\})?\s+([0-9eE.+\-]+)\s*$"
)


def scrape_metrics(url: str, timeout: float = 2.0) -> dict[str, float]:
    """Fetches a Prometheus text-exposition endpoint and returns
    {metric_name: last_seen_value} (summed across label combinations, which
    is what every counter this endpoint reports actually wants).
    """
    out: dict[str, float] = {}
    try:
        resp = httpx.get(url, timeout=timeout)
        resp.raise_for_status()
    except httpx.HTTPError:
        return out
    for line in resp.text.splitlines():
        if line.startswith("#") or not line.strip():
            continue
        m = _METRIC_LINE.match(line)
        if not m:
            continue
        name, _labels, value = m.groups()
        try:
            out[name] = out.get(name, 0.0) + float(value)
        except ValueError:
            continue
    return out


class PipelineStats(BaseModel):
    collector_reachable: bool
    processor_reachable: bool
    events_received_total: float
    udp_drops_total: float
    ingest_bytes_total: float
    dlq_total: float


@router.get("/pipeline", response_model=PipelineStats)
def pipeline_stats(
    collector_url: str = settings.collector_metrics_url,
    processor_url: str = settings.processor_metrics_url,
    _: CurrentUser = Depends(require_role("admin", "engineer", "analyst", "auditor")),
):
    collector_metrics = scrape_metrics(collector_url)
    processor_metrics = scrape_metrics(processor_url)
    return PipelineStats(
        collector_reachable=bool(collector_metrics),
        processor_reachable=bool(processor_metrics),
        events_received_total=collector_metrics.get("logkrama_events_received_total", 0.0),
        udp_drops_total=collector_metrics.get("logkrama_udp_drops_total", 0.0),
        ingest_bytes_total=collector_metrics.get("logkrama_ingest_bytes_total", 0.0),
        dlq_total=processor_metrics.get("logkrama_dlq_total", 0.0),
    )


class SourceStats(ORMModel):
    log_source_id: str
    name: str
    vendor: str
    product: str
    parser_id: str | None
    status: str
    last_seen: str


@router.get("/sources", response_model=list[SourceStats])
def source_stats(
    db: Session = Depends(get_db),
    _: CurrentUser = Depends(require_role("admin", "engineer", "analyst", "auditor")),
):
    """Quality score / coverage % per source is a Presto aggregate (Q6 in
    docs/QUERIES.md) once that's reachable; this returns the inventory half
    from logkrama_meta today.
    """
    rows = db.query(Source).order_by(Source.name).all()
    return [
        SourceStats(
            log_source_id=r.log_source_id,
            name=r.name,
            vendor=r.vendor,
            product=r.product,
            parser_id=r.parser_id,
            status=r.status,
            last_seen=r.last_seen.isoformat(),
        )
        for r in rows
    ]
