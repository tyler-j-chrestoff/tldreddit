package memory

import (
	"cmp"
	"slices"
)

// Ranked is one bit's place in a ranked reading of a view, carrying the two
// numbers that decided it rather than the one they would collapse into.
//
// Two and not a total, for [Score]'s reason: a sum is how an agent outvotes a
// human. [View.Rank] reads them in order and never adds them, so the second
// number can only arrange bits the first one left level with each other. A
// caller that adds them has undone the tier, and no later code can put it back.
type Ranked struct {
	// ID is the bit, at the position it held in the view that was ranked. A
	// [View] may name one address twice, and such a view yields two entries
	// here rather than one: this orders the rows a reader was shown, not the
	// distinct bits behind them.
	ID string

	// Own is what the ranking participant currently says about this bit: +1,
	// -1, or 0 for a bit they have not voted on. An int rather than a
	// [Direction] because zero is deliberately not a direction, and "no
	// opinion" is a third answer the ordering has to be able to hold.
	Own int

	// Others is every other voter's standing vote on this bit, summed. Merging
	// them is safe here and only here: they sit entirely below Own, so no total
	// this can reach moves a bit past one the participant ranked themselves.
	Others int
}

// Rank is a second way to read v: the same bits, in the order the votes put them
// in rather than the order they happened in.
//
// It reorders nothing and returns no [View]. The transcript has one legitimate
// order and it is time (D30); a ranked reading is a different document about the
// same record, the way a forum's "top" is a different page from its "new" rather
// than a rearrangement of it. Handing back a View would invite a caller to show
// this as the transcript, and a transcript out of time order is a record that
// lies about what followed what.
//
// **The order is two tiers and they never mix.** by's own standing vote decides
// first — upvoted above untouched above downvoted, whatever anyone else said —
// and only within one of those three bands does the sum of everyone else's votes
// arrange anything. That is D18(d)'s per-participant budget expressed as
// priority rather than as a count, which is the form D24 argues for: a ceiling
// stops an agent voting a million times, and a tier makes the millionth vote
// worth nothing. It is the rule [Stay.Holds] already runs on, one step out —
// there the second tier could not move the cut, here it cannot move the order.
//
// What it reads is the standing votes cast *on each bit*, never a total carried
// by a handle across bits. A score attached to a participant rather than to a
// claim is karma, and karma ranks tenure: it converts length of chain into
// standing and prices out whoever arrived late (D45(h)). This reads
// [standing] directly rather than [Tally]'s output for the same reason
// [Stay.Holds] does — one traversal, so a screen drawing a vote, a fold
// honouring it and a ranking ordering by it cannot reach three different
// answers about which vote is the standing one.
//
// **A tie keeps view order, and ties are the common case.** With no votes at all
// every bit is level and Rank hands back exactly the view it was given, in its
// own order. That is the property worth having: the ordering is checkable by
// eye, because the only rows that moved are the ones somebody voted on. Both
// alternatives are worse in the way [Tally] already describes about its own tie.
// Content address settles "which of these did the room stand behind" with "which
// hash sorts higher", and nobody can explain that to the voter it demoted.
// Recency within a tier is a second ranking rule nobody has decided, and it
// would move rows nobody voted on.
//
// The cost is the one [Tally] states about itself: Rank is a function of the
// sequence and not of the set. Reorder v and the ties come back in the new
// order, and rebuild votes in a different order and an instant tie between two
// votes can resolve the other way. Both are the same fact — order is part of a
// view's meaning here rather than an artifact of how it was built.
//
// **Nothing decays.** A hold decays because permanent holds accumulate until no
// run of two unheld bits survives anywhere and the view stops folding at all,
// which is measured (see [Stay].For). A rank has no such failure: the ordering
// is total however old the votes are, and nothing is being spent. Decay here
// would instead mean the bit a person marked an hour ago quietly leaving the top
// of the reading they opened to find it. So Rank takes no lifetime, reads no
// clock, and is pure in [View.Fold]'s sense — the same view and the same votes
// rank the same way whenever you ask.
//
// **A scar ranks on the votes cast on it and inherits none.** A [Compaction] is
// a bit like any other here, so a fold that absorbs an upvoted bit carries that
// bit's standing out of this ordering with it — the votes are still in the
// record, on a row the view no longer shows. That is the caller's to avoid
// rather than this function's to paper over: rank the view you have, and a
// caller who wants the whole conversation ranked keeps a view that was never
// folded. D13 makes the other answer reachable, since a cold bit's Prev is every
// bit in the window, but inheriting a vote would print a number beside a row
// nobody cast it on. That is a decision, and it has not been made.
//
// A zero [Handle] for by is legal and means the first tier is empty, so the
// whole ordering is the second one. That is the agent-only forum, and D24 is
// what it produces.
//
// Rank panics, through the traversal it shares with [Tally], on a view that is
// not votes and on a vote that does not name exactly one target — including a
// folded vote view, for the reason [Tally] gives.
func (v View) Rank(s *Store, votes View, by Handle) []Ranked {
	own := map[string]int{}
	others := map[string]int{}
	for who, vote := range standing(s, votes) {
		// One entry per ballot, so by's branch assigns at most once per target
		// and the other branch sums over a set. Neither depends on the order
		// this map hands its keys out in, which is what keeps Rank pure: a Go
		// map's iteration order is deliberately not stable, and anything that
		// reached the result through it would rank differently run to run.
		dir := int(vote.Payload.(Vote).dir)
		if who.voter == by {
			own[who.target] = dir
			continue
		}
		others[who.target] += dir
	}

	// Built by walking v, so the input to the sort is already in view order and
	// the sort only has to be stable to leave a tie where it found it.
	out := make([]Ranked, 0, len(v))
	for _, id := range v {
		out = append(out, Ranked{ID: id, Own: own[id], Others: others[id]})
	}

	slices.SortStableFunc(out, func(a, b Ranked) int {
		// Descending, hence b before a. The early return is the tier: Others is
		// never consulted where Own has already decided, which is the whole of
		// what stops a second-tier voter overturning a first-tier one.
		if tier := cmp.Compare(b.Own, a.Own); tier != 0 {
			return tier
		}
		return cmp.Compare(b.Others, a.Others)
	})
	return out
}
