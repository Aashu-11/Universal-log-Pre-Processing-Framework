import type { ButtonHTMLAttributes } from "react";

type Variant = "primary" | "secondary" | "danger" | "ghost";

const VARIANT_CLASSES: Record<Variant, string> = {
  primary: "border border-[#9c79ff] bg-[linear-gradient(115deg,#7950e8,#3978ee)] text-white shadow-[0_8px_24px_rgba(86,63,200,0.22)] hover:brightness-110",
  secondary:
    "bg-white/[0.025] text-[var(--color-text)] border border-[var(--color-border-strong)] hover:border-white/30 hover:bg-white/[0.06]",
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
      className={`inline-flex min-h-8 items-center justify-center gap-1.5 rounded-lg px-3.5 py-1.5 text-[11.5px] font-semibold tracking-[0.02em] transition-all duration-200 disabled:cursor-not-allowed disabled:opacity-40 ${VARIANT_CLASSES[variant]} ${className}`}
      {...rest}
    />
  );
}
