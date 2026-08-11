package memory

import (
	"maps"
	"slices"
	"testing"
	"time"
)

// base is a fully populated bit. Every case below is a departure from it, so a
// difference in ID is attributable to exactly the field that moved.
func base() Bit {
	return Bit{
		At:      at(0),
		From:    Handle{Ref: "tyler", Display: "Tyler"},
		Channel: "tui",
		Payload: Utterance{Text: "the deploy failed"},
		Prev:    []string{"a", "b"},
	}
}

// The address must be pinned, not merely self-consistent. A test that only
// compares ID(x) to ID(x) passes just as happily after someone reorders the
// encoding, and the damage from that is silent: old stores and new stores stop
// agreeing on what any object is called. If this fails, either the encoding
// changed on purpose — in which case every existing store is a different store
// and that needs saying out loud — or it changed by accident.
func TestIDIsPinned(t *testing.T) {
	const want = "9620faf0a485728843fb2c1c1ef86e2dcd3d8477aadff0a9016e121654326cb8"
	if got := ID(base()); got != want {
		t.Errorf("ID(base) = %s, want %s", got, want)
	}
}

func TestIDDependsOnEveryField(t *testing.T) {
	tests := []struct {
		name string
		bit  Bit
	}{
		{"different instant", func() Bit { b := base(); b.At = at(1); return b }()},
		{"different nanosecond", func() Bit { b := base(); b.At = b.At.Add(1); return b }()},
		{"different handle ref", func() Bit { b := base(); b.From.Ref = "agent"; return b }()},
		{"different display name", func() Bit { b := base(); b.From.Display = "tyler"; return b }()},
		{"different channel", func() Bit { b := base(); b.Channel = "internal"; return b }()},
		{"different text", func() Bit { b := base(); b.Payload = Utterance{Text: "it worked"}; return b }()},
		{"different payload kind", func() Bit { b := base(); b.Payload = Compaction{count: 1}; return b }()},
		{"different parent", func() Bit { b := base(); b.Prev = []string{"a", "c"}; return b }()},
		{"reordered parents", func() Bit { b := base(); b.Prev = []string{"b", "a"}; return b }()},
		{"fewer parents", func() Bit { b := base(); b.Prev = []string{"a"}; return b }()},
		{"no parents", func() Bit { b := base(); b.Prev = nil; return b }()},

		// Field boundaries have to survive the encoding. Without a length in
		// front of every string these three would concatenate to the same
		// bytes as the base bit and share its address.
		{"boundary shifted right", func() Bit {
			b := base()
			b.From = Handle{Ref: "tyl", Display: "erTyler"}
			return b
		}()},
		{"boundary shifted left", func() Bit {
			b := base()
			b.From, b.Channel = Handle{Ref: "tyler", Display: "Tylert"}, "ui"
			return b
		}()},
		{"parents run together", func() Bit { b := base(); b.Prev = []string{"ab"}; return b }()},
	}

	want := ID(base())
	seen := map[string]string{want: "base"}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ID(tt.bit)
			if prior, clash := seen[got]; clash {
				t.Errorf("ID = %s, same as %q — the encoding cannot tell them apart",
					Short(got), prior)
			}
			seen[got] = tt.name
		})
	}
}

// Equal content addresses equally however it was built, and whatever the bit
// was already labelled. The label case matters because a bit that comes back
// out of a store carries an ID, and re-addressing it must not fold that ID
// back into the hash.
func TestIDIgnoresLabelAndBuildOrder(t *testing.T) {
	labelled := base()
	labelled.ID = "some other thing entirely"

	built := Bit{Prev: []string{"a", "b"}, Channel: "tui"}
	built.Payload = Utterance{Text: "the deploy failed"}
	built.From = Handle{Display: "Tyler", Ref: "tyler"}
	built.At = at(0)

	want := ID(base())
	for name, b := range map[string]Bit{"labelled": labelled, "built out of order": built} {
		if got := ID(b); got != want {
			t.Errorf("ID(%s) = %s, want %s", name, Short(got), Short(want))
		}
	}
}

// A time carries a zone and, straight out of time.Now, a process-local
// monotonic reading. Neither is content. If either reached the hash, the same
// moment would address differently depending on who recorded it and whether it
// had been through a serializer.
func TestIDNormalizesTime(t *testing.T) {
	tokyo := time.FixedZone("JST", 9*60*60)

	utc := base()
	elsewhere := base()
	elsewhere.At = utc.At.In(tokyo)

	if !utc.At.Equal(elsewhere.At) {
		t.Fatal("the two times are not the same instant; the test is wrong")
	}
	if ID(utc) != ID(elsewhere) {
		t.Errorf("the same instant in UTC and JST addressed differently: %s vs %s",
			Short(ID(utc)), Short(ID(elsewhere)))
	}
}

// Compaction holds two maps, and Go randomizes map iteration on every run. An
// unordered walk would give one compaction a different address each time it
// was hashed, which is the kind of bug that passes locally and splits a store
// in production.
func TestIDIsStableAcrossMapOrder(t *testing.T) {
	b := coldBase()
	want := ID(b)
	for range 50 {
		if got := ID(b); got != want {
			t.Fatalf("ID = %s then %s — map order leaked into the address",
				Short(want), Short(got))
		}
	}
}

// Two bags with the same words and different totals are different material.
// Counting only the keys would let a run of ten "deploy" collapse into a run
// of one.
func TestIDSeesBagCounts(t *testing.T) {
	one, ten := base(), base()
	one.Payload = Compaction{count: 1, bag: map[string]int{"deploy": 1}}
	ten.Payload = Compaction{count: 10, bag: map[string]int{"deploy": 10}}

	if ID(one) == ID(ten) {
		t.Error("bags differing only in counts addressed alike")
	}
}

// coldBase is a fully populated compaction bit. Every block of the compaction
// encoding is exercised at once: a non-zero count, two distinct ends of the
// span, handles, both maps holding different contents so the blocks cannot be
// swapped unnoticed, and a receipt.
func coldBase() Bit {
	b := base()
	b.Payload = Compaction{
		count:    3,
		from:     at(0),
		to:       at(5),
		handles:  []Handle{{Ref: "tyler", Display: "Tyler"}, {Ref: "deploy-bot", Display: "deploy"}},
		kinds:    map[string]int{"utterance": 3},
		bag:      map[string]int{"the": 2, "deploy": 2, "failed": 1},
		absorbed: []string{"a", "b", "c"},
	}
	return b
}

// A compaction's address must be pinned for the same reason an utterance's is,
// and more urgently: its encoding has six blocks rather than two, so there is
// far more of it to silently stop writing. See [TestIDIsPinned] on what a
// failure here means.
func TestCompactionIDIsPinned(t *testing.T) {
	const want = "29dbf1e66cceb64b4dc575ddf1e41cc45f87b86e4ca453498611e2e0bb1ebda9"
	if got := ID(coldBase()); got != want {
		t.Errorf("ID(coldBase) = %s, want %s", got, want)
	}
}

// Every block of the compaction encoding has to be able to tell two
// compactions apart, and a missing block is not a visible bug. It is a
// collision generator: two folds that absorbed different material address
// alike, the store keeps whichever landed first, and the second one's receipt
// resolves to somebody else's conversation.
//
// What this table cannot see is the encoding changing shape underneath it.
// Every case is differential — each ID is compared against ID(coldBase)
// computed by the same encoder — so reordering two blocks or dropping a tag
// moves every address here in step and the table stays green. Verified by
// mutation, not assumed: three such mutants left this test passing and only
// [TestCompactionIDIsPinned] failed. That is what the pin is for, and it is
// worth knowing before anyone "just updates" the golden to make a build go
// green.
func TestCompactionIDDependsOnEveryField(t *testing.T) {
	with := func(edit func(*Compaction)) Bit {
		b := coldBase()
		c := b.Payload.(Compaction)
		c.handles = slices.Clone(c.handles)
		c.kinds, c.bag = maps.Clone(c.kinds), maps.Clone(c.bag)
		c.absorbed = slices.Clone(c.absorbed)
		edit(&c)
		b.Payload = c
		return b
	}

	tests := []struct {
		name string
		bit  Bit
	}{
		{"different count", with(func(c *Compaction) { c.count = 4 })},
		{"different start", with(func(c *Compaction) { c.from = at(1) })},
		{"different end", with(func(c *Compaction) { c.to = at(6) })},
		{"span collapsed", with(func(c *Compaction) { c.to = c.from })},
		{"an extra handle", with(func(c *Compaction) {
			c.handles = append(c.handles, Handle{Ref: "agent", Display: "agent"})
		})},
		{"no handles", with(func(c *Compaction) { c.handles = nil })},
		{"reordered handles", with(func(c *Compaction) { slices.Reverse(c.handles) })},
		{"a handle's display name", with(func(c *Compaction) { c.handles[0].Display = "tyler" })},
		{"different kinds", with(func(c *Compaction) { c.kinds = map[string]int{"utterance": 2} })},
		{"no kinds", with(func(c *Compaction) { c.kinds = nil })},
		{"different bag", with(func(c *Compaction) { c.bag["deploy"] = 3 })},
		{"no bag", with(func(c *Compaction) { c.bag = nil })},
		{"different receipt", with(func(c *Compaction) { c.absorbed = []string{"a", "b", "d"} })},
		{"reordered receipt", with(func(c *Compaction) { slices.Reverse(c.absorbed) })},
		{"empty receipt", with(func(c *Compaction) { c.absorbed = nil })},

		// The two maps are separate aggregates, not one merged tally: moving
		// contents from either into the other has to move the address.
		{"maps exchanged", with(func(c *Compaction) { c.kinds, c.bag = c.bag, c.kinds })},
	}

	seen := map[string]string{ID(coldBase()): "coldBase"}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ID(tt.bit)
			if prior, clash := seen[got]; clash {
				t.Errorf("ID = %s, same as %q — the encoding cannot tell them apart",
					Short(got), prior)
			}
			seen[got] = tt.name
		})
	}
}

// A value method is in the pointer type's method set too, so *Compaction
// satisfies Payload however closed the set claims to be. Refusing to address
// one keeps it out of every store, which is the point at which it would start
// being shared mutable state inside an immutable record.
func TestIDRejectsAPointerPayload(t *testing.T) {
	for name, p := range map[string]Payload{
		"pointer to utterance": &Utterance{Text: "the deploy failed"},
		"pointer to compaction": &Compaction{
			count: 1, kinds: map[string]int{"utterance": 1}, absorbed: []string{"a"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("ID of a pointer payload did not panic")
				}
			}()
			b := base()
			b.Payload = p
			ID(b)
		})
	}
}

func TestIDPanicsWithoutPayload(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("ID of a payloadless bit did not panic")
		}
	}()
	ID(Bit{At: at(0), Channel: "tui"})
}

func TestShort(t *testing.T) {
	tests := []struct {
		name, id, want string
	}{
		{"full address", "0123456789abcdef0123", "01234567"},
		{"exactly the cut", "01234567", "01234567"},
		{"shorter than the cut", "0123", "0123"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Short(tt.id); got != tt.want {
				t.Errorf("Short(%q) = %q, want %q", tt.id, got, tt.want)
			}
		})
	}
}
