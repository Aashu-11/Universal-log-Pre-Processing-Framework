package normalize

import "github.com/ulpf/ulpf/internal/schema"

// FailedEvent builds the minimal valid UES event for the total-failure
// path: identify found no parser at all, or the matched parser's pipeline
// extracted nothing. Per CLAUDE.md, this case is never dropped — it still
// carries a valid raw pointer and lineage so it's traceable and
// re-processable once a parser exists, just with parse_status=failed and
// quality.score=0.
func FailedEvent(id string, ingestedAtNs int64, raw schema.Raw, lineage schema.Lineage) *schema.Event {
	lineage.ParseStatus = schema.ParseStatusFailed
	return &schema.Event{
		Event: schema.EventMeta{
			ID:         id,
			Kind:       "event",
			Dataset:    lineage.ParserID,
			IngestedAt: ingestedAtNs,
		},
		Observer: schema.Observer{Type: "unknown"},
		Raw:      raw,
		Lineage:  lineage,
		Quality:  schema.Quality{Score: 0},
		Unmapped: map[string]string{},
	}
}
