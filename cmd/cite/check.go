package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// verdict is what one citation came back as. The vocabulary is the point: a
// figure that disagrees with its source, a figure the record does not actually
// state, and a source nobody can find are three different findings, and a tool
// that collapses them into "failed" hands its reader a mood instead of a fact.
type verdict string

const (
	agrees      verdict = "agrees"
	disagrees   verdict = "disagrees"
	truncated   verdict = "truncated"
	unquoted    verdict = "unquoted"
	misquoted   verdict = "misquoted"
	ambiguous   verdict = "ambiguous-region"
	missing     verdict = "evidence-missing"
	moved       verdict = "evidence-moved"
	unsourced   verdict = "unsourced"
	overquoted  verdict = "quoted-twice"
	unreachable verdict = "not-in-source"
)

// why says what each verdict means, and doubles as the set of verdicts a block
// may declare — a block naming a verdict absent from this map is a parse error
// rather than a claim nobody can reach.
var why = map[verdict]string{
	agrees:      "the source says what the record says it says",
	disagrees:   "the source says a different number",
	truncated:   "the quotation is cut short of the source's own sentence",
	unquoted:    "the figure this block expects is not in the sentence it cites",
	misquoted:   "the sentence is not in that clause of the record, or is there twice",
	ambiguous:   "a region delimiter matched twice, or not at all",
	missing:     "the source is not in the cache",
	moved:       "the cached bytes do not hash to the address the manifest gives",
	unsourced:   "the block names a source no manifest block declares",
	overquoted:  "the quoted words occur more than once in the source",
	unreachable: "the quoted words are not in the source at all",
}

// result is one citation's answer, carrying enough to print the disagreement
// rather than only the fact of it. A checker that says "wrong" without saying
// what the source said sends its reader back to the PDF, which is the trip this
// exists to save.
type result struct {
	cite
	verdict verdict

	got  int    // what the source says, for a count
	saw  string // what the source says next, for a quotation
	note string // the one thing a reader needs that the numbers do not carry
}

// as reports whether the citation landed where its block said it would.
func (r result) as() bool {
	for _, d := range r.declared {
		if d == r.verdict {
			return true
		}
	}
	return false
}

// flatten collapses every run of whitespace to one space.
//
// The record's side is always flattened: docs/DECISIONS.md is hard-wrapped at
// 72 columns and where a sentence happens to break is not what any citation is
// about. The source's side is flattened only where the block says so, because
// there it changes the answer — see [cite].flat.
//
// What it does not do is rejoin a word the extractor hyphenated across a line
// break. Removing "-\n" would silently weld "agent-as-a-judge" into
// "agentas-a-judge", so a quotation spanning such a break simply is not found
// — a loud not-in-source that sends the author to look, rather than a quiet
// near-match. The failure runs in the safe direction and that is the whole
// argument for leaving it alone.
func flatten(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// check settles one citation.
func check(c cite, sources []source, cache string, log entries) result {
	r := result{cite: c}

	var src source
	found := false
	for _, s := range sources {
		if s.id == c.source {
			src, found = s, true
		}
	}
	if !found {
		r.verdict, r.note = unsourced, fmt.Sprintf("no source block declares %q", c.source)
		return r
	}

	// The record's side first. A block whose sentence is not where it says it is
	// has nothing to compare against, and saying so is more useful than counting
	// something and reporting it under a citation that does not resolve.
	clause, err := log.clause(c.entry)
	if err != nil {
		r.verdict, r.note = misquoted, err.Error()
		return r
	}
	switch n := strings.Count(clause, c.sentence); {
	case n == 0:
		r.verdict, r.note = misquoted, fmt.Sprintf("that sentence is not in %s", c.entry)
		return r
	case n > 1:
		r.verdict, r.note = misquoted, fmt.Sprintf("that sentence is in %s %d times, so the citation names no one place", c.entry, n)
		return r
	}
	if lack, ok := bound(c); !ok {
		r.verdict, r.note = unquoted, fmt.Sprintf("%s does not appear in the sentence this block cites", lack)
		return r
	}

	text, err := load(src, cache)
	if err != nil {
		if os.IsNotExist(err) {
			r.verdict = missing
			r.note = fmt.Sprintf("fetch %s and extract it; see the manifest's recipe", src.origin)
			return r
		}
		r.verdict, r.note = moved, err.Error()
		return r
	}

	// The normalisation is the block's, applied to the source, the delimiters and
	// the needle alike — three statements of one rule, and a count taken with
	// them out of step would be a number about nothing anybody wrote down.
	norm := func(s string) string { return s }
	if c.flat {
		norm = flatten
	}
	span, err := region(norm(text), norm(c.from), norm(c.to))
	if err != nil {
		r.verdict, r.note = ambiguous, err.Error()
		return r
	}

	hay, needle := span, norm(c.needle)
	if c.fold {
		hay, needle = strings.ToLower(hay), strings.ToLower(needle)
	}
	r.got = strings.Count(hay, needle)

	if c.kind == "quotation" {
		switch r.got {
		case 0:
			r.verdict = unreachable
			return r
		case 1:
		default:
			r.verdict = overquoted
			return r
		}
		r.saw = rest(span, needle)
		if r.saw != c.then {
			r.verdict = truncated
			return r
		}
		r.verdict = agrees
		return r
	}

	if r.got != c.expect {
		r.verdict = disagrees
		return r
	}
	r.verdict = agrees
	return r
}

// bound reports whether the sentence states the figure this block expects.
//
// This is the key property and it is worth naming plainly: without it the
// catalog is a second copy of a number, and the way a second copy fails is that
// somebody quiets a disagreement by editing the copy. Here that move fails —
// change the expectation and the sentence stops stating it — so the only thing
// that makes a block agree is a record that is right.
//
// A quotation binds on its own words. A count binds on its figures where the
// block gives them, which is how a table row works: the row asserts a count of
// one and the sentence says nothing about one, it carries the two percentages.
func bound(c cite) (string, bool) {
	if c.kind == "quotation" {
		if !strings.Contains(c.sentence, flatten(c.needle)) {
			return "the quoted words", false
		}
		return "", true
	}
	if len(c.figures) > 0 {
		for _, f := range c.figures {
			if !strings.Contains(c.sentence, f) {
				return strconv.Quote(f), false
			}
		}
		return "", true
	}
	for _, s := range spellings(c.expect) {
		if strings.Contains(c.sentence, s) {
			return "", true
		}
	}
	return strconv.Itoa(c.expect), false
}

// spellings is every way the record might write a figure.
//
// The table is here because entries spell small numbers out — D68(c) says a word
// appears "twice" — and a binding that only accepted digits would reject the
// prose this project actually writes. It stops at twelve, and a figure past it
// has to be written as digits, which is the right pressure on an entry: a
// stranger settles a number in one command, and a number he has to parse out of
// English is one command further away.
func spellings(n int) []string {
	out := []string{strconv.Itoa(n)}
	words := []string{"zero", "one", "two", "three", "four", "five", "six",
		"seven", "eight", "nine", "ten", "eleven", "twelve"}
	if n >= 0 && n < len(words) {
		out = append(out, words[n])
	}
	switch n {
	case 1:
		out = append(out, "once")
	case 2:
		out = append(out, "twice")
	case 3:
		out = append(out, "thrice")
	}
	return out
}

// region narrows the text to the span a citation is about.
//
// Each delimiter has to match exactly once. An anchor resolved to its first
// occurrence fails safe on the day it is written and turns into a false
// accusation against a healthy citation the first time the source is
// re-extracted — cmd/seam paid for that lesson in its own anchors and the fix
// there was the same refusal.
func region(text, from, to string) (string, error) {
	if from == "" {
		return text, nil
	}
	if n := strings.Count(text, from); n != 1 {
		return "", fmt.Errorf("%q occurs %d times in the source; a region delimiter has to occur once", from, n)
	}
	if n := strings.Count(text, to); n != 1 {
		return "", fmt.Errorf("%q occurs %d times in the source; a region delimiter has to occur once", to, n)
	}
	i := strings.Index(text, from)
	j := strings.Index(text, to)
	if j <= i {
		return "", fmt.Errorf("%q comes before %q, so the region runs backwards", to, from)
	}
	return text[i:j], nil
}

// sentenceEnd finds where a sentence stops: a full stop, question or
// exclamation mark with whitespace after it.
var sentenceEnd = regexp.MustCompile(`[.?!] `)

// rest is what the source says after a quotation, to the end of the sentence
// the quotation was cut out of.
//
// The span is the source's own sentence rather than a fixed number of
// characters, so nothing here has a tuned constant in it. A quotation that ends
// its sentence yields the empty string, which is what a block declares by
// leaving `then` empty.
//
// It is rough at an abbreviation — "et al. " ends a sentence as far as this is
// concerned — and that roughness is safe in the same direction as everything
// else here: it stops the continuation early, so an author sees a shorter span
// than the truth rather than a longer one, and the block still has to match it
// exactly.
func rest(region, needle string) string {
	i := strings.Index(strings.ToLower(region), strings.ToLower(needle))
	if i < 0 {
		return ""
	}
	tail := region[i+len(needle):]
	if m := sentenceEnd.FindStringIndex(tail); m != nil {
		return flatten(tail[:m[1]])
	}
	return flatten(tail)
}

// load reads a source out of the cache and re-derives its address. The bytes
// come back as the extractor left them; whether the lines are joined before
// anything is counted is the citation's own declaration and not this
// function's, because it is a dimension the answer moves in.
//
// The file is named for its address, so re-hashing it looks circular and is
// not: the cache is written by hand from a recipe, and the one thing a
// hand-written cache does is hold a file that was truncated, re-extracted by a
// different version, or filed under the wrong name. Every count below rests on
// these bytes being the bytes the catalog is about, and that is cheap to
// establish and expensive to assume.
func load(s source, cache string) (string, error) {
	b, err := os.ReadFile(filepath.Join(cache, s.sha+".txt"))
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	if got := hex.EncodeToString(sum[:]); got != s.sha {
		// The length is reported only when it moved. A file whose bytes changed
		// without its length changing is the interesting case and saying "135985
		// bytes, not 135985" beside it reads as a second, contradictory finding.
		size := ""
		if len(b) != s.bytes {
			size = fmt.Sprintf(", and is %d bytes rather than %d", len(b), s.bytes)
		}
		return "", fmt.Errorf("%s.txt hashes to %s%s", s.sha[:8], got[:8], size)
	}
	return string(b), nil
}
