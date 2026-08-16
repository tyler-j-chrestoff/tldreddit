// Command seam re-breaks what this repository claims is guarded, and reports
// which of its checks failed to notice.
//
// The claims live in docs/CLAIMS.md, in prose, each followed by the exact
// mutation that breaks the behaviour it describes and the exact checks that must
// go red when it does. This runs them. A check that stays green under the break
// it is cited for is proven by nothing but its own existence, and this is the
// tool that says so out loud.
//
// It exists because of what happened to the prose around it. Four review rounds
// on one change caught false guarantees, retracted cost figures and stale
// numbers, and every one of them was found by a person re-deriving a number
// rather than by anybody reading the sentence. Meanwhile the repository's own
// tables of "this mutation fails exactly these tests" were hand-run once and
// never again. A claim nobody re-derives is the defect this project keeps
// relearning, and a table of them is a pile of it.
//
// What it does not do: score anything. There is no percentage here and there
// will not be one — a number that goes up is a number that can be made to go up,
// and the thing being measured is whether the checks bite, which a ratio cannot
// say. The output is an inventory.
//
// It never writes inside the repository. Every mutation lands in a copy of the
// working tree under the system temp dir, so an interrupted run leaves the tree
// exactly as it found it rather than leaving it to a restore step that a crash
// skips.
//
// Every verdict is printed under the address of the tree it was taken against
// — see [ident]. A verdict is a statement about a tree, and this tool used to
// print its verdicts beside a wall clock, which named the moment and not the
// subject: "22 proven" stopped being checkable the instant anything committed.
// The address is derived from the tree's content, which is what the record this
// tool guards does with its own identities.
//
// Somebody editing while it runs is expected rather than exceptional, since a
// catalog takes minutes and other work does not stop for it. Such a run is
// reported as what it is — see [moved] — and never printed under one address as
// though one tree had produced it.
//
// Usage:
//
//	go run ./cmd/seam            # run every claim
//	go run ./cmd/seam -list      # the catalog and the tree's address, running nothing
//	go run ./cmd/seam -run <id>  # one claim
//	go run ./cmd/seam -json      # the same verdicts, machine-readable
//
// Exit status: 0 when every claim is where its own block says it will be, 2 when
// one is not, 3 when the tree moved mid-run so the verdicts do not all belong to
// the address above them. 1 is the tool failing to run at all. [status] argues
// the distinction between 2 and 3.
package main

import (
	"errors"
	"flag"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"time"
)

// everything is the whole module: what a claim is judged against, always. An
// isolated run is the one exception, and it names a single package instead.
const everything = "./..."

func main() {
	var (
		list   = flag.Bool("list", false, "print the catalog and run nothing")
		only   = flag.String("run", "", "run one claim, by id")
		asJSON = flag.Bool("json", false, "report as JSON rather than as an inventory")
		keep   = flag.Bool("keep", false, "leave each mutated copy on disk and name it")
	)
	flag.Parse()

	if err := run(*list, *only, *asJSON, *keep); err != nil {
		fmt.Fprintf(os.Stderr, "seam: %v\n", err)
		os.Exit(1)
	}
}

func run(list bool, only string, asJSON, keep bool) error {
	root, err := repoRoot()
	if err != nil {
		return err
	}

	f, err := os.Open(filepath.Join(root, "docs", "CLAIMS.md"))
	if err != nil {
		return err
	}
	defer f.Close()

	claims, err := parseClaims(f)
	if err != nil {
		return err
	}
	if len(claims) == 0 {
		return errors.New("docs/CLAIMS.md holds no seam blocks")
	}

	if only != "" {
		var one []claim
		for _, c := range claims {
			if c.id == only {
				one = append(one, c)
			}
		}
		if len(one) == 0 {
			return fmt.Errorf("no claim with id %q; try -list", only)
		}
		claims = one
	}

	if list {
		id, err := identify(root)
		if err != nil {
			return err
		}
		printCatalog(id, claims)
		return nil
	}

	results, took, id, err := runCatalog(root, claims, keep, !asJSON)
	if err != nil {
		return err
	}

	if asJSON {
		if err := printJSON(id, results); err != nil {
			return err
		}
	} else {
		printReport(root, id, took, results)
	}

	// The gate: every claim where it says it is. Not "everything proven" — a
	// claim may declare a verdict, or a set of them where its honest answer is
	// nondeterministic — and not a threshold either: a verdict outside the
	// declared set fails whether it is worse than declared or better. Beside it,
	// and distinct from it, a run whose verdicts do not all belong to one tree.
	// [status] settles which code and argues why; the report reads the same two
	// functions, so the two cannot come to different conclusions.
	if code := status(results); code != 0 {
		os.Exit(code)
	}
	return nil
}

// runCatalog runs the catalog: one baseline, then, per claim, a control and a mutant.
//
// The baseline is not ceremony and it is not an optimisation. Every verdict below
// is a statement about what a mutation changed, and a suite that was already red
// says nothing about what anything changed — so a working tree that does not pass
// is a refusal, not a run with a caveat. It doubles as the list of checks that
// exist and ran, which is what turns a citation of a check nobody has written,
// or one that skipped, into a finding instead of a silent green.
//
// The [ident] it returns is what binds the verdicts to a tree. It is taken from
// the baseline's own copy, so the identity printed is the identity of the thing
// that was actually run rather than of the directory at some other moment.
func runCatalog(root string, claims []claim, keep, chatty bool) ([]result, baseline, ident, error) {
	racing := false
	for _, c := range claims {
		racing = racing || c.race
	}

	base, err := copyTree(root)
	if err != nil {
		return nil, baseline{}, ident{}, err
	}
	defer os.RemoveAll(base)

	tree, err := digestTree(base)
	if err != nil {
		return nil, baseline{}, ident{}, err
	}
	head, dirty := gitAnchor(root)
	id := ident{tree: tree, head: head, dirty: dirty}

	known := map[check]bool{}
	control := map[bool]suite{}
	took := baseline{at: time.Now(), race: racing}

	for _, race := range []bool{false, true} {
		if race && !racing {
			continue
		}
		if chatty {
			fmt.Fprintf(os.Stderr, "seam: baseline%s\n", raceLabel(race))
		}
		s, err := runSuite(base, race, "", everything)
		if err != nil {
			return nil, took, id, err
		}
		if s.build != "" {
			return nil, took, id, fmt.Errorf("the working tree does not build:\n%s", s.build)
		}
		for at, o := range s.tests {
			switch o {
			case green:
				known[at] = true
			case skipped:
				// Not a failure and not a check either. It stays out of known, so
				// a claim citing it comes back stale-citation rather than resting
				// on a green that was never earned.
				took.skipped++
			default:
				return nil, took, id, fmt.Errorf("the working tree is not green: %s in %s is %s%s",
					at.name, at.pkg, o, raceLabel(race))
			}
		}
		control[race] = s
	}
	took.checks = len(known)

	var out []result
	for _, c := range claims {
		if chatty {
			fmt.Fprintf(os.Stderr, "seam: %s\n", c.id)
		}
		r, err := checkOne(root, base, id.tree, c, known, control[c.race], keep)
		if err != nil {
			return nil, took, id, fmt.Errorf("%s: %w", c.id, err)
		}
		out = append(out, r)
	}
	return out, took, id, nil
}

// baseline is the control the whole report rests on, carried so it can be
// printed rather than assumed. A reader has no other way to know it was taken.
type baseline struct {
	at      time.Time
	checks  int
	skipped int
	race    bool
}

// checkOne runs one claim. tree is the address the baseline was taken against,
// and a copy that does not match it is marked rather than refused — see below.
func checkOne(root, base, tree string, c claim, known map[check]bool, first suite, keep bool) (r result, err error) {
	// Before the copy and before the mutation. A claim citing a check nobody has
	// written, or one that skipped, has nothing to run; a claim citing a name two
	// packages both carry cannot be run at all, since an isolated run would have
	// to guess which package to target and a verdict would then be about whichever
	// one it guessed.
	cited, bad := resolve(c.red, known)
	if bad != "" {
		return result{claim: c, verdict: staleCitation, note: bad}, nil
	}
	among, bad := resolve(c.among, known)
	if bad != "" {
		return result{claim: c, verdict: staleCitation, note: "among: " + bad}, nil
	}

	dir, err := copyTree(root)
	if err != nil {
		return result{}, err
	}
	if keep {
		fmt.Fprintf(os.Stderr, "seam: %s mutated in %s\n", c.id, dir)
	} else {
		defer os.RemoveAll(dir)
	}

	// Every claim gets its own copy, taken minutes after the baseline's, so the
	// report's one printed identity is a claim about all of them. Somebody
	// editing during a run makes that false — quietly, since each half would run
	// fine — and a report that binds its verdicts to the wrong tree is worse than
	// one that binds them to nothing.
	//
	// Marked, not refused, and the reversal is worth its own sentence: refusing
	// made every write anywhere under the repository fatal for the minutes a
	// catalog takes, so no other work could go on beside a check, which bought
	// receipt integrity with the one resource here that is actually scarce. The
	// receipt is kept honest instead by naming the other tree — on this row, in
	// the identity block, and in the exit status. What is not allowed is a single
	// address printed as though it covered a run that spanned two.
	//
	// Set by defer because there are five ways out below and a sixth would forget
	// one. A row that lost its mark is exactly the false receipt this is here to
	// prevent, and it would be indistinguishable from an honest one.
	copied, err := digestTree(dir)
	if err != nil {
		return result{}, err
	}
	if copied != tree {
		defer func() { r.against = copied }()
	}

	// An anchor that matches in more than one place, in a block that does not say
	// which, is refused rather than resolved to the first. It fails safe today —
	// the wrong occurrence produces a loud vacuous rather than a quiet green — but
	// a routine refactor that moves two functions past each other turns that into
	// a false accusation against a healthy check, and the block would still read
	// as though somebody had chosen.
	n, err := occurrences(dir, c)
	if err != nil {
		return result{}, err
	}
	switch {
	case n == 0 || n < c.occ:
		return result{claim: c, verdict: staleAnchor,
			note: fmt.Sprintf("%s holds %d occurrences of the anchor, and the block asks for number %d",
				c.file, n, c.occ)}, nil
	case n > 1 && !c.occSet:
		return result{claim: c, verdict: ambiguousAnchor,
			note: fmt.Sprintf("%s holds %d occurrences of the anchor and the block does not say which; add occ",
				c.file, n)}, nil
	}
	if err := mutate(dir, c); err != nil {
		return result{}, err
	}

	// The control, in the unmutated copy, sampled exactly as many times and in
	// exactly the same shape as the mutant is about to be. Same shape is the
	// whole point: an isolated check runs alone in its own package on both sides,
	// because a check sampled one way and compared against a check sampled
	// another way is two measurements and one conclusion.
	//
	// The baseline's own run is the first control sample where the shape matches,
	// which is not a shortcut — it is the same command against the same tree.
	control, err := sample(base, c, cited, first)
	if err != nil {
		return result{}, err
	}
	// Checked on the control side as well as the mutant side. The baseline built,
	// so this is very nearly unreachable — and "very nearly" is the state in which
	// an unchecked error waits.
	for _, s := range control {
		if s.build != "" {
			return result{}, fmt.Errorf("the unmutated copy stopped building:\n%s", s.build)
		}
	}

	// One whole run of the mutant always happens, whatever else does. It is where
	// everything outside the cited set is accounted for, and a claim judged
	// without it would be a claim about the checks it already trusted.
	whole, err := runSuite(dir, c.race, "", everything)
	if err != nil {
		return result{}, err
	}
	if whole.build != "" {
		return result{claim: c, verdict: brokenBuild, note: whole.build}, nil
	}

	runs, err := sample(dir, c, cited, whole)
	if err != nil {
		return result{}, err
	}
	for _, s := range runs {
		if s.build != "" {
			return result{claim: c, verdict: brokenBuild, note: s.build}, nil
		}
	}
	return judge(c, cited, among, control, runs, known), nil
}

// sample takes one claim's runs against one tree, mutated or not.
//
// first is a whole run already taken against that tree, used as the first sample
// where the claim's shape is a whole run. Nothing is short-circuited beyond that:
// every sample is taken, including after a check has already reddened, because
// the two rates the report prints are only comparable if they were counted the
// same way.
func sample(dir string, c claim, cited []check, first suite) ([]suite, error) {
	var out []suite
	for i := range c.runs {
		if !c.isolate {
			if i == 0 {
				out = append(out, first)
				continue
			}
			s, err := runSuite(dir, c.race, "", everything)
			if err != nil {
				return nil, err
			}
			out = append(out, s)
			continue
		}

		// The cited checks observed one at a time, over the whole run's picture of
		// everything else. Merged rather than reported separately because a
		// verdict is about one mutation, and two half-pictures of it are what a
		// reader would then have to combine by hand.
		merged := suite{tests: maps.Clone(first.tests)}
		for _, at := range cited {
			s, err := runSuite(dir, c.race, at.name, at.pkg)
			if err != nil {
				return nil, err
			}
			if s.build != "" {
				return []suite{s}, nil
			}
			delete(merged.tests, at)
			if o, ran := s.tests[at]; ran {
				merged.tests[at] = o
			}
		}
		out = append(out, merged)
	}
	return out, nil
}

func raceLabel(race bool) string {
	if race {
		return " (-race)"
	}
	return ""
}

// repoRoot walks up from the working directory to the module root, so the tool
// runs the same from anywhere inside the tree.
func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		up := filepath.Dir(dir)
		if up == dir {
			return "", errors.New("no go.mod above the working directory")
		}
		dir = up
	}
}
