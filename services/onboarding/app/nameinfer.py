"""Field name inference from a Drain3 template. For key=value-shaped
wildcards ("srcip=<*>") the preceding token up to the separator IS the
field name and needs no guessing — that's exactly what the `kv` parse
operator already extracts directly, so onboarding only needs positional
name inference for templates the `kv` structural check rejects (space-
delimited positional text like "from <*> to <*>" -> from/to, or
undifferentiated CSV where a column-index name is the honest fallback).
"""

import re

_WILDCARD = "<*>"
_TRAILING_PUNCT = re.compile(r"[:=\[\(]+$")
_NOT_IDENT = re.compile(r"[^a-zA-Z0-9_]+")


def infer_field_names(template: str) -> list[str]:
    """Returns one field name per <*> in template, in order, using the
    nearest preceding literal token as the name (falling back to
    field_N when no usable literal precedes a wildcard — e.g. at the very
    start of the template).
    """
    parts = template.split(_WILDCARD)
    names: list[str] = []
    seen: dict[str, int] = {}

    for i in range(len(parts) - 1):
        preceding = parts[i].strip()
        tokens = preceding.split()
        raw_name = tokens[-1] if tokens else ""
        raw_name = _TRAILING_PUNCT.sub("", raw_name)
        raw_name = _NOT_IDENT.sub("_", raw_name).strip("_").lower()

        if not raw_name:
            raw_name = f"field_{i + 1}"

        if raw_name in seen:
            seen[raw_name] += 1
            raw_name = f"{raw_name}_{seen[raw_name]}"
        else:
            seen[raw_name] = 1

        names.append(raw_name)

    return names


def looks_like_kv(sample_lines: list[str], min_pairs: int = 2) -> bool:
    """Structural sniff: does this look like space-separated key=value
    pairs? Used to pick the draft parser's primary operator.
    """
    kv_pattern = re.compile(r"\b[a-zA-Z_][a-zA-Z0-9_]*=\S+")
    hits = 0
    for line in sample_lines[:20]:
        if len(kv_pattern.findall(line)) >= min_pairs:
            hits += 1
    return hits >= max(1, len(sample_lines[:20]) // 2)


def looks_like_csv(
    sample_lines: list[str], delimiter: str = ",", min_columns: int = 3
) -> bool:
    """Structural sniff: consistent comma (or other delimiter) column count
    across the sample."""
    counts = {line.count(delimiter) for line in sample_lines[:20] if line.strip()}
    return len(counts) == 1 and next(iter(counts), 0) + 1 >= min_columns


def looks_like_json(sample_lines: list[str]) -> bool:
    stripped = [line.strip() for line in sample_lines[:20] if line.strip()]
    return bool(stripped) and all(
        s.startswith("{") and s.endswith("}") for s in stripped
    )
