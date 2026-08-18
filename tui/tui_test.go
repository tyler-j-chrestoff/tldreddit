package tui

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/tyler-j-chrestoff/tldreddit/memory"
	"github.com/tyler-j-chrestoff/tldreddit/persona"
)

// fixtureBudget and fixtureKeep are what [New]'s own terminal sets, and they are
// what most fixtures below mean when they say "fill the view".
//
// They were two constants in the package until the budget became a function of
// the screen ([Model.budget]), and every use of them here meant "the number this
// surface folds at". That number now depends on the terminal, so it is read off a
// Model rather than written down — [New] is 80x24, and asking it is the only way
// these stay true when [chrome] moves.
//
// **Named differently from anything in the package on purpose.** This file's own
// `lines` once caught a mutation of `tui.go` and made it a silent no-op, because
// a package-scope test identifier resolves in the test build and not in
// `go build`. Calling these `coolAt` and `keepHot` would rebuild that trap for
// exactly the constants this change removed.
//
// fixtureKeep is half the budget and not [Model.keep], because [record] writes
// every bit under one handle: the round-aware search finds the human at the first
// place it looks, so keep is the plain half in every fixture built that way. A
// fixture with two speakers in it must ask the model rather than use this.
var (
	fixtureBudget = New().budget()
	fixtureKeep   = New().budget() / 2
)

// record builds a view of n bits under the local handle.
//
// It goes through say rather than send, and that is deliberate rather than
// incidental. send now also asks a persona and holds the next send until the
// answer arrives, so a helper built on it would drive exactly one bit into the
// record and would tie every test below to whether a machine has ollama
// running. These tests are about the record and the screen. What send itself
// does is [TestSendAsksAndHoldsTheNext] and the tests around it.
func record(n int) Model {
	m := New()
	for i := range n {
		m.say(localHandle, fmt.Sprintf("bit %d", i))
	}
	return m
}

// shot is the transcript this model would draw, at a width and an openness of
// the test's choosing and with everything else — the fade, the holds, the votes,
// the caret, the clock — resolved by [Model.frame], the way the program resolves
// it. A test that built its own frame would be looking at an arrangement nobody
// is running.
func shot(m Model, width int, open bool) string {
	f := m.frame()
	f.width, f.open = width, open
	body, _ := transcript(f)
	return body
}

// receiptOf is the block a scar's receipt draws, at a width of the test's
// choosing and from this model's own frame — so the votes, the clock and the
// store are the ones the program would hand it.
func receiptOf(m Model, c memory.Compaction, width int) string {
	f := m.frame()
	f.width = width
	return unfold(f, c)
}

// atNine is the day the hand-built fixtures below happen on, so their stamps
// read as the four digits a conversation inside one day gets. The zero clock
// would print a full date on every row, which is [clock]'s deliberate alarm for
// a caller that never said which day it is — correct there, and noise here.
var atNine = clock{ref: time.Date(2026, 8, 11, 9, 0, 0, 0, time.UTC)}

// Sending past the limit must fold on its own. If pressure only ever released
// when a human pressed a key, the record would grow without bound exactly when
// they were too busy to notice.
func TestSendFoldsAtLimit(t *testing.T) {
	m := record(fixtureBudget)
	if got := m.foldable(); got != fixtureBudget {
		t.Fatalf("hot = %d after %d sends, want no fold yet", got, fixtureBudget)
	}

	// Through send, so the path a person actually presses is the one under
	// test: the composer empties, the bit lands, and the fold fires on write.
	m.composer.SetValue("one too many")
	m.send()

	if got := m.composer.Value(); got != "" {
		t.Errorf("composer still holds %q after a send", got)
	}
	if got := m.foldable(); got != fixtureKeep {
		t.Errorf("hot = %d after fold, want %d", got, fixtureKeep)
	}
	// The fold is at the head, because nothing split it. That is a property of
	// this fixture rather than an invariant — D32 ended the invariant, a single
	// upvote makes a view that begins with a hot row, and every test below that
	// votes reads what a fold took through [took] instead.
	shown := m.shown.Bits(m.store)
	if _, cold := shown[0].Payload.(memory.Compaction); !cold {
		t.Errorf("shown[0] = %T, want the fold at the head of a view nobody voted in", shown[0].Payload)
	}
}

// The budget is the screen's own height, and nothing about where the caret is
// parked can move it.
//
// Both halves are the point and the second is the one with teeth. Every bit on
// this surface draws as one row except the one the caret is on, which is drawn
// whole — so a budget that counted rows as drawn would make a fold something a
// person causes by pressing an arrow key, which is memory forgetting because of a
// navigation gesture and is the thing this whole surface exists to prevent. The
// fixture puts a reply on the record long enough to draw many rows and then walks
// the caret over it.
//
// The heights are the ones [TestHarnessFits] prints, and what is asserted about
// them is that the budget rises with the terminal rather than any particular
// number, because [chrome] can move and this is not the pin for it.
func TestTheBudgetIsTheScreenAndTheCaretCannotMoveIt(t *testing.T) {
	long := strings.Repeat("the schema drift is in three columns and nobody backfilled them, ", 8)

	last := 0
	for _, h := range []int{24, 30, 40, 60, 80} {
		m := sized(100, h)
		if got := m.budget(); got <= last {
			t.Errorf("a terminal %d rows tall budgets %d, and a shorter one budgeted %d — a taller screen must not hold less",
				h, got, last)
		}
		last = m.budget()
	}

	m := sized(100, 30)
	m = talk(m, 6)
	m.utter(m.persona.Handle(), memory.Utterance{Text: long})
	m = talk(m, 4)

	want, pressure := m.budget(), m.foldable()
	tall := 0
	for range len(m.shown) {
		m.move(-1)
		if rows := m.anchors.rows; rows > tall {
			tall = rows
		}
		if got := m.budget(); got != want {
			t.Fatalf("the caret moved and the budget went from %d to %d", want, got)
		}
		if got := m.foldable(); got != pressure {
			t.Fatalf("the caret moved and the pressure went from %d to %d", pressure, got)
		}
	}
	if tall < 2 {
		t.Fatalf("the caret never landed on a row taller than %d line(s), so nothing here was at risk", tall)
	}
}

// The budget is the largest number of rows the frame will show, and this is the
// pin for the value rather than for its shape.
//
// The test above asserts that the budget rises with the terminal and that the
// caret cannot move it. Both are real and **neither pins the number**: a review
// found four mutants that survive every other check here — the viewport's rows
// minus two, the terminal's rows minus nine, the terminal's rows with no chrome
// subtracted at all, and the viewport's rows plus three. Two accidents were
// catching some of them, a column-width fixture whose constants moved for an
// unrelated reason and a vacuity guard doing an assertion's work, and an accident
// is not a pin.
//
// Stated on the drawn frame in both directions, because one direction is not a
// pin either: a budget that is too small still *fits*, and a budget that is too
// large still rises with the terminal. Exactly the budget must leave nothing off
// screen, and one more than the budget must not.
//
// The over-full view is built at a taller terminal and then resized down, which
// is the only way to hold a view past its budget still enough to look at — a
// write would fold it. That leans on nothing folding on a resize
// ([TestResizingTheTerminalNeverFoldsTheRecord]), which is the neighbouring pin,
// and it is stated here so the dependency is not silent.
//
// Only above the floor. Below it [coolFloor] governs and a view of twelve cannot
// fit in seven rows by construction; that half is
// [TestTheBudgetNeverFallsBelowWhatThisSurfaceAlwaysFoldedAt].
func TestTheBudgetIsExactlyWhatTheFrameWillShow(t *testing.T) {
	checked := 0
	for _, h := range []int{20, 24, 30, 40, 60} {
		at := sized(100, h)
		if at.viewport.Height() < coolFloor {
			continue
		}
		checked++
		want := at.budget()

		// Filled at a terminal ten rows taller, where the budget is ten higher, so
		// nothing folds while it is being built.
		fill := func(n int) Model {
			m := sized(100, h+10)
			for i := range n {
				m.say(localHandle, fmt.Sprintf("bit %d", i))
			}
			if m.scars() != 0 {
				t.Fatalf("%d bits folded while being built at %d rows, so this is not a view of %d", n, h+10, n)
			}
			mm, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: h})
			m = mm.(Model)
			if len(m.shown) != n {
				t.Fatalf("the resize to %d rows changed the view from %d bits to %d", h, n, len(m.shown))
			}
			return m
		}

		if a, b := fill(want).offscreen(); a+b != 0 {
			t.Errorf("a terminal %d rows tall budgets %d, and %d bits already run off the frame by %d row(s)",
				h, want, want, a+b)
		}
		if a, b := fill(want + 1).offscreen(); a+b == 0 {
			t.Errorf("a terminal %d rows tall budgets %d, and %d bits still fit — the frame holds more than the budget claims",
				h, want, want+1)
		}
	}
	if checked == 0 {
		t.Fatal("no height in the sweep was above the floor, so the budget's own value was never checked")
	}
}

// No terminal is worse off than it was before the budget existed. Below the
// height that sets one, the floor is what this surface folded at from the
// beginning, at every size.
//
// The number below is deliberately not re-derived from [chrome]: it is read off
// the model, so a shorter terminal proving out at the floor is the assertion and
// the arithmetic that gets there is not.
func TestTheBudgetNeverFallsBelowWhatThisSurfaceAlwaysFoldedAt(t *testing.T) {
	floored := 0
	for h := 1; h <= 60; h++ {
		m := sized(100, h)
		if got := m.budget(); got < coolFloor {
			t.Fatalf("a terminal %d rows tall budgets %d, under the floor of %d", h, got, coolFloor)
		}
		if m.budget() == coolFloor {
			floored++
		}
		if got := m.keep(); got < 1 || got > m.budget() {
			t.Fatalf("a terminal %d rows tall keeps %d of a budget of %d", h, got, m.budget())
		}
	}
	if floored == 0 {
		t.Fatal("no terminal in the sweep landed on the floor, so the floor is not being exercised")
	}

	// And the cut only ever moves in the direction that keeps more. Moving it
	// forward would tidy the same boundary by absorbing the orphan instead, and
	// it would take a bit the plain half kept — possibly the one under the caret.
	// A fixture that alternates, because [record] writes one handle and the cut
	// never moves there at all.
	//
	// The floor is counted in rows, which is the change [Model.rows] made and the
	// reason this loop was rewritten rather than retuned: half a *budget* is half
	// a screen of rows, and the kept tail is measured against it in the same unit.
	// A view holding fewer rows than that keeps all of them, which is the
	// direction this check is about — the one that keeps more.
	moved := 0
	for n := 2; n <= 60; n++ {
		m := talk(sized(100, 30), n)
		bits, room := m.shown.Bits(m.store), m.room()

		rows := func(k int) int {
			n := 0
			for _, b := range bits[max(len(bits)-k, 0):] {
				n += m.rows(b, room)
			}
			return n
		}

		// The floor cannot demand more than D32's size rule allows: a keep that
		// leaves fewer than two bits to fold is refused in silence, so the cut is
		// clamped there and the floor is clamped with it. That clamp is what a
		// budget in rows made reachable — a view can now be large in rows and small
		// in bits — and it is [keepFrom]'s, stated once.
		k := m.keep()
		floor := min(m.budget()/2, rows(max(len(bits)-2, 1)))
		if got := rows(k); got < floor {
			t.Fatalf("%d bits: the cut keeps %d rows, under a floor of %d on a budget of %d — the cut moved forward",
				n, got, floor, m.budget())
		}
		if k > m.budget()/2 && rows(k-1) >= m.budget()/2 {
			moved++
		}
	}
	if moved == 0 {
		t.Fatal("the cut never moved anywhere in the sweep, so its direction is not being checked")
	}
}

// The gauge is the antecedent for a fold, so what it is filling toward has to be
// what the fold is waiting for. Those were one constant and are now two
// expressions, which is exactly the shape that drifts.
//
// Read off the drawn footer rather than off [Model.budget], because the number a
// person acts on is the one on the screen.
// The sweep has to cross the floor, and the first version of it did not. Below
// the height where the viewport is [coolFloor] rows the denominator stops being
// the viewport and becomes the floor, so a gauge that dropped the floor from its
// own arithmetic — and only from its own — passed a sweep of tall terminals
// while a short one filled toward 7 against a fold waiting for 12. The heights
// below are checked to land on both sides of it rather than assumed to.
func TestTheGaugeFillsTowardTheBudgetItIsWaitingOn(t *testing.T) {
	seen := map[string]bool{}
	sides := map[bool]int{}
	for _, h := range []int{10, 14, 18, 19, 24, 30, 50} {
		m := talk(sized(100, h), 8)
		want := fmt.Sprintf("%d/%d", m.foldable(), m.budget())
		got := ansi.Strip(m.footer())
		if !strings.Contains(got, want) {
			t.Errorf("a terminal %d rows tall draws a footer reading %q, with no %q in it", h, got, want)
		}
		seen[want] = true
		sides[m.viewport.Height() < coolFloor]++
	}
	if len(seen) < 2 {
		t.Fatalf("every terminal in the sweep showed the same gauge %v, so a constant would pass this", seen)
	}
	if sides[true] == 0 || sides[false] == 0 {
		t.Fatalf("the sweep is all on one side of the floor (%v), so whichever half of the denominator it does not reach is unchecked", sides)
	}
}

// The footer says when it has stopped naming keys, at every width, and says
// nothing when it has not.
//
// This is the one place on this surface where material left the screen with no
// receipt. Every other cut here is marked — [clip] and [said] put an ellipsis on
// a cut row, a fragment degrades to a dash — while the footer dropped four whole
// bindings at eighty columns and looked exactly like a footer that had them all.
// A key that is not printed and a key that does not exist are the same picture,
// and this surface's own package doc calls that the one thing it may not do.
//
// Both directions, and the second one is what stops the mark becoming wallpaper:
// there has to be a width where the whole index is drawn *without* a mark, or the
// mark is on in every frame anybody ever sees and says nothing.
//
// The bottom rung is the mechanism D59(k) named. Every ladder used to end in "",
// which fits any width, so [fit] returned an empty row before it could ever reach
// its own ellipsis — the narrowest footer on this surface was a blank claiming
// the screen had nothing to say about its keys.
func TestTheFooterSaysWhenItHasDroppedAKey(t *testing.T) {
	widest := "enter send · shift+↑/ctrl+o keep · shift+↓/ctrl+r let go · ctrl+u unfold · ctrl+c quit"

	complete, abridgedAt, bare := 0, 0, 0
	for width := 1; width <= 200; width++ {
		m := talk(sized(width, 24), 8)
		got := ansi.Strip(m.footer())
		keys, _, _ := strings.Cut(got, "▓")
		keys, _, _ = strings.Cut(keys, "░")
		keys = strings.TrimSpace(keys)

		switch {
		case keys == asDrawn(m, widest):
			complete++
			if strings.Contains(keys, "…") {
				t.Errorf("the complete index at width %d is marked as cut: %q", width, keys)
			}
		case keys == "…":
			bare++
		case keys == "":
			t.Errorf("the footer at width %d names no key and does not say so: %q", width, got)
		case !strings.HasSuffix(keys, "…"):
			t.Errorf("the footer at width %d dropped keys with nothing saying so: %q", width, keys)
		default:
			abridgedAt++
		}
	}

	// Each of the three states has to occur, or whichever one does not is
	// unchecked and the assertions above are satisfied by an accident.
	if complete == 0 {
		t.Error("no width draws the whole index, so an unmarked footer is never checked and the mark is on in every frame")
	}
	if abridgedAt == 0 {
		t.Error("no width draws a marked rung, which is the whole subject of this test")
	}
	if bare == 0 {
		t.Error("no width falls all the way to the mark alone, so the rung the empty candidate used to hide is unchecked")
	}
}

// asDrawn is [Model.surfaced] over one rung, for a test that needs to know what
// the widest rung reads as on the surface that is up.
func asDrawn(m Model, rung string) string { return m.surfaced([]string{rung})[0] }

// The word `held` on the footer means somebody is holding something. Nothing
// else on this surface may produce it.
//
// It shipped able to, for a review, and the frame is the argument: at 100x30
// with one bit from the human and the rest from agents, the footer read
// `24/23 held` on a record with no ballot in it, over fourteen rows drawn
// cooling — the screen promising a fold and denying it at once, and a person who
// believed the word going to look for a vote they never cast. The cause was
// [Model.keep]'s ceiling reaching the budget exactly, which makes the fold's
// window one bit, which D32's size rule refuses.
//
// Swept over where the human's one turn falls, because that position is the whole
// of it, and over three terminal heights so the budget is not one number. The
// positive direction — `held` on a view that genuinely is held — is
// [TestHoldingEveryOtherBitBlocksTheFoldAndLettingGoReleasesIt], which is why
// this one only has to say when the word is wrong.
func TestNothingOnScreenSaysHeldUnlessSomethingIsHeld(t *testing.T) {
	bot := memory.Handle{Ref: "ollama/qwen3.5:latest", Display: "qwen3.5"}

	full := 0
	for _, h := range []int{20, 24, 30} {
		for at := range 40 {
			m := sized(100, h)
			for i := range 40 {
				who := bot
				if i == at {
					who = localHandle
				}
				m.say(who, lines[i%len(lines)])

				// The frame is at its limit. Counted rather than pressure,
				// because pressure is resolved inside the write that produced it
				// — [Model.utter] folds and it is gone — and what stays true
				// afterwards is a view sitting at the edge, which is where this
				// goes wrong.
				if m.foldable() >= m.budget() {
					full++
				}
				if !m.blocked() {
					continue
				}
				if len(m.live()) == 0 {
					t.Fatalf("%dx%d, the human's turn at %d, after %d bits: blocked with %d votes and nothing held; footer %q",
						100, h, at, i+1, len(m.votes), ansi.Strip(m.footer()))
				}
			}
		}
	}
	if full == 0 {
		t.Fatal("no frame in the sweep ever reached its budget, so the state this is about was never entered")
	}
}

// The fold's window is never one bit, which is the mechanism under the test
// above rather than the thing a person sees.
//
// D32 refuses a run of one, so a cut of one bit is a fold that cannot happen —
// and on this surface a refused fold is drawn as a hold. Stated over the two
// numbers rather than over the footer, so a change that keeps the word right by
// some other route still has to say what it did about this.
//
// Asserted over the view rather than over [Model.pressured], and the difference
// is what makes this a check rather than a check-shaped thing: pressure is
// resolved inside the write that raises it — [Model.utter] folds and it is gone —
// so a loop that waits to see it sees nothing at all, and the first version of
// this passed its own vacuity guard by failing it.
//
// The precondition is a view longer than its budget, which is the state a fold is
// decided in and is reachable on every frame at the top of the cycle, since a
// scar takes a row and does not count toward [Model.foldable]. Without it this
// asserts about views far under the budget where the base is simply longer than
// the view — true, uninteresting, and it fires at seven bits on a budget of
// thirteen, where no fold is attempted and nothing is drawn.
func TestTheFoldsWindowIsNeverASingleBit(t *testing.T) {
	bot := memory.Handle{Ref: "ollama/qwen3.5:latest", Display: "qwen3.5"}

	narrow, over := 0, 0
	for _, h := range []int{20, 24, 30} {
		for at := range 40 {
			m := sized(100, h)
			for i := range 40 {
				who := bot
				if i == at {
					who = localHandle
				}
				m.say(who, lines[i%len(lines)])
				if len(m.shown) <= m.budget() {
					continue
				}
				over++
				if window := len(m.shown) - m.keep(); window == 1 {
					t.Fatalf("%dx%d, the human's turn at %d: the view is %d bits and keep is %d, so the fold's window is one bit and no fold can take it",
						100, h, at, len(m.shown), m.keep())
				} else if window == 2 {
					narrow++
				}
			}
		}
	}
	if over == 0 {
		t.Fatal("no view in the sweep ever ran past its budget, so the state a fold is decided in was never entered")
	}
	if narrow == 0 {
		t.Fatalf("%d views ran past their budget and none came within one bit of the refusal, so the bound is not being approached", over)
	}
}

// A fold never keeps more than the screen holds.
//
// This is [Model.keep]'s ceiling, and it needed a fixture with votes in it,
// because with nobody voting the ceiling is unreachable: the view sits at
// budget+2 when the trigger fires (one scar, which does not count toward
// [Model.foldable]), so [keepFrom]'s own `len-2` clamp lands on exactly the same
// number and removing the ceiling changes nothing at all — measured at seven
// human-bit rates, byte-identical output. Holds are what push the view past
// budget+2 and let the two bounds differ.
//
// **What removing it does is not what the first version of this test said.** It
// was written expecting a storm — a keep so large the fold takes two bits and
// fires again on the next write. Measured, the opposite: without the ceiling the
// search runs further back, keeps more, and the fold takes *less*, so folds over
// 400 bits went 19 → 6 and the worst view 275 → 300. A fold that stops taking
// anything, not one that fires constantly. The rate-based check that mistake
// produced is gone, because it was measuring for a direction that does not
// happen.
//
// Asserted on the two numbers rather than on a rate, which is what the ceiling
// actually says and needs no fixture-shaped constant to be true.
func TestAFoldNeverKeepsMoreThanTheScreenHolds(t *testing.T) {
	bot := memory.Handle{Ref: "ollama/qwen3.5:latest", Display: "qwen3.5"}

	atTheBound := 0
	for _, h := range []int{20, 24, 30, 50} {
		m := sized(100, h)
		for i := range 200 {
			who := bot
			if i%40 == 1 {
				who = localHandle
			}
			m.say(who, lines[i%len(lines)])
			if i%3 == 0 {
				m.vote(memory.Up)
			}
			if got := m.keep(); got > m.budget() {
				t.Fatalf("100x%d after %d bits: keep is %d on a budget of %d, so a fold would leave the view over its budget",
					h, i+1, got, m.budget())
			} else if got == m.budget() {
				atTheBound++
			}
		}
	}
	if atTheBound == 0 {
		t.Fatal("keep never reached the budget anywhere in the sweep, so the bound this pins is not being approached")
	}
}

// Dragging a window is not a memory operation.
//
// The budget moves with the terminal, so making one shorter can leave a view
// standing over its budget — and the honest thing at that moment is a full gauge
// and a screen full of rows drawn cooling, not a fold. What folds a record here
// is somebody saying something, and this is what keeps that true: the fade is the
// antecedent, and it has to arrive before the fold rather than with it.
func TestResizingTheTerminalNeverFoldsTheRecord(t *testing.T) {
	m := talk(sized(100, 60), 30)
	if m.pressured() {
		t.Fatal("the fixture is already over its budget on the tall terminal")
	}
	before := slices.Clone(m.shown)

	mm, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	m = mm.(Model)

	if !m.pressured() {
		t.Fatal("the shorter terminal is not over its budget, so this asserts nothing")
	}
	if !slices.Equal(m.shown, before) {
		t.Fatalf("the view went from %d bits to %d because the window was resized", len(before), len(m.shown))
	}
	if len(m.absorbing()) == 0 {
		t.Error("nothing is drawn cooling on the frame after the resize, so the fold that is now one keystroke away has no antecedent")
	}

	m.say(localHandle, "and now something happens")
	if len(m.shown) >= len(before) {
		t.Errorf("the view is %d bits after a write that should have folded it, from %d", len(m.shown), len(before))
	}
}

// A fold never cuts between a question and the answer to it.
//
// [memory.View.Fold] cuts at a count and has no notion of a round, so the
// boundary lands mid-exchange about as often as not: measured on this surface
// before [Model.keep] moved it, 24 of 60 frames at 100x30 held a round with one
// half of it behind a scar, and in every one of those the head of the view was a
// reply to a question nobody could see. Parity was all that decided it.
//
// Nobody votes here, and that is a real limit rather than a convenience. A hold
// splits the fold at the held bit, which is a cut this surface does not choose
// and cannot move — upvote an answer and its question is folded out from under
// it. That is [memory.View.Fold]'s rule and it is stated in docs/DEBT.md rather
// than asserted here, because a test that failed on it would be asserting against
// a decision this package does not own.
func TestAFoldNeverCutsBetweenAQuestionAndItsAnswer(t *testing.T) {
	m := sized(100, 30)
	bot := memory.Handle{Ref: "ollama/qwen3.5:latest", Display: "qwen3.5"}

	type round struct{ q, a string }
	var rounds []round
	folds := 0

	for r := range 60 {
		m.say(localHandle, lines[(2*r)%len(lines)])
		q := m.shown[len(m.shown)-1]
		was := len(m.shown)
		m.say(bot, lines[(2*r+1)%len(lines)])
		if len(m.shown) <= was {
			folds++
		}
		rounds = append(rounds, round{q, m.shown[len(m.shown)-1]})

		in := map[string]bool{}
		for _, id := range m.shown {
			in[id] = true
		}
		for i, rd := range rounds {
			if in[rd.q] != in[rd.a] {
				t.Fatalf("after round %d, round %d is half on screen: question %v, answer %v",
					r, i, in[rd.q], in[rd.a])
			}
		}
	}
	if folds < 3 {
		t.Fatalf("only %d fold(s) in 60 rounds, so the cut this is about barely happened", folds)
	}
}

// A fold must leave the record walkable. This is D1 seen from the surface: the
// screen has dropped bits, and every edge on screen still resolves, because
// dropping happened in the view and not in the store.
//
// Both views, and that is the half of it a vote surface adds. D14 defines
// reachable as discoverable by walking Prev and Absorbed from the view a reader
// holds — and a reader here holds two, because a vote is a bit that nothing in
// the transcript points at. Walking from the transcript alone, every vote cast
// on this screen is an orphan: still filed, still addressed, reachable by
// nobody. That is exactly the finding D34 records, and it was found by mutation
// rather than by reading, so it is asserted here rather than trusted.
func TestFoldKeepsGraphWalkable(t *testing.T) {
	m := record(fixtureBudget * 3)

	// Vote from where the caret actually is, through the key, so the votes under
	// test are the ones the program writes rather than ones a test built.
	for range 4 {
		m.vote(memory.Up)
		m.move(-3)
	}
	m.vote(memory.Down)
	if len(m.votes) == 0 {
		t.Fatal("no votes were cast, so the second view is not being exercised")
	}

	for _, v := range []memory.View{m.shown, m.votes} {
		for _, b := range v.Bits(m.store) {
			for _, p := range b.Prev {
				if _, ok := m.store.Get(p); !ok {
					t.Errorf("bit %s points at %s, which the store lost",
						memory.Short(b.ID), memory.Short(p))
				}
			}
		}
	}

	// And nothing at all is stranded. The count is the assertion: the store hands
	// out no iterator, deliberately, so what is checked is that the walk from
	// both views reaches as many distinct bits as the store holds.
	if got, want := len(reachable(m.store, m.shown, m.votes)), m.store.Len(); got != want {
		t.Errorf("walking both views reaches %d bits of %d in the record — %d are stranded",
			got, want, want-got)
	}

	// The mutation that made the assertion above worth writing: drop the vote
	// view and the same walk must come up short, or it is a check that cannot
	// fail (D27).
	if got := len(reachable(m.store, m.shown)); got >= m.store.Len() {
		t.Errorf("walking the transcript alone reached %d of %d bits, so the vote view is not carrying anything",
			got, m.store.Len())
	}
}

// reachable is every bit discoverable by walking Prev and Absorbed out from the
// views a reader holds — D14's definition, transplanted from memory's own
// reach_test.go because this package cannot call an unexported test helper in
// another one.
func reachable(s *memory.Store, views ...memory.View) map[string]bool {
	seen := map[string]bool{}

	var walk func(string)
	walk = func(id string) {
		if seen[id] {
			return
		}
		b, ok := s.Get(id)
		if !ok {
			return
		}
		seen[id] = true

		for _, p := range b.Prev {
			walk(p)
		}
		if c, cold := b.Payload.(memory.Compaction); cold {
			for a := range c.Absorbed() {
				walk(a)
			}
		}
	}

	for _, v := range views {
		for _, id := range v {
			walk(id)
		}
	}
	return seen
}

// Nothing may be lost across folds: every original bit is either still on
// screen or named on a receipt.
func TestFoldConservesCount(t *testing.T) {
	sends := fixtureBudget * 3
	m := record(sends)

	total := 0
	for _, b := range m.shown.Bits(m.store) {
		if c, cold := b.Payload.(memory.Compaction); cold {
			total += c.Count()
		} else {
			total++
		}
	}
	if total != sends {
		t.Errorf("view accounts for %d bits, want %d", total, sends)
	}
}

// The D1 guarantee, end to end through the surface: after enough folding to
// push most of the conversation off screen, every absorbed bit is still in the
// store under the ID its receipt names.
func TestFoldedBitsStayResolvable(t *testing.T) {
	m := record(fixtureBudget * 3)

	receipts := 0
	for _, b := range m.shown.Bits(m.store) {
		c, cold := b.Payload.(memory.Compaction)
		if !cold {
			continue
		}
		for id := range c.Absorbed() {
			receipts++
			if _, ok := m.store.Get(id); !ok {
				t.Errorf("receipt names %s, which the store does not hold", memory.Short(id))
			}
		}
	}
	if receipts == 0 {
		t.Fatal("no bits were folded, so the guarantee was not exercised")
	}
}

// took is what a write just absorbed: every scar it put on the view that was not
// there before, and everything named in its Prev — which is every bit in the
// window it folded, in window order (D13).
//
// A comparison of two views rather than a read of m.shown[0]. Two tests here
// read the head of the view as "the fold" and were right only because their
// fixtures cast no votes: D32 ended the invariant that a view holds at most one
// [memory.Compaction] and that it sits at index 0, and those fixtures now vote,
// so a fold can land anywhere in the view and can land more than once. The one
// place that still reads the head is [TestSendFoldsAtLimit], where it says so.
func took(m Model, before memory.View) map[string]bool {
	was := map[string]bool{}
	for _, id := range before {
		was[id] = true
	}

	out := map[string]bool{}
	for _, b := range m.shown.Bits(m.store) {
		if was[b.ID] {
			continue
		}
		if _, cold := b.Payload.(memory.Compaction); !cold {
			continue
		}
		for _, id := range b.Prev {
			out[id] = true
		}
	}
	return out
}

// The fade has to name the bits the *next* fold takes, not the bits a fold
// happening this instant would take. A send appends and then tests the band, so
// the cut is computed against a view one longer than the one last drawn: after
// fixtureBudget sends the next send folds seven, and the arithmetic that answered six
// let one bit go from full brightness to absorbed with no frame in between.
//
// Asserted as exact equality against what the fold then actually took, rather
// than against a number this test worked out for itself. A count would pass on
// the right number of the wrong bits, which is precisely what a set can be and a
// prefix could not.
func TestFadeMarksWhatIsNext(t *testing.T) {
	m := record(fixtureBudget)

	drawn := m.absorbing()
	if len(drawn) == 0 {
		t.Fatal("nothing was drawn cooling with a fold one send away")
	}

	before := slices.Clone(m.shown)
	m.say(localHandle, "one too many")

	if got := took(m, before); !maps.Equal(drawn, got) {
		t.Errorf("the frame before the fold drew %d bits cooling and the fold took %d; they must be the same bits",
			len(drawn), len(got))
	}
}

// The same promise with two speakers in the record, which is where the lookahead
// stopped being one subtraction.
//
// [Model.keep] moves the fold's cut back to a bit the human said, so the number
// of bits kept depends on who spoke near the boundary — and [Model.absorbing] has
// to name what the *next* write will fold, on a view one bit shorter than the one
// the fold will see. Running the same search one lower is what makes those agree
// exactly; lowering only its base, or subtracting one from [Model.keep] as if it
// were still a constant, both draw a cut the fold does not make. In the direction
// that matters that is a bit absorbed with no frame in between.
//
// Every fixture that reaches [Model.absorbing] elsewhere in this file is built by
// [record], which writes every bit under one handle — so the search finds the
// human at the first place it looks and the whole nudge is invisible there. This
// is the one that alternates.
// The second fixture is the one that reaches the search's *ceiling* rather than
// its base, and it took a mutation coming back green to find it. An alternating
// conversation puts a bit the human said every other row, so the search stops at
// the first place it looks and a ceiling one out either way is unreachable. A
// long stretch with nobody human in it is not exotic — a persona answering itself,
// or `tldr say` from agents — and there the search runs to the end of its range,
// which is where two ceilings differ and where the fade under-predicts.
func TestFadeMarksWhatIsNextWithTwoSpeakers(t *testing.T) {
	bot := memory.Handle{Ref: "ollama/qwen3.5:latest", Display: "qwen3.5"}

	for _, s := range []struct {
		name string
		// deep says this arm is the one that has to reach a view past the budget
		// plus two, which is where the two ceilings differ.
		deep bool
		fill func(m Model, n int) Model
	}{
		{name: "the two take turns", fill: func(m Model, n int) Model { return talk(m, n) }},
		{name: "the human said one thing and the agents carried on", fill: func(m Model, n int) Model {
			m.say(localHandle, "carry on without me")
			for i := range n - 1 {
				m.say(bot, lines[i%len(lines)])
			}
			return m
		}},
		// The third arm is here because a mutation came back green twice, and both
		// times the mutant was real and the fixtures could not reach it. The
		// lookahead's ceiling and the fold's differ only where the view is longer
		// than the budget plus two — and with nobody voting it never is, because at
		// the trigger the view is the budget plus one scar. **Holds are what push
		// it further**, so a fixture that does not vote cannot test the ceiling at
		// all, only the base.
		//
		// Every fourth bit kept, and the bits are written microseconds apart, so no
		// hold in here ever expires — which keeps this clear of the one hole
		// [Model.absorbing] concedes and makes the comparison exact rather than
		// nearly exact.
		//
		// Fourth and not third, which is a fact about the fold rather than a
		// nudge to make a test pass. A hold in memory now covers the bit its own
		// bit answers as well as itself, so one vote in three spares two rows in
		// every three and leaves free runs of one everywhere — D32's size rule
		// refuses all of them and this fixture folds not once in the whole sweep.
		// One in four is the densest voting that still folds. The cliff is real
		// and it is `memory`'s to decide about, not this file's; recorded here so
		// the next person to change this number knows what they are standing on.
		{name: "somebody has been keeping things", deep: true, fill: func(m Model, n int) Model {
			for i := range n {
				who := localHandle
				if i%2 == 1 {
					who = bot
				}
				m.say(who, lines[i%len(lines)])
				if i%4 == 0 {
					m.vote(memory.Up)
				}
			}
			return m
		}},
	} {
		deep := 0
		seen := 0
		for n := fixtureBudget; n <= fixtureBudget*3; n++ {
			m := s.fill(New(), n)

			drawn := m.absorbing()
			before := slices.Clone(m.shown)
			m.say(localHandle, "one too many")

			got := took(m, before)
			if len(got) == 0 {
				continue
			}
			seen++
			if len(before) > m.budget()+2 {
				deep++
			}
			if !maps.Equal(drawn, got) {
				t.Errorf("%s, %d bits: the frame before the fold drew %d cooling and the fold took %d; they must be the same bits",
					s.name, n, len(drawn), len(got))
			}
		}
		if seen == 0 {
			t.Fatalf("%s: no fold fired anywhere in the sweep, so nothing here was compared", s.name)
		}

		// The third arm exists to reach a view deeper than the budget plus two,
		// which is the only place the two ceilings differ, and until now the
		// counter that says whether it got there was computed and thrown away
		// (`_ = deep`). A fixture whose whole reason is to reach one state, with
		// nothing checking that it does, is a check that cannot fail: the vote
		// rate in that arm has already had to move once, and the next move could
		// take it out of range with every assertion above still green. Measured
		// when this was closed: 6 folds in that arm, all six deep.
		if s.deep && deep == 0 {
			t.Fatalf("%s: %d folds and not one of them on a view past the budget plus two, so the ceiling was never reached",
				s.name, seen)
		}
	}
}

// Nothing may be absorbed that was not drawn cooling in the frame before it
// went. That one sentence is the whole reason a fold firing on its own is not
// something happening behind the user's back, and it is the property the fade
// exists to deliver — so it is checked against every fold in a long record
// rather than against one hand-picked frame.
//
// It votes, and that is what makes it a test of this program rather than of the
// half of it nobody changed. The vote-free version of this loop could only ever
// exercise the one path where the fade is easy: a write appends, the window
// slides by one, and the frame before it named exactly that window. Every way
// the promise can actually break runs through a hold — a hold makes a bright row
// that the fold is not taking, and withdrawing one makes a foldable row that was
// bright a moment ago. So the human here keeps every fourth thing that is said
// and walks up the view to let one go every so often, which is the shape that
// blocks a fold and then frees it.
//
// Fourth and not third, and the number is a fact about the fold rather than a
// nudge to make this pass — the same fact, and the same repair, that
// [TestFadeMarksWhatIsNextWithTwoSpeakers] already carries. A hold spares the
// bit its own bit answers as well as itself, so at one vote in three every free
// stretch here is a single bit and D32's size rule refuses all of them: the loop
// runs to the end without folding once and this test compares nothing. The bits
// are written microseconds apart so no hold in here ever lapses, which is the
// regime that cliff lives in. Measured at the moment of the change: 0 folds at
// one in three, 5 at one in four, 6 at one in eight.
//
// The second assertion is what stops the first from being satisfied by fading
// everything: on the frame immediately before a fold there must still be bits
// drawn hot, or the warning has stopped distinguishing anything.
//
// The claim asserted is the strict one, with no allowance for the exception
// [Model.absorbing] concedes, because no hold in this fixture can reach it — see
// the span check at the end, which is what makes that a checked precondition
// rather than an assumption. [TestAnExpiringHoldIsTheOneHoleInTheFade] is where
// the exception itself is held to its exact shape.
func TestNothingIsAbsorbedWithoutFadingFirst(t *testing.T) {
	m := New()
	began := time.Now()

	folds, kept, freed := 0, 0, 0
	for i := range fixtureBudget * 8 {
		// The frame the person is looking at, right now, before they press
		// anything.
		faded := m.absorbing()
		stillHot := 0
		for _, id := range m.shown {
			if !faded[id] {
				stillHot++
			}
		}
		before := slices.Clone(m.shown)

		// One action, whichever it is, so that every write in this loop — a bit
		// said, a hold granted, a hold withdrawn — is checked against the frame
		// drawn in front of it. A vote is a write like any other and used to fold
		// like any other, which is the defect this loop exists to catch.
		switch {
		case i%11 == 10:
			m.move(-10)
			m.vote(memory.Down)
			m.move(len(m.shown)) // and back to the newest, where a caret rides
			freed++
		case i%4 == 1:
			m.vote(memory.Up)
			kept++
		default:
			m.say(localHandle, fmt.Sprintf("bit %d", i))
		}

		gone := took(m, before)
		if len(gone) == 0 {
			continue
		}
		folds++

		for id := range gone {
			if !faded[id] {
				t.Errorf("fold %d absorbed %s, which was drawn hot in the frame before it went",
					folds, memory.Short(id))
			}
		}
		if stillHot == 0 {
			t.Errorf("fold %d: every row was already faded on the frame before it, so the fade marked nothing",
				folds)
		}
	}

	if folds < 3 {
		t.Fatalf("only %d folds in %d writes, so the guarantee was barely exercised", folds, fixtureBudget*8)
	}
	if kept == 0 || freed == 0 {
		t.Fatalf("%d holds granted and %d withdrawn, so the voting paths were not exercised", kept, freed)
	}

	// Why the strict claim above is the right one here. A hold decays against the
	// conversation's own clock, and this whole conversation is shorter than one
	// hold, so no hold in it ever expires and the one case [Model.absorbing]
	// concedes cannot arise. Checked rather than reasoned about: bits are stamped
	// with time.Now, so the span of the record is at most the wall time this loop
	// took.
	if span := time.Since(began); span >= holdFor {
		t.Fatalf("the fixture ran for %s, which is longer than a hold (%s) — a hold could have expired in it, and the strict claim asserted above is then the wrong one",
			span, holdFor)
	}
}

// split is the frame the fade is decided on: a fold window a hold has cut into
// three pieces — a run the next fold takes, one bit standing between them that it
// will not, and another run.
//
// A view exactly at the budget, with the bit in the middle of the window the next
// fold would take held. At the budget the trigger is one write away, so nothing
// has folded yet and the whole window is on screen; the hold splits it. The caret
// is left where the key was pressed, which is where a hand leaves it.
//
// Both numbers were written down — twelve bits, the fourth from the window's end
// — until [Model.budget] stopped being a constant, and they are derived now.
// Deriving them restates arithmetic that lives in [Model.absorbing], which this
// file is otherwise careful not to do; what makes it safe is that the test using
// this counts the bands off the drawn frame and refuses the fixture by name if
// the arrangement is not the one it is named for. The derivation is a guess and
// the frame is the check.
//
// It is here rather than in harness_test.go because [TestHarnessFade] prints
// exactly this and the test below asserts over exactly this. A picture taken of a
// different arrangement from the one the pin describes is a picture of something
// else.
func split(m Model) Model {
	n := fixtureBudget
	m = talk(m, n)
	window := n - (fixtureBudget/2 - 1)
	m = back(m, n-1-window/2)
	m.vote(memory.Up)
	return m
}

// lone is the frame where the step does all the work by itself: a scar, and
// beneath it exactly one row that steps.
//
// A scar counts toward [memory.View.Fold]'s size rule and cannot step, so a run
// of two holding one spoken bit draws a single jogged row. It is the frame the
// width of the step was decided on, and the one that disproved the claim the
// first version of this rested on — that every step in the left edge is at least
// two rows deep.
//
// It used to be thirteen bits of an alternating conversation, and it is one
// speaker now, because the arrangement moved when [Model.keep] started landing
// the cut on a bit the human said. **Measured over lengths 2 to 120, an
// alternating conversation no longer draws a lone jogged row at all** — at 80x24
// or 100x30, 210 absorbing runs between them and not one of them lonely — while
// one speaker draws it at 18 bits and every tenth length after. That is not a
// claim that it cannot happen with two voices; it is the sweep that was run, and
// the case is real, so the fixture is the one that still reaches it.
func lone(m Model) Model { return record(fixtureBudget + 1) }

// bare is the transcript as a terminal with no colour at all shows it, grouped by
// bit: one entry per bit, holding its row and any continuation rows. It is the
// only form in which the property below is a property — with the escapes left in,
// a cooling row and a hot row differ whatever the screen does.
//
// Grouped rather than flat because a bit is no longer always one row: the caret's
// is drawn whole ([transcript]). The grouping is read off [anchors].rows, which is
// the count the thing that drew the rows made, rather than recomputed here — a
// second count of the arrangement under test is a second answer to the question,
// and it would agree on the day it was written.
func bare(m Model, width int) [][]string {
	f := m.frame()
	f.width = width
	return grouped(transcript(f))
}

// flat is the same frame with nothing drawn as going: the twin each row is
// compared against.
//
// Emptying the absorbing set is the whole of the difference. Everything else a
// row is built from — the vote column, the handle column, the width of the
// sentence, and how many rows the caret's own bit takes — is computed before it
// and does not read it, so the two renderings differ in exactly the thing under
// test, group for group and row for row, and a row can be compared against its
// own self.
func flat(m Model, width int) [][]string {
	f := m.frame()
	f.width, f.absorbing = width, map[string]bool{}
	return grouped(transcript(f))
}

// grouped cuts a drawn transcript into one entry per bit, using the renderer's
// own count of how tall the caret's row came out.
func grouped(body string, at anchors) [][]string {
	rows := strings.Split(ansi.Strip(body), "\n")

	var out [][]string
	for i := 0; i < len(rows); i++ {
		n := 1
		if i == at.mark && at.rows > 1 {
			n = at.rows
		}
		out = append(out, rows[i:min(i+n, len(rows))])
		i += n - 1
	}
	return out
}

// first is one row per bit: the row each bit opens with, which is where every
// column this surface arranges is drawn.
func first(groups [][]string) []string {
	out := make([]string, 0, len(groups))
	for _, g := range groups {
		out = append(out, g[0])
	}
	return out
}

// stepped is a row with [step] columns taken out of the margin, which is what a
// row does as it starts cooling.
//
// They come out from behind the caret rather than in front of it, so the caret
// keeps column 0 and its row steps out from under it. Removing the leading
// columns instead would be the opposite arrangement, and this is what tells the
// two apart.
func stepped(row string) string {
	r := []rune(row)
	if len(r) < caretColumn+step {
		return row
	}
	return string(r[:caretColumn]) + string(r[caretColumn+step:])
}

// The fade has to survive a terminal with no colour, and until this it did not:
// a cooling row and a hot row were byte-identical once the escapes were stripped,
// so under NO_COLOR, TERM=dumb, a pipe or a screenshot a fold arrived with no
// antecedent at all. That is the one thing this package's own doc says cannot
// happen here, and it was silently untrue everywhere colour was not.
//
// So a row that the next fold will take steps left, into the margin the caret
// reserves — see [caretCell] for why left and why two columns. This asserts it in
// the only form that means anything: ANSI stripped, every row compared against
// the same bit drawn as though it were staying.
//
// Exact equality in both directions, which is what stops it being satisfied by
// stepping everything or nothing. A row that is staying must be byte-identical to
// its twin; a row that is going must be its twin with exactly the step's columns
// gone from behind the caret, which also pins the width and the direction — a
// step of one, or a step to the right, passes neither.
//
// Four fixtures, and each carries the properties it is there for so that a
// fixture which quietly stops producing them fails rather than narrowing the
// claim. Two of them are the window a hold has split in three, which is the frame
// colour was carrying two boundaries at once in; they differ only in where the
// caret is, because the caret is the one thing that touches the columns the step
// comes out of. The third is a conversation nobody has voted in, which is both
// the common frame and the one whose floors were stated in a comment and pinned
// by nothing. The fourth is the one with a fold in the cut, without which the
// scar case below is asserted over nothing.
func TestTheFadeIsDrawnInSpaceAndNotOnlyInColour(t *testing.T) {
	// The floors, measured, per fixture. Read off [TestHarnessFade], which prints
	// them with every row from width 1 up, and never worked out from the constants
	// — three claims in this repository derived that way came out wrong, and the
	// width of the step itself was chosen on a cost nobody had measured.
	//
	// steps is where a going row is exactly its twin with the step's columns gone.
	// Below it the row is wider than the terminal, so [clip] cuts the tail and the
	// going row — starting further left — carries more of the sentence than its
	// twin does. differs is where even that stops: below it the two are the same
	// bytes and there is no fade of any kind.
	//
	// They belong to the fixture rather than to the surface, which is why both are
	// pinned twice here at different values: the floors move with the row's own
	// lead and the vote column is two columns of it.
	for _, s := range []struct {
		where          string
		m              Model
		steps, differs int
		bands          int
		markedGoing    bool
		// scars is how many folds are in the fixture's view, and it is a
		// precondition rather than a detail. A scar in the cut is drawn exactly as
		// one that is staying — the fade's second hole, stated in this package's
		// doc — so a fixture set with no scars in it would leave that case
		// untouched while looking like it had been covered. Three of the four here
		// have none and one has one, and all four say which.
		scars int
	}{
		{"a split window, the caret on the bit that is staying", split(New()), 11, 5, 2, false, 0},
		{"a split window, the caret on a row that is going", back(split(New()), 2), 11, 5, 2, true, 0},
		{"nobody has voted", talk(New(), 12), 9, 3, 1, false, 0},
		{"a scar the next fold will take", lone(New()), 9, 3, 1, false, 1},
	} {
		m, f := s.m, s.m.frame()
		steps, differs := s.steps, s.differs

		// The arrangement each fixture is named for, counted off the frame the
		// rows are drawn from rather than assumed.
		bands, staying, markedGoing := 0, 0, false
		for i, b := range f.bits {
			switch {
			case !f.absorbing[b.ID]:
				staying++
			case i == 0 || !f.absorbing[f.bits[i-1].ID]:
				bands++
			}
			if f.absorbing[b.ID] && b.ID == f.mark {
				markedGoing = true
			}
		}
		if bands != s.bands {
			t.Fatalf("%s: the fixture draws %d band(s) of cooling rows, want %d — this is not the frame it is named for",
				s.where, bands, s.bands)
		}
		if staying == 0 {
			t.Fatalf("%s: every row in the fixture is cooling, so there is no boundary for the step to be at", s.where)
		}
		if markedGoing != s.markedGoing {
			t.Fatalf("%s: the caret is on a going row = %v, so this fixture is not the one it is named for",
				s.where, markedGoing)
		}
		if m.scars() != s.scars {
			t.Fatalf("%s: the fixture holds %d fold(s), want %d — the scar case is covered by counting it, not by hoping for it",
				s.where, m.scars(), s.scars)
		}

		for width := 1; width <= 120; width++ {
			got, plain := bare(m, width), flat(m, width)
			if len(got) != len(f.bits) || len(plain) != len(f.bits) {
				t.Fatalf("%s: width %d drew %d and %d rows for %d bits",
					s.where, width, len(got), len(plain), len(f.bits))
			}

			for i, b := range f.bits {
				// The caret's bit is more than one row, and every one of them is
				// asserted rather than only its first: a block that stepped on its
				// opening line and stood still underneath would be a fade drawn on a
				// tenth of the object it describes.
				if len(got[i]) != len(plain[i]) {
					t.Fatalf("%s: width %d bit %d drew %d rows going and %d staying — the two renderings differ in height, so nothing below compares like with like",
						s.where, width, i+1, len(got[i]), len(plain[i]))
				}

				if !f.absorbing[b.ID] {
					for j := range got[i] {
						if got[i][j] != plain[i][j] {
							t.Errorf("%s: width %d row %d.%d is staying and was drawn %q, want its own twin %q",
								s.where, width, i+1, j+1, got[i][j], plain[i][j])
						}
					}
					continue
				}

				// A scar in the cut is drawn exactly as one that is staying, and
				// that is asserted here rather than skipped, because it is the
				// fade's second hole and this package's doc promises it. A scar
				// cannot step — it is already at the left edge — and [transcript]
				// hands every scar to seamInk whichever set it is in. Asserting the
				// identity rather than excusing it means that closing the hole
				// fails here, which is the right failure: the screen and the doc
				// have to move together.
				if _, cold := b.Payload.(memory.Compaction); cold {
					if got[i][0] != plain[i][0] {
						t.Errorf("%s: width %d row %d is a fold the next fold will take and was drawn %q, but the package doc promises it is drawn exactly as one that is staying, %q — if this is now closed, that doc is what has to change first",
							s.where, width, i+1, got[i][0], plain[i][0])
					}
					continue
				}

				for j := range got[i] {
					// Both floors are pinned in both directions, so improving one is
					// as loud as losing one.
					if want := got[i][j] != plain[i][j]; want != (width >= differs) {
						verb := "is drawn exactly as a row that is staying"
						if want {
							verb = "still differs from a row that is staying below the width where that was measured to stop"
						}
						t.Errorf("%s: width %d row %d.%d %s (differs at %d): %q against %q",
							s.where, width, i+1, j+1, verb, differs, got[i][j], plain[i][j])
					}

					if want := got[i][j] == stepped(plain[i][j]) && got[i][j] != plain[i][j]; want != (width >= steps) {
						verb := "does not step"
						if want {
							verb = "steps below the width where that was measured to stop"
						}
						t.Errorf("%s: width %d row %d.%d is going and %s (steps at %d): drawn %q, want its twin stepped left: %q",
							s.where, width, i+1, j+1, verb, steps, got[i][j], stepped(plain[i][j]))
					}
				}
			}
		}
	}
}

// Widening the terminal never takes anything off a row.
//
// It sounds too obvious to assert and it was false in this file for the length of
// one review: the threshold deciding whether the vote column narrows measured the
// margin with one constant while the row was built with another, so at exactly
// one width a terminal one column *wider* showed two characters *fewer* of a
// speaker's name. That is the failure [widest] and [nameColumn] exist to prevent
// — two agents arriving on screen under one string — reached from the other
// direction, and the whole suite passed over it.
//
// So what is pinned is the property rather than the arithmetic: across every
// width, the columns of handle a row shows only ever go up, and the columns of
// sentence go up except where the vote column buys the drain back. Any future
// disagreement between a lead and a threshold that reads it produces a fall
// somewhere in this sweep.
//
// Measured off drawn rows, both of them. The handle is counted as the longest
// prefix of the speaker's name the row contains, which is robust to the ellipsis
// [cell] adds; the sentence is counted by giving the last bit nothing but X to
// say and counting the X's, which is the only way to read a column width off a
// frame rather than recomputing the arithmetic under test.
func TestWideningTheTerminalNeverTakesAnythingOffARow(t *testing.T) {
	for _, s := range []struct {
		name string
		m    Model
		// falls is how many times the sentence column may shrink as the terminal
		// widens, measured per fixture. It is 0 wherever no vote column exists,
		// because the fall is the drain being bought back and an unvoted
		// conversation has no drain to buy.
		falls int
	}{
		{"nobody has voted", talk(New(), 11), 0},
		{"a hold splitting the window", split(New()), 1},
		{"a scar in the view", lone(New()), 0},
	} {
		// The last bit says nothing but X, under the longest handle in the fixture,
		// so that one row carries both measurements. Said under the short handle
		// instead, the sentence is countable and the handle column is not — which
		// is how the first version of this silently measured nothing at all on the
		// half it was written for, and let the regression it exists to catch pass.
		m := s.m
		m.say(memory.Handle{Ref: "ollama/llama3", Display: "coordinator-7"},
			strings.Repeat("X", 300))

		// And the caret is taken off it. The columns measured below are a cut row's
		// columns, and the caret's row is the one row on this surface that is not
		// cut — it is drawn whole and wrapped, so leaving the caret where say puts
		// it would count the X's on the last line of a paragraph and call it the
		// sentence column. The property is unchanged and so are the numbers; what
		// moved is which row states them.
		m.move(-1)

		shownName := func(w int) int {
			row := lastRow(m, w)
			n := 0
			for i := 1; i <= len("coordinator-7"); i++ {
				if strings.Contains(row, "coordinator-7"[:i]) {
					n = i
				}
			}
			return n
		}
		shownText := func(w int) int {
			row := lastRow(m, w)
			n := strings.Count(row, "X")
			if strings.Contains(row, "…") {
				n++
			}
			return n
		}

		for _, c := range []struct {
			what  string
			at    func(int) int
			falls int
		}{
			// The handle never falls, full stop. That is the property, and it is
			// the one the regression broke.
			{"handle", shownName, 0},

			// The sentence falls exactly once, and that one is older than the
			// step: at the width where the terminal is finally wide enough for the
			// drain, the vote column widens by three while the terminal gained
			// one, so the sentence gives up two. Reproduced at the geometry this
			// file had before the step existed, so it is not the step's doing.
			//
			// Left standing rather than fixed, because it is the ladder three
			// lines above [transcript]'s vote arithmetic doing exactly what that
			// comment says it does — buying the drain back and paying for it out
			// of the sentence — and the thing it spends on is drawn on the same
			// frame, so the reader can see where the columns went. Pinned by count
			// so that a *second* such width is a failure rather than a shrug.
			{"sentence", shownText, s.falls},
		} {
			prev, falls := 0, 0
			for width := 1; width <= 130; width++ {
				got := c.at(width)
				if got < prev {
					falls++
					if falls > c.falls {
						t.Errorf("%s: %s column falls from %d to %d when the terminal widens from %d to %d — %d fall(s), want at most %d\n  %3d %q\n  %3d %q",
							s.name, c.what, prev, got, width-1, width, falls, c.falls,
							width-1, lastRow(m, width-1), width, lastRow(m, width))
					}
				}
				prev = got
			}
			if falls != c.falls {
				t.Errorf("%s: the %s column falls %d time(s) across the sweep, and %d were measured — a fall that stopped happening is as much a change as one that started",
					s.name, c.what, falls, c.falls)
			}
		}
	}
}

// lastRow is the newest transcript row as a terminal with no colour shows it.
func lastRow(m Model, width int) string {
	rows := first(bare(m, width))
	return rows[len(rows)-1]
}

// A row that steps on its own is always drawn against column 0.
//
// The step is a band edge, and a band of one row has no edge unless there is
// something to read it against. The first version of this work claimed there was
// never a band of one, because [memory.View.Fold] never cools a run of one — and
// that was wrong, because a scar counts toward the run's size and does not step.
// A run of [scar, spoken] draws exactly one jogged row.
//
// What actually holds it up is the neighbour rather than the height: the only way
// a run of two or more draws one jogged row is for everything else in the run to
// be a scar, and a scar sits at column 0. So the lone row is read against the
// left edge of the screen and not against a flat wall of rows.
//
// Asserted on the drawn frame rather than on the run, because on the run it is a
// tautology and a check that cannot fail is worse than no check. What is checked
// is that the neighbouring row is a fold *and* that the screen draws it starting
// in column 0 — the second half is a fact about this file, and it is the half a
// change to [caretCell] can break.
func TestALoneJoggedRowIsAlwaysBesideAScar(t *testing.T) {
	lonely := 0
	for n := 2; n <= 60; n++ {
		for _, m := range []Model{talk(New(), n), record(n)} {
			lonely += loneRows(t, n, m)
		}
	}

	// The case has to occur, or this asserts over nothing — and half of this sweep
	// stopped producing it when [Model.keep] started landing the cut on a bit the
	// human said. Measured over lengths 2 to 120: an alternating conversation draws
	// **no** lone jogged row at all, at either terminal size tried, and one speaker
	// draws it at 18 bits and every tenth length after. Both fixtures are swept
	// because the guard below cannot tell "this arrangement is impossible now" from
	// "somebody narrowed the fixture", and the second is the failure that would go
	// unnoticed.
	if lonely == 0 {
		t.Fatal("no run in the sweep drew a single jogged row, so the case this exists for is not being produced")
	}
}

// loneRows is the body of the sweep above, over one model: it checks every
// absorbing run that draws exactly one stepped row and returns how many it found.
func loneRows(t *testing.T, n int, m Model) int {
	t.Helper()

	f := m.frame()
	rows := first(bare(m, 100))
	if len(rows) != len(f.bits) {
		t.Fatalf("%d bits drew %d rows", len(f.bits), len(rows))
	}

	spoken := func(i int) bool {
		_, cold := f.bits[i].Payload.(memory.Compaction)
		return !cold
	}

	lonely := 0
	for i := 0; i < len(f.bits); {
		if !f.absorbing[f.bits[i].ID] {
			i++
			continue
		}
		j := i
		said := []int{}
		for j < len(f.bits) && f.absorbing[f.bits[j].ID] {
			if spoken(j) {
				said = append(said, j)
			}
			j++
		}

		if len(said) == 1 {
			lonely++
			at, beside := said[0], -1
			for _, k := range []int{at - 1, at + 1} {
				if k >= i && k < j && !spoken(k) {
					beside = k
				}
			}
			if beside < 0 {
				t.Fatalf("%d bits: the row at %d steps on its own with no fold beside it in its run",
					n, at)
			}
			if strings.HasPrefix(rows[beside], " ") {
				t.Errorf("%d bits: the lone stepped row at %d is read against row %d, which does not start in column 0: %q",
					n, at, beside, rows[beside])
			}
		}
		i = j
	}
	return lonely
}

// The exception, at its exact size. It is conceded in [Model.absorbing] and in
// this package's own doc, and a concession nobody exercises is a claim nobody
// checked — so this reproduces it, and it is what stops the hole quietly getting
// bigger.
//
// The arrangement is the ordinary one rather than a contrived one: two bits held
// with a single unheld bit between them. D32's size rule spares a run of one, so
// that middle bit is bright and correctly so, and the two holds either side of it
// are bright because they are held. Let both holds lapse in the same write — two
// votes pressed in the same breath expire in the same breath, because they decay
// against one shared [memory.View.Latest] — and the run merges to three and all
// three go, with the middle one never having been voted on by anybody and never
// having been drawn cooling.
//
// What is asserted is the shape of the hole, not its absence: every bit absorbed
// without fading first was spared on the frame before, or stood next to a bit
// that was. Anything else going dark without warning would be a new defect, and
// this is where it would show up.
//
// Spared and not held, and the difference is the size of the hole. A hold in
// memory now covers the bit its own bit answers as well as itself, so two holds
// keep four rows bright here rather than two, and when they lapse the run that
// merges is four rather than three. The row this test would have missed under
// the narrower word is the first bit of the view: nobody voted on it, no
// neighbour of it was ever held, and it stayed only because the cover beside it
// cut its run down to one.
func TestAnExpiringHoldIsTheOneHoleInTheFade(t *testing.T) {
	// Twelve bits is a full view with the trigger one write away, and two holds
	// push it three away: the trigger counts what a fold could take, and a held
	// bit is not that.
	m := record(fixtureBudget)

	// A hair short of expiry, in conversation time. The instant is named rather
	// than read off the clock for the reason [Model.vote] cannot be used here at
	// all: holdFor is two minutes of conversation, and a test cannot wait for it.
	//
	// A hundred milliseconds of that hair, not one, and the difference is a real
	// failure rather than caution. Conversation time here is wall time — every bit
	// below is stamped with time.Now — so the two writes that follow have to fit
	// inside the margin or the holds expire early and the fold happens before the
	// frame this test is about. At one millisecond it did exactly that under
	// -race, where a redraw costs enough to overrun it. The margin now has two
	// orders of magnitude of headroom, and if a machine ever overruns even that,
	// the preconditions below fail loudly rather than passing on a fixture that
	// stopped describing the case.
	const margin = 100 * time.Millisecond
	first := keepAt(t, &m, 2, holdFor-margin)
	second := keepAt(t, &m, 4, holdFor-margin)
	stranded := m.shown[3]

	// Up to the write before the one that folds. These land in microseconds, so
	// the holds are still standing when the frame below is captured.
	m.say(localHandle, "and another")
	m.say(localHandle, "and another")
	if m.scars() != 0 {
		t.Fatalf("%d folds already happened, so the frame under test is not the one before the first", m.scars())
	}

	held := m.stay().Holds(m.store, m.day())
	faded := m.absorbing()

	// What the holds were keeping bright, which is wider than the holds
	// themselves: memory spares the bit each held bit answers as well. Asked of
	// the package that folds rather than rebuilt from Prev here — this was the
	// only restatement of a fold rule in this file, and it lasted one session,
	// which is about how long a second statement of a rule usually agrees with
	// the first.
	spared := m.shown.Sparing(m.store, m.stay())

	for _, id := range []string{first, second} {
		if _, up := held[id]; !up {
			t.Fatalf("%s is not holding, so there is nothing to expire", memory.Short(id))
		}
	}
	if _, up := held[stranded]; up {
		t.Fatal("the bit between the two holds is itself held, so it is not stranded")
	}
	if faded[stranded] {
		t.Fatal("the bit between the two holds is already drawn cooling, so the fold takes nothing by surprise")
	}

	// Past both expiries, and into the write that folds. The sleep is the whole
	// mechanism: bits are stamped with time.Now, so waiting out the margin is what
	// carries the conversation's own clock past two holds that had a tenth of a
	// second left. A sleep only ever runs long, so this direction cannot go wrong
	// on a slow machine.
	time.Sleep(margin + 50*time.Millisecond)
	before := slices.Clone(m.shown)
	m.say(localHandle, "one too many")

	gone := took(m, before)
	if len(gone) == 0 {
		t.Fatal("no fold happened, so nothing was absorbed at all")
	}
	if !gone[stranded] {
		t.Fatal("the stranded bit survived the fold, so this is no longer the case being described")
	}

	// The hole, at its exact size: unfaded material goes, and what it is is
	// material a hold was keeping bright — the held bit itself, the bit it
	// answers, or a bit whose neighbours those were.
	next := func(i int) []string {
		var out []string
		if i > 0 {
			out = append(out, before[i-1])
		}
		if i < len(before)-1 {
			out = append(out, before[i+1])
		}
		return out
	}
	surprises := 0
	for i, id := range before {
		if !gone[id] || faded[id] {
			continue
		}
		surprises++
		if spared[id] {
			continue // the hold over it ran out, which is the bargain a hold makes
		}
		for _, n := range next(i) {
			if spared[n] {
				id = "" // spared by a neighbour's hold, which then lapsed
				break
			}
		}
		if id != "" {
			t.Errorf("%s was absorbed having been neither faded, nor held, nor beside a hold — the hole is wider than it is written down as",
				memory.Short(id))
		}
	}
	if surprises == 0 {
		t.Fatal("nothing went without fading, so the exception this test exists to pin is not being reproduced")
	}
}

// keepAt casts an upvote on the bit at index i of the view, as though the key had
// been pressed ago of conversation time in the past.
//
// It is [Model.vote]'s own two lines with the caret named by position and the
// instant named instead of read off the clock, which is the only way to arrange a
// hold that expires while a test is still running: holdFor is two minutes of
// conversation time. Everything else is the program — the vote is a bit, it goes
// in the store, it goes on the vote view, and [memory.Stay.Holds] decides what it
// is worth by comparing it against [memory.View.Latest].
func keepAt(t *testing.T, m *Model, i int, ago time.Duration) string {
	t.Helper()

	target, ok := m.store.Get(m.shown[i])
	if !ok {
		t.Fatalf("the view names %s at %d, which the store does not hold", memory.Short(m.shown[i]), i)
	}
	m.votes, _ = m.votes.Add(m.store,
		memory.Cast(m.day().Add(-ago), localHandle, memory.Up, target))
	m.sync()
	return target.ID
}

// scar returns the one fold in the view, failing if there is not exactly one.
func scar(t *testing.T, m Model) memory.Compaction {
	t.Helper()
	var found []memory.Compaction
	for _, b := range m.shown.Bits(m.store) {
		if c, cold := b.Payload.(memory.Compaction); cold {
			found = append(found, c)
		}
	}
	if len(found) != 1 {
		t.Fatalf("view holds %d folds, want exactly 1", len(found))
	}
	return found[0]
}

// Pressing the key with nothing folded must not arm anything. If it flipped the
// flag anyway, the next fold would arrive already open and the collapse — the
// one event this surface exists to show — would happen with the screen looking
// unchanged.
func TestUnfoldNeedsAScar(t *testing.T) {
	m := record(fixtureBudget)
	if m.scars() != 0 {
		t.Fatalf("scars = %d before any fold, want 0", m.scars())
	}

	m.unfold()
	if m.unfolded {
		t.Error("unfold armed itself with nothing to follow")
	}
}

// One key, both directions. No mode to be stranded in.
func TestUnfoldToggles(t *testing.T) {
	m := record(fixtureBudget + 1)
	if m.scars() != 1 {
		t.Fatalf("scars = %d after one fold, want 1", m.scars())
	}

	m.unfold()
	if !m.unfolded {
		t.Fatal("first press did not open the receipt")
	}
	m.unfold()
	if m.unfolded {
		t.Error("second press did not close it")
	}
}

// Half of the load-bearing pair for the whole interaction: following a receipt
// is retrieval from the record, not restoration to the view. The store gains
// nothing, the view loses and gains nothing, and the bits on screen are the
// same bits either way.
//
// It is only half, and saying otherwise oversold it. Every assertion below is
// negative, so an unfold that did nothing at all satisfies all of them
// perfectly — which a mutation test confirmed. The opening guard is what makes
// the rest mean something here; that the key retrieves the material is
// [TestUnfoldDrawsOneRowPerAbsorbedBit], and that it is a toggle rather than a
// latch is [TestUnfoldToggles]. All three have to hold before the sentence "it
// retrieves without restoring" is true.
func TestUnfoldChangesNeitherRecordNorView(t *testing.T) {
	m := record(fixtureBudget * 3)

	before := slices.Clone(m.shown)
	stored := m.store.Len()

	m.unfold()
	if !m.unfolded {
		t.Fatal("the key did nothing, so everything asserted below is vacuously true")
	}

	if got := m.store.Len(); got != stored {
		t.Errorf("record holds %d bits after an unfold, want %d — retrieval wrote to the record", got, stored)
	}
	if !slices.Equal(m.shown, before) {
		t.Errorf("view is %v after an unfold, want %v — retrieval changed what is on the view", m.shown, before)
	}
}

// A fold has to be watchable. If the screen were already open when one fired,
// the material would stay put and the collapse would be invisible.
func TestFoldClosesAnOpenUnfold(t *testing.T) {
	m := record(fixtureBudget + 1)
	m.unfold()
	if !m.unfolded {
		t.Fatal("receipt did not open")
	}

	for range fixtureBudget {
		m.say(localHandle, "more")
	}
	if m.unfolded {
		t.Error("a fold fired while the receipt was open and left it open")
	}
}

// The receipt resolves in full: every address it names, in the order it names
// them, and exactly as many as it claims. The count on the scar is a promise a
// person checks by counting rows, so the rows have to be the receipt itself.
func TestRecallFollowsTheWholeReceipt(t *testing.T) {
	m := record(fixtureBudget * 3)
	c := scar(t, m)

	got := recall(m.store, c)
	if len(got) != c.Count() {
		t.Fatalf("recall returned %d bits, but the scar claims %d", len(got), c.Count())
	}

	want := slices.Collect(c.Absorbed())
	for i, r := range got {
		if !r.found {
			t.Errorf("receipt names %s, which the store does not hold", memory.Short(r.id))
		}
		if r.id != want[i] {
			t.Errorf("recall[%d] = %s, want %s — the receipt came back out of order",
				i, memory.Short(r.id), memory.Short(want[i]))
		}
	}
}

// Everything comes back. What the fold took off the screen plus what is still
// on it is the whole conversation, in the order it happened — which is the
// sentence the interaction is supposed to make true, tested as arithmetic
// rather than as a screenshot.
func TestUnfoldAndTheHotTailAccountForEverythingSent(t *testing.T) {
	sends := fixtureBudget * 3
	m := record(sends)

	var got []string
	for _, b := range m.shown.Bits(m.store) {
		switch p := b.Payload.(type) {
		case memory.Compaction:
			for _, r := range recall(m.store, p) {
				// oneLine and not said: this is a question about what the record
				// holds, not about what a row of some width shows.
				got = append(got, oneLine(r.bit))
			}
		case memory.Utterance:
			got = append(got, p.Text)
		}
	}

	want := make([]string, 0, sends)
	for i := range sends {
		want = append(want, fmt.Sprintf("bit %d", i))
	}
	if !slices.Equal(got, want) {
		t.Errorf("the screen accounts for %d bits, want the %d that were sent, in order",
			len(got), len(want))
	}
}

// A receipt that stops resolving is the failure D1 exists to rule out, so it
// has to arrive as something a person can see rather than as a row that is
// quietly missing. This cannot happen against a store that holds the originals,
// so it is provoked with one that never did.
func TestRecallReportsAReceiptItCannotResolve(t *testing.T) {
	c := cooled(t, "the deploy failed", "deploy again")

	got := recall(memory.NewStore(), c)
	if len(got) != c.Count() {
		t.Fatalf("recall returned %d entries, want %d — an unresolvable address was dropped",
			len(got), c.Count())
	}
	for _, r := range got {
		if r.found {
			t.Errorf("%s resolved against a store that never held it", memory.Short(r.id))
		}
	}
}

func TestTopWordsIsDeterministic(t *testing.T) {
	bag := map[string]int{"deploy": 3, "failed": 3, "again": 1, "0044": 1}

	want := []string{"deploy", "failed", "again"}
	for range 20 {
		got := topWords(maps.All(bag), 3)
		if !slices.Equal(got, want) {
			t.Fatalf("topWords = %v, want %v (map order leaked)", got, want)
		}
	}
}

// The most frequent word in any English window is "the", so without a filter
// every scar in the record reports the same four words and the receipt says
// nothing about what it stands for. The filter is display-only: the store's bag
// still counts them, which is what makes editing here safe.
func TestTopWordsSkipsFiller(t *testing.T) {
	bag := map[string]int{"the": 40, "of": 30, "and": 20, "migration": 2}

	if got := topWords(maps.All(bag), 4); !slices.Equal(got, []string{"migration"}) {
		t.Errorf("topWords = %v, want only the word that says something", got)
	}
}

// cooled folds real bits, one per minute from 09:00. A compaction with anything
// in it can only come from memory.Cool, since its fields are unexported, so a
// rendering test folds like everything else does and gets a receipt whose parts
// actually agree.
func cooled(t *testing.T, texts ...string) memory.Compaction {
	t.Helper()
	start := time.Date(2026, 8, 11, 9, 0, 0, 0, time.UTC)

	window := make([]memory.Bit, 0, len(texts))
	for i, text := range texts {
		b := memory.Bit{
			At:      start.Add(time.Duration(i) * time.Minute),
			From:    memory.Handle{Ref: "tyler", Display: "me"},
			Channel: channel,
			Payload: memory.Utterance{Text: text},
		}
		b.ID = memory.ID(b)
		window = append(window, b)
	}
	return memory.Cool(window).Payload.(memory.Compaction)
}

// scarred is [cooled] with the absorbed bits filed in a store and a frame to draw
// the scar against, which is what a scar needs to quote one of them. cooled's own
// store-less form is kept for the tests that provoke a receipt nothing resolves.
func scarred(t *testing.T, texts ...string) (frame, memory.Compaction) {
	t.Helper()
	return named(t, "me", texts...)
}

// named is [scarred] with the speaker's handle chosen, because the columns a
// quotation has left are a function of it: a two-column name and a thirteen-column
// one are two different arrangements and only one of them ever truncates.
func named(t *testing.T, who string, texts ...string) (frame, memory.Compaction) {
	t.Helper()
	start := time.Date(2026, 8, 11, 9, 0, 0, 0, time.UTC)
	s := memory.NewStore()

	window := make([]memory.Bit, 0, len(texts))
	for i, text := range texts {
		window = append(window, s.Put(memory.Bit{
			At:      start.Add(time.Duration(i) * time.Minute),
			From:    memory.Handle{Ref: "tyler", Display: who},
			Channel: channel,
			Payload: memory.Utterance{Text: text},
		}))
	}
	cold := memory.Cool(window)
	s.Put(cold)
	return frame{store: s, clock: atNine, votes: map[string]memory.Score{}},
		cold.Payload.(memory.Compaction)
}

// rows returns the retrieved rows of an unfolded block: the ones inside the
// gutter, which is what one-row-per-bit is counted over.
func rows(block string) []string {
	var out []string
	for _, line := range strings.Split(block, "\n") {
		if plain := ansi.Strip(line); strings.HasPrefix(plain, "│") {
			out = append(out, plain)
		}
	}
	return out
}

// The count on a scar is a claim, and the way anyone checks a claim like that
// is by counting rows. So the block draws exactly one row per absorbed bit at
// every width — never wrapping, never merging two bits onto a line, never
// dropping one that will not fit — and each row carries its own place in the
// count, so the check can be made from whatever part of the block the terminal
// happens to be showing.
func TestUnfoldDrawsOneRowPerAbsorbedBit(t *testing.T) {
	m := record(fixtureBudget * 3)
	c := scar(t, m)

	for _, width := range []int{200, 80, 40, 24, 20, 8, 1} {
		got := rows(receiptOf(m, c, width))
		if len(got) != c.Count() {
			t.Errorf("unfold at width %d drew %d rows, but the scar claims %d bits",
				width, len(got), c.Count())
			continue
		}
		if width < 24 {
			continue // the ordinal is itself being cut down here
		}
		for i, row := range got {
			place := fmt.Sprintf("%d/%d", i+1, c.Count())
			if !strings.Contains(row, place) {
				t.Errorf("unfold at width %d, row %d is missing its place %q: %q",
					width, i+1, place, row)
			}
		}
	}
}

// Two agents whose names share a prefix must never arrive on screen as one
// string. A fixed ten-column handle field turned coordinator-7 and
// coordinator-9 both into "coordinati", with no mark saying anything had been
// cut — so the block whose entire job is to say who said what reported that
// they were the same speaker. No test could see it while every handle was "me".
//
// Two properties, and both are needed. A handle that is not shown in full is
// shown with a mark — that alone would be satisfied by a column still fixed at
// ten, which marks the cut but leaves the two names identical on screen. So the
// second: when the terminal has the room, the handle is shown whole.
func TestUnfoldNeverShortensAHandleWithoutSaying(t *testing.T) {
	names := []string{"coordinator-7", "coordinator-9"}

	s := memory.NewStore()
	var v memory.View
	start := time.Date(2026, 8, 11, 9, 0, 0, 0, time.UTC)
	for i, n := range names {
		v, _ = v.Add(s, memory.Bit{
			At:      start.Add(time.Duration(i) * time.Minute),
			From:    memory.Handle{Ref: n, Display: n},
			Channel: channel,
			// Short on purpose: any ellipsis in a row must have come from the
			// handle, since nothing else here is long enough to need one.
			Payload: memory.Utterance{Text: "ok"},
			Prev:    v.Head(),
		})
	}
	c := memory.Cool(v.Bits(s)).Payload.(memory.Compaction)

	// Wide enough for everything: no handle may be shortened at all, or the two
	// agents arrive on screen under one string with room to spare.
	for _, width := range []int{200, 80, 60} {
		for i, row := range rows(unfold(frame{store: s, clock: atNine, width: width}, c)) {
			if !strings.Contains(row, names[i]) {
				t.Errorf("unfold at width %d shortened %q with room to spare: %q",
					width, names[i], row)
			}
		}
	}

	// Narrow enough that something has to give: whatever gives, says so.
	for _, width := range []int{60, 40, 30, 24, 20} {
		for i, row := range rows(unfold(frame{store: s, clock: atNine, width: width}, c)) {
			if strings.Contains(row, names[i]) {
				continue
			}
			if !strings.Contains(row, "…") {
				t.Errorf("unfold at width %d cut %q with no mark: %q", width, names[i], row)
			}
		}
	}
}

// The one row whose whole purpose is to be seen was the only one built to a
// fixed sixty-six columns, so on a narrow terminal it was cut by the viewport —
// unmarked, and below about thirty columns cut before the word "resolve". A
// receipt that stopped resolving is the failure D1 exists to rule out; a notice
// of it that runs off the edge of the screen is no notice at all.
func TestUnresolvableRowSaysSoAtEveryWidth(t *testing.T) {
	c := cooled(t, "the deploy failed", "deploy again")

	for _, width := range []int{200, 80, 40, 30, 24, 20, 16} {
		for _, row := range rows(unfold(frame{store: memory.NewStore(), clock: atNine, width: width}, c)) {
			if w := lipgloss.Width(row); w > width {
				t.Errorf("unresolvable row at width %d is %d wide: %q", width, w, row)
			}
			if !strings.Contains(row, "unresolved") &&
				!strings.Contains(row, "does not resolve") &&
				!strings.Contains(row, "gone") {
				t.Errorf("unresolvable row at width %d does not say it failed: %q", width, row)
			}
		}
	}
}

// A fragment is a bit that says its speaker did not finish, and the whole
// reason it may be recorded at all is that a person can see which one it is.
// Both places a bit is drawn are checked here, because they are the two places
// a person looks: the transcript, and the receipt a scar opens.
//
// The mark is read as a character, never as a colour. The fade already fails
// silently under a low colour profile — harness_test.go cannot even see that
// happen — so a distinction resting on colour is a distinction that is not
// there on somebody else's terminal.
func TestAFragmentIsDrawnDifferentlyFromAFinishedUtterance(t *testing.T) {
	m := New()
	m.say(localHandle, "what are the three steps")
	m.utter(m.persona.Handle(), memory.Utterance{Text: "the three steps are, first,", Truncated: true})
	m.say(localHandle, "go on")

	bits := m.shown.Bits(m.store)
	for _, width := range []int{200, 120, 80, 60, 40} {
		got := strings.Split(ansi.Strip(shot(m, width, false)), "\n")
		if len(got) != len(bits) {
			t.Fatalf("transcript at width %d drew %d rows for %d bits", width, len(got), len(bits))
		}
		for i, row := range got {
			// Both directions. Marking every row would satisfy the first check
			// and say nothing, which is the shape of a check that cannot fail.
			cut := bits[i].Payload.(memory.Utterance).Truncated
			if marked := strings.Contains(row, "╌"); marked != cut {
				verb := "draws the fragment as though its speaker finished"
				if marked {
					verb = "marks a finished utterance as a fragment"
				}
				t.Errorf("transcript at width %d %s: %q", width, verb, row)
			}
		}
	}
}

// A fold must not be where a fragment goes quiet. Two things carry it past one:
// the row in the block ctrl+u opens still says the speaker stopped, and the
// closed scar tallies how many of the bits it absorbed were unfinished.
//
// The tally is also what checks a coupling the compiler cannot see. A
// [memory.Compaction] keeps no payloads, only a count per kind, and "fragment"
// is a hand-written literal in memory/bit.go matched by a hand-written literal
// in fragmentsIn. Folding a real truncated bit and reading the number back off
// the drawn scar is what fails if either literal ever moves.
func TestAFoldedFragmentIsMarkedOnItsReceiptAndCountedOnItsScar(t *testing.T) {
	m := New()
	m.utter(m.persona.Handle(), memory.Utterance{Text: "the three steps are, first,", Truncated: true})
	for i := range fixtureBudget {
		m.say(localHandle, fmt.Sprintf("carry on %d", i))
	}

	c := scar(t, m)
	if got := fragmentsIn(c); got != 1 {
		t.Fatalf("the fold tallies %d fragments, want 1 — the kind memory writes and the kind this package reads have drifted apart", got)
	}
	if got := ansi.Strip(seam(m.frame(), c, 80)); !strings.Contains(got, "1 unfinished") {
		t.Errorf("scar = %q, want it to report the unfinished bit it absorbed", got)
	}

	// And the persona is told the same thing. The two accounts of a fold no
	// longer agree about its *content* — see [personaWords] — but they still
	// agree about its count, its span, its speakers and this tally, and the
	// tally is the one of those four that lives in two hand-written places.
	note := foldNote(c, m.frame().clock)
	if !strings.Contains(note, "ran out of room") {
		t.Errorf("the fold note does not carry the unfinished bit the scar reports:\n%s", note)
	}

	// And on the receipt, on the row that stands for it and on no other. The
	// fragment was the first bit said, so it is the first row of the block.
	got := rows(receiptOf(m, c, 200))
	if len(got) < 2 {
		t.Fatalf("the receipt drew %d rows, so there is nothing to tell apart", len(got))
	}
	for i, row := range got {
		if marked := strings.Contains(row, "╌"); marked != (i == 0) {
			t.Errorf("receipt row %d: marked = %v, want %v: %q", i+1, marked, i == 0, row)
		}
	}
}

// utterance is a bit whose only interesting property is its payload.
func utterance(text string, cut bool) memory.Bit {
	return memory.Bit{
		At:      time.Date(2026, 8, 11, 9, 0, 0, 0, time.UTC),
		From:    memory.Handle{Ref: "ollama/qwen3.5", Display: "qwen3"},
		Channel: channel,
		Payload: memory.Utterance{Text: text, Truncated: cut},
	}
}

// What said owes, at every width it can be given, stated so that no content can
// satisfy it.
//
// The oracle is a comparison against the bit's own finished twin — same text,
// same everything, one flag apart — rather than a search for the mark's string.
// That matters because the mark is not reserved: a participant can type "╌
// unfinished ╌" into a message, and a test looking for that string would accept
// the forgery. A twin cannot be forged, because whatever a message contains its
// twin contains too, so the only thing that can separate the two renderings is
// the flag.
//
// The second oracle is exact equality, which is what stops the first from being
// satisfied by marking everything: a finished utterance draws its own content
// and nothing else, whatever that content happens to spell.
func TestSaidMarksAFragmentAndNothingElse(t *testing.T) {
	const long = "the three steps are, first, to take a full dump of the schema before anything else touches it"

	texts := map[string]string{
		"a long answer":  long,
		"a short answer": "ok",

		// A fragment with no text is unreachable from the wire — persona.Reply
		// returns a *persona.Error rather than an Answer when the trimmed text
		// is empty, truncated or not, so recordReply never sees one. The branch
		// is reachable all the same: utter takes any Utterance, and a payload
		// that draws as nothing at all is worth pinning before something else
		// starts writing them.
		"an answer with nothing in it": "",

		// The forgery. This is a message that says the mark, rather than a
		// message that is marked.
		"an answer that spells the mark": "and then it printed ╌ unfinished ╌ at the end",
	}

	for name, text := range texts {
		cut, whole := utterance(text, true), utterance(text, false)
		for width := 1; width <= 200; width++ {
			marked, plain := said(frame{}, cut, width), said(frame{}, whole, width)

			if marked == plain {
				t.Errorf("%s at width %d: a fragment and its finished twin draw identically: %q",
					name, width, marked)
			}
			if want := ansi.Truncate(oneLine(whole), width, "…"); plain != want {
				t.Errorf("%s at width %d: a finished utterance drew %q, want its own content %q",
					name, width, plain, want)
			}
			if w := lipgloss.Width(marked); w > width {
				t.Errorf("%s at width %d is %d wide: %q", name, width, w, marked)
			}
		}
	}

	// And the pair is a distinction rather than one glyph doing two jobs. Room
	// for every word: the mark is there and the ellipsis is not, because nothing
	// about that row is the screen's doing. Then the converse.
	if got := said(frame{}, utterance("ok", true), 80); strings.Contains(got, "…") {
		t.Errorf("said = %q, want the speaker's mark and no ellipsis — the two facts are being conflated", got)
	}
	if got := said(frame{}, utterance(long, false), 20); !strings.Contains(got, "…") {
		t.Errorf("said = %q at width 20, want the screen's own cut marked", got)
	}
}

// Where the mark actually stops surviving on each of the two surfaces that draw
// it, pinned in both directions.
//
// said keeps a mark at every width, and that is a claim about said. A row is
// said's output with columns in front of it, handed to clip, and clip cuts the
// tail — which is where the mark is. So there is a width on each surface below
// which a fragment draws exactly as a finished utterance the screen ran out of
// room for, and the honest thing is to know it rather than to state the
// stronger claim, which is what the first version of this did while asserting
// about the helper and being named for the row.
//
// Two floors per surface, because they move for different reasons and the first
// version of this test pinned only one of them — which made its own claim to
// pin things "in both directions" false, the same defect one level up. Widening
// the last rung of the mark by a single column passed it cleanly.
//
// The dash floor is where a fragment stops being distinguishable from its
// finished twin at all. It is set by the row's column arithmetic, and the mark's
// own ladder cannot move it: down there the mark is already the bare dash.
// The word floor is where the mark stops saying "unfinished" in words, and that
// one is exactly what the ladder sets.
//
// All four are measurements. The test fails if any moves in either direction,
// so improving a floor is as loud as losing one.
//
// The transcript's two have moved twice, and both times for a column added in
// front of the row rather than for anything about the mark: first the caret, then
// the fade's step. Both are costs paid at widths where nothing is legible anyway,
// which is what makes them the right place to pay. The receipt's two have not
// moved at all through either change — the step is a transcript column and a
// receipt row does not carry one — and that is the part worth checking rather
// than assuming. The old values are not repeated here; the constants below are
// the only copy, which is the point of a pin.
//
// Read off [TestHarnessFloors], which prints them, and never worked out from the
// constants. The last three claims in this repository derived from constants
// were each one out.
// fragmentText is the sentence the four fixtures below stop in the middle of.
const fragmentText = "the three steps are, first,"

// The fixtures the floors are measured on: two records differing in one flag, so
// the rows are compared against each other rather than against a string, and the
// two ways a bit reaches a screen. They are package-level rather than closures
// inside the test because [TestHarnessFloors] prints from exactly these — a
// measurement taken on a different fixture from the one the pin asserts over is
// a measurement of something else.
// The caret is moved off the fragment for [TestWideningTheTerminalNeverTakesAnythingOffARow]'s
// reason, and it applies to every floor below: these are the widths at which a
// *cut* row stops telling a fragment from a finished utterance, and the caret's
// row is the one row here that is never cut. With the caret left on it the sweep
// would be measuring where a wrapped paragraph puts its mark, which is a
// different question with a different answer.
func unfoldedFragment(cut bool) Model {
	m := New()
	m.say(localHandle, "what are the three steps")
	m.utter(m.persona.Handle(), memory.Utterance{Text: fragmentText, Truncated: cut})
	m.move(-1)
	return m
}

func foldedFragment(cut bool) Model {
	m := New()
	m.utter(m.persona.Handle(), memory.Utterance{Text: fragmentText, Truncated: cut})
	for i := range fixtureBudget {
		m.say(localHandle, fmt.Sprintf("carry on %d", i))
	}
	return m
}

// coolingFragment is the fragment on a row the next fold will take, which is a
// third set of floors and not the transcript's: a going row begins [step] columns
// further left, so [clip] reaches its mark that much later. Eight bits is enough
// to put the first one in the cut and not enough to fold it away.
func coolingFragment(cut bool) Model {
	m := New()
	m.utter(m.persona.Handle(), memory.Utterance{Text: fragmentText, Truncated: cut})
	for i := range 8 {
		m.say(localHandle, fmt.Sprintf("carry on %d", i))
	}
	return m
}

// coldRow is the first transcript row, which is where [coolingFragment] puts the
// fragment, and it fails rather than measures if that row is not actually going —
// a fixture that stopped producing a cooling row would otherwise report the
// transcript's own floors under a different name.
func coldRow(t *testing.T, m Model, width int) string {
	t.Helper()
	f := m.frame()
	if !f.absorbing[f.bits[0].ID] {
		t.Fatal("the fragment row is not in the cut, so these are not a cooling row's floors")
	}
	return ansi.Strip(strings.Split(shot(m, width, false), "\n")[0])
}

// hotRow is the transcript's own newest row, with nothing folded, so the row
// under test is the fragment's and no receipt is in the way.
func hotRow(_ *testing.T, m Model, width int) string {
	rows := strings.Split(ansi.Strip(shot(m, width, false)), "\n")
	return rows[len(rows)-1]
}

// receiptRow is the receipt as the screen draws it: transcript clips every line
// of the block, so a test reading unfold's return value directly is reading
// something no terminal ever shows.
//
// Taken by position rather than by the gutter, because at a handful of columns
// the gutter is cut off too and a row filtered on it disappears rather than
// failing. The fragment was the first bit said, so it is the block's first line.
func receiptRow(t *testing.T, m Model, width int) string {
	c := scar(t, m)
	return ansi.Strip(strings.Split(clip(receiptOf(m, c, width), width), "\n")[0])
}

func TestTheRowsMarkFloorsAreWhereTheyWereMeasured(t *testing.T) {
	const (
		dashInTranscript = 9
		wordInTranscript = 20
		// The receipt's two moved when the fold budget did, and by two columns
		// rather than one: this fixture's scar now absorbs ten bits instead of
		// seven, so its ordinal reads `1/10` where it read `1/7`, and both halves
		// of that column widen. Re-measured, not adjusted until they passed —
		// `HARNESS=1 go test ./tui/ -run TestHarnessFloors -v` prints all four
		// with the row either side of each.
		//
		// Worth stating because it is the general shape here: a floor belongs to
		// the lead of the row it was measured on, and how much a fold absorbs is
		// now part of that lead. The transcript's two did not move, because
		// nothing in front of a hot row counts anything.
		dashInReceipt = 15
		wordInReceipt = 26
	)

	for _, s := range []struct {
		name       string
		dash, word int
		cut, whole Model
		row        func(*testing.T, Model, int) string
	}{
		{"transcript", dashInTranscript, wordInTranscript, unfoldedFragment(true), unfoldedFragment(false), hotRow},
		{"receipt", dashInReceipt, wordInReceipt, foldedFragment(true), foldedFragment(false), receiptRow},
	} {
		for width := 1; width <= 120; width++ {
			marked, plain := s.row(t, s.cut, width), s.row(t, s.whole, width)

			// Above the dash floor the fragment must not draw as its finished
			// twin. Below it, it does — and pinning that is what keeps the
			// number honest rather than aspirational.
			if got, want := marked != plain, width >= s.dash; got != want {
				verb := "still tells a fragment from a finished utterance below its measured floor"
				if want {
					verb = "draws a fragment exactly as a finished utterance"
				}
				t.Errorf("%s at width %d %s (dash floor %d): fragment %q, finished %q",
					s.name, width, verb, s.dash, marked, plain)
			}

			if got, want := strings.Contains(marked, "unfinished"), width >= s.word; got != want {
				verb := "has lost the word"
				if got {
					verb = "still carries the word below its measured floor"
				}
				t.Errorf("%s at width %d %s (word floor %d): %q",
					s.name, width, verb, s.word, marked)
			}
		}
	}
}

// Nothing this surface draws may run past the width it was given. The viewport
// clips rather than wraps, and a row cut by the clip looks exactly like a row
// that happened to end there — which is the one thing a screen arguing that it
// shows you what it dropped is not allowed to do.
func TestNoRowRunsPastTheWidthItWasGiven(t *testing.T) {
	m := record(fixtureBudget * 3)

	// With a vote on screen, so the caret's columns and the vote column are in
	// the arithmetic. They are the two things this pass added in front of every
	// row, and they are what would push one past the margin.
	m.move(-4)
	m.vote(memory.Up)
	m.move(-2)
	m.vote(memory.Down)

	// And a sentence long enough to wrap, with the caret on it, so the rows a
	// wrapped bit hangs under this row are in the sweep as well. Without it every
	// bit here says "bit 31" and nothing on the screen is ever more than one row —
	// the continuation rows would be built by nothing and clipped by nobody.
	// move clamps at the end of the view, which is where the long bit is.
	m.say(localHandle, strings.Repeat("a sentence that will not fit on one row ", 8))
	m.move(len(m.shown))

	for _, width := range []int{200, 100, 80, 40, 24, 20, 16, 12, 8, 4, 1} {
		for _, open := range []bool{false, true} {
			for i, row := range strings.Split(shot(m, width, open), "\n") {
				if w := lipgloss.Width(row); w > width {
					t.Errorf("transcript at width %d (open=%v): row %d is %d wide: %q",
						width, open, i+1, w, ansi.Strip(row))
				}
			}
		}
	}
}

// longAnswer is a reply that does not fit a row at any terminal anybody runs. It
// is one sentence with no punctuation worth wrapping on, so a wrap that broke on
// something other than a space would show up as a lost or doubled word.
const longAnswer = "the three columns that drifted are created_at and updated_at and deleted_at " +
	"and every one of them was added by the soft delete migration that nobody " +
	"backfilled afterwards which is why staging disagrees with production about " +
	"rows that were never touched by anything"

// reading is the fixture the tests below are measured on: one short question and
// one long answer, with the caret on the answer, which is where the caret is when
// an answer arrives.
//
// Nobody has voted in it, so there is no vote column and the floors it produces
// are its own. A floor belongs to its fixture — the vote column is columns of the
// lead, and a lead is what every floor on this surface is a function of.
func reading(cut bool) Model {
	m := New()
	m.say(localHandle, "which columns drifted")
	m.utter(m.persona.Handle(), memory.Utterance{Text: longAnswer, Truncated: cut})
	return m
}

// block is the rows the caret's own bit draws, as a terminal with no colour shows
// them, and it fails rather than measures when the caret is on nothing this frame
// draws.
func block(t *testing.T, m Model, width int) []string {
	t.Helper()
	f := m.frame()
	for i, b := range f.bits {
		if b.ID == f.mark {
			return bare(m, width)[i]
		}
	}
	t.Fatal("the caret is on nothing this frame draws, so there is no block to read")
	return nil
}

// The caret's row shows every word the record holds, and every other row is cut.
//
// Both halves, because either alone is satisfiable by something useless: a
// surface that wrapped nothing passes the second, and one that wrapped everything
// passes the first while costing the fold its antecedent — twelve bits at five
// rows each do not fit a screen, so the rows drawn cooling would be below the
// margin and a fold would arrive with its warning off screen.
//
// Read off drawn rows with the escapes stripped, and asserted as words rather
// than as a string: the block carries a caret, a handle and a hanging indent, and
// what is claimed is that the sentence comes back whole and in order, not that
// the screen is spaces in particular places.
func TestTheCaretsRowShowsEveryWordAndEveryOtherRowIsCut(t *testing.T) {
	m := reading(false)
	want := strings.Fields(longAnswer)

	for _, width := range []int{200, 100, 80, 60, 40, 30} {
		rows := block(t, m, width)

		got := strings.Fields(strings.Join(rows, " "))
		if len(got) < len(want) || !slices.Equal(got[len(got)-len(want):], want) {
			t.Errorf("the caret's row at width %d drew %d rows carrying %q, want the whole of the answer",
				width, len(rows), strings.Join(got, " "))
		}

		// And the bit above it is still one row, cut with the screen's own mark.
		// The question is short, so it is given something long to say first.
		short := reading(false)
		short.move(-1)
		if other := bare(short, width)[1]; len(other) != 1 || !strings.Contains(other[0], "…") {
			t.Errorf("a bit the caret is not on drew %d rows at width %d and was not cut: %q",
				len(other), width, strings.Join(other, "\n"))
		}
	}
}

// Only ever one row more than one. The fold budget is counted in bits ([fixtureBudget]),
// and it survives a variable-height row only because the height of the view is
// the number of bits plus one bit's worth of wrapping — never a multiple of it.
// A second expanded row is the beginning of that bound not holding, so it is
// pinned as a bound rather than left as an intention.
func TestOnlyTheCaretsRowIsEverMoreThanOneRow(t *testing.T) {
	m := talk(New(), 9)
	for i := range 4 {
		m.say(m.persona.Handle(), fmt.Sprintf("%d %s", i, longAnswer))
	}

	for _, width := range []int{100, 60, 40} {
		for at := range len(m.shown) {
			m := m
			m.mark = m.shown[at]

			groups := bare(m, width)
			tall := 0
			for i, g := range groups {
				if len(g) > 1 {
					tall++
					if m.shown[i] != m.mark {
						t.Errorf("width %d: bit %d drew %d rows and the caret is not on it",
							width, i+1, len(g))
					}
				}
			}
			if tall > 1 {
				t.Errorf("width %d, caret on bit %d: %d bits drew more than one row, and the row budget rests on that being at most one",
					width, at+1, tall)
			}
		}
	}
}

// An expanded row of prose shows every word the record holds **and the line
// breaks it holds**, which is a reversal of what this check used to assert.
//
// It was [TestAnExpandedRowShowsEveryWordAndNotTheLineBreaks], and its last
// clause pinned the collapse in [drawn] as a residual: a message's newlines and
// its indentation were joined into one paragraph before anything was drawn, so
// a person's pasted source arrived as a wall. [markdown] closed that for a
// message written with a fence, a heading or a list item in it and left it open
// for everything else, which turned out to be where a person's own text lives.
// [wrapped] closes the rest.
//
// **The rename is the finding.** The old test went on passing through the
// change that falsified its own name: its fixture had no blank line in it, so
// the one assertion standing for "no line break reached the screen" — no blank
// row — was true either way. A check named for a behaviour is not a check on it,
// and the fixture is what decides which. This one carries a blank line, an
// indented line and a line too long for the width, so each half of the claim has
// something in the material to fail on.
//
// The fixture is prose on purpose: [markdown] takes any message written as a
// document, and this is the other path. The document half is
// [TestADocumentKeepsItsHeadingsListsAndCodeBlock].
func TestAnExpandedRowKeepsTheWordsAndTheLineBreaksTheRecordHolds(t *testing.T) {
	reply := "Three things, and the third is the one that matters:\n\n    backfill first, then drop,\nand verify twice against a staging box nobody else is using"

	if structured(reply) {
		t.Fatalf("this fixture is meant to be prose and [markdown] reads it as a document, so the claim under test is not the one being checked")
	}

	m := record(1)
	m.utter(m.persona.Handle(), memory.Utterance{Text: reply})

	if got := m.shown.Bits(m.store)[1].Payload.(memory.Utterance).Text; got != reply {
		t.Fatalf("the record holds %q, want the reply with its own line breaks", got)
	}

	rows := block(t, m, 60)
	if len(rows) < 4 {
		t.Fatalf("the reply drew %d row(s) at width 60, so nothing here is being tested", len(rows))
	}

	got := strings.Fields(strings.Join(rows, " "))
	want := strings.Fields(reply)
	if !slices.Equal(got[len(got)-len(want):], want) {
		t.Errorf("the expanded reply reads %q, want every word of %q", strings.Join(got, " "), reply)
	}

	// The blank line the record holds is a blank row on the screen, and it is the
	// only one: a message that ends without a newline does not gain a row, and a
	// wrap does not invent one.
	blank := 0
	for _, r := range rows {
		if strings.TrimSpace(r) == "" {
			blank++
		}
	}
	if blank != 1 {
		t.Errorf("the block draws %d blank row(s) and the message holds one blank line:\n%s",
			blank, strings.Join(rows, "\n"))
	}

	// And the indented line is drawn indented, measured against the line above it
	// rather than against a column this test would have to know.
	//
	// Row 0 carries the caret and the handle, so the column the sentence starts in
	// is read off the last line of the message — which is unindented in the record
	// and drawn at the block's own lead — rather than off it.
	at := func(row string) int { return len(row) - len(strings.TrimLeft(row, " ")) }
	first, indented := at(rows[len(rows)-1]), at(rows[3])
	if indented-first != 4 {
		t.Errorf("the indented line starts %d columns past the first line and the record indents it by 4:\n%s",
			indented-first, strings.Join(rows, "\n"))
	}
}

// A line too long for the room is wrapped, its continuations carry its own
// indentation, and nothing is dropped doing it.
//
// This is the half of [wrapped] that is not "leave it alone", and it is asserted
// on [drawn] rather than on a frame because what is claimed is about the lines —
// the frame's own copy of the claim is the indent check in
// [TestAnExpandedRowKeepsTheWordsAndTheLineBreaksTheRecordHolds], and the width
// backstop is [TestNoExpandedRowRunsPastTheWidthItWasGiven].
//
// The room is 20 and the fixture's indent is 4, so a continuation at column 0
// and a continuation at column 4 are different frames rather than the same one
// rounded — at a room wide enough for the whole line neither the wrap nor the
// indent would be reached at all, which is the fixture failure this package
// keeps finding.
func TestAWrappedLineCarriesItsOwnIndentAndLosesNoWords(t *testing.T) {
	const text = "head\n    a deeply considered line that will not fit in twenty columns\ntail"

	rows := drawn(memory.Utterance{Text: text}, 20, false)
	if len(rows) < 4 {
		t.Fatalf("the fixture drew %d rows at width 20 and it is meant to wrap:\n%s", len(rows), strings.Join(rows, "\n"))
	}

	if rows[0] != "head" || rows[len(rows)-1] != "tail" {
		t.Errorf("the lines either side of the wrapped one read %q and %q, want \"head\" and \"tail\":\n%s",
			rows[0], rows[len(rows)-1], strings.Join(rows, "\n"))
	}
	for i, r := range rows[1 : len(rows)-1] {
		if !strings.HasPrefix(r, "    ") {
			t.Errorf("row %d of the wrapped line reads %q, want it to carry the four columns the speaker indented it by:\n%s",
				i+1, r, strings.Join(rows, "\n"))
		}
	}
	if got, want := strings.Fields(strings.Join(rows, " ")), strings.Fields(text); !slices.Equal(got, want) {
		t.Errorf("the wrapped block reads %q, want every word of %q", got, want)
	}
}

// Nothing [wrapped] draws runs past the width it was given, at any width, on
// text whose lines are longer than the terminal and whose indentation is deeper
// than the terminal.
//
// The second half is the one that needs a check rather than an argument: the
// continuation indent is the only thing on this path that adds columns to a row
// the wrap already sized, so an indent wider than the room is the shape that
// puts a row past the margin. It is clamped at half the width, and the CJK line
// is here for the reason [TestAnExpandedRowSurvivesAWidthOfNothing] carries one
// — [ansi.Wrap] comes back two columns wide at a limit of one, so [clip] is
// load-bearing and an all-ASCII fixture never asks it to be.
//
// **Corroborating rather than sole, and named as such.** Run against the
// mutation table for this change, every mutant this catches is also caught by
// [TestAnExpandedRowSurvivesAWidthOfNothing] — including one written to separate
// them, a wrap arm clipping to one column past what it was given. It is kept for
// the reason the package already keeps two others: the claim is the one a reader
// of this path comes looking for, over the material that path is now for, and a
// test file a person reads needs the sentence itself.
func TestNoExpandedRowRunsPastTheWidthItWasGiven(t *testing.T) {
	const text = "short\n" +
		"                                a line indented far past a narrow terminal\n" +
		"    ゆっくりと進む長い行がここにあります\n" +
		"a plain line of prose that is longer than most terminals are wide by some margin"

	for width := 1; width <= 60; width++ {
		for _, row := range drawn(memory.Utterance{Text: text}, width, false) {
			if got := lipgloss.Width(row); got > width {
				t.Errorf("at width %d a row is %d columns wide: %q", width, got, row)
			}
		}
	}
}

// A tab never reaches a drawn row, because every measurement on this surface
// counts it as one column and a terminal draws it to the next stop.
//
// This is the check under the tab expansion in [wrapped], and it cannot be a
// width check: the width test above measures with [lipgloss.Width], which is the
// very function that gets a tab wrong, so it passes on a row that runs three
// columns past the margin on a real terminal. What is mechanical is the
// character, so that is what is asserted. [markdown] does the same thing on the
// document path for the same reason and says so in its own comment.
//
// The record keeps the tab — [TestDrawingADocumentChangesNothingTheRecordHolds]
// holds that for the other path, and [oneLine] is untouched by either.
func TestNoDrawnRowCarriesATab(t *testing.T) {
	const text = "func main() {\n\tfor i := range 3 {\n\t\tfmt.Println(i, \"a line long enough that it has to wrap somewhere\")\n\t}\n}"

	if structured(text) {
		t.Fatal("this fixture is meant to reach the plain path and [structured] sends it to [markdown]")
	}
	for _, width := range []int{16, 20, 40, 80, 200} {
		rows := drawn(memory.Utterance{Text: text}, width, false)
		for i, row := range rows {
			if strings.Contains(row, "\t") {
				t.Errorf("at width %d row %d carries a tab, which measures one column and draws up to eight: %q",
					width, i+1, row)
			}
		}
	}
}

// An indentation deeper than the terminal is wide does not squeeze a line's own
// words off the screen.
//
// The continuation indent is the one thing [wrapped] adds to a row, and left
// unclamped it is unbounded: a line indented thirty-two columns drawn into
// sixteen leaves nothing for the words, and because [clip] cuts the row back to
// the width, the failure is not a row past the margin — it is a column of
// ellipses where a line used to be. That is why this is a *loss* check rather
// than a width check, and why the width sweep beside it comes back green on the
// same mutation.
//
// Asserted from [textFloor] up, which is the narrowest room [transcript] ever
// asks a block for: below that a row is cut to one line and this path is not
// reached in the program at all.
func TestADeeplyIndentedLineKeepsItsOwnCharacters(t *testing.T) {
	text := "head\n" + strings.Repeat(" ", 32) +
		"a nested line whose indentation is deeper than a narrow terminal is wide"

	strip := func(s string) string { return strings.Join(strings.Fields(s), "") }
	want := strip(text)

	for width := textFloor; width <= 60; width++ {
		rows := drawn(memory.Utterance{Text: text}, width, false)
		if got := strip(strings.Join(rows, " ")); got != want {
			t.Errorf("at width %d the block draws %q, want every character of the two lines:\n%s",
				width, got, strings.Join(rows, "\n"))
		}
	}
}

// A speaker who did not finish is marked, and an expanded row never trades a word
// for the mark. In [said] the two compete for one row's columns and the sentence
// gives way; here there is always another row, so the mark takes one of its own
// rather than the end of the sentence.
func TestAnExpandedFragmentNeverTradesAWordForItsMark(t *testing.T) {
	m := reading(true)
	want := strings.Fields(longAnswer)

	for _, width := range []int{200, 100, 60, 40, 30} {
		rows := block(t, m, width)
		got := strings.Fields(strings.Join(rows, " "))

		marked := strings.Contains(strings.Join(rows, "\n"), "╌")
		if !marked {
			t.Errorf("the expanded fragment at width %d carries no mark at all, so it claims its speaker finished:\n%s",
				width, strings.Join(rows, "\n"))
		}

		// Every word, with the mark's own words allowed to follow them.
		if len(got) < len(want) || !slices.Equal(got[len(got)-len(want)-len(strings.Fields(unfinished[0])):len(got)-len(strings.Fields(unfinished[0]))], want) {
			t.Errorf("the expanded fragment at width %d reads %q, want every word of the answer and then the mark",
				width, strings.Join(got, " "))
		}
	}
}

// Where the caret's row goes back to being cut, pinned in both directions.
//
// Below this width the fixed columns have already outrun the terminal — the
// handle is clamped at [nameFloor] and the sentence column at its own floor — and
// wrapping into a column that narrow produces rubble rather than a reading. That
// regime is one [clip] and [nameColumn] already concede in as many words; what is
// not conceded is being quiet about where it starts.
//
// Measured off drawn rows and belonging to [reading], which has no vote column. A
// fixture somebody has voted in has a wider lead and a higher floor.
func TestTheCaretsRowIsCutWhereTheArrangementAlreadyGaveUp(t *testing.T) {
	const whole = 24

	m := reading(false)
	for width := 1; width <= 120; width++ {
		rows := block(t, m, width)
		if got, want := len(rows) > 1, width >= whole; got != want {
			verb := "is cut"
			if got {
				verb = "is drawn whole below the width where that was measured to stop"
			}
			t.Fatalf("the caret's row at width %d %s (whole at %d): %q",
				width, verb, whole, strings.Join(rows, "\n"))
		}
	}
}

// The anchors name the rows they were actually drawn on, across a caret row that
// wraps.
//
// [anchors] is a cache with a one-frame life, and its whole value is that the
// renderer counts it instead of the caller: `ctrl+u` scrolls to anchors.scar and
// [Model.sync] scrolls to anchors.mark, and neither of them can tell a wrong
// number from a right one. While every bit was one row the count was the loop
// index and there was nothing to get wrong. It is not any more — the caret's row
// is drawn whole — so each continuation line has to be counted as it is emitted,
// by a bare increment inside a loop that nothing else in this package observes.
//
// Delete that increment and the suite was green while anchors.scar was short by
// the height of the block, which lands the one key that follows a receipt in the
// middle of the caret's own paragraph and calls it the receipt. That is this
// project's signature defect — a load-bearing line with no witness — so the
// arrangement is built to reach it: the caret on a held bit at the head of the
// view, its answer wrapped, and the fold behind it.
func TestTheAnchorsNameTheRowsTheyWereDrawnOn(t *testing.T) {
	const width = 60

	// The caret has to be taken off the newest bit before the conversation carries
	// on, or it rides away from the answer it is parked on and this measures a row
	// that says "carry on 13". The upvote goes first, because it is what keeps the
	// bit at the head of the view once the fold arrives.
	m := New()
	m.utter(localHandle, memory.Utterance{Text: longAnswer})
	m.vote(memory.Up)
	m.say(localHandle, "carry on 0")
	m.move(-1)
	for i := 1; i < fixtureBudget+2; i++ {
		m.say(localHandle, fmt.Sprintf("carry on %d", i))
	}

	f := m.frame()
	f.width = width
	body, at := transcript(f)
	rows := strings.Split(ansi.Strip(body), "\n")

	// A scar is the one thing on this surface drawn from column 0, marked or not,
	// so it is found by where it starts rather than by what it says — a ladder can
	// drop every word on it and it still hangs into the margin.
	var scars []int
	for i, r := range rows {
		for _, lead := range []string{"─", "┌", caret + "─", caret + "┌"} {
			if strings.HasPrefix(r, lead) {
				scars = append(scars, i)
				break
			}
		}
	}

	// The arrangement, counted off the frame this asserts over rather than assumed
	// of it. Each of these failing means the fixture stopped producing the state
	// the test is named for, which is a finding and not a pass.
	switch {
	case at.rows < 2:
		t.Fatalf("the caret's row drew %d row(s) at width %d, so no continuation is being counted (mark %d):\n%s",
			at.rows, width, at.mark, strings.Join(rows, "\n"))
	case len(scars) != 1:
		t.Fatalf("the frame draws %d scars, want exactly one:\n%s", len(scars), strings.Join(rows, "\n"))
	case scars[0] <= at.mark+at.rows-1:
		t.Fatalf("the scar is drawn on row %d and the caret's block ends on row %d — the scar is not below the block, so an uncounted continuation row would not move it",
			scars[0], at.mark+at.rows-1)
	}

	if at.scar != scars[0] {
		t.Errorf("anchors.scar is %d and the scar is drawn on row %d — ctrl+u scrolls to the first, which is %d rows into the caret's own answer:\n%s",
			at.scar, scars[0], scars[0]-at.scar, strings.Join(rows, "\n"))
	}
	if at.mark < 0 || !strings.HasPrefix(rows[at.mark], caret) {
		t.Errorf("anchors.mark is %d and that row does not carry the caret: %q", at.mark, rows[max(at.mark, 0)])
	}
}

// The caret is on screen on the ranked surface too, which is a claim about the
// one line in [ranked] that says how tall its rows are.
//
// [Model.sync] frames the caret's *block* now, and a block of no rows is nothing
// to frame — so a surface that reported zero would silently stop scrolling to its
// own caret, and the next vote would land on a row nobody could see. That line
// reads as bookkeeping and it is not: it is the ranked view's half of a contract
// the transcript owns the other half of, and nothing but this notices when it is
// wrong. The seat's own craft record has this surface putting its caret off screen
// at three of four sizes once already, caught by looking rather than by a test,
// which is what this closes.
func TestTheRankedCaretIsInsideTheFrame(t *testing.T) {
	for _, size := range [][2]int{{100, 30}, {80, 24}, {60, 14}} {
		m := press(judging(sized(size[0], size[1]), 80), "ctrl+t")

		if len(m.list()) < m.viewport.Height() {
			t.Fatalf("%dx%d: the ranked list is %d rows in a viewport of %d, so it does not overflow and nothing here is being tested",
				size[0], size[1], len(m.list()), m.viewport.Height())
		}

		// Walked, because the caret entering at the top of the list is the easy
		// half: every step has to keep it on screen, and the step that leaves the
		// frame is the one a vote is then aimed through.
		for step := range len(m.list()) {
			top, h := m.viewport.YOffset(), m.viewport.Height()
			if at := m.anchors.mark; at < top || at >= top+h {
				t.Fatalf("%dx%d: after %d steps the caret is drawn on row %d and the frame shows rows %d to %d",
					size[0], size[1], step, at, top, top+h-1)
			}
			m = press(m, "down")
		}
	}
}

// A notice is never off screen, whatever the caret is doing.
//
// This is guard's regression, kept as a witness rather than as a story. A save
// that did not reach the disk is drawn below the transcript, the caret cannot
// reach it, and nothing else on the screen says it — so the frame being scrolled
// somewhere else is the whole of what it takes to lose it, and an answer taller
// than the terminal scrolls the frame somewhere else. It went off the bottom of an
// eighty-by-twenty-four frame on the change that made the answer readable: this
// surface committing the failure it exists to report, in the commit that repaired
// a smaller one.
//
// Driven through [Model.Update] and a real keystroke, because that is the only
// path where [Model.saved] runs at all — the notice is raised by the save hook
// failing, not by anything a test can set and mean.
func TestANoticeIsOnScreenUnderAnAnswerTallerThanTheFrame(t *testing.T) {
	s := &saver{err: errors.New("no space left on device")}
	m, _ := Load(memory.NewStore(), nil, nil, s.hook()).
		Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	// Long enough to overflow an eighty-by-twenty-four frame on its own, which is
	// the state the notice was lost in and is an ordinary answer rather than a
	// contrived one.
	answer := m.(Model)
	answer.say(localHandle, strings.TrimSpace(strings.Repeat(longAnswer+" ", 5)))

	// A vote is a change to the record, so the save fires on this keystroke and
	// fails. It lands on the caret, which is riding the answer.
	answer = press(answer, "shift+up")

	switch {
	case !answer.trouble.up():
		t.Fatal("the save hook failed and no notice is up, so there is nothing here to be off screen")
	case answer.anchors.rows <= answer.viewport.Height():
		t.Fatalf("the answer drew %d rows into a viewport of %d, so the frame is not being scrolled away from the notice",
			answer.anchors.rows, answer.viewport.Height())
	}

	frame := ansi.Strip(answer.viewport.View())
	for _, want := range []string{"not on disk", "no space left on device"} {
		if !strings.Contains(frame, want) {
			t.Errorf("the frame does not carry %q while the record is not on disk:\n%s", want, frame)
		}
	}
}

// A continuation row starts in the same column as the sentence above it.
//
// That is the whole of what makes a wrapped bit read as one paragraph under one
// name rather than as rows nobody said: the handle column is blank on a
// continuation, and blank in a column only means "the same speaker" if the column
// is still there. It is alignment rather than colour, so it is what survives a
// terminal with neither.
//
// Measured off drawn rows at every width the block is drawn whole at, because the
// margin, the vote column and the handle column all move with the terminal and
// the indent is the sum of the three.
func TestAContinuationRowHangsUnderItsOwnSentence(t *testing.T) {
	for _, m := range []Model{reading(false), keptReading(t)} {
		checked := 0
		for width := 1; width <= 120; width++ {
			rows := block(t, m, width)

			// Below the fixture's own floor there is no continuation to place, and
			// the floor moves with the fixture — the vote column is columns of the
			// lead, and every floor on this surface is a function of the lead. So the
			// sweep runs from one and counts what it actually checked, rather than
			// starting at a width that is only right for one of these.
			if len(rows) < 2 {
				continue
			}
			checked++

			// In columns rather than in bytes, which is not pedantry: at
			// twenty-four the handle is cut to "qw…" and the ellipsis is three
			// bytes wide and one column wide. Counted in bytes this passes and
			// fails at different widths than the screen does.
			cut := strings.Index(rows[0], "the three")
			if cut < 0 {
				t.Fatalf("width %d: the caret's first row does not carry the start of its own sentence: %q",
					width, rows[0])
			}
			at := lipgloss.Width(rows[0][:cut])
			for i, r := range rows[1:] {
				if got := lipgloss.Width(r) - lipgloss.Width(strings.TrimLeft(r, " ")); got != at {
					t.Errorf("width %d: continuation row %d starts in column %d and the sentence above it starts in column %d:\n%s",
						width, i+2, got, at, strings.Join(rows, "\n"))
				}
			}
		}

		// And the sweep has to have found the case. A fixture that stopped being
		// drawn whole would otherwise pass this by checking nothing.
		if checked < 90 {
			t.Errorf("only %d of 120 widths drew the caret's row whole, so this asserts almost nothing", checked)
		}
	}
}

// keptReading is [reading] with the answer upvoted, so the vote column is in the
// lead the indent is measured against. Without it the sweep above never sees the
// column that pushed the sentence five columns right.
func keptReading(t *testing.T) Model {
	t.Helper()
	m := reading(false)
	m.vote(memory.Up)
	if !m.frame().voted() {
		t.Fatal("nothing in the fixture carries a vote, so the vote column is not in this measurement")
	}
	return m
}

// An answer taller than the frame is shown from its beginning.
//
// [Model.sync] pins the caret's first row to the top of the viewport when its
// block does not fit, which is the same rule the trouble block follows and for
// the same reason: the top of the block is the part carrying the claim — who
// spoke, the reader's own mark, and the start of the sentence. Going to the
// bottom, which is what riding the newest bit does the rest of the time, would
// answer "what did it say" with the end of the answer.
//
// The precondition is asserted rather than assumed: a fixture whose block fits
// would pass this by drawing everything and testing nothing.
func TestAnAnswerTallerThanTheFrameIsShownFromItsBeginning(t *testing.T) {
	m := talk(sized(60, 12), 3)
	m.utter(m.persona.Handle(), memory.Utterance{Text: longAnswer})

	if m.anchors.rows <= m.viewport.Height() {
		t.Fatalf("the answer drew %d rows into a viewport of %d, so nothing here is being tested",
			m.anchors.rows, m.viewport.Height())
	}
	if got, want := m.viewport.YOffset(), m.anchors.mark; got != want {
		t.Errorf("the frame is scrolled to row %d and the caret's row starts at %d — an answer taller than the screen is being shown from somewhere other than its beginning",
			got, want)
	}

	// And a block that fits still rides the newest bit to the bottom, which is what
	// keeps the pending line and the failure block in view while they matter.
	wide := talk(sized(120, 30), 3)
	wide.utter(wide.persona.Handle(), memory.Utterance{Text: longAnswer})
	if wide.anchors.rows > wide.viewport.Height() {
		t.Fatalf("the answer drew %d rows into a viewport of %d, so this half is the other case",
			wide.anchors.rows, wide.viewport.Height())
	}
	if got := wide.viewport.YOffset(); got != 0 {
		t.Errorf("a short conversation is scrolled to row %d, want the bottom of a frame nothing overflows", got)
	}
}

// The width nobody should pass and somebody will. [ansi.Wrap] returns the string
// *unwrapped* when the limit is below one, so a caller handing [saidWhole] a zero
// gets the whole bit on a single line and straight past the margin — a silent
// failure in the one direction this surface may not fail. The clamp is the fix
// and this is the check that it is there.
//
// The second fixture is not decoration. Wrapped to one column, a double-width
// grapheme comes back *two* columns wide — the wrap has nowhere narrower to put
// it — so the per-line cut inside [saidWhole] is what actually holds the promise
// here, and with an all-ASCII fixture nothing would ever ask it to.
func TestAnExpandedRowSurvivesAWidthOfNothing(t *testing.T) {
	for _, text := range []string{longAnswer, "日本語のテキストです"} {
		b := utterance(text, false)
		for _, width := range []int{0, -1, -80} {
			rows := saidWhole(frame{}, b, width)
			if len(rows) < 2 {
				t.Errorf("saidWhole at width %d returned %d row(s), want the text wrapped rather than handed back whole",
					width, len(rows))
			}
			for i, r := range rows {
				if w := lipgloss.Width(r); w > 1 {
					t.Errorf("saidWhole(%.10q…) at width %d: row %d is %d wide: %q",
						text, width, i+1, w, r)
				}
			}
		}
	}

	// The two rungs at the bottom of the mark's ladder, which [transcript] cannot
	// reach — it draws a row whole only when the sentence column is at least
	// textFloor, and that is wider than the widest wording of the mark, so a row
	// on the screen never sees either of these. They are reachable here, through
	// the function's own front door, and this is the only thing that reaches them:
	// without it two branches of shipped code have no witness at all, which is the
	// shape this file keeps having to hunt.
	//
	// A fragment marked at one column is the bare dash and there is no rung under
	// it, because a row with no mark claims its speaker finished. A fragment with
	// nothing in it is the mark alone with no leading gap, because a speaker who
	// got nothing out should read flush in their column.
	if rows := saidWhole(frame{}, utterance(longAnswer, true), 0); rows[len(rows)-1] != "╌" {
		t.Errorf("a fragment at one column ends %q, want the bare dash — there is no width at which a fragment draws unmarked",
			rows[len(rows)-1])
	}
	if rows := saidWhole(frame{}, utterance("", true), 80); len(rows) != 1 || rows[0] != unfinished[0] {
		t.Errorf("a fragment with nothing in it drew %q, want the mark alone and no leading gap", rows)
	}
}

// The scar never spends the quotation's columns on its span, and a wider
// terminal never shows less of what somebody said.
//
// Both halves are one defect seen from two sides, and it was live before this
// test existed. Written the obvious way — widest rung first, [fit] takes the
// first that fits — the rung carrying the span has fourteen fewer columns to
// quote into, so the step up into it *cost* thirteen characters of somebody's
// sentence: a terminal one column wider showing strictly less. So the span is
// taken only once the whole quotation already fits beside it.
//
// The span is still the right thing to show where no quotation fits at all,
// which is the third rung and is why this is not "the span goes first". Nothing
// is being traded there; there is simply no room for words.
func TestSeamNeverSpendsTheQuotationsColumnsOnItsSpan(t *testing.T) {
	// Two fixtures because the two columns of a quotation have different floors
	// and only one of them belongs to a short handle. "me" never truncates, so a
	// sweep over it alone says nothing at all about the name.
	for _, who := range []string{"me", "coordinator-7"} {
		t.Run(who, func(t *testing.T) { seamSweep(t, who) })
	}
}

// drawnName is the speaker a scar's row attributes its quotation to, read off
// the row: the last field before the opening quotation mark.
func drawnName(row string) string {
	i := strings.Index(row, `"`)
	if i < 1 {
		return ""
	}
	fields := strings.Fields(row[:i])
	return fields[len(fields)-1]
}

func seamSweep(t *testing.T, who string) {
	t.Helper()
	f, c := named(t, who,
		"migration started against staging",
		"staging backfill running",
		"backfill migration finished",
		"staging looks green")

	widest := ansi.Strip(seam(f, c, 400))
	span := fmt.Sprintf("%s–%s", c.From().Format("15:04"), c.To().Format("15:04"))
	quote := who + ` "staging looks green"`
	if !strings.Contains(widest, span) || !strings.Contains(widest, quote) {
		t.Fatalf("seam at full width = %q, want a span and a whole quotation to trade off", widest)
	}

	// said is what the row shows of the sentence: nothing when the scar has no
	// room to quote at all, and otherwise however much of it survived the cut.
	said := func(row string) int {
		i := strings.Index(row, `"`)
		if i < 0 {
			return 0
		}
		return lipgloss.Width(strings.Trim(row[i:], `"…`))
	}

	// name is the columns the speaker gets, which now gives way by the column
	// under width pressure the way every handle column here does. It is swept
	// alongside the words because the two are one arrangement: the first attempt
	// to buy words back at sixty columns did it by shrinking the name, and a rule
	// that shrinks one to feed the other is exactly where a wider terminal starts
	// showing less.
	name := func(row string) int { return lipgloss.Width(drawnName(row)) }

	last, lastName, names, sawQuoteAlone := 0, 0, 0, false
	for width := 1; width <= lipgloss.Width(widest); width++ {
		got := ansi.Strip(seam(f, c, width))
		hasSpan, hasQuote := strings.Contains(got, span), said(got) > 0

		if n := said(got); n < last {
			t.Errorf("seam at width %d shows %d columns of the sentence where width %d showed %d: %q",
				width, n, width-1, last, got)
		} else {
			last = n
		}
		if n := name(got); n < lastName {
			t.Errorf("seam at width %d shows %d columns of the speaker's name where width %d showed %d: %q",
				width, n, width-1, lastName, got)
		} else {
			// A cut name that does not say it was cut is the coordinator-7 /
			// coordinator-9 failure [widest]'s own doc names: two speakers arriving
			// on screen as one string. It is the reason the name is truncated here
			// rather than sliced.
			if shown := drawnName(got); shown != "" && shown != who && !strings.HasSuffix(shown, "…") {
				t.Errorf("seam at width %d cut the speaker's name to %q with no mark: %q", width, shown, got)
			}
			if n > 0 && n != lastName {
				names++
			}
			lastName = n
		}
		if hasSpan && hasQuote && !strings.Contains(got, quote) {
			t.Errorf("seam at width %d took the span and cut the quotation to pay for it: %q", width, got)
		}
		sawQuoteAlone = sawQuoteAlone || (hasQuote && !hasSpan)
	}

	// And there is a band where the whole quotation is shown without the span.
	// Without it the rule above is satisfied by a ladder that never separates
	// them, which is the arrangement that had the defect.
	if !sawQuoteAlone {
		t.Error("no width quotes a bit without also showing the span, so the two never trade and this test proves nothing")
	}

	// The name sweep is only a sweep where the name can move. On a handle short
	// enough never to be cut it is one value at every width, and the monotonicity
	// check above it is vacuous — which is what the second fixture is for.
	if lipgloss.Width(who) > nameFloor+1 && names < 2 {
		t.Errorf("the speaker's column took %d distinct value(s) across the whole sweep, so nothing here checks that it gives way", names)
	}
}

// What a fold holds is a count. What it stands for is not in it.
//
// [oneLine] answers "what does the record hold" from one bit and nothing else,
// and for a fold that is the count — the bits it absorbed are in the store, not
// in the payload. It used to answer with the four most frequent words of the
// fold's bag, which was the only content account a caller with no store could
// get and was assembled by this package out of a structure memory/cool.go
// documents as destroying what was said.
//
// Nothing that draws reaches this branch now, and this is its witness through
// the function's own front door. Written because a branch with no witness is
// the shape this package keeps having to hunt, and because the wrong repair for
// it — deleting the arm and letting a fold fall into "unhandled kind" — would
// put "<unrendered memory.Compaction>" back on a screen the day somebody adds a
// third surface.
func TestOneLineSaysWhatAFoldHoldsAndNothingItStandsFor(t *testing.T) {
	f, c := scarred(t, "the deploy failed", "deploy again", "and again")

	fold := memory.Bit{Payload: c}
	if got := oneLine(fold); got != "3 bits" {
		t.Errorf("oneLine on a fold = %q, want the count alone", got)
	}

	// And the words the bag would offer are absent, which is the half a count
	// alone does not assert: "3 bits · deploy again" also starts with the count.
	for _, w := range topWords(c.Bag(), 4) {
		if strings.Contains(oneLine(fold), w) {
			t.Errorf("oneLine on a fold carries the bag's word %q: %q", w, oneLine(fold))
		}
	}

	// [said] routes around it, which is what makes the branch unreached rather
	// than wrong. Asserted here so the disclosure in its doc has a check under it.
	if got := said(f, fold, 200); got == oneLine(fold) {
		t.Errorf("said on a fold = %q, the same as oneLine — the drawn row is not going through scarLine", got)
	}
}

// Everything the scar says about its content is something somebody actually
// said. That is the whole of D59(j): the label used to be the four most frequent
// words in the fold's bag, and memory/cool.go documents that bag as destroying
// what was said about the things it counts — so the most-read row in the program
// was a phrase assembled by this package and attributable to nobody.
//
// The check is that the quoted characters are a prefix of exactly one absorbed
// bit, which no word list can pass: sorted-by-frequency words are not a run of
// any sentence, and the two real examples that provoked this ("name same box s",
// "understood s before migration") are not runs of anything at all. Read off the
// drawn row rather than off [frame.quoted], because the defect was always in
// what got drawn.
func TestAScarQuotesSomethingSomebodyActuallySaid(t *testing.T) {
	f, c := scarred(t,
		"migration started against staging",
		"the backfill is running now",
		"staging looks green after the backfill")

	row := ansi.Strip(seam(f, c, 200))
	open, shut := strings.Index(row, `"`), strings.LastIndex(row, `"`)
	if open < 0 || shut <= open {
		t.Fatalf("the scar says nothing anybody said: %q", row)
	}
	said := row[open+1 : shut]

	hits := 0
	for id := range c.Absorbed() {
		b, ok := f.store.Get(id)
		if ok && strings.HasPrefix(oneLine(b), said) {
			hits++
		}
	}
	if hits != 1 {
		t.Errorf("the scar quotes %q, which begins %d of the bits it absorbed — want exactly one, or it is not a quotation",
			said, hits)
	}

	// And the words the bag would have offered are not on the row. Without this,
	// a scar that drew both would pass the check above.
	for _, w := range topWords(c.Bag(), 4) {
		if strings.Contains(row, " "+w+" ") && !strings.Contains(said, w) {
			t.Errorf("the scar still carries the bag's word %q outside its quotation: %q", w, row)
		}
	}
}

// A quotation closes before this surface's own mark, never around it.
//
// Quotation marks are an assertion that what is between them is exactly what
// somebody said, and `qwen3.5 "the three steps are, ╌ unfinished ╌"` puts this
// program's vocabulary inside that assertion — the forgery [recordReply] refuses
// to commit from the other direction, arrived at through a new door. It was the
// first thing this drew, and it was found by looking at the row rather than by
// reasoning about it.
//
// It is reachable by keystrokes: keep a reply that ran out of room, let the hold
// lapse, keep talking, and the fold takes it with the vote still standing.
func TestAQuotedFragmentIsMarkedOutsideItsOwnWords(t *testing.T) {
	m := sized(100, 30)
	for i := range 6 {
		m.say(localHandle, lines[i%len(lines)])
	}
	m.recordReply(persona.Answer{Text: "the three steps are, first,", Truncated: true})
	m = heldSince(m, holdFor+time.Second)
	m = talk(m, 22)

	f, folds := m.frame(), 0
	for _, b := range f.bits {
		c, cold := b.Payload.(memory.Compaction)
		if !cold {
			continue
		}
		q, ok := f.quoted(c)
		if !ok {
			continue
		}
		if u, said := q.Payload.(memory.Utterance); !said || !u.Truncated {
			continue
		}
		folds++

		for _, width := range []int{200, 120, 100, 90, 80} {
			row := ansi.Strip(seam(f, c, width))
			shut := strings.LastIndex(row, `"`)
			if shut < 0 {
				continue
			}
			quoted := row[strings.Index(row, `"`)+1 : shut]
			for _, mark := range unfinished {
				if strings.Contains(quoted, mark) {
					t.Errorf("the scar at width %d quotes the speaker as having said %q: %q",
						width, mark, row)
				}
			}
			if !strings.Contains(row[shut:], "╌") {
				t.Errorf("the scar at width %d quotes a fragment as though its speaker finished: %q",
					width, row)
			}
		}
	}
	if folds != 1 {
		t.Fatalf("%d scars in this frame quote a fragment, want exactly 1 — the fixture is not producing the arrangement", folds)
	}
}

// The other half of that rule, and the one that was nearly true: a mark the
// *speaker* typed stays inside their own quotation.
//
// [unmarked] used to decide by suffix alone, so a participant whose message ended
// in the surface's own glyph had their characters taken off them and re-attributed
// to the program — `all done ╌ unfinished ╌` drew as `me "all done" ╌ unfinished
// ╌`, and a bit that was nothing but the glyph drew as `me ""`. Two falsehoods in
// one row: the surface claims somebody stopped when they finished, and the marks
// whose whole job is verbatim drop characters that were typed.
//
// U+254C makes this near unreachable by accident, which is why it is a check and
// not a panic. It is trivially reachable on purpose, and this surface's answer to
// "could a participant forge this" may not be "only if they try".
func TestASpeakersOwnMarkStaysInsideTheirQuotation(t *testing.T) {
	// Every one of these finishes. Nothing on the drawn row may say otherwise, and
	// every character has to be inside the marks.
	for _, text := range []string{"all done ╌ unfinished ╌", "╌", "shipped ╌"} {
		f, c := named(t, "me", "the deploy failed on staging tonight", text)
		row := ansi.Strip(seam(f, c, 200))

		open, shut := strings.Index(row, `"`), strings.LastIndex(row, `"`)
		if open < 0 || open == shut {
			t.Fatalf("the scar for %q draws no quotation at all: %q", text, row)
		}
		if quoted := row[open+1 : shut]; quoted != text {
			t.Errorf("the scar quotes %q of a bit that reads %q: %q", quoted, text, row)
		}
		if after := row[shut+1:]; strings.Contains(after, "╌") && !strings.Contains(after, "──") {
			t.Errorf("the scar marks a finished speaker as unfinished: %q", row)
		}
	}

	// And the mark still comes off a bit that really was cut, or the fix above is
	// [unmarked] doing nothing at all — which the fragment test would catch, but on
	// a fixture this one does not share. Built here rather than borrowed because a
	// truncated payload is a different address and cannot be swapped into a store
	// after the fact.
	s := memory.NewStore()
	window := []memory.Bit{
		s.Put(memory.Bit{
			At:      time.Date(2026, 8, 11, 9, 0, 0, 0, time.UTC),
			From:    memory.Handle{Ref: "tyler", Display: "me"},
			Channel: channel,
			Payload: memory.Utterance{Text: "the deploy failed on staging tonight"},
		}),
		s.Put(memory.Bit{
			At:      time.Date(2026, 8, 11, 9, 1, 0, 0, time.UTC),
			From:    memory.Handle{Ref: "tyler", Display: "me"},
			Channel: channel,
			Payload: memory.Utterance{Text: "stopped here", Truncated: true},
		}),
	}
	cold := memory.Cool(window)
	s.Put(cold)
	cut := frame{store: s, clock: atNine, votes: map[string]memory.Score{}}
	if got, _ := cut.quotation(cold.Payload.(memory.Compaction), 200); !strings.HasSuffix(got, `"stopped here" ╌ unfinished ╌`) {
		t.Errorf("a fragment quotes as %q, want the mark outside the closing quote", got)
	}
}

// With nobody voting — which at three standing votes in thirty-four bits is
// nearly every scar on the live record — the scar quotes the *last* bit it took.
//
// This is the tie-break and it is a decision rather than a fallback, so it is
// pinned as one. The scar sits in the slot its bits vacated, the row directly
// beneath it is the oldest thing still on screen, and the newest thing the fold
// absorbed is the row that used to sit exactly there — so the seam reads straight
// down. [memory.View.Rank]'s own tie is view order, which here would name the
// *first* absorbed bit; after a re-fold that is the first thing said in the whole
// session and never changes again.
func TestWithNothingVotedAScarQuotesTheLastBitItTook(t *testing.T) {
	f, c := scarred(t, "first thing said", "second thing said", "third thing said")

	got, _ := f.quotation(c, 200)
	if !strings.Contains(got, "third thing said") {
		t.Errorf("an unvoted scar quotes %q, want the last bit it absorbed", got)
	}

	// Both directions: the first bit is a legal answer under the other tie rule,
	// so it has to be visibly not the one taken.
	if strings.Contains(got, "first thing said") {
		t.Errorf("an unvoted scar quotes %q, which is view order rather than recency", got)
	}
}

// A vote outlives the hold it bought: the bit somebody kept is what the scar
// that eventually took it remembers.
//
// That is ranking doing the summarising, which is this project's own thesis, and
// it is reachable by keystrokes rather than in theory — a hold decays, the bit it
// was protecting folds like any other, and the standing vote does not decay with
// it. Driven through the surface for that reason: an upvote whose hold has
// already lapsed, then a conversation long enough to fold over it.
func TestAKeptBitIsWhatTheScarThatTookItQuotes(t *testing.T) {
	m := back(talk(sized(100, 30), 10), 6)
	m = heldSince(m, holdFor+time.Second)
	kept, ok := m.store.Get(m.mark)
	if !ok {
		t.Fatal("the caret is not on a bit, so nothing was kept")
	}
	m = talk(m, 22)

	f := m.frame()
	var found, newest int
	for _, b := range f.bits {
		c, cold := b.Payload.(memory.Compaction)
		if !cold {
			continue
		}
		ids := slices.Collect(c.Absorbed())
		if !slices.Contains(ids, kept.ID) {
			continue
		}
		found++
		if ids[len(ids)-1] == kept.ID {
			t.Fatal("the kept bit is the last one absorbed, so the tie-break would pass this test on its own")
		}
		q, _ := f.quoted(c)
		if q.ID != kept.ID {
			t.Errorf("the scar that took the kept bit quotes %q, want the bit the vote was cast on (%q)",
				oneLine(q), oneLine(kept))
		}
		newest = len(ids)
	}
	if found != 1 {
		t.Fatalf("the kept bit is inside %d scars in this frame, want exactly one — the fixture is not producing the arrangement", found)
	}
	if newest < 2 {
		t.Fatalf("the scar absorbed %d bits, so there is nothing for the vote to outrank", newest)
	}
}

// The scar quotes a bit into a column wide enough to read or does not quote at
// all, and the floor is measured rather than chosen — [quoteFloor], off the
// frames [TestHarnessScar] prints.
//
// Both directions, because either alone is satisfiable by a mistake. Above the
// floor there is always a quotation, so a scar cannot silently stop having one;
// below it there is never one, so the old behaviour — shedding words one at a
// time down to a single word off a frequency count — cannot come back in.
func TestAScarQuotesAWholeBitOrNothingAtAll(t *testing.T) {
	f, c := scarred(t, "one", "the last thing anybody said before the fold")

	// The narrowest width at which any quotation is drawn, walked rather than
	// computed: every floor on this surface belongs to its own fixture's lead.
	first := 0
	for w := 1; w <= 200; w++ {
		got, _ := f.quotation(c, w)
		switch {
		case got == "" && first > 0:
			t.Fatalf("width %d draws no quotation where width %d did", w, first)
		case got != "" && first == 0:
			first = w
		}
		if n := lipgloss.Width(got); n > w {
			t.Errorf("the quotation at width %d is %d columns: %q", w, n, got)
		}
	}
	// Nineteen, measured on this fixture and written down as a number rather than
	// derived from [quoteFloor] — a floor checked against its own constant is a
	// check that cannot fail, and this one could not: raising or lowering the
	// constant moved the assertion with it. The number belongs to the fixture,
	// like every floor on this surface: it is the handle "me", a space, two
	// quotation marks and the floor's own columns of words.
	if first != 19 {
		t.Errorf("the narrowest scar quotation on this fixture is %d columns, was measured at 19", first)
	}

	// And the floor is a floor on the *words*, not on the whole thing, so it is
	// checked against what is inside the marks.
	got, _ := f.quotation(c, first)
	said := got[strings.Index(got, `"`)+1 : strings.LastIndex(got, `"`)]
	if n := lipgloss.Width(said); n < quoteFloor {
		t.Errorf("the narrowest quotation shows %d columns of what was said (%q), under the floor of %d",
			n, said, quoteFloor)
	}
}

// [said] promises that a row is drawn to the width it was handed, and until the
// guard at the end of [frame.quotation] that was nearly true rather than true.
//
// Two floors sit under a quotation — [nameFloor] on the speaker, and [said]'s own
// clamp of any width below one back up to one — so the narrowest thing the
// arithmetic can produce is a three-column name, three columns of punctuation and
// a column of words. Seven columns, whatever the caller asked for. Everything
// wider than a single character is refused before that by [quoteFloor], because a
// one-column cut of it is an ellipsis rather than words; a one-character bit is
// its own whole sentence, passes the floor, and comes out at full width. Measured
// before the guard: `said` on a fold at width 1 drew fifteen columns.
//
// What the overflow costs is not tidiness. Every caller here cuts a long row with
// [clip], which takes the closing quotation mark off and leaves `me "x` — this
// surface asserting that a cut sentence is the whole of what somebody said, which
// is the one reading those two characters exist to rule out, and the reason
// [said]'s doc forbids truncating outside it.
//
// [seam] is deliberately not swept: its ladder ends in [fit], which clips its own
// last rung, so a sweep of it would pass with the guard reverted and be a check
// that cannot fail.
func TestNoScarRunsPastTheWidthItWasGiven(t *testing.T) {
	// "x" is the whole point of this fixture: one character, so it survives the
	// clamp whole and is quotable in a room that has no columns to give it.
	f, c := named(t, "coordinator-7", "the deploy failed on staging tonight", "x")
	fold := memory.Bit{Payload: c}

	for w := 1; w <= 120; w++ {
		if n := lipgloss.Width(said(f, fold, w)); n > w {
			t.Errorf("said on a fold at width %d drew %d columns: %q", w, n, said(f, fold, w))
		}
		for i, line := range saidWhole(f, fold, w) {
			if n := lipgloss.Width(line); n > w {
				t.Errorf("saidWhole line %d on a fold at width %d drew %d columns: %q", i, w, n, line)
			}
		}
	}

	// And the fixture still quotes somewhere, or the sweep above is satisfied by a
	// scar that never draws a quotation at any width and holds nothing.
	quoted := false
	for w := 1; w <= 120; w++ {
		if strings.Contains(said(f, fold, w), `"x"`) {
			quoted = true
			break
		}
	}
	if !quoted {
		t.Error("no width quotes this fixture's bit at all, so the sweep above never reaches the quotation")
	}
}

func TestSeamShowsItsReceipt(t *testing.T) {
	f, c := scarred(t, "the deploy failed", "deploy again", "and again")
	got := ansi.Strip(seam(f, c, 80))
	for _, want := range []string{"3 bits", "09:00–09:02", `me "and again"`} {
		if !strings.Contains(got, want) {
			t.Errorf("seam = %q, missing %q", got, want)
		}
	}
}

// The scar has to stay readable when the terminal will not hold everything it
// wants to say. What it may never drop is the count — the claim — and the key
// that lets someone check it, down to the narrowest width that can hold both.
//
// Below that width it is cut with a mark rather than by the terminal. This test
// used to assert that the count and the key survived width 1, which they did:
// the row came back thirty-six columns wide and the viewport clipped it, so
// what "survived" was off screen and the row ended mid-word with nothing saying
// so. A claim nobody can see is not a claim kept.
func TestSeamKeepsItsClaimAndItsKeyAtAnyWidth(t *testing.T) {
	f, c := scarred(t, "the deploy failed", "deploy again", "and again")

	// The scar's own narrowest rung. The two leading dashes are no longer its to
	// draw — [caretCell] puts them in the margin, and gives one of them up to the
	// caret — so what this measures is the body: a space to join onto them, the
	// count, and the key.
	floor := lipgloss.Width(fmt.Sprintf(" %d bits · ctrl+u", c.Count()))

	for width := floor; width <= 200; width++ {
		got := ansi.Strip(seam(f, c, width))
		for _, want := range []string{"3 bits", "ctrl+u"} {
			if !strings.Contains(got, want) {
				t.Errorf("seam at width %d = %q, missing %q", width, got, want)
			}
		}
		if w := lipgloss.Width(got); w > width {
			t.Errorf("seam at width %d is %d wide: %q", width, w, got)
		}
	}

	for width := 1; width < floor; width++ {
		got := ansi.Strip(seam(f, c, width))
		if w := lipgloss.Width(got); w > width {
			t.Errorf("seam at width %d is %d wide: %q", width, w, got)
		}
		if !strings.HasSuffix(got, "…") {
			t.Errorf("seam at width %d was cut with no mark: %q", width, got)
		}
	}
}

// The caret rides the newest bit until somebody moves it, and then stays where
// they put it. Both halves, because either one alone is a different interaction:
// riding always makes the caret unaimable, and never riding leaves it on the
// first thing anybody said an hour ago.
func TestTheCaretRidesUntilItIsMoved(t *testing.T) {
	m := record(5)
	if !m.riding() {
		t.Fatal("the caret is not on the newest bit after five sends")
	}

	m.say(localHandle, "and another")
	if got := m.shown[len(m.shown)-1]; m.mark != got {
		t.Errorf("the caret did not follow the newest bit: mark %s, newest %s",
			memory.Short(m.mark), memory.Short(got))
	}

	m.move(-2)
	parked := m.mark
	if m.riding() {
		t.Fatal("the caret is still on the newest bit after being moved twice")
	}

	m.say(localHandle, "something else arrives")
	if m.mark != parked {
		t.Errorf("a bit arriving took the caret away from where it was put")
	}

	// Speaking is the exception, and it is the whole reason the caret is not a
	// mode: your own words are what you are looking at the moment you send them.
	m.composer.SetValue("mine")
	m.send()
	if !m.riding() {
		t.Error("sending did not bring the caret back to what was just said")
	}
}

// The caret is an address, so a fold moves it onto the scar that absorbed its
// bit rather than leaving it pointing at a row that is no longer there. An index
// would still be pointing at a row, and it would be a different row.
func TestTheCaretFollowsItsBitIntoTheScar(t *testing.T) {
	m := record(fixtureBudget)
	m.move(-fixtureBudget + 1) // the oldest bit, which the next fold is certain to take
	was := m.mark

	m.say(localHandle, "one too many")
	if m.mark == was {
		t.Fatal("the bit the caret was on was not folded, so nothing is being tested")
	}
	if !slices.Contains(m.shown, m.mark) {
		t.Fatalf("the caret is on %s, which is not in the view at all", memory.Short(m.mark))
	}

	// And it is on the scar that stands for it, which is what makes the move
	// something a person can follow rather than a jump.
	b, ok := m.store.Get(m.mark)
	if !ok {
		t.Fatalf("the caret names %s, which the store does not hold", memory.Short(m.mark))
	}
	c, cold := b.Payload.(memory.Compaction)
	if !cold {
		t.Fatalf("the caret landed on a %T rather than on the fold that absorbed its bit", b.Payload)
	}
	if !slices.Contains(slices.Collect(c.Absorbed()), was) {
		t.Errorf("the caret moved to a scar whose receipt does not name %s", memory.Short(was))
	}
}

// Whatever else happens, the caret is on something the view holds. It is the
// invariant the vote rests on: the key acts on wherever the caret is, so a caret
// pointing at nothing is a key that either does nothing or does something
// somewhere the person cannot see.
func TestTheCaretIsAlwaysOnSomethingTheViewHolds(t *testing.T) {
	m := New()

	for i := range fixtureBudget * 6 {
		m.say(localHandle, fmt.Sprintf("bit %d", i))
		switch i % 5 {
		case 0:
			m.move(-3)
		case 2:
			m.vote(memory.Up)
		case 3:
			m.vote(memory.Down)
		}
		if !slices.Contains(m.shown, m.mark) {
			t.Fatalf("after %d bits the caret is on %s, which the view does not hold",
				i+1, memory.Short(m.mark))
		}
	}
}

// A vote is a bit. It goes in the record, it does not go in the transcript, and
// nothing about the screen is stored anywhere.
func TestAVoteIsABitAndTheViewIsUnchanged(t *testing.T) {
	m := record(4)
	before, rows := m.store.Len(), slices.Clone(m.shown)

	m.vote(memory.Up)

	if got := m.store.Len(); got != before+1 {
		t.Errorf("record holds %d bits after one vote, want %d — a vote is one bit", got, before+1)
	}
	if !slices.Equal(m.shown, rows) {
		t.Error("the vote changed the transcript view")
	}
	if len(m.votes) != 1 {
		t.Errorf("the vote view holds %d bits, want 1", len(m.votes))
	}

	// And the standing vote is on the bit the caret was on, which is the whole of
	// what the key means.
	if got := memory.Tally(m.store, m.votes)[m.mark][localHandle]; got != 1 {
		t.Errorf("the tally says %d for the bit under the caret, want 1", got)
	}
}

// One upvote takes two rows off the trigger, not one. A hold spares the bit its
// own bit answers as well as itself, so a vote on an answer puts both it and the
// question above it out of every fold's reach — and the count of what a fold
// could take has to drop by both, or the screen is counting material no fold
// will ever get to.
//
// Read off the drawn footer, because the number a person acts on is the one on
// the screen and this is what their keystroke bought.
//
// Asserted as a difference of two rather than as a value, so nothing here has to
// restate how many bits are in the fixture. The precondition — that the bit
// above the caret is one nothing was already sparing — is checked rather than
// assumed, because a fixture that voted twice in a row would make the answer one
// and this test would then be pinning the defect.
func TestKeepingAnAnswerTakesItsQuestionOffTheTriggerToo(t *testing.T) {
	m := talk(sized(100, 30), 8)
	m.move(-1) // off the newest, so the bit voted on has a bit after it as well

	target := m.mark
	bits := m.shown.Bits(m.store)
	i := slices.IndexFunc(bits, func(b memory.Bit) bool { return b.ID == target })
	if i < 1 {
		t.Fatalf("the caret is at index %d, so there is no question above the answer it is on", i)
	}
	if spared := m.shown.Sparing(m.store, m.stay()); len(spared) != 0 {
		t.Fatalf("%d bits are already spared before anybody voted, so this measures nothing", len(spared))
	}

	before := gaugeReading(t, m)
	m = press(m, "ctrl+o")

	if _, held := m.stay().Holds(m.store, m.day())[target]; !held {
		t.Fatal("ctrl+o did not hold the bit under the caret, so nothing was measured")
	}
	if got := gaugeReading(t, m); got != before-2 {
		t.Errorf("the gauge read %d before one upvote and %d after; a hold spares the answer and the question, so it must fall by two",
			before, got)
	}

	// And the bit that came off is the one directly above, named by Prev — not
	// merely some second bit somewhere.
	if !m.shown.Sparing(m.store, m.stay())[bits[i-1].ID] {
		t.Errorf("the bit above the one kept, %s, is not spared", memory.Short(bits[i-1].ID))
	}
}

// gaugeReading is the numerator the footer is actually printing, parsed back off
// the drawn frame rather than read from the method behind it.
func gaugeReading(t *testing.T, m Model) int {
	t.Helper()

	foot := ansi.Strip(m.footer())
	want := fmt.Sprintf("/%d", m.budget())
	at := strings.Index(foot, want)
	if at < 0 {
		t.Fatalf("no gauge reading %q anywhere in the footer %q", want, foot)
	}
	start := at
	for start > 0 && foot[start-1] >= '0' && foot[start-1] <= '9' {
		start--
	}
	n, err := strconv.Atoi(foot[start:at])
	if err != nil {
		t.Fatalf("the footer %q has no number in front of %q", foot, want)
	}
	return n
}

// The trigger falls back under its budget after every fold it fires, at every
// vote rate — which is the whole reason [foldable] counts what a fold can take
// rather than what is merely hot.
//
// A trigger that is still over the line on the frame after a fold fires again on
// the very next write, and the fold after that has almost nothing left to take:
// a scar per bit, on a record whose receipts are supposed to stand for
// something. [memory.Stay.Holds] describes that failure in its own doc and this
// is the executable form of it.
//
// It is a property of every fold in a long conversation rather than of one
// arranged frame, because the state that breaks it is not reachable by hand: it
// needs enough holds accumulated that the bits they spare outnumber the slack
// between the budget and the keep, which takes a hundred bits or so to build up.
//
// Both counters are checked, so a version of this that never folded, or one
// where every rate behaved identically, reports that instead of passing.
func TestTheTriggerFallsBackUnderItsBudgetAfterEveryFold(t *testing.T) {
	folded := map[int]int{}
	for _, rate := range []int{4, 5, 7, 10} {
		m := sized(100, 30)
		for i := range 200 {
			was := len(m.shown)
			m.say(localHandle, fmt.Sprintf("bit %d", i))

			// A write that left the view no longer than it was is a fold: one bit
			// went on and at least two came off as one scar. Counting scars cannot
			// see it — a fold absorbs the previous scar too, so the count stays put.
			if len(m.shown) <= was {
				folded[rate]++
				if m.pressured() {
					t.Fatalf("one upvote in %d, bit %d: a fold has just fired and the trigger reads %d of %d, so the next thing said folds again",
						rate, i, m.foldable(), m.budget())
				}
			}
			if i%rate == 0 {
				m.vote(memory.Up)
			}
		}
	}

	for rate, n := range folded {
		if n == 0 {
			t.Fatalf("one upvote in %d folded nothing in 200 bits, so nothing at that rate was checked", rate)
		}
	}
	if len(folded) < 2 {
		t.Fatalf("only %d vote rates reached a fold: %v", len(folded), folded)
	}
}

// Keep holds a bit out of the fold that would otherwise have taken it, and the
// screen says so before the fold rather than after: a held bit is never in the
// set drawn cooling.
func TestKeepHoldsABitOutOfTheFold(t *testing.T) {
	m := record(fixtureBudget)
	m.move(-fixtureBudget + 2)
	kept := m.mark
	m.vote(memory.Up)

	if m.absorbing()[kept] {
		t.Error("a bit that is being held was drawn as material the next fold takes")
	}

	// Enough sends to force one. It takes more than one, and that is the hold
	// working rather than a fixture detail: a held bit is not foldable, so the
	// trigger — which counts what a fold could take — is one short until the
	// conversation makes up the difference.
	for range 3 {
		m.say(localHandle, "carry on")
	}

	if !slices.Contains(m.shown, kept) {
		t.Fatal("the fold took a bit that was being held")
	}
	if m.scars() == 0 {
		t.Fatal("no fold happened at all, so the hold was not tested against one")
	}
}

// Letting go withdraws the hold, on the same frame, and does it through the
// record rather than by deleting anything: both votes stay, and the later one is
// what counts.
func TestLettingGoWithdrawsTheHold(t *testing.T) {
	m := record(fixtureBudget)
	m.move(-fixtureBudget + 2)
	target := m.mark

	m.vote(memory.Up)
	if _, held := m.stay().Holds(m.store, m.day())[target]; !held {
		t.Fatal("keep did not hold the bit, so there is nothing to withdraw")
	}
	after := m.store.Len()

	m.vote(memory.Down)
	if _, held := m.stay().Holds(m.store, m.day())[target]; held {
		t.Error("the bit is still held after letting go of it")
	}
	if got := m.store.Len(); got != after+1 {
		t.Errorf("record holds %d bits, want %d — changing your mind adds a vote rather than removing one",
			got, after+1)
	}
	if len(m.votes) != 2 {
		t.Errorf("the vote view holds %d votes, want both of them", len(m.votes))
	}
}

// Holding every third bit is a thing a person will do, and it stops the fold
// dead: no run of two unspared bits survives anywhere, so [memory.View.Fold]
// refuses and the view grows past the limit. The screen has to say that rather
// than show a full gauge and no fold — and letting one go has to visibly undo it.
//
// # Third and not second, and the difference is a whole other state
//
// This fixture held every *other* bit until a hold began sparing the bit its
// own bit answers. At that density every row in the view is either held or
// covered, so [Model.foldable] is zero, [Model.pressured] is false, and the
// screen this test is about — a full gauge with the word `held` on it — is not
// the screen a person gets. What they get is a gauge reading nothing against a
// view that keeps growing, which is honest (there is genuinely no material a
// fold could take) and is a different thing from being blocked. It is in
// `docs/DEBT.md` under its own name.
//
// Every third bit is the densest voting that still reaches this one: held,
// covered, free, held, covered, free — every free stretch is one bit, D32
// refuses all of them, and the free bits still outnumber the budget. The whole
// arrangement is one bit of hold density wide, which is why the fixture states
// its own preconditions below rather than trusting the arithmetic.
//
// Visibly is the whole of the second half, and it is where this test changed.
// Letting go used to fold on the keystroke, so the rows collapsed under the hand
// that released them: measured on this fixture, three bits went, all of them at
// full brightness on the frame the key was pressed, two of them bits nobody had
// ever voted on. Nothing on screen was fading at that moment, because a blocked
// view has nothing to fade — so the fold arrived with no antecedent at all, on
// the one surface whose argument is that it never does that.
//
// So the sequence is now three frames and not two, and it is better to watch:
// held so hard it cannot fold, then the freed rows fading on the frame the key
// was pressed, then the next thing anybody says taking exactly those rows.
func TestHoldingEveryThirdBitBlocksTheFoldAndLettingGoReleasesIt(t *testing.T) {
	m := New()
	for i := range fixtureBudget*4 + 2 {
		m.say(localHandle, fmt.Sprintf("bit %d", i))
		if i%3 == 2 {
			m.vote(memory.Up)
		}
	}

	if got := m.foldable(); got <= fixtureBudget {
		t.Fatalf("foldable is %d of %d, so the view never reached the limit", got, fixtureBudget)
	}
	if !m.blocked() {
		t.Fatal("the view is past its limit with nothing a fold can take, and blocked says otherwise")
	}
	if m.scars() != 0 {
		t.Fatalf("%d folds happened while every third bit was held", m.scars())
	}
	if n := len(m.absorbing()); n != 0 {
		t.Fatalf("%d rows are drawn cooling on a view that cannot fold", n)
	}

	// Now let one go, from a row well up the view. Nothing may leave the screen on
	// this keystroke.
	//
	// The caret has to land on a bit somebody actually voted on, or the keystroke
	// below is a downvote on an unheld row and withdraws nothing. That was true of
	// this offset by arithmetic nobody stated, and it survived the density change
	// by luck — so it is asserted here rather than counted out again.
	rows := slices.Clone(m.shown)
	m.move(-10)
	if _, held := m.stay().Holds(m.store, m.day())[m.mark]; !held {
		t.Fatal("the caret is not on a held bit, so letting go here withdraws nothing")
	}
	m.vote(memory.Down)

	if gone := took(m, rows); len(gone) > 0 {
		t.Errorf("letting go folded %d bits on the keystroke, and nothing on screen had been drawn cooling first",
			len(gone))
	}
	if !slices.Equal(m.shown, rows) {
		t.Errorf("the view is %d rows after letting go, was %d — the vote changed the transcript",
			len(m.shown), len(rows))
	}
	if m.blocked() {
		t.Error("still blocked after a hold was withdrawn")
	}

	// What happens instead, on that same frame: the rows the withdrawn hold was
	// keeping bright start fading. That is the antecedent, and it is one keystroke
	// early rather than absent.
	freed := m.absorbing()
	if len(freed) == 0 {
		t.Fatal("letting go freed the fold and nothing on screen is fading, so the fold that follows has no warning in front of it")
	}

	// And the next thing said takes them, and only them.
	before := slices.Clone(m.shown)
	m.say(localHandle, "carry on")

	gone := took(m, before)
	if len(gone) == 0 {
		t.Fatal("the fold that letting go freed never happened")
	}
	for id := range gone {
		if !freed[id] {
			t.Errorf("the fold took %s, which the frame before it drew hot", memory.Short(id))
		}
	}
	// And the screen is shorter than the write alone would have left it, which is
	// the assertion rather than the view being shorter than it was before the
	// keystroke. Withdrawing one hold here frees exactly two rows — the bit voted
	// on and the bit it answers, which memory now covers with it — and two rows
	// cooled come back as one scar, so the fold nets one row against a write that
	// added one. Compared against the view before the keystroke that reads as
	// nothing having happened, and this comparison is what tells the two apart.
	if len(m.shown) >= len(before)+1 {
		t.Errorf("the view is %d rows after a write and a fold, and the write alone would have left %d",
			len(m.shown), len(before)+1)
	}
}

// No vote reaches the persona, ever. Asserted as byte equality of everything the
// model would be sent, before and after voting on half the conversation: the
// vote changes what survives consolidation and never tells the thing being
// judged that it was judged.
//
// The reason it matters is not tidiness. A model that can see which of its
// answers were kept will write the next one to be kept, and the human's
// consolidation signal quietly becomes a behavioural one — a sycophancy pump
// wired into the only signal this product has.
//
// The second half exists because byte equality could not see the other door.
// [standingInstruction] is sent ahead of every request and does not change when
// anybody votes, so a sentence in it saying the person keeps some answers and
// lets others go would pass the check above untouched — and would produce the
// same behaviour without a single verdict crossing. A model does not need the
// score to write toward the score; it needs only to know one is being kept.
func TestNoVoteReachesThePersona(t *testing.T) {
	m := record(6)
	before := m.turns()

	for range 3 {
		m.vote(memory.Up)
		m.move(-1)
		m.vote(memory.Down)
		m.move(-1)
	}
	if len(m.votes) == 0 {
		t.Fatal("no votes were cast")
	}

	after := m.turns()
	if len(before) != len(after) {
		t.Fatalf("the persona is sent %d turns after voting, was %d", len(after), len(before))
	}
	for i := range after {
		if before[i] != after[i] {
			t.Errorf("turn %d changed when a vote was cast:\n before %q\n  after %q",
				i, before[i].Content, after[i].Content)
		}
	}

	// The vote's own lexicon, and nothing wider. These are words that could only
	// be telling a participant that its output is being judged; "keep" and "hold"
	// are deliberately absent, because both are ordinary English the notes
	// already use about the record rather than about anybody's opinion of it.
	judged := func(s string) string {
		for _, w := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsNumber(r)
		}) {
			for _, bad := range []string{"vote", "upvot", "downvot", "rank", "score"} {
				if strings.HasPrefix(w, bad) {
					return w
				}
			}
		}
		return ""
	}

	// Read off the constructed persona rather than the constant, so that a
	// defaultPersona() that stopped using it is not silently unchecked.
	if w := judged(New().persona.System); w != "" {
		t.Errorf("the standing instruction says %q — the persona is told it is being judged, which is the sycophancy pump arriving by the front door", w)
	}
	for i, turn := range after {
		if w := judged(turn.Content); w != "" {
			t.Errorf("turn %d says %q:\n%s", i, w, turn.Content)
		}
	}

	// And the third door, which opened the day the scar started quoting a bit
	// the reader's votes chose.
	//
	// A fold reaches the model as a note ([foldNote]), and the scar reaches the
	// person as a quotation of one absorbed bit — the same object, two accounts,
	// and the second one is now a function of the vote. Making them agree, which
	// is what the retired scarWords/personaWords pairing asked for, would hand
	// the model a message *selected by the human's approval*: not the verdict,
	// but the thing the verdict picked out, which is all a model needs to write
	// toward it.
	//
	// Byte equality of everything the model would be sent, over a record that has
	// actually folded, with an upvote cast on a bit that is inside the fold. The
	// arm above cannot see this — its fixture is six bits and has no scar in it,
	// so the fold note it compares is one that does not exist.
	folded := back(talk(sized(100, 30), 10), 6)
	folded = heldSince(folded, holdFor+time.Second)
	inside, ok := folded.store.Get(folded.mark)
	if !ok {
		t.Fatal("the caret is on nothing, so nothing was kept")
	}
	folded = talk(folded, 22)

	var scars int
	for _, b := range folded.frame().bits {
		if c, cold := b.Payload.(memory.Compaction); cold {
			if slices.Contains(slices.Collect(c.Absorbed()), inside.ID) {
				scars++
			}
		}
	}
	if scars == 0 {
		t.Fatal("no scar in the view absorbed the kept bit, so the fold note under test carries nothing the vote could reach")
	}

	// The same conversation with no ballot in it at all. Anything the vote
	// reaches shows up as a difference between the two.
	plain := talk(back(talk(sized(100, 30), 10), 6), 22)
	quiet, loud := plain.turns(), folded.turns()
	if len(quiet) != len(loud) {
		t.Fatalf("a folded record sends %d turns with a vote in it and %d without", len(loud), len(quiet))
	}
	for i := range loud {
		if quiet[i] != loud[i] {
			t.Errorf("turn %d differs between a folded record with a vote in it and one without:\n without %q\n    with %q",
				i, quiet[i].Content, loud[i].Content)
		}
	}
}

// A hold is measured in the conversation's own time, so it decays as the
// conversation moves and not as the machine's clock does. This is holdFor's
// schedule seen from the surface: a vote cast far enough back is no longer
// holding anything, and the vote itself is untouched by that.
func TestAHoldDecaysAgainstTheConversation(t *testing.T) {
	m := record(4)
	target, ok := m.store.Get(m.mark)
	if !ok {
		t.Fatal("the caret is on nothing")
	}

	// Cast as though it had been pressed a whole holdFor ago, which is exactly
	// what Model.vote does with time.Now() in its place.
	m.votes, _ = m.votes.Add(m.store,
		memory.Cast(m.day().Add(-holdFor-time.Second), localHandle, memory.Up, target))

	if _, held := m.stay().Holds(m.store, m.day())[target.ID]; held {
		t.Error("a vote older than holdFor is still holding its bit")
	}
	if got := memory.Tally(m.store, m.votes)[target.ID][localHandle]; got != 1 {
		t.Errorf("the standing vote is %d, want 1 — what expires is the hold, never the vote", got)
	}
}

// Moving the caret never leaves it off screen. The key that votes acts on
// wherever it is, so a caret above the top of the frame is a key aimed at
// something the person cannot see — which is the same defect as an operation
// happening behind their back, with the person holding the trigger.
//
// Stated exactly, because paging is the case it does not cover: pgup and pgdn
// scroll without moving the caret, so they can leave it off screen. What catches
// that is the same reveal on the other side — voting scrolls back to the caret
// before the frame is drawn, so the frame that records the vote is a frame
// showing the row it landed on.
func TestTheCaretStaysOnScreen(t *testing.T) {
	m := New()
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 14})
	m = mm.(Model)

	for i := range fixtureBudget * 2 {
		m.say(localHandle, fmt.Sprintf("bit %d", i))
	}

	// Up from the newest to the oldest, one bit at a time, checking every stop.
	for range len(m.shown) + 2 {
		m.move(-1)

		top, h := m.viewport.YOffset(), m.viewport.Height()
		if m.anchors.mark < top || m.anchors.mark >= top+h {
			t.Fatalf("the caret is on row %d with the frame showing %d..%d",
				m.anchors.mark, top, top+h-1)
		}
	}

	// And back down again, because a scroll that only ever went one way would
	// pass the loop above from the moment it reached the top.
	for range len(m.shown) + 2 {
		m.move(1)

		top, h := m.viewport.YOffset(), m.viewport.Height()
		if m.anchors.mark < top || m.anchors.mark >= top+h {
			t.Fatalf("on the way back the caret is on row %d with the frame showing %d..%d",
				m.anchors.mark, top, top+h-1)
		}
	}
}

// A loaded session picks up where the last one stopped: the same record, the
// same two views, and the caret on the newest bit rather than on nothing.
//
// The caret is the assertion worth making. Everything else here is a field
// copied into a struct; where the caret lands is a decision, and the wrong
// landing is silent — the first vote key a returning reader presses acts on a
// row they did not choose.
func TestLoadPutsTheCaretWhereTheSessionLeftIt(t *testing.T) {
	live := record(fixtureBudget)
	live.vote(memory.Up)
	shown, votes := live.Views()

	back := Load(live.store, shown, votes, nil)

	if !slices.Equal(back.shown, shown) || !slices.Equal(back.votes, votes) {
		t.Fatalf("loaded views %v / %v, want %v / %v", back.shown, back.votes, shown, votes)
	}
	if back.mark != live.mark {
		t.Errorf("the caret loaded onto %s, want %s", memory.Short(back.mark), memory.Short(live.mark))
	}
	if !back.riding() {
		t.Error("the caret did not load onto the newest bit, so the next thing said will not carry it")
	}

	// It also has to be a session and not a snapshot: saying something has to
	// extend the view that was loaded rather than start beside it.
	back.say(localHandle, "and one more thing")
	if len(back.shown) != len(shown)+1 {
		t.Errorf("speaking into a loaded session left %d rows, want %d", len(back.shown), len(shown)+1)
	}
	if back.mark != back.shown[len(back.shown)-1] {
		t.Error("the caret did not follow what was just said into a loaded session")
	}
}

// An empty record loads as the session New builds, which is what lets New be
// Load over an empty store rather than a second constructor.
func TestLoadOfNothingIsWhatNewBuilds(t *testing.T) {
	fresh, loaded := New(), Load(memory.NewStore(), nil, nil, nil)

	if fresh.mark != "" || loaded.mark != "" {
		t.Errorf("caret at %q fresh and %q loaded, want neither on anything",
			memory.Short(fresh.mark), memory.Short(loaded.mark))
	}
	if len(loaded.shown) != 0 || len(loaded.votes) != 0 {
		t.Errorf("loaded views %v / %v, want both empty", loaded.shown, loaded.votes)
	}
	if fresh.store.Len() != 0 || loaded.store.Len() != 0 {
		t.Errorf("records of %d and %d bits, want both empty", fresh.store.Len(), loaded.store.Len())
	}
}

// The same record shows the same clock whoever opens it, and whenever.
//
// A live bit carries `time.Local` from `time.Now`; a bit read back off the file
// carries UTC, because the wire format normalizes the instant on purpose (D12,
// and [memory.Bit].At says so in its own doc). [clock] reads every row in
// [Model.day]'s location, so before this was decided the whole screen adopted
// whatever zone the newest bit happened to arrive with — and the same record read
// 19:47 in the session that wrote it and 01:47 the next day in the session that
// reopened it, on a machine six hours behind UTC. The header's *date* moves with
// it, so it is wrong by a day and not only by hours.
//
// Driven the way it actually happens: bits stamped in a zone that is not the
// reader's, which is what a reload produces, against the same bits stamped local.
// The two frames must be identical. A zone six hours off is chosen so the failure
// crosses midnight, because a smaller offset passes on most inputs and would make
// this a check that fails only in the afternoon.
func TestTheClockReadsTheSameWhoeverOpensTheRecord(t *testing.T) {
	at := time.Date(2026, 8, 14, 19, 47, 0, 0, time.Local)

	frames := map[string]string{}
	for _, zone := range []*time.Location{time.Local, time.UTC, time.FixedZone("far", 11*3600)} {
		m := sized(100, 30)
		for i := range 30 {
			from := localHandle
			if i%2 == 1 {
				from = m.persona.Handle()
			}
			// Added straight to the view, so the instant under test survives —
			// [Model.say] would stamp time.Now. Restating it in another zone is
			// exactly what ReadStore hands back.
			m.shown, _ = m.shown.Add(m.store, memory.Bit{
				At:      at.Add(time.Duration(i) * time.Minute).In(zone),
				From:    from,
				Channel: channel,
				Payload: memory.Utterance{Text: lines[i%len(lines)]},
				Prev:    m.shown.Head(),
			})
		}
		m.mark = m.shown[len(m.shown)-1]

		// Folded and opened, because **a transcript row carries no clock at all**
		// — only the header's date and a receipt's rows do, which is an older
		// disclosure in docs/DEBT.md and is the reason the first version of this
		// check asserted against a frame that could never have contained a time.
		// It passed until a mutation made it fail, which is the wrong order.
		m.fold()
		m.unfold()
		if !m.unfolded {
			t.Fatal("nothing folded, so no receipt opened and no row on this frame carries a clock")
		}
		frames[zone.String()] = ansi.Strip(m.View().Content)
	}

	want := frames[time.Local.String()]
	for zone, got := range frames {
		if got != want {
			t.Errorf("a record whose instants carry %s draws a different frame from the same record in the reader's own zone;\n got %q\nwant %q",
				zone, firstDiff(got, want), firstDiff(want, got))
		}
	}

	// Consistent is not enough on its own: forcing every frame to UTC is also
	// consistent, and it is consistently the wrong clock under a header that says
	// `local`. So the drawn time has to be the reader's reading of the instant.
	if hh := at.Format("15:04"); !strings.Contains(want, hh) {
		t.Errorf("a bit at %s draws no %q on the opened receipt, so the clock is consistent and not local",
			at, hh)
	}
	if day := at.Format("2006-01-02"); !strings.Contains(want, day) {
		t.Errorf("a record whose newest bit is %s carries no %q in its header — the date is the half of this that can be wrong by a whole day",
			at, day)
	}

	// And the screen says whose clock it is, or a bare 15:04 means a different
	// moment on every machine that opens the record.
	if !strings.Contains(want, "local") {
		t.Errorf("nothing on the frame says which zone its clocks are in: %q", strings.SplitN(want, "\n", 2)[0])
	}
}

// firstDiff is the line where two frames stop agreeing, so a failure names a row
// rather than printing two screens.
func firstDiff(a, b string) string {
	as, bs := strings.Split(a, "\n"), strings.Split(b, "\n")
	for i := range as {
		if i >= len(bs) || as[i] != bs[i] {
			return as[i]
		}
	}
	return ""
}

// The fade names what the next fold takes, swept over where the human's turn
// falls — which is the variable the other fade tests hold still.
//
// [Model.keep] searches back for a bit the human said, so the *position* of that
// bit decides how far the search runs, and the lookahead in [Model.absorbing]
// has to run the same search one lower at both ends. Getting only the base one
// lower is an error that needs three things at once to show: a view exactly two
// past its budget, which is the ordinary state once a scar is in it; the human's
// only turn in reach sitting at exactly the budget's depth; and agents either
// side of it. No fixture that alternates speakers, and no fixture that fixes the
// human at one end, ever produces that — measured exhaustively over speaker
// patterns, the two ceilings disagree on 1,716 views of 4.5 million, and every
// one of them has that shape.
//
// So this sweeps the position rather than the length, and it is the sole reason
// the ceiling in [Model.absorbing] is written the way it is.
func TestTheFadeNamesTheFoldWhereverTheHumanSpoke(t *testing.T) {
	bot := memory.Handle{Ref: "ollama/qwen3.5:latest", Display: "qwen3.5"}

	compared, deep := 0, 0
	for _, h := range []int{20, 24, 30} {
		for at := range 40 {
			// Votes, because the view is only ever one past its budget while it
			// holds one scar, and one scar is all a fold leaves. A hold splits the
			// fold and leaves two, which is what carries the view to budget+2 —
			// so without a vote in here the guard at the end fires and says the
			// deep case was never entered. Every fifth bit kept, offset by the
			// arm, so the holds do not line up with the human's turn.
			m := sized(100, h)
			for i := range 40 {
				who := bot
				if i == at {
					who = localHandle
				}

				drawn := m.absorbing()
				before := slices.Clone(m.shown)
				over := len(before) > m.budget()+1

				m.say(who, lines[i%len(lines)])
				if (i+at)%5 == 0 {
					m.vote(memory.Up)
				}

				got := took(m, before)
				if len(got) == 0 {
					continue
				}
				compared++
				if over {
					deep++
				}
				if !maps.Equal(drawn, got) {
					t.Fatalf("100x%d, the human's turn at %d, after %d bits: the frame before the fold drew %d cooling and the fold took %d",
						h, at, i+1, len(drawn), len(got))
				}
			}
		}
	}
	if compared == 0 {
		t.Fatal("no fold fired anywhere in the sweep, so nothing here was compared")
	}
	if deep == 0 {
		t.Fatalf("%d folds compared and none from a view more than one past its budget, which is the only place the two ceilings differ", compared)
	}
}

// covering builds a frame with all five things a row can be on it at once: a
// scar, rows the next fold will take, a row merely staying, a held row, and the
// row that hold is covering.
//
// It drives the real surface until the state arrives rather than arranging one,
// because every part of it — which bits are held, where the cut lands, which
// runs the size rule refuses — is the program's own arithmetic and a hand-built
// view would be a picture of a program nobody is running.
func covering(t *testing.T, w, h int) Model {
	t.Helper()

	m := sized(w, h)
	bot := memory.Handle{Ref: "ollama/llama3", Display: "coordinator-7"}
	for i := range 400 {
		who := localHandle
		if i%2 == 1 {
			who = bot
		}
		m.say(who, lines[i%len(lines)])
		if i%9 == 4 {
			m.vote(memory.Up)
		}
		if m.scars() > 0 && len(m.absorbing()) > 0 && len(m.covered(m.live())) > 0 {
			return m
		}
	}
	t.Fatalf("%dx%d: no frame in 400 bits carried a scar, a fade and a covered row at once", w, h)
	return m
}

// A covered row — one a hold is sparing because the row below it was kept — is
// drawn differently from a row that is merely staying, and does not carry a
// ballot.
//
// Both halves are the claim. Before this, an upvote on an answer made the
// question above it stop fading with nothing on screen saying why: one keystroke,
// two rows changed, and only one of them said anything about it. And the mark it
// says it with cannot be a vote glyph, because nobody voted on that row — the
// fold rule underneath ([memory.View.Sparing]) goes out of its way not to report
// a ballot that was never cast, and a screen that drew one would give the whole
// thing back.
//
// Read off the drawn frame with the escapes stripped, so the difference is one a
// terminal with no colour still has.
func TestACoveredRowIsDrawnAsKeptAndNotAsVotedFor(t *testing.T) {
	m := covering(t, 100, 30)
	m.move(-3) // off the newest, whose row is the one row drawn whole

	// The lead is everything a row draws in front of the handle: the margin the
	// fade steps in, and the vote column. Cut at the handle rather than at a
	// column count, because a row the fold is taking begins two columns further
	// left and the point here is to compare what is *in* the lead.
	bits := m.shown.Bits(m.store)
	drawn := strings.Split(shot(m, 100, false), "\n")
	if len(drawn) != len(bits) {
		t.Fatalf("%d rows drawn for %d bits, so a row cannot be found by its index", len(drawn), len(bits))
	}
	lead := func(row, who string) string { return row[:strings.Index(row, who)] }

	holds, plain, covered := "", "", ""
	for i, b := range bits {
		if _, cold := b.Payload.(memory.Compaction); cold {
			continue
		}
		row := lead(ansi.Strip(drawn[i]), b.From.Display)
		switch {
		case m.covered(m.live())[b.ID]:
			covered = row
		case m.live()[b.ID] != 0:
			holds = row
		case !m.absorbing()[b.ID] && b.ID != m.caret():
			plain = row
		}
	}
	for what, row := range map[string]string{"held": holds, "plain": plain, "covered": covered} {
		if row == "" {
			t.Fatalf("no %s row on the frame, so there is nothing to compare", what)
		}
	}

	if covered == plain {
		t.Errorf("a covered row and a row that is merely staying begin identically (%q), so the vote that spared it says nothing on the row it spared", plain)
	}
	if covered == holds {
		t.Errorf("a covered row and a held row begin identically (%q), so the screen cannot say which of them anybody voted on", holds)
	}
	for _, ballot := range []string{"▲", "▼", "△"} {
		if strings.Contains(covered, ballot) {
			t.Errorf("a covered row draws %q in the vote column, which reports a ballot nobody cast: %q", ballot, covered)
		}
	}
}

// The tie hangs from the mark it belongs to: it is in the same column as the ▲
// on the row directly beneath it, and that row is the one being held.
//
// This is the whole of why the shape reads without a legend, and it is a fact
// about the record rather than about the renderer — a hold covers the bit its held
// bit names through Prev, Prev is the head of the view when that bit was written, and
// a fold only ever replaces runs *between* bits, so nothing can get in between
// them afterwards. Asserted on every covered row of a real frame at three sizes,
// because the arrangement that would break it (a covered row whose holder is
// somewhere else entirely) is exactly the one a renderer cannot see.
//
// It says the two rows are adjacent and deliberately nothing about why. Prev is a
// position; on this fixture, which alternates, it is also the turn being replied
// to, and a bit written through `tldr say` need not be. See [Model.covered].
//
// Every row of this fixture is one line, which is what lets a bit be found by its
// index — and it is also what makes this test blind to the caret's own block.
// [TestATieReachesTheMarkItPointsAt] is the case where they differ.
//
// It finds the tie by looking for whatever the covered row draws in front of its
// handle rather than by looking for the glyph, so that this test is about the
// column and [TestACoveredRowIsDrawnAsKeptAndNotAsVotedFor] is about the mark.
// Two checks that both grep for one constant are one check.
func TestTheTieHangsFromTheMarkOnTheRowBelowIt(t *testing.T) {
	for _, sz := range [][2]int{{100, 30}, {60, 14}, {200, 80}} {
		m := covering(t, sz[0], sz[1])
		bits := m.shown.Bits(m.store)
		drawn := strings.Split(shot(m, sz[0], false), "\n")
		covered, holds := m.covered(m.live()), m.live()

		ties := 0
		for i, b := range bits {
			if !covered[b.ID] {
				continue
			}
			ties++
			if i+1 >= len(bits) {
				t.Fatalf("%dx%d: the last row of the view is covered, so nothing is below it to be holding it", sz[0], sz[1])
			}
			if holds[bits[i+1].ID] == 0 {
				t.Errorf("%dx%d: %s is covered and the row under it is not held",
					sz[0], sz[1], memory.Short(b.ID))
			}
			at, below := marked(ansi.Strip(drawn[i])), marked(ansi.Strip(drawn[i+1]))
			if at < 0 {
				t.Errorf("%dx%d: the covered row draws nothing in front of its handle: %q", sz[0], sz[1], ansi.Strip(drawn[i]))
				continue
			}
			if at != below {
				t.Errorf("%dx%d: the covered row's mark is at column %d and the mark it hangs from is at column %d\n  %s\n  %s",
					sz[0], sz[1], at, below, ansi.Strip(drawn[i]), ansi.Strip(drawn[i+1]))
			}
		}
		if ties == 0 {
			t.Fatalf("%dx%d: no covered row on the frame, so nothing was checked", sz[0], sz[1])
		}
	}
}

// A tie reaches the mark it points at, on the one row where a bit is more than a
// line: the caret's.
//
// [voteCell] promises half a stroke hanging into the ▲ on the row below. The
// caret's row is drawn whole, so a covered row the caret is on is a *block* — and
// the stroke used to be drawn on its first line only, which on this project's own
// record at 100x30 left it five lines above the mark it points at, with a
// paragraph in between. The measurement that says this is ordinary rather than
// exotic: over a fixture of model-length replies, every covered row opens that way
// under the caret, against none at all on a fixture of one-line bits — which is
// why every other test on this glyph is blind to it.
//
// Asserted as a column and an adjacency rather than as a count of glyphs: what has
// to be true is that the line immediately above the mark carries the tie, in the
// mark's own column. How many lines the block took is a fact about the message.
func TestATieReachesTheMarkItPointsAt(t *testing.T) {
	bot := memory.Handle{Ref: "ollama/llama3", Display: "coordinator-7"}
	long := strings.Repeat("this is the sort of length a model actually replies at. ", 6)

	m := sized(100, 30)
	m.say(localHandle, "what did we settle on")
	m.say(bot, long)
	m.say(localHandle, "that is the one")
	m = heldSince(m, 0) // the newest bit, so the long reply above it is covered

	m.move(-1)
	if !m.covered(m.live())[m.caret()] {
		t.Fatalf("the caret is on %s and it is not covered, so nothing is being tested", memory.Short(m.caret()))
	}
	if m.anchors.rows < 2 {
		t.Fatalf("the caret's block is %d line(s); this fixture is not reaching the case", m.anchors.rows)
	}

	rows := strings.Split(ansi.Strip(shot(m, 100, false)), "\n")
	at := -1
	for i, row := range rows {
		if strings.Contains(row, "▲") {
			at = i
		}
	}
	if at <= 0 {
		t.Fatalf("no held row on the frame:\n%s", strings.Join(rows, "\n"))
	}

	above := rows[at-1]
	if !strings.Contains(above, tie) {
		t.Errorf("the line above the mark carries no tie, so the stroke stops %d lines short of what it points at:\n%s",
			m.anchors.rows-1, strings.Join(rows, "\n"))
	}
	if strings.Index(above, tie) != strings.Index(rows[at], "▲") {
		t.Errorf("the tie is at column %d and the mark it hangs from is at column %d\n  %s\n  %s",
			strings.Index(above, tie), strings.Index(rows[at], "▲"), above, rows[at])
	}
}

// marked is the column a row draws its vote mark in, counted in cells, or -1
// where it draws none.
//
// Two things are not marks and are skipped. The caret is the margin's rather than
// the vote column's, and it is on exactly one row. And an ellipsis is [clip]
// saying the terminal cut the row — at a handful of columns every row on this
// surface is spaces and then that, and reading it as a mark would have this
// answer "yes there is one" on a frame with nothing on it.
func marked(row string) int {
	for i, r := range []rune(row) {
		if r == ' ' || string(r) == caret {
			continue
		}
		if r == '…' {
			return -1
		}
		return i
	}
	return -1
}

// The tie lives exactly as long as the mark it hangs from, at every width, which
// is the reason it costs nothing: the column it is in exists only because
// somebody voted, and a covered row only exists because somebody voted.
//
// So it can never be the thing a narrow terminal drops, and there is no ladder
// rung anywhere that has to decide about it. Swept down to a width of one, and
// what is asserted is the equality rather than a floor — a floor here would be a
// number to go stale, and the claim is that there is no width where one is drawn
// without the other.
//
// Asked of the covered row's own cell rather than of the whole frame, for
// [TestTheTieHangsFromTheMarkOnTheRowBelowIt]'s reason: a sweep that greps the
// frame for one glyph is answering a question about that glyph and not about the
// column.
func TestTheTieIsDrawnAtEveryWidthTheMarkIs(t *testing.T) {
	m := covering(t, 100, 30)

	bits := m.shown.Bits(m.store)
	covered, holds := m.covered(m.live()), m.live()
	// -1 rather than 0, because index 0 is a legal answer to both questions and
	// a zero sentinel would report a held row at the top of the view as a
	// fixture with no held row at all. It is a scar there today, so neither can
	// land on 0 — which is exactly the kind of reason that stops being true
	// quietly, and the same sentinel `marked` and `anchors` already use here.
	up, cov := -1, -1
	for i, b := range bits {
		if covered[b.ID] && cov == -1 {
			cov = i
		}
		if holds[b.ID] != 0 && up == -1 {
			up = i
		}
	}
	if cov == -1 || up == -1 {
		t.Fatalf("the fixture has a covered row at %d and a held row at %d", cov, up)
	}

	both, neither := 0, 0
	for w := 1; w <= 120; w++ {
		f := m.frame()
		f.width = w
		body, at := transcript(f)
		drawn := strings.Split(body, "\n")

		// Both rows are above the caret's, which is the one row here that is not
		// one line — so their index in the view is their line in the frame, and it
		// stays that way at every width rather than only at the wide ones.
		if at.mark >= 0 && (cov >= at.mark || up >= at.mark) {
			t.Fatalf("width %d: the caret is on line %d, at or above the rows this measures (%d, %d)", w, at.mark, cov, up)
		}
		mark := strings.Contains(ansi.Strip(drawn[up]), "▲")
		hang := marked(ansi.Strip(drawn[cov])) >= 0
		if mark != hang {
			t.Fatalf("at width %d the held row draws a mark=%v and the covered row draws one=%v\n  %s\n  %s",
				w, mark, hang, ansi.Strip(drawn[cov]), ansi.Strip(drawn[up]))
		}
		if mark {
			both++
		} else {
			neither++
		}
	}
	if both == 0 || neither == 0 {
		t.Fatalf("the sweep never crossed the width where the column gives way: %d wide enough, %d not", both, neither)
	}
}

// No scar is ever covered, which is what lets [voteCell] treat the tie and the
// scar's rule as two things that cannot meet.
//
// It is an unreachability rather than a rule, and it is written as a check for
// D58(h)'s reason: a guard defended by an argument nobody can run is a prior, and
// a prior is what this project keeps finding out was wrong. The argument, so a
// reader knows what changing would break it: [Model.utter] takes Prev from the
// view's last entry, a fold always leaves a kept tail behind the scar it puts at
// the front, and [Model.keep] never returns less than half a budget — so no bit
// is ever written while a scar is the newest thing in the view.
func TestNoScarIsEverCovered(t *testing.T) {
	seen := 0
	for _, rate := range []int{2, 3, 5, 9} {
		m := sized(100, 30)
		bot := memory.Handle{Ref: "ollama/llama3", Display: "coordinator-7"}
		for i := range 200 {
			who := localHandle
			if i%2 == 1 {
				who = bot
			}
			m.say(who, lines[i%len(lines)])
			if i%rate == 0 {
				m.vote(memory.Up)
			}
			covered := m.covered(m.live())
			for _, b := range m.shown.Bits(m.store) {
				if _, cold := b.Payload.(memory.Compaction); !cold || !covered[b.ID] {
					continue
				}
				t.Fatalf("one upvote in %d, bit %d: the scar %s is covered, so a tie would be drawn inside its own rule",
					rate, i, memory.Short(b.ID))
			}
			seen += len(covered)
		}
	}
	if seen == 0 {
		t.Fatal("no covered row anywhere in the sweep, so the absence of a covered scar means nothing")
	}
}

// The ranked surface draws no tie, and that is a decision rather than an
// oversight.
//
// The tie is positional: it says "the row below this one is why this is being
// kept", and it is honest on the transcript because the holder is always the next
// row down. The ranked list is ordered by what the human said about each bit, so
// the row below a covered one there is whatever the ranking put there — the tie
// would point at a stranger. A mark that meant the same thing without pointing
// would be a fourth glyph to learn on the one screen that already states no
// ordering of its own.
//
// # The guard is live now, and it used to be the thing that could not fire
//
// [Model.frame] leaves the covered set empty on that surface. While the ranked
// list was the voted bits alone that guard was unreachable — a covered bit is by
// definition one nobody voted on — so the real assertion was that the two sets did
// not intersect, and the guard was a backstop against the widening this test's own
// doc named in advance. [Model.judged] widened. The sets intersect, and the first
// thing asserted below is that they do, because a version of this check whose
// intersection is empty is a check that cannot fail.
func TestTheRankedSurfaceDrawsNoTie(t *testing.T) {
	m := covering(t, 100, 30)
	if !strings.Contains(ansi.Strip(shot(m, 100, false)), tie) {
		t.Fatal("the transcript is drawing no tie, so this proves nothing about the other surface")
	}

	m = press(m, "ctrl+t")
	if !m.ranked {
		t.Fatal("ctrl+t did not reach the ranked surface")
	}

	covered := m.covered(m.live())
	if len(covered) == 0 {
		t.Fatal("nothing is covered, so there is nothing this surface could wrongly draw")
	}
	listed, both := 0, 0
	for _, r := range m.ranking() {
		listed++
		if covered[r.ID] {
			both++
		}
	}
	if listed == 0 {
		t.Fatal("the ranked list is empty, so nothing was compared")
	}
	if both == 0 {
		t.Fatal("no covered bit is on the ranked list, so the guard below has nothing to guard and this check cannot fail")
	}

	body, _ := ranked(m.frame())
	if strings.Contains(ansi.Strip(body), tie) {
		t.Errorf("the ranked surface draws a tie for one of %d covered rows it lists, where the row under a covered one is not the row holding it",
			both)
	}
}
