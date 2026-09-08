"""Subprocess wrapper around bin/ulpfctl.exe — same rationale as the control
plane's identical module: one real validator/executor shared across
languages instead of two independently-drifting implementations. See
docs/DECISIONS.md D-008.
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


def _binary_path(repo_root: str) -> str:
    env_path = os.environ.get("ULPFCTL_PATH")
    if env_path:
        return env_path
    for name in ("ulpfctl.exe", "ulpfctl"):
        candidate = Path(repo_root) / "bin" / name
        if candidate.exists():
            return str(candidate)
    found = shutil.which("ulpfctl")
    if found:
        return found
    raise UlpfctlError(
        "ulpfctl binary not found", "set ULPFCTL_PATH or build bin/ulpfctl(.exe)"
    )


def lint(parser_yaml_path: str, repo_root: str) -> tuple[bool, str]:
    binary = _binary_path(repo_root)
    result = subprocess.run(
        [binary, "parser", "lint", parser_yaml_path],
        cwd=repo_root,
        capture_output=True,
        text=True,
        timeout=30,
    )
    return result.returncode == 0, (result.stdout or "") + (result.stderr or "")


def run_parser(
    parser_yaml_path: str,
    mapping_yaml_path: str | None,
    sample_lines: list[str],
    repo_root: str,
) -> list[dict]:
    """Runs the candidate parser (and optional mapping) against every
    sample line via `ulpfctl parser run`, returning the parsed JSON-lines
    output as a list of dicts — one per input line, in order.
    """
    binary = _binary_path(repo_root)
    args = [binary, "parser", "run", "--parser", parser_yaml_path]
    if mapping_yaml_path:
        args += ["--mapping", mapping_yaml_path]

    result = subprocess.run(
        args,
        cwd=repo_root,
        input="\n".join(sample_lines) + "\n",
        capture_output=True,
        text=True,
        timeout=60,
    )
    if result.returncode != 0:
        raise UlpfctlError("parser run failed", result.stderr or result.stdout)

    out = []
    for line in result.stdout.splitlines():
        if not line.strip():
            continue
        out.append(json.loads(line))
    return out


def run_shapes(
    parser_yaml_path: str, mapping_yaml_path: str, sample_line: str, repo_root: str
) -> dict:
    """Runs one line through `ulpfctl parser run --shapes`, returning the
    single JSON result with its `shapes` field — the same UES event
    rendered as UES/ECS/OCSF/CEF (PS requirement (g)), computed for real by
    internal/normalize/shape, never faked in the console.
    """
    binary = _binary_path(repo_root)
    args = [
        binary,
        "parser",
        "run",
        "--parser",
        parser_yaml_path,
        "--mapping",
        mapping_yaml_path,
        "--shapes",
    ]
    result = subprocess.run(
        args,
        cwd=repo_root,
        input=sample_line + "\n",
        capture_output=True,
        text=True,
        timeout=30,
    )
    if result.returncode != 0:
        raise UlpfctlError("parser run --shapes failed", result.stderr or result.stdout)
    lines = [line for line in result.stdout.splitlines() if line.strip()]
    if not lines:
        raise UlpfctlError("parser run --shapes produced no output", result.stdout)
    return json.loads(lines[0])
