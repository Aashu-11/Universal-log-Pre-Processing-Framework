package shape

import "github.com/logkrama/logkrama/internal/schema"

// OCSF renders e as an OCSF "Network Activity" event (class_uid 4001,
// category_uid 4 "Network Activity"), the OCSF class perimeter-device
// traffic logs map onto. Field names/shape follow the OCSF 1.x schema for
// that class closely enough to be recognizable and load-bearing for the
// demo, without vendoring the full OCSF schema definitions.
func OCSF(e *schema.Event) map[string]any {
	statusID := 0 // Unknown
	switch e.Event.Outcome {
	case "success":
		statusID = 1
	case "failure":
		statusID = 2
	}

	return map[string]any{
		"class_uid":     4001,
		"class_name":    "Network Activity",
		"category_uid":  4,
		"category_name": "Network Activity",
		"activity_id":   1,
		"activity_name": "Traffic",
		"type_uid":      400101,
		"severity_id":   e.Event.SeverityID,
		"status_id":     statusID,
		"status":        e.Event.Outcome,
		"time":          e.Event.ObservedAt / 1_000_000, // OCSF times are epoch milliseconds

		"src_endpoint": map[string]any{
			"ip":   e.Src.IP,
			"port": e.Src.Port,
		},
		"dst_endpoint": map[string]any{
			"ip":   e.Dst.IP,
			"port": e.Dst.Port,
		},
		"connection_info": map[string]any{
			"protocol_name": e.Network.Transport,
			"direction":     e.Network.Direction,
		},
		"traffic": map[string]any{
			"bytes_in":  e.Network.BytesIn,
			"bytes_out": e.Network.BytesOut,
			"bytes":     e.Network.BytesTotal,
		},
		"metadata": map[string]any{
			"product": map[string]any{
				"vendor_name": e.Observer.Vendor,
				"name":        e.Observer.Product,
				"version":     e.Observer.Version,
			},
			"uid": e.Event.ID,
		},
		"unmapped": e.Unmapped,
	}
}
