package enrich

import (
	"fmt"

	"github.com/ulpf/ulpf/internal/schema"
)

type mitreRecord struct {
	Tactic, TechniqueID, TechniqueName string
}

// MITRE maps threat.signature_id to a MITRE ATT&CK tactic/technique from
// enrichment/mitre-map.csv. A no-op for events with no signature_id (our 3
// perimeter-traffic parsers don't emit one; an IDS/IPS-class parser like
// the Phase 5+ Suricata pack would).
type MITRE struct {
	bySignature map[string]mitreRecord
}

func NewMITRE(csvPath string) (*MITRE, error) {
	rows, err := readCSV(csvPath)
	if err != nil {
		return nil, err
	}
	m := make(map[string]mitreRecord, len(rows))
	for i, row := range rows {
		if len(row) < 4 {
			return nil, fmt.Errorf("%s: row %d: expected 4 columns, got %d", csvPath, i, len(row))
		}
		m[row[0]] = mitreRecord{Tactic: row[1], TechniqueID: row[2], TechniqueName: row[3]}
	}
	return &MITRE{bySignature: m}, nil
}

func (mt *MITRE) Name() string { return "mitre" }

func (mt *MITRE) Enrich(e *schema.Event) {
	if e.Threat.SignatureID == "" {
		return
	}
	rec, ok := mt.bySignature[e.Threat.SignatureID]
	if !ok {
		return
	}
	e.Threat.MitreTactic = rec.Tactic
	e.Threat.MitreTechnique = rec.TechniqueID
}
