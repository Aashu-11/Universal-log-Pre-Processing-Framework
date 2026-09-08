import { useState } from "react";

import { Card } from "../components/ui/Card";
import { Button } from "../components/ui/Button";
import { ErrorState } from "../components/ui/States";
import { controlPlane, ApiError } from "../lib/api";
import { SAVED_QUERIES } from "../lib/savedQueries";
import type { QueryResponse } from "../lib/types";

const CATALOGS = [
  { name: "lake", connector: "hive", purpose: "Normalized UES events, Parquet on MinIO" },
  { name: "stream", connector: "kafka", purpose: "Live normalized events, last 30 min" },
  { name: "meta", connector: "postgresql", purpose: "Parser registry, source inventory, audit log" },
  { name: "vault", connector: "hive", purpose: "Raw Vault segment index + Merkle roots" },
];

export function Explorer() {
  const [sql, setSql] = useState(SAVED_QUERIES[0]?.sql ?? "");
  const [result, setResult] = useState<QueryResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [running, setRunning] = useState(false);
  const [elapsedMs, setElapsedMs] = useState<number | null>(null);

  const runQuery = async () => {
    setRunning(true);
    setError(null);
    setResult(null);
    const start = performance.now();
    try {
      const resp = await controlPlane.post<QueryResponse>("/v1/query", { sql, catalog: "lake", schema_: "ulpf" });
      setResult(resp);
    } catch (err) {
      if (err instanceof ApiError && err.status === 502) {
        setError(`Presto unreachable: ${err.message} (Docker/Presto not up in this environment yet)`);
      } else {
        setError(err instanceof Error ? err.message : "query failed");
      }
    } finally {
      setElapsedMs(performance.now() - start);
      setRunning(false);
    }
  };

  return (
    <div className="space-y-6">
      <h1 className="text-[16px] font-semibold text-[var(--color-text)]">Explorer</h1>

      <Card title="Catalogs">
        <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
          {CATALOGS.map((c) => (
            <div key={c.name} className="rounded border border-[var(--color-border)] px-3 py-2">
              <div className="font-mono-data text-[13px] font-semibold text-[var(--color-accent)]">{c.name}</div>
              <div className="text-[10.5px] text-[var(--color-text-muted)]">{c.connector}</div>
              <div className="mt-1 text-[11px] text-[var(--color-text-secondary)]">{c.purpose}</div>
            </div>
          ))}
        </div>
      </Card>

      <div className="grid grid-cols-[240px_1fr] gap-4">
        <Card title="Saved queries" className="h-fit">
          <div className="space-y-1">
            {SAVED_QUERIES.map((q) => (
              <button
                key={q.id}
                onClick={() => setSql(q.sql)}
                className="block w-full rounded px-2 py-1.5 text-left text-[11.5px] text-[var(--color-text-secondary)] hover:bg-white/5 hover:text-[var(--color-text)]"
                title={q.explains}
              >
                {q.title}
              </button>
            ))}
          </div>
        </Card>

        <Card
          title="SQL"
          action={
            <Button variant="primary" onClick={runQuery} disabled={running}>
              {running ? "Running…" : "Run"}
            </Button>
          }
        >
          <textarea
            value={sql}
            onChange={(e) => setSql(e.target.value)}
            spellCheck={false}
            className="h-48 w-full resize-y rounded border border-[var(--color-border-strong)] bg-black/40 p-3 font-mono-data text-[12.5px] text-[var(--color-text)] outline-none focus:border-[var(--color-accent)]"
          />

          {elapsedMs !== null && (
            <div className="mt-2 text-[11px] text-[var(--color-text-muted)]">
              {result ? `${result.row_count} rows` : "failed"} · {elapsedMs.toFixed(0)}ms client round trip
            </div>
          )}

          {error && (
            <div className="mt-3">
              <ErrorState message={error} />
            </div>
          )}

          {result && (
            <div className="mt-3 overflow-x-auto rounded border border-[var(--color-border)]">
              <table className="w-full text-left text-[11.5px]">
                <thead>
                  <tr className="border-b border-[var(--color-border)] bg-white/5">
                    {result.columns.map((c) => (
                      <th key={c} className="px-3 py-1.5 font-mono-data font-medium text-[var(--color-text-secondary)]">
                        {c}
                      </th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {result.rows.map((row, i) => (
                    <tr key={i} className="border-b border-[var(--color-border)] last:border-0">
                      {row.map((cell, j) => (
                        <td key={j} className="px-3 py-1.5 font-mono-data text-[var(--color-text)]">
                          {String(cell)}
                        </td>
                      ))}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </Card>
      </div>
    </div>
  );
}
