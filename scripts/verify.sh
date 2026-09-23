#!/usr/bin/env bash
# `make verify` — checks every LOGKRAMA service (infra + app layer) is actually
# up and responding, not just that `docker compose ps` shows "running".
# Exits non-zero if anything fails, printing a PASS/FAIL table either way.
set -uo pipefail
cd "$(dirname "$0")/.."
export MSYS_NO_PATHCONV=1

PASS=0
FAIL=0

check() {
  local name="$1" cmd="$2"
  if eval "$cmd" >/dev/null 2>&1; then
    printf "  PASS  %s\n" "$name"
    PASS=$((PASS+1))
  else
    printf "  FAIL  %s\n" "$name"
    FAIL=$((FAIL+1))
  fi
}

echo "=== Infrastructure ==="
check "postgres"          "docker exec logkrama-postgres pg_isready -U logkrama -d logkrama_meta"
check "minio"              "curl -sf http://localhost:9000/minio/health/live"
check "kafka"              "docker exec ulpf-kafka /opt/kafka/bin/kafka-broker-api-versions.sh --bootstrap-server localhost:9092"
check "hive-metastore"     "bash -c 'echo > /dev/tcp/127.0.0.1/9083'"
check "presto"             "curl -sf http://localhost:8080/v1/info | grep -q '\"starting\":false'"
check "prometheus"         "curl -sf http://localhost:9090/-/healthy"
check "grafana"            "curl -sf http://localhost:3000/api/health"

echo "=== LOGKRAMA application layer ==="
check "collector metrics"  "curl -sf http://localhost:9100/metrics"
check "processor metrics"  "curl -sf http://localhost:9101/metrics"
check "control-plane"      "curl -sf http://localhost:8000/healthz"
check "onboarding"         "curl -sf http://localhost:8001/docs"
check "console"            "curl -sf http://localhost:5173/"

echo "=== Presto federated catalogs ==="
check "lake catalog"   "docker exec logkrama-presto presto-cli --server localhost:8080 --catalog lake --schema logkrama --execute 'SELECT 1'"
check "vault catalog"  "docker exec logkrama-presto presto-cli --server localhost:8080 --catalog vault --schema logkrama --execute 'SELECT 1'"
check "meta catalog"   "docker exec logkrama-presto presto-cli --server localhost:8080 --catalog meta --schema information_schema --execute 'SELECT 1'"
check "stream catalog" "docker exec logkrama-presto presto-cli --server localhost:8080 --catalog stream --execute 'SHOW SCHEMAS'"

echo ""
echo "$PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
