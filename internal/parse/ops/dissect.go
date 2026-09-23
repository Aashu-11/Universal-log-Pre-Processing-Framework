package ops

import (
	"fmt"
	"strings"

	"github.com/logkrama/logkrama/internal/parse/dsl"
	"github.com/logkrama/logkrama/internal/parse/fields"
)

func init() { register("dissect", buildDissect) }

// dissectToken is one piece of a compiled dissect pattern: either a literal
// delimiter to match verbatim, or a field name to capture up to the next
// literal (or end of string).
type dissectToken struct {
	literal string // set when this token is a literal to match
	field   string // set when this token is a %{field} capture
}

// buildDissect compiles a %{field} pattern into a linear delimiter walk —
// no regex, no backtracking, just literal-string matching between capture
// points. This is the fast path CLAUDE.md calls for: dissect must be a
// linear delimiter walk, not regex.
func buildDissect(spec dsl.Operator, _ Deps) (Op, error) {
	tokens, err := compileDissectPattern(spec.Pattern)
	if err != nil {
		return nil, err
	}
	srcField := spec.Field

	return func(f *fields.Fields, raw []byte) error {
		s := string(raw)
		if srcField != "" {
			s = f.GetString(srcField)
		}
		return applyDissect(f, tokens, s)
	}, nil
}

func compileDissectPattern(pattern string) ([]dissectToken, error) {
	var tokens []dissectToken
	i := 0
	for i < len(pattern) {
		open := strings.IndexByte(pattern[i:], '%')
		if open < 0 || i+open+1 >= len(pattern) || pattern[i+open+1] != '{' {
			tokens = append(tokens, dissectToken{literal: pattern[i:]})
			break
		}
		open += i
		if open > i {
			tokens = append(tokens, dissectToken{literal: pattern[i:open]})
		}
		close := strings.IndexByte(pattern[open:], '}')
		if close < 0 {
			return nil, fmt.Errorf("dissect: unterminated %%{ in pattern %q", pattern)
		}
		close += open
		fieldName := pattern[open+2 : close]
		tokens = append(tokens, dissectToken{field: fieldName})
		i = close + 1
	}
	return tokens, nil
}

func applyDissect(f *fields.Fields, tokens []dissectToken, s string) error {
	pos := 0
	for ti, tok := range tokens {
		if tok.literal != "" {
			if !strings.HasPrefix(s[pos:], tok.literal) {
				idx := strings.Index(s[pos:], tok.literal)
				if idx < 0 {
					return fmt.Errorf("dissect: literal %q not found after position %d", tok.literal, pos)
				}
				pos += idx
			}
			pos += len(tok.literal)
			continue
		}
		if tok.field == "" || tok.field == "_" {
			continue // anonymous capture, discarded
		}

		end := len(s)
		if ti+1 < len(tokens) && tokens[ti+1].literal != "" {
			nextLit := tokens[ti+1].literal
			if idx := strings.Index(s[pos:], nextLit); idx >= 0 {
				end = pos + idx
			}
		}
		if pos > len(s) {
			pos = len(s)
		}
		f.Set(tok.field, s[pos:end])
		pos = end
	}
	return nil
}
