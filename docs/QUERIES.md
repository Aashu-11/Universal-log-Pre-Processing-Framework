# Federated Demo Queries

Seven queries proving PrestoDB's actual superpower: one SQL statement
spanning the `lake` (Parquet/MinIO), `stream` (live Kafka), and `meta`
(Postgres parser registry) catalogs, plus `vault` for chain-of-custody.
Column names here are the real `schema/presto/ddl.sql` / `internal/schema`
column names — adapted from the illustrative versions in the original build
plan where our schema differs (noted inline where it does).

Run these once Docker is up (`make up`) via the Presto CLI or the
console's Explorer page — each is pre-loaded there with a one-line
explanation of what it proves.

## Q1 — Cross-vendor unified visibility

One query, every vendor — before normalization this needed N vendor-specific
queries against N different log shapes.

```sql
SELECT observer_vendor, observer_product, event_action, count(*) AS events,
       sum(network_bytes_total) AS bytes
FROM lake.ulpf.events
WHERE dt = current_date AND dst_port = 443
GROUP BY 1, 2, 3
ORDER BY events DESC;
```

## Q2 — Federated hot + cold

The live tier (last 30 minutes, off Kafka) and the historical tier
(Parquet/MinIO) in one UNION ALL — Presto reading two entirely different
storage systems as one logical table.

```sql
SELECT 'live' AS tier, src_ip, count(*) AS events
FROM stream.ulpf.events_normalized
GROUP BY 1, 2
UNION ALL
SELECT 'historical' AS tier, src_ip, count(*) AS events
FROM lake.ulpf.events
WHERE dt = current_date
GROUP BY 1, 2;
```

## Q3 — Federated join across three catalogs

Normalized events (`lake`) joined to the parser registry (`meta`, a
PostgreSQL table Phase 7's control plane owns) — this is what
"traceability across the whole platform" looks like as one SQL statement,
not an application-layer join.

```sql
SELECT e.observer_vendor, e.lineage_parser_id, p.version AS registered_version,
       p.published_at, count(*) AS events,
       avg(e.quality_score) AS avg_quality
FROM lake.ulpf.events e
JOIN meta.public.parser_registry p
  ON e.lineage_parser_id = p.parser_id
WHERE e.dt = current_date
GROUP BY 1, 2, 3, 4;
```

## Q4 — Traceability / chain of custody

Every event carries a pointer straight back to its sealed segment and
Merkle root — `vault.ulpf.raw_segments` records `sealed_at_ns` (epoch
nanoseconds, consistent with every other timestamp in this schema) rather
than a SQL `TIMESTAMP`, the one column name that differs from the
illustrative build-plan version of this query.

```sql
SELECT e.event_id, e.raw_sha256, e.raw_segment_id, e.raw_offset, e.raw_length,
       s.merkle_root, s.prev_root, s.sealed_at_ns
FROM lake.ulpf.events e
JOIN vault.ulpf.raw_segments s ON e.raw_segment_id = s.segment_id
WHERE e.event_id = ?;
```

## Q5 — Losslessness proof

`missing_raw_ref` must be 0 for every vendor, every day — that's the
"preserve complete raw event data" requirement (a), provable in one line.

```sql
SELECT observer_vendor,
       count(*) AS events,
       avg(cardinality(unmapped)) AS avg_unmapped_fields,
       sum(CASE WHEN raw_sha256 IS NULL THEN 1 ELSE 0 END) AS missing_raw_ref
FROM lake.ulpf.events
WHERE dt = current_date
GROUP BY 1;
```

## Q6 — Parser health / regression detection

Quality score and partial-parse rate, by parser and hour — the input to
the console's "quality score drops >10% in 15min" alert on the Sources
page.

```sql
SELECT lineage_parser_id, hour,
       avg(quality_score) AS avg_quality,
       sum(CASE WHEN lineage_parse_status = 'partial' THEN 1 ELSE 0 END) AS partials
FROM lake.ulpf.events
WHERE dt = current_date
GROUP BY 1, 2
ORDER BY 1, 2;
```

## Q7 — ML feature extraction

Per-source-IP, per-hour features — distinct destinations/ports, byte-ratio,
block-rate — exactly the shape a downstream anomaly model (or the port-scan
demo in Phase 9) consumes directly from SQL, no separate ETL step.

```sql
SELECT src_ip,
       date_trunc('hour', from_unixtime(event_observed_at / 1e9)) AS hr,
       count(DISTINCT dst_ip) AS distinct_dsts,
       count(DISTINCT dst_port) AS distinct_ports,
       sum(network_bytes_out) * 1.0 / nullif(sum(network_bytes_in), 0) AS out_in_ratio,
       sum(CASE WHEN event_action = 'blocked' THEN 1 ELSE 0 END) * 1.0 / count(*) AS block_rate
FROM lake.ulpf.events
WHERE dt = current_date
GROUP BY 1, 2;
```
