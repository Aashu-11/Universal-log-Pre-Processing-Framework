import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { WebGLFallback } from "../WebGLFallback";
import { rowToLogVerseEvent } from "../../../lib/logverse/data";
import type { SourceNode } from "../../../lib/logverse/types";

function makeEvent(id: string, ingestedAtMs: number) {
  const row = new Array(31).fill(null);
  return { ...rowToLogVerseEvent(row), eventId: id, vendor: "cisco", ingestedAtMs };
}

describe("WebGLFallback", () => {
  const sources: SourceNode[] = [
    { sourceId: "s1", name: "Cisco ASA", vendor: "cisco", product: "asa", status: "active", enabled: true },
  ];

  it("shows an explanatory message and the real event data as a table", () => {
    const events = [makeEvent("ev-1", 1000)];
    render(<WebGLFallback sources={sources} events={events} onSelect={() => {}} />);
    expect(screen.getByText(/3D view unavailable/i)).toBeInTheDocument();
    expect(screen.getByText(/ev-1/)).toBeInTheDocument();
    expect(screen.getByText(/Cisco ASA/)).toBeInTheDocument();
  });

  it("shows an empty-state row when there are no buffered events", () => {
    render(<WebGLFallback sources={sources} events={[]} onSelect={() => {}} />);
    expect(screen.getByText(/No events buffered yet/i)).toBeInTheDocument();
  });

  it("is keyboard-accessible: a row is focusable and Enter selects it, as the equivalent of clicking a 3D particle", () => {
    const onSelect = vi.fn();
    const events = [makeEvent("ev-keyboard", 1000)];
    render(<WebGLFallback sources={sources} events={events} onSelect={onSelect} />);
    const row = screen.getByText(/ev-keyboard/).closest('[role="button"]')!;
    expect(row).toHaveAttribute("tabIndex", "0");
    fireEvent.keyDown(row, { key: "Enter" });
    expect(onSelect).toHaveBeenCalledWith(events[0]);
  });

  it("also selects on click, not only keyboard", () => {
    const onSelect = vi.fn();
    const events = [makeEvent("ev-click", 1000)];
    render(<WebGLFallback sources={sources} events={events} onSelect={onSelect} />);
    fireEvent.click(screen.getByText(/ev-click/));
    expect(onSelect).toHaveBeenCalledWith(events[0]);
  });
});
