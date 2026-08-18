package tui

import (
	"cmp"
	"fmt"
	"iter"
	"slices"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/tyler-j-chrestoff/tldreddit/memory"
)

// frame is everything one drawing of the transcript is made from.
//
// A struct rather than eight parameters, and the reason is not the count. Every
// field here is an answer to a question about the same two views, resolved once
// by [Model.frame] and handed over whole: which bits are on screen, which of
// them the next fold takes, which are held and for how much longer, what the
// human has said about each of them, and where the caret is. Asked one at a
// time, at the point each row is drawn, they are readings taken at different
// moments of a record that a keystroke can change between them — and the screen
// that results disagrees with itself, quietly, in the direction nobody checks.
type frame struct {
	store *memory.Store
	bits  []memory.Bit

	// clock decides how much of an instant each row shows. See [clock].
	clock clock

	// absorbing is the set of addresses the next fold will take, from
	// [memory.View.Absorbing]. Those rows are drawn cooling.
	absorbing map[string]bool

	// holds is how much longer each held bit has, from [memory.Stay.Holds],
	// narrowed to the bits a hold can still hold back — see [Model.live]. A bit
	// that is not held is absent rather than present with nothing left.
	holds map[string]time.Duration

	// covered is the bits a hold is sparing that nobody voted on: the bit each
	// held bit names through Prev, from [memory.View.Sparing] with the holds above
	// taken out of it. Those rows are drawn with a tie in the vote column and
	// nothing else — see [voteCell].
	//
	// This doc said "the question each held answer was answering" until the
	// reading was checked against the record. Prev is the head of the view at the
	// instant a bit was written, which coincides with the turn being answered in an
	// alternating session and does not for anything written through `tldr say` —
	// measured at 24% of said bits on this project's own record. What this set
	// licenses the screen to say is a position, not a relation; see [Model.covered]
	// for the whole of the boundary and memory's own `sparing` for the same line
	// drawn from the other side.
	//
	// Held and covered are disjoint by construction and that is the whole point
	// of keeping them apart here rather than as one set with a flag. A held bit
	// carries a ballot and a covered one does not, and the column they share is
	// the column that says what the human said.
	covered map[string]bool

	// order is the ranked reading of [Model.judged] — everything said, plus
	// anything else voted on — from [memory.View.Rank], and it is empty whenever
	// the transcript is the surface up. It is resolved
	// here with everything else for this struct's own reason: the bands a reader
	// sees, the mark drawn on each row and the caret's place in the list are three
	// readings of one vote view, and a screen that took them at three moments
	// would disagree with itself about which band a row is in.
	order []memory.Ranked

	// votes is every standing vote on every bit, from [memory.Tally]. Only the
	// local handle's entry is drawn: this row is what *you* said about that bit,
	// and a screen showing an agent's votes in the same column would be showing
	// two tiers of a decision as though they were one (see [memory.Score]).
	votes map[string]memory.Score

	// mark is the address the caret is drawn on, which is not always the address
	// [Model.mark] holds: a caret survives the fold that took its bit, and the
	// surface can only draw the scar that absorbed it. [stands] is where that
	// resolution happens, once, in [Model.frame] — so a renderer draws the caret
	// by comparing addresses and never has to know a fold happened.
	//
	// It can still name a bit this frame does not hold, when nothing in the view
	// stands for it at all. Then no caret is drawn, which is the honest picture of
	// a caret pointing at something this surface cannot show.
	mark string

	width int
	open  bool
}

// standing is what the human at this keyboard currently says about a bit: +1,
// -1, or 0 for nothing.
func (f frame) standing(id string) int { return f.votes[id][localHandle] }

// quoted is the one bit a scar shows out of everything it absorbed: the bit this
// reader's own votes rank highest, in that speaker's own words.
//
// **Why a bit and not a word count.** A [memory.Compaction] carries a bag of
// words, and memory/cool.go says what that bag is in as many words — "a good
// index and a poor record: it preserves what was discussed and destroys what was
// said about it". That was this scar's label for the life of the project, on the
// most-read row in the program. Rendered as four words in a row it reads as a
// sentence and it is not one: two real frames off this project's own record
// summarised a window as "name same box s" and "understood s before migration",
// where "s" is a crumb from splitting "let's" and "understood" is a model's
// verbal tic. Nobody said either phrase. A quotation cannot invent a phrase,
// because it is one somebody actually said, and it is followable — press ctrl+u
// and it is one of the rows, spelled the same way.
//
// **The ranking is the summarising, which is this project's own thesis doing
// work.** The tiers are [memory.View.Rank]'s, restated here rather than borrowed
// only because the absorbed set is not a view: this reader's own vote decides
// first, and only where they said nothing does anyone else's sum arrange
// anything. So a bit somebody kept an hour ago, whose hold has long since
// decayed, is what the scar that eventually took it remembers — the vote outliving
// the hold it bought, which is the second and permanent thing a keep is worth.
//
// **The tie is recency, and at three standing votes in thirty-four bits the tie
// is the common case, so it is a design decision rather than a fallback.** The
// scar sits in the slot its bits vacated: the row directly beneath it is the
// oldest thing still on screen, and the newest thing this absorbed is the row
// that used to sit right there. So with nobody voting the seam reads straight
// down — this many bits went, the last of them said this, and then the transcript
// continues. Ranking by view order instead (memory's own tie rule) would name the
// *oldest* absorbed bit, which after a re-fold is the first thing said in the
// whole session and never changes again: a label that is the same on every frame
// forever is a property of the object rather than news about it.
//
// It quotes only utterances. A scar absorbs originals, never the folds beneath it
// (see [memory.Cool]), so the only other thing in reach is an empty one, and a
// quotation of nothing is not one.
//
// And only utterances with something quotable in them — see [opening], which
// owns that rule and the reason for it. A message that opens with a fenced
// program is passed over here the same way an empty one is.
func (f frame) quoted(c memory.Compaction) (memory.Bit, bool) {
	ids := slices.Collect(c.Absorbed())

	var best memory.Bit
	found := false
	bestOwn, bestOthers := 0, 0
	for i := len(ids) - 1; i >= 0; i-- {
		b, ok := f.store.Get(ids[i])
		if !ok {
			continue
		}
		u, said := b.Payload.(memory.Utterance)
		if !said || strings.TrimSpace(u.Text) == "" {
			continue
		}

		// A bit with nothing quotable in it is passed over here rather than
		// refused later, and that is the other half of [opening]. A message that
		// opens with a fenced program has no prose to put quotation marks round;
		// refusing it at the draw would cost the scar its whole account of itself
		// even though nine of the ten bits it absorbed were sentences. So the
		// choice is over the bits this reader's votes rank highest *and* that have
		// words a quotation may carry, which is the same narrowing already made one
		// line above for a bit that is not an utterance at all.
		if opening(u.Text) == "" {
			continue
		}

		own, others := 0, 0
		for who, dir := range f.votes[b.ID] {
			if who == localHandle {
				own = dir
				continue
			}
			others += dir
		}

		// Strictly greater, walking newest first, so a tie keeps the newer bit.
		if !found || own > bestOwn || (own == bestOwn && others > bestOthers) {
			best, found, bestOwn, bestOthers = b, true, own, others
		}
	}
	return best, found
}

// quotation is [frame.quoted] drawn in the room a scar has for it: the speaker,
// then their words in quotation marks, cut with an ellipsis like any other row
// here.
//
// The marks are two columns and they are the point. What went wrong with the word
// list was never that it was uninformative — it was that it sat in the slot a
// summary sits in and looked like prose the machine had written. Quotation marks
// are the one piece of punctuation nobody needs taught: they say these are
// somebody's words, verbatim, and this screen did not compose them.
//
// The speaker is never dropped and is truncated instead, which is the handle
// column's own rule everywhere else on this surface (see [cell] and
// [nameColumn]) rather than a new one. Two things were tried first and both were
// wrong. Dropping it leaves an anonymous sentence on the surface whose subject is
// provenance — and it is not even free: with the name gone at one width and
// present at the next, a terminal one column *wider* showed three fewer
// characters of what somebody said, which is what
// [TestWideningTheTerminalNeverTakesAnythingOffARow] exists for, on a different
// row. Keeping it whole is worse in the other direction, and the frame said so: at
// sixty columns with a vote cast, `coordinator-7` is thirteen columns of a
// twenty-four-column allowance and the row went from three words to no account of
// its content at all.
//
// So the name gives way by the column, floored at [nameFloor], and the words take
// what is left. Both are monotone in the width by construction: the name never
// grows faster than the room, so the words never shrink as a terminal widens.
//
// Below [quoteFloor] columns of actual words it returns nothing and the caller
// falls back to a rung with no account of the content in it. That is deliberate:
// the old ladder shed words one at a time down to a single word, and one word
// drawn from a bag is exactly the noise this replaced. A count, a span and a key
// that still works is a better narrow scar than a random noun.
//
// whole reports that the words were not cut, which is what lets a caller decide
// whether it can afford the span — see [seam]. It reads through [said] so that a
// speaker who did not finish is marked here exactly as they are marked on their
// own row, rather than being quoted as though they had stopped where the fold did.
func (f frame) quotation(c memory.Compaction, width int) (string, bool) {
	b, ok := f.quoted(c)
	if !ok {
		return "", false
	}

	// The name, the space after it, and the two quotation marks. The name is cut
	// with an ellipsis when the room runs out, exactly as the handle column is,
	// and never below [nameFloor] — under that it is not a name, it is a letter.
	who := b.From.Display
	who = ansi.Truncate(who, min(lipgloss.Width(who), max(width-3-quoteFloor, nameFloor)), "…")

	// The quotation closes before the unfinished mark rather than around it — see
	// [unmarked]. The mark keeps the columns it had either way, because the space
	// that held it off the sentence is the same space that now holds it off the
	// closing quote, so nothing here changes what fits.
	//
	// Whether there is a mark to take off is answered from the bit rather than
	// from the row, which is what makes [unmarked] exact: a speaker who typed the
	// glyph themselves keeps it inside their own quotation.
	// [opening] rather than [said], because these two characters assert that what
	// is between them is what somebody said and [said] draws a *rendering* now —
	// a heading in capitals with its hashes spent, a fence quoted as one line of
	// code. Quoting that would put the surface's own editing inside the marks that
	// promise it did none. The ladder is shared and only the words differ; see
	// [oneRow].
	u, _ := b.Payload.(memory.Utterance)
	text, mark := unmarked(oneRow(opening(u.Text), width-lipgloss.Width(who)-3, u.Truncated), u.Truncated)
	whole, _ := unmarked(oneRow(opening(u.Text), uncut, u.Truncated), u.Truncated)

	// The floor is on the words that survived, and it applies only to a quotation
	// that was cut. Both halves matter and the second was wrong first: a floor on
	// the room refuses to quote "and again" at any width, because the bit is
	// shorter than the floor and always will be. What is unreadable is a *fragment
	// of* a sentence too short to be one — a whole short sentence is a whole short
	// sentence. And measuring the words rather than the room is what stops a
	// truncated bit spending its whole allowance on its own mark and quoting a
	// single ellipsis.
	if text != whole && lipgloss.Width(text) < quoteFloor {
		return "", false
	}

	q := who + ` "` + text + `"`
	if mark != "" {
		q += " " + mark
	}

	// And it never returns more columns than it was given, which is the contract
	// [said] states for every row on this surface and which the arithmetic above
	// nearly keeps rather than keeps. Two floors sit under it — [nameFloor] on the
	// speaker and [said]'s own clamp to one column — so a room of five columns
	// still buys a three-column name, three columns of punctuation and a column of
	// words. Measured on a one-character bit: `said` on a fold asked for one column
	// drew fifteen.
	//
	// The caller cannot be left to cut it. Truncating a quotation from outside is
	// what takes the closing mark off it, and an unclosed quotation is this
	// surface asserting that a cut sentence is the whole of what somebody said —
	// the one thing these two characters exist to rule out. So the quotation is
	// refused entirely and the caller falls back to a rung with no content account
	// in it, exactly as it does below [quoteFloor].
	if lipgloss.Width(q) > width {
		return "", false
	}
	return q, text == whole
}

// voted reports whether anything in this frame carries a vote at all. It is what
// decides whether the vote column exists: before the first vote there is nothing
// for it to say, and a column of blanks is columns taken from the sentence.
func (f frame) voted() bool {
	for _, b := range f.bits {
		if f.standing(b.ID) != 0 {
			return true
		}
	}
	return false
}

// anchors is where the two things a caller may want to scroll to ended up,
// counted in lines of the drawn transcript by the thing that drew them. Either
// is -1 when this frame does not draw it at all.
//
// Counted here rather than worked out by the caller because a bit is not always
// one row: an open receipt puts a block of them under its scar, and a second
// count of that arrangement would be a second answer to a question the renderer
// has already answered.
type anchors struct {
	// mark is the line the caret is on.
	mark int

	// rows is how many lines the caret's own bit drew, which is one for every bit
	// on this surface except the one the caret is on — see [transcript], where the
	// caret's row is the only row drawn whole. It is 0 when no caret is drawn, and
	// it counts the bit's own rows only: a receipt opened under a scar is anchored
	// separately, by scar.
	//
	// The renderer counts it for the same reason it counts the other two: a second
	// count of a variable-height row is a second answer to a question the thing
	// that drew it has already answered. [Model.sync] is what reads it.
	rows int

	// scar is the line of the first fold drawn. It is what ctrl+u scrolls to,
	// and it is the first rather than the only one because D32 ended the
	// invariant that there is at most one and that it sits at the top.
	scar int
}

// transcript renders the view, and reports where the caret and the first scar
// ended up in it.
//
// Bits in the absorbing set are the ones the next fold will take, and say so in
// two ways: drawn dimmer, and stepped left into the margin. Two
// channels because one of them is colour and colour is the first thing a
// terminal takes away — see [caretCell], which owns the column. A held bit is
// never in that set — a hold splits the fold around it — so it stays bright and
// in place while the material either side of it fades and steps, which is the
// whole picture this screen exists to produce.
//
// One row per bit, except the caret's, which is drawn whole and takes as many
// rows as its sentence needs. That is the only variable-height row here and the
// bound matters: the view's height is the number of bits plus one bit's worth of
// wrapping, never a multiple of it. See [saidWhole] and this package's doc.
//
// When open is set, every scar is followed by the bits it stands for, resolved
// from the store as this runs. Nothing is cached: the block on screen is a
// fresh walk of the receipt every time the transcript is drawn, which is the
// strongest available form of the claim it makes.
func transcript(f frame) (string, anchors) {
	bits, width := f.bits, f.width

	// Who spoke is a column, not a prefix. With one speaker the two are the
	// same picture, which is why this went unnoticed; with a model answering
	// under its own handle the left edge goes ragged, and a reader scanning for
	// one voice has to read every line to find it. The handle column in the
	// block a receipt opens already worked this way — this is the same column,
	// one indent out, rather than a second way of saying the same thing.
	names := make([]string, 0, len(bits))
	for _, b := range bits {
		if _, cold := b.Payload.(memory.Compaction); !cold {
			names = append(names, b.From.Display)
		}
	}

	vote, name, room := columns(names, width, f.voted())

	var out strings.Builder
	line, at := 0, anchors{mark: -1, scar: -1}
	for _, b := range bits {
		if b.ID == f.mark {
			at.mark, at.rows = line, 1
		}

		// going is the fade, and it is now two channels rather than one. The row
		// is drawn dimmer, and it also steps left into the margin —
		// see [caretCell], which is where the column comes from and why the
		// direction is left.
		going := f.absorbing[b.ID]

		style := hot
		if going {
			style = cooling
		}

		switch p := b.Payload.(type) {
		case memory.Utterance:
			// Read through said, which is the one definition of a bit drawn as
			// a row, used by the transcript and the receipt alike so that the
			// two cannot drift on either thing it does.
			//
			// It cuts with an ellipsis rather than letting the viewport clip at
			// the edge: text that stops at the margin and text that ran out of
			// margin look identical, and a surface whose whole argument is that
			// it shows you what it dropped cannot drop the end of a sentence
			// without saying so. It collapses whitespace runs, which stopped
			// being a nicety the moment a model became a speaker — a reply with
			// a newline in it would otherwise put a raw newline inside a row,
			// and every column measured on that row is arithmetic on a single
			// line. And it marks a speaker who did not finish, which is a
			// different fact from either of those and is drawn as one.
			who, pad := cell(b.From.Display, name)
			head := caretCell(b.ID == f.mark, false, false, going) +
				voteCell(f, b.ID, vote, style, false) +
				style.Render(speaker.Render(who)) +
				pad

			// Every row but one is a single line cut at the margin. The caret's is
			// drawn whole — see this package's doc for why that is the caret's job
			// and why it is not a key.
			//
			// Below textFloor it is cut like the rest, and that is the same
			// concession [clip] and [nameColumn] already make in as many words: down
			// there the fixed columns have outrun the terminal, the handle is already
			// clamped at its floor, and wrapping a sentence into a column two
			// characters wide produces a hundred rows of rubble rather than a reading
			// of anything. The floor is measured rather than chosen, and it is pinned
			// by [TestTheCaretsRowIsCutWhereTheArrangementAlreadyGaveUp].
			if b.ID != f.mark || room < textFloor {
				fmt.Fprintf(&out, "%s\n", clip(head+style.Render(said(f, b, room)), width))
				line++
				continue
			}

			// What a continuation row keeps: exactly the columns its first row spent
			// on the margin, the vote and the handle, so the block hangs under its own
			// sentence and the handle column stays the only place a name appears. A
			// blank handle column is what says "still the same speaker", which is
			// alignment rather than colour and survives a terminal with neither.
			//
			// It steps with its own row. A going block moves left as one object,
			// because the fade is a fact about the bit and not about its first line —
			// and the sentence width is [room] on every line whichever way it steps,
			// so nothing re-wraps when a bit starts cooling or a vote saves it. That
			// is [caretCell]'s no-reflow rule, applied to a taller object.
			hang := caretColumn + step + vote + name + colGap
			if going {
				hang -= step
			}

			// hang and room add up to exactly the width the row was given, which is
			// why the [clip] on a continuation row below cannot currently fire — it
			// is the backstop this surface puts under every row rather than a check,
			// and nothing can catch its removal while the guard above holds. Said
			// here rather than left for somebody to discover by deleting it.
			whole := saidWhole(f, b, room)
			at.rows = len(whole)
			fmt.Fprintf(&out, "%s\n", clip(head+style.Render(whole[0]), width))
			line++
			// A tie carries down the whole block, for the same reason the step does:
			// it is a fact about the bit and not about its first line. The step's
			// rule was already written three paragraphs up and the tie did not
			// follow it, which was invisible until somebody put the caret on a
			// covered row and looked. What [voteCell] promises is that half a stroke
			// hangs down into the ▲ on the row below; measured on this project's own
			// record at 100x30, opening a covered reply left the stroke **five lines**
			// above the mark it points at, with the block between them — and on a
			// fixture of model-length replies every covered row opens that way.
			//
			// The column is read off [caretCell] rather than restated as
			// caretColumn+step, so the substitution cannot drift from the geometry
			// and there is no going/staying branch here to be wrong: a covered row is
			// never in the absorbing set, so the branch would be unreachable anyway
			// and asking the function is exact either way. hang is spaces, so a byte
			// index into it is a column.
			lead := strings.Repeat(" ", hang)
			if f.covered[b.ID] && vote > 0 {
				at := lipgloss.Width(caretCell(false, false, false, going))
				lead = lead[:at] + rule.Render(tie) + lead[at+1:]
			}
			for _, l := range whole[1:] {
				fmt.Fprintf(&out, "%s\n", clip(lead+style.Render(l), width))
				line++
			}

		case memory.Compaction:
			if at.scar < 0 {
				at.scar = line
			}

			// The scar keeps column 0 while everything anybody said is indented
			// behind the caret. That is the whole of the structural change to it,
			// and it is deliberately not a colour: the one object on screen
			// standing for something absent now owns two columns of margin that
			// nothing else may enter, so it hangs into the left edge and the eye
			// finds it by shape.
			//
			// It keeps them while cooling too. going is handed over rather than
			// dropped here so that [caretCell] stays the one place deciding what a
			// margin is worth, and it is ignored there: a scar is already at the
			// edge the step moves toward. This package's doc carries that as the
			// fade's second hole, with the measurement that says to leave it open.
			fmt.Fprintln(&out, clip(
				caretCell(b.ID == f.mark, true, f.open, going)+
					voteCell(f, b.ID, vote, seamInk, true)+
					seam(f, p, max(width-caretWidth-vote, 1)), width))
			line++

			if f.open {
				block := unfold(f, p)
				fmt.Fprintln(&out, clip(block, width))
				line += strings.Count(block, "\n") + 1
			}

		default:
			// Payload is a closed set but Go does not check switch
			// exhaustiveness, so an unhandled kind must be loud rather than
			// invisible.
			fmt.Fprintf(&out, "%s\n", warm.Render(fmt.Sprintf("<unrendered %T>", p)))
			line++
		}
	}
	return strings.TrimRight(out.String(), "\n"), at
}

// columns is the transcript's own column arithmetic: how wide the vote column
// is, how wide the handle column is, and how many columns are left for what
// somebody said.
//
// It was inside [transcript] until the fold budget needed it. [Model.rows] asks
// how many rows a bit draws, and a bit's height is a function of the column its
// sentence is wrapped into — so the budget and the renderer have to agree about
// that number exactly, and this package's standing rule is that two statements of
// one rule agree on the day they are written. Here that failure would be silent
// and total: the fold would be sized against a screen that is not the screen.
//
// The margin is never given up to the terminal: it is reserved whether or not the
// caret is in it, and a caret that moved the row it sits on would make every row
// jump as it passed. It is given up to the fade, and only to the fade — a row the
// next fold will take spends [step] of it saying so, which is margin reserved for
// one thing being spent on another thing about the same row rather than on width.
// See [caretCell].
//
// The drain goes before the handle shrinks. It is the machinery column here — it
// says how much longer, and the mark beside it already says that something is
// being held — and provenance outranks machinery on this surface, which is the
// same order the receipt's own ladder uses.
//
// The room it returns is measured from where a row that is *staying* begins, not
// from where the row being drawn begins. A cooling row starts [step] columns
// further left and so has that much slack at its right end, and the slack is left
// unspent on purpose: spending it would mean a sentence gains characters at the
// moment its bit starts cooling and loses them again if a vote saves it, so the
// text would reflow on exactly the two events this screen exists to make legible.
func columns(names []string, width int, voted bool) (vote, name, room int) {
	if voted {
		vote = markWidth + drainWidth + 1
		if width-caretColumn-step-vote-widest(names)-colGap < textFloor {
			vote = markWidth + 1
		}
	}

	lead := caretColumn + step + vote
	name = nameColumn(names, width-lead)
	return vote, name, max(width-lead-name-colGap, 1)
}

// Column widths in front of a row: the margin, then the vote's, which is a mark
// and — where the terminal can hold it — a gauge draining beside it.
const (
	// caretWidth is the margin a scar hangs into, and only that. A spoken row's
	// margin is caretColumn or caretColumn+step and is never this — the name is
	// older than the step and is kept because the scar's arithmetic reads better
	// with it than with a literal 2.
	caretWidth = 2

	// caretColumn is the one column the caret occupies, and the least a spoken
	// row's margin can be. Nothing takes it: a row with no margin left is a row
	// the caret cannot be drawn on, and moving the caret out in front of the row
	// instead would make every row jump as it passed.
	caretColumn = 1

	// step is how far left of a row that is staying a row the next fold will take
	// begins. See [caretCell], which spends it.
	step = 2

	markWidth  = 1
	drainWidth = 3

	// caret is the one mark, and there is always exactly one. It points at the
	// row it is on rather than boxing or highlighting it, because a highlight is
	// a background colour and this surface degrades to no colour at all.
	caret = "▸"

	// tie is what a covered row draws where a voted row draws its mark: the
	// bottom half of a stroke, descending into the mark on the row below. See
	// [voteCell].
	tie = "╷"
)

// caretCell is the margin in front of every row: the caret, or the space it would
// occupy, and [step] columns fewer on a row the next fold is taking.
//
// That margin is where the fade is drawn as space rather than as colour, and it
// is here because it is the only part of a row that is not content. Colour is not
// removed — this is a second channel beside it, and it is the one that survives a
// terminal with none.
//
// The geometry, which is the part an editor has to keep true:
//
//   - a scar begins at column 0, and is material that has gone;
//   - a row the next fold will take begins at caretColumn, and is going;
//   - a row that is staying begins at caretColumn+step.
//
// Three things a row can be, in the order the material moves, so the left edge
// reads as a gradient toward the margin the scar hangs in and a step between two
// bands is the boundary of the fold window.
//
// Four constraints hold that arrangement in place. The caret keeps column 0 on
// every spoken row, going or staying, so a bit that starts cooling under it does
// not make it jump — which is why a going row cannot begin further left than
// caretColumn. The sentence column is measured from a row that is *staying* and
// used for every row, so a going row ends short of the right margin rather than
// gaining a character, and nothing reflows when a bit starts or stops cooling. A
// scar does not step: it is already at the edge the step moves toward. And [step]
// is two rather than one because of what the frame with one scar and one row
// beneath it looks like — see the craft record for the frames and the measurement
// that decided it, and [TestHarnessFade] for the frames themselves.
//
// What it costs is one column, and who pays depends on the width in a way worth
// stating exactly, because stating it loosely twice made this change read cheaper
// than it is. The handle column can only absorb the cost while it is between
// [nameFloor] and the widest handle actually present: below that range it is
// already at the floor and the sentence pays, above it the column is already as
// wide as its material and cannot grow, so the sentence pays there too — for
// every width from that point up, which is every terminal anybody runs. So the
// ordinary case is that a sentence is one column shorter, and the band where the
// handle quietly takes it instead is the narrow exception rather than the rule.
// The marks in [said] each lose a column of headroom.
//
// No number for any of that is written here on purpose — a figure in a comment is
// one nothing re-derives, and every one this function's doc has carried was
// wrong. They live in [TestWideningTheTerminalNeverTakesAnythingOffARow] and
// [TestTheRowsMarkFloorsAreWhereTheyWereMeasured], which fail when they move.
//
// One row can step alone: a scar counts toward [memory.View.Fold]'s size rule and
// does not step, so a run of [scar, spoken] draws a single jogged row. It stays
// legible because everything else in such a run is a scar and a scar is at column
// 0, so it is never read against a flat edge — [TestALoneJoggedRowIsAlwaysBesideAScar].
//
// A scar spends this margin differently. Its own leading dashes reach into the
// space spoken rows leave empty, so the rule runs further left than anything
// anybody said — and when the caret is on the scar it takes the first column and
// the dash keeps the second, so the row neither shifts nor loses the corner that
// says its receipt is open. going is taken and ignored on that branch, so the
// reason sits with the decision rather than at the call site.
func caretCell(marked, scar, open, going bool) string {
	if !scar {
		if going {
			if marked {
				return hot.Render(caret)
			}
			return strings.Repeat(" ", caretColumn)
		}
		if marked {
			return hot.Render(caret) + strings.Repeat(" ", step)
		}
		return strings.Repeat(" ", caretColumn+step)
	}

	// The corner is the scar saying its receipt is open below it, and it gives up
	// its column to the caret rather than its meaning: unmarked it leads, marked
	// it moves one column right and the caret leads.
	if marked {
		if open {
			return hot.Render(caret) + seamInk.Render("┌")
		}
		return hot.Render(caret) + seamInk.Render("─")
	}
	if open {
		return seamInk.Render("┌─")
	}
	return seamInk.Render("──")
}

// voteCell is what the human said about this bit — and, in one case, what
// somebody said about the row underneath it — drawn in the column between the
// caret and the handle.
//
// Three states and a blank, and the asymmetry between them is the explanation
// rather than something a legend has to carry. An upvote holds the bit out of
// the next fold for a while, so it is drawn as a solid mark with that while
// draining beside it. A downvote holds nothing, so it has nothing to drain — it
// is one mark and no gauge. And an upvote whose hold has run out is the hollow
// mark: the vote is still on the record, permanently, and the stay of execution
// it bought is spent. Nothing here is a colour.
//
// The gauge is the same two glyphs the footer's pressure bar uses, because it is
// the same idea pointed at one row: how close is this to happening on its own.
//
// # The fourth state, which is not a vote and must not read as one
//
// A hold spares the bit a held bit names through Prev as well as the bit it was
// cast on, so upvoting a row keeps the row above it too. Until this existed, that row
// stopped fading with nothing on screen saying why: a person pressed one key and
// watched two rows change, one of which grew a mark and one of which just quietly
// became bright. The behaviour was right and the surface did not explain it, which
// is the same shape of defect as a fold with no antecedent.
//
// **What the pair is allowed to mean, exactly.** Prev is the head of the view when
// a bit was written, which is a position and not a relation. In an alternating
// conversation it coincides with "the turn this replies to" and this doc used to
// say so; for anything written through `tldr say` it does not, and on this
// project's own record 24% of said bits came that way — including a correction
// whose Prev is a greeting rather than the claim it corrects. So the tie may say
// *the row below is what is keeping this row out of the next fold*, which is true
// on every frame, and it may not say the two rows are a question and its answer.
// The mark is the same either way; what changes is what a later feature is allowed
// to build on the edge under it.
//
// What it draws is a [tie]: the bottom half of a stroke, in the mark column, on
// the row directly above the ▲ it belongs to — and down the whole of that row when
// the caret has opened it, for the reason [transcript] gives where it draws them.
// Four things decide that shape and none of them is taste.
//
//   - **It is in the mark column because the mark column is already paid for.**
//     This column only exists once somebody has voted ([frame.voted]), and a
//     covered row only exists once somebody has voted, so the tie can never be
//     the thing that costs a sentence a character, and it can never be the rung
//     that falls off a ladder — it narrows and disappears exactly when the mark
//     it hangs from does.
//   - **It is not in the margin**, which is where the fade lives. Left means
//     nearer gone there ([caretCell]'s three columns), and a covered row is the
//     opposite of going; a fourth position in that staircase would say it was
//     half-way out.
//   - **It is not a triangle.** ▲ ▼ △ are ballots and this row carries none —
//     [memory.View.Sparing] is emphatic about the difference, because the whole
//     reason the fold rule was written the way it was is that nothing may report
//     a vote nobody cast. Half a stroke is not a mark; it is the end of the one
//     below.
//   - **It points down, at the row it belongs to.** That reads because the row
//     it belongs to is always the next one down — see [Model.frame], where the
//     adjacency is argued and measured. The measurement is over *rows*, and the
//     screen draws *lines*: the two were the same thing until the caret's row
//     started being drawn whole, and the stroke now carries down the block so
//     that it still reaches the mark. [TestATieReachesTheMarkItPointsAt].
//
// It gets no key and appears in no ladder. There is nothing to do to a covered
// row that is not already offered on every row (vote on it yourself, which turns
// the tie into a mark), and a key implying otherwise would advertise an
// operation that does not exist. What it changed instead is the gauge, which
// counts what a fold could take and so already drops by two when one upvote
// covers a second row — see [foldable].
func voteCell(f frame, id string, width int, style lipgloss.Style, scar bool) string {
	if width == 0 {
		return ""
	}

	// A scar's own dashes fill this column rather than spaces, so the rule runs
	// unbroken from the margin to the words. A gap of blanks inside a horizontal
	// line reads as two lines.
	fill := func(n int) string {
		if n <= 0 {
			return ""
		}
		if scar {
			return seamInk.Render(strings.Repeat("─", n))
		}
		return strings.Repeat(" ", n)
	}

	switch {
	case f.standing(id) > 0:
		left, held := f.holds[id]
		if !held {
			return style.Render("△") + fill(width-markWidth)
		}
		if width < markWidth+drainWidth+1 {
			return style.Render("▲") + fill(width-markWidth)
		}
		return style.Render("▲") + drain(left, style) + fill(width-markWidth-drainWidth)

	case f.standing(id) < 0:
		return style.Render("▼") + fill(width-markWidth)

	// A covered row: kept out of the next fold, and not by a vote of its own. See
	// the paragraph in this function's doc.
	//
	// The scar guard cannot fire today and is kept as a backstop, disclosed here
	// rather than left for somebody to delete and wonder about. Covering a scar
	// needs a bit whose Prev *is* a scar, which needs the view to have been a scar
	// and nothing else when that bit was written — [Model.utter] takes Prev from
	// the view's last entry, a fold always leaves a kept tail after the scar, and
	// [Model.keep] never goes below half a budget. So `covered` has never named a
	// [memory.Compaction] in any frame anybody has measured, and
	// [TestNoScarIsEverCovered] is the executable form of that rather than a
	// figure here that nothing re-derives. If it ever does,
	// what this guard prevents is a tick inside the scar's own rule, which [fill]'s
	// comment already explains is read as two lines rather than one.
	case f.covered[id] && !scar:
		return rule.Render(tie) + fill(width-markWidth)
	}
	return fill(width)
}

// drain is how much of a hold is left, in drainWidth cells of the same glyphs
// the footer's gauge uses.
//
// It never empties: a hold with a minute left and a hold with a second left are
// both still holding, and a bar showing nothing beside a mark that says
// something is held would be the screen arguing with itself. What runs out is
// the mark, which goes hollow at the instant the hold does.
func drain(left time.Duration, style lipgloss.Style) string {
	full := max(min(int((left*time.Duration(drainWidth)+holdFor-1)/holdFor), drainWidth), 1)

	// The empty half is skipped rather than rendered empty, because a style
	// wrapped round an empty string still emits its colour and then turns it off
	// again — invisible on the screen, and the first thing anything reading the
	// frame back finds on the row.
	out := style.Render(strings.Repeat("▓", full))
	if full < drainWidth {
		out += rule.Render(strings.Repeat("░", drainWidth-full))
	}
	return out
}

// clock is how much of an instant a row shows: the shortest form that is not
// ambiguous against the day this screen has already stated.
//
// The header carries one date, the newest the view holds. A row on that day
// therefore only needs its time, and a row on any other day carries as much of
// its own date as it takes to differ — the month and day, or the whole thing
// once the years differ too. So the common screen costs five columns a row and
// says which day it is, and a conversation that crossed midnight says so on
// exactly the rows that crossed it.
//
// The zero clock has a zero reference, so everything differs from it and every
// stamp comes back with its full date. That is the loud failure rather than the
// quiet one: a caller that forgot to say which day it is gets eleven columns of
// date it did not ask for, instead of four digits that mean nothing.
type clock struct{ ref time.Time }

func (c clock) stamp(t time.Time) string {
	// Read in the reference's own zone, because "which day is this" has no answer
	// until somebody says whose day. Two bits recorded a minute apart in
	// different zones are the same evening to the person reading them, and
	// comparing each instant in its own location would put a date on one of them
	// and not the other.
	t = t.In(c.ref.Location())

	ry, rm, rd := c.ref.Date()
	y, mo, d := t.Date()
	switch {
	case y == ry && mo == rm && d == rd:
		return t.Format("15:04")
	case y == ry:
		return t.Format("01-02 15:04")
	}
	return t.Format("2006-01-02 15:04")
}

// widestStamp is how wide a stamp column has to be for the material going in it.
// Measured rather than fixed, for the reason [widest] gives about handles: a
// column sized by a constant is a column that silently cuts whatever outgrows
// it, and here what outgrows it is the date.
func (c clock) widestStamp(at ...time.Time) int {
	w := 0
	for _, t := range at {
		w = max(w, lipgloss.Width(c.stamp(t)))
	}
	return w
}

// seam renders a compaction as a visible scar rather than hiding it. The
// receipt is the point: how much was folded, over what span, and what it was
// about. A harness that compacts silently teaches you to stop trusting it.
//
// The scar carries its own key, because a footer is where a returning user
// looks and the scar is where a first-time user is already looking. It carries
// the same key in both states, in the same place, so opening the receipt does
// not move the thing that closes it again.
//
// The scar does not go away when the material behind it is on screen — a scar
// that vanished while the bits came back would read as the fold having been
// undone, and a fold is not undoable. It is followable, which is a different
// promise and the one this product actually makes. What says the receipt is open
// is the corner in [caretCell], two columns to the left of everything here.
//
// The span is stamped by the frame's own [clock], so a fold that crossed
// midnight says which day each end of it was on. That is the reading an auditor
// needs and it costs nothing on a conversation inside one day, where both ends
// come back as four digits exactly as they always did.
//
// What it says about the content is a quotation of one absorbed bit, chosen by
// this reader's own votes — see [frame.quoted], which carries the argument for
// why it is not the word count it was for the life of the project.
func seam(f frame, c memory.Compaction, width int) string {
	span := fmt.Sprintf("%s–%s", f.clock.stamp(c.From()), f.clock.stamp(c.To()))

	// The count, qualified. A fold absorbs whatever was in the window, and if a
	// fragment was in there the count alone now reports it as one more thing
	// somebody said. So the tally rides next to the number it qualifies, and it
	// is the last thing dropped before the number itself.
	//
	// This is the only place a folded fragment is visible without pressing the
	// key. The rows in the block ctrl+u opens each carry their own mark, so the
	// tally is one press from being back — which is the same reason the span is
	// dropped before the words, applied to the same ladder.
	// The leading space is the join with whatever [caretCell] and [voteCell] put
	// in front of this: dashes, a caret, a corner, a vote. Every candidate below
	// is built from this string, so it is the one place that has to hold.
	head := fmt.Sprintf(" %d bits", c.Count())
	tally := head
	if n := fragmentsIn(c); n > 0 {
		tally = fmt.Sprintf("%s · %d unfinished", head, n)
	}

	// Widest first. The count and the key survive every cut: the count is the
	// claim the receipt makes, and the key is the only thing on screen saying
	// the claim can be checked.
	//
	// The span is dropped before the quotation, which is the opposite of what it
	// looks like. Every absorbed bit carries its own time in the block the key
	// opens, so the span is one press from being back; the quotation is the only
	// account of what the window was about and is shown nowhere else.
	//
	// The quotation is sized rather than chosen, which is why these rungs are built
	// against the room left over instead of being written out whole. A rung with a
	// quotation in it fits by construction, so [fit] would take the first one every
	// time and the ladder would never step; what steps it is [quoteFloor] refusing
	// to quote into a column too narrow to read.
	//
	// **The span is taken only when it costs the quotation nothing**, and that rule
	// is measured rather than tidy. Written the obvious way — widest rung first,
	// [fit] picks it — a terminal one column wider took thirteen characters off
	// what somebody said, because the span rung has fourteen fewer columns to quote
	// into and the ladder stepped up into it the moment it fit. So the span appears
	// only once the whole quotation already fits beside it, by which width the rung
	// beneath was carrying that same whole quotation anyway.
	//
	// **What is monotone in the width is what somebody said and who said it, and
	// not every column on the row** — an earlier wording of this paragraph claimed
	// the general rule and the row it is written on breaks it. Measured on
	// [seamSweep]'s own fixture: at forty-one columns the scar shows the span, at
	// forty-two it shows a quotation instead and the span is gone. That is the
	// trade above, working, and it is deliberate — the span is one keystroke from
	// being back, since every absorbed bit carries its own stamp in the block
	// ctrl+u opens, while the quotation is the only account of the content
	// anywhere. The two columns that may never shrink as a terminal grows are the
	// ones nothing else on the screen can give back, and they are what
	// [TestSeamNeverSpendsTheQuotationsColumnsOnItsSpan] sweeps.
	var candidates []string
	key := " ── ctrl+u ──"
	withSpan := fmt.Sprintf("%s · %s · ", tally, span)
	bare := fmt.Sprintf("%s · ", tally)
	if q, whole := f.quotation(c, width-lipgloss.Width(withSpan)-lipgloss.Width(key)); whole {
		candidates = append(candidates, withSpan+q+key)
	} else if q, _ := f.quotation(c, width-lipgloss.Width(bare)-lipgloss.Width(key)); q != "" {
		candidates = append(candidates, bare+q+key)
	}
	candidates = append(candidates, fmt.Sprintf("%s · %s ── ctrl+u ──", tally, span))
	if tally != head {
		candidates = append(candidates,
			fmt.Sprintf("%s ── ctrl+u ──", tally),
			fmt.Sprintf("%s · ctrl+u", tally))
	}
	candidates = append(candidates,
		fmt.Sprintf("%s ── ctrl+u ──", head),
		fmt.Sprintf("%s · ctrl+u", head))

	return seamInk.Render(fit(width, candidates...))
}

// fragmentsIn is how many of the bits a fold absorbed were unfinished.
//
// A [memory.Compaction] keeps no payloads, only a tally of their kinds, and
// "fragment" is the name a truncated [memory.Utterance] gives itself — see
// memory/bit.go, where the constraint that makes a field reach a content address
// through kind alone is written down. The name is a hand-written literal there
// and matched by a hand-written literal here, which is a coupling between two
// packages that the compiler cannot see. It is checked end to end instead: fold
// a real truncated bit, draw the scar, and read the number back.
func fragmentsIn(c memory.Compaction) int {
	for kind, n := range c.Kinds() {
		if kind == "fragment" {
			return n
		}
	}
	return 0
}

// filler is the words the fold note does not report to the persona. They are the
// most frequent words in any English text, so without this the index of every
// fold is the same four words and says nothing about the window it stands for.
//
// It used to filter the scar's own label too, which is why it is worded as a
// display rule; the scar quotes a bit now ([frame.quoted]) and this is the word
// index's alone.
//
// This is a display filter and only a display filter. The store's bag still
// counts every word, and the sentences those words came from are one key away —
// which is the whole reason it is safe to edit here. It is the small version of
// what this program is about: the record keeps everything, the view is the only
// place anything is dropped.
//
// It is English, and that is a real limit rather than an oversight. A record in
// another language gets no filtering and its scars go back to reading like
// function words. Fixing that properly means learning the filler from the bag
// itself, which is a change worth making when there is a record that needs it.
var filler = func() map[string]bool {
	seen := map[string]bool{}
	for _, w := range strings.Fields(`
		a an and are as at be been being but by can could did do does doing done
		for from get got had has have he her him his i if in into is it its just
		me my no not of off on one or our out she should so some than that the
		their them then there these they this those to up us was we were what
		when which who will with would you your`) {
		seen[w] = true
	}
	return seen
}()

// topWords returns the n most telling words: most frequent first, then longest,
// then alphabetical, skipping [filler] and anything shorter than three
// characters.
//
// # The short ones, which are code rather than words
//
// Re-derived on this package's own fixture, a forty-bit conversation folded at
// 100x30, with one fenced Go reply in the window and without it: distinct words
// 111 → 178, tokens of two characters or fewer 16 → 30, and the twelve words the
// persona is told went from
//
//	migration backfill staging production columns minutes writing schema …
//
// to
//
//	j s 1 migration backfill reverse staging nothing names slice fmt xs
//
// — the three most prominent slots spent on a loop index, a slice and a literal.
// The bag counts by frequency and a loop variable in a program outruns every
// English word in the window. (An earlier measurement on a ten-bit window put it
// at four slots, `j s 1 t`; the shape is the same and the count is a property of
// the fixture, which is why the procedure rather than the figure is what to
// re-run.)
//
// **What it costs when there is no program in the window is nothing, measured:**
// the same fold with no fenced reply in it returns the identical twelve words
// with the filter and without it.
//
// The cut is at three characters and it is a rule about what a token can carry
// rather than a threshold that was tuned: below three characters a token is a
// letter, a digit or a piece of punctuation-grade syntax, and a word index made
// of those indexes nothing. What it costs is the handful of real two-letter
// words, and [filler] already drops almost every one of them.
//
// # Why it is here and not where the bag is made
//
// [memory.Compaction]'s bag is built by memory/cool.go and reaches `ID(cold)`,
// so a filter there re-addresses every scar already on disk. This is a display
// filter over the same bag, exactly like [filler] beside it — the store still
// counts every token, and the sentences they came from are one `ctrl+u` away.
//
// **It does not cross D60 or D39(a), and the line is worth stating because it is
// close.** D60 keeps the model-facing half of a fold note a *word index* on
// purpose; skipping punctuation-grade tokens leaves it a word index rather than
// making it a quotation. What D39(a) forbids is a bit the human's own approval
// selected reaching the persona, and nothing here reads a vote:
// [TestNoVoteReachesThePersona] is the pin, and this filter is a function of the
// spelling of a token and nothing else.
func topWords(bag iter.Seq2[string, int], n int) []string {
	type word struct {
		text  string
		count int
	}

	var words []word
	for text, count := range bag {
		if filler[text] || len([]rune(text)) < 3 {
			continue
		}
		words = append(words, word{text, count})
	}
	// Length breaks a tie before spelling does. Splitting on punctuation leaves
	// short numeric crumbs behind — "go1.25" becomes "go1" and "25" — and on a
	// count of one, alphabetical order hands the scar to the crumb every time.
	// Both rules together are still a total order, which is the property that
	// matters: a bag is yielded in map order, which Go randomizes on every run,
	// so without one this line renders differently for identical input.
	slices.SortFunc(words, func(a, b word) int {
		if d := cmp.Compare(b.count, a.count); d != 0 {
			return d
		}
		if d := cmp.Compare(len(b.text), len(a.text)); d != 0 {
			return d
		}
		return cmp.Compare(a.text, b.text)
	})

	words = words[:min(n, len(words))]
	top := make([]string, 0, len(words))
	for _, w := range words {
		top = append(top, w.text)
	}
	return top
}

// gauge draws the pressure on the record: how much material a fold could take,
// against how much it waits for. Cooling should never be a surprise.
//
// Both numbers are rows and they were not always. The numerator counted bits
// while the denominator was a screen's height, which was one number stated twice
// for as long as every bit drew one row — and stopped being that the day a
// message could be drawn as a document. The frame that named it: five bits, one
// of them a fenced 36-row answer, drawing forty rows in a viewport of 23 under a
// gauge reading `5/23`. See [Model.rows].
//
// The number can now exceed the limit, and it is left saying so rather than
// clamped. A held bit is not foldable, so a view can sit at 14/12 with the bar
// full and nothing happening — and the reading a person needs there is the true
// count, because the gap between it and the limit is how much they are holding
// back. The bar itself stops at full, since there is no such thing as more than
// a full bar.
//
// **There is a second way to reach that reading and it takes no votes at all.**
// The limit is [Model.budget] and the budget is the screen's own height, so
// making a window shorter lowers it under a view already standing: seen at
// 100x30 dragged to 100x18, the footer read `23/12` with every row above the
// keep drawn cooling and nothing folding. That is the honest report — eleven rows
// over, and the next thing said takes them — and it is why nothing folds on a
// resize. The gap between the two numbers is the size of what the next keystroke
// will cost, which is the most an antecedent can say.
//
// blocked is the case the bar cannot draw at all: over the limit *and* nothing a
// fold can take, because no run in the window reaches two (D32). A full gauge
// that never fires teaches exactly the distrust this surface exists to prevent,
// so the word goes on screen. It is the last thing dropped after the number, and
// before the number there is nothing left worth printing.
//
// **The cause is the size rule, not holds specifically**, and this sentence said
// holds until a review found the difference on a frame. Holds are how it is
// normally reached — every unheld bit in the window with a held bit either side —
// but any arrangement that leaves no run of two does it, and one of them needed
// no vote at all: a keep one bit short of the view, which made the window a
// single bit. That printed `held` on a record with no ballot in it. Fixed at the
// source in [Model.keep]; the wording is corrected here because a reader who
// believes this line will go looking for a vote that does not exist.
func gauge(n, limit, width int, blocked bool) string {
	if width < 1 {
		width = 1
	}
	filled := min(n*width/max(limit, 1), width)
	bar := strings.Repeat("▓", filled) + strings.Repeat("░", width-filled)

	style := dim
	if filled >= width {
		style = warm
	}

	tail := fmt.Sprintf(" %d/%d", n, limit)
	if blocked {
		tail = fmt.Sprintf(" %d/%d held", n, limit)
	}
	return style.Render(bar) + dim.Render(tail)
}
