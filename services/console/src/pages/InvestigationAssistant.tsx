import { useState } from "react";

import { Button } from "../components/ui/Button";
import { Card } from "../components/ui/Card";
import { ErrorState } from "../components/ui/States";
import { controlPlane } from "../lib/api";

type Analysis = { mode: string; summary: string; parser_hints: string[]; field_hints: string[]; next_steps: string[] };

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

  return <div className="space-y-6">
    <div><h1 className="text-[22px] font-semibold text-white">Investigation Assistant</h1><p className="mt-1 text-[14px] text-[var(--color-text-muted)]">Private parser and triage guidance with a deterministic offline fallback.</p></div>
    <Card title="Analyze an unfamiliar log line" action={<Button variant="primary" onClick={analyze} disabled={running}>{running ? "Analyzing…" : "Analyze locally"}</Button>}>
      <textarea value={logLine} onChange={(e) => setLogLine(e.target.value)} className="h-36 w-full rounded-xl border p-4 font-mono-data text-[13px] text-white outline-none" spellCheck={false} />
      <p className="mt-3 text-[12px] text-[var(--color-text-muted)]">If Ollama is configured on a local endpoint, LogKrama uses it. Otherwise the deterministic analysis stays fully offline.</p>
    </Card>
    {error && <ErrorState message={error} />}
    {analysis && <div className="grid gap-5 lg:grid-cols-3">
      <Card title={analysis.mode === "ollama-local" ? "Local model analysis" : "Offline analysis"}><p className="text-[14px] leading-7 text-[var(--color-text-secondary)]">{analysis.summary}</p></Card>
      <Card title="Parser & field hints"><div className="space-y-3 text-[13px] text-[var(--color-text-secondary)]">{analysis.parser_hints.map((x) => <p key={x}>• {x}</p>)}<div className="border-t border-white/[0.06] pt-3 font-mono-data text-[12px] text-[var(--color-accent-hover)]">{analysis.field_hints.join(" · ")}</div></div></Card>
      <Card title="Recommended next steps"><ol className="space-y-3 text-[13px] leading-6 text-[var(--color-text-secondary)]">{analysis.next_steps.map((x, i) => <li key={x}>{i + 1}. {x}</li>)}</ol></Card>
    </div>}
  </div>;
}
