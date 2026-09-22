"""Thin subprocess wrapper around the `ulpfctl` Go binary. Rather than
reimplementing DSL validation, unsafe-regex linting, and golden-fixture
testing a second time in Python, the control plane's publish pipeline
shells out to the exact same compiled engine the data plane runs — one
implementation of "is this parser valid," not two that can silently drift
apart. This is a deliberate architecture choice for a polyglot prototype;
a larger deployment might expose this as a gRPC call instead of a
subprocess, but the invariant (one validator, shared by both languages)
is what actually matters.
"""

import json
import os
import shutil
import subprocess
from pathlib import Path


class UlpfctlError(RuntimeError):
    def __init__(self, message: str, output: str):
        super().__init__(message)
        self.output = output


def _binary_path() -> str:
    env_path = os.environ.get("ULPFCTL_PATH")
    if env_path:
        return env_path
    candidates = [
        Path(__file__).resolve().parents[3] / "bin" / "ulpfctl.exe",
        Path(__file__).resolve().parents[3] / "bin" / "ulpfctl",
    ]
    for c in candidates:
        if c.exists():
            return str(c)
    found = shutil.which("ulpfctl")
    if found:
        return found
    raise UlpfctlError(
        "ulpfctl binary not found", "set ULPFCTL_PATH or build bin/ulpfctl(.exe)"
    )


def _vault_env(settings) -> dict[str, str]:
    """Translates the control plane's ULPF_MINIO_* Settings into the plain
    MINIO_* names cmd/ulpfctl's vault subcommands actually read (see
    cmd/ulpfctl/vault_cmd.go's openVaultStore). subprocess.run inherits the
    parent's environment by default, and the two naming schemes don't line
    up on their own — a control plane started with only ULPF_MINIO_* set
    (correct for its own Settings) would otherwise silently make every
    `ulpfctl vault ...` subprocess call fall back to a local ./data/vault
    directory instead of the real MinIO-backed store, failing with a
    misleading "key not found" rather than a config error.

    Only returns MINIO_* overrides, and only when minio_endpoint is
    actually configured — the local-dir fallback (ULPF_VAULT_LOCAL_DIR)
    already has matching names on both the Python and Go sides, so it
    passes through correctly via ordinary environment inheritance without
    needing an override here. Deliberately not set unconditionally: tests
    use monkeypatch.setenv("ULPF_VAULT_LOCAL_DIR", ...) at test-run time,
    which Settings (loaded once at import time) never observes — hardcoding
    settings.vault_local_dir here would silently override that back to the
    stale import-time default.
    """
    if not settings.minio_endpoint:
        return {}
    return {
        "MINIO_ENDPOINT": settings.minio_endpoint,
        "MINIO_ACCESS_KEY": settings.minio_access_key,
        "MINIO_SECRET_KEY": settings.minio_secret_key,
        "MINIO_RAW_BUCKET": settings.minio_raw_bucket,
        "MINIO_USE_SSL": "true" if settings.minio_use_ssl else "false",
    }


def _run(
    args: list[str], cwd: str | None = None, extra_env: dict[str, str] | None = None
) -> tuple[bool, str]:
    binary = _binary_path()
    env = None
    if extra_env:
        env = {**os.environ, **extra_env}
    try:
        result = subprocess.run(
            [binary, *args],
            cwd=cwd,
            env=env,
            capture_output=True,
            text=True,
            timeout=30,
        )
    except FileNotFoundError as e:
        raise UlpfctlError(f"ulpfctl not found at {binary}", "") from e
    except subprocess.TimeoutExpired as e:
        raise UlpfctlError("ulpfctl timed out", str(e)) from e
    output = (result.stdout or "") + (result.stderr or "")
    return result.returncode == 0, output


def lint(parser_yaml_path: str, repo_root: str) -> tuple[bool, str]:
    """Runs `ulpfctl parser lint <file>` — DSL validation + unsafe-regex
    rejection."""
    return _run(["parser", "lint", parser_yaml_path], cwd=repo_root)


def vault_read(
    segment_id: str, offset: int, length: int, sha256: str, settings
) -> dict:
    """Reads one event's exact original bytes back from the vault, SHA-256
    verified by the Go vault package itself — the control plane never
    re-implements that check. Returns {sha256_verified, length, raw_base64}
    or raises UlpfctlError with the failure reason (e.g. a mismatch).
    """
    ok, output = _run(
        [
            "vault",
            "read",
            "--json",
            "--segment-id",
            segment_id,
            "--offset",
            str(offset),
            "--length",
            str(length),
            "--sha256",
            sha256,
        ],
        cwd=settings.repo_root,
        extra_env=_vault_env(settings),
    )
    if not ok:
        raise UlpfctlError("vault read failed", output)
    try:
        return json.loads(output)
    except json.JSONDecodeError as e:
        raise UlpfctlError("vault read returned non-JSON output", output) from e


def vault_prove(
    segment_id: str, offset: int, length: int, sha256: str, settings
) -> dict:
    """Returns the Merkle inclusion proof for one event."""
    ok, output = _run(
        [
            "vault",
            "prove",
            "--segment-id",
            segment_id,
            "--offset",
            str(offset),
            "--length",
            str(length),
            "--sha256",
            sha256,
        ],
        cwd=settings.repo_root,
        extra_env=_vault_env(settings),
    )
    if not ok:
        raise UlpfctlError("vault prove failed", output)
    # Output is "proof valid: <bool>\n<json>" — the JSON is the second line.
    lines = output.strip().splitlines()
    json_line = lines[-1] if lines else "{}"
    try:
        proof = json.loads(json_line)
        verdict = lines[0].strip().lower() if lines else ""
        proof["verified"] = True if verdict == "proof valid: true" else False if verdict == "proof valid: false" else None
        return proof
    except json.JSONDecodeError as e:
        raise UlpfctlError("vault prove returned non-JSON output", output) from e


def vault_verify(from_date: str, to_date: str, settings) -> tuple[bool, str]:
    return _run(
        ["vault", "verify", "--from", from_date, "--to", to_date],
        cwd=settings.repo_root,
        extra_env=_vault_env(settings),
    )


def test_fixtures(parser_id: str, repo_root: str) -> tuple[bool, str]:
    """Runs golden fixtures for parser_id, if any exist under
    testdata/golden. A parser with no fixtures yet (freshly onboarded, not
    committed to testdata/) is not a lint failure — Phase 8's onboarding
    flow publishes before fixtures necessarily exist.
    """
    ok, output = _run(["parser", "test", parser_id], cwd=repo_root)
    if not ok and "no fixtures matched" in output:
        return True, "no golden fixtures found for this parser id yet (not a failure)"
    return ok, output
