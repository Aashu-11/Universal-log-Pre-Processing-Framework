import { describe, expect, it } from "vitest";

import {
  JOURNEY_VISUAL_DURATION_MS,
  advanceClock,
  clampToBounds,
  hasArrived,
  progressForEvent,
  stageForProgress,
  stageIndexForProgress,
  timelineBounds,
} from "../playback";
import { rowToLogVerseEvent } from "../data";
import type { LogVerseEvent } from "../types";

function ev(ingestedAtMs: number): LogVerseEvent {
  return { ...rowToLogVerseEvent(new Array(31).fill(null)), eventId: "e", ingestedAtMs };
}

describe("progressForEvent", () => {
  it("is 0 the instant an event is ingested", () => {
    expect(progressForEvent(ev(1000), 1000)).toBe(0);
  });

  it("is 0 for a clock before ingestion (event hasn't spawned yet)", () => {
    expect(progressForEvent(ev(1000), 500)).toBe(0);
  });

  it("is 1 once the full visual journey has elapsed", () => {
    expect(progressForEvent(ev(0), JOURNEY_VISUAL_DURATION_MS)).toBe(1);
    expect(progressForEvent(ev(0), JOURNEY_VISUAL_DURATION_MS * 10)).toBe(1); // clamped, never > 1
  });

  it("is deterministic: same event + same clock always gives the same progress", () => {
    const e = ev(1_000_000);
    const a = progressForEvent(e, 1_000_500);
    const b = progressForEvent(e, 1_000_500);
    expect(a).toBe(b);
  });

  it("is linear halfway through the journey", () => {
    const half = JOURNEY_VISUAL_DURATION_MS / 2;
    expect(progressForEvent(ev(0), half)).toBeCloseTo(0.5, 5);
  });
});

describe("stageIndexForProgress / stageForProgress", () => {
  it("maps progress 0 to the first stage (ingest)", () => {
    expect(stageIndexForProgress(0)).toBe(0);
    expect(stageForProgress(0)).toBe("ingest");
  });

  it("maps progress 1 to the last stage (route), not an out-of-range index", () => {
    expect(stageIndexForProgress(1)).toBe(7);
    expect(stageForProgress(1)).toBe("route");
  });

  it("clamps out-of-range progress values", () => {
    expect(stageIndexForProgress(-0.5)).toBe(0);
    expect(stageIndexForProgress(1.5)).toBe(7);
  });
});

describe("hasArrived", () => {
  it("is false mid-journey and true once the journey duration has elapsed", () => {
    const e = ev(0);
    expect(hasArrived(e, JOURNEY_VISUAL_DURATION_MS / 2)).toBe(false);
    expect(hasArrived(e, JOURNEY_VISUAL_DURATION_MS)).toBe(true);
  });
});

describe("timelineBounds", () => {
  it("falls back to a 1-minute window ending now when there are no events", () => {
    const bounds = timelineBounds([], 100_000);
    expect(bounds).toEqual({ minMs: 40_000, maxMs: 100_000 });
  });

  it("spans from the oldest event to the newest event's full arrival", () => {
    const events = [ev(1000), ev(5000), ev(2000)];
    const bounds = timelineBounds(events, 999_999);
    expect(bounds.minMs).toBe(1000);
    expect(bounds.maxMs).toBe(5000 + JOURNEY_VISUAL_DURATION_MS);
  });
});

describe("clampToBounds", () => {
  it("clamps below min and above max, passes through in range", () => {
    const bounds = { minMs: 100, maxMs: 200 };
    expect(clampToBounds(50, bounds)).toBe(100);
    expect(clampToBounds(250, bounds)).toBe(200);
    expect(clampToBounds(150, bounds)).toBe(150);
  });
});

describe("advanceClock", () => {
  const bounds = { minMs: 0, maxMs: 1000 };

  it("advances by realDeltaMs * speed", () => {
    const result = advanceClock(100, 50, 2, bounds);
    expect(result).toEqual({ clockMs: 200, reachedEnd: false });
  });

  it("stops exactly at the bound and reports reachedEnd instead of overshooting", () => {
    const result = advanceClock(950, 100, 1, bounds);
    expect(result).toEqual({ clockMs: 1000, reachedEnd: true });
  });

  it("never advances below the lower bound", () => {
    const result = advanceClock(0, -100, 1, bounds);
    expect(result.clockMs).toBeGreaterThanOrEqual(bounds.minMs);
  });
});
