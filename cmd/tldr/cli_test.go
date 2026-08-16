package main

import (
	"bytes"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/tyler-j-chrestoff/tldreddit/memory"
	"github.com/tyler-j-chrestoff/tldreddit/tui"
)

// saved writes a record to a scratch file and hands back the path, which is what
// every command in this file takes: these are tests of a program that has no
// state but the file.
func saved(t *testing.T, rec record) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "record")
	if err := rec.save(path); err != nil {
		t.Fatalf("saving the record these commands run against: %v", err)
	}
	return path
}

// ran runs one command against a path and reports what it wrote to each stream.
func ran(t *testing.T, c command, path string, in string, args ...string) (out, errs string, err error) {
	t.Helper()

	var o, e bytes.Buffer
	err = c.run(streams{in: strings.NewReader(in), out: &o, err: &e}, path, args)
	return o.String(), e.String(), err
}

// No command on this surface can cast a vote, and that is the claim this whole
// program is arranged around — cli.go says why at length.
//
// It walks the table rather than naming say and top, so a third verb is covered
// the day it is added rather than the day somebody remembers this test. The
// coverage check below is what keeps that honest: a command with no invocation
// here fails outright, because a command run with no arguments would refuse, do
// nothing, and pass this test while voting freely with real ones.
func TestNoCommandOnThisSurfaceCanCastAVote(t *testing.T) {
	// One sample invocation per verb, chosen to be the most it can do rather
	// than the least: everything that writes, writing.
	invocations := map[string][]string{
		"say": {"-as", "an-agent", "-name", "an agent", "voting is not mine to do"},
		"top": {"-n", "0"},
	}
	if missing := slices.Sorted(maps.Keys(commands)); !slices.Equal(missing, slices.Sorted(maps.Keys(invocations))) {
		t.Fatalf("the table has %v and this test invokes %v; an uncovered command passes this "+
			"check by never running", missing, slices.Sorted(maps.Keys(invocations)))
	}

	for name, args := range invocations {
		t.Run(name, func(t *testing.T) {
			rec := fixture(t)
			path := saved(t, rec)

			before := slices.Clone(rec.votes)
			if _, _, err := ran(t, commands[name], path, "", args...); err != nil {
				t.Fatalf("%s %v: %v", name, args, err)
			}

			after, err := load(path)
			if err != nil {
				t.Fatalf("the record does not load after %s: %v", name, err)
			}
			if !slices.Equal(after.votes, before) {
				t.Errorf("%s changed the vote view from %d entries to %d", name, len(before), len(after.votes))
			}

			var cast int
			for b := range after.store.All() {
				if _, is := b.Payload.(memory.Vote); is {
					cast++
				}
			}
			if want := len(before); cast != want {
				t.Errorf("the record holds %d votes after %s, want the %d it started with — a vote "+
					"reached the record from a surface that has no vote key", cast, name, want)
			}
		})
	}
}

// say puts an ordinary bit on the record: the same channel, the same view and the
// same edge back to what it follows as anything typed at the surface.
func TestSayPutsAnOrdinaryBitOnTheRecord(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		args    []string
		want    string
		ref     string
		display string
	}{
		{
			name:    "said as arguments",
			args:    []string{"-as", "session-15", "the columns", "were dropped"},
			want:    "the columns were dropped",
			ref:     "session-15",
			display: "session-15",
		},
		{
			name:    "said on standard input",
			in:      "what landed\n\n- the seam\n",
			args:    []string{"-as", "session-15"},
			want:    "what landed\n\n- the seam",
			ref:     "session-15",
			display: "session-15",
		},
		{
			name:    "a display name beside the ref",
			args:    []string{"-as", "session-15", "-name", "session 15", "hello"},
			want:    "hello",
			ref:     "session-15",
			display: "session 15",
		},
		{
			name:    "surrounding whitespace is not content",
			in:      "\n\n  a handoff ends with a newline  \n\n",
			args:    []string{"-as", "session-15"},
			want:    "a handoff ends with a newline",
			ref:     "session-15",
			display: "session-15",
		},
		{
			// A ref one character off the human's is recorded, not refused. The
			// refusal is an equality and stays one on purpose (say.go says why),
			// so this row is the scope of it written down where somebody adding a
			// similarity check would have to delete it deliberately.
			name:    "a ref that merely resembles the human's",
			args:    []string{"-as", tui.Human().Ref + "2", "close, and not the same handle"},
			want:    "close, and not the same handle",
			ref:     tui.Human().Ref + "2",
			display: tui.Human().Ref + "2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := fixture(t)
			path := saved(t, rec)
			head := rec.shown[len(rec.shown)-1]

			out, _, err := ran(t, commands["say"], path, tt.in, tt.args...)
			if err != nil {
				t.Fatalf("say %v: %v", tt.args, err)
			}

			after, err := load(path)
			if err != nil {
				t.Fatalf("the record does not load after say: %v", err)
			}
			if got, want := len(after.shown), len(rec.shown)+1; got != want {
				t.Fatalf("the view holds %d rows, want %d — what was said is not on the screen "+
					"the next session opens", got, want)
			}

			id := after.shown[len(after.shown)-1]
			if got := strings.TrimSpace(out); got != id {
				t.Errorf("standard output is %q, want the address %s", got, memory.Short(id))
			}

			b, ok := after.store.Get(id)
			if !ok {
				t.Fatalf("the view names %s, which the record does not hold", memory.Short(id))
			}
			if got := b.Payload.(memory.Utterance).Text; got != tt.want {
				t.Errorf("recorded %q, want %q", got, tt.want)
			}
			if got, want := b.From, (memory.Handle{Ref: tt.ref, Display: tt.display}); got != want {
				t.Errorf("recorded from %+v, want %+v", got, want)
			}
			if got, want := b.Channel, tui.Channel(); got != want {
				t.Errorf("recorded on channel %q, want %q — memory.Cool panics on a window "+
					"spanning two channels, so this is the next fold rather than a display detail",
					got, want)
			}
			if got := b.Prev; !slices.Equal(got, []string{head}) {
				t.Errorf("it follows %v, want the row it was said after, %s", got, memory.Short(head))
			}
		})
	}
}

// Every way say refuses, and the assertion that matters is the same in each: the
// file did not move. A record with no delete cannot take back a bit written by a
// command that should have refused, so the refusal has to happen before the save
// rather than be reported after one.
func TestSayRefusesAndLeavesTheRecordAlone(t *testing.T) {
	tests := []struct {
		name string
		in   string
		args []string
	}{
		{"nobody said it", "", []string{"there is no -as here"}},
		{"an empty ref", "", []string{"-as", "", "still nobody"}},
		{"as the person at the keyboard", "", []string{"-as", tui.Human().Ref, "not mine to say"}},
		{"nothing on standard input", "", []string{"-as", "session-15"}},
		{"whitespace is not something said", "   \n\t\n", []string{"-as", "session-15"}},
		{"a flag this verb does not have", "", []string{"-as", "session-15", "-vote", "up"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := saved(t, fixture(t))
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}

			if _, _, err := ran(t, commands["say"], path, tt.in, tt.args...); err == nil {
				t.Error("it was recorded")
			}

			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) {
				t.Errorf("the record changed under a refused say: %d bytes before, %d after",
					len(before), len(after))
			}
		})
	}
}

// The human's ref is the one handle this program will not write under, and the
// two halves of that are separate facts.
//
// **It refuses before it reads anything.** The path here does not exist and must
// still not exist afterwards, and the reader handed to it must come back
// undrained — a refusal that arrived after the load would be a correct answer
// delivered after making somebody type a paragraph at a keyboard for a bit that
// was never going to be recorded. It also means this test needs no record at all,
// which is the shape of the guarantee rather than a convenience.
//
// **It says what to do.** A message that only refuses leaves a person guessing at
// a rule nobody has written down, so this requires an example ref in it that is
// not the one being refused. That is a coupling to the message's shape and it is
// the point: the sentence is the feature here, the same way [top]'s output is.
//
// A display name does not get round it, because the record keys a voter on the
// ref ([memory.Handle], [memory.Tally]) and the ref is what the refusal is about.
func TestSayWillNotSpeakAsThePersonAtTheKeyboard(t *testing.T) {
	me := tui.Human()
	path := filepath.Join(t.TempDir(), "no-record-was-ever-written-here")

	// The second row takes its text from standard input, and that is what makes
	// the undrained-reader assertion below bite at all: [words] does not read the
	// reader when the text is in the arguments, so a row of arguments alone would
	// leave that assertion unfalsifiable — measured, by moving the guard below
	// [words] and watching this test stay green.
	for _, args := range [][]string{
		{"-as", me.Ref, "the human said this, apparently"},
		{"-as", me.Ref, "-name", "an agent"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			in := strings.NewReader("nothing here should ever be read")
			var out, errs bytes.Buffer

			err := commands["say"].run(streams{in: in, out: &out, err: &errs}, path, args)
			if err == nil {
				t.Fatalf("say %v recorded an utterance under the human's own handle", args)
			}
			if out.Len() != 0 {
				t.Errorf("a refusal printed an address on standard output: %q", out.String())
			}
			if in.Len() == 0 {
				t.Error("standard input was drained by a command that was going to refuse")
			}
			if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
				t.Errorf("the record at %s was opened or written by a refused say (%v)", path, statErr)
			}

			msg := err.Error()
			if !strings.Contains(msg, me.Ref) {
				t.Errorf("the refusal does not name the ref it refused: %q", msg)
			}

			// The way forward: some `-as <ref>` in the message that is not the ref
			// being refused. Fails on a message that only says no, and on one whose
			// only example is the handle it just declined.
			var offered bool
			fields := strings.Fields(msg)
			for i, f := range fields[:max(len(fields)-1, 0)] {
				if strings.TrimSuffix(f, ",") != "-as" {
					continue
				}
				if next := strings.TrimSuffix(fields[i+1], ","); next != "" && next != me.Ref {
					offered = true
				}
			}
			if !offered {
				t.Errorf("the refusal offers no ref a person could use instead: %q", msg)
			}
		})
	}
}

// A bit said from the command line folds like any other, which is the one thing
// about it that cannot be checked by reading the record back.
//
// [memory.Cool] panics on a window spanning two channels, and say writes into the
// same view the surface folds — so a channel of its own here would not be a
// display oddity, it would be a crash in the next fold of a session somebody is
// in the middle of. This drives the real surface until a fold fires and then
// looks for the bit inside the scar.
func TestABitSaidFromTheCommandLineFoldsLikeAnyOther(t *testing.T) {
	path := saved(t, record{store: memory.NewStore()})

	if _, _, err := ran(t, commands["say"], path, "", "-as", "session-15", "the handoff"); err != nil {
		t.Fatalf("say: %v", err)
	}
	rec, err := load(path)
	if err != nil {
		t.Fatal(err)
	}
	said := rec.shown[0]

	m := tui.Load(rec.store, rec.shown, rec.votes, rec.checkpoint(path))
	m = press(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	// esc after every send, because enter does not send while a request is in
	// flight and nothing here answers one — the surface's own way of calling one
	// off is what lets this drive bit after bit through it in a row.
	//
	// Driven until the fold reaches the bit rather than a fixed number of times.
	// tui's trigger was a constant this package could not see and copied as
	// [seeded]; it is the terminal's own height now, so a copy here would be
	// wrong at every size but one. The bound is what keeps this a test rather
	// than a loop: it is generous, and reaching it is the same failure the fixed
	// count used to report.
	const bound = 200
	shown, _ := m.Views()
	for range bound {
		if !slices.Contains(shown, said) {
			break
		}
		m = press(t, m, typed('h'))
		m = press(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
		m = press(t, m, tea.KeyPressMsg{Code: tea.KeyEscape})
		shown, _ = m.Views()
	}

	if slices.Contains(shown, said) {
		t.Fatalf("%s is still in the view after %d bits, so no fold reached it and this test "+
			"asserts nothing", memory.Short(said), bound)
	}

	var absorbed bool
	for _, b := range shown.Bits(rec.store) {
		c, cold := b.Payload.(memory.Compaction)
		if !cold {
			continue
		}
		absorbed = absorbed || slices.Contains(slices.Collect(c.Absorbed()), said)
	}
	if !absorbed {
		t.Errorf("no scar in the view names %s; it left the screen without a receipt",
			memory.Short(said))
	}
}

// press drives one message through the surface, failing the test rather than the
// program if the fold above ever panics on a channel this package chose.
func press(t *testing.T, m tui.Model, msg tea.Msg) tui.Model {
	t.Helper()

	next, _ := m.Update(msg)
	return next.(tui.Model)
}

// Every verb is in the usage, because a verb nobody can find is a verb that gets
// written twice. It is also where the absence of a vote command is visible to a
// person rather than only to the test above.
func TestUsageNamesEveryVerb(t *testing.T) {
	var out, errs bytes.Buffer
	if code := dispatch([]string{"help"}, "unused", streams{out: &out, err: &errs}); code != 0 {
		t.Errorf("asking for help exited %d", code)
	}
	for name := range commands {
		if !strings.Contains(out.String(), "tldr "+name) {
			t.Errorf("the usage does not mention %q:\n%s", name, out.String())
		}
	}
	if errs.Len() != 0 {
		t.Errorf("help went to standard error: %q", errs.String())
	}
}

// A verb nobody has written is refused, and refusing prints what does exist.
func TestAnUnknownVerbIsRefusedWithTheList(t *testing.T) {
	var out, errs bytes.Buffer
	code := dispatch([]string{"vote", "-as", "an-agent"}, "unused", streams{out: &out, err: &errs})

	if code == 0 {
		t.Error("an unknown verb exited 0")
	}
	if !strings.Contains(errs.String(), "tldr say") {
		t.Errorf("the refusal does not say what this program can do:\n%s", errs.String())
	}
	if out.Len() != 0 {
		t.Errorf("a refusal wrote to standard output: %q", out.String())
	}
}
