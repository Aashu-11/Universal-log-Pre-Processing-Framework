type Tone = "success" | "danger" | "warning" | "neutral" | "accent";

const TONE_CLASSES: Record<Tone, string> = {
  success: "bg-[var(--color-success)]/10 text-[var(--color-success)] border-[var(--color-success)]/25",
  danger: "bg-[var(--color-danger)]/10 text-[var(--color-danger)] border-[var(--color-danger)]/25",
  warning: "bg-[var(--color-warning)]/10 text-[var(--color-warning)] border-[var(--color-warning)]/25",
  accent: "bg-[var(--color-accent)]/12 text-[#b9a2ff] border-[var(--color-accent)]/30",
  neutral: "bg-white/5 text-[var(--color-text-secondary)] border-[var(--color-border-strong)]",
};

export function Badge({ children, tone = "neutral" }: { children: string; tone?: Tone }) {
  return (
    <span
      className={`inline-flex items-center rounded-md border px-2 py-1 text-[9px] font-semibold uppercase tracking-[0.1em] font-mono-data leading-none ${TONE_CLASSES[tone]}`}
    >
      {children}
    </span>
  );
}

export function statusTone(status: string): Tone {
  const s = status.toLowerCase();
  if (["ok", "pass", "passed", "active", "success", "allowed", "published"].includes(s)) return "success";
  if (["fail", "failed", "error", "blocked", "dropped"].includes(s)) return "danger";
  if (["partial", "warning", "unknown", "rolled_back"].includes(s)) return "warning";
  return "neutral";
}
