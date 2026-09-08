package enrich

import (
	"fmt"

	"github.com/ulpf/ulpf/internal/schema"
)

type identityRecord struct {
	Department, Manager, EmployeeID string
}

// Identity enriches actor.user/src.user against a synthetic IAM directory
// (enrichment/identities.csv).
type Identity struct {
	byUsername map[string]identityRecord
}

func NewIdentity(csvPath string) (*Identity, error) {
	rows, err := readCSV(csvPath)
	if err != nil {
		return nil, err
	}
	m := make(map[string]identityRecord, len(rows))
	for i, row := range rows {
		if len(row) < 4 {
			return nil, fmt.Errorf("%s: row %d: expected 4 columns, got %d", csvPath, i, len(row))
		}
		m[row[0]] = identityRecord{Department: row[1], Manager: row[2], EmployeeID: row[3]}
	}
	return &Identity{byUsername: m}, nil
}

func (id *Identity) Name() string { return "identity" }

func (id *Identity) Enrich(e *schema.Event) {
	user := e.Actor.User
	if user == "" {
		user = e.Src.User
	}
	if user == "" {
		return
	}
	if _, ok := id.byUsername[user]; ok {
		if e.Actor.User == "" {
			e.Actor.User = user
		}
	}
}

func (id *Identity) Lookup(username string) (department, manager, employeeID string, ok bool) {
	rec, found := id.byUsername[username]
	return rec.Department, rec.Manager, rec.EmployeeID, found
}
