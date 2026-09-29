"""Offline-first investigation assistant.

Ollama is optional and is contacted only on an explicitly configured local
endpoint. The deterministic diagnosis is always returned, so air-gapped
deployments retain useful parser and triage assistance without a model.
"""

import re

import httpx
from fastapi import APIRouter, Depends
from pydantic import BaseModel, Field

from app.config import settings
from app.security import CurrentUser, require_role

router = APIRouter(prefix="/v1/assistant", tags=["assistant"])


class AssistRequest(BaseModel):
    log_line: str = Field(min_length=1, max_length=20_000)
    question: str = Field(default="Explain this event and suggest a parser.", max_length=2_000)


class AssistResponse(BaseModel):
    mode: str
    summary: str
    parser_hints: list[str]
    field_hints: list[str]
    next_steps: list[str]
    signals: dict[str, bool]


def _deterministic_assist(line: str) -> AssistResponse:
    hints: list[str] = []
    fields: list[str] = []
    lowered = line.lower()
    signals = {
        "syslog": False,
        "json": False,
        "kv": False,
        "action": False,
        "src_ip": False,
        "dst_ip": False,
        "threat": False,
    }
    if "<" in line and ">" in line and re.search(r"<\d+>", line):
        signals["syslog"] = True
        hints.append("syslog priority prefix detected; start with the syslog operator")
        fields.extend(["observer.timestamp", "observer.hostname"])
    if "{" in line and "}" in line:
        signals["json"] = True
        hints.append("JSON-shaped payload detected; use the json operator before field mapping")
    if re.search(r"\b\w+=[^\s]+", line):
        signals["kv"] = True
        hints.append("key=value fields detected; use the kv operator with quoted-value support")
    if re.search(r"\b(?:allow|deny|blocked|drop)\b", lowered):
        signals["action"] = True
        fields.append("event.action")
    if re.search(r"\b(?:src|source)[_=:-]", lowered):
        signals["src_ip"] = True
        fields.append("src_ip / src_port")
    if re.search(r"\b(?:dst|dest|destination)[_=:-]", lowered):
        signals["dst_ip"] = True
        fields.append("dst_ip / dst_port")
    if re.search(r"\b(?:alert|malware|exploit|attack)\b", lowered):
        signals["threat"] = True
        fields.extend(["threat.category", "threat.severity"])
    if not hints:
        hints.append("start with dissect for a stable delimiter layout, then promote variable fields to regex")
    return AssistResponse(
        mode="deterministic",
        summary="The line was analyzed locally; no event data was sent outside LogKrama.",
        parser_hints=hints,
        field_hints=list(dict.fromkeys(fields)) or ["event.original", "observer.vendor", "event.action"],
        next_steps=[
            "Paste a representative 100–200 line sample into Parser Workbench.",
            "Validate the generated pack against fixtures before publishing.",
            "Replay affected DLQ records after a corrected parser is published.",
        ],
        signals=signals,
    )


@router.post("/analyze", response_model=AssistResponse)
def analyze(
    body: AssistRequest,
    _: CurrentUser = Depends(require_role("admin", "engineer", "analyst")),
):
    fallback = _deterministic_assist(body.log_line)
    if not settings.ollama_url:
        return fallback
    try:
        response = httpx.post(
            f"{settings.ollama_url.rstrip('/')}/api/generate",
            json={
                "model": settings.ollama_model,
                "stream": False,
                "prompt": (
                    f"{body.question}\n\nLog line:\n{body.log_line}\n\n"
                    "Return a concise security triage and parser suggestion, formatted as short "
                    "markdown paragraphs under the exact headings '### Security Triage' and "
                    "'### Parser Suggestion' (each on its own line, blank line before and after). "
                    "Keep each section to 2-4 sentences of plain prose."
                ),
            },
            # A cold model load can legitimately take 60-100s (observed on a
            # hybrid-GPU laptop where CUDA context init retries several times
            # before succeeding) — 15s cut off every first request and fell
            # back to deterministic even with a healthy Ollama. Subsequent
            # calls within Ollama's keep_alive window (default 5m) are fast.
            timeout=120,
        )
        response.raise_for_status()
        text = response.json().get("response", "").strip()
        if text:
            fallback.mode = "ollama-local"
            fallback.summary = text
    except httpx.HTTPError:
        pass
    return fallback
