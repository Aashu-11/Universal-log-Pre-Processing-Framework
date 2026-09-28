# LogVerse — 3D Forensic Time Machine

## What it does

The default **Nexus graph** uses the same `react-force-graph-3d` renderer as
the supplied LedgerSpy visualization, adapted to LOGKRAMA data and the console's
dark violet theme. It retains the force-driven motion, moving link particles,
hover neighbor highlighting and three-second camera flight on click. The
camera starts centered on the graph and fits the node cloud after its initial
force layout settles. A floating header and left-side analysis cards echo the
reference view in the console's dark palette. Vendor nodes
connect to event nodes, which connect to observed source and destination IPs.
Hovering a node highlights its immediate links; clicking an event opens the
existing forensic inspector. The **Pipeline** toggle retains the eight-stage
time-machine scene and animated event flow. Graph positions start from
deterministic seeds and then move under the force layout; they are visual
coordinates, not geographic positions or inferred network
topology. A link means only that the displayed event row contains that vendor
or IP field. No bank-statement logic, uploaded CSV, or synthetic risk score is
used. The graph displays at most 150 event nodes and their endpoints, with
reserved slots for DLQ events during high normal-event volume. DLQ events
connect to their recorded validation-reason nodes. Clicking a vendor, IP,
reason, or event shows the related loaded events; selecting an event starts
its backward forensic trace automatically.
The left sidebar charts show loaded-window event volume and observed state
distribution. Selecting an event adds its risk score versus the loaded-window
average, a count of other events sharing observed IPs, and the existing
raw-evidence forensic trace. These are descriptive summaries of loaded rows;
they do not claim an attack or invent enrichment fields.

LogVerse renders the live LOGKRAMA pipeline as a navigable 3D scene: real
registered sources around the outside, the eight CLAUDE.md pipeline stages
laid out as labeled stations, real events as particles flying between them,
the Raw Vault's Merkle chain as a row of linked blocks, and the four Presto
destination catalogs (lake/stream/meta/vault) plus a separate DLQ branch as
where a particle ends up. Clicking a particle (or a row in the keyboard/
no-WebGL fallback table) opens an inspector with a **Trace Event** action
that steps backward through the stages that event actually reached, ending
at its raw bytes' SHA-256 and Merkle-proof verification state.

It is a visualization layer over data the console already exposes — it does
not introduce a second copy of any event, source, or DLQ record.

## How to open it

The **LogVerse** navigation option opens a dedicated full-screen view in a new
browser tab. The same authenticated route remains available at `/logverse`.

Console → **LogVerse** in the left nav (between Live Theater and Pipeline),
or `/logverse` directly. Available to every authenticated role (admin,
engineer, analyst, auditor) — same as every other console page; nothing here
requires elevated permissions to *view*, matching how Sources/Explorer/
Traceability already work. The page is code-split and lazy-loaded, so its
~260KB (gzipped) three.js/@react-three/fiber/@react-three/drei bundle only
downloads when someone actually navigates to it.

## APIs it uses

No new backend endpoint was added. LogVerse reuses exactly what every other
console page already calls:

| Data | Endpoint | Notes |
|---|---|---|
| Sources | `GET /v1/sources` | same rows Sources page lists |
| Recent normalized events | `POST /v1/query` against `lake.logkrama.events` | superset of Live Theater/Traceability's column list — adds `src_ip/port`, `dst_ip/port`, `enrich_dst_is_internal`, `enrich_ioc_*`, `threat_*` |
| DLQ events | `GET /v1/dlq?limit=80` | newest bounded rows; `raw_ref` (segment_id/offset/length/sha256) makes a DLQ event traceable |
| Vault chain | `POST /v1/query` against `vault.logkrama.raw_segments` | one row per sealed segment — this *is* the Merkle chain |
| Forensic trace | `GET /v1/events/{id}/trace` | same endpoint Traceability's "Verify now" uses — raw bytes + SHA-256 + Merkle proof |
| Pipeline stats | `GET /v1/stats/pipeline` | source of the derived EPS figure and the real dropped-packet count |
| Vault integrity | `GET /v1/integrity/verify` | manual "Verify vault chain" button only — this is a full chain recompute, expensive, never auto-polled |

Data arrives via polling (`usePolling`/a purpose-built `useLogVerseData`
hook), the same transport every other console page already uses — no
WebSocket/SSE infrastructure exists in this codebase, and CLAUDE.md's
"reuse existing transport" guidance applied directly. Events poll every 3s;
sources/DLQ/vault chain poll every 8s (they change far less often).

## What "internal assets" actually is

There is no asset-inventory endpoint in this API. The "Internal assets"
cluster is **derived** from real, already-fetched event data: distinct
`dst_ip` values where `enrich_dst_is_internal = true` in the current event
buffer, ranked by frequency (`deriveInternalAssets` in
`src/lib/logverse/data.ts`). It is honest inference from real fields, not a
fabricated CMDB, and the scene labels it "observed destinations, not a
formal inventory" rather than implying it's a complete estate inventory.

## Visual states — what each one means

| Color | Shape | Meaning | Backed by |
|---|---|---|---|
| Cyan sphere | sphere | Normal | risk score < 40, no IOC/threat |
| Amber sphere | sphere (larger) | Elevated | risk score ≥ 40 (the enrichment pipeline's own `ioc_match` weight, `internal/enrich/risk.go`), no confirmed threat |
| Red spiked polyhedron | octahedron | Threat / IOC match | `enrich_ioc_match = true` **or** a populated `threat_category` — never from score alone |
| Purple tilted cube | box | DLQ | sourced from `GET /v1/dlq`, not the lake |
| Green block | box | Integrity verified | only after `GET /v1/integrity/verify` returns `passed: true` |
| Bright red fractured block (pulsing wireframe) | box + overlay | Integrity FAILED | only after that same call returns `passed: false` |

Color is never the only signal — every state above also has a distinct
shape, and the SceneLegend spells out both. The vault chain never shows
"verified" or "failed" until the investigator has actually clicked **Verify
vault chain**; its default state is "not yet verified" (gray), not a guess.

## What's measured vs. what's a visual estimate

- **Real, server-recorded**: `event_ingested_at` (nanoseconds since epoch,
  set by `internal/processor` from `time.Now().UnixNano()`), every field
  value shown in the inspector, `lineage_processing_ms` (one total parse
  duration), SHA-256/Merkle verification results, the DLQ reason, the
  derived EPS figure (computed from two real Prometheus counter samples —
  always shown with a "derived, Ns" label, never presented as a
  server-measured rate).
- **Visual/UI pacing, not measured**: how long a particle takes to visually
  cross all 8 stages (`JOURNEY_VISUAL_DURATION_MS` = 2880ms,
  `src/lib/logverse/playback.ts`), and the stage-reveal pacing in the
  backward trace playback (420ms/stage). The API has no per-stage timing
  breakdown — only a total `lineage_processing_ms` and two absolute
  timestamps — so a per-stage animation duration cannot be "measured" and
  is documented as an estimate everywhere it reaches the UI.
- **Routing behavior**: the real router writes Lake+Stream and additionally
  writes a DLQ copy on validation failure. The trace includes ROUTE for DLQ
  events and labels the extra copy explicitly. A DLQ row does not contain
  normalized risk, quality, parse status, or the exact failing field; those
  appear as unavailable rather than as zero or a guessed failure.

## Forensic trace — what "Unavailable" means

Every field in the Trace Event panel that the API didn't actually return is
shown as `Unavailable — <reason>`, never blank and never guessed. Two
specific, permanent Unavailable states, by design (not a bug):

- **Parser match reason**: the trace API returns which parser was selected,
  not *why* it beat other candidates — that confidence/match-rule detail
  isn't part of the response.
- **Risk-score breakdown**: `internal/enrich/risk.go` computes an
  explainable `risk_factors` map internally, but the lake's Presto schema
  (`schema/presto/ddl.sql`) only exposes the final `enrich_risk_score`
  column — the per-factor breakdown isn't queryable, so LogVerse says so
  rather than reconstructing a plausible-looking guess.

## Limits (bounded, not unbounded, memory/render footprint)

| Limit | Value | Where |
|---|---|---|
| Client event ring buffer | 240 events | `MAX_EVENT_BUFFER`, oldest dropped first once full |
| Particles actually rendered at once | 150 | `MAX_VISIBLE_PARTICLES` — reserves up to 25% of slots for DLQ events, fills the rest from newest normal events, and reports overflow as "+N aggregated, not individually rendered" |
| DLQ rows kept | 80 | `MAX_DLQ_VISIBLE` |
| Vault chain blocks shown | 20 | `MAX_VAULT_BLOCKS_VISIBLE` |
| Derived internal assets | 12 | `MAX_ASSETS_VISIBLE` |

Real, uncapped pipeline stats (total events received, total UDP drops,
DLQ total in Postgres) are still shown as numbers from `/v1/stats/pipeline`

## Query performance — partition pruning and in-flight guarding

Found live while testing this feature against this project's own dev lake:
an unfiltered `ORDER BY event_ingested_at DESC LIMIT n` against
`lake.logkrama.events` took **15s+ for a plain `count(*)`**, with only 29
partitions registered — not a partition explosion, just a slow response
from this lightly-resourced dev environment's Presto/MinIO. At LogVerse's
3s event-poll interval, that meant a new expensive query could fire before
the previous one had even returned, piling up concurrent queries that
starved each other and left the page stuck on "Connecting…" indefinitely —
looking exactly like the backend was down when it wasn't. Two fixes, both
in `src/lib/logverse/data.ts` and `useLogVerseData.ts`:

1. Every query against a `dt`-partitioned table (`lake.logkrama.events`,
   `vault.logkrama.raw_segments`) now filters to `dt >= current_date - 14 days`,
   so Hive-side partition pruning has something to prune regardless of how
   much older history has accumulated from past test runs.
2. Each poller (events/slow/eps) now skips a tick if its previous request
   hasn't resolved yet, instead of firing a new one on top of it — so a
   slow-but-eventually-successful backend degrades to a slower refresh
   cadence, never an unbounded pile-up.

With both fixes, first load still took ~19s against this specific dev
stack (down from indefinite) — the loading state says so explicitly rather
than looking stuck. The deeper root cause (why a 29-partition, 435K-row
table is this slow to begin with) is a Presto/MinIO/Docker-Desktop-resource
question outside LogVerse's scope; the fixes above make the page correct
and eventually-responsive regardless of how that gets resolved.
— only what LogVerse itself renders/retains is bounded.

## Air-gapped operation

- **No remote fonts**: drei's `<Text>` (troika-three-text) defaults to
  fetching a font file from a CDN at runtime unless given a local font path.
  LogVerse avoids `<Text>` entirely and labels everything with drei's
  `<Html>`, which renders real DOM elements using the app's already-bundled
  IBM Plex Sans (`@fontsource-variable/ibm-plex-sans`) — zero risk of a
  runtime font fetch.
- **No remote textures/geometry**: every shape is built-in three.js
  primitive geometry (sphere/box/octahedron/cylinder/ring), no loaded assets.
- **No new outbound calls**: every data source above is one of this
  project's own services, reached the same way every other console page
  reaches them.

## Demo flow for reviewers

1. Send some traffic through the collector (any `loggen` invocation from
   the main README/runbook works) so the lake has events.
2. Open Console → **LogVerse**. The scene should show source nodes on the
   outer edge, 8 pipeline stations, and particles moving between them within
   a few seconds.
3. Click **Verify vault chain** — watch the block row turn green (or, if
   you want to see the failure state, this only turns red on an actual
   chain-verification failure; it does not have a synthetic "demo failure"
   mode, per the no-fabricated-evidence rule).
4. Click any particle → the inspector opens with a summary. Click **Trace
   Event** → watch it step backward through PARSE/IDENTIFY/NORMALIZE/etc.,
   ending at the raw bytes' SHA-256 and Merkle proof.
5. Toggle a severity filter off (e.g. uncheck "Normal") — particles for that
   state disappear from the scene immediately, no reload.
6. Use the timeline: **Pause**, drag the scrub slider, or hit **Replay** at
   2×/4× to watch recently-buffered events move again deterministically.
7. If you don't have a WebGL-capable browser/VM handy, the same page still
   renders a plain, keyboard-navigable table with the same real data —
   worth demonstrating the graceful-degradation path deliberately.

## Known limitations / follow-ups

- ~~The `stream` Presto catalog issue...~~ **Fixed 2026-09-28.** Root cause
  was two real, separate bugs, not container-restart flakiness: (1)
  PrestoDB 0.286's kafka connector requires `kafka.table-names` set
  explicitly — `kafka.table-description-dir` alone never auto-discovers
  tables in this version, unlike Trino's later fork — `deploy/presto/etc/
  catalog/stream.properties`. (2) The Kafka table description's own `_key`
  field collided with Presto's reserved internal `_key` column, and its
  `dataFormat: "varchar"` was invalid for the raw-key decoder — both fixed
  in `deploy/presto/etc/kafka/logkrama.events_normalized.json` (renamed to
  `event_key`, dropped the invalid per-field dataFormat). Verified live:
  `SELECT count(*) FROM stream.logkrama.events_normalized` and the real
  Explorer Q2 hot+cold UNION both return real rows now. LogVerse doesn't
  query `stream` directly, so this didn't block it, but the "live stream"
  route-destination node's label is no longer describing a catalog that's
  actually broken.
- WebGL context-loss recovery currently asks the user to reload the page
  rather than attempting an in-place renderer re-initialization.
- No automated screenshot exists yet (see the README's Screenshots/Demo
  section) — deliberately left as a TODO placeholder rather than a
  fabricated image; a real one should be captured from a running stack.
