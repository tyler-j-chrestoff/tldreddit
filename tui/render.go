package tui

import (
	"cmp"
	"fmt"
	"iter"
	"slices"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/tyler-j-chrestoff/tldreddit/memory"
)

// transcript renders the view. Bits before fadeBefore are the ones the next
// fold will absorb, and are drawn dimmer to say so.
//
// When open is set, every scar is followed by the bits it stands for, resolved
// from the store as this runs. Nothing is cached: the block on screen is a
// fresh walk of the receipt every time the transcript is drawn, which is the
// strongest available form of the claim it makes.
func transcript(s *memory.Store, bits []memory.Bit, fadeBefore, width int, open bool) string {
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
	name := nameColumn(names, width)

	var out strings.Builder
	for i, b := range bits {
		style := hot
		if i < fadeBefore {
			style = cooling
		}

		switch p := b.Payload.(type) {
		case memory.Utterance:
			// Cut with an ellipsis rather than letting the viewport clip at the
			// edge. Text that stops at the margin and text that ran out of
			// margin look identical, and a surface whose whole argument is that
			// it shows you what it dropped cannot drop the end of a sentence
			// without saying so.
			who, pad := cell(b.From.Display, name)
			room := max(width-name-colGap, 1)
			fmt.Fprintf(&out, "%s\n", clip(
				style.Render(speaker.Render(who))+
					pad+
					style.Render(ansi.Truncate(p.Text, room, "…")), width))
		case memory.Compaction:
			fmt.Fprintln(&out, clip(seam(p, width, open), width))
			if open {
				fmt.Fprintln(&out, clip(unfold(s, p, width), width))
			}
		default:
			// Payload is a closed set but Go does not check switch
			// exhaustiveness, so an unhandled kind must be loud rather than
			// invisible.
			fmt.Fprintf(&out, "%s\n", warm.Render(fmt.Sprintf("<unrendered %T>", p)))
		}
	}
	return strings.TrimRight(out.String(), "\n")
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
// Open changes the left end from a rule to a corner and nothing else. The scar
// does not go away when the material behind it is on screen — a scar that
// vanished while the bits came back would read as the fold having been undone,
// and a fold is not undoable. It is followable, which is a different promise
// and the one this product actually makes.
func seam(c memory.Compaction, width int, open bool) string {
	corner := "──"
	if open {
		corner = "┌─"
	}

	span := fmt.Sprintf("%s–%s", c.From().Format("15:04"), c.To().Format("15:04"))
	words := topWords(c.Bag(), 4)

	// Widest first. The count and the key survive every cut: the count is the
	// claim the receipt makes, and the key is the only thing on screen saying
	// the claim can be checked.
	//
	// The span is dropped before the words, which is the opposite of what it
	// looks like. Every absorbed bit carries its own time in the block the key
	// opens, so the span is one press from being back; the words are the only
	// account of what the window was about and are shown nowhere else.
	var candidates []string
	if len(words) > 0 {
		candidates = append(candidates, fmt.Sprintf("%s %d bits · %s · %s ── ctrl+u ──",
			corner, c.Count(), span, strings.Join(words, " ")))
		for n := len(words); n >= 1; n-- {
			candidates = append(candidates, fmt.Sprintf("%s %d bits · %s ── ctrl+u ──",
				corner, c.Count(), strings.Join(words[:n], " ")))
		}
	} else {
		// A window of nothing but filler has no words to report, and then the
		// span is the only thing the scar can say about what it stands for.
		candidates = append(candidates, fmt.Sprintf("%s %d bits · %s ── ctrl+u ──",
			corner, c.Count(), span))
	}
	candidates = append(candidates,
		fmt.Sprintf("%s %d bits ── ctrl+u ──", corner, c.Count()),
		fmt.Sprintf("%s %d bits · ctrl+u", corner, c.Count()))

	return rule.Render(fit(width, candidates...))
}

// filler is the words a scar does not report. They are the most frequent words
// in any English text, so without this the summary of every fold is the same
// four words and the receipt says nothing about the window it stands for.
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
// then alphabetical, skipping [filler].
func topWords(bag iter.Seq2[string, int], n int) []string {
	type word struct {
		text  string
		count int
	}

	var words []word
	for text, count := range bag {
		if filler[text] {
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

// gauge draws the pressure on the record: how full the hot band is, and
// therefore how close the next fold is. Cooling should never be a surprise.
func gauge(hot, limit, width int) string {
	if width < 1 {
		width = 1
	}
	filled := min(hot*width/max(limit, 1), width)
	bar := strings.Repeat("▓", filled) + strings.Repeat("░", width-filled)

	style := dim
	if filled >= width {
		style = warm
	}
	return style.Render(bar) + dim.Render(fmt.Sprintf(" %d/%d", hot, limit))
}
