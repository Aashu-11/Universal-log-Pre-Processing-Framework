# Decisions Log

One short entry per non-obvious choice, newest first.

## D-019 — Merkle chain tip must be bootstrapped from the ledger on startup
Found by running `ulpfctl vault verify` by hand against a store written to
by several restarted collector processes: 4 of that day's segments were
reported `BROKEN` — not from tampering or data loss, but because
`Vault.New()` never read the existing ledger, so every fresh process
started its in-memory chain tip (`lastRoot`) at zero. The first segment
sealed after any restart then carried `prevRoot=0`, which doesn't match the
real previous segment's root that the prior process instance had sealed.
This isn't a test-only artifact — a legitimate production restart (deploy,
crash-recovery, not just a hard kill) hits the exact same path. Fixed with
`Vault.Bootstrap(ctx)`: reads today's ledger once at startup and resumes
`lastRoot`/`lastDay` from the last entry, a no-op on a fresh day/deployment
with no prior entries. Wired into `cmd/ulpf-collector` right after
`vault.New(...)`, before the first `WriteBatch`.
`TestBootstrapResumesChainAcrossRestart` proves two separate `Vault`
instances on the same store — process A seals a segment, "exits" (discarded,
nothing more done with it), process B calls `Bootstrap` then seals its
own — produce a chain `VerifyRange` reports PASS across, and this was
independently confirmed live: after deploying the fix, two consecutive real
collector-process restarts against the same MinIO-backed store both sealed
segments that verified clean, while the 4 segments broken *before* the fix
existed remain part of that day's permanent, unrepairable history (you
cannot retroactively fix a chain link once a segment has sealed with the
wrong `prevRoot` baked into its Merkle proof — only prevent it going
forward).

## D-018 — `MaxSegmentAge` was only enforced reactively; a quiet source could leave a segment unsealed indefinitely
The real root cause behind what looked, across several verification runs,
like repeated "orphaned segment" data loss (initially misattributed to
force-killing collector processes between test iterations, per D-007):
`Vault.WriteBatch` only checks `time.Since(active.startedAt) >=
MaxSegmentAge` *reactively*, as part of handling a new write. There was no
background timer enforcing it independently. Consequence: the moment a
source stops sending (a load test finishes; a real device goes quiet) the
currently-open segment simply sits there, unsealed, for as long as the gap
in traffic lasts — which could be indefinitely. Any Kafka ref already
published for that segment's earlier-flushed events (refs are published
per-flush, well before the segment itself seals) fails every read attempt
until either unrelated traffic eventually arrives to trigger another
`WriteBatch` call, or the process shuts down gracefully. This reliably
reproduced during Phase 6 verification: run a load test, then pause to
check Presto/write docs/read logs for a couple of minutes (completely
normal operator behavior), and the tail of that test's traffic would be
stuck unreadable until the next test's first write incidentally sealed it —
easy to misdiagnose as leftover debris from an earlier kill rather than a
live bug, since by the time it's noticed the segment often *has* since
sealed (just very late). Fixed with `Vault.SealIfStale(ctx)`, which seals
the active segment only if it's actually past `MaxSegmentAge` (a no-op
otherwise, so it doesn't fragment output by sealing on every tick
regardless of age), called every second from a ticker goroutine in
`cmd/ulpf-collector`'s main loop
(`runSealTicker`) — independent of whether new writes are arriving.
`TestSealIfStaleSealsAnAgedOutSegmentWithNoNewWrites` proves a segment with
exactly one write and zero follow-up traffic still becomes durable and
readable once its age threshold passes, with no second write required.

## D-017 — vault.ulpf.raw_index / raw_segments had no live producer
Discovered while verifying Phase 6 across all 4 catalogs for real: `lake`
and `stream` had real data, but `vault.ulpf.raw_index` and
`.raw_segments` returned 0 rows despite correct DDL and a working Presto
catalog. Root cause: `internal/sink/vaultindex.IndexSink` (writes
`raw_index`) and `vaultindex.ExportSegments` (writes `raw_segments`) were
both fully implemented and unit-tested but never invoked by any running
process — a real wiring gap, not a design gap. Fixed two ways: (1)
`cmd/ulpf-collector` now fans every ref out to Kafka (primary, the handoff
`ulpf-processor` depends on) *and* to an `IndexSink` (secondary, best-effort
— see `internal/collector/fanout.go`'s `FanoutPublisher`), flushed every
30s and on shutdown; (2) added `ulpfctl vault export-index --from --to`,
which reads a day's ledger (already written by every `Vault.Seal`) and
rolls it into `raw_segments` Parquet — meant to run after a load burst or
on a schedule, followed by `ulpfctl partitions sync` so Presto discovers
the new `dt=` partition.

## D-016 — `Vault.Read` had no segment cache: a real O(N) per-event throughput bug
Found while diagnosing why `cmd/ulpf-processor` processed real, correctly
sealed events at ~1/sec instead of anywhere near ingest rate, with zero
errors logged (ruling out the D-015 retry issue). `Vault.Read` fetched the
*entire* sealed segment object from the store and zstd-decompressed it on
**every single call**, with no caching — reading N events out of a
500-2000-event segment meant N redundant full-segment object-store GETs
and N redundant decompressions, just to read each event's own few bytes
back out. Fixed with a bounded, thread-safe LRU (`internal/vault/cache.go`,
`segmentCache`, default 256MB, keyed by segment ID) consulted by `Read`
before hitting the store. `TestReadCachesDecompressedSegment` proves a
50-event segment now costs exactly one store `Get` call, not 50. This one
was the least visible of the four bugs found this session: no error, no
log line, no metric named it — it just made the processor slow, and would
have been very easy to misattribute to "MinIO/Docker network is slow" or
"the enrichment pipeline is doing too much work" without profiling.

## D-015 — Processor's vault-read retry deadline must exceed the collector's segment-seal window
A second, more subtle bug hiding behind D-013's stall: `cmd/ulpf-processor`
retried a failed vault `Read` (absorbing the gap between "ref published to
Kafka" and "that ref's segment actually sealed to the store") for a
hardcoded 20 seconds. The collector's own default segment-seal window
(`ULPF_VAULT_SEGMENT_MAX_SECONDS`) is 300 seconds — meaning under default
production settings, an event published to Kafka the instant it landed in
a freshly-opened segment could legitimately need up to 5 minutes before
its segment sealed, but the processor would give up after 20 seconds and
silently drop it forever, logging one error line easy to miss under load.
This was masked during testing because the test harness overrode the seal
window to 15s, well under the old 20s retry budget — it never would have
been caught without deliberately testing with production-realistic seal
timing. Fixed by making the deadline a parameter
(`ULPF_VAULT_READ_RETRY_SECONDS`, default 330s — 300s default seal window
+ 30s margin) instead of a hardcoded constant; `readWithRetry` in
`cmd/ulpf-processor/main.go` now takes it explicitly, and
`main_test.go` proves both that a sealed segment resolves near-instantly
(no retry tax on the common case) and that the deadline is actually honored
for a segment that will never exist.

## D-013 / D-014 — Root-caused and fixed a real full pipeline stall under live Docker load
The first genuine end-to-end load test against the live Docker stack
(loggen -> real TCP listener -> collector -> Kafka -> processor -> real
MinIO-backed vault) sent 120,000 events; the listener correctly accepted
and counted all of them, but only ~600 ever reached a sealed vault segment,
and `ulpf_vault_write_duration_seconds_count` froze entirely — the
collector's pipeline goroutine was alive but not making forward progress.
Root cause (D-013): `internal/collector.Pipeline.flush()` called
`RefPublisher.Publish` once per event, synchronously, inside the flush
loop — up to `BatchSize` (500) sequential Kafka produce round trips per
flush, each waiting on `RequiredAcks: RequireAll`. The goroutine that
should have been draining the buffer was instead stuck deep inside that
per-event loop, so nothing downstream of it (spool, buffer, new TCP
connections) could make progress either. Fixed by adding
`BatchPublisher`/`RefEntry` to `internal/collector/pipeline.go`:
`KafkaPublisher.PublishBatch` now sends an entire flush as one
`WriteMessages` call; `Pipeline.flush` prefers it via type assertion,
falling back to the old per-event loop only for publishers (tests,
`NoopPublisher`) that don't implement it.
Compounding cause (D-014): even after batching, kafka-go's own default
`Writer.BatchTimeout` is 1 second — meant for callers trickling in single
messages who want the writer to accumulate its own batch before flushing.
Both `KafkaPublisher` (collector) and `internal/sink/kafka.Sink` (processor's
normalized-stream and DLQ writers, which publish one message per event from
a serial consume loop) hit this: without an explicit short `BatchTimeout`,
every `WriteMessages` call — even one already carrying a full
application-level batch — sat idle waiting on a second, redundant internal
timer. Fixed by setting `BatchTimeout: 10ms` on both writers.
Verified for real after both fixes: the same 120,000-event / 60s / 2000eps
TCP load test that stalled at ~600 events fully drained in real time (spool
depth returned to 0 within ~10s of the generator finishing, 17 sealed
segments vs. 1 before). Regression tests added:
`TestPipelineFlushUsesBatchPublisherInOneCall`,
`TestKafkaPublisherPublishBatchSingleRoundTrip`,
`TestNewSetsShortBatchTimeout` (guards the kafka-go default specifically).
None of this was caught by the pre-existing unit test suite, which used
small event counts against an instant local-filesystem vault — it only
surfaced against the real Docker stack with real network latency to Kafka
and MinIO, which is exactly why the Phase 6 Docker-backed load test
mattered enough to run for real rather than mark PENDING.

## D-012 — `lake` and `vault` catalogs share one Hive Metastore database
Verified live once Docker was up: Presto's `lake` and `vault` catalogs both
point `hive.metastore.uri` at the same single `hive-metastore` container,
and both use schema name `ulpf` — and Hive Metastore has no concept of a
"Presto catalog," only databases/schemas. The practical effect:
`lake.ulpf` and `vault.ulpf` are the same underlying Hive database, so
`SHOW TABLES FROM vault.ulpf` lists `events` (a lake table) alongside
`raw_segments`/`raw_index`. Data itself stays correctly separated (each
table's `external_location` points at its own S3 prefix —
`s3a://ulpf-lake/normalized/` vs `s3a://ulpf-raw/index/...` — so there's no
data crosstalk), and every demo query (Q1-Q7, docs/QUERIES.md) still
returns correct results through the `lake.`/`vault.` catalog-qualified
names Presto's SQL layer expects. What's lost is metadata-level isolation
(a `DROP TABLE vault.ulpf.events` would drop the lake table too). A
"more correct" fix is a second, independent Hive Metastore + Postgres
database per catalog; not done here since it roughly doubles that part of
the compose stack for a prototype where the demo queries — the actual
graded capability — already work correctly as written.

## D-010 — Onboarding demo vendor: synthetic SonicWall, not a real download
Phase 8's acceptance gate calls for "a vendor format NOT in our packs (e.g.
Sophos XG or SonicWall)". Rather than fetching or hand-copying a real
vendor's log samples (which would violate the no-scraped-data rule the same
way a real IOC feed would), `internal/loggen.SonicWallTraffic` generates a
synthetic SonicWall-shaped kv log from the same Generator every other
vendor line comes from — deliberately left out of `AllVendors` so it stays
a genuinely-never-seen-by-any-pack format for the onboarding test. Exposed
via `ulpfctl gen sonicwall --count N` so the Python onboarding service (and
its pytest suite) can generate a fresh sample without duplicating the
generator logic across languages.

## D-011 — `ulpfctl parser run`: one execution path for onboarding, Workbench, and manual debugging
Onboarding needs to run a not-yet-published candidate parser against a raw
sample and report per-field success — the same operation the Phase 9 Parser
Workbench's live re-parse needs, and the same operation a human debugging a
parser by hand wants from the CLI. Implemented once
(`cmd/ulpfctl/parser_run_cmd.go`, using the same `parse.Compile` +
`normalize.Mapper` the data plane runs) and reused by both the Python
onboarding service (`app/ulpfctl.py`) and, later, the console — instead of
onboarding re-implementing a second parse-and-map loop in Python that could
silently diverge from the Go engine's actual behavior.

## D-008 — Control plane validates parsers by shelling out to `ulpfctl`
Phase 7's publish endpoint (`services/control-plane/app/routers/parsers.py`)
needs to lint a candidate parser and run its golden fixtures before storing
it. Rather than re-implementing DSL validation, unsafe-regex rejection, and
fixture comparison a second time in Python — a second implementation that
could silently drift from the Go one — it shells out to the exact compiled
`bin/ulpfctl.exe` the data plane itself uses (`app/ulpfctl.py`). One
validator, shared by both languages. Verified for real:
`test_publish_rejects_unsafe_regex` and `test_publish_then_rollback` in
`tests/test_parsers.py` invoke the actual Go binary as a subprocess, not a
mock — a genuine cross-language integration test. Tradeoff: `ulpfctl.exe`
must be built (`go build -o bin/ulpfctl.exe ./cmd/ulpfctl`) before these
tests will do anything but skip; CI's Go job already does this before the
Python job runs.

## D-009 — Hot-reload distribution reuses Phase 3's fsnotify watch, not Kafka
CLAUDE.md's Phase 7 spec describes processors subscribing to a
`ulpf.control.parsers` Kafka topic and fetching artifacts on publish. For
this single-node prototype, the publish endpoint instead writes the
artifact straight into the real `packs/` directory on disk (in addition to
storing it in `ulpf_meta`), which the already-built, already-tested
`internal/parse.Registry.WatchDir` (Phase 3, fsnotify-based) picks up with
zero new code. This is simpler and, critically, actually verifiable without
Docker/Kafka reachable — the Kafka-topic path remains the natural next step
for a deployment where processors aren't co-located with the packs/
directory (e.g. multiple nodes on different hosts), and the publish
endpoint's docstring flags exactly that.

## D-001 — Module path
Using `github.com/ulpf/ulpf` as a placeholder Go module path since this is a
prototype not yet published to a real org. Trivial to rename with a global
`gofmt -r` / import-path rewrite before publishing.

## D-002 — Synthetic GeoIP/ASN instead of a downloaded .mmdb
CLAUDE.md pins DB-IP City/ASN Lite (.mmdb, CC BY 4.0, no signup) as the GeoIP
source. Building this prototype in a sandboxed agent session, we generate a
synthetic CIDR-range -> {country, city, asn, as_org} CSV dataset ourselves
(`enrichment/geoip.csv`, `enrichment/asn.csv`) instead of fetching a real
binary .mmdb, for two reasons: (1) writing the MaxMind binary DB format
requires an encoder library beyond what a reader-only Go module gives us, and
(2) it keeps the enrichment fully deterministic and inspectable for the demo.
The `internal/enrich/geoip` and `.../asn` packages are written against a small
`Lookup(ip) (Record, bool)` interface, so swapping in a real DB-IP .mmdb
reader (e.g. `oschwald/geoip2-golang`, MIT) at deploy time is a one-file change
— the interface does not need to change. Documented in docs/LICENSES.md.

## D-003 — Environment: Docker not installed on the build machine
The build machine (Windows, F:\SIH) had no Docker Desktop and no Go toolchain
at session start. Go was installed via `winget install GoLang.Go` during the
build (safe, reversible, no admin/reboot required) so the entire Go data plane
could be compiled and unit-tested for real. Docker Desktop was left to the
user to install, since it requires admin rights, WSL2/Hyper-V, and typically a
reboot — not something to do unilaterally. Consequence: every acceptance gate
that only needs `go test`, `pytest`, or `npm run build` was actually executed
and its real output captured. Every gate that needs `docker compose up`
(Kafka, Presto, Hive Metastore, MinIO, the federated SQL queries, the air-gap
bundle install) is written to spec and ready to run, but is marked PENDING in
docs/BUILD_PLAN.md until Docker is available. `make verify` reports this
honestly rather than faking a PASS — see the "docker" column it prints.

## D-004 — Postgres-only metadata store, SQLite fallback for local dev
The control plane's SQLAlchemy models target PostgreSQL 16 (the pinned
metadata DB, reachable by Presto's `meta` catalog). For local development
without Docker, `services/control-plane` also runs against a local SQLite file
via the same models purely so we can exercise and test the API with pytest
before Postgres is reachable. `ulpf_meta` in Postgres remains the only target
Presto's `meta` catalog is configured against; SQLite is dev-only and never
referenced by any catalog config.

## D-007 — Durability boundary is "sealed segment", not "WriteBatch returned"
Verified during the Phase 2 throughput gate: `Vault.WriteBatch` appends to
the currently-open segment's in-memory buffer and returns immediately (that's
required for 20k+ EPS — see the real run below). The bytes only become
durable in the store when that segment **seals**, which per CLAUDE.md happens
at 64MB or 5 minutes, whichever comes first. Killing the collector process
*ungracefully* (no SIGTERM handled) between seals loses whatever is sitting
in the still-open segment — confirmed empirically: a 50,001-event / 10s run
that never crossed either seal threshold and was then hard-killed left
**zero** segments in the ledger; a 1,200,007-event / 60s run that crossed the
64MB threshold five times over lost only the ~125k events still sitting in
the sixth, not-yet-sealed segment when it was hard-killed.
This is the same buffered-durability trade-off every comparable system makes
(Kafka producer `linger.ms`, Elasticsearch bulk indexing, Fluentd buffers) —
the fix is graceful shutdown, not synchronous per-event fsync, which would
destroy throughput. `cmd/ulpf-collector` already installs a SIGTERM/SIGINT
handler that calls `vault.Seal(ctx)` before exit; this works correctly under
`docker stop` (the real deployment path, confirmed by Go's os/signal
handling SIGTERM natively on Linux) and under interactive Ctrl+C on any
platform. What did NOT work in this dev session: sending a termination
signal to a *backgrounded, MSYS-spawned* process from Git Bash on Windows —
`kill` didn't deliver anything the process observed, and `taskkill` without
`/F` was refused by Windows outright ("can only be terminated forcefully")
because such a process has no console handle for a graceful close message to
target. That is a quirk of this specific local testing harness, not of the
shipped signal-handling code, which is unit-covered by
`internal/vault/vault_test.go`'s explicit `Seal()` calls.
Operational takeaway for the demo/production: EPS-heavy deployments that
want a tighter crash-loss window should lower
`ULPF_VAULT_SEGMENT_MAX_BYTES`/`_SECONDS` rather than rely on always-graceful
shutdowns.

## D-006 — Unprivileged listener ports
CLAUDE.md / BUILD_PLAN reference syslog UDP/514, TCP/601, TLS/6514. Those are
sub-1024 "well-known" ports that need root in containers and complicate
rootless deployment. Default config uses 5514/6601/6614 (and 8088 for HTTP
bulk) instead — trivially reconfigurable in `.env`, and a real deployment can
still bind 514/601/6514 by setting the env vars plus `CAP_NET_BIND_SERVICE` or
port-forwarding.

## D-005 — `make` not available on the Windows build machine
No `make` binary on this machine either. The Makefile is written for the
Linux/CI/Docker environment (where it is authoritative, per CI and the
eventual deployment target). For local Windows verification during the build,
equivalent `go`/`npm`/`pytest` commands were run directly and their output
captured in this log / build session instead of going through `make`.
