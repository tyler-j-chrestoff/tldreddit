package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/tyler-j-chrestoff/tldreddit/memory"
)

// document is a reply written the way a model writes one: a heading, a
// bulleted list with emphasis in it, a fenced block of Go with tabs and a long
// line, and a second fence for the program's output.
//
// It carries a tab on purpose. A tab is one column to lipgloss.Width and eight
// to a terminal, so a fixture of spaces cannot ask whether the width arithmetic
// is being told the truth — which is the same reason
// [TestAnExpandedRowSurvivesAWidthOfNothing] carries a CJK fixture.
const document = "### Reversing a slice\n\n" +
	"Two things matter:\n\n" +
	"- it swaps from **both ends**, so it allocates nothing\n" +
	"- it is generic over `T`\n\n" +
	"```go\npackage main\n\n" +
	"func reverse[T any](s []T) {\n" +
	"\tfor i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {\n" +
	"\t\ts[i], s[j] = s[j], s[i]\n" +
	"\t}\n}\n```\n\n" +
	"It prints:\n\n" +
	"```text\n[5 4 3 2 1]\n```\n"

// opened puts the caret on a bit carrying text and returns the rows its block
// drew, with the escapes stripped.
func opened(t *testing.T, text string, width int) []string {
	t.Helper()
	m := record(1)
	m.utter(m.persona.Handle(), memory.Utterance{Text: text})
	return block(t, m, width)
}

// A message written as a document is drawn as one: the heading on a row of its
// own, each list item on a row of its own, and the code block on its own rows
// indented clear of the prose.
//
// The claim this replaces is the one [saidWhole] used to make and
// [TestAnExpandedRowShowsEveryWordAndNotTheLineBreaks] used to hold with this
// same fixture — every word, none of the line breaks. That was the defect, found
// by using the thing: a person asked for a Go program and got one line.
//
// Every assertion here is about *rows*, which is the unit the defect was in.
// Colour is asserted nowhere, deliberately: the whole argument of the style is
// that the structure is carried by marks and space, so a check that reads an SGR
// would be checking the channel that does not matter and would pass on a
// terminal that shows none of it.
func TestADocumentKeepsItsHeadingsListsAndCodeBlock(t *testing.T) {
	for _, width := range []int{100, 72, 46} {
		rows := opened(t, document, width)
		text := make([]string, len(rows))
		for i, r := range rows {
			text[i] = strings.TrimRight(ansi.Strip(r), " ")
		}
		joined := strings.Join(text, "\n")

		// The heading is alone on its row, in capitals, with a blank row under it —
		// and it does not carry the hashes it was written with. All four are
		// character-carried on purpose, which is why they are asserted here on
		// stripped text rather than through any style.
		head := -1
		for i, r := range text {
			if strings.Contains(r, "REVERSING A SLICE") {
				head = i
			}
		}
		if head < 0 {
			t.Fatalf("width %d: no row carries the heading in capitals:\n%s", width, joined)
		}
		if strings.Contains(text[head], "Two things matter") {
			t.Errorf("width %d: the heading shares its row with the paragraph under it:\n%s", width, joined)
		}
		if head+1 >= len(text) || strings.TrimSpace(text[head+1]) != "" {
			t.Errorf("width %d: no blank row under the heading, so half of what says it is one is missing:\n%s",
				width, joined)
		}

		// Both list items, each opening a row of its own.
		items := 0
		for _, r := range text {
			if strings.HasPrefix(strings.TrimLeft(r, " "), "· ") {
				items++
			}
		}
		if items != 2 {
			t.Errorf("width %d: %d rows open with a bullet, want 2:\n%s", width, items, joined)
		}

		// Every line of the program is on a row of its own, in order, and none of
		// them shares a row with the prose either side.
		//
		// Searched over rows with a continuation rejoined onto the row it continues,
		// because at the narrow end a code line is broken by [prewrapped] and looking
		// for it whole would fail on the surface working correctly. Rejoining is also
		// the stronger check: it asserts the break is *only* a break, so a wrap that
		// dropped or duplicated a character shows up here rather than nowhere.
		joinedRows := rejoin(text)
		code := []string{"package main", "func reverse[T any](s []T) {", "s[i], s[j] = s[j], s[i]", "[5 4 3 2 1]"}
		at := 0
		for _, want := range code {
			found := -1
			for i := at; i < len(joinedRows); i++ {
				if strings.Contains(joinedRows[i], want) {
					found = i
					break
				}
			}
			if found < 0 {
				t.Fatalf("width %d: %q is not on any row after row %d:\n%s", width, want, at, joined)
			}
			if strings.Contains(joinedRows[found], "It prints") || strings.Contains(joinedRows[found], "Two things") {
				t.Errorf("width %d: %q shares its row with prose:\n%s", width, want, joined)
			}
			at = found
		}

		// And the code is indented past the prose, which is the only thing telling
		// code from a sentence once colour is gone.
		prose := indentOf(t, text, "Two things matter")
		body := indentOf(t, text, "package main")
		if body <= prose {
			t.Errorf("width %d: the code block starts at column %d and the prose at %d, so nothing but colour separates them:\n%s",
				width, body, prose, joined)
		}
	}
}

func indentOf(t *testing.T, rows []string, find string) int {
	t.Helper()
	for _, r := range rows {
		if strings.Contains(r, find) {
			return len(r) - len(strings.TrimLeft(r, " "))
		}
	}
	t.Fatalf("no row carries %q", find)
	return -1
}

// Prose is never drawn as a document, and every character a person typed comes
// back.
//
// This is the fidelity half of [structured]'s gate and it is the half that can
// go wrong silently. Markdown rendering *spends punctuation*: asterisks become
// weight, backticks become colour, a hyphen becomes a bullet. On the block whose
// contract is that the record's own words are on the screen, doing that to a
// sentence somebody wrote is the program re-attributing their characters to
// itself — [unmarked]'s defect, from a new direction.
//
// The fixture is every inline mark at once inside one line of ordinary prose,
// which is the case a person actually produces: arithmetic, a shell flag, a
// snake_case identifier.
func TestProseIsNeverDrawnAsADocument(t *testing.T) {
	prose := "the answer is 2 * 3 * 4, run it with --dry-run and check\ncreated_at against updated_at, and _do not_ use `rm -rf` for this"

	if structured(prose) {
		t.Fatalf("this fixture is prose and [structured] says otherwise, so the check below is not the one being made")
	}

	// Compared as a collapsed run of words, because the block wraps and a mark can
	// land either side of a row boundary — which is the screen doing its job and
	// not the thing under test. What is under test is that the characters are on
	// the screen at all.
	rows := opened(t, prose, 60)
	got := strings.Join(strings.Fields(ansi.Strip(strings.Join(rows, " "))), " ")
	for _, want := range []string{"2 * 3 * 4", "--dry-run", "created_at", "_do not_", "`rm -rf`"} {
		if !strings.Contains(got, want) {
			t.Errorf("%q is not on the screen, so a mark the speaker typed was spent:\n%s",
				want, strings.Join(rows, "\n"))
		}
	}
}

// A document on a row the next fold is taking keeps every column it had when it
// was hot.
//
// The fade is two channels here, as it is everywhere else on this surface: the
// block steps left as one object, and it loses its colour. Losing the colour is
// what [markdown]'s quiet arm is for, and the thing that could go wrong with it
// is geometry — a style that changed a prefix or a margin as well as a colour
// would make a bit re-wrap at the moment it started cooling, which is text moving
// on exactly the event the screen exists to make legible.
//
// So this asserts line for line, on the stripped rows, and it asserts the two
// differ *before* stripping — otherwise a quiet arm that was never quiet would
// pass it.
func TestACoolingDocumentKeepsEveryColumnItHadWhenItWasHot(t *testing.T) {
	for _, width := range []int{100, 46, 24} {
		hot, okHot := markdown(document, width, false)
		cool, okCool := markdown(document, width, true)
		if !okHot || !okCool {
			t.Fatalf("width %d: the fixture is not being drawn as a document (%v/%v)", width, okHot, okCool)
		}
		if strings.Join(hot, "\n") == strings.Join(cool, "\n") {
			t.Fatalf("width %d: the cooling arm is byte-identical to the hot one, so it is not dropping any colour and this check has no subject", width)
		}

		// And the cooling arm carries **no style instruction at all**, which is a
		// stronger claim than "it differs" and is the one that keeps the fade whole.
		//
		// Two separate defects sit behind it and neither is visible in the geometry.
		// Glamour registers a Chroma block under a single global style name and
		// skips the registration when that name is taken, so two configs differing
		// only in colour do not give two styles — whichever renderer draws first
		// wins for the life of the process, and the cooling arm has to have no
		// Chroma rather than a colourless one. And an inner span of *any* kind ends
		// with a full reset, so a single bold heading inside cooling.Render turns
		// the dim off for the rest of that line: measured on a frame, a cooling
		// document's heading row and every row carrying a bold span came back at
		// full brightness while the rest of the block faded.
		//
		// The check is an SGR count rather than a colour test because the second
		// defect is not about colour. Anything that opens a style here can close the
		// one wrapped around it.
		//
		// Read before any downgrade, which is safe in this one direction: a style
		// absent from the string a renderer is handed is absent from every terminal
		// it reaches. The reverse reading is the one that cannot fail and this
		// package has a scar about it.
		for i, l := range cool {
			if m := sgr.FindAllStringSubmatch(l, -1); len(m) > 0 {
				t.Errorf("width %d row %d of a cooling document carries %d style instruction(s), the first being %q: %q",
					width, i+1, len(m), m[0][1], ansi.Strip(l))
			}
		}

		if len(hot) != len(cool) {
			t.Fatalf("width %d: %d rows hot against %d cooling, so a bit re-wraps at the moment it starts cooling",
				width, len(hot), len(cool))
		}
		for i := range hot {
			h, c := ansi.Strip(hot[i]), ansi.Strip(cool[i])
			if h != c {
				t.Errorf("width %d row %d: %q hot against %q cooling", width, i+1, h, c)
			}
		}
	}
}

// No row of a document is wider than the room it was given.
//
// Two things make this a real check rather than a restatement of [clip]. Glamour
// pads every line it emits out to the wrap width, so a block drawn behind this
// surface's margin, vote and handle columns is already over the terminal before
// anything else happens — and the mark [clip] would put on it says the screen cut
// a sentence when it cut nothing but padding. And a tab is one column to
// lipgloss.Width and eight to a terminal, so a code line carrying two of them is
// measured seven columns narrower than it draws and wraps too late.
//
// Swept from one column up rather than at the sizes the program uses, because
// both faults are worse where there is least room and neither is visible at a
// hundred.
func TestNoDocumentRowRunsPastTheWidthItWasGiven(t *testing.T) {
	for width := 1; width <= 120; width++ {
		lines, ok := markdown(document, width, false)
		if !ok {
			continue
		}
		for i, l := range lines {
			if got := lipgloss.Width(l); got > width {
				t.Fatalf("width %d: row %d is %d columns: %q", width, i+1, got, ansi.Strip(l))
			}
			if strings.ContainsRune(l, '\t') {
				t.Fatalf("width %d: row %d carries a tab, which every measurement here counts as one column and a terminal draws as eight: %q",
					width, i+1, ansi.Strip(l))
			}
		}
	}
}

// A speaker who ran out of room mid-document is marked on a row of its own.
//
// [saidWhole]'s ladder hangs the mark off the end of the last line when it fits,
// which is right for a sentence and wrong for a document: the last line of a
// truncated document is normally inside a fenced block, and `╌ unfinished ╌`
// indented into a region the reader is being told is code reads as code. A row of
// its own puts it back in the prose column, where this surface's own vocabulary
// belongs.
func TestATruncatedDocumentIsMarkedOnARowOfItsOwn(t *testing.T) {
	cut := "### Steps\n\n1. first\n\n```go\nfunc main() {\n\tfmt.Prin"

	m := record(1)
	m.utter(m.persona.Handle(), memory.Utterance{Text: cut, Truncated: true})
	rows := block(t, m, 60)

	last := ansi.Strip(rows[len(rows)-1])
	if strings.TrimSpace(last) != "╌ unfinished ╌" {
		t.Fatalf("the last row of a truncated document is %q, want the mark alone on it:\n%s",
			last, strings.Join(rows, "\n"))
	}
	if len(rows) < 2 {
		t.Fatal("the block is one row, so the mark is not on a row of its own")
	}
	if strings.Contains(ansi.Strip(rows[len(rows)-2]), "╌") {
		t.Errorf("the mark is on two rows:\n%s", strings.Join(rows, "\n"))
	}
}

// [structured] is what decides whether a message is drawn as a document, and it
// is the only thing standing between a person's own punctuation and a renderer
// that spends it. So it is checked directly and in both directions.
//
// Every "no" row here is a shape somebody actually types. The hyphen with no
// space after it is the one worth naming: `well-formed` opening a line is not a
// list and never was, and a gate that read it as one would turn a sentence into a
// bullet.
func TestStructuredIsBlockMarksAndNothingElse(t *testing.T) {
	for _, c := range []struct {
		want bool
		text string
	}{
		{true, "intro\n\n### heading\n\nmore"},
		{true, "intro\n\n- one\n- two"},
		{true, "intro\n\n1. one\n2. two"},
		{true, "intro\n\n1) one"},
		{true, "here it is\n\n```go\nx := 1\n```"},
		{true, "here it is\n\n~~~\nx := 1\n~~~"},
		{true, "a list under prose\n  - indented item"},

		{false, "one line with ### in the middle of it"},
		{false, "a sentence about 2 * 3 and nothing else"},
		{false, "two lines\nand the second one is prose too"},
		{false, "-well-formed opens this line\nand it is not a list"},
		{false, "###nospace\nis not a heading"},
		{false, "#######\ntoo many hashes to be one"},
		// Seven hashes *and* a space, which is the only row that reaches the upper
		// bound in [atx]. The row above it does not: with no space after the hashes
		// it is refused by the space rule whatever the count is, so a version of
		// [atx] with no upper bound at all passes it. That was recorded as caught
		// in this session's first mutation table and it was not — the table cell
		// was wrong, in the way this seat's own record has a scar about.
		{false, "####### seven is too many\nand this is not a heading"},
		{false, "*emphasis* opens this line\nand it is not a bullet"},
		{false, "1.no space after the dot\nis not an item"},
		{false, "```a fence needs a line break to matter"},
		{false, ""},
	} {
		if got := structured(c.text); got != c.want {
			t.Errorf("structured(%q) = %v, want %v", c.text, got, c.want)
		}
	}
}

// The record is untouched by any of this.
//
// The whole change is a view change and the one way it could stop being one is if
// something here reached [oneLine], which answers "what does the record hold" and
// is what the reachability tests read. So: file a document, and assert the stored
// text and oneLine's reading of it are exactly what they were, mark for mark.
func TestDrawingADocumentChangesNothingTheRecordHolds(t *testing.T) {
	m := record(1)
	m.utter(m.persona.Handle(), memory.Utterance{Text: document})

	b := m.shown.Bits(m.store)[len(m.shown)-1]
	if got := b.Payload.(memory.Utterance).Text; got != document {
		t.Errorf("the record holds %q, want the document byte for byte", got)
	}
	if got, want := oneLine(b), strings.Join(strings.Fields(document), " "); got != want {
		t.Errorf("oneLine reads %q, want the collapsed text it always gave: %q", got, want)
	}

	// And drawing it does not file anything.
	before := m.store.Len()
	_ = block(t, m, 100)
	_ = block(t, m, 40)
	if m.store.Len() != before {
		t.Errorf("drawing put %d bits in the record", m.store.Len()-before)
	}
}

// A document is a document at every width: there is no column at which the
// surface changes character.
//
// This is the negative result [markdown]'s doc states, held as a check rather
// than as a sentence. A floor would mean a person dragging a terminal narrower
// watches a document turn into a wall of text at one particular column and back
// again — which is the same failure as a wider terminal showing fewer characters,
// a thing this package has been caught by three times. If somebody adds a floor,
// this is what says so.
func TestADocumentIsADocumentAtEveryWidth(t *testing.T) {
	drawn := 0
	for width := 1; width <= 200; width++ {
		lines, ok := markdown(document, width, false)
		if !ok {
			t.Fatalf("width %d: refused, so there is a width at which this surface changes character", width)
		}
		if len(lines) == 0 {
			t.Fatalf("width %d: drawn as a document and empty", width)
		}
		drawn++
	}
	if drawn != 200 {
		t.Fatalf("%d widths drawn, want 200", drawn)
	}
}

// A document does not show its own source.
//
// This is the check the whole style answers to and it is deliberately the
// crudest one in the file: strip every escape, and what is left must read as a
// document and not as a text file somebody forgot to render. Two halves.
//
// **Nothing that is only markup survives.** A hash opening a row is markup: it
// says what the row is, and once the row is drawn as a heading it says nothing
// the drawing does not. A fence is markup. An emphasis asterisk is markup.
//
// **And the structure is still there without them**, which is the half that
// makes the first half safe rather than lossy — the heading is in capitals and
// nothing else is, so it is findable with no colour, no weight and no hashes.
//
// A backtick is deliberately *not* in the first list, and the reason is in
// [mdStyle]: it is the only carrier of where a code span begins and ends in a
// channel made of characters, so removing it loses something the reader cannot
// get back from the drawing at any terminal width.
func TestADocumentDoesNotShowItsOwnSource(t *testing.T) {
	for _, width := range []int{100, 72, 46} {
		lines, ok := markdown(document, width, true)
		if !ok {
			t.Fatalf("width %d: the fixture is not being drawn as a document", width)
		}
		joined := strings.Join(lines, "\n")

		for _, l := range lines {
			bare := strings.TrimLeft(ansi.Strip(l), " ")
			if strings.HasPrefix(bare, "#") {
				t.Errorf("width %d: a row still opens with a hash, so the heading is drawn and its markup is kept too: %q\n%s",
					width, bare, joined)
			}
			if strings.HasPrefix(bare, "```") || strings.HasPrefix(bare, "~~~") {
				t.Errorf("width %d: a fence marker is on the screen: %q\n%s", width, bare, joined)
			}
		}
		if strings.Contains(ansi.Strip(joined), "**") {
			t.Errorf("width %d: emphasis markup is on the screen:\n%s", width, joined)
		}

		// And a code span still has its delimiters, which is the other side of the
		// same rule rather than an exception to it. A hash is redundant with the
		// heading that gets drawn; a backtick pair is the only thing in a channel
		// made of characters saying where the span starts and stops, and this arm
		// has no colour to carry it instead. Drawn bare the deciding row reads
		// `the loop condition is i < j`, which is a sentence with a comparison in
		// it. See [mdStyle].
		if !strings.Contains(ansi.Strip(joined), "`T`") {
			t.Errorf("width %d: the code span lost its delimiters, so nothing on a colourless row says it is code:\n%s",
				width, joined)
		}

		// And the heading is still findable with everything stripped, because it is
		// the only row in capitals. Compared against the body rather than asserted
		// in isolation: a check that merely found an uppercase row would pass on a
		// document that had been shouted at wholesale.
		caps := 0
		for _, l := range lines {
			b := strings.TrimSpace(ansi.Strip(l))
			if b != "" && b == strings.ToUpper(b) && strings.ContainsAny(b, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") {
				caps++
			}
		}
		if caps != 1 {
			t.Errorf("width %d: %d rows are in capitals, want exactly the heading:\n%s", width, caps, joined)
		}
	}
}

// A line too long for a fenced block is broken inside the block, and the break
// is marked and stays in the block's own column.
//
// Left to glamour, a code line that does not fit is continued at column zero —
// level with the prose either side of it. The indent is the only thing telling
// code from prose once colour is gone, so that continuation reads as a sentence:
// measured on the real binary at 52 columns, `// reverse turns s around in` sat
// at the code column and `in place. It allocates` sat at the prose column, one
// row apart, inside one comment.
//
// So the assertion is about columns and about the mark, and it is swept across
// the widths where a break actually happens rather than at one size.
func TestAWrappedCodeLineStaysInsideItsBlock(t *testing.T) {
	broke := 0
	for width := 30; width <= 60; width++ {
		lines, ok := markdown(document, width, true)
		if !ok {
			t.Fatalf("width %d: not drawn as a document", width)
		}
		joined := strings.Join(lines, "\n")

		body := indentOf(t, lines, "package main")
		prose := indentOf(t, lines, "Two things matter")

		for i, l := range lines {
			bare := ansi.Strip(l)
			if !strings.Contains(bare, "…") {
				continue
			}
			broke++
			at := len(bare) - len(strings.TrimLeft(bare, " "))
			if at != body {
				t.Errorf("width %d row %d: a continuation starts at column %d, the code block at %d and the prose at %d:\n%s",
					width, i+1, at, body, prose, joined)
			}
			if !strings.HasPrefix(strings.TrimLeft(bare, " "), "… ") {
				t.Errorf("width %d row %d: %q does not open with the cut mark:\n%s", width, i+1, bare, joined)
			}
		}
	}
	if broke == 0 {
		t.Fatal("no code line was broken at any width in the sweep, so nothing here is being tested")
	}
}

// rejoin puts a continuation back onto the row it continues, so a check about
// what is on a row can be written once and still hold at the widths where
// [prewrapped] has had to break a code line.
//
// It is the inverse of exactly one thing and nothing else: the "… " a
// continuation opens with. Anything that arrived on the screen by some other
// route is left alone, so this cannot quietly repair a defect it was not written
// for.
func rejoin(rows []string) []string {
	var out []string
	for _, r := range rows {
		bare := strings.TrimLeft(r, " ")
		if tail, cut := strings.CutPrefix(bare, "… "); cut && len(out) > 0 {
			out[len(out)-1] += tail
			continue
		}
		out = append(out, r)
	}
	return out
}

// A code line broken to fit reassembles into exactly what the speaker wrote.
//
// This is what makes the break a cut with a receipt rather than a cut. Every
// other wrap on this surface breaks at a word, which is right for prose and
// wrong here: [ansi.Wrap] consumes the space it breaks at, so a code line broken
// on a word boundary comes back one character short and a reader who copies it
// out has something the speaker did not write. Broken at the column, the rows
// concatenate back exactly.
//
// Swept across the widths where breaks happen, and it counts what it checked, so
// a fixture that stopped producing a break reports that rather than passing.
func TestAWrappedCodeLineIsExactlyReversible(t *testing.T) {
	const long = "```go\n" +
		"func reverseTheWholeSliceInPlace[T comparable](s []T, from, to int) (int, error) {\n" +
		"\treturn 0, nil\n}\n```\n"
	src := "intro\n\n" + long

	checked := 0
	for width := 20; width <= 90; width++ {
		lines, ok := markdown(src, width, true)
		if !ok {
			t.Fatalf("width %d: not drawn as a document", width)
		}

		var got string
		seen := false
		for _, l := range lines {
			bare := strings.TrimLeft(ansi.Strip(l), " ")
			if tail, cut := strings.CutPrefix(bare, "… "); cut {
				got += tail
				continue
			}
			if strings.HasPrefix(bare, "func reverseTheWholeSliceInPlace") {
				got, seen = bare, true
				continue
			}
			if seen && got != "" {
				break
			}
		}
		if !seen {
			continue
		}
		want := "func reverseTheWholeSliceInPlace[T comparable](s []T, from, to int) (int, error) {"
		if got != want {
			t.Errorf("width %d: the rows reassemble to\n  %q\nwant\n  %q", width, got, want)
			continue
		}
		checked++
	}
	if checked < 40 {
		t.Fatalf("only %d widths carried the line, so the sweep is not reaching what it claims", checked)
	}
}
