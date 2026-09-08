"""Integration tests that shell out to the real bin/ulpfctl.exe binary —
the same one the API routes invoke — against a throwaway vault directory.
Skipped automatically if the binary hasn't been built yet
(`go build -o bin/ulpfctl.exe ./cmd/ulpfctl` from the repo root).
"""

import os
import subprocess
from pathlib import Path

import pytest

from app.config import settings

REPO_ROOT = Path(settings.repo_root)
ULPFCTL = REPO_ROOT / "bin" / ("ulpfctl.exe" if os.name == "nt" else "ulpfctl")

pytestmark = pytest.mark.skipif(
    not ULPFCTL.exists(), reason="bin/ulpfctl not built — run go build first"
)


@pytest.fixture()
def vault_dir(tmp_path, monkeypatch):
    d = tmp_path / "vault"
    d.mkdir()
    monkeypatch.setenv("ULPF_VAULT_LOCAL_DIR", str(d))
    return d


def _write_events(vault_dir, lines: list[str]):
    env = dict(os.environ, ULPF_VAULT_LOCAL_DIR=str(vault_dir))
    result = subprocess.run(
        [str(ULPFCTL), "vault", "write"],
        input="\n".join(lines) + "\n",
        capture_output=True,
        text=True,
        env=env,
        cwd=str(REPO_ROOT),
    )
    assert result.returncode == 0, result.stderr
    return result.stdout


def test_verify_endpoint_passes_on_clean_vault(client, admin_headers, vault_dir):
    _write_events(vault_dir, ["event one", "event two", "event three"])

    resp = client.get("/v1/integrity/verify", headers=admin_headers)
    assert resp.status_code == 200, resp.text
    body = resp.json()
    assert body["passed"] is True
    assert "PASS" in body["output"]


def test_verify_endpoint_fails_on_corrupted_segment(client, admin_headers, vault_dir):
    _write_events(vault_dir, ["event one", "event two"])

    # Corrupt the sealed segment file directly, same technique as the Go
    # test — flip one byte in the middle of the .zst object.
    segment_files = list(vault_dir.rglob("*.zst"))
    assert segment_files, "expected at least one sealed segment"
    path = segment_files[0]
    data = bytearray(path.read_bytes())
    data[len(data) // 2] ^= 0xFF
    path.write_bytes(bytes(data))

    resp = client.get("/v1/integrity/verify", headers=admin_headers)
    assert resp.status_code == 200, resp.text
    body = resp.json()
    assert body["passed"] is False
    assert "FAIL" in body["output"]
