package listener

import (
	"bufio"
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"golang.org/x/time/rate"

	"github.com/logkrama/logkrama/internal/collector"
	"github.com/logkrama/logkrama/internal/collector/batch"
	"github.com/logkrama/logkrama/internal/telemetry"
)

// HTTPConfig configures the HTTP bulk ingest endpoint (POST /v1/ingest,
// NDJSON, optionally gzip-compressed).
type HTTPConfig struct {
	Addr            string
	ListenerID      string
	MaxBodyBytes    int64
	RateLimitPerSec float64
	RateBurst       int
}

func (c HTTPConfig) withDefaults() HTTPConfig {
	if c.MaxBodyBytes <= 0 {
		c.MaxBodyBytes = 32 << 20 // 32MB
	}
	if c.RateLimitPerSec <= 0 {
		c.RateLimitPerSec = 5000
	}
	if c.RateBurst <= 0 {
		c.RateBurst = int(c.RateLimitPerSec)
	}
	return c
}

// HTTP is the bulk ingest listener: each line of the (optionally gzipped)
// NDJSON body is stored as exactly one raw event, byte-for-byte — we do not
// re-serialize the JSON, so whatever the client sent is exactly what lands
// in the vault.
type HTTP struct {
	cfg     HTTPConfig
	buf     *batch.Buffer
	metrics *telemetry.Metrics
	limiter *rate.Limiter
	server  *http.Server
}

func NewHTTP(cfg HTTPConfig, buf *batch.Buffer, m *telemetry.Metrics) *HTTP {
	cfg = cfg.withDefaults()
	return &HTTP{
		cfg:     cfg,
		buf:     buf,
		metrics: m,
		limiter: rate.NewLimiter(rate.Limit(cfg.RateLimitPerSec), cfg.RateBurst),
	}
}

func (h *HTTP) ID() string { return h.cfg.ListenerID }

type ingestResponse struct {
	BatchID      string `json:"batch_id"`
	EventsQueued int    `json:"events_queued"`
}

func (h *HTTP) Serve(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/ingest", h.handleIngest)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	h.server = &http.Server{Addr: h.cfg.Addr, Handler: mux}

	errCh := make(chan error, 1)
	go func() {
		if err := h.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("listener/http: %w", err)
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = h.server.Shutdown(shutdownCtx)
		return nil
	case err := <-errCh:
		return err
	}
}

func (h *HTTP) handleIngest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.limiter.Allow() {
		http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
		return
	}

	peer, _, _ := net.SplitHostPort(r.RemoteAddr)
	body := http.MaxBytesReader(w, r.Body, h.cfg.MaxBodyBytes)
	defer body.Close()

	var reader io.Reader = body
	if r.Header.Get("Content-Encoding") == "gzip" {
		gz, err := gzip.NewReader(body)
		if err != nil {
			http.Error(w, "invalid gzip body", http.StatusBadRequest)
			return
		}
		defer gz.Close()
		reader = gz
	}

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)

	queued := 0
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		payload := make([]byte, len(line))
		copy(payload, line)

		ev := collector.RawEvent{
			Envelope: collector.Envelope{
				ListenerID: h.cfg.ListenerID,
				PeerIP:     peer,
				ReceivedAt: time.Now(),
				ByteLength: len(payload),
			},
			Payload: payload,
		}
		if err := h.buf.Push(ev); err != nil {
			http.Error(w, "backpressure: buffer full, retry later", http.StatusTooManyRequests)
			return
		}
		h.metrics.IngestBytesTotal.WithLabelValues(h.cfg.ListenerID).Add(float64(len(payload)))
		h.metrics.EventsReceivedTotal.WithLabelValues(h.cfg.ListenerID, peer).Inc()
		queued++
	}
	if err := scanner.Err(); err != nil {
		http.Error(w, "error reading body: "+err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(ingestResponse{BatchID: randomBatchID(), EventsQueued: queued})
}

func randomBatchID() string {
	var b [12]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func (h *HTTP) Close() error {
	if h.server != nil {
		return h.server.Close()
	}
	return nil
}
