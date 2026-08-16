package tui

import (
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/tyler-j-chrestoff/tldreddit/memory"
)

// press is one keystroke through Update, so what these tests exercise is the key
// a person presses rather than the method behind it. The binding is half of what
// is being claimed: a surface reached by a key nobody bound is not reachable.
func press(m Model, key string) Model {
	var msg tea.KeyPressMsg
	switch key {
	case "up":
		msg = tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		msg = tea.KeyPressMsg{Code: tea.KeyDown}
	case "shift+up":
		msg = tea.KeyPressMsg{Code: tea.KeyUp, Mod: tea.ModShift}
	case "shift+down":
		msg = tea.KeyPressMsg{Code: tea.KeyDown, Mod: tea.ModShift}
	default:
		msg = tea.KeyPressMsg{Code: rune(key[len(key)-1]), Mod: tea.ModCtrl}
	}
	next, _ := m.Update(msg)
	return next.(Model)
}

// rankShot is the ranked list this model would draw, from its own frame, ANSI
// stripped. Everything but the width is resolved the way the program resolves it
// — a test that built its own frame would be looking at an arrangement nobody is
// running.
func rankShot(m Model, width int) []string {
	f := m.frame()
	f.width = width
	body, _ := ranked(f)
	return strings.Split(ansi.Strip(body), "\n")
}

// shorten is a view as short addresses, so a failure prints something a person
// can compare row by row rather than eight lines of hex.
func shorten(v memory.View) []string {
	out := make([]string, 0, len(v))
	for _, id := range v {
		out = append(out, memory.Short(id))
	}
	return out
}

// judgedTalk is a conversation with votes cast as it went, some of them long
// enough ago that the hold is spent and the fold has taken the bit. That is the
// state this surface exists for: a list whose rows are mostly behind a scar.
//
// The spent holds are the part that matters. A vote cast with the moment of the
// key never ages here — [Model.say] writes microseconds apart and a hold decays
// against the conversation's own clock — so a fixture of nothing but those folds
// none of what it judged, and every row would still be on the transcript.
func judgedTalk(m Model, n int) Model {
	handles := []memory.Handle{localHandle, {Ref: "ollama/llama3", Display: "coordinator-7"}}
	for i := range n {
		m.say(handles[i%len(handles)], lines[i%len(lines)])
		switch {
		case i%3 == 0:
			m = heldSince(m, holdFor+time.Second)
		case i%7 == 0:
			m.vote(memory.Down)
		}
	}
	return m
}

// The list is everything anybody said, each once, plus anything else somebody
// voted on — and nothing else.
//
// Every one of them, because a ranked reading that quietly dropped what the
// transcript can no longer show would be ranking the screen rather than the
// record, and most of what has been said in any real conversation is behind a
// scar. Each once, because a person who changes their mind casts a second vote
// and the record keeps both; two rows for one bit would report one bit as two.
//
// It used to be the voted bits alone, and the count is the whole finding: on this
// project's own record that filter drew three rows out of twenty-nine said bits,
// with a claim later found unsourced at rank one and the correction to it absent
// from the screen. A ballot is still not a row and an unvoted fold is still not a
// row — the material a fold stands for is in here on its own account — and the
// two exclusions are asserted below rather than left to the doc comment.
func TestTheRankedListIsEverythingSaidPlusAnythingElseJudged(t *testing.T) {
	// Through the key, because [Model.list] answers for whichever surface is up
	// and asking it on the transcript is a question about the transcript. That is
	// not a subtlety of the test: it is the first thing the test got wrong, and
	// what it reported was thirteen rows nobody had voted on.
	m := press(judgedTalk(sized(100, 30), 40), "ctrl+t")

	want, judged := map[string]bool{}, map[string]bool{}
	for _, v := range m.votes.Bits(m.store) {
		judged[v.Prev[0]] = true
		want[v.Prev[0]] = true
	}
	said, scars, ballots := 0, 0, 0
	for b := range m.store.All() {
		switch b.Payload.(type) {
		case memory.Utterance:
			said++
			want[b.ID] = true
		case memory.Compaction:
			scars++
		default:
			ballots++
		}
	}
	if len(judged) < 5 || said < 20 || scars == 0 || ballots == 0 {
		t.Fatalf("the fixture has %d judged, %d said, %d scars and %d ballots — not enough of each to be testing the rule",
			len(judged), said, scars, ballots)
	}

	got := m.list()
	if len(got) != len(want) {
		t.Errorf("the ranked list holds %d rows for %d said-or-judged bits", len(got), len(want))
	}
	for _, id := range got {
		if !want[id] {
			t.Errorf("%s is in the ranked list and it was neither said nor voted on", memory.Short(id))
		}
	}
	for id := range want {
		if !slices.Contains(got, id) {
			t.Errorf("%s was said or voted on and is not in the ranked list", memory.Short(id))
		}
	}

	// The two exclusions, on the addresses rather than on the doc comment. A vote
	// never gets a row of its own; a fold gets one only where somebody voted on it.
	for b := range m.store.All() {
		if _, cold := b.Payload.(memory.Compaction); cold && !judged[b.ID] && slices.Contains(got, b.ID) {
			t.Errorf("%s is a fold nobody voted on and it has a row of its own", memory.Short(b.ID))
		}
	}
	for _, v := range m.votes.Bits(m.store) {
		if slices.Contains(got, v.ID) {
			t.Errorf("%s is a ballot and it has a row of its own", memory.Short(v.ID))
		}
	}

	// And most of it is not on the transcript, which is what makes this retrieval
	// rather than a re-sort of what is already on screen. Stated as a majority
	// rather than as a count: the count moves with fixtureBudget and the fixture, and the
	// property does not.
	behind := 0
	for _, id := range got {
		if !slices.Contains(m.shown, id) {
			behind++
		}
	}
	if behind*2 <= len(got) {
		t.Errorf("%d of %d ranked rows are off the transcript; this fixture is not exercising retrieval",
			behind, len(got))
	}
}

// A ranked list nobody has voted in spends no columns on the vote column.
//
// This case did not exist before [Model.judged] widened: a record with no votes in
// it drew an empty list, so the column could be unconditional and cost nothing.
// The list is now everything said, so the first thing anybody sees on a fresh
// record is this screen with nothing judged on it — and columns are the scarcest
// thing on either surface here. Measured on the transcript, one upvote costs five
// columns off every sentence in the view; a column that says nothing on any row
// must not cost anything on any row.
//
// Pinned as the column the handle starts in, measured on a named fixture, rather
// than as a comparison across widths — because a sweep asserting the unvoted lead
// is always the narrower one is **false**, and finding that out is the useful half.
// Measured: at 58 columns the unvoted list still carries the address and the voted
// one has already shed it, so the voted lead is 22 and the unvoted lead is 28. The
// two ladders step at different widths, so any claim relating them holds inside a
// rung and not across the sweep. That is this package's standing trap about rungs
// sized against each other, met from a third direction.
//
// The four numbers below are read off drawn frames and are what goes red when the
// column stops being conditional. Two widths, because one of them is above every
// step of both ladders and the other is below the step that narrows the column.
func TestAnUnvotedRankedListSpendsNothingOnTheVoteColumn(t *testing.T) {
	plain := press(talk(sized(100, 30), 12), "ctrl+t")
	for _, r := range plain.ranking() {
		if r.Own != 0 {
			t.Fatal("the fixture has a vote in it, so there is no unvoted list to measure")
		}
	}
	voted := press(heldSince(press(plain, "ctrl+t"), 0), "ctrl+t")

	// The column the handle starts in, on the first row of the list that draws it.
	// Both lists hold the same bits in the same order.
	handle := func(m Model, width int) int {
		for _, row := range rankShot(m, width) {
			if at := strings.Index(row, "coordinator-7"); at >= 0 {
				return at
			}
		}
		return -1
	}

	for _, want := range []struct{ width, plain, voted int }{
		{100, 28, 41},
		{56, 18, 22},
	} {
		if got := handle(plain, want.width); got != want.plain {
			t.Errorf("at width %d a list nobody voted in starts its handle at column %d, measured %d\n%s",
				want.width, got, want.plain, strings.Join(rankShot(plain, want.width), "\n"))
		}
		if got := handle(voted, want.width); got != want.voted {
			t.Errorf("at width %d a list with one vote in it starts its handle at column %d, measured %d\n%s",
				want.width, got, want.voted, strings.Join(rankShot(voted, want.width), "\n"))
		}
	}
}

// Kept above let go, and every row under a heading carries the mark that heading
// names.
//
// It is [memory.View.Rank]'s tier seen from the screen, and it is asserted on the
// drawn rows rather than on the ordering, because the ordering is memory's own
// test. What is this package's to get wrong is the heading: a band drawn over a
// run it does not describe is a screen that has relabelled somebody's vote, and
// no ordering test can see it.
func TestKeptRanksAboveLetGo(t *testing.T) {
	m := press(judgedTalk(sized(100, 30), 40), "ctrl+t")

	// The mark each heading claims about the rows under it. They are the vote's
	// own glyphs: solid and hollow for a hold that is live and one that is spent,
	// both of them a bit somebody kept.
	//
	// The middle band is the one this screen most needs to be honest about, and its
	// claim is an *absence*: a row under "not judged" must carry no ballot glyph at
	// all. That heading was unreachable while the list was the voted bits alone,
	// and it is now most of the list, so a mark leaking into it would be the screen
	// reporting a vote nobody cast — beside a heading saying nobody had.
	marks := map[string]string{"kept": "▲△", "not judged": "", "let go": "▼"}
	order := map[string]int{"kept": 0, "not judged": 1, "let go": 2}

	heading, seen, counts := "", -1, map[string]int{}
	for _, row := range rankShot(m, 100) {
		if word, _, found := strings.Cut(row, " · "); found {
			if _, band := order[word]; band {
				if order[word] <= seen {
					t.Fatalf("the %q band came after %q, so the tiers are out of order", word, heading)
				}
				heading, seen = word, order[word]
				continue
			}
		}
		if heading == "" {
			t.Fatalf("a row was drawn before any band said what it is: %q", row)
		}
		if marks[heading] == "" {
			if strings.ContainsAny(row, "▲△▼") {
				t.Errorf("a row under %q carries a ballot glyph: %q", heading, row)
			}
		} else if !strings.ContainsAny(row, marks[heading]) {
			t.Errorf("a row under %q carries none of %q: %q", heading, marks[heading], row)
		}
		counts[heading]++
	}
	for _, word := range []string{"kept", "not judged", "let go"} {
		if counts[word] == 0 {
			t.Fatalf("the fixture drew no rows under %q, so that band is untested: %v", word, counts)
		}
	}
}

// A fold takes nothing off the ranked list, and that is the property this screen
// demonstrates rather than a nicety of the implementation.
//
// A fold changes what the transcript shows and changes nothing about what was
// said, so a list built over the record survives it whole. Build the same list
// over [Model.shown] instead and half of it disappears at the first fold, which
// is the mutation this is cited against.
//
// **It used to assert the drawn rows were byte-identical, and that is no longer
// true for an honest reason.** The list is now everything said rather than the
// voted bits alone, so the conversation that triggers the fold also puts its own
// bits into the list, every ordinal after them moves, and the counts in the band
// headings move with it. What is asserted instead is the property the byte
// comparison was standing in for: every address that was on the list before is
// still on it, in the same relative order, in the same band. The rows that
// arrived are asserted separately, because a list that lost nothing and gained
// nothing would pass the first half and be broken.
//
// It is driven through the keys: ctrl+t to get there, and then a conversation
// carrying on underneath it.
func TestTheRankedListDoesNotMoveWhenAFoldFires(t *testing.T) {
	m := press(judgedTalk(sized(100, 30), 24), "ctrl+t")
	if !m.ranked {
		t.Fatal("ctrl+t did not reach the ranked surface")
	}

	before, mark, was := slices.Clone(m.list()), m.mark, slices.Clone(m.shown)
	bands := map[string]int{}
	for _, r := range m.ranking() {
		bands[r.ID] = r.Own
	}
	if len(before) < 5 {
		t.Fatalf("the ranked list is %d rows, which is not enough to notice it moving", len(before))
	}

	// Enough for at least one fold, and the assertion is that one happened rather
	// than that this loop is long enough.
	//
	// A fold is a row leaving the transcript, not a scar appearing: a scar counts
	// toward the size rule like any other bit (D32), so folding again absorbs the
	// old scar into the new one and the count of them does not move. Counting them
	// was this test's first version, and it reported "1 scars before, 1 after" over
	// a fold that had plainly happened.
	for range fixtureBudget {
		m.say(localHandle, "carrying on")
	}
	took := 0
	for _, id := range was {
		if !slices.Contains(m.shown, id) {
			took++
		}
	}
	if took == 0 {
		t.Fatal("no fold fired: every row that was on the transcript is still there")
	}

	after := m.list()
	kept := make([]string, 0, len(before))
	for _, id := range after {
		if slices.Contains(before, id) {
			kept = append(kept, id)
		}
	}
	if !slices.Equal(before, kept) {
		t.Errorf("the fold moved the ranked list.\nbefore: %v\nafter, of those still there: %v",
			shorten(before), shorten(kept))
	}
	now := map[string]int{}
	for _, r := range m.ranking() {
		now[r.ID] = r.Own
	}
	for id, own := range bands {
		if now[id] != own {
			t.Errorf("%s was in band %+d before the fold and is in %+d after", memory.Short(id), own, now[id])
		}
	}
	if len(after) <= len(before) {
		t.Errorf("the list is %d rows and was %d: the conversation that caused the fold reached the record and not this screen",
			len(after), len(before))
	}
	if m.mark != mark {
		t.Errorf("a fold took the caret from %s to %s while the ranked view was up",
			memory.Short(mark), memory.Short(m.mark))
	}
}

// Nothing arriving takes the caret off this screen.
//
// The caret rides the newest bit until somebody moves it, which is right on the
// transcript and wrong here: what arrives is not drawn on this surface until
// somebody votes on it, so following it would park the caret somewhere the reader
// cannot see and aim the vote key there. The state that reaches it is ordinary —
// keep the thing that just arrived, then press ctrl+t — because that is the one
// case where the caret is on a judged bit *and* on the newest bit at once.
func TestNothingArrivingTakesTheCaretOffTheRankedList(t *testing.T) {
	m := judgedTalk(sized(100, 30), 24)
	m.vote(memory.Up) // the caret is riding, so this judges the newest bit
	m = press(m, "ctrl+t")

	parked := m.mark
	if !slices.Contains(m.list(), parked) {
		t.Fatal("the caret is not in the ranked list, so this fixture never reaches the state")
	}
	if !slices.Contains(m.shown, parked) || parked != m.shown[len(m.shown)-1] {
		t.Fatal("the caret is not on the newest bit, so nothing here could follow anything")
	}

	m.say(localHandle, "something else arrives")
	if m.mark != parked {
		t.Errorf("a bit arriving moved the caret from %s to %s, off the list on screen",
			memory.Short(parked), memory.Short(m.mark))
	}
	if !slices.Contains(m.list(), m.mark) {
		t.Errorf("the caret is on %s, which this surface does not draw", memory.Short(m.mark))
	}
}

// The caret survives a fold that happened while it was parked on the other
// surface, and the transcript draws it on the scar that absorbed its bit.
//
// This is the price of one caret and two screens. [Model.fold] re-attaches the
// caret as it folds, which is enough while the transcript is the only surface;
// nothing re-attaches it when the fold fires under a screen that does not fold.
// Without [stands] resolving it at draw time the transcript comes back with no
// caret on it at all and nothing saying so — the frame is not wrong, it is
// missing, which is the failure mode this whole program is an argument against.
func TestTheCaretComesBackFromTheRankedViewOntoTheScarThatAbsorbedItsBit(t *testing.T) {
	m := judgedTalk(sized(100, 30), 12)

	// The caret goes onto an old judged bit, and the ranked surface goes up.
	m = press(m, "ctrl+t")
	for range len(m.list()) {
		m = press(m, "down")
	}
	parked := m.mark
	if !slices.Contains(m.shown, parked) {
		t.Fatal("the bit the caret is on has already been folded, so the fold below is not what moves it")
	}

	for range fixtureBudget * 2 {
		m.say(localHandle, "carrying on")
	}
	if slices.Contains(m.shown, parked) {
		t.Fatalf("%s was never folded, so nothing is being tested", memory.Short(parked))
	}
	if m.mark != parked {
		t.Fatalf("the caret moved off %s while the ranked view was up", memory.Short(parked))
	}

	m = press(m, "ctrl+t")
	if m.ranked {
		t.Fatal("ctrl+t did not come back to the transcript")
	}

	// The frame the transcript draws has a caret in it, on a scar whose receipt
	// names the bit the caret was on. Asserted on the drawn frame rather than on
	// [Model.mark], because the mark is deliberately still the folded bit: what is
	// being claimed is that the screen shows it somewhere.
	f := m.frame()
	if f.mark == parked {
		t.Errorf("the frame draws the caret on %s, which this view does not hold",
			memory.Short(parked))
	}
	b, ok := m.store.Get(f.mark)
	if !ok {
		t.Fatalf("the frame draws the caret on %s, which the store does not hold",
			memory.Short(f.mark))
	}
	c, cold := b.Payload.(memory.Compaction)
	if !cold {
		t.Fatalf("the caret came back onto a %T rather than onto a fold", b.Payload)
	}
	if !slices.Contains(slices.Collect(c.Absorbed()), parked) {
		t.Errorf("the caret came back onto a scar whose receipt does not name %s",
			memory.Short(parked))
	}

	if m.anchors.mark < 0 {
		t.Error("the transcript drew no caret at all")
	}
	body := strings.Split(ansi.Strip(shot(m, 100, false)), "\n")
	if row := body[m.anchors.mark]; !strings.HasPrefix(row, caret) {
		t.Errorf("the row the caret is said to be on does not carry it: %q", row)
	}
}

// A vote cast on a bit that was folded long ago draws no gauge, because there is
// nothing left for it to hold back.
//
// [memory.Stay.Holds] is a function of the vote view and one instant and takes no
// view at all, so a fresh upvote on a folded bit comes back with the whole
// lifetime remaining. The transcript never noticed, because it only draws bits
// its own view holds. This surface draws bits that left the view an hour ago and
// puts a vote key on them, so it is the first thing that can ask the question —
// and the answer has to be the hollow mark [voteCell] already documents: the vote
// is on the record permanently, and the stay of execution it bought is spent.
func TestAFoldedBitsFreshUpvoteDrawsNoGauge(t *testing.T) {
	m := press(judgedTalk(sized(100, 30), 24), "ctrl+t")

	// The first row in the list that the transcript no longer holds, walked to
	// with the arrow keys from wherever ctrl+t left the caret. It used to count
	// down from row zero, on the assumption that reaching this surface snapped the
	// caret to the top of the list — true while the list was the voted bits alone
	// and the caret was usually on something unjudged, and false now that the caret
	// is nearly always already on a row of this list.
	at, from := -1, slices.Index(m.list(), m.caret())
	for i, id := range m.list() {
		if !slices.Contains(m.shown, id) {
			at = i
			break
		}
	}
	if at < 0 || from < 0 {
		t.Fatalf("caret at %d and the first folded row at %d: there is nothing to walk to", from, at)
	}
	for range max(at-from, from-at) {
		if at > from {
			m = press(m, "down")
		} else {
			m = press(m, "up")
		}
	}
	folded := m.mark
	if slices.Contains(m.shown, folded) {
		t.Fatalf("the caret landed on %s, which the transcript still holds", memory.Short(folded))
	}

	m = press(m, "shift+up")
	if got := memory.Tally(m.store, m.votes)[folded][localHandle]; got != 1 {
		t.Fatalf("the upvote did not land: the tally says %d for %s", got, memory.Short(folded))
	}
	if _, live := m.stay().Holds(m.store, m.day())[folded]; !live {
		t.Fatal("the hold has already expired, so the filter below is not what is being tested")
	}

	if _, drawn := m.frame().holds[folded]; drawn {
		t.Errorf("%s is folded and the frame carries a live hold for it: a gauge would be draining beside a row nothing can hold out of anything",
			memory.Short(folded))
	}

	// And on the screen: the hollow mark, with no gauge glyph anywhere on the row.
	// Found by address rather than by index, because the upvote just moved it into
	// the kept band and every ordinal after it moved with it.
	row := ""
	for _, r := range rankShot(m, 100) {
		if strings.Contains(r, memory.Short(folded)) {
			row = r
		}
	}
	if row == "" {
		t.Fatalf("%s has no row in the ranked list", memory.Short(folded))
	}
	if !strings.Contains(row, "△") {
		t.Errorf("the row does not carry the spent-hold mark: %q", row)
	}
	if strings.ContainsAny(row, "▓░") {
		t.Errorf("the row draws a gauge for a hold that holds nothing: %q", row)
	}
}

// A scar in the ranked list is drawn as what it is. It can get there without
// anybody meaning it to: [Model.fold] moves the caret onto the scar that absorbed
// its bit, and the vote key acts on wherever the caret is, so a person can upvote
// a fold in two keystrokes.
//
// Before this surface existed, nothing drew a bit by address — the transcript
// switches on the payload first and hands a fold to [seam] — so [oneLine]'s
// default branch had never been reached in anger, and the row would have read
// "<unrendered memory.Compaction>".
func TestAVotedScarIsARowRatherThanAnUnrenderedPayload(t *testing.T) {
	m := talk(sized(100, 30), 9)
	m = back(m, 6)
	for range fixtureBudget {
		m.say(localHandle, "carrying on")
	}
	if !m.cold(m.mark) {
		t.Fatal("the caret did not follow its bit onto a scar, so there is no scar to vote on")
	}

	m.vote(memory.Up)
	m = press(m, "ctrl+t")

	// The scar's own row, found by address. It is the only fold in this list — the
	// rest of the list is everything that was said, including every bit this scar
	// stands for — so it is the row under the first heading, and asserting that
	// rather than the length is what survives the list holding the material too.
	rows := rankShot(m, 100)
	if len(rows) < 2 || rows[0] != "kept · 1" {
		t.Fatalf("want a kept band of one over the voted scar, got:\n%s", strings.Join(rows, "\n"))
	}
	row := rows[1]
	if !strings.Contains(row, memory.Short(m.mark)) {
		t.Fatalf("the row under the kept heading is not the scar's:\n%s", strings.Join(rows, "\n"))
	}
	if strings.Contains(row, "unrendered") {
		t.Errorf("the scar drew its Go type instead of its receipt: %q", row)
	}

	c, _ := m.store.Get(m.mark)
	count := c.Payload.(memory.Compaction).Count()
	if want := memory.Short(m.mark); !strings.Contains(row, want) {
		t.Errorf("the row does not carry the scar's address %s: %q", want, row)
	}
	if want := strings.Fields(row); len(want) == 0 || !strings.Contains(row, "bits") {
		t.Errorf("the row does not say how much the fold took: %q", row)
	}
	if !strings.Contains(row, strings.TrimSpace(strings.Split(row, "  ")[0])) || count == 0 {
		t.Fatalf("the fixture's scar absorbed nothing: %q", row)
	}
}

// One fold, one account of itself, on both screens.
//
// This is the whole reason the quotation needed the store threaded to it rather
// than being computed inside [seam] alone. Before, the transcript's scar and the
// ranked list's scar were two summaries of one object built by two functions —
// they happened to agree because both called topWords on the same bag, which is
// agreement by coincidence rather than by construction, and it is exactly the
// arrangement that drifts. They now come from one [frame.quoted].
//
// Asserted on the two drawn rows rather than on the function they share, because
// the claim is about what a person reads on two screens.
func TestAScarQuotesTheSameBitOnBothSurfaces(t *testing.T) {
	m := talk(sized(100, 30), 9)
	m = back(m, 6)
	for range fixtureBudget {
		m.say(localHandle, "carrying on")
	}
	if !m.cold(m.mark) {
		t.Fatal("the caret did not follow its bit onto a scar, so there is no scar to vote on")
	}
	m.vote(memory.Up)

	c, _ := m.store.Get(m.mark)
	fold := c.Payload.(memory.Compaction)
	said, _ := m.frame().quoted(fold)
	want := oneLine(said)
	if want == "" {
		t.Fatal("the scar quotes nothing, so there is nothing for the two surfaces to agree about")
	}

	// The transcript, at a width where the whole quotation fits.
	if got := ansi.Strip(seam(m.frame(), fold, 200)); !strings.Contains(got, want) {
		t.Errorf("the transcript's scar row does not quote %q: %q", want, got)
	}

	// And the ranked list's row for the same scar, at the same generosity.
	var row string
	for _, r := range rankShot(press(m, "ctrl+t"), 200) {
		if strings.Contains(r, memory.Short(m.mark)) {
			row = r
		}
	}
	if row == "" {
		t.Fatalf("the voted scar has no row in the ranked list")
	}
	if !strings.Contains(row, want) {
		t.Errorf("the ranked list's row for the same scar does not quote %q: %q", want, row)
	}
}

// The keys that mean something only on the transcript do nothing here, and doing
// nothing is the decision. ctrl+k is a shortcut for something that happens on its
// own, and taking it on a screen where the collapse cannot be watched is the
// failure this surface exists to prevent.
func TestFoldingAndUnfoldingDoNothingOnTheRankedSurface(t *testing.T) {
	m := press(judgedTalk(sized(100, 30), 24), "ctrl+t")
	before, open, rows := slices.Clone(m.shown), m.unfolded, rankShot(m, 100)

	m = press(m, "ctrl+k")
	if !slices.Equal(m.shown, before) {
		t.Errorf("ctrl+k folded the transcript from a screen that cannot show it happening")
	}
	m = press(m, "ctrl+u")
	if m.unfolded != open {
		t.Error("ctrl+u opened a receipt on a surface that draws no scar to open it on")
	}
	if after := rankShot(m, 100); !slices.Equal(rows, after) {
		t.Error("a key that does nothing here changed what this screen draws")
	}
}

// Voting on a ranked row moves it between the bands, under the caret, on the
// frame after the key. That is the vote ranking something rather than delaying a
// fold, and it is the one thing this screen demonstrates that the transcript
// cannot.
func TestAVoteMovesARowBetweenTheBands(t *testing.T) {
	m := press(judgedTalk(sized(100, 30), 24), "ctrl+t")

	kept := m.ranking()
	if kept[0].Own <= 0 {
		t.Fatal("the top of the list is not a kept row, so there is no band to move out of")
	}
	target := kept[0].ID

	// Walked to with the arrow keys rather than assumed. Reaching this surface used
	// to snap the caret to the top of the list whenever the caret was on something
	// unjudged, which was most of the time; the list is now everything said, so the
	// caret arrives already on a row and the key below would act on that one.
	at := slices.Index(m.list(), m.caret())
	if at < 0 {
		t.Fatal("the caret is on nothing this surface draws")
	}
	for range at {
		m = press(m, "up")
	}
	if m.mark != target {
		t.Fatalf("walking to the top of the list landed on %s, not on %s",
			memory.Short(m.mark), memory.Short(target))
	}

	m = press(m, "shift+down")
	if m.mark != target {
		t.Errorf("the caret let go of %s and did not stay on it", memory.Short(target))
	}

	after := m.ranking()
	if after[0].ID == target {
		t.Error("a row let go of is still at the top of the kept band")
	}
	at = slices.IndexFunc(after, func(r memory.Ranked) bool { return r.ID == target })
	if at < 0 {
		t.Fatalf("%s left the ranked list when it was let go of", memory.Short(target))
	}
	if after[at].Own >= 0 {
		t.Errorf("the row this reader let go of ranks as %+d", after[at].Own)
	}
	if at != len(after)-1 {
		// Newest first inside a band, and it was the newest kept bit, so letting go
		// of it puts it at the head of the other band rather than the end of the
		// list — unless the let go band was empty, which this fixture rules out.
		if after[at].Own != after[at+1].Own {
			t.Errorf("the row landed at %d, between two bands", at)
		}
	}
}

// readingRanked is this surface in the state the work on it is about: a long
// answer, judged, with the caret on it and other rows around it.
//
// It says nothing about *where* in the list that row lands, and that omission is
// deliberate rather than lax. The order is [memory.View.Rank]'s and its tiebreak
// is memory's own claim, held by memory's own tests; a fixture that asserted the
// answer was the top row would turn every test built on it into a second, worse
// witness for somebody else's ordering — which is exactly what happened, and
// `cmd/seam` reported it as four ranked tests going red under a mutation to
// `memory/rank.go`. Everything below finds the caret's rows through [anchors],
// which is where the renderer says it put them.
func readingRanked(t *testing.T) Model {
	t.Helper()

	m := judgedTalk(New(), 12)
	m.utter(memory.Handle{Ref: "ollama/llama3", Display: "coordinator-7"},
		memory.Utterance{Text: longAnswer})
	m.vote(memory.Up)
	m = press(m, "ctrl+t")

	if list := m.list(); len(list) < 2 || !slices.Contains(list, m.caret()) {
		t.Fatalf("the caret is not on one of the %d rows this list draws, so this is not the fixture it is named for", len(list))
	}
	return m
}

// rankRows is the ranked surface as a terminal with no colour shows it, with the
// anchors the renderer counted while drawing it — which is the only honest way to
// say where the caret's block ends, a block being however many rows its sentence
// needed.
func rankRows(m Model, width int) ([]string, anchors) {
	f := m.frame()
	f.width = width
	body, at := ranked(f)
	return strings.Split(ansi.Strip(body), "\n"), at
}

// The caret's row shows every word the record holds, and every other row is still
// cut. Both halves, for [TestTheCaretsRowShowsEveryWordAndEveryOtherRowIsCut]'s
// reason: either alone is satisfied by something useless.
//
// Down to twenty columns, which is where this surface's own floor is and is not
// the transcript's. The question this work opened with was whether to open a row
// at all at the narrowest widths, and the answer has two halves: the transcript's
// gate is a floor on the column *it* wraps into, and the matching column here is
// the preview, which the block does not wrap into — so inheriting that gate would
// have left this screen cut at sizes where it reads perfectly. The block wraps
// into the terminal, and that width has a floor of its own, which is
// [TestARankedBlockRefusesToOpenIntoAColumnTooNarrowForIt].
func TestTheRankedCaretsRowShowsEveryWordAndEveryOtherRowIsCut(t *testing.T) {
	m := readingRanked(t)
	want := strings.Fields(longAnswer)

	for _, width := range []int{200, 100, 80, 60, 40, 30, 20} {
		rows, at := rankRows(m, width)
		if at.mark < 0 || at.rows < 2 {
			t.Fatalf("width %d: the caret's row drew %d row(s) at %d, so nothing is open",
				width, at.rows, at.mark)
		}

		// The gutter comes off each line before the words are read, because it is
		// the frame around the quote rather than part of it.
		var said []string
		for _, r := range rows[at.mark+1 : at.mark+at.rows] {
			said = append(said, strings.TrimPrefix(r, gutter))
		}
		if got := strings.Fields(strings.Join(said, " ")); !slices.Equal(got, want) {
			t.Errorf("the block under the caret at width %d reads %q, want every word of the answer",
				width, strings.Join(got, " "))
		}

		// The row it opened from keeps its columns and gives up its preview, which
		// the block below repeats at full width.
		if strings.Contains(rows[at.mark], "the three") {
			t.Errorf("width %d: the reference row still carries the preview the block repeats: %q",
				width, rows[at.mark])
		}

		// And exactly one row on the screen opened. A surface that wrapped every row
		// would satisfy everything above it and cost the list its shape — thirteen
		// entries at six rows each is not a list anybody can rank by eye.
		opened := 0
		for _, r := range rows {
			if strings.HasPrefix(r, gutter+strings.Repeat(" ", colGap)) {
				opened++
			}
		}
		if opened != at.rows-1 {
			t.Errorf("width %d: %d rows on the screen are quoted continuations and the caret's block is %d — some other row has opened",
				width, opened, at.rows-1)
		}
	}
}

// The block refuses to open into a column too narrow to carry prose, and the row
// falls back to being cut.
//
// This is the floor the first version of this surface did not have, and the
// argument for having none was half right in a way worth keeping written down: it
// is correct that [transcript]'s gate must not be inherited, because that one is a
// floor on the column a transcript row wraps *into*, and the matching column here
// is the preview — which is not what the block wraps into. It does not follow that
// nothing has a floor. The block has a wrap width of its own, and a wrap width
// with no floor under it shreds.
//
// Measured, before the floor existed, on this fixture: at ten columns the block
// wrapped into six and broke ordinary words across rows, and at four it was 223
// rows of "│  …" — every character clipped away and the answer completely
// invisible, against eight rows and a visible ellipsis for not opening at all.
// Falling back to the cut row is degrading; that was losing the material.
//
// Both directions, so that lowering the floor is as loud as raising it.
func TestARankedBlockRefusesToOpenIntoAColumnTooNarrowForIt(t *testing.T) {
	// Read off the sweep below rather than computed from the constants, which is
	// this file's standing rule about floors. It happens to be textFloor plus the
	// gutter and its inset; that is where it comes from and not what it is.
	const floor = 20

	m := readingRanked(t)
	for width := 1; width <= 120; width++ {
		rows, at := rankRows(m, width)

		if got, want := at.rows > 1, width >= floor; got != want {
			verb := "is cut"
			if got {
				verb = "opens below the width where the block stops being wide enough to read"
			}
			t.Fatalf("width %d: the caret's row %s (floor %d), drawing %d rows in a list of %d:\n%s",
				width, verb, floor, at.rows, len(rows), strings.Join(rows[:min(len(rows), 6)], "\n"))
		}

		if width >= floor {
			continue
		}

		// What it falls back to has to be visibly a cut rather than a row that
		// happened to end there — the whole surface's one rule about its own edges.
		if !strings.Contains(rows[at.mark], "…") {
			t.Errorf("width %d: the row below the floor carries no cut of any kind: %q", width, rows[at.mark])
		}

		// And nothing else on the screen opened either, which is what makes the
		// fallback a list of eight rows rather than a screen of two hundred.
		for i, r := range rows {
			if strings.HasPrefix(r, gutter+strings.Repeat(" ", colGap)) {
				t.Errorf("width %d: row %d is a quoted continuation below the floor: %q", width, i+1, r)
			}
		}
	}
}

// The block is quoted in the gutter and carries nothing that would let it be
// counted as an entry.
//
// That is the whole of what keeps a list a list while one of its rows is open: the
// gutter says the material is still quoted out of the record, the inset says it
// belongs to the row above, and the absence of an ordinal says it is not a row of
// its own. Every one of those is alignment rather than colour.
func TestARankedBlockIsQuotedInTheGutterAndCountsForNothing(t *testing.T) {
	m := readingRanked(t)

	for _, width := range []int{100, 60, 40, 24} {
		rows, at := rankRows(m, width)
		for i, r := range rows[at.mark+1 : at.mark+at.rows] {
			if !strings.HasPrefix(r, gutter) {
				t.Fatalf("width %d: block row %d does not open with the gutter: %q", width, i+1, r)
			}

			rest := strings.TrimPrefix(r, gutter)
			if inset := lipgloss.Width(rest) - lipgloss.Width(strings.TrimLeft(rest, " ")); inset != colGap {
				t.Errorf("width %d: block row %d is inset %d columns past the gutter, want %d — the inset is what says it belongs to the row above rather than being a row of its own",
					width, i+1, inset, colGap)
			}
		}

		// The block is wrapped to the terminal and not to the column the preview
		// lives in, which is the decision this shape exists to make: every line is
		// full enough that the next word would not have fitted on it.
		//
		// That is the mechanical form of "it takes the width of the screen". Wrapped
		// to the row's own text column instead — the transcript's shape, moved here —
		// every one of these lines would have room to spare and this is what says so.
		block := rows[at.mark+1 : at.mark+at.rows]
		for i := 0; i < len(block)-1; i++ {
			next, _, _ := strings.Cut(strings.TrimSpace(strings.TrimPrefix(block[i+1], gutter)), " ")
			if got := lipgloss.Width(block[i]) + 1 + lipgloss.Width(next); got <= width {
				t.Errorf("width %d: block row %d ends at column %d and the next word would have fitted at %d — the block is wrapped to something narrower than the terminal:\n%s",
					width, i+1, lipgloss.Width(block[i]), got, strings.Join(block, "\n"))
			}
		}

		// And the row the block hangs from does not carry the padding of a column it
		// is no longer spending: a run of trailing spaces is a bar on a terminal with
		// a themed background.
		if r := rows[at.mark]; r != strings.TrimRight(r, " ") {
			t.Errorf("width %d: the row the block hangs from ends in %d trailing spaces: %q",
				width, lipgloss.Width(r)-lipgloss.Width(strings.TrimRight(r, " ")), r)
		}
	}
}

// A row opens exactly when its own row cannot hold the message, and the same bit
// answers both ways as the terminal changes.
//
// Both directions, on one message, because the condition is the whole design and
// each direction fails differently. Opening a row that already says everything
// spends a screen on a keystroke nobody pressed — the caret is not a mode, and on
// most rows of most lists it changes nothing at all. Not opening one that is cut
// leaves the surface broken for the ordinary case while it looks fixed for the
// dramatic one: a forty-character line at forty columns shows sixteen of them, and
// it is nobody's idea of a long answer.
func TestARankedRowOpensExactlyWhenItsOwnRowCannotHoldTheMessage(t *testing.T) {
	m := readingRanked(t)
	before, was := rankRows(m, 100)

	// Walked to one of judgedTalk's own ordinary lines — long enough that a narrow
	// row cannot hold it, short enough that the screen can. Walked rather than
	// stepped once, because which row is next is [memory.View.Rank]'s business and
	// not this test's; what this needs is any row in that band.
	var text string
	for range len(m.list()) {
		m = press(m, "down")
		b, _ := m.store.Get(m.caret())
		if n := lipgloss.Width(oneLine(b)); n >= 30 && n <= 50 {
			text = oneLine(b)
			break
		}
	}
	if text == "" {
		t.Fatal("no row in the list carries an ordinary-length line, so the band this test is about is not in the fixture")
	}
	after, at := rankRows(m, 100)

	if at.rows != 1 {
		t.Fatalf("at a hundred columns the caret's row drew %d rows for a message that fits it", at.rows)
	}
	if want := len(before) - (was.rows - 1); len(after) != want {
		t.Errorf("the list is %d rows with the caret on a short message and %d rows want — the block did not close cleanly",
			len(after), want)
	}
	if head := strings.Join(strings.Fields(text)[:3], " "); !strings.Contains(after[at.mark], head) {
		t.Errorf("a row that opens nothing has lost its own text: want it to carry %q, got %q",
			head, after[at.mark])
	}

	// The same bit, the same caret, a narrower terminal. Now its own row cannot
	// hold it and the block is how it is read — at a width where the message still
	// fits the *screen* easily and only the row is too small for it. That gap is
	// the case a condition written against the terminal rather than against the row
	// would miss, and it is the ordinary case rather than the dramatic one.
	narrow, at := rankRows(m, 60)
	if at.rows < 2 {
		t.Fatalf("at sixty columns the same message drew %d row(s), and its own row holds a fraction of it:\n%s",
			at.rows, strings.Join(narrow, "\n"))
	}

	var said []string
	for _, r := range narrow[at.mark+1 : at.mark+at.rows] {
		said = append(said, strings.TrimPrefix(r, gutter))
	}
	if got := strings.Fields(strings.Join(said, " ")); !slices.Equal(got, strings.Fields(text)) {
		t.Errorf("the block at sixty columns reads %q, want every word of %q", strings.Join(got, " "), text)
	}
}

// The anchors name the rows they were drawn on, on this surface too.
//
// It is [TestTheAnchorsNameTheRowsTheyWereDrawnOn] over the second surface and it
// is a separate check rather than a second fixture for the same one: these are two
// renderers, each with its own loop and its own bare increment, and the scar this
// one can draw is a voted fold rather than a fold in the view. Delete the count
// inside either loop and ctrl+u scrolls into the middle of somebody's paragraph.
func TestTheRankedAnchorsNameTheRowsTheyWereDrawnOn(t *testing.T) {
	const width = 60

	// A judged scar first, so it is in the list and below the answer: the caret's
	// block has to sit between the top of the list and the scar for an uncounted
	// row to move it.
	m := judgedTalk(New(), 12)
	m = back(m, 6)
	for range fixtureBudget {
		m.say(localHandle, "carrying on")
	}
	if !m.cold(m.mark) {
		t.Fatal("the caret did not follow its bit onto a scar, so there is no scar to vote on")
	}

	// Let go of the scar and keep the answer, so the two land in different bands
	// and the scar is below the block whatever the order inside a band turns out to
	// be. The tier is [memory.View.Rank]'s own claim and the tiebreak is a different
	// one; a fixture that leaned on the tiebreak made these tests a second witness
	// for it, which `cmd/seam` reports as over-red rather than as a pass.
	m.vote(memory.Down)

	// The caret is on the scar, so it does not ride what arrives next: it is walked
	// onto the answer, which move clamps at the end of the view.
	m.utter(memory.Handle{Ref: "ollama/llama3", Display: "coordinator-7"},
		memory.Utterance{Text: longAnswer})
	m.move(len(m.shown))
	m.vote(memory.Up)
	m = press(m, "ctrl+t")

	rows, at := rankRows(m, width)

	var scars []int
	for i, r := range rows {
		if strings.HasPrefix(r, "─") || strings.HasPrefix(r, caret+"─") {
			scars = append(scars, i)
		}
	}

	switch {
	case at.rows < 2:
		t.Fatalf("the caret's row drew %d row(s), so no block row is being counted:\n%s",
			at.rows, strings.Join(rows, "\n"))
	case len(scars) != 1:
		t.Fatalf("the list draws %d scars, want exactly one:\n%s", len(scars), strings.Join(rows, "\n"))
	case scars[0] <= at.mark+at.rows-1:
		t.Fatalf("the scar is on row %d and the caret's block ends on row %d, so an uncounted row would not move it:\n%s",
			scars[0], at.mark+at.rows-1, strings.Join(rows, "\n"))
	}

	if at.scar != scars[0] {
		t.Errorf("anchors.scar is %d and the scar is drawn on row %d — %d rows into the caret's own answer:\n%s",
			at.scar, scars[0], scars[0]-at.scar, strings.Join(rows, "\n"))
	}
	if !strings.HasPrefix(rows[at.mark], caret) {
		t.Errorf("anchors.mark is %d and that row does not carry the caret: %q", at.mark, rows[at.mark])
	}
}

// Nothing on this surface runs past the width it was given, and anything cut says
// so. It is [TestNoRowRunsPastTheWidthItWasGiven] over the second surface: the
// column arithmetic is supposed to make [clip] a no-op, and below about fifteen
// columns the fixed parts already outrun the terminal, so what is actually being
// held is that the backstop is there.
func TestNoRankedRowRunsPastTheWidthItWasGiven(t *testing.T) {
	for _, m := range []Model{
		press(judgedTalk(sized(100, 30), 24), "ctrl+t"),
		press(talk(sized(100, 30), 9), "ctrl+t"), // nothing judged: the empty state
		readingRanked(t),                         // a block open, whose rows are built elsewhere
	} {
		for _, width := range []int{200, 100, 80, 40, 24, 20, 16, 12, 8, 4, 1} {
			for i, row := range rankShot(m, width) {
				if w := lipgloss.Width(row); w > width {
					t.Errorf("ranked at width %d: row %d is %d wide: %q", width, i+1, w, row)
				}
			}
		}
	}
}
