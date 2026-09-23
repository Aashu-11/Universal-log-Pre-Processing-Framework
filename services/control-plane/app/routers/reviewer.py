"""Backing endpoints for the console's Reviewer Mode page — the graded
"Prove it" buttons for PS requirements (a)-(k). Every one of these shells
out to a real check (the compiled Go binary, `go test`, or `docker compose
ps`) and returns exactly what came back, including failures. Nothing here
is allowed to hardcode a PASS — see CLAUDE.md.
"""

import os
import shutil
import subprocess
from pathlib import Path

from fastapi import APIRouter
from pydantic import BaseModel

from app.config import settings
from app.logkramactl import LogKramactlError, _binary_path

router = APIRouter(prefix="/v1/reviewer", tags=["reviewer"])


class ProofResult(BaseModel):
    requirement: str
    passed: bool
    summary: str
    output: str


def _run(
    args: list[str], cwd: str, timeout: int = 60, env: dict[str, str] | None = None
) -> tuple[bool, str]:
    try:
        result = subprocess.run(
            args, cwd=cwd, capture_output=True, text=True, timeout=timeout, env=env
        )
    except FileNotFoundError as e:
        return False, f"command not found: {e}"
    except subprocess.TimeoutExpired:
        return False, f"timed out after {timeout}s"
    return result.returncode == 0, (result.stdout or "") + (result.stderr or "")


@router.get("/parser-fixtures", response_model=ProofResult)
def parser_fixtures() -> ProofResult:
    """(b) Parse source attributes — runs every shipped parser's golden
    fixtures for real via `logkramactl parser test --all`."""
    try:
        binary = _binary_path()
    except LogKramactlError as e:
        return ProofResult(
            requirement="b", passed=False, summary="logkramactl not built", output=str(e)
        )
    ok, output = _run([binary, "parser", "test", "--all"], settings.repo_root)
    summary = output.strip().splitlines()[-1] if output.strip() else "no output"
    return ProofResult(requirement="b", passed=ok, summary=summary, output=output)


@router.get("/airgap-check", response_model=ProofResult)
def airgap_check() -> ProofResult:
    """(j) Air-gapped — TestNoNetworkImports statically proves
    internal/enrich imports no network-capable package. This is the
    source-level half of the air-gap proof (see docs/BUILD_PLAN.md); the
    Docker-network-blocked half needs Docker, checked separately."""
    go_binary = shutil.which("go") or _default_go_path()
    if not go_binary:
        return ProofResult(
            requirement="j", passed=False, summary="go toolchain not found", output=""
        )
    go_env = dict(os.environ)
    go_env.setdefault("GOCACHE", str(Path(settings.repo_root) / ".cache" / "go-build"))
    ok, output = _run(
        [
            go_binary,
            "test",
            "./internal/enrich/...",
            "-run",
            "TestNoNetworkImports",
            "-v",
        ],
        settings.repo_root,
        env=go_env,
    )
    summary = "TestNoNetworkImports PASS" if ok else "TestNoNetworkImports FAILED"
    return ProofResult(requirement="j", passed=ok, summary=summary, output=output)


def _default_go_path() -> str | None:
    candidate = Path("C:/Program Files/Go/bin/go.exe")
    return str(candidate) if candidate.exists() else None


@router.get("/containerized", response_model=ProofResult)
def containerized() -> ProofResult:
    """(k) Containerized — `docker compose ps`. Genuinely fails (not faked)
    when Docker isn't reachable in this environment; see docs/DECISIONS.md."""
    docker = shutil.which("docker")
    if not docker:
        return ProofResult(
            requirement="k",
            passed=False,
            summary="docker not found on PATH",
            output="Docker Desktop is not installed/running in this environment. "
            "docker-compose.yml defines 8 services with healthchecks — see the file directly.",
        )
    ok, output = _run([docker, "compose", "ps"], settings.repo_root, timeout=15)
    return ProofResult(
        requirement="k",
        passed=ok,
        summary="docker compose ps" if ok else "docker unreachable",
        output=output,
    )


@router.get("/onboarding-timing", response_model=ProofResult)
def onboarding_timing() -> ProofResult:
    """(i) Reduced parser effort — re-runs the actual Phase 8 pytest gate
    (`test_full_onboarding_flow_under_ten_minutes`) and reports its real
    wall-clock time, rather than quoting a number from memory."""
    onboarding_dir = Path(settings.repo_root) / "services" / "onboarding"
    python = str(onboarding_dir / ".venv" / "Scripts" / "python.exe")
    if not Path(python).exists():
        python = "python"
    ok, output = _run(
        [
            python,
            "-m",
            "pytest",
            "-v",
            "-s",
            "tests/test_onboarding_flow.py::test_full_onboarding_flow_under_ten_minutes",
        ],
        str(onboarding_dir),
        timeout=120,
    )
    summary = "onboarding flow completed" if ok else "onboarding flow test failed"
    for line in output.splitlines():
        if "onboarding flow" in line and "s" in line:
            summary = line.strip()
    return ProofResult(requirement="i", passed=ok, summary=summary, output=output)
