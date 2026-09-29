import { useState, type ReactNode } from "react";
import { Bar, BarChart, CartesianGrid, Cell, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";

import { Button } from "../components/ui/Button";
import { Card } from "../components/ui/Card";
import { Badge } from "../components/ui/Badge";
import { ErrorState } from "../components/ui/States";
import { controlPlane } from "../lib/api";

type Analysis = {
  mode: string;
  summary: string;
  parser_hints: string[];
  field_hints: string[];
  next_steps: string[];
  signals: Record<string, boolean>;
};

const TOOLTIP_STYLE = { background: "#0b0b10", border: "1px solid rgba(255,255,255,.14)", borderRadius: 8, fontSize: 10, boxShadow: "0 16px 40px rgba(0,0,0,.4)" };

const SIGNAL_LABELS: Record<string, string> = {
  syslog: "Syslog prefix",
  json: "JSON payload",
  kv: "Key=value fields",
  action: "Action verb",
  src_ip: "Source IP",
  dst_ip: "Destination IP",
  threat: "Threat keyword",
};

// Renders the handful of markdown constructs a local model actually produces
// (### headings, **bold**, *italic*, blank-line paragraphs, "N. "/"- " list
// items) as real DOM elements instead of showing literal ** characters — a
// full markdown library is unnecessary for this narrow, known output shape.
function renderMarkdown(text: string): ReactNode {
  const blocks = text.trim().split(/\n\s*\n/);
  return blocks.map((block, bi) => {
    const lines = block.split("\n").map((l) => l.trim()).filter(Boolean);
    const firstLine = lines[0];
    if (lines.length === 0 || firstLine === undefined) return null;
    if (lines.length === 1 && /^([-*_])\1{2,}$/.test(firstLine)) return null;

    const heading = firstLine.match(/^#{2,4}\s+(.*)/);
    if (heading) {
      return (
        <h3 key={bi} className="mt-4 text-[13px] font-semibold uppercase tracking-[0.04em] text-[var(--color-accent-hover)] first:mt-0">
          {renderInline(heading[1] ?? "")}
        </h3>
      );
    }

    const isListBlock = lines.every((l) => /^(\d+\.|-)\s+/.test(l));
    if (isListBlock) {
      const ordered = /^\d+\./.test(firstLine);
      const items = lines.map((l) => l.replace(/^(\d+\.|-)\s+/, ""));
      const ListTag = ordered ? "ol" : "ul";
      return (
        <ListTag key={bi} className={`mt-2 space-y-1.5 text-[13px] leading-6 text-[var(--color-text-secondary)] ${ordered ? "list-decimal" : "list-disc"} pl-5`}>
          {items.map((it, ii) => <li key={ii}>{renderInline(it)}</li>)}
        </ListTag>
      );
    }

    return (
      <p key={bi} className="mt-2 text-[14px] leading-7 text-[var(--color-text-secondary)] first:mt-0">
        {renderInline(lines.join(" "))}
      </p>
    );
  });
}

function renderInline(text: string): ReactNode {
  const parts = text.split(/(\*\*[^*]+\*\*|\*[^*]+\*)/g).filter((p) => p !== "");
  return parts.map((part, i) => {
    if (part.startsWith("**") && part.endsWith("**")) {
      return <strong key={i} className="font-semibold text-[var(--color-text)]">{part.slice(2, -2)}</strong>;
    }
    if (part.startsWith("*") && part.endsWith("*")) {
      return <em key={i}>{part.slice(1, -1)}</em>;
    }
    return part;
  });
}

export function InvestigationAssistant() {
  const [logLine, setLogLine] = useState("<134>Sep 23 12:00:02 firewall action=deny src=10.1.2.3 dst=172.16.0.8 dpt=443 reason=threat");
  const [analysis, setAnalysis] = useState<Analysis | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [running, setRunning] = useState(false);

  const analyze = async () => {
    setRunning(true); setError(null);
    try { setAnalysis(await controlPlane.post<Analysis>("/v1/assistant/analyze", { log_line: logLine })); }
    catch (err) { setError(err instanceof Error ? err.message : "analysis failed"); }
    finally { setRunning(false); }
  };

  const signalData = analysis
    ? Object.entries(analysis.signals).map(([key, matched]) => ({ name: SIGNAL_LABELS[key] ?? key, value: matched ? 1 : 0 }))
    : [];
  const matchedCount = signalData.filter((s) => s.value === 1).length;

  return <div className="space-y-6">
    <div><h1 className="text-[22px] font-semibold text-white">Investigation Assistant</h1><p className="mt-1 text-[14px] text-[var(--color-text-muted)]">Private parser and triage guidance with a deterministic offline fallback.</p></div>
    <Card title="Analyze an unfamiliar log line" action={<Button variant="primary" onClick={analyze} disabled={running}>{running ? "Analyzing…" : "Analyze locally"}</Button>}>
      <textarea value={logLine} onChange={(e) => setLogLine(e.target.value)} className="h-36 w-full rounded-xl border p-4 font-mono-data text-[13px] text-white outline-none" spellCheck={false} />
      <p className="mt-3 text-[12px] text-[var(--color-text-muted)]">If Ollama is configured on a local endpoint, LogKrama uses it. Otherwise the deterministic analysis stays fully offline.</p>
    </Card>
    {error && <ErrorState message={error} />}
    {analysis && <div className="grid gap-5 lg:grid-cols-3">
      <Card
        className="lg:col-span-2"
        title={analysis.mode === "ollama-local" ? "Local model analysis" : "Offline analysis"}
        action={<Badge tone={analysis.mode === "ollama-local" ? "success" : "neutral"}>{analysis.mode}</Badge>}
      >
        {renderMarkdown(analysis.summary)}
      </Card>
      <Card title="Detected signal coverage" action={<span className="font-mono-data text-[11px] text-[var(--color-text-muted)]">{matchedCount}/{signalData.length} matched</span>}>
        <div className="h-[180px]">
          <ResponsiveContainer width="100%" height="100%">
            <BarChart data={signalData} layout="vertical" margin={{ top: 4, right: 12, left: 0, bottom: 0 }}>
              <CartesianGrid stroke="rgba(255,255,255,.045)" horizontal={false} />
              <XAxis type="number" domain={[0, 1]} hide />
              <YAxis type="category" dataKey="name" width={110} tick={{ fontSize: 9, fill: "#aaaab8" }} axisLine={false} tickLine={false} />
              <Tooltip contentStyle={TOOLTIP_STYLE} cursor={{ fill: "rgba(139,92,246,.05)" }} formatter={(v) => (v ? "detected" : "not detected")} />
              <Bar dataKey="value" radius={[0, 3, 3, 0]} barSize={11}>
                {signalData.map((s, i) => <Cell key={i} fill={s.value ? "var(--color-accent)" : "rgba(255,255,255,.08)"} />)}
              </Bar>
            </BarChart>
          </ResponsiveContainer>
        </div>
        <p className="mt-2 text-[11px] text-[var(--color-text-muted)]">From the same deterministic checks that always run, regardless of mode.</p>
      </Card>
      <Card title="Parser & field hints"><div className="space-y-3 text-[13px] text-[var(--color-text-secondary)]">{analysis.parser_hints.map((x) => <p key={x}>• {x}</p>)}<div className="border-t border-white/[0.06] pt-3 font-mono-data text-[12px] text-[var(--color-accent-hover)]">{analysis.field_hints.join(" · ")}</div></div></Card>
      <Card title="Recommended next steps" className="lg:col-span-2"><ol className="space-y-3 text-[13px] leading-6 text-[var(--color-text-secondary)]">{analysis.next_steps.map((x, i) => <li key={x}>{i + 1}. {x}</li>)}</ol></Card>
    </div>}
  </div>;
}
