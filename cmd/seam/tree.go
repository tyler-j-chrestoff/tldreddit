package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// copyTree copies the working tree into a fresh directory under the system's
// temp dir and returns it.
//
// The whole tree, minus .git. Not `git archive`, not the index, not tracked
// files only: this tool checks the code as it sits on disk, which is the same
// thing .githooks/pre-commit checks and the same thing a person editing has in
// front of them. A tool that quietly checked HEAD instead would report green on
// a claim the working tree has already broken.
//
// The copy is the entire safety story. Nothing here ever opens a file in the
// repository for writing — the anchor is read out of the copy and spliced into
// the copy — so an interrupted run leaves the repository byte-identical rather
// than leaving it to a restore step that a crash skips. Words' own version
// mutates in place and restores afterward, and it needs a guard for the case
// where it does not get to.
func copyTree(root string) (string, error) {
	dir, err := os.MkdirTemp("", "seam-")
	if err != nil {
		return "", err
	}

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		// .git is skipped because it is large, because nothing under test reads
		// it, and because a copy of it is a second repository one wrong command
		// away from being pushed.
		if d.IsDir() && d.Name() == ".git" {
			return fs.SkipDir
		}

		target := filepath.Join(dir, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		// A symlink is copied as a link rather than followed: following one that
		// points outside the tree would put material in the copy that is not in
		// the repository, and this tool's whole claim is that it ran the tree.
		if d.Type()&fs.ModeSymlink != 0 {
			dest, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(dest, target)
		}
		if !d.Type().IsRegular() {
			return nil
		}

		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, info.Mode().Perm())
	})
	if err != nil {
		os.RemoveAll(dir)
		return "", err
	}
	return dir, nil
}

// occurrences counts how many times a claim's anchor appears in the file it names.
//
// Counted rather than searched-for, and separately from applying the mutation,
// because two of the three answers are findings: none means the catalog has gone
// stale against the code, and more than one — in a block that does not say which
// — means nobody chose, and the tool must not choose on their behalf.
func occurrences(dir string, c claim) (int, error) {
	src, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(c.file)))
	if err != nil {
		return 0, err
	}
	return strings.Count(string(src), c.find), nil
}

// mutate applies one claim's break inside a copied tree. The anchor is expected
// to be there and to be unambiguous; [occurrences] is what settles that, and it
// runs first.
func mutate(dir string, c claim) error {
	path := filepath.Join(dir, filepath.FromSlash(c.file))
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	src := string(b)
	at := nth(src, c.find, c.occ)
	if at < 0 {
		return fmt.Errorf("%s holds no occurrence %d of the anchor", c.file, c.occ)
	}

	out := src[:at] + c.after + src[at+len(c.find):]
	if out == src {
		return fmt.Errorf("the mutation changes nothing: after is what find already says")
	}
	return os.WriteFile(path, []byte(out), 0o644)
}

// nth is the index of the occ'th occurrence of find, or -1.
func nth(s, find string, occ int) int {
	at := 0
	for range occ {
		i := strings.Index(s[at:], find)
		if i < 0 {
			return -1
		}
		at += i + len(find)
	}
	return at - len(find)
}
