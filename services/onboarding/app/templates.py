"""Template mining via Drain3 (MIT license) — the first step of onboarding
an unknown source: cluster a sample of raw lines into a small number of
`<*>`-wildcarded templates and report what fraction of the sample each one
covers. Real Drain3, not a stand-in: this is the exact library, tuned with
two knobs (sim_th, depth) exposed to the API caller per CLAUDE.md.
"""

from dataclasses import dataclass, field

from drain3 import TemplateMiner
from drain3.template_miner_config import TemplateMinerConfig


@dataclass
class TemplateCluster:
    template: str
    size: int
    coverage_pct: float
    example_lines: list[str] = field(default_factory=list)


@dataclass
class MiningResult:
    clusters: list[TemplateCluster]
    total_lines: int
    total_coverage_pct: float


def mine_templates(
    lines: list[str], sim_th: float = 0.4, depth: int = 4
) -> MiningResult:
    """Clusters lines into templates. sim_th (0..1) is Drain3's similarity
    threshold — higher means stricter clustering (more, narrower templates);
    depth is the parse-tree depth Drain3 uses to bucket by token count/prefix
    before similarity comparison. Defaults match Drain3's own documented
    starting point for unstructured log text.
    """
    config = TemplateMinerConfig()
    config.drain_sim_th = sim_th
    config.drain_depth = depth
    config.drain_max_children = 200
    config.profiling_enabled = False

    miner = TemplateMiner(config=config)

    examples: dict[int, list[str]] = {}
    for line in lines:
        result = miner.add_log_message(line)
        cluster_id = result["cluster_id"]
        examples.setdefault(cluster_id, [])
        if len(examples[cluster_id]) < 3:
            examples[cluster_id].append(line)

    total = len(lines)
    clusters: list[TemplateCluster] = []
    for cluster in miner.drain.clusters:
        pct = (cluster.size / total * 100.0) if total else 0.0
        clusters.append(
            TemplateCluster(
                template=cluster.get_template(),
                size=cluster.size,
                coverage_pct=round(pct, 2),
                example_lines=examples.get(cluster.cluster_id, []),
            )
        )
    clusters.sort(key=lambda c: c.size, reverse=True)

    covered = sum(c.size for c in clusters)
    total_coverage = round((covered / total * 100.0), 2) if total else 0.0

    return MiningResult(
        clusters=clusters, total_lines=total, total_coverage_pct=total_coverage
    )
