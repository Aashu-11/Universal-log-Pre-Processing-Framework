"""Regression tests for a real bug: subprocess calls to `logkramactl vault ...`
silently used a local ./data/vault directory instead of the real
MinIO-backed store whenever the control plane was started with only its own
LOGKRAMA_MINIO_* settings set, because subprocess.run's default environment
inheritance doesn't translate those into the plain MINIO_* names
cmd/logkramactl actually reads. Caught by hand while testing /v1/events/{id}/raw
against a live Docker MinIO — it failed with a misleading "key not found"
for a segment that genuinely existed.
"""

from types import SimpleNamespace

from app.logkramactl import _vault_env


def _settings(**overrides):
    base = dict(
        minio_endpoint="",
        minio_access_key="",
        minio_secret_key="",
        minio_raw_bucket="logkrama-raw",
        minio_use_ssl=False,
        vault_local_dir="./data/vault",
    )
    base.update(overrides)
    return SimpleNamespace(**base)


def test_vault_env_translates_minio_settings_when_configured():
    settings = _settings(
        minio_endpoint="127.0.0.1:9000",
        minio_access_key="logkramaadmin",
        minio_secret_key="logkrama_dev_only",
        minio_raw_bucket="logkrama-raw",
        minio_use_ssl=False,
    )
    env = _vault_env(settings)
    assert env == {
        "MINIO_ENDPOINT": "127.0.0.1:9000",
        "MINIO_ACCESS_KEY": "logkramaadmin",
        "MINIO_SECRET_KEY": "logkrama_dev_only",
        "MINIO_RAW_BUCKET": "logkrama-raw",
        "MINIO_USE_SSL": "false",
    }


def test_vault_env_use_ssl_true_translates_to_string_true():
    settings = _settings(minio_endpoint="minio.internal:9000", minio_use_ssl=True)
    assert _vault_env(settings)["MINIO_USE_SSL"] == "true"


def test_vault_env_returns_empty_when_minio_not_configured():
    """No MINIO_* override at all when minio_endpoint is unset — the local-dir
    fallback (LOGKRAMA_VAULT_LOCAL_DIR) already has matching names on both the
    Python and Go sides, so it must pass through via ordinary environment
    inheritance untouched. Overriding it here would break tests (and real
    local-dev runs) that set LOGKRAMA_VAULT_LOCAL_DIR at process-run time via
    monkeypatch/env, which Settings — loaded once at import time — never
    observes.
    """
    settings = _settings(minio_endpoint="")
    assert _vault_env(settings) == {}
