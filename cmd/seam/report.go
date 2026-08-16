package main

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"slices"
	"sort"
	"strings"
)

// named is a list of checks as a reader cites them. The package each one lives in
// is what the tool keys on and is not printed: a name that needed its package to
// be unambiguous would have been refused at the block.
func named(checks []check) []string {
	out := make([]string, 0, len(checks))
	for _, at := range checks {
		out = append(out, at.name)
	}
	return out
}

// The order verdicts are inventoried in: what the reader has to act on first.
// Proven is last because it is the boring one, and a report that leads with its
// good news is a report people stop reading at the top.
var order = []verdict{
	vacuous, overRed, crashProof, killed, neverRan, unattributable,
	staleAnchor, ambiguousAnchor, staleCitation, brokenBuild, proven,
}

// why says what each verdict means for the claim it lands on, in the terms the
// reader has to act in. It is printed every run rather than kept in a manual,
// because the whole output is six words per claim and six words are not enough
// to carry a definition.
var why = map[verdict]string{
	proven:          "the break was made, the tree built, and every cited check failed by its own assertion",
	vacuous:         "the break was made and a cited check went on passing — the claim rests on the check existing, not on it biting",
	overRed:         "the claim said these checks alone would notice, and something else noticed too",
	crashProof:      "a cited check reddened without asserting: the process died, so what it caught may not be what it claims to catch",
	staleAnchor:     "the source no longer says what the catalog anchors on, so nothing was checked",
	staleCitation:   "the catalog cites a check this suite does not have, or one that skipped rather than ran",
	brokenBuild:     "the mutated tree does not build or does not vet, so every red under it proves nothing",
	unattributable:  "a cited check reddened on the unmutated tree too, so nothing here can be attributed to the mutation",
	killed:          "a cited check started and never finished: the process died under it, so its own assertion never ran",
	neverRan:        "a cited check never started, so nothing at all was observed of it",
	ambiguousAnchor: "the anchor matches in more than one place and the block does not say which",
}

// printIdent writes the tree every verdict below it is a verdict about — and,
// where that sentence is not true of every verdict, says so here rather than
// leaving the address to speak for a run it does not cover.
//
// elsewhere is the rows taken against another tree, from [moved]; -list passes
// nil, having run nothing. It is a parameter rather than a lookup so that no
// caller can print this block without having answered the question.
//
// Printed by the report and by -list alike, from one definition, because the
// point of the second is to be comparable with the first by eye.
func printIdent(id ident, elsewhere []result) {
	fmt.Printf("\n  tree: %s\n", id.tree)
	fmt.Println("    a sha-256 over the copy the claims are run in — every path, every")
	fmt.Println("    file's bytes, the execute bit, every symlink's target. Not .git, not")
	fmt.Println("    mtimes, not owners, not permissions beyond execute.")

	// Attached to the address and not filed further down beside the verdicts. The
	// reader this warns is the one who lifts a single address out of this block
	// and quotes it over a whole run, and that reader never gets as far as the
	// verdicts.
	if len(elsewhere) > 0 {
		byTree := map[string][]string{}
		for _, r := range elsewhere {
			byTree[r.against] = append(byTree[r.against], r.claim.id)
		}
		fmt.Println("  ** THE TREE MOVED WHILE THIS RAN. The address above is the baseline's")
		fmt.Println("     and covers every claim but these, which were taken against another")
		fmt.Println("     tree, because the repository was written to while the catalog ran:")
		for _, addr := range slices.Sorted(maps.Keys(byTree)) {
			fmt.Printf("       %s\n", addr)
			fmt.Printf("         %s\n", strings.Join(byTree[addr], " "))
		}
		fmt.Println("     Those verdicts stand against the tree named beside them and against")
		fmt.Println("     no other. Run again on a still tree for a receipt with one subject.")
	}

	switch {
	case id.head == "":
		fmt.Println("  no git anchor — this is not a repository, or git could not be asked.")
		fmt.Println("    The address above stands on its own.")
	case id.dirty:
		fmt.Printf("  at %s, dirty — git's name for where this came from. Orientation and\n", id.head)
		fmt.Println("    not identity: uncommitted work means many trees share that sha.")
	default:
		fmt.Printf("  at %s, clean — git's name for where this came from. Orientation and\n", id.head)
		fmt.Println("    not identity; the address above is the identity.")
	}
}

func printCatalog(id ident, claims []claim) {
	printIdent(id, nil)
	fmt.Println()

	for _, c := range claims {
		fmt.Printf("%-34s %s\n", c.id, c.title)
		fmt.Printf("%-34s %s:%d  %s\n", "", "docs/CLAIMS.md", c.line, c.file)
		fmt.Printf("%-34s red: %s\n", "", strings.Join(c.red, " "))

		var flags []string
		if c.sole {
			flags = append(flags, "sole")
		}
		if len(c.among) > 0 {
			flags = append(flags, "among "+strings.Join(c.among, " "))
		}
		if !slices.Equal(c.declared, []verdict{proven}) {
			flags = append(flags, "declared "+declaration(c.declared))
		}
		if c.race {
			flags = append(flags, "-race")
		}
		if c.isolate {
			flags = append(flags, "isolated")
		}
		if c.runs > 1 {
			flags = append(flags, fmt.Sprintf("%d runs", c.runs))
		}
		if len(flags) > 0 {
			fmt.Printf("%-34s %s\n", "", strings.Join(flags, ", "))
		}
		fmt.Println()
	}
	fmt.Printf("%d claims\n", len(claims))
}

// printReport writes the inventory.
//
// An inventory and never a score. A percentage of claims proven would be a
// number that goes up when a claim is deleted, and the claims most worth having
// are the ones most likely to come back vacuous — so the instrument would push
// exactly the wrong way. What a reader needs is which claim, in which state, and
// what to go and look at.
func printReport(root string, id ident, took baseline, results []result) {
	fmt.Printf("\nseam · %s\n", root)
	fmt.Println("  Every claim in docs/CLAIMS.md, re-broken in a copy of the working tree.")
	fmt.Println("  This says whether each cited check bites. It never says the code is right.")

	// Which tree, before any verdict about it. A verdict here answers "does this
	// check bite"; it cannot be read at all without the answer to "of what" — and
	// where the answer is more than one tree, that is the first thing said.
	printIdent(id, moved(results))
	fmt.Println("    `go run ./cmd/seam -list` prints that address in a second, running")
	fmt.Println("    nothing, which is how to tell whether this report is about your tree.")

	// The limit, printed where the copying out happens. Binding a verdict to a
	// tree makes staleness detectable; it does not make it impossible, and a
	// figure lifted out of here without the address beside it is exactly as
	// unbound as it was before any of this existed.
	fmt.Println("    A verdict quoted elsewhere without that address beside it is unbound")
	fmt.Println("    again. Nothing here can prevent that; it stays a discipline.")

	// The control, printed rather than assumed. Every verdict below is a
	// difference between two runs, and a reader who cannot see that the green one
	// was taken is being asked to trust the half that does the work.
	fmt.Printf("\n  baseline: %d checks green, %d skipped%s, %s\n",
		took.checks, took.skipped, raceLabel(took.race), took.at.Format("2006-01-02 15:04:05 MST"))
	fmt.Println("  every claim below was sampled the same number of times unmutated as mutated")

	// The gate's own sentence, at the top, in the words the exit status means.
	// A claim may declare a verdict other than proven where it honestly cannot do
	// better, or a set of them where its honest answer is nondeterministic; what
	// fails here is a claim that landed outside what it declared, in either
	// direction.
	off := adrift(results)
	switch {
	case len(off) > 0:
		fmt.Printf("\n  NOT WHERE THEY SAY THEY ARE: %s\n", strings.Join(off, " "))
	case len(moved(results)) > 0:
		// True, and not an all-clear. This is the one line a reader carries away,
		// and under drift it is true of more than one tree — printing only its
		// first half is precisely how a partial receipt gets quoted as a clean
		// one, which the block above cannot prevent on its own.
		fmt.Println("\n  every claim is where it says it is — but not all of one tree, see above")
	default:
		fmt.Println("\n  every claim is where it says it is")
	}

	byVerdict := map[verdict][]result{}
	for _, r := range results {
		byVerdict[r.verdict] = append(byVerdict[r.verdict], r)
	}

	for _, v := range order {
		rs := byVerdict[v]
		if len(rs) == 0 {
			continue
		}
		fmt.Printf("\n── %s — %s\n", v, why[v])
		for _, r := range rs {
			fmt.Printf("\n  %s\n", r.claim.id)
			fmt.Printf("    %s · %s:%d\n", r.claim.title, "docs/CLAIMS.md", r.claim.line)
			// Repeated here as well as in the identity block, because a row is
			// what gets read on its own and quoted on its own.
			if r.against != "" {
				fmt.Println("    ** the tree moved mid-run; this verdict is against")
				fmt.Printf("       %s\n", r.against)
				fmt.Println("       and not the address at the top of this report")
			}
			switch {
			case !r.asDeclared():
				fmt.Printf("    ** declared %s, and it is not — this is what the gate fails on\n",
					declaration(r.claim.declared))
			case len(r.claim.declared) > 1:
				fmt.Printf("    declared %s in the block, and it is one of them\n",
					declaration(r.claim.declared))
			case r.claim.declared[0] != proven:
				fmt.Printf("    declared %s in the block, and it is\n", declaration(r.claim.declared))
			}
			fmt.Printf("    %s: %s → %s\n", r.claim.file, oneLine(r.claim.find), oneLine(r.claim.after))
			if r.claim.isolate {
				fmt.Printf("    each cited check run in a binary of its own — see isolate\n")
			}
			for _, at := range sorted(setOf(r.cited)) {
				from := ""
				if n, ok := r.firstRed[at]; ok && r.claim.runs > 1 {
					from = fmt.Sprintf(", from run %d", n)
				}
				fmt.Printf("    %-58s red %d/%d unmutated · %d/%d mutated · %s%s\n",
					at.name, r.controlRed[at], r.claim.runs, r.mutantRed[at], r.claim.runs,
					r.cited[at], from)
			}
			if len(r.others) > 0 {
				fmt.Printf("    also red: %s\n", some(r.others))
			}
			if len(r.stranded) > 0 {
				fmt.Printf("    never finished: %s\n", some(r.stranded))
			}
			if r.note != "" {
				fmt.Printf("    %s\n", indent(r.note))
			}
		}
	}

	fmt.Printf("\n── inventory ──\n")
	for _, v := range order {
		if n := len(byVerdict[v]); n > 0 {
			fmt.Printf("  %-18s %3d\n", v, n)
		}
	}
	fmt.Printf("  %-18s %3d\n", "claims", len(results))
	fmt.Printf("  %-18s %3d\n", "as declared", len(results)-len(off))
	fmt.Printf("  %-18s %3d\n", "adrift", len(off))
	// Printed only when it happened. A permanent "elsewhere 0" would read as
	// another verdict rather than as the exception it is, and the inventory's
	// last line is where the eye stops.
	if n := len(moved(results)); n > 0 {
		fmt.Printf("  %-18s %3d  ** not the tree named above\n", "taken elsewhere", n)
	}
}

// oneLine makes a multi-line anchor printable without pretending it was one
// line: the escapes are the ones the catalog itself is written in, so what the
// report shows can be pasted back into it.
func oneLine(s string) string {
	if s == "" {
		return "(nothing)"
	}
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\t", `\t`)
	if len(s) > 72 {
		s = s[:72] + "…"
	}
	return s
}

// some names a list of checks without letting one mutation that killed a whole
// package take the report over. The count is what carries the size; the names
// are there to start from, and -json carries all of them.
func some(checks []check) string {
	const show = 6

	names := make([]string, 0, len(checks))
	for _, at := range checks {
		names = append(names, at.name)
	}
	if len(names) <= show {
		return strings.Join(names, " ")
	}
	return fmt.Sprintf("%s … and %d more", strings.Join(names[:show], " "), len(names)-show)
}

// declaration writes a verdict set the way a block spells one.
func declaration(vs []verdict) string {
	out := make([]string, 0, len(vs))
	for _, v := range vs {
		out = append(out, string(v))
	}
	return strings.Join(out, "|")
}

// setOf is the keys of a map of checks, for the one place a result is printed in
// a fixed order rather than the order it was cited in.
func setOf(m map[check]outcome) map[check]bool {
	out := map[check]bool{}
	for at := range m {
		out[at] = true
	}
	return out
}

func indent(s string) string {
	return strings.ReplaceAll(strings.TrimSpace(s), "\n", "\n    ")
}

// printJSON is the same verdicts for something other than a person. It carries
// the outcome of every cited check by name rather than a pass/fail, because the
// distinctions this tool exists to draw are all inside the word "failed".
//
// An object with the tree at the top rather than the bare array this used to
// emit. The identity belongs to the run and not to each row, and repeating it
// per claim would invite a consumer to read two rows as two trees. Anything
// parsing the old shape has to move; that is the cost of a machine-readable
// verdict having had no subject.
func printJSON(id ident, results []result) error {
	type out struct {
		ID         string            `json:"id"`
		Title      string            `json:"title"`
		Verdict    verdict           `json:"verdict"`
		Declared   []verdict         `json:"declared"`
		AsDeclared bool              `json:"as_declared"`
		File       string            `json:"file"`
		Find       string            `json:"find"`
		After      string            `json:"after"`
		Runs       int               `json:"runs"`
		Race       bool              `json:"race"`
		Isolate    bool              `json:"isolate"`
		Sole       bool              `json:"sole"`
		Cited      map[string]string `json:"cited"`
		ControlRed map[string]int    `json:"red_unmutated"`
		MutantRed  map[string]int    `json:"red_mutated"`
		FirstRed   map[string]int    `json:"first_red,omitempty"`
		Others     []string          `json:"also_red,omitempty"`
		Stranded   []string          `json:"never_finished,omitempty"`
		Against    string            `json:"taken_against,omitempty"`
		Note       string            `json:"note,omitempty"`
	}

	var rows []out
	for _, r := range results {
		cited := map[string]string{}
		for at, o := range r.cited {
			cited[at.name] = o.String()
		}
		control, mutant := map[string]int{}, map[string]int{}
		for at, n := range r.controlRed {
			control[at.name] = n
		}
		for at, n := range r.mutantRed {
			mutant[at.name] = n
		}
		firstRed := map[string]int{}
		for at, n := range r.firstRed {
			firstRed[at.name] = n
		}
		rows = append(rows, out{
			ID: r.claim.id, Title: r.claim.title, Verdict: r.verdict,
			Declared: r.claim.declared, AsDeclared: r.asDeclared(),
			File: r.claim.file, Find: r.claim.find, After: r.claim.after,
			Runs: r.claim.runs, Race: r.claim.race, Isolate: r.claim.isolate, Sole: r.claim.sole,
			Cited: cited, ControlRed: control, MutantRed: mutant,
			FirstRed: firstRed, Others: named(r.others), Stranded: named(r.stranded),
			Against: r.against, Note: r.note,
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })

	e := json.NewEncoder(os.Stdout)
	e.SetIndent("", "  ")
	// moved is written whether or not it happened, and that is the difference
	// between it and taken_against on a row. A consumer that reads the top-level
	// tree and stops has to be able to find out that the tree is not the whole
	// story; a key that is absent in the ordinary case cannot tell it that.
	return e.Encode(struct {
		Tree   string `json:"tree"`
		Head   string `json:"head,omitempty"`
		Dirty  bool   `json:"dirty"`
		Moved  bool   `json:"moved"`
		Claims []out  `json:"claims"`
	}{Tree: id.tree, Head: id.head, Dirty: id.dirty, Moved: len(moved(results)) > 0, Claims: rows})
}
