package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The address is the whole of what binds a run's verdicts to a tree, so what it
// distinguishes and what it deliberately does not is the only interesting thing
// about it. Every row below is one of those two facts, stated as a change to a
// tree and an answer to "is this still the same tree".
//
// The rows that expect the same address are not filler. They are where the
// limit lives: a checkout's permissions beyond the execute bit come from the
// umask of whoever cloned, so digesting them would give two identical checkouts
// two addresses — and the price is that a change nobody can see in git is a
// change this cannot see either.
//
// **Prove these can fail before trusting them.** In [digestTree], write
// `a.str("")` in place of the *file* case's `a.str(e.detail)` — the link case
// spells it identically, and stubbing both reddens a third row — and the two
// content rows go red while every other row stays green. Drop the
// `a.str(e.path)` line and the rename row goes red alone. Neither stub touches
// the rows that expect the address to hold; `a.str(dir)` after the format tag
// is what reddens those, and it reddens the first row too, which is what that
// first row is for.
//
// The catalog carries the first of those as a claim of its own —
// `tree-address-drops-contents` in docs/CLAIMS.md — so it is re-derived rather
// than left here as a sentence somebody ran once.
func TestWhatTheTreesAddressDistinguishes(t *testing.T) {
	for _, s := range []struct {
		what string
		do   func(t *testing.T, dir string)
		same bool
	}{
		{
			what: "nothing at all",
			do:   func(*testing.T, string) {},
			same: true,
		},
		{
			what: "one byte of one file",
			do: func(t *testing.T, dir string) {
				write(t, filepath.Join(dir, "a", "one.go"), "package a\n\nconst n = 2\n")
			},
		},
		{
			what: "two files' contents swapped",
			do: func(t *testing.T, dir string) {
				one := filepath.Join(dir, "a", "one.go")
				two := filepath.Join(dir, "a", "two.go")
				first, second := read(t, one), read(t, two)
				write(t, one, second)
				write(t, two, first)
			},
		},
		{
			// The new name keeps the old one's place in the sorted order, and
			// that is the whole of what this row is testing. `uno.go` was the
			// obvious choice and it sorts past `two.go`, so the entries change
			// places and the address moves whether or not paths are in it at
			// all — the row passed for a reason that had nothing to do with the
			// claim, and the control that was supposed to redden it did not.
			what: "a file renamed, byte for byte the same",
			do: func(t *testing.T, dir string) {
				if err := os.Rename(filepath.Join(dir, "a", "one.go"), filepath.Join(dir, "a", "ones.go")); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			what: "the execute bit",
			do: func(t *testing.T, dir string) {
				if err := os.Chmod(filepath.Join(dir, "run.sh"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			what: "where a symlink points",
			do: func(t *testing.T, dir string) {
				link := filepath.Join(dir, "latest.go")
				if err := os.Remove(link); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("a/two.go", link); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			what: "an empty directory nothing reads",
			do: func(t *testing.T, dir string) {
				if err := os.Mkdir(filepath.Join(dir, "empty"), 0o755); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			what: "when a file was last written",
			do: func(t *testing.T, dir string) {
				old := time.Date(1998, 3, 1, 0, 0, 0, 0, time.UTC)
				if err := os.Chtimes(filepath.Join(dir, "a", "one.go"), old, old); err != nil {
					t.Fatal(err)
				}
			},
			same: true,
		},
		{
			what: "a permission bit that is not execute",
			do: func(t *testing.T, dir string) {
				if err := os.Chmod(filepath.Join(dir, "a", "one.go"), 0o664); err != nil {
					t.Fatal(err)
				}
			},
			same: true,
		},
	} {
		t.Run(s.what, func(t *testing.T) {
			before := seed(t)
			after := seed(t)
			s.do(t, after)

			// Two trees built the same way, addressed the same way. The row that
			// changes nothing is what says the address is a function of the tree
			// and not of the directory it happens to sit in — every other row
			// rests on that, since each compares two different directories.
			was, is := address(t, before), address(t, after)
			if (was == is) != s.same {
				verb := "changed"
				if s.same {
					verb = "held"
				}
				t.Errorf("changing %s: the address should have %s, and it did not\n  %s\n  %s",
					s.what, verb, was, is)
			}
		})
	}
}

// A copy is what the address is defined over, so the copy has to carry
// everything the address reads. Anything [copyTree] flattens — a link followed,
// an execute bit dropped — would show up here as a tree that stopped being
// itself the moment it was copied, which would make every identity this tool
// prints a statement about the copier rather than about the code.
func TestCopyingATreeDoesNotChangeItsAddress(t *testing.T) {
	root := seed(t)

	dir, err := copyTree(root)
	if err != nil {
		t.Fatalf("copy: %v", err)
	}
	defer os.RemoveAll(dir)

	if was, is := address(t, root), address(t, dir); was != is {
		t.Errorf("the copy is not the tree it was copied from\n  %s\n  %s", was, is)
	}
}

// Every claim gets its own copy of the tree, minutes apart, and the report binds
// all of them to one address. Somebody editing during a run makes that false
// quietly — both halves run fine — so the copies are compared, and a claim taken
// against a copy that no longer matches the baseline is marked with the tree it
// was actually taken against.
//
// Marked and still judged: this used to be a refusal, and the refusal made any
// write anywhere under the repository fatal for the minutes a catalog takes. Both
// halves are run here because only the pair is a contract. A mark that fires on
// every run is not a warning, and a verdict that goes missing when the tree moves
// is the refusal back under another name.
//
// What this cannot check, and what [printIdent] and the exit status carry
// instead: that the mark is *said*. A marked row nobody prints is a false receipt
// that reads exactly like an honest one.
func TestATreeThatMovedMidRunIsMarkedRatherThanRefused(t *testing.T) {
	// Its own module rather than [seed]'s, because this one has to run: a claim
	// judged either way needs a check that exists, passes unmutated and asserts
	// under the mutation.
	root := t.TempDir()
	write(t, filepath.Join(root, "go.mod"), "module fixture\n\ngo 1.25.4\n")
	write(t, filepath.Join(root, "n", "n.go"), "package n\n\nconst N = 1\n")
	write(t, filepath.Join(root, "n", "n_test.go"), "package n\n\nimport \"testing\"\n\n"+
		"func TestNIsOne(t *testing.T) {\n\tif N != 1 {\n\t\tt.Fatalf(\"N is %d\", N)\n\t}\n}\n")

	at := check{pkg: "fixture/n", name: "TestNIsOne"}
	c := claim{id: "moved", file: "n/n.go", find: "N = 1", after: "N = 0", occ: 1,
		red: []string{at.name}, declared: []verdict{proven}, runs: 1}
	// The baseline's own run, standing in as the control's first and only sample:
	// with runs 1 and no isolation nothing is executed against base, which is why
	// base and root can be the same directory here.
	first := suite{tests: map[check]outcome{at: green}}
	here := address(t, root)

	for _, s := range []struct {
		what     string
		baseline string
		want     string
	}{
		{what: "the tree held", baseline: here, want: ""},
		{what: "the tree moved", baseline: "an address this tree does not have", want: here},
	} {
		t.Run(s.what, func(t *testing.T) {
			r, err := checkOne(root, root, s.baseline, c, map[check]bool{at: true}, first, false)
			if err != nil {
				t.Fatalf("checkOne: %v", err)
			}
			if r.verdict != proven {
				t.Errorf("verdict = %s, want proven — a drifted claim is still judged (note %q)",
					r.verdict, r.note)
			}
			if r.against != s.want {
				t.Errorf("taken against %q, want %q", r.against, s.want)
			}
		})
	}
}

// seed writes a tree with one of everything the address has an opinion about:
// nested files, an executable, and a symlink.
func seed(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	write(t, filepath.Join(dir, "go.mod"), "module fixture\n")
	write(t, filepath.Join(dir, "a", "one.go"), "package a\n\nconst n = 1\n")
	write(t, filepath.Join(dir, "a", "two.go"), "package a\n\nconst m = 2\n")
	write(t, filepath.Join(dir, "run.sh"), "#!/bin/sh\nexit 0\n")

	if err := os.Chmod(filepath.Join(dir, "run.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("a/one.go", filepath.Join(dir, "latest.go")); err != nil {
		t.Fatal(err)
	}
	return dir
}

func address(t *testing.T, dir string) string {
	t.Helper()

	d, err := digestTree(dir)
	if err != nil {
		t.Fatalf("address %s: %v", dir, err)
	}
	if len(d) != 64 {
		t.Fatalf("address %s is %q, which is not a sha-256", dir, d)
	}
	return d
}
