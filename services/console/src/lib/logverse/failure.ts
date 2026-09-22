export type FailureFinding = { code: string; stage: "validate" | "unknown"; explanation: string; nextCheck: string };

const KNOWN: Record<string, Omit<FailureFinding, "code">> = {
  bad_ip: { stage: "validate", explanation: "At least one source or destination IP did not parse as an IP address.", nextCheck: "Inspect the raw IP field and the parser's IP mapping. The DLQ row does not identify which side failed." },
  bad_port: { stage: "validate", explanation: "At least one source or destination port was outside the allowed 0–65535 range.", nextCheck: "Inspect the raw port value and the parser's numeric conversion. The DLQ row does not identify which side failed." },
  schema_violation: { stage: "validate", explanation: "The normalized event did not satisfy the Universal Event Schema.", nextCheck: "Compare the parser mapping with the schema. The DLQ row does not contain the specific schema error." },
  timestamp_out_of_range: { stage: "validate", explanation: "The event timestamp fell outside the validator's allowed time window.", nextCheck: "Check source timezone, clock, and timestamp parsing. The DLQ row does not contain the rejected timestamp." },
};

export function explainDlqReason(reason: string | null): FailureFinding[] {
  const codes = (reason || "unknown").split(",").map((part) => part.trim()).filter(Boolean);
  return codes.map((code) => ({
    code,
    ...(KNOWN[code] ?? { stage: "unknown" as const, explanation: "The DLQ reason is not recognized by this viewer.", nextCheck: "Inspect the raw event and the processor logs for this reason code." }),
  }));
}
