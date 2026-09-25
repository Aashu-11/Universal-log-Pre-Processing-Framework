# LogKrama benchmarks

Benchmark results are reported with the generator, run length, and machine
context so they can be reproduced instead of treated as marketing claims.

| Workload | Command | Metric captured |
| --- | --- | --- |
| Parser correctness | `go run ./cmd/logkramactl parser test --all` | passing fixtures / total fixtures |
| Enrichment latency | `go test ./internal/enrich -bench BenchmarkEnrich -benchmem` | ns/op, allocations, bytes/op |
| Parser resilience | `go test ./internal/parse -fuzz FuzzParseAllPacks -fuzztime 60s` | executions and panics |
| Ingest throughput | `./bin/loggen.exe --proto=tcp --host=127.0.0.1 --port=6601 --eps=20000 --duration=60 --vendors=all` | requested EPS, received total, loss |

Record host CPU, RAM, OS, Go version, Docker allocation, batch size, and run
duration beside every published result. Publish p50/p95/p99 latency from the
collector and processor metrics when the production-like stack is used.

The current verified baseline is 20,000 TCP events/s sustained for 60 seconds
with no reported send errors, and offline enrichment averages approximately
9.4 microseconds per event. These are baselines, not universal hardware
guarantees.
