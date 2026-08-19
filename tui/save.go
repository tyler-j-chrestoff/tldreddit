package tui

import (
	"slices"

	"github.com/tyler-j-chrestoff/tldreddit/memory"
)

// Save writes the two views this surface holds, wherever they go. It is called
// after any change to the record, which is the whole of the arrangement: the
// file matches memory continuously rather than at quit.
//
// The store is not a parameter. It is a pointer whoever called [Load] handed in
// and has held ever since, so every bit this session wrote is already theirs —
// [Model.Views] makes the same argument at more length about why the views are
// the half that has to be given back.
//
// No path, no file, no format. This package does not know where a record lives
// or whether it lives anywhere at all: a nil Save is a session that is not
// persisted, which is what [New] builds and what every test in this package
// runs. The caller that owns the path is cmd/tldr, and it owns with it every
// question this package would get wrong — what a partial write costs, which
// directory, what to do about a rename that fails.
type Save func(shown, votes memory.View) error

// checkpoint is everything a save writes, reduced to what is cheap enough to
// compare after every message: which record, how many bits are in it, and the
// two views entry for entry.
//
// The mechanism is the decision worth explaining. The invariant is *after any
// change to the store or to either view, the file matches memory*, and the
// obvious way to keep it is a flag raised at each place that mutates. That flag
// is wrong the day somebody adds the sixth such place without knowing it exists,
// and it is wrong silently — a program that has stopped saving looks exactly
// like one that has nothing to save. Comparing the state before and after
// instead has no list of sites to fall off: a mutation nobody has written yet is
// noticed because the record is different afterwards, not because the code that
// changed it remembered to say so.
//
// The bit count stands in for the record's contents, and that rests on a
// property [memory.Store] states rather than on optimism: it only grows, and
// identical content collapses to one entry. So a count that did not move means
// nothing was filed, and a Put of content already there — which changes no byte
// of any file — correctly reads as nothing having happened. If a delete ever
// arrives in that package this stops being true, which is the one sentence to
// re-read before adding one.
//
// The pointer is compared as well, because a Model handed a different store of
// the same size is a different record and no count would say so. Nothing does
// that today; it costs one comparison to not have to notice when something
// starts to.
//
// What it deliberately does not compare is the encoded bytes. Encoding the whole
// record to find out whether it is worth writing costs more than writing it, and
// these three fields are exactly the inputs that encoding reads.
type checkpoint struct {
	store        *memory.Store
	bits         int
	shown, votes memory.View
}

func (m Model) checkpoint() checkpoint {
	return checkpoint{store: m.store, bits: m.store.Len(), shown: m.shown, votes: m.votes}
}

// same reports whether these two would write the same file.
func (c checkpoint) same(d checkpoint) bool {
	return c.store == d.store &&
		c.bits == d.bits &&
		slices.Equal(c.shown, d.shown) &&
		slices.Equal(c.votes, d.votes)
}

// saved is the invariant, applied: if anything a save writes has changed since
// was, the file is brought level before this message is over.
//
// It runs after every message rather than at the sites that mutate, and
// [checkpoint] is where that choice is argued.
//
// Synchronous, and not a tea.Cmd. Bits land when a person presses enter and
// when a model answers, so the save rate is human-paced rather than
// keystroke-paced, and a command would buy nothing while costing the one thing
// this exists to rule out: two writes in flight over one file, which is losing
// the record by a different route.
//
// A failure does not end the session and does not stop the next attempt. The
// conversation is in memory and the store is a pointer the caller still holds,
// so a full disk at the fortieth bit costs the file and not the record — the
// person makes room, says the next thing, and that save carries everything
// since. Quitting there is the only move that spends it, which is why the notice
// says so.
func (m Model) saved(was checkpoint) Model {
	if m.save == nil || m.checkpoint().same(was) {
		return m
	}

	switch err := m.save(m.shown, m.votes); {
	case err != nil:
		m.trouble = saveFailed(err)
		m.sync()

	case m.trouble.kind == troubleUnsaved:
		// A save that got through makes the standing notice's own claim false,
		// and a false claim about what is on disk is worse here than no claim at
		// all. Cleared rather than left for esc: nobody should have to dismiss a
		// warning that has stopped being true, and the next write is the only
		// evidence anybody has that the disk came back.
		m.trouble = notice{}
		m.sync()
	}
	return m
}

// saveFailed is the notice for a checkpoint that did not reach wherever it was
// going, in the two halves [notice] keeps apart.
//
// The problem is whatever the caller said, because the caller is the one holding
// the path and the path is the first thing a person needs. The fix is written
// here because it is the same sentence whatever went wrong, and because it is
// the half that is not obvious: the session is still running, the next change
// tries again, and the loss is only realised by quitting before one gets
// through.
func saveFailed(err error) notice {
	return notice{
		kind:    troubleUnsaved,
		problem: err.Error(),
		fix: "nothing said here is lost. Fix the above and the next change writes all of it." +
			" Quit before that and the file keeps only what already reached it.",
	}
}
