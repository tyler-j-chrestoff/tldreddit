package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// The catalog is a file people edit by hand, so everything it will not
// understand has to be refused rather than defaulted. A block that parses into
// something other than what it says is worse here than one that fails to parse:
// this file is spliced into source.
func TestABlockParsesIntoExactlyWhatItSays(t *testing.T) {
	doc := `
# claims

## The view's capped append

Prose about the claim, naming where it is asserted.

` + "```seam" + `
id: view-add-uncapped
file: memory/view.go
find: v[:len(v):len(v)]
after: v
red: TestConcurrentAddDoesNotShareAViewsSpareCapacity, TestOther
sole: true
race: true
runs: 3
` + "```" + `
`
	got, err := parseClaims(strings.NewReader(doc))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("parsed %d claims, want 1", len(got))
	}

	c := got[0]
	if c.id != "view-add-uncapped" || c.file != "memory/view.go" {
		t.Errorf("id %q file %q", c.id, c.file)
	}
	if c.find != "v[:len(v):len(v)]" || c.after != "v" {
		t.Errorf("find %q after %q", c.find, c.after)
	}
	if len(c.red) != 2 || c.red[0] != "TestConcurrentAddDoesNotShareAViewsSpareCapacity" {
		t.Errorf("red %q", c.red)
	}
	if !c.sole || !c.race || c.runs != 3 || c.occ != 1 {
		t.Errorf("sole %v race %v runs %d occ %d", c.sole, c.race, c.runs, c.occ)
	}
	// The heading is the claim in human words, and the line number is where to
	// go and edit it. A report that could only name an id would make a stale
	// anchor a hunt.
	if c.title != "The view's capped append" {
		t.Errorf("title %q", c.title)
	}
	if c.line != 8 {
		t.Errorf("block at line %d, want 8", c.line)
	}
}

// A claim says what verdict it expects, and proven is only the default. The gate
// is an equality against that, so both halves need saying: a block with no
// verdict key expects proven, and a block with one expects what it says.
func TestAClaimDeclaresItsOwnVerdict(t *testing.T) {
	quiet := "```seam\nid: quiet\nfile: a.go\nfind: q\nafter: r\nred: T\n```\n"
	loud := "```seam\nid: loud\nfile: a.go\nfind: q\nafter: r\nred: T\nverdict: killed-mid-check\n```\n"
	both := "```seam\nid: both\nfile: a.go\nfind: q\nafter: r\nred: T\nverdict: proven|killed-mid-check\n```\n"

	got, err := parseClaims(strings.NewReader(quiet + loud + both))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	for i, want := range [][]verdict{
		{proven},
		{killed},
		// A set, for a claim whose honest answer is two verdicts. Order is the
		// block's, since it is what gets printed back.
		{proven, killed},
	} {
		if !slices.Equal(got[i].declared, want) {
			t.Errorf("%s declares %v, want %v", got[i].id, got[i].declared, want)
		}
	}
}

// A set is not an opt-out, which is the whole reason it replaced one. Every
// verdict outside it still fails, including the ones that look like good news:
// a claim declaring that it sometimes dies and sometimes proves must still fail
// when it goes vacuous, which is the case a suppression would have hidden.
func TestADeclaredSetStillFailsOnEverythingOutsideIt(t *testing.T) {
	at := func(id string, declared []verdict, got verdict) result {
		return result{claim: claim{id: id, declared: declared}, verdict: got}
	}
	pair := []verdict{proven, killed}

	results := []result{
		at("one-member-hit", []verdict{proven}, proven),
		at("set-hit-first", pair, proven),
		at("set-hit-second", pair, killed),
	}
	if off := adrift(results); len(off) != 0 {
		t.Errorf("adrift = %v, want none", off)
	}

	results = append(results,
		at("set-missed-into-vacuous", pair, vacuous),
		at("set-missed-into-crash", pair, crashProof),
		at("one-member-missed", []verdict{proven}, vacuous))
	if off := adrift(results); len(off) != 3 {
		t.Errorf("adrift = %v, want the three that landed outside their set", off)
	}
}

// The declaration is not a suppression, and this is the half that says so: the
// gate fails in either direction, so a claim that quietly starts passing cleanly
// trips it exactly as loudly as one that stops.
func TestADeclarationFailsInEitherDirection(t *testing.T) {
	for _, s := range []struct {
		name     string
		declared []verdict
		got      verdict
		want     bool
	}{
		{"proven and proven", []verdict{proven}, proven, true},
		{"proven and not", []verdict{proven}, vacuous, false},
		{"killed declared and killed", []verdict{killed}, killed, true},
		{"killed declared and proven after all", []verdict{killed}, proven, false},
		{"killed declared and vacuous instead", []verdict{killed}, vacuous, false},
	} {
		t.Run(s.name, func(t *testing.T) {
			r := result{claim: claim{declared: s.declared}, verdict: s.got}
			if r.asDeclared() != s.want {
				t.Errorf("declared %v, came back %s: asDeclared = %v, want %v",
					s.declared, s.got, r.asDeclared(), s.want)
			}
		})
	}
}

// The exit status carries two findings and they are not the same finding. A
// claim outside its declaration is something to go and read; a run that spanned
// two trees is something to run again on a still one. Both are non-zero, because
// an exit 0 is what gets quoted afterwards as "seam passed" and a partial receipt
// has not passed anything it can name — and they are different codes, because a
// caller that cannot tell them apart will treat the cheap one as the expensive
// one and stop running the tool.
//
// The last row is the precedence, and it is the only row with an argument behind
// it rather than a definition: drift weakens a claim's evidence rather than
// removing it, so a claim outside its own declaration is still the louder thing.
func TestTheExitStatusSaysWhichKindOfFindingItIs(t *testing.T) {
	ok := result{claim: claim{id: "ok", declared: []verdict{proven}}, verdict: proven}
	off := result{claim: claim{id: "off", declared: []verdict{proven}}, verdict: vacuous}
	elsewhere := ok
	elsewhere.claim.id = "elsewhere"
	elsewhere.against = "an address the baseline did not have"

	for _, s := range []struct {
		name string
		in   []result
		want int
	}{
		{"every claim where it says it is, one tree", []result{ok}, 0},
		{"a claim outside its declaration", []result{ok, off}, 2},
		{"one tree short of a receipt", []result{ok, elsewhere}, 3},
		{"both, and the gate wins", []result{off, elsewhere}, 2},
	} {
		t.Run(s.name, func(t *testing.T) {
			if got := status(s.in); got != s.want {
				t.Errorf("status = %d, want %d", got, s.want)
			}
		})
	}
}

// A multi-line anchor is the reason the escapes exist: the only way to unlock
// the store and go on compiling is to replace a field declaration and the lines
// around it.
func TestAnAnchorCanSpanLines(t *testing.T) {
	doc := "```seam\nid: x\nfile: a.go\nfind: one\\ntwo\nafter: \\tthree\\\\four\nred: TestX\n```\n"

	got, err := parseClaims(strings.NewReader(doc))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if want := "one\ntwo"; got[0].find != want {
		t.Errorf("find %q, want %q", got[0].find, want)
	}
	if want := "\tthree\\four"; got[0].after != want {
		t.Errorf("after %q, want %q", got[0].after, want)
	}
}

func TestAMalformedBlockIsRefused(t *testing.T) {
	for _, s := range []struct{ name, block, says string }{
		{"an unknown key", "id: x\nfile: a.go\nfind: q\nafter: r\nred: T\nsold: true\n", "not a key"},
		{"no id", "file: a.go\nfind: q\nafter: r\nred: T\n", "id is required"},
		{"no after", "id: x\nfile: a.go\nfind: q\nred: T\n", "after is required"},
		{"no red", "id: x\nfile: a.go\nfind: q\nafter: r\n", "red is required"},
		{"a key twice", "id: x\nid: y\nfile: a.go\nfind: q\nafter: r\nred: T\n", "given twice"},
		{"a line that is not a pair", "id: x\nnonsense\n", "not key: value"},
		{"an occurrence that is not one", "id: x\nfile: a.go\nfind: q\nafter: r\nred: T\nocc: 0\n", "not an occurrence"},
		{"an escape nobody defined", "id: x\nfile: a.go\nfind: a\\qb\nafter: r\nred: T\n", "not an escape"},
		{"among without sole", "id: x\nfile: a.go\nfind: q\nafter: r\nred: T\namong: A\n", "sole is not set"},
		{"a verdict nobody reports", "id: x\nfile: a.go\nfind: q\nafter: r\nred: T\nverdict: fine\n", "not a verdict"},
		{"a verdict set with a member nobody reports", "id: x\nfile: a.go\nfind: q\nafter: r\nred: T\nverdict: proven|fine\n", "not a verdict"},
		{"a flag that is not a bool", "id: x\nfile: a.go\nfind: q\nafter: r\nred: T\nisolate: yes please\n", "not true or false"},
	} {
		t.Run(s.name, func(t *testing.T) {
			_, err := parseClaims(strings.NewReader("```seam\n" + s.block + "```\n"))
			if err == nil {
				t.Fatalf("parsed, want a refusal mentioning %q", s.says)
			}
			if !strings.Contains(err.Error(), s.says) {
				t.Errorf("refused with %q, want it to mention %q", err, s.says)
			}
		})
	}
}

// Two blocks under one id would make -run ambiguous and the report a lie about
// which mutation produced which verdict.
func TestADuplicateIdIsRefused(t *testing.T) {
	one := "```seam\nid: same\nfile: a.go\nfind: q\nafter: r\nred: T\n```\n"
	_, err := parseClaims(strings.NewReader(one + one))
	if err == nil || !strings.Contains(err.Error(), "already used") {
		t.Fatalf("parsed twice under one id: %v", err)
	}
}

func TestAnUnclosedBlockIsRefused(t *testing.T) {
	_, err := parseClaims(strings.NewReader("```seam\nid: x\nfile: a.go\n"))
	if err == nil || !strings.Contains(err.Error(), "never closed") {
		t.Fatalf("%v", err)
	}
}

func TestNthOccurrence(t *testing.T) {
	const src = "aXbXcX"
	for _, s := range []struct {
		occ, want int
	}{{1, 1}, {2, 3}, {3, 5}, {4, -1}} {
		if got := nth(src, "X", s.occ); got != s.want {
			t.Errorf("occurrence %d at %d, want %d", s.occ, got, s.want)
		}
	}
}

// The safety invariant, and the only one in this tool worth a test of its own:
// the repository is never opened for writing. Words' version mutates in place
// and restores afterward, which is one interrupted run away from a dirty tree.
func TestTheMutationLandsInTheCopyAndNeverInTheTree(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "go.mod"), "module x\n")
	write(t, filepath.Join(root, "a.go"), "package x\n\nconst step = 2\n")
	write(t, filepath.Join(root, ".git", "HEAD"), "ref: refs/heads/main\n")

	dir, err := copyTree(root)
	if err != nil {
		t.Fatalf("copy: %v", err)
	}
	defer os.RemoveAll(dir)

	if _, err := os.Stat(filepath.Join(dir, ".git")); !os.IsNotExist(err) {
		t.Error("the copy carries .git, which nothing under test reads and which a wrong command could push")
	}

	if err := mutate(dir, claim{file: "a.go", find: "step = 2", after: "step = 0", occ: 1}); err != nil {
		t.Fatalf("mutate: %v", err)
	}
	if got := read(t, filepath.Join(dir, "a.go")); !strings.Contains(got, "step = 0") {
		t.Errorf("the copy was not mutated: %q", got)
	}
	if got := read(t, filepath.Join(root, "a.go")); !strings.Contains(got, "step = 2") {
		t.Errorf("the tree was written to: %q", got)
	}
}

// How many times the anchor appears is counted before anything is mutated,
// because two of the three answers are findings: none is a catalog gone stale,
// and more than one in a block that never chose is a claim nobody has finished
// writing.
func TestTheAnchorIsCountedBeforeItIsUsed(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "a.go"), "package x\n\nconst a = 1\nconst b = 1\n")

	for _, s := range []struct {
		find string
		want int
	}{
		{"nothing like this", 0},
		{"const a", 1},
		{"= 1", 2},
	} {
		got, err := occurrences(dir, claim{file: "a.go", find: s.find})
		if err != nil {
			t.Fatalf("occurrences: %v", err)
		}
		if got != s.want {
			t.Errorf("%q occurs %d times, want %d", s.find, got, s.want)
		}
	}
}

// A mutation that changes nothing would run the whole suite to prove that a
// green tree is green, and report it as a claim standing up.
func TestAMutationThatChangesNothingIsRefused(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "a.go"), "package x\n")

	if err := mutate(dir, claim{file: "a.go", find: "package", after: "package", occ: 1}); err == nil {
		t.Error("accepted a mutation that changes nothing")
	}
}

// The event lines below are recorded from real runs of go1.25.4, not composed:
// the distinction the tool rests on — a check reporting its own failure versus
// the process dying under it — is one only the toolchain can settle.
func TestAnAssertionAndACrashAreNotTheSameRed(t *testing.T) {
	events := strings.Join([]string{
		`{"Action":"run","Package":"p","Test":"TestPasses"}`,
		`{"Action":"pass","Package":"p","Test":"TestPasses"}`,
		`{"Action":"run","Package":"p","Test":"TestAsserts"}`,
		`{"Action":"output","Package":"p","Test":"TestAsserts","Output":"    p_test.go:6: boom\n"}`,
		`{"Action":"fail","Package":"p","Test":"TestAsserts"}`,
		`{"Action":"run","Package":"p","Test":"TestPanics"}`,
		`{"Action":"output","Package":"p","Test":"TestPanics","Output":"--- FAIL: TestPanics (0.00s)\n"}`,
		`{"Action":"fail","Package":"p","Test":"TestPanics"}`,
		`{"Action":"output","Package":"p","Test":"TestPanics","Output":"panic: kaboom [recovered, repanicked]\n"}`,
		`{"Action":"run","Package":"p","Test":"TestThrows"}`,
		`{"Action":"fail","Package":"p"}`,
	}, "\n")

	s, err := readEvents(strings.NewReader(events))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	for name, want := range map[string]outcome{
		"TestPasses":  green,
		"TestAsserts": assertRed,
		// The panic banner arrives after the failure, because the testing
		// package reports the check before the stack unwinds past it.
		"TestPanics": crashRed,
		// Started, never finished: something killed the binary under it.
		"TestThrows": abortedRun,
	} {
		if got := s.tests[check{"p", name}]; got != want {
			t.Errorf("%s came back %s, want %s", name, got, want)
		}
	}
}

// A race the detector reports is a check doing its job — the detector is the
// oracle those checks were written to consult — and must not be filed with the
// crashes.
func TestADetectedRaceIsTheCheckAsserting(t *testing.T) {
	events := strings.Join([]string{
		`{"Action":"run","Package":"p","Test":"TestRacy"}`,
		`{"Action":"output","Package":"p","Test":"TestRacy","Output":"WARNING: DATA RACE\n"}`,
		`{"Action":"output","Package":"p","Test":"TestRacy","Output":"    testing.go:1617: race detected during execution of test\n"}`,
		`{"Action":"fail","Package":"p","Test":"TestRacy"}`,
	}, "\n")

	s, _ := readEvents(strings.NewReader(events))
	if got := s.tests[check{"p", "TestRacy"}]; got != assertRed {
		t.Errorf("a detected race came back %s, want %s", got, assertRed)
	}
}

// A check that skipped said nothing, and a claim resting on it is resting on a
// green nobody earned. The harness frames in this repository skip unless HARNESS
// is set, so this is the live case rather than the theoretical one.
func TestASkipIsNotAPass(t *testing.T) {
	events := strings.Join([]string{
		`{"Action":"run","Package":"p","Test":"TestSkips"}`,
		`{"Action":"output","Package":"p","Test":"TestSkips","Output":"--- SKIP: TestSkips (0.00s)\n"}`,
		`{"Action":"skip","Package":"p","Test":"TestSkips"}`,
	}, "\n")

	s, _ := readEvents(strings.NewReader(events))
	if got := s.tests[check{"p", "TestSkips"}]; got != skipped {
		t.Errorf("a skipped check came back %s, want %s", got, skipped)
	}
}

// A tree that does not compile — or does not vet, which arrives the same way —
// has to be reported before any red under it is read as evidence.
func TestABuildFailureIsReadBeforeAnyRed(t *testing.T) {
	events := strings.Join([]string{
		`{"ImportPath":"p [p.test]","Action":"build-output","Output":"# p\n"}`,
		`{"ImportPath":"p [p.test]","Action":"build-output","Output":"p/p.go:5:45: cannot use \"nope\" as int\n"}`,
		`{"ImportPath":"p [p.test]","Action":"build-fail"}`,
		`{"Action":"fail","Package":"p"}`,
	}, "\n")

	s, _ := readEvents(strings.NewReader(events))
	if !strings.Contains(s.build, "cannot use") {
		t.Errorf("build error came back %q", s.build)
	}
}

// Subtests are their parent's verdict. The testing package fails the parent
// alongside them, and a catalog cites whole checks.
func TestASubtestIsItsParentsVerdict(t *testing.T) {
	events := strings.Join([]string{
		`{"Action":"run","Package":"p","Test":"TestParent"}`,
		`{"Action":"run","Package":"p","Test":"TestParent/case"}`,
		`{"Action":"fail","Package":"p","Test":"TestParent/case"}`,
		`{"Action":"fail","Package":"p","Test":"TestParent"}`,
	}, "\n")

	s, _ := readEvents(strings.NewReader(events))
	if len(s.tests) != 1 || s.tests[check{"p", "TestParent"}] != assertRed {
		t.Errorf("tests %v", s.tests)
	}
}

func TestJudge(t *testing.T) {
	// One package, since what is under test here is the verdict and not the
	// keying; TestTwoPackagesWithOneTestNameDoNotShareASlot covers the keying.
	a, b, c := check{"p", "A"}, check{"p", "B"}, check{"p", "C"}
	known := map[check]bool{a: true, b: true, c: true}

	suites := func(runs ...map[check]outcome) []suite {
		var out []suite
		for _, r := range runs {
			out = append(out, suite{tests: r})
		}
		return out
	}

	// clean is the control a claim is entitled to expect: the same checks, the
	// same number of samples, nothing red. Where a case wants a dirty control it
	// says so, because that is the case worth reading.
	clean := func(n int) []suite {
		var out []suite
		for range n {
			out = append(out, suite{tests: map[check]outcome{a: green, b: green, c: green}})
		}
		return out
	}

	for _, s := range []struct {
		name    string
		claim   claim
		cited   []check
		among   []check
		control []suite
		runs    []suite
		want    verdict
	}{
		{
			"a cited check that bites",
			claim{runs: 1}, []check{a}, nil, clean(1),
			suites(map[check]outcome{a: assertRed, b: green}),
			proven,
		},
		{
			"a cited check that does not",
			claim{runs: 1}, []check{a}, nil, clean(1),
			suites(map[check]outcome{a: green, b: assertRed}),
			vacuous,
		},
		{
			// The reason runs exists: an interleaving is not a property of the
			// program, so one green run is not evidence the check cannot catch
			// it, while one red run is evidence that it can — provided the
			// control below it stayed green throughout.
			"a check that reddens in only one run",
			claim{runs: 3}, []check{a}, nil, clean(3),
			suites(
				map[check]outcome{a: green},
				map[check]outcome{a: assertRed},
				map[check]outcome{a: green},
			),
			proven,
		},
		{
			// The same shape, with the control reddening once. Without it this
			// reads as a proof; with it, the check is simply flaky and the
			// mutation is not what made it red.
			"a check that reddens without the mutation too",
			claim{runs: 3}, []check{a}, nil,
			suites(
				map[check]outcome{a: green},
				map[check]outcome{a: assertRed},
				map[check]outcome{a: green},
			),
			suites(
				map[check]outcome{a: assertRed},
				map[check]outcome{a: assertRed},
				map[check]outcome{a: green},
			),
			unattributable,
		},
		{
			// Three ways short of proven that are not the same fact, and were one
			// verdict until a reviewer pointed at the heading saying "reddened"
			// over a rate of zero.
			"a check reddened by a panic",
			claim{runs: 1}, []check{a}, nil, clean(1),
			suites(map[check]outcome{a: crashRed}),
			crashProof,
		},
		{
			"a check the process died under",
			claim{runs: 1}, []check{a}, nil, clean(1),
			suites(map[check]outcome{a: abortedRun}),
			killed,
		},
		{
			"a cited check that never started",
			claim{runs: 1}, []check{a, b}, nil, clean(1),
			suites(map[check]outcome{a: assertRed}),
			neverRan,
		},
		{
			"soleness claimed and broken",
			claim{sole: true, runs: 1}, []check{a}, nil, clean(1),
			suites(map[check]outcome{a: assertRed, b: assertRed, c: green}),
			overRed,
		},
		{
			"soleness never claimed",
			claim{runs: 1}, []check{a}, nil, clean(1),
			suites(map[check]outcome{a: assertRed, b: assertRed}),
			proven,
		},
		{
			// A table comparing three checks against each other claims nothing
			// about the rest of the suite, and judging it against the suite
			// would report a failure the table never made.
			"soleness narrowed to the checks the claim compares",
			claim{sole: true, runs: 1}, []check{a}, []check{a, b}, clean(1),
			suites(map[check]outcome{a: assertRed, b: green, c: assertRed}),
			proven,
		},
	} {
		t.Run(s.name, func(t *testing.T) {
			got := judge(s.claim, s.cited, s.among, s.control, s.runs, known)
			if got.verdict != s.want {
				t.Errorf("verdict %s, want %s (cited %v, unmutated %v, mutated %v, also red %v)",
					got.verdict, s.want, got.cited, got.controlRed, got.mutantRed, got.others)
			}
		})
	}
}

// The keying bug this file exists to keep closed. Two packages may carry one test
// name; go test runs packages in parallel; a map keyed on the bare name gives them
// one slot and the last event wins — so a red check can be overwritten by a green
// one somewhere else and the baseline reports a tree green that never was.
func TestTwoPackagesWithOneTestNameDoNotShareASlot(t *testing.T) {
	events := strings.Join([]string{
		`{"Action":"run","Package":"one","Test":"TestSame"}`,
		`{"Action":"fail","Package":"one","Test":"TestSame"}`,
		`{"Action":"run","Package":"two","Test":"TestSame"}`,
		`{"Action":"pass","Package":"two","Test":"TestSame"}`,
	}, "\n")

	s, _ := readEvents(strings.NewReader(events))
	if len(s.tests) != 2 {
		t.Fatalf("two packages' checks landed in %d slot(s): %v", len(s.tests), s.tests)
	}
	if got := s.tests[check{"one", "TestSame"}]; got != assertRed {
		t.Errorf("the failing one came back %s — the passing one wrote over it", got)
	}
	if got := s.tests[check{"two", "TestSame"}]; got != green {
		t.Errorf("the passing one came back %s", got)
	}
}

// And a citation of such a name is refused rather than resolved to whichever
// package won the race, because an isolated run has to target one of them and a
// verdict would then be about whichever was guessed.
func TestAnAmbiguousCitationIsRefused(t *testing.T) {
	known := map[check]bool{
		{"one", "TestSame"}: true,
		{"two", "TestSame"}: true,
		{"one", "TestOnly"}: true,
	}

	if _, bad := resolve([]string{"TestOnly"}, known); bad != "" {
		t.Errorf("an unambiguous name was refused: %s", bad)
	}
	_, bad := resolve([]string{"TestSame"}, known)
	if bad == "" {
		t.Fatal("a name two packages carry resolved to one of them")
	}
	for _, want := range []string{"one", "two"} {
		if !strings.Contains(bad, want) {
			t.Errorf("the refusal %q does not name package %q", bad, want)
		}
	}
}

// Both rates reach the report, whatever the verdict. The control is the half a
// reader has no other way to know was taken.
func TestBothRatesAreCounted(t *testing.T) {
	a := check{"p", "A"}
	r := judge(
		claim{runs: 3}, []check{a}, nil,
		[]suite{
			{tests: map[check]outcome{a: green}},
			{tests: map[check]outcome{a: green}},
			{tests: map[check]outcome{a: green}},
		},
		[]suite{
			{tests: map[check]outcome{a: green}},
			{tests: map[check]outcome{a: assertRed}},
			{tests: map[check]outcome{a: assertRed}},
		},
		map[check]bool{a: true},
	)
	if r.controlRed[a] != 0 || r.mutantRed[a] != 2 {
		t.Errorf("rates: %d/3 unmutated, %d/3 mutated; want 0/3 and 2/3",
			r.controlRed[a], r.mutantRed[a])
	}
}

// Which run a nondeterministic proof landed on is part of the finding. A claim
// proven once in three runs and one proven in all three are different facts
// about the same check, and reporting them identically is the averaging this
// tool exists not to do.
func TestTheRunAProofLandedInIsReported(t *testing.T) {
	a := check{"p", "A"}
	r := judge(
		claim{runs: 3}, []check{a}, nil,
		[]suite{
			{tests: map[check]outcome{a: green}},
			{tests: map[check]outcome{a: green}},
			{tests: map[check]outcome{a: green}},
		},
		[]suite{
			{tests: map[check]outcome{a: green}},
			{tests: map[check]outcome{a: green}},
			{tests: map[check]outcome{a: assertRed}},
		},
		map[check]bool{a: true},
	)
	if r.firstRed[a] != 3 {
		t.Errorf("first red in run %d, want 3", r.firstRed[a])
	}
}

// A check outside the cited set that never finished is not a failure — it never
// got to say anything — and it is not silence either.
func TestAStrandedCheckIsNeitherRedNorIgnored(t *testing.T) {
	a, b := check{"p", "A"}, check{"p", "B"}
	r := judge(
		claim{sole: true, runs: 1}, []check{a}, nil,
		[]suite{{tests: map[check]outcome{a: green, b: green}}},
		[]suite{{tests: map[check]outcome{a: assertRed, b: abortedRun}}},
		map[check]bool{a: true, b: true},
	)
	if r.verdict != proven {
		t.Errorf("verdict %s, want %s: a stranded check is not a failure", r.verdict, proven)
	}
	if len(r.stranded) != 1 || r.stranded[0] != b {
		t.Errorf("stranded %v, want [B]", r.stranded)
	}
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
