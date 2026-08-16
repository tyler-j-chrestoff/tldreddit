package memory

import (
	"maps"
	"testing"
)

// A fragment is an utterance whose speaker ran out of room. It is the same kind
// of occurrence as any other thing said — someone said it, and it is in the
// record forever — and the only thing separating it from a complete thought is
// that the record says so. These tests are about that saying-so surviving each
// of the three places it could quietly stop: the address, the store, and the
// fold.
//
// What is deliberately not here is a check that a complete utterance still
// addresses the way it did before fragments existed. [TestIDIsPinned] already
// is that check, and it stayed green through this change, which is the claim:
// nothing about a complete utterance changed.

// cutOff is [said] for a speaker who did not get to finish. Same shape on
// purpose — every test below pairs one against the other, so anything that
// differs has to be the truncation.
func cutOff(min int, who, text string, prev ...string) Bit {
	b := Bit{
		At:      at(min),
		From:    Handle{Ref: who, Display: who},
		Channel: "tui",
		Payload: Utterance{Text: text, Truncated: true},
		Prev:    prev,
	}
	b.ID = ID(b)
	return b
}

// The address is the whole guard. If these two hash alike, a store holding both
// keeps whichever arrived first and hands it to everyone who asks for either —
// so half the time an auditor following a receipt to a fragment is shown a
// complete utterance instead, with nothing anywhere reporting a substitution.
func TestAFragmentAddressesApartFromACompleteUtterance(t *testing.T) {
	tests := []struct {
		name, text string
	}{
		{"a sentence that stops", "the deploy failed because the"},
		{"nothing said at all", ""},
		{"one word", "yes"},

		// The kind names reach the hash as tags, so text that spells a tag is
		// the case where an encoding without length prefixes would let a
		// speaker forge one.
		{"text spelling its own tag", "fragment"},
		{"text spelling the other tag", "utterance"},
		{"text spelling a tag and more", "fragmentthe deploy failed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			whole, cut := base(), base()
			whole.Payload = Utterance{Text: tt.text}
			cut.Payload = Utterance{Text: tt.text, Truncated: true}

			if ID(whole) == ID(cut) {
				t.Errorf("%q addressed to %s complete and cut off alike",
					tt.text, Short(ID(whole)))
			}
		})
	}
}

// Pinned for the reason [TestIDIsPinned] gives, and one more: every test above
// is differential, comparing two IDs from the same encoder, so collapsing
// Utterance.kind back to a constant would move both sides in step and leave
// them green. This is the row that would fail.
func TestFragmentIDIsPinned(t *testing.T) {
	// Derived from the encoding rather than copied out of a failing run: the
	// same bytes hashed by hand outside Go give this, and the same computation
	// with "utterance" in place of the tag gives [TestIDIsPinned]'s golden. A
	// pin taken from the output it is supposed to check is not a pin.
	const want = "b601ac07869ceaa23112547deb82de075e702e19cdf1a2b8cde23a58b902736c"

	b := base()
	b.Payload = Utterance{Text: "the deploy failed", Truncated: true}
	if got := ID(b); got != want {
		t.Errorf("ID(fragment) = %s, want %s", got, want)
	}
}

// Two bits, not one. A Store collapses identical content by design, which is
// exactly why the distinction has to live in the content rather than beside it.
func TestAStoreFilesAFragmentApartFromItsCompleteTwin(t *testing.T) {
	s := NewStore()
	whole := s.Put(Bit{At: at(0), Channel: "tui", Payload: Utterance{Text: "the deploy failed because the"}})
	cut := s.Put(Bit{At: at(0), Channel: "tui", Payload: Utterance{Text: "the deploy failed because the", Truncated: true}})

	if whole.ID == cut.ID {
		t.Fatalf("both stored at %s", Short(whole.ID))
	}
	if s.Len() != 2 {
		t.Errorf("Len = %d, want 2", s.Len())
	}

	for name, want := range map[string]Bit{"complete": whole, "cut off": cut} {
		got, ok := s.Get(want.ID)
		if !ok {
			t.Fatalf("%s: the store does not hold %s", name, Short(want.ID))
		}
		if got.Payload != want.Payload {
			t.Errorf("%s: Get(%s) resolved to %#v, want %#v",
				name, Short(want.ID), got.Payload, want.Payload)
		}
	}
}

// The fold is where the text stops being available, so it is where a fragment
// would stop being knowable. Kinds is the one thing that still reports it, and
// it has to keep summing to Count while doing so.
func TestAFoldTalliesFragmentsSeparately(t *testing.T) {
	c := Cool([]Bit{
		said(0, "tyler", "why did the deploy fail"),
		cutOff(1, "persona", "the deploy failed because the"),
		said(2, "tyler", "thanks"),
	}).Payload.(Compaction)

	want := map[string]int{"utterance": 2, "fragment": 1}
	if got := maps.Collect(c.Kinds()); !maps.Equal(got, want) {
		t.Errorf("Kinds = %v, want %v", got, want)
	}
	if c.Count() != 3 {
		t.Errorf("Count = %d, want 3", c.Count())
	}

	// A fragment is still something said, so its words are still words. Dropping
	// them from the bag would be the opposite error to the one this change
	// exists to fix: material lost because it was labelled.
	if got := maps.Collect(c.Bag()); got["because"] != 1 {
		t.Errorf("Bag[because] = %d, want 1 — the fragment's words did not reach the bag", got["because"])
	}
}

// Aggregates merge rather than restart, so the tally has to survive a
// generation. This is the leak [TestCoolIsClosedUnderItself] is about, asked
// about the key that is new.
func TestAFoldOfAFoldKeepsTheFragmentTally(t *testing.T) {
	first := Cool([]Bit{
		said(0, "tyler", "why did the deploy fail"),
		cutOff(1, "persona", "the deploy failed because the"),
	})
	c := Cool([]Bit{first, said(2, "tyler", "thanks")}).Payload.(Compaction)

	want := map[string]int{"utterance": 2, "fragment": 1}
	if got := maps.Collect(c.Kinds()); !maps.Equal(got, want) {
		t.Errorf("Kinds = %v, want %v — a generation was flattened", got, want)
	}
}

// D14: a fragment absorbed by a fold is still walkable from the view, which is
// what makes the tally above checkable rather than merely stated. The scar says
// one fragment; this is the reader being able to go and read it.
func TestAFragmentStaysReachableThroughAFold(t *testing.T) {
	s := NewStore()
	var v View

	v, _ = v.Add(s, Bit{At: at(0), Channel: "tui", From: Handle{Ref: "tyler"},
		Payload: Utterance{Text: "why did the deploy fail"}, Prev: v.Head()})
	v, cut := v.Add(s, Bit{At: at(1), Channel: "tui", From: Handle{Ref: "persona"},
		Payload: Utterance{Text: "the deploy failed because the", Truncated: true}, Prev: v.Head()})
	v, _ = v.Add(s, Bit{At: at(2), Channel: "tui", From: Handle{Ref: "tyler"},
		Payload: Utterance{Text: "thanks"}, Prev: v.Head()})

	v, folded := v.Fold(s, 1, Stay{})
	if !folded {
		t.Fatal("nothing folded; the test is wrong")
	}

	found := reachable(t, s, v)
	if !found[cut.ID] {
		t.Errorf("the fragment %s is in the store and no reader can walk to it", Short(cut.ID))
	}
	if len(found) != s.Len() {
		t.Errorf("the record holds %d bits and the view reaches %d", s.Len(), len(found))
	}
}
