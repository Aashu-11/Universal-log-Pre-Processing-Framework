// Command ulpf-collector runs the multi-protocol ingest layer that writes
// incoming logs to the Raw Vault and publishes raw references to Kafka.
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"net/http/pprof" // debug-only: /debug/pprof/* on the metrics listener, for diagnosing stalls
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/ulpf/ulpf/internal/collector"
	"github.com/ulpf/ulpf/internal/collector/batch"
	"github.com/ulpf/ulpf/internal/collector/listener"
	"github.com/ulpf/ulpf/internal/sink/vaultindex"
	"github.com/ulpf/ulpf/internal/telemetry"
	"github.com/ulpf/ulpf/internal/vault"
	"github.com/ulpf/ulpf/internal/vault/store"
)

// Version is set at build time via -ldflags "-X main.Version=...".
var Version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "print version and exit")
	metricsAddr := flag.String("metrics-addr", ":9100", "Prometheus metrics listen address")
	flag.Parse()

	if *showVersion {
		fmt.Printf("ulpf-collector %s\n", Version)
		return
	}

	if err := run(*metricsAddr); err != nil {
		fmt.Fprintf(os.Stderr, "ulpf-collector: %v\n", err)
		os.Exit(1)
	}
}

func run(metricsAddr string) error {
	reg := prometheus.NewRegistry()
	m := telemetry.New(reg)

	st, err := openStore()
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	v := vault.New(st, vault.Config{
		MaxSegmentBytes: envInt64("ULPF_VAULT_SEGMENT_MAX_BYTES", 64<<20),
		MaxSegmentAge:   time.Duration(envInt64("ULPF_VAULT_SEGMENT_MAX_SECONDS", 300)) * time.Second,
		NodeID:          envOr("ULPF_NODE_ID", "collector-1"),
	})
	if err := v.Bootstrap(context.Background()); err != nil {
		return fmt.Errorf("bootstrap vault chain: %w", err)
	}

	udpBuf, err := batch.New(batch.Config{ListenerID: "syslog-udp", SpoolDir: os.TempDir()})
	if err != nil {
		return err
	}
	tcpBuf, err := batch.New(batch.Config{ListenerID: "syslog-tcp", SpoolDir: os.TempDir()})
	if err != nil {
		return err
	}
	httpBuf, err := batch.New(batch.Config{ListenerID: "http-bulk", SpoolDir: os.TempDir()})
	if err != nil {
		return err
	}
	fileDir := envOr("ULPF_FILE_TAIL_DIR", "./data/filedrop")
	if err := os.MkdirAll(fileDir, 0o755); err != nil {
		return fmt.Errorf("create file-tail dir: %w", err)
	}
	fileBuf, err := batch.New(batch.Config{ListenerID: "file-tail", SpoolDir: os.TempDir()})
	if err != nil {
		return err
	}

	udp := listener.NewUDP(listener.UDPConfig{Addr: envOr("ULPF_SYSLOG_UDP_ADDR", ":5514"), ListenerID: "syslog-udp"}, udpBuf, m)
	tcp := listener.NewTCP(listener.TCPConfig{Addr: envOr("ULPF_SYSLOG_TCP_ADDR", ":6601"), ListenerID: "syslog-tcp"}, tcpBuf, m)
	httpL := listener.NewHTTP(listener.HTTPConfig{Addr: envOr("ULPF_HTTP_ADDR", ":8088"), ListenerID: "http-bulk"}, httpBuf, m)
	fileL := listener.NewFile(listener.FileConfig{Dir: fileDir, ListenerID: "file-tail"}, fileBuf, m)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 8)
	go func() { errCh <- udp.Serve(ctx) }()
	go func() { errCh <- tcp.Serve(ctx) }()
	go func() { errCh <- httpL.Serve(ctx) }()
	go func() { errCh <- fileL.Serve(ctx) }()

	go reportSpoolDepth(ctx, m, map[string]*batch.Buffer{
		"syslog-udp": udpBuf, "syslog-tcp": tcpBuf, "http-bulk": httpBuf, "file-tail": fileBuf,
	})
	go runSealTicker(ctx, v)

	indexSink := vaultindex.NewIndexSink(st, 0, 0)
	pub := openRefPublisher(indexSink)
	go flushIndexSink(ctx, indexSink)

	pipelines := []*collector.Pipeline{
		collector.NewPipeline(udpBuf, v, pub, m, collector.PipelineConfig{}),
		collector.NewPipeline(tcpBuf, v, pub, m, collector.PipelineConfig{}),
		collector.NewPipeline(httpBuf, v, pub, m, collector.PipelineConfig{}),
		collector.NewPipeline(fileBuf, v, pub, m, collector.PipelineConfig{}),
	}
	for _, p := range pipelines {
		p := p
		go func() { errCh <- p.Run(ctx) }()
	}

	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	// net/http/pprof's init() only registers onto http.DefaultServeMux, which
	// this custom mux doesn't use — wire the same handlers in explicitly so
	// /debug/pprof/* is reachable on the metrics listener for stall diagnosis.
	metricsMux.HandleFunc("/debug/pprof/", pprof.Index)
	metricsMux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	metricsMux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	metricsMux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	metricsMux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	metricsSrv := &http.Server{Addr: metricsAddr, Handler: metricsMux}
	go func() { errCh <- metricsSrv.ListenAndServe() }()

	fmt.Printf("ulpf-collector: listening udp=%s tcp=%s http=%s metrics=%s\n",
		envOr("ULPF_SYSLOG_UDP_ADDR", ":5514"), envOr("ULPF_SYSLOG_TCP_ADDR", ":6601"),
		envOr("ULPF_HTTP_ADDR", ":8088"), metricsAddr)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)

	select {
	case <-sig:
		fmt.Println("ulpf-collector: shutting down")
		cancel()
		return v.Seal(context.Background())
	case err := <-errCh:
		cancel()
		return err
	}
}

func openStore() (store.Store, error) {
	if endpoint := os.Getenv("MINIO_ENDPOINT"); endpoint != "" {
		return store.NewMinIO(endpoint, os.Getenv("MINIO_ACCESS_KEY"), os.Getenv("MINIO_SECRET_KEY"),
			envOr("MINIO_RAW_BUCKET", "ulpf-raw"), os.Getenv("MINIO_USE_SSL") == "true")
	}
	dir := envOr("ULPF_VAULT_LOCAL_DIR", "./data/vault")
	return store.NewLocal(dir)
}

// openRefPublisher returns a real Kafka publisher when KAFKA_BROKERS is
// set, else NoopPublisher — so the collector runs standalone (vault-only,
// no Kafka needed) for local dev/tests — and publishes to ulpf.raw.refs for
// cmd/ulpf-processor to pick up once Kafka is reachable. Every entry is
// additionally fanned out to indexSink so vault.ulpf.raw_index (Presto's
// per-event chain-of-custody index) actually gets populated, without that
// secondary write ever blocking or failing the primary Kafka handoff.
func openRefPublisher(indexSink *vaultindex.IndexSink) collector.RefPublisher {
	var primary collector.RefPublisher = collector.NoopPublisher{}
	if brokersEnv := os.Getenv("KAFKA_BROKERS"); brokersEnv != "" {
		brokers := strings.Split(brokersEnv, ",")
		topic := envOr("KAFKA_TOPIC_RAW_REFS", "ulpf.raw.refs")
		primary = collector.NewKafkaPublisher(brokers, topic)
	}
	return &collector.FanoutPublisher{Primary: primary, Secondary: []collector.RefPublisher{indexSink}}
}

// flushIndexSink periodically rolls buffered raw_index rows to Parquet so
// they land in MinIO well before IndexSink's own maxAge default (5m) would
// otherwise require another Publish call to notice the age threshold; also
// flushes once on shutdown so a partial buffer isn't silently dropped.
func flushIndexSink(ctx context.Context, s *vaultindex.IndexSink) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			_ = s.Flush(context.Background())
			return
		case <-ticker.C:
			if err := s.Flush(ctx); err != nil {
				fmt.Fprintf(os.Stderr, "ulpf-collector: index sink flush failed: %v\n", err)
			}
		}
	}
}

// runSealTicker enforces MaxSegmentAge on a wall-clock schedule independent
// of write volume: WriteBatch only checks that threshold reactively, on the
// next write to the active segment, so a segment that stops receiving
// traffic would otherwise sit open (and its already-published Kafka refs
// unreadable) indefinitely — see docs/DECISIONS.md for the real incident
// this fixes.
func runSealTicker(ctx context.Context, v *vault.Vault) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := v.SealIfStale(ctx); err != nil {
				fmt.Fprintf(os.Stderr, "ulpf-collector: periodic seal check failed: %v\n", err)
			}
		}
	}
}

// reportSpoolDepth periodically publishes each listener's disk-spool depth
// so overflow onto disk is visible in /metrics before it turns into
// backpressure — previously defined but never wired to a real value.
func reportSpoolDepth(ctx context.Context, m *telemetry.Metrics, buffers map[string]*batch.Buffer) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for listenerID, b := range buffers {
				m.SpoolDepth.WithLabelValues(listenerID).Set(float64(b.SpoolDepth()))
			}
		}
	}
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
