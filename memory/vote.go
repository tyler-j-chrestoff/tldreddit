package memory

import (
	"fmt"
	"time"
)

// Direction is which way a vote points, and it is also what the vote is worth:
// Up counts +1 and Down counts -1, so a tally adds directions rather than
// looking them up in a table that could disagree with them.
//
// There are two directions and the zero value is not one of them. That is
// deliberate: a Vote nobody filled in is refused by [Vote.kind] instead of
// counting as a downvote, which is what a two-valued type would have quietly
// made it.
type Direction int

const (
	Up   Direction = 1
	Down Direction = -1
)

// Vote is one participant's judgment on one other bit: the cheap act D4 makes
// the primary one, and the signal consolidation is supposed to run on.
//
// It carries the direction and nothing else. What it votes on is the bit it
// follows — [Bit].Prev — so a vote is an edge in the record rather than a
// pointer parked beside one. Two things fall out of that and neither is
// incidental. The walk that proves nothing is orphaned (D14) reaches a vote's
// target without being taught what a vote is, because Prev is what it already
// follows. And the tally has a structure instead of an index: the bits that
// point at a bit are its votes.
//
// Nothing about a vote can be revised, here or anywhere. Changing your mind
// casts another one; [Tally] keeps the later of the two, and the record keeps
// both.
//
// Nothing outside this package can write one. The direction is unexported, so
// the only Vote a caller can spell is Vote{}, whose direction is neither Up nor
// Down — and [Vote.kind] refuses to name it, so it cannot be addressed and
// therefore cannot be stored. That is a stronger closure than [Compaction] has,
// where a bare literal is legal and merely says nothing. What it is not is
// absolute: a caller holding a vote out of a stored bit can put that payload
// into a new [Bit] with the wrong Prev, exactly as they could with a compaction,
// and [Tally] is where that shows up.
type Vote struct {
	// dir is which way it points, and it is the whole payload. Unexported so
	// that [Cast] is the only way to make a vote that says anything, because a
	// vote is only meaningful inside a bit whose Prev names what it voted on
	// and a payload cannot check the bit it is in.
	dir Direction
}

// Dir is which way this vote points. There is no third direction — see
// [Vote.kind] for what becomes of one anyway.
func (v Vote) Dir() Direction { return v.dir }

// kind names each direction, which is how the direction reaches the content
// address although [Vote.canonical] never writes it. That is the mechanism
// [Utterance.kind] uses for a fragment, and D26's one-to-one rule is what makes
// it sound.
//
// Utterance qualifies for that rule by arithmetic: one bool, two names.
// Direction does not, and that is the whole reason this panics rather than
// falling through. A defined integer type holds every int, not two, so a default
// branch returning any name at all would put Direction(7) and Direction(8) under
// one tag — two different payloads encoding identically, which is the collision
// D26 exists about. Refusing is what restores the map: over the values that can
// be addressed there are exactly two, and they have exactly two names.
//
// The alternative was a constant "vote" tag with the direction written into
// canonical. It costs two things. [Compaction].Kinds is all a cold bit still
// says about payloads whose content is gone, and under a constant tag it would
// say "vote" — a fold could no longer report which way its window leaned.
// And a direction that cannot be named would then address perfectly well, so
// nonsense would reach the record and [Tally] would have to decide what it
// counts as, with no honest answer available. The cost of this choice is that
// "upvote" and "downvote" are now vocabulary: they reach content addresses, so
// renaming either re-addresses every vote ever cast.
func (v Vote) kind() string {
	switch v.dir {
	case Up:
		return "upvote"
	case Down:
		return "downvote"
	}
	panic(fmt.Sprintf("memory: vote direction %d is neither Up (%d) nor Down (%d)",
		v.dir, Up, Down))
}

// Cast builds and addresses one vote, by who, on the bit they voted on.
//
// It is to [Vote] what [Cool] is to [Compaction], and one step further: the
// direction is unexported, so this is the only way to obtain a vote that names a
// direction at all. The reason it has to be a constructor rather than a literal
// is the edge — a vote only means anything inside a bit whose Prev is exactly
// the one bit it votes on, and a [Bit] cannot state an arity that depends on the
// payload it happens to be carrying. So the constructor states it, and every
// check here is about that edge rather than about the direction.
//
// The one hole left, stated because a claim of closure that is not quite true is
// worse than none: a caller can take a Vote out of a stored bit by type
// assertion and put it into a [Bit] of their own with no Prev at all. That is
// true of every payload in this package, it is a caller deliberately building a
// bit that lies, and [Tally] is where it surfaces.
//
// The channel comes from the target and not from the caller. A vote about a bit
// happened wherever that bit did; letting a caller name a different one would
// let a vote on an internal channel be filed as public, which is the laundering
// [Cool] refuses across a mixed window.
//
// Cast reads no clock, for [Cool]'s reason turned around. Cool has no moment of
// its own so it takes the end of its span; a vote does have one, so it takes it
// from the caller — and because when reaches the address, two processes
// recording the same vote agree on one bit only if they agree on when it
// happened rather than on when they got around to writing it down.
//
// Cast panics on a target that is unaddressed, which includes the zero Bit.
// Either would put an empty string in Prev — an edge to nothing, which surfaces
// much later as [View.Bits] reporting that the store does not hold something.
// That message is this package's alarm for a record that lost a bit, and
// spending it on a caller who passed an unstored bit sends the next reader
// hunting a reachability failure that never happened.
func Cast(when time.Time, by Handle, dir Direction, on Bit) Bit {
	if on.ID == "" {
		panic(fmt.Sprintf("memory: Cast on an unaddressed bit from %q; store it first", on.From.Ref))
	}

	v := Bit{
		At:      when,
		From:    by,
		Channel: on.Channel,
		Payload: Vote{dir: dir},
		Prev:    []string{on.ID},
	}
	v.ID = ID(v)
	return v
}

// Score is what a bit's voters currently say about it: one standing vote each,
// +1 or -1, never a sum.
//
// Per voter rather than merged into one number, because merging is exactly how
// an agent outvotes a human. [View.Fold] reads one voter's entry and the rest of
// the map is a different tier that cannot reach that decision — a priority
// order, which is what D18(d)'s per-participant budget looks like when it is
// expressed as rank instead of as a count. A single int would make the tiers
// unrecoverable by construction, and no later code could put them back.
//
// A voter who has not voted has no entry, and a missing key reads as 0 out of a
// Go map. That is the answer this wants, and it is why nothing here writes a
// zero: not voting is not a stay.
type Score map[Handle]int

// Tally reads a view of votes and reports, for every bit voted on, what each
// voter currently says about it.
//
// One standing vote per voter per target: casting again replaces, so a
// participant who changes their mind is one voter with one opinion rather than
// two votes cancelling out. Both bits stay in the store. That division is the
// one this package makes everywhere — the record keeps every vote ever cast,
// and the view answers what is true now.
//
// "Currently" is decided by [Bit].At, and a tie by position in votes. The tie is
// real rather than theoretical: two votes differing only in direction can carry
// the same instant, while two votes identical in every field are one bit and
// cannot disagree with each other. So the later position wins, which is what a
// view's order already means. The consequence, stated because it is a real
// limit: Tally is a function of the sequence, not of the set, so reordering a
// view whose instants collide can change the winner. Breaking the tie by content
// address instead would be a function of the set, and it would settle "which one
// did they cast second" with "which hash sorts higher" — an answer nobody can
// explain to the voter whose vote it discarded.
//
// Tally panics on a bit that is not a vote, and on a vote that does not name
// exactly one target. The first is the enforcement behind the limit in [Cool]:
// fold a vote view and its head is a [Compaction], so a folded vote view fails
// here, loudly, at the next tally — rather than reporting no votes at all and
// silently lifting every stay in [View.Fold]. The second is the arity [Cast]
// exists to guarantee; a vote with two parents has two targets and no way to say
// which it meant.
func Tally(s *Store, votes View) map[string]Score {
	out := map[string]Score{}
	for who, vote := range standing(s, votes) {
		if out[who.target] == nil {
			out[who.target] = Score{}
		}
		out[who.target][who.voter] = int(vote.Payload.(Vote).dir)
	}
	return out
}

// ballot is one voter's paper on one target: what a standing vote is keyed by.
// Handle is comparable, so this is.
type ballot struct {
	voter  Handle
	target string
}

// standing reduces a vote view to the vote that currently counts for each
// ballot, and hands back the whole bit rather than the direction, because when
// it was cast is the other half of what a caller needs — [Tally] asks what it
// says, [Stay.Holds] asks how old it is, and both have to agree about which vote
// is the standing one.
//
// The rule is one map and one comparison, and [Tally] documents it in full: the
// latest [Bit].At wins, and a tie goes to the later position in votes.
func standing(s *Store, votes View) map[ballot]Bit {
	out := map[ballot]Bit{}
	for _, b := range votes.Bits(s) {
		if _, ok := b.Payload.(Vote); !ok {
			panic(fmt.Sprintf("memory: tallying %s, which carries a %s rather than a vote",
				Short(b.ID), b.Payload.kind()))
		}
		if len(b.Prev) != 1 {
			panic(fmt.Sprintf("memory: vote %s follows %d bits; a vote follows exactly the one it votes on",
				Short(b.ID), len(b.Prev)))
		}

		who := ballot{voter: b.From, target: b.Prev[0]}
		if held, seen := out[who]; seen && b.At.Before(held.At) {
			continue
		}
		out[who] = b
	}
	return out
}
