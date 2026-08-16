package memory

import (
	"fmt"
	"iter"
	"time"
)

// View is what a reader is currently shown: an ordered window of content
// addresses into a [Store].
//
// A View is where forgetting is allowed to happen. It drops bits, replaces
// runs of them with a summary, and will one day reorder them — all of which is
// safe only because the Store it points into never drops anything, so every ID
// a View lets go of still resolves.
//
// It is a slice of IDs and not a slice of bits on purpose. A view that held
// bits would be a second copy of the record, and the moment two copies exist
// one of them starts being the real one. Holding addresses keeps it honest:
// the view can only ever show what the store already has.
type View []string

// Add files b in the store and appends it to the view, returning both the
// extended view and the stored bit with its ID set.
//
// The two steps are one method because separating them is how the record loses
// something: a bit shown but never stored is a bit that disappears at the next
// fold with nothing left to resolve.
func (v View) Add(s *Store, b Bit) (View, Bit) {
	b = s.Put(b)

	// Capped so the append always allocates. Without the cap, adding to a view
	// could write into spare capacity another copy of that view is still
	// reading, and views get copied constantly — every Bubble Tea update
	// passes one by value.
	return append(v[:len(v):len(v)], b.ID), b
}

// Head is the Prev for the next bit written after this view: the last bit
// shown, or nothing at all if the view is empty, which is how a first bit
// becomes a root.
func (v View) Head() []string {
	if len(v) == 0 {
		return nil
	}
	return []string{v[len(v)-1]}
}

// Bits resolves the view against the store, in view order.
//
// It panics on an ID the store does not hold. That is an invariant, not input
// validation: a view built by [View.Add] and [View.Fold] can only name bits
// that were stored first, so an unresolvable ID means the view and the store
// came from different records, and rendering it as a gap would hide that.
func (v View) Bits(s *Store) []Bit {
	out := make([]Bit, 0, len(v))
	for _, id := range v {
		b, ok := s.Get(id)
		if !ok {
			panic(fmt.Sprintf("memory: view names %s, which the store does not hold", Short(id)))
		}
		out = append(out, b)
	}
	return out
}

// Latest is the record's own present, as this view sees it: the newest instant
// any bit in it carries, and the zero instant for an empty view.
//
// It exists because a hold has to decay against something, and every other
// candidate is worse. A wall clock would make [View.Fold] impure — the same view
// and the same votes would fold differently depending on when you asked, and
// determinism is what lets two processes and one replay agree about a record.
// Counting rows would not survive a fold, which replaces many rows with one and
// would resurrect a hold that had already aged out. The conversation's own
// newest instant only moves forward, moves for the same reason the view needs
// folding at all, and is a fact about the record rather than about the machine
// reading it.
//
// What this buys is stated plainly in [Stay].For, because it is a real cost: age
// is measured in conversation time, so a view whose bits arrive quickly ages
// holds slowly in rows.
func (v View) Latest(s *Store) time.Time {
	var newest time.Time
	for _, b := range v.Bits(s) {
		if b.At.After(newest) {
			newest = b.At
		}
	}
	return newest
}

// Stay is the right to be held out of a fold: which votes to read, whose vote
// decides, and how long a vote holds for.
//
// Whose is a parameter because this package cannot know. A [Handle] is a trace
// on a channel and nothing in the record marks one of them as the human — the
// only thing that could is the program that ran the conversation, so that is
// what says so. A constant here would be this package making a claim about who
// is using the software, inside the thing that is supposed to be evidence of
// what happened.
//
// The zero Stay grants nothing: no votes, so nothing is held and [View.Fold]
// sees its window as one unbroken run. That is what to pass where there are no
// votes yet, and it is what makes the vote additive rather than a second kind of
// fold to keep in step with the first.
//
// A Stay is a value holding a value, and that is load-bearing under D18(e): two
// views over one record, folding on different rules, are two callers each
// holding their own [View] and their own Stay. Nothing here is shared, so
// nothing here needs a lock — which is the only reason a View can go on having
// none.
type Stay struct {
	// Votes is a view of vote bits, the second view over one record. It is
	// never folded; see [Tally] for what happens if it is.
	//
	// Its order is part of its meaning, not an artifact of how it was built.
	// Two votes by one voter on one target at the same instant are settled by
	// which comes later here, so a vote view rebuilt in a different order can
	// hold different things back. That matters most where it is least visible:
	// under D18(b) this view has to survive a restart, and rebuilding it from
	// anything unordered — a map, a set, a query with no ORDER BY — silently
	// changes what consolidation keeps, with every vote still present and
	// correct.
	//
	// That is why a vote view is persisted as a view and never rebuilt by
	// scanning the record for votes: [View.WriteAgainst] writes the order out
	// and [ReadViewAgainst] reads it back, while a scan of the store would have
	// to invent one, and a map has none to invent from.
	Votes View

	// By is the one voter whose upvote holds a bit back. Everyone else's votes
	// are tallied and cannot move this cut.
	By Handle

	// For is how long one upvote holds its bit, measured in the conversation's
	// own time: a hold survives while [View.Latest] is less than For past the
	// vote that granted it, and voting again renews it from the new instant.
	//
	// A hold decays because a score that does not is a pin rather than a rank,
	// and because permanent holds only accumulate. Measured: with a permanent
	// hold on every second bit, two hundred sends leave two hundred rows and not
	// one successful fold, because no run of two unheld bits survives anywhere
	// for [View.Fold] to cool. It fails hardest exactly when the human
	// participates most, which is the wrong way round for D4. The vote itself is
	// untouched by any of this — it is a bit, it is permanent, and what expires
	// is only the stay of execution it bought.
	//
	// Conversation time, not rows, is the denominator that survives a fold and
	// only ever moves forward — see [View.Latest] for why the alternatives do
	// not. The cost is that a fast talker ages holds slowly in rows, so a
	// surface with a quick model wants a smaller For than one with a person
	// typing. [DefaultHold] is a starting point measured against one bit a
	// minute, which is what this package's own fixtures do; it is not a
	// measurement of anybody's real conversation.
	//
	// A Stay with votes and no For is refused rather than guessed at, because
	// zero could as easily mean "hold nothing" as "hold forever" and those are
	// opposites. A caller wanting the old permanence says so with a century.
	For time.Duration
}

// DefaultHold is how long one upvote holds a bit, absent a reason to choose
// otherwise: thirty minutes of conversation time.
//
// Picked by measurement rather than taste, against a fixture writing one bit a
// minute over two hundred sends, folding at twelve foldable hot bits and keeping
// six. Across vote rates from one bit in two to one in twenty-five, the worst
// view it ever produced was 31 rows — about one screen, which is the criterion.
// Ten minutes bounds harder (18 rows) and starts expiring holds on material a
// person would still call recent; an hour reaches 61 rows, which is two screens
// of things somebody has to scroll past. Without decay at all, the same schedule
// at one in two reaches 200 rows and cannot fold at all, because a hold every
// other bit leaves no run of two anywhere for [View.Fold] to cool.
//
// It is a starting point, not a measurement of anybody's conversation: the
// denominator is conversation time, so a surface whose model answers in seconds
// is packing many more rows into thirty minutes than this fixture does. See
// [Stay].For.
const DefaultHold = 30 * time.Minute

// Holds is what this stay is currently holding back, and how much longer each
// one has. The map is the caller's, built fresh on every call, and a bit that is
// not held is simply absent rather than present with nothing left.
//
// Exported, and used by [View.Fold] itself, for two reasons. A caller needs the
// same answer for a different question — *when* to fold — because a held bit is
// hot until it expires, so a trigger that counts hot bits stops falling back
// under its threshold and fires on every write, absorbing a little less each
// time; what such a caller wants to count is the hot bits a fold could actually
// absorb, and **this map is no longer the whole of that answer**: a hold also
// covers the bit its own bit names through Prev ([sparing]), so a bit absent
// from here can still be one the fold will refuse, and a trigger built on this
// alone overcounts by up to one bit per hold. [View.Absorbing] is the exact set
// and costs one traversal. And a hold that ends with
// nothing on screen having said it was ending is the machine acting behind the
// human's back, which is the one thing this surface exists not to do — so the
// time remaining is handed out, not just the yes or no, and a screen can draw it
// draining.
//
// Both readings come from one traversal of the same vote view, so the strength a
// screen draws is the strength the next fold will act on. Restating the rule at
// the call site is how a screen comes to promise one cut while the fold makes
// another.
//
// The order of Votes decides this, not just its contents — see [Stay].Votes.
// Rebuild that view in a different order and this map can come back different
// with every vote still present.
func (stay Stay) Holds(s *Store, now time.Time) map[string]time.Duration {
	if len(stay.Votes) > 0 && stay.For <= 0 {
		panic(fmt.Sprintf("memory: a stay of %d votes with no lifetime; a hold decays, so For must say how long",
			len(stay.Votes)))
	}

	out := map[string]time.Duration{}
	for who, vote := range standing(s, stay.Votes) {
		// Strictly Up, and strictly one voter. A downvote is not a negative
		// hold, it is the absence of one — testing for "not Down" would hold
		// back every bit this voter ever touched, and reading anyone else's vote
		// here is what would let an agent outvote the human.
		if who.voter != stay.By || vote.Payload.(Vote).Dir() != Up {
			continue
		}
		if left := vote.At.Add(stay.For).Sub(now); left > 0 {
			out[who.target] = left
		}
	}
	return out
}

// Fold cools everything but the last keep bits, and returns the view that shows
// the cold bits in their place.
//
// What gets cooled is every run of two or more adjacent bits in that window that
// no vote is sparing. Two or more, because one bit cooled is one row replaced by
// one row that says less, for a new object and a hop — that is a cost with no
// saving, whatever the bit was. Everything else in the window is passed through
// untouched: a run of one, every bit a vote is holding, and the bit each of
// those holds names through Prev — [sparing] is the whole of that rule, and
// what Prev does and does not say is stated there too.
//
// So the second return is false whenever no run reached two — the window is
// shorter than two bits, or every bit in it is spared, or what is spared has cut
// it into single bits — and the view comes back exactly as it went in. The useful
// consequence, and it is a guarantee rather than a tendency: a fold that returns
// true has always made the view shorter, because the only thing it ever does is
// replace two or more rows with one. A caller can therefore fold on a loop
// without checking whether it is making progress.
//
// Nothing is removed by this. A cold bit goes into the store alongside the bits
// it absorbed, all of which stay addressable, so the scar the surface draws is
// one a reader can follow back to what was folded rather than one they can only
// read about.
//
// A bit stay.By has voted up is held out and stays hot, for as long as
// stay.For — see [Stay.Holds], which decides it, and which a surface can read to
// draw a hold draining before it goes. That is D4 cashed: the human is a
// participant whose cheapest act decides what survives consolidation, and this
// is the one place in the package where a vote does anything.
//
// **A hold covers the bit its own bit names through Prev, and that bit is not
// thereby held.** [sparing] carries the measurement that forced it, what Prev
// is and is not evidence of, and the reason the two categories may never be
// merged: a vote is a fact about a person and a cover is a fact about a fold.
//
// **A hold splits the fold rather than being lifted out of it.** Each contiguous
// run of unspared bits is cooled on its own, so N holds leave up to N+1 cold bits
// and every spared bit keeps its true position between them. The reason is the
// receipt, and it is not a matter of taste: [Cool] derives a compaction's span
// from the window it was given, so one scar prepended to the whole window would
// span 09:31–09:50 and sit directly above a survivor stamped 09:47 that it never
// absorbed. Nothing about that is false on its face, which is what makes it the
// bad kind of wrong — a reader has to already know the rule to know it is not
// being told what it looks like. Split, every span covers exactly what is under
// it, and the view stays in the order things happened. The cost is more scars on
// a screen, which is a screen's problem and can be solved on a screen.
//
// Only stay.By's votes reach this decision, and that is the design rather than a
// simplification. Everyone else's votes are tallied and read by nobody here:
// they cannot hold a bit back and they cannot push one out. An agent therefore
// never outvotes the human, because it is never voting in the same tier —
// D18(d)'s per-participant budget expressed as priority instead of as a count,
// which is the form D24 argues for, since a ceiling stops an agent voting a
// million times and a tier makes the millionth vote worth nothing. The second
// tier orders nothing today because there is nothing here to order: survivors
// keep the order they arrived in, and how to sort one transcript is not a
// question anyone has asked. The place it becomes one is a list of threads
// (D18(c)), and this is not that.
//
// What a fold is about to take can be asked for without taking it, by
// [View.Absorbing], and a surface that draws the cut before it happens should
// ask rather than work it out. Both read the same traversal, which is the only
// reason the drawing and the taking cannot disagree.
//
// Fold panics on a negative keep, because keeping fewer than no bits has no
// meaning and the caller's arithmetic has to be reported as the caller's.
// Unrefused, what happens today is an index panic inside [View.runs], which
// states why that is the floor: loud, but naming a function this caller never
// called.
//
// It used to be worse, and the older mechanism is kept here because it is one
// edit from returning rather than because it is what the code does now: while
// the window was sliced off the [View] rather than off the resolved bits, v[:cut]
// read the spare capacity behind the view instead of refusing, and the empty IDs
// it picked up surfaced later as [View.Bits] reporting that the store does not
// hold something. That message is this package's alarm for a record that lost a
// bit, and spending it on a caller's arithmetic sends the next debugger hunting a
// reachability bug that never happened.
//
// It also panics, through [Stay.Holds], on a stay carrying votes with no
// lifetime — now on every call rather than only where the window had something
// in it, because the traversal reads the holds before it can know whether it has
// anything to fold. A stay that cannot be honoured is a mistake in the caller's
// configuration, and finding it on the first fold beats finding it on the first
// fold that would have done something.
//
// Two invariants a stay ends, both recorded because code outside this package
// is resting on them. D3's addendum has it that under Add and Fold alone a view
// holds at most one [Compaction] and it is always at index 0, proved over 45,000
// view states. Splitting ends both halves: a view can hold many scars, and it
// can begin with a hot bit, because a held bit at the front of the window has no
// run in front of it to cool. Neither "the fold is v[0]" nor "the hot band
// starts at the first non-fold" survives that, and both are written down
// elsewhere in this repository.
func (v View) Fold(s *Store, keep int, stay Stay) (View, bool) {
	if keep < 0 {
		panic(fmt.Sprintf("memory: Fold keeping %d bits", keep))
	}

	next := make(View, 0, len(v))
	folded := false

	// Every stretch of the view comes back from runs, cooled or not, so this
	// rebuilds the whole thing and never has to know where the window ended.
	for run, cool := range v.runs(s, keep, stay) {
		if !cool {
			for _, b := range run {
				next = append(next, b.ID)
			}
			continue
		}
		cold := s.Put(Cool(run))
		next = append(next, cold.ID)
		folded = true
	}

	// Nothing was cooled: no run in the window reached two bits. Hand back the
	// view that came in rather than the identical one just rebuilt, so a refused
	// fold is visibly a no-op.
	if !folded {
		return v, false
	}

	// next is its own array, which matters for [View.Add]'s reason: a view is a
	// value that gets copied constantly, and an append landing in spare capacity
	// writes into a copy somebody else is still reading.
	return next, true
}

// Absorbing is which of v's bits a fold at this keep and stay would cool — the
// material a surface can draw as going, before it goes. The map is the caller's,
// built fresh on every call, keyed by content address; a bit that survives the
// fold is simply absent rather than present and false.
//
// It exists because the cut is a set and not a prefix. A hold splits the window
// — twice, since it covers the bit it names through Prev as well as itself — so
// what a fold takes is scattered through the view with survivors standing
// between the pieces, and the index a screen used to keep cannot say that. Nor
// can a screen find out by folding a copy and looking: [View.Fold] puts a
// [Compaction] in the store for every run it cools, so asking that way makes the
// record grow because something was drawn — and a record that grows for what a
// reader looked at is no longer a record of what happened.
//
// Mints nothing, stores nothing, reads no clock. It is one traversal of the same
// rule [View.Fold] runs on, sharing the keep window, the split at every bit
// [sparing] hands back, and the size rule that leaves a run of one alone. That
// last is the one worth naming, because it is the case a call site gets wrong: a
// lone bit between two holds looks exactly like material about to go and is not,
// so a screen that dims it has promised a fold that will then refuse. It is also
// where a caller most easily reads a cover as a vote: a bit absent from here
// carries no ballot of its own, and [Stay.Holds] is the only thing that knows
// which bits do. [Stay.Holds]
// states the general form — restating the rule at the call site is how a screen
// comes to promise one cut while the fold makes another — and this is that
// argument one step out, where the rule being restated is the fold itself.
//
// Keyed by address, which is worth stating because a [View] is a list of
// addresses and nothing stops one naming the same address twice. Such a view
// gets one answer for both positions, and the direction it errs in is the safe
// one: a bit reported here that the fold then leaves alone is a row drawn
// cooling that stays, while the promise a surface makes — nothing goes without
// having been drawn going first — survives untouched.
//
// Absorbing panics on a negative keep, for [View.Fold]'s reason and in its own
// name, since a panic naming a method the caller did not call is a panic
// pointing at somebody else's arithmetic.
func (v View) Absorbing(s *Store, keep int, stay Stay) map[string]bool {
	if keep < 0 {
		panic(fmt.Sprintf("memory: Absorbing keeping %d bits", keep))
	}

	out := map[string]bool{}
	for run, cool := range v.runs(s, keep, stay) {
		if !cool {
			continue
		}
		for _, b := range run {
			out[b.ID] = true
		}
	}
	return out
}

// Sparing is which of v's bits a hold is keeping out of a fold: the ones this
// stay holds, and the one each of those names through Prev. The map is the
// caller's, built fresh on every call; a bit that is not spared is absent rather
// than present and false.
//
// It exists because [Stay.Holds] stopped being the whole answer to "what can a
// fold take" and said so in its own doc without offering the rest of it. A hold
// covers the bit its own bit names through Prev ([sparing]), so a caller
// counting hot bits that no vote holds overcounts by up to one per hold — and a
// trigger built on
// that count never falls back under its threshold once a few holds land, which
// is the fold storm [Stay.Holds] describes and the thing that count exists to
// prevent. This is the set that closes it.
//
// # What it is not, and both halves have caught somebody
//
// **It is not the complement of [View.Absorbing].** A bit can be in neither: the
// kept tail is not spared and is not taken, and neither is a lone unspared bit
// the size rule refuses. Absorbing is a question about one fold at one keep;
// this is a question about the holds alone, which is why it takes no keep.
//
// **Membership here is not evidence of a ballot.** Note which way that runs:
// this set is a *superset* of [Stay.Holds], not a disjoint companion to it, so
// every held bit is in here and every one of those does carry a ballot. What
// [Stay.Holds] alone knows is which bits somebody voted *on*, and it goes on
// answering that about ballots alone. What this adds to them is a bit the fold
// will pass over because of *somebody else's* vote, and a surface that drew one
// of those as a vote would be reporting a ballot nobody cast — subtract the
// holds to get them, which is what a caller drawing the two states apart has to
// do. See [sparing], where the same line is drawn from the other side.
//
// Narrowed to v's own bits, unlike the unexported rule underneath it, which is
// free to name a held bit's Prev whether or not the view still holds it. A
// method on a view that named something outside the view would be handing a
// caller an address it cannot draw and cannot fold.
//
// now is read from [View.Latest], once, exactly as [View.runs] reads it, so a
// caller asking this and then folding gets one answer to a question that has
// one. It panics through [Stay.Holds] on a stay carrying votes with no lifetime,
// for [View.Fold]'s reason and on every call rather than only where there is
// something to spare.
func (v View) Sparing(s *Store, stay Stay) map[string]bool {
	bits := v.Bits(s)
	all := sparing(bits, stay.Holds(s, v.Latest(s)))

	out := make(map[string]bool, len(all))
	for _, b := range bits {
		if all[b.ID] {
			out[b.ID] = true
		}
	}
	return out
}

// runs cuts the view into the stretches a fold at this keep and stay would act
// on, in view order, saying of each whether the fold cools it.
//
// It is the whole rule in one traversal — the keep window, the split at every
// bit [sparing] hands back, and the size rule that leaves a run of one alone —
// and neither caller knows any of it. [View.Fold] cools what it is handed and
// [View.Absorbing] names it. That is the entire point of the arrangement: two
// statements of this rule agree on the day they are written and are one edit
// from a screen promising a cut the fold does not make.
//
// Everything that passes through comes back too, marked false: a spared bit as a
// stretch of its own, a lone unspared bit as itself, and the kept tail as one
// last stretch. So a caller can rebuild the whole view from these and never has
// to know where the window ended, which is the second thing that would otherwise
// be stated twice.
//
// No stretch is ever empty, including on an empty view, and that is a promise
// rather than an accident of the arithmetic: [Cool] panics on an empty window,
// so a caller is entitled to cool whatever it is handed without first checking
// there is something in it.
//
// now is read here, once, from [View.Latest], for both callers. A hold near
// expiry decides differently on either side of that instant, and two readings of
// it are two answers to a question that has one.
//
// keep must not be negative, and both callers refuse that in their own name
// before they get here. Nothing checks it again: this walks bits already
// resolved out of the store, whose length is exact, so an over-long cut indexes
// out of range loudly rather than reading spare capacity quietly. That is a
// property of resolving first and slicing second, not a guard, and it is why the
// window is never cut off the [View] itself.
func (v View) runs(s *Store, keep int, stay Stay) iter.Seq2[[]Bit, bool] {
	return func(yield func([]Bit, bool) bool) {
		bits := v.Bits(s)
		spared := sparing(bits, stay.Holds(s, v.Latest(s)))
		cut := len(bits) - keep

		for i := 0; i < cut; {
			if spared[bits[i].ID] {
				// Spared: it passes through where it happened and splits the runs
				// either side of it, rather than being lifted out from between
				// them. [View.Fold] carries the reason, and it is about the
				// receipt rather than the screen.
				//
				// Capped, as every slice handed out of this package is, so that
				// a caller appending to a stretch cannot write over the one
				// after it.
				if !yield(bits[i:i+1:i+1], false) {
					return
				}
				i++
				continue
			}

			j := i
			for j < cut && !spared[bits[j].ID] {
				j++
			}

			// The size rule, and the only place it is written: two or more is a
			// fold, one is not. One bit cooled is one row replaced by one row
			// that says less, for a new object and a hop in the walk back to it
			// — a cost with no saving, whatever the bit was, and the same
			// whether it is hot or already a scar (D32). Two or more is a real
			// fold whichever bits are in it, which is what lets several
			// compactions merge and what keeps folding idempotent: merging two
			// leaves one, and one is the case that refuses.
			run := bits[i:j:j]
			if !yield(run, len(run) > 1) {
				return
			}
			i = j
		}

		// The kept tail, and everything else when keep is longer than the view.
		//
		// Guarded on the tail's own length rather than on the cut, which is not
		// the same test on an empty view: there the cut is negative, so a cut
		// short of the end is true and the stretch handed out has nothing in it.
		// Both callers today survive that by luck — the fold appends no IDs and
		// [View.Absorbing] names no bits — so it would have arrived as [Cool]
		// panicking on an empty window in whatever called runs third.
		if tail := bits[max(cut, 0):]; len(tail) > 0 {
			yield(tail, false)
		}
	}
}

// sparing is every bit a fold passes through untouched: the ones this stay is
// holding, and — the reason this is not simply the hold map — the bits each hold
// names through Prev.
//
// A vote lands where a person's eye already is, and a hold that covered only the
// bit it was cast on — which is what this was until the measurement below —
// cooled the row above it and left the kept row standing between two scars,
// stripped of the context it was read in. That was the one gesture the product
// rests on making its own screen worse.
//
// # What Prev is, and where it stops meaning what it looks like it means
//
// Prev is positional. Every writer in this repository fills it from [View.Head]
// — the last row of the view at the moment a bit was written — and nothing
// anywhere marks a bit as a reply to another one. In an interactive session
// where a person types and a model answers, the head *is* the turn just spoken,
// so "the row above" and "the turn this replies to" name the same bit and the
// stronger reading costs nothing.
//
// They come apart the moment anything writes outside that alternation, and this
// is measured rather than imagined: 7 of the 29 said bits on this project's own
// record — 24% — were written from a shell against whatever view was on disk.
// One of them is a correction whose Prev is a greeting rather than the claim it
// corrects, because that greeting was the head when the correction was typed.
// The greeting is a model's, and it is on the same subject — which is the sharp
// version of the point rather than a softening of it: Prev did not land somewhere
// obviously wrong, it landed on a plausible neighbour, and a reader with no other
// evidence would take it for the thing being answered.
//
// So what this rule spares is **local context, and not an answer's question**.
// That is still worth sparing: a kept row alone above a scar reads as half of
// something whether or not the missing half was a question, which is what the
// measurement below is about. It is less than an earlier wording here claimed,
// and the difference matters because the claim was checkable and nobody had
// checked it.
//
// The schedule, because a measurement without one is D36(a)'s defect: 400 bits
// through the surface's own trigger, fold and cut, one bit every 3.5 seconds of
// conversation time, a two-minute hold, a 23-row budget, **one upvote every rate
// bits on the bit just added**. A frame counts as stranded when some bit still on
// screen is held and hot and names a bit through Prev that the view no longer
// holds. Before this rule: 93.5% of frames at one vote in ten, 92.8% at one in
// five, 91.2% at one in three, 22.5% at one in two, 0% with nobody voting.
// After: 0.0% at every one of them.
//
// **The clause in bold is load-bearing, and for a session nothing treated it as
// though it were.** A bit just added carries the head of the view as its Prev,
// and the head of the view is in the view — so the one case this rule cannot
// reach, a parent already folded away by the time the vote lands, cannot occur
// in that experiment. A 0.0% taken there is what the schedule entails, not what
// the rule achieves. Move the vote back and the parent is behind a scar before
// there is anything to spare: at the same budget and cut, an upvote twelve said
// rows above the newest strands 83.0% of frames at one vote in three and 78.8%
// at one in ten, and the onset tracks the keep rather than the rate. Every
// figure in both paragraphs is a row of tui/testdata/stranding.txt, which the
// commit gate re-derives; every "before" figure reproduces there exactly under
// the mutation that deletes this rule.
//
// What the residue is *not* is a D1 or D14 failure, and that is measured in the
// same table: in every schedule swept, a stranded row has the fold that took its
// parent on the row directly above it. The receipt is adjacent and one key away.
//
// So a hold reaches one step back along Prev — the record's own account of what
// came before a bit, rather than a guess made from where rows happen to sit on a
// screen. One step and no further: reaching transitively would walk the whole
// chain back to the root, which is every bit there has ever been.
//
// **Only a hot hold reaches back at all.** A [Compaction]'s Prev is every bit in
// the window it absorbed (D13), so an upvote on a scar would cover a whole
// generation rather than one row, and a fold that spares a generation is a
// fold that stops taking anything. That state is not reachable through [View.Add]
// and [View.Fold] alone today — the bits a scar names are the bits it replaced,
// so they are not in the view to be spared — which makes this guard a prior
// rather than a live protection, and the cost of being wrong about it is the
// whole fold rather than one row. [TestAHeldScarSparesOnlyItself] holds it up
// against a view built by hand.
//
// **A bit spared this way is not held, and nothing here says it is.** [Stay.Holds]
// answers "what did this voter's ballots keep" and goes on answering it about
// ballots alone, so a fade, a gauge or a footer reading it reports a vote that
// was cast and never one that was inferred. What moves is what a fold *takes*,
// which is [View.Absorbing]'s question, and a surface asking that gets the true
// answer — this row is not going — without being told a vote exists that does
// not.
//
// The cost is paid in hold density, and it is the thing to watch: every hold now
// spares two rows where it spared one, so any threshold about how much voting a
// fold survives moves. On the schedule above it is not visible — folds over 400
// bits go 28 → 25 at one vote in ten and 23 → 30 at one in three, mean rows
// 22.7 → 25.1 and 33.9 → 38.5, worst view 30 → 34 and 43 → 47 — because a
// two-minute hold decays inside the record and the pairs it kept cool together
// when it lapses. Hold the same conversation with a thirty-minute hold and the
// cost is a cliff rather than a slope: one vote in three folded 16 times and now
// folds not once, because every free stretch between two covers is a single bit
// and D32's size rule refuses all of them. The rate at which this record stops
// consolidating was one vote in two and is now one in three.
//
// **That 16 is at 400 bits and a 23-row budget — the schedule two paragraphs up
// — and it needs both, because folding runs about linearly with length, so the
// same rule folds 16 times at 200 bits and a 12-row budget as well.** One number
// reachable two ways spent two sessions being attributed to either, until the
// sweep swept the length (tui/testdata/stranding.txt, and D59(c)). The lesson is
// narrower than "state the schedule": a fold count needs *every* parameter its
// value is sensitive to, and the ones that scale it are the ones nobody thinks
// to write down.
//
// **A thirty-minute hold does not "never expire" — it outlives a conversation of
// about 515 bits at this cadence, which is 1,800 seconds of conversation at 3.5
// seconds a bit, and that is a fact about the conversation.** Past it the oldest
// votes start lapsing and folding resumes: at one vote in three, 0 folds at 200
// and 400 bits at every budget swept, and 95 at 800. What the flat part costs
// meanwhile is the view itself, which reaches 517 rows before the first hold
// goes. So the record does not stop consolidating; it stops until the votes
// holding it do, and a figure quoted from the flat part reads as permanence.
//
// **The six "after" figures in that paragraph were 37, 122, 24.5, 34.4, 30 and
// 37 when it was written, and nothing anywhere noticed them go stale.** Each was
// exactly right against the tree that measured it. What moved underneath them is
// in another package: `tui.foldable` counted hot-and-unheld then and counts
// hot-and-unspared now, so the trigger it drives fired up to one bit early per
// hold on the day these were taken. Every "before" figure in this comment still
// reproduces to the decimal, which is how the cause was found rather than
// guessed. The generalisation is the reason all of it is written down: a figure
// about a fold is a figure about every rule the fold runs on, and only one of
// those rules lives in this file.
func sparing(bits []Bit, held map[string]time.Duration) map[string]bool {
	out := make(map[string]bool, 2*len(held))
	for _, b := range bits {
		if _, hold := held[b.ID]; !hold {
			continue
		}
		out[b.ID] = true
		if !hot(b) {
			continue
		}
		for _, id := range b.Prev {
			out[id] = true
		}
	}
	return out
}

// hot reports whether b is an original rather than a fold of other bits.
func hot(b Bit) bool {
	_, cold := b.Payload.(Compaction)
	return !cold
}
