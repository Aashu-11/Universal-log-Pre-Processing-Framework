# LogKrama — Log Pre-Processing Platform

> **Ingest anything. Lose nothing. Prove everything. Onboard a new source in minutes.**

A vendor-neutral log pre-processing platform for perimeter network devices. LOGKRAMA ingests logs in
any format over any protocol, preserves the **original bytes** in a cryptographically chained Raw
Vault *before* parsing, normalizes them into a single **Universal Event Schema (UES)**, enriches
them entirely offline, and exposes raw bytes, normalized events, live streams and metadata through
**PrestoDB** as one federated SQL layer.

<p>
  <img alt="Go" src="https://img.shields.io/badge/Go-1.22%2B-00ADD8?logo=go&logoColor=white">
  <img alt="Python" src="https://img.shields.io/badge/Python-3.11-3776AB?logo=python&logoColor=white">
  <img alt="React" src="https://img.shields.io/badge/React-19-61DAFB?logo=react&logoColor=black">
  <img alt="TypeScript" src="https://img.shields.io/badge/TypeScript-strict-3178C6?logo=typescript&logoColor=white">
  <img alt="PrestoDB" src="https://img.shields.io/badge/PrestoDB-0.286-5890FF">
  <img alt="Kafka" src="https://img.shields.io/badge/Kafka-KRaft-231F20?logo=apachekafka&logoColor=white">
  <img alt="MinIO" src="https://img.shields.io/badge/MinIO-S3%20API-C72E49?logo=minio&logoColor=white">
  <img alt="Docker Compose" src="https://img.shields.io/badge/Docker-Compose%20v2-2496ED?logo=docker&logoColor=white">
  <img alt="Air-gapped" src="https://img.shields.io/badge/runtime-air--gapped-success">
  <img alt="FOSS only" src="https://img.shields.io/badge/dependencies-100%25%20FOSS-brightgreen">
</p>

---

## Table of Contents

- [Overview](#overview)
- [Features](#features)
- [Tech Stack](#tech-stack)
- [How It Works](#how-it-works)
- [Installation & Setup](#installation--setup)
- [Usage](#usage)
- [Project Structure](#project-structure)
- [Screenshots / Demo](#screenshots--demo)

---

## Overview

Security teams running firewalls, IDS/IPS and proxies from different vendors end up with N
incompatible log formats, N sets of queries, and no way to prove that what landed in the SIEM is
what the device actually emitted. LOGKRAMA solves that with four hard guarantees baked into the
architecture:

| Guarantee | How it is enforced in code |
|---|---|
| **Lossless preservation** | Raw bytes are written to the Raw Vault *before* any parsing. Nothing in the pipeline mutates them. Every normalized event carries `raw.sha256`, `raw.segment_id`, `raw.offset`, `raw.length` and `raw.retrieval_uri`. |
| **Provable integrity** | Every vault segment is Merkle-hashed, chained to the previous segment's root, and recorded in a per-day append-only ledger. `logkramactl vault verify` / `prove` re-derive the chain and emit per-event inclusion proofs. |
| **Zero field loss** | Every extracted field either lands on a UES path or is retained verbatim in `unmapped map<varchar,varchar>`. Events that fail to parse are still emitted (as `FailedEvent`) and are still queryable — nothing is ever dropped. |
| **Air-gapped runtime** | All enrichment (GeoIP, ASN, IOC, MITRE, asset, identity) ships as CSV files in the repo. A test (`TestNoNetworkImports`) statically proves the enrichment package imports no network-capable package. |

Adding a new log source requires **no code and no restart**: parsers are YAML artifacts, hot-swapped
at runtime via an `fsnotify` watch on `packs/`.

---

## Features

### Ingest (`logkrama-collector`)
- **Syslog UDP** — non-blocking; a full buffer drops immediately and *visibly* (`logkrama_udp_drops_total`) rather than building an unbounded queue.
- **Syslog TCP** — octet-counting/newline framing, backpressure by pausing reads instead of dropping.
- **HTTP bulk** — `POST /v1/ingest`, NDJSON (optionally gzipped), 32 MB body cap, token-bucket rate limiting, `429` on overflow. Lines are stored byte-for-byte, never re-serialized.
- **File tail** — directory watcher with a persisted byte-offset checkpoint, so a restart neither skips nor duplicates lines.
- **Bounded ring buffer + disk spool** per listener, with spool depth exported to Prometheus.
- **Multiline joining & truncation detection** that never transcodes or corrupts malformed UTF-8.

### Preserve (Raw Vault)
- Append-only, zstd-compressed segments in MinIO (or a local directory for dev).
- Seals on **64 MB or 5 minutes**, whichever comes first, plus a background wall-clock seal ticker so a source going quiet cannot leave a segment unsealed indefinitely.
- Per-segment Merkle tree + `prev_root` chaining; daily `ledger.jsonl`; chain tip bootstrapped from the ledger on startup so restarts don't break the chain.
- Bounded LRU decompressed-segment cache so reading N events from one segment costs one object GET, not N.
- `Read` verifies SHA-256 on every retrieval and returns an error on mismatch — never silently.

### Identify → Parse → Normalize
- **Three-tier identification**: explicit binding (`listener:peer` → parser), structural sniff (JSON/CEF/LEEF/syslog), then signature match on each parser's YAML predicates by priority. Unknown sources are tagged `source.unknown` and queued for onboarding, never dropped.
- **19 declarative DSL operators**: `syslog`, `dissect`, `grok`, `regex`, `json`, `xml`, `csv`, `kv`, `cef`, `leef`, `date`, `convert`, `lookup`, `rename`, `copy`, `drop_field`, `gsub`, `split`, `conditional`.
- **Parsers are data**: each source is a `<class>.yaml` (extraction) + `<class>.mapping.yaml` (UES projection) pair. Splitting the two means the same extraction can be re-targeted at UES/ECS/OCSF/CEF by swapping only the mapping file.
- **Hot reload** via `fsnotify`; a bad publish is rejected and the previously working plan stays live.
- Shipped packs: `paloalto.panos.traffic`, `fortinet.fortigate.traffic`, `cisco.asa.302013`, plus an `acme.firewall.traffic` example produced through the onboarding flow.
- **Centralized timestamp resolution**: multi-format, IANA timezones, numeric epochs (s/ms/ns), and December→January rollover-aware year inference for formats (like RFC3164) that carry no year.
- **Value dictionaries** (`config/dictionaries/`) normalize vendor vocabulary for action, outcome, severity, protocol, direction, observer type and threat category.
- **Alternate shape renderers**: the same UES event can be emitted as ECS, OCSF or CEF (`internal/normalize/shape`).

### Enrich (offline, 8 enrichers)
`geoip` → `asn` → `cidr_classify` → `asset` → `identity` → `ioc` (bloom pre-filter + exact match) →
`mitre` → `risk_score`. All from in-repo CSV datasets with a `MANIFEST.sha256`; ordered so the risk
scorer reads fields the earlier stages set.

### Validate & Route
- JSON-Schema validation against the embedded `schema/ues-1.0.json`, plus port range, IP parseability and ±1-year timestamp-skew checks.
- `quality.score = (mapped_fields / expected_fields) × parse_confidence`, clamped to `[0,1]`.
- Router writes **every** event to the lake and the live stream regardless of outcome; violations *additionally* get a DLQ copy tagged with a reason code and bump `logkrama_dlq_total{reason}`.

### Query (PrestoDB federation)
Four catalogs, one SQL dialect:

| Catalog | Connector | Contents |
|---|---|---|
| `lake` | `hive-hadoop2` | Normalized UES events, Parquet on MinIO, partitioned `dt`/`hour`/`vendor` |
| `stream` | `kafka` | Live normalized events (30-minute topic retention) |
| `meta` | `postgresql` | Parser registry, source inventory, audit log, DLQ, rollout status |
| `vault` | `hive-hadoop2` | Per-event raw index + per-segment Merkle roots |

Seven documented demo queries (`docs/QUERIES.md`, Q1–Q7) cover cross-vendor visibility, hot+cold
`UNION ALL`, a three-catalog join, chain-of-custody traceability, a losslessness proof, parser-health
regression detection, and ML feature extraction — all pre-loaded in the console's Explorer page.

### Control Plane (FastAPI)
JWT auth with four RBAC roles (`admin`, `engineer`, `analyst`, `auditor` — read-only), full audit log,
and these endpoint groups:

| Endpoint | Purpose |
|---|---|
| `POST /v1/auth/login` | Issue JWT |
| `/v1/sources` | Source inventory CRUD with listener/peer bindings |
| `/v1/parsers` | Publish (lint + golden-fixture validated), list, versions, rollback, per-node rollout status |
| `/v1/events/{id}/raw`, `/trace` | Original bytes with SHA-256 verification + Merkle inclusion proof |
| `/v1/integrity/verify` | Merkle chain verification over a date range |
| `/v1/dlq`, `/v1/dlq/replay` | Dead-letter inspection and resolution |
| `/v1/stats/pipeline`, `/sources` | Live Prometheus scrape of collector/processor + source inventory |
| `POST /v1/query` | Read-only Presto proxy — `SELECT`/`EXPLAIN`/`SHOW` allowlist enforced **before** any connection is opened |
| `/v1/reviewer/*` | "Prove it" endpoints that shell out to real checks and return real output, pass or fail |

Parser publish deliberately **shells out to the compiled `logkramactl` binary** for DSL linting,
unsafe-regex rejection and golden-fixture validation — one validator shared by Go and Python instead
of two implementations that can silently diverge.

### Onboarding Engine (FastAPI + Drain3)
Paste an unknown log sample → get a working parser. Real Drain3 template mining, ordered type
detectors (`ipv4`, `ipv6`, `mac`, `timestamp`, `url`, `hash`, `verdict`, `severity`, `port`,
`byte_count`, `username`, `integer`, `float`, `enum`, `free_text`), field-name inference,
shape-driven draft generation (json/kv/csv/dissect), and Logstash-grok + CEF importers. Every number
returned (template coverage, per-field success rate) comes from actually executing the candidate
parser through the Go engine.

### Console (React + TypeScript)
Eight authenticated pages plus a login screen:

| Page | What it does |
|---|---|
| **Live Theater** | Walks a single live event through all eight pipeline stages, one provable step at a time |
| **Pipeline** | Rolling 2-minute EPS / bytes / drops / DLQ charts from real Prometheus counters |
| **Sources** | Source inventory and binding management |
| **Explorer** | Federated SQL console with the four catalogs and Q1–Q7 pre-loaded |
| **Traceability** | Click any recent event → raw bytes (hex or text), SHA-256 verification, Merkle proof |
| **Parser Workbench** | Paste a sample → analyze → live re-parse → publish |
| **DLQ** | Group by reason, inspect, bulk-resolve |
| **Reviewer Mode** | One "Prove it" button per requirement (a)–(k), each wired to a real backend check |

### Observability
Prometheus metrics from both data-plane binaries (`logkrama_events_received_total`, `logkrama_udp_drops_total`,
`logkrama_ingest_bytes_total`, `logkrama_spool_depth`, `logkrama_vault_write_duration_seconds`,
`logkrama_parse_duration_seconds`, `logkrama_enrich_duration_seconds`, `logkrama_dlq_total`), a provisioned
Grafana dashboard, and `/debug/pprof/*` on the collector's metrics listener for stall diagnosis.

---

## Tech Stack

| Layer | Technology | License |
|---|---|---|
| Data plane | Go (module declares `go 1.27.0`; builds on 1.22+ with automatic toolchain fetch) | BSD-3 |
| Query engine | PrestoDB `0.286` | Apache-2.0 |
| Metastore | Apache Hive 4.0.0 standalone metastore + PostgreSQL 16 | Apache-2.0 |
| Object store | MinIO (S3 API) | AGPL-3.0 |
| Stream buffer | Apache Kafka 3.7.0 — **KRaft mode, no ZooKeeper** | Apache-2.0 |
| Table format | Parquet on MinIO via Hive external tables | Apache-2.0 |
| Control plane | Python 3.11 + FastAPI + Pydantic v2 + SQLAlchemy 2 | MIT/BSD |
| Metadata DB | PostgreSQL 16 (`logkrama_meta`) | PostgreSQL License |
| Template mining | Drain3 | MIT |
| Console | React 19 + TypeScript (`strict`) + Vite + Tailwind v4 + Recharts + Oxlint | MIT |
| Metrics | Prometheus + Grafana OSS | Apache-2.0 / AGPL-3.0 |
| GeoIP / ASN / IOC / MITRE | Synthetic, generated in-repo (`tools/gen-enrichment`) | generated |
| Packaging | Docker + Docker Compose v2 | Apache-2.0 |

**Notable Go dependencies:** `klauspost/compress` (zstd), `minio-go/v7`, `parquet-go`, `segmentio/kafka-go`,
`prestodb/presto-go-client`, `santhosh-tekuri/jsonschema/v5`, `fsnotify`, `spf13/cobra`,
`prometheus/client_golang`.

Every dependency, image and dataset is free and open source with no paid tier, trial, account signup
or license key — see `docs/LICENSES.md` for the full audit.

---

## How It Works

### Eight stages

```
INGEST → PRESERVE → IDENTIFY → PARSE → NORMALIZE → ENRICH → VALIDATE → ROUTE
```

### Runtime topology

```
                  syslog UDP/TCP · HTTP bulk NDJSON · file tail
                                    │
                          ┌─────────▼──────────┐
                          │   logkrama-collector   │  INGEST + PRESERVE
                          │  buffer → spool    │
                          └────┬──────────┬────┘
                               │          │
                 raw bytes ────┘          └──── raw refs
                               │                    │
                    ┌──────────▼─────────┐   ┌──────▼──────────────┐
                    │ Raw Vault (MinIO)  │   │ Kafka: raw.refs     │
                    │ zstd segments +    │   │ (KRaft)             │
                    │ Merkle ledger      │   └──────┬──────────────┘
                    └──────────┬─────────┘          │
                               │  read + SHA verify │
                          ┌────▼────────────────────▼────┐
                          │       logkrama-processor         │
                          │ IDENTIFY→PARSE→NORMALIZE→    │
                          │ ENRICH→VALIDATE→ROUTE        │
                          └──┬──────────┬──────────┬─────┘
                             │          │          │
                  Parquet ───┘          │          └─── DLQ topic
                  (logkrama-lake)     normalized topic
                             │          │          │
                       ┌─────▼──────────▼──────────▼─────┐
                       │  PrestoDB  lake · stream · meta │
                       │            · vault             │
                       └─────┬───────────────────────────┘
                             │
        ┌────────────────────▼─────────┐   ┌─────────────────────┐
        │ control-plane (FastAPI/JWT)  │◄──│ onboarding (Drain3) │
        └────────────────┬─────────────┘   └─────────────────────┘
                         │
               ┌─────────▼─────────┐
               │ console (React)   │
               └───────────────────┘
```

### The handoff that makes losslessness work

The collector returns from `WriteBatch` as soon as bytes are appended to the open segment (required
for high EPS) and publishes each event's `RawRef` to Kafka immediately. The segment itself seals on
its own schedule — 64 MB or `LOGKRAMA_VAULT_SEGMENT_MAX_SECONDS`, whichever first — so a ref can reach
Kafka up to a full seal window before its bytes are readable. The processor therefore retries vault
reads with exponential backoff up to `LOGKRAMA_VAULT_READ_RETRY_SECONDS` (default **330s** = the 300s
default seal window + 30s margin), and the collector runs a wall-clock seal ticker so a quiet source
can't leave a segment open forever. Both of these exist because the naive versions silently dropped
valid events under real load — see `docs/DECISIONS.md` D-015 and D-018.

### Durability boundary

Durability begins at **segment seal**, not at `WriteBatch` return. Killing the collector
*ungracefully* loses whatever sits in the still-open segment; `SIGTERM`/`SIGINT` seals before exit.
Deployments wanting a tighter crash-loss window should lower `LOGKRAMA_VAULT_SEGMENT_MAX_BYTES` /
`_SECONDS` rather than fsync per event. Documented in full as `docs/DECISIONS.md` D-007.

---

## Installation & Setup

### Prerequisites

| For | Requirement |
|---|---|
| Full stack | Docker Desktop / Engine + Docker Compose v2 (WSL2 on Windows) |
| Go data plane & CLI | Go 1.22+ (`go.mod` declares `go 1.27.0`; the toolchain is fetched automatically) |
| Python services | Python 3.11 |
| Console | Node.js 20+ |
| `make` targets | GNU Make + Bash (the Makefile targets a Linux/CI shell) |

### 1. Clone and configure

```bash
git clone <your-fork-url> Universal-log-Pre-Processing-Framework
cd Universal-log-Pre-Processing-Framework
cp .env.example .env          # dev-only defaults matching docker-compose.yml
cp services/console/.env.example services/console/.env
```

Optional — generate a self-signed cert for the TLS syslog listener:

```bash
make certs
```

### 2. Bring the stack up

```bash
make up          # docker compose up -d --build, then waits for healthchecks
```

This starts 12 long-running services (Postgres, MinIO, Kafka in KRaft mode, Hive Metastore, Presto,
Prometheus, Grafana, collector, processor, control-plane, onboarding, console) plus two one-shot
init jobs that create the MinIO buckets and Kafka topics.

| Service | URL | Credentials |
|---|---|---|
| Console | http://localhost:5173 | `admin` / `admin` |
| Control API (OpenAPI docs) | http://localhost:8000/docs | JWT via `/v1/auth/login` |
| Onboarding API | http://localhost:8001/docs | — |
| Presto UI | http://localhost:8080 | — |
| MinIO console | http://localhost:9001 | `logkramaadmin` / `logkrama_dev_only` |
| Grafana | http://localhost:3000 | anonymous viewer |
| Prometheus | http://localhost:9090 | — |

> All credentials in `.env.example` and `docker-compose.yml` are dev-only placeholders. Change them
> before any non-local deployment, and set `LOGKRAMA_JWT_SECRET` / `LOGKRAMA_ADMIN_PASSWORD`.

### 3. Apply the Presto DDL

```bash
docker exec -i logkrama-presto presto-cli --server localhost:8080 < schema/presto/ddl.sql
```

### 4. Verify

```bash
make verify      # PASS/FAIL table across infra, app layer and all four Presto catalogs
```

### Ingest ports

Unprivileged by default so containers need no `root` / `CAP_NET_BIND_SERVICE`
(`docs/DECISIONS.md` D-006):

| Protocol | Default | Env var |
|---|---|---|
| Syslog UDP | `5514` | `LOGKRAMA_SYSLOG_UDP_ADDR` |
| Syslog TCP | `6601` | `LOGKRAMA_SYSLOG_TCP_ADDR` |
| Syslog TLS | `6614` | `LOGKRAMA_SYSLOG_TLS_ADDR` |
| HTTP bulk | `8088` | `LOGKRAMA_HTTP_ADDR` |
| Collector metrics | `9100` | `--metrics-addr` |
| Processor metrics | `9101` | `--metrics-addr` |

### Local development without Docker

Every component runs standalone. The collector falls back to a local-filesystem vault when
`MINIO_ENDPOINT` is unset, and to a no-op ref publisher when `KAFKA_BROKERS` is unset — so the
ingest → preserve → verify path is fully exercisable with nothing but Go installed.

```bash
# Go data plane + CLI
go build ./...
go build -o bin/logkramactl ./cmd/logkramactl        # required by the control-plane parser tests
go run ./cmd/logkrama-collector                  # local vault at ./data/vault

# Control plane (SQLite dev fallback, see DECISIONS.md D-004)
cd services/control-plane && pip install -r requirements.txt -r requirements-dev.txt
uvicorn app.main:app --reload --port 8000

# Onboarding engine
cd services/onboarding && pip install -r requirements.txt -r requirements-dev.txt
uvicorn app.main:app --reload --port 8001

# Console
cd services/console && npm ci && npm run dev
```

---

## Usage

### End-to-end demo

```bash
make demo
```

Brings the stack up, waits for health, sends **5,000 real synthetic events** through the real TCP
ingest path, waits for the segment-seal window, syncs Presto partitions, then prints live event
counts, a per-vendor breakdown and a Merkle chain verification. No canned output — every number comes
from a live command.

### Generate load

`tools/loggen` produces syntactically correct vendor logs from documented format templates (never
copied from real logs) and sends them through the real ingest protocols:

```bash
go run ./tools/loggen --proto=tcp  --port=6601 --eps=20000 --duration=60s --vendors=all
go run ./tools/loggen --proto=udp  --port=5514 --eps=5000  --vendors=paloalto,fortinet
go run ./tools/loggen --proto=http --port=8088 --eps=1000
go run ./tools/loggen --proto=tcp  --port=6601 --anomaly=port-scan     # feeds the Q7 demo
```

### Operator CLI — `logkramactl`

```bash
# Chain of custody
logkramactl vault verify --from 2026-09-09 --to 2026-09-09     # PASS/FAIL table per segment
logkramactl vault prove  --segment-id <id> --offset N --length N --sha256 <hex>
logkramactl vault read   --segment-id <id> --offset N --length N --sha256 <hex> --json
echo 'a raw log line' | logkramactl vault write
logkramactl vault export-index --from 2026-09-09 --to 2026-09-09   # populates vault.logkrama.raw_segments

# Parsers
logkramactl parser test --all                     # golden-fixture regression (75 fixtures)
logkramactl parser lint packs/<v>/<p>/<c>.yaml    # DSL validation + unsafe-regex rejection
cat sample.log | logkramactl parser run --parser draft.yaml --mapping draft.mapping.yaml --shapes

# Presto / demo data
logkramactl partitions sync                        # register new dt=/hour=/vendor= partitions
logkramactl gen sonicwall --count 200              # a format no shipped pack has ever seen
```

### Onboard a new source in minutes

1. Open **Parser Workbench** in the console (or `POST /v1/onboarding/analyze` directly) and paste
   ~100–200 raw sample lines.
2. Drain3 mines templates, types and names the fields, and generates a draft `parser.yaml` +
   `mapping.yaml`. The draft is immediately executed against your sample, so the reported coverage
   and per-field success rates are measured, not estimated.
3. Iterate in the editor, hit re-parse, then **Publish** — the control plane lints the artifact, runs
   its fixtures, records it in `parser_registry`, and writes it into `packs/`.
4. The processor's `fsnotify` watch hot-swaps it within one filesystem-event round trip. No restart,
   no rebuild.

Rollback is one call: `POST /v1/parsers/{parser_id}/rollback`.

### Query federated

```sql
-- Chain of custody: normalized event → its exact original bytes → the Merkle root that seals them
SELECT e.event_id, e.raw_sha256, e.raw_segment_id, e.raw_offset, e.raw_length,
       s.merkle_root, s.prev_root, s.sealed_at_ns
FROM lake.logkrama.events e
JOIN vault.logkrama.raw_segments s ON e.raw_segment_id = s.segment_id
WHERE e.event_id = ?;

-- Losslessness: missing_raw_ref must be 0, every vendor, every day
SELECT observer_vendor, count(*) AS events,
       avg(cardinality(unmapped)) AS avg_unmapped_fields,
       sum(CASE WHEN raw_sha256 IS NULL THEN 1 ELSE 0 END) AS missing_raw_ref
FROM lake.logkrama.events
WHERE dt = current_date
GROUP BY 1;
```

All seven queries with explanations: `docs/QUERIES.md`.

### Tests & lint

```bash
make test        # go test ./... -race  +  pytest (both services)  +  npm run build
make lint        # gofmt + go vet + ruff + black + oxlint
```

Coverage: 18 Go packages with table-driven tests (including a `FuzzParseAllPacks` fuzz target and
static air-gap assertions), pytest suites for both Python services (the control-plane parser tests
invoke the real Go binary as a subprocess — a genuine cross-language integration test), and a
`strict`-mode TypeScript build. CI (`.github/workflows/ci.yml`) runs all three toolchains on every
push and PR.

---

## Project Structure

```
.
├── cmd/
│   ├── logkrama-collector/       # INGEST + PRESERVE binary (listeners, vault, ref publish)
│   ├── logkrama-processor/       # IDENTIFY→ROUTE binary (Kafka consumer)
│   └── logkramactl/              # operator CLI: vault, parser, partitions, gen
├── internal/
│   ├── collector/            # pipeline, envelope, Kafka/fanout publishers
│   │   ├── batch/            # bounded ring buffer + disk-backed spool
│   │   ├── framing/          # multiline joining, truncation detection
│   │   └── listener/         # udp, tcp, tls, http, file
│   ├── vault/                # segments, Merkle tree, ledger, LRU cache
│   │   └── store/            # Store interface: local FS + MinIO
│   ├── identify/             # three-tier source/parser resolution
│   ├── parse/                # registry, compiler, fsnotify hot reload
│   │   ├── dsl/              # YAML parser-artifact schema + predicates
│   │   ├── fields/           # extracted-field container
│   │   └── ops/              # the 19 operators
│   ├── normalize/            # mapper, dictionaries, timestamps, failed events
│   │   └── shape/            # UES / ECS / OCSF / CEF renderers
│   ├── enrich/               # geoip, asn, cidr, asset, identity, ioc, mitre, risk
│   ├── validate/             # JSON-Schema + range rules + quality score
│   ├── route/                # lake / stream / DLQ fan-out
│   ├── sink/                 # parquet, kafka, vaultindex
│   ├── processor/            # the full per-event pipeline, unit-testable
│   ├── schema/               # UES Go types, flat Parquet row, UUIDv7
│   ├── telemetry/            # Prometheus metric definitions
│   └── loggen/               # vendor log generators (shared by CLI + tools)
├── schema/
│   ├── ues-1.0.json          # Universal Event Schema (embedded in the binary)
│   └── presto/ddl.sql        # lake.logkrama.events, vault.logkrama.raw_{segments,index}
├── packs/                    # parser + mapping YAML artifacts (data, not code)
│   ├── paloalto/panos/       ├── fortinet/fortigate/
│   ├── cisco/asa/            └── acme/firewall/traffic/
├── config/dictionaries/      # action, outcome, severity, protocol, direction, …
├── enrichment/               # synthetic geoip/asn/ioc/mitre/asset/identity CSVs + MANIFEST
├── services/
│   ├── control-plane/        # FastAPI: auth, sources, parsers, events, integrity, dlq, stats, query, reviewer
│   ├── onboarding/           # FastAPI + Drain3: templates, type/name inference, draftgen, importers
│   └── console/              # React + TS + Vite + Tailwind (8 pages)
├── tools/
│   ├── loggen/               # multi-protocol load generator
│   ├── gen-corpus/           # golden-fixture corpus generator
│   └── gen-enrichment/       # synthetic enrichment dataset generator
├── deploy/                   # Dockerfiles, Presto catalogs, Hive, Grafana, Postgres init
├── testdata/golden/          # 75 golden fixtures (25 × 3 vendors)
├── scripts/                  # demo.sh, verify.sh
├── docs/                     # BUILD_PLAN.md, DECISIONS.md, QUERIES.md, LICENSES.md, LOGVERSE.md
├── docker-compose.yml
└── Makefile
```

---

## Screenshots / Demo

<!-- TODO: add a LogVerse screenshot here (console → LogVerse, live scene with a few particles in flight) once captured from a running stack. Not committed yet — see docs/LOGVERSE.md for the feature writeup and demo flow in the meantime. -->

---
