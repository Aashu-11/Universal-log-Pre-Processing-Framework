"""Field type inference for onboarding: given every observed value at one
wildcard position across the sample, classify it. Detectors run in a fixed
priority order (CLAUDE.md's list) and the first one whose pattern matches a
strong majority of observed values wins — ties toward the earlier, more
specific detector. Every result carries a confidence (fraction of sample
values that actually matched), never a bare guess.
"""

import ipaddress
import re
from dataclasses import dataclass
from pathlib import Path

import yaml

_INT_RE = re.compile(r"^-?\d+$")
_FLOAT_RE = re.compile(r"^-?\d+\.\d+$")
_MAC_RE = re.compile(r"^([0-9A-Fa-f]{2}:){5}[0-9A-Fa-f]{2}$")
_URL_RE = re.compile(r"^https?://\S+$")
_DOMAIN_RE = re.compile(
    r"^(?=.{1,253}$)([a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$"
)
_HASH_RE = {
    32: re.compile(r"^[0-9a-fA-F]{32}$"),
    40: re.compile(r"^[0-9a-fA-F]{40}$"),
    64: re.compile(r"^[0-9a-fA-F]{64}$"),
}
_TIMESTAMP_PATTERNS = [
    re.compile(r"^\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}"),  # ISO8601-ish
    re.compile(r"^\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}$"),  # PAN-OS style
    re.compile(r"^[A-Za-z]{3}\s+\d{1,2} \d{2}:\d{2}:\d{2}$"),  # RFC3164
    re.compile(r"^\d{10,13}$"),  # epoch seconds/millis
]


@dataclass
class TypeInference:
    type_name: str
    confidence: float


def _match_ratio(values: list[str], predicate) -> float:
    if not values:
        return 0.0
    hits = sum(1 for v in values if predicate(v))
    return hits / len(values)


def _is_ip(v: str) -> bool:
    try:
        ipaddress.ip_address(v)
        return True
    except ValueError:
        return False


def _load_dictionary_keys(repo_root: str, name: str) -> set[str]:
    path = Path(repo_root) / "config" / "dictionaries" / f"{name}.yaml"
    if not path.exists():
        return set()
    data = yaml.safe_load(path.read_text(encoding="utf-8")) or {}
    return {k.lower() for k in data if k != "_default"}


def infer_type(
    values: list[str], field_name_hint: str, repo_root: str
) -> TypeInference:
    values = [v for v in values if v != ""]
    if not values:
        return TypeInference("free_text", 0.0)

    hint = field_name_hint.lower()

    checks: list[tuple[str, float]] = [
        ("ipv4", _match_ratio(values, lambda v: _is_ip(v) and ":" not in v)),
        ("ipv6", _match_ratio(values, lambda v: _is_ip(v) and ":" in v)),
        ("mac", _match_ratio(values, lambda v: bool(_MAC_RE.match(v)))),
        (
            "timestamp",
            _match_ratio(
                values, lambda v: any(p.match(v) for p in _TIMESTAMP_PATTERNS)
            ),
        ),
        ("url", _match_ratio(values, lambda v: bool(_URL_RE.match(v)))),
        (
            "hash",
            _match_ratio(values, lambda v: any(r.match(v) for r in _HASH_RE.values())),
        ),
        (
            "domain",
            _match_ratio(values, lambda v: bool(_DOMAIN_RE.match(v)) and not _is_ip(v)),
        ),
    ]

    action_keys = _load_dictionary_keys(repo_root, "action")
    severity_keys = _load_dictionary_keys(repo_root, "severity")
    checks.append(("verdict", _match_ratio(values, lambda v: v.lower() in action_keys)))
    checks.append(
        ("severity", _match_ratio(values, lambda v: v.lower() in severity_keys))
    )

    is_port_hint = "port" in hint
    checks.append(
        (
            "port",
            (
                _match_ratio(
                    values,
                    lambda v: _INT_RE.match(v) is not None and 0 <= int(v) <= 65535,
                )
                if is_port_hint
                else 0.0
            ),
        )
    )

    is_byte_hint = any(k in hint for k in ("byte", "sent", "rcvd", "recv"))
    checks.append(
        (
            "byte_count",
            (
                _match_ratio(values, lambda v: _INT_RE.match(v) is not None)
                if is_byte_hint
                else 0.0
            ),
        )
    )

    is_user_hint = "user" in hint
    checks.append(
        (
            "username",
            (
                _match_ratio(values, lambda v: bool(re.match(r"^[\w.\-@]+$", v)))
                if is_user_hint
                else 0.0
            ),
        )
    )

    checks.append(("integer", _match_ratio(values, lambda v: bool(_INT_RE.match(v)))))
    checks.append(("float", _match_ratio(values, lambda v: bool(_FLOAT_RE.match(v)))))

    distinct = set(values)
    if len(distinct) <= min(10, max(2, len(values) // 3)):
        checks.append(("enum", 1.0 - (len(distinct) / len(values))))

    best_type, best_score = "free_text", 0.0
    for type_name, score in checks:
        if score > best_score or (
            score == best_score and score > 0 and best_type == "free_text"
        ):
            best_type, best_score = type_name, score

    if best_score < 0.6:
        return TypeInference(
            "free_text", round(1.0 - best_score, 2) if best_type == "free_text" else 0.5
        )
    return TypeInference(best_type, round(best_score, 2))
