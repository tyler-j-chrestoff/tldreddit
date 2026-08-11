package memory

import (
	"slices"
	"strings"
	"testing"
)

func TestPutAddressesAndResolves(t *testing.T) {
	s := NewStore()
	b := s.Put(base())

	if b.ID != ID(base()) {
		t.Errorf("Put returned ID %s, want %s", Short(b.ID), Short(ID(base())))
	}
	got, ok := s.Get(b.ID)
	if !ok {
		t.Fatalf("Get(%s) missed the bit just put", Short(b.ID))
	}
	if got.ID != b.ID || got.Channel != b.Channel {
		t.Errorf("Get returned %+v, want the bit that was put", got)
	}
}

// The collapse is the feature. Two agents arriving separately at the same
// content must land on one object, without a merge step and without either of
// them having to check first.
func TestPutCollapsesIdenticalContent(t *testing.T) {
	tests := []struct {
		name string
		bits []Bit
		want int
	}{
		{"the same bit twice", []Bit{base(), base()}, 1},
		{"a bit and its stored self", func() []Bit {
			b := base()
			b.ID = ID(b)
			return []Bit{base(), b}
		}(), 1},
		{"differing only in text", func() []Bit {
			b := base()
			b.Payload = Utterance{Text: "it worked"}
			return []Bit{base(), b}
		}(), 2},
		{"differing only in instant", func() []Bit {
			b := base()
			b.At = at(1)
			return []Bit{base(), b}
		}(), 2},
		{"differing only in parents", func() []Bit {
			b := base()
			b.Prev = []string{"b", "a"}
			return []Bit{base(), b}
		}(), 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewStore()
			var ids []string
			for _, b := range tt.bits {
				ids = append(ids, s.Put(b).ID)
			}
			if got := s.Len(); got != tt.want {
				t.Errorf("store holds %d bits, want %d", got, tt.want)
			}
			if same := ids[0] == ids[1]; same != (tt.want == 1) {
				t.Errorf("ids %s and %s, want them %s",
					Short(ids[0]), Short(ids[1]),
					map[bool]string{true: "equal", false: "distinct"}[tt.want == 1])
			}
		})
	}
}

func TestGetMisses(t *testing.T) {
	s := NewStore()
	if _, ok := s.Get(ID(base())); ok {
		t.Error("an empty store resolved an address")
	}
}

// An edited bit is a different bit. Silently re-addressing it would let the
// caller go on holding something it believes is what the store has.
func TestPutRejectsAMismatchedLabel(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("Put of a bit with a stale ID did not panic")
		}
	}()

	b := base()
	b.ID = ID(b)
	b.Payload = Utterance{Text: "edited after storing"}
	NewStore().Put(b)
}

func TestViewAddStoresWhatItShows(t *testing.T) {
	s := NewStore()
	var v View

	v, first := v.Add(s, said(0, "tyler", "one"))
	v, second := v.Add(s, said(1, "agent", "two", first.ID))

	if len(v) != 2 || v[0] != first.ID || v[1] != second.ID {
		t.Fatalf("view = %v, want [%s %s]", v, Short(first.ID), Short(second.ID))
	}
	if s.Len() != 2 {
		t.Errorf("store holds %d bits, want 2 — the view showed what it never stored", s.Len())
	}
	if got := v.Head(); len(got) != 1 || got[0] != second.ID {
		t.Errorf("Head = %v, want [%s]", got, Short(second.ID))
	}
}

// Add must not write into a view a caller is still holding. Bubble Tea passes
// models by value on every keystroke, so aliased backing arrays would show up
// as one copy's bits appearing in another's.
func TestViewAddDoesNotDisturbTheOldView(t *testing.T) {
	s := NewStore()
	var v View
	v, _ = v.Add(s, said(0, "tyler", "one"))

	branch, _ := v.Add(s, said(1, "tyler", "two"))
	other, _ := v.Add(s, said(2, "tyler", "three"))

	if len(v) != 1 {
		t.Errorf("the original view grew to %d", len(v))
	}
	if branch[1] == other[1] {
		t.Error("two adds off one view produced the same second entry")
	}
}

func TestViewHeadOfNothing(t *testing.T) {
	if got := (View)(nil).Head(); got != nil {
		t.Errorf("Head of an empty view = %v, want nil — a first bit is a root", got)
	}
}

func TestViewBitsPanicsOnAnUnheldAddress(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("resolving a view against the wrong store did not panic")
		}
	}()
	View{ID(base())}.Bits(NewStore())
}

// This is D1. Folding is allowed to take bits off the screen and is not
// allowed to take them out of the record; a receipt naming an address the
// store cannot resolve is the exact failure content addressing exists to
// prevent.
func TestFoldLeavesEveryAbsorbedBitResolvable(t *testing.T) {
	tests := []struct {
		name  string
		sends int
		keep  int
	}{
		{"one absorbed", 3, 2},
		{"most absorbed", 12, 2},
		{"all but the last", 12, 1},
		{"nothing kept hot", 12, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewStore()
			var v View
			for i := range tt.sends {
				v, _ = v.Add(s, said(i, "tyler", "bit", v.Head()...))
			}
			before := s.Len()

			folded, ok := v.Fold(s, tt.keep)
			if !ok {
				t.Fatalf("Fold(%d) of %d bits refused", tt.keep, tt.sends)
			}
			if got, want := len(folded), tt.keep+1; got != want {
				t.Errorf("view has %d entries, want %d", got, want)
			}
			if s.Len() != before+1 {
				t.Errorf("store went from %d to %d bits; a fold may only add one",
					before, s.Len())
			}

			c, cold := folded.Bits(s)[0].Payload.(Compaction)
			if !cold {
				t.Fatalf("head of the folded view is %T, want a Compaction", folded.Bits(s)[0].Payload)
			}
			if got, want := len(c.absorbed), tt.sends-tt.keep; got != want {
				t.Fatalf("receipt names %d bits, want %d", got, want)
			}
			for _, id := range c.absorbed {
				if _, held := s.Get(id); !held {
					t.Errorf("receipt names %s, which the store does not hold", Short(id))
				}
			}
			for _, id := range v {
				if _, held := s.Get(id); !held {
					t.Errorf("the pre-fold view named %s, which the store no longer holds", Short(id))
				}
			}
		})
	}
}

func TestFoldRefusesWhenThereIsNothingToAbsorb(t *testing.T) {
	tests := []struct {
		name  string
		sends int
		keep  int
	}{
		{"empty view", 0, 6},
		{"fewer bits than kept", 3, 6},
		{"exactly as many as kept", 6, 6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewStore()
			var v View
			for i := range tt.sends {
				v, _ = v.Add(s, said(i, "tyler", "bit", v.Head()...))
			}

			got, ok := v.Fold(s, tt.keep)
			if ok {
				t.Errorf("Fold(%d) of %d bits folded anyway", tt.keep, tt.sends)
			}
			if len(got) != len(v) {
				t.Errorf("view changed from %d to %d entries on a refused fold", len(v), len(got))
			}
			if s.Len() != tt.sends {
				t.Errorf("store holds %d bits, want %d — a refused fold wrote something",
					s.Len(), tt.sends)
			}
		})
	}
}

// The same fold twice is the same object. This is the dedup guarantee reaching
// consolidation itself: re-folding is free, so a caller never has to remember
// whether it already did.
func TestFoldOfTheSameWindowCollapses(t *testing.T) {
	s := NewStore()
	var v View
	for i := range 8 {
		v, _ = v.Add(s, said(i, "tyler", "bit", v.Head()...))
	}

	first, _ := v.Fold(s, 2)
	after := s.Len()
	second, _ := v.Fold(s, 2)

	if first[0] != second[0] {
		t.Errorf("two folds of one window gave %s and %s",
			Short(first[0]), Short(second[0]))
	}
	if s.Len() != after {
		t.Errorf("store grew from %d to %d on a repeat fold", after, s.Len())
	}
}

// Cooling a folded view again must keep the earlier receipt reachable, or the
// oldest material becomes unreachable one generation at a time — the drop is
// silent, which is what makes it dangerous.
func TestFoldsNestWithoutLosingTheFirstOne(t *testing.T) {
	s := NewStore()
	var v View
	for i := range 12 {
		v, _ = v.Add(s, said(i, "tyler", "bit", v.Head()...))
	}

	v, _ = v.Fold(s, 6)
	firstFold := v[0]
	for i := 12; i < 18; i++ {
		v, _ = v.Add(s, said(i, "tyler", "bit", v.Head()...))
	}
	v, _ = v.Fold(s, 6)

	if v[0] == firstFold {
		t.Fatal("the second fold did not produce a new bit")
	}
	if _, held := s.Get(firstFold); !held {
		t.Errorf("the first fold %s is gone from the store", Short(firstFold))
	}

	c := v.Bits(s)[0].Payload.(Compaction)
	if c.count != 12 {
		t.Errorf("the nested receipt stands for %d bits, want 12", c.count)
	}
	for _, id := range c.absorbed {
		if _, held := s.Get(id); !held {
			t.Errorf("the nested receipt names %s, which the store does not hold", Short(id))
		}
	}
}

// A window that has already been folded has nothing left to give. Cooling it
// again merges the same totals into the same answer, so the screen is unchanged
// — while the store gains an object and the walk back to the originals gains a
// hop. Pressing the fold key twice has to be free.
func TestFoldRefusesAWindowWithNothingHot(t *testing.T) {
	s := NewStore()
	var v View
	for i := range 8 {
		v, _ = v.Add(s, said(i, "tyler", "bit", v.Head()...))
	}

	v, ok := v.Fold(s, 6)
	if !ok {
		t.Fatal("the first fold refused")
	}
	held := s.Len()

	// The view is now one cold bit and six hot ones, so keeping six leaves a
	// window of exactly the cold bit.
	got, ok := v.Fold(s, 6)
	if ok {
		t.Errorf("folded a window of one compaction; view is now %d entries", len(got))
	}
	if s.Len() != held {
		t.Errorf("store grew from %d to %d on a refused fold", held, s.Len())
	}
}

// This is the load-bearing invariant: what a caller gets out of the
// store cannot be used to change what is in it. A bit edited through a returned
// copy would still be filed under the address of what it used to say, and
// nothing would ever report it — the record would have quietly become an
// assertion about the past rather than evidence of it.
//
// Only Prev is tested here because Prev is the only writable thing a Bit still
// reaches. A payload's contents are closed by the compiler rather than by a
// copy, which is why this test can afford to run [ID] over the whole record and
// [Store.Get] can afford to run on every frame.
func TestGetCannotAlterTheStore(t *testing.T) {
	s := NewStore()
	var v View
	for i := range 8 {
		v, _ = v.Add(s, said(i, "tyler", "the deploy failed", v.Head()...))
	}
	v, _ = v.Fold(s, 2)

	for _, id := range v {
		b, _ := s.Get(id)
		for i := range b.Prev {
			b.Prev[i] = "tampered"
		}
	}

	for _, id := range v {
		again, ok := s.Get(id)
		if !ok {
			t.Fatalf("the store lost %s", Short(id))
		}
		if slices.Contains(again.Prev, "tampered") {
			t.Errorf("editing a returned bit's Prev reached the store's copy of %s", Short(id))
		}
		if got := ID(again); got != id {
			t.Errorf("%s now addresses to %s; the store holds a bit under the wrong name",
				Short(id), Short(got))
		}
	}
}

// Put copies too, for the other half of the same invariant: a caller that keeps
// the slice it handed over must not be able to reach into the store with it
// afterwards.
func TestPutDoesNotShareTheCallersPrev(t *testing.T) {
	s := NewStore()
	prev := []string{"a", "b"}

	b := s.Put(Bit{At: at(0), Channel: "tui", Payload: Utterance{Text: "one"}, Prev: prev})
	prev[0] = "tampered"

	again, ok := s.Get(b.ID)
	if !ok {
		t.Fatalf("the store lost %s", Short(b.ID))
	}
	if !slices.Equal(again.Prev, []string{"a", "b"}) {
		t.Errorf("Prev = %v, want [a b] — the store shares the caller's slice", again.Prev)
	}
}

// The third direction, and the one the other two miss between them:
// [TestGetCannotAlterTheStore] walks what Get hands out and
// [TestPutDoesNotShareTheCallersPrev] walks what the caller handed in, while
// this walks what Put hands *back*, which is the slice a caller goes on
// holding. Both paths through Put are here because they used to disagree — a
// duplicate leaves the filed bit alone and so was accidentally safe, which is
// how the first-Put case stayed invisible — and [View.Add] is here because it
// returns what Put returned, so it is the route the product actually offers.
func TestPutDoesNotHandBackTheStoredPrev(t *testing.T) {
	made := func() Bit {
		return Bit{At: at(0), Channel: "tui", Payload: Utterance{Text: "one"}, Prev: []string{"a", "b"}}
	}

	tests := []struct {
		name string
		put  func() (*Store, Bit)
	}{
		{"first put", func() (*Store, Bit) {
			s := NewStore()
			return s, s.Put(made())
		}},
		{"the same content again", func() (*Store, Bit) {
			s := NewStore()
			s.Put(made())
			return s, s.Put(made())
		}},
		{"through View.Add", func() (*Store, Bit) {
			s := NewStore()
			_, b := View(nil).Add(s, made())
			return s, b
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, b := tt.put()
			b.Prev[0] = "tampered"

			again, ok := s.Get(b.ID)
			if !ok {
				t.Fatalf("the store lost %s", Short(b.ID))
			}
			if !slices.Equal(again.Prev, []string{"a", "b"}) {
				t.Errorf("Prev = %v, want [a b] — Put returned the slice it filed", again.Prev)
			}
			if got := ID(again); got != b.ID {
				t.Errorf("%s now addresses to %s; the store holds a bit under the wrong name",
					Short(b.ID), Short(got))
			}
		})
	}
}

// Fold has to refuse a keep it cannot mean, at the call. The cost of not doing
// so is not a wrong answer: cut runs past the end of the view, the slice reads
// spare capacity instead of failing, and the empty IDs it collects trip
// View.Bits — whose panic says the store has lost a bit. That is this package's
// alarm for a broken record, and spending it on a caller's arithmetic points
// the next reader at a reachability bug that does not exist.
func TestFoldPanicsOnANegativeKeep(t *testing.T) {
	s := NewStore()
	var v View
	for i := range 8 {
		v, _ = v.Add(s, said(i, "tyler", "bit", v.Head()...))
	}

	defer func() {
		r := recover()
		if msg, _ := r.(string); !strings.Contains(msg, "Fold") {
			t.Errorf("recovered %v, want a panic naming Fold — the caller's mistake has "+
				"to be reported as theirs and not as a hole in the record", r)
		}
	}()
	v.Fold(s, -1)
}
