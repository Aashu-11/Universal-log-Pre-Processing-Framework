import { forwardRef, useImperativeHandle, useMemo, useRef, useState } from "react";
import { Canvas } from "@react-three/fiber";
import { Html, OrbitControls } from "@react-three/drei";
import type { OrbitControls as OrbitControlsImpl } from "three-stdlib";

import { buildVendorPositions } from "../../lib/logverse/layout";
import { PIPELINE_STAGES, type LogVerseEvent, type PipelineStage, type SourceNode } from "../../lib/logverse/types";
import type { DerivedAsset } from "../../lib/logverse/data";
import type { VaultBlock } from "../../lib/logverse/types";
import type { Vec3 } from "../../lib/logverse/layout";
import { STAGE_POSITIONS } from "../../lib/logverse/layout";
import { PipelineStageNode } from "./PipelineStageNode";
import { SourceCluster } from "./SourceCluster";
import { AssetCluster } from "./AssetCluster";
import { EventParticleSystem } from "./EventParticleSystem";
import { VaultChain, type IntegrityState } from "./VaultChain";
import { RouteDestinations, DlqZoneMarker } from "./RouteDestinations";

export interface LogVerseCanvasHandle {
  resetCamera: () => void;
}

export const LogVerseCanvas = forwardRef<
  LogVerseCanvasHandle,
  {
    sources: SourceNode[];
    events: LogVerseEvent[];
    assets: DerivedAsset[];
    vaultChain: VaultBlock[];
    integrityState: IntegrityState;
    dlqCount: number;
    clockMsRef: React.RefObject<number>;
    selectedId: string | null;
    highlightedStage: PipelineStage | null;
    reducedMotion: boolean;
    paused: boolean;
    onSelect: (ev: LogVerseEvent) => void;
    onSourceHover: (s: SourceNode | null) => void;
    onVaultHover: (b: VaultBlock | null) => void;
    onContextLost: () => void;
    onContextRestored: () => void;
  }
>(function LogVerseCanvas(
  {
    sources,
    events,
    assets,
    vaultChain,
    integrityState,
    dlqCount,
    clockMsRef,
    selectedId,
    highlightedStage,
    reducedMotion,
    paused,
    onSelect,
    onSourceHover,
    onVaultHover,
    onContextLost,
    onContextRestored,
  },
  ref,
) {
  const controlsRef = useRef<OrbitControlsImpl>(null);
  const [hovered, setHovered] = useState<{ event: LogVerseEvent; pos: Vec3 } | null>(null);

  const vendorPositions = useMemo(() => buildVendorPositions(sources), [sources]);

  useImperativeHandle(ref, () => ({
    resetCamera: () => controlsRef.current?.reset(),
  }));

  return (
    <div className="relative h-full w-full">
      <Canvas
        camera={{ position: [4, 24, 48], fov: 48, near: 0.1, far: 300 }}
        dpr={[1, 1.75]}
        frameloop={paused ? "never" : "always"}
        gl={{ antialias: true, powerPreference: "high-performance" }}
        onCreated={({ gl }) => {
          const el = gl.domElement;
          const lost = (e: Event) => {
            e.preventDefault();
            onContextLost();
          };
          const restored = () => onContextRestored();
          el.addEventListener("webglcontextlost", lost);
          el.addEventListener("webglcontextrestored", restored);
        }}
      >
        <OrbitControls
          ref={controlsRef}
          enableDamping={!reducedMotion}
          dampingFactor={0.08}
          minDistance={12}
          maxDistance={110}
          target={[0, 0, 0]}
        />

        <color attach="background" args={["#050506"]} />
        <fog attach="fog" args={["#050506", 40, 130]} />
        <ambientLight intensity={0.55} />
        <directionalLight position={[20, 30, 20]} intensity={0.9} />
        <pointLight position={[0, 10, 0]} intensity={0.3} color="#8b5cf6" />

        {PIPELINE_STAGES.map((stage) => (
          <PipelineStageNode
            key={stage}
            stage={stage}
            position={STAGE_POSITIONS[stage]}
            highlighted={highlightedStage === stage}
            reducedMotion={reducedMotion}
          />
        ))}

        <SourceCluster sources={sources} onHover={onSourceHover} />
        <AssetCluster assets={assets} />
        <RouteDestinations />
        <DlqZoneMarker count={dlqCount} />
        <VaultChain blocks={vaultChain} integrityState={integrityState} onHover={onVaultHover} />

        <EventParticleSystem
          events={events}
          vendorPositions={vendorPositions}
          clockMsRef={clockMsRef}
          selectedId={selectedId}
          onSelect={onSelect}
          onHover={(ev, pos) => setHovered(ev && pos ? { event: ev, pos } : null)}
        />

        {hovered && (
          <Html position={hovered.pos} center occlude={false} style={{ pointerEvents: "none" }}>
            <div className="pointer-events-none max-w-[220px] rounded border border-white/15 bg-black/80 px-2 py-1.5 font-mono text-[10px] text-white shadow-xl">
              <div className="truncate font-semibold">{hovered.event.eventId.slice(0, 22)}…</div>
              <div className="text-white/60">
                {hovered.event.vendor} · {hovered.event.visualState}
                {hovered.event.eventAction ? ` · ${hovered.event.eventAction}` : ""}
              </div>
            </div>
          </Html>
        )}
      </Canvas>
    </div>
  );
});
