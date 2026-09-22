import { afterEach, describe, expect, it, vi } from "vitest";

import { isWebglAvailable } from "../webgl";

describe("isWebglAvailable", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("returns true when the canvas can produce a webgl context", () => {
    vi.spyOn(HTMLCanvasElement.prototype, "getContext").mockReturnValue({} as never);
    expect(isWebglAvailable()).toBe(true);
  });

  it("returns false when getContext returns null for every variant", () => {
    vi.spyOn(HTMLCanvasElement.prototype, "getContext").mockReturnValue(null);
    expect(isWebglAvailable()).toBe(false);
  });

  it("returns false rather than throwing when getContext itself throws", () => {
    vi.spyOn(HTMLCanvasElement.prototype, "getContext").mockImplementation(() => {
      throw new Error("no GPU");
    });
    expect(isWebglAvailable()).toBe(false);
  });
});
