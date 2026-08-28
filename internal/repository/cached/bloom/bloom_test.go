package bloom_test

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/omar-shahieen/url-shortner/internal/repository/cached/bloom"
)

// TestNoFalseNegatives verifies that every added item is always reported as
// present (the core guarantee of a bloom filter).
func TestNoFalseNegatives(t *testing.T) {
	f := bloom.New(1000, 0.01)
	items := make([]string, 500)
	for i := range items {
		items[i] = fmt.Sprintf("item-%d", i)
		f.Add(items[i])
	}
	for _, item := range items {
		if !f.Test(item) {
			t.Errorf("false negative for %q — bloom filter must never miss an added item", item)
		}
	}
}

// TestAbsentItemsReturnFalse verifies that a filter with no items returns
// false for all queries.
func TestAbsentItemsReturnFalse(t *testing.T) {
	f := bloom.New(1000, 0.01)
	for i := 0; i < 100; i++ {
		if f.Test(fmt.Sprintf("not-added-%d", i)) {
			// A false positive is theoretically possible but with an empty
			// filter it should never happen.
			t.Errorf("unexpected positive result from empty filter for not-added-%d", i)
		}
	}
}

// TestFalsePositiveRateApproximatelyMet populates the filter to its expected
// capacity and then checks 10 000 random strings that were never added.
// The measured FP rate must stay below 5× the target (1 %) to have a
// comfortable margin while still catching badly broken implementations.
func TestFalsePositiveRateApproximatelyMet(t *testing.T) {
	const (
		n      = 10_000 // expected items
		target = 0.01   // 1 % FP rate
		margin = 5.0    // allow up to 5× the target
		probes = 10_000 // random non-member probes
	)

	f := bloom.New(n, target)
	rng := rand.New(rand.NewSource(42))

	// Add n items using a distinct prefix so probe strings won't collide.
	added := make(map[string]struct{}, n)
	for i := 0; i < n; i++ {
		s := fmt.Sprintf("member-%d", i)
		f.Add(s)
		added[s] = struct{}{}
	}

	// Probe with random hex strings — very unlikely to collide with members.
	fps := 0
	for i := 0; i < probes; i++ {
		probe := fmt.Sprintf("probe-%016x", rng.Uint64())
		if _, isMember := added[probe]; isMember {
			continue // skip actual members
		}
		if f.Test(probe) {
			fps++
		}
	}

	fpRate := float64(fps) / float64(probes)
	if fpRate > target*margin {
		t.Errorf("false-positive rate %.4f exceeds %.4f (target %.4f × margin %.1f)",
			fpRate, target*margin, target, margin)
	}
	t.Logf("false-positive rate: %.4f (target: %.4f)", fpRate, target)
}

// TestAddThenTest is a simple smoke test ensuring basic Add/Test semantics.
func TestAddThenTest(t *testing.T) {
	f := bloom.New(100, 0.01)
	f.Add("hello")
	f.Add("world")

	if !f.Test("hello") {
		t.Error("Test(hello) = false, want true")
	}
	if !f.Test("world") {
		t.Error("Test(world) = false, want true")
	}
}
