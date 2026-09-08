"""Integration tests for the publish pipeline — shells to the real
bin/ulpfctl.exe for lint + fixture testing, same as test_integrity.py.
"""

import os
from pathlib import Path

import pytest

from app.config import settings

REPO_ROOT = Path(settings.repo_root)
ULPFCTL = REPO_ROOT / "bin" / ("ulpfctl.exe" if os.name == "nt" else "ulpfctl")

pytestmark = pytest.mark.skipif(
    not ULPFCTL.exists(), reason="bin/ulpfctl not built — run go build first"
)


@pytest.fixture(autouse=True)
def _cleanup_test_pack():
    """Publishing stages the candidate into the REAL packs/ directory (see
    app/routers/parsers.py's docstring for why) — every parser id used by
    these tests lives under packs/test/, so removing that one directory
    after each test keeps the actual repo's packs/ tree exactly as
    committed, never polluted by test artifacts.
    """
    yield
    test_pack_dir = REPO_ROOT / "packs" / "test"
    if test_pack_dir.exists():
        import shutil

        shutil.rmtree(test_pack_dir)


VALID_PARSER_YAML = """
metadata:
  id: test.pytest.demo
  version: "1.0.0"
  vendor: test
  product: pytest
  observer_type: firewall
  event_class: demo
match:
  priority: 50
  confidence: 0.8
  any:
    - contains: "PYTESTDEMO"
pipeline:
  - op: dissect
    pattern: "PYTESTDEMO src=%{src} dst=%{dst}"
on_failure: emit_partial
"""

VALID_MAPPING_YAML = """
metadata:
  parser_id: test.pytest.demo
  version: "1.0.0"
fields:
  event.category: { const: network }
  src.ip: { from: src }
  dst.ip: { from: dst }
unmapped_policy: retain
"""

INVALID_PARSER_YAML = """
metadata:
  id: test.pytest.badregex
  version: "1.0.0"
match:
  any:
    - contains: "X"
pipeline:
  - op: regex
    pattern: "(a+)+$"
"""


def test_publish_valid_parser_succeeds(client, admin_headers):
    resp = client.post(
        "/v1/parsers",
        json={
            "parser_yaml": VALID_PARSER_YAML,
            "mapping_yaml": VALID_MAPPING_YAML,
            "changelog": "pytest demo",
        },
        headers=admin_headers,
    )
    assert resp.status_code == 201, resp.text
    body = resp.json()
    assert body["parser_id"] == "test.pytest.demo"
    assert body["version"] == "1.0.0"
    assert body["sha256"]

    versions = client.get(
        "/v1/parsers/test.pytest.demo/versions", headers=admin_headers
    ).json()
    assert len(versions) == 1
    assert versions[0]["version"] == "1.0.0"


def test_publish_rejects_unsafe_regex(client, admin_headers):
    resp = client.post(
        "/v1/parsers",
        json={"parser_yaml": INVALID_PARSER_YAML, "mapping_yaml": VALID_MAPPING_YAML},
        headers=admin_headers,
    )
    assert resp.status_code == 422
    assert "nested quantifier" in resp.text or "lint failed" in resp.text


def test_publish_then_rollback(client, admin_headers):
    v1 = VALID_PARSER_YAML
    v2 = VALID_PARSER_YAML.replace('version: "1.0.0"', 'version: "1.1.0"')

    r1 = client.post(
        "/v1/parsers",
        json={"parser_yaml": v1, "mapping_yaml": VALID_MAPPING_YAML},
        headers=admin_headers,
    )
    assert r1.status_code == 201, r1.text
    r2 = client.post(
        "/v1/parsers",
        json={"parser_yaml": v2, "mapping_yaml": VALID_MAPPING_YAML},
        headers=admin_headers,
    )
    assert r2.status_code == 201, r2.text

    rollback = client.post(
        "/v1/parsers/test.pytest.demo/rollback", headers=admin_headers
    )
    assert rollback.status_code == 200, rollback.text
    assert rollback.json()["version"] == "1.0.0"

    versions = client.get(
        "/v1/parsers/test.pytest.demo/versions", headers=admin_headers
    ).json()
    states = {v["version"]: v["state"] for v in versions}
    assert states["1.1.0"] == "rolled_back"


def test_publish_requires_engineer_or_admin(client, db_session):
    from app.models import User
    from app.security import hash_password

    db_session.add(
        User(username="analyst1", hashed_password=hash_password("pw"), role="analyst")
    )
    db_session.commit()
    token = client.post(
        "/v1/auth/login", json={"username": "analyst1", "password": "pw"}
    ).json()["access_token"]

    resp = client.post(
        "/v1/parsers",
        json={"parser_yaml": VALID_PARSER_YAML, "mapping_yaml": VALID_MAPPING_YAML},
        headers={"Authorization": f"Bearer {token}"},
    )
    assert resp.status_code == 403


def test_rollout_report_and_status(client, admin_headers):
    client.post(
        "/v1/parsers",
        json={"parser_yaml": VALID_PARSER_YAML, "mapping_yaml": VALID_MAPPING_YAML},
        headers=admin_headers,
    )
    report = client.post(
        "/v1/parsers/test.pytest.demo/rollout/report",
        json={"node_id": "processor-1", "loaded_version": "1.0.0"},
    )
    assert report.status_code == 204

    status_resp = client.get(
        "/v1/parsers/test.pytest.demo/rollout", headers=admin_headers
    )
    assert status_resp.status_code == 200
    rows = status_resp.json()
    assert any(
        r["node_id"] == "processor-1" and r["loaded_version"] == "1.0.0" for r in rows
    )
