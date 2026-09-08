package normalize

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadMappingFile reads and YAML-decodes one Mapping artifact from disk.
func LoadMappingFile(path string) (Mapping, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Mapping{}, err
	}
	var m Mapping
	if err := yaml.Unmarshal(b, &m); err != nil {
		return Mapping{}, fmt.Errorf("yaml: %w", err)
	}
	if m.Metadata.ParserID == "" {
		return Mapping{}, fmt.Errorf("metadata.parser_id is required")
	}
	return m, nil
}

// LoadMappingsDir walks dir for *.mapping.yaml artifacts and returns them
// keyed by parser id, so a processor can look up "which mapping applies to
// the parser that just ran" in O(1).
func LoadMappingsDir(dir string) (map[string]Mapping, error) {
	out := make(map[string]Mapping)
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".mapping.yaml") {
			return nil
		}
		m, err := LoadMappingFile(path)
		if err != nil {
			return fmt.Errorf("load %s: %w", path, err)
		}
		out[m.Metadata.ParserID] = m
		return nil
	})
	return out, err
}
