export function StatTile({
  label,
  value,
  sublabel,
  tone = "default",
}: {
  label: string;
  value: string;
  sublabel?: string;
  tone?: "default" | "success" | "danger" | "warning";
}) {
  const valueColor = {
    default: "text-[var(--color-text)]",
    success: "text-[var(--color-success)]",
    danger: "text-[var(--color-danger)]",
    warning: "text-[var(--color-warning)]",
  }[tone];

  return (
    <div className="rounded-md border border-[var(--color-border)] bg-[var(--color-surface)] px-4 py-3">
      <div className="text-[11px] uppercase tracking-wider text-[var(--color-text-muted)]">{label}</div>
      <div className={`mt-1 font-mono-data text-2xl font-semibold ${valueColor}`}>{value}</div>
      {sublabel && <div className="mt-0.5 text-[11px] text-[var(--color-text-secondary)]">{sublabel}</div>}
    </div>
  );
}
