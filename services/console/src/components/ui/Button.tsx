import type { ButtonHTMLAttributes } from "react";

type Variant = "primary" | "secondary" | "danger" | "ghost";

const VARIANT_CLASSES: Record<Variant, string> = {
  primary: "neo-button border border-[#a18dff]/70 bg-[linear-gradient(135deg,#7962d9,#4c78cf)] text-white shadow-[7px_7px_16px_rgba(3,7,14,.35),inset_1px_1px_rgba(255,255,255,.18)] hover:brightness-110",
  secondary:
    "neo-button bg-white/[0.035] text-[var(--color-text)] border border-[var(--color-border-strong)] hover:border-white/30 hover:bg-white/[0.06]",
  danger: "border border-[var(--color-danger)]/60 bg-[var(--color-danger)]/15 text-[var(--color-danger)] hover:bg-[var(--color-danger)]/25",
  ghost: "bg-transparent text-[var(--color-text-secondary)] hover:text-[var(--color-text)] hover:bg-white/5",
};

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant;
}

export function Button({ variant = "secondary", className = "", disabled, ...rest }: ButtonProps) {
  return (
    <button
      disabled={disabled}
      className={`inline-flex min-h-10 items-center justify-center gap-1.5 rounded-xl px-4 py-2 text-[13px] font-semibold tracking-[0.01em] transition-all duration-200 disabled:cursor-not-allowed disabled:opacity-40 ${VARIANT_CLASSES[variant]} ${className}`}
      {...rest}
    />
  );
}
