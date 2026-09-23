import { describe, expect, it, vi } from "vitest";

import { controlPlane } from "../../api";
import {
  classifyVisualState,
  computeEps,
  deriveInternalAssets,
  dlqEventToLogVerseEvent,
  fetchForensicTrace,
  fetchLogVerseEvents,
  fetchVaultChain,
  rowToLogVerseEvent,
  selectVisibleParticles,
} from "../data";
import type { LogVerseEvent } from "../types";
import type { DLQEventOut } from "../../types";

vi.mock("../../api", async () => {
  const actual = await vi.importActual<typeof import("../../api")>("../../api");
  return { ...actual, controlPlane: { ...actual.controlPlane, post: vi.fn(), get: vi.fn() } };
});

const EVENT_COLUMNS_ORDER = [
  "event_id", "observer_vendor", "event_kind", "event_action",
  "lineage_parser_id", "lineage_parser_version", "lineage_parse_status", "lineage_processing_ms",
  "quality_score", "enrich_risk_score",
  "src_ip", "src_port", "dst_ip", "dst_port", "enrich_dst_is_internal", "network_bytes_total",
  "enrich_ioc_match", "enrich_ioc_indicator",
  "threat_category", "threat_severity", "threat_mitre_tactic", "threat_mitre_technique",
  "enrich_src_geo_country", "enrich_src_as_org",
  "raw_segment_id", "raw_offset", "raw_length", "raw_sha256",
  "dt", "hour", "event_ingested_at",
];

function makeRow(overrides: Record<string, unknown>): unknown[] {
  const base: Record<string, unknown> = {
    event_id: "ev-1",
    observer_vendor: "cisco",
    event_kind: "event",
    event_action: "allowed",
    lineage_parser_id: "cisco.asa.302013",
    lineage_parser_version: "1.0.0",
    lineage_parse_status: "ok",
    lineage_processing_ms: 0.42,
    quality_score: 0.95,
    enrich_risk_score: 5,
    src_ip: "10.0.0.1",
    src_port: 5555,
    dst_ip: "8.8.8.8",
    dst_port: 443,
    enrich_dst_is_internal: false,
    network_bytes_total: 1024,
    enrich_ioc_match: false,
    enrich_ioc_indicator: "",
    threat_category: "",
    threat_severity: "",
    threat_mitre_tactic: "",
    threat_mitre_technique: "",
    enrich_src_geo_country: "US",
    enrich_src_as_org: "Example ISP",
    raw_segment_id: "seg-1",
    raw_offset: 128,
    raw_length: 64,
    raw_sha256: "abc123",
    dt: "2026-09-10",
    hour: "12",
    event_ingested_at: 1_800_000_000_000_000_000, // ns
    ...overrides,
  };
  return EVENT_COLUMNS_ORDER.map((c) => base[c]);
}

describe("classifyVisualState", () => {
  it("is normal when risk is low and nothing is flagged", () => {
    expect(classifyVisualState({ iocMatch: false, threatCategory: "", riskScore: 0 })).toBe("normal");
  });

  it("is elevated once risk crosses the enrichment pipeline's own ioc_match weight (40)", () => {
    expect(classifyVisualState({ iocMatch: false, threatCategory: "", riskScore: 40 })).toBe("elevated");
    expect(classifyVisualState({ iocMatch: false, threatCategory: "", riskScore: 39.9 })).toBe("normal");
  });

  it("is threat on a real IOC match regardless of risk score", () => {
    expect(classifyVisualState({ iocMatch: true, threatCategory: "", riskScore: 1 })).toBe("threat");
  });

  it("is threat on a populated threat category even with a low risk score", () => {
    // Visual truthfulness: a high score alone never implies "attack", but a
    // real threat classification does, independent of the score.
    expect(classifyVisualState({ iocMatch: false, threatCategory: "malware", riskScore: 0 })).toBe("threat");
  });

  it("threat takes priority over elevated", () => {
    expect(classifyVisualState({ iocMatch: true, threatCategory: "", riskScore: 99 })).toBe("threat");
  });
});

describe("rowToLogVerseEvent", () => {
  it("maps every column by name, not position, and converts ns to ms", () => {
    const ev = rowToLogVerseEvent(makeRow({}));
    expect(ev.eventId).toBe("ev-1");
    expect(ev.vendor).toBe("cisco");
    expect(ev.srcIp).toBe("10.0.0.1");
    expect(ev.dstPort).toBe(443);
    expect(ev.ingestedAtMs).toBeCloseTo(1_800_000_000_000, 5);
    expect(ev.isDlq).toBe(false);
    expect(ev.visualState).toBe("normal");
  });

  it("classifies a genuine IOC-flagged row as threat", () => {
    const ev = rowToLogVerseEvent(makeRow({ enrich_ioc_match: true, enrich_ioc_indicator: "198.51.100.9" }));
    expect(ev.visualState).toBe("threat");
    expect(ev.iocMatch).toBe(true);
  });

  it("defaults missing/null fields to safe zero values instead of throwing", () => {
    const row = EVENT_COLUMNS_ORDER.map(() => null);
    const ev = rowToLogVerseEvent(row);
    expect(ev.eventId).toBe("");
    expect(ev.riskScore).toBe(0);
    expect(ev.visualState).toBe("normal");
  });
});

describe("dlqEventToLogVerseEvent", () => {
  const dlq: DLQEventOut = {
    event_id: "dlq-1",
    raw_ref: { segment_id: "seg-9", offset: 10, length: 20, sha256: "deadbeef" },
    reason: "bad_port",
    parser_id: "sonicwall.firewall.traffic",
    occurred_at: "2026-09-10T12:00:00Z",
    resolved: false,
  };

  it("derives vendor from the parser_id's first dot-segment (documented convention, not fabricated)", () => {
    const ev = dlqEventToLogVerseEvent(dlq);
    expect(ev.vendor).toBe("sonicwall");
  });

  it("carries the raw vault reference through for tracing", () => {
    const ev = dlqEventToLogVerseEvent(dlq);
    expect(ev.rawSegmentId).toBe("seg-9");
    expect(ev.rawOffset).toBe(10);
    expect(ev.rawLength).toBe(20);
    expect(ev.rawSha256).toBe("deadbeef");
  });

  it("is always visualState dlq and isDlq true, and keeps the real reason", () => {
    const ev = dlqEventToLogVerseEvent(dlq);
    expect(ev.visualState).toBe("dlq");
    expect(ev.isDlq).toBe(true);
    expect(ev.dlqReason).toBe("bad_port");
  });

  it("falls back to 'unknown' vendor when parser_id is null", () => {
    const ev = dlqEventToLogVerseEvent({ ...dlq, parser_id: null });
    expect(ev.vendor).toBe("unknown");
  });
});

describe("computeEps", () => {
  it("returns null with no prior sample", () => {
    expect(computeEps(null, { atMs: 1000, total: 10 })).toBeNull();
  });

  it("derives events/sec from two counter samples", () => {
    const result = computeEps({ atMs: 0, total: 100 }, { atMs: 5000, total: 600 });
    expect(result).toEqual({ eps: 100, overSeconds: 5 });
  });

  it("returns null when the counter goes backwards (service restart), never a negative rate", () => {
    expect(computeEps({ atMs: 0, total: 500 }, { atMs: 1000, total: 10 })).toBeNull();
  });

  it("returns null for a non-positive time delta", () => {
    expect(computeEps({ atMs: 1000, total: 10 }, { atMs: 1000, total: 20 })).toBeNull();
  });
});

describe("partition pruning — dt filter on every unfiltered query", () => {
  // Regression: an unfiltered "ORDER BY ... LIMIT n" against these
  // Hive-partitioned tables forces Presto to scan every partition to find
  // the true top-n — found live to take 15s+ / hang under load with as few
  // as 29 partitions in this project's own dev lake (see docs/LOGVERSE.md).
  // Every query LogVerse issues against a dt-partitioned table must filter
  // by a recent dt window so Hive-side partition pruning actually applies.
  it("fetchLogVerseEvents' SQL restricts to a recent dt window", async () => {
    const post = vi.mocked(controlPlane.post);
    post.mockResolvedValue({ columns: [], rows: [], row_count: 0 });
    await fetchLogVerseEvents(60);
    const [, body] = post.mock.calls.at(-1)!;
    const sql = (body as { sql: string }).sql;
    expect(sql).toMatch(/WHERE\s+dt\s*>=\s*CAST\(current_date/i);
  });

  it("fetchVaultChain's SQL restricts to a recent dt window", async () => {
    const post = vi.mocked(controlPlane.post);
    post.mockResolvedValue({ columns: [], rows: [], row_count: 0 });
    await fetchVaultChain(20);
    const [, body] = post.mock.calls.at(-1)!;
    const sql = (body as { sql: string }).sql;
    expect(sql).toMatch(/WHERE\s+dt\s*>=\s*CAST\(current_date/i);
  });
});

describe("fetchForensicTrace", () => {
  it("reads the vault's actual capitalized proof fields and verification verdict", async () => {
    vi.mocked(controlPlane.get).mockResolvedValueOnce({
      raw: { sha256_verified: true, length: 4, raw_base64: "dGVzdA==" },
      merkle_proof: { Leaf: [1, 2], Root: [3, 4], Path: [{ Sibling: [5, 6], SiblingOn: "left" }], Index: 7, verified: true },
    });
    const trace = await fetchForensicTrace(rowToLogVerseEvent(makeRow({})));
    expect(trace.merkleRoot).toEqual({ available: true, value: "0304" });
    expect(trace.merkleLeaf).toEqual({ available: true, value: "0102" });
    expect(trace.merklePath).toEqual({ available: true, value: [{ sibling: "0506", side: "left" }] });
    expect(trace.merkleVerified).toEqual({ available: true, value: true });
    expect(trace.merkleIndex).toEqual({ available: true, value: 7 });
  });
  it("short-circuits to Unavailable for every field when the event has no raw vault reference, without calling the API", async () => {
    const ev = rowToLogVerseEvent(makeRow({ raw_segment_id: "" }));
    const trace = await fetchForensicTrace(ev);
    expect(trace.sha256Verified).toEqual({ available: false, reason: "no raw vault reference on this event" });
    expect(trace.merkleRoot.available).toBe(false);
    expect(trace.error).toBeNull();
  });
});

describe("deriveInternalAssets", () => {
  it("counts distinct internal destination IPs, ranked by frequency", () => {
    const events = [
      rowToLogVerseEvent(makeRow({ event_id: "a", dst_ip: "10.0.0.5", enrich_dst_is_internal: true })),
      rowToLogVerseEvent(makeRow({ event_id: "b", dst_ip: "10.0.0.5", enrich_dst_is_internal: true })),
      rowToLogVerseEvent(makeRow({ event_id: "c", dst_ip: "10.0.0.9", enrich_dst_is_internal: true })),
      rowToLogVerseEvent(makeRow({ event_id: "d", dst_ip: "8.8.8.8", enrich_dst_is_internal: false })),
    ];
    const assets = deriveInternalAssets(events, 10);
    expect(assets).toEqual([
      { ip: "10.0.0.5", hits: 2 },
      { ip: "10.0.0.9", hits: 1 },
    ]);
  });

  it("respects the cap", () => {
    const events = ["a", "b", "c"].map((id, i) =>
      rowToLogVerseEvent(makeRow({ event_id: id, dst_ip: `10.0.0.${i}`, enrich_dst_is_internal: true })),
    );
    expect(deriveInternalAssets(events, 2)).toHaveLength(2);
  });
});

describe("selectVisibleParticles", () => {
  function ev(id: string, ingestedAtMs: number): LogVerseEvent {
    return { ...rowToLogVerseEvent(makeRow({ event_id: id })), eventId: id, ingestedAtMs };
  }

  it("returns everything with zero overflow when under the cap", () => {
    const events = [ev("a", 1), ev("b", 2)];
    expect(selectVisibleParticles(events, 5)).toEqual({ visible: events, overflow: 0 });
  });

  it("keeps the most-recently-arrived N and reports the rest as overflow", () => {
    const events = [ev("old", 1), ev("mid", 2), ev("new", 3)];
    const { visible, overflow } = selectVisibleParticles(events, 2);
    expect(visible.map((e) => e.eventId)).toEqual(["new", "mid"]);
    expect(overflow).toBe(1);
  });

  it("reserves visible slots for DLQ events even during heavy normal traffic", () => {
    const normals = Array.from({ length: 20 }, (_, index) => ev(`normal-${index}`, 1000 + index));
    const dlq = { ...ev("dlq-event", 1), isDlq: true, visualState: "dlq" as const };
    const result = selectVisibleParticles([...normals, dlq], 8);
    expect(result.visible.some((event) => event.eventId === "dlq-event" && event.isDlq)).toBe(true);
    expect(result.visible).toHaveLength(8);
  });

  it("prefers the DLQ copy when an event also exists in the lake", () => {
    const lake = ev("same", 2);
    const dlq = { ...lake, isDlq: true, visualState: "dlq" as const };
    expect(selectVisibleParticles([lake, dlq], 2).visible).toEqual([dlq]);
  });
});
