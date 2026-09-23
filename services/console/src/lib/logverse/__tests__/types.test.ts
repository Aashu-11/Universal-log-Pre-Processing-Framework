import { describe, expect, it } from "vitest";

import { rowToLogVerseEvent } from "../data";
import { defaultFilter, matchesFilter, sourceNodeVisible, type SourceNode } from "../types";

function baseEvent(overrides: Record<string, unknown> = {}) {
  const row = new Array(31).fill(null);
  const ev = rowToLogVerseEvent(row);
  return { ...ev, vendor: "cisco", eventAction: "allowed", visualState: "normal" as const, isDlq: false, ...overrides };
}

describe("defaultFilter / matchesFilter", () => {
  it("passes everything through by default", () => {
    const f = defaultFilter();
    expect(matchesFilter(baseEvent(), f)).toBe(true);
    expect(matchesFilter(baseEvent({ visualState: "threat" }), f)).toBe(true);
    expect(matchesFilter(baseEvent({ isDlq: true, visualState: "dlq" }), f)).toBe(true);
  });

  it("filters by visual state", () => {
    const f = defaultFilter();
    f.states = new Set(["threat"]);
    expect(matchesFilter(baseEvent({ visualState: "normal" }), f)).toBe(false);
    expect(matchesFilter(baseEvent({ visualState: "threat" }), f)).toBe(true);
  });

  it("dlqOnly hides non-DLQ events even if their state would otherwise pass", () => {
    const f = defaultFilter();
    f.dlqOnly = true;
    expect(matchesFilter(baseEvent({ isDlq: false }), f)).toBe(false);
    expect(matchesFilter(baseEvent({ isDlq: true, visualState: "dlq" }), f)).toBe(true);
  });

  it("filters by vendor (the field events actually carry, standing in for 'source')", () => {
    const f = defaultFilter();
    f.vendors = new Set(["fortinet"]);
    expect(matchesFilter(baseEvent({ vendor: "cisco" }), f)).toBe(false);
    expect(matchesFilter(baseEvent({ vendor: "fortinet" }), f)).toBe(true);
  });

  it("filters by outcome, treating an empty action as 'unknown'", () => {
    const f = defaultFilter();
    f.outcomes = new Set(["unknown"]);
    expect(matchesFilter(baseEvent({ eventAction: "" }), f)).toBe(true);
    expect(matchesFilter(baseEvent({ eventAction: "allowed" }), f)).toBe(false);
  });
});

describe("sourceNodeVisible", () => {
  const source: SourceNode = { sourceId: "s1", name: "Cisco ASA", vendor: "cisco", product: "asa", status: "active", enabled: true };

  it("shows every source when no source filter is set", () => {
    expect(sourceNodeVisible(source, defaultFilter())).toBe(true);
  });

  it("hides sources not in the filter set once one is set", () => {
    const f = defaultFilter();
    f.sources = new Set(["other-id"]);
    expect(sourceNodeVisible(source, f)).toBe(false);
    f.sources = new Set(["s1"]);
    expect(sourceNodeVisible(source, f)).toBe(true);
  });
});
