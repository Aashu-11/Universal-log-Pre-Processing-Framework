package normalize

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadDictionaries reads every *.yaml file in dir as a flat
// string->string value dictionary, keyed by filename without extension
// (e.g. config/dictionaries/action.yaml becomes dictionary "action"). Every
// dictionary should carry a "_default" entry — see config/dictionaries/ —
// but a missing one just means an unrecognized value passes through
// unchanged rather than being coerced to a sentinel.
func LoadDictionaries(dir string) (map[string]map[string]string, error) {
	out := make(map[string]map[string]string)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		var dict map[string]string
		if err := yaml.Unmarshal(b, &dict); err != nil {
			return nil, err
		}
		name := strings.TrimSuffix(entry.Name(), ".yaml")
		out[name] = dict
	}
	return out, nil
}
