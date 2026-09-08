// Package framing handles multiline joining, truncation detection and
// malformed-UTF-8-safe byte handling before events reach the vault.
package framing

import "regexp"

// Config controls multiline joining and truncation.
type Config struct {
	// MultilineStart, if set, is a regex matched against each incoming
	// line: a match starts a new event, and every line that does NOT match
	// is appended (with a newline) to the event currently being
	// accumulated. Nil/empty means every line is its own event.
	MultilineStart string
	// MaxEventBytes caps a single (possibly joined) event's size. Bytes
	// beyond this are dropped and Truncated is set — never silently, the
	// flag always travels with the event through raw.truncated in the
	// eventual UES record.
	MaxEventBytes int
}

// Joiner accumulates lines into events per Config. It holds no assumptions
// about UTF-8 validity of the input — every byte given to Feed is either
// entirely retained (up to MaxEventBytes) or entirely dropped past that
// limit; nothing is ever transcoded or replaced, so a malformed multi-byte
// sequence at a line boundary can't corrupt other bytes around it.
type Joiner struct {
	cfg      Config
	start    *regexp.Regexp
	buf      []byte
	buffered bool
}

// New builds a Joiner. An invalid MultilineStart regex is treated as "no
// pattern" (every line is its own event) rather than failing construction —
// callers doing config validation should compile the pattern themselves
// first if they want to surface a bad regex as an error.
func New(cfg Config) *Joiner {
	j := &Joiner{cfg: cfg}
	if cfg.MultilineStart != "" {
		if re, err := regexp.Compile(cfg.MultilineStart); err == nil {
			j.start = re
		}
	}
	if j.cfg.MaxEventBytes <= 0 {
		j.cfg.MaxEventBytes = 1 << 20 // 1MB default
	}
	return j
}

// Result is one completed event.
type Result struct {
	Payload   []byte
	Truncated bool
}

// Feed processes one line (without its trailing newline). It returns a
// completed Result when this line's arrival closes out the *previous*
// event (either joining is disabled, so every line is its own event, or
// this line matches MultilineStart and therefore begins a new one) — or
// ok=false if the line was absorbed into the still-open event.
func (j *Joiner) Feed(line []byte) (res Result, ok bool) {
	if j.start == nil {
		return j.finish(line), true
	}

	if !j.buffered {
		j.appendToBuffer(line)
		return Result{}, false
	}

	if j.start.Match(line) {
		res = j.flushBuffer()
		j.appendToBuffer(line)
		return res, true
	}

	j.appendToBuffer(line)
	return Result{}, false
}

// Flush returns any event still accumulating (e.g. at connection close).
func (j *Joiner) Flush() (Result, bool) {
	if !j.buffered {
		return Result{}, false
	}
	return j.flushBuffer(), true
}

func (j *Joiner) finish(line []byte) Result {
	return truncate(append([]byte(nil), line...), j.cfg.MaxEventBytes)
}

func (j *Joiner) appendToBuffer(line []byte) {
	if j.buffered {
		j.buf = append(j.buf, '\n')
		j.buf = append(j.buf, line...)
		return
	}
	j.buf = append([]byte(nil), line...)
	j.buffered = true
}

func (j *Joiner) flushBuffer() Result {
	res := truncate(j.buf, j.cfg.MaxEventBytes)
	j.buf = nil
	j.buffered = false
	return res
}

func truncate(b []byte, max int) Result {
	if len(b) <= max {
		return Result{Payload: b}
	}
	return Result{Payload: b[:max], Truncated: true}
}
