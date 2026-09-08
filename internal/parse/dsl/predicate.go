package dsl

import (
	"fmt"
	"regexp"
	"strings"
)

// Compile turns a Predicate into a fast string test, shared by
// internal/identify's signature matching, the parse-engine's `match`
// evaluation, and the `conditional` operator — one implementation so all
// three agree on what "contains"/"prefix"/"regex" mean.
func (p Predicate) Compile() (func(string) bool, error) {
	switch {
	case p.Contains != "":
		needle := p.Contains
		return func(s string) bool { return strings.Contains(s, needle) }, nil
	case p.Prefix != "":
		prefix := p.Prefix
		return func(s string) bool { return strings.HasPrefix(s, prefix) }, nil
	case p.Regex != "":
		re, err := regexp.Compile(p.Regex)
		if err != nil {
			return nil, fmt.Errorf("predicate: compile regex %q: %w", p.Regex, err)
		}
		return re.MatchString, nil
	default:
		return nil, fmt.Errorf("predicate: empty (need contains/prefix/regex)")
	}
}
