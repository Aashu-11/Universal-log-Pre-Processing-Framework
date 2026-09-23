import { describe, expect, it } from "vitest";

import { relatedEventCount, riskBuckets, volumeBuckets } from "../analysis";
import type { LogVerseEvent } from "../types";

function event(id: string, time: number, state: LogVerseEvent["visualState"], srcIp = "", dstIp = ""): LogVerseEvent {
  return { eventId: id, ingestedAtMs: time, visualState: state, srcIp, dstIp } as LogVerseEvent;
}

describe("Nexus analysis", () => {
  it("counts only observed states and time-bucketed events", () => {
    const events = [event("a", 100, "normal"), event("b", 200, "threat"), event("c", 200, "dlq")];
    expect(riskBuckets(events).map((bucket) => bucket.count)).toEqual([1, 0, 1, 1]);
    expect(volumeBuckets(events, 2)).toEqual([1, 2]);
  });

  it("counts related events only when an IP was actually observed", () => {
    const selected = event("a", 100, "normal", "10.0.0.1");
    expect(relatedEventCount(selected, [selected, event("b", 200, "threat", "", "10.0.0.1"), event("c", 300, "normal")])).toBe(1);
  });
});
