// Package tui is the human surface onto a [memory] record.
//
// The organizing idea: a harness that forgets silently teaches you to stop
// trusting it. So this one shows its own memory working. Bits about to be
// folded away fade before they go, folds leave a visible scar with a receipt,
// and a gauge shows how close the next fold is. The machine still does the
// work — the human is never asked to manage memory by hand — but nothing
// happens behind their back, so their judgement stays in the loop.
//
// The fade is space before it is colour, and that order is deliberate. A row the
// next fold will take steps two columns left into the margin the caret reserves,
// and is *also* drawn dimmer. Colour is the first thing a terminal takes away —
// NO_COLOR, TERM=dumb, a pipe, a screenshot — and for as long as the fade was
// colour alone, a cooling row and a hot row were the same bytes, so on every one
// of those the fold arrived with no antecedent at all and this paragraph was
// quietly false. See [caretCell] for where the column comes from and
// [TestTheFadeIsDrawnInSpaceAndNotOnlyInColour] for what holds it there.
//
// The fade has two holes in it, and they are stated here rather than left to be
// found, because a promise with an unmentioned exception is worse than a
// narrower promise honestly made.
//
// The first is a hold expiring. A hold decays against the conversation's own
// clock, so a single write can carry the record past an expiry and fold in the
// same step, with no frame in between — and what goes without having faded is
// then the held bit itself, or a lone bit that D32's size rule was sparing only
// while the holds either side of it stood. Closing it would mean deciding a
// hold against the instant of a write that has not happened yet, which is not
// knowable. [Model.absorbing] states it exactly and
// [TestAnExpiringHoldIsTheOneHoleInTheFade] holds it to its size.
//
// The second is a scar. A fold absorbs a cold bit like any other — D32's size
// rule counts it — so a scar goes into the set the next fold will take, and
// nothing on screen says so in either channel: it cannot step, being already at
// the left edge, and [transcript] draws every scar in seamInk whether it is going
// or not. It stays open deliberately, and it is the smaller hole: measured, a
// scar in a conversation nobody has voted in is *always* in the next fold, so a
// mark for it would never be off and would teach nothing — and a merge takes
// nothing away from a reader anyway, since [memory.Cool] flattens the absorbed
// set and sums the counts, so the merged scar's receipt names everything the old
// one named. What goes without warning is the row, not the record. The
// measurement, and the shape that was drawn and rejected, are in this seat's
// craft record.
//
// So the claim this surface actually makes: nothing anybody said is folded
// without having been drawn going first, unless a hold ran out in the write that
// folded it — and a scar merged into a larger scar goes without warning.
// [TestNothingIsAbsorbedWithoutFadingFirst] holds the record side of that and
// draws no frame, which is why the second hole survived being asserted about;
// [TestTheFadeIsDrawnInSpaceAndNotOnlyInColour] holds the screen side, so closing
// either hole fails a test and this paragraph has to change before it passes again.
//
// The other half of that sentence is the vote, and it is what this surface is
// for rather than a feature on it. Watching a fold coming is only worth
// anything if there is something a person can do about it, and the something is
// one key: a caret rides the newest bit, up and down move it, and shift+up says
// keep this. A kept bit does not fade and the fold splits around it. That is the
// whole of the interaction — one mark, two keys, no mode — and it is the cheapest
// possible act, which is the point (D4): the human stays in the loop by spending
// a keystroke, not by managing memory.
//
// What a vote is, precisely: a bit. It goes into the same append-only store, on
// the same channel, addressed by its content, and it can be walked back to from
// the bit it was cast on. Nothing on this screen is stored anywhere else, and
// nothing stored here is ever revised — changing your mind casts another vote and
// the record keeps both.
//
// One vote keeps two rows, and the second one is drawn differently from both the
// first and from a row nothing is keeping. [memory.View.Sparing] also spares the
// bit a held bit names through Prev, so a vote keeps the row that was in front of
// the speaker when they wrote — otherwise the fold takes it and leaves the kept row standing
// between two scars, which is the product making its own screen worse at the one
// gesture it rests on. That row is *covered*, not voted on: it draws a [tie], half
// a stroke in the mark column hanging down into the ▲ on the row below, and never
// a ballot glyph, because nobody cast one on it. Three states in one column, and
// which of them a person did is the one that is a triangle. [voteCell] carries the
// argument for the shape and what was rejected.
//
// **The tie says a position and not a relation, and the difference is the whole
// of what it is allowed to mean.** Prev records the head of the view at the moment
// a bit was written. In an alternating conversation at this keyboard that is the
// turn being answered, which is why it is tempting to draw the pair as a question
// and its answer — and it is not what the record says. Anything written from
// outside the surface takes whatever happened to be newest: measured on this
// project's own record, 7 of 29 said bits (24%) came from `tldr say`, and one of
// them is a correction whose Prev is a greeting rather than the claim it
// corrects. So the tie's meaning is the one that is true every time — *the row
// below is what is keeping this row out of the next fold* — and the surface says
// nothing about why the two were adjacent. Anything built on this edge inherits
// that limit; see [Model.covered].
//
// # When the fold happens, and where it cuts
//
// Both were constants and neither is now. How much the view holds is
// [Model.budget], the rows the terminal has — twelve, at every size, until this;
// measured beforehand, a 100x30 screen showed twelve bits in a twenty-three-row
// frame and a 200x80 screen showed the same twelve in seventy-three. Where the
// fold cuts is [Model.keep], half the budget moved back to the last thing the
// human said, because [memory.View.Fold] counts and has no notion of a round:
// measured before it moved, 24 of 60 frames on a one-voice conversation held a
// round with one half of it behind a scar, and the head of the view was a reply
// to a question nobody could see.
//
// Neither reads the caret, and that is the load-bearing part rather than a
// detail. The caret's row is the one row here drawn whole, so a budget counted in
// rows *as drawn* would make a fold something a person causes by pressing an
// arrow — memory forgetting because of a navigation gesture, with a gauge that
// could only report the cursor. Nothing folds on a resize either: making a window
// shorter raises the gauge and fades the rows the next fold takes, and then
// waits. Dragging a window is not a memory operation.
//
// **What this used to leave broken, and what is left of it.** A hold splits the
// fold at the held bit, and that cut belongs to [memory.View.Fold] rather than to
// this package — so upvoting a row used to fold the row in front of it away and
// leave the kept one standing between two scars, in about nine frames in ten at
// any real vote rate. That is closed: [memory.View.Sparing] also spares the bit a
// held bit names through Prev, the same measurement re-run comes back 0.0%, and
// the spared row draws a [tie] so a person can see which keystroke did it. This
// paragraph described the defect as open for two sessions after it was fixed,
// which is worth leaving in the file rather than tidying away.
//
// What is genuinely left is one name, not a chain. A hold reaches exactly the one
// bit its held bit names, so a third turn further up is folded as it always was, and the row the tie
// points at is a *position* rather than a relation — see [Model.covered]. Both are
// in docs/DEBT.md.
//
// # Reading one whole bit
//
// The row the caret is on shows its whole message. Every other row is one line,
// cut at the margin with an ellipsis. Move the caret and the row you land on
// opens; move off and it closes.
//
// It takes no key, and that is the argument rather than a saving. The object
// that needs a key here is, by definition, the object with no room left to
// advertise one: a row is truncated exactly when it has run out of columns, so
// there is nowhere on it to print the key that would open it. A scar can carry
// ctrl+u because a scar's row is short. The footer cannot carry it either —
// measured, inserting an entry costs the live rung a working key, and the ranked
// view's own label already loses its rung at eighty columns. So the only
// affordance a full row has left is the caret already sitting on it, and that
// caret is already the thing a vote lands on. What you can read is what you can
// vote on, which is one sentence to learn instead of two.
//
// It is also why the common case costs nothing at all. The caret rides the newest
// bit, and [Load] puts it on the newest bit of a record read back from disk — so
// the answer that just arrived, and the last thing said when the program is
// opened again tomorrow, are whole without anybody pressing anything.
//
// What this closes, stated exactly, because the surrounding claim is bigger than
// the fix. A `…` is an antecedent with no receipt: it is the one cut on this
// surface that is visible and unfollowable, where a fold leaves a scar with a key
// on it and a receipt names every address it stands for. The record was never the
// problem — the store holds each bit whole, and [Model.turns] hands it whole to
// the model on the very next request, so until now the other participant in the
// conversation could read more of the record than the person at the keyboard.
//
// It holds on both surfaces, in two shapes, and the difference is the columns
// rather than a preference. A transcript row *is* a sentence — the lead is a
// margin and a handle — so it wraps where it stands and the continuation lines
// hang under the sentence, where a blank handle column says "still the same
// speaker". A ranked row is a *reference*: an ordinal, an address, a clock, a
// mark, a handle, and a preview at the end, which measured on a hundred-column
// frame is forty-four columns before the sentence starts. Wrapping in place there
// would repeat forty-four blanks on every line of an answer, so instead the
// reference stays whole and the material is quoted underneath it in the gutter,
// at the width of the terminal — which is what [unfold] already does beneath a
// scar, one row instead of many.
//
// Both shapes stop at a floor, and the floors are on different columns for the
// same reason the shapes differ: each is a floor on the width that shape wraps
// into. Under it the row is drawn cut, as it always was, which is a degradation
// somebody can see rather than a screen of ellipses.
//
// Four things it does not do, all of them true today:
//
//   - A bit behind a scar is one cut line inside its receipt and cannot be
//     opened there — the caret walks the view, and a receipt's rows are not in
//     the view. Reading one whole means leaving for ctrl+t, which lists every bit
//     anybody said whether a fold took it or not; on any conversation long enough
//     to have folded, most of that list is material the transcript can no longer
//     show. So what is left of this is one surface away rather than unreachable.
//   - A scar in the ranked list does not open at all, because what it would open
//     into is the row it already draws — and ctrl+u does nothing on that screen,
//     so the one row whose subject is absent material is the one row there that
//     cannot be followed to it.
//   - The line breaks a message was written with are gone. [saidWhole] wraps the
//     collapsed text, so every word the record holds is on the screen and its
//     shape is not.
//   - A tall block open while a fold fires underneath it is a frame nobody has
//     looked at. The order the two are framed in is stated in [Model.sync]; what
//     that looks like is not.
//
// # The second surface
//
// ctrl+t swaps the transcript for a ranked reading of the record: everything
// anybody has said, plus anything else somebody at this keyboard voted on, in
// three bands — kept, let go, and not judged — with a clock and a content address
// on every row. It is one key, one caret and no new glyphs, and the same key
// comes back.
//
// It ranks over the *record* and not over the view, which is the difference
// between retrieval and theatre: most of what was said is behind a scar by the
// time anybody wants it again, and reordering the rows already on screen would be
// a shuffle of things you can see. Three consequences follow, and each one is a
// reason this key exists. The list does not move when a fold fires, so it is the
// one surface here that is stable while the transcript collapses. An upvote on
// this screen does something an upvote on the transcript cannot: a bit folded an
// hour ago is held out of nothing, and voting on it still moves the row. And it
// is the only place a folded bit can be read whole. A receipt already gives one
// its address, its clock and its speaker, and cuts it to a line; the caret cannot
// be walked into a receipt, so nothing there opens. Here the row opens.
//
// **What it does not rank is still what nobody has read; that is a heading now
// rather than a filter, and the difference is worth stating as the reversal it
// is.** With one person's plus or minus one as the only signal, most of this list
// is placed by the tiebreak, which is time, which is the transcript with the rows
// shuffled — so this surface does not allocate attention nobody has spent, and it
// does not discharge D3. Closing that needs a second signal, not a cleverer sort.
// What changed is how that gets said. It used to be said by leaving the rows out,
// which is a screen answering "what is on this record" with only the part somebody
// already had an opinion about: measured, the filter hid the correction to the
// very claim sitting at rank 1, because nobody had voted on the correction, while
// `tldr top` — the same reading with no screen to fit — printed it. The honest
// report of a thin signal is a list that says how much of its own order a person
// decided, which is what [band] writes over each run of rows, and not a shorter
// list. [Model.judged] carries that measurement and the two exclusions.
//
// One qualification on the fade's promise above, since this screen is a way to
// not be looking at the transcript. A fold that fires while the ranked view is up
// still draws its antecedent — [Model.draw] runs either way — but it draws it on a
// screen nobody is watching. What crosses is the gauge: it is in the footer of
// both surfaces, it is drawn as a bar as well as a colour, and it is climbing the
// whole time. The fade itself does not cross, and its columns are not spent on
// anything else here — a homonym in the one channel that survives a terminal with
// no colour is worse than an absent fact.
package tui

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/tyler-j-chrestoff/tldreddit/memory"
	"github.com/tyler-j-chrestoff/tldreddit/persona"
)

const (
	// coolFloor is the smallest fold budget there is, and the budget a terminal
	// too short to set one of its own gets.
	//
	// How many bits the view holds before it folds is [Model.budget] now, and it
	// is the screen's own height. This is the number that used to be the whole
	// answer, at every terminal size — so flooring here is the promise that no
	// terminal is worse off than it was before the budget existed, and only
	// terminals with rows to spare change at all.
	//
	// Twelve rather than a rounder number for that reason and no other. It is not
	// re-derived here: it is the value this surface folded at from the beginning,
	// kept so that the change has one direction. What is measured is that going
	// *below* it is bad — over 400 bits at one every 3.5 seconds, a budget of 12
	// folds 65 times, 7 folds 99 times and 3 folds 199 times, which is a receipt
	// every other bit and a scar that stands for almost nothing. See
	// [TestTheBudgetNeverFallsBelowWhatThisSurfaceAlwaysFoldedAt].
	coolFloor = 12

	// channel is the only channel this surface speaks on. Cool refuses to fold
	// across channels, and that guard is load-bearing rather than fussy.
	channel = "tui"

	// holdFor is how long one upvote holds a bit out of a fold, measured in the
	// conversation's own time — see [memory.Stay].For, which decays it against
	// [memory.View.Latest] rather than against a wall clock.
	//
	// MEASURED, NOT DEFAULTED, and the difference is the whole point of the
	// number. [memory.DefaultHold] is thirty minutes, calibrated in that
	// package's own fixtures against one bit a minute. This surface is not that
	// conversation: the live run behind the demo page recorded 343 bits in about
	// twenty minutes, one bit every 3.5 seconds, so thirty minutes of
	// conversation time here is on the order of five hundred bits. A hold that
	// outlives five hundred bits is a pin rather than a hold, and a pin is D31's
	// inverted-D4 failure coming back: the human who participates most is the one
	// whose view stops folding.
	//
	// The schedule, because a figure with no schedule attached is the figure D36
	// exists to have withdrawn. Fixture: alternating human and model bits every
	// 3.5 seconds, 400 of them, budget 12 and keep 6, the human upvoting one bit
	// in every N — driving this program's own trigger through [foldable] rather
	// than a fixture's restatement of it. **Those two are [coolFloor] and its
	// half, so this whole table is the schedule at the shortest terminal there
	// is.** A taller one folds later and keeps more, so every row here is a floor
	// on the rows a real screen shows rather than a figure for it; re-run
	// [TestHarnessHoldSchedule] at the budget in question rather than reading
	// across. The figure is the worst view length
	// reached at any point in the run, in rows, with the number of folds beside it
	// so the row count is never read alone (D36 again).
	// [TestHarnessHoldSchedule] prints the whole sweep, at this cadence and at 2s
	// and 10s either side of it.
	//
	//	 hold │ 1 in 2 │ 1 in 5 │ 1 in 10 │ 1 in 25
	//	  30s │     18 │     16 │      14 │      14
	//	   1m │     22 │     19 │      16 │      15
	//	   2m │     36 │     26 │      20 │      16
	//	   5m │     87 │     46 │      30 │      20
	//	  30m │    400 │    168 │      91 │      44
	//
	// The bottom row is the reason this constant exists. At thirty minutes and one
	// vote in two, four hundred bits produce *zero* folds — the view is every bit
	// ever said, because no run of two unheld bits survives anywhere for a fold to
	// take. That is D31's inverted-D4 failure exactly: the person who participates
	// most is the one whose screen stops working, on the value memory ships as its
	// default.
	//
	// Two minutes, and the trade is worth stating rather than hiding, because the
	// measurement does not hand over a clean answer. One minute is tighter on rows
	// — 22 at the heaviest plausible voting, which is one screen, and it is what
	// D31's own criterion would pick. It was rejected on the other cadence: a
	// slower model answering every ten seconds turns a one-minute hold into about
	// six bits, which is no more than the keep at the floor, so the vote holds
	// nothing that was
	// not staying anyway and the key does nothing anybody can see. A vote with no
	// visible effect is a worse failure than a screen and a half of rows, because
	// the visible effect is the entire argument for asking for the vote (D4).
	//
	// So: at the measured cadence, two minutes costs 36 rows in the worst case
	// somebody who upvotes every model reply will see — a screen and a half, on a
	// screen that says how much is past its edge — and 20 rows at the one-in-ten
	// rate D36 calls plausible.
	//
	// It is a schedule and not a law, and one constant cannot serve a cadence that
	// varies by five times. The way to change it is to re-run the sweep at the
	// cadence in question, not to reason about it from here.
	holdFor = 2 * time.Minute

	// chrome is the row budget for everything that is not the transcript:
	// header, two edges, composer, footer. That enumerates to seven — 1 + 2 + 3
	// + 1 — and it was 8 for long enough that three separate claims about the
	// consequence were written down, each derived from the constant rather than
	// read off a frame, and each one high.
	//
	// So this one is read off frames. [TestHarnessFits] prints, at every size
	// worth looking at, how many rows the frame drew against how many the
	// terminal has: they are equal from height 8 up, and at 7 and below the frame
	// is 8 rows and overflows, because the viewport is clamped to one row and
	// seven of chrome plus one is eight however short the terminal gets. That
	// floor is unchanged by this constant — what changed is that every terminal
	// above it now uses the row this was wasting.
	chrome = 7

	// gaugeWidth is the widest the pressure bar is drawn. It shrinks with the
	// terminal rather than being dropped, because the gauge is the antecedent
	// for a fold that fires on its own, and an automatic operation with no
	// visible antecedent is the thing this surface exists to prevent.
	gaugeWidth = 12
)

// Channel is the channel this surface speaks on, and therefore the channel a bit
// written into its transcript from anywhere else has to carry.
//
// Exported because the alternative is a literal in cmd/tldr, and this one is not
// cosmetic: [memory.Cool] panics on a window spanning two channels, so a bit
// appended to this surface's view on some other channel is not a display oddity
// but a crash in the next fold, in a program a person is in the middle of using.
// One statement of it, asked for rather than remembered.
func Channel() string { return channel }

// Model is the whole application state.
type Model struct {
	// store is the record and it only grows. It is a pointer, so every copy of
	// this Model shares it — which is safe only because a content-addressed
	// store is append-only: a copy can see bits a sibling added, never a bit
	// that changed under it.
	store *memory.Store

	// shown is the view: which bits are on screen and in what order. It is a
	// value, so Update stays a pure function of the Model it was handed, and
	// folding a copy cannot fold the original.
	shown memory.View

	// votes is the second view over the same record: every vote cast on this
	// surface, in the order they were cast. Two views, one store, which is the
	// arrangement D18(e) describes and the reason neither of them needs a lock —
	// each is a value and every copy of this Model gets its own.
	//
	// It is never folded. A folded vote view puts a [memory.Compaction] at its
	// head, [memory.Tally] panics on it, and the failure mode it panics to avoid
	// is worse than the panic: a vote view that reports no votes lifts every hold
	// at once, silently. Nothing here folds it and nothing here may.
	//
	// Order is part of its meaning rather than an artifact — two votes by one
	// voter on one target at the same instant are settled by which comes later
	// here — so it is appended to and never rebuilt.
	votes memory.View

	// mark is the content address of the bit the caret is on: the one bit a vote
	// would land on. Exactly one, always, whenever there is anything to mark.
	//
	// An address rather than an index, because an index is a claim about a
	// position in a view that folds under it. Held by address, the caret follows
	// its bit onto the scar that absorbs it — which is the honest thing for it to
	// do, since the material is still there and the scar is how you reach it —
	// where an index would have silently come to mean a different bit.
	//
	// It is display state and it is not a bit. Where the caret sits decides
	// nothing about the record; pressing a key with it there writes a vote, and
	// the vote is the bit.
	mark string

	// ranked is which of the two surfaces is up: false for the transcript, true
	// for the ranked reading of everything anybody said. Display state,
	// in the same sense as unfolded — the record and both views read the same
	// either way, and nothing here decides any bit, address or order.
	//
	// A toggle rather than a mode, on [Model.unfold]'s argument: the same key
	// goes both ways, so there is no state to be stranded in and nothing to learn
	// past "press it again." The footer names the destination rather than the
	// state, which is the only "which screen am I on" anybody has to be told.
	ranked bool

	// unfolded is whether the scars on screen are currently showing what they
	// stand for. It is display state and nothing else: no bit, no address and
	// no order is decided here, so the record and the view read the same either
	// way. If this field could change what m.shown holds it would be an undo
	// button, and there is no undoing a fold — only following it.
	unfolded bool

	// save is how anything that happens here outlives the process, and nil is a
	// session that does not — which is what [New] builds and what every test in
	// this package runs. See [Save] for the boundary it draws and [Model.saved]
	// for when it fires.
	save Save

	// persona is the other participant, and ollama is how it is reached. The
	// client is a pointer, so every copy of this Model shares one — safe for
	// the reason persona.Client documents: its fields are read at call time and
	// nothing writes them after New.
	persona persona.Persona
	ollama  *persona.Client

	// waiting and trouble are the two things that can be true about a request
	// that is not a bit: one is in flight, or the last one failed. Both are
	// display state in the same sense as unfolded. A reply that has not arrived
	// did not happen, and a failure is a fact about this harness rather than
	// about the conversation, so neither may be written to the record.
	//
	// epoch numbers the requests so that an answer arriving after the human
	// called it off can be told apart from the one they are waiting for.
	waiting waiting
	trouble notice
	epoch   int

	viewport viewport.Model
	composer textarea.Model

	// anchors is where the caret and the first scar landed in the transcript
	// last drawn, as counted by the thing that drew it. It is a cache with a
	// one-frame life: draw sets it, sync and unfold scroll by it, and nothing
	// else reads it. See [anchors] for why the renderer is the one counting.
	anchors anchors

	width, height int
}

// New returns a Model over an empty record: a session where nothing has been
// said yet.
//
// It is [Load] over a store nobody has written to, rather than a second
// constructor, so there is one place the widgets are set up. Two constructors
// would be two statements of one arrangement, and the one that drifts is the one
// nobody runs in a test — which here would be the loaded session, since every
// test in this package starts from New.
//
// Nothing it builds is saved anywhere. That is not an omission left for a caller
// to fix — a session over an empty store with nowhere to put it is exactly what
// a test wants, and the tests are the caller this constructor exists for.
func New() Model { return Load(memory.NewStore(), nil, nil, nil) }

// Load returns a Model over a record that already exists: the store and the two
// views a previous session wrote, read back before the program starts.
//
// Both views are taken and neither is derivable. shown is where forgetting has
// already happened — which bits were dropped and which runs were folded is not a
// function of the record ([memory.View.WriteAgainst] says why it is persisted at
// all). votes is the only place their *order* survives, and the order decides
// which hold stands ([memory.Stay].Votes), so a vote view rebuilt by scanning the
// record for votes would have no order to rebuild from.
//
// The caret lands on the last bit of shown, which is where it would be if that
// bit had just arrived: [Model.utter] puts it there and [Model.riding] is what
// keeps it there. Any other landing spot and the first thing a returning reader's
// vote key does is act on a row they did not choose.
//
// Nothing here checks that the views resolve against the store. [memory.View.Bits]
// panics on an address the store does not hold, which is the correct alarm for a
// view and a record that came from different places — but the caller that read
// them off a file is the one holding the filename, so it is the one that can
// report it as an error instead. See cmd/tldr.
//
// save is how what happens next gets written back, and a nil one is a session
// that is not persisted at all. Handing it in here rather than at each write is
// the point of [Save]: this package holds a function, and where a record lives
// stays with the caller that read it.
func Load(store *memory.Store, shown, votes memory.View, save Save) Model {
	if store == nil {
		panic("tui: Load with no store; every address on this surface is resolved against one")
	}

	ta := textarea.New()
	ta.Placeholder = "say something"
	ta.Prompt = openPrompt
	ta.CharLimit = 4000
	ta.ShowLineNumbers = false
	ta.SetVirtualCursor(false)
	ta.SetHeight(3)
	ta.Focus()

	// The cursor line highlight fights the fade, which is the one signal this
	// UI cannot afford to lose.
	s := ta.Styles()
	s.Focused.CursorLine = lipgloss.NewStyle()
	ta.SetStyles(s)

	// Enter sends. A composer that swallows Enter for newlines makes the
	// common case cost two keys to save the rare one.
	ta.KeyMap.InsertNewline.SetEnabled(false)

	// The transcript scrolls on keys the composer cannot want. Left and right
	// belong to the cursor; the bare letters the viewport binds by default
	// (u, d, f, b, j, k, space) belong to whatever the human is typing, and
	// binding them here means a message with a space in it scrolls the record
	// out from under its author. Half-page is dropped rather than rebound:
	// ctrl+u is the key that reaches the record now, and two meanings for one
	// key is one meaning too many.
	vp := viewport.New()
	vp.KeyMap.Left.SetEnabled(false)
	vp.KeyMap.Right.SetEnabled(false)
	vp.KeyMap.HalfPageUp.SetEnabled(false)
	vp.KeyMap.HalfPageDown.SetEnabled(false)
	//
	// Up and down are gone from here entirely now: they move the caret, and
	// Update takes them before this ever sees them. Leaving them bound would be a
	// second meaning for a key that already has one, which is the same reason
	// half-page went.
	vp.KeyMap.Up.SetEnabled(false)
	vp.KeyMap.Down.SetEnabled(false)
	vp.KeyMap.PageUp = key.NewBinding(key.WithKeys("pgup"))
	vp.KeyMap.PageDown = key.NewBinding(key.WithKeys("pgdown"))

	m := Model{
		store:    store,
		shown:    shown,
		votes:    votes,
		save:     save,
		ollama:   &persona.Client{},
		persona:  defaultPersona(),
		composer: ta,
		viewport: vp,
		width:    80,
		height:   24,
	}
	if len(shown) > 0 {
		m.mark = shown[len(shown)-1]
	}
	m.layout()
	m.sync()
	return m
}

// Views is the pair of views this model holds, for a caller that has to write
// them down when the program ends.
//
// Views and not the store, although the caller wants all three: [Model.store] is
// a pointer every copy of this Model shares, so whoever handed it to [Load]
// already has it and has seen every bit put in it. A view is a value, so the only
// place the last one exists is the Model the program returns — which is why there
// is an accessor at all rather than a field somebody stashes on the way past.
func (m Model) Views() (shown, votes memory.View) { return m.shown, m.votes }

// defaultPersona is who this surface talks to, and there is nobody else to talk
// to: no path through cmd/tldr that opens this surface takes a flag, so there is
// nowhere to make a choice. Its non-interactive verbs do take flags, and that
// does not weaken this — a flag there belongs to the verb it follows and never
// reaches a session, which is the line cmd/tldr's dispatch draws. It was exported
// and took a model name, for a flag that does not exist and a caller outside this
// package that does not exist either — D5's mistake, in miniature. The day a flag
// arrives on the session itself, this grows a parameter and one call site
// changes.
//
// Temperature is written down rather than left to a default, because
// persona.Persona says to: a voice that depends on a number nobody recorded
// cannot be reproduced, and reproducibility is the only reason to keep a record
// at all. 0.7 is ollama's own default for most models, chosen so that the
// transcript looks like what the same model gives anyone else.
func defaultPersona() persona.Persona {
	return persona.Persona{
		Name:        speakerName(persona.DefaultModel),
		Model:       persona.DefaultModel,
		System:      standingInstruction,
		Temperature: 0.7,
	}
}

func (m Model) Init() tea.Cmd { return nil }

// Update is [Model.update] with the invariant around it: whatever this message
// did to the record, the file matches memory before the frame it produced is
// drawn. [Model.saved] is that step and [checkpoint] is how it decides.
//
// The wrapper is what makes the invariant hold over messages nobody has written
// yet. A save at the end of each branch that mutates is a list, and the sixth
// entry on that list is the one somebody forgets; asking the state whether it
// moved is not a list at all.
//
// It is also the one place in this package that is not a pure function of the
// Model and the message, and that is worth saying out loud rather than leaving
// to be discovered. The purity everything here is tested through is
// [Model.update]'s, which is untouched — and a nil [Model.save] is a session
// that does nothing at this step, which is what every test in this package
// runs.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	was := m.checkpoint()
	next, cmd := m.update(msg)
	return next.saved(was), cmd
}

// update is the surface's whole response to a message, and it is pure: it reads
// the Model it was handed, writes a copy, and touches nothing outside itself.
// It returns a Model rather than a [tea.Model] so that [Model.Update] can reach
// its fields without an assertion on the way past.
func (m Model) update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.layout()
		m.sync()

	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		// Sequenced rather than written as "return m, m.send()". send has a
		// pointer receiver and mutates this copy of the Model, and Go orders
		// function calls in an expression but says nothing about where a
		// non-call operand — the m being returned — is evaluated against them.
		// gc happens to call first, so the one-liner works and has worked; it
		// works by luck. The Model that goes back must be the one send wrote to.
		case "enter":
			cmd := m.send()
			return m, cmd
		// esc clears whatever the machine has put in the human's way, and it is
		// one key because the two things it clears can never both be up: a
		// request in flight, or the failure of the last one. Calling off a wait
		// and dismissing a notice are the same sentence — "I am done with this"
		// — so they are the same key, and there is never a question of which
		// one it means.
		case "esc":
			if m.waiting.live {
				m.settle()
				m.sync()
			} else if m.trouble.up() {
				m.trouble = notice{}
				m.sync()
			}
			return m, nil
		// ctrl+k and ctrl+u are a pair on purpose: adjacent keys for opposite
		// directions on the same object, which is a thing a hand learns once.
		// Both are taken from the composer, where they are readline's kill
		// bindings — a fair trade, since this program's whole subject is what
		// leaves the screen and readline's is what leaves a line.
		//
		// Returning here rather than falling through is what takes them: the
		// composer and the viewport both see every message that reaches the
		// bottom of this function.
		//
		// Both do nothing on the ranked surface, and doing nothing is the
		// decision rather than an omission. ctrl+k is a shortcut for something
		// that happens on its own, and taking it on a screen where the collapse
		// cannot be watched is the precise failure this surface exists to
		// prevent; ctrl+u opens a receipt on a scar in the transcript, and there
		// is no scar on this screen whose corner could say it had opened.
		case "ctrl+k":
			if !m.ranked {
				m.fold()
			}
			return m, nil
		case "ctrl+u":
			if !m.ranked {
				m.unfold()
			}
			return m, nil

		// The second surface, and the same key back. It is free: the viewport
		// binds no ctrl+t and the composer spends it on readline's
		// transpose-two-characters, which is the cheapest thing left in a
		// three-row draft box and the same trade this program already made twice.
		// Unlike a shifted arrow it needs no terminal protocol, so it cannot
		// silently arrive as a different key.
		case "ctrl+t":
			m.rank()
			return m, nil

		// The caret, and the two things that can be said about where it is.
		//
		// up and down are taken from the viewport, which used to scroll a line at
		// a time with them. That is not a loss worth mourning: the caret moves a
		// bit at a time and the screen follows it, so the arrows still scroll —
		// they just scroll by the unit the record is made of instead of by the
		// unit the terminal is made of. pgup and pgdn are still rows.
		//
		// They are taken from the composer too, where they moved the cursor
		// between wrapped lines of a draft. That is a real cost, paid because
		// there is no second pair of arrow keys and because a draft is three rows
		// while the record is the whole screen.
		case "up":
			m.move(-1)
			return m, nil
		case "down":
			m.move(1)
			return m, nil

		// Keep and let go. The shifted arrows are the pair worth learning —
		// the caret moves with up and down, and the vote is the same key with a
		// shift on it, which is one idea rather than two.
		//
		// The aliases are not a convenience. shift+arrow is not reportable on
		// every terminal: it needs a protocol the terminal has to speak, and where
		// it is not spoken the key arrives as a bare arrow, which moves the caret
		// instead of voting. That failure is silent and this program cannot detect
		// it — so there is a second way in that needs nothing of the terminal at
		// all, and the footer names it beside the arrows wherever there is room to
		// print both.
		case "shift+up", "ctrl+o":
			m.vote(memory.Up)
			return m, nil
		case "shift+down", "ctrl+r":
			m.vote(memory.Down)
			return m, nil
		}

	// The three ways a request ends, and the clock under it. Every one of them
	// checks the epoch first: a context that has been cancelled does not stop a
	// request that already succeeded, so an answer can still arrive after the
	// human pressed esc, and writing it down then would be the machine acting
	// after being told not to.
	case replyMsg:
		if !m.mine(msg.epoch) {
			return m, nil
		}
		m.settle()
		m.recordReply(msg.answer)
		return m, nil

	case failedMsg:
		if !m.mine(msg.epoch) {
			return m, nil
		}
		m.settle()
		// A wait the human called off is not a failure and gets no notice. They
		// pressed the key and watched the line go; a banner telling them what
		// they just did is noise, and dismissing it is a second keystroke for
		// nothing.
		var e *persona.Error
		if !errors.As(msg.err, &e) || e.Kind != persona.Canceled {
			m.trouble = explain(msg.err)
		}
		m.sync()
		return m, nil

	case tickMsg:
		if !m.mine(msg.epoch) {
			return m, nil
		}
		m.waiting.elapsed = time.Since(m.waiting.since)
		m.draw()
		return m, beat(msg.epoch)
	}

	var taCmd, vpCmd tea.Cmd
	m.composer, taCmd = m.composer.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)
	return m, tea.Batch(taCmd, vpCmd)
}

func (m Model) View() tea.View {
	// Two numbers, and the gap between them is the product. The view is what
	// this screen is showing; the record is everything that has ever been put
	// in the store, derived bits included. They start equal and diverge at the
	// first fold, which is the moment worth noticing — and they teach the two
	// words the unfolded block then uses for itself.
	//
	// The count of hot bits moved to the gauge in the footer, where the
	// pressure it belongs to already lives. The count of cold ones was never
	// more than one and is on screen as the scar.
	//
	// Both numbers shrink rather than run off the edge. The header was the last
	// row on this screen still built to a width it had not been told, and at
	// twenty columns it read "view 12 · recor" — a truncation the terminal did,
	// not the program, which is the one kind this surface is not allowed.
	//
	// The ranked surface swaps the first of the two for its own, and the two
	// numbers there reconcile in a way they cannot on the transcript: what it says
	// is the number of rows you can count down the bands.
	shown, word := len(m.shown), "view"
	if m.ranked {
		shown, word = len(m.list()), "ranked"
	}
	counts := fit(max(m.width-lipgloss.Width("tldr")-1, 1),
		fmt.Sprintf("%s %d · record %d", word, shown, m.store.Len()),
		fmt.Sprintf("%d · %d", shown, m.store.Len()),
		"")

	// The date, stated once, on the left with the name of the program. Every
	// other clock on this screen is four digits, which cannot say which day a
	// conversation happened on — the largest hole an auditor can poke in this
	// surface, and the cheapest to close, because the record has kept the whole
	// instant all along and only the format strings dropped it.
	//
	// Once rather than per row, because per row is fifteen columns of the same
	// eight characters. A row whose day differs from this one says so itself; see
	// [clock].
	//
	// It goes before the counts on the ladder. The counts are the claim this
	// header makes and the date is the frame around it, and an empty record has
	// no day at all, which is the honest reading of a screen where nothing has
	// happened yet.
	// And the word that says whose day it is. Every clock on this screen is drawn
	// in the reader's own zone ([Model.day]), and a bare `15:04` means a different
	// moment on every machine that opens the same record — which is exactly the
	// question the reader this surface is hardest for arrives with. `local`
	// rather than a zone abbreviation: `MDT` looks like a fact about when it
	// happened, and the record does not know that (D12 keeps the instant and
	// throws the zone away). This says the true and narrower thing, and it reads
	// to somebody who has never heard of a time zone.
	//
	// First off the ladder, because it is the least of the three: a date with no
	// zone is still a date, where a screen with no date at all cannot say which
	// day anything happened on.
	name := "tldr"
	if day := m.day(); !day.IsZero() {
		name = fit(max(m.width-lipgloss.Width(counts)-1, 1),
			"tldr · "+day.Format("2006-01-02")+" local",
			"tldr · "+day.Format("2006-01-02"),
			"tldr · "+day.Format("01-02"),
			"tldr")
	}
	header := m.spread(dim.Render(name), dim.Render(counts))

	above, below := m.offscreen()

	// One exit path, so AltScreen is set once. The renderer diffs each frame
	// against the last, and a branch that forgot it would silently drop out of
	// the alternate screen and scribble over the shell.
	v := tea.NewView(lipgloss.JoinVertical(lipgloss.Left,
		header,
		m.edge("↑", "pgup", above),
		m.viewport.View(),
		m.edge("↓", "pgdn", below),
		m.composer.View(),
		m.footer(),
	))
	v.AltScreen = true
	return v
}

// offscreen is how many transcript rows sit above and below what is drawn.
//
// The transcript has always been taller than its frame — a full hot band plus a
// scar was thirteen rows in the twelve an eighty-by-twenty terminal leaves — and
// until now nothing on screen said so. Unfolding made it unmissable rather than
// new: the block a receipt opens is one row per absorbed bit, so past one fold
// it is taller than any common terminal, and its closing bar sat below the
// margin with the screen looking finished.
//
// The overflow narrowed at the top and widened at the bottom when the budget
// became the screen's own height ([Model.budget]). A view now stops one row past
// the frame instead of eleven, so these arrows are usually reporting a row or two
// — and a *receipt* is longer than it was, because a bigger budget means a bigger
// fold and a scar stands for more bits. Seen in a real terminal: 100x30 dragged
// to 100x18 and one thing said folded thirty-one bits into one scar, whose block
// is three pages.
func (m Model) offscreen() (above, below int) {
	above = m.viewport.YOffset()
	below = max(m.viewport.TotalLineCount()-above-m.viewport.Height(), 0)
	return above, below
}

// edge draws one of the two rules that bracket the transcript, carrying how
// much is past it in that direction and the key that goes there.
//
// It costs no rows, which is the whole reason it is here rather than as a line
// of its own: at eighty by twenty there are no rows to spare, and an indicator
// that only appears on a big terminal is an indicator for the case that did not
// need it. A plain rule now means something it could not mean before — that
// what is between the two rules is all there is.
//
// The key rides on the indicator for the same reason the scar carries ctrl+u: a
// footer is where a returning user looks and the edge is where someone who has
// just noticed there is more is already looking.
//
// With nothing past it, it is a blank row rather than a rule. That is not
// restraint about decoration, it is the one rule this surface has about rules: a
// horizontal line means something is being claimed. The scar is a horizontal
// line making the biggest claim on the screen — this many bits went — and it was
// competing with two rules a row away that claimed nothing, drawn in the same
// dashes and the same colour. Now the only lines on screen are the ones with
// something to say, and the blank row costs nothing because the row was already
// spent.
func (m Model) edge(arrow, key string, n int) string {
	w := max(m.width, 1)
	if n <= 0 {
		return ""
	}

	// The count survives every cut. It is the whole claim: this many rows are
	// past this line, and the screen is not finished.
	tag := fit(w,
		fmt.Sprintf(" %s %d more · %s ─", arrow, n, key),
		fmt.Sprintf(" %s %d more ─", arrow, n),
		fmt.Sprintf(" %s %d ─", arrow, n),
		fmt.Sprintf("%s%d", arrow, n))
	return rule.Render(strings.Repeat("─", max(w-lipgloss.Width(tag), 0)) + tag)
}

// footer is the pressure on the record, and an index of the keys.
//
// When there is not room for both, the gauge wins and the help is cut. The
// gauge is the antecedent for something the machine does without being asked;
// the help is a convenience, and the one key it names that a person cannot
// guess is already printed on the scar it operates.
//
// The cut is drawn through [abridged] rather than [fit], and the difference is
// the whole of what this ladder used to get wrong. Every rung below the first
// drops a *binding*, not a wording, and a key that is not printed is
// indistinguishable from a key that does not exist — at eighty columns this row
// stopped naming ctrl+c and looked exactly like a footer that had it. Every
// ladder here also used to end in "", which fits any width, so the narrowest
// footer on this surface was a blank row rather than a mark. Both are marked now,
// with the same ellipsis every other cut on this screen uses.
func (m Model) footer() string {
	// One reading of blocked for both halves of this row. The gauge prints the
	// word and the ladder answers it, and two readings of one question is how a
	// screen comes to disagree with itself in the direction nobody checks — see
	// [frame], which is the same argument about the transcript.
	blocked := m.blocked()

	// The denominator is [Model.budget] and so it moves with the terminal. That
	// is the honest reading rather than a complication: what the gauge is filling
	// toward is the screen, and a person who makes the window shorter should see
	// it fill, because it has. Nothing folds at that moment — see [Model.budget].
	g := gauge(m.foldable(), m.budget(), min(gaugeWidth, max(m.width/4, 1)), blocked)
	room := m.width - lipgloss.Width(g) - 1

	// The vote is the last key standing, and it displaced unfold to get there.
	// Folding happens on its own, so ctrl+k is only a shortcut; quitting is ctrl+c
	// everywhere; sending is enter everywhere. Following a receipt is still the
	// thing nobody can guess, but it is printed on the scar it operates, at the
	// left edge of the screen, in both states — so the footer is its second
	// mention, while the vote's key is printed nowhere else at all. D4 says which
	// of the two acts is the primary one.
	//
	// While a reply is in flight the ladder changes, and that is the point
	// rather than a nicety: enter does not send then, so an index that went on
	// offering "enter send" would be the screen lying about its own keys. The
	// key that does work takes the place, in the same column, so the index
	// answers the question a waiting person actually has.
	//
	// The aliases go before the keys they alias. They are there because
	// shift+arrow is not reportable on every terminal and this program cannot
	// tell; they are shed first because a person whose terminal does report it
	// never needs them, and dropping seven columns of alias is cheaper than
	// dropping a binding.
	//
	// It used to shed the whole let-go entry at the second rung, and the
	// consequence was measured rather than argued: at eighty columns — the
	// standard terminal, and the one this ladder is written for — the footer read
	// "enter send · shift+↑/ctrl+o keep · ctrl+u unfold · ctrl+c quit" while the
	// gauge beside it read "13/12 held". The one state on this surface that needs
	// a key pressed, named, with the key printed nowhere. Two rungs of alias
	// removal now sit where that cliff was, and ctrl+c quit is what gives way
	// instead, because quitting is the one key on the list nobody has to be told.
	ladder := []string{
		"enter send · shift+↑/ctrl+o keep · shift+↓/ctrl+r let go · ctrl+u unfold · ctrl+c quit",
		"enter send · shift+↑/ctrl+o keep · shift+↓/ctrl+r let go · ctrl+u unfold",
		"enter send · shift+↑/ctrl+o keep · shift+↓ let go · ctrl+u unfold",
		"enter send · shift+↑ keep · shift+↓ let go · ctrl+u unfold",
		"enter send · shift+↑ keep · shift+↓ let go",
		"shift+↑ keep · shift+↓ let go",
		"shift+↑ keep",
	}
	switch {
	case m.waiting.live:
		ladder = []string{
			"esc stop · shift+↑/ctrl+o keep · ctrl+u unfold · ctrl+c quit",
			"esc stop · shift+↑/ctrl+o keep · ctrl+c quit",
			"esc stop · shift+↑/ctrl+o keep",
			"esc stop · shift+↑ keep",
			"esc stop",
		}
	case m.trouble.up():
		ladder = []string{
			"enter send · esc dismiss · shift+↑/ctrl+o keep · ctrl+u unfold · ctrl+c quit",
			"enter send · esc dismiss · shift+↑/ctrl+o keep · ctrl+u unfold",
			"enter send · esc dismiss · ctrl+u unfold · ctrl+c quit",
			"enter send · esc dismiss · ctrl+c quit",
			"esc dismiss",
		}

	// Held so hard it cannot fold, and then the ranking changes rather than the
	// contents. Let go outranks everything but sending, because it is the only
	// key that ends this state; keep is shed first, because it is the key that
	// caused it and pressing it again makes the state worse. The gauge next to
	// this is already saying "held" — this is the half of that sentence that says
	// what to do about it.
	//
	// Last in the switch, so a wait or a failure still takes the footer. Both of
	// those are transient and this is not: a hold lasts holdFor of conversation
	// time, so the ladder comes back the moment the other thing clears, and the
	// keys it names go on working the whole time.
	case blocked:
		ladder = []string{
			"enter send · shift+↓/ctrl+r let go · shift+↑/ctrl+o keep · ctrl+u unfold · ctrl+c quit",
			"enter send · shift+↓/ctrl+r let go · shift+↑/ctrl+o keep · ctrl+u unfold",
			"enter send · shift+↓/ctrl+r let go · ctrl+u unfold",
			"enter send · shift+↓ let go · ctrl+u unfold",
			"enter send · shift+↓ let go",
			"shift+↓ let go",
		}
	}
	return m.spread(dim.Render(abridged(room, m.surfaced(ladder)...)), g)
}

// surfaced puts the key for the other surface into a ladder, in the rung ctrl+u
// used to hold.
//
// It is a swap and not an addition, because there is no room for one: measured on
// a real hundred-column frame, inserting an entry pushes the live rung past what
// the gauge leaves and the whole ladder drops a rung, which costs a key that
// works. Swapping costs nothing there — the rung that is live at a hundred columns
// is the same width with this in it as it was with ctrl+u, read off the frame
// rather than counted out of the strings. Which of the two gives way is decided by
// this footer's own standing rule — a key printed nowhere else outranks one
// printed on the object it operates — and ctrl+u is printed on the scar it opens,
// in both states, at the left edge. ctrl+t is printed nowhere at all.
//
// The two labels are not the same width, and the ranked one is wider, so there is
// a band of terminals where the transcript names the way in and the ranked view
// does not name the way back. It is narrow and it is real: at eighty columns the
// rung carrying it misses by a single column and the whole entry goes. Recorded
// rather than fixed, because every fix costs something worse — a shorter word
// stops naming a destination, and buying the rung back means shedding a vote key
// on the surface whose subject is voting. [TestHarnessRanked] prints both footers
// at four sizes, which is where to look before changing this.
//
// It names the destination rather than the state: "ctrl+t ranked" on the
// transcript, "ctrl+t transcript" on the ranked view. A toggle whose label named
// where you already are would be the one thing a person cannot check, and this
// pair is the whole of what anybody has to be told about which screen is up.
//
// It sheds with its rung on both surfaces, and being stranded is not the risk it
// looks like. A version of this held it back to the narrowest rung on the ranked
// view, on the argument that a screen with no visible way out is a mode; the
// frames said otherwise. At sixty columns that arrangement printed "shift+↑ keep
// · ctrl+t transcript" in place of "shift+↑ keep · shift+↓ let go", spending a
// vote key to name the way back — and the only way onto this screen is the key
// that leaves it, since the program starts on the transcript. Nobody arrives here
// without having pressed it.
func (m Model) surfaced(ladder []string) []string {
	key := "ctrl+t ranked"
	if m.ranked {
		key = "ctrl+t transcript"
	}
	for i, rung := range ladder {
		ladder[i] = strings.ReplaceAll(rung, "ctrl+u unfold", key)
	}
	return ladder
}

// spread puts left and right on one row, pushing right to the margin, and cuts
// the result if the two together will not fit.
func (m Model) spread(left, right string) string {
	gap := max(m.width-lipgloss.Width(left)-lipgloss.Width(right), 1)
	return clip(left+strings.Repeat(" ", gap)+right, m.width)
}

// send records what is in the composer as a new bit under the local handle, and
// asks the persona to answer it.
//
// A second send while a reply is in flight is held, not queued and not sent.
// Three things were possible and two of them are worse. Sending anyway puts two
// questions in front of a model that will answer them as one, and leaves a
// record where the second reply appears to answer the second question and does
// not — a permanent, invisible falsehood in an append-only store. Queueing is
// nearer, and it fails on this surface's own rule: the reply lands, and the
// queued line fires without the person looking, so their words go to the model
// after they have read an answer that might have changed them. That is the
// machine acting behind their back, which is the one thing here that is not
// negotiable.
//
// So the send is held, and the holding is visible before the key is pressed
// rather than discovered by pressing it: the pending line says "enter is held
// until it answers", the composer's prompt goes from solid to dashed, and the
// footer stops offering "enter send". Typing is untouched — a person waiting
// twenty-six seconds should be able to draft the next thing, and the draft sits
// in the composer where they can still change it.
func (m *Model) send() tea.Cmd {
	if m.waiting.live {
		return nil
	}

	text := strings.TrimSpace(m.composer.Value())
	if text == "" {
		return nil
	}
	m.composer.Reset()

	// A new question supersedes the last failure: it is about to be answered or
	// to fail the same way, and either way the old notice is stale.
	//
	// This used to be conditional, because one kind of notice could not be
	// superseded by anything — a reply thrown away for being truncated, whose
	// text was not coming back and whose only trace anywhere was the notice
	// itself. Clearing that on a keystroke pressed for an unrelated reason
	// erased the last evidence that a reply had arrived at all. A truncated
	// reply is a bit now, so the exception went with the hazard.
	m.trouble = notice{}
	m.say(localHandle, text)

	// The caret comes back to what was just said, wherever it had been parked.
	// Speaking is the one act that says where a person's attention is without
	// their having to move anything, and a caret left twenty rows up after you
	// have typed a sentence is a vote key aimed at something you stopped looking
	// at. It is the only place anything overrides where the caret was put, which
	// is why it is here and not in utter: a reply arriving is the model's doing
	// and must not move it.
	//
	// Not while the ranked surface is up, and for the same reason turned around.
	// What was just said is not on that screen — nothing is, until somebody votes
	// on it — so bringing the caret to it would take the caret off the list the
	// person is looking at and put it somewhere they cannot see. Speaking says
	// where their attention is only when the thing they said is drawn.
	if len(m.shown) > 0 && !m.ranked {
		m.mark = m.shown[len(m.shown)-1]
		m.sync()
	}

	return m.begin()
}

// say records text from h as a new bit, and folds if that pushed the hot band
// over the limit. Folding on write rather than on a timer keeps the program
// free of background state: pressure only ever changes here.
//
// The handle is a parameter rather than a constant because the human is not the
// only speaker for much longer. Every other part of this surface already treats
// the handle as data — the scar merges them, the unfolded block aligns them —
// and this was the last place that assumed there was one.
func (m *Model) say(from memory.Handle, text string) {
	m.utter(from, memory.Utterance{Text: text})
}

// utter is say for an utterance that is not just its text, and it is the one
// place a bit is written. Pressure only ever changes here.
//
// It exists because a truncated reply is recorded rather than refused: a
// [memory.Utterance] with Truncated set is a different bit at a different
// content address (D26), and recordReply needs to write one without a second
// copy of the fold-on-write rule. The payload is an Utterance rather than a
// [memory.Payload] on purpose — a Compaction is derived by [memory.View.Fold]
// from bits that already exist, and nothing on this surface may mint one.
func (m *Model) utter(from memory.Handle, u memory.Utterance) {
	riding := m.riding()

	var b memory.Bit
	m.shown, b = m.shown.Add(m.store, memory.Bit{
		At:      time.Now(),
		From:    from,
		Channel: channel,
		Payload: u,
		Prev:    m.shown.Head(),
	})

	// The caret rides the newest bit until somebody moves it, and stays where
	// they put it afterwards. Both halves matter and they pull opposite ways: a
	// caret that never followed would sit on the first bit of the session for an
	// hour, and a caret that always followed could not be aimed at anything,
	// because the next reply would take it away mid-keystroke.
	//
	// Riding is derived from where the caret is rather than remembered in a flag.
	// A flag would have to be lowered when the caret is moved and raised again
	// when it happens to arrive back at the end, and the path nobody thinks of is
	// the one it goes stale on.
	if riding || m.mark == "" {
		m.mark = b.ID
	}

	if m.pressured() {
		m.fold()
	}
	m.sync()
}

// riding reports whether the caret is on the newest bit in the transcript, which
// is the state in which it follows what arrives next.
//
// Never while the ranked surface is up, and that is the meaning of the word
// rather than a special case bolted onto it. The reason used to be that the
// newest bit was not on that screen at all until somebody voted on it; the list
// is every said bit now, so it is there — and it is somewhere else. Measured on a
// conversation with votes in it, a bit nobody has judged sits at the head of the
// `not judged` band, which is the middle of the list, so following what arrives
// scrolls a reading surface to a row the person did not ask for while their next
// vote key aims at it. On the transcript the same act is the screen keeping up
// with the conversation, because there the newest bit is the bottom row and the
// bottom row is where the eye already is.
func (m Model) riding() bool {
	return !m.ranked && len(m.shown) > 0 && m.mark == m.shown[len(m.shown)-1]
}

// move walks the caret d bits through the view, and stops at the ends rather
// than wrapping. Wrapping would mean one press of a key can take the caret from
// the newest thing on screen to the oldest, and the caret is what a vote lands
// on.
// It walks whichever list is on screen, and it walks it from where the caret is
// *drawn* rather than from the address the mark holds — [Model.caret] is the
// difference, and without it a caret sitting on a folded bit would jump instead
// of stepping off the scar the screen shows it on.
func (m *Model) move(d int) {
	list := m.list()
	if len(list) == 0 {
		return
	}

	at := slices.Index(list, m.caret())
	if at < 0 {
		// The caret is on nothing this surface can show and nothing stands for
		// it. That should not happen, so the recovery is to put it somewhere true
		// rather than to guess: the newest bit, which is where it starts.
		m.mark = list[len(list)-1]
		m.sync()
		return
	}

	m.mark = list[min(max(at+d, 0), len(list)-1)]
	m.sync()
}

// vote records what the human at this keyboard thinks of the bit under the
// caret. It is the cheap act D4 makes the primary one, and it is the only thing
// on this surface besides speaking that writes to the record.
//
// A vote is a bit — [memory.Cast] addresses it and it goes into the same store,
// on the same channel as the thing it votes on — so the record number in the
// header goes up when you press the key. Nothing here is display state and
// nothing here is revised: changing your mind casts another vote, both stay in
// the store, and [memory.Tally] keeps the later one.
//
// Up holds the bit out of the next fold, for holdFor of conversation time. Down
// does not push anything out and is not a negative hold; what it does is end an
// upvote, because [memory.Tally] keeps one standing vote per voter per target
// and the newer one wins. So the two keys are not symmetric, and the screen
// says so without a legend: the mark an upvote leaves has a gauge draining
// beside it, and the mark a downvote leaves has nothing to drain.
//
// It does not fold, and that is the part worth stating plainly, because the
// obvious thing to do here is the wrong one. Letting go of the last hold on an
// over-full view is exactly the moment a fold becomes possible again, and this
// key used to take it on the same frame, so that the rows collapsed under the
// hand that released them. Measured on this program's own fixture, that cost the
// fade outright: with every other bit held the view is blocked, so nothing on
// screen is drawn cooling at all, and one press of let go absorbed three rows
// that were at full brightness — two of them bits nobody had ever voted on. The
// machine performing the one operation this surface exists to make visible, with
// no antecedent, on the keystroke that asked for something else.
//
// So the key withdraws the hold and stops. The rows it frees enter the cooling
// set on this very frame — [Model.absorbing] reads the vote view, and the vote is
// already in it — the gauge stops saying held, and the next thing anybody says
// takes them. You watch the heat leave before it goes, which is this package's
// own sentence, rather than watching rows vanish.
// It votes on the bit the caret is *drawn* on, which is [Model.caret] and not
// always [Model.mark]: a caret whose bit has been folded is shown on the scar
// that absorbed it, and a key that voted on the bit behind the scar instead
// would put the reader's mark somewhere the screen is not drawing it. What you
// see is what you vote on, and on this surface that is not a slogan — a vote
// nobody can see is indistinguishable from a key that did nothing.
func (m *Model) vote(dir memory.Direction) {
	if m.caret() == "" {
		return
	}
	target, ok := m.store.Get(m.caret())
	if !ok {
		return
	}

	m.votes, _ = m.votes.Add(m.store, memory.Cast(time.Now(), localHandle, dir, target))
	m.sync()
}

// fold takes everything but the most recent [Model.keep] bits off the screen and
// puts one cold bit in their place. Only the screen changes: the bits it
// absorbed are still in the store, still addressed the same way, so the scar
// the transcript draws has something behind it.
//
// The one new edge is the cold bit's own: it names every bit it absorbed, so
// the way back into what left the screen is on the record and not only on the
// receipt. Nothing existing is repointed. The surviving bits go on naming
// absorbed bits in their Prev, and that is correct rather than dangling — the
// graph the record keeps is the graph as it happened, and shortening it was
// only ever necessary because folding used to delete.
func (m *Model) fold() {
	shown, folded := m.shown.Fold(m.store, m.keep(), m.stay())
	if !folded {
		return
	}
	m.shown = shown

	// The caret follows its bit into the scar that absorbed it. Prev rather than
	// Absorbed, because Absorbed lists originals only: a caret sitting on an
	// older scar that has just been folded again is named in the new cold bit's
	// Prev and nowhere else (D13).
	//
	// This is the caret being an address rather than an index. An index would
	// still be pointing at a row, and it would be a different row.
	//
	// Only while the transcript is the surface up. The ranked view holds every
	// bit anybody said, and anything else judged, whether or not a fold has taken
	// it, so a caret sitting in that list needs no rescue — and moving it here
	// would take it out from under the hand, onto a scar nobody voted on, which
	// that list does not even draw. What that leaves is a mark on a folded bit
	// when the transcript comes back, and [stands] is what draws it there; this
	// line is the fold's own version of the same rescue, kept because it is the
	// one moment where which scar absorbed which bit is known rather than
	// searched for.
	if !m.ranked && !slices.Contains(m.shown, m.mark) {
		for _, b := range m.shown.Bits(m.store) {
			if _, cold := b.Payload.(memory.Compaction); cold && slices.Contains(b.Prev, m.mark) {
				m.mark = b.ID
				break
			}
		}
	}

	// A fold closes an open unfold, so the collapse is something you watch
	// happen. Folding while the screen stayed full would be the machine doing
	// the one operation this surface exists to make visible, invisibly. The
	// material is one key away and the key is on the new scar.
	m.unfolded = false
	m.sync()
}

// rank swaps the transcript for the ranked reading of the record, and back.
//
// Which bits, and in what order, is [Model.judged] and [memory.View.Rank]; what
// this owns is the caret, because there is one caret and two surfaces and it has
// to be somewhere the surface that is up can draw it.
//
// So entering takes it to the top of the list when it is on a bit this list does
// not hold. That used to be any bit nobody had voted on, which is where the caret
// usually is, so the jump was the common case; now the list is every said bit and
// the jump is the exception. Measured on a conversation nobody has voted in and
// again on one with votes in it, the caret riding the newest bit stays exactly
// where it was — on the first record it lands at the head of the list and on the
// second at the head of the `not judged` band. What is left that this list does
// not hold is a scar nobody voted on, which is where [Model.fold] parks the caret
// when a fold takes the bit it was on.
//
// It moves rather than staying put because a caret that is not on screen is a
// vote key aimed at something the person cannot see, and that is the failure this
// whole surface exists to prevent. It is not behind anybody's back either: the
// caret is visibly at the top of the list in the same frame as the key that put
// it there.
//
// Leaving takes nothing back. The mark may name a bit that has been folded while
// this screen was up, and the transcript draws the caret on the scar that
// absorbed it — see [stands], which is why that rescue had to move to draw time
// rather than staying in [Model.fold].
func (m *Model) rank() {
	m.ranked = !m.ranked
	if list := m.list(); m.ranked && len(list) > 0 && !slices.Contains(list, m.mark) {
		m.mark = list[0]
	}
	m.sync()
}

// list is the view the surface that is up draws, in the order it draws it. It is
// what the caret moves through and what an address is resolved against.
//
// In ranked order and not in [Model.judged]'s order, which is the whole point of
// it being a separate list: the caret steps to the next row *on screen*, and on
// this surface that is the next row the votes put there rather than the next
// thing that was said.
func (m Model) list() memory.View {
	if !m.ranked {
		return m.shown
	}
	order := m.ranking()
	out := make(memory.View, 0, len(order))
	for _, r := range order {
		out = append(out, r.ID)
	}
	return out
}

// caret is the address the caret is drawn on, which is not always the address
// [Model.mark] holds — see [stands]. Every act that asks where the caret *is*
// asks this, so what a key does is what the screen shows.
func (m Model) caret() string { return stands(m.store, m.list(), m.mark) }

// live is [memory.Stay.Holds] narrowed to the bits a hold can still hold back:
// the ones the transcript is showing.
//
// [memory.Stay.Holds] is a function of the vote view and one instant and takes no
// view at all, so an upvote on a bit that was folded an hour ago comes back with
// time remaining. The transcript never noticed because it only ever draws bits
// its own view holds; the ranked view draws bits that left the view long ago, and
// without this it would put a live draining gauge beside a row nothing can hold
// out of anything.
//
// What such a row draws instead is the hollow mark, unchanged, and [voteCell]'s
// existing sentence is already exactly right about it: the vote is still on the
// record, permanently, and the stay of execution it bought is spent. A folded
// bit's stay is spent by definition.
func (m Model) live() map[string]time.Duration {
	held := m.stay().Holds(m.store, m.day())
	for id := range held {
		if !slices.Contains(m.shown, id) {
			delete(held, id)
		}
	}
	return held
}

// covered is the bits a hold is sparing that nobody voted on — the bit each held
// bit names through Prev — which is [memory.View.Sparing] with the holds taken out
// of it.
//
// **It is a positional edge and this package may not draw it as a semantic one.**
// Prev is the head of the view at the instant a bit was written. Both write paths
// set it that way ([Model.utter] and `tldr say`), so in an alternating
// conversation at this keyboard it is the turn being replied to, and outside one
// it is whatever happened to be newest. Measured on this project's own record:
// 7 of 29 said bits came in through `tldr say`, one of them a correction whose
// Prev is a greeting rather than the claim it corrects — so a screen that drew
// this edge as "the question this answers" would have asserted exactly that. What
// the surface says instead is that the row below is what keeps this row out of the
// next fold, which is true of every covered row on every frame. [voteCell] carries
// the shape and this package's doc carries the boundary; anything later built on
// Prev — a parent line, a thread, a jump — inherits the same limit and should be
// argued against this paragraph before it is drawn.
//
// The holds come in rather than being read again here, because the two sets are
// drawn in the same column on the same frame and a bit that appeared in both
// would be a row saying two things about one vote. [Model.frame] takes one
// reading and hands it to both.
//
// **It is not the same question as "is this row staying".** Three different
// things keep a row bright — it is in the kept tail, D32's size rule refuses the
// run it is in, or a hold is sparing it — and only the third is a consequence of
// something a person did. The first two are the fold's own arithmetic and the
// screen already says them by the absence of a fade. This is the one a person is
// owed an antecedent for, because it is the one their keystroke caused.
func (m Model) covered(holds map[string]time.Duration) map[string]bool {
	spared := m.shown.Sparing(m.store, m.stay())
	for id := range holds {
		delete(spared, id)
	}
	return spared
}

// unfold shows, or stops showing, what the scars on screen stand for.
//
// A toggle, not a mode and not a drill-down. The same key opens and closes, so
// there is no state to be stranded in and nothing to learn beyond "press it
// again." It applies to every scar in the view at once — today at most one —
// because "show me what these lines stand for" is a sentence a person already
// holds, while "show me what the third one stands for" needs a cursor, a
// selection highlight, and a legend explaining both.
//
// Neither the record nor the view changes here. No bit is stored, none is put
// back on screen in the sense of the view holding it again, and m.shown is not
// touched. The transcript resolves the receipt while it draws and drops the
// result on the next press.
func (m *Model) unfold() {
	if m.scars() == 0 {
		// Nothing to follow, so nothing happens. Flipping the flag anyway would
		// arm the next fold to arrive already open, and the fold would then
		// collapse the screen without the screen appearing to collapse.
		return
	}

	m.unfolded = !m.unfolded

	// Opening puts the retrieved material somewhere, and a key that appears to do
	// nothing has done nothing as far as the person is concerned — so the frame
	// goes to where it went.
	//
	// Where that is used to be answered with GotoTop, on the grounds that the scar
	// sits at the top of the view. It does not. D32 ended that invariant in memory
	// and this surface reached the state that proves it: upvote the oldest bit and
	// the fold splits around it, so the view begins with a hot row and the scar is
	// at index 1 — and with two holds there are two scars and neither is first.
	// GotoTop was then scrolling to whatever happened to be above the receipt, and
	// because it also went round sync, the caret was never scrolled back to. At a
	// hundred by thirty it landed off screen, on the key that is printed on the
	// scar itself.
	//
	// So: the scar being opened decides where the frame goes, and the caret says
	// which scar that is when it is sitting on one. Closing hands the frame back
	// to sync, which is the one place that knows where the caret is.
	//
	// It reveals rather than jumps — the same least-distance scroll every other
	// movement on this surface uses, and it does nothing at all when the scar is
	// already on screen. Jumping was measurably worse in both directions: it
	// scrolled when there was no need to, which moves everything the person is
	// reading in order to move nothing, and on a fixture where the caret sat one
	// row above the scar it pushed the caret off the top to show a block that was
	// already in view.
	//
	// It can still leave the caret off screen, and that is a real cost rather than
	// an oversight: a 294-bit receipt is taller than any terminal, so showing the
	// top of it and a caret in the hot tail is not a thing a frame can do. It is
	// tolerable for pgup and pgdn's reason — the next vote scrolls back to the
	// caret before the frame that records it is drawn, so no key ever acts on a row
	// the person cannot see.
	if !m.unfolded {
		m.sync()
		return
	}

	// Drawn first, because where the scar landed is a fact about the frame with
	// the receipts in it and not about the one before.
	m.draw()

	anchor := m.anchors.scar
	if m.cold(m.mark) && m.anchors.mark >= 0 {
		anchor = m.anchors.mark
	}
	m.reveal(anchor)
}

// cold reports whether the bit at this address is a fold rather than something
// somebody said. An address the store does not hold is not one, which is the
// answer that keeps every caller's question ("is the caret on a scar") true for
// an empty view.
func (m Model) cold(id string) bool {
	b, ok := m.store.Get(id)
	if !ok {
		return false
	}
	_, is := b.Payload.(memory.Compaction)
	return is
}

// scars counts the folds currently in the view.
func (m Model) scars() int {
	n := 0
	for _, b := range m.shown.Bits(m.store) {
		if _, cold := b.Payload.(memory.Compaction); cold {
			n++
		}
	}
	return n
}

// foldable is how many bits in a view a fold could actually take: the hot ones
// that no vote is holding.
//
// It replaces a hot() that returned m.shown[i:] from the first bit that was not
// a fold, on the assumption that scars are a prefix. D32 killed that invariant
// in memory — a hold splits a fold, so a view can hold many scars with hot bits
// between them — and wiring a vote in here is exactly what would have sprung it:
// len(hot()) would have counted interior scars as hot and fired the trigger
// early.
//
// Counting the *foldable* bits rather than the hot ones is the other half, and
// [memory.Stay.Holds] says why in its own words: a held bit stays hot, so a
// trigger counting hot bits stops falling back under its threshold once a few
// holds land, and then fires on every single write, absorbing a little less
// each time. What the trigger wants to know is how much material a fold could
// actually take off the screen, and that is this.
//
// # It asks memory which bits a hold spares, and for a while it worked that out
//
// The set was [memory.Stay.Holds] until a hold began covering the bit its own
// bit answers as well as itself. A covered bit is not held — nobody voted on it
// — and a fold passes over it exactly as it passes over the bit that is, so
// counting it here counted material no fold could take. That is not a rounding
// error at the trigger: the overcount is one bit per hold and it never goes
// away, so the count sits over the budget permanently and every single write
// fires a fold that takes one short run. Measured over 400 bits at 100x30 with
// one upvote in three, on the surface's own cadence and hold: **122 folds
// against 30**, which is a scar for every third thing said. The exact failure
// the paragraph above says this function exists to prevent, arriving through the
// one map it was reading.
//
// So it asks [memory.View.Sparing], which is that rule stated once, in the
// package that folds. Working it out here from Prev would be a second statement
// of it, and the two would agree on the day they were written.
//
// Scars are skipped before the spared set is consulted, and the reason
// originally written here was wrong. It said a bit written straight after a fold
// names the scar above it in Prev, so upvoting that bit would spare the scar.
// The frames say no, and [voteCell] carries the proof this same change produced:
// covering a scar needs a bit whose Prev *is* a scar, [Model.utter] takes Prev
// from the view's last entry, and a fold always leaves a kept tail after the
// scar — so no bit written by this program can name one. Nor by the other one:
// `say` takes Prev from the same head and never folds.
//
// So the order is harmless either way rather than load-bearing, and the skip
// stays first for the reason that was always true — a scar was never counted
// here anyway, see [Model.budget] for what the trigger's arithmetic rests on.
// The retired sentence is left named rather than deleted, because the next
// reader deciding whether the skip can be reordered will reach for exactly it.
//
// A free function rather than a method because the harness measures the hold
// schedule with it and must count the way the program counts. Restating the
// rule in a fixture is how a measurement comes to describe a program nobody is
// running.
func foldable(s *memory.Store, v memory.View, stay memory.Stay) int {
	spared := v.Sparing(s, stay)

	n := 0
	for _, b := range v.Bits(s) {
		if _, cold := b.Payload.(memory.Compaction); cold {
			continue
		}
		if spared[b.ID] {
			continue
		}
		n++
	}
	return n
}

// foldable is [foldable] over this model's own view and votes.
func (m Model) foldable() int { return foldable(m.store, m.shown, m.stay()) }

// pressured is the fold trigger: more material a fold could take than the record
// holds before it cools.
//
// One method because it was written out three times — at the write that fires
// it, at the vote that used to, and inside [Model.blocked], where it is a
// *prediction* of the other two. That third one is the reason this exists rather
// than being tidiness: [Model.blocked] puts a word on the screen saying a fold is
// waiting, and one `>` drifting to `>=` would have the footer say held on a frame
// that is folding. The whole argument for asking [memory.View.Absorbing] rather
// than working the cut out here is that a screen must not restate the fold's
// conditions; this is that argument one level up, where the condition is when
// rather than what.
func (m Model) pressured() bool { return m.foldable() > m.budget() }

// budget is how many foldable bits this view holds before it cools, and it is
// the height of the screen drawing it.
//
// D18(e) asked for a view that folds on a budget that is "a screen in rows";
// until this existed the answer was twelve, at every terminal size, and the debt
// entry that named it said so. Measured before it was changed: at 100x30 the
// viewport is 23 rows and the view drew 12 of them; at 200x80 it drew 12 of 73.
// A taller terminal bought emptiness rather than conversation.
//
// It is the viewport's own height rather than an arithmetic on [chrome], because
// [Model.layout] already owns that subtraction and two statements of it agree on
// the day they are written. That is also why there is no figure here: the numbers
// are printed by [TestHarnessFits] and pinned by
// [TestTheBudgetIsTheScreenAndTheCaretCannotMoveIt].
//
// # What it is not, and this one is a gap rather than a choice
//
// It is a budget on *foldable* bits, and the view is drawn in *rows* — those
// differ by the scars and the held bits in it, because [Model.foldable] excludes
// both and both still take a row. So the view runs past the frame, and how far
// depends on which side of [coolFloor] the terminal is on:
//
//   - **Where the budget is the screen** — every height whose viewport is at
//     least the floor, which is 19 rows and up — it is over by exactly one, the
//     scar at the head. Measured with nobody voting: 24 at a viewport of 23, 44
//     at 43, 74 at 73.
//   - **Where the floor is the budget**, the two are unrelated and the gap is
//     whatever the terminal is short by: 6 rows at 60x14 (viewport 7, worst view
//     13), 11 at a height of 9. That is the behaviour this surface always had at
//     small terminals and the floor is what preserves it, but it is not "one
//     row", and this comment said one row at every size until a review measured
//     the other end.
//
// Either way it can be much further once holds accumulate, which is the state
// [Model.blocked] names. The arrows [Model.edge] draws say how much is past the
// edge in both directions, which is the whole of what is done about it.
//
// Counting rows instead was measured and refused: with `len(view) > budget` as
// the trigger, held bits keep the view permanently over its budget and every
// single write fires a fold that takes one run — 302 folds over 400 bits at one
// upvote in five, against 47. That is a scar per bit, and a receipt that stands
// for almost nothing is worse than a row past the edge. [Model.foldable]'s own
// doc is where the reason lives; this is the same argument arriving from the
// other side.
//
// # Three things it does not count, each of which would fold for a reason that is
// not about the record
//
// **The caret's own row.** It is the one row on this surface with a variable
// height — [transcript] draws it whole — so a budget that counted rows as drawn
// would make the pressure on the record a function of where a cursor is parked:
// move the caret onto a long answer and the view crosses its budget and folds,
// move it off and the view is spared. That is memory forgetting because of a
// navigation gesture, and no gauge could give it an honest antecedent, because
// the gauge would be reporting the cursor. Measured, it is not a small
// difference: at 60x14 a caret on a 490-character reply draws eleven rows where
// every other bit draws one. So a bit costs one row here whatever it draws, and
// [Model.foldable] is what does the counting.
//
// **The note under the transcript** — a request in flight, a failure, a save that
// did not reach disk. Counting those rows would fold the record because the disk
// is broken, and the block that says so is the last thing on this screen that
// should cost anything. It pushes the transcript up instead, which the edge
// arrows already report.
//
// **A resize.** Nothing folds when the terminal changes size: [Model.pressured]
// is asked after a write and on ctrl+k, and never in the size branch of
// [Model.update]. Making a window shorter raises the gauge and fades the rows the
// next fold would take — the antecedent arrives immediately and the fold waits
// for somebody to say something. Dragging a window is not a memory operation, and
// [TestResizingTheTerminalNeverFoldsTheRecord] is what keeps it from becoming
// one.
func (m Model) budget() int { return max(m.viewport.Height(), coolFloor) }

// keep is how many bits stay hot through a fold: half the budget, moved back to
// the last thing the human said.
//
// Half because that is the ratio this surface folded at before the budget existed
// — twelve and six — and nothing measured argues for another one. Folding
// everything would leave nothing legible on screen, and a record with no hot tail
// is a filing cabinet rather than a conversation; half means the screen never
// empties below its middle and never fills past its edge.
//
// # Why it moves, which is the part that is not arithmetic
//
// [memory.View.Fold] cuts at a count and has no notion of a round, so the
// boundary lands between a question and the answer to it about as often as not.
// Measured on this surface with one voice, nobody voting, 100x30: **24 of 60
// frames held a round with one half of it behind a scar**, and in every one of
// those the head of the view was a reply to a question nobody could see. Parity
// is the whole of it — against a two-bit round an odd keep orphans the head in
// 94% of frames and an even one in 42% — which is a screen whose worst behaviour
// is decided by whether a number happens to be even.
//
// So the cut moves to where the human last spoke, which is the one boundary in a
// transcript that is not arbitrary: a person's own turn is where they would cut
// it themselves. Measured over 400 bits at four vote rates and five budgets, this
// takes the orphaned head to zero in every case, and costs about a tenth more
// folds because a longer tail refills to the trigger sooner.
//
// It moves back and never forward. Forward would tidy the same boundary by
// absorbing the orphan instead, fold slightly less often, and take a bit the
// previous rule kept — possibly the one under the caret. On a surface whose whole
// promise is that consolidation derives and never deletes, the direction that
// keeps more is the only one available.
//
// # Two bounds on how far it moves, and they are not the same bound
//
// **The budget, because past it the fold storms.** A search with no ceiling runs
// to the end of a stretch with nobody human in it and keeps almost everything, so
// the fold takes two bits and the next write folds again — a scar per bit, on a
// record whose receipts are supposed to stand for something. After a fold keeping
// k the view is a scar plus k bits and [Model.foldable] skips the scar, so the
// trigger is `k > budget`: **at the budget exactly it is false**, and this comment
// said "at or past it" for a round, which was wrong at "at". The bound is the
// budget itself and it is not tighter.
//
// **And never so far that fewer than two bits are left to fold**, which is a rule
// about D32 rather than about the budget and lives in [keepFrom]. That one is not
// belt-and-braces: with only the budget bound, `ctrl+k` on a view under the
// budget can move the cut to a single bit and refuse in silence.
//
// The two were briefly one — a ceiling of budget-1, which covers the pressured
// case by accident — and the mutation table said so by catching neither of them.
// Two guards where each hides the other's mutation is one guard and one
// decoration, and which is which is not visible from either.
//
// # What that ceiling cost while it was one too low
//
// It stopped at the budget minus one for a review, and there the cut is a single
// bit: D32's size rule refuses a run of one, [memory.View.Absorbing] comes back
// empty, [Model.blocked] goes true, and the footer prints the word `held` **on a
// record nobody has voted in**, over fourteen rows drawn cooling — one frame
// saying a fold is coming and that a fold cannot happen. Measured over 3,000
// fresh sessions at 100x30 with nobody voting and one bit in N from the human:
// 0.3% at one in three, 1.7% at one in five, **3.5% at one in ten**, 2.9% at one
// in twenty, which is the ratio a stretch of `tldr say` from agents produces.
// Session-open only, because after one fold there is always a scar and the cut is
// never one again. [TestNothingOnScreenSaysHeldUnlessSomethingIsHeld] and
// [TestTheFoldsWindowIsNeverASingleBit].
//
// Nothing here reads the caret, the terminal's width, or a clock. It is a
// function of the view's own speakers, so two processes over one record fold it
// the same way at the same terminal height.
func (m Model) keep() int {
	return keepFrom(m.shown.Bits(m.store), m.budget()/2, m.budget())
}

// keepFrom is [Model.keep]'s rule against bits already resolved: the smallest
// keep from base up to ceiling whose kept tail begins on a bit the human said,
// and base when there is no such bit in reach.
//
// Two parameters rather than one, for the off-by-one [Model.absorbing] lives on.
// The fade has to name what the *next* write will fold, on a view one bit shorter
// than the one that fold will see. A view only ever grows at the end, so the tail
// beginning at v'[len(v')-k] is the same bit as v[len(v)-(k-1)] — the same search
// run one lower against the drawn view returns exactly the fold's own keep minus
// one, for every k, which is what [Model.absorbing]'s lookahead needs and what a
// restatement of the rule over there would not give. Two statements of a rule
// agree on the day they are written; this is one statement asked twice.
//
// The second bound is here rather than at either call site, and it has to be, for
// the identity above: **the search may never return a keep that leaves fewer than
// two bits to fold**, because D32's size rule refuses a run of one and a refused
// fold is drawn as `held`. That is `len(bits)-2`, a function of the view's own
// length — so applying it inside means the caller one bit shorter gets a ceiling
// exactly one lower, which is what the identity needs. Clamped at either call
// site instead, the two would be clamped against the same number on views that
// differ by one, and the fade would name a cut the fold does not make.
//
// The clamp subsumes the loop's old `k <= len(bits)` guard, which is why there is
// no longer one: a ceiling of at most len-2 cannot index out of range, and on a
// view of nothing it is negative, so the loop does not run at all.
func keepFrom(bits []memory.Bit, base, ceiling int) int {
	for k := base; k <= min(ceiling, len(bits)-2); k++ {
		if bits[len(bits)-k].From.Ref == localHandle.Ref {
			return k
		}
	}
	return base
}

// stay is the right to be held out of a fold, as this surface grants it: the
// votes it has recorded, the human at this keyboard as the one voter whose
// upvote holds, and holdFor as how long one holds for.
//
// The handle is named here rather than in memory because memory refuses to
// guess: nothing in a record marks one handle as the human, and the program
// that ran the conversation is the only thing that knows. This is that program.
func (m Model) stay() memory.Stay {
	return memory.Stay{Votes: m.votes, By: localHandle, For: holdFor}
}

// absorbing is which bits on screen the next fold will take. They are the ones
// drawn cooling.
//
// The claim that rests on it, stated exactly, because it used to be stated
// larger than it is: nothing is absorbed that was not drawn cooling first,
// unless a hold ran out in the same write that folded it. The exception is not
// hedging and it is not small — see the last paragraph here, which is where it
// comes from — and it is written into the promise rather than beside it, because
// this surface's own doctrine (D35) is that a false claim of completeness is a
// worse failure than an honest absence.
//
// One thing this map names that the screen does not honour: a scar. A cold bit
// counts toward the fold's size rule like any other, so a [memory.Compaction] in
// the view lands in here on every frame a reader who has not voted ever sees,
// measured — and [transcript]'s Compaction branch draws
// every scar in seamInk and at column 0 regardless, so it fades in neither
// colour nor space. That is a gap between this map and the frame rather than a
// gap in this map, it predates the space channel, and the package doc states it
// as the second hole. Nothing here is wrong; what is wrong is downstream.
//
// A set and not an index, and that is the change rather than a refactor. The
// old answer was a cut point, because the cut was a prefix; a hold splits a
// fold, so what goes is now scattered through the view with bits that are
// staying standing between the pieces. An index cannot say that, and an index
// that tried would fade the held bit sitting in the middle of the window — the
// one row on screen whose whole claim is that it is not going anywhere.
//
// It asks [memory.View.Absorbing] rather than working it out. That function and
// [memory.View.Fold] are one traversal of one rule, so the cut this screen draws
// is the cut the fold makes; a second statement of the rule here would agree on
// the day it was written and drift after that.
//
// One less than the fold's own keep is the whole of the lookahead, and it is
// arithmetic rather than a fudge. The trigger fires *after* a bit is written, on
// a view one longer than the one last drawn, and cuts everything but the last
// keep. A view only ever grows at the end, so the window that fold will take —
// new[:len+1-keep] — is exactly this view's own first len-(keep-1) entries.
// Asking with keep instead would name a window one short, and one bit per fold
// would go from full brightness to absorbed with no frame in between.
//
// Since [Model.keep] stopped being a constant, "one less" has to mean one less
// *of the same search*, not one less than a number computed here — the rule looks
// at who spoke at the boundary, and the boundary is a bit further along on the
// view the fold will see. [keepFrom] takes its base and its ceiling for that
// reason, and both are lowered by one here. Lowering only the base would leave
// the search able to run one bit past where the fold's own search stops, and the
// two would disagree exactly at the top of the range — a bit absorbed without
// fading, which is the one direction this map may not err in.
//
// Over-predicting is the safe direction and this does it in two ways: ctrl+k
// folds immediately and so takes a subset, and a bit that is faded now can be
// kept by voting on it before the fold arrives, which is the entire point of
// drawing it.
//
// The one direction it can under-predict is an expiry, and the reach of that is
// wider than it first looks. A hold decays against [memory.View.Latest], so the
// write that carries the conversation past a vote's lifetime is the same write
// that fires the trigger: the bit was bright because it was held, the hold ran
// out as the newest bit landed, and the fold took it in that same step, with no
// frame between the two. That much is the held bit's own bargain and reads as
// fair.
//
// What is not obvious is that unheld material goes the same way. D32's size rule
// spares a lone unheld bit between two holds — a run of one is never cooled — so
// that bit is bright, correctly, and this map correctly leaves it out. Let both
// holds lapse in one write and the run merges to three, and all three go. The
// bit in the middle was never voted on by anybody and was never drawn cooling.
// Two holds pressed in the same breath expire in the same breath, because they
// decay against one shared [memory.View.Latest], so this is the ordinary case
// rather than a contrived one.
//
// It cannot be closed from here. Predicting it means deciding a hold against the
// instant of the *next* write, and when that write arrives is unknowable — it is
// whenever somebody types. What can be done is to not claim otherwise, and to
// leave the antecedent that does exist standing: the gauge draining beside the
// caret's mark, which says the hold is nearly up. That is thinner than the fade
// and it is honest about being thinner. See [TestAnExpiringHoldIsTheOneHoleInTheFade],
// which reproduces exactly this and is the reason the promise above is worded the
// way it is.
func (m Model) absorbing() map[string]bool {
	ahead := keepFrom(m.shown.Bits(m.store), m.budget()/2-1, m.budget()-1)
	return m.shown.Absorbing(m.store, ahead, m.stay())
}

// blocked reports a fold that cannot happen: the view is past its limit and
// there is nothing a fold could take.
//
// It is the one state the gauge cannot say by filling up, because the gauge is
// already full and the number beside it has kept climbing. What causes it is
// D32's size rule finding no run of two in the window, so [memory.View.Fold]
// refuses — and the honest thing is to name it, since the alternative is a full
// gauge and a screen that visibly is not folding.
//
// **The usual cause is holds and it is not the only one**, which this comment
// asserted for two sessions before a review found the other on a frame. Holds are
// how it is normally reached: every unheld bit in the window with a held bit on
// both sides. But the window itself can be one bit, which needs no vote at all —
// see [Model.keep], where a ceiling one too high did exactly that and put the word
// `held` under a record nobody had voted in. That is fixed at its source rather
// than excused here, and the general statement is the one above: the size rule,
// however the window came to be too small for it.
//
// Derived rather than remembered. The cheap version is a flag set where Fold
// returns false, and that flag is wrong the moment a hold expires without
// anybody pressing a key.
func (m Model) blocked() bool {
	return m.pressured() && len(m.shown.Absorbing(m.store, m.keep(), m.stay())) == 0
}

// day is the reference the times on this screen are read against: the newest
// instant the view holds, which is the record's own present.
//
// A screen with a clock on every row and no date anywhere cannot say which day
// a conversation happened on, which was the largest hole an auditor could poke
// in this surface and among the smallest to fix. The fix is not a date on every
// row — that is fifteen columns of the same eight characters — it is one date,
// stated once in the header, and rows that carry their own only when they
// differ from it. See [clock].
//
// # In the reader's own zone, and that is the whole of the fix for a real bug
//
// [clock] reads every row in this reference's location, so whatever zone this
// carries is the zone the entire screen is drawn in — and it used to be whatever
// zone the newest bit happened to arrive with. A live bit comes from `time.Now`
// and carries `time.Local`; a bit read back off the file comes from
// [memory.Bit].At, which the wire format normalizes to UTC on purpose. So **the
// same record showed one clock in the session that wrote it and another in the
// session that reopened it** — seen in a real terminal, 19:47 becoming 01:47 the
// next day on a machine six hours behind, which moves the header's *date* and not
// only the times.
//
// The record is not what is wrong. D12 keeps one normalized instant precisely so
// that the same moment recorded in two zones is one bit, and [memory.Bit].At's
// own doc says in as many words that anything drawing this to a person "has to
// decide for itself whose local time to show". This is that decision, made once:
// the reader's. A drawn row is a view, and D1 says a view is derived for whoever
// is reading it; there is one person at this keyboard. It deliberately does not
// answer "what time was it where the speaker was standing", because the record
// threw that away and nothing here can invent it — which is why the header says
// `local` rather than naming a zone, since an abbreviation would read as a fact
// about when it happened.
//
// `tldr top` is a second surface and deliberately does not follow: `stamp` there
// gives a reason this ruling does not reach — that reading is consumed by another
// session as often as by a person. Named here so the difference is a decision
// somebody can find rather than a discrepancy somebody trips over.
func (m Model) day() time.Time { return m.shown.Latest(m.store).Local() }

// draw rebuilds the transcript without moving the viewport.
//
// The note — a request in flight, or the failure of the last one — is appended
// here rather than passed into transcript, because it is not part of the view
// and transcript renders the view. Keeping it outside is what stops a pending
// row from ever being built by the same code that builds bits.
func (m *Model) draw() {
	// The composer's prompt is part of the frame and changes with the same
	// state, so it is set where the frame is built rather than at each of the
	// four places a wait can start or end. SetWidth is re-applied because the
	// textarea measures its prompt there.
	m.composer.Prompt = openPrompt
	if m.waiting.live {
		m.composer.Prompt = heldPrompt
	}
	m.composer.SetWidth(m.width)

	draw := transcript
	if m.ranked {
		draw = ranked
	}
	body, at := draw(m.frame())
	m.anchors = at

	if note := m.note(); note != "" {
		if body != "" {
			body += "\n"
		}
		body += note
	}
	m.viewport.SetContent(body)
}

// frame is everything the transcript is drawn from, resolved once here so that
// no part of the rendering asks the record a second question and gets a second
// answer. The holds and the absorbing set in particular are two readings of one
// vote view, and a row drawn from one while the gauge is drawn from the other is
// a screen disagreeing with itself.
// It is one struct for both surfaces rather than two, because everything in it
// but the bits is a question about the same two views and the answers must not
// differ between the screens. order is empty unless the ranked surface is up:
// ranking is a walk of the whole record and there is no reason to take it for a
// frame that will not draw it.
func (m Model) frame() frame {
	var order []memory.Ranked
	if m.ranked {
		order = m.ranking()
	}
	// The covered set is the transcript's alone, and it is empty on the ranked
	// surface for the same kind of reason order is empty on the transcript — but
	// the reason here is that the mark would be false rather than merely unused.
	// What a covered row draws is the tail of the mark on the row *below* it (see
	// [voteCell]), and that reads because the bit a hold covers is always the row
	// directly above it: they were adjacent when the cover was written, folds only
	// ever replace runs between bits, and both survive the fold together. Measured
	// over 26,467 covered rows at three terminal sizes and three vote rates, the
	// holder was the next row down every single time. Re-order the list and none
	// of that is true, so the tie would point at a stranger.
	holds := m.live()
	var covered map[string]bool
	if !m.ranked {
		covered = m.covered(holds)
	}
	return frame{
		store:     m.store,
		bits:      m.shown.Bits(m.store),
		clock:     clock{ref: m.day()},
		absorbing: m.absorbing(),
		holds:     holds,
		covered:   covered,
		votes:     memory.Tally(m.store, m.votes),
		order:     order,
		mark:      m.caret(),
		width:     m.width,
		open:      m.unfolded,
	}
}

// sync redraws the transcript and moves the viewport the least distance that
// keeps the caret's own block on screen, and then the least distance that keeps
// whatever the harness is saying about itself on screen. Every mutation ends
// here, so the viewport can never disagree with the record.
//
// Two steps and the second one wins, which is the whole arrangement. It used to
// go to the bottom unconditionally, which was right while the newest row was the
// only thing anyone could act on; then it followed the caret, because the caret
// can be twenty rows up and the key that votes acts on wherever it is. Both of
// those left the rows *below* the transcript — a pending line, a failure, a save
// that did not reach the disk — visible by luck rather than by rule, and the luck
// ran out when the caret's row grew taller than the frame. Neither of those rows
// is a bit and the caret cannot reach either, so nothing else on the screen brings
// them back.
func (m *Model) sync() {
	m.draw()

	// Where the caret is, and as much of its own block as the frame will hold.
	//
	// [Model.revealBlock] rather than [Model.reveal], because the caret's row is
	// drawn whole and a block is not a row: revealing its first line alone leaves
	// an answer open two lines deep on a short terminal, which is the promise this
	// surface makes stated and then not kept.
	//
	// Going to the bottom is what a conversation does, and it is what this did
	// unconditionally for as long as the newest row was the only thing anybody
	// could act on. It is no longer a case of its own. Riding the newest bit puts
	// the caret's block last in the transcript, so revealing that block ends at the
	// bottom anyway — and where it does not, the bottom is the wrong place, because
	// a block taller than the frame shown from its end answers "what did it say"
	// with the end of the answer.
	if m.anchors.mark < 0 {
		m.viewport.GotoBottom()
	} else {
		m.revealBlock(m.anchors.mark, m.anchors.rows)
	}

	// And then whatever the harness has to say about itself, which outranks all of
	// it. A note is not a bit — a request in flight, a failure, a save that did not
	// reach the disk — so the caret cannot reach it and nothing else on the screen
	// says it. It is drawn below the transcript, which means the one thing that
	// hides it is the frame being scrolled somewhere else, and the two things that
	// scroll the frame somewhere else are a parked caret and an answer taller than
	// the terminal. Both used to.
	//
	// This is where a save that failed became invisible: "recorded here, not on
	// disk" under a twenty-one-row answer, off the bottom of an eighty-by-twenty-four
	// frame, on the release that made the answer readable. A surface that hides that
	// notice is committing the failure it exists to report, so the notice comes last
	// and unconditionally.
	//
	// It is the same call for all three notices rather than a special case for the
	// one that broke: a pending line hidden under a long question is a person
	// waiting on a model with nothing on screen saying so, which is the same defect
	// with a shorter fuse.
	if note := m.note(); note != "" {
		rows := strings.Count(note, "\n") + 1
		m.revealBlock(m.viewport.TotalLineCount()-rows, rows)
	}
}

// revealBlock puts a run of rows on screen: whole when it fits, and from its
// first row down when it does not.
//
// Two reveals rather than one, in that order, and the order is the whole of it.
// Revealing the last row scrolls far enough that everything above it is as near
// the frame as it can be; revealing the first then wins any argument, so a block
// too tall to fit shows its beginning rather than its end. Both are least-distance
// scrolls, so a block already on screen does not move.
//
// Its first row is the part carrying the claim, on every block this is used for.
// On the caret's own bit that is who spoke, the reader's mark and the start of the
// sentence; on a notice it is what went wrong, with the fix beneath it. What falls
// below the margin is announced by the lower edge, the same way everything else
// past the frame already is, and pgdn reaches it.
func (m *Model) revealBlock(first, rows int) {
	if first < 0 || rows < 1 {
		return
	}
	m.reveal(first + rows - 1)
	m.reveal(first)
}

// reveal scrolls the least distance that puts row inside the frame, and does
// nothing at all when it is already there.
//
// The least distance, rather than centring: a screen that recentres on every
// press moves everything the person is reading in order to move one caret, and
// the caret is the only thing that moved.
func (m *Model) reveal(row int) {
	if row < 0 {
		return
	}

	top, h := m.viewport.YOffset(), m.viewport.Height()
	switch {
	case row < top:
		m.viewport.SetYOffset(row)
	case row >= top+h:
		m.viewport.SetYOffset(row - h + 1)
	}
}

func (m *Model) layout() {
	m.composer.SetWidth(m.width)
	m.viewport.SetWidth(m.width)
	m.viewport.SetHeight(max(m.height-chrome, 1))
}
