-- LOGKRAMA Presto DDL. Run against the `lake` and `vault` catalogs (both are
-- the hive-hadoop2 connector against the same Hive Metastore, S3A -> MinIO
-- — see deploy/presto/etc/catalog/{lake,vault}.properties).
--
-- Column names/types here MUST match internal/schema.FlatRow's `parquet:`
-- tags exactly (internal/schema/flatrow.go) — Presto's Hive connector maps
-- Parquet files to these declared columns by name.

CREATE SCHEMA IF NOT EXISTS lake.logkrama
WITH (location = 's3a://logkrama-lake/');

CREATE SCHEMA IF NOT EXISTS vault.logkrama
WITH (location = 's3a://logkrama-raw/index/');

-- ---------------------------------------------------------------------------
-- lake.logkrama.events — normalized UES events, partitioned by dt/hour/vendor.
-- Written by internal/sink/parquet.Writer to
-- s3a://logkrama-lake/normalized/dt=YYYY-MM-DD/hour=HH/vendor=<v>/*.parquet
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS lake.logkrama.events (
    event_id                    VARCHAR,
    event_kind                  VARCHAR,
    event_category               VARCHAR,
    event_type                  VARCHAR,
    event_action                VARCHAR,
    event_outcome                VARCHAR,
    event_severity_id            BIGINT,
    event_original_time_string   VARCHAR,
    event_observed_at            BIGINT,
    event_ingested_at            BIGINT,
    event_time_skew_ms           BIGINT,
    event_dataset                VARCHAR,

    observer_vendor              VARCHAR,
    observer_product             VARCHAR,
    observer_type                VARCHAR,
    observer_hostname            VARCHAR,
    observer_ip                  VARCHAR,
    observer_version             VARCHAR,

    src_ip                       VARCHAR,
    src_port                     BIGINT,
    src_mac                      VARCHAR,
    src_hostname                 VARCHAR,
    src_user                     VARCHAR,
    src_zone                     VARCHAR,

    dst_ip                       VARCHAR,
    dst_port                     BIGINT,
    dst_mac                      VARCHAR,
    dst_hostname                 VARCHAR,
    dst_zone                     VARCHAR,

    network_transport            VARCHAR,
    network_direction            VARCHAR,
    network_bytes_in             BIGINT,
    network_bytes_out            BIGINT,
    network_bytes_total          BIGINT,
    network_packets_in           BIGINT,
    network_packets_out          BIGINT,
    network_duration_ms          BIGINT,
    network_protocol             VARCHAR,

    http_method                  VARCHAR,
    http_status_code             BIGINT,
    http_user_agent              VARCHAR,

    url_full                     VARCHAR,
    url_domain                   VARCHAR,
    url_path                     VARCHAR,

    dns_question_name            VARCHAR,
    dns_question_type            VARCHAR,
    dns_response_code            VARCHAR,

    tls_version                  VARCHAR,
    tls_sni                      VARCHAR,
    tls_cipher                   VARCHAR,

    actor_user                   VARCHAR,
    actor_id                     VARCHAR,

    threat_signature_id          VARCHAR,
    threat_signature_name        VARCHAR,
    threat_category              VARCHAR,
    threat_severity              VARCHAR,
    threat_mitre_tactic          VARCHAR,
    threat_mitre_technique       VARCHAR,

    enrich_src_geo_country       VARCHAR,
    enrich_src_geo_city          VARCHAR,
    enrich_src_asn               BIGINT,
    enrich_src_as_org            VARCHAR,
    enrich_dst_geo_country       VARCHAR,
    enrich_dst_geo_city          VARCHAR,
    enrich_dst_asn               BIGINT,
    enrich_dst_as_org            VARCHAR,
    enrich_src_is_internal       BOOLEAN,
    enrich_dst_is_internal       BOOLEAN,
    enrich_ioc_match             BOOLEAN,
    enrich_ioc_indicator         VARCHAR,
    enrich_risk_score            DOUBLE,

    raw_sha256                   VARCHAR,
    raw_segment_id                VARCHAR,
    raw_offset                   BIGINT,
    raw_length                   BIGINT,
    raw_retrieval_uri             VARCHAR,

    lineage_parser_id            VARCHAR,
    lineage_parser_version       VARCHAR,
    lineage_parse_status         VARCHAR,
    lineage_node_id               VARCHAR,
    lineage_processing_ms        DOUBLE,

    quality_score                 DOUBLE,

    unmapped                      MAP(VARCHAR, VARCHAR),

    dt                            VARCHAR,
    hour                          VARCHAR,
    vendor                        VARCHAR
)
WITH (
    external_location = 's3a://logkrama-lake/normalized/',
    format = 'PARQUET',
    partitioned_by = ARRAY['dt', 'hour', 'vendor']
);

-- ---------------------------------------------------------------------------
-- vault.logkrama.raw_segments — one row per sealed Raw Vault segment, the
-- Merkle chain. Written by internal/sink/vaultindex.ExportSegments to
-- s3a://logkrama-raw/index/segments/dt=YYYY-MM-DD/*.parquet
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS vault.logkrama.raw_segments (
    segment_id     VARCHAR,
    merkle_root    VARCHAR,
    prev_root      VARCHAR,
    event_count    BIGINT,
    sealed_at_ns   BIGINT,
    byte_size      BIGINT,
    dt             VARCHAR
)
WITH (
    external_location = 's3a://logkrama-raw/index/segments/',
    format = 'PARQUET',
    partitioned_by = ARRAY['dt']
);

-- ---------------------------------------------------------------------------
-- vault.logkrama.raw_index — one row per event, pointing at its exact bytes
-- inside a sealed segment. Written by internal/sink/vaultindex.IndexSink to
-- s3a://logkrama-raw/index/dt=YYYY-MM-DD/*.parquet
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS vault.logkrama.raw_index (
    event_id     VARCHAR,
    segment_id   VARCHAR,
    "offset"     BIGINT,
    length       BIGINT,
    sha256       VARCHAR,
    dt           VARCHAR
)
WITH (
    external_location = 's3a://logkrama-raw/index/',
    format = 'PARQUET',
    partitioned_by = ARRAY['dt']
);

-- After writing new partition directories, register them so Presto can see
-- them (cmd/logkramactl partitions sync runs this on a schedule):
--   CALL system.sync_partition_metadata('logkrama', 'events', 'FULL');
--   CALL system.sync_partition_metadata('logkrama', 'raw_segments', 'FULL');
--   CALL system.sync_partition_metadata('logkrama', 'raw_index', 'FULL');
