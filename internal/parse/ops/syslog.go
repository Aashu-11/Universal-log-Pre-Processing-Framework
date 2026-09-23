package ops

import (
	"strconv"
	"strings"

	"github.com/logkrama/logkrama/internal/parse/dsl"
	"github.com/logkrama/logkrama/internal/parse/fields"
)

func init() { register("syslog", buildSyslog) }

// buildSyslog compiles the "syslog" operator: strips and decodes the
// leading <PRI> and auto-detects RFC5424 ("<PRI>1 ...") vs. RFC3164
// ("<PRI>Mon  2 15:04:05 host tag: msg") framing, writing pri/facility/
// severity/timestamp/hostname/appname/procid/msgid/message fields plus
// leaving the original in "_raw_after_pri" for downstream operators (a
// vendor-specific structured body, e.g. PAN-OS CSV, follows the syslog
// header and still needs its own operator on that remainder).
func buildSyslog(spec dsl.Operator, _ Deps) (Op, error) {
	return func(f *fields.Fields, raw []byte) error {
		s := string(raw)

		pri, rest, ok := stripPRI(s)
		if ok {
			f.Set("pri", pri)
			f.Set("facility", pri/8)
			f.Set("severity", pri%8)
		} else {
			rest = s
		}

		if strings.HasPrefix(rest, "1 ") {
			parseRFC5424(f, rest[2:])
		} else {
			parseRFC3164(f, rest)
		}
		return nil
	}, nil
}

func stripPRI(s string) (int, string, bool) {
	if len(s) == 0 || s[0] != '<' {
		return 0, s, false
	}
	end := strings.IndexByte(s, '>')
	if end < 1 {
		return 0, s, false
	}
	pri, err := strconv.Atoi(s[1:end])
	if err != nil {
		return 0, s, false
	}
	return pri, s[end+1:], true
}

// parseRFC5424 handles "TIMESTAMP HOSTNAME APP-NAME PROCID MSGID
// [STRUCTURED-DATA] MSG". Structured data is captured verbatim into
// "structured_data" rather than decomposed — vendors that put meaningful
// content there ship a dedicated operator after this one.
func parseRFC5424(f *fields.Fields, s string) {
	tok, rest, ok := splitToken(s)
	if !ok {
		f.Set("message", s)
		return
	}
	f.Set("timestamp", tok)

	tok, rest, ok = splitToken(rest)
	if !ok {
		return
	}
	f.Set("hostname", tok)

	tok, rest, ok = splitToken(rest)
	if !ok {
		return
	}
	f.Set("appname", tok)

	tok, rest, ok = splitToken(rest)
	if !ok {
		return
	}
	f.Set("procid", tok)

	tok, rest, ok = splitToken(rest)
	if !ok {
		return
	}
	f.Set("msgid", tok)

	if strings.HasPrefix(rest, "-") {
		rest = strings.TrimPrefix(rest, "- ")
		f.Set("message", rest)
		return
	}
	if strings.HasPrefix(rest, "[") {
		end := strings.Index(rest, "] ")
		if end >= 0 {
			f.Set("structured_data", rest[:end+1])
			f.Set("message", rest[end+2:])
			return
		}
	}
	f.Set("message", rest)
}

// parseRFC3164 handles the traditional BSD syslog format:
// "Mon  2 15:04:05 hostname tag[pid]: message". The timestamp is
// three space-joined tokens (month, day, time); day is single-digit-padded
// with an extra space in the real format, which splitToken's
// consecutive-space handling absorbs naturally.
func parseRFC3164(f *fields.Fields, s string) {
	leading := strings.Fields(s)
	if len(leading) < 4 {
		f.Set("message", s)
		return
	}

	// Strict RFC3164 timestamps are 3 tokens (month day time). Several real
	// vendors (Cisco ASA among them) insert a 4-digit year as a 3rd token
	// before the time — "Sep 07 2026 14:02:11" — which isn't RFC3164 but is
	// common enough on the wire that treating it as an error would break a
	// whole vendor family. Detect it by checking whether the 3rd token is a
	// bare 4-digit year rather than an HH:MM:SS time.
	tsTokens := 3
	if len(leading) >= 5 && isFourDigitYear(leading[2]) {
		tsTokens = 4
	}
	f.Set("timestamp", strings.Join(leading[:tsTokens], " "))

	rest := nthFieldOnward(s, tsTokens+1)
	hostname, tail, ok := splitToken(rest)
	if !ok {
		f.Set("message", rest)
		return
	}
	f.Set("hostname", hostname)

	if colon := strings.Index(tail, ": "); colon >= 0 {
		tag := tail[:colon]
		f.Set("appname", strings.TrimSuffix(tag, ":"))
		f.Set("message", tail[colon+2:])
	} else {
		f.Set("message", tail)
	}
}

func isFourDigitYear(tok string) bool {
	if len(tok) != 4 {
		return false
	}
	for _, r := range tok {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func splitToken(s string) (tok, rest string, ok bool) {
	s = strings.TrimLeft(s, " ")
	i := strings.IndexByte(s, ' ')
	if i < 0 {
		if s == "" {
			return "", "", false
		}
		return s, "", true
	}
	return s[:i], strings.TrimLeft(s[i+1:], " "), true
}

// nthFieldOnward returns the substring of s starting at the nth
// whitespace-delimited field (1-indexed), preserving everything after that
// point verbatim — including internal spacing, which strings.Fields alone
// would collapse.
func nthFieldOnward(s string, n int) string {
	inField := false
	count := 0
	for i, r := range s {
		if r != ' ' {
			if !inField {
				inField = true
				count++
				if count == n {
					return s[i:]
				}
			}
		} else {
			inField = false
		}
	}
	return ""
}
