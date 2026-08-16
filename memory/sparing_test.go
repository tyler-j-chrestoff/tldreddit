package memory

import (
	"maps"
	"slices"
	"testing"
)

// What a hold covers, and what it does not.
//
// [sparing] is one rule with one exception, and the two halves fail in opposite
// directions: cover too little and a row somebody kept stands on screen stripped
// of the row it names through Prev, which is the defect this was written for;
// cover too much and the fold stops taking anything, which is worse, because a
// record that will not consolidate is the failure the whole surface exists to
// avoid.
//
// So the tests here are in pairs. Each one that asserts a bit is spared has a
// neighbour asserting the next bit along is not, and the guard against covering
// too much has both the case it refuses and the check that says whether that
// case is reachable at all today.

// The headline: a vote lands on a bit, and the bit it names through Prev is not
// cooled out from under it.
//
// Two speakers alternating, which is the shape the rule was measured against — a
// person writes, something writes back, and the vote goes on the newest row
// because that is the row being read when the decision is made. In that shape
// the named bit is also the turn being replied to, and this test is not evidence
// of the stronger reading: every bit here takes [View.Head] as its Prev, so the
// two coincide by construction. [sparing] carries where they come apart.
//
// The second assertion is the one that keeps this from being a rule with no
// edge: the bit one step further back is cooled. A cover reaches one step along
// Prev and stops. Walking transitively would reach the root of the record from
// any held bit, which is every bit there has ever been.
func TestAFoldKeepsTheBitAHeldBitNamesThroughPrev(t *testing.T) {
	s := NewStore()
	tyler := Handle{Ref: "tyler", Display: "tyler"}

	var shown View
	for i := range 8 {
		who := "tyler"
		if i%2 == 1 {
			who = "persona"
		}
		shown, _ = shown.Add(s, said(i, who, "bit", shown.Head()...))
	}

	// Bit 5 is upvoted, bit 4 is what it names through Prev, bit 3 is the turn
	// before that. All three are inside the window at keep 2.
	held, named, earlier := shown[5], shown[4], shown[3]

	var votes View
	votes, _ = votes.Add(s, Cast(at(8), tyler, Up, shown.Bits(s)[5]))
	stay := Stay{Votes: votes, By: tyler, For: DefaultHold}

	going := shown.Absorbing(s, 2, stay)
	if going[named] {
		t.Errorf("the fold is drawn taking %s, which the upvoted %s names through Prev",
			Short(named), Short(held))
	}
	if going[held] {
		t.Errorf("the fold is drawn taking %s, which is upvoted", Short(held))
	}
	if !going[earlier] {
		t.Errorf("the fold spares %s, which is a step further back than any hold reaches",
			Short(earlier))
	}

	folded, ok := shown.Fold(s, 2, stay)
	if !ok {
		t.Fatal("the fold refused; nothing here can be said about what it kept")
	}
	for _, id := range []string{named, held} {
		if !slices.Contains(folded, id) {
			t.Errorf("the fold took %s", Short(id))
		}
	}
	if slices.Contains(folded, earlier) {
		t.Errorf("the fold left %s on screen", Short(earlier))
	}

	// And the position, because a spared pair drawn out of the order things
	// happened is not the row and the row above it. runs yields spared bits where
	// they happened.
	if i, j := slices.Index(folded, named), slices.Index(folded, held); i+1 != j {
		t.Errorf("the named bit is at %d and the held bit at %d, want them adjacent and in that order", i, j)
	}
}

// A cover is not a hold, and this is the assertion that says so from the outside.
//
// It matters more than it reads. Everything a surface draws about a vote — the
// mark, the gauge draining, the word held in a footer — comes from [Stay.Holds],
// and the commit before this rule landed had to fix a footer printing held over
// a record with no votes in it at all. Reintroducing that from the other
// direction, by filing covered bits into the hold map so the fold could find
// them, would have been the same falsehood with more machinery behind it.
//
// So: the covered bit is absent from Holds, absent from [View.Absorbing], and
// present in the view after the fold. Three answers, and only the first two are
// about a vote.
func TestABitACoverSparesIsNotHeld(t *testing.T) {
	s := NewStore()
	tyler := Handle{Ref: "tyler", Display: "tyler"}

	var shown View
	for i := range 8 {
		shown, _ = shown.Add(s, said(i, "persona", "bit", shown.Head()...))
	}

	var votes View
	votes, _ = votes.Add(s, Cast(at(8), tyler, Up, shown.Bits(s)[4]))
	stay := Stay{Votes: votes, By: tyler, For: DefaultHold}

	held := stay.Holds(s, shown.Latest(s))
	if _, up := held[shown[4]]; !up {
		t.Fatalf("%s is not held, so there is no cover under test", Short(shown[4]))
	}
	if _, up := held[shown[3]]; up {
		t.Errorf("%s reports as held with no vote on it anywhere in the record", Short(shown[3]))
	}
	if len(held) != 1 {
		t.Errorf("one vote produced %d holds", len(held))
	}

	folded, ok := shown.Fold(s, 2, stay)
	if !ok {
		t.Fatal("the fold refused")
	}
	if !slices.Contains(folded, shown[3]) {
		t.Errorf("the fold took %s, which the hold on %s covers", Short(shown[3]), Short(shown[4]))
	}
}

// The guard: a hold on a scar covers nothing.
//
// A [Compaction]'s Prev is every bit in the window it absorbed (D13), so if a
// cold bit covered what it names, one upvote on a receipt would spare a whole
// generation. The view here is built by hand to be exactly that arrangement — a
// scar sitting beside the bits it stands for — and with the guard the fold cools
// them, which is what a fold is for.
//
// Measured, by deleting `if !hot(b) { continue }` from [sparing] and running this
// alone: the fold refuses, the view comes back its original length, and nothing
// is ever consolidated again while that vote stands.
//
// The arrangement is not reachable through [View.Add] and [View.Fold] today —
// [TestAScarInAViewNeverNamesABitStillInIt] is that claim, executed rather than
// asserted — which makes this guard a prior about a rule that could change
// rather than a live protection. It is four lines and the cost of being wrong
// about it is every fold, so it stays, and the check next door is what will say
// the day the prior stops holding.
func TestAHeldScarSparesOnlyItself(t *testing.T) {
	s := NewStore()
	tyler := Handle{Ref: "tyler", Display: "tyler"}

	var written View
	for i := range 4 {
		written, _ = shownAdd(s, written, i)
	}
	scar := s.Put(Cool(written.Bits(s)[:3]))

	// The scar and the three bits it names, in one view. Hand-built on purpose:
	// see above for why nothing else can produce it.
	shown := View{written[0], written[1], written[2], scar.ID, written[3]}
	if !slices.Contains(scar.Prev, written[0]) {
		t.Fatal("the scar does not name the first bit, so this fixture is not the case being described")
	}

	var votes View
	votes, _ = votes.Add(s, Cast(at(8), tyler, Up, scar))
	stay := Stay{Votes: votes, By: tyler, For: DefaultHold}

	going := shown.Absorbing(s, 1, stay)
	for _, id := range scar.Prev {
		if !going[id] {
			t.Errorf("%s is spared, and the only vote in the record is on the scar that absorbed it",
				Short(id))
		}
	}
	if going[scar.ID] {
		t.Errorf("the scar %s is drawn cooling, and it is upvoted", Short(scar.ID))
	}

	folded, ok := shown.Fold(s, 1, stay)
	if !ok {
		t.Fatal("the fold refused: one vote on one receipt stopped the record consolidating")
	}
	if len(folded) >= len(shown) {
		t.Errorf("the view is %d rows after the fold, from %d", len(folded), len(shown))
	}
}

// Whether the arrangement above can happen on its own, which is the difference
// between a guard and a decoration.
//
// It cannot, today, and the reason is [Cool]: a scar's Prev is the window it
// replaced, and replaced means those rows left the view in the same operation
// that minted it. So no view built by [View.Add] and [View.Fold] holds both a
// cold bit and a bit that cold bit names.
//
// This is written as a check rather than as a sentence in a comment because it
// is exactly the kind of claim that goes quietly false: any change letting a
// fold keep material beside its own receipt — an unfold that writes back into
// the view, a merge of two records' views, a scar that inherits an edge — makes
// this red, and [TestAHeldScarSparesOnlyItself] the live protection it is not
// yet.
func TestAScarInAViewNeverNamesABitStillInIt(t *testing.T) {
	s := NewStore()
	tyler := Handle{Ref: "tyler", Display: "tyler"}

	var shown, votes View
	folds := 0
	for i := range 120 {
		var b Bit
		shown, b = shownAdd(s, shown, i)
		if i%7 == 0 {
			votes, _ = votes.Add(s, Cast(at(i), tyler, Up, b))
		}

		stay := Stay{Votes: votes, By: tyler, For: DefaultHold}
		if len(shown) > 12 {
			if next, ok := shown.Fold(s, 6, stay); ok {
				shown, folds = next, folds+1
			}
		}

		in := map[string]bool{}
		for _, id := range shown {
			in[id] = true
		}
		for _, x := range shown.Bits(s) {
			if hot(x) {
				continue
			}
			for _, id := range x.Prev {
				if in[id] {
					t.Fatalf("after %d bits the view holds the scar %s and %s, which it names",
						i+1, Short(x.ID), Short(id))
				}
			}
		}
	}

	if folds == 0 {
		t.Fatal("no fold happened, so no scar was ever in the view and nothing here was checked")
	}
}

// A hold whose bit names something the view has already lost spares only itself.
//
// The cover is a lookup, not a promise: [sparing] names what a held bit's Prev
// names, and if the fold before this one already absorbed that bit, the name
// resolves to nothing on screen. Nothing is resurrected — D1 is about the
// record, and this is the view.
//
// It is the case that says the rule costs a row where there is a row to cost and
// nothing where there is not, which is what keeps a held bit at the top of a
// freshly folded view from behaving differently to one anywhere else.
func TestAHoldSparesOnlyItselfWhenWhatItNamesHasGone(t *testing.T) {
	s := NewStore()
	tyler := Handle{Ref: "tyler", Display: "tyler"}

	var shown View
	for i := range 10 {
		shown, _ = shownAdd(s, shown, i)
	}

	// One fold with nobody voting, which takes bits 0–5 and leaves a scar in
	// front of bit 6. Bit 6's Prev names bit 5, which is now inside that scar.
	shown, ok := shown.Fold(s, 4, Stay{})
	if !ok {
		t.Fatal("the first fold refused; the fixture has no absorbed question in it")
	}
	held := shown[1]
	if b, _ := s.Get(held); len(b.Prev) != 1 || slices.Contains(shown, b.Prev[0]) {
		t.Fatalf("%s names something still on screen, so this is not the case being described",
			Short(held))
	}

	var votes View
	up, _ := s.Get(held)
	votes, _ = votes.Add(s, Cast(at(11), tyler, Up, up))
	stay := Stay{Votes: votes, By: tyler, For: DefaultHold}

	// Written out by hand from the rule rather than read off a run. The window at
	// keep 1 is the scar and the three bits after it. The scar stands alone in
	// front of the hold and D32's size rule leaves it; the held bit stays; the
	// last two are a run of two and go. No row here is spared by a cover, because
	// the bit the held one names is inside the scar.
	want := map[string]bool{shown[2]: true, shown[3]: true}
	if got := shown.Absorbing(s, 1, stay); !maps.Equal(got, want) {
		t.Errorf("the fold takes %v, want %v", shortenSet(got), shortenSet(want))
	}

	// The control, and it is what makes the line above mean something: with no
	// vote in force the same window goes whole, so every bit spared above was
	// spared by the vote and by the arithmetic around it.
	if got := len(shown.Absorbing(s, 1, Stay{})); got != 4 {
		t.Errorf("the unvoted fold takes %d bits of a four-bit window", got)
	}
}

// [View.Sparing] is the exported reading of that rule, and it answers about the
// view it is asked of and nothing else.
//
// Two properties, and the second is the whole reason it is not simply [sparing]
// with a capital letter. A hold whose named bit has already been folded away
// still names it in Prev, and the unexported rule is free to put it in the map —
// nothing downstream of `runs` ever looks it up. A caller holding this map is not
// so lucky: it is drawing a screen or counting a trigger, and an address the view
// does not hold is one it cannot draw and cannot fold.
//
// Built on the arrangement [TestAHoldSparesOnlyItselfWhenWhatItNamesHasGone]
// already establishes — a fold, then an upvote on the bit right behind the scar
// — because that is the one place a hold reaches at something the view has lost.
func TestSparingAnswersAboutTheViewItIsAsked(t *testing.T) {
	s := NewStore()
	tyler := Handle{Ref: "tyler", Display: "tyler"}

	var shown View
	for i := range 10 {
		shown, _ = shownAdd(s, shown, i)
	}

	// Before anybody votes: a hold-free view spares nothing at all, which is what
	// makes every entry below attributable to the one vote.
	if got := shown.Sparing(s, Stay{}); len(got) != 0 {
		t.Fatalf("a view nobody has voted in spares %v", shortenSet(got))
	}

	// A vote on a bit whose named bit is still on screen: both are spared, and the
	// row before the named one is not — a cover reaches one step and stops.
	held := shown[6]
	up, _ := s.Get(held)
	var votes View
	votes, _ = votes.Add(s, Cast(at(11), tyler, Up, up))
	stay := Stay{Votes: votes, By: tyler, For: DefaultHold}

	want := map[string]bool{held: true, shown[5]: true}
	if got := shown.Sparing(s, stay); !maps.Equal(got, want) {
		t.Errorf("the view spares %v, want the held bit and the one it names through Prev, %v",
			shortenSet(got), shortenSet(want))
	}

	// And a hold whose named bit has left the view spares only itself. The address
	// is still in the held bit's Prev and the store still has the bit; what has
	// changed is that this view does not, and that is the question being asked.
	folded, ok := shown.Fold(s, 4, Stay{})
	if !ok {
		t.Fatal("the fold refused; there is no absorbed question in this fixture")
	}
	stranded := folded[1]
	b, _ := s.Get(stranded)
	if len(b.Prev) != 1 {
		t.Fatalf("%s has %d parents, so it does not name exactly one lost bit", Short(stranded), len(b.Prev))
	}
	if slices.Contains(folded, b.Prev[0]) {
		t.Fatalf("%s names something still on screen, so this is not the case being described", Short(stranded))
	}
	if _, held := s.Get(b.Prev[0]); !held {
		t.Fatalf("the record has lost %s, which is a different failure entirely", Short(b.Prev[0]))
	}

	votes = View{}
	up, _ = s.Get(stranded)
	votes, _ = votes.Add(s, Cast(at(12), tyler, Up, up))
	stay = Stay{Votes: votes, By: tyler, For: DefaultHold}

	if got := folded.Sparing(s, stay); !maps.Equal(got, map[string]bool{stranded: true}) {
		t.Errorf("the folded view spares %v, want only the held bit — the bit it names is in the store and not in this view",
			shortenSet(got))
	}
}
