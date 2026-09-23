// Pure DOM — no canvas dependency, so it stays legible (and screen-reader
// reachable) even when WebGLFallback is showing instead of the 3D scene.
// Every row pairs a color with a distinct shape word and a plain-language
// meaning, so the state reads correctly under color-vision deficiency too.
const ROWS: { swatch: string; shape: string; label: string; meaning: string }[] = [
  { swatch: "bg-[var(--color-cyan)]", shape: "sphere", label: "Normal", meaning: "Processed cleanly, unremarkable risk" },
  { swatch: "bg-[var(--color-warning)]", shape: "haloed sphere", label: "Elevated", meaning: "Risk score ≥ 40, no confirmed threat" },
  { swatch: "bg-[var(--color-danger)]", shape: "spiked polyhedron", label: "Threat / IOC match", meaning: "Confirmed IOC match or a populated threat category" },
  { swatch: "bg-[#b06bf0]", shape: "tilted cube", label: "DLQ", meaning: "Failed validation, routed to the Dead Letter Queue" },
  { swatch: "bg-[var(--color-success)]", shape: "solid block", label: "Integrity verified", meaning: "SHA-256 + Merkle proof both checked out" },
  { swatch: "bg-[var(--color-danger)]", shape: "fractured block", label: "Integrity FAILED", meaning: "Raw bytes or chain verification did not match" },
];

export function SceneLegend({ mode = "pipeline" }: { mode?: "nexus" | "pipeline" }) {
  return (
    <div
      role="region"
      aria-label="Scene legend"
      className="pointer-events-auto max-w-[240px] rounded-lg border border-[var(--color-border)] bg-[var(--color-surface)]/95 p-3 text-[11px] shadow-lg backdrop-blur"
    >
      <div className="mb-2 font-mono-data text-[9.5px] font-semibold uppercase tracking-wide text-[var(--color-text-muted)]">
        {mode === "nexus" ? "Nexus graph — color + shape" : "Legend — color + shape"}
      </div>
      {mode === "nexus" && <p className="mb-2 text-[10px] text-[var(--color-text-muted)]">Violet icosahedron: vendor · blue octahedron: observed IP · colored sphere: event. Links show fields present in an event, not inferred network topology.</p>}
      <ul className="space-y-1.5">
        {ROWS.map((r) => (
          <li key={r.label} className="flex items-start gap-2">
            <span className={`mt-0.5 h-2.5 w-2.5 shrink-0 rounded-sm ${r.swatch}`} aria-hidden="true" />
            <span>
              <span className="font-medium text-[var(--color-text)]">{r.label}</span>{" "}
              <span className="text-[var(--color-text-muted)]">({r.shape})</span>
              <div className="text-[10px] text-[var(--color-text-muted)]">{r.meaning}</div>
            </span>
          </li>
        ))}
      </ul>
    </div>
  );
}
