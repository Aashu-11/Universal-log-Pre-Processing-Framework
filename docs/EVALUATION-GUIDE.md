# LogKrama evaluator guide

This guide is a reproducible, offline-first tour of LogKrama. It maps each
claim to a command, endpoint, or visible outcome. Run the stack from the
repository root with `docker compose up --build -d` and open
`http://localhost:5173`.

| Capability | How to evaluate it | Expected evidence |
| --- | --- | --- |
| Multi-protocol ingest | Run `./bin/loggen.exe --proto=tcp --host=127.0.0.1 --port=6601 --eps=500 --duration=15 --vendors=all` | Live Theater receives events and Pipeline shows collector activity. |
| Parser quality | Run `go run ./cmd/logkramactl parser test --all` | Every shipped fixture passes. |
| Federated investigation | Open Explorer and run `SELECT count(*) FROM lake.logkrama.events` | A Parquet-lake result from the `lake` catalog. Then inspect `stream`, `meta`, and `vault` saved queries. |
| Chain of custody | Open Traceability, choose an event, then choose **Verify now** | Raw SHA-256 and the Merkle inclusion proof are returned by the vault. |
| Tamper resistance | In Reviewer Mode run the integrity proof after deliberately corrupting an isolated demo vault segment | Verification returns FAIL and identifies the corrupt segment; production data is never touched. |
| DLQ recovery | Open Dead Letter Queue, select an event, and choose replay | The event is re-emitted from its preserved reference and the audit entry records the action. |
| Air-gapped enrichment | Run `go test ./internal/enrich/... -run TestNoNetworkImports -v` | The static test proves enrichment does not import network-capable packages. |
| Parser onboarding | Submit an unfamiliar sample in Parser Workbench | The service mines templates, drafts a pack, and validates it with the same Go parser engine. |

## Break it on purpose

1. Send malformed lines. They must remain queryable and be copied to the
   Dead Letter Queue instead of silently disappearing.
2. Change a parser fixture so it no longer matches. `parser test --all` must
   fail before a pack can be trusted.
3. Stop Presto. The console must state that the query service is unavailable;
   it must not return made-up results.
4. Use a non-privileged role on an admin-only endpoint. The control plane must
   return an authorization failure.

## Requirement evidence

The Reviewer Mode page is the fastest evidence map. It runs real checks for
parser fixtures, integrity verification, source inventory, SQL allowlisting,
and air-gap checks. Each result includes its unedited command output.
