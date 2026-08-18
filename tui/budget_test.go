package tui

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/tyler-j-chrestoff/tldreddit/memory"
)

// A bit that draws forty rows costs the budget more than a bit that draws one.
//
// That sentence is the whole of what changed here and it was false for the life
// of the project: [Model.budget] returns the screen's rows (D58(b)) and the load
// against it was a count of bits, which are the same number only while every bit
// draws one row. A message written as a document does not — at 100x30 a fenced Go
// answer is 36 rows against a viewport of 23 — so five bits could fill three
// screens while the gauge read a fifth full and no fold fired. Measured on this
// project's own fixture at 100x30: the first fold used to arrive after 24 writes
// and arrives after 14.
//
// **Corroborating rather than sole, and named as such** — every mutation it
// catches is caught by one of the three below it, because it is the direct
// statement of a claim they each hold one half of. It is kept for the reason this
// package keeps the other one: a test file a person reads needs the sentence
// itself, not only its consequences.
func TestATallBitCostsMoreOfTheBudgetThanAShortOne(t *testing.T) {
	agent := memory.Handle{Ref: "ollama/llama3", Display: "coordinator-7"}

	m := talk(sized(100, 30), 4)
	room := m.room()

	short := m.shown.Bits(m.store)[0]
	m.say(agent, reply)
	tall := m.shown.Bits(m.store)[len(m.shown)-1]

	if got := m.rows(short, room); got != 1 {
		t.Fatalf("a one-line message costs %d rows, want 1", got)
	}
	if m.rows(tall, room) <= m.rows(short, room) {
		t.Fatalf("a %d-row document costs %d against the budget and a one-line message costs %d",
			bitRows(tall, room), m.rows(tall, room), m.rows(short, room))
	}

	// And the load the trigger reads carries it, which is the half that reaches
	// the fold. A count of bits cannot tell these two records apart.
	plain := talk(sized(100, 30), 5)
	if m.foldable() <= plain.foldable() {
		t.Errorf("five bits with a document in them load the budget at %d and five one-line bits at %d",
			m.foldable(), plain.foldable())
	}
}

// The budget and the renderer agree about how tall a bit is, because they ask the
// same function.
//
// Two statements of one rule agree on the day they are written, and the way this
// pair would come apart is invisible: both numbers are plausible, neither is
// printed, and the consequence is a fold sized against a screen nobody is looking
// at. So [Model.rows] is `len` of what [saidWhole] draws, and this is that
// sentence as a check rather than as a comment.
func TestTheBudgetAndTheRendererAgreeAboutABitsHeight(t *testing.T) {
	agent := memory.Handle{Ref: "ollama/llama3", Display: "coordinator-7"}

	for _, size := range [][2]int{{100, 30}, {80, 24}, {46, 30}} {
		m := talk(sized(size[0], size[1]), 4)
		m.say(agent, reply)
		f, room := m.frame(), m.room()

		checked := 0
		for _, b := range m.shown.Bits(m.store) {
			rows := len(saidWhole(f, b, room))
			if got := bitRows(b, room); got != rows {
				t.Errorf("%dx%d: the budget calls %s %d rows and the transcript draws it in %d",
					size[0], size[1], memory.Short(b.ID), got, rows)
			}

			// And through the memo, which is the same claim with a cache in front of
			// it. The table is shared by every model in the process and keyed by a
			// content address, so what makes it safe is being dropped when the room
			// changes — this loop is three rooms in a row, and without that drop the
			// second size reads the first size's heights.
			if got, want := m.rows(b, room), costOf(rows, m.budget()); got != want {
				t.Errorf("%dx%d: the budget charges %s %d rows and the transcript draws it in %d",
					size[0], size[1], memory.Short(b.ID), got, want)
			}
			if rows > 1 {
				checked++
			}
		}
		if checked == 0 {
			t.Fatalf("%dx%d: nothing in the fixture drew more than one row, so the two could not disagree",
				size[0], size[1])
		}
	}

	// The same bits at two rooms, which is the only way the memo can be caught
	// returning a stale height: it is keyed by content address, and two models at
	// two sizes hold different bits. One model resized holds the same ones.
	// A message short enough that its height is under the cap at both widths.
	// A tall one cannot catch a stale height, because the cap clamps the stale
	// number and the fresh one to the same value — which is a fixture reaching
	// past the thing under test, and it took a green mutant to notice.
	medium := "- it swaps from both ends inward, so it allocates nothing\n" +
		"- it is generic over T, so one function covers every element type\n"

	m := talk(sized(100, 30), 4)
	m.say(agent, medium)
	wide := m.rows(m.shown.Bits(m.store)[len(m.shown)-1], m.room())

	mm, _ := m.Update(tea.WindowSizeMsg{Width: 46, Height: 30})
	m = mm.(Model)
	f, room := m.frame(), m.room()
	b := m.shown.Bits(m.store)[len(m.shown)-1]
	if got, want := m.rows(b, room), costOf(len(saidWhole(f, b, room)), m.budget()); got != want {
		t.Errorf("after a resize the budget charges the message %d rows and the transcript draws it in %d (it was %d before)",
			got, want, wide)
	}
}

// No bit is charged more than half a screen, and the cap is what keeps two other
// decisions standing.
//
// A bit charged more than the keep target can never sit inside a kept tail. The
// moment it cannot, the cut stops moving back to the last thing the human said —
// every tail reaching that far is already over the budget — which strands an
// answer from its question, and the fold's window collapses to a single bit,
// which D32 refuses and the footer reports as `held`. Measured with the cap at
// the whole budget instead of half: 16.5% of frames opening on an orphaned answer
// and 33.0% printing `held`, against 0.0% and 0.0% here and today.
//
// So the two consequences are what this asserts, not the arithmetic: on a record
// of documents the cut still lands where the human last spoke, and the fold's
// window is never one bit.
func TestNoBitIsChargedMoreThanHalfAScreen(t *testing.T) {
	agent := memory.Handle{Ref: "ollama/llama3", Display: "coordinator-7"}

	m := sized(100, 30)
	room := m.room()
	m.say(agent, reply)
	tall := m.shown.Bits(m.store)[0]

	if bitRows(tall, room) <= m.budget()/2 {
		t.Fatalf("the fixture's document is %d rows, which is under half a budget of %d — nothing here is capped",
			bitRows(tall, room), m.budget())
	}
	if got, want := m.rows(tall, room), m.budget()/2; got != want {
		t.Fatalf("a document is charged %d of a budget of %d, want half a screen (%d)", got, m.budget(), want)
	}

	// And what the cap is for, on the density this is written for: a conversation
	// where a model answers with a program every few turns. The cut still lands
	// where the human last spoke, and the footer never reports a fold it cannot
	// make.
	orphans, blocked, folds := 0, 0, 0
	for i := range 120 {
		before := slices.Clone(m.shown)
		if i%6 == 5 {
			m.say(agent, reply)
		} else {
			m.say(localHandle, lines[i%len(lines)])
		}
		if m.blocked() {
			blocked++
		}
		if len(took(m, before)) == 0 {
			continue
		}
		folds++

		for _, b := range m.shown.Bits(m.store) {
			if _, cold := b.Payload.(memory.Compaction); cold {
				continue
			}
			if b.From.Ref != localHandle.Ref {
				orphans++
			}
			break
		}
	}
	if folds < 10 {
		t.Fatalf("only %d folds in 120 writes, so neither consequence was exercised", folds)
	}
	if blocked > 0 {
		t.Errorf("%d folds: %d frames read `held`, and the cap exists so that none does", folds, blocked)
	}

	// The orphan is printed rather than bounded, and that is the open half of this
	// change rather than a number nobody decided.
	//
	// D58 took the orphaned head to zero by moving the cut back to the last thing
	// the human said, and a budget in rows takes some of it back: a record with
	// documents in it is large in rows and *small in bits*, D32 requires the
	// fold's window to be at least two bits, and those two together leave
	// [keepFrom] no range to move the cut in. Measured here rather than asserted,
	// because what would close it is a ruling in another package — D32 refuses a
	// run of one on the grounds that folding one bit into a scar standing for one
	// bit buys nothing, which stops being true when the one bit is half a screen.
	// `docs/DEBT.md` carries it.
	t.Logf("a document every sixth bit at 100x30: %d folds, %d opened on an answer whose question had gone",
		folds, orphans)
}

// The density at which the screen stops being able to hold a round, stated as a
// check rather than left for somebody to discover.
//
// At one document in two on a hundred-column, thirty-row terminal, two answers
// are the whole budget: the view cannot hold a question, its answer and anything
// else at once, so [keepFrom]'s search has no range to move the cut in and the
// tail does not always begin where the human spoke. That is a fact about a screen
// that size against messages that large rather than a defect in the rule — the
// alternative is the fold not firing at all, which is what left a five-bit view
// drawing forty rows.
//
// What is asserted is that it degrades rather than breaks: folds keep happening,
// and every bit that leaves the screen is in the receipt of the scar that
// replaced it. The orphan count is printed rather than bounded, because bounding
// it would be pinning a number nobody has decided.
func TestAScreenTooSmallForARoundDegradesRatherThanBreaking(t *testing.T) {
	agent := memory.Handle{Ref: "ollama/llama3", Display: "coordinator-7"}
	m := sized(100, 30)

	folds, orphans, lost := 0, 0, 0
	for i := range 120 {
		before := slices.Clone(m.shown)
		if i%2 == 1 {
			m.say(agent, reply)
		} else {
			m.say(localHandle, lines[i%len(lines)])
		}

		gone := took(m, before)
		if len(gone) == 0 {
			continue
		}
		folds++

		absorbed := map[string]bool{}
		for _, b := range m.shown.Bits(m.store) {
			if c, cold := b.Payload.(memory.Compaction); cold {
				for id := range c.Absorbed() {
					absorbed[id] = true
				}
			}
		}
		for id := range gone {
			// A scar absorbed into a newer scar is named in its Prev and not in
			// Absorbed, which lists originals only (D13). Counting it here was this
			// check's own first defect.
			if b, ok := m.store.Get(id); ok {
				if _, cold := b.Payload.(memory.Compaction); cold {
					continue
				}
			}
			if !absorbed[id] {
				lost++
			}
		}
		for _, b := range m.shown.Bits(m.store) {
			if _, cold := b.Payload.(memory.Compaction); cold {
				continue
			}
			if b.From.Ref != localHandle.Ref {
				orphans++
			}
			break
		}
	}

	if folds < 10 {
		t.Fatalf("only %d folds in 120 writes, so nothing here was exercised", folds)
	}
	if lost > 0 {
		t.Errorf("%d of the bits that left the screen are in no receipt", lost)
	}
	t.Logf("a document every other bit at 100x30: %d folds, %d of them opened on an answer whose question had gone",
		folds, orphans)
}

// Nothing is absorbed that was not drawn cooling first — with documents in the
// record, which is the case that broke three arrangements of this before one held.
//
// [TestNothingIsAbsorbedWithoutFadingFirst] holds the same promise over a record
// of one-line messages, where it is an identity: a view only grows at the end, so
// the same search run one lower on the shorter view returns the same boundary.
// That identity is what a budget in rows destroys, because the boundary then
// depends on the height of a bit nobody has typed yet, and the error is in the one
// direction this surface may not err in. Measured on the arrangement that ran the
// search with its targets lowered: **150 to 246 bits absorbed without ever fading,
// per 300 writes, at every terminal size**.
//
// What holds it is that the fold no longer predicts the fade — it honours it.
// [Model.keep] asks for the boundary of the view *without the bit just written*,
// at that view's own room, and keeps one more; [Model.absorbing] asks the same
// question of the same bits. They name the same boundary by construction rather
// than by an identity somebody has to re-derive.
//
// Three sizes because the two mistakes this catches are both about columns: the
// room a sentence is wrapped into decides a bit's height, and at 60x14 a handle
// arriving under a longer name was enough to move every height on the frame.
func TestNothingIsAbsorbedWithoutFadingFirstWithDocumentsInTheRecord(t *testing.T) {
	agent := memory.Handle{Ref: "ollama/llama3", Display: "coordinator-7"}

	for _, size := range [][2]int{{100, 30}, {80, 24}, {60, 14}} {
		for _, every := range []int{2, 6} {
			m := sized(size[0], size[1])

			folds, tall := 0, 0
			for i := range 120 {
				faded := m.absorbing()
				before := slices.Clone(m.shown)

				// A third speaker, arriving late under a longer name. That widens the
				// handle column, which shortens every sentence on the surface, which
				// makes every bit taller than it was on the frame that named the fold
				// — so the fold and the fade have to be asked at the *same* view's
				// room and not merely with the same rule. Measured before they were:
				// six to eight bits absorbed without fading, at 60x14, over three
				// hundred writes, and nothing else in this package reaches it.
				switch {
				case i%7 == 6:
					m.say(memory.Handle{Ref: "ollama/a", Display: "coordinator-seventeen"}, lines[i%len(lines)])
				case i%every == every-1:
					m.say(agent, reply)
					tall++
				default:
					m.say(localHandle, lines[i%len(lines)])
				}

				gone := took(m, before)
				if len(gone) == 0 {
					continue
				}
				folds++
				for id := range gone {
					if !faded[id] {
						t.Errorf("%dx%d, a document every %d: fold %d absorbed %s, which was drawn hot on the frame before it went",
							size[0], size[1], every, folds, memory.Short(id))
					}
				}
			}
			if folds < 5 || tall < 5 {
				t.Fatalf("%dx%d, a document every %d: %d folds and %d documents, so the promise was barely exercised",
					size[0], size[1], every, folds, tall)
			}
		}
	}
}

// The gauge counts what the budget counts, which is now rows.
//
// It is the antecedent for a fold, so what it fills toward and what it fills with
// have to be the same unit. Before this the numerator was bits and the
// denominator was a screen's rows, and the founder's own frame read `5/23` over a
// view holding forty rows — the surface reporting a fifth full on a screen it had
// already overrun.
func TestTheGaugeReadsTheSameUnitAtBothEnds(t *testing.T) {
	agent := memory.Handle{Ref: "ollama/llama3", Display: "coordinator-7"}

	m := talk(sized(100, 30), 4)
	plain := m.foldable()
	m.say(agent, reply)

	if got := m.foldable(); got != plain+m.budget()/2 {
		t.Errorf("a document arrived and the load went from %d to %d, want it up by half a screen (%d)",
			plain, got, m.budget()/2)
	}
	if want := fmt.Sprintf("%d/%d", m.foldable(), m.budget()); !strings.Contains(ansi.Strip(m.View().Content), want) {
		t.Errorf("the footer does not read %q:\n%s", want, m.View().Content)
	}
}
