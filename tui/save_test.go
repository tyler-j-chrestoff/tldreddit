package tui

import (
	"errors"
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/tyler-j-chrestoff/tldreddit/memory"
	"github.com/tyler-j-chrestoff/tldreddit/persona"
)

// saver is a [Save] that remembers what it was handed, so a test can ask both
// halves of the invariant: whether a save happened, and whether what it carried
// is what the Model holds.
//
// The views are cloned. They are values and [memory.View.Add]'s capped append
// already stops two holders growing into one array, so this is not load-bearing
// — it is here so that a failure reads as "the wrong thing was saved" rather
// than as a puzzle about which copy moved.
type saver struct {
	n            int
	shown, votes memory.View
	err          error
}

func (s *saver) hook() Save {
	return func(shown, votes memory.View) error {
		s.n++
		s.shown, s.votes = slices.Clone(shown), slices.Clone(votes)
		return s.err
	}
}

// Every term in the comparison is one thing a save writes, and dropping any of
// them is a change that would go to disk late or never.
//
// Hand-built rather than driven through the surface, and that is the point of
// the test. Two of these rows are unreachable from any key today — nothing files
// a bit without a view naming it, and nothing swaps the record under a running
// session — so a test that only pressed keys would leave those terms
// unfalsifiable, which is the state where a term is deleted as dead weight and
// the invariant quietly narrows.
//
// The last row is the control. Without it every assertion here passes against a
// same() that always answers false.
func TestACheckpointNoticesEachThingASaveWrites(t *testing.T) {
	one, two := memory.NewStore(), memory.NewStore()
	base := checkpoint{store: one, bits: 3, shown: memory.View{"a", "b"}, votes: memory.View{"v"}}

	for name, other := range map[string]checkpoint{
		"a bit was filed":            {store: one, bits: 4, shown: memory.View{"a", "b"}, votes: memory.View{"v"}},
		"the transcript grew":        {store: one, bits: 3, shown: memory.View{"a", "b", "c"}, votes: memory.View{"v"}},
		"the transcript folded":      {store: one, bits: 3, shown: memory.View{"scar"}, votes: memory.View{"v"}},
		"the transcript reordered":   {store: one, bits: 3, shown: memory.View{"b", "a"}, votes: memory.View{"v"}},
		"a vote was cast":            {store: one, bits: 3, shown: memory.View{"a", "b"}, votes: memory.View{"v", "w"}},
		"the record itself was sw":   {store: two, bits: 3, shown: memory.View{"a", "b"}, votes: memory.View{"v"}},
		"nothing but the vote order": {store: one, bits: 3, shown: memory.View{"a", "b"}, votes: memory.View{"v"}},
	} {
		t.Run(name, func(t *testing.T) {
			if name == "nothing but the vote order" {
				// The control, carried in the same table so it cannot drift from
				// the rows it certifies.
				if !base.same(other) {
					t.Fatal("two identical checkpoints compare as different, so every row above passes for free")
				}
				return
			}
			if base.same(other) {
				t.Error("this change would not have been written")
			}
			if other.same(base) {
				t.Error("the comparison answers differently depending on which side it is asked from")
			}
		})
	}
}

// The invariant, through the keys that reach it: after any change to the store
// or to either view, the record has been handed to the hook — and after anything
// else, it has not.
//
// Both directions are the claim. Saving on every message would satisfy the first
// half and would be a program that rewrites the whole file on every keystroke;
// saving on none would satisfy the second.
//
// Driven through Update rather than through the methods behind it, because the
// wrapper is the mechanism: a branch that mutates and returns is saved by virtue
// of returning, and nothing in it says the word save.
func TestEveryChangeToTheRecordIsSavedAndNothingElseIs(t *testing.T) {
	type step struct {
		what string
		do   func(*Model) tea.Msg
		want bool
	}

	// A record with room to fold under it, so ctrl+k has something to take, and
	// a caret with somewhere to move.
	steps := []step{
		{"the terminal resizes", func(*Model) tea.Msg { return tea.WindowSizeMsg{Width: 80, Height: 24} }, false},
		{"a key goes into the composer", func(*Model) tea.Msg {
			return tea.KeyPressMsg{Code: 'x', Text: "x"}
		}, false},
		{"the caret moves", func(*Model) tea.Msg { return tea.KeyPressMsg{Code: tea.KeyUp} }, false},
		{"the surfaces swap", func(*Model) tea.Msg { return tea.KeyPressMsg{Code: 't', Mod: tea.ModCtrl} }, false},
		{"the surfaces swap back", func(*Model) tea.Msg { return tea.KeyPressMsg{Code: 't', Mod: tea.ModCtrl} }, false},
		{"a vote is cast", func(*Model) tea.Msg {
			return tea.KeyPressMsg{Code: tea.KeyUp, Mod: tea.ModShift}
		}, true},
		{"the same key again casts another", func(*Model) tea.Msg {
			return tea.KeyPressMsg{Code: tea.KeyUp, Mod: tea.ModShift}
		}, true},
		{"a fold is asked for", func(*Model) tea.Msg { return tea.KeyPressMsg{Code: 'k', Mod: tea.ModCtrl} }, true},
		{"a receipt is opened", func(*Model) tea.Msg { return tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl} }, false},
		{"a bit is said", func(m *Model) tea.Msg {
			m.composer.SetValue("what happened to the migration")
			return tea.KeyPressMsg{Code: tea.KeyEnter}
		}, true},
		{"a clock tick under the wait", func(m *Model) tea.Msg { return tickMsg{epoch: m.epoch} }, false},
		{"a reply lands", func(m *Model) tea.Msg {
			return replyMsg{epoch: m.epoch, answer: persona.Answer{Text: "the columns were dropped"}}
		}, true},
		{"a request fails", func(m *Model) tea.Msg {
			m.epoch++
			m.waiting = waiting{live: true, epoch: m.epoch}
			return failedMsg{epoch: m.epoch, err: &persona.Error{Kind: persona.Unreachable, Problem: "down"}}
		}, false},
		{"esc dismisses the failure", func(*Model) tea.Msg { return tea.KeyPressMsg{Code: tea.KeyEscape} }, false},
	}

	var hook saver
	m := record(fixtureBudget)
	m.save, m.ollama = hook.hook(), offline()

	for _, s := range steps {
		before := hook.n
		next, _ := m.Update(s.do(&m))
		m = next.(Model)

		switch got := hook.n > before; {
		case got && !s.want:
			t.Errorf("%s wrote the record, and nothing about the record changed", s.what)
		case !got && s.want:
			t.Errorf("%s changed the record and did not write it", s.what)
		}
		if !s.want {
			continue
		}

		// The other half: what went out is what is held. A hook that fired on
		// every change and sent the views from before it would satisfy every
		// assertion above.
		if !slices.Equal(hook.shown, m.shown) || !slices.Equal(hook.votes, m.votes) {
			t.Errorf("%s wrote %d/%d rows, and the surface holds %d/%d",
				s.what, len(hook.shown), len(hook.votes), len(m.shown), len(m.votes))
		}
	}
}

// A change made by nothing that exists yet is still written, because the
// decision is a comparison of the record and not a line at the end of a branch.
//
// This is the honest reach of the mechanism, and it is worth stating what it is
// not. It cannot prove that some future *caller* outside this package saves —
// nothing can, and the defence there is that [Model.update] is unexported and
// [Model.Update] is the only way in. What it proves is the part that would
// otherwise be a promise: a mutation the save path has never heard of is noticed
// by the save path.
func TestAChangeNoBranchMadeIsStillWritten(t *testing.T) {
	var hook saver
	m := record(4)
	m.save = hook.hook()

	was := m.checkpoint()
	m.say(localHandle, "written by a caller that does not exist")
	m = m.saved(was)

	if hook.n != 1 {
		t.Fatalf("%d saves for a bit written outside every branch of Update, want 1", hook.n)
	}
	if !slices.Equal(hook.shown, m.shown) {
		t.Errorf("saved %d rows, want the %d the surface holds", len(hook.shown), len(m.shown))
	}
}

// A save that fails keeps the session. The bits stay, the surface stays up, the
// trouble is visible, and the next change tries again — which is the whole
// arrangement, because a full disk at the fortieth bit is a thing a person can
// go and fix while the conversation carries on.
func TestAFailedSaveKeepsTheSessionAndSaysSo(t *testing.T) {
	full := errors.New("/state/record: no space left on device")

	var hook saver
	hook.err = full

	m := record(4)
	m.save, m.ollama = hook.hook(), offline()
	m.composer.SetValue("does this survive")

	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(Model)

	if got := len(m.shown); got != 5 {
		t.Fatalf("the view holds %d rows after a failed save, want 5 — the session went with the file", got)
	}
	if !m.trouble.up() {
		t.Fatal("a save failed and the screen says nothing about it")
	}
	if !m.trouble.unsaved {
		t.Error("the failure is drawn as a request that never reached the record, which is the opposite of what happened")
	}
	if m.trouble.problem != full.Error() {
		t.Errorf("the notice says %q, want the error the caller wrote: %q", m.trouble.problem, full)
	}

	// And it goes on trying. A program that stopped after one failure would be a
	// program that could not be repaired without restarting it, which is the
	// restart that loses the session.
	//
	// The next change is a vote rather than a second send, because the send above
	// is still in flight and enter is held until it answers — a real property of
	// this surface, and one that would otherwise make this half of the test
	// silently about nothing.
	was := hook.n
	next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp, Mod: tea.ModShift})
	m = next.(Model)

	if hook.n != was+1 {
		t.Errorf("%d saves after the next change, want %d — a failure stopped it trying", hook.n, was+1)
	}
	if got := len(m.votes); got != 1 {
		t.Errorf("%d votes recorded, want 1 — the session stopped taking them after a failed save", got)
	}
	if !m.trouble.unsaved {
		t.Error("the second failure is not on screen, so a person freeing space has nothing telling them to")
	}
}

// A save that gets through takes the notice down with it.
//
// Derived rather than dismissed: the notice claims the conversation is not on
// disk, and the moment that stops being true nobody should have to press a key
// to stop being told it. The failing case is the one worth naming — a person
// frees some space, votes, the vote is written, and the screen goes on warning
// about a disk that came back.
func TestASaveThatGetsThroughClearsTheNoticeTheLastOneRaised(t *testing.T) {
	var hook saver
	hook.err = errors.New("/state/record: read-only file system")

	m := record(fixtureBudget)
	m.save = hook.hook()

	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyUp, Mod: tea.ModShift})
	m = next.(Model)
	if !m.trouble.unsaved {
		t.Fatal("the failure never went up, so there is nothing here to clear")
	}

	hook.err = nil
	next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown, Mod: tea.ModShift})
	m = next.(Model)

	if m.trouble.up() {
		t.Errorf("the screen still says %q after a save that worked", m.trouble.problem)
	}
}

// A failure that is not about the file is left alone by a save that works. The
// two share one field, and the persona's failure means the record does not hold
// what happened — which a successful write does nothing about.
func TestASaveDoesNotClearSomebodyElsesFailure(t *testing.T) {
	var hook saver
	m := record(fixtureBudget)
	m.save = hook.hook()
	m.trouble = explain(&persona.Error{Kind: persona.Unreachable, Problem: "ollama is not running", Fix: "ollama serve"})

	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyUp, Mod: tea.ModShift})
	m = next.(Model)

	if hook.n != 1 {
		t.Fatalf("%d saves for one vote, want 1", hook.n)
	}
	if !m.trouble.up() {
		t.Error("a successful save cleared a failure that had nothing to do with it")
	}
}
