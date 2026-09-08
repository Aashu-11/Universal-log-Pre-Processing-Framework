import { useState, type FormEvent } from "react";

import { Card } from "../components/ui/Card";
import { Badge, statusTone } from "../components/ui/Badge";
import { Button } from "../components/ui/Button";
import { LoadingState, ErrorState, EmptyState } from "../components/ui/States";
import { controlPlane } from "../lib/api";
import { useApi } from "../lib/useApi";
import { useAuth, canWrite } from "../lib/auth";
import type { Source } from "../lib/types";

export function Sources() {
  const { role } = useAuth();
  const { data, error, loading, reload } = useApi(() => controlPlane.get<Source[]>("/v1/sources"), []);
  const [showForm, setShowForm] = useState(false);

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-[16px] font-semibold text-[var(--color-text)]">Sources</h1>
        {canWrite(role) && (
          <Button variant="primary" onClick={() => setShowForm((v) => !v)}>
            {showForm ? "Cancel" : "+ Add source"}
          </Button>
        )}
      </div>

      {showForm && (
        <Card title="New source">
          <SourceForm
            onCreated={() => {
              setShowForm(false);
              reload();
            }}
          />
        </Card>
      )}

      <Card title="Inventory">
        {loading && <LoadingState />}
        {error && <ErrorState message={error} />}
        {!loading && !error && (!data || data.length === 0) && (
          <EmptyState message="No sources registered yet. Sources appear here once bound via the API, or add one manually above." />
        )}
        {!loading && data && data.length > 0 && (
          <div className="overflow-x-auto">
            <table className="w-full text-left text-[12px]">
              <thead>
                <tr className="border-b border-[var(--color-border)] text-[11px] uppercase tracking-wide text-[var(--color-text-muted)]">
                  <th className="py-2 pr-4">Name</th>
                  <th className="py-2 pr-4">Vendor / Product</th>
                  <th className="py-2 pr-4">Parser</th>
                  <th className="py-2 pr-4">Binding</th>
                  <th className="py-2 pr-4">Status</th>
                  <th className="py-2 pr-4">Last seen</th>
                </tr>
              </thead>
              <tbody>
                {data.map((s) => (
                  <tr key={s.log_source_id} className="border-b border-[var(--color-border)] last:border-0">
                    <td className="py-2 pr-4 font-medium text-[var(--color-text)]">{s.name}</td>
                    <td className="py-2 pr-4 font-mono-data text-[var(--color-text-secondary)]">
                      {s.vendor}.{s.product}
                    </td>
                    <td className="py-2 pr-4 font-mono-data text-[var(--color-text-secondary)]">
                      {s.parser_id ?? <span className="text-[var(--color-text-muted)]">unbound</span>}
                    </td>
                    <td className="py-2 pr-4 font-mono-data text-[var(--color-text-secondary)]">
                      {s.binding_listener ?? "—"} {s.binding_peer_ip ? `/ ${s.binding_peer_ip}` : ""}
                    </td>
                    <td className="py-2 pr-4">
                      <Badge tone={statusTone(s.status)}>{s.status}</Badge>
                    </td>
                    <td className="py-2 pr-4 text-[var(--color-text-secondary)]">
                      {new Date(s.last_seen).toLocaleString()}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Card>
    </div>
  );
}

function SourceForm({ onCreated }: { onCreated: () => void }) {
  const [form, setForm] = useState({
    name: "",
    vendor: "",
    product: "",
    observer_type: "firewall",
    binding_peer_ip: "",
    binding_listener: "",
    parser_id: "",
  });
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setSubmitting(true);
    setError(null);
    try {
      await controlPlane.post("/v1/sources", form);
      onCreated();
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to create source");
    } finally {
      setSubmitting(false);
    }
  };

  const field = (key: keyof typeof form, label: string, required = false) => (
    <div>
      <label className="mb-1 block text-[11px] text-[var(--color-text-secondary)]">{label}</label>
      <input
        required={required}
        value={form[key]}
        onChange={(e) => setForm((f) => ({ ...f, [key]: e.target.value }))}
        className="w-full rounded border border-[var(--color-border-strong)] bg-black/30 px-2.5 py-1.5 text-[12.5px] text-[var(--color-text)] outline-none focus:border-[var(--color-accent)]"
      />
    </div>
  );

  return (
    <form onSubmit={onSubmit} className="space-y-3">
      <div className="grid grid-cols-2 gap-3">
        {field("name", "Name", true)}
        {field("vendor", "Vendor", true)}
        {field("product", "Product", true)}
        {field("observer_type", "Observer type", true)}
        {field("binding_peer_ip", "Binding peer IP")}
        {field("binding_listener", "Binding listener")}
        {field("parser_id", "Parser id")}
      </div>
      {error && <ErrorState message={error} />}
      <Button type="submit" variant="primary" disabled={submitting}>
        {submitting ? "Creating…" : "Create source"}
      </Button>
    </form>
  );
}
