// Stranding, counted rather than entailed.
//
// A held row is stranded when the bit it names through Prev is no longer on
// screen. [memory.View]'s sparing rule was published with a figure for how often
// that happens — 93.5% of frames before the rule, 0.0% at every vote rate after
// — and nothing in this tree could produce either number: the harness that made
// them was thrown away, and the simulator that survived returns rows, folds and
// votes and has never counted a strand.
//
// Worse than missing. The schedule the surviving simulator runs is one where the
// defect cannot occur. It casts every upvote on the bit just added, whose Prev is
// by construction the head of the view it was added to — so the one case sparing
// cannot reach, a parent that left the view before the vote was cast, is
// unreachable by the experiment rather than absent from the record. A 0.0% taken
// that way is entailed by its own schedule.
//
// So this file adds the axis that case needs ([schedule].back, where the vote
// lands), turns the two numbers the old simulator held constant into parameters
// (the budget and the keep, two of the five axes D36 wants swept), counts a
// strand three ways, and freezes the whole sweep into testdata/stranding.txt so
// that a later session diffs a figure instead of quoting one:
//
//	go test ./tui/ -run TestTheStrandingSweep -update
//
// # Why this one is not behind HARNESS
//
// Everything in harness_test.go is skipped unless HARNESS is set, and the line
// is not "expensive" — it is what a failure means. A frame dump's output is
// taste, so a diff of it is an argument about taste and belongs in front of a
// person. A row of this table is a count that either reproduces or does not, and
// nobody has to have an opinion about it. That makes it a test, and a test that
// only runs when somebody remembers is how one stranding figure came to be
// published in four places with nothing in the tree able to produce it.
//
// The price is real and is named here rather than discovered: the sweep is
// sensitive to the whole fold rule, so a mutation anywhere in it moves this
// table. That is the cost of the reconciliation being automatic, and it is the
// bill that went unpaid when the figures were first written down.
package tui

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tyler-j-chrestoff/tldreddit/memory"
)

// rewrite is -update: freeze the sweep as it currently runs, rather than compare
// against what was frozen. Named for what it does to the file, because the flag
// is spelled the way every Go golden test spells it and the variable is the only
// place the direction can be said.
var rewrite = flag.Bool("update", false, "rewrite tui/testdata/stranding.txt from this run")

// golden is where the frozen sweep lives.
const golden = "testdata/stranding.txt"

// schedule is one run of the simulator. Every field decides what the record
// does; none of them decides how it is reported.
type schedule struct {
	// bits is how many utterances the conversation runs to, alternating between
	// the human and a model.
	bits int

	// rate is one upvote every rate bits. Zero is nobody voting, which is the
	// control every stranding figure needs beside it: a fold strands rows all by
	// itself, and a rate with no zero beside it cannot say which of the two a
	// number is about.
	rate int

	// back is how many said rows above the newest the upvote lands on, and it is
	// the axis this file exists for. Zero is the bit just added — the schedule
	// every published stranding figure was taken at, and the one schedule where
	// a held bit's parent is always still on screen when the vote lands.
	//
	// Counted over said bits with scars skipped rather than over rows. An upvote
	// on a scar is a different mechanism with its own guard — [memory] spares
	// only the scar itself, never the generation it names — and a back that
	// sometimes landed on one would put two mechanisms in one column.
	//
	// A vote is not cast at all when the view does not reach back that far,
	// which is what [outcome].wanted counts against [outcome].votes. Silently
	// clamping to the top row would turn a deep back into back zero and report a
	// schedule nobody asked for as though it had run.
	back int

	// budget is the fold trigger's threshold: [Model.budget], the height of the
	// screen, floored at [coolFloor].
	budget int

	// keep is how many bits stay hot through a fold, and zero means the
	// program's own rule — [keepFrom], which moves the cut back to the last
	// thing the human said. A number here is a claim about a program nobody is
	// running, and it is here anyway because the keep is one of the two axes the
	// old simulator held constant, so nothing has ever measured what moving it
	// does.
	keep int

	cadence time.Duration

	// hold is [memory.Stay].For. An axis rather than a constant because the two
	// regimes disagree completely and the record already knows it: a hold that
	// decays inside the conversation lets a spared pair cool together when it
	// lapses, and one that outlives the run never lets go of anything.
	hold time.Duration
}

// outcome is what one schedule produced. Counts, never rates: a rate is one
// division away and a count is what another instrument can be compared against.
type outcome struct {
	// frames is the denominator, and it is here rather than assumed from
	// [schedule].bits so that a row of this table carries its own division.
	frames int

	// wanted is how many times the vote schedule came round; votes is how many
	// of those found a bit to land on. They differ exactly when back reaches
	// past the top of the view, and that is the difference between a schedule
	// that stranded nothing and one that never voted.
	wanted, votes int

	folds int

	// worst is the longest the view ever got and rows is every frame's length
	// added up, so that mean view length is a division rather than a second
	// accumulator. Both, never one: D36 is the entry about a row figure that was
	// unsound because it was quoted alone.
	worst, rows int

	// any, held and noscar are frames containing at least one strand of each
	// kind; see [strand] for what each kind is. peak is the most held strands
	// standing in any single frame, which separates one row stranded for three
	// hundred frames from three hundred rows stranded once.
	any, held, noscar, peak int
}

// simulate runs a conversation at a fixed cadence with the human upvoting one
// bit in every rate, and reports what the view did.
//
// It drives the program's own trigger through [foldable] and the program's own
// fold through [memory.View.Fold], because the number this produces is a claim
// about what this program does — a fixture restating either would be measuring a
// program nobody is running (D36). The keep is the third of those and used not
// to be: the old simulator cut at half the floor, where the program cuts at
// [keepFrom], so every figure taken through it was about a cut this surface does
// not make. That is now the default and a fixed keep is the deliberate exception.
//
// Votes are cast with the bit's own instant, exactly as [Model.vote] casts them
// with the moment the key was pressed, and a hold decays against
// [memory.View.Latest] — so what ages a hold here is the conversation moving on,
// never the wall clock this test is running against.
func simulate(sc schedule) outcome {
	s := memory.NewStore()
	var view, ballots memory.View
	out := outcome{}

	stay := func() memory.Stay {
		return memory.Stay{Votes: ballots, By: localHandle, For: sc.hold}
	}

	at := time.Date(2026, 8, 12, 9, 0, 0, 0, time.UTC)
	handles := []memory.Handle{localHandle, {Ref: "ollama/qwen3.5", Display: "qwen3"}}

	for i := range sc.bits {
		view, _ = view.Add(s, memory.Bit{
			At:      at,
			From:    handles[i%len(handles)],
			Channel: channel,
			Payload: memory.Utterance{Text: lines[i%len(lines)]},
			Prev:    view.Head(),
		})

		if sc.rate > 0 && i%sc.rate == 0 {
			out.wanted++
			if target, ok := nthSaid(s, view, sc.back); ok {
				ballots, _ = ballots.Add(s, memory.Cast(at, localHandle, memory.Up, target))
				out.votes++
			}
		}

		if foldable(s, view, stay()) > sc.budget {
			keep := sc.keep
			if keep == 0 {
				keep = keepFrom(view.Bits(s), sc.budget/2, sc.budget)
			}
			if next, ok := view.Fold(s, keep, stay()); ok {
				view, out.folds = next, out.folds+1
			}
		}

		any, held, noscar := strand(s, view, stay())
		out.frames++
		out.worst = max(out.worst, len(view))
		out.rows += len(view)
		out.peak = max(out.peak, held)
		for _, hit := range [][2]*int{{&any, &out.any}, {&held, &out.held}, {&noscar, &out.noscar}} {
			if *hit[0] > 0 {
				*hit[1]++
			}
		}

		at = at.Add(sc.cadence)
	}
	return out
}

// nthSaid is the bit n said rows above the newest one in v, scars skipped, and
// whether the view reaches back that far.
//
// The caret this stands in for moves over rows and a scar is a row, so counting
// over said bits is a simplification and is the point of it: [schedule].back
// exists to sweep where a vote on an utterance lands, and a column that
// sometimes voted on a scar instead would be measuring two rules at once.
func nthSaid(s *memory.Store, v memory.View, n int) (memory.Bit, bool) {
	seen := 0
	for i := len(v) - 1; i >= 0; i-- {
		b, ok := s.Get(v[i])
		if !ok {
			panic("tui: the simulator's own view names a bit its own store does not hold")
		}
		if _, cold := b.Payload.(memory.Compaction); cold {
			continue
		}
		if seen == n {
			return b, true
		}
		seen++
	}
	return memory.Bit{}, false
}

// strand counts what is stranded in one frame: a said bit still on screen that
// names, through Prev, a bit the view no longer holds.
//
// Three counts and not one, because they answer three different questions and
// only the middle one is anybody's claim.
//
//   - any is every strand, held or not. It is the control, and it is the reason
//     a zero in the next column can be believed: a fold strands the row after
//     every scar by construction, so a run where any is zero is a run where
//     nothing folded, and held's zero there means nothing at all.
//   - held is the strands the human's ballots are actually holding. This is the
//     definition [memory.View]'s own sparing doc states — on screen, said, held,
//     and naming a Prev the view has let go — and it is the figure every
//     published number answers to.
//   - noscar is the held strands with no fold on the row directly above them.
//     That is the D1 and D14 question rather than the legibility one: a strand
//     whose receipt is the next row up is a row a person can walk back from, and
//     one with nothing above it is not.
//
// # noscar is zero in every row of the frozen table, and that is a prior
//
// Said out loud because a column of zeroes is what an instrument that cannot
// fire looks like from the outside, and this project has shipped an instrument
// that could not fail (D27), a check that certified nothing (D48) and a check
// that required a defect (D52(c)) without any of them announcing itself. This
// column cannot fire *here*, and the argument is short: a
// bit written on this surface takes [memory.View.Head] as its Prev, so its
// parent is the row immediately above it, and a fold replaces a contiguous run
// with one scar standing in the run's own place. Lose the parent and the scar
// that ate it is the row above. The view's first row is the other candidate and
// it is not one either — after any fold it is a scar, and before any fold it is
// the root, which has no Prev to lose.
//
// So the zero is a fact about how this surface writes, not a measurement of the
// fold. Two things would reach the case and neither exists yet: a bit with more
// than one Prev, which D13 already mints for a [memory.Compaction] and nothing
// else, and a bit whose Prev is not the row above it — which is what a second
// writer against a stale view would produce.
//
// **Its own re-check is the column.** The day either arrives, the frozen table
// goes red with a number in it, which is more than a comment saying this would
// have done.
//
// Reads no clock and mints nothing. The hold map comes from [memory.Stay.Holds]
// against [memory.View.Latest], which is what [memory.View.Fold] asks, so this
// and the fold that just ran agree about which holds were alive.
func strand(s *memory.Store, v memory.View, stay memory.Stay) (any, held, noscar int) {
	shown := make(map[string]bool, len(v))
	for _, id := range v {
		shown[id] = true
	}
	holds := stay.Holds(s, v.Latest(s))

	for i, b := range v.Bits(s) {
		if _, cold := b.Payload.(memory.Compaction); cold {
			continue
		}
		lost := false
		for _, p := range b.Prev {
			lost = lost || !shown[p]
		}
		if !lost {
			continue
		}
		any++
		if _, holding := holds[b.ID]; !holding {
			continue
		}
		held++
		if i == 0 {
			noscar++
			continue
		}
		above, _ := s.Get(v[i-1])
		if _, cold := above.Payload.(memory.Compaction); !cold {
			noscar++
		}
	}
	return any, held, noscar
}

// A held strand with no fold above it is counted, and one with a fold above it
// is not.
//
// This is the witness under [strand]'s noscar prior, and it exists because the
// prior alone is not enough. The argument there is sound — nothing this surface
// writes can produce the case — and a reader outside the code cannot tell a
// column that is zero because the case never arises from a column that is zero
// because the condition is inverted, mistyped, or reading the row below. D27,
// D48 and D52(c) are three instances of this project being wrong about exactly
// that on a reading, so the column gets a check that makes it fire.
//
// Both views are built by hand, which is the point rather than a shortcut: the
// simulator cannot reach the first arrangement, so a witness that went through it
// would be measuring the same unreachability the prior already states. What is
// hand-built is only the *shape* — every bit here is filed by [memory.View.Add]
// and the scar is minted by [memory.Cool], so the payloads and addresses are the
// record's own.
//
// The hold is a century so that decay plays no part. It is not the variable
// under test, and pinning it out of the way is what keeps this check from moving
// the day something else moves the view's newest instant — a fold that stamped a
// wall clock would otherwise expire these holds and turn this check into a second
// witness for somebody else's claim (`docs/CLAIMS.md`, cool-reads-the-clock).
func TestAHeldStrandWithNoFoldAboveItIsCounted(t *testing.T) {
	s := memory.NewStore()
	at := time.Date(2026, 8, 12, 9, 0, 0, 0, time.UTC)

	say := func(when time.Time, text string, prev []string) memory.Bit {
		_, b := memory.View(nil).Add(s, memory.Bit{
			At:      when,
			From:    localHandle,
			Channel: channel,
			Payload: memory.Utterance{Text: text},
			Prev:    prev,
		})
		return b
	}

	// lost and with it are what the fold took; neither is in either view below.
	lost := say(at, "the parent that leaves", nil)
	with := say(at.Add(time.Minute), "and its neighbour", []string{lost.ID})
	_, scar := memory.View(nil).Add(s, memory.Cool([]memory.Bit{lost, with}))

	// plain names nothing, so it is not itself stranded and the row under it is
	// answering the question this test asks rather than two of them.
	plain := say(at.Add(2*time.Minute), "a said row, not a fold", nil)
	strandRow := say(at.Add(3*time.Minute), "the held row whose parent has gone", []string{lost.ID})

	ballots, _ := memory.View(nil).Add(s, memory.Cast(strandRow.At, localHandle, memory.Up, strandRow))
	stay := memory.Stay{Votes: ballots, By: localHandle, For: 100 * 365 * 24 * time.Hour}

	for _, tc := range []struct {
		name              string
		view              memory.View
		any, held, noscar int
	}{
		{
			name:   "a said row above the strand",
			view:   memory.View{plain.ID, strandRow.ID},
			any:    1,
			held:   1,
			noscar: 1,
		},
		{
			// The ordinary arrangement, and the reason the row above it is a
			// witness rather than a tautology: the same strand, the same ballot,
			// the same hold, and the only thing that changed is what is on the row
			// above. Without this the check would pass on a noscar that counted
			// every held strand.
			name:   "the fold that took its parent, directly above it",
			view:   memory.View{scar.ID, strandRow.ID},
			any:    1,
			held:   1,
			noscar: 0,
		},
		{
			// The other way to have no receipt above you, and [strand]'s other
			// branch: be the top row. Unreachable here too — after any fold the
			// first row is a scar and before any fold it is the root, which has no
			// Prev to lose — and cheap to hold up, since the branch is one line
			// away from the one above and nothing else in the tree runs it.
			name:   "the strand is the top row, with nothing above it at all",
			view:   memory.View{strandRow.ID},
			any:    1,
			held:   1,
			noscar: 1,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			any, held, noscar := strand(s, tc.view, stay)
			if any != tc.any || held != tc.held || noscar != tc.noscar {
				t.Errorf("strand = any %d, held %d, noscar %d; want any %d, held %d, noscar %d",
					any, held, noscar, tc.any, tc.held, tc.noscar)
			}
		})
	}
}

// grid is every schedule the frozen table covers.
//
// Two blocks, and the split is deliberate. The first sweeps the record's own
// published schedule — 400 bits at one every 3.5 seconds — across both hold
// regimes, three terminal budgets and the vote rates the figures were quoted at,
// with back as the axis nothing had ever moved. The second holds all of that
// still and moves the keep alone, because a keep is a claim about a program
// nobody is running (see [schedule].keep) and mixing it into the first block
// would make every row there ambiguous about which program it describes.
//
// Nobody-voting appears once per record shape rather than once per back: with no
// vote to place there is nowhere to place it, and five identical rows would read
// as five measurements.
func grid() []schedule {
	const bits = 400
	const cadence = 3500 * time.Millisecond

	var out []schedule
	for _, hold := range []time.Duration{holdFor, memory.DefaultHold} {
		for _, budget := range []int{coolFloor, 23, 73} {
			out = append(out, schedule{bits: bits, budget: budget, cadence: cadence, hold: hold})
			for _, rate := range []int{2, 3, 5, 10} {
				for _, back := range []int{0, 1, 3, 6, 12} {
					out = append(out, schedule{
						bits: bits, rate: rate, back: back,
						budget: budget, cadence: cadence, hold: hold,
					})
				}
			}
		}
	}
	for _, keep := range []int{3, 6, 11, 17} {
		for _, back := range []int{0, 6, 12} {
			out = append(out, schedule{
				bits: bits, rate: 5, back: back,
				budget: 23, keep: keep, cadence: cadence, hold: holdFor,
			})
		}
	}
	return out
}

// swept is the grid, run once for the whole test binary and every schedule at
// the same time.
//
// Both of those are about the same number, which is what this costs a commit.
// Two tests read the sweep — the table and the null hypothesis under it — and
// each schedule is four hundred bits of folding, so running the grid twice and
// serially is the difference between a check that belongs in a gate and one that
// gets skipped and then forgotten. Measured on this machine under `-race`: 19.6s
// serial, and the `tui` package's whole suite is 41s without any of this.
//
// Safe to share and safe to run wide because [simulate] is a pure function of
// its [schedule] — it mints its own store, reads no clock, and touches no
// package state — and results land in a slice by index, so the table is the same
// table whatever order the workers finish in.
var swept = sync.OnceValue(func() []outcome {
	all := grid()
	out := make([]outcome, len(all))

	var wg sync.WaitGroup
	for i, sc := range all {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out[i] = simulate(sc)
		}()
	}
	wg.Wait()
	return out
})

// sweep renders the grid as the table testdata holds.
func sweep() string {
	var b strings.Builder
	b.WriteString(`Stranding, driven through this surface's own trigger, fold and cut.
One row per schedule, every number a count except the last. Frozen by
	go test ./tui/ -run TestTheStrandingSweep -update

keep     'cut' is keepFrom, the program's own rule; a number is a fixed cut.
back     said rows above the newest that the upvote landed on, scars skipped.
missed   times the schedule wanted a vote and the view did not reach back
         that far, so none was cast.
any      frames holding a said bit whose Prev has left the view, anyone's
         vote or nobody's. It is the control: a fold does this to the row
         under every scar, so a zero here beside a fold means the count is
         reading the wrong edge.
held     the same, restricted to bits the human's ballots are holding. This
         is the figure every published stranding number answers to.
mean     view length averaged over every frame, beside worst rather than
         instead of it.
peak     the most held strands standing in any one frame.
noscar   held strands with no fold on the row directly above them — nothing
         to walk back through. This is the D1 question rather than the
         legibility one.
held%    held over frames, and blank where missed exceeds votes: a schedule
         that placed fewer votes than it failed to place is not the vote
         rate its row names, and a percentage there would be read as one.

`)
	fmt.Fprintf(&b, "%6s %5s %5s %5s %5s │ %6s %6s %6s %6s %6s │ %6s %6s %6s %6s %5s %6s │ %7s\n",
		"hold", "budg", "keep", "rate", "back",
		"bits", "frames", "votes", "missed", "folds",
		"worst", "mean", "any", "held", "peak", "noscar", "held%")

	for i, sc := range grid() {
		o := swept()[i]

		keep := "cut"
		if sc.keep > 0 {
			keep = fmt.Sprint(sc.keep)
		}
		rate := "none"
		if sc.rate > 0 {
			rate = fmt.Sprintf("1/%d", sc.rate)
		}
		where := fmt.Sprint(sc.back)
		if sc.rate == 0 {
			where = "-"
		}
		// The one thing this table must never do is print a rate that means "the
		// experiment did not run". A deep back on a short view fails to place
		// most of the votes it wanted, and 0.0% there says the schedule stranded
		// nothing when what happened is that the schedule barely voted.
		missed := o.wanted - o.votes
		share := "     -"
		if missed <= o.votes {
			share = fmt.Sprintf("%5.1f%%", 100*float64(o.held)/float64(o.frames))
		}

		fmt.Fprintf(&b, "%6s %5d %5s %5s %5s │ %6d %6d %6d %6d %6d │ %6d %6.1f %6d %6d %5d %6d │ %7s\n",
			sc.hold, sc.budget, keep, rate, where,
			sc.bits, o.frames, o.votes, missed, o.folds,
			o.worst, float64(o.rows)/float64(o.frames), o.any, o.held, o.peak, o.noscar, share)
	}
	return b.String()
}

// The sweep is the table it was frozen as.
//
// This is the whole of the instrument: a figure about stranding is now a diff
// against a file rather than a sentence in a commit message, and changing what a
// fold takes changes this table in the same commit that changes the fold.
func TestTheStrandingSweepReproducesItsFrozenTable(t *testing.T) {
	got := sweep()

	if *rewrite {
		if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("rewrote %s", golden)
		return
	}

	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("%v — run: go test ./tui/ -run TestTheStrandingSweep -update", err)
	}
	if string(want) == got {
		return
	}

	// Row by row rather than as one blob. A hundred and fifty rows of table
	// diffed whole is a wall nobody reads, and the question a reader has is
	// always which schedules moved.
	old, now := strings.Split(string(want), "\n"), strings.Split(got, "\n")
	for i := range max(len(old), len(now)) {
		a, b := "", ""
		if i < len(old) {
			a = old[i]
		}
		if i < len(now) {
			b = now[i]
		}
		if a != b {
			t.Errorf("line %d\n frozen: %s\n    now: %s", i+1, a, b)
		}
	}
	t.Log("if the change is intended: go test ./tui/ -run TestTheStrandingSweep -update")
}

// The sweep can report a strand, and can report the absence of one.
//
// Without this the table above is D27's shape in its purest form: a golden over
// a sweep that stopped folding — or stopped running conversations at all —
// passes forever the moment somebody regenerates it, and a page of
// well-formatted zeroes is indistinguishable from a defect that got fixed.
//
// So the null hypothesis is asserted against the sweep rather than against the
// file, and in both directions: there has to be a schedule that strands a held
// row and a schedule that strands none, or the axis separating them is not being
// swept and the table cannot tell a fix from a fixture.
//
// The one per-row assertion is the control column. Wherever a fold happened,
// something must be stranded — a fold leaves the row after the scar naming a
// parent it just absorbed, always, for anyone's votes and nobody's — so an `any`
// of zero beside a fold means [strand] is reading the wrong edge and every zero
// in the column beside it is worthless.
//
// **A schedule that never folds is a result and not a failure**, which this
// asserted the other way round for one run. At a hold that outlives the whole
// conversation, one upvote in three leaves no free stretch longer than a single
// bit and D32's size rule refuses every one of them: the record stops
// consolidating entirely. That is D58(i)'s cliff, it is in the frozen table
// deliberately, and a check that called it broken would have been a check
// enforcing that the cliff stay unmeasured.
func TestTheStrandingSweepCanReportEitherAnswer(t *testing.T) {
	all := grid()
	if len(all) == 0 {
		t.Fatal("the grid is empty, so the table above it is a heading")
	}

	folding, stranded, clean := 0, 0, 0
	for i, sc := range all {
		o := swept()[i]
		if o.folds == 0 {
			continue
		}
		folding++
		if o.any == 0 {
			t.Errorf("%+v folded %d times and stranded nothing at all, which is not what a scar "+
				"does — strand is reading the wrong edge", sc, o.folds)
		}
		switch {
		case o.held > 0:
			stranded++
		case o.votes > 0:
			clean++
		}
	}
	if folding == 0 {
		t.Fatal("no schedule in the grid ever folds, so the whole table is about a record that " +
			"never consolidated")
	}
	if stranded == 0 {
		t.Error("no schedule in the grid strands a held row, so the table cannot go red for the " +
			"defect it exists to measure")
	}
	if clean == 0 {
		t.Error("every schedule in the grid strands a held row, so the table cannot tell a fix " +
			"from a fixture")
	}
}
