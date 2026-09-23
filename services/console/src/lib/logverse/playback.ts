// Timeline / forensic-replay math. Kept pure and separate from the R3F
// scene and from the React hook that drives it (useEventPlayback.ts) so
// particle positioning is unit-testable without a WebGL context.
import { PIPELINE_STAGES, type LogVerseEvent } from "./types";

// A single event's visual journey across all 8 stages takes this long on
// screen. This is a UI pacing constant, NOT a measured per-stage latency —
// the API only exposes lineage_processing_ms (one total parse duration) and
// event_observed_at/event_ingested_at (two absolute timestamps), never a
// per-stage breakdown. Every place this constant reaches the UI must label
// it as an estimate (see TimelineControls/SceneLegend copy) per CLAUDE.md's
// visual-truthfulness rule. Mirrors Live Theater's existing STAGE_STEP_MS
// convention (320ms/stage) at a slightly slower, more "flyable" pace.
export const STAGE_VISUAL_DURATION_MS = 360;
export const JOURNEY_VISUAL_DURATION_MS = STAGE_VISUAL_DURATION_MS * PIPELINE_STAGES.length;

export type TimelineMode = "live" | "paused" | "replay";
export const PLAYBACK_SPEEDS = [0.5, 1, 2, 4] as const;
export type PlaybackSpeed = (typeof PLAYBACK_SPEEDS)[number];

/** 0 = just spawned (still at INGEST), 1 = fully arrived at ROUTE. Clamped,
 * deterministic: the same (event, clockMs) pair always returns the same
 * number, which is what makes scrubbing reproducible. */
export function progressForEvent(event: LogVerseEvent, clockMs: number): number {
  const elapsed = clockMs - event.ingestedAtMs;
  if (elapsed <= 0) return 0;
  if (elapsed >= JOURNEY_VISUAL_DURATION_MS) return 1;
  return elapsed / JOURNEY_VISUAL_DURATION_MS;
}

export function stageIndexForProgress(progress: number): number {
  const clamped = Math.min(1, Math.max(0, progress));
  return Math.min(PIPELINE_STAGES.length - 1, Math.floor(clamped * PIPELINE_STAGES.length));
}

export function stageForProgress(progress: number) {
  return PIPELINE_STAGES[stageIndexForProgress(progress)];
}

/** true once an event has fully finished its visual journey at clockMs —
 * used to retire particles instead of instancing them forever. */
export function hasArrived(event: LogVerseEvent, clockMs: number): boolean {
  return clockMs - event.ingestedAtMs >= JOURNEY_VISUAL_DURATION_MS;
}

export interface TimelineBounds {
  minMs: number;
  maxMs: number;
}

/** The scrubbable range: from the oldest buffered event to the newest
 * event's full arrival. Falls back to a 1-minute window ending now when the
 * buffer is empty, so the scrubber never divides by zero. */
export function timelineBounds(events: LogVerseEvent[], nowMs: number): TimelineBounds {
  if (events.length === 0) {
    return { minMs: nowMs - 60_000, maxMs: nowMs };
  }
  let minMs = Infinity;
  let maxMs = -Infinity;
  for (const e of events) {
    if (e.ingestedAtMs < minMs) minMs = e.ingestedAtMs;
    if (e.ingestedAtMs > maxMs) maxMs = e.ingestedAtMs;
  }
  return { minMs, maxMs: maxMs + JOURNEY_VISUAL_DURATION_MS };
}

export function clampToBounds(clockMs: number, bounds: TimelineBounds): number {
  return Math.min(bounds.maxMs, Math.max(bounds.minMs, clockMs));
}

/** Advances the replay clock by one animation-frame tick. Pure so the
 * stepping math (speed multiplier, bounds clamping, auto-stop at the end)
 * is unit-testable without faking requestAnimationFrame. */
export function advanceClock(
  clockMs: number,
  realDeltaMs: number,
  speed: number,
  bounds: TimelineBounds,
): { clockMs: number; reachedEnd: boolean } {
  const next = clockMs + realDeltaMs * speed;
  if (next >= bounds.maxMs) {
    return { clockMs: bounds.maxMs, reachedEnd: true };
  }
  return { clockMs: Math.max(bounds.minMs, next), reachedEnd: false };
}
