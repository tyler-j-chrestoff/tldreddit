package main

import (
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"
)

// The record has a second mouth, and it is not a screen.
//
// This file and the two beside it are that second mouth: `tldr say` puts a bit on
// the record and `tldr top` prints a ranked reading of it, both without a
// terminal, so a participant who is not a person at a keyboard can take part.
// D51(e) is what asked for it — a session's handoff written *into the record*
// rather than into a markdown file, and the next session reading it back ranked
// rather than by filename — and what is built here is the general capability
// rather than that errand. Nothing below knows what a handoff is. There is no
// handoff type, no handoff channel, no handoff flag: there is a way to say
// something and a way to read what mattered, and a handoff is one thing a
// participant might say. D51(f) is the test that forced it — a feature that
// exists only because we need it is D40's trap wearing dogfooding's clothes.
//
// # Writing is open here and voting is not
//
// The asymmetry is the design rather than an omission, and it is why the verbs
// below are a table instead of a switch. A vote is a human's cheapest act and the
// only signal this product has (D4, D30). An agent that can cast one can
// manufacture the signal it will later be measured by — karma farming, which
// D39(a) withholds the vote from the persona to prevent, and which D51(d) names
// as the way this whole strategy fails quietly: a launch pitched on "ranked by a
// human's votes", shipping a record full of votes no human cast, would be
// indistinguishable from the outside from one that worked. D52(j) ruled the
// constraint before any of this existed, in those words — the skill gets Claude a
// write, never a vote.
//
// So there is no `vote` command, and its absence is checked rather than merely
// intended: [TestNoCommandOnThisSurfaceCanCastAVote] runs every entry in the
// table against a real record and asserts the vote view is untouched, so the day
// somebody adds one it goes red rather than the record filling up quietly.
//
// The same argument reaches one handle, and it is the human's own. [say] refuses
// a `-as` naming the person at the keyboard ([tui.Human]), because an utterance
// minted under that ref produces the human's judgment by a different door: it is
// the ref every ranked reading of this record is ordered *by*, and the one an
// audit of the record would be about. The write stays open to everyone else, so
// this is the write-yes/vote-no line drawn once more rather than a second rule,
// and [say]'s own guard carries the argument in full.
//
// *Until that ruling this file said the opposite* — that `say` would spell any
// handle it was given, the human's own included, and that what could not be
// forged here was a vote and not an identity. The reasoning was that a wrong
// attribution is a lie a reader can catch by reading it. It survives as the
// reason the refusal is narrow rather than as a reason to have none: what a
// reader catches is the *residue*, a `-as local2` or a display name borrowed,
// and the refusal removes only the case where there is nothing to catch.
//
// Catching it by reading is a claim about what a reader is shown, so it is paid
// for rather than asserted. [top]'s rows name the ref the record keys on and not
// only the display name somebody chose ([speaker]) — without which a near-miss
// handle and the human draw the same column on the only reading of this record
// that has no screen, and "catch it by reading it" is not available to anybody.

// command is one non-interactive verb.
type command struct {
	// args is the synopsis after the verb, for usage.
	args string

	// what is one line saying what it does, for usage.
	what string

	// run is the verb. It takes the streams so a test can hand it buffers, and
	// the path so that no command works out where the record lives for itself —
	// there is one answer to that and [recordPath] is it.
	run func(s streams, path string, args []string) error
}

// commands is every non-interactive verb this program has. It reads as a list of
// what a participant who is not at the keyboard can do to the record, which is
// the point of the paragraphs above: two entries, one of them read-only, and no
// third one that votes.
var commands = map[string]command{
	"say": {
		args: "-as <ref> [-name <display>] [text …]",
		what: "put something on the record; with no text, reads standard input",
		run:  say,
	},
	"top": {
		args: "[-n <rows>]",
		what: "read the record back, ordered by what the human voted mattered",
		run:  top,
	},
}

// streams is the program's three ends, taken as a value rather than reached for
// as globals so a command can run in a test against buffers. A command writing to
// os.Stdout directly is one whose output nothing can assert about, and on a
// program whose output *is* the product that is the wrong trade.
type streams struct {
	in  io.Reader
	out io.Writer
	err io.Writer
}

// dispatch runs one non-interactive command.
//
// Any argument at all means a command; no argument means the surface. That line
// is worth stating because the surface's own persona rests on it: tui's
// defaultPersona is written the way it is because there is nowhere on the path
// that opens a terminal to choose a model. There still is not — a flag here
// belongs to the verb it follows, never to the session — and the day one of these
// grows a `-model` is the day that comment stops being true.
func dispatch(args []string, path string, s streams) int {
	name, rest := args[0], args[1:]
	switch name {
	case "help", "-h", "-help", "--help":
		usage(s.out)
		return 0
	}

	c, ok := commands[name]
	if !ok {
		fmt.Fprintf(s.err, "tldr: no command %q\n", name)
		usage(s.err)
		return 1
	}
	if err := c.run(s, path, rest); err != nil {
		fmt.Fprintf(s.err, "tldr %s: %v\n", name, err)
		return 1
	}
	return 0
}

// usage prints what this program does, in the order a person meets it: the
// terminal client first, because that is what tldr is, and the verbs after it.
func usage(w io.Writer) {
	var b strings.Builder
	b.WriteString("tldr — a forum-shaped memory you can watch think\n\n")
	b.WriteString("  tldr\n      open the conversation\n")
	for _, name := range slices.Sorted(maps.Keys(commands)) {
		c := commands[name]
		fmt.Fprintf(&b, "  tldr %s %s\n      %s\n", name, c.args, c.what)
	}
	b.WriteString("\nThe record is $TLDR_RECORD, or $XDG_STATE_HOME/tldreddit/record,\n")
	b.WriteString("or ~/.local/state/tldreddit/record — whichever is set, in that order.\n")
	fmt.Fprint(w, b.String())
}
