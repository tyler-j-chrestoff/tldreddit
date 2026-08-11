package memory

import (
	"maps"
	"slices"
	"testing"
	"time"
)

// t0 is a fixed origin. Tests never read the clock, so a failure means the code
// changed, not that time passed.
var t0 = time.Date(2026, 8, 11, 9, 0, 0, 0, time.UTC)

func at(min int) time.Time { return t0.Add(time.Duration(min) * time.Minute) }

// said builds an utterance and addresses it. Tests carry real content
// addresses rather than stand-in names because a receipt full of made-up IDs
// looks exactly like a correct one — it just does not resolve.
func said(min int, who, text string, prev ...string) Bit {
	b := Bit{
		At:      at(min),
		From:    Handle{Ref: who, Display: who},
		Channel: "tui",
		Payload: Utterance{Text: text},
		Prev:    prev,
	}
	b.ID = ID(b)
	return b
}

func TestBagFoldsCaseAndPunctuation(t *testing.T) {
	got := map[string]int{}
	words(got, "the deploy failed")
	words(got, "The deploy failed AGAIN!")

	want := map[string]int{"the": 2, "deploy": 2, "failed": 2, "again": 1}
	for word, n := range want {
		if got[word] != n {
			t.Errorf("bag[%q] = %d, want %d", word, got[word], n)
		}
	}
	if len(got) != len(want) {
		t.Errorf("bag has %d words, want %d: %v", len(got), len(want), got)
	}
}

func TestCoolAbsorbsWindow(t *testing.T) {
	root := said(0, "tyler", "root")
	window := []Bit{
		said(1, "tyler", "the deploy failed", root.ID),
		said(5, "tyler", "again"),
	}

	cold := Cool(window)

	c, ok := cold.Payload.(Compaction)
	if !ok {
		t.Fatalf("payload = %T, want Compaction", cold.Payload)
	}
	if c.count != 2 {
		t.Errorf("Count = %d, want 2", c.count)
	}
	if got, want := len(c.absorbed), 2; got != want {
		t.Errorf("Absorbed = %v, want %d ids", c.absorbed, want)
	}
	if !c.from.Equal(at(1)) || !c.to.Equal(at(5)) {
		t.Errorf("span = %s..%s, want %s..%s", c.from, c.to, at(1), at(5))
	}
	if cold.Channel != "tui" {
		t.Errorf("Channel = %q, want %q", cold.Channel, "tui")
	}
	// Prev is what the bit was derived from: the whole window, in order. The
	// root is reached through the window, not instead of it.
	want := []string{window[0].ID, window[1].ID}
	if !slices.Equal(cold.Prev, want) {
		t.Errorf("Prev = %v, want the window %v", cold.Prev, want)
	}
	if slices.Contains(cold.Prev, root.ID) {
		t.Error("Prev names the root, which is the window's parent and not the window")
	}
}

// The receipt keeps originals only, so when a fold absorbs the fold before it,
// the cold bit's Prev is the one edge naming that earlier fold. Losing it
// strands a whole generation in the store with nothing pointing at it.
func TestCoolNamesTheFoldItAbsorbed(t *testing.T) {
	first := Cool([]Bit{said(0, "tyler", "one"), said(1, "tyler", "two")})
	second := Cool([]Bit{first, said(2, "tyler", "three")})

	if !slices.Contains(second.Prev, first.ID) {
		t.Errorf("Prev = %v, want it to name the absorbed fold %s",
			second.Prev, Short(first.ID))
	}
	if c := second.Payload.(Compaction); slices.Contains(c.absorbed, first.ID) {
		t.Error("Absorbed names a fold; it is supposed to carry originals only")
	}
}

// Folding the same window twice must land on the same object. If it did not,
// every re-fold would mint a near-duplicate summary differing only in when it
// was made, and the store would fill with them.
func TestCoolIsDeterministic(t *testing.T) {
	window := []Bit{
		said(0, "tyler", "the deploy failed"),
		said(1, "agent", "rolling back"),
	}

	first, second := Cool(window), Cool(window)
	if first.ID != second.ID {
		t.Errorf("Cool gave %s then %s for one window", Short(first.ID), Short(second.ID))
	}
	if first.ID != ID(first) {
		t.Errorf("Cool returned ID %s but the bit addresses to %s",
			Short(first.ID), Short(ID(first)))
	}
	// At is the end of the span, not the moment of folding — that is what
	// keeps the two calls above from disagreeing.
	if !first.At.Equal(at(1)) {
		t.Errorf("At = %s, want the end of the span %s", first.At, at(1))
	}
}

// Cooling a cold bit must preserve what it held. This is the leak that kills a
// tiered memory: nothing crashes, the oldest material just quietly evaporates
// one generation at a time.
func TestCoolIsClosedUnderItself(t *testing.T) {
	b0 := said(0, "tyler", "the deploy failed")
	b1 := said(5, "tyler", "the deploy failed again")

	first := Cool([]Bit{b0, b1})
	second := Cool([]Bit{first})

	c, ok := second.Payload.(Compaction)
	if !ok {
		t.Fatalf("payload = %T, want Compaction", second.Payload)
	}
	if c.count != 2 {
		t.Errorf("Count = %d, want 2 — a generation was dropped", c.count)
	}
	if c.bag["deploy"] != 2 {
		t.Errorf("Bag[deploy] = %d, want 2 — words were lost on the second fold", c.bag["deploy"])
	}
	if want := []string{b0.ID, b1.ID}; !slices.Equal(c.absorbed, want) {
		t.Errorf("Absorbed = %v, want %v — the receipt must name original bits", c.absorbed, want)
	}
	if !c.from.Equal(at(0)) || !c.to.Equal(at(5)) {
		t.Errorf("span = %s..%s, want %s..%s — the span shrank", c.from, c.to, at(0), at(5))
	}
}

// The zero time is a legal instant, not a signal that the span accumulator is
// still empty. A bit carrying it anywhere but last used to have its own start
// overwritten by the bit after it, which produced a receipt counting material
// that its span excludes — the one thing a receipt may never do, and the reason
// this is worth a test for a case the surface cannot currently reach.
func TestCoolSpansABitWithNoInstant(t *testing.T) {
	unstamped := Bit{From: Handle{Ref: "tyler", Display: "tyler"}, Channel: "tui",
		Payload: Utterance{Text: "no clock"}}
	unstamped.ID = ID(unstamped)

	c := Cool([]Bit{unstamped, said(5, "tyler", "later")}).Payload.(Compaction)

	if c.count != 2 {
		t.Fatalf("Count = %d, want 2", c.count)
	}
	if !c.from.IsZero() {
		t.Errorf("from = %s, want the zero instant the first bit carries", c.from)
	}
	if !c.to.Equal(at(5)) {
		t.Errorf("to = %s, want %s", c.to, at(5))
	}
}

// Handles and Kinds have to merge like every other aggregate. Reading the cold
// bit's own metadata instead loses this in the quietest possible way: the seam
// renders identically either way, so a person watching sees nothing at all
// while the record forgets who was in the conversation.
func TestCoolMergesHandlesAndKindsAcrossGenerations(t *testing.T) {
	first := Cool([]Bit{
		said(0, "tyler", "the deploy failed"),
		said(1, "deploy-bot", "rolling back"),
	})
	second := Cool([]Bit{first, said(2, "tyler", "thanks")})

	c := second.Payload.(Compaction)
	refs := []string{}
	for _, h := range c.handles {
		refs = append(refs, h.Ref)
	}
	if !slices.Equal(refs, []string{"tyler", "deploy-bot"}) {
		t.Errorf("Handles = %v, want [tyler deploy-bot] merged in first-seen order", refs)
	}
	if slices.Contains(refs, "cool") {
		t.Error("Handles names the fold itself, which was never in the conversation")
	}
	if want := (map[string]int{"utterance": 3}); !maps.Equal(c.kinds, want) {
		t.Errorf("Kinds = %v, want %v — a fold is not a payload kind of its own", c.kinds, want)
	}
}

func TestCoolDedupesHandlesInOrder(t *testing.T) {
	cold := Cool([]Bit{
		said(0, "tyler", "one"),
		said(1, "agent", "two"),
		said(2, "tyler", "three"),
	})

	c := cold.Payload.(Compaction)
	if len(c.handles) != 2 {
		t.Fatalf("Handles = %v, want 2 distinct", c.handles)
	}
	if c.handles[0].Ref != "tyler" || c.handles[1].Ref != "agent" {
		t.Errorf("Handles = %v, want first-seen order [tyler agent]", c.handles)
	}
}

// The accessors are the only way in from outside the package, so they have to
// agree with what Cool built. A holder of what they yield must not be able to
// reach back into it — that is checked in TestGetCannotAlterTheStore, which is
// where it would actually bite.
func TestCompactionAccessorsAgreeWithTheFold(t *testing.T) {
	c := Cool([]Bit{
		said(0, "tyler", "the deploy failed"),
		said(1, "agent", "rolling back the deploy"),
	}).Payload.(Compaction)

	if got := c.Count(); got != 2 {
		t.Errorf("Count() = %d, want 2", got)
	}
	if !c.From().Equal(at(0)) || !c.To().Equal(at(1)) {
		t.Errorf("span = %s..%s, want %s..%s", c.From(), c.To(), at(0), at(1))
	}
	if got := slices.Collect(c.Handles()); !slices.Equal(got, c.handles) {
		t.Errorf("Handles() = %v, want %v", got, c.handles)
	}
	if got := maps.Collect(c.Kinds()); !maps.Equal(got, c.kinds) {
		t.Errorf("Kinds() = %v, want %v", got, c.kinds)
	}
	if got := maps.Collect(c.Bag()); !maps.Equal(got, c.bag) {
		t.Errorf("Bag() = %v, want %v", got, c.bag)
	}
	if got := slices.Collect(c.Absorbed()); !slices.Equal(got, c.absorbed) {
		t.Errorf("Absorbed() = %v, want %v", got, c.absorbed)
	}
}

func TestCoolPanics(t *testing.T) {
	tests := []struct {
		name string
		bits []Bit
	}{
		{"empty window", nil},
		{"across channels", []Bit{
			said(0, "tyler", "here"),
			{At: at(1), Channel: "internal", Payload: Utterance{Text: "private"}},
		}},
		// Both Prev and Absorbed would take an empty string and promise it
		// resolves. Neither reports itself later, so it has to fail here.
		{"an unaddressed bit", []Bit{
			{At: at(0), Channel: "tui", Payload: Utterance{Text: "never stored"}},
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Errorf("Cool(%s) did not panic", tt.name)
				}
			}()
			Cool(tt.bits)
		})
	}
}
