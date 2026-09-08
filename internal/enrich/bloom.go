package enrich

import (
	"hash/fnv"
	"math"
)

// bloomFilter is a minimal, self-contained Bloom filter — no external
// dependency, since a fixed bit array plus double hashing is ~30 lines and
// exactly matches CLAUDE.md's "Bloom filter pre-check then exact map" spec
// for the IOC enricher. False positives fall through to the exact map
// (io.go), so correctness never depends on the filter's tuning; it only
// affects how often the negative fast-path is skipped.
type bloomFilter struct {
	bits []uint64
	k    int
	m    uint64
}

func newBloomFilter(expectedItems int, falsePositiveRate float64) *bloomFilter {
	m := optimalBits(expectedItems, falsePositiveRate)
	k := optimalHashes(expectedItems, m)
	if k < 1 {
		k = 1
	}
	return &bloomFilter{bits: make([]uint64, (m+63)/64), k: k, m: m}
}

func optimalBits(n int, p float64) uint64 {
	// m = -(n * ln(p)) / (ln(2)^2)
	m := -(float64(n) * math.Log(p)) / (math.Ln2 * math.Ln2)
	if m < 64 {
		m = 64
	}
	return uint64(m)
}

func optimalHashes(n int, m uint64) int {
	if n == 0 {
		return 1
	}
	// k = (m/n) * ln(2)
	k := (float64(m) / float64(n)) * math.Ln2
	return int(k + 0.5)
}

func (b *bloomFilter) add(key string) {
	h1, h2 := b.hashes(key)
	for i := 0; i < b.k; i++ {
		bit := (h1 + uint64(i)*h2) % b.m
		b.bits[bit/64] |= 1 << (bit % 64)
	}
}

func (b *bloomFilter) mightContain(key string) bool {
	h1, h2 := b.hashes(key)
	for i := 0; i < b.k; i++ {
		bit := (h1 + uint64(i)*h2) % b.m
		if b.bits[bit/64]&(1<<(bit%64)) == 0 {
			return false
		}
	}
	return true
}

func (b *bloomFilter) hashes(key string) (uint64, uint64) {
	h1 := fnv.New64a()
	h1.Write([]byte(key))
	sum1 := h1.Sum64()

	h2 := fnv.New64()
	h2.Write([]byte(key))
	sum2 := h2.Sum64() | 1 // ensure odd, so double-hashing covers all slots

	return sum1, sum2
}
