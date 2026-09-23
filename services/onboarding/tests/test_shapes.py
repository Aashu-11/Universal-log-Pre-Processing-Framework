"""PS requirement (g): one real parsed+mapped event rendered as
UES/ECS/OCSF/CEF, computed by internal/normalize/shape via
`logkramactl parser run --shapes` — see app/routers/onboarding.py's /shapes.
"""

import os
from pathlib import Path

import pytest

from app.config import settings

REPO_ROOT = Path(settings.repo_root)
LOGKRAMACTL = REPO_ROOT / "bin" / ("logkramactl.exe" if os.name == "nt" else "logkramactl")

pytestmark = pytest.mark.skipif(
    not LOGKRAMACTL.exists(), reason="bin/logkramactl not built — run go build first"
)

PARSER_YAML = """
metadata:
  id: test.shapes.demo
  version: "1.0.0"
  vendor: test
  product: demo
  observer_type: firewall
  event_class: traffic
match:
  any:
    - contains: "srcip"
pipeline:
  - op: kv
"""

MAPPING_YAML = """
metadata:
  parser_id: test.shapes.demo
  version: "1.0.0"
fields:
  event.category: { const: network }
  event.action: { from: action }
  observer.vendor: { const: test }
  observer.product: { const: demo }
  observer.type: { const: firewall }
  src.ip: { from: srcip }
  dst.ip: { from: dstip }
  dst.port: { from: dstport, transform: int }
unmapped_policy: retain
"""


def test_shapes_endpoint_returns_all_four_shapes(client):
    resp = client.post(
        "/v1/onboarding/shapes",
        json={
            "parser_yaml": PARSER_YAML,
            "mapping_yaml": MAPPING_YAML,
            "sample_line": "srcip=1.2.3.4 dstip=5.6.7.8 dstport=443 action=allow",
        },
    )
    assert resp.status_code == 200, resp.text
    body = resp.json()

    assert body["status"] == "ok"
    shapes = body["shapes"]

    assert shapes["ues"]["src"]["ip"] == "1.2.3.4"
    assert shapes["ecs"]["destination"]["ip"] == "5.6.7.8"
    assert shapes["ocsf"]["class_uid"] == 4001
    assert shapes["ocsf"]["src_endpoint"]["ip"] == "1.2.3.4"
    assert shapes["cef"].startswith("CEF:0|test|demo|")
    assert "src=1.2.3.4" in shapes["cef"]
