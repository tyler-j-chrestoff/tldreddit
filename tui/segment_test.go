package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/tyler-j-chrestoff/tldreddit/memory"
)

// program is a message that is nothing but a fenced block, which is the shape
// that breaks a quotation: there is no prose in it to put quotation marks round,
// and the characters that look most like prose are the ones inside a string
// literal.
const program = "```go\npackage main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println(\"hi\") }\n```\n"

// A row shows what somebody wrote, not the marks they wrote it with.
//
// This is the founder-visible half of the change and it is the one that reaches
// almost every row on the surface: the transcript's cut rows, the ranked list's
// previews and a receipt's rows are all [said]. Before it, a model's answer
// arrived on all of them as its own source.
func TestARowShowsWordsAndNotMarkdownSource(t *testing.T) {
	row := said(frame{}, memory.Bit{Payload: memory.Utterance{Text: reply}}, uncut)

	for _, mark := range []string{"###", "**", "```", "- it swaps"} {
		if strings.Contains(row, mark) {
			t.Errorf("the row carries %q, which is markdown source rather than what anybody said:\n%s", mark, row)
		}
	}
	for _, word := range []string{"REVERSING A SLICE IN PLACE", "three things worth saying", "it swaps from both ends inward"} {
		if !strings.Contains(row, word) {
			t.Errorf("the row does not carry %q:\n%s", word, row)
		}
	}

	// And the blocks are separated rather than run together. Without this the
	// heading, the sentence under it and each list item butt up against one
	// another and the row is the wall again with four characters taken off it —
	// which is what dropping the marks and not replacing them would produce.
	if strings.Count(row, " · ") < 3 {
		t.Errorf("the row runs its blocks together — %d separators for four blocks:\n%s",
			strings.Count(row, " · "), row)
	}
}

// A message with no block mark in it is what it always was, byte for byte.
//
// The fidelity gate is [structured]'s and it is shared with the block renderer
// on purpose, so this is the same claim as "prose is never rendered" made on the
// other surface. Without the share, a message could be prose under the caret and
// a rendering one row above it.
func TestAMessageWithNoBlockMarkIsUntouchedOnItsRow(t *testing.T) {
	for _, text := range []string{
		"the loop condition is i < j, not i != j",
		"2 * 3 * 4 is 24 and --dry-run takes no argument",
		"a paragraph\n\nand a second one with no mark in it",
		"snake_case_name and CONSTANT_CASE",
	} {
		b := memory.Bit{Payload: memory.Utterance{Text: text}}
		if got, want := said(frame{}, b, uncut), oneLine(b); got != want {
			t.Errorf("said(%q) = %q, want the record's own words %q", text, got, want)
		}
	}
}

// Doubled emphasis is spent on a row and a single mark never is.
//
// The asymmetry is the whole rule and it is not tidiness: a single asterisk is
// what a multiplication is made of and a single underscore is what an identifier
// is made of, so a rule that took those would be taking characters a speaker
// typed and calling them markup.
func TestOnlyDoubledEmphasisIsSpentOnARow(t *testing.T) {
	for text, want := range map[string]string{
		"- a **bold** claim":         "a bold claim",
		"- a __bold__ claim":         "a bold claim",
		"- 2 * 3 * 4 and a ** b":     "2 * 3 * 4 and a ** b",
		"- snake_case_name":          "snake_case_name",
		"- an **unclosed bold":       "an **unclosed bold",
		"- spaced ** not emph ** by": "spaced ** not emph ** by",
		"- `code` stays":             "`code` stays",
	} {
		got := lede(text + "\nand a second line")
		if !strings.HasPrefix(got, want) {
			t.Errorf("lede(%q) = %q, want it to open with %q", text, got, want)
		}
	}
}

// A quotation never opens a fence, because the two characters it is made of are
// an assertion that what is between them is what somebody said.
//
// The frame this was found on: a fold that absorbed a model's program drew
//
//	coordinator-7 "```go package main import "fmt" func main() { fmt.Println("…"
//
// — the fence markers inside the quotation, and the `"` from `import "fmt"`
// closing it four words early. What the scar falls back to is the rung with no
// account of the content in it, which [frame.quotation] already argues is better
// than a bad one.
//
// **This check is corroborating rather than sole, and it is named as such rather
// than quietly kept**, per the precedent already in this package for tests that
// differ by fixture. Every mutation it catches is also caught by
// [TestAFoldWithAProgramInItStillQuotesASentence], because the two are the same
// mechanism asked from opposite ends — one that a program is never the words, the
// other that passing over it costs the scar nothing. It is kept because it is the
// direct statement of the claim, and because the frame in the comment above is
// what a person reading this file needs to see.
func TestAScarNeverQuotesFromInsideAFence(t *testing.T) {
	f, c := scarred(t, program, program)

	if q, _ := f.quotation(c, 120); q != "" {
		t.Fatalf("a fold of nothing but programs quoted %q", q)
	}

	row := ansi.Strip(seam(f, c, 120))
	if strings.Contains(row, "`") || strings.Contains(row, `"`) {
		t.Errorf("the scar carries a fence marker or a quotation mark:\n%s", row)
	}
	if !strings.Contains(row, "2 bits") || !strings.Contains(row, "ctrl+u") {
		t.Errorf("the scar dropped the count or the key it falls back to:\n%s", row)
	}
}

// And a fold that absorbed one program and one sentence quotes the sentence.
//
// This is the half that stops the rule above from being a loss. [frame.quoted]
// ranks by this reader's votes and then by recency, so the program is the bit it
// would otherwise take — passing over it costs the scar nothing, because the
// account it can give of a sentence is a real one.
func TestAFoldWithAProgramInItStillQuotesASentence(t *testing.T) {
	f, c := scarred(t, "the null default worries me", program)

	q, _ := f.quotation(c, 120)
	if !strings.Contains(q, "the null default worries me") {
		t.Errorf("the scar quoted %q, want the sentence it absorbed", q)
	}
}

// A quotation is the speaker's own characters. A row is a rendering. The two
// share a ladder and not a text, and this is what says so.
//
// Without it, [frame.quotation] reads [said] — which is what it did — and a
// heading arrives inside quotation marks with its hashes spent and its words
// upper-cased, which is the surface editing somebody's words inside the marks
// that promise it did not.
func TestAQuotationKeepsTheMarksTheSpeakerTyped(t *testing.T) {
	f, c := scarred(t, "### Reversing a slice in place\n\nand then some prose")

	q, _ := f.quotation(c, 120)
	if !strings.Contains(q, "### Reversing a slice in place") {
		t.Errorf("the quotation is %q, want the heading exactly as it was typed", q)
	}
	if strings.Contains(q, "REVERSING") {
		t.Errorf("the quotation carries the row's own rendering:\n%s", q)
	}
}

// The word index is words. A loop index is not one.
//
// Measured before the filter: one fenced Go reply in a ten-bit fold window took
// four of the twelve slots the persona is told, and the four most prominent, with
// `j`, `s`, `1` and `t`. The bag counts by frequency and a loop variable in a
// program outruns every English word in the window.
func TestTheWordIndexSkipsPunctuationGradeTokens(t *testing.T) {
	bag := map[string]int{"j": 40, "s": 30, "1": 20, "t": 12, "backfill": 3, "migration": 2, "up": 9}

	top := topWords(func(yield func(string, int) bool) {
		for w, n := range bag {
			if !yield(w, n) {
				return
			}
		}
	}, 12)

	for _, w := range top {
		if len([]rune(w)) < 3 {
			t.Errorf("the index carries %q, which is a character rather than a word: %v", w, top)
		}
	}
	for _, w := range []string{"backfill", "migration"} {
		if !containsWord(top, w) {
			t.Errorf("the index dropped %q: %v", w, top)
		}
	}
}

func containsWord(words []string, w string) bool {
	for _, got := range words {
		if got == w {
			return true
		}
	}
	return false
}

// An unclosed fence runs to the end of the text, and a fence marker of a
// different run does not close one it never opened.
//
// Three things in this package now agree about where the code in a message is —
// goldmark by way of [markdown], [prewrapped], and [segments] — and they have to,
// because a segmenter that disagreed with the renderer would put a quotation mark
// round a fence marker on the very row the renderer is drawing as code.
func TestAFenceIsClosedOnlyByItsOwnRun(t *testing.T) {
	if got := opening("~~~\n```\nstill code\n"); got != "" {
		t.Errorf("a fence of one run was closed by another and let %q out as prose", got)
	}
	if got := opening("prose first\n\n```\ncode\n```\nand prose after"); got != "prose first" {
		t.Errorf("opening = %q, want the prose in front of the fence", got)
	}
	if got := opening("```\ncode\n```\n\nand prose after"); got != "and prose after" {
		t.Errorf("opening = %q, want the sentence under the program", got)
	}
}

// A message that is nothing but a program still says something on its row.
//
// The alternative is a blank row, which is what skipping fences outright would
// draw for it, and that is the case this whole design turns on. What it says is
// the speaker's own first line of code inside the mark this surface already means
// code by — not a count of lines and not a language name, because those would be
// the program composing a sentence about somebody's message.
func TestAMessageThatIsOnlyAProgramStillHasARow(t *testing.T) {
	row := said(frame{}, memory.Bit{Payload: memory.Utterance{Text: program}}, uncut)
	if row != "`package main`" {
		t.Errorf("a program's row is %q, want its own first line in backticks", row)
	}
}
