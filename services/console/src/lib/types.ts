// Shared response types, matching the Pydantic models in
// services/control-plane/app and services/onboarding/app exactly — see
// each field's originating router if a shape looks surprising.

export type Role = "admin" | "engineer" | "analyst" | "auditor";

export interface TokenResponse {
  access_token: string;
  token_type: string;
  role: Role;
}

export interface Source {
  log_source_id: string;
  name: string;
  vendor: string;
  product: string;
  observer_type: string;
  binding_peer_ip: string | null;
  binding_listener: string | null;
  parser_id: string | null;
  timezone: string;
  enabled: boolean;
  first_seen: string;
  last_seen: string;
  status: string;
}

export interface ParserVersion {
  parser_id: string;
  version: string;
  pack_id: string;
  sha256: string;
  state: string;
  published_at: string;
  published_by: string;
  changelog: string | null;
}

export interface PublishResponse {
  parser_id: string;
  version: string;
  state: string;
  lint_output: string;
  fixture_output: string;
  sha256: string;
}

export interface RolloutStatus {
  node_id: string;
  parser_id: string;
  loaded_version: string;
  reported_at: string;
}

export interface VerifyResponse {
  passed: boolean;
  output: string;
}

export interface DLQEventOut {
  event_id: string;
  raw_ref: Record<string, unknown>;
  reason: string;
  parser_id: string | null;
  occurred_at: string;
  resolved: boolean;
}

export interface PipelineStats {
  collector_reachable: boolean;
  processor_reachable: boolean;
  events_received_total: number;
  udp_drops_total: number;
  ingest_bytes_total: number;
  dlq_total: number;
}

export interface SourceStats {
  log_source_id: string;
  name: string;
  vendor: string;
  product: string;
  parser_id: string | null;
  status: string;
  last_seen: string;
}

export interface QueryResponse {
  columns: string[];
  rows: unknown[][];
  row_count: number;
}

export interface OnboardingField {
  name: string;
  inferred_type: string;
  confidence: number;
  ues_path: string | null;
  mapped: boolean;
}

export interface TemplateSummary {
  template: string;
  size: number;
  coverage_pct: number;
}

export interface SampleComparison {
  raw: string;
  status: string;
  extracted_field_count: number;
  mapped: Record<string, unknown> | null;
  unmapped: Record<string, string> | null;
}

export interface AnalyzeResponse {
  parser_id: string;
  shape: string;
  template_coverage_pct: number;
  template_count: number;
  top_templates: TemplateSummary[];
  fields: OnboardingField[];
  parser_yaml: string;
  mapping_yaml: string;
  parse_success_rate: number;
  per_field_success_rate: Record<string, number>;
  sample_comparison: SampleComparison[];
  lint_ok: boolean;
  lint_output: string;
}

export interface TestResponse {
  parse_success_rate: number;
  results: Array<{
    raw: string;
    status: string;
    fields: Record<string, unknown>;
    mapped?: Record<string, unknown>;
    unmapped?: Record<string, string>;
  }>;
}

export interface ApiErrorBody {
  detail?: string;
}
