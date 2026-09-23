# LOGKRAMA — LogKrama

## What we are building
A system that ingests logs from any perimeter network device in any format,
preserves the original bytes losslessly with cryptographic integrity, normalizes
them into a single Universal Event Schema (UES), and exposes everything through
PrestoDB as a federated SQL query layer.

The core claim we must be able to demonstrate on demand:
**Ingest anything. Lose nothing. Prove everything. Onboard a new source in minutes.**

## Non-negotiable constraints

### Licensing
Every dependency, image, dataset and tool MUST be free and open source with no
paid tier, no trial, no account signup, and no license key.

### Air-gap
The running system must make ZERO outbound network calls. All enrichment data
(GeoIP, ASN, IOC, MITRE mappings) ships as files in the repo. All images are
pulled at build time only. Any code path that calls out to the internet at
runtime is a bug.

### Data integrity
Original event bytes are written to the Raw Vault BEFORE any parsing. Nothing in
the pipeline may mutate raw bytes. Every normalized event must carry a valid
pointer back to its exact original bytes plus a SHA-256.

### Parsers are data, not code
Adding support for a new log source must never require recompiling or restarting
the data plane. Parsers are YAML artifacts loaded and hot-swapped at runtime.

## Pinned stack

| Layer | Technology | License |
|---|---|---|
| Data plane | Go 1.22+ | BSD-3 |
| Query engine | PrestoDB (`prestodb/presto`) | Apache 2.0 |
| Metastore | Apache Hive standalone metastore + PostgreSQL | Apache 2.0 |
| Object store | MinIO (S3 API) | AGPL-3.0 |
| Stream buffer | Apache Kafka in KRaft mode | Apache 2.0 |
| Table format | Parquet on MinIO, Hive external tables | Apache 2.0 |
| Control plane | Python 3.11 + FastAPI + Pydantic v2 + SQLAlchemy | MIT/BSD |
| Metadata DB | PostgreSQL 16 | PostgreSQL License |
| Template mining | Drain3 | MIT |
| Console UI | React 18 + TypeScript + Vite + Tailwind + Recharts | MIT |
| Metrics | Prometheus + Grafana OSS | Apache 2.0 / AGPL |
| GeoIP/ASN | Synthetic CIDR-based dataset (see DECISIONS.md) | generated |
| Packaging | Docker + Docker Compose v2 | Apache 2.0 |

**Kafka runs in KRaft mode. No ZooKeeper.**

## Architecture — eight stages
INGEST -> PRESERVE -> IDENTIFY -> PARSE -> NORMALIZE -> ENRICH -> VALIDATE -> ROUTE

## Universal Event Schema — mandatory namespaces
`event.*`, `observer.*`, `src.*`, `dst.*`, `network.*`, `http.*`, `url.*`, `dns.*`,
`tls.*`, `actor.*`, `threat.*`, `enrich.*`, plus:
- `raw.*` — sha256, segment_id, offset, length, retrieval_uri
- `lineage.*` — parser_id, parser_version, parse_status, node_id, processing_ms
- `unmapped` — map<varchar,varchar> of every extracted field with no schema home

## Code standards
- Go: `gofmt`, table-driven tests, errors wrapped with context, no `panic` in
  library code, no per-event allocations in the hot path where avoidable.
- Python: type hints everywhere, Pydantic models for API boundaries.
- TypeScript: `strict: true`, no `any`.
- Every package gets tests in the same commit as the code.

## Anti-patterns — do not do these
- Do not hardcode sample data anywhere in the pipeline. Demo data comes from
  `tools/loggen` or a replayed corpus file through the real ingest path.
- Do not fake any output. Unimplemented paths are marked `// NOT IMPLEMENTED`.
- Do not add a dependency that pulls in a paid or account-gated service.
- Do not skip the acceptance gate at the end of a phase.
- Do not silently change the pinned stack — record deviations in DECISIONS.md.

## Environment note (this build)
This prototype was built on a Windows workstation with no Docker installed at
the time of the initial build session. Go and Node/Python toolchains were
verified locally (`go test`, `npm run build`, `pytest`). The Docker-Compose
based infra (Kafka/Presto/Hive/MinIO) is written to the pinned spec but the
`make up` / `make verify` / Presto federated-query gates require Docker Desktop
(with WSL2) to be installed and are pending verification on this machine —
see docs/DECISIONS.md for the exact status of each acceptance gate.

## Verification commands
```
make up / make demo / make test / make verify / make bench / make airgap
```
