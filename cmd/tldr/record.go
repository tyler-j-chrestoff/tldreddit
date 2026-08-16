package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

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
	return rec, nil
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
// `tldr top`, which reads the store rather than a view, and by the next session,
// which loads the file this writes. Merging the views instead would put a row on
// a screen the person is looking at without them asking, and would have to decide
// where in their transcript it goes, which nothing here can answer.
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
