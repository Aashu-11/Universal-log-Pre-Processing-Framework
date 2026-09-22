import type { LogVerseEvent } from "./types";

export type NexusNode = {
  id: string;
  label: string;
  kind: "vendor" | "endpoint" | "reason" | "event";
  position: [number, number, number];
  event?: LogVerseEvent;
};

export type NexusLink = { source: string; target: string; state: LogVerseEvent["visualState"] };

function hash(value: string): number {
  let result = 2166136261;
  for (let i = 0; i < value.length; i++) result = Math.imul(result ^ value.charCodeAt(i), 16777619);
  return result >>> 0;
}

function position(key: string, radius: number, height: number): [number, number, number] {
  const h = hash(key);
  const angle = (h % 3600) * Math.PI / 1800;
  return [Math.cos(angle) * radius, ((h >>> 12) % 1000) / 1000 * height - height / 2, Math.sin(angle) * radius];
}

/** A bounded, deterministic relationship graph from actual lake/DLQ rows. */
export function buildNexus(events: LogVerseEvent[]): { nodes: NexusNode[]; links: NexusLink[] } {
  const nodes = new Map<string, NexusNode>();
  const links: NexusLink[] = [];
  for (const event of events.slice(-150)) {
    const vendor = event.vendor || "Unknown vendor";
    const vendorId = `vendor:${vendor}`;
    if (!nodes.has(vendorId)) nodes.set(vendorId, { id: vendorId, label: vendor, kind: "vendor", position: position(vendorId, 15, 9) });

    const eventId = `event:${event.eventId}`;
    nodes.set(eventId, { id: eventId, label: event.eventId.slice(0, 12), kind: "event", position: position(eventId, 8, 12), event });
    links.push({ source: vendorId, target: eventId, state: event.visualState });

    if (event.isDlq) {
      for (const reason of (event.dlqReason || "unknown reason").split(",").map((part) => part.trim()).filter(Boolean)) {
        const reasonId = `reason:${reason}`;
        if (!nodes.has(reasonId)) nodes.set(reasonId, { id: reasonId, label: reason.replaceAll("_", " "), kind: "reason", position: position(reasonId, 11, 8) });
        links.push({ source: eventId, target: reasonId, state: "dlq" });
      }
    }

    for (const ip of [event.srcIp, event.dstIp]) {
      if (!ip) continue;
      const endpointId = `ip:${ip}`;
      if (!nodes.has(endpointId)) nodes.set(endpointId, { id: endpointId, label: ip, kind: "endpoint", position: position(endpointId, 3.5, 7) });
      links.push({ source: eventId, target: endpointId, state: event.visualState });
    }
  }
  return { nodes: [...nodes.values()], links };
}

export function eventsForNode(node: NexusNode, events: LogVerseEvent[]): LogVerseEvent[] {
  const matches = events.filter((event) => {
    if (node.kind === "event") return event.eventId === node.event?.eventId;
    if (node.kind === "vendor") return (event.vendor || "Unknown vendor") === node.label;
    if (node.kind === "endpoint") return event.srcIp === node.label || event.dstIp === node.label;
    return event.isDlq && (event.dlqReason || "unknown reason").split(",").some((reason) => reason.trim().replaceAll("_", " ") === node.label);
  });
  return matches.sort((a, b) => b.ingestedAtMs - a.ingestedAtMs);
}
