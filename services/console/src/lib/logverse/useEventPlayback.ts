import { useCallback, useEffect, useRef, useState } from "react";

import {
  PLAYBACK_SPEEDS,
  advanceClock,
  clampToBounds,
  timelineBounds,
  type PlaybackSpeed,
  type TimelineBounds,
  type TimelineMode,
} from "./playback";
import type { LogVerseEvent } from "./types";

const DOM_SYNC_MS = 100; // React-state sync cadence for DOM consumers (scrubber) — see clockMsRef below

export interface EventPlayback {
  mode: TimelineMode;
  speed: PlaybackSpeed;
  /** React state, throttled to ~10Hz — safe for DOM display (the scrub
   * slider), NOT for driving per-frame 3D motion (would re-render the
   * whole tree at 60fps). */
  clockMs: number;
  /** The same clock, updated every animation frame, but as a mutable ref
   * — the 3D particle system reads clockMsRef.current inside its own
   * useFrame, getting smooth 60fps motion with zero React re-renders. */
  clockMsRef: React.RefObject<number>;
  bounds: TimelineBounds;
  goLive: () => void;
  pause: () => void;
  playReplay: () => void;
  setSpeed: (s: PlaybackSpeed) => void;
  scrubTo: (ms: number) => void;
}

/** Drives one shared timeline clock for the whole scene — every particle
 * reads its position from the SAME clock via playback.progressForEvent,
 * which is what makes scrubbing/replay deterministic (no per-particle
 * independent clock drift). Live mode tracks real wall-clock time; replay
 * mode advances the clock itself at `speed`x via requestAnimationFrame,
 * bounded to the currently-buffered event window. */
export function useEventPlayback(events: LogVerseEvent[]): EventPlayback {
  const [mode, setMode] = useState<TimelineMode>("live");
  const [speed, setSpeedState] = useState<PlaybackSpeed>(1);
  const [clockMs, setClockMsState] = useState<number>(() => Date.now());

  const clockMsRef = useRef<number>(clockMs);
  // "Latest value" refs for the rAF loop below to read without restarting
  // itself on every mode/speed/events change — synced via effects (not
  // during render) so React 19's Strict Mode double-render can't leave one
  // out of sync with what actually committed.
  const modeRef = useRef(mode);
  const speedRef = useRef(speed);
  const eventsRef = useRef(events);
  useEffect(() => {
    modeRef.current = mode;
  }, [mode]);
  useEffect(() => {
    speedRef.current = speed;
  }, [speed]);
  useEffect(() => {
    eventsRef.current = events;
  }, [events]);
  const rafRef = useRef<number | null>(null);
  const lastFrameRef = useRef<number | null>(null);
  const lastDomSyncRef = useRef<number>(0);

  // clockMs (React state) already tracks Date.now() closely in live mode —
  // synced at DOM_SYNC_MS cadence by setClock below — so bounds can read it
  // directly instead of calling the impure Date.now() during render.
  const bounds = timelineBounds(events, clockMs);

  const setClock = useCallback((next: number, forceDomSync = false) => {
    clockMsRef.current = next;
    const now = performance.now();
    if (forceDomSync || now - lastDomSyncRef.current >= DOM_SYNC_MS) {
      lastDomSyncRef.current = now;
      setClockMsState(next);
    }
  }, []);

  useEffect(() => {
    const tick = (now: number) => {
      const last = lastFrameRef.current ?? now;
      const realDeltaMs = now - last;
      lastFrameRef.current = now;

      if (modeRef.current === "live") {
        setClock(Date.now());
      } else if (modeRef.current === "replay") {
        const b = timelineBounds(eventsRef.current, clockMsRef.current);
        const { clockMs: next, reachedEnd } = advanceClock(clockMsRef.current, realDeltaMs, speedRef.current, b);
        setClock(next);
        if (reachedEnd) {
          // Auto-stop at the end of the buffered window rather than
          // looping silently — looping would imply more data exists than
          // actually does.
          setMode("paused");
        }
      }
      rafRef.current = requestAnimationFrame(tick);
    };

    if (mode === "live" || mode === "replay") {
      lastFrameRef.current = null;
      rafRef.current = requestAnimationFrame(tick);
    }
    return () => {
      if (rafRef.current !== null) cancelAnimationFrame(rafRef.current);
      rafRef.current = null;
    };
  }, [mode, setClock]);

  const goLive = useCallback(() => {
    setMode("live");
    setClock(Date.now(), true);
  }, [setClock]);

  const pause = useCallback(() => setMode("paused"), []);

  const playReplay = useCallback(() => setMode("replay"), []);

  const setSpeed = useCallback((s: PlaybackSpeed) => {
    if (!PLAYBACK_SPEEDS.includes(s)) return;
    setSpeedState(s);
  }, []);

  const scrubTo = useCallback(
    (ms: number) => {
      setMode("paused");
      setClock(clampToBounds(ms, timelineBounds(eventsRef.current, ms)), true);
    },
    [setClock],
  );

  return { mode, speed, clockMs, clockMsRef, bounds, goLive, pause, playReplay, setSpeed, scrubTo };
}
