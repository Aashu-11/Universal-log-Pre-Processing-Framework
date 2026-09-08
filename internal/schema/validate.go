package schema

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v5"

	uesschema "github.com/ulpf/ulpf/schema"
)

var (
	compileOnce sync.Once
	compiled    *jsonschema.Schema
	compileErr  error
)

func uesValidator() (*jsonschema.Schema, error) {
	compileOnce.Do(func() {
		c := jsonschema.NewCompiler()
		c.Draft = jsonschema.Draft2020
		if err := c.AddResource("ues-1.0.json", bytes.NewReader(uesschema.UES10JSON)); err != nil {
			compileErr = fmt.Errorf("schema: add UES resource: %w", err)
			return
		}
		s, err := c.Compile("ues-1.0.json")
		if err != nil {
			compileErr = fmt.Errorf("schema: compile UES schema: %w", err)
			return
		}
		compiled = s
	})
	return compiled, compileErr
}

// Validate marshals e to JSON and checks it against the embedded UES 1.0
// JSON Schema (schema/ues-1.0.json). Returns a descriptive error listing
// every violation on failure.
func (e *Event) Validate() error {
	v, err := uesValidator()
	if err != nil {
		return err
	}

	b, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("schema: marshal event: %w", err)
	}

	var doc any
	if err := json.Unmarshal(b, &doc); err != nil {
		return fmt.Errorf("schema: unmarshal event for validation: %w", err)
	}

	if err := v.Validate(doc); err != nil {
		return fmt.Errorf("schema: UES validation failed: %w", err)
	}
	return nil
}
