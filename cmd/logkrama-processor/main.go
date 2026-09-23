// Command logkrama-processor runs the identify/parse/normalize/enrich/validate/route
// pipeline that turns raw vault references into normalized UES events.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	kg "github.com/segmentio/kafka-go"

	"github.com/logkrama/logkrama/internal/collector"
	"github.com/logkrama/logkrama/internal/enrich"
	"github.com/logkrama/logkrama/internal/identify"
	"github.com/logkrama/logkrama/internal/normalize"
	"github.com/logkrama/logkrama/internal/parse"
	"github.com/logkrama/logkrama/internal/parse/ops"
	"github.com/logkrama/logkrama/internal/processor"
	"github.com/logkrama/logkrama/internal/route"
	sinkkafka "github.com/logkrama/logkrama/internal/sink/kafka"
	sinkparquet "github.com/logkrama/logkrama/internal/sink/parquet"
	"github.com/logkrama/logkrama/internal/telemetry"
	"github.com/logkrama/logkrama/internal/vault"
	"github.com/logkrama/logkrama/internal/vault/store"
)

// Version is set at build time via -ldflags "-X main.Version=...".
var Version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "print version and exit")
	metricsAddr := flag.String("metrics-addr", ":9101", "Prometheus metrics listen address")
	flag.Parse()

	if *showVersion {
		fmt.Printf("logkrama-processor %s\n", Version)
		return
	}

	if err := run(*metricsAddr); err != nil {
		fmt.Fprintf(os.Stderr, "logkrama-processor: %v\n", err)
		os.Exit(1)
	}
}

func run(metricsAddr string) error {
	reg := prometheus.NewRegistry()
	m := telemetry.New(reg)

	rawStore, err := openRawStore()
	if err != nil {
		return fmt.Errorf("open raw store: %w", err)
	}
	lakeStore, err := openLakeStore()
	if err != nil {
		return fmt.Errorf("open lake store: %w", err)
	}
	v := vault.New(rawStore, vault.Config{NodeID: envOr("LOGKRAMA_NODE_ID", "processor-1")})

	packsDir := envOr("LOGKRAMA_PACKS_DIR", "packs")
	registry := parse.NewRegistry(ops.Deps{})
	if _, err := registry.LoadDir(packsDir); err != nil {
		return fmt.Errorf("load packs: %w", err)
	}
	mappings, err := normalize.LoadMappingsDir(packsDir)
	if err != nil {
		return fmt.Errorf("load mappings: %w", err)
	}
	dicts, err := normalize.LoadDictionaries(envOr("LOGKRAMA_DICTIONARIES_DIR", "config/dictionaries"))
	if err != nil {
		return fmt.Errorf("load dictionaries: %w", err)
	}
	enrichPipeline, err := enrich.NewDefaultPipeline(envOr("LOGKRAMA_ENRICHMENT_DIR", "enrichment"), nil, m)
	if err != nil {
		return fmt.Errorf("build enrich pipeline: %w", err)
	}

	brokers := strings.Split(envOr("KAFKA_BROKERS", "localhost:29092"), ",")
	lake := sinkparquet.New(lakeStore, sinkparquet.Config{})
	stream := sinkkafka.New(brokers, envOr("KAFKA_TOPIC_NORMALIZED", "logkrama.events.normalized"))
	dlq := sinkkafka.New(brokers, envOr("KAFKA_TOPIC_DLQ", "logkrama.dlq"))

	proc := &processor.Processor{
		Resolver: identify.NewResolver(registry),
		Mappings: mappings,
		Dicts:    dicts,
		Enrich:   enrichPipeline,
		Router:   &route.Router{Lake: lake, Stream: stream, DLQ: dlq, Metrics: m},
		NodeID:   envOr("LOGKRAMA_NODE_ID", "processor-1"),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reader := kg.NewReader(kg.ReaderConfig{
		Brokers: brokers,
		Topic:   envOr("KAFKA_TOPIC_RAW_REFS", "logkrama.raw.refs"),
		GroupID: "logkrama-processor",
	})
	defer reader.Close()

	// Must comfortably exceed the collector's LOGKRAMA_VAULT_SEGMENT_MAX_SECONDS
	// (default 300s): a ref can be published to Kafka the instant an event
	// lands in a freshly-opened segment, up to that full window before the
	// segment actually seals and its bytes become readable from the vault.
	// A shorter deadline here would silently drop every event whose segment
	// hadn't sealed yet — not a rare edge case, the common one for events
	// near the front of a segment.
	readRetryDeadline := time.Duration(envInt64("LOGKRAMA_VAULT_READ_RETRY_SECONDS", 330)) * time.Second

	errCh := make(chan error, 4)
	go func() { errCh <- consumeLoop(ctx, reader, v, proc, readRetryDeadline, newMissingSegmentCache()) }()

	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	metricsSrv := &http.Server{Addr: metricsAddr, Handler: metricsMux}
	go func() { errCh <- metricsSrv.ListenAndServe() }()

	fmt.Printf("logkrama-processor: consuming %s from %v, metrics=%s\n", envOr("KAFKA_TOPIC_RAW_REFS", "logkrama.raw.refs"), brokers, metricsAddr)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)

	select {
	case <-sig:
		fmt.Println("logkrama-processor: shutting down")
		cancel()
		_ = lake.Flush(context.Background())
		return nil
	case err := <-errCh:
		cancel()
		return err
	}
}

func consumeLoop(ctx context.Context, reader *kg.Reader, v *vault.Vault, proc *processor.Processor, readRetryDeadline time.Duration, missing *missingSegmentCache) error {
	for {
		msg, err := reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("read message: %w", err)
		}

		var rr collector.RawRefMessage
		if err := json.Unmarshal(msg.Value, &rr); err != nil {
			fmt.Fprintf(os.Stderr, "logkrama-processor: bad raw-ref message: %v\n", err)
			continue
		}

		if missing.has(rr.Ref.SegmentID) {
			// Already proved, for a sibling event, that this segment never
			// sealed — a real, permanently missing segment (e.g. orphaned by
			// an ungraceful collector restart, see docs/DECISIONS.md D-007)
			// doesn't become readable later. Paying the full readRetryDeadline
			// again per event would mean a batch of N events referencing the
			// same lost segment costs N*deadline serially — hours, for a
			// batch that size in practice — instead of one.
			continue
		}

		raw, err := readWithRetry(ctx, v, rr.Ref, readRetryDeadline)
		if err != nil {
			fmt.Fprintf(os.Stderr, "logkrama-processor: vault read failed for %s after retries: %v\n", rr.EventID, err)
			missing.add(rr.Ref.SegmentID)
			continue
		}

		env := processor.Envelope{ListenerID: rr.ListenerID, PeerIP: rr.PeerIP, ReceivedAt: rr.ReceivedAt}
		if err := proc.Process(ctx, rr.EventID, raw, rr.Ref, env); err != nil {
			fmt.Fprintf(os.Stderr, "logkrama-processor: process failed for %s: %v\n", rr.EventID, err)
		}
	}
}

// missingSegmentCache remembers segment IDs a full readRetryDeadline has
// already been spent on with no result, for the lifetime of this process.
// Safe to cache permanently (never expired) because the only way v.Read
// keeps failing past the full deadline is a segment that was never sealed
// and never will be — a segment still in the process of sealing succeeds
// well within one deadline window, per the invariant readRetryDeadline is
// configured to (comfortably exceed the collector's own seal window).
type missingSegmentCache struct {
	mu sync.Mutex
	m  map[string]struct{}
}

func newMissingSegmentCache() *missingSegmentCache {
	return &missingSegmentCache{m: make(map[string]struct{})}
}

func (c *missingSegmentCache) has(segmentID string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.m[segmentID]
	return ok
}

func (c *missingSegmentCache) add(segmentID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[segmentID] = struct{}{}
}

// readWithRetry absorbs the eventual-consistency window between "collector
// published this event's ref to Kafka" and "the segment holding those bytes
// actually sealed and landed in MinIO" (PRESERVE finishes on its own
// schedule — 64MB or LOGKRAMA_VAULT_SEGMENT_MAX_SECONDS, whichever first —
// independent of when ROUTE's Kafka notification goes out, and a ref can be
// published the instant an event lands in a freshly-opened segment). A
// consumer that gave up too early would drop every event whose segment
// hadn't sealed yet; retrying with backoff up to deadline (which the caller
// must set comfortably above the collector's actual segment-seal window)
// covers normal seal latency without blocking the partition indefinitely on
// a truly missing segment.
func readWithRetry(ctx context.Context, v *vault.Vault, ref vault.RawRef, deadlineDur time.Duration) ([]byte, error) {
	backoff := 200 * time.Millisecond
	const maxBackoff = 2 * time.Second
	deadline := time.Now().Add(deadlineDur)

	var lastErr error
	for time.Now().Before(deadline) {
		raw, err := v.Read(ctx, ref)
		if err == nil {
			return raw, nil
		}
		lastErr = err
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}
		if backoff < maxBackoff {
			backoff *= 2
		}
	}
	return nil, lastErr
}

func openRawStore() (store.Store, error) {
	if endpoint := os.Getenv("MINIO_ENDPOINT"); endpoint != "" {
		return store.NewMinIO(endpoint, os.Getenv("MINIO_ACCESS_KEY"), os.Getenv("MINIO_SECRET_KEY"),
			envOr("MINIO_RAW_BUCKET", "logkrama-raw"), os.Getenv("MINIO_USE_SSL") == "true")
	}
	return store.NewLocal(envOr("LOGKRAMA_VAULT_LOCAL_DIR", "./data/vault"))
}

func openLakeStore() (store.Store, error) {
	if endpoint := os.Getenv("MINIO_ENDPOINT"); endpoint != "" {
		return store.NewMinIO(endpoint, os.Getenv("MINIO_ACCESS_KEY"), os.Getenv("MINIO_SECRET_KEY"),
			envOr("MINIO_LAKE_BUCKET", "logkrama-lake"), os.Getenv("MINIO_USE_SSL") == "true")
	}
	return store.NewLocal(envOr("LOGKRAMA_LAKE_LOCAL_DIR", "./data/lake"))
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt64(key string, def int64) int64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return def
}
