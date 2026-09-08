package vault

import (
	"container/list"
	"sync"
)

// segmentCache is a bounded, thread-safe LRU cache of decompressed segment
// bytes, keyed by segment ID. Read() populates and consults it: without
// this, reading N events out of the same segment cost N redundant
// full-segment object-store fetches and N redundant zstd decompressions —
// the actual bottleneck behind cmd/ulpf-processor's per-event consume loop
// running orders of magnitude slower than the collector's write throughput,
// since every event in a segment re-downloaded and re-decompressed the
// whole segment just to read its own few bytes out of it. Bounded by total
// decompressed bytes (not entry count) since segment size varies with
// Config.MaxSegmentBytes.
type segmentCache struct {
	mu       sync.Mutex
	maxBytes int64
	curBytes int64
	ll       *list.List // front = most recently used
	items    map[string]*list.Element
}

type cacheEntry struct {
	key   string
	bytes []byte
}

func newSegmentCache(maxBytes int64) *segmentCache {
	if maxBytes <= 0 {
		maxBytes = 256 << 20 // 256MB — a handful of full segments at the default 64MB seal threshold
	}
	return &segmentCache{maxBytes: maxBytes, ll: list.New(), items: make(map[string]*list.Element)}
}

func (c *segmentCache) get(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[key]
	if !ok {
		return nil, false
	}
	c.ll.MoveToFront(el)
	return el.Value.(*cacheEntry).bytes, true
}

func (c *segmentCache) put(key string, data []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[key]; ok {
		c.ll.MoveToFront(el)
		old := el.Value.(*cacheEntry)
		c.curBytes += int64(len(data)) - int64(len(old.bytes))
		old.bytes = data
		return
	}
	el := c.ll.PushFront(&cacheEntry{key: key, bytes: data})
	c.items[key] = el
	c.curBytes += int64(len(data))
	for c.curBytes > c.maxBytes && c.ll.Len() > 1 {
		back := c.ll.Back()
		if back == nil {
			break
		}
		entry := back.Value.(*cacheEntry)
		c.curBytes -= int64(len(entry.bytes))
		delete(c.items, entry.key)
		c.ll.Remove(back)
	}
}
