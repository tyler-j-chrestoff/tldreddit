package memory

import (
	"testing"
)

// reachable walks the record from the views a reader holds, the way a reader
// would: follow every Prev, and follow every Absorbed on a receipt. It returns
// the IDs found.
//
// This is the operational meaning of "permanently reachable" in D1, and there
// are three readings of it that a record can satisfy separately. Content
// addressing makes a bit *retrievable* by anyone already holding its address.
// [Store.All] makes the record *enumerable* by any process holding the whole
// store — more than retrievable, since no address has to be known first, and
// less than discoverable, since it comes with no starting point: a reader is
// handed every bit and nothing that says which ones they were meant to begin
// from. Only the edges make a bit *discoverable*, and D14 counts that one alone,
// because a reader arrives holding a view rather than a hash or a store.
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

	// Only to name the casualties, and this is the one place enumeration belongs
	// in this test: an orphan is a bit the store enumerates and the walk did not
	// reach, so naming one takes two of the three readings at once.
	//
	// [Store.All] exists and the product uses it — ranking has to see every bit
	// rather than the ones a view is already showing. What it makes the record is
	// enumerable, which D14 does not count, which is why the walk above follows
	// Prev and Absorbed rather than asking All.
	//
	// What getting that wrong would cost is worth stating exactly, because it is
	// not the obvious thing. This check would go on reddening either way — a
	// manufactured orphan reddens it, measured. What stops being falsifiable is
	// the *decision*: let enumeration count as reachability and an orphan is
	// compliant, so this stays red-capable while testing nothing D14 requires — a
	// preference with a t.Errorf attached. (Writing the walk itself over All
	// would be the ordinary vacuity on top of that, D27's class, since every bit
	// in the store is in All by construction.)
	//
	// All hands them back in address order, which is stable and is all a message
	// printing hashes needs.
	var orphans []string
	for b := range s.All() {
		if !found[b.ID] {
			orphans = append(orphans, Short(b.ID))
		}
	}

	t.Errorf("the record holds %d bits and the view reaches %d; %d orphaned: %v",
		s.Len(), len(found), len(orphans), orphans)
}
