import { useState } from "react";

import { Card } from "../components/ui/Card";
import { Badge } from "../components/ui/Badge";
import { Button } from "../components/ui/Button";
import { LoadingState, ErrorState, EmptyState } from "../components/ui/States";
import { controlPlane } from "../lib/api";
import { useApi } from "../lib/useApi";
import { canWrite, useAuth } from "../lib/auth";
import type { DLQEventOut } from "../lib/types";

export function DLQ() {
  const { role } = useAuth();
  const { data, error, loading, reload } = useApi(() => controlPlane.get<DLQEventOut[]>("/v1/dlq"), []);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [replaying, setReplaying] = useState(false);

  const toggle = (id: string) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const replaySelected = async () => {
    setReplaying(true);
    try {
      await controlPlane.post("/v1/dlq/replay", { event_ids: Array.from(selected) });
      setSelected(new Set());
      reload();
    } finally {
      setReplaying(false);
    }
  };

  const grouped = groupByReason(data ?? []);

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-[16px] font-semibold text-[var(--color-text)]">Dead Letter Queue</h1>
        {canWrite(role) && selected.size > 0 && (
          <Button variant="primary" onClick={replaySelected} disabled={replaying}>
            {replaying ? "Replaying…" : `Replay ${selected.size} event${selected.size === 1 ? "" : "s"}`}
          </Button>
        )}
      </div>

      <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
        {Object.entries(grouped).map(([reason, items]) => (
          <div key={reason} className="rounded border border-[var(--color-border)] px-3 py-2">
            <div className="font-mono-data text-[12px] text-[var(--color-text-secondary)]">{reason}</div>
            <div className="text-xl font-semibold text-[var(--color-warning)]">{items.length}</div>
          </div>
        ))}
      </div>

      <Card title="Events">
        {loading && <LoadingState />}
        {error && <ErrorState message={error} />}
        {!loading && !error && (!data || data.length === 0) && <EmptyState message="DLQ is empty." />}
        {!loading && data && data.length > 0 && (
          <table className="w-full text-left text-[12px]">
            <thead>
              <tr className="border-b border-[var(--color-border)] text-[11px] uppercase tracking-wide text-[var(--color-text-muted)]">
                <th className="w-8 py-2"></th>
                <th className="py-2 pr-4">Event ID</th>
                <th className="py-2 pr-4">Reason</th>
                <th className="py-2 pr-4">Parser</th>
                <th className="py-2 pr-4">Occurred</th>
                <th className="py-2 pr-4">Status</th>
              </tr>
            </thead>
            <tbody>
              {data.map((e) => (
                <tr key={e.event_id} className="border-b border-[var(--color-border)] last:border-0">
                  <td className="py-2">
                    <input
                      type="checkbox"
                      checked={selected.has(e.event_id)}
                      disabled={e.resolved}
                      onChange={() => toggle(e.event_id)}
                    />
                  </td>
                  <td className="py-2 pr-4 font-mono-data">{e.event_id}</td>
                  <td className="py-2 pr-4">
                    <Badge tone="warning">{e.reason}</Badge>
                  </td>
                  <td className="py-2 pr-4 font-mono-data text-[var(--color-text-secondary)]">{e.parser_id ?? "—"}</td>
                  <td className="py-2 pr-4 text-[var(--color-text-secondary)]">{new Date(e.occurred_at).toLocaleString()}</td>
                  <td className="py-2 pr-4">
                    <Badge tone={e.resolved ? "success" : "neutral"}>{e.resolved ? "resolved" : "open"}</Badge>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </Card>
    </div>
  );
}

function groupByReason(events: DLQEventOut[]): Record<string, DLQEventOut[]> {
  const out: Record<string, DLQEventOut[]> = {};
  for (const e of events) {
    out[e.reason] ??= [];
    out[e.reason]!.push(e);
  }
  return out;
}
