package main

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// claim is one row of the catalog: a break to make, and the checks that have to
// notice.
//
// Every field comes from a seam block in docs/CLAIMS.md and nothing is inferred,
// because a default the catalog does not spell is a claim nobody wrote down.
type claim struct {
	// id names the claim on the command line and in the report. Unique across
	// the catalog; a duplicate is refused rather than resolved, since the two
	// blocks would then be arguing about which one -run means.
	id string

	// title is the heading the block sits under, carried so the report can say
	// which claim in human words rather than only which identifier.
	title string

	// line is where the block starts in docs/CLAIMS.md, so a parse error and a
	// stale anchor both point at somewhere a person can edit.
	line int

	// file is the source the mutation lands in, relative to the repo root.
	file string

	// find is an exact substring, and occ says which occurrence when the anchor
	// is not unique. Exact rather than a pattern on purpose: an anchor that
	// still matches after the code moved under it is how a mutation ends up
	// breaking something other than what the claim is about.
	find string
	occ  int

	// occSet records that the block chose the occurrence rather than inheriting
	// the default. An anchor matching twice in a block that never chose is
	// refused; see [occurrences].
	occSet bool

	// after is what replaces it. Empty is legal and means deletion, which is why
	// the key is required even when the value is not — a missing key would make
	// a typo silently delete.
	after string

	// red names the checks that must fail. This is the whole assertion: a claim
	// citing a check is a claim that the check fails when the behaviour is
	// broken, and everything else here is bookkeeping around that sentence.
	red []string

	// sole says no check outside red may fail. It is how "this check is the only
	// one that catches this" stops being prose.
	sole bool

	// among narrows sole to a named set. A table comparing three checks against
	// each other claims nothing about the rest of the suite, and judging it
	// against the whole suite would report a failure the table never made.
	among []string

	// race runs the suite under the detector. For a claim whose oracle is the
	// detector rather than an assertion, that is the only run where the cited
	// check can fail at all.
	race bool

	// declared is the set of verdicts this claim says it may come back as, and it
	// is what the gate compares against. Defaults to proven alone.
	//
	// A set rather than one verdict, because some claims are honestly
	// nondeterministic and a gate may not be flaky by construction. The store's
	// unlocked reader is the case: the runtime's throw usually kills that check
	// before its own assertion runs, and occasionally does not, so its honest
	// answer is two verdicts and neither is a surprise. Gating on the set is
	// deterministic where gating on either member alone would flap.
	//
	// It is not a way to opt out. Every verdict outside the set still fails, in
	// either direction, so a claim that stops reddening at all — the thing a
	// suppression would hide — trips exactly as loudly as before. A set must be
	// exhaustive, and the block must show the measurement justifying every member:
	// a two-member set is a claim about nondeterminism and carries the same burden
	// of proof as any other claim in that file. docs/CLAIMS.md carries the
	// argument; this is the sentence and the pointer.
	declared []verdict

	// isolate runs each cited check in a test binary of its own.
	//
	// It exists for one shape of claim and should not spread past it: a mutation
	// whose damage kills the process takes every check after it off the run, so
	// a claim about several checks in one package cannot be seen whole however
	// many times it runs. The store's own doc comment already prescribes exactly
	// this discipline for exactly this mutation. What is given up is stated in
	// the report rather than buried — those checks were observed alone, and a
	// check that only reddens when nothing else is running is a weaker fact than
	// one that reddens in the suite.
	isolate bool

	// runs is how many samples are taken, of the mutated tree and of the
	// unmutated one alike.
	//
	// A red is not evidence without a green control. An interleaving is a
	// property of a run rather than of a program, so a check that reddens under
	// the mutation has shown nothing until the same check, sampled the same
	// number of times on the same tree unmutated, stayed green throughout. Both
	// rates are reported; neither is averaged, because averaging a
	// nondeterministic result is how a real finding gets rounded away.
	//
	// How large a sample a claim needs is that claim's own business, argued in
	// its block beside the command the rate was measured by. No figure here: a
	// rate cited away from the command that produced it is a different experiment
	// wearing the same number, which this catalog has already done once.
	runs int
}

// parseClaims reads the catalog out of docs/CLAIMS.md.
//
// The file is the catalog. Words' own version keeps the prose and the table of
// mutants in two places joined by a string, and the two drift the way any two
// statements of one thing drift; here the block a person reads the prose beside
// is the block the tool executes.
//
// Everything it will not understand is an error rather than a default: an
// unknown key, a missing required key, an occurrence that is not a number, an
// escape it does not know. A catalog that parses loosely is a catalog whose
// entries can stop meaning what they say without anything failing.
func parseClaims(r io.Reader) ([]claim, error) {
	var (
		out   []claim
		title string
		in    bool
		block []string
		start int
	)

	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for n := 1; sc.Scan(); n++ {
		line := sc.Text()

		if !in {
			if h, ok := strings.CutPrefix(line, "## "); ok {
				title = strings.TrimSpace(h)
			}
			if strings.TrimSpace(line) == "```seam" {
				in, block, start = true, nil, n
			}
			continue
		}

		if strings.TrimSpace(line) == "```" {
			c, err := parseBlock(block, title, start)
			if err != nil {
				return nil, fmt.Errorf("docs/CLAIMS.md:%d: %w", start, err)
			}
			out = append(out, c)
			in = false
			continue
		}
		block = append(block, line)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if in {
		return nil, fmt.Errorf("docs/CLAIMS.md:%d: seam block is never closed", start)
	}

	seen := map[string]int{}
	for _, c := range out {
		if at, dup := seen[c.id]; dup {
			return nil, fmt.Errorf("docs/CLAIMS.md:%d: id %q was already used at line %d", c.line, c.id, at)
		}
		seen[c.id] = c.line
	}
	return out, nil
}

func parseBlock(lines []string, title string, start int) (claim, error) {
	c := claim{title: title, line: start, occ: 1, runs: 1, declared: []verdict{proven}}

	got := map[string]bool{}
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		key, raw, ok := strings.Cut(line, ":")
		if !ok {
			return c, fmt.Errorf("%q is not key: value", line)
		}
		key = strings.TrimSpace(key)
		if got[key] {
			return c, fmt.Errorf("%s is given twice", key)
		}
		got[key] = true

		// Only the leading space after the colon is eaten. A trailing space is
		// part of an anchor as often as it is a typo, and this is a file of
		// exact substrings.
		val, err := unescape(strings.TrimPrefix(raw, " "))
		if err != nil {
			return c, fmt.Errorf("%s: %w", key, err)
		}

		switch key {
		case "id":
			c.id = strings.TrimSpace(val)
		case "file":
			c.file = strings.TrimSpace(val)
		case "find":
			c.find = val
		case "after":
			c.after = val
		case "occ":
			if c.occ, err = strconv.Atoi(strings.TrimSpace(val)); err != nil || c.occ < 1 {
				return c, fmt.Errorf("occ: %q is not an occurrence (1 or more)", val)
			}
			c.occSet = true
		case "runs":
			if c.runs, err = strconv.Atoi(strings.TrimSpace(val)); err != nil || c.runs < 1 {
				return c, fmt.Errorf("runs: %q is not a count (1 or more)", val)
			}
		case "red":
			c.red = names(val)
		case "among":
			c.among = names(val)
		case "sole":
			if c.sole, err = strconv.ParseBool(strings.TrimSpace(val)); err != nil {
				return c, fmt.Errorf("sole: %q is not true or false", val)
			}
		case "race":
			if c.race, err = strconv.ParseBool(strings.TrimSpace(val)); err != nil {
				return c, fmt.Errorf("race: %q is not true or false", val)
			}
		case "verdict":
			c.declared = nil
			for f := range strings.SplitSeq(val, "|") {
				v := verdict(strings.TrimSpace(f))
				if why[v] == "" {
					return c, fmt.Errorf("verdict: %q is not a verdict this tool reports", f)
				}
				c.declared = append(c.declared, v)
			}
		case "isolate":
			if c.isolate, err = strconv.ParseBool(strings.TrimSpace(val)); err != nil {
				return c, fmt.Errorf("isolate: %q is not true or false", val)
			}
		default:
			return c, fmt.Errorf("%q is not a key this tool knows", key)
		}
	}

	for key, have := range map[string]bool{
		"id":    c.id != "",
		"file":  c.file != "",
		"find":  c.find != "",
		"after": got["after"],
		"red":   len(c.red) > 0,
	} {
		if !have {
			return c, fmt.Errorf("%s is required", key)
		}
	}
	if len(c.among) > 0 && !c.sole {
		return c, fmt.Errorf("among narrows sole, and sole is not set")
	}
	return c, nil
}

// names splits a comma-separated list of test names.
func names(s string) []string {
	var out []string
	for f := range strings.SplitSeq(s, ",") {
		if f = strings.TrimSpace(f); f != "" {
			out = append(out, f)
		}
	}
	return out
}

// unescape reads the three escapes a one-line value may carry.
//
// A multi-line anchor has to be expressible — the cleanest way to unlock a store
// is to replace a field declaration and the lines around it — and a fenced block
// of key: value lines has no other way to say so. An escape this does not know
// is an error rather than a literal backslash: a value in this file is spliced
// into source, and the one thing it may not do is quietly mean something other
// than what was written.
func unescape(s string) (string, error) {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' {
			b.WriteByte(s[i])
			continue
		}
		if i+1 >= len(s) {
			return "", fmt.Errorf("a backslash ends the value")
		}
		i++
		switch s[i] {
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case '\\':
			b.WriteByte('\\')
		default:
			return "", fmt.Errorf("\\%c is not an escape this tool knows (\\n, \\t, \\\\)", s[i])
		}
	}
	return b.String(), nil
}
