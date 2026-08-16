// Package memory records what happened.
//
// A [Bit] is the atom: one occurrence, captured with its provenance — who,
// when, where, and what it followed. Bits name their predecessors by ID, so a
// record is a DAG rather than a list: branches diverge and rejoin the way
// commits do. Identity is derived, not assigned — [ID] is a hash of everything
// the bit says, so two agents that independently arrive at the same content
// arrive at the same bit, and a [Store] collapses them without being asked.
//
// The record does not forget; the view does. A [Store] only ever grows, and
// nothing in this package removes anything from one. [Cool] is the
// counterweight, and it works by derivation rather than deletion: it folds a
// window of bits into a single cold bit, which a [View] then shows in the
// window's place. The originals stay addressable forever. That split — the
// record is permanent, the view is free to drop and reorder — is what lets
// consolidation be aggressive without being a loss.
//
// Cooling is lossy on purpose and says so: a [Compaction] carries the count,
// the span, and the IDs of everything it absorbed, so the cost of forgetting
// stays visible and, because the store kept them, walkable.
//
// What decides is a vote, and a [Vote] is a bit like any other. It carries a
// direction and follows the bit it votes on, so what someone thinks of a thing
// is recorded the same way, in the same place and under the same rules as the
// thing itself — including permanence, so changing your mind adds a vote rather
// than editing one. [Tally] reads a view of them, and [View.Fold] holds an
// upvoted bit out of the window it would otherwise have gone into. That is the
// only lever in this package that is not the caller's arithmetic: one
// participant's cheapest possible act, deciding what survives consolidation.
//
// The same act decides what comes first. [View.Rank] is a second reading of one
// view — the same bits ordered by the votes rather than by the clock — and it is
// a separate document rather than a rearrangement, because a transcript has one
// legitimate order and it is time. Two tiers, never summed: the participant the
// caller names decides, and everyone else can only arrange what that participant
// left level.
//
// The record outlives the process. A [Store] writes itself to a stream and
// [ReadStore] builds one back, and the bytes on the wire are the same bytes an
// address is the hash of — so loading is arithmetic rather than trust: every
// bit is re-addressed as it arrives and compared against the address it was
// filed under. A [View] persists too, and separately, because what a view has
// let go of is not a function of the record; it carries the address of the
// record it was taken against so that a view from some other session cannot be
// read over this one.
package memory

import "time"

// Bit is one recorded occurrence. The zero Bit carries nothing and is not
// meaningful; build them with every field you have.
type Bit struct {
	// ID is the bit's content address, as returned by [ID]. It is derived from
	// every other field, so it is an output, not an input: set it by putting
	// the bit in a [Store], and treat any bit you have edited as having no ID
	// until you do. Two bits with the same ID are the same bit.
	ID string

	// At is when it happened. It is the only field that cannot be recovered
	// after the fact, which is why it is always worth capturing at ingest. On a
	// derived bit there is no such moment, so [Cool] uses the end of the span
	// it stands for — see there for why it must not read a clock.
	//
	// The instant is recorded; the zone is not. A bit read back from a stream
	// reads UTC whatever zone it was captured in, because the encoding stores
	// seconds and nanoseconds and identity is the moment rather than where
	// somebody was standing (see canon.at). Anything drawing this to a person
	// is drawing UTC after a restart and has to decide for itself whose local
	// time to show.
	At time.Time

	// From is the handle that produced it, as observed. Read it together with
	// Channel: a Ref is only unique within the channel it was seen on.
	From Handle

	// Channel is where it happened — "tui", "discord/#general", "internal".
	// It is authoritative for the bit. Bits on an internal channel are never
	// sent anywhere; that is the only thing "internal" means here.
	Channel string

	// Payload is what the bit carries.
	Payload Payload

	// Prev holds the IDs of the bits this one follows. One is the common case,
	// none marks a root, and more than one is a join. Order is part of the
	// value and reaches the content address, so a join listing the same
	// parents in a different order is a different bit. Sorting here would be a
	// claim that parent order is meaningless, and nothing yet says it is.
	Prev []string
}

// Handle is an actor as observed: the trace something left on a channel, never
// a person. The same actor may leave many handles, and deciding which handles
// belong to the same actor is a separate, softer question that this package
// deliberately does not answer.
//
// Handle is comparable, so it works as a map key.
type Handle struct {
	// Ref is the stable identifier on the channel: a user id, an address, a
	// session. Unique only within a channel.
	Ref string

	// Display is what the actor called itself at the time. Point-in-time
	// truth — never updated, because the bit it sits in never changes.
	Display string
}

// Payload is what a [Bit] carries.
//
// The set is closed: both methods are unexported, so only this package can add
// a member. Two holes in that, both worth knowing. Go does not check switch
// exhaustiveness — adding a member will not make existing type switches fail to
// compile, so give any switch that must handle every payload a default case.
// And a value method belongs to the pointer type's method set as well, so
// *Utterance satisfies this interface too; [ID] rejects pointer payloads, which
// is enough, because a bit that cannot be addressed cannot be stored.
//
// Both methods are on the interface rather than in a helper so that a new
// payload cannot be added without deciding how it names and hashes itself. A
// payload with no canonical encoding is a payload with no identity, and the
// compiler should be the one that says so.
type Payload interface {
	// kind is the payload's stable name, and it is a property of the value
	// rather than of the Go type: a truncated [Utterance] names itself
	// "fragment". It reaches content addresses, through the canonical encoding
	// and through [Compaction].Kinds, so it is a hand-written literal rather
	// than %T — a printed Go type name carries the package with it, so moving a
	// type would re-address every object that mentioned it, and it cannot see a
	// distinction that lives in a field at all.
	//
	// The constraint that makes value-dependence safe, and it is not optional:
	// a field may reach the address through kind alone only if the value→name
	// map is one-to-one over every value that field can take. Utterance
	// qualifies by arithmetic — one bool, two names — and that is the whole of
	// why its canonical needs no second write.
	//
	// Add a field that kind does not fully distinguish and two different
	// payloads encode identically, [Store] keeps whichever landed first, and
	// Get hands back the other one's content under the right address, silently.
	// A second value-dependent field is the reachable way to do this: were
	// Utterance to grow, say, a Redacted bool that also reported "fragment",
	// {Truncated} and {Truncated, Redacted} would collide. So either give every
	// combination its own name, or write the field in canonical as well and
	// accept that doing so re-addresses every payload of that type ever
	// written.
	kind() string

	// canonical appends this payload's unambiguous byte encoding.
	canonical(*canon)
}

// Utterance is something said: the hot, uncompacted form of a bit's content.
type Utterance struct {
	// Text is what was said, as said.
	Text string

	// Truncated marks an utterance whose speaker did not get to finish — a
	// model that ran out of context room mid-sentence, not one that stopped
	// because it was done.
	//
	// It is not a rendering hint. A reply cut off by a context window is a
	// well-formed sentence that simply ends, and nothing about the text says
	// which kind of ending it was. Recorded unmarked in a store with no delete
	// and no edit, it is a permanent claim that a participant said something
	// they never said, and it is a claim that keeps propagating: every fold
	// that absorbs it inherits the error and no later generation can find its
	// way back past one.
	//
	// So it reaches the content address, through [Utterance.kind]. The same
	// text complete and cut off are two bits, not one.
	Truncated bool
}

// kind names a truncated utterance "fragment", so the distinction outlives the
// bit itself in the two places that matter. It is the tag the canonical
// encoding writes, which is what makes a fragment address apart from the
// complete utterance with the same text. And it is the key [Compaction] tallies
// in Kinds, which is how a fold reports on its own — holding nothing but the
// tally — that a fragment was in the window it absorbed. A bool could not have
// done that second job: it lives on the payload, and the tally is what a
// Compaction carries in the payload's place.
//
// Nothing is destroyed by the fold, and this comment must not be read as
// saying so. The absorbed bits stay in the [Store] with Truncated intact, and
// [Compaction].Absorbed names every one of them, so a reader with the store can
// walk back and read the fragment itself. Kinds is the answer to the narrower
// question — what does the cold bit alone say — which is the question a screen
// drawing a scar is asking.
//
// Collapsing this back to a constant loses both at once, and loses them
// quietly: every fragment takes the address of its complete twin, and [Store]
// keeps whichever content landed there first.
func (u Utterance) kind() string {
	if u.Truncated {
		return "fragment"
	}
	return "utterance"
}
