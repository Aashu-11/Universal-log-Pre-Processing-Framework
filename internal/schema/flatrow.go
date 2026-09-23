package schema

// FlatRow is the underscored, flat projection of Event used by every
// consumer that isn't the nested JSON wire format: the Parquet lake sink,
// the Kafka JSON stream sink (its field names must match
// deploy/presto/etc/kafka/logkrama.events_normalized.json exactly), and
// schema/presto/ddl.sql. Namespace.field becomes namespace_field — e.g.
// Event.Src.IP becomes FlatRow.SrcIP with column name "src_ip".
//
// dt/hour/vendor are Hive/Presto partition columns, derived from
// event.observed_at and observer.vendor at write time, not carried on Event
// itself.
type FlatRow struct {
	EventID                 string `json:"event_id" parquet:"event_id"`
	EventKind               string `json:"event_kind" parquet:"event_kind"`
	EventCategory           string `json:"event_category,omitempty" parquet:"event_category,optional"`
	EventType               string `json:"event_type,omitempty" parquet:"event_type,optional"`
	EventAction             string `json:"event_action,omitempty" parquet:"event_action,optional"`
	EventOutcome            string `json:"event_outcome,omitempty" parquet:"event_outcome,optional"`
	EventSeverityID         int64  `json:"event_severity_id" parquet:"event_severity_id"`
	EventOriginalTimeString string `json:"event_original_time_string,omitempty" parquet:"event_original_time_string,optional"`
	EventObservedAt         int64  `json:"event_observed_at" parquet:"event_observed_at"`
	EventIngestedAt         int64  `json:"event_ingested_at" parquet:"event_ingested_at"`
	EventTimeSkewMs         int64  `json:"event_time_skew_ms" parquet:"event_time_skew_ms"`
	EventDataset            string `json:"event_dataset" parquet:"event_dataset"`

	ObserverVendor   string `json:"observer_vendor" parquet:"observer_vendor"`
	ObserverProduct  string `json:"observer_product" parquet:"observer_product"`
	ObserverType     string `json:"observer_type" parquet:"observer_type"`
	ObserverHostname string `json:"observer_hostname,omitempty" parquet:"observer_hostname,optional"`
	ObserverIP       string `json:"observer_ip,omitempty" parquet:"observer_ip,optional"`
	ObserverVersion  string `json:"observer_version,omitempty" parquet:"observer_version,optional"`

	SrcIP       string `json:"src_ip,omitempty" parquet:"src_ip,optional"`
	SrcPort     int64  `json:"src_port,omitempty" parquet:"src_port,optional"`
	SrcMAC      string `json:"src_mac,omitempty" parquet:"src_mac,optional"`
	SrcHostname string `json:"src_hostname,omitempty" parquet:"src_hostname,optional"`
	SrcUser     string `json:"src_user,omitempty" parquet:"src_user,optional"`
	SrcZone     string `json:"src_zone,omitempty" parquet:"src_zone,optional"`

	DstIP       string `json:"dst_ip,omitempty" parquet:"dst_ip,optional"`
	DstPort     int64  `json:"dst_port,omitempty" parquet:"dst_port,optional"`
	DstMAC      string `json:"dst_mac,omitempty" parquet:"dst_mac,optional"`
	DstHostname string `json:"dst_hostname,omitempty" parquet:"dst_hostname,optional"`
	DstZone     string `json:"dst_zone,omitempty" parquet:"dst_zone,optional"`

	NetworkTransport  string `json:"network_transport,omitempty" parquet:"network_transport,optional"`
	NetworkDirection  string `json:"network_direction,omitempty" parquet:"network_direction,optional"`
	NetworkBytesIn    int64  `json:"network_bytes_in,omitempty" parquet:"network_bytes_in,optional"`
	NetworkBytesOut   int64  `json:"network_bytes_out,omitempty" parquet:"network_bytes_out,optional"`
	NetworkBytesTotal int64  `json:"network_bytes_total,omitempty" parquet:"network_bytes_total,optional"`
	NetworkPacketsIn  int64  `json:"network_packets_in,omitempty" parquet:"network_packets_in,optional"`
	NetworkPacketsOut int64  `json:"network_packets_out,omitempty" parquet:"network_packets_out,optional"`
	NetworkDurationMs int64  `json:"network_duration_ms,omitempty" parquet:"network_duration_ms,optional"`
	NetworkProtocol   string `json:"network_protocol,omitempty" parquet:"network_protocol,optional"`

	HTTPMethod     string `json:"http_method,omitempty" parquet:"http_method,optional"`
	HTTPStatusCode int64  `json:"http_status_code,omitempty" parquet:"http_status_code,optional"`
	HTTPUserAgent  string `json:"http_user_agent,omitempty" parquet:"http_user_agent,optional"`

	URLFull   string `json:"url_full,omitempty" parquet:"url_full,optional"`
	URLDomain string `json:"url_domain,omitempty" parquet:"url_domain,optional"`
	URLPath   string `json:"url_path,omitempty" parquet:"url_path,optional"`

	DNSQuestionName string `json:"dns_question_name,omitempty" parquet:"dns_question_name,optional"`
	DNSQuestionType string `json:"dns_question_type,omitempty" parquet:"dns_question_type,optional"`
	DNSResponseCode string `json:"dns_response_code,omitempty" parquet:"dns_response_code,optional"`

	TLSVersion string `json:"tls_version,omitempty" parquet:"tls_version,optional"`
	TLSSNI     string `json:"tls_sni,omitempty" parquet:"tls_sni,optional"`
	TLSCipher  string `json:"tls_cipher,omitempty" parquet:"tls_cipher,optional"`

	ActorUser string `json:"actor_user,omitempty" parquet:"actor_user,optional"`
	ActorID   string `json:"actor_id,omitempty" parquet:"actor_id,optional"`

	ThreatSignatureID    string `json:"threat_signature_id,omitempty" parquet:"threat_signature_id,optional"`
	ThreatSignatureName  string `json:"threat_signature_name,omitempty" parquet:"threat_signature_name,optional"`
	ThreatCategory       string `json:"threat_category,omitempty" parquet:"threat_category,optional"`
	ThreatSeverity       string `json:"threat_severity,omitempty" parquet:"threat_severity,optional"`
	ThreatMitreTactic    string `json:"threat_mitre_tactic,omitempty" parquet:"threat_mitre_tactic,optional"`
	ThreatMitreTechnique string `json:"threat_mitre_technique,omitempty" parquet:"threat_mitre_technique,optional"`

	EnrichSrcGeoCountry string  `json:"enrich_src_geo_country,omitempty" parquet:"enrich_src_geo_country,optional"`
	EnrichSrcGeoCity    string  `json:"enrich_src_geo_city,omitempty" parquet:"enrich_src_geo_city,optional"`
	EnrichSrcASN        int64   `json:"enrich_src_asn,omitempty" parquet:"enrich_src_asn,optional"`
	EnrichSrcASOrg      string  `json:"enrich_src_as_org,omitempty" parquet:"enrich_src_as_org,optional"`
	EnrichDstGeoCountry string  `json:"enrich_dst_geo_country,omitempty" parquet:"enrich_dst_geo_country,optional"`
	EnrichDstGeoCity    string  `json:"enrich_dst_geo_city,omitempty" parquet:"enrich_dst_geo_city,optional"`
	EnrichDstASN        int64   `json:"enrich_dst_asn,omitempty" parquet:"enrich_dst_asn,optional"`
	EnrichDstASOrg      string  `json:"enrich_dst_as_org,omitempty" parquet:"enrich_dst_as_org,optional"`
	EnrichSrcIsInternal bool    `json:"enrich_src_is_internal" parquet:"enrich_src_is_internal"`
	EnrichDstIsInternal bool    `json:"enrich_dst_is_internal" parquet:"enrich_dst_is_internal"`
	EnrichIOCMatch      bool    `json:"enrich_ioc_match" parquet:"enrich_ioc_match"`
	EnrichIOCIndicator  string  `json:"enrich_ioc_indicator,omitempty" parquet:"enrich_ioc_indicator,optional"`
	EnrichRiskScore     float64 `json:"enrich_risk_score,omitempty" parquet:"enrich_risk_score,optional"`

	RawSHA256       string `json:"raw_sha256" parquet:"raw_sha256"`
	RawSegmentID    string `json:"raw_segment_id" parquet:"raw_segment_id"`
	RawOffset       int64  `json:"raw_offset" parquet:"raw_offset"`
	RawLength       int64  `json:"raw_length" parquet:"raw_length"`
	RawRetrievalURI string `json:"raw_retrieval_uri" parquet:"raw_retrieval_uri"`

	LineageParserID      string  `json:"lineage_parser_id" parquet:"lineage_parser_id"`
	LineageParserVersion string  `json:"lineage_parser_version" parquet:"lineage_parser_version"`
	LineageParseStatus   string  `json:"lineage_parse_status" parquet:"lineage_parse_status"`
	LineageNodeID        string  `json:"lineage_node_id" parquet:"lineage_node_id"`
	LineageProcessingMs  float64 `json:"lineage_processing_ms" parquet:"lineage_processing_ms"`

	QualityScore float64 `json:"quality_score" parquet:"quality_score"`

	// Unmapped is a MAP(VARCHAR, VARCHAR) in Presto — every extracted field
	// with no UES home, verbatim. Never nil; an empty map still round-trips
	// as {} so "field_loss == 0" checks can distinguish "no unmapped
	// fields" from "unmapped was dropped".
	Unmapped map[string]string `json:"unmapped" parquet:"unmapped"`

	// Partition columns, set by the sink at write time.
	Dt     string `json:"dt" parquet:"dt"`
	Hour   string `json:"hour" parquet:"hour"`
	Vendor string `json:"vendor" parquet:"vendor"`
}

// ToFlatRow projects a nested Event into its flat, underscored form. It does
// not set Dt/Hour/Vendor — the sink derives those from EventObservedAt /
// ObserverVendor at write time, since partitioning is a storage concern, not
// a schema concern.
func (e *Event) ToFlatRow() FlatRow {
	row := FlatRow{
		EventID:                 e.Event.ID,
		EventKind:               e.Event.Kind,
		EventCategory:           e.Event.Category,
		EventType:               e.Event.Type,
		EventAction:             e.Event.Action,
		EventOutcome:            e.Event.Outcome,
		EventSeverityID:         int64(e.Event.SeverityID),
		EventOriginalTimeString: e.Event.OriginalTimeString,
		EventObservedAt:         e.Event.ObservedAt,
		EventIngestedAt:         e.Event.IngestedAt,
		EventTimeSkewMs:         e.Event.TimeSkewMs,
		EventDataset:            e.Event.Dataset,

		ObserverVendor:   e.Observer.Vendor,
		ObserverProduct:  e.Observer.Product,
		ObserverType:     e.Observer.Type,
		ObserverHostname: e.Observer.Hostname,
		ObserverIP:       e.Observer.IP,
		ObserverVersion:  e.Observer.Version,

		SrcIP:       e.Src.IP,
		SrcPort:     int64(e.Src.Port),
		SrcMAC:      e.Src.MAC,
		SrcHostname: e.Src.Hostname,
		SrcUser:     e.Src.User,
		SrcZone:     e.Src.Zone,

		DstIP:       e.Dst.IP,
		DstPort:     int64(e.Dst.Port),
		DstMAC:      e.Dst.MAC,
		DstHostname: e.Dst.Hostname,
		DstZone:     e.Dst.Zone,

		NetworkTransport:  e.Network.Transport,
		NetworkDirection:  e.Network.Direction,
		NetworkBytesIn:    e.Network.BytesIn,
		NetworkBytesOut:   e.Network.BytesOut,
		NetworkBytesTotal: e.Network.BytesTotal,
		NetworkPacketsIn:  e.Network.PacketsIn,
		NetworkPacketsOut: e.Network.PacketsOut,
		NetworkDurationMs: e.Network.DurationMs,
		NetworkProtocol:   e.Network.Protocol,

		HTTPMethod:     e.HTTP.Method,
		HTTPStatusCode: int64(e.HTTP.StatusCode),
		HTTPUserAgent:  e.HTTP.UserAgent,

		URLFull:   e.URL.Full,
		URLDomain: e.URL.Domain,
		URLPath:   e.URL.Path,

		DNSQuestionName: e.DNS.QuestionName,
		DNSQuestionType: e.DNS.QuestionType,
		DNSResponseCode: e.DNS.ResponseCode,

		TLSVersion: e.TLS.Version,
		TLSSNI:     e.TLS.SNI,
		TLSCipher:  e.TLS.Cipher,

		ActorUser: e.Actor.User,
		ActorID:   e.Actor.ID,

		ThreatSignatureID:    e.Threat.SignatureID,
		ThreatSignatureName:  e.Threat.SignatureName,
		ThreatCategory:       e.Threat.Category,
		ThreatSeverity:       e.Threat.Severity,
		ThreatMitreTactic:    e.Threat.MitreTactic,
		ThreatMitreTechnique: e.Threat.MitreTechnique,

		EnrichSrcGeoCountry: e.Enrich.SrcGeoCountry,
		EnrichSrcGeoCity:    e.Enrich.SrcGeoCity,
		EnrichSrcASN:        int64(e.Enrich.SrcASN),
		EnrichSrcASOrg:      e.Enrich.SrcASOrg,
		EnrichDstGeoCountry: e.Enrich.DstGeoCountry,
		EnrichDstGeoCity:    e.Enrich.DstGeoCity,
		EnrichDstASN:        int64(e.Enrich.DstASN),
		EnrichDstASOrg:      e.Enrich.DstASOrg,
		EnrichIOCIndicator:  e.Enrich.IOCIndicator,
		EnrichRiskScore:     e.Enrich.RiskScore,

		RawSHA256:       e.Raw.SHA256,
		RawSegmentID:    e.Raw.SegmentID,
		RawOffset:       e.Raw.Offset,
		RawLength:       e.Raw.Length,
		RawRetrievalURI: e.Raw.RetrievalURI,

		LineageParserID:      e.Lineage.ParserID,
		LineageParserVersion: e.Lineage.ParserVersion,
		LineageParseStatus:   e.Lineage.ParseStatus,
		LineageNodeID:        e.Lineage.NodeID,
		LineageProcessingMs:  e.Lineage.ProcessingMs,

		QualityScore: e.Quality.Score,

		Unmapped: e.Unmapped,
	}
	if e.Enrich.SrcIsInternal != nil {
		row.EnrichSrcIsInternal = *e.Enrich.SrcIsInternal
	}
	if e.Enrich.DstIsInternal != nil {
		row.EnrichDstIsInternal = *e.Enrich.DstIsInternal
	}
	if e.Enrich.IOCMatch != nil {
		row.EnrichIOCMatch = *e.Enrich.IOCMatch
	}
	if row.Unmapped == nil {
		row.Unmapped = map[string]string{}
	}
	return row
}
