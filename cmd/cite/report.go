package main

import (
	"fmt"
	"io"
	"strings"
)

// The report is an inventory and not a score. There is no percentage here for
// the reason cmd/seam gives for having none: a ratio goes up when a citation is
// deleted, and the thing being measured is whether the record's numbers survive
// contact with their sources, which a fraction cannot say.

func printCatalog(w io.Writer, c catalog) {
	fmt.Fprintf(w, "sources %d, citations %d\n\n", len(c.sources), len(c.cites))
	for _, s := range c.sources {
		fmt.Fprintf(w, "  %-12s %s  %d bytes  %s\n", s.id, s.sha[:16], s.bytes, s.origin)
	}
	fmt.Fprintln(w)
	for _, x := range c.cites {
		fmt.Fprintf(w, "  %-26s %-10s %-8s %s\n", x.id, x.kind, x.entry, x.title)
	}
}

// printManifest is what a machine with no cache can still do, and it is careful
// to say that this is not the check. Every cited source has a manifest block;
// every manifest block has an address, an origin and a recipe. None of that
// counts anything.
func printManifest(w io.Writer, c catalog) int {
	bad := 0
	for _, s := range c.sources {
		fmt.Fprintf(w, "  %-12s %s\n", s.id, s.sha)
		fmt.Fprintf(w, "  %-12s pdf %s\n", "", s.pdf)
		fmt.Fprintf(w, "  %-12s from %s\n", "", s.origin)
		fmt.Fprintf(w, "  %-12s via %s\n\n", "", s.extract)
	}
	for _, x := range c.cites {
		if _, ok := c.source(x.source); !ok {
			fmt.Fprintf(w, "  %-26s names source %q, which no block declares\n", x.id, x.source)
			bad++
		}
	}
	fmt.Fprintf(w, "%d sources, %d citations, %d unsourced\n", len(c.sources), len(c.cites), bad)
	fmt.Fprintln(w, "\nThis checked the manifest and read no source. It is not the check;")
	fmt.Fprintln(w, "run without -sources on a machine that has the cache.")
	return bad
}

func printReport(w io.Writer, cache string, rs []result) int {
	fmt.Fprintf(w, "cache %s\n\n", cache)

	off := 0
	for _, r := range rs {
		mark := " "
		if !r.as() {
			mark, off = "*", off+1
		}
		fmt.Fprintf(w, "%s %-26s %-16s %s\n", mark, r.id, r.verdict, r.entry)

		switch r.verdict {
		case disagrees:
			fmt.Fprintf(w, "    the record says %d, %s says %d, counting %s%s\n",
				r.expect, r.source, r.got, quoted(r.needle), folded(r.fold))
			fmt.Fprintf(w, "    %s\n", cut(r.sentence))
		case truncated:
			fmt.Fprintf(w, "    the quotation stops here, and %s goes on:\n", r.source)
			fmt.Fprintf(w, "      … %s\n", said(r.saw))
			fmt.Fprintf(w, "    the block declared: %s\n", said(r.then))
		case agrees:
			if r.kind == "quotation" {
				fmt.Fprintf(w, "    verbatim in %s, and the sentence continues: %s\n", r.source, said(r.saw))
			} else {
				fmt.Fprintf(w, "    %d in %s, counting %s%s\n", r.got, r.source, quoted(r.needle), folded(r.fold))
			}
		default:
			// A verdict with nothing said under it reads as a tool that stopped
			// halfway. Where there is no particular fact to add, the vocabulary's
			// own sentence is the fact.
			if r.note != "" {
				fmt.Fprintf(w, "    %s\n", r.note)
			} else {
				fmt.Fprintf(w, "    %s: %s\n", quoted(r.needle), why[r.verdict])
			}
		}
		if !r.as() {
			fmt.Fprintf(w, "    ** declared %s; docs/CITATIONS.md:%d\n", strings.Join(declared(r.declared), " or "), r.line)
		}
		fmt.Fprintln(w)
	}

	tally := map[verdict]int{}
	for _, r := range rs {
		tally[r.verdict]++
	}
	fmt.Fprintf(w, "%d citations", len(rs))
	for _, v := range []verdict{agrees, disagrees, truncated, unquoted, misquoted,
		ambiguous, missing, moved, unsourced, overquoted, unreachable} {
		if tally[v] > 0 {
			fmt.Fprintf(w, ", %d %s", tally[v], v)
		}
	}
	fmt.Fprintln(w)

	if off == 0 {
		fmt.Fprintln(w, "every citation is where its own block says it is")
	} else {
		fmt.Fprintf(w, "%d not where its block says it is, marked * above\n", off)
	}
	return off
}

func declared(ds []verdict) []string {
	out := make([]string, len(ds))
	for i, d := range ds {
		out[i] = string(d)
	}
	return out
}

func quoted(s string) string { return `"` + flatten(s) + `"` }

func folded(b bool) string {
	if b {
		return ", case folded"
	}
	return ", case-sensitive"
}

func said(s string) string {
	if s == "" {
		return "(nothing; the sentence ends there)"
	}
	return quoted(s)
}

// cut keeps a cited sentence to one line. The sentence is here so a reader can
// see the record's own words beside the source's answer; the whole of a long
// one would bury the answer it is next to.
func cut(s string) string {
	const n = 96
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
