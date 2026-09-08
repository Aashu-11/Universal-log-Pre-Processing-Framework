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
    <div className={`rounded-md border border-[var(--color-border)] bg-[var(--color-surface)] ${className}`}>
      {title && (
        <div className="flex items-center justify-between border-b border-[var(--color-border)] px-4 py-2.5">
          <h2 className="text-[13px] font-semibold tracking-wide text-[var(--color-text)]">{title}</h2>
          {action}
        </div>
      )}
      <div className="p-4">{children}</div>
    </div>
  );
}
