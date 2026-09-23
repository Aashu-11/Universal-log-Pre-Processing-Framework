import type { LogVerseEvent } from "./types";

export type RiskBucket = { label: string; count: number; color: string };

export function riskBuckets(events: LogVerseEvent[]): RiskBucket[] {
  return [
    { label: "Normal", count: events.filter((event) => event.visualState === "normal").length, color: "#22d3ee" },
    { label: "Elevated", count: events.filter((event) => event.visualState === "elevated").length, color: "#f6be4f" },
    { label: "Threat", count: events.filter((event) => event.visualState === "threat").length, color: "#fb5b71" },
    { label: "DLQ", count: events.filter((event) => event.visualState === "dlq").length, color: "#b06bf0" },
  ];
}

export function volumeBuckets(events: LogVerseEvent[], count = 12): number[] {
  if (events.length === 0) return Array(count).fill(0) as number[];
  const timestamps = events.map((event) => event.ingestedAtMs).filter(Number.isFinite);
  if (timestamps.length === 0) return Array(count).fill(0) as number[];
  const min = Math.min(...timestamps);
  const max = Math.max(...timestamps);
  const span = Math.max(1, max - min);
  const buckets = Array(count).fill(0) as number[];
  for (const time of timestamps) {
    const index = Math.min(count - 1, Math.floor(((time - min) / span) * count));
    buckets[index] = (buckets[index] ?? 0) + 1;
  }
  return buckets;
}

export function relatedEventCount(selected: LogVerseEvent, events: LogVerseEvent[]): number {
  return events.filter((event) => event.eventId !== selected.eventId && (
    (selected.srcIp && (event.srcIp === selected.srcIp || event.dstIp === selected.srcIp)) ||
    (selected.dstIp && (event.srcIp === selected.dstIp || event.dstIp === selected.dstIp))
  )).length;
}
