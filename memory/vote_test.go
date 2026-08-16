package memory

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"
)

// A vote is the cheapest act a participant can take and the only one that
// changes what survives a fold, so these tests are about the three places that
// cheapness could quietly stop meaning anything: the address, where a direction
// that does not reach the hash makes up and down one bit; the tally, where a
// change of mind could be counted twice or not at all; and the fold, where a
// stay is either honoured or is a feature that exists only in a comment.
//
// Every one of them was mutation-checked. The mutations are named where they
// are not obvious, because a test whose failure mode nobody has seen is the
// instrument D27 is about.

// voted is [said] for a participant who is voting rather than talking. Same
// shape on purpose: minute, who, and the thing itself.
func voted(min int, who string, dir Direction, on Bit) Bit {
	return Cast(at(min), Handle{Ref: who, Display: who}, dir, on)
}

// voteBase is a fully populated vote bit, the way [base] is for an utterance:
// every case below is a departure from it, so a difference in ID is
// attributable to the field that moved.
func voteBase() Bit {
	return Bit{
		At:      at(0),
		From:    Handle{Ref: "tyler", Display: "Tyler"},
		Channel: "tui",
		Payload: Vote{dir: Up},
		Prev:    []string{"a"},
	}
}

// The direction is in the tag and the target is in Prev, so both have to reach
// the address — and the reason is the same for each. Two votes that address
// alike are one bit to a [Store], so an upvote and a downvote by the same person
// in the same second would collapse into whichever landed first, and the record
// would hold a judgment nobody made.
func TestAVoteAddressesByDirectionTargetAndVoter(t *testing.T) {
	tests := []struct {
		name string
		bit  Bit
	}{
		{"the other direction", func() Bit { b := voteBase(); b.Payload = Vote{dir: Down}; return b }()},
		{"a different target", func() Bit { b := voteBase(); b.Prev = []string{"b"}; return b }()},
		{"a different voter", func() Bit { b := voteBase(); b.From.Ref = "agent"; return b }()},
		{"a different moment", func() Bit { b := voteBase(); b.At = at(1); return b }()},
		{"a different channel", func() Bit { b := voteBase(); b.Channel = "internal"; return b }()},

		// A vote and an utterance whose text spells the vote's own tag. The tags
		// are length-prefixed, which is what stops a speaker forging one.
		{"an utterance that spells the tag", func() Bit {
			b := voteBase()
			b.Payload = Utterance{Text: "upvote"}
			return b
		}()},
	}

	seen := map[string]string{ID(voteBase()): "an upvote"}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ID(tt.bit)
			if prior, clash := seen[got]; clash {
				t.Errorf("ID = %s, same as %q — the encoding cannot tell them apart",
					Short(got), prior)
			}
			seen[got] = tt.name
		})
	}
}

// Pinned for [TestIDIsPinned]'s reason, and for the one [TestFragmentIDIsPinned]
// adds: every case above is differential, comparing two IDs out of the same
// encoder, so a change to the encoding's shape moves both sides in step and
// leaves them green. This is the row that would fail.
//
// Derived from the encoding rather than copied out of a run: the rules in
// [canon] — eight-byte big-endian lengths, a length-prefixed tag before every
// composite, seconds and nanoseconds in UTC — hashed by hand outside Go give
// this, and the same computation with "utterance" and a text field gives
// [TestIDIsPinned]'s golden, which is how the hand computation was checked.
func TestVoteIDIsPinned(t *testing.T) {
	const want = "a149ab51532b9366aee13e3634d379b2c0093fe2e7bbedb19cf8f0b6edd46a90"
	if got := ID(voteBase()); got != want {
		t.Errorf("ID(voteBase) = %s, want %s", got, want)
	}
}

// What [Cast] is for: the edge. A vote whose Prev is anything but the one bit it
// votes on is a vote about nothing in particular, and nothing downstream can
// repair it.
func TestCastFollowsExactlyWhatItVotesOn(t *testing.T) {
	target := said(3, "persona", "the deploy failed because the disk filled")
	v := Cast(at(4), Handle{Ref: "tyler", Display: "Tyler"}, Up, target)

	if want := []string{target.ID}; !slices.Equal(v.Prev, want) {
		t.Errorf("Prev = %v, want exactly the target %v", v.Prev, want)
	}
	if v.Channel != target.Channel {
		t.Errorf("Channel = %q, want the target's %q", v.Channel, target.Channel)
	}
	if !v.At.Equal(at(4)) {
		t.Errorf("At = %s, want the moment the caller gave, %s", v.At, at(4))
	}
	if p, ok := v.Payload.(Vote); !ok || p.Dir() != Up {
		t.Errorf("Payload = %#v, want an upvote", v.Payload)
	}
	if v.ID != ID(v) {
		t.Errorf("Cast returned ID %s but the bit addresses to %s", Short(v.ID), Short(ID(v)))
	}
}

// Votes with no lifetime is the one thing a [Stay] cannot be asked to guess at:
// zero could mean "hold nothing" or "hold forever" and those are opposite
// answers. A caller who wants the old permanence asks for a century out loud.
func TestAStayWithVotesAndNoLifetimeRefuses(t *testing.T) {
	s := NewStore()
	target := s.Put(said(0, "persona", "roll it back"))
	tyler := Handle{Ref: "tyler", Display: "tyler"}

	var votes View
	votes, _ = votes.Add(s, Cast(at(1), tyler, Up, target))

	defer func() {
		if recover() == nil {
			t.Error("a stay with votes and no lifetime did not panic")
		}
	}()
	Stay{Votes: votes, By: tyler}.Holds(s, at(2))
}

// Two refusals in two different places, so every row says what its panic has to
// contain. Recovering and looking no further cannot tell this package's own
// guard from an unrelated crash — the shape
// [TestTallyPanicsOnAViewThatIsNotVotes] was repaired out of, found here in the
// same pass and deliberately left for a later hand to close (D45(e)).
func TestCastPanics(t *testing.T) {
	stored := said(0, "tyler", "the deploy failed")

	// The direction [Vote.kind] refuses to name, in one place so that the call
	// and what its message has to say about it cannot drift apart.
	const unnamed = Direction(7)

	tests := []struct {
		name string
		call func()

		// says is what the panic must contain, in
		// [TestTallyPanicsOnAViewThatIsNotVotes]'s sense: the identity of the
		// thing that is wrong and what is wrong with it. The first two rows hold
		// only the second half, and say below why there is no first half here to
		// hold.
		says []string
	}{
		// Either would put an empty string in Prev, and an edge to nothing does
		// not report itself — it surfaces later as the store apparently having
		// lost a bit.
		//
		// These two rows want the same three things because [Cast]'s guard
		// prints one message for both: the only field it names is the *target's*
		// Handle.Ref, which is empty in the zero Bit and in any bit a caller
		// built without one, so both calls panic with `from ""` and the two rows
		// are one sentence. That is a gap in the message rather than in this
		// check, and it is named here rather than closed — the hand that finds a
		// weak message is the wrong one to bless a stronger one (D45(e)).
		// Asserting the `from ""` it currently prints would be that blessing, so
		// nothing below asserts it.
		{"the zero bit",
			func() { Cast(at(1), Handle{Ref: "tyler"}, Up, Bit{}) },
			[]string{"Cast", "unaddressed", "store it first"}},
		{"an unaddressed target", func() {
			Cast(at(1), Handle{Ref: "tyler"}, Up, Bit{At: at(0), Channel: "tui",
				Payload: Utterance{Text: "never stored"}})
		}, []string{"Cast", "unaddressed", "store it first"}},

		// Not [Cast]'s own check: it addresses the bit it builds, and kind
		// refuses to name a direction that is not one. The refusal is what keeps
		// D26's map one-to-one, so it is worth a row here rather than only in a
		// comment.
		//
		// What that refusal has to say is the direction it was handed and both
		// directions it was not. The number alone is a value with nothing to be
		// wrong against; "neither Up nor Down" alone leaves a reader holding
		// Direction(0) unable to tell an uninitialised Vote from a number
		// something computed, which is the one question this message exists to
		// answer. The values come from the constants rather than being typed
		// out, so changing a direction's weight moves the assertion with it; the
		// parentheses are the message's own shape, and each name is asserted
		// beside its value rather than separately, because a name and a number
		// that are not next to each other do not say which belongs to which.
		//
		// That rule applies to the refused direction too, and the first version
		// of this row broke it: it wanted a bare "7", which any panic carrying a
		// line number or a byte count satisfies by accident. It was the one want
		// here no mutation had ever reddened on its own — a check that cannot
		// fail (D27) inside the repair of a check that could not fail. The word
		// is what makes it an assertion about a direction.
		{"a direction that is not one",
			func() { Cast(at(1), Handle{Ref: "tyler"}, unnamed, stored) },
			[]string{fmt.Sprintf("direction %d", unnamed), fmt.Sprintf("Up (%d)", Up), fmt.Sprintf("Down (%d)", Down)}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if r == nil {
					t.Fatalf("Cast on %s did not panic", tt.name)
				}
				said := fmt.Sprint(r)
				for _, want := range tt.says {
					if !strings.Contains(said, want) {
						t.Errorf("Cast on %s panicked with %q, which does not say %q",
							tt.name, said, want)
					}
				}
			}()
			tt.call()
		})
	}
}

// Changing your mind is one vote, not two. Counted twice they cancel, and a
// participant who reversed themselves would have less say than one who never
// voted — while the record still has to hold both bits, because it holds
// everything.
//
// Mutation: making [Tally] accumulate (`out[target][b.From] += int(v.Dir)`)
// instead of replace leaves this test reporting 0 for tyler. Run: it fails on
// the score and on nothing else.
func TestTallyKeepsTheLatestVotePerVoter(t *testing.T) {
	s := NewStore()
	target := s.Put(said(0, "persona", "roll it back"))

	var votes View
	votes, up := votes.Add(s, voted(1, "tyler", Up, target))
	votes, down := votes.Add(s, voted(2, "tyler", Down, target))
	votes, _ = votes.Add(s, voted(3, "agent", Up, target))

	got := Tally(s, votes)[target.ID]
	want := Score{
		Handle{Ref: "tyler", Display: "tyler"}: -1,
		Handle{Ref: "agent", Display: "agent"}: 1,
	}
	if !maps.Equal(got, want) {
		t.Errorf("Score = %v, want %v", got, want)
	}

	// The tally forgot the first vote; the record did not. This is the whole
	// division of labour — a view answers what is true now, and the store holds
	// what happened.
	for name, id := range map[string]string{"the first vote": up.ID, "the second": down.ID} {
		if _, held := s.Get(id); !held {
			t.Errorf("%s, %s, is not in the store", name, Short(id))
		}
	}
}

// Two votes at one instant differing only in direction are two bits with equal
// claim to being latest, and something has to decide. The vote view's order is
// the only other thing that means "afterwards" here.
//
// Mutation: dropping the tie to the earlier vote (`b.At.Before(when)` becoming
// `!b.At.After(when)`) reverses both rows below, and each direction fails
// separately — which is why both are here rather than one.
func TestTallyBreaksAnInstantTieByViewOrder(t *testing.T) {
	tests := []struct {
		name  string
		first Direction
		last  Direction
		want  int
	}{
		{"up then down", Up, Down, -1},
		{"down then up", Down, Up, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewStore()
			target := s.Put(said(0, "persona", "roll it back"))

			var votes View
			votes, _ = votes.Add(s, voted(1, "tyler", tt.first, target))
			votes, _ = votes.Add(s, voted(1, "tyler", tt.last, target))

			if got := Tally(s, votes)[target.ID][Handle{Ref: "tyler", Display: "tyler"}]; got != tt.want {
				t.Errorf("score = %d, want %d — the later position did not win", got, tt.want)
			}
		})
	}
}

// A view of anything but votes is not a vote view, and the important case is the
// one that looks like housekeeping: fold the vote view and its head is a
// [Compaction]. Tallying that quietly would report no votes at all, which reads
// as nobody having voted rather than as the votes having been folded away, and
// every stay in the record would lift in the same frame.
//
// What is asserted is the message and not merely that something blew up, and the
// reason is that this guard's whole product is its message. Take it out and
// [Tally] still panics two lines later, on its own type assertion, with
// "interface conversion: memory.Payload is memory.Compaction, not memory.Vote" —
// a true sentence naming a line the caller did not know it was on, about a bit it
// cannot identify. The guard names the offending bit by address and says what it
// carries instead. A test that recovered and looked no further could not tell the
// two apart, and passed happily with the guard deleted; this is that test with the
// hole closed, and cmd/seam is what found the hole.
func TestTallyPanicsOnAViewThatIsNotVotes(t *testing.T) {
	s := NewStore()
	target := s.Put(said(0, "persona", "roll it back"))

	var scar string
	folded := func() View {
		var votes View
		for i := range 3 {
			votes, _ = votes.Add(s, voted(i+1, "tyler", Up, s.Put(said(i+4, "persona", "again"))))
		}
		out, ok := votes.Fold(s, 0, Stay{})
		if !ok {
			t.Fatal("the vote view did not fold; the test is wrong")
		}
		scar = out[0]
		return out
	}

	var twoPrev string
	twoTargets := func() View {
		b := Bit{At: at(9), From: Handle{Ref: "tyler"}, Channel: "tui",
			Payload: Vote{dir: Up}, Prev: []string{target.ID, target.ID}}
		v, b := View{}.Add(s, b)
		twoPrev = b.ID
		return v
	}

	tests := []struct {
		name  string
		votes View

		// says is what the panic has to contain: the address of the bit that is
		// wrong, and what is wrong with it. Both, because either alone is a
		// message somebody still has to go and investigate — the address without
		// the fault names a bit and no complaint, and the fault without the
		// address is a complaint about a record of several hundred.
		says func() []string
	}{
		{"an utterance among the votes", View{target.ID},
			func() []string { return []string{Short(target.ID), "utterance"} }},
		{"a folded vote view", folded(),
			func() []string { return []string{Short(scar), "compaction"} }},
		{"a vote following two bits", twoTargets(),
			func() []string { return []string{Short(twoPrev), "follows 2 bits"} }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if r == nil {
					t.Fatalf("Tally over %s did not panic", tt.name)
				}
				said := fmt.Sprint(r)
				for _, want := range tt.says() {
					if !strings.Contains(said, want) {
						t.Errorf("Tally over %s panicked with %q, which does not say %q",
							tt.name, said, want)
					}
				}
			}()
			Tally(s, tt.votes)
		})
	}
}

// [Stay.Holds] is the rule itself, and it is exported so that a surface deciding
// when to fold, or drawing how long a hold has left, reads the same answer the
// fold will act on rather than a second copy of the rule that drifts from it.
// Five ways not to be held, all of them live: never voted on, voted down, voted
// up by somebody else, voted up and then thought better of, and voted up long
// enough ago that the hold has run out.
func TestAStayHoldsExactlyWhatItsVoterUpvotedAndStillHas(t *testing.T) {
	s := NewStore()
	tyler := Handle{Ref: "tyler", Display: "tyler"}

	var shown View
	for i := range 6 {
		shown, _ = shown.Add(s, said(i, "persona", "bit", shown.Head()...))
	}
	bits := shown.Bits(s)

	var votes View
	votes, _ = votes.Add(s, Cast(at(20), tyler, Up, bits[0]))
	votes, _ = votes.Add(s, Cast(at(21), tyler, Down, bits[1]))
	votes, _ = votes.Add(s, voted(22, "agent", Up, bits[2]))
	votes, _ = votes.Add(s, Cast(at(23), tyler, Up, bits[3]))
	votes, _ = votes.Add(s, Cast(at(24), tyler, Down, bits[3]))
	votes, _ = votes.Add(s, Cast(at(1), tyler, Up, bits[4]))

	// Ten minutes of conversation, read at at(25): the vote at at(1) is long
	// spent, the one at at(20) has five minutes left.
	got := Stay{Votes: votes, By: tyler, For: 10 * time.Minute}.Holds(s, at(25))
	want := map[string]time.Duration{bits[0].ID: 5 * time.Minute}
	if !maps.Equal(got, want) {
		t.Errorf("Holds = %v, want %v", shortenDurations(got), shortenDurations(want))
	}
}

// shortenDurations abbreviates a set of holds for a failure message. Display
// only — [Short] is never what anything compares on.
func shortenDurations(holds map[string]time.Duration) []string {
	out := make([]string, 0, len(holds))
	for id, left := range holds {
		out = append(out, Short(id)+":"+left.String())
	}
	slices.Sort(out)
	return out
}

// A hold is a stay of execution and not a pin: it runs out, and voting again
// grants a fresh one. The vote is untouched by any of that — both votes are
// still in the record afterwards, and the second one is a second bit rather than
// an edit of the first.
//
// Two mutations, both run. Dropping the `left > 0` test in [Stay.Holds], so that
// a hold never expires, fails the spent row. And measuring age against the
// window rather than the whole view — `v[:cut].Latest(s)` in [View.Fold] — fails
// the first row, because a vote cast at the newest bit in the view is in the
// future as far as the window is concerned and every hold would last longer than
// it should.
//
// A third, which no row caught until one was written for it: loosening that same
// test to `left >= 0`, so that a hold with nothing left is still a hold. The
// lifetime is half-open — [Stay].For says a hold survives while [View.Latest] is
// *less than* For past the vote — and the row that pins it is the one cast
// exactly a lifetime before the newest bit in the view, where `left` is exactly
// zero. The spent row cannot see it, because a hold a minute past its lifetime is
// gone under either comparison; only the boundary itself distinguishes them.
// Found by running `go-gremlins` against the package, which is a tool nobody here
// owns and which enumerated in seconds a boundary six sessions of hand-written
// tests had left alone.
func TestAHoldRunsOutAndVotingAgainRenewsIt(t *testing.T) {
	const hold = 10 * time.Minute

	s := NewStore()
	tyler := Handle{Ref: "tyler", Display: "tyler"}

	var shown View
	for i := range 12 {
		shown, _ = shown.Add(s, said(i, "persona", "bit", shown.Head()...))
	}
	kept := shown.Bits(s)[2]

	tests := []struct {
		name  string
		cast  []int // the minutes at which the human upvoted it
		stays bool
	}{
		{"cast just now", []int{11}, true},
		{"cast within the hold", []int{4}, true},
		{"cast and spent", []int{0}, false},
		{"spent, then cast again", []int{0, 11}, true},

		// The boundary. The view's newest bit is minute 11 and the lifetime is
		// ten minutes, so a vote at minute 1 leaves exactly nothing — and a hold
		// with nothing left is not a hold.
		{"cast exactly a lifetime ago", []int{1}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var votes View
			for _, min := range tt.cast {
				votes, _ = votes.Add(s, Cast(at(min), tyler, Up, kept))
			}

			folded, ok := shown.Fold(s, 4, Stay{Votes: votes, By: tyler, For: hold})
			if !ok {
				t.Fatal("the fold refused")
			}
			if got := !absorbedBy(s, folded)[kept.ID]; got != tt.stays {
				t.Errorf("the bit stayed=%v, want %v", got, tt.stays)
			}

			// Whatever the hold did, the votes are still in the record.
			for _, min := range tt.cast {
				if _, held := s.Get(Cast(at(min), tyler, Up, kept).ID); !held {
					t.Errorf("the vote cast at %s is not in the store", at(min))
				}
			}
		})
	}
}

// The bound, which is the reason a hold decays at all. Two hundred sends with
// the human upvoting every second bit — the worst rate there is, since a hold
// every other bit leaves no run of two for a fold to cool — and the view has to
// stay near a screen rather than growing without limit.
//
// The 40 is a ceiling with room in it: the measured worst over this schedule is
// 31 rows. Without decay it is 200, and no fold succeeds at all.
//
// The second assertion is the one that would catch a fold going busy: a fold
// that reports true has always made the view shorter, because the only thing it
// does is replace two or more rows with one.
func TestDecayKeepsTheViewNearAScreen(t *testing.T) {
	const coolAt, keepHot, sends, ceiling = 12, 6, 200, 40

	s := NewStore()
	tyler := Handle{Ref: "tyler", Display: "tyler"}

	worst := 0
	var shown, votes View
	for i := range sends {
		var b Bit
		shown, b = shownAdd(s, shown, i)
		if i%2 == 0 {
			votes, _ = votes.Add(s, Cast(at(i), tyler, Up, b))
		}

		stay := Stay{Votes: votes, By: tyler, For: DefaultHold}
		held := stay.Holds(s, shown.Latest(s))
		band := 0
		for _, x := range shown.Bits(s) {
			if _, hold := held[x.ID]; hot(x) && !hold {
				band++
			}
		}
		if band > coolAt {
			was := len(shown)
			var ok bool
			if shown, ok = shown.Fold(s, keepHot, stay); ok && len(shown) >= was {
				t.Fatalf("after %d sends a fold reported success and left %d rows, from %d",
					i+1, len(shown), was)
			}
		}
		worst = max(worst, len(shown))
	}

	if worst > ceiling {
		t.Errorf("the view reached %d rows over %d sends, want no more than %d",
			worst, sends, ceiling)
	}
}

// The stay itself, and the shape of it. An upvoted bit is held out of the window
// it would otherwise have gone into, the bit it names through Prev is spared
// beside it, the runs either side of the pair are cooled separately, and both sit
// between them where they happened.
//
// The pair is the part to read carefully, because the two rows in it are there
// for different reasons and only one of them is a vote. Bit 3 was upvoted; bit 2
// is what bit 3's Prev names, and it survives because a hold covers the bit it
// names ([sparing]) — the assertion below that it is absent from [Stay.Holds]
// is not decoration. A cover that reported itself as a hold would put the word
// held on a screen over a row nobody voted on.
//
// The two spans are the assertion that matters. One scar over the whole window
// would carry a span running from before this bit to after it while not having
// absorbed it, which is a receipt that reads as covering material it does not
// hold — and a screen drawing it has no way to say otherwise.
//
// Mutation: deleting the `spared[bits[i].ID]` branch in [View.runs] — the whole
// feature — fails this, and so does loosening [Stay.Holds]'s `> 0` to `!= 0`,
// which holds the downvoted bit back as well. Both fail on the view's length,
// which is why the downvoted bit is in the fixture at all: without it, the
// loosened test would have nothing to be wrong about. Dropping the Prev loop in
// [sparing] fails it too, on the length and on which bit view[1] is. Cooling the
// whole window at once and putting the survivor after it — the shape this
// replaced — fails on the spans.
func TestAnUpvotedBitSurvivesAFoldAndSplitsIt(t *testing.T) {
	s := NewStore()
	tyler := Handle{Ref: "tyler", Display: "tyler"}

	var shown View
	var named, kept, sunk Bit
	for i := range 8 {
		shown, _ = shown.Add(s, said(i, "persona", "bit", shown.Head()...))
		switch i {
		case 2:
			named = shown.Bits(s)[i]
		case 3:
			kept = shown.Bits(s)[i]
		case 4:
			sunk = shown.Bits(s)[i]
		}
	}

	var votes View
	votes, _ = votes.Add(s, Cast(at(8), tyler, Up, kept))
	votes, _ = votes.Add(s, Cast(at(9), tyler, Down, sunk))
	stay := Stay{Votes: votes, By: tyler, For: DefaultHold}

	folded, ok := shown.Fold(s, 2, stay)
	if !ok {
		t.Fatal("the fold refused")
	}

	// The run before the pair, the bit the hold covers, the bit held, the run
	// after them, then the two that were never in the window.
	if got, want := len(folded), 6; got != want {
		t.Fatalf("view is %v, want %d entries", shorten(folded), want)
	}
	if folded[2] != kept.ID {
		t.Errorf("view[2] = %s, want the upvoted bit %s", Short(folded[2]), Short(kept.ID))
	}
	if folded[1] != named.ID {
		t.Errorf("view[1] = %s, want %s, which is what the upvoted bit names through Prev",
			Short(folded[1]), Short(named.ID))
	}
	if _, up := stay.Holds(s, shown.Latest(s))[named.ID]; up {
		t.Errorf("%s reports as held, and nobody voted on it — a cover is a fact about a fold "+
			"and a hold is a fact about a person", Short(named.ID))
	}
	if !slices.Equal(folded[4:], shown[6:]) {
		t.Errorf("the tail is %v, want %v untouched", shorten(folded[4:]), shorten(shown[6:]))
	}

	bits := folded.Bits(s)
	before, cold := bits[0].Payload.(Compaction)
	if !cold {
		t.Fatalf("view[0] carries %T, want the run before the pair", bits[0].Payload)
	}
	after, cold := bits[3].Payload.(Compaction)
	if !cold {
		t.Fatalf("view[3] carries %T, want the run after the pair", bits[3].Payload)
	}

	if want := []string{shown[0], shown[1]}; !slices.Equal(before.absorbed, want) {
		t.Errorf("the first receipt names %v, want %v", shorten(before.absorbed), shorten(want))
	}
	if want := []string{shown[4], shown[5]}; !slices.Equal(after.absorbed, want) {
		t.Errorf("the second receipt names %v, want %v", shorten(after.absorbed), shorten(want))
	}
	if !slices.Contains(after.absorbed, sunk.ID) {
		t.Errorf("the second receipt does not name %s, which nobody held back", Short(sunk.ID))
	}

	// Neither span reaches over the two survivors between them.
	if !before.to.Before(named.At) {
		t.Errorf("the first scar spans to %s, at or past the survivor at %s", before.to, named.At)
	}
	if !after.from.After(kept.At) {
		t.Errorf("the second scar spans from %s, at or before the survivor at %s", after.from, kept.At)
	}
}

// The same view, folded twice on the same schedule, differing only in whether a
// vote was in force. If these two absorb the same material then the stay is
// decorative — the fold took the same bits either way and the vote changed
// nothing but the arrangement of a slice.
//
// The vote is on bit 3, and where it lands is a fixture decision rather than an
// arbitrary index: a hold covers the bit it names ([sparing]), so a vote on
// bit 2 would spare bits 1 and 2 and leave bit 0 alone in a run of one, and the
// difference between the two folds would be three bits rather than the two this
// asserts — the third of them a bit the size rule refused, which is D32 showing
// up in an arithmetic this test is not about.
func TestAFoldWithAVoteAbsorbsDifferentMaterialThanOneWithout(t *testing.T) {
	s := NewStore()
	tyler := Handle{Ref: "tyler", Display: "tyler"}

	var shown View
	for i := range 8 {
		shown, _ = shown.Add(s, said(i, "persona", "bit", shown.Head()...))
	}
	kept := shown.Bits(s)[3]

	var votes View
	votes, _ = votes.Add(s, Cast(at(8), tyler, Up, kept))

	byRecency, ok := shown.Fold(s, 2, Stay{})
	if !ok {
		t.Fatal("the fold by recency refused")
	}
	byVote, ok := shown.Fold(s, 2, Stay{Votes: votes, By: tyler, For: DefaultHold})
	if !ok {
		t.Fatal("the fold with a vote refused")
	}

	recency, voted := absorbedBy(s, byRecency), absorbedBy(s, byVote)
	if maps.Equal(recency, voted) {
		t.Fatalf("both folds absorbed %v; the vote made no difference", shortenSet(recency))
	}
	if !recency[kept.ID] {
		t.Errorf("the unvoted fold left %s on screen; the fixture is wrong", Short(kept.ID))
	}
	if voted[kept.ID] {
		t.Errorf("the voted fold absorbed %s anyway", Short(kept.ID))
	}
	if got, want := len(voted), len(recency)-2; got != want {
		t.Errorf("the voted fold absorbed %d bits, want %d — one held, one covered by it, "+
			"the rest unchanged", got, want)
	}
}

// shortenSet abbreviates a set of addresses for a failure message. Display
// only — [Short] is never what anything compares on.
func shortenSet(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for id := range set {
		out = append(out, Short(id))
	}
	slices.Sort(out)
	return out
}

// absorbedBy is every original bit named by every receipt in a view. Across all
// of them, since a split fold leaves several.
func absorbedBy(s *Store, v View) map[string]bool {
	out := map[string]bool{}
	for _, b := range v.Bits(s) {
		if c, cold := b.Payload.(Compaction); cold {
			for id := range c.Absorbed() {
				out[id] = true
			}
		}
	}
	return out
}

// The tier rule, which is the part of this that has to hold when the system has
// more agents in it than people. An agent's vote is tallied and cannot move the
// cut: not to save a bit the human did not save, and not to sink one the human
// kept. It never votes in the same tier, so the count never matters.
//
// Mutation, run: dropping the `who.voter != stay.By` test in [Stay.Holds], so
// that anyone's upvote holds, fails both rows here and also fails
// [TestAStayHoldsExactlyWhatItsVoterUpvotedAndStillHas], which is the only other
// test that puts an agent in the vote view. Both, and nothing else — an earlier
// version of this comment claimed the rest of the file stayed green, which was a
// claim about a check that nobody had re-derived, in a comment about checks.
func TestOnlyTheHumansVoteMovesTheCut(t *testing.T) {
	tyler := Handle{Ref: "tyler", Display: "tyler"}

	// Ten agents against the human, in both directions.
	tests := []struct {
		name  string
		human []Direction // empty for a bit the human never voted on
		agent Direction
		stays bool
	}{
		{"ten agents cannot save what the human ignored", nil, Up, false},
		{"ten agents cannot sink what the human kept", []Direction{Up}, Down, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewStore()

			var shown View
			for i := range 8 {
				shown, _ = shown.Add(s, said(i, "persona", "bit", shown.Head()...))
			}
			contested := shown.Bits(s)[2]

			var votes View
			for _, dir := range tt.human {
				votes, _ = votes.Add(s, Cast(at(8), tyler, dir, contested))
			}
			for i := range 10 {
				votes, _ = votes.Add(s, voted(9+i, "agent-"+string(rune('a'+i)), tt.agent, contested))
			}

			folded, ok := shown.Fold(s, 2, Stay{Votes: votes, By: tyler, For: DefaultHold})
			if !ok {
				t.Fatal("the fold refused")
			}

			absorbed := absorbedBy(s, folded)[contested.ID]
			if absorbed == tt.stays {
				t.Errorf("the contested bit was absorbed=%v, want stays=%v — an agent moved the cut",
					absorbed, tt.stays)
			}
		})
	}
}

// Hold everything back and there is nothing left to cool. [Cool] panics on an
// empty window, so the refusal has to happen here — and a refusal is right
// rather than a failure, because the caller did nothing wrong.
func TestFoldRefusesWhenTheWholeWindowIsHeld(t *testing.T) {
	s := NewStore()
	tyler := Handle{Ref: "tyler", Display: "tyler"}

	var shown View
	for i := range 4 {
		shown, _ = shown.Add(s, said(i, "persona", "bit", shown.Head()...))
	}

	var votes View
	for _, b := range shown.Bits(s)[:2] {
		votes, _ = votes.Add(s, Cast(at(4), tyler, Up, b))
	}

	before := s.Len()
	got, ok := shown.Fold(s, 2, Stay{Votes: votes, By: tyler, For: DefaultHold})
	if ok {
		t.Errorf("folded a window that was entirely held; the view is now %v", shorten(got))
	}
	if !slices.Equal(got, shown) {
		t.Errorf("the view changed to %v on a refused fold, want %v", shorten(got), shorten(shown))
	}
	if s.Len() != before {
		t.Errorf("store grew from %d to %d on a refused fold", before, s.Len())
	}
}

// D14 with votes in it. The transcript never points at a vote, so a reader holds
// two views and the walk starts from both — which is the honest extension, since
// the alternative is to stop asserting that everything is reachable.
//
// Mutation, and it is the one that matters: walking from the transcript alone
// leaves every vote orphaned and this fails naming them. So the test can still
// see the failure it is here for, rather than having been widened until it
// cannot.
func TestVotesInASecondViewLeaveNothingOrphaned(t *testing.T) {
	const coolAt, keepHot = 12, 6

	s := NewStore()
	tyler := Handle{Ref: "tyler", Display: "tyler"}

	var shown, votes View
	for i := range 120 {
		var b Bit
		shown, b = shownAdd(s, shown, i)

		// Vote on every fifth bit, alternating, so some are held and some fold
		// away under a vote that does not save them.
		if i%5 == 0 {
			dir := Up
			if i%10 == 0 {
				dir = Down
			}
			votes, _ = votes.Add(s, Cast(at(i), tyler, dir, b))
		}

		hotEnough := 0
		for _, b := range shown.Bits(s) {
			if hot(b) {
				hotEnough++
			}
		}
		if hotEnough > coolAt {
			shown, _ = shown.Fold(s, keepHot, Stay{Votes: votes, By: tyler, For: DefaultHold})
		}
	}

	found := reachable(t, s, shown, votes)
	if len(found) == s.Len() {
		return
	}

	var orphans []string
	s.mu.RLock()
	for id := range s.bits {
		if !found[id] {
			orphans = append(orphans, Short(id))
		}
	}
	s.mu.RUnlock()
	slices.Sort(orphans)

	t.Errorf("the record holds %d bits and the views reach %d; %d orphaned: %v",
		s.Len(), len(found), len(orphans), orphans)
}

// shownAdd appends one utterance to a transcript and hands back the bit it
// stored, which is what a vote needs to follow.
func shownAdd(s *Store, v View, i int) (View, Bit) {
	return v.Add(s, said(i, "persona", "bit", v.Head()...))
}

// The edge is the tally's structure, so a vote has to be enough to find what it
// was about — from the vote view alone, with no transcript in hand, after the
// target has left the screen. This is the property that makes the target belong
// in Prev rather than in the payload, and it is the one a reachability count
// cannot see: the target stays reachable through the transcript either way.
//
// Mutation, run: building the vote with `Prev: nil` in [Cast], and replacing the
// arity panic in standing() with a `continue` so that a vote naming no target is
// skipped rather than refused — otherwise the panic, not the missing edge, is
// what the run reports. That leaves [TestVotesInASecondViewLeaveNothingOrphaned]
// passing and fails here.
func TestAVoteWalksToWhatItVotedOn(t *testing.T) {
	s := NewStore()
	tyler := Handle{Ref: "tyler", Display: "tyler"}

	var shown View
	for i := range 8 {
		shown, _ = shown.Add(s, said(i, "persona", "bit", shown.Head()...))
	}
	target := shown.Bits(s)[1]

	var votes View
	votes, _ = votes.Add(s, Cast(at(8), tyler, Down, target))

	shown, ok := shown.Fold(s, 2, Stay{Votes: votes, By: tyler, For: DefaultHold})
	if !ok {
		t.Fatal("the fold refused")
	}
	if slices.Contains(shown, target.ID) {
		t.Fatal("the downvoted bit is still on screen; the test is wrong")
	}

	if found := reachable(t, s, votes); !found[target.ID] {
		t.Errorf("no walk from the vote view reaches %s, the bit it voted on", Short(target.ID))
	}
}

// D3's addendum, come due. It proved that a view could hold only one compaction
// and predicted that `slices.ContainsFunc(window, hot)` would one day block a
// legitimate merge of several — and it named the revisit as owed to whoever made
// that state reachable. Splitting makes it reachable: hold a scar back and it
// ends up with scars either side of it, and releasing the hold leaves a run of
// nothing but folds.
//
// The decision, so it is checkable rather than only written down: a run of two or
// more merges, and a run of exactly one is passed through untouched. Merging
// takes rows off a screen for one new object, which is what a fold is for.
// Re-cooling a lone scar takes no rows off anything and costs an object and a hop
// — and refusing it is what keeps a second press of the fold key free, which is
// still true here because merging several leaves one, and one is the case that
// refuses.
//
// The view is built by hand rather than driven to that state through forty sends,
// because the shortest honest path to it is long and the property does not depend
// on the path.
func TestFoldMergesARunOfSeveralScarsButNotOne(t *testing.T) {
	s := NewStore()

	var v View
	for gen := range 3 {
		window := []Bit{
			s.Put(said(gen*2, "persona", "one")),
			s.Put(said(gen*2+1, "persona", "two")),
		}
		v = append(v, s.Put(Cool(window)).ID)
	}
	v, _ = v.Add(s, said(99, "tyler", "still talking"))

	merged, ok := v.Fold(s, 1, Stay{})
	if !ok {
		t.Fatal("a run of three scars refused to merge")
	}
	if got, want := len(merged), 2; got != want {
		t.Fatalf("the view is %v, want %d entries", shorten(merged), want)
	}
	if c := merged.Bits(s)[0].Payload.(Compaction); c.count != 6 {
		t.Errorf("the merged scar stands for %d bits, want 6 — a generation was dropped", c.count)
	}

	// And now the case that must not fold: one scar, alone in its window.
	before := s.Len()
	again, ok := merged.Fold(s, 1, Stay{})
	if ok {
		t.Errorf("re-cooled a lone scar; the view is now %v", shorten(again))
	}
	if s.Len() != before {
		t.Errorf("store grew from %d to %d re-cooling a lone scar", before, s.Len())
	}
}

// D18(e), and the shape the holdout experiment needs: one record, two views,
// folded on different rules at the same moment. Neither view knows about the
// other, neither acquires a lock, and the store is the only thing they share —
// which is the arrangement [TestConcurrentFoldsAgreeWithOneSequentialRun]
// already contends.
//
// What it asserts is that the two disagree about what is on screen and agree
// about what happened. A persona reading the recency view and one reading the
// voted view are being shown different conversations out of one record, and
// that difference is the independent variable the experiment is for.
func TestTwoViewsOverOneRecordFoldOnDifferentRules(t *testing.T) {
	s := NewStore()
	tyler := Handle{Ref: "tyler", Display: "tyler"}

	var written []Bit
	var shown View
	for i := range 10 {
		var b Bit
		shown, b = shownAdd(s, shown, i)
		written = append(written, b)
	}

	var votes View
	votes, _ = votes.Add(s, Cast(at(10), tyler, Up, written[0]))
	votes, _ = votes.Add(s, Cast(at(11), tyler, Up, written[3]))

	// Two holders, two values. The recency view is the same View this one
	// started from, folded on a different rule.
	byRecency, ok := shown.Fold(s, 4, Stay{})
	if !ok {
		t.Fatal("the fold by recency refused")
	}
	byVote, ok := shown.Fold(s, 4, Stay{Votes: votes, By: tyler, For: DefaultHold})
	if !ok {
		t.Fatal("the fold with votes refused")
	}

	if slices.Equal(byRecency, byVote) {
		t.Fatalf("both views are %v; the rule made no difference", shorten(byRecency))
	}
	// The recency view is one scar over six bits and the four never in the
	// window. The voted view is the same six split around two holds and what
	// they cover: the first held bit, with no run in front of it and nothing to
	// cover because a record's first bit names nothing; then bit 1, alone in a
	// run of one and so refused by D32's size rule; then bit 2, which the second
	// hold covers; then the second held bit; then a scar over 4 and 5; then the
	// same four.
	if got, want := len(byVote), 9; got != want {
		t.Errorf("the voted view is %v, want %d entries", shorten(byVote), want)
	}
	if byVote[0] != written[0].ID {
		t.Errorf("the voted view starts with %s, want the held bit %s — a view can now begin hot",
			Short(byVote[0]), Short(written[0].ID))
	}
	for _, id := range []string{written[0].ID, written[3].ID} {
		if !slices.Contains(byVote, id) {
			t.Errorf("the voted view dropped %s, which was upvoted", Short(id))
		}
		if slices.Contains(byRecency, id) {
			t.Errorf("the recency view kept %s, which is older than its cut", Short(id))
		}
	}

	// One record underneath, unchanged by either reading of it.
	found := reachable(t, s, byRecency, byVote, votes)
	if len(found) != s.Len() {
		t.Errorf("the record holds %d bits and the two views together reach %d", s.Len(), len(found))
	}
}

// span is what a row on a screen stands for in time: a scar covers the material
// it absorbed, and anything else covers its own instant.
func span(b Bit) (from, to time.Time) {
	if c, cold := b.Payload.(Compaction); cold {
		return c.From(), c.To()
	}
	return b.At, b.At
}

// The property the split exists to produce, over a long schedule rather than one
// fold: no two rows of a view overlap in time, and they are in order. A held bit
// is never inside the span of a scar it is next to, so a reader can take the
// screen at face value — every scar covers exactly what is under it, and nothing
// that survived is hiding inside one.
//
// This replaces an earlier test asserting that every scar sits ahead of every hot
// bit. That was true of the arrangement this replaced and is deliberately false
// now: a hold at the front of a window leaves a hot bit at index 0 with a scar
// after it. Interleaving is the point, and this is what interleaving has to
// respect.
//
// The schedule holds one bit in three, folds when the view passes eight rows, and
// upvotes the newest scar half the time so that a held [Compaction] is a state
// this actually reaches rather than one it merely permits.
//
// Mutation, run: cooling the window in one piece and putting the survivors after
// it — the shape the brief originally specified — fails here on the first fold
// that holds anything.
func TestASplitFoldLeavesTheViewInTheOrderThingsHappened(t *testing.T) {
	s := NewStore()
	tyler := Handle{Ref: "tyler", Display: "tyler"}

	scars := 0
	var shown, votes View
	for i := range 40 {
		var b Bit
		shown, b = shownAdd(s, shown, i)
		if i%3 == 0 {
			votes, _ = votes.Add(s, Cast(at(i), tyler, Up, b))
		}

		if len(shown) > 8 {
			var folded bool
			shown, folded = shown.Fold(s, 4, Stay{Votes: votes, By: tyler, For: DefaultHold})
			if folded && i%2 == 0 {
				for _, cold := range shown.Bits(s) {
					if !hot(cold) {
						votes, _ = votes.Add(s, Cast(at(i), tyler, Up, cold))
						break
					}
				}
			}
		}

		bits := shown.Bits(s)
		here := 0
		for _, b := range bits {
			if !hot(b) {
				here++
			}
		}
		scars = max(scars, here)

		for j := 1; j < len(bits); j++ {
			_, ends := span(bits[j-1])
			starts, _ := span(bits[j])
			if ends.After(starts) {
				t.Fatalf("after %d sends, row %d ends at %s and row %d starts at %s: the view is %v",
					i+1, j-1, ends, j, starts, shorten(shown))
			}
		}
	}

	// Not an assertion about the design — an assertion that the loop above went
	// anywhere near the case it is about. One scar and no holds would pass every
	// check in it.
	if scars < 2 {
		t.Errorf("the schedule never put more than %d scar in a view; it is not testing the split", scars)
	}
}
