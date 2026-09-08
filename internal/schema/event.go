// Package schema defines the Go types for the Universal Event Schema (UES),
// including UUIDv7 generation and the flat-row Parquet projection.
package schema

// Event is the canonical in-memory representation of a normalized UES 1.0
// event. Field layout mirrors schema/ues-1.0.json namespace-for-namespace;
// see that file for the authoritative description of every field. Keep the
// two in sync — TestEventRoundTripsAgainstJSONSchema in event_test.go fails
// loudly if they drift.
type Event struct {
	Event    EventMeta `json:"event"`
	Observer Observer  `json:"observer"`
	Src      Src       `json:"src,omitempty"`
	Dst      Dst       `json:"dst,omitempty"`
	Network  Network   `json:"network,omitempty"`
	HTTP     HTTP      `json:"http,omitempty"`
	URL      URL       `json:"url,omitempty"`
	DNS      DNS       `json:"dns,omitempty"`
	TLS      TLS       `json:"tls,omitempty"`
	Actor    Actor     `json:"actor,omitempty"`
	Threat   Threat    `json:"threat,omitempty"`
	Enrich   Enrich    `json:"enrich,omitempty"`
	Raw      Raw       `json:"raw"`
	Lineage  Lineage   `json:"lineage"`
	Quality  Quality   `json:"quality"`

	// Unmapped holds every extracted field with no UES home, verbatim. Never
	// nil in a normalized event — normalize.Mapper always initializes it,
	// even to an empty map, so JSON marshaling never omits the key.
	Unmapped map[string]string `json:"unmapped"`
}

type EventMeta struct {
	ID                 string `json:"id"`
	Kind               string `json:"kind"`
	Category           string `json:"category,omitempty"`
	Type               string `json:"type,omitempty"`
	Action             string `json:"action,omitempty"`
	Outcome            string `json:"outcome,omitempty"`
	SeverityID         int    `json:"severity_id,omitempty"`
	OriginalTimeString string `json:"original_time_string,omitempty"`
	ObservedAt         int64  `json:"observed_at"`
	IngestedAt         int64  `json:"ingested_at"`
	TimeSkewMs         int64  `json:"time_skew_ms,omitempty"`
	Dataset            string `json:"dataset"`
}

type Observer struct {
	Vendor   string `json:"vendor"`
	Product  string `json:"product"`
	Type     string `json:"type"`
	Hostname string `json:"hostname,omitempty"`
	IP       string `json:"ip,omitempty"`
	Version  string `json:"version,omitempty"`
}

type Src struct {
	IP       string `json:"ip,omitempty"`
	Port     int    `json:"port,omitempty"`
	MAC      string `json:"mac,omitempty"`
	Hostname string `json:"hostname,omitempty"`
	User     string `json:"user,omitempty"`
	Zone     string `json:"zone,omitempty"`
}

type Dst struct {
	IP       string `json:"ip,omitempty"`
	Port     int    `json:"port,omitempty"`
	MAC      string `json:"mac,omitempty"`
	Hostname string `json:"hostname,omitempty"`
	Zone     string `json:"zone,omitempty"`
}

type Network struct {
	Transport  string `json:"transport,omitempty"`
	Direction  string `json:"direction,omitempty"`
	BytesIn    int64  `json:"bytes_in,omitempty"`
	BytesOut   int64  `json:"bytes_out,omitempty"`
	BytesTotal int64  `json:"bytes_total,omitempty"`
	PacketsIn  int64  `json:"packets_in,omitempty"`
	PacketsOut int64  `json:"packets_out,omitempty"`
	DurationMs int64  `json:"duration_ms,omitempty"`
	Protocol   string `json:"protocol,omitempty"`
}

type HTTP struct {
	Method     string `json:"method,omitempty"`
	StatusCode int    `json:"status_code,omitempty"`
	UserAgent  string `json:"user_agent,omitempty"`
}

type URL struct {
	Full   string `json:"full,omitempty"`
	Domain string `json:"domain,omitempty"`
	Path   string `json:"path,omitempty"`
}

type DNS struct {
	QuestionName string `json:"question_name,omitempty"`
	QuestionType string `json:"question_type,omitempty"`
	ResponseCode string `json:"response_code,omitempty"`
}

type TLS struct {
	Version string `json:"version,omitempty"`
	SNI     string `json:"sni,omitempty"`
	Cipher  string `json:"cipher,omitempty"`
}

type Actor struct {
	User string `json:"user,omitempty"`
	ID   string `json:"id,omitempty"`
}

type Threat struct {
	SignatureID    string `json:"signature_id,omitempty"`
	SignatureName  string `json:"signature_name,omitempty"`
	Category       string `json:"category,omitempty"`
	Severity       string `json:"severity,omitempty"`
	MitreTactic    string `json:"mitre_tactic,omitempty"`
	MitreTechnique string `json:"mitre_technique,omitempty"`
}

type Enrich struct {
	SrcGeoCountry string             `json:"src_geo_country,omitempty"`
	SrcGeoCity    string             `json:"src_geo_city,omitempty"`
	SrcASN        int                `json:"src_asn,omitempty"`
	SrcASOrg      string             `json:"src_as_org,omitempty"`
	DstGeoCountry string             `json:"dst_geo_country,omitempty"`
	DstGeoCity    string             `json:"dst_geo_city,omitempty"`
	DstASN        int                `json:"dst_asn,omitempty"`
	DstASOrg      string             `json:"dst_as_org,omitempty"`
	SrcIsInternal *bool              `json:"src_is_internal,omitempty"`
	DstIsInternal *bool              `json:"dst_is_internal,omitempty"`
	IOCMatch      *bool              `json:"ioc_match,omitempty"`
	IOCIndicator  string             `json:"ioc_indicator,omitempty"`
	RiskScore     float64            `json:"risk_score,omitempty"`
	RiskFactors   map[string]float64 `json:"risk_factors,omitempty"`
}

// Raw points back to the exact original bytes in the Raw Vault. Mandatory on
// every event, including events whose parse failed completely.
type Raw struct {
	SHA256       string `json:"sha256"`
	SegmentID    string `json:"segment_id"`
	Offset       int64  `json:"offset"`
	Length       int64  `json:"length"`
	RetrievalURI string `json:"retrieval_uri"`
}

// Lineage records which parser, which version, on which node, produced this
// event.
type Lineage struct {
	ParserID      string  `json:"parser_id"`
	ParserVersion string  `json:"parser_version"`
	ParseStatus   string  `json:"parse_status"`
	NodeID        string  `json:"node_id"`
	ProcessingMs  float64 `json:"processing_ms,omitempty"`
}

type Quality struct {
	Score float64 `json:"score"`
}

// Parse status values for Lineage.ParseStatus.
const (
	ParseStatusOK      = "ok"
	ParseStatusPartial = "partial"
	ParseStatusFailed  = "failed"
)
