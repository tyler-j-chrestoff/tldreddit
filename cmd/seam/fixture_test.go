package main

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// A tool that reports which checks cannot fail has to be able to fail itself,
// and saying so is not the same as showing it. So this builds a throwaway module
// with checks of its own, writes a catalog against it, and runs the real
// pipeline — the same [runCatalog], the same baseline, the same mutations — asserting
// that every verdict the tool can reach is reachable.
//
// Against a fixture and never against memory/ or tui/. A self-test that mutated
// the product would be a second, silent catalog nobody reads, and it would go red
// for reasons that have nothing to do with this tool.
//
// **Prove this check can fail before trusting it.** Two stubs, both run,
// because one of them alone would leave half this file unexamined.
//
// Replace [judge]'s body with `return result{claim: c, verdict: proven, cited:
// map[string]outcome{}, firstRed: map[string]int{}, controlRed: map[string]int{},
// mutantRed: map[string]int{}}` and the rows judge decides go red: vacuous,
// over-red, reddened-by-crash here, and unattributable in the test below. The
// other three stay green, because [checkOne] settles them before judge is ever
// called — which is exactly the half a single stub would let through.
//
// So also put `return result{claim: c, verdict: proven}, nil` at the top of
// [checkOne] (go vet reports the unreachable code; that is the stub working, not
// a problem to fix). Every row but the one that expects `proven` goes red, which
// is the whole table.
func TestTheToolCanReportEveryVerdictItHas(t *testing.T) {
	root := fixture(t)

	claims, err := parseClaims(strings.NewReader(fixtureClaims))
	if err != nil {
		t.Fatalf("the fixture catalog does not parse: %v", err)
	}

	got, took, id, err := runCatalog(root, claims, false, false)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if took.checks == 0 {
		t.Error("the baseline found no checks at all, so nothing below is a control")
	}
	// The verdicts below are about a tree, and the pipeline has to say which one
	// even here, where the tree is a throwaway module that no repository knows
	// about. That second half is the part worth running: [gitAnchor] failing has
	// to leave an identity behind, not an empty line.
	if len(id.tree) != 64 {
		t.Errorf("the run produced no address for the tree it ran: %q", id.tree)
	}

	want := map[string]verdict{
		"fixture-proven":         proven,
		"fixture-vacuous":        vacuous,
		"fixture-over-red":       overRed,
		"fixture-crash":          crashProof,
		"fixture-stale-anchor":   staleAnchor,
		"fixture-stale-citation": staleCitation,
		"fixture-broken-build":   brokenBuild,
		"fixture-declared":       vacuous,
		"fixture-set":            vacuous,
		"fixture-killed":         killed,
	}
	if len(got) != len(want) {
		t.Fatalf("ran %d claims, want %d", len(got), len(want))
	}
	for _, r := range got {
		if w, named := want[r.claim.id]; !named {
			t.Errorf("%s is not in the table", r.claim.id)
		} else if r.verdict != w {
			t.Errorf("%s came back %s, want %s (cited %v, unmutated %v, mutated %v, also red %v, note %q)",
				r.claim.id, r.verdict, w, r.cited, r.controlRed, r.mutantRed, r.others, r.note)
		}

		// The gate, end to end. Two claims here make the identical break: the one
		// that declares nothing expects proven and is adrift, and the one that
		// declares vacuous is exactly where it says it is. Same verdict, opposite
		// sides of the exit status.
		if want := r.claim.id == "fixture-declared" || r.verdict == proven; r.asDeclared() != want {
			t.Errorf("%s: asDeclared = %v (verdict %s, declared %s), want %v",
				r.claim.id, r.asDeclared(), r.verdict, r.claim.declared, want)
		}
	}

	// fixture-declared and fixture-set both come back vacuous. The first declares
	// exactly that and is where it says it is; the second declares a two-member
	// set that vacuous is not in, and is adrift — which is the half that says a
	// set is not an opt-out. Most of the other fixtures are adrift by design,
	// since they exist to come back somewhere other than proven.
	off := adrift(got)
	if slices.Contains(off, "fixture-declared") {
		t.Error("a claim that declared its own verdict was called adrift")
	}
	if !slices.Contains(off, "fixture-set") {
		t.Error("a claim that landed outside its declared set was not called adrift")
	}
}

// The unattributable verdict needs a check that reddens on its own, which means
// a fixture that fails on a run it was already green on. It counts its own runs
// in a file beside itself: the first run passes and every one after it fails, so
// the baseline is honestly green and the control that follows is honestly red.
//
// Its own module and its own [runCatalog], because a check that reddens on the second
// run would poison the baseline of anything sharing a tree with it — which is
// itself the reason the control exists.
func TestAFlakyCheckIsUnattributableRatherThanProven(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "go.mod"), "module fixture\n\ngo 1.25.4\n")
	write(t, filepath.Join(root, "flake", "flake.go"), fixtureFlakeSrc)
	write(t, filepath.Join(root, "flake", "flake_test.go"), fixtureFlakeTest)

	claims, err := parseClaims(strings.NewReader(fixtureFlakeClaim))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	got, _, _, err := runCatalog(root, claims, false, false)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if got[0].verdict != unattributable {
		t.Errorf("a check that reddens without the mutation came back %s, want %s (unmutated %v, mutated %v)",
			got[0].verdict, unattributable, got[0].controlRed, got[0].mutantRed)
	}
}

// fixture writes the throwaway module the catalog above is written against.
func fixture(t *testing.T) string {
	root := t.TempDir()
	write(t, filepath.Join(root, "go.mod"), "module fixture\n\ngo 1.25.4\n")
	write(t, filepath.Join(root, "thing", "thing.go"), fixtureSrc)
	write(t, filepath.Join(root, "thing", "thing_test.go"), fixtureTest)
	return root
}

// The fixture's own source. Step is what a mutation moves; Blind is here to be
// broken by a mutation nothing looks at, which is what a vacuous claim is.
const fixtureSrc = `package thing

import (
	"fmt"
	"os"
)

var _, _ = fmt.Println, os.Exit

const Step = 2

func Wide(n int) int { return n * Step }

func Blind() string { return "nobody checks this" }

func Boom() int { return 1 }

func Throw() int { return 2 }
`

// Two checks, so soleness has something to be wrong about, and neither of them
// reads Blind.
const fixtureTest = `package thing

import "testing"

func TestWideIsStepped(t *testing.T) {
	if got := Wide(3); got != 6 {
		t.Errorf("Wide(3) = %d, want 6", got)
	}
}

func TestWideIsEven(t *testing.T) {
	if Wide(1)%2 != 0 {
		t.Errorf("Wide(1) = %d, which is odd", Wide(1))
	}
}

func TestBoom(t *testing.T) {
	if Boom() != 1 {
		t.Error("Boom is not 1")
	}
}

func TestThrow(t *testing.T) {
	if Throw() != 2 {
		t.Error("Throw is not 2")
	}
}
`

// One claim per verdict [checkOne] and [judge] can reach between them. The prose
// each block would carry in the real catalog is the comment above it here.
const fixtureClaims = "" +
	// Both checks read Step, so citing both and claiming nothing else is a claim
	// that holds.
	"```seam\nid: fixture-proven\nfile: thing/thing.go\nfind: Step = 2\nafter: Step = 3\n" +
	"red: TestWideIsStepped, TestWideIsEven\nsole: true\n```\n" +

	// Nothing reads Blind, so the cited check goes on passing: proven by its own
	// existence and by nothing else.
	"```seam\nid: fixture-vacuous\nfile: thing/thing.go\nfind: nobody checks this\nafter: something else entirely\n" +
	"red: TestWideIsStepped\n```\n" +

	// One check cited, two of them notice.
	"```seam\nid: fixture-over-red\nfile: thing/thing.go\nfind: Step = 2\nafter: Step = 3\n" +
	"red: TestWideIsStepped\nsole: true\n```\n" +

	// The cited check dies rather than fails, so what it caught is not its own
	// assertion.
	"```seam\nid: fixture-crash\nfile: thing/thing.go\nfind: func Boom() int { return 1 }\n" +
	"after: func Boom() int { panic(\"boom\") }\nred: TestBoom\n```\n" +

	"```seam\nid: fixture-stale-anchor\nfile: thing/thing.go\nfind: Step = 9999\nafter: Step = 3\n" +
	"red: TestWideIsStepped\n```\n" +

	"```seam\nid: fixture-stale-citation\nfile: thing/thing.go\nfind: Step = 2\nafter: Step = 3\n" +
	"red: TestNobodyEverWrote\n```\n" +

	"```seam\nid: fixture-broken-build\nfile: thing/thing.go\nfind: Step = 2\nafter: Step = \"two\"\n" +
	"red: TestWideIsStepped\n```\n" +

	// The same break as fixture-vacuous, declaring what it honestly is. The gate
	// passes this one and fails the other, and neither is suppressed.
	"```seam\nid: fixture-declared\nfile: thing/thing.go\nfind: nobody checks this\nafter: something else entirely\n" +
	"red: TestWideIsStepped\nverdict: vacuous\n```\n" +

	// The same break again, declaring a two-member set that the verdict it comes
	// back as is not in. A set widens what a claim may honestly be; it does not
	// stop the claim being checked.
	"```seam\nid: fixture-set\nfile: thing/thing.go\nfind: nobody checks this\nafter: something else again\n" +
	"red: TestWideIsStepped\nverdict: proven|killed-mid-check\n```\n" +

	// The throw shape, which is the one the catalog's own crash claim rides and
	// which a panic does not reproduce: the check starts, the binary dies under
	// it, and go test reports no terminal action for it at all. Written as an
	// explicit exit rather than a real concurrent-map throw, because a real one is
	// nondeterministic and would make this test the flake it is about; the event
	// shape — a run with nothing after it — is identical, and that shape is what
	// the classification rests on.
	"```seam\nid: fixture-killed\nfile: thing/thing.go\nfind: func Throw() int { return 2 }\n" +
	"after: func Throw() int {\\n\\tfmt.Println(\"fatal error: concurrent map read and map write\")\\n\\tos.Exit(2)\\n\\treturn 0\\n}\n" +
	"red: TestThrow\n```\n"

const fixtureFlakeSrc = `package flake

const Step = 2

func Wide(n int) int { return n * Step }
`

// Counted in a file rather than in a clock or a random number, so the run this
// reddens on is the same run every time and this test does not become the flake
// it is about.
const fixtureFlakeTest = `package flake

import (
	"os"
	"strconv"
	"testing"
)

func TestWideIsStepped(t *testing.T) {
	n := 0
	if b, err := os.ReadFile("runs"); err == nil {
		n, _ = strconv.Atoi(string(b))
	}
	if err := os.WriteFile("runs", []byte(strconv.Itoa(n+1)), 0o644); err != nil {
		t.Fatal(err)
	}
	if n > 0 {
		t.Errorf("run %d of this check fails for reasons of its own", n+1)
	}
	if got := Wide(3); got != 6 {
		t.Errorf("Wide(3) = %d, want 6", got)
	}
}
`

// Two samples: the baseline takes the first, which passes, and the control takes
// the second, which does not.
const fixtureFlakeClaim = "```seam\nid: fixture-unattributable\nfile: flake/flake.go\n" +
	"find: Step = 2\nafter: Step = 3\nred: TestWideIsStepped\nruns: 2\n```\n"
