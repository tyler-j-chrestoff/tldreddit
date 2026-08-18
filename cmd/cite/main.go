// Command cite re-derives the countable claims this record makes about texts it
// did not write.
//
// docs/CITATIONS.md holds them: a sentence quoted from docs/DECISIONS.md, the
// source it is about named by the SHA-256 of its bytes, and a count that
// settles it. This runs them and prints what the source actually says.
//
// It exists because of D68. Four figures in D67 were wrong; all four were found
// at the publication gate, days after the entry was permanent; and D68(f)
// recorded that nothing checks a prose entry's numbers before it is committed.
// The pre-commit gate reads code and cmd/seam reads claims about code. A count
// in an entry was checked by nobody.
//
// It is not cmd/seam and should not become it. That tool's oracle is a test —
// break the behaviour, assert the check reddens — and it samples a control on
// both sides of every mutation, which is why it takes seventeen minutes and
// cannot live in the commit hook. This tool's oracle is arithmetic. It takes
// milliseconds, which is the whole reason it can run at the moment D68(f) names.
//
// What keeps it from being a second copy of a number: every block quotes the
// entry's own sentence, and the figure it expects has to appear in that
// sentence. So a disagreement cannot be silenced by editing the block — change
// the expectation and the sentence no longer states it, and the block fails a
// different way. The only thing that makes a block agree is a record that is
// right, which in an append-only log means appending a correction and pointing
// the citation at it.
//
// What it cannot reach is in docs/CITATIONS.md rather than here, because the
// limits are the reader's business and this is the mechanism. The short form:
// it does not check that the counted string is what the sentence means, it does
// not check inference, and for a quotation it catches the truncation and never
// the misreading.
//
// Usage:
//
//	go run ./cmd/cite             # every citation
//	go run ./cmd/cite -list       # the catalog and the sources, running nothing
//	go run ./cmd/cite -run <id>   # one citation
//	go run ./cmd/cite -sources    # the manifest only, for a machine with no cache
//
// Exit status: 0 when every citation is where its own block says it will be, 2
// when one is not. 1 is the tool failing to run at all.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func main() {
	list := flag.Bool("list", false, "print the catalog and the sources, run nothing")
	only := flag.String("run", "", "run one citation by id")
	manifest := flag.Bool("sources", false, "check the manifest only, without reading any source")
	flag.Parse()

	code, err := run(os.Stdout, *list, *only, *manifest)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cite: %v\n", err)
		os.Exit(1)
	}
	os.Exit(code)
}

// run returns the exit status rather than taking it, so the whole tool is
// reachable from a test. cmd/seam's own exit vocabulary is invisible under
// `go run`, which flattens it to 1 — the same is true here, and anything that
// wants to branch on the code has to build the binary first.
func run(out io.Writer, list bool, only string, manifest bool) (int, error) {
	root, err := repoRoot()
	if err != nil {
		return 1, err
	}

	cat, err := readCatalog(filepath.Join(root, "docs", "CITATIONS.md"))
	if err != nil {
		return 1, err
	}
	if only != "" {
		if cat, err = cat.one(only); err != nil {
			return 1, err
		}
	}

	if list {
		printCatalog(out, cat)
		return 0, nil
	}
	if manifest {
		if printManifest(out, cat) > 0 {
			return 2, nil
		}
		return 0, nil
	}

	log, err := readEntries(filepath.Join(root, "docs", "DECISIONS.md"))
	if err != nil {
		return 1, err
	}
	cache, err := sourceCache()
	if err != nil {
		return 1, err
	}

	var results []result
	for _, c := range cat.cites {
		results = append(results, check(c, cat.sources, cache, log))
	}
	if printReport(out, cache, results) > 0 {
		return 2, nil
	}
	return 0, nil
}

// repoRoot walks up from the working directory to the go.mod, so the tool finds
// its catalog from anywhere inside the tree and refuses outright from outside
// one. Which tree it reads is decided by where you stand, which matters here
// because there are two of them.
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
			return "", fmt.Errorf("no go.mod above %s; run this inside the repository", dir)
		}
		dir = up
	}
}

// sourceCache locates the directory holding the extracted texts.
//
// The sources are somebody else's papers and they are not in this repository —
// docs/CITATIONS.md argues that at length; the short version is that the public
// tree is append-only from its root and vendoring an unlicensed 1.7 MB of
// third-party text into it is a door that does not open again.
//
// Each variable is tested for emptiness rather than presence. filepath.Join
// drops an empty element, so an XDG variable that is set and empty yields a
// relative path — which does not fail, it succeeds against whatever directory
// the program happened to start in.
func sourceCache() (string, error) {
	if dir := os.Getenv("TLDR_SOURCES"); dir != "" {
		return dir, nil
	}
	if dir := os.Getenv("XDG_CACHE_HOME"); dir != "" {
		return filepath.Join(dir, "tldreddit", "sources"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("no TLDR_SOURCES, no XDG_CACHE_HOME, and no home directory: %w", err)
	}
	return filepath.Join(home, ".cache", "tldreddit", "sources"), nil
}
