package ops

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ulpf/ulpf/internal/parse/dsl"
	"github.com/ulpf/ulpf/internal/parse/fields"
)

func init() {
	register("date", buildDate)
	register("convert", buildConvert)
	register("rename", buildRename)
	register("copy", buildCopy)
	register("drop_field", buildDropField)
	register("gsub", buildGsub)
	register("split", buildSplit)
	register("conditional", buildConditional)
	register("lookup", buildLookup)
}

// buildDate tries each of spec.Formats (Go reference-time layouts) in order
// against spec.Field and stores the first successful parse, as UTC epoch
// nanoseconds, into spec.As. A layout with no year (common in RFC3164)
// parses to year 0000 — deliberately left for internal/normalize's
// timestamp handling to disambiguate via December/January rollover logic,
// not solved here, since the raw parse stage has no notion of "now".
func buildDate(spec dsl.Operator, _ Deps) (Op, error) {
	if len(spec.Formats) == 0 {
		return nil, fmt.Errorf("date: no formats given")
	}
	as := spec.As
	if as == "" {
		as = spec.Field + "_epoch_ns"
	}
	loc := time.UTC
	if spec.Timezone != "" {
		l, err := time.LoadLocation(spec.Timezone)
		if err != nil {
			return nil, fmt.Errorf("date: load timezone %q: %w", spec.Timezone, err)
		}
		loc = l
	}

	return func(f *fields.Fields, _ []byte) error {
		val := f.GetString(spec.Field)
		var lastErr error
		for _, layout := range spec.Formats {
			t, err := time.ParseInLocation(layout, val, loc)
			if err == nil {
				f.Set(as, t.UnixNano())
				return nil
			}
			lastErr = err
		}
		return fmt.Errorf("date: no format matched %q: %w", val, lastErr)
	}, nil
}

func buildConvert(spec dsl.Operator, _ Deps) (Op, error) {
	target := spec.Field
	as := spec.As
	if as == "" {
		as = target
	}

	return func(f *fields.Fields, _ []byte) error {
		val := f.GetString(target)
		switch spec.Type {
		case "int":
			n, err := strconv.ParseInt(strings.TrimSpace(val), 10, 64)
			if err != nil {
				return fmt.Errorf("convert: %q to int: %w", val, err)
			}
			f.Set(as, n)
		case "float":
			n, err := strconv.ParseFloat(strings.TrimSpace(val), 64)
			if err != nil {
				return fmt.Errorf("convert: %q to float: %w", val, err)
			}
			f.Set(as, n)
		case "bool":
			b, err := strconv.ParseBool(strings.TrimSpace(val))
			if err != nil {
				return fmt.Errorf("convert: %q to bool: %w", val, err)
			}
			f.Set(as, b)
		case "string", "":
			f.Set(as, val)
		default:
			return fmt.Errorf("convert: unknown type %q", spec.Type)
		}
		return nil
	}, nil
}

func buildRename(spec dsl.Operator, _ Deps) (Op, error) {
	from, to := spec.From, spec.To
	if from == "" {
		from = spec.Field
	}
	if to == "" {
		to = spec.As
	}
	if from == "" || to == "" {
		return nil, fmt.Errorf("rename: need from/to (or field/as)")
	}
	return func(f *fields.Fields, _ []byte) error {
		f.Rename(from, to)
		return nil
	}, nil
}

func buildCopy(spec dsl.Operator, _ Deps) (Op, error) {
	from, to := spec.From, spec.To
	if from == "" {
		from = spec.Field
	}
	if to == "" {
		to = spec.As
	}
	if from == "" || to == "" {
		return nil, fmt.Errorf("copy: need from/to (or field/as)")
	}
	return func(f *fields.Fields, _ []byte) error {
		if v, ok := f.Get(from); ok {
			f.Set(to, v)
		}
		return nil
	}, nil
}

func buildDropField(spec dsl.Operator, _ Deps) (Op, error) {
	if spec.Field == "" {
		return nil, fmt.Errorf("drop_field: field required")
	}
	return func(f *fields.Fields, _ []byte) error {
		f.Delete(spec.Field)
		return nil
	}, nil
}

func buildGsub(spec dsl.Operator, _ Deps) (Op, error) {
	re, err := regexp.Compile(spec.Match)
	if err != nil {
		return nil, fmt.Errorf("gsub: compile %q: %w", spec.Match, err)
	}
	as := spec.As
	if as == "" {
		as = spec.Field
	}
	return func(f *fields.Fields, _ []byte) error {
		val := f.GetString(spec.Field)
		f.Set(as, re.ReplaceAllString(val, spec.Replacement))
		return nil
	}, nil
}

func buildSplit(spec dsl.Operator, _ Deps) (Op, error) {
	if spec.Separator == "" {
		return nil, fmt.Errorf("split: separator required")
	}
	as := spec.As
	if as == "" {
		as = spec.Field
	}
	idx := 0
	if spec.Index != nil {
		idx = *spec.Index
	}
	return func(f *fields.Fields, _ []byte) error {
		parts := strings.Split(f.GetString(spec.Field), spec.Separator)
		if idx < 0 || idx >= len(parts) {
			return fmt.Errorf("split: index %d out of range (%d parts)", idx, len(parts))
		}
		f.Set(as, parts[idx])
		return nil
	}, nil
}

func buildLookup(spec dsl.Operator, deps Deps) (Op, error) {
	dict := deps.Dictionaries[spec.Dictionary]
	as := spec.As
	if as == "" {
		as = spec.Field
	}
	return func(f *fields.Fields, _ []byte) error {
		val := f.GetString(spec.Field)
		if mapped, ok := dict[val]; ok {
			f.Set(as, mapped)
			return nil
		}
		f.Set(as, spec.Default)
		return nil
	}, nil
}

// buildConditional evaluates spec.When against spec.Field's value (or the
// raw event bytes if Field is empty) and runs the compiled Then pipeline on
// match, Else otherwise. Sub-pipelines are compiled once here, at Build
// time, same as the top-level plan.
func buildConditional(spec dsl.Operator, deps Deps) (Op, error) {
	if spec.When == nil {
		return nil, fmt.Errorf("conditional: 'when' is required")
	}
	pred, err := spec.When.Compile()
	if err != nil {
		return nil, err
	}
	thenOps, err := compileSubPipeline(spec.Then, deps)
	if err != nil {
		return nil, fmt.Errorf("conditional: then: %w", err)
	}
	elseOps, err := compileSubPipeline(spec.Else, deps)
	if err != nil {
		return nil, fmt.Errorf("conditional: else: %w", err)
	}
	field := spec.Field

	return func(f *fields.Fields, raw []byte) error {
		s := string(raw)
		if field != "" {
			s = f.GetString(field)
		}
		branch := elseOps
		if pred(s) {
			branch = thenOps
		}
		for _, op := range branch {
			if err := op(f, raw); err != nil {
				return err
			}
		}
		return nil
	}, nil
}

func compileSubPipeline(specs []dsl.Operator, deps Deps) ([]Op, error) {
	out := make([]Op, 0, len(specs))
	for _, s := range specs {
		op, err := Build(s, deps)
		if err != nil {
			return nil, err
		}
		out = append(out, op)
	}
	return out, nil
}
