package tui

import (
	"strings"
	"sync"

	"charm.land/lipgloss/v2"

	glamour "charm.land/glamour/v2"
	gansi "charm.land/glamour/v2/ansi"
)

// Rendering a message as a document rather than as a run-on sentence.
//
// # Why this exists
//
// [oneLine] collapses every whitespace run in a bit to a single space, and it is
// right to: every column measurement on this surface is arithmetic on one line,
// and a row is one row. [saidWhole] inherited that collapse when it started
// drawing the caret's row whole, and its own doc named the loss — "a numbered
// list comes back as a run-on sentence". Used for real, that is not a footnote.
// A person asked a model for a Go program and the answer arrived as an unbroken
// wall: headings inline, list items inline, forty lines of code on one line, and
// nothing telling code from prose. The record held all of it correctly; the view
// was the only thing that had lost it.
//
// So this is a view change and only a view change. Nothing here reaches the
// store, nothing here reaches a content address, and [oneLine] — which answers
// "what does the record hold" and is what the reachability tests read — is not
// touched.
//
// **The collapse this file was written against is gone from the other path
// too**, and the reason is worth having here rather than only at [wrapped]: the
// first thing this gate was described as doing was keeping a person's
// punctuation and spending a model's, and a message that is *not* written as a
// document is where a person's own text mostly lives — a pasted source file has
// no fence, no heading and no list item anywhere in it. Flattening that is not
// keeping anybody's punctuation. So the fallback preserves the lines it was
// given, this gate keeps its exact job, and the job is now only the one it can
// argue for: whether the marks in a message may be *interpreted*.
//
// # When a message is drawn as a document, and why it is not always
//
// Only when it is written as one: [structured] requires a line break and at
// least one block-level mark — a fence, an ATX heading, or a list item at the
// start of a line. Everything else goes to [wrapped], which spends nothing at
// all: no mark is interpreted, no language is guessed at, and the message's own
// lines and indentation come back as the speaker wrote them.
//
// That gate is a fidelity rule rather than a performance one. Markdown rendering
// *removes characters*: `**bold**` loses four asterisks, a `-` bullet becomes a
// `·`, a fence disappears entirely. On the surface whose contract for this block
// is that every word the record holds is on the screen, spending a participant's
// own punctuation is the same class of thing [unmarked] was fixed for — the
// program re-attributing a speaker's characters to itself. A person typing
// `the a*b product` into a single-line message is the common case and it is
// never rendered; a model answering with three fenced blocks is the case this
// exists for and it always is. Neither needs a legend, because prose still looks
// like prose.
//
// # What it degrades to
//
// The style below carries structure in *marks and space* before colour, which is
// this package's standing rule and is the reason the heading keeps its `#`
// characters instead of being drawn bold-and-blue. Under `NO_COLOR`, a pipe or a
// screenshot, a heading is still `### Something`, a bullet is still `· `, and a
// code block is still indented two columns clear of the prose. Colour is the
// last channel and it fails first; here it fails to a document that still reads
// as one.
//
// # Where this runs, relative to the request goroutine
//
// On the update loop, always. [persona.Client.Reply] runs on a tea.Cmd goroutine
// and a panic in one lands there, but no rendering happens on that goroutine:
// the reply arrives as a message, [Model.recordReply] files it inside Update, and
// every call into this file is reached from View by way of [transcript] or
// [ranked]. That matters because a glamour.TermRenderer is **not** safe for
// concurrent Render calls — goldmark's block stack carries state across the
// public API, so two Renders in flight corrupt each other's output rather than
// racing on a word. The mutex below is therefore a backstop and not the
// mechanism: it is here because the cost is one uncontended lock per frame and
// the failure it prevents is silent, and because `go test` is entitled to run
// two of these at once even where the program never will.

// mdMu guards mdCache and serializes Render.
//
// One mutex for both, rather than a cache lock and a per-renderer lock, because
// this surface renders at most one block per frame and there is nothing to be
// won by finer grain. Crush splits them because it streams; we do not.
var (
	mdMu    sync.Mutex
	mdCache = map[mdKey]*glamour.TermRenderer{}
)

// always is a true nobody may turn off. See [mdStyle]'s Heading.
var always = func() *bool { t := true; return &t }()

type mdKey struct {
	width int
	quiet bool
}

// There is deliberately no width below which this refuses, and that is a
// measured result rather than an omission.
//
// A floor was looked for. Every other threshold in this package has one and the
// standing rule is that a floor belongs to the geometry it was measured in, so
// the expectation going in was that a document shreds somewhere and the plain
// wrap should take over below it. Swept over the fixture in
// [TestHarnessDocumentFloor], no width comes back where refusing is better.
//
// **That sweep is re-run and its first sentence is corrected**, because what it
// compares against changed underneath it: the plain arm used to be a flatten and
// is now [wrapped], which keeps a message's own lines. This paragraph said the
// document costs more rows "at every width from one column up", and against the
// real fallback that is false in a band — read off the sweep, the plain wrap is
// cheaper in rows from about five columns to about nineteen, and dearer from
// twenty up. What survives is the part that decides: inside the band the surface
// can actually produce — [textFloor] and wider, see below — the two are within a
// row of each other, so there is nothing to buy by switching, and at the widths
// where a document is rubble the wall is rubble too.
//
// The figures move when either renderer does, which is why they are a sweep you
// run rather than a table kept here: `HARNESS=1 go test ./tui/ -run
// TestHarnessDocumentFloor -v`.
//
// And a floor would cost something real: a width at which the screen changes
// character. A person dragging a terminal narrower would watch a document turn
// into a wall of text at one particular column and back again, which is the
// same failure as a wider terminal showing fewer characters — this package has
// been caught by that three times and each time the fix was to remove the
// discontinuity rather than to move it. A document is a document at every width.
//
// What does bound this is the caller's own gate, which is older than this file:
// [transcript] draws a row whole only when the sentence column is at least
// [textFloor], and cuts it to one line below that. So nothing here is ever asked
// for a room narrower than that in the program, and the sweep above is reaching
// past what the surface can actually produce.

// structured reports whether a bit's text is written as a document: it has a
// line break in it, and at least one line opens with a block-level mark.
//
// Block-level only, deliberately. Emphasis, inline code and links are all
// *inside* a line and none of them makes a message a document — a sentence with
// an asterisk in it is a sentence. What this looks for is the three marks a
// person or a model uses to say "this part is separate from that part": a fence,
// a heading, a list item.
//
// The line break is required for the same reason. A one-line message cannot have
// structure to lose, so rendering one could only ever spend its punctuation.
//
// It asks [segments] rather than scanning for the marks itself, which it did
// until the one-row surfaces needed the same cut. Two scans for one set of marks
// agree on the day they are written, and the way they would come apart here is
// the worst one available: a message the block renderer calls a document and the
// segmenter calls prose is a message drawn one way under the caret and quoted
// another way in a scar.
func structured(text string) bool {
	if !strings.Contains(text, "\n") {
		return false
	}
	for _, s := range segments(text) {
		if s.kind != segProse {
			return true
		}
	}
	return false
}

// atx is `# ` through `###### `: one to six hashes and then a space. The space
// is required, so a shell comment or a Go build tag is not a heading.
func atx(t string) bool {
	n := 0
	for n < len(t) && t[n] == '#' {
		n++
	}
	return n >= 1 && n <= 6 && n < len(t) && t[n] == ' '
}

// bullet is `- `, `* `, `+ `, `1. ` or `1) `: a list item's opening.
//
// The trailing space is required here too. Without it `*emphasis*` opening a
// line would read as a bullet, which is the inline-versus-block confusion this
// whole gate exists to avoid.
func bullet(t string) bool {
	if len(t) >= 2 && (t[0] == '-' || t[0] == '*' || t[0] == '+') && t[1] == ' ' {
		return true
	}
	n := 0
	for n < len(t) && t[n] >= '0' && t[n] <= '9' {
		n++
	}
	return n > 0 && n+1 < len(t) && (t[n] == '.' || t[n] == ')') && t[n+1] == ' '
}

// markdown draws a bit's text as a document in the room it was given, or reports
// that it will not.
//
// quiet drops every colour and keeps every mark and every column, so a block on
// a row the next fold is taking fades whole instead of arguing with itself. That
// is the fade doing what it does everywhere else here: a cooling row is dim, and
// a syntax-highlighted block drawn inside a dim style is not dim, because the
// inner colours win (see [cell], which records the same fact about a handle).
// Dropping the colour is a stronger statement of the same thing and it costs no
// geometry — [TestACoolingDocumentKeepsEveryColumnItHadWhenItWasHot] is the pin
// that the two agree line for line.
//
// It refuses in two cases and both fall back to the plain wrap: the text is not
// written as a document, or glamour returned an error. The last is not defensive tidiness — a renderer that fails
// on one message must not take the transcript with it, and the plain wrap is a
// complete rendering of the same bit rather than a placeholder.
func markdown(text string, width int, quiet bool) (out []string, ok bool) {
	if width < 1 || !structured(text) {
		return nil, false
	}

	// A recover around somebody else's parser, on content this program did not
	// write. It is a backstop rather than a check and it is disclosed here rather
	// than left to be deleted as paranoia: the reason it exists is that the first
	// run of this code panicked out of chroma.MustNewStyle, three packages down,
	// on a style value glamour accepted and chroma did not — a panic reached from
	// Render, on a message somebody had sent, which takes the whole surface down
	// and loses the composer's draft with it. That particular cause is fixed. What
	// is not fixed and cannot be is that goldmark, chroma and a lexer chosen by an
	// info string are three parsers running on text a model produced, and the
	// honest posture toward all three is that a message may not be able to crash
	// the record's window onto itself. The fallback is a complete rendering of the
	// same bit, not a placeholder.
	defer func() {
		if recover() != nil {
			out, ok = nil, false
		}
	}()

	// Tabs are expanded before anything measures the text, and this is a
	// correctness step rather than a preference. A terminal draws a tab to the
	// next stop; lipgloss.Width and ansi.Truncate count it as one column. Left in,
	// a code line carrying two tabs is measured seven columns narrower than it
	// draws, so glamour wraps it too late and the row runs past the margin — the
	// one thing every row on this surface may not do. The record keeps the tab;
	// this is the view, which is the only place anything is allowed to change.
	text = strings.ReplaceAll(text, "\t", "    ")
	text = prewrapped(text, width)

	mdMu.Lock()
	defer mdMu.Unlock()

	key := mdKey{width, quiet}
	r, ok := mdCache[key]
	if !ok {
		var err error
		r, err = glamour.NewTermRenderer(
			glamour.WithStyles(mdStyle(quiet)),
			glamour.WithWordWrap(width),
		)
		if err != nil {
			return nil, false
		}
		mdCache[key] = r
	}

	rendered, err := r.Render(text)
	if err != nil {
		return nil, false
	}

	// Glamour pads every line out to the wrap width, and this takes the padding
	// back off.
	//
	// **It was a backstop with no mutation and it is not one any more, which is
	// worth leaving in the file rather than tidying.** When it was written the trim
	// was provably inert: both callers ask for a room that is the terminal width
	// minus their own lead, so lead plus room is the width exactly and a padded
	// line landed flush rather than over — checked by rendering
	// [TestHarnessDocument] at both sizes with and without it and diffing, not by
	// reading the arithmetic. It was kept anyway, as a guard against a surface that
	// draws a block at a lead it did not subtract.
	//
	// What made it live is [prewrapped], added afterwards for a different reason.
	// A code line broken to fit is only honest if the rows put back together are
	// what the speaker wrote, and glamour's padding sits between them — so removing
	// this trim now reddens [TestAWrappedCodeLineIsExactlyReversible]. The guard
	// nobody could catch acquired a witness from a change that was not about it.
	//
	// The [clip] beside it is separately live: a code line can genuinely exceed the
	// room glamour was given, and removing the clip reddens
	// [TestNoDocumentRowRunsPastTheWidthItWasGiven].
	lines := strings.Split(rendered, "\n")
	for i, l := range lines {
		lines[i] = clip(strings.TrimRight(l, " "), width)
	}

	// And the blank rows at either end go, for the same reason the padding does:
	// they are glamour framing a document on a page, and this block is not on a
	// page — it sits between a speaker's handle and the next thing anybody said.
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		return nil, false
	}
	return lines, true
}

// mdStyle is this surface's markdown style, and every choice in it is the same
// choice made everywhere else in this package: marks and space before colour,
// nothing that fills a background, nothing that needs a legend.
//
//   - **Headings keep their hashes.** Bold and a colour is glamour's default and
//     is the wrong trade here: bold is one SGR and a colour is another, and on
//     the terminals this package promises to degrade honestly onto there is
//     neither. `### Reversing a slice` is four columns of noise to a professional
//     and the only thing a layman on a monochrome pipe has. The depth is legible
//     from the count, which no weight can carry.
//   - **A bullet is `· `**, the separator glyph this surface already uses, rather
//     than `•`. One dot vocabulary, not two.
//   - **A code block is indented two columns and never boxed.** A background or a
//     border is colour doing structural work; an indent is space, and space is
//     what survives. It is also what tells a wrapped code line from a paragraph
//     at every width where the block still reads.
//   - **No margin on the document.** The gutter this block sits in belongs to
//     [transcript], which has already spent it on the caret, the vote and the
//     handle. A second margin from inside would be two functions indenting one
//     object.
//
// # The one place this package does not use an ANSI index, and why
//
// tui/style.go says the palette is ANSI-indexed on purpose, so it inherits
// whatever theme the terminal already has instead of fighting it. The syntax
// colours below are 24-bit hex and they break that rule. They are not allowed to
// do otherwise: glamour hands its Chroma block to chroma, and chroma's own
// parser accepts `#rrggbb`, `bold`, `italic` and nothing else — an index goes in
// as `unknown style element "240"` and comes back out of `chroma.MustNewStyle`
// as a **panic, inside Render, on a message somebody sent**. That was found by
// running it rather than by reading it, and it is the reason the recover in
// [markdown] is there as well as this comment.
//
// So the six values are picked to survive the downgrade rather than to be a
// theme: each one sits nearest a standard ANSI slot, so a 256-colour terminal
// gets roughly these and a 16-colour terminal gets blue, cyan, green, yellow,
// grey and red, which is what a syntax palette has meant since before any of
// this. Under no colour at all they are simply gone, and the block is still
// indented two columns clear of the prose, which is the channel that was
// carrying the structure anyway.
func mdStyle(quiet bool) gansi.StyleConfig {
	s := func(v string) *string { return &v }
	u := func(v uint) *uint { return &v }

	colour := func(v string) *string {
		if quiet {
			return nil
		}
		return s(v)
	}

	// The quiet arm emits **no styling at all**, not merely no colour, and that is
	// a correction rather than a simplification. Drawn with bold left in, a cooling
	// document did not fade: glamour closes an inner span with a full reset, so
	// `\x1b[1m### \x1b[m` inside cooling.Render(...) turns the dim off for the whole
	// rest of that line — the heading row and every line carrying a **bold** span
	// came back at full brightness on a block the next fold was taking, which is
	// the fade's own antecedent failing on the one row a person had opened. Seen on
	// a frame, not derived: the harness colour column read `- 35 1` where every
	// other row of the same block read `38;5;242`.
	//
	// This is the [ansi.Wrap] hazard this seat already has a note about, arriving
	// through a different library: a styled substring is only safe inside an outer
	// style if it restores what it interrupted, and nothing in this ecosystem does.
	// The fix that does not depend on anybody's reset discipline is to have nothing
	// to interrupt, so a cooling block is plain characters and the fade is one
	// unbroken run. It costs nothing: what the block is carrying is `###`, `· `,
	// backticks and an indent, and every one of those is a character.
	b := func(v bool) *bool {
		if quiet {
			return nil
		}
		return &v
	}

	// Inline code keeps the backticks the speaker typed. A heading loses its
	// hashes. Those look like the same decision made two ways and they are not, and
	// the difference is the whole rule.
	//
	// **A hash is positional syntax.** It exists to say what the line is, the
	// parser eats it, and once the line is drawn as a heading it says nothing the
	// drawing does not. Strip it and nothing is lost, because an uppercase row with
	// air under it *is* the heading — the information moved channels rather than
	// going away.
	//
	// **A backtick pair is delimiting syntax**, and in a channel made only of
	// characters it is the sole carrier of where the span starts and stops. Strip
	// it and there is nothing left: drawn bare, the deciding frame reads
	// `It prints [5 4 3 2 1].` and `the loop condition is i < j`, which are an
	// English sentence with some brackets in it and an English sentence with a
	// comparison in it. The reader cannot recover the span from the drawing, and no
	// wider terminal gives it back. Colour could carry it and colour is exactly
	// what may not, because this block goes grey when its bit starts cooling.
	//
	// So the test is not "is this mark markdown" but **"is the mark redundant with
	// what is drawn"**. And it settles the reuse question at the same time: a
	// backtick here is not new vocabulary this surface is minting, it is a
	// character the speaker typed, which is the one kind of mark this block is
	// already contracted to show.
	//
	// **bold** is the third case and it goes the other way: emphasis carries
	// loudness rather than meaning, so losing it loses nothing a reader needs, and
	// four characters of markup in every emphatic phrase would buy nothing.
	// Prefix and Suffix rather than BlockPrefix and BlockSuffix, which is not
	// interchangeable and cost a round to find. CodeSpanElement.Render reads
	// Prefix and Suffix and nothing else (glamour v2.0.1, ansi/codespan.go); the
	// Block pair is dropped on the floor for this element. Glamour's own
	// no-colour style sets block_prefix here, so the backticks it means to keep
	// under notty are not kept — read at the source, not remembered.
	code := gansi.StylePrimitive{Color: colour("6"), Prefix: "`", Suffix: "`"}
	return gansi.StyleConfig{
		Document:  gansi.StyleBlock{Margin: u(0)},
		Paragraph: gansi.StyleBlock{},

		// A heading is drawn in capitals with a blank row under it, and it does not
		// keep its hashes. Both halves are character-carried, so both survive the
		// fade and a terminal with no colour, and neither costs a column.
		//
		// Capitals rather than a glyph because there is no glyph to spare. This
		// surface means something by a solid rule (a scar), a dashed one
		// (unfinished), a vertical bar (quoted out of the record), a triangle (a
		// ballot), half a stroke (a covered row) and a middle dot (a separator) —
		// six pieces of vocabulary already, every one of them load-bearing. A
		// seventh mark for "heading" would have to be learned, and this is the
		// oldest heading convention a terminal has: it is what a man page does, and
		// nobody has ever needed it explained.
		//
		// The blank row is the other half and it is doing more work than the case
		// is. Space is the channel this package trusts before any other — the fade
		// is drawn in it, the left edge's three bands are drawn in it — and it is
		// the one thing no downgrade can take away.
		//
		// **The depth is dropped, and that is a decision rather than an omission.**
		// A count of hashes is the only thing that carried it and capitals cannot;
		// indenting the body under each level was drawn and rejected, because it
		// spends two columns of every prose line on this surface's scarcest
		// dimension and because a two-column relative indent is the fade's own step,
		// which would put two different meanings in one channel one row apart. What
		// depth buys a reader of a chat reply is close to nothing — models emit ##
		// and ### interchangeably — and the record still has it.
		//
		// Upper does **not** go through b. That helper drops a style in the quiet
		// arm, which is right for bold and italic and wrong here: capitals are not a
		// style instruction, they emit no escape at all, and they are precisely the
		// half of this that has to survive the fade. Written with b first, a cooling
		// heading came back in mixed case and indistinguishable from body text —
		// the exact failure this whole design is arranged to prevent, introduced by
		// the arrangement itself.
		Heading: gansi.StyleBlock{StylePrimitive: gansi.StylePrimitive{Upper: always, BlockSuffix: "\n"}},
		H1:      gansi.StyleBlock{},
		H2:      gansi.StyleBlock{},
		H3:      gansi.StyleBlock{},
		H4:      gansi.StyleBlock{},
		H5:      gansi.StyleBlock{},
		H6:      gansi.StyleBlock{},

		List:        gansi.StyleList{LevelIndent: 2},
		Item:        gansi.StylePrimitive{BlockPrefix: "· "},
		Enumeration: gansi.StylePrimitive{BlockPrefix: ". "},
		Task:        gansi.StyleTask{Ticked: "[x] ", Unticked: "[ ] "},

		Strong: gansi.StylePrimitive{Bold: b(true)},
		Emph:   gansi.StylePrimitive{Italic: b(true)},

		// A horizontal rule is drawn as three dashes and not as a full-width bar.
		// A run of dashes across this screen is a scar's rule or an edge indicator,
		// and both of those are claims the harness is making about the record. A
		// participant's own divider may not wear them.
		HorizontalRule: gansi.StylePrimitive{Format: "\n---\n"},

		// The blockquote gutter is the receipt's, because it is the same idea: this
		// material is quoted from somewhere else.
		BlockQuote: gansi.StyleBlock{Indent: u(1), IndentToken: s(gutter)},

		Code:      gansi.StyleBlock{StylePrimitive: code},
		CodeBlock: gansi.StyleCodeBlock{StyleBlock: gansi.StyleBlock{Margin: u(2)}, Chroma: chroma(quiet)},

		Link:      gansi.StylePrimitive{Color: colour("4"), Underline: b(true)},
		LinkText:  gansi.StylePrimitive{Bold: b(true)},
		Image:     gansi.StylePrimitive{Underline: b(true)},
		ImageText: gansi.StylePrimitive{Format: "image: {{.text}}"},

		Table: gansi.StyleTable{
			CenterSeparator: s("+"),
			ColumnSeparator: s("|"),
			RowSeparator:    s("-"),
		},
	}
}

// chroma is the syntax palette, or nothing at all when the block is cooling.
//
// Nothing at all, rather than the same entries with their colours dropped, and
// the reason is a property of glamour rather than a preference. It registers a
// Chroma block under **one global style name** and skips the registration if
// that name is already taken (`ansi/codeblock.go`, `chromaStyleTheme`), so two
// configs differing only in colour do not produce two styles: whichever renderer
// draws first wins for the life of the process, and the other silently gets its
// palette. A cooling block would come back highlighted, or a hot one would come
// back grey, depending on which bit the caret happened to be on first — a
// difference nothing on the screen would explain.
//
// So the cooling variant has no Chroma and no Theme, which is chroma not running
// at all. That is also what it should be on the merits: a row the next fold is
// taking is dim, and a syntax theme inside a dim style is not dim, because the
// inner colours win.
//
// The values are hex because chroma refuses anything else — see [mdStyle]. Each
// is the nearest thing to a standard slot: blue for keywords, cyan for types and
// functions, green for strings, orange for numbers, grey for comments.
func chroma(quiet bool) *gansi.Chroma {
	if quiet {
		return nil
	}
	s := func(v string) *string { return &v }
	at := func(hex string) gansi.StylePrimitive {
		return gansi.StylePrimitive{Color: s(hex)}
	}
	return &gansi.Chroma{
		Text: gansi.StylePrimitive{},

		// **Error is deliberately unset**, which is a colour removed rather than
		// chosen, and it was removed on a frame. [prewrapped] puts this surface's
		// own cut mark inside the fenced text, so chroma lexes it — and as a token
		// no language knows, which made it the one thing on the row drawn in the
		// error colour. The screen was saying *the lexer could not read this* about
		// a character the screen had just put there.
		//
		// Left unset rather than mapped somewhere quieter, because the reading it
		// would carry is one this surface has no business making: the record holds
		// text a participant sent, not a compile, and whether some lexer recognised
		// it is a fact about chroma's tables. Unlexable code now draws in the
		// terminal's own foreground, which is what it is.
		Comment:          at("#565f89"),
		CommentPreproc:   at("#565f89"),
		Keyword:          at("#7aa2f7"),
		KeywordReserved:  at("#7aa2f7"),
		KeywordNamespace: at("#7aa2f7"),
		KeywordType:      at("#7dcfff"),
		Operator:         gansi.StylePrimitive{},
		Punctuation:      gansi.StylePrimitive{},
		Name:             gansi.StylePrimitive{},
		NameBuiltin:      at("#7dcfff"),
		NameClass:        at("#7dcfff"),
		NameFunction:     at("#7dcfff"),
		Literal:          at("#9ece6a"),
		LiteralString:    at("#9ece6a"),
		LiteralNumber:    at("#ff9e64"),
		LiteralDate:      at("#ff9e64"),
	}
}

// prewrapped breaks the long lines inside a fenced block before glamour sees
// them, and marks each continuation with this surface's own cut mark.
//
// # What it fixes
//
// Glamour indents a code block two columns and then wraps its lines at the full
// width, so a line that does not fit is continued **at column zero** — level with
// the prose either side of the block. Measured on the real binary at 52 columns:
// `// reverse turns s around in` sat at the code column and `in place. It
// allocates` sat at the prose column, one row apart, inside the same comment.
// The indent is the only thing telling code from prose once colour is gone, so
// under NO_COLOR that continuation simply reads as a sentence.
//
// # Why it is done here rather than fought with there
//
// Because there is nothing to fight. The wrap is glamour's to do only if a line
// arrives too long for it; handed lines that already fit, it leaves them alone.
// So this is not a workaround layered over the renderer's behaviour, it is the
// caller sizing its input — the same thing the tab expansion above it does, for
// the same reason, one line earlier.
//
// # The mark, and why it is not a new one
//
// A continuation opens with `… `, which is what this surface has always meant by
// a cut: [clip], [cell], [said] and [abridged] all write it, and what it says
// there is *the screen ran out of columns and a wider one gives this back*. That
// is exactly and literally true of this cut, which is why it is the right mark
// rather than merely an available one. Two columns of indent alone was drawn and
// rejected — a continuation indented past the code column is indistinguishable
// from a genuinely nested line, and Go is a language full of nested lines.
//
// # What it costs, stated because it is real
//
// The code drawn is no longer byte-identical to the code in the record: this
// inserts breaks that the speaker did not write. That is the same trade every
// other line of this file makes and it is bounded the same way — the record has
// the original, `ctrl+u` and `tldr top` read it, and the mark says a cut happened
// rather than hiding it. What it is *not* is silent, which is what it replaced.
//
// The break is at the column and not at a word, which is the opposite of what
// every other wrap on this surface does and is right here for one reason:
// [ansi.Wrap] consumes the space it breaks at, so a code line broken on a word
// boundary is a code line with a character missing, and a reader who copies it
// back out gets something that is not what the speaker wrote. Broken at the
// column, the drawn rows concatenate back to the original exactly — the cut has a
// receipt, which is the same promise the rest of this program makes about
// everything it drops. That it can split an identifier is the price and it is
// visible rather than silent, which is the trade this surface always takes.
// [TestAWrappedCodeLineIsExactlyReversible] is the pin.
//
// Fences are tracked by their own opening run, so a fence marker inside an
// indented block does not close a block it never opened, and an unclosed fence
// runs to the end of the text — which is what goldmark does with one too, so the
// two agree about where the code is.
func prewrapped(text string, width int) string {
	// Two for the margin glamour puts on a code block, two for the mark a
	// continuation opens with. Below that there is no room to break into and the
	// lines are handed over whole, which leaves glamour doing what it did before
	// rather than doing something worse.
	room := width - 4
	if room < 1 {
		return text
	}

	var out []string
	fence := ""
	for _, l := range strings.Split(text, "\n") {
		t := strings.TrimLeft(l, " ")
		switch {
		case fence == "" && (strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~")):
			fence = t[:3]
			out = append(out, l)
			continue
		case fence != "" && strings.HasPrefix(t, fence):
			fence = ""
			out = append(out, l)
			continue
		case fence == "":
			out = append(out, l)
			continue
		}

		if lipgloss.Width(l) <= room {
			out = append(out, l)
			continue
		}
		for i, part := range chunks(l, room) {
			if i == 0 {
				out = append(out, part)
				continue
			}
			out = append(out, "… "+part)
		}
	}
	return strings.Join(out, "\n")
}

// chunks cuts s into runs of at most n display columns, breaking at the column
// rather than at a word so that concatenating them returns s exactly.
//
// Rune by rune rather than by byte or by index, because a display column is
// neither: a CJK glyph is two columns and one rune, and cutting a code line at a
// byte offset would split a multi-byte character into two invalid ones. There is
// no combining-mark handling here and there does not need to be — the caller
// only reaches this for a line inside a fenced block, and a break that lands
// between a base and its mark is a rendering blemish rather than a lost
// character, which is the property this function is for.
func chunks(s string, n int) []string {
	if n < 1 {
		return []string{s}
	}
	rs := []rune(s)

	var out []string
	for len(rs) > 0 {
		cut, w := 0, 0
		for cut < len(rs) {
			rw := lipgloss.Width(string(rs[cut]))
			if w+rw > n && cut > 0 {
				break
			}
			w += rw
			cut++
		}

		// Never end a chunk on a space. Glamour strips the trailing whitespace off
		// every code line it emits, so a break that lands after one silently eats
		// it and the rows stop reassembling into what the speaker wrote — which is
		// the whole property this function exists for. Backing the break up leaves
		// the spaces to open the next chunk, where they are measured against its
		// budget like anything else.
		//
		// Guarded on leaving something behind, because a line of nothing but indent
		// would otherwise back up to zero and never advance.
		back := cut
		for back > 0 && rs[back-1] == ' ' {
			back--
		}
		if back > 0 {
			cut = back
		}

		out = append(out, string(rs[:cut]))
		rs = rs[cut:]
	}
	return out
}
