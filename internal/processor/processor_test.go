package processor_test

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/logkrama/logkrama/internal/enrich"
	"github.com/logkrama/logkrama/internal/identify"
	"github.com/logkrama/logkrama/internal/loggen"
	"github.com/logkrama/logkrama/internal/normalize"
	"github.com/logkrama/logkrama/internal/parse"
	"github.com/logkrama/logkrama/internal/parse/ops"
	"github.com/logkrama/logkrama/internal/processor"
	"github.com/logkrama/logkrama/internal/route"
	"github.com/logkrama/logkrama/internal/schema"
	"github.com/logkrama/logkrama/internal/telemetry"
	"github.com/logkrama/logkrama/internal/vault"
)

type memSink struct {
	mu     sync.Mutex
	events []*schema.Event
}

func (m *memSink) Write(_ context.Context, e *schema.Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, e)
	return nil
}
func (m *memSink) Flush(context.Context) error { return nil }
func (m *memSink) Close() error                { return nil }

func newTestProcessor(t *testing.T) (*processor.Processor, *memSink, *memSink, *memSink) {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	registry := parse.NewRegistry(ops.Deps{})
	if _, err := registry.LoadDir(filepath.Join(root, "packs")); err != nil {
		t.Fatalf("load packs: %v", err)
	}
	mappings, err := normalize.LoadMappingsDir(filepath.Join(root, "packs"))
	if err != nil {
		t.Fatalf("load mappings: %v", err)
	}
	dicts, err := normalize.LoadDictionaries(filepath.Join(root, "config", "dictionaries"))
	if err != nil {
		t.Fatalf("load dictionaries: %v", err)
	}
	enrichPipeline, err := enrich.NewDefaultPipeline(filepath.Join(root, "enrichment"), nil, telemetry.New(prometheus.NewRegistry()))
	if err != nil {
		t.Fatalf("build enrich pipeline: %v", err)
	}

	lake := &memSink{}
	stream := &memSink{}
	dlq := &memSink{}
	router := &route.Router{Lake: lake, Stream: stream, DLQ: dlq}

	p := &processor.Processor{
		Resolver: identify.NewResolver(registry),
		Mappings: mappings,
		Dicts:    dicts,
		Enrich:   enrichPipeline,
		Router:   router,
		NodeID:   "test-node",
	}
	return p, lake, stream, dlq
}

func TestProcessorEndToEndKnownVendor(t *testing.T) {
	p, lake, stream, dlq := newTestProcessor(t)
	gen := loggen.NewGenerator(11)
	now := time.Date(2026, 9, 7, 14, 2, 11, 0, time.UTC)
	line := gen.Line(loggen.VendorPaloAlto, now)

	ref := vault.RawRef{
		SegmentID: "2026-09-07-deadbeef", Offset: 0, Length: int64(len(line)),
		SHA256: "9a134e421a5579eba9617f5817d6f2929755b22f2c12eef31b4a89a955589bf5",
	}
	env := processor.Envelope{ListenerID: "syslog-tcp", PeerIP: "10.1.1.1", ReceivedAt: time.Now()}

	if err := p.Process(context.Background(), "evt-1", []byte(line), ref, env); err != nil {
		t.Fatalf("Process: %v", err)
	}

	if len(lake.events) != 1 {
		t.Fatalf("lake got %d events, want 1", len(lake.events))
	}
	if len(stream.events) != 1 {
		t.Fatalf("stream got %d events, want 1", len(stream.events))
	}
	e := lake.events[0]

	if e.Event.ID != "evt-1" {
		t.Errorf("event.id = %q, want evt-1", e.Event.ID)
	}
	if e.Lineage.ParserID != "paloalto.panos.traffic" || e.Lineage.ParseStatus != "ok" {
		t.Errorf("lineage wrong: %+v", e.Lineage)
	}
	if e.Raw.SHA256 == "" || e.Raw.SegmentID == "" || e.Raw.RetrievalURI == "" {
		t.Errorf("raw pointer incomplete (Q5's missing_raw_ref check would fail): %+v", e.Raw)
	}
	if e.Quality.Score <= 0 {
		t.Errorf("quality.score = %v, want > 0", e.Quality.Score)
	}
	if e.Enrich.SrcGeoCountry == "" && e.Enrich.SrcIsInternal == nil {
		t.Error("enrichment did not run at all")
	}
	if len(dlq.events) != 0 {
		t.Errorf("clean event should not reach DLQ, got %d", len(dlq.events))
	}
}

func TestProcessorUnknownSourceNeverDrops(t *testing.T) {
	p, lake, _, dlq := newTestProcessor(t)
	ref := vault.RawRef{SegmentID: "2026-09-07-unknown", Offset: 0, Length: 10, SHA256: "xyz"}
	env := processor.Envelope{ListenerID: "syslog-tcp", PeerIP: "10.1.1.1", ReceivedAt: time.Now()}

	if err := p.Process(context.Background(), "evt-2", []byte("totally unrecognized garbage"), ref, env); err != nil {
		t.Fatalf("Process: %v", err)
	}

	if len(lake.events) != 1 {
		t.Fatalf("unknown-source event must still reach lake, got %d events", len(lake.events))
	}
	e := lake.events[0]
	if e.Lineage.ParseStatus != schema.ParseStatusFailed {
		t.Errorf("parse_status = %q, want failed", e.Lineage.ParseStatus)
	}
	if e.Lineage.ParserID != identify.UnknownSourceTag {
		t.Errorf("parser_id = %q, want %q", e.Lineage.ParserID, identify.UnknownSourceTag)
	}
	if len(dlq.events) != 1 {
		t.Errorf("unknown-source event should also land in DLQ, got %d", len(dlq.events))
	}
}
