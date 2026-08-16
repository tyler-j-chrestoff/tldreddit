// Command tldr is the tldreddit terminal client.
package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/tyler-j-chrestoff/tldreddit/tui"
)

func main() {
	os.Exit(run(os.Args[1:], streams{in: os.Stdin, out: os.Stdout, err: os.Stderr}))
}

// run is main with a return value, because main cannot be called and this can.
//
// No arguments is the conversation; anything else is one of the verbs in cli.go,
// which never opens a terminal. That split is the whole of the dispatch and it is
// deliberately not a flag: see [dispatch] for what rests on the session itself
// taking none.
func run(args []string, s streams) int {
	path, err := recordPath()
	if err != nil {
		fmt.Fprintf(s.err, "tldr: %v\n", err)
		return 1
	}
	if len(args) > 0 {
		return dispatch(args, path, s)
	}
	return session(path, s)
}

// session is the terminal client: the record loaded, the surface run over it, and
// the file kept level with what the surface holds.
//
// Two non-zero codes, and they are two different sentences: 1 is a session that
// did not start or did not run, and 2 is a session whose record on disk may be
// behind what the session held. The second is the worse one and wins when both
// are true.
//
// 2 used to mean the stronger thing — the conversation is gone — and it stopped
// meaning that when the record began being written after every change rather
// than at the end. What is at risk on that path now is at most the last change,
// so the message says that rather than the old sentence, which would now be an
// alarm about a file that is very likely current.
//
// They are only visible through a built binary. `go run` reports every non-zero
// exit as 1 and prints the real one to stderr as "exit status 2" (D50(f),
// measured against go1.25.4).
func session(path string, s streams) int {
	// Nothing starts on a record that did not load. A surface over half a record
	// is the failure this program is about — see cmd/tldr's record.go, which says
	// which half and what it costs.
	rec, err := load(path)
	if err != nil {
		fmt.Fprintf(s.err, "tldr: %v\n", err)
		fmt.Fprintf(s.err, "tldr: nothing was started and nothing was written\n")
		return 1
	}

	final, runErr := tea.NewProgram(tui.Load(rec.store, rec.shown, rec.votes, rec.checkpoint(path))).Run()
	if runErr != nil {
		fmt.Fprintf(s.err, "tldr: %v\n", runErr)
	}

	// The store is the pointer this function has held all along, so every bit the
	// session wrote is already in it. The views are values the Model owns, and the
	// last ones exist only here, in what Run handed back.
	//
	// Bubble Tea returns the initial model on a startup failure and nil only when
	// it recovered a panic (verified against bubbletea v2.0.8, tea.go:991-1174,
	// not remembered), so a failed assertion is the panic case, and after a panic
	// the surface's own state is the last thing to trust.
	//
	// What that costs is now one change rather than a session. The hook above has
	// been writing the file after every change all along, so the record on disk is
	// current as of the last change that reached it — and the only gap is a change
	// whose own save did not finish, which is exactly what the panic could have
	// interrupted. Saying otherwise would be an alarm about a file that is
	// probably complete, on the one surface that may not overstate what it knows.
	m, ok := final.(tui.Model)
	if !ok {
		fmt.Fprintf(s.err,
			"tldr: the program ended without handing its surface back; %s holds the conversation"+
				" as of the last change that got written, which may be one short\n", path)
		return 2
	}

	// One last save, and it is the last rather than the only one. Ordinarily it
	// rewrites bytes that are already there — but a save that failed mid-session
	// leaves the file behind and the session running, deliberately (see
	// tui.Save), so this is where a disk that came back gets its chance. It is
	// also the only save on the path where Run never started: without a terminal
	// Bubble Tea hands back the model it was given, and what that writes is what
	// was loaded.
	rec.shown, rec.votes = m.Views()
	if err := rec.save(path); err != nil {
		fmt.Fprintf(s.err, "tldr: %v\n", err)
		return 2
	}
	if runErr != nil {
		return 1
	}
	return 0
}
