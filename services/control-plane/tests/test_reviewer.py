"""Reviewer Mode's backing endpoints — every one shells to a real check.
Skipped (not faked) when the underlying tool isn't available, same pattern
as test_integrity.py / test_parsers.py.
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


def test_parser_fixtures_runs_real_golden_fixtures(client, admin_headers):
    resp = client.get("/v1/reviewer/parser-fixtures", headers=admin_headers)
    assert resp.status_code == 200, resp.text
    body = resp.json()
    assert body["requirement"] == "b"
    assert body["passed"] is True, body["output"]
    assert "fixtures passed" in body["output"]


def test_airgap_check_runs_real_go_test(client, admin_headers):
    resp = client.get("/v1/reviewer/airgap-check", headers=admin_headers)
    assert resp.status_code == 200, resp.text
    body = resp.json()
    assert body["requirement"] == "j"
    assert body["passed"] is True, body["output"]
    assert "TestNoNetworkImports" in body["output"]


def test_containerized_reports_real_docker_state(client, admin_headers):
    """Whatever Docker's real state is (installed/running or not), the
    endpoint must report that real state — never hardcode passed=True."""
    resp = client.get("/v1/reviewer/containerized", headers=admin_headers)
    assert resp.status_code == 200, resp.text
    body = resp.json()
    assert body["requirement"] == "k"
    assert isinstance(body["passed"], bool)
    assert body["output"] != ""
