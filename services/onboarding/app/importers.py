"""Converters from two common external formats into our Parser DSL:
Logstash grok filters, and a CEF field-name mapping. Both produce
syntactically valid Parser + Mapping YAML the same generate_draft-adjacent
way as onboarding's own draft generator, so they go through the exact same
lint/run verification path afterward.
"""

import re

import yaml

# Grok pattern names our internal library (internal/parse/ops/regex.go)
# actually implements. A Logstash filter using anything outside this set
# still imports — the unrecognized pattern is preserved as GREEDYDATA
# (best-effort) rather than failing the whole import, and reported to the
# caller so they know which fields need a manual regex.
_SUPPORTED_GROK_PATTERNS = {
    "IP",
    "IPORHOST",
    "INT",
    "NUMBER",
    "WORD",
    "NOTSPACE",
    "GREEDYDATA",
    "DATA",
    "HOSTNAME",
    "TIMESTAMP",
}

_GROK_REF = re.compile(r"%\{([A-Za-z0-9_]+)(?::([A-Za-z0-9_]+))?\}")


def import_logstash_grok(
    grok_filter: str, parser_id: str, vendor: str, product: str
) -> tuple[str, str, list[str]]:
    """Returns (parser_yaml, mapping_yaml, unsupported_patterns)."""
    unsupported: list[str] = []
    fields: list[str] = []

    def replace(m: re.Match) -> str:
        pattern_name, field_name = m.group(1), m.group(2)
        mapped = (
            pattern_name if pattern_name in _SUPPORTED_GROK_PATTERNS else "GREEDYDATA"
        )
        if pattern_name not in _SUPPORTED_GROK_PATTERNS:
            unsupported.append(pattern_name)
        if field_name:
            fields.append(field_name)
            return "%{" + mapped + ":" + field_name + "}"
        return "%{" + mapped + "}"

    converted_pattern = _GROK_REF.sub(replace, grok_filter)

    parser_doc = {
        "metadata": {
            "id": parser_id,
            "version": "1.0.0",
            "vendor": vendor,
            "product": product,
            "observer_type": "firewall",
            "event_class": "imported",
        },
        "match": {"priority": 20, "confidence": 0.3, "any": [{"regex": ".+"}]},
        "pipeline": [{"op": "grok", "pattern": converted_pattern}],
        "on_failure": "emit_partial",
    }
    mapping_doc = {
        "metadata": {"parser_id": parser_id, "version": "1.0.0"},
        "fields": {"event.category": {"const": "network"}},
        "unmapped_policy": "retain",
    }
    return _dump(parser_doc), _dump(mapping_doc), sorted(set(unsupported))


def import_cef_mapping(
    field_map: dict[str, str], parser_id: str, vendor: str, product: str
) -> tuple[str, str]:
    """field_map is {cef_extension_key: ues_path}, e.g. {"src": "src.ip",
    "dpt": "dst.port"} — CEF itself is already natively parsed by our `cef`
    operator, so importing "a CEF mapping" only ever means generating the
    Mapping half; the Parser half is the same one-operator pipeline for
    every CEF source.
    """
    parser_doc = {
        "metadata": {
            "id": parser_id,
            "version": "1.0.0",
            "vendor": vendor,
            "product": product,
            "observer_type": "firewall",
            "event_class": "imported",
        },
        "match": {"priority": 60, "confidence": 0.8, "all": [{"prefix": "CEF:"}]},
        "pipeline": [{"op": "cef"}],
        "on_failure": "emit_partial",
    }
    fields = {"event.category": {"const": "network"}}
    for cef_key, ues_path in field_map.items():
        fields[ues_path] = {"from": cef_key}
    mapping_doc = {
        "metadata": {"parser_id": parser_id, "version": "1.0.0"},
        "fields": fields,
        "unmapped_policy": "retain",
    }
    return _dump(parser_doc), _dump(mapping_doc)


def _dump(doc: dict) -> str:
    return yaml.safe_dump(doc, sort_keys=False, default_flow_style=False)
