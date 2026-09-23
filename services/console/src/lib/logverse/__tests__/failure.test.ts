import { describe, expect, it } from "vitest";
import { explainDlqReason } from "../failure";

describe("DLQ failure explanations", () => {
  it("explains every comma-separated validation code without inventing a field", () => {
    const findings = explainDlqReason("schema_violation,bad_port");
    expect(findings.map((finding) => finding.code)).toEqual(["schema_violation", "bad_port"]);
    expect(findings.every((finding) => finding.stage === "validate")).toBe(true);
    expect(findings[1]?.nextCheck).toContain("does not identify which side");
  });

  it("labels unknown codes as unknown", () => {
    expect(explainDlqReason("new_code")[0]?.stage).toBe("unknown");
  });
});
