package main

import (
	"flag"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/tyler-j-chrestoff/tldreddit/memory"
	"github.com/tyler-j-chrestoff/tldreddit/tui"
)

// top prints the record back, ordered by what the human said mattered.
//
// It is the read half of D51(e) and the only reading of this record available
// without a terminal. What it ranks is the whole record and not the transcript,
// which is the same difference [tui.Model.judged] draws between retrieval and
// theatre: the one vote in the live record at the time this was written is on a
// bit three folds back, unreachable from the transcript, and a reading that could
// not reach it would be ranking only what is already on screen.
//
// # What this degrades to, printed on the face of it
//
// With one voter and few votes, the first tier holds almost nothing and almost
// every row is a tie — and [memory.View.Rank] leaves ties in the order it was
// given them, which here is newest first. So at zero votes this is `tail` and at
// one vote it is `tail` with one row lifted out of it. That is not hidden and
// cannot be: the header counts the three bands, so a reader sees "kept 1 · not
// judged 19" and knows exactly how much of the order was decided by a person and
// how much by the clock. A ranking that quietly degrades to recency while still
// calling itself ranked is the failure this line exists to make impossible, and
// the remedy for it is not code — it is votes (D51(d)).
//
// The surface's ranked screen takes the same population by the same rule:
// everything said, with a ballot and a scar each excluded for its own reason,
// which [reading] gives in full. It keeps one row this does not — a scar somebody
// voted on — because that screen has a caret a person can park on a fold and
// nowhere else to put the ballot, where this reading has no caret and confesses
// the gap in its header instead. [tui.Model.judged] listed *only* the voted bits
// once, on this paragraph's own argument: a second screen beside a transcript,
// showing the same rows reshuffled, is theatre. What killed that filter is that
// it hid the correction to its own top-ranked row, and the measurement is in
// [tui.Model.judged]'s own doc.
//
// So both readings degrade to recency, and what differs is what that costs each
// of them. There the list has to earn a keystroke away from a transcript, and it
// says how much of its own order a person decided in a heading over every band.
// Here there is no transcript — this is the only reading — so recency is a floor
// rather than a pretence, and a reading that went empty until somebody voted
// would send the next session back to the markdown file it is replacing.
//
// **Nothing binds this paragraph to the function it describes, and it was false
// for a checkpoint because of that.** [reading] and [tui.Model.judged] are one
// walk written twice in two packages, and no test compares them: `judged` is
// unexported and this is package main, so neither side can import the other to
// ask. So the standing re-check is by eye — read [tui.Model.judged] before
// trusting the two sentences above, and treat the day the walk moves into one
// place both packages import as the day this stops needing that.
//
// Ranked for [tui.Human] rather than for whoever asks. The tier that decides is
// the human's own standing vote, so naming the voter is naming who this reading
// is *for*, and there is one person at this record. A flag here would be a way to
// read the record as somebody else, which is a different feature with different
// questions attached to it and no one asking for it yet.
func top(s streams, path string, args []string) error {
	fs := flag.NewFlagSet("top", flag.ContinueOnError)
	fs.SetOutput(s.err)
	show := fs.Int("n", 10, "how many `rows` to print, newest and best first; 0 for all of them")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *show < 0 {
		return fmt.Errorf("-n %d: a reading cannot be a negative number of rows", *show)
	}

	rec, err := load(path)
	if err != nil {
		return err
	}

	said := reading(rec.store)
	ranked := said.Rank(rec.store, rec.votes, tui.Human())
	return draw(s.out, rec, ranked, *show)
}

// reading is the view [top] ranks: everything anybody said, newest first.
//
// Everything said, which is narrower than everything recorded, and the two
// exclusions are decisions rather than filtering for tidiness.
//
// A vote is not a row. It is one participant's judgment *about* a row, and it
// already reaches this reading as that row's own mark — printed as a row of its
// own it would be an entry whose entire content is an edge, saying "somebody
// approved of something" beside the thing they approved of.
//
// A scar is not a row either, and leaving it out shows *more* rather than less: a
// [memory.Compaction] is what a view did to fit on a screen, and every bit it
// stands for is in this reading on its own account. This reading has no screen to
// fit, so the summary has nothing to do here that the material it summarises does
// not do better. The cost is real and is named rather than papered over: a vote
// cast on a scar — which the surface's ranked screen permits — has no row here to
// land on, so it is not visible in this reading at all.
//
// Newest first because that is the order the ties fall in, and at one vote nearly
// everything is a tie ([top] says what that means). [memory.View.Rank] sorts
// stably and reads no clock, so the tiebreak belongs to whoever builds the view —
// here, and stated: time descending, and within one instant the order
// [memory.Store.All] hands them out in, which is address order. That last one
// settles nothing anybody would defend to a reader; it is there so that two runs
// over one record print the same thing, which is the only claim it makes.
func reading(s *memory.Store) memory.View {
	type row struct {
		id string
		at time.Time
	}

	var rows []row
	for b := range s.All() {
		if _, said := b.Payload.(memory.Utterance); !said {
			continue
		}
		rows = append(rows, row{id: b.ID, at: b.At})
	}
	slices.SortStableFunc(rows, func(a, b row) int { return b.at.Compare(a.at) })

	out := make(memory.View, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.id)
	}
	return out
}

// draw writes the reading: what the record holds, how much of the order a person
// actually decided, and then the rows.
//
// The two header lines are the instrument. Without them a reader has a list and
// no way to tell whether the ranking did anything, which on a record with one
// vote in it is the whole question — see [top].
//
// # Four vote counts, and each is over a different population
//
// They are separate numbers because they are separate facts, and printing one of
// them as if it were all four is how this reading would overstate how much a
// person decided. Named in the order they appear:
//
//   - **ballots** is every vote in the vote view, by anybody, including ones
//     since superseded. It is how many times somebody pressed a key.
//   - **standing** is what those ballots currently amount to: one per voter per
//     bit, [memory.Tally]'s own rule. Changing your mind is two ballots and one
//     standing vote, and the record keeps both — so ballots above standing is
//     revision, and it is information rather than an error.
//   - **kept and let go** are the bands below, and they count only this reading's
//     own participant, on rows this reading actually has. That is what makes them
//     reconcile with the rows a person can see.
//   - **on nothing this reading shows** is the remainder of that participant's
//     standing votes: a vote cast on a scar, which the surface's ranked screen
//     permits and [reading] deliberately has no row for. Printed only when it is
//     not zero, because a zero there is the ordinary case and a fourth number on
//     every line would bury the three that move.
//
// This header once printed ballots alone and called them votes, next to bands
// counting standing votes on rows — two ways of disagreeing, both flattering.
func draw(w io.Writer, rec record, ranked []memory.Ranked, show int) error {
	var b strings.Builder

	kept, level, gone := 0, 0, 0
	for _, r := range ranked {
		switch {
		case r.Own > 0:
			kept++
		case r.Own < 0:
			gone++
		default:
			level++
		}
	}

	me := tui.Human()
	standing, elsewhere := 0, 0
	shown := make(map[string]bool, len(ranked))
	for _, r := range ranked {
		shown[r.ID] = true
	}
	for target, score := range memory.Tally(rec.store, rec.votes) {
		standing += len(score)
		if _, mine := score[me]; mine && !shown[target] {
			elsewhere++
		}
	}

	fmt.Fprintf(&b, "%d said of %d bits on the record · %d %s, %d standing · ranked for %s\n",
		len(ranked), rec.store.Len(), len(rec.votes), plural(len(rec.votes), "ballot"),
		standing, me.Display)
	fmt.Fprintf(&b, "kept %d · not judged %d · let go %d", kept, level, gone)
	if elsewhere > 0 {
		fmt.Fprintf(&b, " · %d on nothing this reading shows", elsewhere)
	}
	if show > 0 && show < len(ranked) {
		fmt.Fprintf(&b, " · showing the first %d", show)
	}
	b.WriteString("\n")

	for i, r := range ranked {
		if show > 0 && i == show {
			break
		}

		// A row the store cannot resolve is drawn rather than skipped, for
		// [tui.Model.judged]'s reason: under an append-only store it cannot
		// happen, so it is D1's own failure and it should arrive as a row naming
		// the address it could not find.
		bit, ok := rec.store.Get(r.ID)
		if !ok {
			fmt.Fprintf(&b, "\n%s  %s  the record does not hold this bit\n", own(r.Own), memory.Short(r.ID))
			continue
		}

		fmt.Fprintf(&b, "\n%s  %s  %s  %s%s%s\n",
			own(r.Own), memory.Short(bit.ID), stamp(bit.At), speaker(bit.From), unfinished(bit), others(r.Others))
		for line := range strings.SplitSeq(text(bit), "\n") {
			fmt.Fprintf(&b, "    %s\n", line)
		}
	}

	if len(ranked) == 0 {
		b.WriteString("\nnothing has been said on this record yet\n")
	}

	_, err := io.WriteString(w, b.String())
	return err
}

// speaker is who said it, as `display (ref)` when those differ and as the one
// name when they do not — except that nothing draws bare under the display name
// the person at the keyboard uses, whatever its ref.
//
// The ref is here because this is the only reading of the record without a
// terminal, and `tldr say` will spell nearly any handle it is given: it refuses
// the human's exact ref and nothing resembling it, so `-as local2` is available
// and a display name is free for the asking. [cli] argues that an utterance
// attributed wrongly is a lie a reader can catch by reading it, and that argument
// is only true where the reader is shown enough to catch it: a display name alone
// draws `me` for the person at the keyboard and `me` for whoever borrowed it, in
// the same column, in the same font. The ref is the field the record actually
// keys a voter on ([memory.Handle], [memory.Tally]), so it is the one that
// decides things and therefore the one worth printing.
//
// # The exception, and the two simpler rules it sits between
//
// Two fields that agree carry one fact, and that is the ordinary row rather than
// an edge case: `tldr say -as session-15` with no -name records the ref as the
// display. Parenthesising always would print `session-15 (session-15)` on nearly
// every line — the same word twice, everywhere, to disambiguate a case that
// mostly does not arise.
//
// Where it does arise is the one display name that means something on this
// record. `say -as me` drew a bare `me`: *less* said about it than the human's
// own `me (local)`, so the row that had never been near a keyboard read as the
// more human of the two. The fix is an equality against [tui.Human]'s display and
// deliberately not a resemblance test — the refusal in [say] declines to guess at
// intent and this would be the same guess, on the same evidence, one file later.
// `me (me)` is odd to read and is what it should be: one participant keyed on
// `me`, beside another keyed on `local`.
//
// It is not a proof of identity and nothing here is. What it buys is that the two
// are distinguishable when they differ, which is the whole of the "catch it by
// reading it" claim and, before this, was not true through this command.
func speaker(h memory.Handle) string {
	if h.Display == "" || (h.Display == h.Ref && h.Display != tui.Human().Display) {
		return h.Ref
	}
	return fmt.Sprintf("%s (%s)", h.Display, h.Ref)
}

// own is the column that decides the order: what the person this reading is for
// currently says about this bit. Written as a signed number rather than as the
// surface's arrowheads because this output is read by pipes as well as people,
// and a sign is greppable where a glyph is a rendering question.
func own(n int) string {
	switch {
	case n > 0:
		return "+1"
	case n < 0:
		return "-1"
	}
	return " 0"
}

// others is everybody else's standing votes, summed, and shown only when there
// are any. It is separate from [own] and never added to it: the two are tiers,
// and a sum is how an agent outvotes a human ([memory.Ranked] says so at length).
func others(n int) string {
	if n == 0 {
		return ""
	}
	return fmt.Sprintf("  (%+d from others)", n)
}

// text is what the bit says, as it was said. Not wrapped and not collapsed: the
// line breaks are the author's, this output has no width to fit, and a terminal
// wraps what it has to. The transcript flows a message into one paragraph
// (`tui/render.go`), so a structured message read back here keeps a shape the
// screen never shows.
//
// Nothing this program has to say is ever added to it, which is why the
// unfinished mark is on the row above rather than after the last word. A reading
// whose own vocabulary can appear inside a quotation is a reading a participant
// can forge by typing the mark — `docs/DEBT.md` records that the surface has
// exactly that ambiguity at narrow widths, and there is no width here to force it.
func text(b memory.Bit) string {
	u, said := b.Payload.(memory.Utterance)
	if !said {
		// Unreachable through [reading], which admits nothing else. Written
		// anyway, because the alternative when somebody widens that filter is a
		// row printed as a bare Go type — which is what the ranked screen did the
		// first time a fold reached it (`tui/ranked_test.go`).
		return fmt.Sprintf("(%T, which this reading has no words for)", b.Payload)
	}
	return u.Text
}

// unfinished marks a bit whose speaker was cut off mid-sentence, in this
// program's own words rather than the surface's glyph. A fragment recorded
// unmarked is a permanent claim that somebody said something they never finished
// saying ([memory.Utterance].Truncated), and a reading that drops the mark hands
// the next session that claim as fact.
func unfinished(b memory.Bit) string {
	if u, said := b.Payload.(memory.Utterance); said && u.Truncated {
		return "  (cut off mid-sentence)"
	}
	return ""
}

// stamp is when it happened, in UTC and to the second.
//
// UTC because a zone is not recorded — [memory.Bit].At keeps the instant and
// nothing else, so a bit read back from a file carries no answer to "whose
// afternoon was this". Drawing it in the reader's own zone would invent that
// answer, and this output is read by a session on another machine as often as by
// a person. RFC 3339 for the same reason the address goes out whole: it sorts,
// greps and parses, where a friendlier format only reads.
func stamp(at time.Time) string { return at.UTC().Format(time.RFC3339) }

// plural is the difference between "1 vote" and "0 votes", which matters here
// because the vote count is the number this whole reading turns on.
func plural(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}
