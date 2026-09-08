// Package fields defines the extracted-field container operators read from
// and write to. It exists as its own leaf package (rather than living in
// internal/parse or internal/parse/ops) purely to avoid an import cycle:
// both the compiler (internal/parse) and every operator
// (internal/parse/ops) need this type.
package fields

// Fields holds one event's in-progress extracted fields during parsing.
// It's a small ordered map, not a bare map[string]any sprinkled through the
// parse loop: insertion order is preserved (useful for deterministic
// unmapped-field iteration later) and lookups are O(1) via the index map.
// Values are string, int64, float64, or bool — whatever `convert` has
// coerced them to; everything starts as string from extraction.
type Fields struct {
	keys []string
	vals []any
	idx  map[string]int
}

// New returns an empty Fields sized for n expected fields.
func New(n int) *Fields {
	return &Fields{
		keys: make([]string, 0, n),
		vals: make([]any, 0, n),
		idx:  make(map[string]int, n),
	}
}

// Set inserts or overwrites the value for key.
func (f *Fields) Set(key string, val any) {
	if i, ok := f.idx[key]; ok {
		f.vals[i] = val
		return
	}
	f.idx[key] = len(f.keys)
	f.keys = append(f.keys, key)
	f.vals = append(f.vals, val)
}

// Get returns key's value and whether it was present.
func (f *Fields) Get(key string) (any, bool) {
	i, ok := f.idx[key]
	if !ok {
		return nil, false
	}
	return f.vals[i], true
}

// GetString returns key's value coerced to string ("" if absent).
func (f *Fields) GetString(key string) string {
	v, ok := f.Get(key)
	if !ok {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// Delete removes key, if present.
func (f *Fields) Delete(key string) {
	i, ok := f.idx[key]
	if !ok {
		return
	}
	delete(f.idx, key)
	f.keys = append(f.keys[:i], f.keys[i+1:]...)
	f.vals = append(f.vals[:i], f.vals[i+1:]...)
	for k := i; k < len(f.keys); k++ {
		f.idx[f.keys[k]] = k
	}
}

// Rename moves the value at from to to, deleting from. A no-op if from is
// absent.
func (f *Fields) Rename(from, to string) {
	v, ok := f.Get(from)
	if !ok {
		return
	}
	f.Delete(from)
	f.Set(to, v)
}

// Keys returns every field name, in insertion order. The returned slice
// must not be mutated by the caller.
func (f *Fields) Keys() []string {
	return f.keys
}

// Len returns the number of fields currently set.
func (f *Fields) Len() int {
	return len(f.keys)
}

// Each calls fn for every field in insertion order.
func (f *Fields) Each(fn func(key string, val any)) {
	for i, k := range f.keys {
		fn(k, f.vals[i])
	}
}
