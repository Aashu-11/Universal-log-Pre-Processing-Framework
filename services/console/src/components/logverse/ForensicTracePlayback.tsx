import { useEffect, useState } from "react";

import type { ForensicTrace, LogVerseEvent, Maybe, PipelineStage } from "../../lib/logverse/types";
import { explainDlqReason } from "../../lib/logverse/failure";

// One stage revealed every this many ms while stepping backward through the
// trace — a UI pacing constant (mirrors Live Theater's STAGE_STEP_MS), not
// a measured per-stage latency. Always labeled as such wherever it reaches
// the user.
const REVEAL_STEP_MS = 420;

function MaybeRow({ label, value, format }: { label: string; value: Maybe<unknown>; format?: (v: unknown) => string }) {
  return (
    <div className="flex items-center justify-between gap-3 py-1 text-[11px]">
      <span className="text-[var(--color-text-muted)]">{label}</span>
      {value.available ? (
        <span className="font-mono-data text-[var(--color-text)]">
          {format ? format(value.value) : String(value.value)}
        </span>
      ) : (
        <span className="font-mono-data text-[var(--color-text-faint,var(--color-text-muted))] italic">
          Unavailable — {value.reason}
        </span>
      )}
    </div>
  );
}

function Row({ label, value, tone }: { label: string; value: string; tone?: "danger" | "success" | "warning" }) {
  const toneClass =
    tone === "danger" ? "text-[var(--color-danger)]" : tone === "success" ? "text-[var(--color-success)]" : tone === "warning" ? "text-[var(--color-warning)]" : "text-[var(--color-text)]";
  return (
    <div className="flex items-center justify-between gap-3 py-1 text-[11px]">
      <span className="text-[var(--color-text-muted)]">{label}</span>
      <span className={`font-mono-data ${toneClass}`}>{value || "—"}</span>
    </div>
  );
}

function stageCard(stage: PipelineStage, event: LogVerseEvent, trace: ForensicTrace | null) {
  switch (stage) {
    case "route":
      return (
        <>
          <Row label="Destination" value={event.isDlq ? "Lake + live stream; additional DLQ copy" : "Lake + live stream"} tone={event.isDlq ? "warning" : undefined} />
          <Row label="Lake partition" value={event.dt ? `dt=${event.dt} hour=${event.hour}` : ""} />
          <Row label="Vendor partition" value={event.vendor} />
        </>
      );
    case "validate":
      return (
        <>
          <Row label="Quality score" value={event.isDlq ? "Unavailable in DLQ record" : event.qualityScore.toFixed(2)} />
          {event.isDlq && <Row label="DLQ reason" value={event.dlqReason ?? "unspecified"} tone="warning" />}
          {event.isDlq && explainDlqReason(event.dlqReason).map((finding) => <div key={finding.code} className="mt-2 rounded border border-[var(--color-warning)]/25 bg-[var(--color-warning)]/5 p-2 text-[10px] text-[var(--color-text-secondary)]">
            <div className="font-mono font-semibold text-[var(--color-warning)]">{finding.code} · {finding.stage.toUpperCase()}</div>
            <p className="mt-1">{finding.explanation}</p><p className="mt-1">Next check: {finding.nextCheck}</p>
          </div>)}
        </>
      );
    case "enrich":
      return (
        <>
          <Row label="Risk score" value={event.isDlq ? "Unavailable in DLQ record" : event.riskScore.toFixed(1)} tone={event.riskScore >= 40 ? "warning" : undefined} />
          <Row label="IOC match" value={event.isDlq ? "Unavailable in DLQ record" : event.iocMatch ? `yes — ${event.iocIndicator || "indicator not named"}` : "no"} tone={event.iocMatch ? "danger" : undefined} />
          <Row label="Threat category" value={event.threatCategory} tone={event.threatCategory ? "danger" : undefined} />
          <Row label="MITRE tactic / technique" value={[event.threatMitreTactic, event.threatMitreTechnique].filter(Boolean).join(" / ")} />
          <Row label="Src geo / AS org" value={[event.geoCountry, event.asOrg].filter(Boolean).join(" / ")} />
          <div className="mt-1 text-[10px] italic text-[var(--color-text-muted)]">
            Unavailable — a per-factor risk-score breakdown is computed internally (internal/enrich/risk.go) but not
            exposed by the lake schema; only the final score is queryable.
          </div>
        </>
      );
    case "normalize":
      return (
        <>
          <Row label="event.kind" value={event.eventKind} />
          <Row label="event.action" value={event.eventAction} />
          <Row label="observer.vendor" value={event.vendor} />
        </>
      );
    case "parse":
      return (
        <>
          <Row label="Parse status" value={event.isDlq ? "Unavailable in DLQ record" : event.parseStatus} tone={event.parseStatus === "ok" ? "success" : event.parseStatus === "failed" ? "danger" : "warning"} />
          <Row label="Took" value={event.isDlq ? "Unavailable in DLQ record" : `${event.processingMs.toFixed(3)}ms`} />
        </>
      );
    case "identify":
      return (
        <>
          <Row label="Parser selected" value={event.parserId || "(none matched)"} />
          <Row label="Parser version" value={event.parserVersion} />
          <div className="mt-1 text-[10px] italic text-[var(--color-text-muted)]">
            Unavailable — the specific match rule/confidence that chose this parser over others isn't returned by
            the trace API, only the final selection.
          </div>
        </>
      );
    case "preserve":
      return (
        <>
          <MaybeRow label="SHA-256 verified" value={trace?.sha256Verified ?? { available: false, reason: "trace not yet fetched" }} format={(v) => (v ? "PASS" : "FAIL")} />
          <Row label="Segment" value={event.rawSegmentId ? event.rawSegmentId.slice(-16) : ""} />
          <MaybeRow label="Raw length" value={trace?.rawLength ?? { available: false, reason: "trace not yet fetched" }} format={(v) => `${v} bytes`} />
          <MaybeRow label="Merkle root" value={trace?.merkleRoot ?? { available: false, reason: "trace not yet fetched" }} format={(v) => String(v).slice(0, 20) + "…"} />
          <MaybeRow label="Merkle leaf" value={trace?.merkleLeaf ?? { available: false, reason: "trace not yet fetched" }} format={(v) => String(v).slice(0, 20) + "…"} />
          <MaybeRow label="Inclusion proof steps" value={trace?.merkleProofSteps ?? { available: false, reason: "trace not yet fetched" }} />
          <MaybeRow label="Proof verified" value={trace?.merkleVerified ?? { available: false, reason: "trace not yet fetched" }} format={(v) => v ? "PASS" : "FAIL"} />
          <MaybeRow label="Leaf index" value={trace?.merkleIndex ?? { available: false, reason: "trace not yet fetched" }} />
          {trace?.merklePath.available && <details className="mt-2 text-[10px] text-[var(--color-text-secondary)]"><summary className="cursor-pointer">Show {trace.merklePath.value.length} Merkle sibling hashes</summary><ol className="mt-1 space-y-1">{trace.merklePath.value.map((step, index) => <li key={index} className="break-all font-mono">{index + 1}. {step.side}: {step.sibling}</li>)}</ol></details>}
          {trace?.rawBase64.available && <RawEvidencePreview base64={trace.rawBase64.value} />}
        </>
      );
    default:
      return null;
  }
}

const STAGE_LABEL: Record<PipelineStage, string> = {
  ingest: "INGEST",
  preserve: "PRESERVE",
  identify: "IDENTIFY",
  parse: "PARSE",
  normalize: "NORMALIZE",
  enrich: "ENRICH",
  validate: "VALIDATE",
  route: "ROUTE",
};

function RawEvidencePreview({ base64 }: { base64: string }) {
  let text = "unavailable";
  let hex = "unavailable";
  try {
    // A 1024-character base64 prefix is at most 768 raw bytes. Preview only;
    // the SHA check above applies to the complete bytes returned by the vault.
    const prefix = base64.slice(0, 1024);
    const decoded = atob(prefix.slice(0, Math.floor(prefix.length / 4) * 4));
    const bytes = Uint8Array.from(decoded, (char) => char.charCodeAt(0));
    text = new TextDecoder("utf-8", { fatal: false }).decode(bytes).slice(0, 512);
    hex = Array.from(bytes.slice(0, 64), (byte) => byte.toString(16).padStart(2, "0")).join(" ");
  } catch {
    // The proof still displays even if the preview cannot decode.
  }
  return <div className="mt-2 space-y-1 rounded border border-[var(--color-border)] bg-black/30 p-2 text-[10px]">
    <div className="font-semibold text-[var(--color-text-secondary)]">Original bytes · preview only</div>
    <pre className="max-h-24 overflow-auto whitespace-pre-wrap break-all text-[var(--color-text)]">{text}</pre>
    <div className="text-[var(--color-text-muted)]">First 64 bytes in hex</div>
    <code className="break-all text-[var(--color-cyan)]">{hex}</code>
  </div>;
}

export function ForensicTracePlayback({
  event,
  trace,
  traceLoading,
  traceError,
  playing,
  onStageHighlight,
}: {
  event: LogVerseEvent;
  trace: ForensicTrace | null;
  traceLoading: boolean;
  traceError: string | null;
  playing: boolean;
  onStageHighlight?: (stage: PipelineStage | null) => void;
}) {
  // The router always writes lake + stream, and additionally copies an
  // event to DLQ on a violation; DLQ is not a diversion before ROUTE.
  const order: PipelineStage[] = ["route", "validate", "enrich", "normalize", "parse", "identify", "preserve"];

  const [revealed, setRevealed] = useState(0);

  useEffect(() => {
    if (!playing) {
      setRevealed(0);
      onStageHighlight?.(null);
      return;
    }
    setRevealed(1);
    onStageHighlight?.(order[0] ?? null);
    let i = 1;
    const id = setInterval(() => {
      if (i >= order.length) {
        clearInterval(id);
        return;
      }
      onStageHighlight?.(order[i] ?? null);
      i += 1;
      setRevealed(i);
    }, REVEAL_STEP_MS);
    return () => {
      clearInterval(id);
      onStageHighlight?.(null);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [playing, event.eventId]);

  if (!playing) return null;

  return (
    <div className="space-y-2">
      <div className="text-[10.5px] uppercase tracking-wide text-[var(--color-text-muted)]">
        Tracing backward from ROUTE {event.isDlq ? "(including the DLQ copy)" : ""} to the preserved raw bytes —
        visualization pacing {REVEAL_STEP_MS}ms/stage, not a measured latency.
      </div>
      {traceLoading && <div className="text-[11px] text-[var(--color-text-muted)]">Fetching raw bytes + Merkle proof from the vault…</div>}
      {traceError && <div className="rounded border border-[var(--color-danger)]/30 bg-[var(--color-danger)]/10 px-2.5 py-1.5 text-[11px] text-[var(--color-danger)]">{traceError}</div>}
      {order.slice(0, revealed).map((stage) => (
        <div key={stage} className="rounded-md border border-[var(--color-border)] bg-white/[0.02] p-2.5">
          <div className="mb-1 font-mono-data text-[10px] font-semibold uppercase tracking-wide text-[var(--color-accent)]">
            {STAGE_LABEL[stage]}
          </div>
          {stageCard(stage, event, trace)}
        </div>
      ))}
    </div>
  );
}
