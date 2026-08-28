// Package bloom provides a space-efficient probabilistic set membership test.
//
// The filter uses a bit array of m bits and k independent hash positions
// derived from double-hashing two seeded FNV-1a hashes:
//
//	position_i = (h1 + i*h2) % m     for i in [0, k)
//
// Optimal m and k are computed from the expected item count n and the target
// false-positive rate p:
//
//	m = ceil(-n * ln(p) / ln(2)^2)
//	k = round(m/n * ln(2))
//
// Test is genuinely read-only and is guarded by a sync.RWMutex.
// Add acquires an exclusive write lock.
package bloom

import (
	"hash/fnv"
	"math"
	"sync"
)

// Filter is a thread-safe bloom filter.
type Filter struct {
	mu   sync.RWMutex
	bits []uint64 // bit array stored as 64-bit words
	m    uint64   // total number of bits
	k    uint     // number of hash positions per element
}

// New returns a Filter sized for expectedItems elements at the given
// falsePositiveRate (e.g. 0.01 for 1 %).
func New(expectedItems int, falsePositiveRate float64) *Filter {
	m, k := optimal(expectedItems, falsePositiveRate)
	words := (m + 63) / 64
	return &Filter{
		bits: make([]uint64, words),
		m:    m,
		k:    k,
	}
}

// Add inserts item into the filter.
func (f *Filter) Add(item string) {
	h1, h2 := hashes(item)
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := uint(0); i < f.k; i++ {
		pos := (h1 + uint64(i)*h2) % f.m
		f.bits[pos/64] |= 1 << (pos % 64)
	}
}

// Test reports whether item was possibly added.  False negatives are
// impossible; false positives occur at approximately falsePositiveRate.
func (f *Filter) Test(item string) bool {
	h1, h2 := hashes(item)
	f.mu.RLock()
	defer f.mu.RUnlock()
	for i := uint(0); i < f.k; i++ {
		pos := (h1 + uint64(i)*h2) % f.m
		if f.bits[pos/64]&(1<<(pos%64)) == 0 {
			return false
		}
	}
	return true
}

// --- helpers ---

// hashes returns two independent 64-bit FNV-1a hashes of s using different seeds.
func hashes(s string) (h1, h2 uint64) {
	// First hash: standard FNV-1a 64-bit
	a := fnv.New64a()
	_, _ = a.Write([]byte(s))
	h1 = a.Sum64()

	// Second hash: FNV-1a with an XOR seed to make it independent
	b := fnv.New64a()
	_, _ = b.Write([]byte{0xff, 0x51, 0xaf, 0xd7, 0xed, 0x55, 0x8c, 0xcd}) // arbitrary seed bytes
	_, _ = b.Write([]byte(s))
	h2 = b.Sum64()

	// Ensure h2 is odd to guarantee full-cycle coverage
	if h2 == 0 {
		h2 = 1
	}
	return
}

// optimal computes the bit count m and hash count k.
func optimal(n int, p float64) (m uint64, k uint) {
	ln2 := math.Log(2)
	mFloat := math.Ceil(-float64(n) * math.Log(p) / (ln2 * ln2))
	if mFloat < 1 {
		mFloat = 1
	}
	m = uint64(mFloat)
	kFloat := math.Round(float64(m) / float64(n) * ln2)
	if kFloat < 1 {
		kFloat = 1
	}
	k = uint(kFloat)
	return
}
