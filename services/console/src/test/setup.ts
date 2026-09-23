import { afterEach } from "vitest";
import { cleanup } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";

// @testing-library/react's own auto-cleanup relies on a global `afterEach`,
// which vitest only injects when `test.globals: true` is set — this repo's
// test style imports describe/it/expect explicitly instead, so cleanup is
// registered explicitly here to match.
afterEach(() => {
  cleanup();
});
