import { Html } from "@react-three/drei";

import type { DerivedAsset } from "../../lib/logverse/data";
import { assetPositionForIndex } from "../../lib/logverse/layout";

/** "Internal assets" — derived, not inventoried (see deriveInternalAssets'
 * doc comment). Rendered visibly closer to scene center than the outer
 * SourceCluster ring, per spec, and clearly labeled as observed rather than
 * a formal CMDB so nobody mistakes this for ground truth about the whole
 * estate — only what's been seen as a destination in the current buffer. */
export function AssetCluster({ assets }: { assets: DerivedAsset[] }) {
  return (
    <group>
      {assets.map((a, i) => {
        const pos = assetPositionForIndex(i, assets.length);
        return (
          <group key={a.ip} position={pos}>
            <mesh>
              <icosahedronGeometry args={[0.32, 0]} />
              <meshStandardMaterial color="#3b82f6" emissive="#3b82f6" emissiveIntensity={0.3} />
            </mesh>
            <Html center distanceFactor={30} occlude={false} style={{ pointerEvents: "none" }}>
              <div className="whitespace-nowrap rounded border border-white/10 bg-black/55 px-1.5 py-0.5 font-mono text-[8px] text-white/70 backdrop-blur">
                {a.ip}
              </div>
            </Html>
          </group>
        );
      })}
      {assets.length > 0 && (
        <Html position={[0, 11.5, 0]} center occlude={false} style={{ pointerEvents: "none" }}>
          <div className="whitespace-nowrap rounded border border-white/10 bg-black/55 px-2 py-1 font-mono text-[8.5px] uppercase tracking-wide text-white/60 backdrop-blur">
            Internal assets — observed destinations, not a formal inventory
          </div>
        </Html>
      )}
    </group>
  );
}
