package ops

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/logkrama/logkrama/internal/parse/dsl"
	"github.com/logkrama/logkrama/internal/parse/fields"
)

func init() {
	register("regex", buildRegex)
	register("grok", buildGrok)
}

// buildRegex compiles spec.Pattern once (Go's regexp = RE2, so this is
// always linear-time — no catastrophic backtracking is possible, per
// CLAUDE.md's "no backtracking engine" rule) and, on each event, sets one
// field per named capture group.
func buildRegex(spec dsl.Operator, _ Deps) (Op, error) {
	re, err := regexp.Compile(spec.Pattern)
	if err != nil {
		return nil, fmt.Errorf("regex: compile %q: %w", spec.Pattern, err)
	}
	if err := rejectUnsafeRegex(spec.Pattern); err != nil {
		return nil, err
	}
	names := re.SubexpNames()
	srcField := spec.Field

	return func(f *fields.Fields, raw []byte) error {
		s := string(raw)
		if srcField != "" {
			s = f.GetString(srcField)
		}
		m := re.FindStringSubmatch(s)
		if m == nil {
			return fmt.Errorf("regex: pattern did not match")
		}
		for i, name := range names {
			if name == "" {
				continue
			}
			f.Set(name, m[i])
		}
		return nil
	}, nil
}

// grokPatterns is a small built-in library of named sub-patterns, enough to
// cover common vendor log shapes without pulling in an external grok
// library (keeps every dependency license-clean per CLAUDE.md).
var grokPatterns = map[string]string{
	"IP":         `\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}`,
	"IPORHOST":   `[a-zA-Z0-9_.:-]+`,
	"INT":        `-?\d+`,
	"NUMBER":     `-?\d+(?:\.\d+)?`,
	"WORD":       `\w+`,
	"NOTSPACE":   `\S+`,
	"GREEDYDATA": `.*`,
	"DATA":       `.*?`,
	"HOSTNAME":   `[a-zA-Z0-9.-]+`,
	"TIMESTAMP":  `[A-Za-z]{3}\s+\d{1,2}\s\d{2}:\d{2}:\d{2}`,
}

var grokRefPattern = regexp.MustCompile(`%\{([A-Za-z0-9_]+)(?::([A-Za-z0-9_]+))?\}`)

// buildGrok expands %{PATTERN:field} references against grokPatterns into a
// plain Go regexp (named capture groups), then reuses the regex engine —
// grok is syntactic sugar over RE2, not a separate matcher.
func buildGrok(spec dsl.Operator, deps Deps) (Op, error) {
	expanded, err := expandGrok(spec.Pattern)
	if err != nil {
		return nil, err
	}
	regexSpec := spec
	regexSpec.Pattern = expanded
	return buildRegex(regexSpec, deps)
}

func expandGrok(pattern string) (string, error) {
	var sb strings.Builder
	last := 0
	for _, m := range grokRefPattern.FindAllStringSubmatchIndex(pattern, -1) {
		sb.WriteString(pattern[last:m[0]])
		name := pattern[m[2]:m[3]]
		def, ok := grokPatterns[name]
		if !ok {
			return "", fmt.Errorf("grok: unknown pattern %%{%s}", name)
		}
		if m[4] >= 0 {
			field := pattern[m[4]:m[5]]
			sb.WriteString(fmt.Sprintf("(?P<%s>%s)", field, def))
		} else {
			sb.WriteString("(?:" + def + ")")
		}
		last = m[1]
	}
	sb.WriteString(pattern[last:])
	return sb.String(), nil
}

// rejectUnsafeRegex is a defense-in-depth lint check for
// `logkramactl parser lint`: Go's RE2 engine can't actually catastrophically
// backtrack, but nested quantifiers like (a+)+ are still a readability/
// intent smell worth flagging before a parser ships.
func rejectUnsafeRegex(pattern string) error {
	nested := regexp.MustCompile(`\([^()]*[+*]\)[+*]`)
	if nested.MatchString(pattern) {
		return fmt.Errorf("regex: pattern %q looks like a nested quantifier (a+)+ — rejected even though RE2 can't backtrack, because it almost always signals a mistake", pattern)
	}
	return nil
}
