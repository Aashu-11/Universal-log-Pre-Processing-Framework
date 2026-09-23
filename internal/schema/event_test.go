package schema

import (
	"encoding/json"
	"strings"
	"testing"
)

func sampleEvent() *Event {
	srcInternal := false
	iocMatch := true
	return &Event{
		Event: EventMeta{
			ID:                 NewUUIDv7(),
			Kind:               "event",
			Category:           "network",
			Type:               "connection",
			Action:             "blocked",
			Outcome:            "success",
			SeverityID:         3,
			OriginalTimeString: "Sep  7 14:02:11",
			ObservedAt:         1757253731000000000,
			IngestedAt:         1757253731500000000,
			TimeSkewMs:         500,
			Dataset:            "paloalto.panos.traffic",
		},
		Observer: Observer{
			Vendor: "paloalto", Product: "panos", Type: "firewall",
			Hostname: "pa-edge-01", IP: "10.0.0.5", Version: "10.2.3",
		},
		Src: Src{IP: "203.0.113.5", Port: 51514, Zone: "untrust"},
		Dst: Dst{IP: "10.0.0.10", Port: 443, Zone: "trust"},
		Network: Network{
			Transport: "tcp", Direction: "inbound",
			BytesIn: 1200, BytesOut: 4800, BytesTotal: 6000,
			DurationMs: 120, Protocol: "https",
		},
		Threat: Threat{
			SignatureID: "12345", Category: "exploit",
			MitreTactic: "TA0001", MitreTechnique: "T1190",
		},
		Enrich: Enrich{
			SrcGeoCountry: "US", SrcASN: 15169, SrcASOrg: "Example ISP",
			SrcIsInternal: &srcInternal, IOCMatch: &iocMatch,
			IOCIndicator: "203.0.113.5", RiskScore: 72.5,
			RiskFactors: map[string]float64{"ioc_match": 40, "external_src": 32.5},
		},
		Raw: Raw{
			SHA256:       "9a134e421a5579eba9617f5817d6f2929755b22f2c12eef31b4a89a955589bf5",
			SegmentID:    "seg-0001",
			Offset:       128,
			Length:       256,
			RetrievalURI: "s3a://logkrama-raw/segments/dt=2026-09-07/seg-0001.zst",
		},
		Lineage: Lineage{
			ParserID: "paloalto.panos.traffic", ParserVersion: "1.0.0",
			ParseStatus: ParseStatusOK, NodeID: "node-1", ProcessingMs: 0.42,
		},
		Quality: Quality{Score: 0.95},
		Unmapped: map[string]string{
			"seqno": "42", "actionflags": "0x8000000000000000",
		},
	}
}

func TestEventRoundTripsAgainstJSONSchema(t *testing.T) {
	e := sampleEvent()

	if err := e.Validate(); err != nil {
		t.Fatalf("sample event failed UES schema validation: %v", err)
	}

	b, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got Event
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	b2, err := json.Marshal(&got)
	if err != nil {
		t.Fatalf("re-marshal: %v", err)
	}
	if string(b) != string(b2) {
		t.Fatalf("round trip not byte-identical:\nfirst:  %s\nsecond: %s", b, b2)
	}
}

func TestEventMissingRequiredFieldsFailsValidation(t *testing.T) {
	e := &Event{} // no event.id, no raw.*, no lineage.*, no quality.score
	e.Unmapped = map[string]string{}
	if err := e.Validate(); err == nil {
		t.Fatal("expected validation to fail for an empty event, got nil error")
	}
}

func TestToFlatRowPreservesUnmapped(t *testing.T) {
	e := sampleEvent()
	row := e.ToFlatRow()

	if len(row.Unmapped) != len(e.Unmapped) {
		t.Fatalf("unmapped field count mismatch: flat=%d event=%d", len(row.Unmapped), len(e.Unmapped))
	}
	for k, v := range e.Unmapped {
		if row.Unmapped[k] != v {
			t.Errorf("unmapped[%q] = %q, want %q", k, row.Unmapped[k], v)
		}
	}
	if row.SrcIP != e.Src.IP || row.DstPort != int64(e.Dst.Port) {
		t.Errorf("flat row did not project src/dst correctly: %+v", row)
	}
	if row.EnrichSrcIsInternal != false || row.EnrichIOCMatch != true {
		t.Errorf("flat row did not project enrich booleans correctly: %+v", row)
	}
}

func TestNewUUIDv7IsWellFormedAndSortable(t *testing.T) {
	ids := make([]string, 1000)
	for i := range ids {
		ids[i] = NewUUIDv7()
	}
	for i, id := range ids {
		if len(id) != 36 {
			t.Fatalf("id %d wrong length: %q", i, id)
		}
		parts := strings.Split(id, "-")
		if len(parts) != 5 {
			t.Fatalf("id %d wrong shape: %q", i, id)
		}
		if parts[2][0] != '7' {
			t.Fatalf("id %d not version 7: %q", i, id)
		}
	}
	for i := 1; i < len(ids); i++ {
		if ids[i] < ids[i-1] {
			t.Fatalf("uuids not monotonically non-decreasing at index %d: %q < %q", i, ids[i], ids[i-1])
		}
	}
}
