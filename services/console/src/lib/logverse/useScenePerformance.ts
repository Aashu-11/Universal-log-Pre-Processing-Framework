import { useEffect, useState } from "react";

import { MAX_VISIBLE_PARTICLES } from "./data";

export interface ScenePerformance {
  /** prefers-reduced-motion: reduce (OR'd with the in-app manual override
   * below) — the scene should drop camera auto-drift and easing/trail
   * effects, but stays interactive. */
  reducedMotion: boolean;
  reducedMotionOverride: boolean;
  setReducedMotionOverride: (v: boolean) => void;
  /** document.hidden — the render loop should stop entirely while true
   * (performance requirement: pause rendering when the tab is hidden). */
  tabHidden: boolean;
  maxVisibleParticles: number;
}

export function useScenePerformance(): ScenePerformance {
  const [osReducedMotion, setOsReducedMotion] = useState<boolean>(
    () => typeof window !== "undefined" && window.matchMedia?.("(prefers-reduced-motion: reduce)").matches === true,
  );
  const [reducedMotionOverride, setReducedMotionOverride] = useState(false);
  const [tabHidden, setTabHidden] = useState<boolean>(() => typeof document !== "undefined" && document.hidden);

  useEffect(() => {
    if (typeof window === "undefined" || !window.matchMedia) return;
    const mq = window.matchMedia("(prefers-reduced-motion: reduce)");
    const onChange = () => setOsReducedMotion(mq.matches);
    mq.addEventListener("change", onChange);
    return () => mq.removeEventListener("change", onChange);
  }, []);

  useEffect(() => {
    if (typeof document === "undefined") return;
    const onVis = () => setTabHidden(document.hidden);
    document.addEventListener("visibilitychange", onVis);
    return () => document.removeEventListener("visibilitychange", onVis);
  }, []);

  return {
    reducedMotion: osReducedMotion || reducedMotionOverride,
    reducedMotionOverride,
    setReducedMotionOverride,
    tabHidden,
    maxVisibleParticles: MAX_VISIBLE_PARTICLES,
  };
}
