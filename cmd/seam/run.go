package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"slices"
	"strings"
)

// outcome is what one run of the suite did to one check.
//
// Green and red are not the whole vocabulary, and that is the point. A check
// that reddened because the process died before it could assert has not shown it
// bites; a check that never ran at all has shown nothing whatsoever. Collapsing
// those into "failed" is how a tool that exists to catch a claim nobody
// re-derived starts making claims nobody re-derives.
type outcome int

const (
	green outcome = iota

	// assertRed is the check reporting its own failure: the testing package
	// called it, which includes the race detector's own verdict, because there
	// the detector is the oracle the check was written to consult.
	assertRed

	// crashRed is a red arrived at through a panic or a runtime throw. The
	// mutation broke something, and what it broke may be somewhere the check
	// never reached.
	crashRed

	// abortedRun is a check that started and never finished, because something
	// else in its package killed the binary first.
	abortedRun

	// skipped is a check the run declined to make: it exists and it said
	// nothing. Green is a check that ran and passed, and the two are only the
	// same word to a reader who stops at the exit status — a cited check that
	// skips is exactly as much evidence as one that was deleted.
	skipped

	// absent is a check that never started in this run.
	absent
)

// strength orders the outcomes by how much a run's saying it settles, so that
// unioning several runs is one comparison rather than a table of cases. An
// assertion red is the strongest — it is a check doing its job — and never
// having run is the weakest, because it settles nothing at all.
func strength(o outcome) int {
	switch o {
	case assertRed:
		return 4
	case crashRed:
		return 3
	case abortedRun:
		return 2
	case green:
		return 1
	case skipped:
		return 1
	}
	return 0
}

func (o outcome) String() string {
	switch o {
	case green:
		return "green"
	case assertRed:
		return "red, by its own assertion"
	case crashRed:
		return "red, by a crash"
	case abortedRun:
		return "aborted mid-run"
	case skipped:
		return "skipped"
	}
	return "never ran"
}

// check is one test, in the package it lives in.
//
// Both halves, and the package half is not decoration. Go permits two packages to
// carry the same test name, `go test ./...` runs packages in parallel, and a map
// keyed on the bare name gives them one slot where the last event to arrive wins
// — so a red check in one package can be overwritten by a green one of the same
// name in another, and the baseline reports a tree green that never was. There
// are no duplicate names in this repository today, which makes this latent rather
// than live, and latent is exactly how it would arrive.
//
// It is also what an isolated run has to target: the package a check lives in,
// not whichever package won that race.
type check struct {
	pkg  string
	name string
}

func (c check) String() string { return c.name }

// suite is one run of `go test` against one tree.
type suite struct {
	// tests is every check the run has an opinion about. Subtests are folded into
	// their parent, which the testing package fails alongside them.
	tests map[check]outcome

	// build is the compiler's or vet's complaint when the tree did not get as
	// far as running. A red under a tree that does not build proves nothing, so
	// this is checked before anything else is read.
	build string
}

// event is the part of `go test -json` this reads. Verified against go1.25.4 by
// running each case rather than from documentation: an assertion failure and a
// panic both arrive as Action "fail" on the test, a compile error and a vet
// failure both arrive as "build-fail" on an ImportPath, and a runtime throw
// arrives as no terminal action at all for whatever was running.
type event struct {
	Action     string
	Package    string
	Test       string
	Output     string
	ImportPath string
}

// runSuite runs go test once against a tree and reports what happened to every
// check it had an opinion about.
//
// Every claim is judged against one run of the whole module, always. A run of
// only the cited checks could not answer the second half of a claim — that
// nothing else notices — and that half is the one prose gets wrong. The
// narrowing arguments below never replace that run; they add to it.
//
// only and target are that addition, and they exist for [claim].isolate alone:
// one named check, in one named package. Both halves are load-bearing. A check
// run by itself against the whole module still builds and runs every other
// package beside it, and that load is enough to decide which of two racers
// finishes first — measured on the store's own unlocked mutation, where the
// runtime's throw beat the check's assertion every time under the module-wide
// run and lost to it regularly in the package alone.
//
// -count=1 because a second run of an unchanged tree is otherwise served from
// the test cache, which would make the runs key mean three readings of one run.
func runSuite(dir string, race bool, only, target string) (suite, error) {
	args := []string{"test", "-json", "-count=1", "-timeout=120s"}
	if race {
		args = append(args, "-race")
	}
	if only != "" {
		args = append(args, "-run", "^"+only+"$")
	}
	args = append(args, target)

	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	// HARNESS is cleared rather than inherited. The frames it prints assert
	// nothing, and a tool whose verdicts depend on the environment of whoever
	// invoked it is a tool with a hidden argument.
	cmd.Env = append(cmd.Environ(), "HARNESS=")

	out, err := cmd.StdoutPipe()
	if err != nil {
		return suite{}, err
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return suite{}, err
	}

	s, perr := readEvents(out)
	// The exit status is deliberately not read: `go test` exits non-zero for
	// every mutant that works, so it carries no information this does not
	// already have from the events.
	_ = cmd.Wait()
	return s, perr
}

func readEvents(r io.Reader) (suite, error) {
	s := suite{tests: map[check]outcome{}}

	crashed := map[check]bool{}
	var build strings.Builder

	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for sc.Scan() {
		var e event
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			// Not every line is an event: a test binary writing to stdout
			// outside the harness lands here. Skipping is right — the events
			// carry the verdicts and this line carries none.
			continue
		}

		switch e.Action {
		case "build-output":
			build.WriteString(e.Output)
		case "build-fail":
			if build.Len() == 0 {
				build.WriteString(e.ImportPath)
				build.WriteString(" failed to build")
			}
		}
		if e.Test == "" {
			continue
		}

		// A subtest's verdict is its parent's — the testing package fails the
		// parent alongside it — so the map is keyed by whole checks, which is
		// what a catalog cites.
		name, _, _ := strings.Cut(e.Test, "/")
		at := check{pkg: e.Package, name: name}
		switch e.Action {
		case "run":
			// Aborted until something says otherwise: a check that starts and is
			// never spoken of again is one the binary died under, and the run
			// saying nothing about it is the only evidence there will be.
			if _, seen := s.tests[at]; !seen {
				s.tests[at] = abortedRun
			}
		case "pass":
			s.tests[at] = green
		case "skip":
			s.tests[at] = skipped
		case "fail":
			if crashed[at] {
				s.tests[at] = crashRed
			} else {
				s.tests[at] = assertRed
			}
		case "output":
			if crashy(e.Output) {
				crashed[at] = true
				// The panic often arrives after the fail, since the testing
				// package reports the failure before the stack unwinds past it.
				if s.tests[at] == assertRed {
					s.tests[at] = crashRed
				}
			}
		}
	}
	if err := sc.Err(); err != nil {
		return s, err
	}
	s.build = strings.TrimSpace(build.String())
	return s, nil
}

// crashy reports whether a line of test output is the runtime talking rather
// than the check.
//
// Column zero is what makes this safe to read literally: everything the testing
// package prints on a check's behalf is indented and prefixed with a file and a
// line, so an unindented panic banner is the process dying and not a message
// somebody wrote.
//
// A check that prints such a line itself — with fmt.Println rather than through
// the testing package — would be misfiled, and the direction that errs in is the
// safe one: a claim that would have been proven is reported reddened-by-crash
// instead, which fails the gate and gets read. Left as it is, and said here so
// the next reader does not have to work it out twice.
func crashy(line string) bool {
	for _, p := range []string{
		"panic:",
		"fatal error:",
		"goroutine ",
		"[signal ",
		"SIGQUIT:",
		"*** Test killed",
		"test timed out after",
	} {
		if strings.HasPrefix(line, p) {
			return true
		}
	}
	return false
}

// verdict is what the tool concluded about one claim. Seven of them, and every
// one names a different state the catalog or the tree can be in; a boolean here
// would report the two most interesting findings as the same thing.
type verdict string

const (
	// proven: every cited check failed by its own assertion, and if the claim
	// says so, nothing outside the cited set failed with it.
	proven verdict = "proven"

	// vacuous: the mutation landed, the tree built, and a cited check went on
	// passing. This is the finding the tool exists to produce. It is not fixed
	// by editing the check until it fails — it is reported.
	vacuous verdict = "vacuous"

	// overRed: the claim said this check was the only one that would notice, and
	// another one noticed too.
	overRed verdict = "over-red"

	// crashProof: a cited check reddened, but through a panic rather than through
	// its own assertion, so what it caught may not be what it claims to catch.
	crashProof verdict = "reddened-by-crash"

	// killed: a cited check started and never finished, because the process died
	// under it. Its own assertion never ran, so it has not shown it is watching
	// anything — a check with no assertions in it would die exactly as loudly.
	//
	// Separate from reddened-by-crash because the two report different numbers. A
	// panic is a red this tool counted; this is not a red at all, and printing
	// "reddened" over a rate of zero was a sentence contradicting its own figure
	// on the line below it.
	killed verdict = "killed-mid-check"

	// neverRan: a cited check did not start. Something earlier killed the binary,
	// or the run never reached it. Nothing whatsoever was observed, which is a
	// third thing again.
	neverRan verdict = "never-ran"

	// staleAnchor: the source no longer says what the catalog anchors on. The
	// claim is unchecked and the catalog is out of date about the code.
	staleAnchor verdict = "stale-anchor"

	// ambiguousAnchor: the anchor matches in more than one place and the block
	// does not say which. Nobody chose, so the tool does not choose either.
	ambiguousAnchor verdict = "ambiguous-anchor"

	// staleCitation: the catalog cites a check the suite does not have, or one
	// that skipped rather than ran. A check that declined to say anything is
	// exactly as much evidence as one that was deleted.
	staleCitation verdict = "stale-citation"

	// unattributable: a cited check reddened on the unmutated tree too, so
	// nothing here can be attributed to the mutation. A true finding about the
	// claim, and never repaired by sampling until it behaves.
	unattributable verdict = "unattributable"

	// brokenBuild: the mutated tree does not compile or does not vet, so every
	// red under it proves nothing.
	brokenBuild verdict = "broken-build"
)

// result is one claim, run.
type result struct {
	claim   claim
	verdict verdict

	// against is the tree this claim was actually taken against, set only when
	// it is not the one the baseline was taken against. Empty is the ordinary
	// case and means the run's one printed address covers this row.
	//
	// A run used to be refused outright the moment two copies disagreed. That was
	// right about the receipt and wrong about the cost: a catalog takes minutes,
	// and refusing made any write anywhere under the repository during those
	// minutes fatal, so nothing else could happen beside a check. What the refusal
	// was protecting is that a verdict names its own tree, and a field naming it
	// does that as well — provided it is never silent. See [printIdent], which is
	// the half that would make this a false receipt if it were dropped.
	against string

	// cited is what became of each check the claim names, unioned over the runs.
	cited map[check]outcome

	// firstRed is the run each cited check first reddened in, one-based. It is
	// what makes a nondeterministic proof legible: a claim proven on run three
	// of three is a different fact from one proven on every run, and reporting
	// the two the same way is the averaging this tool refuses.
	firstRed map[check]int

	// others is every check outside the cited set that failed by its own
	// assertion in any run, in name order. Reported whether or not the claim
	// asked, because a mutation that reddens half the suite is worth seeing even
	// where soleness was never claimed.
	others []check

	// stranded is every check outside the cited set that aborted or never ran.
	// Not a failure — it never got to say anything — and not silence either.
	stranded []check

	// controlRed and mutantRed are how many of the runs reddened each cited
	// check, on the unmutated tree and on the mutated one. Both are printed for
	// every claim, whatever the verdict: the control is the half a reader has no
	// other way to check was taken.
	controlRed map[check]int
	mutantRed  map[check]int

	// note carries what the verdict cannot: a build error, a missing anchor.
	note string
}

// asDeclared reports whether the claim came back somewhere it said it might.
//
// Membership rather than equality, and the difference is only that some claims
// honestly have two answers. It is still exact in the direction that matters: a
// verdict outside the declared set fails, whether it is worse than declared or
// better, because a claim whose own account of itself has gone out of date is the
// defect either way.
func (r result) asDeclared() bool {
	return slices.Contains(r.claim.declared, r.verdict)
}

// adrift is every claim that is not where it says it is: the gate, in one place,
// read by the exit status and by the report alike so the two cannot come to
// different conclusions about the same run.
func adrift(results []result) []string {
	var out []string
	for _, r := range results {
		if !r.asDeclared() {
			out = append(out, r.claim.id)
		}
	}
	return out
}

// moved is every claim whose own copy of the tree did not address as the
// baseline's, because somebody wrote to the repository while the catalog ran.
//
// Not [adrift] under another name, though the two words nearly collide. Adrift is
// a claim whose declaration has gone out of date — a fact about the catalog and
// the code. This is a fact about the ground underneath the whole run: the report
// prints one address, and these are the rows it does not describe. A claim can be
// either, both, or neither, and collapsing them would leave a reader with one
// word for two things to go and do.
func moved(results []result) []result {
	var out []result
	for _, r := range results {
		if r.against != "" {
			out = append(out, r)
		}
	}
	return out
}

// status is the exit code of a run that finished, and it keeps two findings apart
// that one non-zero would collapse.
//
// 2 is the gate: a claim is not where it says it is. That is a statement about
// the catalog and the code, and it is the finding this tool exists to make.
//
// 3 is a partial receipt: the tree moved mid-run, so some verdicts were taken
// against a tree other than the address printed over them. Nothing here was shown
// to be wrong, so this is not a failed gate — but 0 is not available either. An
// exit 0 is what gets quoted later as "seam passed", and a run that spanned two
// trees has not passed anything it can name. Distinct from 2 so a caller can tell
// "go and read a claim" from "go and run it again on a still tree", and so that
// the day one of these becomes a script's condition, the script can say which.
//
// 2 wins when both hold. Drift makes a claim's evidence weaker rather than
// absent, and a claim outside its own declaration is the louder thing to go and
// read whichever tree produced it.
//
// 1 is not here and is not this function's: it is the tool failing to run at all,
// returned as an error from [run].
func status(results []result) int {
	switch {
	case len(adrift(results)) > 0:
		return 2
	case len(moved(results)) > 0:
		return 3
	}
	return 0
}

// judge turns one mutant's runs, and the control runs beside them, into a
// verdict.
//
// Two rates, never one. The mutated rate alone would count a check that reddens
// on its own as a proof of attribution, which is the shape of every
// flaky-test-mistaken-for-a-finding there has ever been; so the same checks are
// sampled the same number of times on the unmutated tree, and a claim is proven
// only where that control is clean throughout and the mutant reddened at least
// once. A control that reddens even once makes the claim unattributable — not
// false, not proven, unmeasurable by this instrument — and both rates are
// printed so that a reader can see it rather than take it.
//
// The union across mutant runs stays asymmetric on purpose: one red is evidence
// the check can catch it, and a later green does not take that back. What has
// changed is that the union is no longer allowed to speak alone.
//
// The four ways short of proven are kept apart all the way to the verdict,
// because they are four different facts: the check passed, it never started, the
// process died under it, or it reddened through a panic. The vocabulary at the
// top of this file says collapsing those is how a tool built to catch
// unre-derived claims starts making them, and judging them together was this
// function doing exactly that.
func judge(c claim, cited, among []check, control, runs []suite, known map[check]bool) result {
	r := result{
		claim:      c,
		cited:      map[check]outcome{},
		firstRed:   map[check]int{},
		controlRed: map[check]int{},
		mutantRed:  map[check]int{},
	}

	is := map[check]bool{}
	for _, at := range cited {
		is[at] = true
		r.cited[at] = absent
		// Both rates start at an explicit zero rather than at a missing key. A
		// reader of the JSON is being told the control was taken and came back
		// clean, which is not what an absent field says.
		r.controlRed[at] = 0
		r.mutantRed[at] = 0
	}

	// The control first, because nothing the mutant says means anything until it
	// comes back clean.
	for _, s := range control {
		for at := range is {
			switch s.tests[at] {
			case assertRed, crashRed:
				r.controlRed[at]++
			}
		}
	}

	failed := map[check]bool{}
	stranded := map[check]bool{}
	for i, s := range runs {
		for at := range is {
			o, ran := s.tests[at]
			if !ran {
				o = absent
			}
			// The strongest thing any run said stands. An assertion red once is
			// a check that bites; a green run afterwards does not take it back.
			if strength(o) > strength(r.cited[at]) {
				r.cited[at] = o
			}
			if o == assertRed || o == crashRed {
				r.mutantRed[at]++
				if _, seen := r.firstRed[at]; !seen {
					r.firstRed[at] = i + 1
				}
			}
		}
		// Over every check the baseline found, not over the ones this run
		// happened to report. A mutation that kills a package takes every check
		// after it off the run entirely, and reading only what the run mentioned
		// would let a claim of soleness rest on dozens of checks that never
		// executed — the silence would look like agreement.
		for at := range known {
			if is[at] {
				continue
			}
			switch o, ran := s.tests[at]; {
			case !ran, o == abortedRun:
				stranded[at] = true
			case o == assertRed, o == crashRed:
				failed[at] = true
			}
		}
	}
	r.others = sorted(failed)
	r.stranded = sorted(stranded)

	was := func(o outcome) bool {
		return slices.ContainsFunc(cited, func(at check) bool { return r.cited[at] == o })
	}
	switch {
	case slices.ContainsFunc(cited, func(at check) bool { return r.controlRed[at] > 0 }):
		r.verdict = unattributable
	case was(green), was(skipped):
		// Ran under the mutation and did not fail. Distinct from all three below:
		// this check looked and saw nothing, and those never opened their eyes.
		r.verdict = vacuous
	case was(absent):
		r.verdict = neverRan
	case was(abortedRun):
		r.verdict = killed
	case was(crashRed):
		r.verdict = crashProof
	case c.sole && len(within(r.others, among)) > 0:
		r.verdict = overRed
	default:
		r.verdict = proven
	}

	// Soleness is a claim about checks that ran. Where the mutation took some of
	// them off the run, the claim is not so much true as unexamined, and saying
	// which is the difference between a verdict and a guess.
	if n := len(within(r.stranded, among)); c.sole && n > 0 {
		r.note = fmt.Sprintf(
			"%d check(s) never finished under this mutation, so soleness is unexamined over them", n)
	}
	return r
}

// resolve turns the names a block cites into the checks the baseline actually
// ran.
//
// A name the suite does not have is a catalog gone stale against the tree. A name
// two packages both carry is a citation that cannot be resolved at all, and
// guessing which was meant is how the wrong package gets mutated and the wrong
// answer gets printed — so it is refused, loudly, in the block's own terms.
func resolve(names []string, known map[check]bool) ([]check, string) {
	var out []check
	for _, name := range names {
		var found []check
		for at := range known {
			if at.name == name {
				found = append(found, at)
			}
		}
		switch len(found) {
		case 1:
			out = append(out, found[0])
		case 0:
			return nil, fmt.Sprintf("%s is not a check that ran in this suite", name)
		default:
			slices.SortFunc(found, byCheck)
			return nil, fmt.Sprintf("%s is carried by %d packages (%s and %s); cite one that is not ambiguous",
				name, len(found), found[0].pkg, found[1].pkg)
		}
	}
	return out, ""
}

// within narrows a set of failures to the ones soleness is judged over. With no
// among, that is all of them: the claim was about the suite.
func within(failed, among []check) []check {
	if len(among) == 0 {
		return failed
	}
	var out []check
	for _, at := range failed {
		if slices.Contains(among, at) {
			out = append(out, at)
		}
	}
	return out
}

// byCheck is one order for every list this tool prints: package, then name.
func byCheck(a, b check) int {
	if d := strings.Compare(a.pkg, b.pkg); d != 0 {
		return d
	}
	return strings.Compare(a.name, b.name)
}

func sorted(set map[check]bool) []check {
	out := make([]check, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	slices.SortFunc(out, byCheck)
	return out
}
