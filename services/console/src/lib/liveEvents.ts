// Shared "recent real events from the lake" query — used by both the
// Traceability page (click one, see its full trace) and the Live Theater
// page (auto-play the newest one). One column order, one row parser, so the
// two pages can never silently drift on what a RecentEvent contains.
import { controlPlane } from "./api";
import type { QueryResponse } from "./types";

export const RECENT_COLUMNS = [
  "event_id", "observer_vendor", "event_kind", "event_action",
  "lineage_parser_id", "lineage_parser_version", "lineage_parse_status", "lineage_processing_ms",
  "quality_score", "enrich_risk_score", "enrich_src_geo_country", "enrich_src_as_org",
  "raw_segment_id", "raw_offset", "raw_length", "raw_sha256",
  "dt", "hour", "vendor", "event_ingested_at",
] as const;

export function recentEventsSql(limit: number): string {
  return `SELECT ${RECENT_COLUMNS.join(", ")}\nFROM events\nORDER BY event_ingested_at DESC\nLIMIT ${limit}`;
}

export interface RecentEvent {
  event_id: string;
  observer_vendor: string;
  event_kind: string;
  event_action: string;
  lineage_parser_id: string;
  lineage_parser_version: string;
  lineage_parse_status: string;
  lineage_processing_ms: number;
  quality_score: number;
  enrich_risk_score: number;
  enrich_src_geo_country: string;
  enrich_src_as_org: string;
  raw_segment_id: string;
  raw_offset: number;
  raw_length: number;
  raw_sha256: string;
  dt: string;
  hour: string;
  vendor: string;
  event_ingested_at: number;
}

export function rowToRecentEvent(row: unknown[]): RecentEvent {
  const get = (name: (typeof RECENT_COLUMNS)[number]) => row[RECENT_COLUMNS.indexOf(name)];
  return {
    event_id: String(get("event_id") ?? ""),
    observer_vendor: String(get("observer_vendor") ?? ""),
    event_kind: String(get("event_kind") ?? ""),
    event_action: String(get("event_action") ?? ""),
    lineage_parser_id: String(get("lineage_parser_id") ?? ""),
    lineage_parser_version: String(get("lineage_parser_version") ?? ""),
    lineage_parse_status: String(get("lineage_parse_status") ?? ""),
    lineage_processing_ms: Number(get("lineage_processing_ms") ?? 0),
    quality_score: Number(get("quality_score") ?? 0),
    enrich_risk_score: Number(get("enrich_risk_score") ?? 0),
    enrich_src_geo_country: String(get("enrich_src_geo_country") ?? ""),
    enrich_src_as_org: String(get("enrich_src_as_org") ?? ""),
    raw_segment_id: String(get("raw_segment_id") ?? ""),
    raw_offset: Number(get("raw_offset") ?? 0),
    raw_length: Number(get("raw_length") ?? 0),
    raw_sha256: String(get("raw_sha256") ?? ""),
    dt: String(get("dt") ?? ""),
    hour: String(get("hour") ?? ""),
    vendor: String(get("vendor") ?? ""),
    event_ingested_at: Number(get("event_ingested_at") ?? 0),
  };
}

export async function fetchRecentEvents(limit: number): Promise<RecentEvent[]> {
  const resp = await controlPlane.post<QueryResponse>("/v1/query", {
    sql: recentEventsSql(limit),
    catalog: "lake",
    schema_: "ulpf",
  });
  return resp.rows.map(rowToRecentEvent);
}

export interface TraceResult {
  event_id: string;
  raw: { sha256_verified: boolean; length: number; raw_base64: string };
  merkle_proof: { leaf: string; root: string; path: unknown[]; index: number };
}

// The complete, uniform, normalized record — every UES namespace, whatever
// vendor it came from. `SELECT *` (not an explicit column list) so this
// stays correct automatically if schema/presto/ddl.sql ever gains a column;
// columns come back named, so callers group by name prefix rather than
// trusting positional order.
export interface FullRecord {
  columns: string[];
  values: unknown[];
}

export async function fetchFullRecord(eventId: string): Promise<FullRecord> {
  const safeId = eventId.replace(/'/g, "''");
  const resp = await controlPlane.post<QueryResponse>("/v1/query", {
    sql: `SELECT * FROM events WHERE event_id = '${safeId}' LIMIT 1`,
    catalog: "lake",
    schema_: "ulpf",
  });
  const row = resp.rows[0];
  if (resp.row_count === 0 || !row) {
    throw new Error(`event ${eventId} not found in lake.ulpf.events`);
  }
  return { columns: resp.columns, values: row };
}

export async function fetchTrace(ev: RecentEvent): Promise<TraceResult> {
  const params = new URLSearchParams({
    segment_id: ev.raw_segment_id,
    offset: String(ev.raw_offset),
    length: String(ev.raw_length),
    sha256: ev.raw_sha256,
  });
  return controlPlane.get<TraceResult>(`/v1/events/${encodeURIComponent(ev.event_id)}/trace?${params}`);
}
