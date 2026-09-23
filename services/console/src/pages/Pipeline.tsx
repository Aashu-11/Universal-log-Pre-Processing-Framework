import { useEffect, useRef, useState } from "react";
import { Area, AreaChart, Bar, BarChart, CartesianGrid, Cell, Line, LineChart, Pie, PieChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";

import { Card } from "../components/ui/Card";
import { StatTile } from "../components/ui/StatTile";
import { Badge } from "../components/ui/Badge";
import { controlPlane } from "../lib/api";
import { fetchRecentEvents, type RecentEvent } from "../lib/liveEvents";
import { usePolling } from "../lib/usePolling";
import type { PipelineStats } from "../lib/types";

const REFRESH_MS = 3000;
const HISTORY_LENGTH = 40;

interface RatePoint { at: number; received: number; bytes: number; drops: number; dlq: number; }

const STAGES = [
  { name: "INGEST", short: "Acquire", owner: "Collector", phase: 1 },
  { name: "PRESERVE", short: "Raw vault", owner: "Collector", phase: 2 },
  { name: "IDENTIFY", short: "Resolve", owner: "Processor", phase: 3 },
  { name: "PARSE", short: "Extract", owner: "Processor", phase: 4 },
  { name: "NORMALIZE", short: "UES map", owner: "Processor", phase: 5 },
  { name: "ENRICH", short: "Context", owner: "Processor", phase: 6 },
  { name: "VALIDATE", short: "Assure", owner: "Processor", phase: 7 },
  { name: "ROUTE", short: "Deliver", owner: "Processor", phase: 8 },
];

export function Pipeline() {
  const { data, error, lastUpdated } = usePolling<PipelineStats>(() => controlPlane.get<PipelineStats>("/v1/stats/pipeline"), REFRESH_MS);
  const { data: recentEvents, error: eventError, lastUpdated: eventSampleUpdated } = usePolling<RecentEvent[]>(() => fetchRecentEvents(200), 10000);
  const prevPoint = useRef<RatePoint | null>(null);
  const [eps, setEps] = useState(0);
  const [bytesPerSec, setBytesPerSec] = useState(0);
  const [dropsPerSec, setDropsPerSec] = useState(0);
  const [history, setHistory] = useState<Array<{ t: string; eps: number; bytes: number }>>([]);

  useEffect(() => {
    if (!data || !lastUpdated) return;
    const point: RatePoint = { at: lastUpdated, received: data.events_received_total, bytes: data.ingest_bytes_total, drops: data.udp_drops_total, dlq: data.dlq_total };
    const prev = prevPoint.current;
    if (prev && prev.at === point.at) return;
    const dt = prev ? (point.at - prev.at) / 1000 : 0;
    if (prev && dt > 0) {
      const nextEps = Math.max(0, (point.received - prev.received) / dt);
      const nextBytes = Math.max(0, (point.bytes - prev.bytes) / dt);
      setEps(nextEps);
      setBytesPerSec(nextBytes);
      setDropsPerSec(Math.max(0, (point.drops - prev.drops) / dt));
      setHistory((h) => [...h, { t: new Date(point.at).toLocaleTimeString([], { minute: "2-digit", second: "2-digit" }), eps: nextEps, bytes: nextBytes }].slice(-HISTORY_LENGTH));
    }
    prevPoint.current = point;
  }, [data, lastUpdated]);

  const collectorOk = data?.collector_reachable ?? false;
  const processorOk = data?.processor_reachable ?? false;
  const systemOk = collectorOk && processorOk && !error;
  const totalEvents = data?.events_received_total ?? 0;
  const sample = recentEvents ?? [];
  const analytics = buildAnalytics(sample);
  const latestEvent = sample[0] ?? null;

  return (
    <div className="space-y-5">
      <div className="flex flex-col justify-between gap-4 sm:flex-row sm:items-end">
        <div>
          <div className="eyebrow">Operations / Telemetry pipeline</div>
          <div className="mt-2 flex items-center gap-3">
            <h1 className="text-[24px] font-medium tracking-[-0.035em] text-white">Pipeline Command Center</h1>
            <Badge tone={systemOk ? "success" : "warning"}>{systemOk ? "Nominal" : "Attention"}</Badge>
          </div>
          <p className="mt-1.5 max-w-2xl text-[11.5px] leading-relaxed text-[var(--color-text-muted)]">Real-time integrity, throughput, and routing visibility across the universal event processing fabric.</p>
        </div>
        <div className="flex items-center gap-3 rounded-lg border border-white/[0.08] bg-black/20 px-3.5 py-2.5">
          <span className={`h-2 w-2 rounded-full ${systemOk ? "status-pulse bg-[var(--color-success)]" : "bg-[var(--color-warning)]"}`} />
          <div>
            <div className="text-[9px] font-semibold uppercase tracking-[0.14em] text-[var(--color-text-secondary)]">Live telemetry</div>
            <div className="mt-0.5 font-mono-data text-[9px] text-[var(--color-text-muted)]">{lastUpdated ? `SYNC ${new Date(lastUpdated).toLocaleTimeString([], { hour12: false })}` : "AWAITING SIGNAL"} · 3S INTERVAL</div>
          </div>
        </div>
      </div>

      {error && <div className="rounded-xl border border-[var(--color-warning)]/25 bg-[var(--color-warning)]/[0.07] px-4 py-3 text-[11.5px] text-[var(--color-warning)]"><span className="mr-2 font-semibold uppercase tracking-wider">Telemetry warning</span>{error}. Displaying the last confirmed counters.</div>}

      <div className="grid grid-cols-2 gap-3 xl:grid-cols-4">
        <StatTile label="Event velocity" value={formatRate(eps)} sublabel="events processed / second" trend={systemOk ? "● LIVE" : undefined} />
        <StatTile label="Ingress bandwidth" value={formatBytes(bytesPerSec)} sublabel="raw telemetry / second" />
        <StatTile label="Transport loss" value={dropsPerSec.toFixed(1)} tone={dropsPerSec > 0 ? "warning" : "success"} sublabel="UDP drops / second" trend={dropsPerSec === 0 ? "ZERO LOSS" : undefined} />
        <StatTile label="Exception queue" value={(data?.dlq_total ?? 0).toLocaleString()} tone={(data?.dlq_total ?? 0) > 0 ? "warning" : "success"} sublabel="cumulative validation failures" trend={(data?.dlq_total ?? 0) === 0 ? "CLEAR" : undefined} />
      </div>

      <div className="grid gap-4 xl:grid-cols-[minmax(0,1.75fr)_minmax(310px,0.75fr)]">
        <Card title="Event velocity / rolling two-minute window" action={<span className="font-mono-data text-[9px] uppercase tracking-wider text-[var(--color-text-muted)]">EPS · live</span>}>
          <div className="mb-4 flex items-end justify-between border-b border-white/[0.07] pb-4">
            <div><div className="eyebrow">Current throughput</div><div className="mt-1.5 font-mono-data text-[34px] font-medium tracking-[-0.05em] text-white">{formatRate(eps)}<span className="ml-2 text-[11px] font-normal tracking-normal text-[var(--color-text-muted)]">events/sec</span></div></div>
            <div className="text-right"><div className="eyebrow">Total observed</div><div className="mt-2 font-mono-data text-[16px] text-[#b7a1ff]">{totalEvents.toLocaleString()}</div></div>
          </div>
          <div className="h-[218px]">
            {history.length > 1 ? <ResponsiveContainer width="100%" height="100%">
              <AreaChart data={history} margin={{ top: 8, right: 5, left: -18, bottom: 0 }}>
                <defs>
                  <linearGradient id="epsFill" x1="0" y1="0" x2="0" y2="1"><stop offset="0%" stopColor="#8b5cf6" stopOpacity={0.42} /><stop offset="70%" stopColor="#4f71ed" stopOpacity={0.08} /><stop offset="100%" stopColor="#4f71ed" stopOpacity={0} /></linearGradient>
                  <linearGradient id="epsStroke" x1="0" y1="0" x2="1" y2="0"><stop offset="0%" stopColor="#9c77ff" /><stop offset="100%" stopColor="#4b8df8" /></linearGradient>
                </defs>
                <CartesianGrid stroke="rgba(255,255,255,0.055)" vertical={false} />
                <XAxis dataKey="t" tick={{ fontSize: 9, fill: "#626273" }} axisLine={false} tickLine={false} interval="preserveStartEnd" />
                <YAxis tick={{ fontSize: 9, fill: "#626273" }} axisLine={false} tickLine={false} width={46} />
                <Tooltip contentStyle={{ background: "#0b0b10", border: "1px solid rgba(255,255,255,.14)", borderRadius: 8, fontSize: 10, boxShadow: "0 16px 40px rgba(0,0,0,.4)" }} labelStyle={{ color: "#7d7d8c" }} itemStyle={{ color: "#c4b5fd" }} />
                <Area type="monotone" dataKey="eps" stroke="url(#epsStroke)" strokeWidth={2} fill="url(#epsFill)" activeDot={{ r: 3, fill: "#a78bfa", stroke: "#09090d", strokeWidth: 2 }} isAnimationActive={false} />
              </AreaChart>
            </ResponsiveContainer> : <div className="relative grid h-full place-items-center overflow-hidden rounded-lg border border-white/[0.06] bg-black/20">
              <div className="absolute inset-x-0 top-1/2 h-px bg-[linear-gradient(90deg,transparent,rgba(139,92,246,.7),transparent)] scan-line" />
              <div className="text-center"><div className="font-mono-data text-[10px] uppercase tracking-[0.18em] text-[var(--color-text-secondary)]">Establishing baseline</div><div className="mt-2 text-[10px] text-[var(--color-text-muted)]">Two samples required · next poll in ≤3 seconds</div></div>
            </div>}
          </div>
        </Card>

        <Card title="Control plane status" action={<Badge tone={systemOk ? "success" : "warning"}>{systemOk ? "All systems" : "Degraded"}</Badge>}>
          <div className="space-y-1.5">
            <ServiceRow label="Collector plane" detail="Ingress · vault · Kafka" ok={collectorOk} port=":9100" />
            <ServiceRow label="Processor plane" detail="Parse · UES · enrich" ok={processorOk} port=":9101" />
            <ServiceRow label="Integrity chain" detail="SHA-256 · Merkle ledger" ok={collectorOk} port="ACTIVE" />
            <ServiceRow label="Routing fabric" detail="Lake · stream · DLQ" ok={processorOk} port="ACTIVE" />
          </div>
          <div className="mt-4 rounded-lg border border-white/[0.07] bg-black/25 p-3.5">
            <div className="flex items-center justify-between"><span className="eyebrow">Operational posture</span><span className={`font-mono-data text-[9px] ${systemOk ? "text-[var(--color-success)]" : "text-[var(--color-warning)]"}`}>{systemOk ? "PROTECTED" : "REVIEW"}</span></div>
            <div className="mt-3 h-1 overflow-hidden rounded-full bg-white/[0.06]"><div className={`h-full rounded-full bg-[linear-gradient(90deg,#7651de,#3b82f6)] transition-all duration-700 ${systemOk ? "w-full" : collectorOk || processorOk ? "w-1/2" : "w-[8%]"}`} /></div>
            <p className="mt-3 text-[10px] leading-relaxed text-[var(--color-text-muted)]">Health is derived from direct Prometheus scrapes. No simulated availability signals are displayed.</p>
          </div>
        </Card>
      </div>

      <Card title="Eight-stage secure processing chain" action={<span className="eyebrow">End-to-end control path</span>}>
        <div className="grid grid-cols-2 gap-2 sm:grid-cols-4 xl:grid-cols-8">
          {STAGES.map((stage, index) => {
            const ok = stage.owner === "Collector" ? collectorOk : processorOk;
            return <div key={stage.name} className="group relative min-w-0">
              <div className={`relative min-h-[104px] overflow-hidden rounded-lg border p-3 transition-all duration-300 ${ok ? "border-[var(--color-accent)]/20 bg-[linear-gradient(145deg,rgba(124,79,224,.105),rgba(59,130,246,.035))] hover:border-[var(--color-accent)]/45" : "border-white/[0.07] bg-white/[0.015] opacity-60"}`}>
                <div className="flex items-center justify-between"><span className={`font-mono-data text-[9px] ${ok ? "text-[#a98dff]" : "text-[var(--color-text-muted)]"}`}>0{stage.phase}</span><span className={`h-1.5 w-1.5 rounded-full ${ok ? "bg-[var(--color-success)] shadow-[0_0_7px_var(--color-success)]" : "bg-[var(--color-danger)]"}`} /></div>
                <div className="mt-4 truncate text-[10.5px] font-semibold tracking-[0.08em] text-white">{stage.name}</div>
                <div className="mt-1 text-[9px] text-[var(--color-text-muted)]">{stage.short}</div>
                <div className="absolute inset-x-0 bottom-0 h-[2px] bg-[linear-gradient(90deg,var(--color-accent),var(--color-blue))] opacity-50" />
              </div>
              {index < STAGES.length - 1 && <span className="absolute -right-2.5 top-1/2 z-10 hidden -translate-y-1/2 bg-[var(--color-surface)] px-0.5 text-[10px] text-[var(--color-text-muted)] xl:block">›</span>}
            </div>;
          })}
        </div>
      </Card>

      <div className="flex items-end justify-between pt-2">
        <div><div className="eyebrow">Normalized event intelligence</div><h2 className="mt-1.5 text-[17px] font-medium tracking-[-0.02em] text-white">Signal analytics</h2></div>
        <div className="font-mono-data text-[8.5px] uppercase tracking-[0.1em] text-[var(--color-text-muted)]">{eventSampleUpdated ? `${sample.length}-event sample · ${new Date(eventSampleUpdated).toLocaleTimeString([], { hour12: false })}` : "Awaiting event sample"}</div>
      </div>

      {eventError && <div className="rounded-lg border border-[var(--color-warning)]/20 bg-[var(--color-warning)]/[0.05] px-4 py-2.5 text-[10.5px] text-[var(--color-warning)]">Event analytics unavailable: {eventError}. Infrastructure telemetry remains live.</div>}

      <div className="grid gap-4 xl:grid-cols-[1.1fr_0.8fr_1.35fr]">
        <Card title="Telemetry by source vendor" action={<span className="eyebrow">Latest sample</span>}>
          <ChartFrame empty={analytics.vendorData.length === 0}>
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={analytics.vendorData} layout="vertical" margin={{ top: 8, right: 8, left: 0, bottom: 0 }}>
                <defs><linearGradient id="vendorBar" x1="0" y1="0" x2="1" y2="0"><stop offset="0%" stopColor="#8157e7" /><stop offset="100%" stopColor="#4687f3" /></linearGradient></defs>
                <CartesianGrid stroke="rgba(255,255,255,.045)" horizontal={false} />
                <XAxis type="number" tick={{ fontSize: 8, fill: "#747486" }} axisLine={false} tickLine={false} />
                <YAxis type="category" dataKey="name" width={78} tick={{ fontSize: 9, fill: "#aaaab8" }} axisLine={false} tickLine={false} />
                <Tooltip contentStyle={TOOLTIP_STYLE} cursor={{ fill: "rgba(139,92,246,.05)" }} />
                <Bar dataKey="value" fill="url(#vendorBar)" radius={[0, 3, 3, 0]} barSize={13} />
              </BarChart>
            </ResponsiveContainer>
          </ChartFrame>
          <ChartFooter label="Unique parsers" value={String(analytics.parserCount)} secondary={`${sample.length} events analyzed`} />
        </Card>

        <Card title="Risk posture" action={<span className="eyebrow">Enriched score</span>}>
          <ChartFrame empty={sample.length === 0}>
            <div className="relative h-full">
              <ResponsiveContainer width="100%" height="100%">
                <PieChart>
                  <Pie data={analytics.riskData} dataKey="value" nameKey="name" innerRadius="57%" outerRadius="78%" paddingAngle={3} stroke="none">
                    {analytics.riskData.map((entry) => <Cell key={entry.name} fill={entry.color} />)}
                  </Pie>
                  <Tooltip contentStyle={TOOLTIP_STYLE} />
                </PieChart>
              </ResponsiveContainer>
              <div className="pointer-events-none absolute inset-0 grid place-items-center text-center"><div><div className="font-mono-data text-[24px] font-medium text-white">{analytics.averageRisk.toFixed(1)}</div><div className="mt-1 text-[8px] uppercase tracking-[0.12em] text-[var(--color-text-muted)]">Avg risk</div></div></div>
            </div>
          </ChartFrame>
          <div className="grid grid-cols-2 gap-x-3 gap-y-2 border-t border-white/[0.07] pt-3">
            {analytics.riskData.map((item) => <div key={item.name} className="flex items-center justify-between text-[9px]"><span className="flex items-center gap-1.5 text-[var(--color-text-muted)]"><span className="h-1.5 w-1.5 rounded-full" style={{ background: item.color }} />{item.name}</span><span className="font-mono-data text-[var(--color-text-secondary)]">{item.value}</span></div>)}
          </div>
        </Card>

        <Card title="Quality and risk progression" action={<span className="eyebrow">Event sequence</span>}>
          <ChartFrame empty={analytics.timeline.length === 0}>
            <ResponsiveContainer width="100%" height="100%">
              <LineChart data={analytics.timeline} margin={{ top: 8, right: 4, left: -22, bottom: 0 }}>
                <CartesianGrid stroke="rgba(255,255,255,.045)" vertical={false} />
                <XAxis dataKey="event" tick={{ fontSize: 8, fill: "#6f6f80" }} axisLine={false} tickLine={false} interval="preserveStartEnd" />
                <YAxis domain={[0, 100]} tick={{ fontSize: 8, fill: "#6f6f80" }} axisLine={false} tickLine={false} />
                <Tooltip contentStyle={TOOLTIP_STYLE} />
                <Line type="monotone" dataKey="quality" name="Quality" stroke="#8b5cf6" strokeWidth={2} dot={false} activeDot={{ r: 3 }} />
                <Line type="monotone" dataKey="risk" name="Risk" stroke="#3b82f6" strokeWidth={1.5} dot={false} activeDot={{ r: 3 }} />
              </LineChart>
            </ResponsiveContainer>
          </ChartFrame>
          <div className="grid grid-cols-3 gap-2 border-t border-white/[0.07] pt-3">
            <MiniKpi label="Avg quality" value={`${analytics.averageQuality.toFixed(1)}%`} good={analytics.averageQuality >= 80} />
            <MiniKpi label="P95 process" value={`${analytics.p95Processing.toFixed(2)}ms`} />
            <MiniKpi label="Raw linked" value={`${analytics.rawLinkedPct.toFixed(0)}%`} good={analytics.rawLinkedPct === 100} />
          </div>
        </Card>
      </div>

      <Card title="Transformation transparency / latest normalized event" action={latestEvent ? <Badge tone="accent">Live evidence</Badge> : <Badge>Waiting</Badge>}>
        {latestEvent ? <div>
          <div className="mb-4 flex flex-col justify-between gap-3 border-b border-white/[0.07] pb-4 sm:flex-row sm:items-center">
            <div><div className="eyebrow">Event identifier</div><div className="mt-1.5 break-all font-mono-data text-[11px] text-white">{latestEvent.event_id}</div></div>
            <div className="flex flex-wrap gap-2"><Badge tone={latestEvent.lineage_parse_status === "success" ? "success" : "warning"}>{latestEvent.lineage_parse_status}</Badge><Badge tone="neutral">{latestEvent.observer_vendor || latestEvent.vendor}</Badge><Badge tone={riskTone(latestEvent.enrich_risk_score)}>{`Risk ${latestEvent.enrich_risk_score.toFixed(1)}`}</Badge></div>
          </div>
          <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-6">
            <TransformStep index="01" title="Preserve" accent="#8b5cf6" rows={[["segment", compact(latestEvent.raw_segment_id)], ["sha256", compact(latestEvent.raw_sha256)], ["bytes", String(latestEvent.raw_length)]]} />
            <TransformStep index="02" title="Identify" accent="#9467ed" rows={[["vendor", latestEvent.observer_vendor], ["parser", compact(latestEvent.lineage_parser_id)], ["version", latestEvent.lineage_parser_version]]} />
            <TransformStep index="03" title="Parse" accent="#7d71f1" rows={[["status", latestEvent.lineage_parse_status], ["latency", `${latestEvent.lineage_processing_ms.toFixed(3)}ms`], ["kind", latestEvent.event_kind]]} />
            <TransformStep index="04" title="Normalize" accent="#687bf3" rows={[["taxonomy", "UES 1.0"], ["action", latestEvent.event_action || "—"], ["quality", `${(latestEvent.quality_score * 100).toFixed(1)}%`]]} />
            <TransformStep index="05" title="Enrich" accent="#5285f5" rows={[["risk", latestEvent.enrich_risk_score.toFixed(1)], ["country", latestEvent.enrich_src_geo_country || "—"], ["ASN org", compact(latestEvent.enrich_src_as_org)]]} />
            <TransformStep index="06" title="Route" accent="#3b8ff7" rows={[["lake", `${latestEvent.dt}/${latestEvent.hour}`], ["vendor", latestEvent.vendor], ["stream", "Kafka + Parquet"]]} />
          </div>
          <p className="mt-4 text-[9.5px] leading-relaxed text-[var(--color-text-muted)]">Every value above comes from the latest row in <span className="font-mono-data text-[var(--color-text-secondary)]">lake.logkrama.events</span>. This view exposes processing lineage instead of reducing the pipeline to a single green status.</p>
        </div> : <EmptyChart label="No normalized event is available yet" />}
      </Card>

      <div className="grid gap-4 lg:grid-cols-2">
        <Card title="Collector telemetry" action={<Badge tone={collectorOk ? "success" : "danger"}>{collectorOk ? "Reachable" : "Offline"}</Badge>}>
          <MetricRow label="Events received" value={totalEvents.toLocaleString()} meta="lifetime counter" />
          <MetricRow label="Raw volume ingested" value={formatBytes(data?.ingest_bytes_total ?? 0)} meta="byte-exact input" />
          <MetricRow label="UDP datagrams dropped" value={(data?.udp_drops_total ?? 0).toLocaleString()} meta="loss is never silent" safe={(data?.udp_drops_total ?? 0) === 0} />
        </Card>
        <Card title="Processor telemetry" action={<Badge tone={processorOk ? "success" : "danger"}>{processorOk ? "Reachable" : "Offline"}</Badge>}>
          <MetricRow label="Dead-letter events" value={(data?.dlq_total ?? 0).toLocaleString()} meta="validation exceptions" safe={(data?.dlq_total ?? 0) === 0} />
          <MetricRow label="Normalized stream" value={processorOk ? "ONLINE" : "UNAVAILABLE"} meta="Kafka / 30-minute tier" safe={processorOk} />
          <MetricRow label="Historical route" value={processorOk ? "ONLINE" : "UNAVAILABLE"} meta="Parquet / MinIO lake" safe={processorOk} />
        </Card>
      </div>
    </div>
  );
}

const TOOLTIP_STYLE = { background: "#0b0b10", border: "1px solid rgba(255,255,255,.14)", borderRadius: 8, fontSize: 10, boxShadow: "0 16px 40px rgba(0,0,0,.4)" };

function ChartFrame({ empty, children }: { empty: boolean; children: React.ReactNode }) {
  return <div className="h-[210px]">{empty ? <EmptyChart label="Awaiting normalized event data" /> : children}</div>;
}

function EmptyChart({ label }: { label: string }) {
  return <div className="grid h-full min-h-[120px] place-items-center rounded-lg border border-dashed border-white/[0.09] bg-black/15"><div className="text-center"><div className="mx-auto mb-2 h-1.5 w-1.5 rounded-full bg-[var(--color-text-muted)]" /><div className="text-[9px] uppercase tracking-[0.13em] text-[var(--color-text-muted)]">{label}</div></div></div>;
}

function ChartFooter({ label, value, secondary }: { label: string; value: string; secondary: string }) {
  return <div className="flex items-center justify-between border-t border-white/[0.07] pt-3"><div><div className="eyebrow">{label}</div><div className="mt-1 font-mono-data text-[12px] text-white">{value}</div></div><span className="text-[9px] text-[var(--color-text-muted)]">{secondary}</span></div>;
}

function MiniKpi({ label, value, good }: { label: string; value: string; good?: boolean }) {
  return <div className="rounded-md bg-white/[0.025] px-2.5 py-2"><div className="text-[8px] uppercase tracking-[0.09em] text-[var(--color-text-muted)]">{label}</div><div className={`mt-1 font-mono-data text-[10.5px] ${good === true ? "text-[var(--color-success)]" : good === false ? "text-[var(--color-warning)]" : "text-white"}`}>{value}</div></div>;
}

function TransformStep({ index, title, accent, rows }: { index: string; title: string; accent: string; rows: [string, string][] }) {
  return <div className="relative overflow-hidden rounded-lg border border-white/[0.08] bg-white/[0.018] p-3.5">
    <div className="flex items-center justify-between"><span className="font-mono-data text-[8px]" style={{ color: accent }}>{index}</span><span className="h-1.5 w-1.5 rounded-full" style={{ background: accent, boxShadow: `0 0 8px ${accent}` }} /></div>
    <div className="mt-3 text-[10px] font-semibold uppercase tracking-[0.1em] text-white">{title}</div>
    <div className="mt-3 space-y-1.5">{rows.map(([key, value]) => <div key={key} className="min-w-0"><div className="text-[7.5px] uppercase tracking-[0.08em] text-[var(--color-text-muted)]">{key}</div><div className="mt-0.5 truncate font-mono-data text-[9px] text-[var(--color-text-secondary)]" title={value}>{value || "—"}</div></div>)}</div>
    <div className="absolute inset-x-0 bottom-0 h-px" style={{ background: `linear-gradient(90deg, ${accent}, transparent)` }} />
  </div>;
}

function buildAnalytics(events: RecentEvent[]) {
  const vendors = countBy(events, (event) => event.observer_vendor || event.vendor || "unknown");
  const vendorData = Object.entries(vendors).sort((a, b) => b[1] - a[1]).slice(0, 6).map(([name, value]) => ({ name, value }));
  const riskData = [
    { name: "Critical", value: events.filter((event) => event.enrich_risk_score >= 75).length, color: "#fb5b71" },
    { name: "High", value: events.filter((event) => event.enrich_risk_score >= 50 && event.enrich_risk_score < 75).length, color: "#f6a54f" },
    { name: "Moderate", value: events.filter((event) => event.enrich_risk_score >= 25 && event.enrich_risk_score < 50).length, color: "#8b5cf6" },
    { name: "Low", value: events.filter((event) => event.enrich_risk_score < 25).length, color: "#3b82f6" },
  ];
  const timeline = events.slice(0, 40).reverse().map((event, index) => ({ event: index + 1, quality: Math.round(event.quality_score * 100), risk: Math.round(event.enrich_risk_score) }));
  const averageQuality = average(events.map((event) => event.quality_score * 100));
  const averageRisk = average(events.map((event) => event.enrich_risk_score));
  const latencies = events.map((event) => event.lineage_processing_ms).sort((a, b) => a - b);
  const p95Processing = percentile(latencies, 0.95);
  const rawLinkedPct = events.length === 0 ? 0 : events.filter((event) => Boolean(event.raw_sha256 && event.raw_segment_id)).length / events.length * 100;
  const parserCount = new Set(events.map((event) => event.lineage_parser_id).filter(Boolean)).size;
  return { vendorData, riskData, timeline, averageQuality, averageRisk, p95Processing, rawLinkedPct, parserCount };
}

function countBy<T>(items: T[], key: (item: T) => string): Record<string, number> { return items.reduce<Record<string, number>>((out, item) => { const name = key(item); out[name] = (out[name] ?? 0) + 1; return out; }, {}); }
function average(values: number[]): number { return values.length === 0 ? 0 : values.reduce((sum, value) => sum + value, 0) / values.length; }
function percentile(values: number[], p: number): number { return values.length === 0 ? 0 : values[Math.min(values.length - 1, Math.floor(values.length * p))] ?? 0; }
function compact(value: string): string { return value.length > 18 ? `${value.slice(0, 8)}…${value.slice(-7)}` : value || "—"; }
function riskTone(score: number): "danger" | "warning" | "accent" | "success" { return score >= 75 ? "danger" : score >= 50 ? "warning" : score >= 25 ? "accent" : "success"; }

function ServiceRow({ label, detail, ok, port }: { label: string; detail: string; ok: boolean; port: string }) {
  return <div className="flex items-center gap-3 rounded-lg border border-transparent px-2.5 py-2.5 transition hover:border-white/[0.07] hover:bg-white/[0.02]">
    <span className={`grid h-7 w-7 place-items-center rounded-md border ${ok ? "border-[var(--color-success)]/20 bg-[var(--color-success)]/[0.07]" : "border-[var(--color-danger)]/20 bg-[var(--color-danger)]/[0.07]"}`}><span className={`h-1.5 w-1.5 rounded-full ${ok ? "bg-[var(--color-success)]" : "bg-[var(--color-danger)]"}`} /></span>
    <div className="min-w-0 flex-1"><div className="truncate text-[11px] font-medium text-white">{label}</div><div className="mt-0.5 truncate text-[9px] text-[var(--color-text-muted)]">{detail}</div></div>
    <span className="font-mono-data text-[8.5px] text-[var(--color-text-muted)]">{port}</span>
  </div>;
}

function MetricRow({ label, value, meta, safe }: { label: string; value: string; meta: string; safe?: boolean }) {
  return <div className="flex items-center justify-between border-b border-white/[0.07] py-3 first:pt-0 last:border-0 last:pb-0">
    <div><div className="text-[11px] font-medium text-[var(--color-text-secondary)]">{label}</div><div className="mt-0.5 text-[9px] text-[var(--color-text-muted)]">{meta}</div></div>
    <div className={`font-mono-data text-[12px] font-medium ${safe === true ? "text-[var(--color-success)]" : safe === false ? "text-[var(--color-danger)]" : "text-white"}`}>{value}</div>
  </div>;
}

function formatRate(n: number): string { return n >= 1000 ? `${(n / 1000).toFixed(n >= 10000 ? 1 : 2)}K` : n.toFixed(0); }
function formatBytes(n: number): string {
  if (n < 1024) return `${n.toFixed(0)} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  if (n < 1024 * 1024 * 1024) return `${(n / 1024 / 1024).toFixed(1)} MB`;
  return `${(n / 1024 / 1024 / 1024).toFixed(2)} GB`;
}
