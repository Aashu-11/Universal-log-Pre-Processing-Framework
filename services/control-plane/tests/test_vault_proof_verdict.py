import json
from types import SimpleNamespace

from app import logkramactl


def test_vault_prove_exposes_cli_verification_verdict(monkeypatch):
    proof = {"Leaf": [1], "Root": [2], "Path": [], "Index": 0}
    monkeypatch.setattr(
        logkramactl,
        "_run",
        lambda *args, **kwargs: (True, "proof valid: true\n" + json.dumps(proof)),
    )
    settings = SimpleNamespace(minio_endpoint="", repo_root=".")
    result = logkramactl.vault_prove("segment", 0, 1, "hash", settings)
    assert result["verified"] is True
    assert result["Root"] == [2]
