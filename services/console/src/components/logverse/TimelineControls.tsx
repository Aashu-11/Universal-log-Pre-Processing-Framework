import { PLAYBACK_SPEEDS, type PlaybackSpeed, type TimelineMode } from "../../lib/logverse/playback";
import type { ConnectionStatus } from "../../lib/logverse/types";

const STATUS_COPY: Record<ConnectionStatus, { label: string; dot: string }> = {
  connecting: { label: "Connecting…", dot: "bg-[var(--color-text-muted)]" },
  live: { label: "Live", dot: "bg-[var(--color-success)]" },
  reconnecting: { label: "Reconnecting…", dot: "bg-[var(--color-warning)]" },
  error: { label: "Unavailable", dot: "bg-[var(--color-danger)]" },
};

export function TimelineControls({
  mode,
  speed,
  clockMs,
  bounds,
  status,
  eps,
  visibleCount,
  overflowCount,
  droppedEventsTotal,
  dlqCount,
  reducedMotion,
  reducedMotionOverride,
  onReducedMotionOverrideChange,
  onGoLive,
  onPause,
  onPlayReplay,
  onSpeedChange,
  onScrub,
  onResetCamera,
}: {
  mode: TimelineMode;
  speed: PlaybackSpeed;
  clockMs: number;
  bounds: { minMs: number; maxMs: number };
  status: ConnectionStatus;
  eps: { eps: number; overSeconds: number } | null;
  visibleCount: number;
  overflowCount: number;
  droppedEventsTotal: number | null;
  dlqCount: number;
  reducedMotion: boolean;
  reducedMotionOverride: boolean;
  onReducedMotionOverrideChange: (v: boolean) => void;
  onGoLive: () => void;
  onPause: () => void;
  onPlayReplay: () => void;
  onSpeedChange: (s: PlaybackSpeed) => void;
  onScrub: (ms: number) => void;
  onResetCamera: () => void;
}) {
  const statusInfo = STATUS_COPY[status];
  const range = Math.max(1, bounds.maxMs - bounds.minMs);

  return (
    <div
      role="region"
      aria-label="Timeline controls"
      className="pointer-events-auto flex flex-wrap items-center gap-3 rounded-lg border border-[var(--color-border)] bg-[var(--color-surface)]/95 px-3 py-2.5 text-[11px] shadow-lg backdrop-blur"
    >
      <span className="flex items-center gap-1.5">
        <span className={`h-1.5 w-1.5 rounded-full ${statusInfo.dot}`} aria-hidden="true" />
        <span className="text-[var(--color-text-secondary)]">{statusInfo.label}</span>
      </span>

      <div className="h-4 w-px bg-[var(--color-border-strong)]" />

      <div className="flex items-center gap-1">
        <button
          type="button"
          aria-pressed={mode === "live"}
          onClick={onGoLive}
          className={`rounded border px-2.5 py-1 font-mono-data text-[10.5px] transition ${
            mode === "live"
              ? "border-[var(--color-accent)]/40 bg-[var(--color-accent)]/15 text-[var(--color-text)]"
              : "border-[var(--color-border-strong)] text-[var(--color-text-muted)] hover:text-[var(--color-text)]"
          }`}
        >
          ● Live
        </button>
        <button
          type="button"
          aria-pressed={mode === "paused"}
          onClick={onPause}
          className={`rounded border px-2.5 py-1 font-mono-data text-[10.5px] transition ${
            mode === "paused"
              ? "border-[var(--color-accent)]/40 bg-[var(--color-accent)]/15 text-[var(--color-text)]"
              : "border-[var(--color-border-strong)] text-[var(--color-text-muted)] hover:text-[var(--color-text)]"
          }`}
        >
          ‖ Pause
        </button>
        <button
          type="button"
          aria-pressed={mode === "replay"}
          onClick={onPlayReplay}
          className={`rounded border px-2.5 py-1 font-mono-data text-[10.5px] transition ${
            mode === "replay"
              ? "border-[var(--color-accent)]/40 bg-[var(--color-accent)]/15 text-[var(--color-text)]"
              : "border-[var(--color-border-strong)] text-[var(--color-text-muted)] hover:text-[var(--color-text)]"
          }`}
        >
          ▶ Replay
        </button>
      </div>

      <div className="flex items-center gap-1">
        <span className="text-[10px] text-[var(--color-text-muted)]">speed</span>
        {PLAYBACK_SPEEDS.map((s) => (
          <button
            key={s}
            type="button"
            aria-pressed={speed === s}
            onClick={() => onSpeedChange(s)}
            className={`rounded border px-1.5 py-0.5 font-mono-data text-[10px] transition ${
              speed === s
                ? "border-[var(--color-accent)]/40 bg-[var(--color-accent)]/15 text-[var(--color-text)]"
                : "border-[var(--color-border-strong)] text-[var(--color-text-muted)] hover:text-[var(--color-text)]"
            }`}
          >
            {s}×
          </button>
        ))}
      </div>

      <div className="flex min-w-[180px] flex-1 items-center gap-2">
        <label htmlFor="logverse-scrub" className="sr-only">
          Scrub timeline
        </label>
        <input
          id="logverse-scrub"
          type="range"
          min={bounds.minMs}
          max={bounds.maxMs}
          step={Math.max(1, Math.floor(range / 500))}
          value={Math.min(bounds.maxMs, Math.max(bounds.minMs, clockMs))}
          onChange={(e) => onScrub(Number(e.target.value))}
          className="w-full accent-[var(--color-accent)]"
          aria-valuetext={new Date(clockMs).toLocaleTimeString()}
        />
        <span className="w-[68px] shrink-0 font-mono-data text-[10px] text-[var(--color-text-muted)]">
          {new Date(clockMs).toLocaleTimeString([], { hour12: false })}
        </span>
      </div>

      <button
        type="button"
        onClick={onResetCamera}
        className="rounded border border-[var(--color-border-strong)] px-2.5 py-1 font-mono-data text-[10.5px] text-[var(--color-text-secondary)] transition hover:text-[var(--color-text)]"
      >
        Reset view
      </button>

      <label className="flex items-center gap-1.5 text-[10px] text-[var(--color-text-muted)]">
        <input
          type="checkbox"
          checked={reducedMotionOverride}
          onChange={(e) => onReducedMotionOverrideChange(e.target.checked)}
        />
        reduced motion{reducedMotion && !reducedMotionOverride ? " (from OS)" : ""}
      </label>

      <div className="h-4 w-px bg-[var(--color-border-strong)]" />

      <div className="flex items-center gap-3 font-mono-data text-[10px] text-[var(--color-text-muted)]">
        <span title="Derived from two counter samples of the collector's Prometheus metric, not a server-measured rate">
          {eps ? `${eps.eps.toFixed(1)} eps (derived, ${eps.overSeconds.toFixed(0)}s)` : "eps —"}
        </span>
        <span>{visibleCount} visible{overflowCount > 0 ? ` (+${overflowCount} aggregated, not individually rendered)` : ""}</span>
        <span title="Real ulpf_udp_drops_total from the collector — actual ingest-side packet drops, not a rendering limit">
          {droppedEventsTotal !== null ? droppedEventsTotal.toLocaleString() : "—"} dropped
        </span>
        <span className="text-[var(--color-warning)]">{dlqCount} DLQ</span>
      </div>
    </div>
  );
}
