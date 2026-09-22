import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { LogVerseFilters } from "../LogVerseFilters";
import { defaultFilter } from "../../../lib/logverse/types";

describe("LogVerseFilters", () => {
  it("toggles a severity state off when its button is clicked", () => {
    const onChange = vi.fn();
    render(<LogVerseFilters filter={defaultFilter()} onChange={onChange} vendors={[]} outcomes={[]} />);
    fireEvent.click(screen.getByRole("button", { name: "Threat / IOC" }));
    expect(onChange.mock.calls[0]).toBeDefined();
    const next = onChange.mock.calls[0]![0];
    expect(next.states.has("threat")).toBe(false);
    expect(next.states.has("normal")).toBe(true); // untouched
  });

  it("dlqOnly checkbox is a real accessible checkbox toggling filter.dlqOnly", () => {
    const onChange = vi.fn();
    render(<LogVerseFilters filter={defaultFilter()} onChange={onChange} vendors={[]} outcomes={[]} />);
    const checkbox = screen.getByRole("checkbox", { name: /DLQ events only/i });
    fireEvent.click(checkbox);
    expect(onChange.mock.calls[0]![0].dlqOnly).toBe(true);
  });

  it("shows vendor buttons only when vendors are supplied, and toggling one narrows the vendor set", () => {
    const onChange = vi.fn();
    render(<LogVerseFilters filter={defaultFilter()} onChange={onChange} vendors={["cisco", "fortinet"]} outcomes={[]} />);
    fireEvent.click(screen.getByRole("button", { name: "cisco" }));
    expect(onChange.mock.calls[0]![0].vendors.has("cisco")).toBe(true);
  });

  it("every interactive control is a real <button> or <input>, reachable by keyboard without any canvas dependency", () => {
    render(<LogVerseFilters filter={defaultFilter()} onChange={() => {}} vendors={["cisco"]} outcomes={["allowed"]} />);
    screen.getAllByRole("button").forEach((btn) => expect(btn.tagName).toBe("BUTTON"));
    screen.getAllByRole("checkbox").forEach((cb) => expect(cb.tagName).toBe("INPUT"));
  });
});
