package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/tyler-j-chrestoff/tldreddit/memory"
)

// Column widths for a retrieved row. Time is dropped first, then the address;
// the handle then shrinks; the text takes whatever is left. Text is last because
// it is the only column a person reads rather than checks, and the handle
// outlasts the machinery columns because provenance is the question this block
// exists to answer.
//
// The address outlasting the clock is a reversal, and it is the one change here
// that is an argument rather than an arrangement. The old ladder shed the
// content hash first, which put the auditor's own instrument at the front of the
// queue to be dropped: the hash is the only thing on the row that can be taken
// somewhere else and checked, and a row without one is a row you have to believe.
// A time can be recovered — the span on the scar above still brackets it, and the
// rows are in the order they happened — while an address cannot be recovered
// from anything else on the screen.
//
// There is no constant for the clock's width any more. It is measured from the
// material, like the handle column, because a stamp is five columns on a
// conversation inside one day and eleven on one that crossed midnight — and a
// column fixed at five would have cut the date off silently, which is the exact
// failure the handle column was measured to stop.
const (
	gutter    = "│ "
	addrWidth = 8
	nameFloor = 3
	textFloor = 16
	colGap    = 2

	// uncut is a width no row on any terminal reaches, for asking a width-taking
	// function what it would say with nothing in its way. It is not a layout
	// constant and nothing is ever drawn at it.
	uncut = 1 << 20

	// quoteFloor is the least room a scar will quote a bit into. Below it the
	// scar says how much went, over what span, and which key opens it, and says
	// nothing about the content — see [frame.quotation] for why that is better
	// than a shorter quotation. It is measured rather than chosen: the frames it
	// was read off are printed by [TestHarnessScar], and the widths where the
	// quotation appears and disappears are pinned by
	// [TestAScarQuotesAWholeBitOrNothingAtAll].
	quoteFloor = 14
)

// retrieved is one entry of a receipt, looked up at the moment of drawing.
//
// found is false when the address does not resolve. Under an append-only store
// that cannot happen, which is exactly why it is carried rather than dropped: a
// receipt that stopped resolving is the failure D1 exists to rule out, and it
// should arrive as a line on the screen naming the missing address, not as a
// row that quietly is not there.
type retrieved struct {
	id    string
	bit   memory.Bit
	found bool
}

// recall follows a compaction's receipt into the store.
//
// It takes no view and returns no view. That absence is the claim the whole
// interaction makes: the material comes out of the record, and the record is
// the only thing consulted, so nothing here can put a bit back on screen in the
// sense of the view holding it again.
//
// A compaction's Absorbed lists originals only, flattened across however many
// generations of folding produced it, in the order they happened. So one call
// answers the whole question — there is no second level to descend into, and
// the number of entries is the number the scar advertises.
//
// Nothing is preallocated from Count. That reads as a free hint and is not one:
// a compaction can arrive from a persisted record written by another process,
// Count is a number in that stream, and sizing a make from a number this
// program does not own is how a damaged file becomes `fatal error: out of
// memory` — a death with no panic, no defers and no receipt, in the package
// whose whole job is to draw what the record says. memory's decoder now refuses
// a compaction whose count disagrees with its receipt, which closes the same
// hole from the other side; both are wanted, because this rule holds whatever
// the decoder does and the decoder's rule holds whatever this does.
func recall(s *memory.Store, c memory.Compaction) []retrieved {
	var out []retrieved
	for id := range c.Absorbed() {
		b, ok := s.Get(id)
		out = append(out, retrieved{id: id, bit: b, found: ok})
	}
	return out
}

// unfold renders what a scar stands for: one row per absorbed bit, in the order
// they happened, inside a gutter that says the material is quoted from the
// record rather than present in the view.
//
// One row per bit, never wrapped, is what makes the receipt checkable. Every
// row carries its own place in the receipt — 3/21 — so the count can be checked
// from any single row that happens to be visible. That is the load-bearing
// change: a block of fourteen rows in a twelve-row terminal cannot show both of
// its ends, so a receipt whose only proof was a number at each end was a proof
// nobody past the first fold could reach. A number on every row is reachable
// from wherever the screen happens to be.
//
// Every row is built to the width it was given and cut with an ellipsis if the
// content will not fit. Nothing here may run past the margin: this viewport
// clips rather than wraps, and a row cut by the clip and a row that happened to
// end there look identical.
func unfold(f frame, c memory.Compaction) string {
	at, width := f.clock, f.width
	bits := recall(f.store, c)

	// Handles are aligned to the widest one actually present and are not cut to
	// a fixed column. Two agents whose names share a prefix — coordinator-7 and
	// coordinator-9 — would arrive at the same ten characters, and a fold that
	// merges two speakers into one string has destroyed the one thing the block
	// exists to preserve. Provenance has to survive a fold, and on a monochrome
	// terminal alignment is all that is left to carry it.
	names := make([]string, 0, len(bits))
	stamps := make([]time.Time, 0, len(bits))
	for _, r := range bits {
		if r.found {
			names = append(names, r.bit.From.Display)
			stamps = append(stamps, r.bit.At)
		}
	}
	timeWidth := at.widestStamp(stamps...)

	digits := len(strconv.Itoa(max(len(bits), 1)))
	idxWidth := 2*digits + 1

	// A vote survives the fold that took the bit it was cast on, so the receipt
	// is where it has to still be legible: the transcript cannot show it any
	// more, and the record has not forgotten it. Without this a person watches
	// their own mark disappear at the fold, on the one surface whose argument is
	// that nothing disappears.
	//
	// The column exists only when something in this receipt carries a vote, and
	// it is never dropped once it does. It is the reader's own act — the thing
	// they are most likely to be looking for — and the ladder below sheds
	// machinery before it sheds that.
	//
	// There is no gauge in here, and that is structural rather than a saving:
	// [memory.View.Fold] never absorbs a held bit, so nothing in a receipt can be
	// under a live hold. What a receipt shows is a downvote, or an upvote whose
	// hold ran out before the fold came.
	vote := 0
	for _, r := range bits {
		if r.found && f.standing(r.id) != 0 {
			vote = markWidth + 1
			break
		}
	}

	// The drop ladder, in order: the time goes, then the address, then the
	// handle shrinks, and the text takes whatever is left. Provenance before
	// content, and within provenance the thing that can be checked outlives the
	// thing that can be inferred — see the constants above for why that order is
	// the reverse of what it was.
	addr, when := true, true
	lead := func() int {
		n := lipgloss.Width(gutter) + idxWidth + colGap + vote
		if addr {
			n += addrWidth + colGap
		}
		if when {
			n += timeWidth + colGap
		}
		return n
	}

	if width-lead()-widest(names)-colGap < textFloor {
		when = false
	}
	if width-lead()-widest(names)-colGap < textFloor {
		addr = false
	}
	name := nameColumn(names, width-lead())
	text := max(width-lead()-name-colGap, 1)

	var out strings.Builder
	for i, r := range bits {
		// Padding sits outside the styles. A styled run of trailing spaces is
		// invisible under this palette and would not be under a theme that
		// gives any of these a background, and the block would grow bars nobody
		// asked for on somebody else's terminal.
		row := rule.Render(gutter)
		row += column(rule, fmt.Sprintf("%*d/%d", digits, i+1, len(bits)), idxWidth)

		if !r.found {
			room := max(width-lipgloss.Width(gutter)-idxWidth-colGap, 1)
			row += unresolved(r.id, room)
			fmt.Fprintln(&out, row)
			continue
		}

		if addr {
			row += column(rule, memory.Short(r.id), addrWidth)
		}
		if when {
			row += column(rule, at.stamp(r.bit.At), timeWidth)
		}
		row += voteCell(f, r.id, vote, dim, false)
		row += column(dim, r.bit.From.Display, name)
		row += dim.Render(said(f, r.bit, text))
		fmt.Fprintln(&out, row)
	}

	// The closing bar counts the rows that were drawn, not the number the scar
	// claims. Cool guarantees they agree, so this is normally the same number
	// twice — which is the point. It is no longer the only way to check that,
	// because the ordinal on each row carries the same total; it is the
	// terminator, and it says where the material came from, which no ordinal
	// can.
	out.WriteString(seamInk.Render(fit(width,
		fmt.Sprintf("└─ %d bits from the record ──", len(bits)),
		fmt.Sprintf("└─ %d bits ──", len(bits)))))
	return out.String()
}

// unresolved is the row an address the store does not hold gets, on whatever
// surface was drawing it.
//
// The one row whose whole purpose is to be seen, so it is built to the width
// budget the row had rather than to a fixed one. A failure notice that runs off
// the edge of the terminal is a failure notice nobody receives.
//
// The address goes before the alarm does. A row reading only "6c968f40" is
// indistinguishable from a bit that resolved and had nothing to say, which is
// the precise confusion this row exists to prevent — so the last word standing
// is the one that says it failed, not the one that says which.
//
// It is shared by the receipt and the ranked view because it is one claim: an
// address in a list came out of the record and the record no longer has it. Two
// wordings of that would be two failures as far as a reader is concerned.
func unresolved(id string, room int) string {
	short := memory.Short(id)
	return warm.Render(ansi.Truncate(fit(room,
		short+" does not resolve — the receipt outlived the record",
		short+" does not resolve",
		short+" unresolved",
		short+" gone",
		"unresolved",
		"gone",
	), room, "…"))
}

// unfinished is how a row says its speaker did not get to the end of the
// sentence, widest first. It is the mark for [memory.Utterance.Truncated].
//
// Two facts end a row short and they are not the same fact, so they must not
// look alike. "…" is this screen running out of columns: it says nothing about
// the speaker, and a wider terminal undoes it. This says the speaker stopped —
// a model that ran out of context room mid-answer — and no terminal is wide
// enough to undo that. A person who read one as the other would read a cut-off
// answer as a complete one, which is the whole reason
// [memory.Utterance.Truncated] reaches the content address.
//
// So it is a word rather than a glyph, and dashes rather than colour. The
// dashes are the vocabulary this surface already uses for something the harness
// is saying about the record rather than something a participant said: the
// pending line, the failure block and the held composer prompt are all dashed,
// and solid means settled and on the record. The word is what carries it on a
// monochrome terminal, which is the only kind of distinction worth resting this
// on — the fade is already invisible under a low colour profile, and
// harness_test.go cannot even see that happen.
//
// One word at every rung, and only the closing dash is traded for room. A
// second phrasing was drafted and cut: two rows of one receipt marked in
// different words read as two different facts, and they are the same fact.
// Below the last rung here [said] falls back to the dash alone, which is the
// floor and not a rung: there is no width at which [said] returns something
// unmarked.
//
// That is a property of [said] and not of a row, and the difference is worth
// stating precisely because the first version of this comment claimed the
// stronger thing and was wrong. said's output is the tail of a row that
// [transcript] and the receipt then hand to [clip], and clip cuts the tail —
// which is where the mark is.
//
// Both surfaces keep some mark down to a width, and the word down to a wider one,
// and below the first of each pair the glyph left standing is "…" — the exact
// substitution this mark exists to prevent. The four numbers are measured by
// rendering at every width from 1 and they are written down in exactly one place,
// [TestTheRowsMarkFloorsAreWhereTheyWereMeasured], which fails when any of them
// moves. They were repeated here for a while and this copy went stale by two
// generations without anything noticing, which is what a number in prose beside a
// number under test does.
//
// The two kinds of floor move for different reasons, and conflating them is how
// the pin on them was itself briefly wrong. The dash floor is set by the row's
// column arithmetic — gutter, ordinal, handle, gaps — and lengthening or
// shortening the rungs above does not move it, because at those widths the mark
// is already the bare dash. The word floor is what this ladder sets, and it
// moves with any edit to these strings.
// [TestTheRowsMarkFloorsAreWhereTheyWereMeasured] pins all four in both
// directions, so improving one is as loud as losing one and this paragraph
// cannot go stale quietly. Both mutations were run: widening the last rung by a
// single column moves the word floors and fails, and narrowing it fails too.
//
// There is no arrangement of a gutter, an ordinal, a three-column handle and a
// mark that fits in twelve columns, and clip's own comment already concedes that
// regime for the whole surface. What is not conceded is saying otherwise here.
//
// The mark is also not reserved: nothing stops a participant typing "╌
// unfinished ╌" into a message, and at some widths that row is byte-identical to
// a real fragment's. It is the mirror of the reason [recordReply] appends
// nothing to a participant's own words, running the other way, and it is the
// first place harness vocabulary shares a row with content rather than owning a
// row of its own. The tests do not rest on the string for that reason: what they
// assert is that a fragment never draws as its own finished twin, which is a
// difference no content can manufacture, since any forgery is present in both.
var unfinished = []string{
	"╌ unfinished ╌",
	"╌ unfinished",
}

// said is a bit's content as one row of the width it was given: whitespace
// collapsed, cut with an ellipsis if the terminal cannot hold it, and marked if
// its speaker did not finish.
//
// One definition for the transcript and the receipt alike, and the width is a
// parameter rather than the caller's business, so the two cannot drift on
// either half of the job. Truncating outside this function is what would let a
// caller cut a fragment's mark off the end of its own row.
//
// It takes the frame because a scar drawn as a row is not a fact about that bit
// alone: what a fold says about itself is a quotation of one of the bits it
// absorbed, and those live in the store beside it — see [frame.quoted]. Every
// caller here already holds a frame, which is [frame]'s own argument about
// asking one question of one reading rather than several of several.
func said(f frame, b memory.Bit, width int) string {
	width = max(width, 1)

	if c, cold := b.Payload.(memory.Compaction); cold {
		return scarLine(f, c, width)
	}
	text := oneLine(b)

	u, ok := b.Payload.(memory.Utterance)
	if !ok || !u.Truncated {
		return ansi.Truncate(text, width, "…")
	}

	// The gap holds the mark off the sentence, and there is no gap when there is
	// no sentence: a fragment with nothing in it should read as a speaker who
	// got nothing out, flush in its column rather than indented past it.
	//
	// That case does not arrive from ollama. [persona.Client.Reply] returns an
	// error rather than an Answer whenever the trimmed text is empty, truncated
	// or not, so [recordReply] never sees one — an earlier note here said it was
	// the common case on this setup, which was wrong in the direction that
	// matters, since the reasoning a model spends its allowance on is stripped
	// before this ever runs. It is reachable all the same: [Model.utter] takes
	// any Utterance, and a payload that draws as nothing at all is worth
	// deciding about before something starts writing them.
	gap := ""
	if text != "" {
		gap = " "
	}

	// Widest first, while the sentence still fits whole beside it. A longer
	// mark that pushed the text into an ellipsis would be spending a fact the
	// terminal can give back on one it already has.
	for _, mark := range unfinished {
		if width-lipgloss.Width(gap+mark) >= lipgloss.Width(text) {
			return text + gap + mark
		}
	}

	// The sentence is being cut whatever happens now, so the narrowest mark
	// that still says it in words wins and the columns it saves go to the text.
	// The direction is what matters: an ellipsis is this screen's own doing and
	// a wider terminal undoes it, while a row that lost its mark claims its
	// speaker finished and no width brings that back.
	narrow := unfinished[len(unfinished)-1]
	if room := width - lipgloss.Width(gap+narrow); room >= 1 {
		return ansi.Truncate(text, room, "…") + gap + narrow
	}

	// No room for both, so the mark goes on alone. It is never itself cut with
	// an ellipsis: a mark ending in "…" would read as the screen's own cut,
	// which is the one thing it may not be mistaken for. Below even that there
	// is a single dash, and there is no rung under it — a row with no mark is a
	// row claiming its speaker finished.
	if lipgloss.Width(narrow) <= width {
		return narrow
	}
	return clip("╌", width)
}

// unmarked splits a row [said] drew into the words their speaker got out and the
// mark this surface put after them.
//
// It exists for one caller and one problem. [frame.quotation] puts quotation
// marks around a bit, and quotation marks are an assertion that what is between
// them is exactly what somebody said — so `me "the three steps are, ╌ unfinished
// ╌"` is this screen's own vocabulary presented as a participant's words, which
// is the forgery [recordReply] refuses to commit from the other direction. The
// mark belongs outside the closing quote, where it says what it has always said:
// this surface reporting that the speaker stopped.
//
// Which rung of the ladder [said] took is read back off the row rather than
// restated here, because that choice is [said]'s and a second statement of it is
// the drift this pair exists to prevent. *Whether there is a mark at all* is not
// a fact about the ladder, though — it is a fact about the bit — and marked is
// how the caller says so.
//
// That parameter is the difference between exact and nearly exact, and the
// nearly was found by measurement rather than by reading. Without it this
// function matched a suffix and nothing else, so a participant who typed the
// glyph got their own characters taken off them and re-attributed to the
// program: a bit reading `all done ╌ unfinished ╌` drew as
// `me "all done" ╌ unfinished ╌`, and a bit reading `╌` drew as `me ""`. Both
// halves are wrong in the direction that matters here — the surface asserts
// somebody did not finish when they did, and it drops characters the speaker
// typed out of the marks whose whole job is verbatim. U+254C makes it near
// unreachable in practice, which is an argument about how often and never one
// about whether.
//
// With marked in hand it is exact. [said] appends the mark last, so on a
// fragment the final suffix is the one it put there; on anything else nothing is
// stripped at all. No cut can leave a partial mark, since [said] never truncates
// one.
func unmarked(row string, marked bool) (said, mark string) {
	if !marked {
		return row, ""
	}
	for _, m := range append(append([]string{}, unfinished...), "╌") {
		if row == m {
			return "", m
		}
		if rest, cut := strings.CutSuffix(row, " "+m); cut {
			return rest, m
		}
	}
	return row, ""
}

// saidWhole is a bit's content as however many rows of the width it was given it
// takes: the same text [said] draws, wrapped rather than cut.
//
// It exists because a `…` is an antecedent with no receipt — the one cut on this
// surface that is visible and unfollowable. Everything else this screen drops
// leaves something a person can follow: a fold leaves a scar with a key printed
// on it, a receipt names every address it stands for. A sentence cut at the
// margin leaves three dots and no way back, while the store holds the bit whole
// and hands it, whole, to the model on the very next request ([Model.turns]). So
// the other participant in the conversation could read it and the person at the
// keyboard could not.
//
// It wraps [oneLine]'s collapsed text, which means it shows every word the record
// holds and not the line breaks it holds. That is a real fidelity loss and it is
// named rather than left to be found: a numbered list comes back as a run-on
// sentence. It is the same trade the scar's filler filter makes — the record
// keeps everything, the view is the only place anything is dropped — and it keeps
// this function's arithmetic what every other row's is, one line at a time.
// [TestAnExpandedRowShowsEveryWordAndNotTheLineBreaks] holds both halves.
//
// The mark for a speaker who did not finish is placed differently here than in
// [said], and the difference is the whole advantage of having more than one row.
// There, the mark and the sentence compete for one row's columns and the ladder
// decides how much sentence to give up; here there is always another row, so the
// mark goes at the end of the last line when it fits beside it and on a row of
// its own when it does not. No width of terminal makes an expanded bit trade a
// word for its mark.
//
// The width is clamped before it reaches [ansi.Wrap], which is not defensive
// tidiness: Wrap returns the string *unwrapped* when the limit is below one, so a
// caller passing zero would get the whole bit on a single line and straight past
// the margin. [TestAnExpandedRowSurvivesAWidthOfNothing] is that trap, held.
//
// A scar comes back as exactly one line, and that is a decision rather than a
// consequence. This block exists to show a *message* whole, and a fold has no
// message of its own — what it has is a quotation of one of the bits it absorbed,
// already cut to the row it is on by [scarLine]. Quoting that bit at greater
// length here would be one surface making a larger claim about a fold than the
// fold's own row makes, on the surface that cannot open the receipt.
func saidWhole(f frame, b memory.Bit, width int) []string {
	width = max(width, 1)

	if c, cold := b.Payload.(memory.Compaction); cold {
		return []string{scarLine(f, c, width)}
	}

	lines := strings.Split(ansi.Wrap(oneLine(b), width, ""), "\n")
	for i, l := range lines {
		lines[i] = clip(l, width)
	}

	u, ok := b.Payload.(memory.Utterance)
	if !ok || !u.Truncated {
		return lines
	}

	// Widest first, and the first mark narrow enough for the terminal wins — the
	// ladder is about the terminal here rather than about the sentence, since the
	// sentence has already been given every row it asked for.
	last := len(lines) - 1
	for _, mark := range unfinished {
		if lipgloss.Width(mark) > width {
			continue
		}
		// No gap when there is no sentence, for [said]'s reason: a speaker who got
		// nothing out should read flush in their column rather than indented past
		// it.
		if lines[last] == "" {
			lines[last] = mark
			return lines
		}
		if lipgloss.Width(lines[last])+lipgloss.Width(" "+mark) <= width {
			lines[last] += " " + mark
			return lines
		}
		return append(lines, mark)
	}

	// Narrower than the narrowest wording, so the bare dash goes on its own row.
	// There is no rung under it: a row with no mark claims its speaker finished.
	return append(lines, clip("╌", width))
}

// scarLine is a fold drawn as an ordinary row, for a surface that draws bits by
// address rather than by walking a view and switching on what it finds.
//
// The transcript never asks for this: it sees the payload first and hands a fold
// to [seam], which spends a whole row on the receipt. Anything that looks a bit
// up and draws it — the ranked view today, a search result tomorrow — gets here
// instead. It is reachable by two keystrokes rather than in theory: [Model.fold]
// moves the caret onto the scar that absorbed its bit and [Model.vote] casts on
// whatever the caret names, so a person can vote on a fold by accident and a
// voted fold is in the ranked list.
//
// What it says is what the scar's own row says with far fewer columns to say it
// in: how much went, and one of the bits it stands for, quoted. Not the span and
// not the key — the row it lands on has a clock column of its own, and printing
// ctrl+u on a surface where ctrl+u does nothing would be the screen naming a dead
// key.
//
// It is the same account the transcript gives, from the same [frame.quoted], so
// one object does not have two stories about itself on two screens. That was the
// state this replaced: the scar's own row summarised by word count and this row
// summarised by the same word count, both of them a phrase nobody said.
func scarLine(f frame, c memory.Compaction, width int) string {
	head := fmt.Sprintf("%d bits", c.Count())
	if q, _ := f.quotation(c, width-lipgloss.Width(head)-3); q != "" {
		return head + " · " + q
	}
	return ansi.Truncate(head, max(width, 1), "…")
}

// oneLine is a bit's content, whitespace runs collapsed, at no particular width.
// The collapse is not cosmetic: a pasted newline would otherwise cost a block
// its one-row-per-bit count, and every column measurement on this surface is
// arithmetic on a single line.
//
// It carries no mark and is not for drawing — [said] is. What it answers is
// "what does the record hold", which is why the reachability tests read it.
func oneLine(b memory.Bit) string {
	switch p := b.Payload.(type) {
	case memory.Utterance:
		return strings.Join(strings.Fields(p.Text), " ")

	case memory.Compaction:
		// How much went, and nothing else, because nothing else is in this bit.
		//
		// This branch used to carry the fold's top four words, and that was the one
		// account of a fold a caller could get without a store. It is gone on
		// purpose: memory/cool.go documents the bag those words come from as
		// destroying what was said, and a caller with no store in hand has no honest
		// way to say more than the count. What a scar actually shows is a quotation
		// of a bit it absorbed, and that bit is in the store rather than in here —
		// so it is [scarLine]'s to draw, and [said] routes a fold there before it
		// ever reaches this function.
		//
		// Which leaves this as what it says it is: what the record holds, at no
		// particular width, for a reader asking about this bit alone.
		//
		// Nothing that draws reaches this branch any more — [said] and [saidWhole]
		// both send a fold to [scarLine] first — and it is kept rather than deleted
		// for two reasons. Deleting it would drop a fold into the default arm below,
		// which says "this kind is unhandled", and Compaction is handled. And this
		// function is the answer to "what does the record hold", which callers ask
		// about every kind of bit. It has a witness through its own front door:
		// [TestOneLineSaysWhatAFoldHoldsAndNothingItStandsFor].
		return fmt.Sprintf("%d bits", p.Count())

	default:
		// Payload is a closed set and Go does not check switch exhaustiveness,
		// so an unhandled kind is loud here as it is in the transcript.
		return fmt.Sprintf("<unrendered %T>", p)
	}
}

// column left-aligns s in w columns and adds the gap after it, marking s with
// an ellipsis if it had to be cut. Columns that do not line up are worse than
// columns that are short: alignment is the part of this block that survives a
// terminal with no color, so it is the part that has to be right.
//
// The ellipsis is not decoration. A bare cut makes a shortened name and a short
// name look the same, and two different speakers arrive on screen under one
// string with nothing saying so.
func column(style lipgloss.Style, s string, w int) string {
	text, pad := cell(s, w)
	return style.Render(text) + pad
}

// cell is [column] with the styling left to the caller, for the one row that
// needs two styles on one column: the transcript wraps the fade round the
// speaker's own colour rather than replacing it.
//
// What that actually renders is worth writing down, because the sentence here
// used to claim the opposite and no instrument could see it. The inner style
// wins: a handle is drawn in the speaker's colour whether its bit is cooling or
// not, so the fade is carried by the sentence and not by the name. Measured, on
// real frames, by the colour column in harness_test.go's [colours] — a cooling
// row reports "35 242" and a hot row "35 -", the same 35 on both. Whether the
// name ought to fade with its bit is a live question and not this comment's to
// settle; what is settled is that today it does not.
func cell(s string, w int) (text, pad string) {
	s = ansi.Truncate(s, w, "…")
	return s, strings.Repeat(" ", max(w-lipgloss.Width(s), 0)+colGap)
}

// widest is how wide the handle column wants to be: the widest handle actually
// present, never narrower than nameFloor.
//
// It is measured from the material rather than fixed by a constant. A constant
// is what let two agents named coordinator-7 and coordinator-9 arrive on screen
// as one string: the column was ten wide, both names were thirteen, and the cut
// was silent and unmarked. Measuring means a handle is shown whole whenever the
// terminal can hold it, and marked with an ellipsis when it cannot.
func widest(names []string) int {
	w := nameFloor
	for _, n := range names {
		w = max(w, lipgloss.Width(n))
	}
	return w
}

// nameColumn is [widest] cut down to what room columns will spare once the text
// beside it has its floor. It is the last column to give, after every other one
// has been dropped, because who said a thing is the question this surface
// exists to answer and the machinery columns beside it are only how you check.
func nameColumn(names []string, room int) int {
	return min(max(room-colGap-textFloor, nameFloor), widest(names))
}

// fit returns the first candidate that fits the width, cut with an ellipsis if
// even the last one does not. The caller orders them widest first and puts what
// must never be dropped in all of them.
func fit(width int, candidates ...string) string {
	for _, c := range candidates {
		if lipgloss.Width(c) <= width {
			return c
		}
	}
	return clip(candidates[len(candidates)-1], width)
}

// abridged is [fit] for an index of keys: the widest rung that fits, with a mark
// saying that the rungs above it exist.
//
// A separate function because the two objects degrade differently and the
// difference is the whole point. Every other ladder here *rewords* — a scar with
// no span still describes the same fold, a row with fewer columns of sentence is
// still that sentence cut, and [clip] and [said] put an ellipsis on the cut so
// the reader knows what happened. The footer's ladder does not reword. It drops
// whole bindings, and a key that is not printed is indistinguishable from a key
// that does not exist: at eighty columns this surface silently stopped naming
// ctrl+c, and a person could reasonably conclude the program has no way out.
//
// The mark is the same ellipsis every other cut on this screen uses, and that
// is deliberate rather than economical. bubbles/help writes "…" for exactly this
// (its ShortHelpView drops bindings from the end and writes an Ellipsis), so the
// ecosystem already agreed the glyph; and using the surface's own cut mark makes
// this one rule rather than a second vocabulary for a second kind of cut.
//
// It is on at nearly every terminal anybody runs, and that is honest rather than
// a signal gone soft. The widest rung is wider than a hundred columns leaves once
// the gauge has taken its share, so keys genuinely are being dropped nearly
// always — the state a reader almost never sees is the *complete* index, not the
// abridged one. What the mark buys is not rarity: it is that "the index stops
// here" and "that is every key" stop being the same picture.
//
// There is deliberately no count. A number of dropped bindings is one this screen
// cannot be checked against — the other ladders' counts ([Model.edge]'s "12
// more") name rows a keystroke brings into view, and no keystroke brings a
// dropped rung back.
//
// It never returns the empty string, which is what the ladders it replaces all
// ended in. That terminator is the mechanism D59(k) named: an empty candidate
// fits every width, so [fit] returned it before it ever reached its own ellipsis
// rung, and the narrowest footer on this surface was a blank row claiming the
// screen had nothing to say about its keys.
func abridged(width int, ladder ...string) string {
	if len(ladder) == 0 {
		return ""
	}
	if lipgloss.Width(ladder[0]) <= width {
		return ladder[0]
	}

	// One space and the mark. Held off the last key rather than run onto it,
	// because "keep …" is a cut index and "keep…" is a cut word.
	const mark = " …"
	for _, rung := range ladder[1:] {
		if lipgloss.Width(rung+mark) <= width {
			return rung + mark
		}
	}
	return clip(strings.TrimSpace(mark), width)
}

// clip is the backstop under every row on this surface: nothing is ever wider
// than the width it was given, and anything cut says so.
//
// The column arithmetic above is meant to make this a no-op, and above about
// fifteen columns it is. Below that the fixed parts — a gutter, an ordinal, a
// three-character handle, one character of text — already outrun the terminal,
// and there is no arrangement of them that does not. What there is a choice
// about is whether the overflow is marked, and the viewport clips without a
// mark. A row cut by the terminal and a row that happened to end there look
// identical, which is the one thing this screen may not do.
func clip(s string, width int) string {
	width = max(width, 1)
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = ansi.Truncate(l, width, "…")
	}
	return strings.Join(lines, "\n")
}
