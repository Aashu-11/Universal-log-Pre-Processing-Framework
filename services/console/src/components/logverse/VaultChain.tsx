import { useMemo, useRef } from "react";
import { useFrame } from "@react-three/fiber";
import { Html, Line } from "@react-three/drei";
import type { Group } from "three";

import type { VaultBlock } from "../../lib/logverse/types";
import { vaultBlockPosition } from "../../lib/logverse/layout";

export type IntegrityState = "unknown" | "verified" | "failed";

const STATE_COLOR: Record<IntegrityState, string> = {
  unknown: "#5b6478",
  verified: "#36d399",
  failed: "#fb5b71",
};

function FractureOverlay({ scale }: { scale: number }) {
  const ref = useRef<Group>(null);
  useFrame((state) => {
    if (!ref.current) return;
    // A slow, smooth pulse — deliberately not a rapid flash/strobe, which
    // the spec explicitly warns against for accessibility.
    const pulse = 1 + Math.sin(state.clock.elapsedTime * 1.6) * 0.06;
    ref.current.scale.setScalar(scale * 1.25 * pulse);
  });
  return (
    <group ref={ref} rotation={[0.4, 0.5, 0.2]}>
      <mesh>
        <boxGeometry args={[1, 1, 1]} />
        <meshBasicMaterial color="#fb5b71" wireframe transparent opacity={0.8} />
      </mesh>
    </group>
  );
}

/** The Raw Vault's Merkle chain — one block per sealed vault segment
 * (vault.ulpf.raw_segments), linked prev_root -> merkle_root exactly as
 * the real chain is, laid out beneath PRESERVE. integrityState is only
 * ever "verified"/"failed" after the caller has actually run
 * GET /v1/integrity/verify — this component never assumes verification
 * happened just because blocks are visible (spec: never display
 * "Verified" unless verification data actually says so). */
export function VaultChain({
  blocks,
  integrityState,
  onHover,
}: {
  blocks: VaultBlock[];
  integrityState: IntegrityState;
  onHover?: (b: VaultBlock | null) => void;
}) {
  const positions = useMemo(() => blocks.map((_, i) => vaultBlockPosition(i, blocks.length)), [blocks]);
  const color = STATE_COLOR[integrityState];

  if (blocks.length === 0) {
    return (
      <Html position={vaultBlockPosition(0, 1)} center occlude={false} style={{ pointerEvents: "none" }}>
        <div className="whitespace-nowrap rounded border border-white/10 bg-black/55 px-2 py-1 font-mono text-[9px] text-white/50 backdrop-blur">
          Vault chain — no sealed segments yet
        </div>
      </Html>
    );
  }

  return (
    <group>
      {positions.length > 1 && (
        <Line points={positions} color={color} lineWidth={1.5} transparent opacity={0.55} />
      )}
      {blocks.map((b, i) => {
        const scale = Math.min(1.4, 0.5 + Math.log10(Math.max(1, b.eventCount)) * 0.25);
        return (
          <group key={b.segmentId} position={positions[i]}>
            <mesh
              scale={scale}
              onPointerOver={(e) => {
                e.stopPropagation();
                onHover?.(b);
              }}
              onPointerOut={() => onHover?.(null)}
            >
              <boxGeometry args={[1, 1, 1]} />
              <meshStandardMaterial color={color} emissive={color} emissiveIntensity={integrityState === "unknown" ? 0.1 : 0.5} />
            </mesh>
            {integrityState === "failed" && <FractureOverlay scale={scale} />}
          </group>
        );
      })}
      <Html position={[positions[0]![0], positions[0]![1] - 1.4, positions[0]![2]]} center occlude={false} style={{ pointerEvents: "none" }}>
        <div className="whitespace-nowrap rounded border border-white/10 bg-black/55 px-2 py-1 font-mono text-[8.5px] uppercase tracking-wide text-white/60 backdrop-blur">
          Vault chain ({blocks.length}) —{" "}
          {integrityState === "unknown" ? "not yet verified" : integrityState === "verified" ? "PASS" : "FAIL"}
        </div>
      </Html>
    </group>
  );
}
