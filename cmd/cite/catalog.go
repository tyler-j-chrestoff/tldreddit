package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// source is an artifact a citation is about, named by the address of its bytes.
//
// The address is the identity and the filename is not: the cache stores each
// source at <sha256>.txt, so two different extractions of one paper are two
// sources and cannot be confused for each other. That is the only thing content
// addressing buys here and it is enough — a claim about "rqgm.txt" is
// meaningless, and a claim about d1e4473c… is not.
type source struct {
	id string

	// sha is the address of the extracted text, lowercase hex, and bytes is its
	// length. The length is redundant against the address and is carried anyway,
	// because it is the one field a person can check by eye against an ls.
	sha   string
	bytes int

	// origin is where the PDF came from and pdf is its own address. Together
	// with extract they are the recipe a stranger runs to get the same text; the
	// recipe is prose and nothing checks it, which is exactly why both addresses
	// are here to check it against.
	origin  string
	pdf     string
	extract string

	line int
}

// cite is one claim: a sentence in the record, and the computation that settles
// it against a source.
type cite struct {
	id    string
	kind  string // count or quotation
	title string
	line  int

	// entry is the clause the sentence comes from, spelled Dnn(x). The clause
	// and not the entry, because entries here run to thousands of words and a
	// sentence that occurs once in a clause occurs twice in the log — D67(i)'s
	// wrong figure is quoted verbatim by D68(a), which is the correction.
	entry string

	// sentence is the entry's own words, whitespace-collapsed. It has to occur
	// exactly once in the clause, and the expected figure has to occur in it.
	// That pairing is what stops this file from being a second copy of a number
	// that drifts from the first.
	sentence string

	// source names a block above; from and to delimit a region inside it, each
	// of which must match exactly once. No region means the whole text, which is
	// a different claim and is written as one.
	source   string
	from, to string

	// needle is the string being counted, or for a quotation the words the entry
	// attributes to the source.
	needle string

	// fold ignores case. Required rather than defaulted: "erasure" is 27 in RQGM
	// unfolded and 28 folded, so a count with no case rule is two claims wearing
	// one number, and the record already carries a bullet that does not say which
	// of them it means.
	fold    bool
	foldSet bool

	// flat joins the source's lines before counting. Required alongside fold and
	// for the same reason, learned the same way: "selective erasure" is 9 times
	// in RQGM as the extractor left it and 10 times with the lines joined,
	// because one occurrence is broken across a column break. Those are two
	// questions and the record's own bullet asks neither of them out loud.
	//
	// A quotation is always joined and does not take this key — prose in a
	// two-column PDF crosses a line break every ninety-odd characters, so a
	// quotation matched against the raw extraction would never be found and the
	// key would have one usable value.
	flat    bool
	flatSet bool

	// expect is the count the sentence states. A table row is expect 1 over the
	// row's own text rather than a kind of its own.
	expect    int
	expectSet bool

	// figures overrides what has to appear in the sentence. A row asserts a
	// count of one and the sentence says nothing about one — it carries the two
	// percentages on the row, and those are the figures worth binding.
	figures []string

	// then is what the source says after a quotation, to the end of the sentence
	// the quotation was cut out of. Required on every quotation and empty only
	// when the quotation ends that sentence itself.
	//
	// This is the only key here that is about meaning rather than arithmetic,
	// and it is deliberately weaker than it looks: it forces an author who cuts
	// a sentence short to write down the rest of it, which catches a truncation
	// and never a misreading. docs/CITATIONS.md argues why that is still worth
	// the key.
	then    string
	thenSet bool

	// declared is what the block says it will come back as, and the gate is an
	// equality against it in either direction. cmd/seam's mechanism, for
	// cmd/seam's reason: a claim that quietly starts agreeing trips as loudly as
	// one that stops. It is what lets D67's four wrong figures sit in the catalog
	// as wrong figures in a log that may not be edited.
	declared []verdict
}

type catalog struct {
	sources []source
	cites   []cite
}

func (c catalog) one(id string) (catalog, error) {
	for _, x := range c.cites {
		if x.id == id {
			return catalog{sources: c.sources, cites: []cite{x}}, nil
		}
	}
	return c, fmt.Errorf("no citation with id %q; -list prints the catalog", id)
}

func (c catalog) source(id string) (source, bool) {
	for _, s := range c.sources {
		if s.id == id {
			return s, true
		}
	}
	return source{}, false
}

func readCatalog(path string) (catalog, error) {
	f, err := os.Open(path)
	if err != nil {
		return catalog{}, err
	}
	defer f.Close()
	return parseCatalog(f)
}

// parseCatalog reads the blocks out of docs/CITATIONS.md.
//
// The file is the catalog. Everything the parser does not understand is an
// error rather than a default — an unknown key, a missing required one, a kind
// it has never heard of — because a default the catalog does not spell is a
// claim nobody wrote down.
func parseCatalog(r io.Reader) (catalog, error) {
	var (
		out   catalog
		title string
		in    bool
		block []string
		start int
	)

	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for n := 1; sc.Scan(); n++ {
		line := sc.Text()

		if !in {
			if h, ok := strings.CutPrefix(line, "## "); ok {
				title = strings.TrimSpace(h)
			}
			if strings.TrimSpace(line) == "```cite" {
				in, block, start = true, nil, n
			}
			continue
		}
		if strings.TrimSpace(line) == "```" {
			if err := parseBlock(&out, block, title, start); err != nil {
				return out, fmt.Errorf("docs/CITATIONS.md:%d: %w", start, err)
			}
			in = false
			continue
		}
		block = append(block, line)
	}
	if err := sc.Err(); err != nil {
		return out, err
	}
	if in {
		return out, fmt.Errorf("docs/CITATIONS.md:%d: cite block is never closed", start)
	}
	return out, ids(out)
}

func ids(c catalog) error {
	seen := map[string]int{}
	for _, s := range c.sources {
		if at, dup := seen[s.id]; dup {
			return fmt.Errorf("docs/CITATIONS.md:%d: source id %q was already used at line %d", s.line, s.id, at)
		}
		seen[s.id] = s.line
	}
	seen = map[string]int{}
	for _, x := range c.cites {
		if at, dup := seen[x.id]; dup {
			return fmt.Errorf("docs/CITATIONS.md:%d: id %q was already used at line %d", x.line, x.id, at)
		}
		seen[x.id] = x.line
	}
	return nil
}

func parseBlock(out *catalog, lines []string, title string, start int) error {
	kv, err := fields(lines)
	if err != nil {
		return err
	}
	switch kv["kind"] {
	case "source":
		s, err := parseSource(kv, start)
		if err != nil {
			return err
		}
		out.sources = append(out.sources, s)
	case "count", "quotation":
		c, err := parseCite(kv, title, start)
		if err != nil {
			return err
		}
		out.cites = append(out.cites, c)
	case "":
		return fmt.Errorf("kind is required (source, count or quotation)")
	default:
		return fmt.Errorf("%q is not a kind this tool knows (source, count, quotation)", kv["kind"])
	}
	return nil
}

// fields splits the block into key/value pairs.
//
// Only the leading space after the colon is eaten. Every value here is either
// an exact substring of somebody's paper or an exact substring of this
// project's own record, and trailing space is part of a substring as often as
// it is a typo.
func fields(lines []string) (map[string]string, error) {
	kv := map[string]string{}
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		key, raw, ok := strings.Cut(line, ":")
		if !ok {
			return nil, fmt.Errorf("%q is not key: value", line)
		}
		key = strings.TrimSpace(key)
		if _, dup := kv[key]; dup {
			return nil, fmt.Errorf("%s is given twice", key)
		}
		kv[key] = strings.TrimPrefix(raw, " ")
	}
	return kv, nil
}

func parseSource(kv map[string]string, start int) (source, error) {
	s := source{
		id:      strings.TrimSpace(kv["id"]),
		sha:     strings.TrimSpace(kv["sha256"]),
		origin:  strings.TrimSpace(kv["origin"]),
		pdf:     strings.TrimSpace(kv["pdf"]),
		extract: strings.TrimSpace(kv["extract"]),
		line:    start,
	}
	n, err := strconv.Atoi(strings.TrimSpace(kv["bytes"]))
	if err != nil {
		return s, fmt.Errorf("bytes: %q is not a length", kv["bytes"])
	}
	s.bytes = n

	if err := known(kv, "kind", "id", "sha256", "bytes", "origin", "pdf", "extract"); err != nil {
		return s, err
	}
	return s, required(map[string]bool{
		"id":      s.id != "",
		"sha256":  len(s.sha) == 64,
		"origin":  s.origin != "",
		"pdf":     len(s.pdf) == 64,
		"extract": s.extract != "",
	})
}

func parseCite(kv map[string]string, title string, start int) (cite, error) {
	c := cite{
		id:       strings.TrimSpace(kv["id"]),
		kind:     kv["kind"],
		title:    title,
		line:     start,
		entry:    strings.TrimSpace(kv["entry"]),
		sentence: flatten(kv["sentence"]),
		source:   strings.TrimSpace(kv["source"]),
		from:     kv["from"],
		to:       kv["to"],
		needle:   kv["needle"],
		declared: []verdict{agrees},
	}

	if v, ok := kv["fold"]; ok {
		b, err := strconv.ParseBool(strings.TrimSpace(v))
		if err != nil {
			return c, fmt.Errorf("fold: %q is not true or false", v)
		}
		c.fold, c.foldSet = b, true
	}
	if v, ok := kv["flatten"]; ok {
		b, err := strconv.ParseBool(strings.TrimSpace(v))
		if err != nil {
			return c, fmt.Errorf("flatten: %q is not true or false", v)
		}
		c.flat, c.flatSet = b, true
	}
	if v, ok := kv["expect"]; ok {
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil || n < 0 {
			return c, fmt.Errorf("expect: %q is not a count", v)
		}
		c.expect, c.expectSet = n, true
	}
	if v, ok := kv["figures"]; ok {
		c.figures = list(v)
	}
	if v, ok := kv["then"]; ok {
		c.then, c.thenSet = flatten(v), true
	}
	if v, ok := kv["verdict"]; ok {
		c.declared = nil
		for f := range strings.SplitSeq(v, "|") {
			d := verdict(strings.TrimSpace(f))
			if why[d] == "" {
				return c, fmt.Errorf("verdict: %q is not a verdict this tool reports", f)
			}
			c.declared = append(c.declared, d)
		}
	}

	if err := known(kv, "kind", "id", "entry", "sentence", "source", "from", "to",
		"needle", "fold", "flatten", "expect", "figures", "then", "verdict"); err != nil {
		return c, err
	}
	if err := required(map[string]bool{
		"id":       c.id != "",
		"entry":    c.entry != "",
		"sentence": c.sentence != "",
		"source":   c.source != "",
		"needle":   c.needle != "",
	}); err != nil {
		return c, err
	}

	switch c.kind {
	case "count":
		if !c.foldSet {
			return c, fmt.Errorf("fold is required on a count; a count with no case rule is two claims")
		}
		if !c.flatSet {
			return c, fmt.Errorf("flatten is required on a count; a count with no line rule is two claims")
		}
		if !c.expectSet {
			return c, fmt.Errorf("expect is required on a count")
		}
		if c.thenSet {
			return c, fmt.Errorf("then belongs to a quotation, not a count")
		}
	case "quotation":
		if !c.thenSet {
			return c, fmt.Errorf("then is required on a quotation, and is empty only when the quotation ends the source's sentence")
		}
		if c.expectSet || c.foldSet || c.flatSet {
			return c, fmt.Errorf("a quotation is verbatim, occurs once and is always joined; expect, fold and flatten do not apply")
		}
		c.expect, c.expectSet, c.flat = 1, true, true
	}

	// A region is two anchors or none. One alone reads as a region and is a
	// count over everything after — or before — whichever anchor was written,
	// which is not what anybody meant by writing it.
	if (c.from == "") != (c.to == "") {
		return c, fmt.Errorf("from and to come together; one alone is not a region")
	}
	return c, nil
}

func known(kv map[string]string, keys ...string) error {
	ok := map[string]bool{}
	for _, k := range keys {
		ok[k] = true
	}
	for k := range kv {
		if !ok[k] {
			return fmt.Errorf("%q is not a key this kind of block takes", k)
		}
	}
	return nil
}

func required(have map[string]bool) error {
	for key, ok := range have {
		if !ok {
			return fmt.Errorf("%s is required", key)
		}
	}
	return nil
}

func list(s string) []string {
	var out []string
	for f := range strings.SplitSeq(s, ",") {
		if f = strings.TrimSpace(f); f != "" {
			out = append(out, f)
		}
	}
	return out
}
