package tui

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/tyler-j-chrestoff/tldreddit/memory"
)

func record(n int) Model {
	m := New()
	for i := range n {
		m.composer.SetValue(fmt.Sprintf("bit %d", i))
		m.send()
	}
	return m
}

// Sending past the limit must fold on its own. If pressure only ever released
// when a human pressed a key, the record would grow without bound exactly when
// they were too busy to notice.
func TestSendFoldsAtLimit(t *testing.T) {
	m := record(coolAt)
	if got := len(m.hot()); got != coolAt {
		t.Fatalf("hot = %d after %d sends, want no fold yet", got, coolAt)
	}

	m.composer.SetValue("one too many")
	m.send()

	if got := len(m.hot()); got != keepHot {
		t.Errorf("hot = %d after fold, want %d", got, keepHot)
	}
	shown := m.shown.Bits(m.store)
	if _, cold := shown[0].Payload.(memory.Compaction); !cold {
		t.Errorf("shown[0] = %T, want the fold at the head", shown[0].Payload)
	}
}

// A fold must leave the record walkable. This is D1 seen from the surface: the
// screen has dropped bits, and every edge on screen still resolves, because
// dropping happened in the view and not in the store.
func TestFoldKeepsGraphWalkable(t *testing.T) {
	m := record(coolAt * 3)

	for _, b := range m.shown.Bits(m.store) {
		for _, p := range b.Prev {
			if _, ok := m.store.Get(p); !ok {
				t.Errorf("bit %s points at %s, which the store lost",
					memory.Short(b.ID), memory.Short(p))
			}
		}
	}
}

// Nothing may be lost across folds: every original bit is either still on
// screen or named on a receipt.
func TestFoldConservesCount(t *testing.T) {
	const sends = coolAt * 3
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
	m := record(coolAt * 3)

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

// The fade has to name the bits the *next* fold takes, not the bits a fold
// happening this instant would take. send appends and then tests the band, so
// the cut is computed against a view one longer than the one last drawn: after
// coolAt sends the next send folds seven, and the arithmetic that answered six
// let one bit go from full brightness to absorbed with no frame in between.
func TestFadeMarksWhatIsNext(t *testing.T) {
	m := record(coolAt)
	if got, want := m.fadeBefore(), coolAt+1-keepHot; got != want {
		t.Errorf("fadeBefore = %d, want %d — the next send folds %d, not %d",
			got, want, want, got)
	}
}

// Nothing may be absorbed that was not drawn cooling in the frame before it
// went. That one sentence is the whole reason a fold firing on its own is not
// something happening behind the user's back, and it is the property the fade
// exists to deliver — so it is checked against every fold in a long record
// rather than against one hand-picked frame.
//
// The second assertion is what stops the first from being satisfied by fading
// everything: on the frame immediately before a fold there must still be bits
// drawn hot, or the warning has stopped distinguishing anything.
func TestNothingIsAbsorbedWithoutFadingFirst(t *testing.T) {
	m := New()

	folds := 0
	for i := range coolAt * 6 {
		// The frame the person is looking at, right now, before they press
		// anything.
		faded := map[string]bool{}
		for _, id := range m.shown[:m.fadeBefore()] {
			faded[id] = true
		}
		stillHot := len(m.shown) - m.fadeBefore()
		before := len(m.shown)

		m.composer.SetValue(fmt.Sprintf("bit %d", i))
		m.send()

		if len(m.shown) == before+1 {
			continue // no fold on this send
		}
		folds++

		// A fold's Prev is every bit in the window it absorbed, in window order
		// (D13), so it names exactly what left the screen.
		cold, ok := m.store.Get(m.shown[0])
		if !ok {
			t.Fatalf("fold %d put %s on the view, which the store does not hold",
				folds, memory.Short(m.shown[0]))
		}
		for _, id := range cold.Prev {
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
		t.Fatalf("only %d folds in %d sends, so the guarantee was barely exercised", folds, coolAt*6)
	}
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
	m := record(coolAt)
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
	m := record(coolAt + 1)
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
	m := record(coolAt * 3)

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
	m := record(coolAt + 1)
	m.unfold()
	if !m.unfolded {
		t.Fatal("receipt did not open")
	}

	for range coolAt {
		m.composer.SetValue("more")
		m.send()
	}
	if m.unfolded {
		t.Error("a fold fired while the receipt was open and left it open")
	}
}

// The receipt resolves in full: every address it names, in the order it names
// them, and exactly as many as it claims. The count on the scar is a promise a
// person checks by counting rows, so the rows have to be the receipt itself.
func TestRecallFollowsTheWholeReceipt(t *testing.T) {
	m := record(coolAt * 3)
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
	const sends = coolAt * 3
	m := record(sends)

	var got []string
	for _, b := range m.shown.Bits(m.store) {
		switch p := b.Payload.(type) {
		case memory.Compaction:
			for _, r := range recall(m.store, p) {
				got = append(got, said(r.bit))
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
	m := record(coolAt * 3)
	c := scar(t, m)

	for _, width := range []int{200, 80, 40, 24, 20, 8, 1} {
		got := rows(unfold(m.store, c, width))
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
		for i, row := range rows(unfold(s, c, width)) {
			if !strings.Contains(row, names[i]) {
				t.Errorf("unfold at width %d shortened %q with room to spare: %q",
					width, names[i], row)
			}
		}
	}

	// Narrow enough that something has to give: whatever gives, says so.
	for _, width := range []int{60, 40, 30, 24, 20} {
		for i, row := range rows(unfold(s, c, width)) {
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
		for _, row := range rows(unfold(memory.NewStore(), c, width)) {
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

// Nothing this surface draws may run past the width it was given. The viewport
// clips rather than wraps, and a row cut by the clip looks exactly like a row
// that happened to end there — which is the one thing a screen arguing that it
// shows you what it dropped is not allowed to do.
func TestNoRowRunsPastTheWidthItWasGiven(t *testing.T) {
	m := record(coolAt * 3)
	bits := m.shown.Bits(m.store)

	for _, width := range []int{200, 100, 80, 40, 24, 20, 16, 12, 8, 4, 1} {
		for _, open := range []bool{false, true} {
			for i, row := range strings.Split(transcript(m.store, bits, m.fadeBefore(), width, open), "\n") {
				if w := lipgloss.Width(row); w > width {
					t.Errorf("transcript at width %d (open=%v): row %d is %d wide: %q",
						width, open, i+1, w, ansi.Strip(row))
				}
			}
		}
	}
}

// A scar drops its span before it drops its words. The span is one press from
// being back — every absorbed bit carries its own time in the block ctrl+u
// opens — while the words are the only account anywhere of what the window was
// about. The comment said this before the code did: the candidate list went
// four words to two before it gave up the span, so there was no width at which
// four words appeared without it.
func TestSeamDropsTheSpanBeforeTheWords(t *testing.T) {
	c := cooled(t,
		"migration started against staging",
		"staging backfill running",
		"backfill migration finished",
		"staging looks green")

	widest := seam(c, 400, false)
	span := fmt.Sprintf("%s–%s", c.From().Format("15:04"), c.To().Format("15:04"))
	words := topWords(c.Bag(), 4)
	if len(words) < 4 || !strings.Contains(widest, span) {
		t.Fatalf("seam at full width = %q, want a span and four words to trade off", widest)
	}

	// Wherever the span is still shown, every word is still shown too.
	for width := 1; width <= lipgloss.Width(widest); width++ {
		got := ansi.Strip(seam(c, width, false))
		if !strings.Contains(got, span) {
			continue
		}
		for _, w := range words {
			if !strings.Contains(got, w) {
				t.Errorf("seam at width %d kept the span but dropped %q: %q", width, w, got)
			}
		}
	}

	// And there is a width where the span is gone and every word is still
	// there. Without this the rule above is satisfied by a ladder that gives up
	// the span and two of the words in the same step — which is what the code
	// did, so there was no width at which four words appeared without a span.
	var dropped bool
	for width := 1; width <= lipgloss.Width(widest) && !dropped; width++ {
		got := ansi.Strip(seam(c, width, false))
		if strings.Contains(got, span) {
			continue
		}
		dropped = true
		for _, w := range words {
			dropped = dropped && strings.Contains(got, w)
		}
	}
	if !dropped {
		t.Errorf("no width shows all of %v without the span, so the words go first, not the span", words)
	}
}

func TestSeamShowsItsReceipt(t *testing.T) {
	got := seam(cooled(t, "the deploy failed", "deploy again", "and again"), 80, false)
	for _, want := range []string{"3 bits", "09:00–09:02", "deploy"} {
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
	c := cooled(t, "the deploy failed", "deploy again", "and again")
	floor := lipgloss.Width(fmt.Sprintf("── %d bits · ctrl+u", c.Count()))

	for width := floor; width <= 200; width++ {
		got := ansi.Strip(seam(c, width, false))
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
		got := ansi.Strip(seam(c, width, false))
		if w := lipgloss.Width(got); w > width {
			t.Errorf("seam at width %d is %d wide: %q", width, w, got)
		}
		if !strings.HasSuffix(got, "…") {
			t.Errorf("seam at width %d was cut with no mark: %q", width, got)
		}
	}
}
