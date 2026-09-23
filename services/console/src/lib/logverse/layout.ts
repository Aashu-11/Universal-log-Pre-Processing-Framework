// Pure spatial layout for the LogVerse scene: every position is a plain
// [x, y, z] tuple computed by ordinary math, with zero dependency on
// three.js or @react-three/fiber — so waypoint/interpolation logic is
// unit-testable without a WebGL context, and rendering code (layout
// consumers in components/logverse/*) stays a thin presentation layer over
// numbers computed here.
import { PIPELINE_STAGES, type LogVerseEvent, type PipelineStage, type SourceNode } from "./types";

export type Vec3 = [number, number, number];

// Stages laid out along a straight line on X, evenly spaced — the clearest
// possible read of "one pipeline, eight steps in order."
const STAGE_SPACING = 4;
const STAGE_START_X = -((PIPELINE_STAGES.length - 1) * STAGE_SPACING) / 2;

export const STAGE_POSITIONS: Record<PipelineStage, Vec3> = Object.fromEntries(
  PIPELINE_STAGES.map((stage, i) => [stage, [STAGE_START_X + i * STAGE_SPACING, 0, 0]]),
) as Record<PipelineStage, Vec3>;

export const DLQ_ZONE_POS: Vec3 = [STAGE_POSITIONS.validate[0], -12, 6];

const SOURCE_X = STAGE_POSITIONS.ingest[0] - 12;
const SOURCE_Z_SPREAD = 18;

/** Sources are laid out along the outer edge (spec: "Left or outer edge:
 * source devices grouped by vendor"), spread across Z at a fixed distance
 * from the pipeline so every source has a distinct, stable spawn point for
 * its particles. */
export function sourcePositionForIndex(index: number, total: number): Vec3 {
  if (total <= 1) return [SOURCE_X, 0, 0];
  const t = index / (total - 1);
  const z = -SOURCE_Z_SPREAD + t * (2 * SOURCE_Z_SPREAD);
  return [SOURCE_X, 0, z];
}

/** Fallback spawn point for an event whose vendor doesn't match any
 * currently-registered source (e.g. the source was removed, or the event
 * predates it) — still visibly "outside the ring", never silently placed
 * at the origin. */
export const UNKNOWN_SOURCE_POS: Vec3 = [SOURCE_X - 4, 0, 0];

const ASSET_RADIUS = 5;
const ASSET_Y = 8;

/** "Internal assets" has no dedicated inventory endpoint (see
 * docs/LOGVERSE.md) — positions here are for whatever distinct internal
 * destination hosts the caller derived from real recent events, arranged
 * in a small ring above-center ("closer to the center" per spec, distinct
 * from the outer source ring). */
export function assetPositionForIndex(index: number, total: number): Vec3 {
  const angle = (index / Math.max(1, total)) * Math.PI * 2;
  return [Math.cos(angle) * ASSET_RADIUS, ASSET_Y, Math.sin(angle) * ASSET_RADIUS];
}

export const ENRICH_SATELLITES = ["geoip", "asn", "ioc", "mitre", "risk"] as const;
export type EnrichSatellite = (typeof ENRICH_SATELLITES)[number];

const SATELLITE_RADIUS = 3.2;

export function enrichSatellitePosition(index: number): Vec3 {
  const angle = (index / ENRICH_SATELLITES.length) * Math.PI * 2;
  const center = STAGE_POSITIONS.enrich;
  return [center[0] + Math.cos(angle) * SATELLITE_RADIUS, center[1] + 2.2, center[2] + Math.sin(angle) * SATELLITE_RADIUS];
}

export const ROUTE_DESTINATIONS = ["lake", "stream", "meta", "vault"] as const;
export type RouteDestination = (typeof ROUTE_DESTINATIONS)[number];

export function routeDestinationPosition(index: number): Vec3 {
  const baseX = STAGE_POSITIONS.route[0] + 6;
  const z = -6 + index * 4;
  return [baseX, 0, z];
}

const VAULT_BLOCK_SPACING = 1.8;

/** The vault chain sits directly beneath PRESERVE, laid out oldest-to-
 * newest along Z — fetchVaultChain already returns blocks in that order. */
export function vaultBlockPosition(index: number, total: number): Vec3 {
  const center = STAGE_POSITIONS.preserve;
  const startZ = -((total - 1) * VAULT_BLOCK_SPACING) / 2;
  return [center[0], center[1] - 5, center[2] + startZ + index * VAULT_BLOCK_SPACING];
}

/** One position per distinct vendor seen in the Sources registry, in
 * first-seen order — the single source of truth both SourceCluster (which
 * draws the node) and EventParticleSystem (which spawns that vendor's
 * particles from the same point) read from, so a particle always appears
 * to originate from its actual source node. */
export function buildVendorPositions(sources: SourceNode[]): Map<string, Vec3> {
  const vendors: string[] = [];
  for (const s of sources) {
    if (!vendors.includes(s.vendor)) vendors.push(s.vendor);
  }
  const map = new Map<string, Vec3>();
  vendors.forEach((v, i) => map.set(v, sourcePositionForIndex(i, vendors.length)));
  return map;
}

export function vendorSpawnPosition(vendor: string, vendorPositions: Map<string, Vec3>): Vec3 {
  return vendorPositions.get(vendor) ?? UNKNOWN_SOURCE_POS;
}

/** Every event's path: its source spawn point, through INGEST..VALIDATE,
 * then either ROUTE (normal) or the DLQ zone — a documented visualization
 * simplification of where a DLQ event "branches away" (the real pipeline
 * decision is made during ROUTE, not visually distinguishable as an
 * earlier fork point without per-stage timing data the API doesn't
 * expose), not a claim about the exact backend branch point. */
export function waypointsForEvent(sourcePos: Vec3, isDlq: boolean): Vec3[] {
  const pts: Vec3[] = [sourcePos];
  for (let i = 0; i < PIPELINE_STAGES.length - 1; i++) {
    pts.push(STAGE_POSITIONS[PIPELINE_STAGES[i]!]);
  }
  pts.push(isDlq ? DLQ_ZONE_POS : STAGE_POSITIONS.route);
  return pts;
}

/** Piecewise-linear interpolation across waypoints by a single 0..1
 * progress value — deterministic and pure, the same function used for both
 * live spawning and timeline scrubbing so a particle's position never
 * depends on *how* it got to a given progress, only on the progress
 * itself. */
export function interpolatePosition(waypoints: Vec3[], progress: number): Vec3 {
  const clamped = Math.min(1, Math.max(0, progress));
  const segments = waypoints.length - 1;
  if (segments <= 0) return waypoints[0] ?? [0, 0, 0];
  const scaled = clamped * segments;
  const segIndex = Math.min(segments - 1, Math.floor(scaled));
  const t = scaled - segIndex;
  const a = waypoints[segIndex]!;
  const b = waypoints[segIndex + 1]!;
  return [a[0] + (b[0] - a[0]) * t, a[1] + (b[1] - a[1]) * t, a[2] + (b[2] - a[2]) * t];
}

export function positionForEvent(ev: LogVerseEvent, sourcePos: Vec3, progress: number): Vec3 {
  return interpolatePosition(waypointsForEvent(sourcePos, ev.isDlq), progress);
}
