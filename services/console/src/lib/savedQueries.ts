// Mirrors docs/QUERIES.md exactly — column names match schema/presto/ddl.sql.
export interface SavedQuery {
  id: string;
  title: string;
  explains: string;
  sql: string;
}

export const SAVED_QUERIES: SavedQuery[] = [
  {
    id: "q1",
    title: "Q1 — Cross-vendor unified visibility",
    explains: "One query, every vendor — before normalization this needed N vendor-specific queries.",
    sql: `SELECT observer_vendor, observer_product, event_action, count(*) AS events,
       sum(network_bytes_total) AS bytes
FROM lake.ulpf.events
WHERE dt = current_date AND dst_port = 443
GROUP BY 1, 2, 3
ORDER BY events DESC;`,
  },
  {
    id: "q2",
    title: "Q2 — Federated hot + cold",
    explains: "Live Kafka tier UNION ALL historical Parquet tier — two storage systems, one logical table.",
    sql: `SELECT 'live' AS tier, src_ip, count(*) AS events
FROM stream.ulpf.events_normalized
GROUP BY 1, 2
UNION ALL
SELECT 'historical', src_ip, count(*)
FROM lake.ulpf.events
WHERE dt = current_date
GROUP BY 1, 2;`,
  },
  {
    id: "q3",
    title: "Q3 — Federated join across three catalogs",
    explains: "Normalized events (lake) joined to the parser registry (meta/Postgres) in one statement.",
    sql: `SELECT e.observer_vendor, e.lineage_parser_id, p.version AS registered_version,
       p.published_at, count(*) AS events, avg(e.quality_score) AS avg_quality
FROM lake.ulpf.events e
JOIN meta.public.parser_registry p ON e.lineage_parser_id = p.parser_id
WHERE e.dt = current_date
GROUP BY 1, 2, 3, 4;`,
  },
  {
    id: "q4",
    title: "Q4 — Traceability / chain of custody",
    explains: "Every event carries a pointer straight back to its sealed segment and Merkle root.",
    sql: `SELECT e.event_id, e.raw_sha256, e.raw_segment_id, e.raw_offset, e.raw_length,
       s.merkle_root, s.prev_root, s.sealed_at_ns
FROM lake.ulpf.events e
JOIN vault.ulpf.raw_segments s ON e.raw_segment_id = s.segment_id
WHERE e.event_id = '<event-id>';`,
  },
  {
    id: "q5",
    title: "Q5 — Losslessness proof",
    explains: "missing_raw_ref must be 0 for every vendor, every day — provable in one line.",
    sql: `SELECT observer_vendor, count(*) AS events,
       avg(cardinality(unmapped)) AS avg_unmapped_fields,
       sum(CASE WHEN raw_sha256 IS NULL THEN 1 ELSE 0 END) AS missing_raw_ref
FROM lake.ulpf.events
WHERE dt = current_date
GROUP BY 1;`,
  },
  {
    id: "q6",
    title: "Q6 — Parser health / regression detection",
    explains: "Quality score and partial-parse rate by parser and hour — feeds the Sources page alert.",
    sql: `SELECT lineage_parser_id, hour, avg(quality_score) AS avg_quality,
       sum(CASE WHEN lineage_parse_status = 'partial' THEN 1 ELSE 0 END) AS partials
FROM lake.ulpf.events
WHERE dt = current_date
GROUP BY 1, 2
ORDER BY 1, 2;`,
  },
  {
    id: "q7",
    title: "Q7 — ML feature extraction",
    explains: "Per-source-IP, per-hour features — consumed directly from SQL, no separate ETL step.",
    sql: `SELECT src_ip, date_trunc('hour', from_unixtime(event_observed_at / 1e9)) AS hr,
       count(DISTINCT dst_ip) AS distinct_dsts,
       count(DISTINCT dst_port) AS distinct_ports,
       sum(network_bytes_out) * 1.0 / nullif(sum(network_bytes_in), 0) AS out_in_ratio,
       sum(CASE WHEN event_action = 'blocked' THEN 1 ELSE 0 END) * 1.0 / count(*) AS block_rate
FROM lake.ulpf.events
WHERE dt = current_date
GROUP BY 1, 2;`,
  },
];
