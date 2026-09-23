"""Draft Parser + Mapping YAML generation. Picks the primary extraction
operator from a structural sniff of the sample (json > kv > csv > dissect
fallback via the Drain3 template), infers field types/names, and writes a
best-effort but syntactically real UES mapping for fields whose inferred
type maps cleanly onto a UES path (ip/port/verdict/severity) — everything
else is left for `unmapped_policy: retain` to catch, same as every
hand-written pack.
"""

import json
from dataclasses import dataclass, field

import yaml

from app.nameinfer import (
    infer_field_names,
    looks_like_csv,
    looks_like_json,
    looks_like_kv,
)
from app.typeinfer import infer_type

# type -> (ues_path_template, dictionary) — %s substitutes src/dst based on
# a name-hint match; dictionary is None when no value-normalization applies.
_UES_HINTS: dict[str, tuple[str, str | None]] = {
    "ipv4": ("%s.ip", None),
    "ipv6": ("%s.ip", None),
    "port": ("%s.port", None),
    "verdict": ("event.action", "action"),
    "severity": ("event.severity_id", "severity"),
}


@dataclass
class FieldPlan:
    name: str
    inferred_type: str
    confidence: float
    ues_path: str | None = None
    dictionary: str | None = None


@dataclass
class DraftResult:
    shape: str
    parser_yaml: str
    mapping_yaml: str
    fields: list[FieldPlan] = field(default_factory=list)


def _side(name: str) -> str:
    n = name.lower()
    if "src" in n or "source" in n:
        return "src"
    if "dst" in n or "dest" in n:
        return "dst"
    return "src"  # unresolvable side defaults to src rather than silently dropping the mapping


def _plan_field(name: str, values: list[str], repo_root: str) -> FieldPlan:
    inf = infer_type(values, name, repo_root)
    fp = FieldPlan(name=name, inferred_type=inf.type_name, confidence=inf.confidence)
    if inf.type_name in _UES_HINTS:
        template, dictionary = _UES_HINTS[inf.type_name]
        fp.ues_path = template % _side(name) if "%s" in template else template
        fp.dictionary = dictionary
    return fp


def _build_mapping_fields(plans: list[FieldPlan], vendor: str, product: str) -> dict:
    # observer.vendor/product/type are known at onboarding time regardless
    # of whether the raw log has a literal field for them — most vendors
    # never emit a field literally named "vendor" or "product". Without
    # these as const literals here, every auto-onboarded source silently
    # writes observer.vendor="" (found live: a real onboarded SonicWall
    # parser produced working, "ok"-status events that were then
    # unfindable by vendor because this was missing — not a query bug,
    # a draft-generation gap affecting every future onboarded source).
    out = {
        "observer.vendor": {"const": vendor},
        "observer.product": {"const": product},
        "observer.type": {"const": "firewall"},
        "event.category": {"const": "network"},
        "event.type": {"const": "connection"},
    }
    for p in plans:
        if not p.ues_path:
            continue
        entry: dict = {"from": p.name}
        if p.dictionary:
            entry["dictionary"] = p.dictionary
        if p.inferred_type in ("port",):
            entry["transform"] = "int"
        out[p.ues_path] = entry
    return out


def generate_draft(
    sample_lines: list[str],
    parser_id: str,
    vendor: str,
    product: str,
    event_class: str,
    repo_root: str,
    match_literal: str,
) -> DraftResult:
    if looks_like_json(sample_lines):
        return _generate_json_draft(
            sample_lines,
            parser_id,
            vendor,
            product,
            event_class,
            repo_root,
            match_literal,
        )
    if looks_like_kv(sample_lines):
        return _generate_kv_draft(
            sample_lines,
            parser_id,
            vendor,
            product,
            event_class,
            repo_root,
            match_literal,
        )
    if looks_like_csv(sample_lines):
        return _generate_csv_draft(
            sample_lines,
            parser_id,
            vendor,
            product,
            event_class,
            repo_root,
            match_literal,
        )
    return _generate_dissect_draft(
        sample_lines, parser_id, vendor, product, event_class, repo_root, match_literal
    )


def _base_metadata(parser_id: str, vendor: str, product: str, event_class: str) -> dict:
    return {
        "id": parser_id,
        "version": "1.0.0",
        "vendor": vendor,
        "product": product,
        "observer_type": "firewall",
        "event_class": event_class,
    }


def _base_mapping_metadata(parser_id: str) -> dict:
    return {"parser_id": parser_id, "version": "1.0.0"}


def _dump(doc: dict) -> str:
    return yaml.safe_dump(doc, sort_keys=False, default_flow_style=False)


def _generate_json_draft(
    lines, parser_id, vendor, product, event_class, repo_root, match_literal
) -> DraftResult:
    samples: dict[str, list[str]] = {}
    for line in lines:
        try:
            doc = json.loads(line)
        except json.JSONDecodeError:
            continue
        for k, v in doc.items():
            samples.setdefault(k, []).append(str(v))

    plans = [_plan_field(k, v, repo_root) for k, v in samples.items()]
    parser_doc = {
        "metadata": _base_metadata(parser_id, vendor, product, event_class),
        "match": (
            {"priority": 50, "confidence": 0.6, "any": [{"contains": match_literal}]}
            if match_literal
            else {"priority": 50, "confidence": 0.5, "any": [{"prefix": "{"}]}
        ),
        "pipeline": [{"op": "json"}],
        "on_failure": "emit_partial",
    }
    mapping_doc = {
        "metadata": _base_mapping_metadata(parser_id),
        "fields": _build_mapping_fields(plans, vendor, product),
        "unmapped_policy": "retain",
    }
    return DraftResult("json", _dump(parser_doc), _dump(mapping_doc), plans)


def _generate_kv_draft(
    lines, parser_id, vendor, product, event_class, repo_root, match_literal
) -> DraftResult:
    import re

    kv_pattern = re.compile(r"\b([a-zA-Z_][a-zA-Z0-9_]*)=(\"[^\"]*\"|\S+)")
    samples: dict[str, list[str]] = {}
    for line in lines:
        for k, v in kv_pattern.findall(line):
            samples.setdefault(k, []).append(v.strip('"'))

    plans = [_plan_field(k, v, repo_root) for k, v in samples.items()]
    parser_doc = {
        "metadata": _base_metadata(parser_id, vendor, product, event_class),
        "match": (
            {"priority": 50, "confidence": 0.7, "any": [{"contains": match_literal}]}
            if match_literal
            else {"priority": 50, "confidence": 0.5, "any": [{"regex": r"\w+=\S+"}]}
        ),
        "pipeline": [{"op": "kv"}],
        "on_failure": "emit_partial",
    }
    mapping_doc = {
        "metadata": _base_mapping_metadata(parser_id),
        "fields": _build_mapping_fields(plans, vendor, product),
        "unmapped_policy": "retain",
    }
    return DraftResult("kv", _dump(parser_doc), _dump(mapping_doc), plans)


def _generate_csv_draft(
    lines, parser_id, vendor, product, event_class, repo_root, match_literal
) -> DraftResult:
    rows = [line.split(",") for line in lines if line.strip()]
    n_cols = len(rows[0]) if rows else 0
    columns = [f"col_{i + 1}" for i in range(n_cols)]
    samples: dict[str, list[str]] = {c: [] for c in columns}
    for row in rows:
        for i, val in enumerate(row[:n_cols]):
            samples[columns[i]].append(val)

    plans = [_plan_field(c, samples[c], repo_root) for c in columns]
    parser_doc = {
        "metadata": _base_metadata(parser_id, vendor, product, event_class),
        "match": (
            {"priority": 40, "confidence": 0.5, "any": [{"contains": match_literal}]}
            if match_literal
            else {
                "priority": 40,
                "confidence": 0.4,
                "any": [{"regex": r"^[^,]+(,[^,]+){%d,}$" % (n_cols - 1)}],
            }
        ),
        "pipeline": [{"op": "csv", "delimiter": ",", "columns": columns}],
        "on_failure": "emit_partial",
    }
    mapping_doc = {
        "metadata": _base_mapping_metadata(parser_id),
        "fields": _build_mapping_fields(plans, vendor, product),
        "unmapped_policy": "retain",
    }
    return DraftResult("csv", _dump(parser_doc), _dump(mapping_doc), plans)


def _generate_dissect_draft(
    lines, parser_id, vendor, product, event_class, repo_root, match_literal
) -> DraftResult:
    from app.templates import mine_templates

    mined = mine_templates(lines)
    if not mined.clusters:
        raise ValueError("no templates could be mined from the sample")
    top = mined.clusters[0]

    names = infer_field_names(top.template)
    values_by_name: dict[str, list[str]] = {n: [] for n in names}
    for line in top.example_lines + lines[: min(len(lines), 50)]:
        extracted = _extract_by_template(top.template, line)
        for name, val in zip(names, extracted):
            if val is not None:
                values_by_name[name].append(val)

    plans = [_plan_field(n, values_by_name.get(n, []), repo_root) for n in names]
    dissect_pattern = top.template.replace("<*>", "PLACEHOLDER")
    for n in names:
        dissect_pattern = dissect_pattern.replace("PLACEHOLDER", "%{" + n + "}", 1)

    parser_doc = {
        "metadata": _base_metadata(parser_id, vendor, product, event_class),
        "match": (
            {"priority": 30, "confidence": 0.5, "any": [{"contains": match_literal}]}
            if match_literal
            else {
                "priority": 30,
                "confidence": 0.4,
                "any": [{"contains": _literal_anchor(top.template)}],
            }
        ),
        "pipeline": [{"op": "dissect", "pattern": dissect_pattern}],
        "on_failure": "emit_partial",
    }
    mapping_doc = {
        "metadata": _base_mapping_metadata(parser_id),
        "fields": _build_mapping_fields(plans, vendor, product),
        "unmapped_policy": "retain",
    }
    return DraftResult("dissect", _dump(parser_doc), _dump(mapping_doc), plans)


def _literal_anchor(template: str) -> str:
    parts = [p.strip() for p in template.split("<*>") if p.strip()]
    return max(parts, key=len) if parts else template[:20]


def _extract_by_template(template: str, line: str) -> list[str | None]:
    """Best-effort re-extraction of wildcard values from one line using the
    mined template's literal segments as anchors — the same linear-walk
    idea as the dissect operator itself, used here only to gather sample
    values for type inference (the real extraction at runtime is the
    generated dissect operator, exercised via `logkramactl parser run`).
    """
    segments = template.split("<*>")
    values: list[str | None] = []
    pos = 0
    for i, seg in enumerate(segments):
        if seg:
            idx = line.find(seg, pos)
            if idx == -1:
                values.append(None)
                continue
            pos = idx + len(seg)
        if i < len(segments) - 1:
            next_seg = segments[i + 1]
            end = line.find(next_seg, pos) if next_seg else len(line)
            if end == -1:
                end = len(line)
            values.append(line[pos:end].strip())
            pos = end
    return values
