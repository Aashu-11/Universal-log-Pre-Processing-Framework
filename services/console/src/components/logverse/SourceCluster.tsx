import { Html } from "@react-three/drei";

import type { SourceNode } from "../../lib/logverse/types";
import { sourcePositionForIndex, type Vec3 } from "../../lib/logverse/layout";

const STATUS_COLOR: Record<string, string> = {
  active: "#36d399",
  inactive: "#78788a",
};

/** The registered Sources inventory (real GET /v1/sources rows), arranged
 * along the outer edge — spec: "Log sources as nodes around the outside of
 * the scene." One node per source row, not per vendor (a vendor can have
 * several registered sources), unlike EventParticleSystem's spawn points
 * which are per-vendor. */
export function SourceCluster({ sources, onHover }: { sources: SourceNode[]; onHover?: (s: SourceNode | null) => void }) {
  return (
    <group>
      {sources.map((s, i) => {
        const pos: Vec3 = sourcePositionForIndex(i, sources.length);
        const color = STATUS_COLOR[s.status] ?? "#8b93a7";
        return (
          <group key={s.sourceId} position={pos}>
            <mesh
              onPointerOver={(e) => {
                e.stopPropagation();
                onHover?.(s);
              }}
              onPointerOut={() => onHover?.(null)}
            >
              <cylinderGeometry args={[0.35, 0.5, 0.9, 6]} />
              <meshStandardMaterial color={color} emissive={color} emissiveIntensity={s.enabled ? 0.35 : 0} />
            </mesh>
            <Html center distanceFactor={26} occlude={false} style={{ pointerEvents: "none" }}>
              <div className="whitespace-nowrap rounded border border-white/10 bg-black/55 px-1.5 py-0.5 font-mono text-[8.5px] text-white/75 backdrop-blur">
                {s.vendor}
              </div>
            </Html>
          </group>
        );
      })}
    </group>
  );
}
