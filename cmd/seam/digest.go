package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"hash"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

// ident is the tree a run was taken against, named two ways.
//
// A verdict with no tree named is a sentence with no subject: "22 proven" is
// true of some tree, and a reader holding only that number has no way left to
// find out which one. So every verdict this tool prints is printed under an
// identity, and the identity is derived from the content of the tree rather
// than assigned to it — the same rule the record itself runs on.
//
// head is the other kind of name, and it is a weaker one on purpose. It is
// where a person starts looking; tree is what settles the question.
type ident struct {
	// tree is [digestTree] over the copy that ran. This is the identity.
	tree string

	// head is git's short sha for the commit the tree was checked out from,
	// empty when there is no repository to ask. Orientation only: a dirty tree
	// is many trees to one sha.
	head string

	// dirty is whether git saw anything uncommitted, tracked or untracked. It
	// is what says how far the anchor above can be trusted, and it is
	// meaningless when head is empty.
	dirty bool
}

// digestTree is the content address of a tree: SHA-256 over every entry in it,
// as lowercase hex.
//
// It is called on the copy and never on the repository, and that is the claim
// it makes — this addresses the tree `go test` was handed, not the directory
// somebody meant. What counts as being *in* a tree is therefore decided by
// [copyTree] alone: .git is out, a device or a socket is out, a file .gitignore
// excludes is in, because it is on disk and can change what runs. This function
// addresses what it is given and holds no second opinion about what belongs.
//
// Ignored files being in has a consequence to say out loud: this addresses a
// working directory and not a commit. A tree that has been built, or that holds
// a database some run left behind, does not address the same as a fresh clone
// of the source it was built from. That is the honest answer — those files are
// in the copy the suite runs against — and the alternative is worse: deciding
// what belongs by asking git would make the identity depend on a tool that is
// only supposed to supply the anchor.
//
// What each entry contributes:
//
//	dir    the tag "dir", the path
//	file   the tag "file", the path, the execute bit, the SHA-256 of its bytes
//	link   the tag "link", the path, the target it was copied pointing at
//
// The tag decides how many fields follow, so no entry can be read as another
// kind of entry. Paths are slash-separated and relative to the root, and the
// entries are written in bytewise order of that path rather than in walk order,
// because a walk order is a property of the walker and this address has to be
// re-derivable by anything that can list a directory. Every variable-length
// piece is written length-first, which is memory/id.go's rule and is defensive
// here rather than load-bearing: with today's fields, a path is followed by
// fixed-width ones or by a tag from a three-word vocabulary, so no two trees
// are known to collide without it. That is a property of this field list and
// not of the encoding, and it would have to be re-argued every time a field is
// added — which is more expensive than the eight bytes.
//
// The execute bit and no other permission, which is a real limit and is stated
// rather than buried. A checkout's group and other bits come from the umask of
// whoever cloned, so digesting them would give two identical checkouts two
// addresses; git itself distinguishes exactly this one bit, and matching git is
// the behaviour a reader will expect. The cost: two trees differing only in,
// say, group-write are one tree here and two trees to [copyTree], which
// preserves the permissions it copies. Mtimes and owners are out for the same
// reason and cost nothing worth naming.
//
// The encoding is written out here rather than borrowed from memory/id.go's
// canon, and the duplication is deliberate. This tool must not import the
// packages it mutates: a claim against memory/ would then compile a mutated
// memory into cmd/seam's own tests, and every such claim would come back
// over-red for reasons that have nothing to do with the claim.
func digestTree(dir string) (string, error) {
	type entry struct {
		kind   string
		path   string
		exec   bool
		detail string
	}

	var entries []entry
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		e := entry{path: filepath.ToSlash(rel)}

		switch {
		case d.IsDir():
			e.kind = "dir"
		case d.Type()&fs.ModeSymlink != 0:
			e.kind = "link"
			// Read, never followed. What the copy holds is a link, and a link is
			// its target string; resolving it would address the file it points
			// at, which may not be in this tree at all.
			if e.detail, err = os.Readlink(path); err != nil {
				return err
			}
		case d.Type().IsRegular():
			e.kind = "file"
			info, err := d.Info()
			if err != nil {
				return err
			}
			e.exec = info.Mode().Perm()&0o100 != 0
			if e.detail, err = fileDigest(path); err != nil {
				return err
			}
		default:
			// Unreachable over a copy, since [copyTree] does not create these.
			// Skipped rather than refused for that reason, and named so the next
			// reader does not wonder whether the omission was noticed.
			return nil
		}
		entries = append(entries, e)
		return nil
	})
	if err != nil {
		return "", err
	}
	slices.SortFunc(entries, func(a, b entry) int { return strings.Compare(a.path, b.path) })

	a := addr{h: sha256.New()}
	a.str("seam-tree")
	a.num(int64(len(entries)))
	for _, e := range entries {
		a.str(e.kind)
		a.str(e.path)
		switch e.kind {
		case "file":
			a.num(b2i(e.exec))
			a.str(e.detail)
		case "link":
			a.str(e.detail)
		}
	}
	return hex.EncodeToString(a.h.Sum(nil)), nil
}

// fileDigest is one file's bytes, as lowercase hex. Streamed rather than read
// whole: a tree can hold a database the .gitignore keeps out of git but that
// [copyTree] copies anyway, and it can be any size at all.
func fileDigest(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// addr writes the canonical encoding above into a hash. Two methods, both
// length-prefixed or fixed-width, which is the whole discipline.
//
// hash.Hash documents that Write never returns an error, which is why none is
// threaded through here.
type addr struct{ h hash.Hash }

func (a *addr) str(s string) {
	a.num(int64(len(s)))
	a.h.Write([]byte(s))
}

// num writes eight bytes, big-endian. Fixed width over varint for the reason
// memory/id.go gives: there is nothing to save and one less rule to get wrong.
func (a *addr) num(n int64) {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(n))
	a.h.Write(buf[:])
}

func b2i(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

// gitAnchor is where git says this tree came from: the short sha of HEAD, and
// whether anything was uncommitted.
//
// Everything here is best-effort and silent about failure, because the anchor is
// orientation and the run does not depend on it. No repository, no git on the
// path, a detached worktree git declines to answer about — all of them give an
// empty head, which prints as "no git anchor" rather than as an error. The
// address is the identity and it does not need git's help.
//
// Both answers or neither: a head with no reliable dirty flag would read as a
// clean checkout, which is the one wrong thing this could say.
func gitAnchor(root string) (head string, dirty bool) {
	head, err := git(root, "rev-parse", "--short", "HEAD")
	if err != nil {
		return "", false
	}
	// Ignored files are not in this and are in the digest, which is the right
	// way round: a stale binary in the tree changes the address and is not
	// something git will call dirty.
	status, err := git(root, "status", "--porcelain")
	if err != nil {
		return "", false
	}
	return head, status != ""
}

// git runs one read-only command in a tree.
//
// --no-optional-locks because `git status` otherwise refreshes and rewrites
// .git/index, and this tool's standing promise is that it never writes inside
// the repository. The answer is identical; git just declines to save its work.
func git(root string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"--no-optional-locks", "-C", root}, args...)...)
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

// identify addresses the working tree and runs nothing. It is what -list uses,
// and it is the cheap way to answer "is this report about the tree I am looking
// at" — a second, against a copy, versus the minutes a whole catalog costs.
//
// It copies, because the address is defined over the copy. Addressing the
// repository directly would need a second statement of [copyTree]'s rules about
// what a tree contains, and two statements of one thing drift.
func identify(root string) (ident, error) {
	dir, err := copyTree(root)
	if err != nil {
		return ident{}, err
	}
	defer os.RemoveAll(dir)

	tree, err := digestTree(dir)
	if err != nil {
		return ident{}, err
	}
	head, dirty := gitAnchor(root)
	return ident{tree: tree, head: head, dirty: dirty}, nil
}
