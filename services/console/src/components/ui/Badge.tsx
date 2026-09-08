type Tone = "success" | "danger" | "warning" | "neutral" | "accent";

const TONE_CLASSES: Record<Tone, string> = {
  success: "bg-[var(--color-success)]/15 text-[var(--color-success)] border-[var(--color-success)]/30",
  danger: "bg-[var(--color-danger)]/15 text-[var(--color-danger)] border-[var(--color-danger)]/30",
  warning: "bg-[var(--color-warning)]/15 text-[var(--color-warning)] border-[var(--color-warning)]/30",
  accent: "bg-[var(--color-accent)]/15 text-[var(--color-accent)] border-[var(--color-accent)]/30",
  neutral: "bg-white/5 text-[var(--color-text-secondary)] border-[var(--color-border-strong)]",
};

export function Badge({ children, tone = "neutral" }: { children: string; tone?: Tone }) {
  return (
    <span
      className={`inline-flex items-center rounded border px-1.5 py-0.5 text-[11px] font-medium font-mono-data leading-none ${TONE_CLASSES[tone]}`}
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
