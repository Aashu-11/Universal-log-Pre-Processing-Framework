import type { ReactNode } from "react";

export function Card({
  title,
  action,
  children,
  className = "",
}: {
  title?: string;
  action?: ReactNode;
  children: ReactNode;
  className?: string;
}) {
  return (
    <section className={`security-panel rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)] ${className}`}>
      {title && (
        <div className="flex min-h-12 items-center justify-between border-b border-[var(--color-border)] px-4 py-3">
          <div className="flex items-center gap-2.5">
            <span className="h-1.5 w-1.5 rounded-full bg-[var(--color-accent)] shadow-[0_0_8px_var(--color-accent)]" />
            <h2 className="text-[12px] font-semibold tracking-[0.04em] text-[var(--color-text)]">{title}</h2>
          </div>
          {action}
        </div>
      )}
      <div className="p-4">{children}</div>
    </section>
  );
}
