package schema

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// uuid7State serializes UUIDv7 generation so two events created in the same
// nanosecond still sort correctly relative to each other (the monotonic
// counter in the low bits of the random section is not required by RFC 9562,
// but ordering under high event rates is exactly what we need it for here).
var uuid7State struct {
	mu       sync.Mutex
	lastMs   int64
	sameMsCt uint16
}

// NewUUIDv7 returns a time-sortable UUIDv7 string (RFC 9562), using the
// current UTC time in milliseconds for the timestamp field. Safe for
// concurrent use.
func NewUUIDv7() string {
	var b [16]byte

	nowMs := time.Now().UnixMilli()

	uuid7State.mu.Lock()
	if nowMs == uuid7State.lastMs {
		uuid7State.sameMsCt++
	} else {
		uuid7State.lastMs = nowMs
		uuid7State.sameMsCt = 0
	}
	counter := uuid7State.sameMsCt
	uuid7State.mu.Unlock()

	b[0] = byte(nowMs >> 40)
	b[1] = byte(nowMs >> 32)
	b[2] = byte(nowMs >> 24)
	b[3] = byte(nowMs >> 16)
	b[4] = byte(nowMs >> 8)
	b[5] = byte(nowMs)

	if _, err := rand.Read(b[6:]); err != nil {
		panic(fmt.Sprintf("schema: crypto/rand unavailable: %v", err))
	}

	// Fold the monotonic counter into the first two random bytes so events
	// within the same millisecond still order correctly.
	b[6] = byte(counter >> 8)
	b[7] = byte(counter)

	// Version 7 in the high nibble of byte 6.
	b[6] = (b[6] & 0x0F) | 0x70
	// RFC 9562 variant (10xxxxxx) in byte 8.
	b[8] = (b[8] & 0x3F) | 0x80

	return formatUUID(b)
}

func formatUUID(b [16]byte) string {
	buf := make([]byte, 36)
	hex.Encode(buf[0:8], b[0:4])
	buf[8] = '-'
	hex.Encode(buf[9:13], b[4:6])
	buf[13] = '-'
	hex.Encode(buf[14:18], b[6:8])
	buf[18] = '-'
	hex.Encode(buf[19:23], b[8:10])
	buf[23] = '-'
	hex.Encode(buf[24:36], b[10:16])
	return string(buf)
}
