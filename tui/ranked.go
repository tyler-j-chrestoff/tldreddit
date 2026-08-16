package tui

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/tyler-j-chrestoff/tldreddit/memory"
)

// judged is the view the ranked reading is taken over: everything anybody has
// said on this record, newest first, plus anything else somebody at this keyboard
// voted on, each named once.
//
// Over the record rather than over the view, and that is the whole difference
// between retrieval and theatre. Ranking [Model.shown] would reorder the rows
// already on screen; most of the bits in here are behind a scar and cannot be
// reached from the transcript at all without following one. It is also what makes
// this list stable when a fold fires — the fold changes what the transcript shows
// and changes nothing about what was said.
//
// # It used to be the voted bits alone, and that filter hid the answer to the
// question this screen is for
//
// The old rule was one line — a bit is here exactly when somebody voted on it —
// and the argument for it was that with one human's ±1 as the only signal, a list
// of everything is mostly rows placed by the tiebreak, which is time, which is the
// transcript with the rows shuffled. That is still true and it is not a reason to
// hide the rows. Measured on this project's own record: 35 bits, 29 of them said,
// three votes. The filter drew `ranked 3 · record 35` — three rows and twenty
// blank ones, a claim later found unsourced sitting at rank 1, and the correction
// to it **absent from the screen entirely**, because nobody had voted on the
// correction. `tldr top`, over the same record, printed the correction and said
// `kept 3 · not judged 26`.
//
// So the surface was hiding material behind the very act it exists to collect,
// and the non-interactive verb built later had already got it right. The honest
// answer to a thin signal is not a shorter list, it is a list that says how much
// of its own order a person decided — which this screen already draws, once per
// band, in [band]'s own headings.
//
// # What is in it, and the two exclusions
//
// Everything said, which is narrower than everything recorded. A ballot is not a
// row: it is one participant's judgment *about* a row, and it reaches this reading
// already, as that row's own mark. A [memory.Compaction] is not a row either — it
// is what a view did to fit on a screen, and every bit it stands for is in here on
// its own account, which is the point rather than the cost. That is the same pair
// of exclusions `cmd/tldr`'s own reading makes, for the same two reasons.
//
// **Except a scar somebody voted on**, which stays. The surface permits a vote on
// a fold — [Model.fold] moves the caret onto the absorbing scar, and the vote key
// casts on whatever the caret names — so dropping scars outright would take a
// standing ballot off the only screen that shows ballots, and would take the
// caret's rescue with it. `tldr top` has no caret and names that gap in its own
// header instead; this screen keeps the row.
//
// Newest first, because that is the order ties fall in. [memory.View.Rank] sorts
// stably and never reads a clock, so the order handed in here is the order equal
// scores come back in — the tiebreak belongs to the caller, expressed as the view
// it passes. Ties in the instant itself keep [memory.Store.All]'s own order, which
// settles nothing anybody would defend to a reader and is there so that two draws
// of one record put the rows in one order.
//
// It does not discharge D3, and widening it does not bring that any closer. It
// ranks what one person marked and lists what everyone said; a bit nobody has
// judged is placed by the clock, and the band heading above it says so in words.
func (m Model) judged() memory.View {
	type row struct {
		id string
		at time.Time
	}

	said := make(map[string]bool, m.store.Len())
	rows := make([]row, 0, m.store.Len())
	for b := range m.store.All() {
		if _, ok := b.Payload.(memory.Utterance); !ok {
			continue
		}
		said[b.ID] = true
		rows = append(rows, row{id: b.ID, at: b.At})
	}

	seen := map[string]bool{}
	for _, v := range m.votes.Bits(m.store) {
		if len(v.Prev) != 1 {
			// A vote with any other arity is diagnosed by memory's own traversal,
			// which names the bit and says how many parents it has.
			// [memory.View.Rank] runs that traversal over this same view a moment
			// later, so skipping here leaves that message as the one the reader
			// gets rather than an index panic in front of it.
			continue
		}

		id := v.Prev[0]
		if said[id] || seen[id] {
			continue
		}
		seen[id] = true

		// A target the store does not hold is kept rather than dropped, for
		// [recall]'s reason: under an append-only store it cannot happen, so it is
		// the failure D1 exists to rule out and it should arrive as a row naming
		// the address rather than as a row that quietly is not there. The zero
		// instant sorts it last.
		b, _ := m.store.Get(id)
		rows = append(rows, row{id: id, at: b.At})
	}

	slices.SortStableFunc(rows, func(a, b row) int { return b.at.Compare(a.at) })

	out := make(memory.View, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.id)
	}
	return out
}

// ranking is the ranked reading this surface draws: [Model.judged], ordered by
// what the human at this keyboard said about each bit.
//
// The tiebreak is not passed to [memory.View.Rank] and could not be. Rank sorts
// stably over the view it is handed, so ties come back in the order they went in
// — which makes the tiebreak the caller's, expressed as the order of the view,
// and [Model.judged] is where this one is decided.
func (m Model) ranking() []memory.Ranked {
	return m.judged().Rank(m.store, m.votes, localHandle)
}

// stands is the address in list that stands for id: id itself when the list
// holds it, and otherwise the scar that absorbed it.
//
// It is the caret's rescue, at draw time rather than at fold time, and the
// difference is the price of one caret shared by two surfaces. [Model.fold]
// re-attaches the caret to the absorbing scar as it folds, which is enough while
// the transcript is the only screen; a caret that was parked in the ranked view
// while a fold happened has had no such moment, and the transcript then draws no
// caret at all with nothing on screen saying so. Every act that asks where the
// caret is goes through here, so what a key does is what the screen shows.
//
// Both edges are checked because they answer different generations. Prev names
// the bits the fold took, which includes an older scar folded again (D13);
// Absorbed names the originals, flattened across however many generations, which
// is how a bit absorbed two folds ago is still found. Between them there is no
// depth to recurse into.
//
// An address nothing stands for comes back unchanged rather than empty. That is
// the same answer the surface gave before this existed — no caret drawn — and it
// keeps the failure loud in the one place it can be seen rather than converting
// it into an empty mark that every caller would then have to test for.
func stands(s *memory.Store, list memory.View, id string) string {
	if id == "" || slices.Contains(list, id) {
		return id
	}
	for _, b := range list.Bits(s) {
		c, cold := b.Payload.(memory.Compaction)
		if !cold {
			continue
		}
		if slices.Contains(b.Prev, id) {
			return b.ID
		}
		if slices.Contains(slices.Collect(c.Absorbed()), id) {
			return b.ID
		}
	}
	return id
}

// ranked draws the second surface: the judged bits in the order the votes put
// them in, and reports where the caret and the first scar ended up.
//
// It is the receipt's row vocabulary rather than the transcript's, because it is
// the same kind of material — quoted out of the record rather than present in
// the view — and the gutter already says exactly that in [unfold]'s own words.
// So every row opens with a gutter, carries its place in the count, its address,
// its clock and the reader's own mark, and the handle column is aligned the way a
// receipt aligns it.
//
// A clock on every row is not a decoration and not a label. In the transcript
// position *is* time, which is why hot rows carry none; the moment position stops
// encoding time every row has to state its own. That is the strongest thing
// saying this is not the transcript, because it is a consequence of the
// reordering rather than a sign put up to announce it.
//
// The drop ladder is the receipt's, inverted, and the inversion is the argument
// rather than an arrangement: [unfold] sheds the clock first because "the span on
// the scar above still brackets it, and the rows are in the order they happened",
// and neither half of that is true here. So the address goes first and the clock
// stays. The drain goes before either, because a hold is machinery about the
// transcript and this is not the transcript.
//
// There is no fade here and its columns are not spent on anything else. Most rows
// in this list have already been folded, so "about to go" is a fact whose moment
// has passed for them, and a two-state signal that is off for the majority
// because it already happened would read as safe. The one channel that survives a
// terminal with no colour may not carry a second meaning on a second screen.
//
// One row per judged bit, except the caret's, which shows its whole message —
// quoted underneath itself when it does not fit, rather than wrapped in place.
// The reason that shape differs from the transcript's is in the block below and
// in this package's doc. Nothing opens when the message already fits, so on most
// rows of most lists the caret changes nothing at all.
func ranked(f frame) (string, anchors) {
	width := f.width
	if len(f.order) == 0 {
		// This list is empty exactly when nobody has said anything, because it is
		// the record's own said bits and not a filter over them. So the sentence
		// names that rather than naming a verdict: telling somebody to vote on an
		// empty record would be a screen asking for a keystroke that cannot be
		// pressed. An empty list on a surface reached by one key is otherwise
		// indistinguishable from a surface that is broken.
		return dim.Render(fit(width,
			"nothing on the record yet — everything said turns up here, best first",
			"nothing on the record yet — everything said turns up here",
			"nothing on the record yet",
			"nothing yet")), anchors{mark: -1, scar: -1}
	}

	type row struct {
		memory.Ranked
		bit   memory.Bit
		found bool
		scar  bool
	}

	rows := make([]row, 0, len(f.order))
	names := make([]string, 0, len(f.order))
	stamps := make([]time.Time, 0, len(f.order))
	for _, r := range f.order {
		b, ok := f.store.Get(r.ID)
		_, cold := b.Payload.(memory.Compaction)
		rows = append(rows, row{Ranked: r, bit: b, found: ok, scar: cold})
		if !ok {
			continue
		}
		stamps = append(stamps, b.At)
		if !cold {
			// A scar has no speaker, so it contributes no name. Sizing the column
			// off a fold's blank would be sizing it off nothing.
			names = append(names, b.From.Display)
		}
	}

	digits := len(strconv.Itoa(len(rows)))
	idxWidth := 2*digits + 1
	timeWidth := f.clock.widestStamp(stamps...)

	// The vote column costs columns and is spent only where it says something,
	// which is [frame.voted]'s question asked over this list rather than over the
	// transcript's bits. It used to be unconditional, on the ground that a bit was
	// in this list exactly when somebody had voted on it; the list is now every bit
	// anybody said, so a record nobody has judged draws a blank column on every row
	// and gets it back for the words instead.
	//
	// What is in question after that is the drain, and it is drawn only where a
	// hold is live: see [Model.live], which is where a hold that no longer holds
	// anything back is dropped rather than drawn.
	vote := 0
	for _, r := range rows {
		if f.standing(r.ID) != 0 {
			vote = markWidth + 1
			break
		}
	}
	if vote > 0 {
		for _, r := range rows {
			if _, held := f.holds[r.ID]; held {
				vote = markWidth + drainWidth + 1
				break
			}
		}
	}

	addr := true
	lead := func() int {
		n := lipgloss.Width(gutter) + idxWidth + colGap + timeWidth + colGap + vote
		if addr {
			n += addrWidth + colGap
		}
		return n
	}
	if width-lead()-widest(names)-colGap < textFloor {
		// min rather than an assignment: a list nobody has voted in spends nothing
		// here at any width, and narrowing a column that is already zero would
		// widen it — the ladder giving a row back a column it never took.
		vote = min(vote, markWidth+1)
	}
	if width-lead()-widest(names)-colGap < textFloor {
		addr = false
	}
	name := nameColumn(names, width-lead())
	text := max(width-lead()-name-colGap, 1)

	var out strings.Builder
	line, at := 0, anchors{mark: -1, scar: -1}
	own := 2 // no band yet: not +1, 0 or -1
	for i, r := range rows {
		if r.Own != own {
			own = r.Own
			n := 0
			for _, s := range rows[i:] {
				if s.Own != own {
					break
				}
				n++
			}
			fmt.Fprintln(&out, clip(band(own, n), width))
			line++
		}
		// One row, until the block below says otherwise: a row that opens overwrites
		// this with its own height, and every row that does not is one row.
		//
		// It is load-bearing rather than bookkeeping, which is worth saying because
		// it reads like bookkeeping and was reviewed as inert. [Model.sync] frames
		// the caret's *block*, and a block of no rows is nothing to frame — so a
		// zero here is a surface that silently stops scrolling to its own caret, and
		// the next vote lands on a row nobody can see. That is the failure this
		// screen was found committing once already, at three of four sizes.
		// [TestTheRankedCaretIsInsideTheFrame] is what notices.
		if r.ID == f.mark {
			at.mark, at.rows = line, 1
		}
		if r.scar && at.scar < 0 {
			at.scar = line
		}

		// Built whole and cut once at the end, which is [clip]'s job on every
		// surface here: the column arithmetic above is meant to make that a no-op
		// and above about twenty columns it is, but below that the fixed parts
		// already outrun the terminal and something has to say so. Written without
		// it first, and measured: at twenty columns every row of this list was
		// twenty-four wide, running under the frame's right edge with the viewport
		// clipping it unmarked.
		row := gutterCell(r.ID == f.mark, r.scar) +
			column(rule, fmt.Sprintf("%*d/%d", digits, i+1, len(rows)), idxWidth)

		if !r.found {
			room := max(width-lipgloss.Width(gutter)-idxWidth-colGap, 1)
			fmt.Fprintln(&out, clip(row+unresolved(r.ID, room), width))
			line++
			continue
		}

		if addr {
			row += column(rule, memory.Short(r.ID), addrWidth)
		}
		row += column(rule, f.clock.stamp(r.bit.At), timeWidth)
		row += voteCell(f, r.ID, vote, hot, false)
		if r.scar {
			// A fold has no speaker and the column is left empty rather than
			// filled with something standing in for one. The gap is legible
			// because every other row lines up either side of it, and the count
			// this row carries in its text column is the receipt saying what it is.
			row += strings.Repeat(" ", name+colGap)
		} else {
			row += column(speaker, r.bit.From.Display, name)
		}

		// The caret's row shows its whole message, and every other row is one line
		// cut at the margin. Same rule as the transcript and a different shape,
		// because these rows are a different kind of thing — see the block below.
		//
		// Two conditions, and they are about two different columns. Nothing opens
		// when the message already fits *its own row*: the row is what it always
		// was, and that half is asked by wrapping rather than by measuring, so the
		// answer is the one [saidWhole] will give when it draws. And nothing opens
		// when the block itself would be too narrow to carry prose.
		//
		// That second one is a floor, and this surface refusing to inherit the
		// transcript's floor is not the same thing as it having none — a distinction
		// this file got wrong in one direction before getting it wrong in the other.
		// [transcript]'s gate is textFloor applied to *room*, the column a row wraps
		// into there, and applying it to [text] here would be a floor on the preview
		// column, which is not what the block wraps into and is why it was refused.
		// The block wraps into width-hang, and that is a wrap width like any other,
		// so it gets the same floor every wrap width on this surface gets.
		//
		// What it prevents was measured rather than imagined. Below it the block
		// shreds: at a terminal ten columns wide it wraps into six and breaks
		// ordinary words across rows, and at four the whole block is 223 rows of
		// "│  …" — every character clipped away, the answer completely invisible,
		// where the cut row it replaced at least showed an ellipsis with its
		// neighbours beside it. Falling back to the cut row is degrading; that was
		// losing the material. [TestARankedBlockRefusesToOpenIntoAColumnTooNarrowForIt]
		// holds the floor in both directions.
		hang := lipgloss.Width(gutter) + colGap
		if r.ID != f.mark || width-hang < textFloor || len(saidWhole(f, r.bit, text)) == 1 {
			fmt.Fprintln(&out, clip(row+said(f, r.bit, text), width))
			line++
			continue
		}

		// A message too long for its row is quoted underneath it, in the gutter, at
		// the width of the terminal rather than the width of the text column.
		//
		// This is where this surface parts from the transcript, and the reason is
		// its own columns rather than taste. A transcript row *is* a sentence: the
		// lead is a margin and a handle, so wrapping in place costs nothing and the
		// blank handle column underneath is what says "still the same speaker". A row
		// here is a *reference* — an ordinal, an address, a clock, a mark, a handle,
		// and a preview of the sentence at the end. Measured on a hundred-column
		// frame those references are forty-four columns; wrapping in place would
		// repeat forty-four blanks on every line of the answer, and at forty columns
		// it is twenty-four of them — sixty per cent of the terminal spent on
		// nothing, nineteen rows deep.
		//
		// So the reference stays a reference and keeps every column it had, and the
		// material appears below it. That is not a new idea on this screen: it is
		// exactly what [unfold] does under a scar — the row stands, and what it
		// stands for is quoted beneath it in this same gutter. One row instead of
		// many, and the same two glyphs.
		//
		// The preview goes while the block is open, because it would otherwise say
		// the first few words twice, and the block below it says them at full width.
		//
		// Two columns of inset past the gutter, which is what makes it read as
		// belonging to the row above rather than as a row of its own: a receipt's
		// own lines start against the gutter and carry an ordinal, and these carry
		// nothing and must not be counted. hang is declared with the floor above,
		// because the floor is a statement about the width this leaves.
		//
		// No clamp on the width handed over: the floor above is stronger than any
		// clamp would be, and [saidWhole] clamps again anyway. A max() here would be
		// a third guard that no mutation could reach, which is the thing this file
		// keeps having to disclose rather than justify.
		whole := saidWhole(f, r.bit, width-hang)
		at.rows = 1 + len(whole)

		// Trailing blanks trimmed rather than drawn: the columns after the handle
		// are padding this row is no longer spending on anything, and a run of
		// spaces at the end of a line is a bar on somebody's themed terminal.
		fmt.Fprintln(&out, clip(strings.TrimRight(row, " "), width))
		line++
		for _, l := range whole {
			fmt.Fprintln(&out, clip(rule.Render(gutter)+
				strings.Repeat(" ", hang-lipgloss.Width(gutter))+l, width))
			line++
		}
	}
	return strings.TrimRight(out.String(), "\n"), at
}

// band is the heading over a run of rows the reader said the same thing about,
// and the count of them.
//
// The words are the footer's own words for the two keys, so there is nothing to
// teach: what you pressed is what the heading says. They sit at column 0 with no
// gutter, so they read as headings by indentation alone rather than by colour.
//
// A heading scrolling off the top costs nothing, which is why there is no sticky
// version of it: every row states its own band in the mark it already carries —
// a solid or hollow caret for kept, an inverted one for let go — so the heading
// is an aid and the row is the record.
//
// The third band is the ordinary one now, and on most records it is nearly the
// whole list. It reads `not judged · 26` over the rows the clock placed rather
// than the reader, which is the single most load-bearing thing on this screen:
// without it a ranked list of everything is indistinguishable from a ranking that
// worked, and with it a person can count the rows a person actually decided. It
// used to be unreachable — a bit reached this list by being voted on, so the
// reader's own standing vote was never nothing — and it was drawn anyway on the
// ground that a run with no heading over it reads as belonging to the band above.
// [Model.judged] is what made it live.
func band(own, n int) string {
	word := "not judged"
	switch {
	case own > 0:
		word = "kept"
	case own < 0:
		word = "let go"
	}
	return fmt.Sprintf("%s · %d", word, n)
}

// gutterCell is the two columns in front of every ranked row: the gutter that
// says this material is quoted from the record, the caret when it is here, and
// the scar's own rule when the row is a fold.
//
// It is [caretCell]'s three glyphs in the same three meanings, and deliberately
// not [caretCell] itself. The transcript spends these columns on the fade as
// well — a row the next fold takes steps into them — and there is no fade on this
// surface, so the geometry is fixed where the transcript's varies. One function
// serving both would have to be told which surface it is on, which is a
// parameter meaning "ignore half of this".
func gutterCell(marked, scar bool) string {
	switch {
	case marked && scar:
		return hot.Render(caret) + seamInk.Render("─")
	case marked:
		return hot.Render(caret) + " "
	case scar:
		return seamInk.Render("─ ")
	}
	return seamInk.Render(gutter)
}
