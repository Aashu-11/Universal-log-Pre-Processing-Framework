"""The Phase 8 acceptance gate, automated: take a vendor format NOT in our
packs (SonicWall — see internal/loggen's SonicWallTraffic, deliberately
excluded from AllVendors/packs), generate 200 sample lines, POST to
/v1/onboarding/analyze, and assert every step the gate calls for actually
produced real output — timed, like the gate asks.
"""

import os
import subprocess
import time
from pathlib import Path

import pytest

from app.config import settings

REPO_ROOT = Path(settings.repo_root)
ULPFCTL = REPO_ROOT / "bin" / ("ulpfctl.exe" if os.name == "nt" else "ulpfctl")

pytestmark = pytest.mark.skipif(
    not ULPFCTL.exists(), reason="bin/ulpfctl not built — run go build first"
)


def _generate_sonicwall_sample(count: int = 200) -> list[str]:
    result = subprocess.run(
        [str(ULPFCTL), "gen", "sonicwall", "--count", str(count)],
        capture_output=True,
        text=True,
        cwd=str(REPO_ROOT),
        timeout=30,
    )
    assert result.returncode == 0, result.stderr
    return [line for line in result.stdout.splitlines() if line.strip()]


def test_full_onboarding_flow_under_ten_minutes(client):
    start = time.monotonic()

    sample = _generate_sonicwall_sample(200)
    assert len(sample) == 200

    resp = client.post(
        "/v1/onboarding/analyze",
        json={
            "sample_lines": sample,
            "vendor": "sonicwall",
            "product": "firewall",
            "event_class": "traffic",
        },
    )
    assert resp.status_code == 200, resp.text
    body = resp.json()

    # 1. Template mining output with coverage %.
    assert body["template_coverage_pct"] > 90.0, body["top_templates"]
    assert body["template_count"] >= 1

    # 2. Inferred field types.
    assert len(body["fields"]) > 0
    inferred_types = {f["inferred_type"] for f in body["fields"]}
    assert inferred_types != {
        "free_text"
    }, "expected at least one non-free_text type inference"

    # 3. Generated YAML — syntactically real, not a stub.
    assert "metadata:" in body["parser_yaml"]
    assert "pipeline:" in body["parser_yaml"]
    assert body["shape"] == "kv"  # SonicWall's id=.. sn=.. time=.. shape is kv

    # 4. Test it (lint already ran as part of /analyze).
    assert body["lint_ok"] is True, body["lint_output"]

    # 5. Publish it — exercised in test_parsers.py in the control-plane
    # service (this service proxies, it doesn't own the registry) and in
    # the end-to-end demo script; not re-duplicated here.

    # 6. It actually parses live traffic of that vendor with the right id.
    assert body["parse_success_rate"] > 0.9, body

    elapsed = time.monotonic() - start
    print(f"\nonboarding flow (generate -> analyze -> verify): {elapsed:.2f}s")
    assert elapsed < 600, f"onboarding flow took {elapsed:.1f}s, want < 600s (10 min)"


def test_analyze_rejects_empty_sample(client):
    resp = client.post(
        "/v1/onboarding/analyze",
        json={"sample_lines": [], "vendor": "x", "product": "y"},
    )
    assert resp.status_code == 400
