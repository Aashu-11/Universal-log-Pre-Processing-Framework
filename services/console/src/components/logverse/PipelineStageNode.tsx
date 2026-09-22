import { useRef } from "react";
import { useFrame } from "@react-three/fiber";
import { Html } from "@react-three/drei";
import { MathUtils, type Mesh } from "three";

import type { PipelineStage } from "../../lib/logverse/types";
import type { Vec3 } from "../../lib/logverse/layout";

const STAGE_LABEL: Record<PipelineStage, string> = {
  ingest: "INGEST",
  preserve: "PRESERVE",
  identify: "IDENTIFY",
  parse: "PARSE",
  normalize: "NORMALIZE",
  enrich: "ENRICH",
  validate: "VALIDATE",
  route: "ROUTE",
};

/** One labeled station along the pipeline rail. VALIDATE additionally
 * renders as a wide flat "gateway" plane (spec: "a visible decision
 * gateway") instead of a plain node, since that's the one stage where a
 * particle's path actually forks (normal vs DLQ). Highlighted (brighter,
 * larger) while ForensicTracePlayback is stepping backward through it. */
export function PipelineStageNode({
  stage,
  position,
  highlighted,
  reducedMotion,
}: {
  stage: PipelineStage;
  position: Vec3;
  highlighted: boolean;
  reducedMotion: boolean;
}) {
  const meshRef = useRef<Mesh>(null);

  useFrame((state) => {
    if (!meshRef.current) return;
    const targetScale = highlighted ? 1.35 : 1;
    const nextScale = reducedMotion ? targetScale : MathUtils.lerp(meshRef.current.scale.x, targetScale, 0.12);
    meshRef.current.scale.setScalar(nextScale);
    if (!reducedMotion) {
      meshRef.current.rotation.y = state.clock.elapsedTime * 0.15;
    }
  });

  const isGate = stage === "validate";

  return (
    <group position={position}>
      {isGate ? (
        <mesh ref={meshRef} rotation={[0, 0, 0]}>
          <boxGeometry args={[0.25, 3.2, 3.2]} />
          <meshStandardMaterial
            color={highlighted ? "#f6be4f" : "#3b3b4a"}
            emissive={highlighted ? "#f6be4f" : "#000000"}
            emissiveIntensity={highlighted ? 0.6 : 0}
            transparent
            opacity={0.55}
          />
        </mesh>
      ) : (
        <mesh ref={meshRef}>
          <octahedronGeometry args={[0.7, 0]} />
          <meshStandardMaterial
            color={highlighted ? "#8b5cf6" : "#4a4a5c"}
            emissive={highlighted ? "#8b5cf6" : "#000000"}
            emissiveIntensity={highlighted ? 0.8 : 0}
          />
        </mesh>
      )}
      <Html center distanceFactor={22} occlude={false} style={{ pointerEvents: "none" }}>
        <div
          className={`whitespace-nowrap rounded border px-1.5 py-0.5 font-mono text-[9px] uppercase tracking-wide backdrop-blur ${
            highlighted
              ? "border-[var(--color-accent)]/60 bg-[var(--color-accent)]/25 text-white"
              : "border-white/10 bg-black/50 text-white/70"
          }`}
        >
          {STAGE_LABEL[stage]}
        </div>
      </Html>
    </group>
  );
}
