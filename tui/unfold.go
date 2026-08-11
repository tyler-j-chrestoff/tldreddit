package tui

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/tyler-j-chrestoff/tldreddit/memory"
)

// Column widths for a retrieved row. Address and time are dropped, in that
// order, when the terminal cannot hold them; the handle then shrinks; the text
// takes whatever is left. Text is last because it is the only column a person
// reads rather than checks, and the handle outlasts the machinery columns
// because provenance is the question this block exists to answer.
const (
	gutter    = "│ "
	addrWidth = 8
	timeWidth = 5
	nameFloor = 3
	textFloor = 16
	colGap    = 2
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
func recall(s *memory.Store, c memory.Compaction) []retrieved {
	out := make([]retrieved, 0, c.Count())
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
func unfold(s *memory.Store, c memory.Compaction, width int) string {
	bits := recall(s, c)

	// Handles are aligned to the widest one actually present and are not cut to
	// a fixed column. Two agents whose names share a prefix — coordinator-7 and
	// coordinator-9 — would arrive at the same ten characters, and a fold that
	// merges two speakers into one string has destroyed the one thing the block
	// exists to preserve. Provenance has to survive a fold, and on a monochrome
	// terminal alignment is all that is left to carry it.
	names := make([]string, 0, len(bits))
	for _, r := range bits {
		if r.found {
			names = append(names, r.bit.From.Display)
		}
	}

	digits := len(strconv.Itoa(max(len(bits), 1)))
	idxWidth := 2*digits + 1

	// The drop ladder, in order: the address goes, then the time, then the
	// handle shrinks, and the text takes whatever is left. Machinery before
	// provenance before content — an address is what a person checks, a handle
	// is what they are asking about, and the sentence is what they read.
	addr, when := true, true
	lead := func() int {
		n := lipgloss.Width(gutter) + idxWidth + colGap
		if addr {
			n += addrWidth + colGap
		}
		if when {
			n += timeWidth + colGap
		}
		return n
	}

	if width-lead()-widest(names)-colGap < textFloor {
		addr = false
	}
	if width-lead()-widest(names)-colGap < textFloor {
		when = false
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
			// The one row whose whole purpose is to be seen, so it is built to
			// the same width budget as every other row rather than to a fixed
			// one. A failure notice that runs off the edge of the terminal is a
			// failure notice nobody receives. Its ordinal stays, so an
			// unresolvable bit still occupies its place in the count.
			room := max(width-lipgloss.Width(gutter)-idxWidth-colGap, 1)
			short := memory.Short(r.id)
			// The address goes before the alarm does. A row reading only
			// "6c968f40" is indistinguishable from a bit that resolved and had
			// nothing to say, which is the precise confusion this row exists to
			// prevent — so the last word standing is the one that says it
			// failed, not the one that says which.
			row += warm.Render(ansi.Truncate(fit(room,
				short+" does not resolve — the receipt outlived the record",
				short+" does not resolve",
				short+" unresolved",
				short+" gone",
				"unresolved",
				"gone",
			), room, "…"))
			fmt.Fprintln(&out, row)
			continue
		}

		if addr {
			row += column(rule, memory.Short(r.id), addrWidth)
		}
		if when {
			row += column(rule, r.bit.At.Format("15:04"), timeWidth)
		}
		row += column(dim, r.bit.From.Display, name)
		row += dim.Render(ansi.Truncate(said(r.bit), text, "…"))
		fmt.Fprintln(&out, row)
	}

	// The closing bar counts the rows that were drawn, not the number the scar
	// claims. Cool guarantees they agree, so this is normally the same number
	// twice — which is the point. It is no longer the only way to check that,
	// because the ordinal on each row carries the same total; it is the
	// terminator, and it says where the material came from, which no ordinal
	// can.
	out.WriteString(rule.Render(fit(width,
		fmt.Sprintf("└─ %d bits from the record ──", len(bits)),
		fmt.Sprintf("└─ %d bits ──", len(bits)))))
	return out.String()
}

// said is a bit's content on one line. Whitespace runs collapse, because a
// pasted newline would otherwise cost the block its one-row-per-bit count.
func said(b memory.Bit) string {
	switch p := b.Payload.(type) {
	case memory.Utterance:
		return strings.Join(strings.Fields(p.Text), " ")
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
// needs two styles on one column: the transcript fades a speaker's name as its
// bit approaches a fold, and the fade has to wrap the name's own style rather
// than replace it.
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
