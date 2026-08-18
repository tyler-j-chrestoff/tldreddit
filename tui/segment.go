package tui

import "strings"

// Cutting a message into the parts its own author marked, for the surfaces that
// have one row rather than a screen.
//
// # Why a segmenter, and why it is here and not in memory
//
// [markdown] draws a message that is written as a document as a document, and
// that closed the wall of text on the one row this surface draws whole. Every
// other row still had it: the transcript's cut rows, the ranked list's previews,
// a receipt's rows and a scar's quotation all read [oneLine], which collapses
// every whitespace run in the bit to a single space. So a model's answer arrives
// on those rows as
//
//	### Reversing a slice in place There are three things worth saying about th…
//
// — the heading's hashes, the list's dashes and the emphasis's asterisks all on
// screen as characters, and the structure that produced them gone. That is most
// of what a person reading a busy record actually sees.
//
// D72(a) ruled where this may live: a fenced region is segmented **in the view,
// for layout, budget, quoting and the word index — it does not become its own
// bit**. Nothing here reaches the store, nothing reaches a content address, and
// [oneLine], which answers "what does the record hold" and is what the
// reachability tests read, is untouched. The seam this cuts on is the one the
// speaker wrote: a fence has an opening and a close somebody typed, where a
// paragraph break is whitespace.
//
// # The two things it is asked for, and they are not the same question
//
//   - [lede] is a *display* of a message in one row. It may spend a mark that is
//     redundant with what it draws, exactly as [mdStyle] does on the block: a
//     heading's hashes go and the words are drawn in capitals, a list item's
//     dash becomes the separator this surface already uses between things on one
//     row.
//   - [opening] is a *quotation* of a message, for [frame.quotation], and it may
//     spend nothing at all. Quotation marks assert that what is between them is
//     what somebody said, so what comes back is the speaker's own characters or
//     nothing.
//
// One function could not serve both, and the difference is the whole reason
// there are two: the first is allowed to be a rendering and the second is not.
type segment struct {
	kind  int
	lines []string
}

// The four things a line can open, and the whole vocabulary this file knows.
// They are the same three block marks [structured] gates on, plus everything
// else, because a message with no marks in it is one paragraph and that is the
// common case rather than a fallback.
const (
	segProse = iota
	segHeading
	segItem
	segFence
)

// segments cuts text into the blocks its author marked.
//
// Fences are tracked by their own opening run, so a fence marker inside an
// indented block does not close a block it never opened, and an unclosed fence
// runs to the end of the text — which is what goldmark does with one too, and
// what [prewrapped] does beside it, so all three agree about where the code is.
// That agreement is the point: a segmenter that disagreed with the renderer
// about which lines are code would put a quotation mark round a fence marker,
// which is exactly the defect this file was written to close.
//
// A blank line ends a paragraph, a heading is its own segment, and each list
// item is its own segment. Nothing here is nested: a sub-list, a quoted block
// and an indented continuation all come back as prose, because the surfaces
// reading this have one row and there is nothing a depth could be drawn in.
func segments(text string) []segment {
	var out []segment
	var run []string
	fence := ""

	flush := func() {
		if len(run) > 0 {
			out = append(out, segment{kind: segProse, lines: run})
			run = nil
		}
	}

	for _, l := range strings.Split(text, "\n") {
		t := strings.TrimLeft(l, " \t")

		switch {
		case fence != "":
			out[len(out)-1].lines = append(out[len(out)-1].lines, l)
			if strings.HasPrefix(t, fence) {
				fence = ""
			}

		case strings.HasPrefix(t, "```"), strings.HasPrefix(t, "~~~"):
			flush()
			fence = t[:3]
			out = append(out, segment{kind: segFence, lines: []string{l}})

		case atx(t):
			flush()
			out = append(out, segment{kind: segHeading, lines: []string{t}})

		case bullet(t):
			flush()
			out = append(out, segment{kind: segItem, lines: []string{t}})

		case strings.TrimSpace(l) == "":
			flush()

		default:
			run = append(run, l)
		}
	}
	flush()
	return out
}

// words is a segment's own text with the mark that opened it taken off, and
// whitespace collapsed the way every row on this surface collapses it.
//
// A fence comes back as the first line inside it, and the fence markers
// themselves never do: they are the delimiters of a region, and a row that is
// not a region has nothing to delimit. What that line is worth is [lede]'s
// business, not this function's.
func (s segment) words() string {
	collapse := func(v string) string { return strings.Join(strings.Fields(v), " ") }

	switch s.kind {
	case segHeading:
		return collapse(strings.TrimLeft(s.lines[0], "# "))

	case segItem:
		t := strings.TrimSpace(s.lines[0])
		if i := strings.IndexByte(t, ' '); i >= 0 {
			return collapse(t[i+1:])
		}
		return ""

	case segFence:
		for _, l := range s.lines[1:] {
			if strings.TrimSpace(l) != "" && !strings.HasPrefix(strings.TrimLeft(l, " \t"), s.fenceMark()) {
				return collapse(l)
			}
		}
		return ""
	}
	return collapse(strings.Join(s.lines, " "))
}

// fenceMark is the run that opened this fence, which is also the only run that
// can close it.
func (s segment) fenceMark() string {
	t := strings.TrimLeft(s.lines[0], " \t")
	if len(t) >= 3 {
		return t[:3]
	}
	return "```"
}

// lede is a message drawn in one row: its blocks in order, joined by the
// separator this surface already uses between things that share a row.
//
// # What it spends, and the rule that decides
//
// The rule is [mdStyle]'s and it is applied here rather than restated: **is the
// mark redundant with what is drawn**. A heading's hashes are positional syntax
// — they say what the line is — and once the words are in capitals they say
// nothing the drawing does not, so they go. A list item's dash is positional
// too, and what replaces it is the middle dot that already separates the parts
// of a scar's row and the rungs of the footer: one dot vocabulary, not two, and
// on a one-row lede "next item" and "next thing" are the same reading.
//
// Emphasis goes with them, for the reason the block does it: loudness is not
// meaning, and four characters of markup buy a reader of one row nothing. Only
// the doubled marks are spent — `**bold**` and `__bold__` — and the single ones
// are left exactly as typed. That asymmetry is deliberate and it is measured
// rather than tidy: a single asterisk is what `2 * 3 * 4` is made of and a
// single underscore is what `snake_case_name` is made of, and a rule that took
// those would be re-attributing a speaker's characters to the program, which is
// [unmarked]'s defect arriving from a third direction. A doubled mark round a
// span that neither opens nor closes on a space has no such twin.
//
// Inline code keeps its backticks, for [mdStyle]'s reason and no other: they are
// delimiting syntax, they are the sole carrier of where the span starts and
// stops in a channel made only of characters, and no wider terminal gives them
// back once dropped.
//
// # A fence contributes its first line, in backticks
//
// It has to contribute something, and the reason is the case that decides this
// whole design: a message that is *only* a fenced program has no prose in it at
// all, and a lede that skipped fences would draw an empty row for it. So the
// region is quoted the way an inline span is — the speaker's own first line of
// code, inside the mark this surface already means "code" by, cut like anything
// else if the row runs out.
//
// It is the first line and not a summary, and that is the line this file will
// not cross. A count of lines or a language name would be the program composing
// a sentence about somebody's message; the first line is a character-for-
// character piece of it, and a reader who wants the rest moves the caret onto
// the row and gets all of it drawn ([saidWhole]).
//
// # It is a rendering and not a record
//
// Nothing else on this surface reads this. [oneLine] still answers "what does
// the record hold", [saidWhole] still draws every character of the block, and
// [opening] — the quotation's path — never sees anything here. A row this
// produces is display, like the ellipsis on the end of it, and a wider terminal
// or the caret gives back everything it spent.
// Everything else, and it is most messages, comes back exactly as [oneLine]
// draws it: whitespace collapsed and not one character spent. The gate is
// [structured]'s, the same one that decides whether the caret's block is drawn
// as a document, and sharing it is what keeps one message from being a document
// under the caret and its own source one row above.
//
// A two-paragraph message with no mark in it is one paragraph here, and that is
// a fact about having one row rather than a residual: a break cannot be drawn in
// a row that has no second line to draw it on, and collapsing it is the same cut
// as the ellipsis on the end. Under the caret it is a break again — [wrapped]
// keeps every line the record holds — so the two surfaces differ in what they
// have room for and not in what they think the message is, which is the property
// sharing [structured]'s gate is for.
func lede(text string) string {
	if !structured(text) {
		return strings.Join(strings.Fields(text), " ")
	}

	segs := segments(text)
	if len(segs) == 0 {
		return ""
	}

	var parts []string
	for _, s := range segs {
		w := s.words()
		if w == "" {
			continue
		}
		switch s.kind {
		case segHeading:
			w = strings.ToUpper(w)
		case segFence:
			w = "`" + w + "`"
		default:
			w = unemphasised(w)
		}
		parts = append(parts, w)
	}
	return strings.Join(parts, " · ")
}

// unemphasised drops the doubled emphasis marks round a span, and only those.
//
// The span may not open or close on a space, which is what keeps a line of
// dashes, a `**` used as multiplication and a doubled underscore inside an
// identifier from being read as emphasis. Unpaired marks are left exactly where
// they were: an opening mark with no close is not emphasis, it is a character
// somebody typed.
func unemphasised(s string) string {
	for _, mark := range []string{"**", "__"} {
		var out strings.Builder
		rest := s
		for {
			open := strings.Index(rest, mark)
			if open < 0 {
				break
			}
			after := rest[open+len(mark):]
			close := strings.Index(after, mark)
			if close < 0 {
				break
			}
			span := after[:close]
			if span == "" || strings.HasPrefix(span, " ") || strings.HasSuffix(span, " ") {
				out.WriteString(rest[:open+len(mark)])
				rest = after
				continue
			}
			out.WriteString(rest[:open])
			out.WriteString(span)
			rest = after[close+len(mark):]
		}
		s = out.String() + rest
	}
	return s
}

// opening is the part of a message a scar may put quotation marks round: its
// first block, verbatim, or nothing at all.
//
// # Why it refuses rather than trims
//
// [frame.quotation] draws `coordinator-7 "…"`, and those two characters are an
// assertion that what is between them is what somebody said. Before this
// existed, the quotation was built from [said], which read the whole message as
// one collapsed line — so a fold that absorbed a model's program came back as
//
//	coordinator-7 "```go package main import "fmt" // reverse turns s aro…"
//
// with the fence markers inside the quotation and the `"` from `import "fmt"`
// closing it four words early. Both halves are the same failure: the one
// assertion those two characters make, broken by content the surface chose to
// put between them.
//
// So a fenced region is never what comes back, and a message made of nothing
// else comes back empty: the caller falls back to the rung with no account of
// the content in it — a count, a span and a key that works, which
// [frame.quotation] already argues is a better narrow scar than a bad one.
// [frame.quoted] does the other half by passing over such a bit when it is
// choosing which one to quote, so a fold that absorbed one program and nine
// sentences still quotes a sentence.
//
// **It skips a fence rather than stopping at one, and that is a narrowing of the
// rule this was ordered as.** The brief said "the first paragraph, refusing on a
// leading fence", which was my own earlier wording, and on the frames it costs
// something for nothing: a model that shows a program and then explains it in a
// sentence underneath is an ordinary answer, and refusing it leaves the scar
// with no account of a bit that has a perfectly quotable sentence in it. The
// hazard being closed is *quoting from inside a fence*, and what closes it is
// that a fenced segment is never returned — a structural property of the
// segmenter rather than a rule about position. Where the message opens with a
// fence and has nothing else, the two behave identically, which is the case the
// defect was found on.
//
// # Verbatim, which is why this is not [lede]
//
// Nothing is spent here. A heading keeps its hashes, emphasis keeps its
// asterisks, and a bullet keeps its dash, because inside quotation marks every
// one of those is a character the speaker typed and dropping it would be this
// surface editing somebody's words while asserting it had not. The only thing
// done to the text is the whitespace collapse every row on this surface does,
// which is a fact about the row rather than about the words.
//
// The residual, named because it is real and not closed: a *prose* paragraph
// containing a quotation mark still puts one inside these. That is ordinary
// English rather than a program, it reads as nested speech rather than as a
// broken assertion, and no rule that removed it could leave the words verbatim.
// It shares [lede]'s gate, for [lede]'s reason: a message with no block mark in
// it is one paragraph on every surface here, and it is quoted whole exactly as
// it has been since the quotation replaced the word bag.
func opening(text string) string {
	if !structured(text) {
		return strings.Join(strings.Fields(text), " ")
	}

	for _, s := range segments(text) {
		if s.kind == segFence {
			continue
		}
		if w := strings.Join(strings.Fields(strings.Join(s.lines, " ")), " "); w != "" {
			return w
		}
	}
	return ""
}
