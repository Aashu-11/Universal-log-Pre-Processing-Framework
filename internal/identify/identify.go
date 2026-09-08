// Package identify resolves an inbound raw event to a log_source_id and
// parser via binding lookup, structural sniffing and signature matching.
package identify

import (
	"strings"
	"sync"

	"github.com/ulpf/ulpf/internal/parse"
)

// UnknownSourceTag is set on lineage.parser_id when no tier resolves a
// parser. Per CLAUDE.md, unknown sources are tagged and queued for
// onboarding — never dropped.
const UnknownSourceTag = "source.unknown"

// Shape is the coarse structural category from the sniff tier — informative
// (telemetry, onboarding hints), not itself a resolution: the signature
// tier still confirms with each candidate parser's real match predicates.
type Shape string

const (
	ShapeJSON    Shape = "json"
	ShapeCEF     Shape = "cef"
	ShapeLEEF    Shape = "leef"
	ShapeSyslog  Shape = "syslog"
	ShapeUnknown Shape = "unknown"
)

// Resolution is the outcome of resolving one raw event.
type Resolution struct {
	Plan  *parse.Plan
	Tier  string // "binding" | "signature" | "unknown"
	Shape Shape
}

// Resolver implements the three-tier resolution CLAUDE.md specifies:
// binding lookup (O(1) map) -> structural sniff (shape hint) -> signature
// match (iterate candidate parsers' match predicates, highest priority
// first). At prototype scale (single-digit-to-low-tens of parsers) a linear
// scan over Registry.All() for the signature tier is simpler and fast
// enough; an Aho-Corasick literal pre-filter would be the next step if the
// pack catalog grows into the hundreds — see docs/DECISIONS.md.
type Resolver struct {
	registry *parse.Registry

	mu       sync.RWMutex
	bindings map[string]string // binding key (e.g. "listener:peerIP") -> parser id
}

func NewResolver(registry *parse.Registry) *Resolver {
	return &Resolver{registry: registry, bindings: make(map[string]string)}
}

// BindKey builds the binding-lookup key from listener id and peer identity
// (IP, or file path for the file listener) — the same shape
// internal/collector.Envelope carries, so callers can build it directly
// from an Envelope without this package depending on the collector package.
func BindKey(listenerID, peer string) string {
	return listenerID + ":" + peer
}

// Bind records a binding: future events from this exact listener+peer
// resolve straight to parserID with no sniffing or matching. Set by the
// control plane's source registry (Phase 7) when an operator pins a source.
func (r *Resolver) Bind(listenerID, peer, parserID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.bindings[BindKey(listenerID, peer)] = parserID
}

// Resolve runs all three tiers in order and returns the first hit.
func (r *Resolver) Resolve(listenerID, peer string, raw []byte) Resolution {
	shape := sniff(raw)

	r.mu.RLock()
	boundID, bound := r.bindings[BindKey(listenerID, peer)]
	r.mu.RUnlock()
	if bound {
		if plan, ok := r.registry.Get(boundID); ok {
			return Resolution{Plan: plan, Tier: "binding", Shape: shape}
		}
	}

	for _, plan := range r.registry.All() {
		if matched, _ := plan.Matches(raw); matched {
			return Resolution{Plan: plan, Tier: "signature", Shape: shape}
		}
	}

	return Resolution{Plan: nil, Tier: "unknown", Shape: shape}
}

// sniff makes a cheap first-bytes guess at structure, used for telemetry
// and to steer the Phase 8 onboarding engine's initial format choice — it
// never gates matching on its own.
func sniff(raw []byte) Shape {
	s := strings.TrimSpace(string(raw))
	switch {
	case strings.HasPrefix(s, "{"):
		return ShapeJSON
	case strings.Contains(s, "CEF:"):
		return ShapeCEF
	case strings.Contains(s, "LEEF:"):
		return ShapeLEEF
	case strings.HasPrefix(s, "<"):
		return ShapeSyslog
	default:
		return ShapeUnknown
	}
}
