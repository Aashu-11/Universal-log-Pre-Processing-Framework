export function StatTile({
  label,
  value,
  sublabel,
  tone = "default",
  trend,
}: {
  label: string;
  value: string;
  sublabel?: string;
  tone?: "default" | "success" | "danger" | "warning";
  trend?: string;
}) {
  const valueColor = {
    default: "text-[var(--color-text)]",
    success: "text-[var(--color-success)]",
    danger: "text-[var(--color-danger)]",
    warning: "text-[var(--color-warning)]",
  }[tone];

  return (
    <div className="security-panel group min-h-[118px] rounded-xl border border-[var(--color-border)] bg-[linear-gradient(145deg,var(--color-surface-raised),var(--color-surface))] px-4 py-3.5 transition-all duration-300 hover:-translate-y-0.5 hover:border-[var(--color-border-strong)]">
      <div className="flex items-start justify-between gap-3">
        <div className="eyebrow">{label}</div>
        <span className={`mt-0.5 h-1.5 w-1.5 rounded-full ${tone === "danger" ? "bg-[var(--color-danger)]" : tone === "warning" ? "bg-[var(--color-warning)]" : "bg-[var(--color-accent)]"}`} />
      </div>
      <div className={`mt-3 font-mono-data text-[28px] font-medium leading-none tracking-[-0.045em] ${valueColor}`}>{value}</div>
      <div className="mt-3 flex items-center justify-between gap-2">
        {sublabel && <div className="truncate text-[10.5px] text-[var(--color-text-muted)]">{sublabel}</div>}
        {trend && <div className="font-mono-data text-[10px] text-[var(--color-success)]">{trend}</div>}
      </div>
    </div>
  );
}
