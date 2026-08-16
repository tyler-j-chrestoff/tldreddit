package memory

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// Ranking is the first thing a community gets to define about itself (D3), and
// what makes it a ranking rather than a popularity count is that it has two
// tiers. So these tests are about the seam between them: that the named
// participant's own vote decides alone, that no amount of anybody else's votes
// reaches across it, and that everything the votes are silent about comes back
// in the order it arrived in.
//
// Every expectation below is written by hand from the rule as stated in
// [View.Rank]'s doc comment, never read off a run and never compared against
// [Tally]. Rank and Tally share one traversal, so a test that asked whether they
// agree could not catch an error in the traversal — both would move together and
// the comparison would pass on two wrong answers.

// ranked is one vote in a fixture: who cast it, which way, and which position in
// the transcript it lands on. The target is an index rather than an ID so a row
// below can be read against the want without resolving anything.
type ranked struct {
	who string
	dir Direction
	on  int
}

// rankFixture builds a transcript of n utterances and casts the given votes on
// it, one minute apart in the order written, so a row that casts twice on one
// bit has a well-defined later vote.
func rankFixture(t *testing.T, n int, cast []ranked) (*Store, View, View) {
	t.Helper()

	s := NewStore()
	var shown View
	for i := range n {
		shown, _ = shown.Add(s, said(i, "persona", fmt.Sprintf("bit %d", i), shown.Head()...))
	}
	bits := shown.Bits(s)

	var votes View
	for i, c := range cast {
		votes, _ = votes.Add(s, Cast(at(n+i), Handle{Ref: c.who, Display: c.who}, c.dir, bits[c.on]))
	}
	return s, shown, votes
}

// order turns a ranking back into transcript positions, which is the only form a
// hand-written expectation can be read in.
func order(t *testing.T, shown View, got []Ranked) []int {
	t.Helper()

	at := map[string]int{}
	for i, id := range shown {
		at[id] = i
	}

	out := make([]int, 0, len(got))
	for _, r := range got {
		i, known := at[r.ID]
		if !known {
			t.Fatalf("Rank returned %s, which is not in the view it ranked", Short(r.ID))
		}
		out = append(out, i)
	}
	return out
}

// The rule in one table: the named participant's vote sorts into three bands,
// everyone else arranges within a band, and anything the votes are silent about
// keeps the position it arrived in.
func TestRankOrdersAViewByItsVotes(t *testing.T) {
	const bits = 5
	tyler := Handle{Ref: "tyler", Display: "tyler"}

	tests := []struct {
		name string
		cast []ranked
		want []int
	}{
		// The degenerate case, and it is the one that makes the whole ordering
		// checkable by eye: with nothing voted on there is nothing to rank, so
		// the reading is the transcript.
		{"no votes at all is the view itself", nil, []int{0, 1, 2, 3, 4}},

		{"an upvote lifts a bit to the front",
			[]ranked{{"tyler", Up, 3}},
			[]int{3, 0, 1, 2, 4}},

		// Below bits nobody voted on, not deleted. D1: the record does not
		// forget and neither does this reading — a downvote is a position.
		{"a downvote sinks below what nobody voted on",
			[]ranked{{"tyler", Down, 1}},
			[]int{0, 2, 3, 4, 1}},

		{"three bands at once",
			[]ranked{{"tyler", Up, 4}, {"tyler", Down, 0}},
			[]int{4, 1, 2, 3, 0}},

		// Two bits in the same band are level, and level means the order they
		// happened in.
		{"a tie inside a band keeps view order",
			[]ranked{{"tyler", Up, 3}, {"tyler", Up, 1}},
			[]int{1, 3, 0, 2, 4}},

		{"an agent orders what the participant left level",
			[]ranked{{"agent", Up, 2}},
			[]int{2, 0, 1, 3, 4}},

		// The tier, in the table as well as in its own sweep below: three agents
		// against one human vote, and the human's bit does not move.
		{"an agent cannot cross the participant",
			[]ranked{{"tyler", Up, 0}, {"agent-a", Up, 4}, {"agent-b", Up, 4}, {"agent-c", Up, 4}},
			[]int{0, 4, 1, 2, 3}},

		// An agent's downvote sinks a bit inside the untouched band and no
		// further: it stays above the one the participant downvoted.
		{"an agent's downvote stays inside its own band",
			[]ranked{{"tyler", Down, 0}, {"agent", Down, 3}},
			[]int{1, 2, 4, 3, 0}},

		// Standing votes, not a sum over time — the same rule [Tally] keeps.
		// Counted twice these would cancel and bit 2 would not move.
		{"changing your mind is one vote",
			[]ranked{{"tyler", Up, 2}, {"tyler", Down, 2}},
			[]int{0, 1, 3, 4, 2}},

		{"an agent voting twice is one voter",
			[]ranked{{"agent", Up, 1}, {"agent", Up, 1}},
			[]int{1, 0, 2, 3, 4}},

		// Others is a sum and not a count, which is the difference this row
		// holds: two agents disagreeing leave the bit exactly where it was.
		{"two agents disagreeing cancel each other",
			[]ranked{{"agent-a", Up, 1}, {"agent-b", Down, 1}},
			[]int{0, 1, 2, 3, 4}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, shown, votes := rankFixture(t, bits, tt.cast)

			got := order(t, shown, shown.Rank(s, votes, tyler))
			if !slices.Equal(got, tt.want) {
				t.Errorf("Rank ordered the view %v, want %v", got, tt.want)
			}
		})
	}
}

// The tier rule at scale, which is the part that has to hold when the record has
// more agents in it than people. However many of them vote and however hard,
// they are never in the same tier as the participant: they cannot lift what the
// participant sank, and they cannot sink what the participant lifted.
//
// The sweep is over the number of agents rather than one fixed crowd, because the
// failure this guards against is arithmetic — a merged score crosses the tier at
// some particular count, and a single fixture picks one count and can sit on
// either side of it by luck.
func TestNoCrowdOfVotersCrossesTheParticipantsOwnVote(t *testing.T) {
	const bits = 4
	tyler := Handle{Ref: "tyler", Display: "tyler"}

	for agents := 1; agents <= 50; agents++ {
		t.Run(fmt.Sprintf("%d agents", agents), func(t *testing.T) {
			// bit 0 is upvoted by the participant and buried by the crowd; bit 3
			// is downvoted by the participant and championed by it.
			cast := []ranked{{"tyler", Up, 0}, {"tyler", Down, 3}}
			for i := range agents {
				who := fmt.Sprintf("agent-%d", i)
				cast = append(cast, ranked{who, Down, 0}, ranked{who, Up, 3})
			}

			s, shown, votes := rankFixture(t, bits, cast)
			got := order(t, shown, shown.Rank(s, votes, tyler))

			if got[0] != 0 {
				t.Errorf("%d agents put %d first; the participant upvoted bit 0 and the ranking is %v",
					agents, got[0], got)
			}
			if last := got[len(got)-1]; last != 3 {
				t.Errorf("%d agents put %d last; the participant downvoted bit 3 and the ranking is %v",
					agents, last, got)
			}
		})
	}
}

// D30: the transcript has one legitimate order and it is time. A ranked reading
// is a second document about the same record, so producing one has to leave the
// view a reader is being shown exactly as it was — and a [View] is a value that
// gets copied constantly, so a sort in place reaches every copy of it at once.
//
// Mutation, run: writing the ranking back over the view (`for i, r := range out
// { v[i] = r.ID }` before the return in [View.Rank]) fails this and
// [TestRankOrdersAViewByItsVotes], and nothing else in the package. Two, not
// one — an earlier version of this comment claimed one, which is the shape of
// claim this repository keeps having to withdraw. The second is worth knowing
// about: [order] indexes the view *after* the call, so an in-place sort makes
// every row of that table report the identity ordering, and the table's failures
// read as a ranking that did nothing rather than as a transcript that was
// rewritten. This is the check that names the actual fault.
func TestRankLeavesTheViewItRanked(t *testing.T) {
	tyler := Handle{Ref: "tyler", Display: "tyler"}

	s, shown, votes := rankFixture(t, 5, []ranked{{"tyler", Up, 4}, {"tyler", Down, 0}})
	before := slices.Clone(shown)

	if order(t, shown, shown.Rank(s, votes, tyler))[0] != 4 {
		t.Fatal("the fixture did not reorder anything; the test is not testing what it says")
	}
	if !slices.Equal(shown, before) {
		t.Errorf("the view is now %v, want %v — ranking rewrote the transcript",
			shorten(shown), shorten(before))
	}
}

// Rank has to be pure in [View.Fold]'s sense — the determinism D38(c) says a
// seeded simulator would be nearly free because of. The reachable way to lose it
// is a Go map: [standing] hands its ballots out in an order that is deliberately
// unstable, so an implementation that let that order reach the result would rank
// differently run to run with nothing having changed.
//
// Sampled rather than reasoned about, because the failure is probabilistic: one
// call cannot tell a stable answer from a lucky one. Twenty voters over fifty
// calls is enough that an order-dependent implementation is caught with
// certainty in practice, and the fixture is built so it would have something to
// be wrong about — every bit carries a different second-tier sum, so any
// reordering of the ballots that changed an accumulation would change the
// output.
func TestRankIsTheSameOrderEveryTimeItIsAsked(t *testing.T) {
	const bits, voters, calls = 6, 20, 50
	tyler := Handle{Ref: "tyler", Display: "tyler"}

	var cast []ranked
	for i := range voters {
		who := fmt.Sprintf("agent-%d", i)
		for on := range bits {
			dir := Up
			if (i+on)%3 == 0 {
				dir = Down
			}
			cast = append(cast, ranked{who, dir, on})
		}
	}
	cast = append(cast, ranked{"tyler", Up, bits - 1}, ranked{"tyler", Down, 0})

	s, shown, votes := rankFixture(t, bits, cast)

	first := shown.Rank(s, votes, tyler)
	for range calls {
		if got := shown.Rank(s, votes, tyler); !slices.Equal(got, first) {
			t.Fatalf("Rank returned %v, having returned %v; the order is not the view's",
				order(t, shown, got), order(t, shown, first))
		}
	}
}

// The numbers a caller is handed, which is the other half of what Rank returns:
// a surface drawing this ordering has to be able to say why a row is where it
// is, and one merged score cannot say it. Own is the participant's own standing
// vote and Others is everybody else's, summed and kept apart from it.
func TestRankedCarriesTheTwoNumbersSeparately(t *testing.T) {
	tyler := Handle{Ref: "tyler", Display: "tyler"}

	s, shown, votes := rankFixture(t, 3, []ranked{
		{"tyler", Up, 0}, {"agent-a", Down, 0}, {"agent-b", Down, 0},
		{"agent-a", Up, 1},
	})

	want := []Ranked{
		{Own: 1, Others: -2},
		{Own: 0, Others: 1},
		{Own: 0, Others: 0},
	}

	got := shown.Rank(s, votes, tyler)
	for i, at := range order(t, shown, got) {
		if got[i].Own != want[at].Own || got[i].Others != want[at].Others {
			t.Errorf("bit %d ranked {Own:%d Others:%d}, want {Own:%d Others:%d}",
				at, got[i].Own, got[i].Others, want[at].Own, want[at].Others)
		}
	}
}

// The refusals Rank inherits by sharing [Tally]'s traversal, and the reason it
// is worth a check of its own rather than resting on Tally's: a second reader of
// the vote view is a second chance to route around the guard, and only a call
// through Rank can say it did not. The folded case is the one that looks like
// housekeeping — fold the vote view and its head is a [Compaction], so a Rank
// that shrugged would order a record whose every vote had silently become
// nothing, with the ordering it produced looking exactly like one nobody had
// voted in.
//
// What is asserted is the message rather than that something blew up, because
// recovering and looking no further cannot tell this package's own guard from
// the interface conversion that panics two lines past it — the shape
// [TestTallyPanicsOnAViewThatIsNotVotes] and [TestCastPanics] were both repaired
// out of, and which D48(g) counts at six remaining sites in this package. This
// is not a seventh.
func TestRankPanicsOnAViewThatIsNotVotes(t *testing.T) {
	s := NewStore()
	tyler := Handle{Ref: "tyler", Display: "tyler"}

	var shown View
	for i := range 4 {
		shown, _ = shown.Add(s, said(i, "persona", "bit", shown.Head()...))
	}
	target := shown.Bits(s)[0]

	var cast View
	for i := range 3 {
		cast, _ = cast.Add(s, voted(i+4, "tyler", Up, shown.Bits(s)[i]))
	}
	folded, ok := cast.Fold(s, 0, Stay{})
	if !ok {
		t.Fatal("the vote view did not fold; the test is wrong")
	}

	twoTargets, forked := View{}.Add(s, Bit{At: at(9), From: tyler, Channel: "tui",
		Payload: Vote{dir: Up}, Prev: []string{target.ID, target.ID}})

	tests := []struct {
		name  string
		votes View

		// says is what the panic has to contain, in
		// [TestTallyPanicsOnAViewThatIsNotVotes]'s sense: the address of the bit
		// that is wrong and what is wrong with it. Either alone is a message
		// somebody still has to go and investigate.
		says []string
	}{
		{"an utterance among the votes", View{target.ID},
			[]string{Short(target.ID), "utterance"}},
		{"a folded vote view", folded,
			[]string{Short(folded[0]), "compaction"}},
		{"a vote following two bits", twoTargets,
			[]string{Short(forked.ID), "follows 2 bits"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if r == nil {
					t.Fatalf("Rank over %s did not panic", tt.name)
				}
				said := fmt.Sprint(r)
				for _, want := range tt.says {
					if !strings.Contains(said, want) {
						t.Errorf("Rank over %s panicked with %q, which does not say %q",
							tt.name, said, want)
					}
				}
			}()
			shown.Rank(s, tt.votes, tyler)
		})
	}
}
