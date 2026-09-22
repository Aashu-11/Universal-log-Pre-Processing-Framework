// Typed view models for LogVerse — the 3D scene never reads a raw API
// response directly. Every value the scene renders is derived here, from
// fields the backend actually returns, so a missing/unavailable piece of
// forensic evidence is a distinct, representable state (`"unavailable"`),
// never silently absent or guessed at. See lib/logverse/data.ts for the
// mapping functions that produce these from real API responses.

/** How one event should read visually — a closed set so color, shape and
 * label always agree with each other and with the legend. Not color alone:
 * each state also implies a distinct marker shape (see SHAPE_FOR_STATE in
 * data.ts) so the scene stays legible under color-vision deficiency. */
export type EventVisualState = "normal" | "elevated" | "threat" | "dlq";

/** The eight CLAUDE.md pipeline stages, fixed order. */
export const PIPELINE_STAGES = [
  "ingest",
  "preserve",
  "identify",
  "parse",
  "normalize",
  "enrich",
  "validate",
  "route",
] as const;
export type PipelineStage = (typeof PIPELINE_STAGES)[number];

export interface LogVerseEvent {
  eventId: string;
  vendor: string;
  eventKind: string;
  eventAction: string;
  parserId: string;
  parserVersion: string;
  parseStatus: string;
  processingMs: number;
  qualityScore: number;
  riskScore: number;
  srcIp: string;
  srcPort: number;
  dstIp: string;
  dstPort: number;
  dstIsInternal: boolean;
  networkBytesTotal: number;
  iocMatch: boolean;
  iocIndicator: string;
  threatCategory: string;
  threatSeverity: string;
  threatMitreTactic: string;
  threatMitreTechnique: string;
  geoCountry: string;
  asOrg: string;
  rawSegmentId: string;
  rawOffset: number;
  rawLength: number;
  rawSha256: string;
  dt: string;
  hour: string;
  /** Milliseconds since epoch — converted from the API's nanosecond
   * `event_ingested_at` once here so every downstream consumer works in a
   * normal JS timestamp. This is a REAL, server-recorded timestamp, not a
   * client-side estimate. */
  ingestedAtMs: number;
  visualState: EventVisualState;
  /** true only for rows sourced from the DLQ endpoint; a DLQ event also
   * carries visualState "dlq" but this flag lets scene code branch without
   * re-deriving it. */
  isDlq: boolean;
  dlqReason: string | null;
}

export interface VaultBlock {
  segmentId: string;
  merkleRoot: string;
  prevRoot: string;
  eventCount: number;
  /** Milliseconds since epoch, converted from the API's nanosecond
   * `sealed_at_ns`. */
  sealedAtMs: number;
  byteSize: number;
}

export interface SourceNode {
  sourceId: string;
  name: string;
  vendor: string;
  product: string;
  status: string;
  enabled: boolean;
}

/** One "Unavailable" reason a forensic field can carry — always shown to
 * the investigator verbatim rather than a blank or a guess. */
export type Unavailable = { available: false; reason: string };
export type Maybe<T> = { available: true; value: T } | Unavailable;

export function available<T>(value: T): Maybe<T> {
  return { available: true, value };
}
export function unavailable<T = never>(reason: string): Maybe<T> {
  return { available: false, reason };
}

export interface ForensicTrace {
  eventId: string;
  sha256Verified: Maybe<boolean>;
  rawLength: Maybe<number>;
  rawBase64: Maybe<string>;
  merkleRoot: Maybe<string>;
  merkleLeaf: Maybe<string>;
  merkleProofSteps: Maybe<number>;
  merkleVerified: Maybe<boolean>;
  merkleIndex: Maybe<number>;
  merklePath: Maybe<{ sibling: string; side: string }[]>;
  /** Populated only when the raw bytes/merkle lookup itself failed —
   * distinct from "field not returned by a successful call". */
  error: string | null;
}

export interface EpsSample {
  atMs: number;
  eventsReceivedTotal: number;
}

/** A derived (never server-measured) events-per-second figure, computed
 * from two counter samples — see computeEps in data.ts. Always rendered
 * with an explicit "derived" label; CLAUDE.md forbids presenting this as a
 * measured rate. */
export interface DerivedEps {
  eps: number;
  overSeconds: number;
}

export type ConnectionStatus = "connecting" | "live" | "reconnecting" | "error";

export interface SeverityFilter {
  states: Set<EventVisualState>;
  vendors: Set<string>;
  sources: Set<string>;
  outcomes: Set<string>;
  dlqOnly: boolean;
}

export function defaultFilter(): SeverityFilter {
  return {
    states: new Set(["normal", "elevated", "threat", "dlq"]),
    vendors: new Set(),
    sources: new Set(),
    outcomes: new Set(),
    dlqOnly: false,
  };
}

// Events carry observer.vendor, not a log_source_id — the lake schema has
// no per-event pointer back to the Sources registry row, so "filter by
// source" for events is implemented as "filter by vendor" (the field that
// actually exists on every event). `sources` instead controls which
// SourceCluster nodes are shown, independent of event filtering — see
// sourceNodeVisible below.
export function matchesFilter(ev: LogVerseEvent, f: SeverityFilter): boolean {
  if (!f.states.has(ev.visualState)) return false;
  if (f.dlqOnly && !ev.isDlq) return false;
  if (f.vendors.size > 0 && !f.vendors.has(ev.vendor)) return false;
  if (f.outcomes.size > 0 && !f.outcomes.has(ev.eventAction || "unknown")) return false;
  return true;
}

export function sourceNodeVisible(source: SourceNode, f: SeverityFilter): boolean {
  if (f.sources.size === 0) return true;
  return f.sources.has(source.sourceId);
}
