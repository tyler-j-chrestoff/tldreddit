package memory

import (
	"slices"
	"testing"
)

// reachable walks the record from the views a reader holds, the way a reader
// would: follow every Prev, and follow every Absorbed on a receipt. It returns
// the IDs found.
//
// This is the operational meaning of "permanently reachable" in D1. Content
// addressing makes an ID *retrievable* by anyone already holding the hash;
// only the edges make it *discoverable*. A bit nothing points at is a bit no
// reader can get to, whatever the store still has filed under its address.
//
// Views, plural, since votes arrived. A vote lives in its own view and nothing
// in the transcript points at one, so a reader holding both is the honest
// starting set — and passing only the transcript is the mutation that shows this
// can still fail.
func reachable(t *testing.T, s *Store, views ...View) map[string]bool {
	t.Helper()

	seen := map[string]bool{}
	var walk func(id string)
	walk = func(id string) {
		if seen[id] {
			return
		}
		b, ok := s.Get(id)
		if !ok {
			t.Fatalf("the record names %s, which the store does not hold", Short(id))
		}
		seen[id] = true

		for _, p := range b.Prev {
			walk(p)
		}
		if c, cold := b.Payload.(Compaction); cold {
			// Through the accessor, not the field. A reader outside this
			// package has exactly this much to navigate with, and the test is
			// only worth anything if it walks what they walk.
			for a := range c.Absorbed() {
				walk(a)
			}
		}
	}

	for _, v := range views {
		for _, id := range v {
			walk(id)
		}
	}
	return seen
}

// This is D1's canonical test. Every bit the store holds must be findable by
// walking out from what is on screen — not merely retrievable by someone who
// already knows its address.
//
// The failure it exists to catch is silent and cumulative: a fold whose edges
// do not name the fold beneath it strands that generation, the store goes on
// paying to hold it, and nothing ever reports it missing because nothing ever
// asks for it. Two hundred sends produces twenty-odd nested folds, so the
// second generation onward is where stranding would start showing.
func TestEveryStoredBitIsReachableFromTheView(t *testing.T) {
	// Mirrors the surface's discipline: fold once the hot band overflows,
	// keeping a readable tail. The exact numbers do not matter; nesting does.
	const coolAt, keepHot = 12, 6

	s := NewStore()
	var v View
	for i := range 200 {
		v, _ = v.Add(s, said(i, "tyler", "bit", v.Head()...))

		originals := 0
		for _, b := range v.Bits(s) {
			if hot(b) {
				originals++
			}
		}
		if originals > coolAt {
			v, _ = v.Fold(s, keepHot, Stay{})
		}
	}

	found := reachable(t, s, v)
	if len(found) == s.Len() {
		return
	}

	// Only to name the casualties. A Store has no enumeration on purpose —
	// nothing in the product walks the record except by following edges, which
	// is the whole point above — so the failure path reaches in rather than
	// growing an API that exists for a test.
	var orphans []string
	s.mu.RLock()
	for id := range s.bits {
		if !found[id] {
			orphans = append(orphans, Short(id))
		}
	}
	s.mu.RUnlock()
	slices.Sort(orphans)

	t.Errorf("the record holds %d bits and the view reaches %d; %d orphaned: %v",
		s.Len(), len(found), len(orphans), orphans)
}
