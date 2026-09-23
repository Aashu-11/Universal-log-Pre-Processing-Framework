package normalize

import (
	"fmt"

	"github.com/logkrama/logkrama/internal/schema"
)

// setUESField writes val onto the UES field named by its dotted path.
// A fixed switch, not reflection, per CLAUDE.md's hot-path guidance — the
// set of valid UES paths is closed (schema/ues-1.0.json), so a switch is
// both faster and gives a compile-time-checkable place to see every field
// a Mapping artifact is allowed to target. Unknown paths are a mapping
// author error, surfaced immediately rather than silently ignored.
func setUESField(e *schema.Event, path string, val any) error {
	switch path {
	case "event.kind":
		e.Event.Kind = str(val)
	case "event.category":
		e.Event.Category = str(val)
	case "event.type":
		e.Event.Type = str(val)
	case "event.action":
		e.Event.Action = str(val)
	case "event.outcome":
		e.Event.Outcome = str(val)
	case "event.dataset":
		e.Event.Dataset = str(val)
	case "event.severity_id":
		e.Event.SeverityID = int(i64(val))
	case "event.original_time_string":
		e.Event.OriginalTimeString = str(val)
	case "event.observed_at":
		e.Event.ObservedAt = i64(val)

	case "observer.vendor":
		e.Observer.Vendor = str(val)
	case "observer.product":
		e.Observer.Product = str(val)
	case "observer.type":
		e.Observer.Type = str(val)
	case "observer.hostname":
		e.Observer.Hostname = str(val)
	case "observer.ip":
		e.Observer.IP = str(val)
	case "observer.version":
		e.Observer.Version = str(val)

	case "src.ip":
		e.Src.IP = str(val)
	case "src.port":
		e.Src.Port = int(i64(val))
	case "src.mac":
		e.Src.MAC = str(val)
	case "src.hostname":
		e.Src.Hostname = str(val)
	case "src.user":
		e.Src.User = str(val)
	case "src.zone":
		e.Src.Zone = str(val)

	case "dst.ip":
		e.Dst.IP = str(val)
	case "dst.port":
		e.Dst.Port = int(i64(val))
	case "dst.mac":
		e.Dst.MAC = str(val)
	case "dst.hostname":
		e.Dst.Hostname = str(val)
	case "dst.zone":
		e.Dst.Zone = str(val)

	case "network.transport":
		e.Network.Transport = str(val)
	case "network.direction":
		e.Network.Direction = str(val)
	case "network.bytes_in":
		e.Network.BytesIn = i64(val)
	case "network.bytes_out":
		e.Network.BytesOut = i64(val)
	case "network.bytes_total":
		e.Network.BytesTotal = i64(val)
	case "network.packets_in":
		e.Network.PacketsIn = i64(val)
	case "network.packets_out":
		e.Network.PacketsOut = i64(val)
	case "network.duration_ms":
		e.Network.DurationMs = i64(val)
	case "network.protocol":
		e.Network.Protocol = str(val)

	case "http.method":
		e.HTTP.Method = str(val)
	case "http.status_code":
		e.HTTP.StatusCode = int(i64(val))
	case "http.user_agent":
		e.HTTP.UserAgent = str(val)

	case "url.full":
		e.URL.Full = str(val)
	case "url.domain":
		e.URL.Domain = str(val)
	case "url.path":
		e.URL.Path = str(val)

	case "dns.question_name":
		e.DNS.QuestionName = str(val)
	case "dns.question_type":
		e.DNS.QuestionType = str(val)
	case "dns.response_code":
		e.DNS.ResponseCode = str(val)

	case "tls.version":
		e.TLS.Version = str(val)
	case "tls.sni":
		e.TLS.SNI = str(val)
	case "tls.cipher":
		e.TLS.Cipher = str(val)

	case "actor.user":
		e.Actor.User = str(val)
	case "actor.id":
		e.Actor.ID = str(val)

	case "threat.signature_id":
		e.Threat.SignatureID = str(val)
	case "threat.signature_name":
		e.Threat.SignatureName = str(val)
	case "threat.category":
		e.Threat.Category = str(val)
	case "threat.severity":
		e.Threat.Severity = str(val)
	case "threat.mitre_tactic":
		e.Threat.MitreTactic = str(val)
	case "threat.mitre_technique":
		e.Threat.MitreTechnique = str(val)

	default:
		return fmt.Errorf("normalize: unknown UES target path %q", path)
	}
	return nil
}

func str(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

func i64(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case float64:
		return int64(n)
	default:
		return 0
	}
}
