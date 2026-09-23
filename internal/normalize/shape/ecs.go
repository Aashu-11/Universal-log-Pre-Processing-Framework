package shape

import (
	"time"

	"github.com/logkrama/logkrama/internal/schema"
)

// ECS renders e as a subset of Elastic Common Schema (ECS 8.x field
// names): dotted-nested JSON, event.*/source.*/destination.*/network.*/
// observer.*/related.*. Not every ECS field has a UES source, and not
// every UES field has an ECS home (raw.*/lineage.*/unmapped have no ECS
// equivalent) — this covers the fields that matter for the "same event,
// four shapes" demo, not the full ECS field catalog.
func ECS(e *schema.Event) map[string]any {
	out := map[string]any{
		"@timestamp": time.Unix(0, e.Event.ObservedAt).UTC().Format(time.RFC3339Nano),
		"event": map[string]any{
			"id":       e.Event.ID,
			"kind":     e.Event.Kind,
			"category": []string{e.Event.Category},
			"type":     []string{e.Event.Type},
			"action":   e.Event.Action,
			"outcome":  e.Event.Outcome,
			"severity": e.Event.SeverityID,
			"dataset":  e.Event.Dataset,
			"original": e.Event.OriginalTimeString,
		},
		"observer": map[string]any{
			"vendor":  e.Observer.Vendor,
			"product": e.Observer.Product,
			"type":    e.Observer.Type,
			"name":    e.Observer.Hostname,
		},
		"source": map[string]any{
			"ip":      e.Src.IP,
			"port":    e.Src.Port,
			"address": e.Src.IP,
		},
		"destination": map[string]any{
			"ip":      e.Dst.IP,
			"port":    e.Dst.Port,
			"address": e.Dst.IP,
		},
		"network": map[string]any{
			"transport": e.Network.Transport,
			"direction": e.Network.Direction,
			"protocol":  e.Network.Protocol,
			"bytes":     e.Network.BytesTotal,
		},
	}
	if e.Threat.SignatureID != "" {
		out["threat"] = map[string]any{
			"indicator": map[string]any{"provider": e.Observer.Vendor},
			"technique": map[string]any{"id": e.Threat.MitreTechnique, "name": e.Threat.MitreTechnique},
			"tactic":    map[string]any{"id": e.Threat.MitreTactic},
		}
	}
	return out
}
