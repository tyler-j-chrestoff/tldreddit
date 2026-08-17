package main

import (
	"cmp"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"github.com/tyler-j-chrestoff/tldreddit/memory"
	"github.com/tyler-j-chrestoff/tldreddit/tui"
)

// One file, and what that buys is a state it makes unrepresentable.
//
// Three streams go into it, in this order: the record, the view of what was on
// screen, the view of the votes. [memory.Store.WriteTo] and
// [memory.View.WriteAgainst] are self-delimiting and say so, which is what lets
// them concatenate. Three files would admit a state one file cannot — a record
// present and a view absent — and [memory.View.WriteAgainst] states exactly what
// that costs on the second view: an absent vote view lifts every hold at once,
// silently, and the fold that follows takes material somebody voted to keep. So a
// half-written checkpoint is not something this program recovers from well; it is
// something it cannot produce.
//
// The other half of that promise is [atomically]. A file the save path can
// destroy halfway through is an append-only record that its own writer can
// shorten, which is this product's thesis failing inside its own binary.
//
// There is a second way a writer shortens a record and it is not a partial
// write: another writer. A save is the whole file, so two of them over one path
// mean the later one's record is the only record — and this program ships two,
// the session and `tldr say`. [record.absorb] is what closes that, and it can be
// this cheap only because the store is content-addressed: the union of two
// records is [memory.Store.Put] in a loop, with no conflict expressible.
//
// It is written after every change and not at quit, which is [checkpoint] and
// [tui.Save]. Saving at quit made the promise above conditional on a clean exit
// and said so nowhere: a crash, an OOM kill or a closed terminal took the entire
// session with no receipt, while [atomically] stood guard over the previous
// record and nothing at all guarded the current one. The whole conversation
// living in one process's memory until that process chose to end is the failure
// this program is about, committed by the program.
//
// Together those two close the arrangement [memory.StaleView] was written for —
// a checkpoint that wrote the record and died before the views. It is not gone,
// because a hand-assembled file or a backup restored out of two snapshots can
// still spell one; it is no longer something this program can do to itself, which
// is why a stale view is refused here rather than recovered from.
//
// Reading is all or nothing, and the only tolerated absence is the whole file: no
// record yet means a session that starts empty, and everything else is fatal
// before the surface is drawn. Starting with a record and no views is precisely
// the silent hold-lift above, so the choice is not between refusing and carrying
// on — it is between saying so and doing it quietly. A person moving a bad file
// aside deliberately is the better outcome, and they can only do that if they are
// told which file.

// record is one saved session: the store, and the two views over it that the
// surface holds. It is the unit that gets written and read, rather than three
// things a caller keeps in step by hand.
type record struct {
	store *memory.Store

	// shown is the transcript's view and votes is the vote view, in the order
	// they sit in the file. Both may be empty; neither may be missing.
	shown, votes memory.View
}

// The two views' names, as a person reading an error should meet them. They are
// constants because each one appears in several messages and a view named two
// ways in two errors is a person diagnosing two problems.
const (
	shownName = "the view of what was on screen"
	votesName = "the view of the votes"
)

// recordPath is where the conversation lives.
//
// $TLDR_RECORD first, and an environment variable rather than a flag on purpose:
// tui's defaultPersona rests on the path that opens the surface taking no flags
// — the verbs in cli.go have their own, and each one belongs to the verb it
// follows rather than to the session — and a scratch record for a demo capture
// or a test is a thing to point the program at from outside rather than a choice
// to put on the surface.
//
// Then the XDG state directory, which is where this belongs: a conversation is
// state a program regenerates for nobody — not a cache, not config, and not data
// a user manages. An empty XDG_STATE_HOME is treated as unset, which is the
// specification's own rule and here also avoids a measured trap: filepath.Join
// drops the empty element and cleans the result, so obeying an empty one
// literally files the record at the *relative* path "tldreddit/record".
//
// A missing home is an error for the same reason. A record in whatever directory
// the program happened to start in is how somebody ends up with several and no
// way to tell which is theirs.
func recordPath() (string, error) {
	if p := os.Getenv("TLDR_RECORD"); p != "" {
		return p, nil
	}
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		return filepath.Join(dir, "tldreddit", "record"), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("no $TLDR_RECORD and nowhere to fall back to: %w", err)
	}
	return filepath.Join(home, ".local", "state", "tldreddit", "record"), nil
}

// load reads the record at path, and reports a missing file as an empty record
// rather than as a failure.
//
// That one absence is tolerated because it is the first run, and it is the only
// one: a file that exists and does not parse is refused with the path in the
// message. Every error out of here already names what failed; this adds where.
func load(path string) (record, error) {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return record{store: memory.NewStore()}, nil
	}
	if err != nil {
		return record{}, err
	}
	defer f.Close()

	rec, err := decode(f)
	if err != nil {
		return record{}, fmt.Errorf("%s: %w", path, err)
	}
	return rec.rejoin(), nil
}

// rejoin puts back into the views everything the record holds that neither of
// them reaches.
//
// # The thing it fixes, which was one command with two outcomes
//
// [record.absorb] takes in the store and never the views, deliberately, so a
// session that was open while something else said a thing goes on drawing the
// screen it had. What that left unsaid is where the other writer's bit then
// lives. The session's checkpoint writes the session's own [record.shown] over
// whatever `tldr say` had put there, so the bit ended up in the store and in *no
// view* — not until the next session, permanently. With no session open the
// identical command left it in the transcript, and the next session opened with
// it on screen, inside the fold window and inside the persona's context.
//
// So one command had two outcomes, selected by whether a terminal happened to be
// open somewhere else on the machine — invisible to whoever ran it and invisible
// to whoever reads the result. The bit stayed *enumerable* the whole time,
// through `tldr top` and the ranked surface, both of which walk
// [memory.Store.All] rather than a view. Enumerable is not what D14 counts: an
// enumeration hands a reader every bit and no starting point, so the bit had
// genuinely stopped being reachable in D14's sense, permanently, and the
// transcript disagreeing with itself about one act was the second fault rather
// than the only one.
//
// # Every writer of a view strands, not only `tldr say`
//
// The first version of this rescued utterances and nothing else, on the ground
// that nothing else could strand. Two sessions falsify that with no exotic file
// involved. The one that saves second writes *both* of its views over both of
// the other's, so a ballot the other cast and a scar the other's fold minted
// each land in the store named by nothing — and nothing later points at either,
// because the edges run backwards: a vote's Prev names the bit it votes on, and
// a fold puts its scar where the run it replaced was, which is behind the head,
// so even the next thing said points past it.
//
// # Why here, and not in absorb
//
// A save may not write a view the surface does not hold: [tui.Save]'s whole
// sentence is that the file matches memory after every change, and a save path
// that merged the file's view into what it writes would make that false in a way
// no test on the surface could see. Reading is where the two arrangements can be
// reconciled without lying to either.
//
// It is also the more general statement. This is a property of a record rather
// than an agreement between the two writers this program happens to ship, so a
// third writer, or a file somebody assembled by hand, gets it without anything
// here learning that they exist.
//
// # What accounts for a bit
//
// D14's own walk, run rather than restated: out from both views, following Prev
// and Absorbed as far as they go. What it does not reach is a stray.
//
// The shallower rule this used to apply — named by the transcript, or absorbed
// by a scar in the transcript — is right about utterances and wrong as soon as
// scars are looked for. A scar beneath another scar is named by the outer one's
// Prev, which D13 makes the whole window in window order, and by nobody's
// Absorbed: memory/cool.go merges the inner generation's *originals* into the
// outer list and drops the scar between them. Under the old rule it would read
// as a stray and be put back underneath the receipt that already stands for it.
//
// # Where each kind goes, and why one rule would not do
//
// An utterance goes into the transcript, which is the old rule unchanged.
//
// A ballot goes into the vote view, the only view that can hold one:
// [memory.Tally] and [memory.View.Rank] panic on a vote view carrying anything
// that is not a vote, and on a vote naming other than exactly one target. A
// stray vote of any other arity is therefore left where it is. No writer this
// program ships can mint one — [memory.Cast] is the only way to make a vote and
// it states the arity — so what would arrive here is a hand-assembled file, and
// trading a stranded bit for a panic on that reader's first frame is the worse
// of the two.
//
// A fold receipt goes into the transcript too, and it is the one kind that can
// be refused. A view never holds both a scar and a bit that scar names — the
// fold's own sparing rule rests on it — and a receipt beside the material it
// summarises would print one conversation twice into the bargain. So a receipt
// goes back only where the transcript names nothing it stands for, which is
// exactly the case where its material would otherwise return as strays and
// un-fold a fold somebody else performed. Receipts are *offered* their places
// before the utterances are, for that reason, and everything under one counts as
// accounted for the moment it goes back, so the two never both appear. Offered
// and not placed: where a receipt lands is decided by its instant like every
// other stray, and the two kinds interleave.
//
// Where the transcript *does* already show that material, the receipt stays
// stranded and this does not close it. The two ways out are both worse than the
// hole: rewriting a transcript a previous session arrived at, against the rule
// below that a load may insert and may not rearrange, or drawing the summary
// beside the thing summarised. Written down in docs/DEBT.md rather than
// papered over.
//
// # Where a stray lands
//
// Where it was said. The strays are ordered by their own instant and merged into
// the view rather than appended to it, and both halves are load-bearing.
// Appending would put an hour-old note below everything said since — and
// [tui.Load] lands the caret on the last row, so the first thing a returning
// reader's vote key aimed at would be the oldest of the strays. Merging also
// tends to restore the edge: `tldr say` writes Prev as the file's head at that
// instant, and the session checkpoints after every change, so the bit it named is
// usually the row this puts it under. Usually and not always — a fold on the
// session's side can have eaten that row, and then the stray sits under the scar
// that took it, which is the honest place for it.
//
// The vote view takes the same merge, and there the order means something
// narrower: [memory.Tally] settles two votes on one ballot by their instants and
// falls back to position only when those are equal, where the *later* position is
// the one that counts. Merging by instant therefore leaves every standing vote
// where it already stood, and hands an exact tie to the vote being put back,
// because [merge] emits every row not later than a stray before the stray itself.
//
// That is worth spelling out where the merge order is justified, because the
// sentence after this one sounds like it settles the question and settles the
// opposite. **Keeping a row's place is not keeping its standing.** The vote
// already in the view keeps its position, the position it keeps is the earlier
// one, and the earlier one is the loser. The two readings come apart only where
// one voter votes twice on one bit in opposite directions inside a nanosecond,
// which [memory.Cast] cannot produce because it reads the clock — so what reaches
// this is a hand-assembled file or a fixture, and a row of
// [TestALoadPutsAStrayBackWhereItWasSaidAndMovesNothingElse] is that fixture.
//
// It is this way round because [merge]'s rule is positional and both views take
// the same one. Putting a stray before an equal instant instead would hand the
// tie to the incumbent, and would mean the transcript and the vote view were
// merged by two different rules — a harder thing to explain than either outcome,
// and refused for that rather than for the outcome. docs/DEBT.md carries it.
//
// A bit the view already named is never moved. The arrangement a previous session
// arrived at is that session's, and nothing here has grounds to rewrite it; the
// merge only ever inserts.
//
// What this does not do is reach a session that is already running. Nothing does,
// and `tldr say` says so at more length: a view is a value that process holds.
func (r record) rejoin() record {
	reached := map[string]bool{}
	r.reaching(reached, r.shown...)
	r.reaching(reached, r.votes...)

	var cold, said, cast []memory.Bit
	for b := range r.store.All() {
		if reached[b.ID] {
			continue
		}
		// Three cases and no default, because [memory.Payload] is sealed by an
		// unexported method and these are all of it. A fourth kind would arrive
		// here as a bit this leaves alone, which is the state it is in already.
		switch b.Payload.(type) {
		case memory.Compaction:
			cold = append(cold, b)
		case memory.Utterance:
			said = append(said, b)
		case memory.Vote:
			cast = append(cast, b)
		}
	}

	outermost(cold)

	drawn := make(map[string]bool, len(r.shown))
	for _, id := range r.shown {
		drawn[id] = true
	}

	var receipts []memory.Bit
	for _, b := range cold {
		if reached[b.ID] || summarises(drawn, b) {
			continue
		}
		receipts = append(receipts, b)

		// Two marks rather than one, and they answer different questions.
		//
		// reaching accounts for everything beneath this receipt — from this one
		// bit rather than from scratch — which is what keeps its own material out
		// of the strays below.
		//
		// drawn is the narrower half, and it is the second statement of what
		// [outermost] arranges: a receipt offered later that stands for this one
		// has to find it already on the screen, or both go back and the view holds
		// a scar beside a bit that scar names. Reaching the case needs an offer
		// order [outermost] cannot arrange, which is the window-of-one record its
		// own doc names; deleting this line reddens exactly the row of
		// [TestALoadPutsAStrayBackWhereItWasSaidAndMovesNothingElse] that builds
		// one.
		drawn[b.ID] = true
		r.reaching(reached, b.ID)
	}

	rows := slices.DeleteFunc(said, func(b memory.Bit) bool { return reached[b.ID] })
	rows = append(rows, receipts...)
	inOrder(rows)
	r.shown = merge(r.store, r.shown, rows)

	// The reached half is a prior rather than a live guard, and is marked as one:
	// the loop above skipped every bit already reached, and the only thing that
	// has joined that set since is the material under a receipt this just put
	// back. A receipt covers a vote only where something folded a vote view, which
	// [memory.Cool] tells callers never to do — so deleting it reddens nothing
	// here today. It stays because "accounted for" is one set and asking it once
	// per kind is how that stays true when a fourth kind arrives.
	cast = slices.DeleteFunc(cast, func(b memory.Bit) bool {
		return reached[b.ID] || len(b.Prev) != 1
	})
	inOrder(cast)
	r.votes = merge(r.store, r.votes, cast)

	return r
}

// reaching adds to seen every bit discoverable from the addresses given,
// following Prev and Absorbed for as far as they go. It is D14's walk, and the
// same one memory/reach_test.go runs to assert the property this maintains.
//
// A worklist rather than recursion, because the depth is the length of a Prev
// chain and that is the length of the conversation.
//
// An address the store does not hold stops the walk rather than failing it.
// [record.check] refuses a file whose views name a bit the record does not hold
// and runs inside [decode], which is the only way a file reaches this; an edge
// that dangles past that is a record that lost a bit, which is a fault to report
// where it is detected rather than to rediscover here.
//
// Following Absorbed as well as Prev cannot change the answer under D13, and
// that is worth stating rather than leaving as an unexercised branch: a scar's
// Prev is every bit in its window in window order, so walking Prev alone already
// descends through the inner scars to the originals Absorbed names directly.
// Measured — deleting the Absorbed branch reddens nothing in the module.
// It stays because this is D14's walk rather than an optimisation of it, and
// because the redundancy is a property of D13 rather than of this function; the
// claim that pins D13 is `prev-is-the-previous-folds-prev`, and the day it goes
// this walk is still right.
func (r record) reaching(seen map[string]bool, from ...string) {
	pending := slices.Clone(from)
	for len(pending) > 0 {
		id := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if seen[id] {
			continue
		}
		b, ok := r.store.Get(id)
		if !ok {
			continue
		}
		seen[id] = true

		pending = append(pending, b.Prev...)
		if c, cold := b.Payload.(memory.Compaction); cold {
			pending = slices.AppendSeq(pending, c.Absorbed())
		}
	}
}

// summarises reports whether a view already names something this receipt stands
// for, which is the condition that makes it unplaceable.
//
// It asks whether the view *names* something beneath this receipt, and never
// whether some other receipt stands for the same material. That second question
// is not free to leave out: two
// sessions at different terminal heights fold the same transcript with different
// windows, whichever saves last keeps its own scar, and the other's is then put
// back beside it — so the same bits are summarised twice, adjacently, and written
// back on the next checkpoint. Refusing it instead would strand it, which trades
// a legibility fault for the D14 fault this whole mechanism closes, so the
// residual is named in docs/DEBT.md with its construction rather than fixed here.
//
// Both edges, because they answer different halves. Absorbed names every
// original at any depth; Prev names the window this fold took, which is where
// the scars of earlier generations are.
//
// The Prev half is the one that sees a window holding nothing but an older
// receipt, and that is not a case the fold produces: D32 refuses a run of one, so
// a fold over nothing but older scars needs two of them adjacent, which takes a
// hold splitting a window in a particular place. What does produce it is a
// [memory.Cool] called by hand over a window of one, and the row of
// [TestALoadPutsAStrayBackWhereItWasSaidAndMovesNothingElse] that builds one is
// what reddens when this half is deleted. It was a prior until that row existed,
// and the reason to keep it was already the right one: a scar stands for its
// window, which is what D13 says Prev is, and an Absorbed-only version would be
// right by accident.
func summarises(drawn map[string]bool, scar memory.Bit) bool {
	for _, id := range scar.Prev {
		if drawn[id] {
			return true
		}
	}
	c, cold := scar.Payload.(memory.Compaction)
	if !cold {
		return false
	}
	for id := range c.Absorbed() {
		if drawn[id] {
			return true
		}
	}
	return false
}

// outermost sorts receipts into the order they can be offered a place in: a fold
// before any fold it absorbed.
//
// The other order is the thing this exists to prevent. Offer an inner scar first
// and it goes back on screen, and the outer one is then refused for naming it —
// so the generation standing for the whole conversation is the one that strands,
// permanently, and the transcript comes back one fold shallower than the record.
//
// Newest first is not that order, which is why this is not [inOrder] reversed. A
// scar's At is the *end* of the span it covers, so an outer fold whose window
// ends on the inner one carries the same instant to the nanosecond. That is not
// exotic: it is what the surface produces whenever a hold splits one fold into
// two runs and a later fold takes both. An instant order then falls through to
// the content address, and which generation survives is settled by a hash —
// measured over twenty records differing only in what their bits said, fourteen
// stranded the outer receipt.
//
// [memory.Compaction.Count] is what does order them, because it is the one field
// that grows strictly with nesting: [memory.Cool] merges an absorbed compaction's
// own count rather than counting it as one bit, and a fold's window is never a
// single bit (D32's size rule, in memory/view.go's runs), so a receipt standing
// for another stands for at least one bit more than it does.
//
// That argument is about the fold and not about the file. A record somebody
// assembled by hand can hold a [memory.Cool] over a window of one, where the two
// counts are equal and the address decides again. What such a file gets is a
// receipt in the wrong place — never two receipts standing for one thing, which
// [record.rejoin]'s drawn map holds separately — and the row of
// [TestALoadPutsAStrayBackWhereItWasSaidAndMovesNothingElse] that builds one is
// where that is stated rather than assumed.
//
// The instant stays the first key, and it cannot contradict the count: an outer
// fold's span contains the inner one's, so its At is never the earlier of the
// two. It is a prior rather than a measured rule, marked as one — deleting it
// reddens nothing in the module, because it decides the order only between two
// receipts where neither stands for the other, and there both go back whatever
// order they arrive in (the duplicate-summary residual in docs/DEBT.md). It is
// kept because the day that residual is closed by refusing one of them, the rule
// that picks the survivor has to be "the newer receipt" and not "the higher
// hash", which is the same argument the paragraphs above make about a pair
// nesting.
func outermost(receipts []memory.Bit) {
	slices.SortFunc(receipts, func(a, b memory.Bit) int {
		return cmp.Or(
			b.At.Compare(a.At),
			cmp.Compare(standsFor(b), standsFor(a)),
			cmp.Compare(b.ID, a.ID),
		)
	})
}

// standsFor is how many original bits a receipt stands for, and 0 for a bit that
// is not a receipt at all.
//
// [record.rejoin] sorts only bits it has already sorted into the cold pile, so
// the second case is unreachable from the one caller. It is written out rather
// than asserted because a panic on the load path is the wrong answer to a bit
// that is merely in the wrong list, and because 0 is the honest ordering for
// something that stands for nothing.
func standsFor(b memory.Bit) int {
	c, cold := b.Payload.(memory.Compaction)
	if !cold {
		return 0
	}
	return c.Count()
}

// inOrder sorts strays into the order they happened in.
//
// [memory.Store.All] hands bits back in address order and says in its own doc
// that this is not a reading order, so the instant is the sort key and the
// address is only the tiebreak — present so that two loads of one file produce
// one view, not because an address means anything about when.
func inOrder(bits []memory.Bit) {
	slices.SortFunc(bits, func(a, b memory.Bit) int {
		return cmp.Or(a.At.Compare(b.At), cmp.Compare(a.ID, b.ID))
	})
}

// merge inserts strays into a view by instant: each goes before the first row
// later than it, so a row sharing its instant keeps its place and the stray lands
// after it. strays must already be [inOrder].
//
// Keeping a place is not winning, and the distinction is the trap: in the vote
// view the standing vote is the *later* position ([memory.Tally]), so the row
// that keeps its place on an exact tie is the row that loses the ballot.
// [record.rejoin] carries why that is the right way round.
//
// It resolves the view against the store to read those instants, which cannot
// panic along the path that matters for [memory.View.Bits]'s reason: [record.check]
// has already refused a file whose views name bits the record does not hold.
func merge(s *memory.Store, v memory.View, strays []memory.Bit) memory.View {
	if len(strays) == 0 {
		return v
	}

	rows := v.Bits(s)
	out := make(memory.View, 0, len(v)+len(strays))
	i := 0
	for _, b := range strays {
		for i < len(rows) && !rows[i].At.After(b.At) {
			out = append(out, v[i])
			i++
		}
		out = append(out, b.ID)
	}
	return append(out, v[i:]...)
}

// save writes the record to path, having first taken in whatever is already
// there.
//
// The taking-in is [record.absorb] and it is not an optimisation. A save writes
// the whole file, so without it the last writer's record is the only record —
// and this program has two writers by design: a session saving after every
// change, and `tldr say` putting a bit on the record from outside one. Either
// order loses the other's bits outright, not merely off the screen, which is D1
// failing inside the one binary that argues for it.
//
// It cannot fail the way a merge normally does, and that is the whole reason it
// is available. The record is content-addressed and only grows, so two writers
// cannot produce contents that disagree: the union is [memory.Store.Put] in a
// loop, and a bit both of them hold collapses to one entry by construction. There
// is nothing to reconcile and no format to invent.
//
// What it does not do is make the file safe to write from two processes at once.
// The window it leaves is between the read below and the rename at the end of
// [place] — milliseconds, against a whole session before — so this narrows a
// certain loss into an unlikely one and does not close it. Closing it means a
// lock, and a lock file that outlives a killed process locks a person out of
// their own conversation, which is a worse failure than the one being fixed.
func (r record) save(path string) error {
	if err := r.absorb(path); err != nil {
		return fmt.Errorf("reading %s before replacing it: %w", path, err)
	}
	return atomically(path, r.encode)
}

// absorb files every bit the record on disk holds and this one does not.
//
// The store only, never the views. That asymmetry is the decision, and it is
// D1's sentence exactly: **a view is allowed to forget; the record is not.** A
// session that was open while something else said a thing goes on drawing the
// screen it had, and the thing that was said is on the record — reachable by
// `tldr top`, which reads the store rather than a view. Merging the views instead
// would put a row on a screen the person is looking at without them asking, and
// would have to decide where in their transcript it goes, which nothing here can
// answer.
//
// The next session is where that row does belong, and this is not what puts it
// there. It used to say it was, and it was wrong: a save writes the session's own
// view over whatever was in the file, so a bit taken in here reached the store
// and no view at all — the next session's transcript included. [record.rejoin]
// is the half that was missing, on the reading side where the two arrangements
// can be reconciled without either of them being overwritten.
//
// The store is the pointer the caller handed in, so this grows the caller's own
// record and not a copy. That is what makes a second save cheap and what makes
// the surface's bit count tick up when the record grows underneath it, which is
// the honest thing for it to do.
//
// A file that will not parse stops the save rather than being overwritten.
// Replacing bytes this build cannot read is the largest possible way to forget,
// and the alternative reading — that a session should always be able to write —
// is served by the failure being loud and recoverable: [tui.Save] keeps the
// session running on a failed save, so a person can move the bad file aside and
// the next change carries everything. The one absence that is not a failure is
// the whole file, which is the first run.
func (r record) absorb(path string) error {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()

	// Only the record is read back, so the views in the file are not decoded and
	// their state cannot block this. That matters: the views on disk belong to
	// whoever wrote last, and a save that refused because somebody else's view
	// was stale would be this program's own checkpoint held hostage by a file it
	// is about to replace.
	on, err := memory.ReadStore(f)
	if err != nil {
		return err
	}
	for b := range on.All() {
		// Put re-derives the address and panics on a bit whose label and content
		// disagree. [memory.ReadStore] has already checked exactly that, so this
		// is the second statement of one rule rather than the first — and it is
		// the statement that survives if the reader's own check is ever relaxed.
		r.store.Put(b)
	}
	return nil
}

// checkpoint is [record.save] bound to one file, in the shape [tui.Load] takes:
// the surface hands back its two views and this puts the whole thing on disk.
//
// The store is not a parameter and does not need to be — it is the pointer this
// record already holds and the one the surface was given, so every bit written
// since the last call is in it. [tui.Save] argues that at more length.
//
// The receiver is a value, so the views this closure sets are its own. That
// matters because the caller goes on using its copy after the program ends: two
// paths to one file, and neither can leave the other holding views it did not
// write.
//
// # What this costs, which is named rather than fixed
//
// A save is the whole file and there is one per change, so the bytes written
// over a session are quadratic in its length. Measured on this machine
// 2026-08-13, over a fixture of plain utterances with no votes and no folds —
// so the byte figures are a floor for a real session, which carries both:
//
//	 bits │    bytes │ per save │ with the read
//	   12 │    4,421 │   1.3 ms │        1.6 ms
//	  100 │   34,253 │   2.0 ms │        2.7 ms
//	  343 │  116,873 │   3.8 ms │        6.1 ms
//	1,000 │  340,253 │   8.6 ms │       16.3 ms
//
// The last column is [record.absorb], added 2026-08-14 and measured the same
// way: a save now reads the file back before replacing it, which roughly
// doubles the variable part and leaves the shape of the curve alone. That is
// the price of two writers not erasing each other, and it is paid per change
// rather than per bit.
//
// About 1.3 ms of each of those is fixed — create, fsync, rename — and the rest
// is the encoding, at roughly 22 ns a byte. A bit lands when a person presses
// enter and when a model answers: the live run behind the demo page averaged one
// every 3.5 seconds over 343 of them, so a save there is a tenth of a percent of
// the gap between two bits, against replies that took 26 seconds. There is
// nothing to feel, and no reason to make this a tea.Cmd — see [tui.Save] for
// what that would cost instead.
//
// The shape is the part worth keeping rather than the millisecond. What matters
// is not today's figure but where the curve stops being free, and the fix when
// it does is not an append-only file: the wire format is whole-record by design
// — [memory.Store.WriteTo] sorts by address so one record is one file byte for
// byte, and a view is sealed against the record's address as a whole — so append
// mode would be a second format, and a second statement of a format is exactly
// what that package refuses to have.
//
// Re-check: a throwaway test that builds a record of n bits, calls save twenty
// times and divides. It is fifteen lines and it does not live here, because a
// measurement kept as a permanent instrument is an instrument nobody asked for.
// The cheap standing version is `wc -c` on the record beside `store.Len()`.
func (r record) checkpoint(path string) tui.Save {
	return func(shown, votes memory.View) error {
		r.shown, r.votes = shown, votes
		return r.save(path)
	}
}

// decode reads the three streams in the order [record.encode] wrote them.
//
// One reader throughout: each stream ends where its own frame says it does and
// reads not one byte further, so the next decode starts where the last stopped.
// That is [memory.Store.WriteTo]'s closing tag and [memory.View.WriteAgainst]'s
// length doing work here, not an assumption about sizes.
func decode(r io.Reader) (record, error) {
	store, err := memory.ReadStore(r)
	if err != nil {
		return record{}, fmt.Errorf("the record: %w", err)
	}

	shown, err := memory.ReadViewAgainst(store, r)
	if err != nil {
		return record{}, viewError(shownName, err)
	}
	votes, err := memory.ReadViewAgainst(store, r)
	if err != nil {
		return record{}, viewError(votesName, err)
	}

	rec := record{store: store, shown: shown, votes: votes}
	if err := rec.check(); err != nil {
		return record{}, err
	}
	return rec, nil
}

// encode writes the three streams. The order is the format: nothing in the file
// says which view is which, because the two are structurally identical and a
// label would be a second thing to keep in step with the order it labels.
func (r record) encode(w io.Writer) error {
	if _, err := r.store.WriteTo(w); err != nil {
		return fmt.Errorf("the record: %w", err)
	}
	if _, err := r.shown.WriteAgainst(r.store, w); err != nil {
		return fmt.Errorf("%s: %w", shownName, err)
	}
	if _, err := r.votes.WriteAgainst(r.store, w); err != nil {
		return fmt.Errorf("%s: %w", votesName, err)
	}
	return nil
}

// viewError says which view failed, and spells a stale one out in full.
//
// [memory.StaleView] is the one failure here whose message a person has to act
// on rather than read — it means this file belongs to another conversation — and
// [memory.StaleView.Error] abbreviates both addresses for a screen. Abbreviated
// is the wrong length for the thing somebody does next, which is comparing them
// against another file, so both go out whole.
//
// The view it carries is deliberately dropped. It exists so a caller can recover
// one on purpose, and there is nothing to recover into here: the record loads or
// the session does not start, and a view from another conversation resolved
// against this one is the stale render [memory.ReadViewAgainst] exists to refuse.
func viewError(name string, err error) error {
	var stale *memory.StaleView
	if errors.As(err, &stale) {
		return fmt.Errorf("%s belongs to a different record\n  it was written against %s\n  this file's record is  %s",
			name, stale.Against, stale.Record)
	}
	return fmt.Errorf("%s: %w", name, err)
}

// check refuses a record whose views name bits it does not hold, or whose vote
// view holds something that is not a vote.
//
// Both are conditions [memory] already enforces by panicking —
// [memory.View.Bits] on the first, [memory.Tally] on the second — and stating
// them again here buys exactly one thing, which is worth being precise about: an
// error naming the file instead of a panic out of a library, on the first frame,
// after the surface has taken the terminal. Neither is reachable from a file this
// program wrote, and the seal on each view means neither is reachable by
// corruption either; what reaches here is a file somebody assembled.
//
// The cost is that this is a second statement of somebody else's rule and can go
// stale in the strict direction: widen what a vote view may legally hold and this
// refuses a legitimate file first, with a message that sounds authoritative.
//
// # The line, which was drawn deliberately and asked about once
//
// Every rule here restates a condition [memory] already enforces by panicking,
// and nothing else. That is what keeps the set principled: each refusal fires
// exactly where the surface could not be drawn at all, so refusing is strictly
// better than the alternative rather than a matter of taste.
//
// It was proposed that this also refuse a file holding a wrong-arity vote in the
// *store* — the bit [record.rejoin] leaves stranded rather than trade for a panic
// — on the same reasoning as a file that will not parse. Declined. Such a record
// draws perfectly; the whole cost of the fault is one ballot missing from the
// vote view; and refusing it denies a person their entire conversation over a bit
// no writer this program ships can produce. What that case actually wants is a
// way to say so and carry on, and there is no channel for it here: [load] returns
// a record or an error and nothing in between. docs/DEBT.md carries the gap under
// that description rather than as a missing refusal.
func (r record) check() error {
	for i, id := range r.shown {
		if _, ok := r.store.Get(id); !ok {
			return fmt.Errorf("%s names %s at entry %d of %d, which the record does not hold",
				shownName, memory.Short(id), i+1, len(r.shown))
		}
	}

	for i, id := range r.votes {
		b, ok := r.store.Get(id)
		if !ok {
			return fmt.Errorf("%s names %s at entry %d of %d, which the record does not hold",
				votesName, memory.Short(id), i+1, len(r.votes))
		}
		if _, is := b.Payload.(memory.Vote); !is {
			return fmt.Errorf("%s names %s at entry %d of %d, which is not a vote",
				votesName, memory.Short(id), i+1, len(r.votes))
		}
		if len(b.Prev) != 1 {
			return fmt.Errorf("%s names the vote %s, which follows %d bits rather than the one it votes on",
				votesName, memory.Short(id), len(b.Prev))
		}
	}
	return nil
}

// atomically hands write a file to fill, and puts it in place only if that
// finishes.
//
// The temp file is in the target's own directory, because a rename is only atomic
// within a filesystem and /tmp is routinely a different one — across that line
// os.Rename either fails or degrades into a copy, and a copy is the thing this
// exists to avoid. Sync before the rename, so what lands is what was written
// rather than what happened to have reached the disk.
//
// Whatever goes wrong, the previous record is untouched: nothing has been written
// to it and nothing has been removed. A disk that fills up halfway through a save
// costs the session, which is bad, and not the conversation, which is the whole
// record. The temp file goes with it — a directory slowly filling with half-saved
// records is its own way of losing the real one.
//
// What this does not do is fsync the directory, so the rename itself can be lost
// to a power failure in the seconds after it returns. That direction of failure
// leaves the *previous* record intact, which is the property being bought here;
// closing it would buy durability of the newest save, which nothing has asked
// for.
func atomically(path string, write func(io.Writer) error) error {
	if err := place(path, write); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

// place is [atomically] without the wrapping, split out so there is exactly one
// place the record gets named rather than one per branch.
//
// That split is a repair, and the thing it repairs is worth stating because the
// tidy version is what broke. Each return used to be wrapped on its own, and
// only one of them actually was — so the failure a person hit in practice, which
// is `os.CreateTemp` refusing on an unwritable directory, reached the screen as
// `open /…/.record.tmp-4052371607: permission denied`. That names a temporary
// file which does not exist, never will, and is not the thing they were told to
// go and fix; their record appeared nowhere in it. Found by running the program
// under tmux and reading the frame, not by reading this function, which had been
// read twice.
//
// One wrap at the boundary cannot have that hole. Every failure in here means
// the same sentence — the record did not get written — and the branch nobody has
// added yet is covered on the way past.
func place(path string, write func(io.Writer) error) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}

	placed := false
	defer func() {
		if placed {
			return
		}
		tmp.Close()
		os.Remove(tmp.Name())
	}()

	if err := write(tmp); err != nil {
		return err
	}
	// Explicitly, although os.CreateTemp already opens at 0600: the mode is a
	// claim about who can read the conversation, and a claim that depends on the
	// umask of whoever started the program is not one.
	if err := tmp.Chmod(0o600); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return err
	}

	placed = true
	return nil
}
