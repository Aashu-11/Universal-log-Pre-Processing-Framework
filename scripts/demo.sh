#!/usr/bin/env bash
# `make demo` — the single command that proves the core claim:
# "Ingest anything. Lose nothing. Prove everything. Onboard a new source in
# minutes." Brings the stack up if it isn't already, sends real synthetic
# traffic through the real ingest path, waits for it to land, then runs
# real federated queries and integrity checks against the result — no
# canned output, every number below comes from a live command.
set -euo pipefail
cd "$(dirname "$0")/.."
export MSYS_NO_PATHCONV=1

echo "### 1/5 — bringing the stack up (infra + collector/processor/control-plane/onboarding/console)"
docker compose up -d --build

echo ""
echo "### 2/5 — waiting for every service to report healthy"
bash scripts/verify.sh || { echo "one or more services failed to come up — see above"; exit 1; }

echo ""
echo "### 3/5 — sending 5,000 real events through the real TCP ingest path"
go build -o bin/loggen.exe ./tools/loggen 2>/dev/null || go build -o bin/loggen ./tools/loggen
LOGGEN=./bin/loggen.exe
[ -f "$LOGGEN" ] || LOGGEN=./bin/loggen
"$LOGGEN" --proto=tcp --host=127.0.0.1 --port=6601 --eps=500 --duration=10s --vendors=all

echo ""
echo "### 4/5 — waiting ~65s for the demo segment-seal window, then syncing Presto partitions"
sleep 65
docker exec ulpf-presto presto-cli --server localhost:8080 --catalog lake --schema ulpf \
  --execute "CALL system.sync_partition_metadata('ulpf', 'events', 'FULL')"

echo ""
echo "### 5/5 — real results, queried live"
echo "--- total normalized events in the lake ---"
docker exec ulpf-presto presto-cli --server localhost:8080 --catalog lake --schema ulpf \
  --execute "SELECT count(*) AS total_events FROM events"

echo "--- per-vendor breakdown ---"
docker exec ulpf-presto presto-cli --server localhost:8080 --catalog lake --schema ulpf \
  --execute "SELECT vendor, count(*) FROM events GROUP BY vendor ORDER BY 2 DESC"

echo "--- integrity: Merkle chain verification for today's segments ---"
docker exec -e MINIO_ENDPOINT=minio:9000 -e MINIO_ACCESS_KEY=ulpfadmin \
  -e MINIO_SECRET_KEY=ulpf_dev_only -e MINIO_RAW_BUCKET=ulpf-raw -e MINIO_USE_SSL=false \
  ulpf-collector /app/ulpfctl vault verify --from "$(date -u +%Y-%m-%d)" --to "$(date -u +%Y-%m-%d)" || true

echo ""
echo "Done. Open the console at http://localhost:5173 (admin/admin) — Pipeline"
echo "and Traceability show this same data live, and Traceability's recent-"
echo "events list lets you click any event to see it trace through every"
echo "phase: PRESERVE -> IDENTIFY+PARSE -> NORMALIZE -> ENRICH -> VALIDATE -> ROUTE."
