package parse

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"gopkg.in/yaml.v3"

	"github.com/ulpf/ulpf/internal/parse/dsl"
	"github.com/ulpf/ulpf/internal/parse/ops"
)

// Registry holds every loaded parser's current Plan behind an atomic
// pointer, keyed by parser id. Publishing a new version swaps the pointer
// atomically — in-flight Run calls on the old Plan finish on that old Plan,
// never torn mid-pipeline, and the next Run picks up the new one with zero
// downtime and zero lock contention on the hot path.
type Registry struct {
	mu    sync.RWMutex
	plans map[string]*atomic.Pointer[Plan]
	deps  ops.Deps
}

func NewRegistry(deps ops.Deps) *Registry {
	return &Registry{plans: make(map[string]*atomic.Pointer[Plan]), deps: deps}
}

// Publish compiles p and atomically installs it as the current Plan for
// p.Metadata.ID. Returns the compiled Plan (or a compile error, in which
// case the previous Plan for this id, if any, is left untouched — a bad
// publish never takes down a working parser).
func (r *Registry) Publish(p dsl.Parser) (*Plan, error) {
	plan, err := Compile(p, r.deps)
	if err != nil {
		return nil, err
	}

	r.mu.Lock()
	ptr, ok := r.plans[p.Metadata.ID]
	if !ok {
		ptr = &atomic.Pointer[Plan]{}
		r.plans[p.Metadata.ID] = ptr
	}
	r.mu.Unlock()

	ptr.Store(plan)
	return plan, nil
}

// Get returns the current Plan for id, if loaded.
func (r *Registry) Get(id string) (*Plan, bool) {
	r.mu.RLock()
	ptr, ok := r.plans[id]
	r.mu.RUnlock()
	if !ok {
		return nil, false
	}
	p := ptr.Load()
	return p, p != nil
}

// All returns a snapshot of every currently loaded Plan, sorted by
// descending Priority (used by internal/identify's signature-match tier).
func (r *Registry) All() []*Plan {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Plan, 0, len(r.plans))
	for _, ptr := range r.plans {
		if p := ptr.Load(); p != nil {
			out = append(out, p)
		}
	}
	sortPlansByPriorityDesc(out)
	return out
}

func sortPlansByPriorityDesc(plans []*Plan) {
	for i := 1; i < len(plans); i++ {
		for j := i; j > 0 && plans[j].Priority > plans[j-1].Priority; j-- {
			plans[j], plans[j-1] = plans[j-1], plans[j]
		}
	}
}

// LoadDir walks dir for *.yaml parser artifacts (skipping *.mapping.yaml,
// which belongs to Phase 4's normalize stage, and *.golden.yaml fixtures)
// and publishes each one. Returns the list of parser ids loaded.
func (r *Registry) LoadDir(dir string) ([]string, error) {
	var loaded []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !isParserArtifact(path) {
			return nil
		}
		p, loadErr := LoadParserFile(path)
		if loadErr != nil {
			return fmt.Errorf("load %s: %w", path, loadErr)
		}
		if _, pubErr := r.Publish(p); pubErr != nil {
			return fmt.Errorf("publish %s: %w", path, pubErr)
		}
		loaded = append(loaded, p.Metadata.ID)
		return nil
	})
	return loaded, err
}

func isParserArtifact(path string) bool {
	if !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml") {
		return false
	}
	base := filepath.Base(path)
	return !strings.Contains(base, ".mapping.") && !strings.Contains(base, ".golden.")
}

// LoadParserFile reads and YAML-decodes one Parser artifact from disk.
func LoadParserFile(path string) (dsl.Parser, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return dsl.Parser{}, err
	}
	var p dsl.Parser
	if err := yaml.Unmarshal(b, &p); err != nil {
		return dsl.Parser{}, fmt.Errorf("yaml: %w", err)
	}
	if p.Metadata.ID == "" {
		return dsl.Parser{}, fmt.Errorf("metadata.id is required")
	}
	return p, nil
}
