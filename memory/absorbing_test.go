package memory

import (
	"maps"
	"slices"
	"strings"
	"testing"
	"time"
)

// [View.Absorbing] exists to keep one promise a screen makes: nothing is ever
// absorbed that was not drawn cooling first. So the only test that means
// anything about it is the one that folds and compares — a prediction agreeing
// with itself is what a screen already had when it kept an index.
//
// Read the tautology risk here before trusting any of it, because it is real and
// it is the point of D27. Absorbing and [View.Fold] share [View.runs], so the
// agreement below cannot catch an error in the rule they share: break the size
// rule inside runs and both sides move together, and the comparison passes on a
// prediction and a fold that are both wrong in the same way. What
// the agreement does catch is the two callers drifting in what they do with the
// stretches they are handed, which is where a divergence could actually be
// written. The rule itself is held up by the want column of the table — indices
// written out by hand, from the rule as stated, rather than derived from either
// path. Mutations run against both instruments are named at each test.

// cooledBy is which of before's bits the fold that produced after actually
// replaced: every cold bit in after that before did not already name, read
// through Prev.
//
// Prev and not Absorbed, and the difference decides this test rather than
// dressing it. Absorbed carries originals only, so a fold that absorbs an
// earlier fold names that fold's originals and never the fold itself — and the
// fold itself is a bit of before, which is exactly the kind of thing being
// predicted. Prev is every bit in the window in window order (D13), so it is the
// only edge that answers "which of these rows went".
//
// The one thing it cannot see is a newly cooled bit that addresses identically
// to one before already held, which would be the same compaction of the same
// window appearing twice in one view. No fixture here builds that, and content
// addressing is what makes it a real possibility rather than an imagined one.
func cooledBy(s *Store, before, after View) map[string]bool {
	was := map[string]bool{}
	for _, id := range before {
		was[id] = true
	}

	out := map[string]bool{}
	for _, b := range after.Bits(s) {
		if _, cold := b.Payload.(Compaction); !cold || was[b.ID] {
			continue
		}
		for _, id := range b.Prev {
			out[id] = true
		}
	}
	return out
}

// upvote is one entry in a fixture's vote schedule: which bit of the view tyler
// upvoted, and the minute he did it. The minute is separate from the bit's own
// because a hold's age is the vote's age, not the material's — that is what the
// decayed row below turns on.
type upvote struct{ bit, min int }

// The table is every shape a hold can cut a window into, and each row asserts
// twice: that Absorbing predicts the bits the rule says it should, and that a
// real fold of the same view absorbs exactly that and nothing else.
//
// Mutations run, the one-sided ones first, since those are what the agreement
// half is here for. Each was run over the whole package and the tallies below
// count rows of *this table* — a scope worth stating, because the one wrong
// tally these notes have carried so far was an exclusivity claim that was true
// of the table and false of the package:
//
//   - [View.Fold] cooling singletons on its own (`if !cool` becoming
//     `if !cool && len(run) != 1`) fails four rows on the agreement — the fold
//     took bits Absorbing never named — and two more on the fold reporting a
//     fold where nothing was predicted.
//   - [View.Absorbing] naming what passes through as well (its `if !cool`
//     becoming `if !cool && false`) fails nine of the ten rows, on want: the kept
//     tail is a stretch too. The exception is "a keep of none", which is the only
//     row with nothing passing through at all — no tail, no hold, one run — so
//     the mutation has nothing extra to name. It said "all nine rows" when there
//     were nine and both numbers moved in the same edit.
//
// Then the shared rule, where the agreement is structurally blind and want is
// not. Both of these leave every agreement green and fail before reaching it,
// on want:
//
//   - the size rule in [View.runs] (`len(run) > 1` becoming `len(run) > 0`)
//     fails the three rows holding a lone unspared bit — "a lone bit between a
//     hold and a bit a hold covers", "a window of one", and "a hold that has run
//     out and one that has not". This is the mutation that would ship if want
//     were derived rather than written out.
//
//     Not only those three, which is a correction and not a footnote: it also
//     fails [TestAbsorbingNamesAScarItWouldCoolAgain] below, on its
//     window-of-one assertion, and several tests elsewhere that reach the size
//     rule through a fold. The set is not restated here — it is `red` in
//     `docs/CLAIMS.md`'s `a-lone-bit-is-cooled`, held there by `sole`, and this
//     note previously carried a count of it that was wrong. A wrong count inside
//     the argument for a check's soundness is D22's own shape, which is the
//     entry recording that a checkable claim nobody re-derived is the defect to
//     expect; the repair is to stop keeping a second copy of the set rather than
//     to correct the copy again.
//
//   - the window (`len(bits) - keep` becoming one less) fails "no holds", "a
//     hold at the front", "one hold cutting the window into two runs" and "a
//     keep of none". The other rows pass it, because their holds fall either
//     side of the moved boundary and predict the same set anyway — worth knowing
//     about a table of this shape, and the reason the two rows with no hold in
//     the middle of the window are in it.
//
// Every row's want moved when a hold learned to cover the bit it answers, and
// the three rows whose *holds* moved are named where they sit. The reason to
// look at them rather than at the diff: two rows would otherwise have gone to
// nothing at all — a hold every third bit spares two rows in every three, which
// leaves runs of one everywhere and a fold with nothing to take — and a table
// row that asserts an empty prediction against a refused fold is a row that has
// stopped distinguishing anything.
func TestAbsorbingIsExactlyWhatTheFoldThenAbsorbs(t *testing.T) {
	tyler := Handle{Ref: "tyler", Display: "tyler"}

	tests := []struct {
		name string
		keep int
		up   []upvote
		hold time.Duration
		want []int
	}{
		// One unbroken run: the shape before a vote existed at all.
		{"no holds", 2, nil, DefaultHold, []int{0, 1, 2, 3, 4, 5}},

		// D32's other dead half: the view now begins with a hot bit, because a
		// hold at the front of the window has no run in front of it to cool. It
		// is also the one hold in this table that spares a single row — the
		// first bit in a record answers nothing, so its Prev is empty and there
		// is nothing in front of it to cover.
		{"a hold at the front", 2, []upvote{{0, 8}}, DefaultHold, []int{1, 2, 3, 4, 5}},

		// Several scars from one fold, which is the arrangement no index can
		// describe and this function exists for — and it takes one vote to
		// produce, because a hold covers the bit it answers as well as itself.
		// Bits 2 and 3 are the pair; the runs either side of them are the scars.
		{"one hold cutting the window into two runs", 2, []upvote{{3, 8}}, DefaultHold,
			[]int{0, 1, 4, 5}},

		// The case a call site gets wrong. Bit 4 is unheld, adjacent to nothing
		// a fold can take — bit 3 is held and bit 5 is the bit the hold on 6
		// covers — and it survives. A screen dimming it has promised a fold that
		// then refuses.
		{"a lone bit between a hold and a bit a hold covers", 2, []upvote{{3, 8}, {6, 8}},
			DefaultHold, []int{0, 1}},

		{"every bit in the window held", 2,
			[]upvote{{0, 8}, {1, 8}, {2, 8}, {3, 8}, {4, 8}, {5, 8}}, DefaultHold, nil},

		{"a window of one", 7, nil, DefaultHold, nil},
		{"a window of none", 8, nil, DefaultHold, nil},
		{"a keep longer than the view", 12, nil, DefaultHold, nil},

		// Keeping nothing, which is legal and was not asserted anywhere until
		// `go-gremlins` pointed out that [View.Absorbing]'s guard could be
		// loosened from `keep < 0` to `keep <= 0` — refusing a keep of none —
		// with the whole suite still green. [View.Fold]'s own guard was covered
		// and this one was not, which is what a second statement of a rule costs
		// when only one of them is exercised.
		{"a keep of none", 0, nil, DefaultHold, []int{0, 1, 2, 3, 4, 5, 6, 7}},

		// A hold is a stay of execution, and Absorbing has to read the same
		// expiry the fold will. The view's newest instant is minute 7, so with a
		// three-minute lifetime the vote cast at minute 2 has run out and the one
		// cast at minute 7 has not. Reading a wall clock on either side, or
		// reading the window's own latest rather than the view's, moves one of
		// these two rows and not the other.
		{"a hold that has run out and one that has not", 2, []upvote{{2, 2}, {4, 7}}, 3 * time.Minute,
			[]int{0, 1, 2}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewStore()

			var shown View
			for i := range 8 {
				shown, _ = shown.Add(s, said(i, "persona", "bit", shown.Head()...))
			}

			var votes View
			for _, u := range tt.up {
				votes, _ = votes.Add(s, Cast(at(u.min), tyler, Up, shown.Bits(s)[u.bit]))
			}
			stay := Stay{Votes: votes, By: tyler, For: tt.hold}

			want := map[string]bool{}
			for _, i := range tt.want {
				want[shown[i]] = true
			}

			filed := s.Len()
			got := shown.Absorbing(s, tt.keep, stay)
			if s.Len() != filed {
				t.Errorf("Absorbing filed %d bits; a question about the record may not add to it",
					s.Len()-filed)
			}
			if !maps.Equal(got, want) {
				t.Fatalf("Absorbing names %v, want %v", shortenSet(got), shortenSet(want))
			}

			folded, ok := shown.Fold(s, tt.keep, stay)
			if ok != (len(tt.want) > 0) {
				t.Fatalf("the fold reported %v with %d bits to absorb", ok, len(tt.want))
			}
			if absorbed := cooledBy(s, shown, folded); !maps.Equal(absorbed, got) {
				t.Errorf("the fold absorbed %v after Absorbing said %v",
					shortenSet(absorbed), shortenSet(got))
			}
		})
	}
}

// A scar in the view is where the two readings of "what went" come apart, and
// the wrong one is the one that looks right. A cold bit cooled again is a row
// that leaves the screen, so Absorbing has to name it — while the receipt that
// replaces it names that scar's originals and never the scar, because Absorbed
// carries originals only. A test asking the receipt would report the prediction
// as one bit too many and be wrong about which one.
//
// Also D32 through this function rather than through the fold: a scar alone in
// the window is not going anywhere, whatever else is true of it.
func TestAbsorbingNamesAScarItWouldCoolAgain(t *testing.T) {
	s := NewStore()

	var shown View
	for i := range 8 {
		shown, _ = shown.Add(s, said(i, "persona", "bit", shown.Head()...))
	}
	shown, ok := shown.Fold(s, 2, Stay{})
	if !ok {
		t.Fatal("the first fold refused; the fixture has no scar in it")
	}
	scar := shown[0]

	for i := 8; i < 13; i++ {
		shown, _ = shown.Add(s, said(i, "persona", "bit", shown.Head()...))
	}

	got := shown.Absorbing(s, 2, Stay{})
	if !got[scar] {
		t.Errorf("Absorbing does not name the scar %s, which the next fold cools into a new one",
			Short(scar))
	}

	folded, ok := shown.Fold(s, 2, Stay{})
	if !ok {
		t.Fatal("the second fold refused")
	}
	if absorbed := cooledBy(s, shown, folded); !maps.Equal(absorbed, got) {
		t.Errorf("the fold absorbed %v after Absorbing said %v",
			shortenSet(absorbed), shortenSet(got))
	}

	// Why cooledBy reads Prev. The receipt is the natural thing to ask and it
	// cannot answer this: it names what the scar stood for, not the scar.
	if absorbedBy(s, folded)[scar] {
		t.Errorf("the receipt names the scar %s; cooledBy has stopped being necessary "+
			"and this test has stopped meaning what it says", Short(scar))
	}

	// A scar alone in the window: nothing else to merge with, so nothing moves.
	if lone := shown.Absorbing(s, len(shown)-1, Stay{}); len(lone) > 0 {
		t.Errorf("Absorbing names %v in a window of one, where a fold cools nothing",
			shortenSet(lone))
	}
}

// The screen redraws constantly and the record may not grow because somebody
// looked at it. Fold on a copy of the view is the obvious way to answer this
// question and this is the assertion that refuses it: the copy shares the store,
// and every run it cools is a [Compaction] filed forever.
//
// The counter is live rather than decorative — the same fixture folded moves it
// by two, one scar either side of the pair the vote spares, which is what the
// second half asserts. Without that, a Len that never moved for any reason would
// pass this.
//
// This sentence said "by three" from the commit that wrote it, where the
// assertion three lines below it already said two. Corrected rather than
// deleted, because a number in a comment that no check holds up is the defect
// this repository keeps logging, and the correction is the only part of it that
// leaves a trace.
//
// The vote is on bit 3 rather than bit 2 so that two scars are still what the
// fold costs: a hold covers the bit it answers ([sparing]), so a vote on bit 2
// spares bits 1 and 2, leaves bit 0 alone in a run of one, and files a single
// scar over 3–5.
func TestAbsorbingFilesNothing(t *testing.T) {
	s := NewStore()
	tyler := Handle{Ref: "tyler", Display: "tyler"}

	var shown View
	for i := range 8 {
		shown, _ = shown.Add(s, said(i, "persona", "bit", shown.Head()...))
	}

	var votes View
	votes, _ = votes.Add(s, Cast(at(8), tyler, Up, shown.Bits(s)[3]))
	stay := Stay{Votes: votes, By: tyler, For: DefaultHold}

	filed := s.Len()
	for range 5 {
		shown.Absorbing(s, 2, stay)
	}
	if s.Len() != filed {
		t.Errorf("five questions filed %d bits", s.Len()-filed)
	}

	// Two scars, and the vote bit was already filed above: what the fold costs
	// the record, and the proof that the count above could have moved.
	if _, ok := shown.Fold(s, 2, stay); !ok {
		t.Fatal("the fold refused; the fixture cannot show the counter moving")
	}
	if got, want := s.Len()-filed, 2; got != want {
		t.Errorf("the fold filed %d bits, want %d", got, want)
	}
}

// [TestFoldPanicsOnANegativeKeep]'s reason, in the second method that takes a
// keep. The message names Absorbing and not Fold because the caller is holding a
// screen, not a fold, and a panic naming a method they did not call points them
// at somebody else's arithmetic.
func TestAbsorbingPanicsOnANegativeKeep(t *testing.T) {
	s := NewStore()
	var v View
	for i := range 8 {
		v, _ = v.Add(s, said(i, "tyler", "bit", v.Head()...))
	}

	defer func() {
		r := recover()
		if msg, _ := r.(string); !strings.Contains(msg, "Absorbing") {
			t.Errorf("recovered %v, want a panic naming Absorbing — the caller's mistake has "+
				"to be reported as theirs and not as a hole in the record", r)
		}
	}()
	v.Absorbing(s, -1, Stay{})
}

// The stretches [View.runs] hands out are the view, in order, once each, and
// never empty. Both callers rebuild from them — the fold literally does — so a
// stretch dropped or repeated is a row that vanishes from a screen with no fold
// having happened and nothing on the record saying so. An empty one is the same
// claim from the other side: [Cool] panics on an empty window rather than invent
// a scar for nothing, so a stretch with nothing in it is a fold that only fails
// to happen because both callers happen to skip it.
//
// Swept over view length as well as keep, and that is the finding rather than
// thoroughness for its own sake: the empty-stretch half of this was asserted
// against an eight-bit fixture that could not produce one, while the code
// produced one on any empty view with a keep above zero. A stated invariant that
// the fixture cannot break is the check D27 is about.
//
// Mutations run, each one under the whole sweep:
//
//   - dropping the tail yield at the end of runs loses the kept bits out of
//     every fold, and fails every pair with a bit in the view and a keep above
//     zero — thirty of them as the sweep stands, which is where that number
//     comes from and what it goes stale against. Eight other tests catch the
//     same mutation, all of them on a consequence — bits gone unreachable, a
//     receipt short, a view the wrong length. This one catches it on the cause,
//     and it is the only one whose failure says which stretch went missing. The
//     table above does not catch it at all: what a fold absorbs is unchanged,
//     and only what survives is wrong.
//   - restoring the tail's old guard (`cut < len(bits)` in place of the tail's
//     own length) fails the four empty-view pairs with a keep above zero, and
//     nothing else in the package — a scope stated on purpose, since it is the
//     other note's mistake. That is the defect this sweep was extended to catch,
//     so it had better be the only thing that moves.
func TestRunsHandOutTheWholeViewInOrder(t *testing.T) {
	tyler := Handle{Ref: "tyler", Display: "tyler"}

	// Lengths either side of every threshold in the rule: nothing at all, one
	// bit (never foldable), two (the smallest run that folds), three (the
	// smallest that a hold can cut into two singletons), and the eight the rest
	// of this file uses.
	for _, n := range []int{0, 1, 2, 3, 8} {
		s := NewStore()

		var shown View
		for i := range n {
			shown, _ = shown.Add(s, said(i, "persona", "bit", shown.Head()...))
		}

		// A hold wherever there is a bit to hold, because a held bit is yielded
		// as a stretch of its own and so is half of what is being claimed here.
		// The empty view gets the zero Stay: there is nothing to vote on, and
		// what Cast does with nothing is a different test's question.
		stay := Stay{}
		if n > 0 {
			var votes View
			votes, _ = votes.Add(s, Cast(at(n), tyler, Up, shown.Bits(s)[n/2]))
			stay = Stay{Votes: votes, By: tyler, For: DefaultHold}
		}

		// Past the end of the view on purpose: a keep longer than the view is
		// the case that cuts to a negative index, and it is where the empty
		// stretch came from.
		for keep := range n + 5 {
			var flat View
			for run := range shown.runs(s, keep, stay) {
				if len(run) == 0 {
					t.Errorf("%d bits, keep %d: an empty stretch, which [Cool] would panic on "+
						"rather than invent a scar for nothing", n, keep)
					continue
				}
				for _, b := range run {
					flat = append(flat, b.ID)
				}
			}
			if !slices.Equal(flat, shown) {
				t.Errorf("%d bits, keep %d: the stretches are %v, want the view %v",
					n, keep, shorten(flat), shorten(shown))
			}
		}
	}
}
