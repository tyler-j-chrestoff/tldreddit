package main

import (
	"cmp"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/tyler-j-chrestoff/tldreddit/memory"
	"github.com/tyler-j-chrestoff/tldreddit/tui"
)

// say puts one bit on the record, from a participant who is not at the keyboard.
//
// It is [tui.Model.utter] with the terminal taken away, and deliberately nothing
// more: the same channel, the same view, the same edge back to what it follows.
// A bit written here is an ordinary bit — a reader cannot tell it from something
// typed at the surface, and that is the property being bought. The moment a bit
// written by an agent carried a mark saying so, every screen and every fold would
// have to decide what to do about the mark, and the forum would have two kinds of
// participant in it (charter: "agents and subagents each hold their own
// forum-memory, and those nest").
//
// One handle is refused, and it is the person at the keyboard's own — the
// paragraph at the guard says why, and it is the same argument as the missing
// vote verb rather than a second one.
//
// # Three things it has to get right, none of them obvious
//
// **The channel is [tui.Channel] and not a name of this command's own.**
// [memory.Cool] panics on a window spanning two channels, and the bit goes into
// the same view the surface folds — so a private channel here is not a display
// nicety, it is a crash in the next fold of a session somebody is in the middle
// of. Asked for rather than spelled, so there is one statement of it.
//
// **It goes into the view, not only into the store.** The edges run backwards,
// so a new bit pointing at the view's head is not discoverable *from* the view,
// and D14 is explicit that reachable means discoverable rather than merely
// retrievable. A bit in no view is still *enumerable* — `tldr top` walks the
// whole store — and enumerable is a third thing D14 does not count: an
// enumeration hands a reader every bit and no starting point, so it cannot say
// which of them anybody was meant to begin from. In the view
// it is the newest row, which is where the caret lands when the next session
// opens the record ([tui.Load]), so what the last session said is the first
// thing on screen and is drawn whole (D54(b)).
//
// **It takes the record's own present and does not compete with it.** At is a
// wall clock here because this is a real occurrence and there is nothing else to
// stamp it with, which is the one place this program reads a clock that
// [memory.Cool] and [memory.Stay] refuse to.
//
// # What a session beside this one costs, and what it does not
//
// A bit said here while `tldr` is running does not appear in that session's
// transcript. It cannot: the view on screen is a value that session holds, and
// nothing reaches into a running process. `tldr top` sees it immediately, and
// that is D1's own division — **a view is allowed to forget; the record is not.**
//
// The next session sees it too, and for a while it did not. Two things had to be
// true for that and only one of them was. [record.absorb] keeps the session's
// next checkpoint from overwriting the bit outright, which is the record; but
// that checkpoint still writes the session's own view, so the bit was left in no
// transcript at all and the next session never drew it — while the identical
// command run with no session open put it on that session's first screen. One
// command, two outcomes, chosen by whether a terminal was open elsewhere.
// [record.rejoin] is what makes the two agree, at load, and it also says why the
// merge cannot live on the save path.
//
// What is left of the two-writer hazard is a window of milliseconds rather than a
// session, and [record.save] says exactly how wide and why it is not worth a
// lock.
func say(s streams, path string, args []string) error {
	fs := flag.NewFlagSet("say", flag.ContinueOnError)
	fs.SetOutput(s.err)
	as := fs.String("as", "", "`ref` of whoever is saying it: stable, and how the record names them")
	name := fs.String("name", "", "`display` name to record beside the ref, when they differ")
	if err := fs.Parse(args); err != nil {
		return err
	}

	// Required rather than defaulted, because every default available is a lie
	// somebody would have to unpick later: a hostname is the machine, a username
	// is whoever started the process, and either one recorded as the speaker
	// makes the record's central question — who said this — unanswerable in
	// exactly the case it is being asked.
	if *as == "" {
		return errors.New("say who: -as <ref>, the handle to record this under")
	}

	// And the human's own ref is refused, which is the write-yes/vote-no asymmetry
	// in [cli] one step removed rather than a second rule.
	//
	// This program already says a machine may produce material and may not produce
	// the human's judgment. Minting an utterance under the human's handle produces
	// the human's judgment by a different door: that ref is the one [memory.View.Rank]
	// takes as `by`, so it decides what every reading of this record is ordered by,
	// and it is the one an audit of this record would be *about*. A record where any
	// local process can speak as the person it is meant to be evidence for is not a
	// provable record, whatever else it is.
	//
	// # What this is not, said here so nobody builds on it as if it were
	//
	// **It stops this ref and no lookalike.** `-as local2`, or `-as locaI` with a
	// capital i, still draws something a reader could take for the human at a
	// glance. Deliberately not closed: a similarity test is a guess about intent,
	// and the refs this program most wants — an agent naming itself after the
	// session it is — are exactly the ones such a guess would refuse. The reading
	// side is where that residue is handled, by printing the ref rather than
	// judging it ([speaker]).
	//
	// **It is not a security boundary and cannot become one.** Anyone who can run
	// this command can write the record file directly, and nothing here
	// authenticates anybody. What it settles is what *this program* will do on its
	// own behalf — the same standing the absent vote verb has, and worth exactly as
	// much: it makes the forgery a deliberate act with a different tool rather than
	// a flag somebody passes without thinking.
	//
	// The ref comes from [tui.Human] rather than a "local" literal here. A second
	// statement of who the human is would be a thing to keep in step, and this
	// repository logs what happens when two copies of one fact drift.
	if me := tui.Human(); *as == me.Ref {
		return fmt.Errorf("%q is the handle this program writes for the person at the keyboard, "+
			"and nothing else may say anything under it; pick a ref that names the agent "+
			"speaking — -as session-15, -as an-agent", me.Ref)
	}

	text, err := words(s, fs.Args())
	if err != nil {
		return err
	}

	rec, err := load(path)
	if err != nil {
		return err
	}

	var b memory.Bit
	rec.shown, b = rec.shown.Add(rec.store, memory.Bit{
		At:      time.Now(),
		From:    memory.Handle{Ref: *as, Display: cmp.Or(*name, *as)},
		Channel: tui.Channel(),
		Payload: memory.Utterance{Text: text},
		Prev:    rec.shown.Head(),
	})
	if err := rec.save(path); err != nil {
		return err
	}

	// The address, alone, on standard output. It is the one thing here a caller
	// might want to keep rather than read: a bit's address is how anything else
	// ever refers to it, so it goes out whole and unadorned, and every word meant
	// for a person goes to standard error.
	fmt.Fprintln(s.out, b.ID)
	fmt.Fprintf(s.err, "tldr: recorded as %s, %d %s on the record at %s\n",
		speaker(b.From), rec.store.Len(), plural(rec.store.Len(), "bit"), path)
	return nil
}

// words is what to record: the arguments if there are any, and otherwise
// everything on standard input.
//
// Both, because they are two different acts. A line as arguments is somebody
// typing a thought at a shell, where the shell has already done the quoting and
// joining the arguments with spaces is what they meant. Anything with a shape to
// it — a handoff, a diff, a paragraph with a blank line in it — arrives on stdin
// and arrives exactly as written, line breaks included. That is worth noting
// against the surface, which collapses them: the transcript draws a message as
// one flowed sentence (`tui/render.go`'s saidWhole), so a structured message read
// back through `top` keeps a shape the screen never shows.
//
// Empty is refused rather than recorded. An empty utterance is a permanent bit
// saying nothing, in a record with no delete, and the likeliest way to write one
// is a pipe that produced no output — a failure upstream, arriving here as a
// participant who said nothing at all.
//
// A person who typed `tldr say -as me` and stopped is met by a line saying what
// is happening, because otherwise the most likely first invocation anybody types
// is a program that appears to have hung. It is a line and not a refusal: typing
// a paragraph at a terminal and ending it with ctrl-d is a real way to use this,
// and the only way to say something with a blank line in it by hand.
func words(s streams, args []string) (string, error) {
	text := strings.Join(args, " ")
	if len(args) == 0 {
		if atATerminal(s.in) {
			fmt.Fprintln(s.err, "tldr say: reading what to say from standard input — "+
				"end it with ctrl-d, or give it as arguments instead")
		}

		read, err := io.ReadAll(s.in)
		if err != nil {
			return "", fmt.Errorf("reading what to say: %w", err)
		}
		text = string(read)
	}

	if text = strings.TrimSpace(text); text == "" {
		return "", errors.New("nothing to say: give the text as arguments, or on standard input")
	}
	return text, nil
}

// atATerminal reports whether reading this would block on a person rather than
// on a pipe or a file.
//
// Through an interface assertion rather than a parameter, so that [streams] stays
// a reader and two writers and a test keeps handing it a bytes.Reader — which
// answers false, correctly, because nothing is waiting on a keyboard there.
//
// os.ModeCharDevice is the standard library's whole answer to this question and
// it is coarser than a terminal: /dev/null is a character device too, so `tldr
// say -as me < /dev/null` prints a line about standard input and then refuses for
// having read nothing. That is one stray sentence on the way to a correct
// refusal, and it is cheaper than a dependency for asking the tty properly.
func atATerminal(r io.Reader) bool {
	f, ok := r.(interface{ Stat() (os.FileInfo, error) })
	if !ok {
		return false
	}
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
