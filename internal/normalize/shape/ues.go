// Package shape renders a normalized UES event into alternate output
// shapes: UES, ECS, OCSF and CEF — proof that separating extraction from
// mapping (see internal/normalize) buys real SIEM/data-lake portability,
// not just a slogan. Same input event, four valid, independently
// inspectable outputs.
package shape

import "github.com/ulpf/ulpf/internal/schema"

// UES returns the event exactly as normalized — the native shape every
// other Render function starts from.
func UES(e *schema.Event) *schema.Event {
	return e
}
