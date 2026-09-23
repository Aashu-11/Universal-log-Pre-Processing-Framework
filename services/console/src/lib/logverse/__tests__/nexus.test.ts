import { describe, expect, it } from "vitest";
import { buildNexus, eventsForNode } from "../nexus";
import type { LogVerseEvent } from "../types";

function event(eventId: string, srcIp: string, dstIp: string): LogVerseEvent {
  return { eventId, vendor: "Fortinet", srcIp, dstIp, visualState: "threat" } as LogVerseEvent;
}

describe("buildNexus", () => {
  it("connects real event rows to their vendor and endpoints without inventing IPs", () => {
    const graph = buildNexus([event("a", "10.0.0.1", ""), event("b", "10.0.0.1", "8.8.8.8")]);
    expect(graph.nodes.map((node) => node.id).sort()).toEqual(["event:a", "event:b", "ip:10.0.0.1", "ip:8.8.8.8", "vendor:Fortinet"]);
    expect(graph.links).toHaveLength(5);
    expect(buildNexus([event("a", "10.0.0.1", "")])).toEqual(buildNexus([event("a", "10.0.0.1", "")]));
  });

  it("limits rendered events to the newest 150", () => {
    const graph = buildNexus(Array.from({ length: 200 }, (_, index) => event(String(index), "", "")));
    expect(graph.nodes.filter((node) => node.kind === "event")).toHaveLength(150);
    expect(graph.nodes.some((node) => node.id === "event:0")).toBe(false);
  });

  it("connects DLQ events to each real failure reason and lists events for any clicked node", () => {
    const dlq = { ...event("dlq-1", "", ""), visualState: "dlq" as const, isDlq: true, dlqReason: "schema_violation,bad_port", ingestedAtMs: 3 };
    const normal = { ...event("normal-1", "10.0.0.1", "8.8.8.8"), ingestedAtMs: 2 };
    const graph = buildNexus([normal, dlq]);
    expect(graph.nodes.filter((node) => node.kind === "reason").map((node) => node.id).sort()).toEqual(["reason:bad_port", "reason:schema_violation"]);
    const reason = graph.nodes.find((node) => node.id === "reason:bad_port")!;
    const vendor = graph.nodes.find((node) => node.id === "vendor:Fortinet")!;
    const endpoint = graph.nodes.find((node) => node.id === "ip:10.0.0.1")!;
    expect(eventsForNode(reason, [normal, dlq]).map((item) => item.eventId)).toEqual(["dlq-1"]);
    expect(eventsForNode(vendor, [normal, dlq]).map((item) => item.eventId)).toEqual(["dlq-1", "normal-1"]);
    expect(eventsForNode(endpoint, [normal, dlq]).map((item) => item.eventId)).toEqual(["normal-1"]);
  });
});
