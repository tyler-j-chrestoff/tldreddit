package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// entries is docs/DECISIONS.md read as what it is: a list of entries, each cut
// into lettered clauses.
//
// A citation names a clause and not an entry, because entries here run to
// thousands of words and the same sentence appears in two of them as a matter
// of course — D67(i)'s wrong figure is quoted verbatim inside D68(a), which is
// the correction. A sentence that occurs once in a clause occurs twice in the
// log, and a tool that searched the log would resolve half of these citations to
// the entry that says the opposite.
type entries map[string]string

var (
	heading = regexp.MustCompile(`^## (D[0-9]+) —`)
	marker  = regexp.MustCompile(`\*\*\(([a-z])\)`)
	spec    = regexp.MustCompile(`^(D[0-9]+)(?:\(([a-z])\))?$`)
)

func readEntries(path string) (entries, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	out := entries{}
	name, body := "", []string{}
	flush := func() {
		if name != "" {
			out[name] = flatten(strings.Join(body, "\n"))
		}
	}
	for _, line := range strings.Split(string(b), "\n") {
		if m := heading.FindStringSubmatch(line); m != nil {
			flush()
			name, body = m[1], nil
			continue
		}
		body = append(body, line)
	}
	flush()
	return out, nil
}

// clause returns the text a citation points at, spelled D68(a) or D68.
func (e entries) clause(at string) (string, error) {
	m := spec.FindStringSubmatch(at)
	if m == nil {
		return "", fmt.Errorf("%q is not an entry; spell it D68 or D68(a)", at)
	}
	body, ok := e[m[1]]
	if !ok {
		return "", fmt.Errorf("the record has no %s", m[1])
	}
	if m[2] == "" {
		return body, nil
	}

	// Clause boundaries are the bold markers the entries already carry. Only a
	// marker counts, so a mention of D67(a) inside another clause's prose does
	// not open a clause — which is the difference between finding the sentence
	// where it was written and finding it where it was cited.
	start, want := -1, "**("+m[2]+")"
	all := marker.FindAllStringIndex(body, -1)
	for _, s := range all {
		if strings.HasPrefix(body[s[0]:], want) {
			start = s[0]
			break
		}
	}
	if start < 0 {
		return "", fmt.Errorf("%s has no clause (%s)", m[1], m[2])
	}
	for _, s := range all {
		if s[0] > start {
			return body[start:s[0]], nil
		}
	}
	return body[start:], nil
}
