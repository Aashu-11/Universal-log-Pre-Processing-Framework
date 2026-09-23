import { useEffect, useState } from "react";

import { Card } from "./ui/Card";
import { ErrorState } from "./ui/States";
import { CodeBlock } from "./ui/CodeBlock";
import { fetchFullRecord } from "../lib/liveEvents";

// Groups every UES namespace CLAUDE.md defines as mandatory, in schema
// order — this is what makes "uniform format" visible: the same groups
// appear whether the source row came from Cisco, Fortinet, or Palo Alto,
// with whichever fields that vendor's parser actually populated.
const NAMESPACES: { label: string; prefix: string }[] = [
  { label: "event", prefix: "event_" },
  { label: "observer", prefix: "observer_" },
  { label: "src", prefix: "src_" },
  { label: "dst", prefix: "dst_" },
  { label: "network", prefix: "network_" },
  { label: "http", prefix: "http_" },
  { label: "url", prefix: "url_" },
  { label: "dns", prefix: "dns_" },
  { label: "tls", prefix: "tls_" },
  { label: "actor", prefix: "actor_" },
  { label: "threat", prefix: "threat_" },
  { label: "enrich", prefix: "enrich_" },
  { label: "raw", prefix: "raw_" },
  { label: "lineage", prefix: "lineage_" },
];

function isEmpty(v: unknown): boolean {
  return v === null || v === undefined || v === "" || (typeof v === "object" && v !== null && Object.keys(v).length === 0);
}

export function FullRecordPanel({ eventId }: { eventId: string }) {
  const [open, setOpen] = useState(false);
  const [record, setRecord] = useState<Record<string, unknown> | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [hideEmpty, setHideEmpty] = useState(true);
  const [asJson, setAsJson] = useState(false);

  useEffect(() => {
    if (!open || record) return;
    setLoading(true);
    setError(null);
    fetchFullRecord(eventId)
      .then((r) => {
        const obj: Record<string, unknown> = {};
        r.columns.forEach((c, i) => (obj[c] = r.values[i]));
        setRecord(obj);
      })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : "could not load full record"))
      .finally(() => setLoading(false));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, eventId]);

  return (
    <Card
      title="Full parsed record — every UES field, uniform regardless of source vendor"
      action={
        <button
          onClick={() => setOpen((v) => !v)}
          className="rounded border border-[var(--color-border-strong)] px-2.5 py-1 text-[11px] text-[var(--color-text-secondary)] hover:bg-white/5"
        >
          {open ? "Collapse" : "Show full record"}
        </button>
      }
    >
      {!open && (
        <p className="text-[11.5px] text-[var(--color-text-muted)]">
          This event's complete normalized record — ~65 UES columns across every mandatory namespace, plus{" "}
          <code>unmapped</code>, exactly as it sits in <code>lake.logkrama.events</code>. Same shape no matter which
          vendor produced the raw log.
        </p>
      )}

      {open && (
        <div>
          <div className="mb-3 flex items-center gap-4 border-b border-[var(--color-border)] pb-3">
            <label className="flex items-center gap-1.5 text-[11.5px] text-[var(--color-text-secondary)]">
              <input type="checkbox" checked={hideEmpty} onChange={(e) => setHideEmpty(e.target.checked)} />
              hide empty fields
            </label>
            <label className="flex items-center gap-1.5 text-[11.5px] text-[var(--color-text-secondary)]">
              <input type="checkbox" checked={asJson} onChange={(e) => setAsJson(e.target.checked)} />
              raw JSON
            </label>
            {record && (
              <span className="ml-auto text-[10.5px] text-[var(--color-text-muted)]">
                {Object.keys(record).length} columns
              </span>
            )}
          </div>

          {loading && <p className="text-[12px] text-[var(--color-text-muted)]">Loading…</p>}
          {error && <ErrorState message={error} />}

          {record && asJson && <CodeBlock wrap>{JSON.stringify(record, null, 2)}</CodeBlock>}

          {record && !asJson && (
            <div className="space-y-4">
              {NAMESPACES.map((ns) => {
                const fields = Object.entries(record)
                  .filter(([k]) => k.startsWith(ns.prefix))
                  .filter(([, v]) => !hideEmpty || !isEmpty(v));
                if (fields.length === 0) return null;
                return (
                  <div key={ns.prefix}>
                    <div className="mb-1.5 font-mono-data text-[10.5px] font-semibold uppercase tracking-wide text-[var(--color-accent)]">
                      {ns.label}.*
                    </div>
                    <div className="grid grid-cols-2 gap-x-6 gap-y-1 rounded border border-[var(--color-border)] bg-white/[0.02] p-2.5 md:grid-cols-3">
                      {fields.map(([k, v]) => (
                        <div key={k} className="truncate text-[11px]" title={String(v)}>
                          <span className="text-[var(--color-text-muted)]">{k.slice(ns.prefix.length)}: </span>
                          <span className={`font-mono-data ${isEmpty(v) ? "text-[var(--color-text-muted)]" : "text-[var(--color-text)]"}`}>
                            {isEmpty(v) ? "—" : String(v)}
                          </span>
                        </div>
                      ))}
                    </div>
                  </div>
                );
              })}

              <div>
                <div className="mb-1.5 font-mono-data text-[10.5px] font-semibold uppercase tracking-wide text-[var(--color-accent)]">
                  quality &amp; partition
                </div>
                <div className="grid grid-cols-2 gap-x-6 gap-y-1 rounded border border-[var(--color-border)] bg-white/[0.02] p-2.5 md:grid-cols-3">
                  {["event_id", "quality_score", "dt", "hour", "vendor"].map((k) => (
                    <div key={k} className="truncate text-[11px]" title={String(record[k])}>
                      <span className="text-[var(--color-text-muted)]">{k}: </span>
                      <span className="font-mono-data text-[var(--color-text)]">{String(record[k] ?? "—")}</span>
                    </div>
                  ))}
                </div>
              </div>

              <div>
                <div className="mb-1.5 font-mono-data text-[10.5px] font-semibold uppercase tracking-wide text-[var(--color-accent)]">
                  unmapped — extracted fields with no UES home
                </div>
                <div className="rounded border border-[var(--color-border)] bg-white/[0.02] p-2.5">
                  {record.unmapped && typeof record.unmapped === "object" && Object.keys(record.unmapped as object).length > 0 ? (
                    <div className="grid grid-cols-2 gap-x-6 gap-y-1 md:grid-cols-3">
                      {Object.entries(record.unmapped as Record<string, unknown>).map(([k, v]) => (
                        <div key={k} className="truncate text-[11px]" title={String(v)}>
                          <span className="text-[var(--color-warning)]">{k}: </span>
                          <span className="font-mono-data text-[var(--color-text)]">{String(v)}</span>
                        </div>
                      ))}
                    </div>
                  ) : (
                    <span className="text-[11px] text-[var(--color-text-muted)]">
                      empty — every extracted field found a schema home (zero field loss)
                    </span>
                  )}
                </div>
              </div>
            </div>
          )}
        </div>
      )}
    </Card>
  );
}
