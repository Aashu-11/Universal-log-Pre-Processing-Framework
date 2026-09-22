import type { EventVisualState, SeverityFilter } from "../../lib/logverse/types";

const STATE_OPTIONS: { key: EventVisualState; label: string }[] = [
  { key: "normal", label: "Normal" },
  { key: "elevated", label: "Elevated" },
  { key: "threat", label: "Threat / IOC" },
  { key: "dlq", label: "DLQ" },
];

export function LogVerseFilters({
  filter,
  onChange,
  vendors,
  outcomes,
}: {
  filter: SeverityFilter;
  onChange: (next: SeverityFilter) => void;
  vendors: string[];
  outcomes: string[];
}) {
  const toggleState = (s: EventVisualState) => {
    const states = new Set(filter.states);
    if (states.has(s)) states.delete(s);
    else states.add(s);
    onChange({ ...filter, states });
  };

  const toggleSetMember = (set: Set<string>, value: string) => {
    const next = new Set(set);
    if (next.has(value)) next.delete(value);
    else next.add(value);
    return next;
  };

  return (
    <div
      role="region"
      aria-label="Event filters"
      className="pointer-events-auto w-[240px] space-y-3 rounded-lg border border-[var(--color-border)] bg-[var(--color-surface)]/95 p-3 text-[11px] shadow-lg backdrop-blur"
    >
      <div>
        <div className="mb-1.5 font-mono-data text-[9.5px] font-semibold uppercase tracking-wide text-[var(--color-text-muted)]">
          Severity
        </div>
        <div className="flex flex-wrap gap-1.5">
          {STATE_OPTIONS.map((opt) => {
            const active = filter.states.has(opt.key);
            return (
              <button
                key={opt.key}
                type="button"
                aria-pressed={active}
                onClick={() => toggleState(opt.key)}
                className={`rounded border px-2 py-1 text-[10.5px] transition ${
                  active
                    ? "border-[var(--color-accent)]/40 bg-[var(--color-accent)]/15 text-[var(--color-text)]"
                    : "border-[var(--color-border-strong)] text-[var(--color-text-muted)] hover:text-[var(--color-text)]"
                }`}
              >
                {opt.label}
              </button>
            );
          })}
        </div>
      </div>

      <label className="flex items-center gap-2 text-[10.5px] text-[var(--color-text-secondary)]">
        <input
          type="checkbox"
          checked={filter.dlqOnly}
          onChange={(e) => onChange({ ...filter, dlqOnly: e.target.checked })}
        />
        DLQ events only
      </label>

      {vendors.length > 0 && (
        <div>
          <div className="mb-1.5 font-mono-data text-[9.5px] font-semibold uppercase tracking-wide text-[var(--color-text-muted)]">
            Vendor
          </div>
          <div className="flex flex-wrap gap-1.5">
            {vendors.map((v) => {
              const active = filter.vendors.size === 0 || filter.vendors.has(v);
              return (
                <button
                  key={v}
                  type="button"
                  aria-pressed={active}
                  onClick={() => onChange({ ...filter, vendors: toggleSetMember(filter.vendors, v) })}
                  className={`rounded border px-2 py-1 font-mono-data text-[10px] transition ${
                    active
                      ? "border-[var(--color-accent)]/40 bg-[var(--color-accent)]/15 text-[var(--color-text)]"
                      : "border-[var(--color-border-strong)] text-[var(--color-text-muted)] hover:text-[var(--color-text)]"
                  }`}
                >
                  {v}
                </button>
              );
            })}
          </div>
        </div>
      )}

      {outcomes.length > 0 && (
        <div>
          <div className="mb-1.5 font-mono-data text-[9.5px] font-semibold uppercase tracking-wide text-[var(--color-text-muted)]">
            Outcome
          </div>
          <div className="flex flex-wrap gap-1.5">
            {outcomes.map((o) => {
              const active = filter.outcomes.size === 0 || filter.outcomes.has(o);
              return (
                <button
                  key={o}
                  type="button"
                  aria-pressed={active}
                  onClick={() => onChange({ ...filter, outcomes: toggleSetMember(filter.outcomes, o) })}
                  className={`rounded border px-2 py-1 font-mono-data text-[10px] transition ${
                    active
                      ? "border-[var(--color-accent)]/40 bg-[var(--color-accent)]/15 text-[var(--color-text)]"
                      : "border-[var(--color-border-strong)] text-[var(--color-text-muted)] hover:text-[var(--color-text)]"
                  }`}
                >
                  {o}
                </button>
              );
            })}
          </div>
        </div>
      )}

      {(filter.vendors.size > 0 || filter.outcomes.size > 0) && (
        <button
          type="button"
          onClick={() => onChange({ ...filter, vendors: new Set(), outcomes: new Set() })}
          className="text-[10px] text-[var(--color-text-muted)] underline-offset-2 hover:text-[var(--color-text)] hover:underline"
        >
          clear vendor/outcome filters
        </button>
      )}
    </div>
  );
}
