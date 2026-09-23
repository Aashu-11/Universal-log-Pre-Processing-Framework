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
    <section className={`security-panel rounded-2xl border border-white/[0.07] bg-[linear-gradient(145deg,#1a2331,#111822)] ${className}`}>
      {title && (
        <div className="flex min-h-14 items-center justify-between border-b border-white/[0.06] px-5 py-3.5">
          <div className="flex items-center gap-2.5">
            <span className="h-2 w-2 rounded-full bg-[var(--color-accent)] shadow-[0_0_10px_var(--color-accent)]" />
            <h2 className="text-[14px] font-semibold tracking-[0.01em] text-[var(--color-text)]">{title}</h2>
          </div>
          {action}
        </div>
      )}
      <div className="p-5">{children}</div>
    </section>
  );
}
