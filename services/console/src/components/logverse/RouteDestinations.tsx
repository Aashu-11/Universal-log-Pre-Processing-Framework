import { Html } from "@react-three/drei";
import { DoubleSide } from "three";

import { DLQ_ZONE_POS, ROUTE_DESTINATIONS, routeDestinationPosition, type RouteDestination } from "../../lib/logverse/layout";

const DESTINATION_META: Record<RouteDestination, { label: string; connector: string; color: string }> = {
  lake: { label: "lake", connector: "hive / Parquet", color: "#36d399" },
  stream: { label: "stream", connector: "kafka, last 30min", color: "#22d3ee" },
  meta: { label: "meta", connector: "postgresql", color: "#3b82f6" },
  vault: { label: "vault", connector: "hive / segment index", color: "#8b5cf6" },
};

/** The four Presto federated catalogs every normal event fans out to —
 * matches Explorer's own CATALOGS list exactly, so this scene and the
 * Explorer page never describe the destinations differently. */
export function RouteDestinations() {
  return (
    <group>
      {ROUTE_DESTINATIONS.map((d, i) => {
        const pos = routeDestinationPosition(i);
        const meta = DESTINATION_META[d];
        return (
          <group key={d} position={pos}>
            <mesh>
              <cylinderGeometry args={[0.55, 0.55, 0.35, 8]} />
              <meshStandardMaterial color={meta.color} emissive={meta.color} emissiveIntensity={0.35} />
            </mesh>
            <Html center distanceFactor={26} occlude={false} style={{ pointerEvents: "none" }}>
              <div className="whitespace-nowrap rounded border border-white/10 bg-black/55 px-1.5 py-0.5 text-center font-mono text-[8.5px] text-white/75 backdrop-blur">
                <div>{meta.label}</div>
                <div className="text-white/45">{meta.connector}</div>
              </div>
            </Html>
          </group>
        );
      })}
    </group>
  );
}

/** The separate DLQ branch zone — spec: "Lower or separate branch: DLQ".
 * Distinct position and a dashed boundary ring (line-style distinction,
 * not just the purple particle color) from the normal route destinations
 * above. */
export function DlqZoneMarker({ count }: { count: number }) {
  return (
    <group position={DLQ_ZONE_POS}>
      <mesh rotation={[Math.PI / 2, 0, 0]}>
        <ringGeometry args={[2.2, 2.4, 32]} />
        <meshBasicMaterial color="#b06bf0" transparent opacity={0.4} side={DoubleSide} />
      </mesh>
      <Html center distanceFactor={24} occlude={false} style={{ pointerEvents: "none" }}>
        <div className="whitespace-nowrap rounded border border-[#b06bf0]/50 bg-black/60 px-2 py-1 text-center font-mono text-[9px] uppercase tracking-wide text-[#d8b6ff] backdrop-blur">
          Dead Letter Queue ({count})
        </div>
      </Html>
    </group>
  );
}
