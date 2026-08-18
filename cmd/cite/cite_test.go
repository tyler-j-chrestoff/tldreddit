package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A checker that cannot disagree is worse than the prose it checks, because it
// launders it. D27 named that failure and this project has built it three
// times, so the first thing this suite establishes is that every verdict in the
// vocabulary has a witness — and that adding a verdict without one fails here
// rather than sitting unreachable in a report nobody can falsify.
//
// The stubs that show this suite can fail, each one line in a scratch copy:
//
//   - in [check], return agrees before the count is compared: five rows redden,
//     measured — the line rule, the case rule, the two quotations the source
//     does not carry as claimed, and the truncated one. Not the eight this
//     comment first predicted; a red list is not knowable by reading.
//   - in [bound], return "", true always: exactly one row reddens, the laundered
//     one, which is the measurement that it is the anti-laundering check and not
//     a restatement of the arithmetic.
//   - delete a verdict from [why] and leave its row: green until the second
//     loop below existed, because a dropped verdict is simply not iterated. That
//     is this file's own D27 instance, found by running the stub rather than
//     reading it, and it is the reason the check runs in both directions.

// fixture is a source small enough to reason about and real enough to break: a
// phrase that occurs twice, one occurrence split across a line the way an
// extractor splits one, and a sentence that keeps going after a clause somebody
// might quote.
const fixture = "Alpha beta gamma. The rule is stated here (but qualified at once).\n" +
	"Alpha beta gamma is said again, and alpha beta gamma in lower case, and Alpha\n" +
	"beta gamma once more across a break.\n"

// What the fixture is worth, counted four ways, because the two rules a block
// must declare are exactly the two that move this number:
//
//	              case-sensitive   folded
//	as extracted        2            3
//	lines joined        3            4

func cache(t *testing.T) (string, source) {
	t.Helper()
	dir := t.TempDir()
	sum := sha256.Sum256([]byte(fixture))
	sha := hex.EncodeToString(sum[:])
	if err := os.WriteFile(filepath.Join(dir, sha+".txt"), []byte(fixture), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir, source{id: "fix", sha: sha, bytes: len(fixture), origin: "nowhere", pdf: sha, extract: "by hand"}
}

// log is a record shaped like the real one: two entries, lettered clauses, and
// one sentence that appears in both — which is the case that makes a citation
// name a clause rather than an entry.
var log = entries{
	"D01": "**(a) A count.** The phrase occurs twice in the fixture. " +
		"**(b) A quotation.** It says \"The rule is stated here\" and stops. " +
		"The words Alpha beta gamma are not a quotation.",
	"D02": "**(a) A correction.** D01(a) said the phrase occurs twice in the fixture. " +
		"**(b) A misquotation.** It says \"The rule is stated plainly\" and stops. " +
		"**(c) A sentence said twice.** Alpha beta gamma. Alpha beta gamma.",
}

func TestEveryVerdictHasAWitness(t *testing.T) {
	dir, src := cache(t)
	sources := []source{src}

	count := func(f func(*cite)) cite {
		c := cite{id: "x", kind: "count", entry: "D01(a)", source: "fix",
			sentence: "The phrase occurs twice in the fixture.",
			needle:   "Alpha beta gamma", expect: 2, expectSet: true,
			declared: []verdict{agrees}}
		f(&c)
		return c
	}
	quote := func(f func(*cite)) cite {
		c := cite{id: "x", kind: "quotation", entry: "D01(b)", source: "fix", flat: true,
			sentence: "It says \"The rule is stated here\" and stops.",
			needle:   "The rule is stated here", expect: 1, expectSet: true,
			then: "(but qualified at once).", thenSet: true,
			declared: []verdict{agrees}}
		f(&c)
		return c
	}

	rows := []struct {
		name  string
		c     cite
		cache string
		want  verdict
	}{
		{"the source says what the record says", count(func(*cite) {}), dir, agrees},
		{"a quotation the source carries", quote(func(*cite) {}), dir, agrees},

		// The fixture's third occurrence is split across a line, so the same
		// needle is 2 raw and 3 joined. One catalog, two answers, which is why
		// the block declares the rule instead of inheriting one.
		{"the line rule changes the answer", count(func(c *cite) { c.flat = true }), dir, disagrees},
		{"the case rule changes the answer", count(func(c *cite) { c.fold = true }), dir, disagrees},

		// The one that matters: a wrong figure cannot be quieted by editing the
		// block, because the sentence stops stating what the block expects.
		{"laundering the disagreement by editing the block", count(func(c *cite) { c.expect = 3 }), dir, unquoted},

		{"the sentence is not in the clause", count(func(c *cite) { c.sentence = "Something nobody wrote." }), dir, misquoted},
		{"the clause does not exist", count(func(c *cite) { c.entry = "D01(z)" }), dir, misquoted},
		{"the entry does not exist", count(func(c *cite) { c.entry = "D99(a)" }), dir, misquoted},
		{"the sentence is in the clause twice", count(func(c *cite) {
			c.entry, c.sentence = "D02(c)", "Alpha beta gamma."
		}), dir, misquoted},

		{"a region delimiter is not unique", count(func(c *cite) { c.from, c.to = "Alpha", "break" }), dir, ambiguous},
		{"a region runs backwards", count(func(c *cite) { c.from, c.to = "across a break", "The rule" }), dir, ambiguous},

		{"the quoted words are not in the source", quote(func(c *cite) {
			c.entry = "D02(b)"
			c.sentence = "It says \"The rule is stated plainly\" and stops."
			c.needle = "The rule is stated plainly"
		}), dir, unreachable},
		{"the quoted words are in the source twice", quote(func(c *cite) {
			c.needle = "Alpha beta gamma"
			c.sentence = "The words Alpha beta gamma are not a quotation."
		}), dir, overquoted},
		{"the quotation is cut short of the sentence", quote(func(c *cite) { c.then = "" }), dir, truncated},

		{"the source is not declared", count(func(c *cite) { c.source = "nobody" }), dir, unsourced},
		{"the source is not on this machine", count(func(*cite) {}), t.TempDir(), missing},
	}

	seen := map[verdict]bool{}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			got := check(row.c, sources, row.cache, log)
			if got.verdict != row.want {
				t.Fatalf("verdict %q, want %q (note %q, count %d)", got.verdict, row.want, got.note, got.got)
			}
		})
		seen[row.want] = true
	}

	// evidence-moved needs bytes that do not hash to their name, which no row
	// above can build out of a shared fixture without spoiling it for the rest.
	t.Run("the cached bytes are not the bytes the manifest names", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, src.sha+".txt"), []byte(fixture+"."), 0o600); err != nil {
			t.Fatal(err)
		}
		got := check(count(func(*cite) {}), sources, dir, log)
		if got.verdict != moved {
			t.Fatalf("verdict %q, want %q", got.verdict, moved)
		}
	})
	seen[moved] = true

	// Both directions, and the second was added after the first was measured and
	// found insufficient. A verdict in the vocabulary with no row here is a
	// promise nobody can falsify; a row here reaching a verdict the vocabulary
	// has dropped is the same defect running the other way, and deleting an
	// entry from [why] left this test green until this loop existed.
	for v := range why {
		if !seen[v] {
			t.Errorf("no row in this table reaches %q, so nothing here says it can happen", v)
		}
	}
	for v := range seen {
		if why[v] == "" {
			t.Errorf("a row reaches %q, which the vocabulary no longer explains", v)
		}
	}
}

// The count is over non-overlapping occurrences, which is what Python's
// str.count does and therefore what every figure already in the record was
// measured with. Stated as a test rather than a comment because the two
// libraries could have differed and the record would have been re-derived
// against the wrong one.
func TestOverlappingOccurrencesAreCountedOnce(t *testing.T) {
	dir := t.TempDir()
	const text = "aaaa"
	sum := sha256.Sum256([]byte(text))
	sha := hex.EncodeToString(sum[:])
	if err := os.WriteFile(filepath.Join(dir, sha+".txt"), []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	src := source{id: "fix", sha: sha, bytes: len(text)}
	c := cite{id: "x", kind: "count", entry: "D01(a)", source: "fix",
		sentence: "The phrase occurs twice in the fixture.",
		needle:   "aa", expect: 2, expectSet: true, declared: []verdict{agrees}}

	if got := check(c, []source{src}, dir, log); got.got != 2 {
		t.Fatalf("counted %d occurrences of %q in %q, want 2", got.got, c.needle, text)
	}
}

// rest is what makes a truncated quotation visible, so what it hands back is
// worth pinning: the remainder of the source's own sentence, and nothing when
// the quotation ends one.
func TestTheContinuationRunsToTheEndOfTheSourcesSentence(t *testing.T) {
	const text = "He said the thing (but qualified it). Then he said another thing."
	for _, row := range []struct {
		quote string
		want  string
	}{
		{"He said the thing", "(but qualified it)."},
		{"He said the thing (but qualified it).", "Then he said another thing."},
		{"Then he said another thing.", ""},
	} {
		if got := rest(text, row.quote); got != row.want {
			t.Errorf("after %q the source says %q, want %q", row.quote, got, row.want)
		}
	}
}

// A block that does not say how to read the text is refused at parse time
// rather than answered with one of the two numbers it could mean. The same goes
// for a key that belongs to another kind: a quotation carrying `expect` is
// somebody assuming this file works a way it does not.
func TestABlockThatDoesNotSayWhatItMeansIsRefused(t *testing.T) {
	const good = "kind: count\nid: x\nentry: D01(a)\nsentence: s\nsource: fix\nneedle: n\nfold: false\nflatten: false\nexpect: 1\n"
	for _, row := range []struct {
		name  string
		block string
		want  string
	}{
		{"a count with no case rule", strings.Replace(good, "fold: false\n", "", 1), "fold is required"},
		{"a count with no line rule", strings.Replace(good, "flatten: false\n", "", 1), "flatten is required"},
		{"a count with no expectation", strings.Replace(good, "expect: 1\n", "", 1), "expect is required"},
		{"a quotation with no continuation", "kind: quotation\nid: x\nentry: D01(b)\nsentence: s\nsource: fix\nneedle: n\n", "then is required"},
		{"a quotation carrying a count's keys", "kind: quotation\nid: x\nentry: D01(b)\nsentence: s\nsource: fix\nneedle: n\nthen: t\nfold: true\n", "do not apply"},
		{"half a region", good + "from: A\n", "one alone is not a region"},
		{"a key from no kind at all", good + "colour: red\n", "not a key"},
		{"a verdict this tool cannot report", good + "verdict: green\n", "not a verdict"},
		{"no kind", strings.Replace(good, "kind: count\n", "", 1), "kind is required"},
	} {
		t.Run(row.name, func(t *testing.T) {
			_, err := parseCatalog(strings.NewReader("```cite\n" + row.block + "```\n"))
			if err == nil {
				t.Fatalf("parsed, and it should not have")
			}
			if !strings.Contains(err.Error(), row.want) {
				t.Fatalf("error %q, want one containing %q", err, row.want)
			}
		})
	}
}

// A clause is the unit a citation names, and this is why: the same sentence
// lives in D01(a) and in D02(a), where the second is the correction. A tool
// that searched the entry, or the log, would resolve half these citations to
// the place that says the opposite.
func TestASentenceIsFoundInItsOwnClause(t *testing.T) {
	const shared = "the phrase occurs twice in the fixture"
	for _, at := range []string{"D01(a)", "D02(a)"} {
		body, err := log.clause(at)
		if err != nil {
			t.Fatalf("%s: %v", at, err)
		}
		if n := strings.Count(strings.ToLower(body), shared); n != 1 {
			t.Errorf("%s carries that sentence %d times, want 1", at, n)
		}
	}
	if body, _ := log.clause("D01"); strings.Count(strings.ToLower(body), shared) != 1 {
		t.Errorf("D01 whole should carry it once; both clauses of it would be two citations for one sentence")
	}
}

// The catalog this repository actually ships has to parse, and every citation
// in it has to name a source the manifest declares. That is what -sources
// checks on a machine with no cache, and it is the one part of this tool the
// commit gate can run anywhere.
func TestTheShippedCatalogParsesAndEverySourceResolves(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	cat, err := readCatalog(filepath.Join(root, "docs", "CITATIONS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cat.cites) == 0 || len(cat.sources) == 0 {
		t.Fatalf("%d citations over %d sources; an empty catalog checks nothing", len(cat.cites), len(cat.sources))
	}
	for _, c := range cat.cites {
		if _, ok := cat.source(c.source); !ok {
			t.Errorf("%s names source %q, which no block declares", c.id, c.source)
		}
	}
}

// Every citation the shipped catalog makes has to resolve into the record, and
// the figure it expects has to be in the sentence it quotes. This half needs no
// cache, so it runs on any machine — which matters, because the half that reads
// the sources does not.
func TestEveryShippedCitationResolvesIntoTheRecord(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	cat, err := readCatalog(filepath.Join(root, "docs", "CITATIONS.md"))
	if err != nil {
		t.Fatal(err)
	}
	rec, err := readEntries(filepath.Join(root, "docs", "DECISIONS.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cat.cites {
		clause, err := rec.clause(c.entry)
		if err != nil {
			t.Errorf("%s: %v", c.id, err)
			continue
		}
		if n := strings.Count(clause, c.sentence); n != 1 {
			t.Errorf("%s: its sentence is in %s %d times, want 1", c.id, c.entry, n)
		}
		if lack, ok := bound(c); !ok {
			t.Errorf("%s: %s is not in the sentence it cites", c.id, lack)
		}
	}
}
