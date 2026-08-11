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
	// kind is the payload's stable name. It reaches content addresses, through
	// the canonical encoding and through [Compaction].Kinds, so it is a
	// hand-written literal rather than %T: a printed Go type name carries the
	// package with it, and moving a type would re-address every object that
	// mentioned it.
	kind() string

	// canonical appends this payload's unambiguous byte encoding.
	canonical(*canon)
}

// Utterance is something said: the hot, uncompacted form of a bit's content.
type Utterance struct {
	Text string
}

func (Utterance) kind() string { return "utterance" }
