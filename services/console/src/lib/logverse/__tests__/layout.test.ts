import { describe, expect, it } from "vitest";

import {
  DLQ_ZONE_POS,
  STAGE_POSITIONS,
  UNKNOWN_SOURCE_POS,
  assetPositionForIndex,
  buildVendorPositions,
  interpolatePosition,
  positionForEvent,
  sourcePositionForIndex,
  vendorSpawnPosition,
  waypointsForEvent,
} from "../layout";
import { rowToLogVerseEvent } from "../data";
import type { SourceNode } from "../types";

describe("sourcePositionForIndex", () => {
  it("spreads sources monotonically across z", () => {
    const a = sourcePositionForIndex(0, 3);
    const b = sourcePositionForIndex(1, 3);
    const c = sourcePositionForIndex(2, 3);
    expect(a[2]).toBeLessThan(b[2]);
    expect(b[2]).toBeLessThan(c[2]);
  });

  it("places a single source at the center line", () => {
    expect(sourcePositionForIndex(0, 1)[2]).toBe(0);
  });
});

describe("assetPositionForIndex", () => {
  it("places assets above the pipeline line (distinct from the outer source ring)", () => {
    const p = assetPositionForIndex(0, 4);
    expect(p[1]).toBeGreaterThan(0);
  });
});

describe("waypointsForEvent / interpolatePosition", () => {
  const source: [number, number, number] = [-30, 0, 5];

  it("starts exactly at the source position and ends exactly at ROUTE for a normal event", () => {
    const pts = waypointsForEvent(source, false);
    expect(interpolatePosition(pts, 0)).toEqual(source);
    expect(interpolatePosition(pts, 1)).toEqual(STAGE_POSITIONS.route);
  });

  it("ends at the DLQ zone, not ROUTE, for a DLQ event", () => {
    const pts = waypointsForEvent(source, true);
    expect(interpolatePosition(pts, 1)).toEqual(DLQ_ZONE_POS);
  });

  it("passes through every intermediate stage position in order as progress advances", () => {
    const pts = waypointsForEvent(source, false);
    const stages: (keyof typeof STAGE_POSITIONS)[] = [
      "ingest", "preserve", "identify", "parse", "normalize", "enrich", "validate",
    ];
    stages.forEach((stage, i) => {
      const progress = (i + 1) / 8; // 8 segments: source->ingest is segment 0 ending at progress 1/8
      const pos = interpolatePosition(pts, progress);
      expect(pos).toEqual(STAGE_POSITIONS[stage]);
    });
  });

  it("is deterministic", () => {
    const pts = waypointsForEvent(source, false);
    expect(interpolatePosition(pts, 0.37)).toEqual(interpolatePosition(pts, 0.37));
  });

  it("clamps progress outside [0,1]", () => {
    const pts = waypointsForEvent(source, false);
    expect(interpolatePosition(pts, -1)).toEqual(source);
    expect(interpolatePosition(pts, 5)).toEqual(STAGE_POSITIONS.route);
  });
});

describe("buildVendorPositions / vendorSpawnPosition", () => {
  const sources: SourceNode[] = [
    { sourceId: "s1", name: "A", vendor: "cisco", product: "asa", status: "active", enabled: true },
    { sourceId: "s2", name: "B", vendor: "fortinet", product: "fortigate", status: "active", enabled: true },
    { sourceId: "s3", name: "C", vendor: "cisco", product: "asa2", status: "active", enabled: true }, // duplicate vendor
  ];

  it("assigns exactly one position per distinct vendor", () => {
    const map = buildVendorPositions(sources);
    expect(map.size).toBe(2);
    expect(map.has("cisco")).toBe(true);
    expect(map.has("fortinet")).toBe(true);
  });

  it("falls back to UNKNOWN_SOURCE_POS for a vendor with no registered source", () => {
    const map = buildVendorPositions(sources);
    expect(vendorSpawnPosition("paloalto", map)).toEqual(UNKNOWN_SOURCE_POS);
  });

  it("returns the assigned position for a known vendor", () => {
    const map = buildVendorPositions(sources);
    expect(vendorSpawnPosition("cisco", map)).toEqual(map.get("cisco"));
  });
});

describe("positionForEvent", () => {
  it("routes a DLQ-flagged event to the DLQ zone at full progress", () => {
    const ev = { ...rowToLogVerseEvent(new Array(31).fill(null)), isDlq: true };
    const pos = positionForEvent(ev, [-30, 0, 0], 1);
    expect(pos).toEqual(DLQ_ZONE_POS);
  });

  it("routes a normal event to ROUTE at full progress", () => {
    const ev = { ...rowToLogVerseEvent(new Array(31).fill(null)), isDlq: false };
    const pos = positionForEvent(ev, [-30, 0, 0], 1);
    expect(pos).toEqual(STAGE_POSITIONS.route);
  });
});
