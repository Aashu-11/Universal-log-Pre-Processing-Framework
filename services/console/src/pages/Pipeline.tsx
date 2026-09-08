import { useEffect, useRef, useState } from "react";
import { Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";

import { Card } from "../components/ui/Card";
import { StatTile } from "../components/ui/StatTile";
import { Badge } from "../components/ui/Badge";
import { controlPlane } from "../lib/api";
import { usePolling } from "../lib/usePolling";
import type { PipelineStats } from "../lib/types";

const REFRESH_MS = 3000;
const HISTORY_LENGTH = 40; // 40 * 3s = 2 minutes of rolling history

interface RatePoint {
  at: number;
  received: number;
  bytes: number;
  drops: number;
  dlq: number;
}

export function Pipeline() {
  const { data, error, lastUpdated } = usePolling<PipelineStats>(
    () => controlPlane.get<PipelineStats>("/v1/stats/pipeline"),
    REFRESH_MS,
  );
  const prevPoint = useRef<RatePoint | null>(null);
  const [eps, setEps] = useState(0);
  const [bytesPerSec, setBytesPerSec] = useState(0);
  const [dropsPerSec, setDropsPerSec] = useState(0);
  const [history, setHistory] = useState<Array<{ t: string; eps: number }>>([]);

  // Rate computation is a side effect of a new poll landing, not something
  // to derive during render — doing it inline (mutating prevPoint.current
  // while rendering) is exactly the anti-pattern oxlint's react(refs) rule
  // flags: refs must only be read/written in effects or event handlers.
  useEffect(() => {
    if (!data || !lastUpdated) return;
    const point: RatePoint = {
      at: lastUpdated,
      received: data.events_received_total,
      bytes: data.ingest_bytes_total,
      drops: data.udp_drops_total,
      dlq: data.dlq_total,
    };
    const prev = prevPoint.current;
    if (prev && prev.at === point.at) return;

    const dt = prev ? (point.at - prev.at) / 1000 : 0;
    if (prev && dt > 0) {
      const nextEps = Math.max(0, (point.received - prev.received) / dt);
      setEps(nextEps);
      setBytesPerSec(Math.max(0, (point.bytes - prev.bytes) / dt));
      setDropsPerSec(Math.max(0, (point.drops - prev.drops) / dt));
      setHistory((h) => [...h, { t: new Date(point.at).toLocaleTimeString(), eps: nextEps }].slice(-HISTORY_LENGTH));
    }
    prevPoint.current = point;
  }, [data, lastUpdated]);

  const stages = [
    { name: "INGEST", ok: data?.collector_reachable ?? false },
    { name: "PRESERVE", ok: data?.collector_reachable ?? false },
    { name: "IDENTIFY", ok: data?.processor_reachable ?? false },
    { name: "PARSE", ok: data?.processor_reachable ?? false },
    { name: "NORMALIZE", ok: data?.processor_reachable ?? false },
    { name: "ENRICH", ok: data?.processor_reachable ?? false },
    { name: "VALIDATE", ok: data?.processor_reachable ?? false },
    { name: "ROUTE", ok: data?.processor_reachable ?? false },
  ];

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-[16px] font-semibold text-[var(--color-text)]">Pipeline</h1>
        <span className="text-[11px] text-[var(--color-text-muted)]">
          auto-refresh every {REFRESH_MS / 1000}s{lastUpdated ? ` · last update ${new Date(lastUpdated).toLocaleTimeString()}` : ""}
        </span>
      </div>

      {error && (
        <div className="rounded border border-[var(--color-warning)]/30 bg-[var(--color-warning)]/10 px-4 py-2.5 text-[12px] text-[var(--color-warning)]">
          {error} — the collector/processor may not be running. Numbers below are the last-known real
          values from ulpf_meta's scrape, not simulated.
        </div>
      )}

      <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
        <StatTile label="Events / sec" value={eps.toFixed(0)} sublabel="collector ingest rate" />
        <StatTile label="Bytes / sec" value={formatBytes(bytesPerSec)} sublabel="raw ingest throughput" />
        <StatTile
          label="UDP drops / sec"
          value={dropsPerSec.toFixed(1)}
          tone={dropsPerSec > 0 ? "warning" : "default"}
          sublabel="always counted, never silent"
        />
        <StatTile
          label="DLQ (cumulative)"
          value={(data?.dlq_total ?? 0).toFixed(0)}
          tone={(data?.dlq_total ?? 0) > 0 ? "warning" : "default"}
          sublabel="validation failures routed to DLQ"
        />
      </div>

      <Card title="Events / sec — last 2 minutes">
        {history.length > 1 ? (
          <ResponsiveContainer width="100%" height={140}>
            <LineChart data={history}>
              <XAxis dataKey="t" tick={{ fontSize: 10, fill: "var(--color-text-muted)" }} interval="preserveStartEnd" />
              <YAxis tick={{ fontSize: 10, fill: "var(--color-text-muted)" }} width={40} />
              <Tooltip
                contentStyle={{ background: "var(--color-surface)", border: "1px solid var(--color-border)", fontSize: 11 }}
                labelStyle={{ color: "var(--color-text-secondary)" }}
              />
              <Line type="monotone" dataKey="eps" stroke="var(--color-accent)" strokeWidth={1.5} dot={false} isAnimationActive={false} />
            </LineChart>
          </ResponsiveContainer>
        ) : (
          <div className="py-6 text-center text-[11.5px] text-[var(--color-text-muted)]">
            Gathering samples — the chart fills in over the next {REFRESH_MS / 1000}s poll cycles.
          </div>
        )}
      </Card>

      <Card title="Stage flow">
        <div className="flex flex-wrap items-center gap-2">
          {stages.map((s, i) => (
            <div key={s.name} className="flex items-center gap-2">
              <div
                className={`rounded border px-3 py-2 text-center text-[11px] font-semibold tracking-wide ${
                  s.ok
                    ? "border-[var(--color-success)]/30 bg-[var(--color-success)]/10 text-[var(--color-success)]"
                    : "border-[var(--color-border-strong)] bg-white/5 text-[var(--color-text-muted)]"
                }`}
              >
                {s.name}
              </div>
              {i < stages.length - 1 && <span className="text-[var(--color-text-muted)]">→</span>}
            </div>
          ))}
        </div>
      </Card>

      <div className="grid grid-cols-2 gap-4">
        <Card title="Collector">
          <StatusRow label="Reachable" ok={data?.collector_reachable ?? false} />
          <StatusRow label="Events received (total)" value={(data?.events_received_total ?? 0).toLocaleString()} />
          <StatusRow label="Ingest bytes (total)" value={formatBytes(data?.ingest_bytes_total ?? 0)} />
          <StatusRow label="UDP drops (total)" value={(data?.udp_drops_total ?? 0).toLocaleString()} />
        </Card>
        <Card title="Processor">
          <StatusRow label="Reachable" ok={data?.processor_reachable ?? false} />
          <StatusRow label="DLQ (total)" value={(data?.dlq_total ?? 0).toLocaleString()} />
          <div className="mt-2 text-[11px] text-[var(--color-text-muted)]">
            Per-stage latency and Kafka consumer lag require Prometheus/Kafka reachable — see the
            Reviewer Mode page for the current infra status.
          </div>
        </Card>
      </div>
    </div>
  );
}

function StatusRow({ label, value, ok }: { label: string; value?: string; ok?: boolean }) {
  return (
    <div className="flex items-center justify-between border-b border-[var(--color-border)] py-1.5 last:border-0">
      <span className="text-[12px] text-[var(--color-text-secondary)]">{label}</span>
      {ok !== undefined ? (
        <Badge tone={ok ? "success" : "danger"}>{ok ? "reachable" : "unreachable"}</Badge>
      ) : (
        <span className="font-mono-data text-[12px] text-[var(--color-text)]">{value}</span>
      )}
    </div>
  );
}

function formatBytes(n: number): string {
  if (n < 1024) return `${n.toFixed(0)} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  if (n < 1024 * 1024 * 1024) return `${(n / 1024 / 1024).toFixed(1)} MB`;
  return `${(n / 1024 / 1024 / 1024).toFixed(2)} GB`;
}
