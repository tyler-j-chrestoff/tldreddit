// This file asserts nothing. It prints frames.
//
// Taste does not belong in a test, and an assertion about how a screen looks is
// taste wearing a lab coat: it locks in whatever the last person thought looked
// right and makes the next person argue with a diff instead of with a picture.
// So everything here is skipped unless HARNESS is set, and everything here
// writes to stdout rather than to t.
//
//	HARNESS=1 go test ./tui/ -run TestHarness -v
//
// What it is for is the other half of the job: looking. The defects this exists
// to catch — a handle silently shortened to collide with another, a block that
// runs off the bottom with the screen looking finished, a row built to a width
// nobody told it — are all invisible in a passing test suite and obvious in
// forty lines of rendered output. Every real property they turned out to stand
// for is asserted in tui_test.go, where it belongs.
package tui

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"

	"github.com/tyler-j-chrestoff/tldreddit/memory"
	"github.com/tyler-j-chrestoff/tldreddit/persona"
)

// sgr matches one colour instruction, as the terminal receives it.
var sgr = regexp.MustCompile("\x1b\\[([0-9;]*)m")

// colours reports every colour a row actually draws something in, in order, with
// "-" for the terminal's own foreground.
//
// It replaces a column that reported the row's *first* SGR, which could not
// answer the question it looked like it was answering. A held row begins with a
// caret and a drained gauge cell, so the first colour on it is the gauge's — and
// the margin therefore read "238" beside a row whose sentence is at full
// brightness, which is indistinguishable in that column from a row that is dim.
// The instrument said something adjacent to what it claimed, which is D27's
// shape, in the same file that partly closed D27's second instance this pass.
//
// So it walks the row, tracks the colour in force, and emits it once for each
// run of visible characters it covers — collapsing repeats, ignoring the padding
// between columns, and saying nothing at all about a row that draws nothing. The
// answer to "is this row's text dim" is then the last entry, and the glyphs in
// front of it are their own entries rather than standing in for it.
func colours(row string) string {
	visible := func(s string) bool { return strings.TrimSpace(ansi.Strip(s)) != "" }

	var out []string
	now, said := "-", ""
	for {
		at := sgr.FindStringSubmatchIndex(row)
		if at == nil {
			if visible(row) && now != said {
				out, said = append(out, now), now
			}
			break
		}
		if visible(row[:at[0]]) && now != said {
			out, said = append(out, now), now
		}

		switch p := row[at[2]:at[3]]; p {
		case "", "0":
			now = "-"
		default:
			now = p
		}
		row = row[at[1]:]
	}
	return strings.Join(out, " ")
}

// screen renders m at its current size the way a terminal of that colour profile
// would: put through the same downgrade a real terminal gets, ANSI then
// stripped for printing, every row clipped to width, boxed so the right margin
// is visible. The column past the right margin is every colour the row draws
// something in *after* the downgrade, in order, with "-" for the terminal's own
// foreground — so the last entry on a transcript row is the colour of the
// sentence, and the ones before it are the caret, the vote mark and the handle.
// See [colours].
//
// That column used to grep the raw frame for the literal "38;5;242m", which is
// D27's instance two: a check that cannot fail, reporting the fade present at
// every terminal size and every profile because it was reading the string before
// anything had a chance to degrade it. It goes through colorprofile.Writer now —
// the same library and the same code path Bubble Tea's renderer uses — so a
// no-colour capture shows what a no-colour terminal shows.
//
// It is emulation and not observation, and the difference is worth keeping
// straight: this runs the downgrade the renderer would run, on the content the
// renderer would be handed. What it still cannot see is the renderer itself
// deciding something different. Closing that needs a real terminal.
func screen(m Model, label string) string { return profiled(m, label, colorprofile.TrueColor) }

func profiled(m Model, label string, p colorprofile.Profile) string {
	var down strings.Builder
	w := colorprofile.Writer{Forward: &down, Profile: p}
	_, _ = w.WriteString(m.View().Content)
	rows := strings.Split(down.String(), "\n")

	var b strings.Builder
	fmt.Fprintf(&b, "── %s · %dx%d · %s %s\n", label, m.width, m.height, p, strings.Repeat("─", 12))
	b.WriteString("┌" + strings.Repeat("─", m.width) + "┐\n")
	for _, r := range rows {
		plain := ansi.Truncate(ansi.Strip(r), m.width, "")
		fmt.Fprintf(&b, "│%s%s│ %s\n", plain,
			strings.Repeat(" ", max(m.width-ansi.StringWidth(plain), 0)), colours(r))
	}
	b.WriteString("└" + strings.Repeat("─", m.width) + "┘ rows: " +
		fmt.Sprint(len(rows)) + " of " + fmt.Sprint(m.height) + "\n")
	return b.String()
}

func sized(w, h int) Model {
	m := New()
	mm, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return mm.(Model)
}

var lines = []string{
	"starting the migration on the auth service",
	"acknowledged, standing by for the schema dump",
	"schema dump is 40MB, uploading now",
	"got it — running the diff against staging",
	"three columns drift: created_at, updated_at, deleted_at",
	"those are the soft-delete columns nobody backfilled",
	"do we backfill or drop them",
	"backfill. dropping loses the audit trail",
	"agreed, writing the backfill migration now",
	"heads up: the staging box is at 90% disk",
	"pausing the upload until that clears",
	"cleared, 40% free after the log rotation",
	"resuming, ETA four minutes",
	"backfill migration is up for review",
	"reviewing — the null default worries me",
	"switching it to an explicit epoch timestamp",
	"that reads better, approving",
	"merged and deploying to staging",
	"staging is green across the board",
	"promoting to production in ten minutes",
	"production deploy started",
	"production is green, migration complete",
	"writing the postmortem note",
	"nothing to post-mortem, it went clean",
	"still worth a note for the next person",
	"fair. filing it under runbooks",
	"closing the incident channel",
	"thanks everyone",
	"one more thing: the disk alert threshold",
	"raise it to 80% so we get more warning",
	"filed as a follow-up ticket",
	"done for the day",
}

// talk sends n bits alternating between a human and a model's persona handle.
func talk(m Model, n int) Model {
	handles := []memory.Handle{
		{Ref: "local", Display: "me"},
		{Ref: "ollama/llama3", Display: "coordinator-7"},
	}
	for i := range n {
		m.say(handles[i%len(handles)], lines[i%len(lines)])
	}
	return m
}

func page(m Model, n int) Model {
	for range n {
		mm, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
		m = mm.(Model)
	}
	return m
}

func TestHarness(t *testing.T) {
	if os.Getenv("HARNESS") == "" {
		t.Skip("set HARNESS=1")
	}
	out := os.Stdout

	for _, size := range [][2]int{{80, 20}, {100, 30}} {
		m := talk(sized(size[0], size[1]), 32)

		// The newest row is a reply that ran out of room. It sits in the hot
		// band among rows that finished, which is the comparison worth looking
		// at: the mark has to be obvious without being louder than the sentence.
		m.recordReply(persona.Answer{Text: "the disk alert threshold should be raised to", Truncated: true})
		fmt.Fprint(out, screen(m, "closed"))
		m.unfold()
		fmt.Fprint(out, screen(m, "unfolded, at the top"))
		fmt.Fprint(out, screen(page(m, 1), "unfolded, one page down"))
		fmt.Fprint(out, screen(page(m, 2), "unfolded, two pages down"))
	}
}

func TestHarnessNarrow(t *testing.T) {
	if os.Getenv("HARNESS") == "" {
		t.Skip("set HARNESS=1")
	}
	out := os.Stdout

	m := talk(sized(80, 20), 32)
	m.recordReply(persona.Answer{Text: "the disk alert threshold should be raised to", Truncated: true})
	for _, w := range []int{40, 24, 16, 8, 1} {
		fmt.Fprintf(out, "── transcript at width %d ──\n%s\n%s\n", w, strings.Repeat("·", w),
			ansi.Strip(shot(m, w, true)))
	}

	c := cooled(t, "the deploy failed", "deploy again", "and again")
	for _, w := range []int{80, 40, 30, 20, 16} {
		fmt.Fprintf(out, "── unresolvable receipt at width %d ──\n%s\n%s\n", w,
			strings.Repeat("·", w), ansi.Strip(unfold(frame{store: memory.NewStore(), clock: atNine, width: w}, c)))
	}
}

// waitingOn puts a request in flight that has been out for d, without asking
// anything of anybody. The point of looking at this frame is the pending state,
// and a harness that needed a live model to draw it could only be run on a
// machine that had one.
func waitingOn(m Model, d time.Duration) Model {
	m.composer.SetValue("what did we decide about the soft-delete columns")
	m.send()
	m.waiting.elapsed = d
	m.sync()
	return m
}

func broken(m Model, n notice) Model {
	m.trouble = n
	m.sync()
	return m
}

// The two states that are not bits. Both are the machine's own doing, so both
// have to be legible at a glance and distinguishable from anything on the
// record — which is what the dashed rules are for, and dashes are exactly the
// thing that has to be looked at rather than asserted about.
func TestHarnessPending(t *testing.T) {
	if os.Getenv("HARNESS") == "" {
		t.Skip("set HARNESS=1")
	}
	out := os.Stdout

	// The measured numbers from this machine: 15s warm for the default model,
	// 27s on the first call while the weights load, and a minute-plus is what a
	// larger model or a cold GPU actually looks like.
	for _, d := range []time.Duration{0, 15 * time.Second, 27 * time.Second, 96 * time.Second} {
		m := waitingOn(talk(sized(80, 20), 9), d)
		fmt.Fprint(out, screen(m, fmt.Sprintf("waiting %s", elapsed(d))))
	}

	// Both real failures, in the persona package's own words.
	down := notice{
		problem: "ollama is not answering at http://localhost:11434 — it does not appear to be running",
		fix:     "start it with: ollama serve",
	}
	missing := notice{
		problem: `ollama is running, but the model "qwen3.5:latest" is not installed`,
		fix:     "pull it with: ollama pull qwen3.5:latest",
	}
	for _, n := range []notice{down, missing} {
		fmt.Fprint(out, screen(broken(talk(sized(80, 20), 9), n), "failed"))
	}

	// A truncated reply is not a notice at all any more — it is a bit, recorded
	// and marked, so the frame to look at is the transcript rather than a block
	// under it. What is being judged here is whether the row reads as "this
	// speaker stopped" at a glance, sitting among rows that finished.
	cut := talk(sized(80, 20), 9)
	cut.recordReply(persona.Answer{Text: "the three steps are, first,", Truncated: true})
	fmt.Fprint(out, screen(cut, "a reply that ran out of room, on the record"))

	// And the frame that matters more: the conversation carried on. The
	// fragment is still there, three rows up, with nothing holding it in place
	// but the record — which is the whole point of the change.
	cut.composer.SetValue("never mind — where did the disk threshold land")
	cut.send()
	answered, _ := cut.Update(replyMsg{
		epoch:  cut.epoch,
		answer: persona.Answer{Text: "80%, filed as a follow-up ticket"},
	})
	fmt.Fprint(out, screen(answered.(Model), "...still on the record under a reply that finished"))

	// Both blocks at the widths where they have to give something up, and
	// beside them the row that replaced the third one. The mark is the thing to
	// watch: it is the only thing on this surface that takes its columns before
	// the text does, so at twelve columns there should be a mark and almost no
	// sentence, rather than the other way round.
	fragment := memory.Bit{
		From:    memory.Handle{Ref: "ollama/qwen3.5", Display: "qwen3"},
		Payload: memory.Utterance{Text: "the three steps are, first,", Truncated: true},
	}
	for _, w := range []int{60, 40, 30, 20, 12} {
		m := waitingOn(sized(w, 20), 96*time.Second)
		fmt.Fprintf(out, "── pending at width %d ──\n%s\n%s\n", w, strings.Repeat("·", w),
			ansi.Strip(m.note()))

		b := broken(sized(w, 20), down)
		fmt.Fprintf(out, "── failure at width %d ──\n%s\n%s\n", w, strings.Repeat("·", w),
			ansi.Strip(b.note()))

		fmt.Fprintf(out, "── a fragment's row at width %d ──\n%s\n%s\n", w, strings.Repeat("·", w),
			ansi.Strip(said(frame{}, fragment, w)))

		// The line under it, which is where the remedy lives. The reason has to
		// outlast the fix: an instruction with no stated cause is what the
		// failure block was rewritten to stop doing.
		u := sized(w, 20)
		u.recordReply(persona.Answer{Text: "the three steps are, first,", Truncated: true})
		fmt.Fprintf(out, "── the line under it at width %d ──\n%s\n%s\n", w, strings.Repeat("·", w),
			ansi.Strip(u.note()))
	}

	// Below that, where the mark is all there is room for. Nothing here is
	// legible as a sentence; what is being checked is that no width silently
	// turns a fragment back into a finished utterance.
	for _, w := range []int{8, 6, 4, 3, 2, 1} {
		fmt.Fprintf(out, "── a fragment's row at width %d ── %s\n", w, ansi.Strip(said(frame{}, fragment, w)))
	}
}

// The other block that wears this shape: the record is whole and the file behind
// it is not.
//
// It is the mirror image of the failure above and shares every row but the
// first, so the frame worth looking at is the pair together — whether a person
// glancing at the top row can tell "the machine did not hear you" from "the
// machine heard you and the disk did not", without reading either sentence.
//
// The two problems are the shapes cmd/tldr's atomically actually produces, at
// the default path recordPath builds, rather than invented ones — the wrapped
// write, which names the record and the temporary file and is the longest thing
// this block will ever be handed, and the bare mkdir, which is the shortest. The
// long one is the case that matters: the path is the first thing a person needs
// and it is also what decides how many transcript rows this costs.
func TestHarnessUnsaved(t *testing.T) {
	if os.Getenv("HARNESS") == "" {
		t.Skip("set HARNESS=1")
	}
	out := os.Stdout

	full := saveFailed(errors.New("writing /home/tyler/.local/state/tldreddit/record:" +
		" write /home/tyler/.local/state/tldreddit/.record.tmp-2917184: no space left on device"))
	ro := saveFailed(errors.New("mkdir /home/tyler/.local/state/tldreddit: read-only file system"))
	down := notice{
		problem: "ollama is not answering at http://localhost:11434 — it does not appear to be running",
		fix:     "start it with: ollama serve",
	}

	for _, size := range [][2]int{{100, 30}, {80, 20}} {
		for _, n := range []notice{full, ro} {
			fmt.Fprint(out, screen(broken(talk(sized(size[0], size[1]), 9), n), "not on disk"))
		}
	}

	// No colour at all, beside the block it has to be told apart from. Warm is the
	// only thing separating either header from the transcript above it, and warm
	// is the first thing to go.
	m := broken(talk(sized(100, 30), 9), full)
	fmt.Fprint(out, profiled(m, "not on disk, no colour at all", colorprofile.NoTTY))
	fmt.Fprint(out, profiled(broken(talk(sized(100, 30), 9), down), "...and the block it is not", colorprofile.NoTTY))

	// The ladder, both of them, at every width where either steps. The headers are
	// printed side by side so the rung each one is standing on can be read against
	// the other at the same terminal.
	for _, w := range []int{100, 80, 60, 43, 42, 33, 32, 31, 27, 26, 25, 20, 17, 16, 15, 14, 13, 12, 8, 6, 5, 1} {
		a, b := broken(sized(w, 20), full), broken(sized(w, 20), down)
		fmt.Fprintf(out, "── width %d ──\n%s\n%s\n%s\n", w, strings.Repeat("·", w),
			ansi.Strip(a.note()), ansi.Strip(b.note()))
	}
}

// A fold is where a fragment could go quiet, so this is the frame that says
// whether it does. Three places to look: the closed scar, which is the only one
// of them visible without pressing a key; the row in the block ctrl+u opens; and
// the whole screen with that block open.
func TestHarnessFragment(t *testing.T) {
	if os.Getenv("HARNESS") == "" {
		t.Skip("set HARNESS=1")
	}
	out := os.Stdout

	m := sized(80, 20)
	m.recordReply(persona.Answer{Text: "the three steps are, first,", Truncated: true})
	for i := range fixtureBudget {
		m.say(localHandle, lines[i%len(lines)])
	}
	c := scar(t, m)

	// The tally has to hold its place on the ladder against the words and the
	// span. What to watch for is the width where it goes: below that, a folded
	// fragment is one press away rather than on screen, and that is the trade
	// being made.
	for _, w := range []int{100, 80, 60, 48, 40, 32, 24, 19, 12} {
		fmt.Fprintf(out, "── scar over a window with a fragment, width %d ──\n%s\n%s\n",
			w, strings.Repeat("·", w), ansi.Strip(seam(m.frame(), c, w)))
	}
	for _, w := range []int{80, 40, 24} {
		fmt.Fprintf(out, "── its receipt at width %d ──\n%s\n%s\n",
			w, strings.Repeat("·", w), ansi.Strip(receiptOf(m, c, w)))
	}

	m.unfold()
	fmt.Fprint(out, screen(m, "a folded fragment, opened"))
}

// The two rows this session's work is about, swept: the scar's own row, and the
// footer's index of keys.
//
// The scar half is where [quoteFloor] comes from. Walk down the widths and watch
// for three things: the width where the span appears (it is taken only once the
// whole quotation already fits beside it, so nothing is ever traded for it); the
// width where the quotation goes entirely, which is the floor; and — the one to
// argue about — whether the row ever reads as prose the machine composed. The
// row above the scar in the frames is the answer to whether a quoted last bit
// makes the seam read continuously, which is the argument the tie-break rests on.
//
// The footer half is the drop mark. Watch for the mark *not* being there on the
// widest rung, which is the only frame in the program where the index is
// complete, and for the bottom of the ladder, where the row is the mark alone
// rather than the blank it used to be.
func TestHarnessScar(t *testing.T) {
	if os.Getenv("HARNESS") == "" {
		t.Skip("set HARNESS=1")
	}
	out := os.Stdout

	m := talk(sized(100, 30), 40)
	c := scar(t, m)
	f := m.frame()
	for _, w := range []int{120, 100, 90, 80, 70, 60, 50, 45, 40, 36, 34, 30, 24, 16, 8} {
		fmt.Fprintf(out, "── scar, width %d ──\n%s\n%s\n",
			w, strings.Repeat("·", w), ansi.Strip(seam(f, c, w)))
	}

	// Held so the quotation has a vote to answer to, drawn beside the same scar
	// with nobody voting, so the two rows are readable against each other.
	fmt.Fprintf(out, "── the same scar, one absorbed bit kept (hold long since lapsed) ──\n")
	v := back(talk(sized(100, 30), 10), 6)
	v = heldSince(v, holdFor+time.Second)
	v = talk(v, 30)
	vf := v.frame()
	for _, b := range vf.bits {
		if p, cold := b.Payload.(memory.Compaction); cold {
			fmt.Fprintf(out, "%s\n", ansi.Strip(seam(vf, p, 100)))
		}
	}

	for _, w := range []int{140, 120, 110, 100, 80, 70, 60, 50, 40, 30, 20, 14, 10, 4} {
		mm := talk(sized(w, 24), 30)
		fmt.Fprintf(out, "── footer, width %d ──\n%s\n%s\n",
			w, strings.Repeat("·", w), ansi.Strip(mm.footer()))
	}
}

// The row a person can actually read, on a real conversation and at the sizes
// this surface is looked at.
//
// What to watch for is the shape of the block rather than its width: a
// continuation row is a blank handle column and nothing else, so the whole
// argument for the hanging indent is whether these read as one paragraph under
// one name or as a run of rows nobody said. The frame beneath each is where the
// answer does not fit, and it is the ordinary case rather than the extreme one —
// at sixty by fourteen the viewport is seven rows.
//
// The floor is printed at the end, walked from width 1 up on the fixture
// [TestTheCaretsRowIsCutWhereTheArrangementAlreadyGaveUp] pins, because that is
// where the number in that test comes from. It is copied into the pin by hand and
// never worked out from the constants.
func TestHarnessRead(t *testing.T) {
	if os.Getenv("HARNESS") == "" {
		t.Skip("set HARNESS=1")
	}
	out := os.Stdout

	for _, size := range [][2]int{{100, 30}, {80, 24}, {60, 14}, {40, 10}} {
		m := talk(sized(size[0], size[1]), 5)
		m.utter(m.persona.Handle(), memory.Utterance{Text: longAnswer})
		fmt.Fprint(out, profiled(m, "an answer under the caret", colorprofile.NoTTY))
	}

	// And the same conversation with the caret one row up, which is the picture the
	// whole design rests on: the row you are on is readable and the rest of the
	// screen is the index to it.
	m := talk(sized(100, 30), 5)
	m.utter(m.persona.Handle(), memory.Utterance{Text: longAnswer})
	m.move(-1)
	fmt.Fprint(out, profiled(m, "the caret one row up", colorprofile.NoTTY))

	// And the caret walked *up* onto a long answer in a transcript that already
	// overflows, at the size where that used to go wrong. Revealing the block's
	// first row alone left an answer open two rows deep here, with the rest below
	// the margin and the screen looking finished — the promise stated in this
	// package's doc and then not kept. [Model.revealBlock] is what fixed it, and
	// this frame is where to look before touching it.
	up := talk(sized(60, 14), 7)
	up.utter(up.persona.Handle(), memory.Utterance{Text: longAnswer})
	up.say(localHandle, "and the one after that")
	up.move(-1)
	fmt.Fprint(out, profiled(up, "the caret walked up onto a long answer", colorprofile.NoTTY))

	// And the caret parked on a row the next fold will take, which is the one
	// arrangement where the two things this surface draws in the same columns meet:
	// the whole block steps two columns left and dims together, so the fade is a
	// fact about the bit rather than about its first line.
	fmt.Fprint(out, profiled(back(talk(sized(60, 20), 12), 7), "a cooling row under the caret",
		colorprofile.NoTTY))

	fmt.Fprintln(out, "\n── where the caret's row goes back to being cut ──")
	for width := 20; width <= 28; width++ {
		rows := bare(reading(false), width)[1]
		fmt.Fprintf(out, "   %3d │%s│ %d row(s)\n", width, rows[0], len(rows))
	}

	// The second surface, where the same rule takes the other shape. What to watch
	// for is the row the block hangs from: it keeps every column it had — ordinal,
	// address, clock, mark, handle — and gives up only its preview, so the list is
	// still a list of references while one of them is open. And the block is the
	// terminal's width rather than the text column's, which is the whole reason
	// this shape exists here and the transcript's does not: at forty columns the
	// reference is sixty per cent of the row.
	//
	// The narrow sizes are the ones to look at, and the last of them is below this
	// surface's own floor: the block refuses to open into a column too narrow to
	// carry prose, and the row falls back to being cut. That frame is here rather
	// than left out because a sweep that stops where a shape still works is how the
	// counterexample got shipped — measured without a floor, the same fixture at
	// four columns drew 223 rows of "│  …" with every character clipped away.
	for _, width := range []int{100, 60, 40, 24, 16} {
		m := judgedTalk(New(), 12)
		m.utter(memory.Handle{Ref: "ollama/llama3", Display: "coordinator-7"},
			memory.Utterance{Text: longAnswer})
		m.move(len(m.shown))
		m.vote(memory.Up)
		m = press(m, "ctrl+t")

		fmt.Fprintf(out, "\n── ranked, an answer under the caret · width %d ──\n%s\n",
			width, strings.Repeat("·", width))
		for _, r := range rankShot(m, width) {
			fmt.Fprintf(out, "%s\n", r)
		}
	}
}

// Where each mark actually stops surviving, walked from width 1 up, on the same
// four fixtures [TestTheRowsMarkFloorsAreWhereTheyWereMeasured] pins.
//
// This is where those four numbers come from, and it exists because they moved
// this pass and the temptation was to work out the new ones from the constants —
// caret two columns, vote column five, and so on. Three claims in this
// repository's history were made that way and all three came out wrong. So the
// floors are read off rendered rows, printed here, and copied into the pin by
// hand.
func TestHarnessFloors(t *testing.T) {
	if os.Getenv("HARNESS") == "" {
		t.Skip("set HARNESS=1")
	}

	for _, s := range []struct {
		name       string
		cut, whole Model
		row        func(*testing.T, Model, int) string
	}{
		{"transcript", unfoldedFragment(true), unfoldedFragment(false), hotRow},
		// The same row on its way out. It begins [step] columns further left, so
		// the terminal reaches its mark later and the dash floor is lower. Here
		// rather than in a craft note, because a note whose stated command does
		// not print its own numbers is a claim nobody re-derives.
		{"transcript, cooling", coolingFragment(true), coolingFragment(false), coldRow},
		{"receipt", foldedFragment(true), foldedFragment(false), receiptRow},
	} {
		dash, word := 0, 0
		for width := 1; width <= 120; width++ {
			marked, plain := s.row(t, s.cut, width), s.row(t, s.whole, width)
			if dash == 0 && marked != plain {
				dash = width
			}
			if word == 0 && strings.Contains(marked, "unfinished") {
				word = width
			}
		}
		fmt.Fprintf(os.Stdout, "── %s: dash floor %d, word floor %d\n", s.name, dash, word)

		// The rows either side of each floor, because a number with no picture
		// beside it is the thing this file exists not to produce.
		for _, w := range []int{dash - 1, dash, word - 1, word} {
			fmt.Fprintf(os.Stdout, "   %3d │%s│  (finished: %s)\n",
				w, s.row(t, s.cut, w), s.row(t, s.whole, w))
		}
	}
}

// The hold schedule: what holdFor should be, measured on this surface's own
// cadence rather than inherited from memory's fixtures.
//
// [memory.DefaultHold] is thirty minutes, calibrated against one bit a minute.
// The live run behind the demo page did 343 bits in about twenty minutes — one
// every 3.5 seconds — and a hold is measured in the conversation's own time, so
// the same thirty minutes means something about seventeen times larger here.
// This prints what it means, at that cadence and at two either side of it.
//
// Rows and folds together, never the row count alone: D36 is the entry about a
// share figure that was unsound because it was quoted without them.
//
// The simulator this reads is [simulate], which now lives in strand_test.go
// beside the table it is frozen into. This prints; that one asserts, and the
// number a person reads here is the number that file compares. Two simulators
// would be the thing D36 is about.
func TestHarnessHoldSchedule(t *testing.T) {
	if os.Getenv("HARNESS") == "" {
		t.Skip("set HARNESS=1")
	}
	out := os.Stdout

	const bits = 400
	holds := []time.Duration{
		30 * time.Second, time.Minute, 2 * time.Minute, 3 * time.Minute,
		5 * time.Minute, 10 * time.Minute, 30 * time.Minute,
	}
	rates := []int{1, 2, 3, 5, 10, 25}

	for _, cadence := range []time.Duration{2 * time.Second, 3500 * time.Millisecond, 10 * time.Second} {
		fmt.Fprintf(out, "\n── %d bits, one every %s, budget %d, keep by keepFrom ──\n",
			bits, cadence, coolFloor)
		fmt.Fprintf(out, "%8s │", "hold")
		for _, r := range rates {
			fmt.Fprintf(out, " 1 in %-2d      │", r)
		}
		fmt.Fprintf(out, "  worst\n")

		for _, hold := range holds {
			fmt.Fprintf(out, "%8s │", hold)
			worst := 0
			for _, r := range rates {
				o := simulate(schedule{bits: bits, rate: r, budget: coolFloor, cadence: cadence, hold: hold})
				worst = max(worst, o.worst)
				fmt.Fprintf(out, " %3d rows %2df │", o.worst, o.folds)
			}
			fmt.Fprintf(out, " %4d\n", worst)
		}
	}
}

// back moves the caret n bits up the view, through the key that moves it.
func back(m Model, n int) Model {
	for range n {
		m.move(-1)
	}
	return m
}

// heldSince is an upvote on the bit under the caret, cast as though the key had
// been pressed d of conversation time ago — which is how a frame shows a hold
// part way through draining rather than at full.
//
// It is [Model.vote]'s own two lines with the instant named instead of read off
// the clock. Everything else about the fixture is the program: the vote is a
// bit, it goes in the store, it goes on the vote view, and [memory.Stay.Holds]
// decides what it is worth by comparing it against [memory.View.Latest].
func heldSince(m Model, d time.Duration) Model {
	target, ok := m.store.Get(m.mark)
	if !ok {
		return m
	}
	m.votes, _ = m.votes.Add(m.store,
		memory.Cast(m.shown.Latest(m.store).Add(-d), localHandle, memory.Up, target))
	m.sync()
	return m
}

// The vote, on screen, at the sizes a person actually runs a terminal at.
//
// Five frames, and the middle one is the one this unit of work exists to
// produce: two bright rows standing between two scars, on a screen where
// everything else is on its way out. Two and not one, which is the second thing
// to look at here — the row somebody voted on carries the mark, and the question
// it answers carries a tie hanging off that mark, because memory spares both and
// for a while the screen only said so about one of them. Everything before and
// after it is the same interaction in the states that are easy to get wrong — a
// hold part spent, a hold that has run out, a downvote, and a downvote taking a
// hold away.
func TestHarnessVote(t *testing.T) {
	if os.Getenv("HARNESS") == "" {
		t.Skip("set HARNESS=1")
	}
	out := os.Stdout

	for _, size := range [][2]int{{100, 30}, {80, 20}, {60, 14}, {40, 10}} {
		w, h := size[0], size[1]

		// A hold part way through draining. The caret is where the vote was cast,
		// which is where a person's eye already is.
		m := back(talk(sized(w, h), 9), 3)
		m = heldSince(m, holdFor/2)
		fmt.Fprint(out, screen(m, "a hold half spent"))

		// The money frame. A vote on an older bit, then the conversation carries
		// on over it until folds have happened either side: the kept row and the
		// question it answers are the only bright things above the live tail, and
		// the two scars are what the fold did around them.
		m = back(talk(sized(w, h), 10), 4)
		m.vote(memory.Up)
		m = talk(m, 22)
		fmt.Fprint(out, screen(m, "an answer kept and its question covered, between two scars"))
		fmt.Fprint(out, profiled(m, "the same frame with no colour at all", colorprofile.NoTTY))

		// The same arrangement with the hold run out. The mark goes hollow — the
		// vote is still on the record and always will be — and the row joins the
		// material the next fold takes, which is what the fade then says about it.
		m = back(talk(sized(w, h), 10), 7)
		m = heldSince(m, holdFor+time.Second)
		fmt.Fprint(out, screen(m, "a hold that has run out"))

		// And where the vote goes when the fold finally takes the bit: onto the
		// receipt, one key away, still marked. The transcript cannot show it any
		// more and the record has not forgotten it.
		m = back(talk(sized(w, h), 8), 5)
		m.vote(memory.Down)
		m = talk(m, 12)
		m.unfold()
		fmt.Fprint(out, screen(m, "a vote that outlived its bit, on the receipt"))

		// A downvote. One mark, no gauge, nothing draining, because a downvote
		// holds nothing — which is the whole of the asymmetry and there is no
		// legend anywhere saying it.
		m = back(talk(sized(w, h), 9), 2)
		m.vote(memory.Down)
		fmt.Fprint(out, screen(m, "let go"))
	}

	// And the sequence that has to be seen in order: holding so much that the fold
	// cannot happen, letting one go, and then the fold arriving on the next thing
	// anybody says.
	//
	// Every third bit is upvoted as it arrives, which is the caret riding the
	// newest bit and a person pressing keep on some of the answers — not a
	// contrived pattern. A hold spares the bit its own bit answers as well as
	// itself, so at that rate the view goes held, covered, free, held, covered,
	// free: no run of two takeable bits exists anywhere, D32's size rule leaves
	// nothing to cool, and memory.View.Fold refuses. The gauge goes past its own
	// limit and says which of the two things that means, and the footer's ladder
	// re-ranks so the key that ends the state is printed.
	//
	// Three frames rather than two, which is the change worth looking at. Letting
	// go used to fold on the keystroke: the rows collapsed under the hand that
	// released them, from a screen where nothing at all was fading, so the one
	// operation this surface exists to make visible arrived with no antecedent.
	// Now the middle frame is the antecedent — the freed rows go dim the instant
	// the key is pressed — and the fold is the frame after it. The three numbers
	// under each frame are what the gauge is reading from.
	handles := []memory.Handle{localHandle, {Ref: "ollama/llama3", Display: "coordinator-7"}}
	state := func(m Model) string {
		return fmt.Sprintf("   foldable %d of %d · blocked %v · cooling %d rows · covered %d rows · view %d rows\n",
			m.foldable(), m.budget(), m.blocked(), len(m.absorbing()), len(m.covered(m.live())), len(m.shown))
	}
	held := func(w, h, n, every int) Model {
		m := sized(w, h)
		for i := range n {
			m.say(handles[i%len(handles)], lines[i%len(lines)])
			if i%every == 0 {
				m.vote(memory.Up)
			}
		}
		return m
	}

	// One denser than that and the state is a different one, which is worth
	// looking at rather than describing. At one vote in two every row is either
	// held or covered, so there is no material a fold could take at all: the gauge
	// reads nothing while the view keeps growing past the frame, and what says why
	// is that every single row carries a mark or a tie. That is honest and it is
	// not the blocked state — blocked is *pressure with nowhere to go*, and here
	// there is no pressure.
	fmt.Fprint(out, screen(held(100, 30, 26, 2), "every row kept or covered: no pressure at all"))
	fmt.Fprint(out, state(held(100, 30, 26, 2)))

	m := held(100, 30, 74, 3)
	fmt.Fprint(out, screen(m, "held so hard it cannot fold"))
	fmt.Fprint(out, state(m))

	// Back to a row somebody actually voted on — a downvote on an unheld row
	// withdraws nothing and the two frames after this would be the same frame.
	m = back(m, 13)
	if _, up := m.live()[m.mark]; !up {
		t.Fatal("the caret is not on a held bit, so letting go here shows nothing")
	}
	m.vote(memory.Down)
	fmt.Fprint(out, screen(m, "...one let go: the rows it frees start cooling, and nothing has left"))
	fmt.Fprint(out, state(m))

	m.say(handles[0], "and one more thing")
	fmt.Fprint(out, screen(m, "...and the next thing said takes them"))
	fmt.Fprint(out, state(m))
}

// judging is a conversation somebody has been voting in as it went: kept, let
// go, and kept a while ago, cast on the newest bit the way a hand riding the
// caret casts them.
//
// The third of those is what makes this fixture worth having rather than a
// convenience. [Model.say] writes its bits microseconds apart, so a hold cast
// with the moment of the key never ages — a held bit is never folded, and a
// fixture of nothing but those produces a ranked list every row of which is
// still on the transcript, drawing a full gauge. That is not what a conversation
// looks like. heldSince casts with an instant far enough back that the hold is
// already spent, so the fold takes those bits and the list fills with rows that
// have left the screen: the hollow mark, and no gauge beside it.
func judging(m Model, n int) Model {
	handles := []memory.Handle{
		{Ref: "local", Display: "me"},
		{Ref: "ollama/llama3", Display: "coordinator-7"},
	}
	for i := range n {
		m.say(handles[i%len(handles)], lines[i%len(lines)])
		switch {
		case i%3 == 0:
			m = heldSince(m, holdFor+time.Second)
		case i%7 == 0:
			m.vote(memory.Down)
		case i%11 == 0:
			m.vote(memory.Up)
		}
	}
	return m
}

// The ranked surface, at the sizes a person runs a terminal at and in a profile
// with no colour at all.
//
// What to look at, none of it assertable. Whether the bands read as bands
// without a legend; whether a clock on every row says "this is not in time
// order" before anybody thinks about it; whether the scar's row reads as a fold
// rather than as somebody who said "16 bits"; and whether the empty state tells
// a first-time reader what to press.
func TestHarnessRanked(t *testing.T) {
	if os.Getenv("HARNESS") == "" {
		t.Skip("set HARNESS=1")
	}
	out := os.Stdout

	for _, size := range [][2]int{{100, 30}, {80, 24}, {60, 14}, {40, 10}} {
		w, h := size[0], size[1]

		m := judging(sized(w, h), 40)
		fmt.Fprint(out, screen(m, "the transcript it is reached from"))
		m.rank()
		fmt.Fprint(out, screen(m, "ranked"))
		fmt.Fprint(out, profiled(m, "ranked, no colour at all", colorprofile.NoTTY))
	}

	// Nothing judged yet, which is the first thing anybody who presses the key
	// will see. It stopped being an empty list when judged() widened to the whole
	// store: nine bits draw here now, under one band, and that is the point of the
	// widening rather than a fixture that got away. The name went with the
	// caption — this frame is about a conversation nobody has voted in, and the
	// genuinely empty record is a different frame nobody reaches by pressing a key.
	unjudged := talk(sized(100, 30), 9)
	unjudged.rank()
	fmt.Fprint(out, screen(unjudged, "nine bits, none of them judged yet"))

	// The two states that are not bits, on this surface. The composer is here, so
	// both can happen while the list is up, and both are appended under it exactly
	// as they are under the transcript. What to look at is whether a block written
	// for the bottom of a conversation still reads at the bottom of a list.
	waiting := judging(sized(100, 30), 24)
	waiting.rank()
	fmt.Fprint(out, screen(waitingOn(waiting, 27*time.Second), "ranked, with a reply in flight"))

	fmt.Fprint(out, screen(broken(waiting, notice{
		problem: "ollama is not answering at http://localhost:11434 — it does not appear to be running",
		fix:     "start it with: ollama serve",
	}), "ranked, after a failure"))

	// A scar in the list. It gets there the way it gets there in life: the caret
	// follows its bit into the fold that absorbed it, and the next key pressed is
	// a vote.
	m := talk(sized(100, 30), 9)
	m = back(m, 6)
	m.say(localHandle, "and one more thing")
	for range fixtureBudget {
		m.say(localHandle, "carrying on")
	}
	m.vote(memory.Up)
	fmt.Fprint(out, screen(m, "the caret has followed its bit onto a scar, and voted"))
	m.rank()
	fmt.Fprint(out, screen(m, "...which is a row here"))
	fmt.Fprint(out, profiled(m, "...and with no colour at all", colorprofile.NoTTY))
}

// The frame the fade is decided on, in a profile that has colour and one that
// has none at all.
//
// A window a hold has split into three pieces: two runs the next fold takes with
// one bit standing between them that it will not. Colour used to be the only
// thing saying which was which, so the NoTTY capture of this frame was the
// argument for doing anything — every row in it was byte-identical to every
// other, and a fold arrived out of a screen that had said nothing.
//
// Two things to look at, and neither is assertable. Whether the step reads at a
// glance across two bands rather than one; and whether it still reads where the
// caret sits on a row that is going, which is where it costs something — the
// caret ends up hard against the vote mark instead of a space away from it.
//
// [TestTheFadeIsDrawnInSpaceAndNotOnlyInColour] is the assertion under this, and
// it reads this same arrangement.
func TestHarnessFade(t *testing.T) {
	if os.Getenv("HARNESS") == "" {
		t.Skip("set HARNESS=1")
	}
	out := os.Stdout

	for _, size := range [][2]int{{100, 30}, {80, 20}, {60, 14}} {
		w, h := size[0], size[1]

		// Twelve bits leaves the fold one write away and the window seven bits
		// long, so a hold four back from its end splits it into three. The caret
		// is left on the held bit, which is where the hand that pressed the key
		// leaves it.
		m := split(sized(w, h))
		fmt.Fprint(out, screen(m, "a window a hold has split in three"))
		fmt.Fprint(out, profiled(m, "the same frame with no colour at all", colorprofile.NoTTY))

		// The caret on a row that is going, which is the arrangement that costs
		// something: the caret keeps column 0 and the row steps out from under it.
		m = back(m, 2)
		fmt.Fprint(out, profiled(m, "...the caret moved onto a row that is going", colorprofile.NoTTY))

		// And the same conversation with nobody voting: one run, one step, which
		// is what the edge looks like before anybody has held anything.
		fmt.Fprint(out, profiled(talk(sized(w, h), 12), "nobody voted: one band, one step",
			colorprofile.NoTTY))

		// The frame the width was decided on, and the one where this channel does
		// all the work alone: a scar, and beneath it exactly one row that steps.
		// A scar counts toward [memory.View.Fold]'s size rule and cannot step, so
		// a run of two containing one spoken bit draws a single jogged row — which
		// is why the step is two columns and not one. What to look at is whether
		// that one row reads as belonging with the scar above it rather than with
		// the block below it.
		fmt.Fprint(out, profiled(lone(sized(w, h)), "a scar and one row that steps",
			colorprofile.NoTTY))
	}

	// Where the step stops surviving, walked from width 1 up on the same fixture
	// [TestTheFadeIsDrawnInSpaceAndNotOnlyInColour] asserts over, printed with the
	// rows either side of it. This is where that pin's number comes from, and it
	// is read off rendered rows rather than worked out from caretWidth — three
	// claims in this repository derived from a constant that way were each wrong.
	//
	// Two rows per width: the row as drawn, and the same bit drawn as though it
	// were staying. Above the floor the first is the second with the step's margin
	// columns taken out. Below it the row is being cut by the terminal at both ends
	// and there is no margin left to take.
	//
	// Every fixture the pin asserts over, because a floor that moved with the
	// fixture would be a measurement of the fixture — and these do move with it,
	// which is why each is pinned at its own value rather than at one number.
	for _, s := range []struct {
		name string
		m    Model
	}{
		{"a hold splitting the window", split(sized(80, 20))},
		{"nobody voting", talk(sized(80, 20), 12)},
		{"a scar the next fold will take", lone(sized(80, 20))},
	} {
		// The first row in the cut that somebody *spoke*. A scar in the cut is
		// drawn exactly as one that is staying, so measuring the floors on one
		// reports 0 for both and looks like the step never existed — which is what
		// this loop did for one run, on the one fixture put here to cover scars.
		f := s.m.frame()
		going := -1
		for i, b := range f.bits {
			if _, cold := b.Payload.(memory.Compaction); cold {
				continue
			}
			if f.absorbing[b.ID] {
				going = i
				break
			}
		}
		// Loud rather than an index panic three lines down. A fixture that stops
		// producing a cooling row has stopped being the fixture these numbers were
		// measured on, and that is the finding, not a crash inside a helper.
		if going < 0 {
			t.Fatalf("%s: nothing anybody said is cooling in this fixture, so there is no floor to measure", s.name)
		}

		steps, differs := 0, 0
		for width := 1; width <= 120; width++ {
			row, twin := first(bare(s.m, width))[going], first(flat(s.m, width))[going]
			if differs == 0 && row != twin {
				differs = width
			}
			if steps == 0 && row == stepped(twin) && row != twin {
				steps = width
			}
		}
		fmt.Fprintf(out, "\n── %s: differs at %d, steps at %d\n", s.name, differs, steps)
		for width := 1; width <= steps+2; width++ {
			row, twin := first(bare(s.m, width))[going], first(flat(s.m, width))[going]
			fmt.Fprintf(out, "   %3d │%s│  (staying: │%s│) same %v\n", width, row, twin, row == twin)
		}
	}
}

// What the frame costs, read off the frame.
//
// chrome is a constant and the temptation is to check it by arithmetic. Three
// claims in this repository's history were checked that way and all three came
// out one high, so this prints the rows a rendered frame actually has beside the
// rows the terminal actually has, at every size worth looking at and then some.
// Anything other than "rows: N of N" above the shortest terminals is a wasted
// row or an overflowing one.
func TestHarnessFits(t *testing.T) {
	if os.Getenv("HARNESS") == "" {
		t.Skip("set HARNESS=1")
	}
	out := os.Stdout

	for _, size := range [][2]int{{100, 30}, {80, 24}, {80, 20}, {60, 14}, {40, 10}, {40, 9}, {40, 8}, {40, 7}, {20, 5}} {
		m := talk(sized(size[0], size[1]), 20)
		rows := strings.Count(m.View().Content, "\n") + 1
		fmt.Fprintf(out, "%3dx%-3d → %2d rows drawn, %2d in the terminal, viewport %2d %s\n",
			size[0], size[1], rows, m.height, m.viewport.Height(),
			map[bool]string{true: "", false: "  ← does not fit"}[rows <= m.height])
	}
}

func TestHarnessShort(t *testing.T) {
	if os.Getenv("HARNESS") == "" {
		t.Skip("set HARNESS=1")
	}
	for _, h := range []int{12, 10, 9, 8, 1} {
		m := talk(sized(60, h), 32)
		m.unfold()
		fmt.Fprint(os.Stdout, screen(m, "unfolded"))
	}

	// The pending line and the failure block cost transcript rows, and a short
	// terminal is where a block that pushes the newest material off the top
	// stops being a nicety. Both are drawn at the bottom, so this is the frame
	// that says whether the thing the person is waiting on is still on screen.
	for _, h := range []int{14, 12, 10, 8} {
		fmt.Fprint(os.Stdout, screen(waitingOn(talk(sized(60, h), 9), 27*time.Second), "waiting"))
		fmt.Fprint(os.Stdout, screen(broken(talk(sized(60, h), 9), notice{
			problem: "ollama is not answering at http://localhost:11434 — it does not appear to be running",
			fix:     "start it with: ollama serve",
		}), "failed"))

		// A fragment costs no rows at all now — it is a bit like any other, and
		// this is the frame that says whether its mark still reads on a screen
		// with nothing to spare.
		d := talk(sized(60, h), 9)
		d.recordReply(persona.Answer{Text: "the three steps are, first,", Truncated: true})
		fmt.Fprint(os.Stdout, screen(d, "a reply that ran out of room"))
	}
}

// reply is the founder's own case, reconstructed: a model asked for a Go program
// and answering the way models answer — headings, a bulleted list, bold spans, a
// fenced block of thirty-odd lines, and a second fence for the output. Before the
// document path existed this arrived on screen as one unbroken sentence with the
// whole program flattened into it, which is the report that ordered this work.
const reply = "### Reversing a slice in place\n\n" +
	"There are three things worth saying about this:\n\n" +
	"- it swaps from **both ends inward**, so it allocates nothing\n" +
	"- it is generic over `T`, so one function covers every element type\n" +
	"- the loop condition is `i < j`, not `i != j`, which matters for odd lengths\n\n" +
	"```go\npackage main\n\nimport \"fmt\"\n\n" +
	"// reverse turns s around in place. It allocates nothing and it is\n" +
	"// stable for the empty slice and for a slice of one.\n" +
	"func reverse[T any](s []T) {\n" +
	"\tfor i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {\n" +
	"\t\ts[i], s[j] = s[j], s[i]\n" +
	"\t}\n}\n\n" +
	"func main() {\n" +
	"\txs := []int{1, 2, 3, 4, 5}\n" +
	"\treverse(xs)\n" +
	"\tfmt.Println(xs)\n\n" +
	"\tnames := []string{\"ada\", \"grace\", \"alan\"}\n" +
	"\treverse(names)\n" +
	"\tfmt.Println(names)\n" +
	"}\n```\n\n" +
	"Running it prints:\n\n" +
	"```text\n[5 4 3 2 1]\n[alan grace ada]\n```\n\n" +
	"The generic version compiles to the same code the `[]int` version would.\n"

// TestHarnessDocument prints the one bit the caret has opened, at the two sizes
// the argument is made on: a hundred columns, where the code block has room to
// read, and a narrow terminal where the wrap starts to bite. Both in TrueColor
// and with every colour taken away, because the claim this style makes is that
// the structure is carried by marks and space and that colour is spending
// nothing load-bearing.
func TestHarnessDocument(t *testing.T) {
	if os.Getenv("HARNESS") == "" {
		t.Skip("set HARNESS=1")
	}

	for _, size := range [][2]int{{100, 30}, {46, 30}} {
		m := talk(sized(size[0], size[1]), 4)
		m.say(memory.Handle{Ref: "ollama/llama3", Display: "coordinator-7"}, reply)
		fmt.Fprint(os.Stdout, screen(m, "a document under the caret"))
		fmt.Fprint(os.Stdout, profiled(m, "the same document, no colour", colorprofile.NoTTY))

		// The caret moved off it, which is the frame nearly every row on this
		// surface is: the document is one row again, and what that row says is
		// [lede]'s. It read `### Reversing a slice in place There are three things
		// worth saying about this: - it swaps from **both en…` until this pass —
		// the source of a document rather than anything anybody said.
		m.move(-1)
		fmt.Fprint(os.Stdout, screen(m, "the same document, one row, caret above it"))
		fmt.Fprint(os.Stdout, profiled(m, "the one row, no colour", colorprofile.NoTTY))
	}
}

// TestHarnessDocumentCooling is the document on a row the next fold is taking,
// which is the frame two channels have to survive at once: the block drops every
// colour it has ([markdown]'s quiet arm) and steps [step] columns left as one
// object, and the row above it that is staying does neither.
func TestHarnessDocumentCooling(t *testing.T) {
	if os.Getenv("HARNESS") == "" {
		t.Skip("set HARNESS=1")
	}

	for _, size := range [][2]int{{100, 30}, {46, 30}} {
		m := sized(size[0], size[1])
		m.say(memory.Handle{Ref: "ollama/llama3", Display: "coordinator-7"}, reply)
		// Up to the trigger and not past it, so the document is still in the view
		// and the lookahead has already named it as the next fold's. Counted
		// against the load rather than against a number of bits, because the
		// document is charged half a screen ([Model.rows]) and the trigger arrives
		// far sooner than it used to — a fixed count here folded the document away
		// and left this frame with no document in it at all.
		handles := []memory.Handle{{Ref: "local", Display: "me"}, {Ref: "ollama/llama3", Display: "coordinator-7"}}
		for i := 0; m.foldable() < m.budget(); i++ {
			m.say(handles[i%2], lines[i%len(lines)])
		}

		// Walk the caret back onto the document so the block is drawn whole while
		// it cools; without that it is one row and the fade has nothing to carry.
		for range len(m.shown.Bits(m.store)) {
			at := m.caret()
			if b, ok := m.store.Get(at); ok {
				if u, said := b.Payload.(memory.Utterance); said && structured(u.Text) {
					break
				}
			}
			m.move(-1)
		}
		fmt.Fprint(os.Stdout, screen(m, "a cooling document"))
		fmt.Fprint(os.Stdout, profiled(m, "a cooling document, no colour", colorprofile.NoTTY))

		// And one more thing said, which is the fold. It arrives here because the
		// document is charged half a screen rather than one row: measured on this
		// conversation at 100x30, the first fold used to wait for 24 writes and now
		// comes after 14.
		m.say(memory.Handle{Ref: "local", Display: "me"}, "so the budget is rows now")
		fmt.Fprint(os.Stdout, screen(m, "the fold that used to wait ten more writes"))
	}
}

// TestHarnessDocumentFloor sweeps the width a document is drawn into against the
// plain wrap it replaced, in rows. It is the sweep [markdown]'s own doc cites for
// the negative result that there is no width at which refusing to render is
// better — the document costs rows at every width and carries structure at every
// width, and where it finally becomes rubble the wall is rubble and more of it.
func TestHarnessDocumentFloor(t *testing.T) {
	if os.Getenv("HARNESS") == "" {
		t.Skip("set HARNESS=1")
	}
	m := talk(sized(100, 30), 4)
	m.say(memory.Handle{Ref: "ollama/llama3", Display: "coordinator-7"}, reply)
	f := m.frame()

	// The plain arm is [wrapped], which is what the fallback actually is. It used
	// to be the flatten spelled out here inline, and that stopped being true the
	// day the fallback started keeping a message's own lines — a comparison
	// against a renderer nobody uses answers a question nobody asked.
	fmt.Fprintln(os.Stdout, "── the document path against the plain wrap, by width ──")
	for w := 1; w <= 40; w++ {
		lines, doc := markdown(reply, w, false)
		_ = f
		fmt.Fprintf(os.Stdout, "width %3d  document=%-5v rows=%3d   plain rows=%3d\n",
			w, doc, len(lines), len(wrapped(reply, w)))
	}
}

// pasted is a person's own text arriving through the composer with no markdown
// in it anywhere: Go source, tabs and blank lines and all. It is the material
// the founder actually pasted, cut to the height a fixture wants — memory's own
// Utterance and the method under it.
//
// There is no fence, no heading and no list item in it, so [structured] says
// prose and it takes [wrapped] rather than [markdown]. That is the point of
// having it here: the plain path is a drawing path now, with an indent, a wrap
// and a blank row in it, and until this fixture existed there was no frame of it
// anywhere.
var pasted = strings.Join([]string{
	"// Utterance is something said: the hot, uncompacted form of a bit's content.",
	"type Utterance struct {",
	"\t// Text is what was said, as said.",
	"\tText string",
	"",
	"\t// Truncated marks an utterance whose speaker did not get to finish — a",
	"\t// model that ran out of context room mid-sentence, not one that stopped",
	"\t// because it was done.",
	"\tTruncated bool",
	"}",
	"",
	"func (u Utterance) kind() string {",
	"\tif u.Truncated {",
	"\t\treturn \"fragment\"",
	"\t}",
	"\treturn \"utterance\"",
	"}",
}, "\n")

// TestHarnessPaste is the founder's paste under the caret, at the two sizes this
// surface is looked at, in colour and with none.
//
// What to look at, in order: the lines are the speaker's own lines; the struct
// body is indented four columns from the type above it; the blank line between
// the fields is a blank row; at 46 the long comment lines wrap and every
// continuation carries the indentation of the line it continues. Before this
// pass the whole thing was one paragraph with every newline and every tab spent.
//
// And the row above the caret's is what a person sees for the same bit when the
// caret is not on it — one row, [lede]'s, whitespace collapsed like every other
// cut on a row that has no second line.
func TestHarnessPaste(t *testing.T) {
	if os.Getenv("HARNESS") == "" {
		t.Skip("set HARNESS=1")
	}

	for _, size := range [][2]int{{100, 30}, {46, 30}} {
		m := talk(sized(size[0], size[1]), 4)
		m.say(memory.Handle{Ref: "local", Display: "me"}, pasted)
		fmt.Fprint(os.Stdout, screen(m, "a paste with no markdown in it, under the caret"))
		fmt.Fprint(os.Stdout, profiled(m, "the same paste, no colour", colorprofile.NoTTY))

		m.move(-1)
		fmt.Fprint(os.Stdout, profiled(m, "the same paste, one row, caret above it", colorprofile.NoTTY))
	}
}
