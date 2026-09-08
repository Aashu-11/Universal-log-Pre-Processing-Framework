import { useEffect, useState } from "react";

import { Card } from "../components/ui/Card";
import { Button } from "../components/ui/Button";
import { Badge } from "../components/ui/Badge";
import { ErrorState } from "../components/ui/States";
import { CodeBlock } from "../components/ui/CodeBlock";
import { controlPlane, onboarding, ApiError } from "../lib/api";
import { canWrite, useAuth } from "../lib/auth";
import type { AnalyzeResponse, PublishResponse, RolloutStatus, TestResponse } from "../lib/types";

const SAMPLE_PLACEHOLDER = `srcip=203.0.113.5 dstip=10.0.0.10 srcport=51514 dstport=443 action="allow" bytes=6000
srcip=198.51.100.7 dstip=10.0.0.20 srcport=44321 dstport=22 action="deny" bytes=0`;

export function ParserWorkbench() {
  const { role } = useAuth();
  const [sampleText, setSampleText] = useState(SAMPLE_PLACEHOLDER);
  const [vendor, setVendor] = useState("acme");
  const [product, setProduct] = useState("firewall");
  const [parserYaml, setParserYaml] = useState("");
  const [mappingYaml, setMappingYaml] = useState("");
  const [testResult, setTestResult] = useState<TestResponse | null>(null);
  const [analyzeResult, setAnalyzeResult] = useState<AnalyzeResponse | null>(null);
  const [publishResult, setPublishResult] = useState<PublishResponse | null>(null);
  const [rollout, setRollout] = useState<RolloutStatus[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState<"infer" | "test" | "publish" | null>(null);

  const sampleLines = () => sampleText.split("\n").filter((l) => l.trim().length > 0);

  const inferFromSample = async () => {
    setBusy("infer");
    setError(null);
    try {
      const resp = await onboarding.post<AnalyzeResponse>("/v1/onboarding/analyze", {
        sample_lines: sampleLines(),
        vendor,
        product,
        event_class: "traffic",
      });
      setAnalyzeResult(resp);
      setParserYaml(resp.parser_yaml);
      setMappingYaml(resp.mapping_yaml);
    } catch (err) {
      setError(describeError(err));
    } finally {
      setBusy(null);
    }
  };

  const runTest = async () => {
    setBusy("test");
    setError(null);
    try {
      const resp = await onboarding.post<TestResponse>("/v1/onboarding/test", {
        parser_yaml: parserYaml,
        mapping_yaml: mappingYaml || null,
        sample_lines: sampleLines(),
      });
      setTestResult(resp);
    } catch (err) {
      setError(describeError(err));
    } finally {
      setBusy(null);
    }
  };

  // Debounced live re-parse whenever the YAML changes and a parser exists.
  useEffect(() => {
    if (!parserYaml.trim()) return;
    const handle = setTimeout(() => {
      runTest();
    }, 600);
    return () => clearTimeout(handle);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [parserYaml, mappingYaml]);

  const publish = async () => {
    setBusy("publish");
    setError(null);
    try {
      const resp = await controlPlane.post<PublishResponse>("/v1/parsers", {
        parser_yaml: parserYaml,
        mapping_yaml: mappingYaml,
        changelog: "published from Parser Workbench",
      });
      setPublishResult(resp);
      const rolloutResp = await controlPlane.get<RolloutStatus[]>(`/v1/parsers/${resp.parser_id}/rollout`);
      setRollout(rolloutResp);
    } catch (err) {
      setError(describeError(err));
    } finally {
      setBusy(null);
    }
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-[16px] font-semibold text-[var(--color-text)]">Parser Workbench</h1>
        <div className="flex gap-2">
          <input
            value={vendor}
            onChange={(e) => setVendor(e.target.value)}
            placeholder="vendor"
            className="w-28 rounded border border-[var(--color-border-strong)] bg-black/30 px-2 py-1 text-[11.5px] outline-none focus:border-[var(--color-accent)]"
          />
          <input
            value={product}
            onChange={(e) => setProduct(e.target.value)}
            placeholder="product"
            className="w-28 rounded border border-[var(--color-border-strong)] bg-black/30 px-2 py-1 text-[11.5px] outline-none focus:border-[var(--color-accent)]"
          />
          <Button onClick={inferFromSample} disabled={busy !== null}>
            {busy === "infer" ? "Inferring…" : "Infer from sample"}
          </Button>
        </div>
      </div>

      {error && <ErrorState message={error} />}

      <div className="grid grid-cols-3 gap-4">
        <Card title="Sample logs">
          <textarea
            value={sampleText}
            onChange={(e) => setSampleText(e.target.value)}
            spellCheck={false}
            className="h-64 w-full resize-y rounded border border-[var(--color-border-strong)] bg-black/40 p-2.5 font-mono-data text-[11.5px] text-[var(--color-text)] outline-none focus:border-[var(--color-accent)]"
          />
          {analyzeResult && (
            <div className="mt-2 flex flex-wrap gap-1.5">
              <Badge tone="accent">{analyzeResult.shape}</Badge>
              <Badge tone={analyzeResult.template_coverage_pct > 90 ? "success" : "warning"}>
                {`${analyzeResult.template_coverage_pct.toFixed(1)}% template coverage`}
              </Badge>
              <Badge tone="neutral">{`${analyzeResult.fields.length} fields inferred`}</Badge>
            </div>
          )}
        </Card>

        <Card title="Parser + Mapping YAML">
          <div className="space-y-2">
            <textarea
              value={parserYaml}
              onChange={(e) => setParserYaml(e.target.value)}
              placeholder="# Parser YAML — click 'Infer from sample' or paste your own"
              spellCheck={false}
              className="h-28 w-full resize-y rounded border border-[var(--color-border-strong)] bg-black/40 p-2.5 font-mono-data text-[11px] text-[var(--color-text)] outline-none focus:border-[var(--color-accent)]"
            />
            <textarea
              value={mappingYaml}
              onChange={(e) => setMappingYaml(e.target.value)}
              placeholder="# Mapping YAML"
              spellCheck={false}
              className="h-28 w-full resize-y rounded border border-[var(--color-border-strong)] bg-black/40 p-2.5 font-mono-data text-[11px] text-[var(--color-text)] outline-none focus:border-[var(--color-accent)]"
            />
          </div>
          <div className="mt-2 flex gap-2">
            <Button onClick={runTest} disabled={busy !== null || !parserYaml.trim()}>
              {busy === "test" ? "Testing…" : "Test"}
            </Button>
            {canWrite(role) && (
              <Button variant="primary" onClick={publish} disabled={busy !== null || !parserYaml.trim()}>
                {busy === "publish" ? "Publishing…" : "Publish"}
              </Button>
            )}
          </div>
        </Card>

        <Card title="Live output">
          {testResult ? (
            <>
              <div className="mb-2">
                <Badge tone={testResult.parse_success_rate > 0.8 ? "success" : "warning"}>
                  {`${(testResult.parse_success_rate * 100).toFixed(0)}% parse success`}
                </Badge>
              </div>
              <CodeBlock>{JSON.stringify(testResult.results, null, 2)}</CodeBlock>
            </>
          ) : (
            <div className="text-[11.5px] text-[var(--color-text-muted)]">
              Edit the YAML (or click Infer from sample) to see live re-parse output here.
            </div>
          )}
        </Card>
      </div>

      {publishResult && (
        <Card title="Publish result">
          <div className="flex flex-wrap items-center gap-2 text-[12px]">
            <Badge tone="success">published</Badge>
            <span className="font-mono-data">
              {publishResult.parser_id} v{publishResult.version}
            </span>
            <span className="text-[var(--color-text-muted)]">sha256:{publishResult.sha256.slice(0, 12)}…</span>
          </div>
          {rollout.length > 0 ? (
            <div className="mt-3 space-y-1">
              {rollout.map((r) => (
                <div key={r.node_id} className="flex justify-between text-[11.5px]">
                  <span className="font-mono-data text-[var(--color-text-secondary)]">{r.node_id}</span>
                  <span className="font-mono-data">{r.loaded_version}</span>
                </div>
              ))}
            </div>
          ) : (
            <div className="mt-2 text-[11px] text-[var(--color-text-muted)]">
              No processor nodes have reported a rollout for this parser yet (they report in via
              hot-reload — needs a running processor).
            </div>
          )}
        </Card>
      )}
    </div>
  );
}

function describeError(err: unknown): string {
  if (err instanceof ApiError) return err.message;
  return err instanceof Error ? err.message : "unexpected error";
}
