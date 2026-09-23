// Package processor wires IDENTIFY -> PARSE -> NORMALIZE -> ENRICH ->
// VALIDATE -> ROUTE into the single call cmd/logkrama-processor makes per raw
// event reference. Kept as its own package (not folded into main.go) so
// the full pipeline is unit-testable end-to-end without Kafka or a live
// processor binary — see processor_test.go.
package processor

import (
	"context"
	"fmt"
	"time"

	"github.com/logkrama/logkrama/internal/enrich"
	"github.com/logkrama/logkrama/internal/identify"
	"github.com/logkrama/logkrama/internal/normalize"
	"github.com/logkrama/logkrama/internal/route"
	"github.com/logkrama/logkrama/internal/schema"
	"github.com/logkrama/logkrama/internal/validate"
	"github.com/logkrama/logkrama/internal/vault"
)

// Envelope is the minimal per-event context the processor needs from the
// collector — deliberately a local, narrow type (not collector.Envelope)
// so this package never depends on internal/collector, which would create
// collector -> vaultindex -> processor -> collector-shaped cycles once
// wired into cmd/logkrama-processor.
type Envelope struct {
	ListenerID string
	PeerIP     string
	ReceivedAt time.Time
}

// Processor holds everything needed to turn one raw event into a routed,
// normalized UES event.
type Processor struct {
	Resolver  *identify.Resolver
	Mappings  map[string]normalize.Mapping
	Dicts     map[string]map[string]string
	Enrich    *enrich.Pipeline
	Router    *route.Router
	NodeID    string
	RawBucket string // e.g. "logkrama-raw"; used only to build raw.retrieval_uri
}

// Process runs the full pipeline on one already-read raw event. raw must be
// the exact bytes vault.Read returned for ref — this function does not read
// the vault itself, so it stays testable with synthetic refs.
func (p *Processor) Process(ctx context.Context, eventID string, raw []byte, ref vault.RawRef, env Envelope) error {
	now := time.Now()
	bucket := p.RawBucket
	if bucket == "" {
		bucket = "logkrama-raw"
	}
	rawField := schema.Raw{
		SHA256: ref.SHA256, SegmentID: ref.SegmentID, Offset: ref.Offset, Length: ref.Length,
		RetrievalURI: vault.RetrievalURI(bucket, ref),
	}

	res := p.Resolver.Resolve(env.ListenerID, env.PeerIP, raw)
	if res.Plan == nil {
		lineage := schema.Lineage{ParserID: identify.UnknownSourceTag, NodeID: p.NodeID}
		event := normalize.FailedEvent(eventID, env.ReceivedAt.UnixNano(), rawField, lineage)
		return p.Router.Route(ctx, event, validate.Result{Violations: []validate.Violation{{Reason: "unknown_source", Message: "no parser matched"}}})
	}

	mapping, ok := p.Mappings[res.Plan.Metadata.ID]
	if !ok {
		lineage := schema.Lineage{ParserID: res.Plan.Metadata.ID, ParserVersion: res.Plan.Metadata.Version, NodeID: p.NodeID}
		event := normalize.FailedEvent(eventID, env.ReceivedAt.UnixNano(), rawField, lineage)
		return p.Router.Route(ctx, event, validate.Result{Violations: []validate.Violation{{Reason: "missing_mapping", Message: fmt.Sprintf("no mapping loaded for parser %q", res.Plan.Metadata.ID)}}})
	}

	parseRes := res.Plan.Run(raw)
	mapper := normalize.NewMapper(mapping, p.Dicts)
	event, mapReport := mapper.Apply(parseRes.Fields, now)

	event.Event.ID = eventID
	event.Event.IngestedAt = env.ReceivedAt.UnixNano()
	if event.Event.ObservedAt == 0 {
		event.Event.ObservedAt = event.Event.IngestedAt
	}
	event.Event.TimeSkewMs = (event.Event.IngestedAt - event.Event.ObservedAt) / int64(time.Millisecond)
	event.Raw = rawField
	event.Lineage = schema.Lineage{
		ParserID: res.Plan.Metadata.ID, ParserVersion: res.Plan.Metadata.Version,
		ParseStatus: parseRes.Status, NodeID: p.NodeID, ProcessingMs: parseRes.ProcessingMs,
	}

	p.Enrich.Run(event)

	expectedFields := len(mapping.Fields)
	valRes := validate.Event(event, mapReport.MappedFields, expectedFields, res.Plan.Confidence, now)
	event.Quality.Score = valRes.QualityScore

	return p.Router.Route(ctx, event, valRes)
}
