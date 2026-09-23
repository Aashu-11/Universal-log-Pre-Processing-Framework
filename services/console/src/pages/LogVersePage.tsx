import { useMemo, useRef, useState } from "react";

import { controlPlane, ApiError } from "../lib/api";
import { LoadingState } from "../components/ui/States";
import {
  deriveInternalAssets,
  selectVisibleParticles,
} from "../lib/logverse/data";
import { useLogVerseData } from "../lib/logverse/useLogVerseData";
import { useEventPlayback } from "../lib/logverse/useEventPlayback";
import { useScenePerformance } from "../lib/logverse/useScenePerformance";
import { isWebglAvailable } from "../lib/logverse/webgl";
import { defaultFilter, matchesFilter, type LogVerseEvent, type PipelineStage, type SourceNode, type VaultBlock } from "../lib/logverse/types";
import type { IntegrityState } from "../components/logverse/VaultChain";
import { LogVerseCanvas, type LogVerseCanvasHandle } from "../components/logverse/LogVerseCanvas";
import { NexusGraph, type NexusGraphHandle } from "../components/logverse/NexusGraph";
import { WebGLFallback } from "../components/logverse/WebGLFallback";
import { SceneLegend } from "../components/logverse/SceneLegend";
import { LogVerseFilters } from "../components/logverse/LogVerseFilters";
import { TimelineControls } from "../components/logverse/TimelineControls";
import { EventInspector } from "../components/logverse/EventInspector";
import { NexusAnalysis } from "../components/logverse/NexusAnalysis";
import { NexusEventList } from "../components/logverse/NexusEventList";
import type { NexusNode } from "../lib/logverse/nexus";

export function LogVersePage() {
  const data = useLogVerseData();
  const filteredEvents = useFilteredEvents(data.events, data.dlqEvents);
  const playback = useEventPlayback([...filteredEvents.events, ...filteredEvents.dlqEvents]);
  const perf = useScenePerformance();

  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [selectedNode, setSelectedNode] = useState<NexusNode | null>(null);
  const [hoveredSource, setHoveredSource] = useState<SourceNode | null>(null);
  const [hoveredVault, setHoveredVault] = useState<VaultBlock | null>(null);
  const [highlightedStage, setHighlightedStage] = useState<PipelineStage | null>(null);
  const [canvasLost, setCanvasLost] = useState(false);
  const [integrityState, setIntegrityState] = useState<IntegrityState>("unknown");
  const [integrityChecking, setIntegrityChecking] = useState(false);
  const [integrityError, setIntegrityError] = useState<string | null>(null);
  const [sceneMode, setSceneMode] = useState<"nexus" | "pipeline">("nexus");

  const canvasRef = useRef<LogVerseCanvasHandle>(null);
  const nexusRef = useRef<NexusGraphHandle>(null);
  const webglOk = useMemo(() => isWebglAvailable(), []);

  const combined = useMemo(
    () => [...filteredEvents.events, ...filteredEvents.dlqEvents],
    [filteredEvents.events, filteredEvents.dlqEvents],
  );
  const { visible, overflow } = useMemo(
    () => selectVisibleParticles(combined, perf.maxVisibleParticles),
    [combined, perf.maxVisibleParticles],
  );
  const assets = useMemo(() => deriveInternalAssets(data.events), [data.events]);

  const selectedEvent = visible.find((e) => e.eventId === selectedId) ?? combined.find((e) => e.eventId === selectedId) ?? null;

  const vendors = useMemo(() => Array.from(new Set(data.events.map((e) => e.vendor).filter(Boolean))).sort(), [data.events]);
  const outcomes = useMemo(
    () => Array.from(new Set(data.events.map((e) => e.eventAction || "unknown"))).sort(),
    [data.events],
  );

  const onSelect = (ev: LogVerseEvent) => {
    setSelectedId(ev.eventId);
    playback.pause();
  };

  const onGraphNodeSelect = (node: NexusNode) => {
    setSelectedNode(node);
    if (!node.event) setSelectedId(null);
  };

  const resetView = () => {
    canvasRef.current?.resetCamera();
    nexusRef.current?.resetCamera();
    playback.goLive();
  };

  const runIntegrityVerify = async () => {
    setIntegrityChecking(true);
    setIntegrityError(null);
    try {
      const resp = await controlPlane.get<{ passed: boolean; output: string }>("/v1/integrity/verify");
      setIntegrityState(resp.passed ? "verified" : "failed");
      if (!resp.passed) setIntegrityError(resp.output);
    } catch (err) {
      setIntegrityState("unknown");
      setIntegrityError(err instanceof ApiError ? err.message : "integrity check unavailable");
    } finally {
      setIntegrityChecking(false);
    }
  };

  const hasEvents = data.events.length > 0 || data.dlqEvents.length > 0;
  const showEmptyState = data.status === "live" && !hasEvents;

  return (
    <div className="flex h-screen min-h-[560px] flex-col gap-5 bg-[var(--color-bg)] p-5 md:p-7">
      <div className="neo-card flex min-h-[82px] items-center justify-between rounded-3xl border border-[var(--color-border)] bg-[var(--color-surface)] px-6 md:px-8">
        <div>
          <h1 className="text-[22px] font-semibold tracking-tight text-[var(--color-text)]">◈ LogVerse <span className="font-normal text-[var(--color-accent-hover)]">Nexus</span></h1>
          <p className="mt-1 text-[13px] text-[var(--color-text-muted)]">
            3D forensic intelligence · real log events · traceable evidence
          </p>
        </div>
        <div className="flex items-center gap-2">
        <div className="neo-inset flex rounded-xl border border-[var(--color-border)] p-1" aria-label="Visualization mode">
          <button type="button" onClick={() => setSceneMode("nexus")} aria-pressed={sceneMode === "nexus"} className={`rounded-lg px-4 py-2 text-[12px] font-medium ${sceneMode === "nexus" ? "neo-button bg-[var(--color-accent)] text-white" : "text-[var(--color-text-secondary)]"}`}>Nexus graph</button>
          <button type="button" onClick={() => setSceneMode("pipeline")} aria-pressed={sceneMode === "pipeline"} className={`rounded-lg px-4 py-2 text-[12px] font-medium ${sceneMode === "pipeline" ? "neo-button bg-[var(--color-accent)] text-white" : "text-[var(--color-text-secondary)]"}`}>Pipeline</button>
        </div>
        <button
          type="button"
          onClick={runIntegrityVerify}
          disabled={integrityChecking}
          className="neo-button rounded-xl border border-[var(--color-border)] bg-white/[0.025] px-4 py-2 text-[12px] font-medium text-[var(--color-text-secondary)] hover:border-white/30 hover:bg-white/[0.06] disabled:opacity-50"
        >
          {integrityChecking ? "Verifying vault chain…" : "Verify vault chain"}
        </button>
        </div>
      </div>

      {integrityError && (
        <div className="rounded border border-[var(--color-danger)]/30 bg-[var(--color-danger)]/10 px-3 py-2 text-[11px] text-[var(--color-danger)]">
          {integrityError}
        </div>
      )}

      {data.status === "connecting" && data.events.length === 0 && (
        <LoadingState label="Connecting to the pipeline — the first load can take up to ~20s on a lightly-resourced dev stack (it queries the full lake once, then stays fast)…" />
      )}

      {data.status === "error" && data.events.length === 0 && (
        <div className="rounded-lg border border-[var(--color-danger)]/30 bg-[var(--color-danger)]/10 p-4 text-[12px] text-[var(--color-danger)]">
          <div className="font-semibold">API unavailable</div>
          <div className="mt-1 text-[var(--color-text-secondary)]">{data.errors.events}</div>
        </div>
      )}

      {showEmptyState && (
        <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-surface)] p-4 text-[12px] text-[var(--color-text-secondary)]">
          No sources registered and no events in the lake yet. Register a source and send traffic through the
          collector — this scene picks it up automatically within a few seconds, same as every other console page.
        </div>
      )}

      {!showEmptyState && hasEvents && (
        <div className="neo-card relative min-h-0 flex-1 overflow-hidden rounded-3xl border border-[var(--color-border)] bg-black">
          {webglOk && !canvasLost && sceneMode === "nexus" && (
            <NexusGraph ref={nexusRef} events={visible} selectedId={selectedId} onSelect={onSelect} onNodeSelect={onGraphNodeSelect} reducedMotion={perf.reducedMotion} paused={perf.tabHidden} />
          )}
          {webglOk && !canvasLost && sceneMode === "pipeline" && (
            <LogVerseCanvas
              ref={canvasRef}
              sources={data.sources}
              events={visible}
              assets={assets}
              vaultChain={data.vaultChain}
              integrityState={integrityState}
              dlqCount={filteredEvents.dlqEvents.length}
              clockMsRef={playback.clockMsRef}
              selectedId={selectedId}
              highlightedStage={highlightedStage}
              reducedMotion={perf.reducedMotion}
              paused={perf.tabHidden}
              onSelect={onSelect}
              onSourceHover={setHoveredSource}
              onVaultHover={setHoveredVault}
              onContextLost={() => setCanvasLost(true)}
              onContextRestored={() => setCanvasLost(false)}
            />
          )}

          {(!webglOk || canvasLost) && (
            <div className="h-full overflow-y-auto p-4">
              <WebGLFallback sources={data.sources} events={combined} onSelect={onSelect} />
              {canvasLost && (
                <p className="mt-3 text-[11px] text-[var(--color-warning)]">
                  The 3D renderer lost its GPU context (common after a laptop sleep/wake or driver reset). Reload the
                  page to try again; your data above is unaffected.
                </p>
              )}
            </div>
          )}

          <div className="pointer-events-none absolute inset-0 flex flex-col justify-between gap-3 p-4">
            <div className="flex min-h-0 flex-1 items-start justify-between gap-3">
              <aside className="pointer-events-auto flex max-h-full w-[350px] shrink-0 flex-col gap-3 overflow-y-auto pr-1" aria-label="Nexus analysis sidebar">
                {selectedNode && <NexusEventList node={selectedNode} events={combined} selectedId={selectedId} onPick={onSelect} />}
                {selectedEvent && <div className="h-[440px] min-h-[440px]"><EventInspector key={selectedEvent.eventId} event={selectedEvent} autoTrace recentEvents={visible} onClose={() => setSelectedId(null)} onTraceStart={playback.pause} onStageHighlight={setHighlightedStage} /></div>}
                <NexusAnalysis events={visible} selected={selectedEvent} />
              </aside>
              <div className="flex flex-col items-end gap-3">
                <LogVerseFilters filter={filteredEvents.filter} onChange={filteredEvents.setFilter} vendors={vendors} outcomes={outcomes} />
                <SceneLegend mode={sceneMode} />
              </div>
            </div>
            {hoveredSource && (
              <div className="pointer-events-none self-end rounded border border-white/10 bg-black/70 px-2 py-1 font-mono text-[10px] text-white/80">
                {hoveredSource.name} — {hoveredSource.status}
              </div>
            )}
            {hoveredVault && (
              <div className="pointer-events-none self-end rounded border border-white/10 bg-black/70 px-2 py-1 font-mono text-[10px] text-white/80">
                segment {hoveredVault.segmentId.slice(-14)} — {hoveredVault.eventCount} events
              </div>
            )}
            <TimelineControls
              mode={playback.mode}
              speed={playback.speed}
              clockMs={playback.clockMs}
              bounds={playback.bounds}
              status={data.status}
              eps={data.eps}
              visibleCount={visible.length}
              overflowCount={overflow}
              droppedEventsTotal={data.droppedEventsTotal}
              dlqCount={filteredEvents.dlqEvents.length}
              reducedMotion={perf.reducedMotion}
              reducedMotionOverride={perf.reducedMotionOverride}
              onReducedMotionOverrideChange={perf.setReducedMotionOverride}
              onGoLive={playback.goLive}
              onPause={playback.pause}
              onPlayReplay={playback.playReplay}
              onSpeedChange={playback.setSpeed}
              onScrub={playback.scrubTo}
              onResetCamera={resetView}
            />
          </div>

        </div>
      )}
    </div>
  );
}

function useFilteredEvents(events: LogVerseEvent[], dlqEvents: LogVerseEvent[]) {
  const [filter, setFilter] = useState(defaultFilter());
  const filtered = useMemo(() => events.filter((e) => matchesFilter(e, filter)), [events, filter]);
  const filteredDlq = useMemo(() => dlqEvents.filter((e) => matchesFilter(e, filter)), [dlqEvents, filter]);
  return { events: filtered, dlqEvents: filteredDlq, filter, setFilter };
}
