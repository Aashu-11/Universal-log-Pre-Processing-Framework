import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { TimelineControls } from "../TimelineControls";

function baseProps() {
  return {
    mode: "live" as const,
    speed: 1 as const,
    clockMs: 1_700_000_000_000,
    bounds: { minMs: 1_700_000_000_000 - 60_000, maxMs: 1_700_000_000_000 },
    status: "live" as const,
    eps: { eps: 42.5, overSeconds: 5 },
    visibleCount: 12,
    overflowCount: 0,
    droppedEventsTotal: 3,
    dlqCount: 2,
    reducedMotion: false,
    reducedMotionOverride: false,
    onReducedMotionOverrideChange: vi.fn(),
    onGoLive: vi.fn(),
    onPause: vi.fn(),
    onPlayReplay: vi.fn(),
    onSpeedChange: vi.fn(),
    onScrub: vi.fn(),
    onResetCamera: vi.fn(),
  };
}

describe("TimelineControls", () => {
  it("renders the connection status label for each ConnectionStatus", () => {
    const { rerender } = render(<TimelineControls {...baseProps()} status="live" />);
    expect(screen.getByText("Live")).toBeInTheDocument();
    rerender(<TimelineControls {...baseProps()} status="reconnecting" />);
    expect(screen.getByText("Reconnecting…")).toBeInTheDocument();
    rerender(<TimelineControls {...baseProps()} status="error" />);
    expect(screen.getByText("Unavailable")).toBeInTheDocument();
  });

  it("marks the active timeline mode button as pressed via aria-pressed, not color alone", () => {
    render(<TimelineControls {...baseProps()} mode="replay" />);
    expect(screen.getByRole("button", { name: /replay/i })).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByRole("button", { name: /● live/i })).toHaveAttribute("aria-pressed", "false");
  });

  it("calls the right handler for each timeline button", () => {
    const props = baseProps();
    render(<TimelineControls {...props} />);
    fireEvent.click(screen.getByRole("button", { name: /pause/i }));
    expect(props.onPause).toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: /replay/i }));
    expect(props.onPlayReplay).toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: /reset view/i }));
    expect(props.onResetCamera).toHaveBeenCalled();
  });

  it("labels the EPS figure as derived, never as a measured rate", () => {
    render(<TimelineControls {...baseProps()} />);
    expect(screen.getByText(/derived/i)).toBeInTheDocument();
  });

  it("shows real dropped-packet count distinctly from the visualization's aggregated-overflow count", () => {
    render(<TimelineControls {...baseProps()} overflowCount={7} droppedEventsTotal={40} />);
    expect(screen.getByText(/40 dropped/)).toBeInTheDocument();
    expect(screen.getByText(/\+7 aggregated/)).toBeInTheDocument();
  });

  it("the scrub slider is keyboard-focusable and labeled for assistive tech", () => {
    render(<TimelineControls {...baseProps()} />);
    const slider = screen.getByLabelText(/scrub timeline/i);
    expect(slider).toBeInTheDocument();
    expect(slider.tagName).toBe("INPUT");
  });
});
