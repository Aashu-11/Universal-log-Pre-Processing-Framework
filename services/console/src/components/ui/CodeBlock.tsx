export function CodeBlock({ children, wrap = false }: { children: string; wrap?: boolean }) {
  return (
    <pre
      className={`max-h-[480px] overflow-auto rounded border border-[var(--color-border)] bg-black/40 p-3 font-mono-data text-[12px] leading-relaxed text-[var(--color-text)] ${
        wrap ? "whitespace-pre-wrap break-words" : "whitespace-pre"
      }`}
    >
      {children}
    </pre>
  );
}
