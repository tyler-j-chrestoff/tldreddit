package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/tyler-j-chrestoff/tldreddit/memory"
	"github.com/tyler-j-chrestoff/tldreddit/tui"
)

// seeded is how many utterances the session below starts with.
//
// It was one under tui's fold trigger, which was a constant that package did not
// export and this one copied. That trigger is the terminal's own height now, so
// there is no number here that is one under it at every size, and this is a plain
// count of bits rather than a copy of anything. What it costs is stated where it
// is spent: the fold in [TestTheFileMatchesMemoryAfterEveryChange] is the one
// ctrl+k asks for rather than one the surface decided on.
const seeded = 12

// talking is a session of n plain utterances with nothing folded yet, which is
// what a saved conversation looks like before anything has left the screen.
func talking(t *testing.T, n int) record {
	t.Helper()

	s := memory.NewStore()
	at := time.Date(2026, 8, 13, 9, 0, 0, 0, time.UTC)

	var shown memory.View
	for i := range n {
		shown, _ = shown.Add(s, memory.Bit{
			At:      at.Add(time.Duration(i) * time.Minute),
			From:    me,
			Channel: "tui",
			Payload: memory.Utterance{Text: fmt.Sprintf("bit %d", i)},
			Prev:    shown.Head(),
		})
	}
	return record{store: s, shown: shown}
}

// level reads the file back and asserts it is the record this surface holds.
//
// By reading rather than by asking whether a save happened, which is the whole
// difference: a hook that fired on every change and wrote last frame's views
// would satisfy any count and would still lose the newest thing said. The store
// is compared by address, so a record that came back one bit short — the failure
// memory/wire.go's closing tag exists for — is a mismatch here rather than a
// pass.
func level(t *testing.T, path string, store *memory.Store, m tui.Model, after string) {
	t.Helper()

	on, err := load(path)
	if err != nil {
		t.Fatalf("after %s the file does not load: %v", after, err)
	}

	shown, votes := m.Views()
	if got, want := on.store.Address(), store.Address(); got != want {
		t.Errorf("after %s the file holds record %s (%d bits), want %s (%d bits)",
			after, memory.Short(got), on.store.Len(), memory.Short(want), store.Len())
	}
	if !slices.Equal(on.shown, shown) {
		t.Errorf("after %s the file's %s has %d rows, want the %d on screen",
			after, shownName, len(on.shown), len(shown))
	}
	if !slices.Equal(on.votes, votes) {
		t.Errorf("after %s the file's %s has %d entries, want the %d cast",
			after, votesName, len(on.votes), len(votes))
	}
}

// typed is one printable character arriving at the composer, which reaches the
// record through nothing at all — it is here as the other half of the claim.
func typed(r rune) tea.Msg { return tea.KeyPressMsg{Code: r, Text: string(r)} }

// The file matches memory after every change, and the changes are driven the way
// a person makes them: through keys, through the real surface, against a real
// file on a real filesystem.
//
// Three kinds are covered because they are three different mutations and only
// one of them is a bit landing. A vote writes the second view. A fold rewrites
// the first one and mints a bit nobody said. Both would be invisible to a save
// that fired only where an utterance is recorded, and both change what a reload
// would show — which is why the invariant is stated over the record rather than
// over the callers that happen to reach it.
func TestTheFileMatchesMemoryAfterEveryChange(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "record")

	// The previous session's file, so this one is a session that resumed. It also
	// means the first assertion below is about a file that already existed rather
	// than one this test's first keystroke created.
	rec := talking(t, seeded)
	if err := rec.save(path); err != nil {
		t.Fatalf("saving the session to resume from: %v", err)
	}

	m := tui.Load(rec.store, rec.shown, rec.votes, rec.checkpoint(path))
	level(t, path, rec.store, m, "loading")

	for _, step := range []struct {
		what string
		msg  tea.Msg
	}{
		{"the terminal was sized", tea.WindowSizeMsg{Width: 100, Height: 30}},
		{"a character was typed", typed('h')},
		{"another was typed", typed('i')},
		{"enter said it", tea.KeyPressMsg{Code: tea.KeyEnter}},
		{"the caret moved up", tea.KeyPressMsg{Code: tea.KeyUp}},
		{"that row was kept", tea.KeyPressMsg{Code: tea.KeyUp, Mod: tea.ModShift}},
		{"and then let go", tea.KeyPressMsg{Code: tea.KeyDown, Mod: tea.ModShift}},
		{"the ranked surface came up", tea.KeyPressMsg{Code: 't', Mod: tea.ModCtrl}},
		{"a row there was kept", tea.KeyPressMsg{Code: tea.KeyUp, Mod: tea.ModShift}},
		{"the transcript came back", tea.KeyPressMsg{Code: 't', Mod: tea.ModCtrl}},
		{"a fold was asked for", tea.KeyPressMsg{Code: 'k', Mod: tea.ModCtrl}},
	} {
		next, _ := m.Update(step.msg)
		m = next.(tui.Model)
		level(t, path, rec.store, m, step.what)
	}

	// The fold is the row this test would otherwise lose without saying so, and
	// the check below is what tells somebody the sequence above stopped
	// exercising it. It used to fire on the message that says "hi", because
	// twelve bits was one under tui's trigger; the trigger is the terminal's own
	// height now, so at 100x30 twelve is well under it and the fold this asserts
	// is the one ctrl+k asks for. That is a weaker exercise than it was — an
	// explicit fold rather than a fold the surface decided on — and it is stated
	// here rather than fixed by adding bits, because what this test is about is
	// the file keeping up with the record and not what caused the change.
	shown, votes := m.Views()
	folded := false
	for _, id := range shown {
		b, ok := rec.store.Get(id)
		if !ok {
			t.Fatalf("the surface is showing %s, which the record does not hold", memory.Short(id))
		}
		if _, cold := b.Payload.(memory.Compaction); cold {
			folded = true
		}
	}
	if !folded {
		t.Error("nothing folded during this session, so the fold half of the invariant is untested")
	}
	if len(votes) != 3 {
		t.Errorf("%d votes were recorded, want 3 — the vote half is thinner than it reads", len(votes))
	}
}

// A change that could not be written is carried by the next one that can, and
// nothing is lost in between.
//
// This is what decision 5 buys and it is the reason a failed save does not end
// the program: a full disk at the fortieth bit is a thing a person goes and fixes
// while the conversation carries on above it. The two assertions that matter are
// that the old file did not move while the disk was full, and that the bit whose
// own save failed is in the file the *next* change writes.
//
// It goes through [atomically] and [record.encode], not through a stub returning
// an error, so what is being exercised is this program's real save path meeting a
// disk that stops taking bytes halfway.
func TestAChangeThatCouldNotBeWrittenIsCarriedByTheNextOne(t *testing.T) {
	path := filepath.Join(t.TempDir(), "record")

	rec := talking(t, 4)
	if err := rec.save(path); err != nil {
		t.Fatalf("saving the session to resume from: %v", err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	disk := 40
	save := func(shown, votes memory.View) error {
		r := record{store: rec.store, shown: shown, votes: votes}
		if disk >= 0 {
			return atomically(path, func(w io.Writer) error {
				return r.encode(&failAfter{w: w, n: disk})
			})
		}
		return r.save(path)
	}

	m := tui.Load(rec.store, rec.shown, rec.votes, save)
	for _, msg := range []tea.Msg{typed('h'), typed('i'), tea.KeyPressMsg{Code: tea.KeyEnter}} {
		next, _ := m.Update(msg)
		m = next.(tui.Model)
	}

	shown, _ := m.Views()
	if len(shown) != 5 {
		t.Fatalf("the surface holds %d rows after a save that failed, want 5 — the session went with the file", len(shown))
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Errorf("the previous record changed under a failed save: %d bytes before, %d after", len(before), len(after))
	}

	// The disk comes back, and the next change is a vote — a different mutation
	// from the one that failed, which is the point: what catches up is the whole
	// record and not the write that was missed.
	disk = -1
	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyUp, Mod: tea.ModShift})
	m = next.(tui.Model)

	level(t, path, rec.store, m, "the disk came back")
	on, err := load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(on.shown) != 5 || len(on.votes) != 1 {
		t.Errorf("the file holds %d rows and %d votes, want 5 and 1 — the change that failed was not carried",
			len(on.shown), len(on.votes))
	}
}

// A save that cannot happen at all says which file it was, because the path is
// the first thing a person needs and this package is the only one that holds it.
func TestAFailedCheckpointNamesTheFile(t *testing.T) {
	dir := t.TempDir()
	blocked := filepath.Join(dir, "notadirectory")
	if err := os.WriteFile(blocked, []byte("in the way"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(blocked, "record")

	rec := talking(t, 2)
	err := rec.checkpoint(path)(rec.shown, rec.votes)
	if err == nil {
		t.Fatal("a record filed under a regular file saved cleanly")
	}
	if !strings.Contains(err.Error(), blocked) {
		t.Errorf("the failure reads %q, and names no file a person could go and look at", err)
	}
}

// A session that is killed keeps everything it had said, which is the whole
// unit: the file is level with memory continuously, so there is no such thing as
// an exit clean enough to matter.
//
// It drives the real binary under a real pty, says something, and sends SIGKILL
// — no deferred anything, no signal handler, no chance to write on the way out.
// Then it opens the file the way the next session would.
//
// Under HARNESS because it wants a terminal and util-linux's script(1), neither
// of which belongs in a suite that has to run anywhere. The skip is not a pass
// and is not counted as one; this is a check somebody runs deliberately:
//
//	HARNESS=1 go test ./cmd/tldr/ -run TestHarnessAKilledSessionKeepsItsBits -v
func TestHarnessAKilledSessionKeepsItsBits(t *testing.T) {
	if os.Getenv("HARNESS") == "" {
		t.Skip("set HARNESS=1")
	}
	script, err := exec.LookPath("script")
	if err != nil {
		t.Skipf("no script(1) on this machine, so there is no pty to run under: %v", err)
	}

	dir := t.TempDir()
	bin := filepath.Join(dir, "tldr")
	path := filepath.Join(dir, "record")

	// Built outside the repository on purpose: `go build` on a single main
	// package writes its output into the tree, and cmd/seam addresses the working
	// directory.
	build := exec.Command("go", "build", "-o", bin, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building tldr: %v\n%s", err, out)
	}

	said := "the columns were dropped on tuesday"
	feed := fmt.Sprintf("sleep 2; printf %%s\\\\r %q; sleep 2", said)

	run := exec.Command("sh", "-c", fmt.Sprintf("{ %s; } | %s -qe -c %q /dev/null", feed, script, bin))
	run.Env = append(os.Environ(), "TLDR_RECORD="+path)
	if err := run.Start(); err != nil {
		t.Fatalf("starting the session: %v", err)
	}

	// Wait for the program to be up, then for the line above to have gone in, and
	// only then kill it. Killing on the first sight of the process is the mistake
	// this loop is shaped against: it succeeds, the test passes its own kill, and
	// the record it then reads is empty because nothing had been said yet.
	up := false
	for range 100 {
		time.Sleep(100 * time.Millisecond)
		if err := exec.Command("pgrep", "-f", bin).Run(); err == nil {
			up = true
			break
		}
	}
	if !up {
		t.Fatal("the program never started, so nothing was killed and this proves nothing")
	}
	time.Sleep(3 * time.Second)

	// SIGKILL to the program itself rather than to the shell around it. pkill
	// matches the binary's own path, which is unique to this test's directory.
	if err := exec.Command("pkill", "-9", "-f", bin).Run(); err != nil {
		t.Fatalf("nothing matched the kill, so the program had already gone: %v", err)
	}
	_ = run.Wait()

	rec, err := load(path)
	if err != nil {
		t.Fatalf("the record a killed session left behind does not load: %v", err)
	}

	var texts []string
	for _, b := range rec.shown.Bits(rec.store) {
		if u, ok := b.Payload.(memory.Utterance); ok {
			texts = append(texts, u.Text)
		}
	}
	if !slices.Contains(texts, said) {
		t.Fatalf("the killed session left %v on the record, and %q is not among it", texts, said)
	}
	t.Logf("killed with %d bits on the record and %d rows in view; %q survived",
		rec.store.Len(), len(rec.shown), said)
}
