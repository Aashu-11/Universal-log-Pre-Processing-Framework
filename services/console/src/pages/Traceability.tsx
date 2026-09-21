import { useEffect, useState } from "react";

import { Card } from "../components/ui/Card";
import { Button } from "../components/ui/Button";
import { Badge, statusTone } from "../components/ui/Badge";
import { ErrorState } from "../components/ui/States";
import { CodeBlock } from "../components/ui/CodeBlock";
import { FullRecordPanel } from "../components/FullRecordPanel";
import { controlPlane, ApiError } from "../lib/api";
import { fetchRecentEvents, type RecentEvent, type TraceResult } from "../lib/liveEvents";

export function Traceability() {
  const [recent, setRecent] = useState<RecentEvent[]>([]);
  const [recentError, setRecentError] = useState<string | null>(null);
  const [recentLoading, setRecentLoading] = useState(true);

  const [selected, setSelected] = useState<RecentEvent | null>(null);
  const [trace, setTrace] = useState<TraceResult | null>(null);
  const [traceError, setTraceError] = useState<string | null>(null);
  const [traceLoading, setTraceLoading] = useState(false);
  const [hexMode, setHexMode] = useState(false);

  const [manual, setManual] = useState({ event_id: "", segment_id: "", offset: "0", length: "0", sha256: "" });
  const [showManual, setShowManual] = useState(false);

  const loadRecent = async () => {
    setRecentLoading(true);
    setRecentError(null);
    try {
      setRecent(await fetchRecentEvents(15));
    } catch (err) {
      setRecentError(
        err instanceof ApiError && err.status === 502
          ? `Presto query unavailable: ${err.message} — you can enter a raw reference manually below`
          : err instanceof ApiError && err.status === 0
            ? `Control plane unreachable: ${err.message}`
            : err instanceof ApiError
              ? `Could not load recent events: ${err.message}`
          : "could not load recent events",
      );
    } finally {
      setRecentLoading(false);
    }
  };

  useEffect(() => {
    loadRecent();
  }, []);

  const fetchTraceFor = async (segment_id: string, offset: number | string, length: number | string, sha256: string, event_id: string) => {
    setTraceLoading(true);
    setTraceError(null);
    setTrace(null);
    try {
      const params = new URLSearchParams({ segment_id, offset: String(offset), length: String(length), sha256 });
      const resp = await controlPlane.get<TraceResult>(`/v1/events/${encodeURIComponent(event_id)}/trace?${params}`);
      setTrace(resp);
    } catch (err) {
      setTraceError(
        err instanceof ApiError && err.status === 502
          ? `Vault unreachable: ${err.message}`
          : err instanceof Error
            ? err.message
            : "trace lookup failed",
      );
    } finally {
      setTraceLoading(false);
    }
  };

  const selectEvent = (ev: RecentEvent) => {
    setSelected(ev);
    fetchTraceFor(ev.raw_segment_id, ev.raw_offset, ev.raw_length, ev.raw_sha256, ev.event_id);
  };

  const runManual = () => {
    setSelected(null);
    fetchTraceFor(manual.segment_id, manual.offset, manual.length, manual.sha256, manual.event_id);
  };

  const rawBytes = trace ? atob(trace.raw.raw_base64) : "";
  const rawDisplay = hexMode
    ? Array.from(rawBytes).map((c) => c.charCodeAt(0).toString(16).padStart(2, "0")).join(" ")
    : rawBytes;

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-[16px] font-semibold text-[var(--color-text)]">Traceability</h1>
        <p className="text-[11px] text-[var(--color-text-muted)]">
          Click any recent event below — every phase it went through is shown automatically, no lookup needed.
        </p>
      </div>

      <Card
        title="Recent events — click one to trace it"
        action={
          <button
            onClick={loadRecent}
            disabled={recentLoading}
            className="rounded border border-[var(--color-border-strong)] px-2 py-0.5 text-[11px] text-[var(--color-text-secondary)] hover:bg-white/5"
          >
            {recentLoading ? "Loading…" : "Refresh"}
          </button>
        }
      >
        {recentError && <ErrorState message={recentError} />}
        {!recentError && recent.length === 0 && !recentLoading && (
          <p className="text-[11.5px] text-[var(--color-text-muted)]">
            No events in the lake yet — send some traffic through the collector first.
          </p>
        )}
        {recent.length > 0 && (
          <div className="overflow-x-auto rounded border border-[var(--color-border)]">
            <table className="w-full text-left text-[11.5px]">
              <thead>
                <tr className="border-b border-[var(--color-border)] bg-white/5">
                  {["event_id", "vendor", "action", "parser", "risk", "quality"].map((h) => (
                    <th key={h} className="px-3 py-1.5 font-mono-data font-medium text-[var(--color-text-secondary)]">
                      {h}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {recent.map((ev) => (
                  <tr
                    key={ev.event_id}
                    onClick={() => selectEvent(ev)}
                    className={`cursor-pointer border-b border-[var(--color-border)] last:border-0 hover:bg-[var(--color-accent)]/10 ${
                      selected?.event_id === ev.event_id ? "bg-[var(--color-accent)]/15" : ""
                    }`}
                  >
                    <td className="px-3 py-1.5 font-mono-data text-[var(--color-text)]">{ev.event_id.slice(0, 16)}…</td>
                    <td className="px-3 py-1.5 font-mono-data text-[var(--color-text)]">{ev.observer_vendor}</td>
                    <td className="px-3 py-1.5">
                      <Badge tone={statusTone(ev.event_action || "unknown")}>{ev.event_action || "—"}</Badge>
                    </td>
                    <td className="px-3 py-1.5 font-mono-data text-[11px] text-[var(--color-text-secondary)]">{ev.lineage_parser_id}</td>
                    <td className="px-3 py-1.5 font-mono-data text-[var(--color-text)]">{ev.enrich_risk_score.toFixed(1)}</td>
                    <td className="px-3 py-1.5 font-mono-data text-[var(--color-text)]">{ev.quality_score.toFixed(2)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        <button
          onClick={() => setShowManual((v) => !v)}
          className="mt-3 text-[11px] text-[var(--color-text-muted)] hover:text-[var(--color-text-secondary)]"
        >
          {showManual ? "hide" : "advanced: enter a raw ref manually"}
        </button>
        {showManual && (
          <div className="mt-3 border-t border-[var(--color-border)] pt-3">
            <div className="grid grid-cols-5 gap-3">
              <LabeledInput label="event_id" value={manual.event_id} onChange={(v) => setManual((f) => ({ ...f, event_id: v }))} />
              <LabeledInput label="segment_id" value={manual.segment_id} onChange={(v) => setManual((f) => ({ ...f, segment_id: v }))} />
              <LabeledInput label="offset" value={manual.offset} onChange={(v) => setManual((f) => ({ ...f, offset: v }))} />
              <LabeledInput label="length" value={manual.length} onChange={(v) => setManual((f) => ({ ...f, length: v }))} />
              <LabeledInput label="sha256" value={manual.sha256} onChange={(v) => setManual((f) => ({ ...f, sha256: v }))} />
            </div>
            <Button variant="primary" className="mt-3" onClick={runManual} disabled={traceLoading || !manual.segment_id}>
              {traceLoading ? "Fetching…" : "Fetch & verify"}
            </Button>
          </div>
        )}
      </Card>

      {traceError && <ErrorState message={traceError} />}

      {selected && trace && (
        <div>
          <h2 className="mb-3 text-[13px] font-semibold text-[var(--color-text)]">
            This event's journey through every phase
          </h2>
          <div className="grid grid-cols-3 gap-3 lg:grid-cols-6">
            <StageCard
              stage="1. PRESERVE"
              tone={trace.raw.sha256_verified ? "success" : "danger"}
              rows={[
                ["SHA-256", trace.raw.sha256_verified ? "verified" : "MISMATCH"],
                ["Segment", selected.raw_segment_id.slice(-12)],
                ["Bytes", `${trace.raw.length}B`],
              ]}
            />
            <StageCard
              stage="2. IDENTIFY + PARSE"
              tone={statusTone(selected.lineage_parse_status || "unknown")}
              rows={[
                ["Parser", selected.lineage_parser_id],
                ["Version", selected.lineage_parser_version],
                ["Status", selected.lineage_parse_status],
                ["Took", `${selected.lineage_processing_ms.toFixed(3)}ms`],
              ]}
            />
            <StageCard
              stage="3. NORMALIZE"
              tone="accent"
              rows={[
                ["event.kind", selected.event_kind],
                ["event.action", selected.event_action || "—"],
                ["observer.vendor", selected.observer_vendor],
              ]}
            />
            <StageCard
              stage="4. ENRICH"
              tone="accent"
              rows={[
                ["Risk score", selected.enrich_risk_score.toFixed(2)],
                ["Src country", selected.enrich_src_geo_country || "—"],
                ["Src AS org", selected.enrich_src_as_org || "—"],
              ]}
            />
            <StageCard
              stage="5. VALIDATE"
              tone={selected.quality_score >= 0.8 ? "success" : selected.quality_score > 0 ? "warning" : "neutral"}
              rows={[["Quality score", selected.quality_score.toFixed(2)]]}
            />
            <StageCard
              stage="6. ROUTE"
              tone="success"
              rows={[
                ["Lake partition", `dt=${selected.dt}`],
                ["hour", selected.hour],
                ["vendor", selected.vendor],
              ]}
            />
          </div>
        </div>
      )}

      {trace && (
        <div className="grid grid-cols-2 gap-4">
          <Card
            title="Raw bytes (PRESERVE — exactly what the collector received)"
            action={
              <button
                onClick={() => setHexMode((v) => !v)}
                className="rounded border border-[var(--color-border-strong)] px-2 py-0.5 text-[11px] text-[var(--color-text-secondary)] hover:bg-white/5"
              >
                {hexMode ? "show text" : "show hex"}
              </button>
            }
          >
            <CodeBlock wrap>{rawDisplay}</CodeBlock>
          </Card>

          <Card title="Chain of custody">
            <div className="space-y-2 text-[12px]">
              <Row label="SHA-256 verified" node={<Badge tone={trace.raw.sha256_verified ? "success" : "danger"}>{trace.raw.sha256_verified ? "PASS" : "FAIL"}</Badge>} />
              <Row label="Length" node={<span className="font-mono-data">{trace.raw.length} bytes</span>} />
              <Row label="Merkle root" node={<span className="font-mono-data text-[11px]">{trace.merkle_proof.root}</span>} />
              <Row label="Leaf hash" node={<span className="font-mono-data text-[11px]">{trace.merkle_proof.leaf}</span>} />
              <Row label="Inclusion path length" node={<span className="font-mono-data">{trace.merkle_proof.path?.length ?? 0} steps</span>} />
            </div>
          </Card>
        </div>
      )}

      {trace && <FullRecordPanel eventId={selected?.event_id ?? manual.event_id} />}
    </div>
  );
}

function StageCard({
  stage,
  tone,
  rows,
}: {
  stage: string;
  tone: "success" | "danger" | "warning" | "neutral" | "accent";
  rows: [string, string][];
}) {
  const borderTone: Record<typeof tone, string> = {
    success: "border-[var(--color-success)]/40",
    danger: "border-[var(--color-danger)]/40",
    warning: "border-[var(--color-warning)]/40",
    accent: "border-[var(--color-accent)]/40",
    neutral: "border-[var(--color-border-strong)]",
  };
  return (
    <div className={`rounded-md border ${borderTone[tone]} bg-[var(--color-surface)] p-3`}>
      <div className="mb-2 text-[10.5px] font-semibold uppercase tracking-wide text-[var(--color-text-secondary)]">{stage}</div>
      <div className="space-y-1">
        {rows.map(([k, v]) => (
          <div key={k} className="text-[11px]">
            <div className="text-[var(--color-text-muted)]">{k}</div>
            <div className="truncate font-mono-data text-[var(--color-text)]" title={v}>
              {v || "—"}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

function LabeledInput({ label, value, onChange }: { label: string; value: string; onChange: (v: string) => void }) {
  return (
    <div>
      <label className="mb-1 block text-[10.5px] text-[var(--color-text-secondary)]">{label}</label>
      <input
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="w-full rounded border border-[var(--color-border-strong)] bg-black/30 px-2 py-1 font-mono-data text-[11.5px] text-[var(--color-text)] outline-none focus:border-[var(--color-accent)]"
      />
    </div>
  );
}

function Row({ label, node }: { label: string; node: React.ReactNode }) {
  return (
    <div className="flex items-center justify-between border-b border-[var(--color-border)] py-1.5 last:border-0">
      <span className="text-[var(--color-text-secondary)]">{label}</span>
      {node}
    </div>
  );
}
