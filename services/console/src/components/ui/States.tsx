export function LoadingState({ label = "Loading…" }: { label?: string }) {
  return <div className="px-4 py-8 text-center text-[12px] text-[var(--color-text-muted)]">{label}</div>;
}

export function ErrorState({ message }: { message: string }) {
  return (
    <div className="rounded border border-[var(--color-danger)]/30 bg-[var(--color-danger)]/10 px-4 py-3 text-[12px] text-[var(--color-danger)]">
      {message}
    </div>
  );
}

export function EmptyState({ message }: { message: string }) {
  return <div className="px-4 py-8 text-center text-[12px] text-[var(--color-text-muted)]">{message}</div>;
}
