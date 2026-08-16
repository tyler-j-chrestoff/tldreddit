package main

import (
	"bytes"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/tyler-j-chrestoff/tldreddit/memory"
	"github.com/tyler-j-chrestoff/tldreddit/tui"
)

// marked is a record with a reading's whole problem in it: material a fold has
// taken off the screen, a vote on some of it, a message with a shape, and a
// speaker who was cut off.
//
// The votes are cast by [tui.Human] rather than by this file's `me`, because the
// tier that decides a reading is that participant's own — a fixture voting as
// anybody else would produce a reading in which nothing is kept, and every
// assertion below would then be about an empty first tier.
type marked struct {
	rec record

	kept  memory.Bit // upvoted, and behind the scar: reachable from the record only
	gone  memory.Bit // downvoted, and still on screen
	multi memory.Bit // said with line breaks in it
	cut   memory.Bit // a speaker who did not finish
}

func judged(t *testing.T) marked {
	t.Helper()

	s := memory.NewStore()
	at := time.Date(2026, 8, 14, 9, 0, 0, 0, time.UTC)

	var shown memory.View
	var said []memory.Bit
	for i := range 8 {
		p := memory.Utterance{Text: fmt.Sprintf("bit %d", i)}
		switch i {
		case 2:
			p = memory.Utterance{Text: "what landed\n\n- the seam\n- the reading"}
		case 3:
			p = memory.Utterance{Text: "the migration ran until it", Truncated: true}
		}

		var b memory.Bit
		shown, b = shown.Add(s, memory.Bit{
			At:      at.Add(time.Duration(i) * time.Minute),
			From:    memory.Handle{Ref: "session-15", Display: "session 15"},
			Channel: tui.Channel(),
			Payload: p,
			Prev:    shown.Head(),
		})
		said = append(said, b)
	}

	folded, ok := shown.Fold(s, 3, memory.Stay{})
	if !ok {
		t.Fatal("the fixture did not fold, so nothing in it is out of view and every test here is weaker than it reads")
	}
	shown = folded

	var votes memory.View
	votes, _ = votes.Add(s, memory.Cast(at.Add(9*time.Minute), tui.Human(), memory.Up, said[0]))
	votes, _ = votes.Add(s, memory.Cast(at.Add(10*time.Minute), tui.Human(), memory.Down, said[7]))

	if slices.Contains(shown, said[0].ID) {
		t.Fatalf("%s is still on screen; the interesting case here is a vote on material the "+
			"transcript can no longer reach", memory.Short(said[0].ID))
	}
	return marked{
		rec:   record{store: s, shown: shown, votes: votes},
		kept:  said[0],
		gone:  said[7],
		multi: said[2],
		cut:   said[3],
	}
}

// row is one entry of a reading as a reader meets it: the mark, the address, and
// everything under it.
type row struct {
	mark    string
	address string
	head    string
	text    string
}

// entries parses a reading back into rows. It leans on the one property the
// format guarantees — every line of what somebody said is indented and no header
// line is — so a message containing a blank line, or the word "+1", cannot be
// read as the start of a row.
func entries(t *testing.T, out string) []row {
	t.Helper()

	head := regexp.MustCompile(`^([+ -][01])  ([0-9a-f]{8})  (.*)$`)

	var rows []row
	for _, line := range strings.Split(out, "\n") {
		if m := head.FindStringSubmatch(line); m != nil {
			rows = append(rows, row{mark: m[1], address: m[2], head: m[3]})
			continue
		}
		if after, indented := strings.CutPrefix(line, "    "); indented && len(rows) > 0 {
			last := &rows[len(rows)-1]
			if last.text != "" {
				last.text += "\n"
			}
			last.text += after
		}
	}
	return rows
}

// read runs top and hands back what it printed.
func read(t *testing.T, m marked, args ...string) (string, []row) {
	t.Helper()

	path := saved(t, m.rec)
	out, _, err := ran(t, commands["top"], path, "", args...)
	if err != nil {
		t.Fatalf("top %v: %v", args, err)
	}
	return out, entries(t, out)
}

// The reading is over the record and not over the transcript, which is the whole
// difference between retrieval and a second look at what is already on screen.
// The bit this fixture upvoted is behind a scar; a reading that could not reach
// it would rank only what a reader can already see.
func TestTopReadsTheRecordAndNotTheTranscript(t *testing.T) {
	m := judged(t)
	_, rows := read(t, m, "-n", "0")

	var said, scars, votes int
	for b := range m.rec.store.All() {
		switch b.Payload.(type) {
		case memory.Utterance:
			said++
		case memory.Compaction:
			scars++
		case memory.Vote:
			votes++
		}
	}
	if said == 0 || scars == 0 || votes == 0 {
		t.Fatalf("the fixture holds %d said, %d scars and %d votes; it cannot separate them", said, scars, votes)
	}
	if len(rows) != said {
		t.Errorf("the reading has %d rows over a record of %d said, %d scars and %d votes",
			len(rows), said, scars, votes)
	}

	addressed := func(b memory.Bit) bool {
		return slices.ContainsFunc(rows, func(r row) bool { return r.address == memory.Short(b.ID) })
	}
	if !addressed(m.kept) {
		t.Errorf("%s is not in the reading, and it is the one bit somebody voted to keep",
			memory.Short(m.kept.ID))
	}

	// A scar is not a row here and neither is a vote, for reasons [reading] gives
	// at length. What makes leaving the scar out safe is that everything it
	// stands for is in the reading on its own account, which the count above is.
	for _, id := range m.rec.shown {
		b, _ := m.rec.store.Get(id)
		if _, cold := b.Payload.(memory.Compaction); !cold {
			continue
		}
		if addressed(b) {
			t.Errorf("the scar %s is a row of its own, so the reading holds the material twice",
				memory.Short(b.ID))
		}
	}
	for _, id := range m.rec.votes {
		b, _ := m.rec.store.Get(id)
		if addressed(b) {
			t.Errorf("the vote %s is a row of its own; a vote is a judgment about a row rather than one",
				memory.Short(b.ID))
		}
	}
}

// The mark on a row is the standing vote the person this reading is for cast on
// that bit, and nobody else's.
//
// Deliberately about the mark rather than about where the row landed. Where an
// upvoted row sorts is [memory.View.Rank]'s claim and memory's own tests hold it;
// asserting it again from here would make this a second, weaker witness for
// somebody else's property — which is exactly what happened once on the ranked
// screen (`tui/ranked_test.go`). What is this command's own is which votes it
// hands Rank and whose tier it asks for, and the mark is where that shows.
func TestTheMarkOnARowIsTheVoteThePersonCast(t *testing.T) {
	m := judged(t)
	_, rows := read(t, m, "-n", "0")

	find := func(b memory.Bit) row {
		t.Helper()
		i := slices.IndexFunc(rows, func(r row) bool { return r.address == memory.Short(b.ID) })
		if i < 0 {
			t.Fatalf("%s is not in the reading at all", memory.Short(b.ID))
		}
		return rows[i]
	}

	for _, tt := range []struct {
		name string
		bit  memory.Bit
		want string
	}{
		{"upvoted, and behind a scar", m.kept, "+1"},
		{"downvoted", m.gone, "-1"},
		{"nobody said anything about it", m.multi, " 0"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := find(tt.bit).mark; got != tt.want {
				t.Errorf("%s is marked %q, want %q", memory.Short(tt.bit.ID), got, tt.want)
			}
		})
	}
}

// The header says how much of the order a person actually decided, which on a
// record with two votes in it is the only thing standing between a ranked
// reading and a list sorted by the clock calling itself one. See [top].
//
// The three rows after the first are the two ways the ballot count and the bands
// disagree, and both flatter us — a header saying "3 votes" over a record where a
// person holds two opinions, one of them about something this reading cannot
// show, claims a third more human judgment than there is. Neither is exotic:
// changing your mind is the commonest thing a voter does, and the surface's
// ranked screen lets a person vote on a scar.
func TestTheHeaderSaysHowMuchOfTheOrderAPersonDecided(t *testing.T) {
	// vote adds one more of tui.Human's, late enough to supersede anything the
	// fixture cast, and hands back the fixture.
	vote := func(m marked, dir memory.Direction, on memory.Bit) marked {
		t.Helper()
		at := on.At.Add(time.Hour)
		m.rec.votes, _ = m.rec.votes.Add(m.rec.store, memory.Cast(at, tui.Human(), dir, on))
		return m
	}

	// scar finds the fold in the transcript, which is a bit the reading has no
	// row for and the ranked screen will happily take a vote on.
	scar := func(m marked) memory.Bit {
		t.Helper()
		for _, id := range m.rec.shown {
			b, _ := m.rec.store.Get(id)
			if _, cold := b.Payload.(memory.Compaction); cold {
				return b
			}
		}
		t.Fatal("the fixture holds no scar, so the case this row is about cannot arise in it")
		return memory.Bit{}
	}

	tests := []struct {
		name string
		with func(marked) marked
		want []string
	}{
		{
			name: "as the fixture stands",
			with: func(m marked) marked { return m },
			want: []string{"2 ballots, 2 standing", "kept 1 · not judged 6 · let go 1"},
		},
		{
			name: "somebody changed their mind",
			with: func(m marked) marked { return vote(m, memory.Up, m.gone) },
			want: []string{"3 ballots, 2 standing", "kept 2 · not judged 6 · let go 0"},
		},
		{
			name: "a vote on something this reading has no row for",
			with: func(m marked) marked { return vote(m, memory.Up, scar(m)) },
			want: []string{"3 ballots, 3 standing", "· 1 on nothing this reading shows"},
		},
		{
			name: "one ballot reads as one",
			with: func(m marked) marked {
				m.rec.votes = m.rec.votes[:1]
				return m
			},
			want: []string{"1 ballot, 1 standing"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.with(judged(t))
			out, rows := read(t, m, "-n", "0")

			want := append([]string{
				fmt.Sprintf("%d said of %d bits on the record", len(rows), m.rec.store.Len()),
				"ranked for " + tui.Human().Display,
			}, tt.want...)
			for _, w := range want {
				if !strings.Contains(out, w) {
					t.Errorf("the header does not say %q:\n%s", w, out)
				}
			}
		})
	}
}

// A row names the ref the record keys its speaker on, not only the display name
// somebody chose — because through this command, and only through this command,
// a reader has no other way to tell the human's own words from a `say -as local`.
func TestARowNamesTheHandleTheRecordKeysOn(t *testing.T) {
	m := judged(t)
	_, rows := read(t, m, "-n", "0")

	if len(rows) == 0 {
		t.Fatal("the reading is empty")
	}
	for _, r := range rows {
		if !strings.Contains(r.head, "(session-15)") {
			t.Errorf("the row %q does not name the ref it was recorded under", r.head)
		}
	}

	// And the rule for the column itself, whose whole job is that no two
	// participants draw the same string. The wants are built from [tui.Human]
	// rather than written out, so this file states nobody's identity twice.
	me := tui.Human()
	for _, tt := range []struct {
		name string
		from memory.Handle
		want string
	}{
		{
			name: "two fields with one fact between them",
			from: memory.Handle{Ref: "an-agent", Display: "an-agent"},
			want: "an-agent",
		},
		{
			name: "no display name at all",
			from: memory.Handle{Ref: "an-agent"},
			want: "an-agent",
		},
		{
			name: "a display name beside a ref",
			from: memory.Handle{Ref: "ollama/qwen3.5", Display: "qwen3"},
			want: "qwen3 (ollama/qwen3.5)",
		},
		{
			name: "the person at the keyboard",
			from: me,
			want: fmt.Sprintf("%s (%s)", me.Display, me.Ref),
		},
		{
			// The row this rule exists for: a handle whose ref *is* the human's
			// display name would otherwise draw a bare `me`, which says less about
			// itself than the human's own row does and therefore reads as more of a
			// person than the person.
			name: "somebody keyed on the name the human displays under",
			from: memory.Handle{Ref: me.Display, Display: me.Display},
			want: fmt.Sprintf("%s (%s)", me.Display, me.Display),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := speaker(tt.from); got != tt.want {
				t.Errorf("%+v draws as %q, want %q", tt.from, got, tt.want)
			}
		})
	}
}

// -n is how much of the reading to print, and the header says when it is holding
// something back. A reading that silently stopped at ten rows would be a fold
// with no receipt, which is this program's own subject.
func TestTheRowLimitPrintsThatManyAndSaysWhenItCut(t *testing.T) {
	m := judged(t)
	_, all := read(t, m, "-n", "0")

	tests := []struct {
		name string
		args []string
		want int
		cut  bool
	}{
		{"all of it", []string{"-n", "0"}, len(all), false},
		{"one row", []string{"-n", "1"}, 1, true},
		{"the default", nil, min(10, len(all)), len(all) > 10},
		{"more rows than there are", []string{"-n", "500"}, len(all), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, rows := read(t, m, tt.args...)
			if len(rows) != tt.want {
				t.Errorf("printed %d rows, want %d", len(rows), tt.want)
			}
			if said := strings.Contains(out, "showing the first"); said != tt.cut {
				t.Errorf("the header %s that it is showing part of the reading, and it is showing %d of %d",
					map[bool]string{true: "says", false: "does not say"}[said], len(rows), len(all))
			}
		})
	}
}

// What was said comes back as it was said, line breaks included — which the
// transcript cannot do: it flows a message into one paragraph, so this reading
// shows a shape the screen never does (`docs/DEBT.md`, and saidWhole's own doc).
//
// The fragment's mark is here too, on the row rather than in the text. A reading
// that dropped it would hand the next session an unfinished sentence as a
// finished one, which is the permanent falsehood [memory.Utterance].Truncated
// exists to prevent.
func TestTheReadingKeepsTheShapeOfWhatWasSaid(t *testing.T) {
	m := judged(t)
	_, rows := read(t, m, "-n", "0")

	find := func(b memory.Bit) row {
		t.Helper()
		i := slices.IndexFunc(rows, func(r row) bool { return r.address == memory.Short(b.ID) })
		if i < 0 {
			t.Fatalf("%s is not in the reading at all", memory.Short(b.ID))
		}
		return rows[i]
	}

	multi := find(m.multi)
	if got, want := multi.text, m.multi.Payload.(memory.Utterance).Text; got != want {
		t.Errorf("read back %q, want %q", got, want)
	}
	if !strings.Contains(multi.text, "\n\n") {
		t.Errorf("the blank line inside the message is gone: %q", multi.text)
	}

	cut := find(m.cut)
	if !strings.Contains(cut.head, "cut off") {
		t.Errorf("the row for an unfinished message reads %q and does not say it was cut off", cut.head)
	}
	if got, want := cut.text, m.cut.Payload.(memory.Utterance).Text; got != want {
		t.Errorf("the unfinished message reads %q, want %q — nothing this program says belongs "+
			"inside a quotation", got, want)
	}
}

// reading's own claim, tested on reading rather than through the ranked output:
// everything anybody said, newest first, and nothing else.
//
// Newest first is this command's tiebreak and not [memory.View.Rank]'s — Rank
// sorts stably and leaves ties where it found them, so the order handed in is the
// order that comes back for every bit nobody voted on. Testing it here rather
// than through the printed reading keeps the two claims apart.
func TestTheViewBeingRankedIsEverythingSaidNewestFirst(t *testing.T) {
	m := judged(t)
	v := reading(m.rec.store)

	var last time.Time
	for i, id := range v {
		b, ok := m.rec.store.Get(id)
		if !ok {
			t.Fatalf("the reading names %s, which the record does not hold", memory.Short(id))
		}
		if _, said := b.Payload.(memory.Utterance); !said {
			t.Errorf("row %d is a %T, and a reading is what participants said", i, b.Payload)
		}
		if i > 0 && b.At.After(last) {
			t.Errorf("row %d is newer than the row above it: %s after %s",
				i, b.At.Format(time.RFC3339), last.Format(time.RFC3339))
		}
		last = b.At
	}
}

// An empty record reads as an empty record, rather than as a header over
// nothing. It is the first thing anybody runs, on the day before anything has
// been said.
func TestAReadingOfAnEmptyRecordSaysSo(t *testing.T) {
	var out, errs bytes.Buffer
	path := saved(t, record{store: memory.NewStore()})

	if err := commands["top"].run(streams{in: strings.NewReader(""), out: &out, err: &errs}, path, nil); err != nil {
		t.Fatalf("top over an empty record: %v", err)
	}
	if !strings.Contains(out.String(), "nothing has been said") {
		t.Errorf("an empty record reads as:\n%s", out.String())
	}
	if rows := entries(t, out.String()); len(rows) != 0 {
		t.Errorf("an empty record produced %d rows", len(rows))
	}
}
