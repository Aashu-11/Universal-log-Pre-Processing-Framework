// Fetchers + row->view-model mappers for LogVerse. Every function here is
// pure (or a thin, directly-testable wrapper around one), deliberately
// separate from any Three.js/scene code — see lib/logverse/types.ts for the
// shapes these produce. Reuses the same /v1/query (Presto) and REST
// endpoints every other console page already uses; no new backend
// endpoint was added for LogVerse.
import { controlPlane } from "../api";
import type { DLQEventOut, QueryResponse } from "../types";
import {
  available,
  unavailable,
  type EventVisualState,
  type ForensicTrace,
  type LogVerseEvent,
  type Maybe,
  type SourceNode,
  type VaultBlock,
} from "./types";

// ---------------------------------------------------------------------------
// Bounds — documented visual/memory limits (CLAUDE.md: no unbounded
// in-memory collections; see docs/LOGVERSE.md "Limits" for the full list).
// ---------------------------------------------------------------------------
export const EVENT_POLL_MS = 3000;
export const MAX_EVENT_BUFFER = 240; // client-side ring buffer of recent events kept for replay/scrub
export const MAX_VISIBLE_PARTICLES = 150; // instanced-mesh capacity actually rendered at once
export const MAX_DLQ_VISIBLE = 80;
export const MAX_VAULT_BLOCKS_VISIBLE = 20;

// Both `lake.logkrama.events` and `vault.logkrama.raw_segments` are Hive-partitioned
// by `dt`. An unfiltered "ORDER BY ... LIMIT n" still has to touch every
// partition to determine the true top-n, which gets slow as a demo/dev
// environment accumulates weeks of small partitions from repeated test
// runs — found live: an unfiltered query against this project's own dev
// lake took 15s+ for a plain count(*) with only 29 partitions registered.
// Restricting to a recent `dt` window lets Hive-side partition pruning
// skip everything older, independent of how much history has piled up.
const RECENT_DT_WINDOW_DAYS = 14;
function recentDtFilter(): string {
  return `dt >= CAST(current_date - interval '${RECENT_DT_WINDOW_DAYS}' day AS varchar)`;
}

const EVENT_COLUMNS = [
  "event_id", "observer_vendor", "event_kind", "event_action",
  "lineage_parser_id", "lineage_parser_version", "lineage_parse_status", "lineage_processing_ms",
  "quality_score", "enrich_risk_score",
  "src_ip", "src_port", "dst_ip", "dst_port", "enrich_dst_is_internal", "network_bytes_total",
  "enrich_ioc_match", "enrich_ioc_indicator",
  "threat_category", "threat_severity", "threat_mitre_tactic", "threat_mitre_technique",
  "enrich_src_geo_country", "enrich_src_as_org",
  "raw_segment_id", "raw_offset", "raw_length", "raw_sha256",
  "dt", "hour", "event_ingested_at",
] as const;

function col(row: unknown[], name: (typeof EVENT_COLUMNS)[number]): unknown {
  return row[EVENT_COLUMNS.indexOf(name)];
}

/** A pure classification: real IOC match or a populated threat category is
 * "threat" regardless of the numeric risk score (a generic high score alone
 * is never enough to call something an attack — CLAUDE.md's visual-
 * truthfulness rule). 40 is the enrichment pipeline's own ioc_match weight
 * (internal/enrich/risk.go) — an elevated-but-unflagged score at or above
 * it is shown as "elevated", distinct from a confirmed IOC/threat match. */
export function classifyVisualState(fields: {
  iocMatch: boolean;
  threatCategory: string;
  riskScore: number;
}): EventVisualState {
  if (fields.iocMatch || fields.threatCategory.trim() !== "") return "threat";
  if (fields.riskScore >= 40) return "elevated";
  return "normal";
}

export function rowToLogVerseEvent(row: unknown[]): LogVerseEvent {
  const str = (name: (typeof EVENT_COLUMNS)[number]) => String(col(row, name) ?? "");
  const num = (name: (typeof EVENT_COLUMNS)[number]) => Number(col(row, name) ?? 0);
  const bool = (name: (typeof EVENT_COLUMNS)[number]) => Boolean(col(row, name));

  const iocMatch = bool("enrich_ioc_match");
  const threatCategory = str("threat_category");
  const riskScore = num("enrich_risk_score");

  return {
    eventId: str("event_id"),
    vendor: str("observer_vendor"),
    eventKind: str("event_kind"),
    eventAction: str("event_action"),
    parserId: str("lineage_parser_id"),
    parserVersion: str("lineage_parser_version"),
    parseStatus: str("lineage_parse_status"),
    processingMs: num("lineage_processing_ms"),
    qualityScore: num("quality_score"),
    riskScore,
    srcIp: str("src_ip"),
    srcPort: num("src_port"),
    dstIp: str("dst_ip"),
    dstPort: num("dst_port"),
    dstIsInternal: bool("enrich_dst_is_internal"),
    networkBytesTotal: num("network_bytes_total"),
    iocMatch,
    iocIndicator: str("enrich_ioc_indicator"),
    threatCategory,
    threatSeverity: str("threat_severity"),
    threatMitreTactic: str("threat_mitre_tactic"),
    threatMitreTechnique: str("threat_mitre_technique"),
    geoCountry: str("enrich_src_geo_country"),
    asOrg: str("enrich_src_as_org"),
    rawSegmentId: str("raw_segment_id"),
    rawOffset: num("raw_offset"),
    rawLength: num("raw_length"),
    rawSha256: str("raw_sha256"),
    dt: str("dt"),
    hour: str("hour"),
    ingestedAtMs: num("event_ingested_at") / 1e6, // API returns UnixNano (internal/processor sets it via time.UnixNano())
    visualState: classifyVisualState({ iocMatch, threatCategory, riskScore }),
    isDlq: false,
    dlqReason: null,
  };
}

export async function fetchLogVerseEvents(limit: number): Promise<LogVerseEvent[]> {
  const sql = `SELECT ${EVENT_COLUMNS.join(", ")}\nFROM events\nWHERE ${recentDtFilter()}\nORDER BY event_ingested_at DESC\nLIMIT ${limit}`;
  const resp = await controlPlane.post<QueryResponse>("/v1/query", { sql, catalog: "lake", schema_: "logkrama" });
  return resp.rows.map(rowToLogVerseEvent);
}

// parser_id is written as "<vendor>.<product>.<class>" by every shipped and
// auto-onboarded parser in this repo (see packs/*/*/*.yaml) — a reasonable,
// documented derivation for a DLQ row's vendor, since DLQEvent itself
// carries no vendor column (see services/control-plane/app/models.py).
// This is inference from a real, present field, not fabricated data.
function vendorFromParserId(parserId: string | null): string {
  if (!parserId) return "unknown";
  return parserId.split(".")[0] || "unknown";
}

export function dlqEventToLogVerseEvent(e: DLQEventOut): LogVerseEvent {
  const ref = e.raw_ref as { segment_id?: string; offset?: number; length?: number; sha256?: string };
  return {
    eventId: e.event_id,
    vendor: vendorFromParserId(e.parser_id),
    eventKind: "event",
    eventAction: "",
    parserId: e.parser_id ?? "",
    parserVersion: "",
    parseStatus: "unknown",
    processingMs: 0,
    qualityScore: 0,
    riskScore: 0,
    srcIp: "",
    srcPort: 0,
    dstIp: "",
    dstPort: 0,
    dstIsInternal: false,
    networkBytesTotal: 0,
    iocMatch: false,
    iocIndicator: "",
    threatCategory: "",
    threatSeverity: "",
    threatMitreTactic: "",
    threatMitreTechnique: "",
    geoCountry: "",
    asOrg: "",
    rawSegmentId: String(ref?.segment_id ?? ""),
    rawOffset: Number(ref?.offset ?? 0),
    rawLength: Number(ref?.length ?? 0),
    rawSha256: String(ref?.sha256 ?? ""),
    dt: "",
    hour: "",
    ingestedAtMs: new Date(e.occurred_at).getTime(),
    visualState: "dlq",
    isDlq: true,
    dlqReason: e.reason,
  };
}

export async function fetchDlqEvents(): Promise<LogVerseEvent[]> {
  const rows = await controlPlane.get<DLQEventOut[]>(`/v1/dlq?limit=${MAX_DLQ_VISIBLE}`);
  return rows.map(dlqEventToLogVerseEvent);
}

export async function fetchSources(): Promise<SourceNode[]> {
  const rows = await controlPlane.get<{ log_source_id: string; name: string; vendor: string; product: string; status: string; enabled: boolean }[]>(
    "/v1/sources",
  );
  return rows.map((r) => ({
    sourceId: r.log_source_id,
    name: r.name,
    vendor: r.vendor,
    product: r.product,
    status: r.status,
    enabled: r.enabled,
  }));
}

const VAULT_COLUMNS = ["segment_id", "merkle_root", "prev_root", "event_count", "sealed_at_ns", "byte_size"] as const;

export async function fetchVaultChain(limit: number): Promise<VaultBlock[]> {
  const sql = `SELECT ${VAULT_COLUMNS.join(", ")} FROM raw_segments WHERE ${recentDtFilter()} ORDER BY sealed_at_ns DESC LIMIT ${limit}`;
  const resp = await controlPlane.post<QueryResponse>("/v1/query", { sql, catalog: "vault", schema_: "logkrama" });
  const blocks = resp.rows.map((row) => {
    const g = (name: (typeof VAULT_COLUMNS)[number]) => row[VAULT_COLUMNS.indexOf(name)];
    return {
      segmentId: String(g("segment_id") ?? ""),
      merkleRoot: String(g("merkle_root") ?? ""),
      prevRoot: String(g("prev_root") ?? ""),
      eventCount: Number(g("event_count") ?? 0),
      sealedAtMs: Number(g("sealed_at_ns") ?? 0) / 1e6,
      byteSize: Number(g("byte_size") ?? 0),
    } satisfies VaultBlock;
  });
  // Oldest-first so the chain reads left-to-right as prev_root -> merkle_root,
  // matching how a Merkle chain is normally drawn.
  return blocks.reverse();
}

export async function fetchForensicTrace(ev: LogVerseEvent): Promise<ForensicTrace> {
  if (!ev.rawSegmentId) {
    return {
      eventId: ev.eventId,
      sha256Verified: unavailable("no raw vault reference on this event"),
      rawLength: unavailable("no raw vault reference on this event"),
      rawBase64: unavailable("no raw vault reference on this event"),
      merkleRoot: unavailable("no raw vault reference on this event"),
      merkleLeaf: unavailable("no raw vault reference on this event"),
      merkleProofSteps: unavailable("no raw vault reference on this event"),
      merkleVerified: unavailable("no raw vault reference on this event"),
      merkleIndex: unavailable("no raw vault reference on this event"),
      merklePath: unavailable("no raw vault reference on this event"),
      error: null,
    };
  }
  const params = new URLSearchParams({
    segment_id: ev.rawSegmentId,
    offset: String(ev.rawOffset),
    length: String(ev.rawLength),
    sha256: ev.rawSha256,
  });
  try {
    const resp = await controlPlane.get<{
      raw: { sha256_verified: boolean; length: number; raw_base64: string };
      merkle_proof: {
        Leaf?: number[]; Root?: number[]; Path?: { Sibling?: number[]; SiblingOn?: string }[]; Index?: number;
        leaf?: string; root?: string; path?: { sibling?: string; side?: string }[]; index?: number;
        verified?: boolean | null;
      };
    }>(`/v1/events/${encodeURIComponent(ev.eventId)}/trace?${params}`);
    const proof = resp.merkle_proof;
    const hex = (value: number[] | string | undefined): string | null => Array.isArray(value)
      ? value.map((byte) => byte.toString(16).padStart(2, "0")).join("")
      : typeof value === "string" && value ? value : null;
    const path = proof.Path?.map((step) => ({ sibling: hex(step.Sibling) ?? "unavailable", side: step.SiblingOn ?? "unknown" }))
      ?? proof.path?.map((step) => ({ sibling: step.sibling ?? "unavailable", side: step.side ?? "unknown" }));
    return {
      eventId: ev.eventId,
      sha256Verified: available(resp.raw.sha256_verified),
      rawLength: available(resp.raw.length),
      rawBase64: available(resp.raw.raw_base64),
      merkleRoot: hex(proof.Root ?? proof.root) ? available(hex(proof.Root ?? proof.root)!) : unavailable("not returned by the vault"),
      merkleLeaf: hex(proof.Leaf ?? proof.leaf) ? available(hex(proof.Leaf ?? proof.leaf)!) : unavailable("not returned by the vault"),
      merkleProofSteps: path
        ? available(path.length)
        : unavailable("not returned by the vault"),
      merkleVerified: typeof proof.verified === "boolean" ? available(proof.verified) : unavailable("vault did not return a proof verdict"),
      merkleIndex: typeof (proof.Index ?? proof.index) === "number" ? available((proof.Index ?? proof.index)!) : unavailable("not returned by the vault"),
      merklePath: path ? available(path) : unavailable("not returned by the vault"),
      error: null,
    };
  } catch (err) {
    const reason = err instanceof Error ? err.message : "vault unreachable";
    return {
      eventId: ev.eventId,
      sha256Verified: unavailable(reason),
      rawLength: unavailable(reason),
      rawBase64: unavailable(reason),
      merkleRoot: unavailable(reason),
      merkleLeaf: unavailable(reason),
      merkleProofSteps: unavailable(reason),
      merkleVerified: unavailable(reason),
      merkleIndex: unavailable(reason),
      merklePath: unavailable(reason),
      error: reason,
    };
  }
}

/** Derives events/sec from two counter samples of the collector's
 * Prometheus `logkrama_events_received_total`. This is NEVER a server-measured
 * rate — the API only exposes the running counter — so every caller must
 * label it as derived (see DerivedEps). Returns null when there isn't yet a
 * second sample, or the counter went backwards (a service restart). */
export function computeEps(
  prev: { atMs: number; total: number } | null,
  next: { atMs: number; total: number },
): { eps: number; overSeconds: number } | null {
  if (!prev) return null;
  const dtSeconds = (next.atMs - prev.atMs) / 1000;
  if (dtSeconds <= 0) return null;
  const dCount = next.total - prev.total;
  if (dCount < 0) return null; // counter reset (service restart) — not a valid rate sample
  return { eps: dCount / dtSeconds, overSeconds: dtSeconds };
}

/** Density-reduction policy for when the filtered event set exceeds what's
 * rendered as individual particles: keep the N most-recently-arrived (so
 * the ingest end of the pipeline still looks continuously busy) and report
 * the rest as an aggregate count rather than silently dropping them or
 * instancing an unbounded number of meshes. */
export function selectVisibleParticles(
  events: LogVerseEvent[],
  cap: number,
): { visible: LogVerseEvent[]; overflow: number } {
  const byId = new Map<string, LogVerseEvent>();
  for (const event of events) {
    const previous = byId.get(event.eventId);
    if (!previous || event.isDlq) byId.set(event.eventId, event);
  }
  const unique = [...byId.values()];
  if (unique.length <= cap) return { visible: unique, overflow: 0 };
  const newest = (items: LogVerseEvent[]) => items.sort((a, b) => b.ingestedAtMs - a.ingestedAtMs);
  const dlq = newest(unique.filter((event) => event.isDlq));
  const normal = newest(unique.filter((event) => !event.isDlq));
  const dlqSlots = Math.min(dlq.length, Math.max(1, Math.ceil(cap * 0.25)));
  const normalSlots = Math.min(normal.length, cap - dlqSlots);
  const spare = cap - dlqSlots - normalSlots;
  return {
    visible: [...dlq.slice(0, dlqSlots + spare), ...normal.slice(0, normalSlots)]
      .sort((a, b) => b.ingestedAtMs - a.ingestedAtMs),
    overflow: unique.length - cap,
  };
}

export interface DerivedAsset {
  ip: string;
  hits: number;
}

export const MAX_ASSETS_VISIBLE = 12;

/** LogVerse has no dedicated asset-inventory endpoint to draw from (there
 * isn't one in this API — see docs/LOGVERSE.md), so "internal assets" is
 * derived from real, already-fetched event data: distinct destination IPs
 * marked enrich_dst_is_internal=true in the current event buffer, ranked
 * by how often they appear. This is honest inference from real fields,
 * not a fabricated CMDB — always labeled as "observed", not "inventory". */
export function deriveInternalAssets(events: LogVerseEvent[], cap: number = MAX_ASSETS_VISIBLE): DerivedAsset[] {
  const counts = new Map<string, number>();
  for (const e of events) {
    if (!e.dstIsInternal || !e.dstIp) continue;
    counts.set(e.dstIp, (counts.get(e.dstIp) ?? 0) + 1);
  }
  return Array.from(counts.entries())
    .map(([ip, hits]) => ({ ip, hits }))
    .sort((a, b) => b.hits - a.hits)
    .slice(0, cap);
}

export type { Maybe };
