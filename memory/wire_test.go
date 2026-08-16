package memory

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"
)

// record builds a store and the two views over it that a real session holds: a
// transcript that has been folded, and a vote view that never is. Every payload
// this package can address is in it — a complete utterance, a fragment, both
// vote directions, and a compaction with all four of its collections
// populated — so a round trip of this one record exercises the whole encoding.
func record(t *testing.T) (*Store, View, View) {
	t.Helper()

	s := NewStore()
	var shown, votes View

	tyler := Handle{Ref: "tyler", Display: "Tyler"}
	say := func(from, to int) {
		for i := from; i < to; i++ {
			shown, _ = shown.Add(s, said(i, "tyler", "the deploy failed again", shown.Head()...))
			shown, _ = shown.Add(s, said(i, "persona", "which deploy", shown.Head()...))
		}
	}

	// The fragment sits in the middle rather than at the end, because the folds
	// below take a prefix: at the end it survives every fold and the scars
	// never carry "fragment" in their tally, which is the case worth testing.
	say(0, 6)
	cut := Bit{
		At:      at(6),
		From:    Handle{Ref: "persona", Display: "persona"},
		Channel: "tui",
		Payload: Utterance{Text: "the disk filled because the log rotation", Truncated: true},
		Prev:    shown.Head(),
	}
	shown, cut = shown.Add(s, cut)
	say(7, 11)

	votes, _ = votes.Add(s, Cast(at(21), tyler, Up, cut))
	votes, _ = votes.Add(s, Cast(at(22), tyler, Down, shown.Bits(s)[0]))

	// Fold twice so the record holds a scar that absorbed a scar, which is the
	// only way handles, kinds, bag and absorbed all carry merged content.
	shown, _ = shown.Fold(s, 6, Stay{})
	shown, _ = shown.Fold(s, 2, Stay{})

	if s.Len() < 17 {
		t.Fatalf("the fixture holds %d bits; it is meant to be a whole session", s.Len())
	}
	return s, shown, votes
}

func wrote(t *testing.T, s *Store) []byte {
	t.Helper()

	var buf bytes.Buffer
	n, err := s.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	if n != int64(buf.Len()) {
		t.Errorf("WriteTo reported %d bytes and wrote %d", n, buf.Len())
	}
	return buf.Bytes()
}

// Two processes, one record. The decoded store is not merely equal in the eyes
// of some comparison written here — it addresses identically, bit for bit, and
// re-encodes to the same bytes. That last is the strong form: a fixed point
// means the encoder, the decoder and the sort all agree, and a defect in any
// one of the three moves the second file.
func TestARecordSurvivesBeingHandedToAnotherProcess(t *testing.T) {
	s, _, _ := record(t)
	first := wrote(t, s)

	back, err := ReadStore(bytes.NewReader(first))
	if err != nil {
		t.Fatalf("ReadStore: %v", err)
	}
	if back.Len() != s.Len() {
		t.Fatalf("read back %d bits, wrote %d", back.Len(), s.Len())
	}
	if back.Address() != s.Address() {
		t.Errorf("the record read back addresses %s, wrote %s",
			Short(back.Address()), Short(s.Address()))
	}

	// Reaching into the field, as reach_test.go does and for its reason: a
	// Store has no enumeration on purpose, because nothing in the product walks
	// the record except by following edges.
	s.mu.RLock()
	ids := slices.Sorted(maps.Keys(s.bits))
	s.mu.RUnlock()
	for _, id := range ids {
		got, ok := back.Get(id)
		if !ok {
			t.Errorf("the record read back does not hold %s", Short(id))
			continue
		}
		// Re-addressed here rather than trusting the loader's own check, so
		// this test still fails if that check is removed.
		if again := ID(got); again != id {
			t.Errorf("bit %s came back addressing %s", Short(id), Short(again))
		}
	}

	if second := wrote(t, back); !bytes.Equal(first, second) {
		t.Errorf("a round trip is not a fixed point: %d bytes out, %d bytes back",
			len(first), len(second))
	}
}

// Go randomizes map iteration, so a walk of the store that did not sort would
// give one record a different file on every write. That is the order fragility
// gob and JSON were refused for, and it would arrive here instead.
func TestOneRecordIsOneFile(t *testing.T) {
	s, _, _ := record(t)

	want := wrote(t, s)
	for range 20 {
		if got := wrote(t, s); !bytes.Equal(got, want) {
			t.Fatal("two writes of one record produced two different files")
		}
	}

	// Same bits, put in the reverse order. A record is a set, so it is one
	// file whichever way it was assembled.
	s.mu.RLock()
	bits := make([]Bit, 0, len(s.bits))
	for _, b := range s.bits {
		bits = append(bits, b)
	}
	s.mu.RUnlock()
	slices.Reverse(bits)

	other := NewStore()
	for _, b := range bits {
		other.Put(b)
	}
	if got := wrote(t, other); !bytes.Equal(got, want) {
		t.Error("the same bits put in a different order produced a different file")
	}
}

func TestEveryPayloadSurvivesTheRoundTrip(t *testing.T) {
	cold := Cool([]Bit{said(0, "tyler", "the deploy failed"), said(1, "persona", "which one")})
	target := said(2, "tyler", "the first")

	tests := []struct {
		name string
		bit  Bit
	}{
		{"an utterance", said(0, "tyler", "the deploy failed")},
		{"an utterance with no text", said(0, "tyler", "")},
		{"an utterance carrying newlines and a tag", said(0, "tyler", "line\nbit\x00\"prev\"")},
		{"an utterance in another script", said(0, "tyler", "デプロイが失敗した")},
		{"a fragment", func() Bit {
			b := said(0, "persona", "the disk filled because")
			b.Payload = Utterance{Text: "the disk filled because", Truncated: true}
			b.ID = ID(b)
			return b
		}()},
		{"an upvote", Cast(at(3), Handle{Ref: "tyler", Display: "Tyler"}, Up, target)},
		{"a downvote", Cast(at(3), Handle{Ref: "tyler", Display: "Tyler"}, Down, target)},
		{"a compaction", cold},
		{"a compaction of a compaction", Cool([]Bit{cold, said(5, "tyler", "and again")})},
		{"a bare compaction", func() Bit {
			b := base()
			b.Payload = Compaction{}
			b.ID = ID(b)
			return b
		}()},
		{"a root bit with no parents", func() Bit {
			b := said(0, "tyler", "first")
			b.Prev = nil
			b.ID = ID(b)
			return b
		}()},
		{"a join naming three parents", func() Bit {
			b := said(0, "tyler", "joined", "a", "b", "c")
			b.ID = ID(b)
			return b
		}()},
		{"a bit at the zero instant", func() Bit {
			b := said(0, "tyler", "before everything")
			b.At = time.Time{}
			b.ID = ID(b)
			return b
		}()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewStore()
			s.Put(tt.bit)

			back, err := ReadStore(bytes.NewReader(wrote(t, s)))
			if err != nil {
				t.Fatalf("ReadStore: %v", err)
			}

			got, ok := back.Get(tt.bit.ID)
			if !ok {
				t.Fatalf("the record read back does not hold %s", Short(tt.bit.ID))
			}
			// The address is the identity, so this is the comparison that
			// counts; a decoder that got the payload wrong cannot reach here.
			if again := ID(got); again != tt.bit.ID {
				t.Errorf("came back addressing %s, want %s", Short(again), Short(tt.bit.ID))
			}
			if got.Payload.kind() != tt.bit.Payload.kind() {
				t.Errorf("came back a %q, want a %q", got.Payload.kind(), tt.bit.Payload.kind())
			}
		})
	}
}

// The two payloads whose value reaches the address through kind alone are the
// two D26 is about, and recovering them is the whole reason the decoder can be
// written at all. Asserted on the field rather than on the address, because the
// address would agree with itself even if both sides had lost the flag.
func TestTheDecoderRecoversWhatOnlyTheTagCarries(t *testing.T) {
	target := said(2, "tyler", "the first")
	tyler := Handle{Ref: "tyler", Display: "Tyler"}

	s := NewStore()
	whole := s.Put(said(0, "persona", "the disk filled"))
	piece := s.Put(func() Bit {
		b := Bit{At: at(1), From: Handle{Ref: "persona", Display: "persona"}, Channel: "tui",
			Payload: Utterance{Text: "the disk filled", Truncated: true}}
		return b
	}())
	s.Put(target)
	up := s.Put(Cast(at(3), tyler, Up, target))
	down := s.Put(Cast(at(4), tyler, Down, target))

	// The same text, cut off and not. If the flag did not survive, these two
	// would be one bit — which is exactly the collision D26 exists about.
	if whole.ID == piece.ID {
		t.Fatal("the fixture is wrong: a fragment and its complete twin share an address")
	}

	back, err := ReadStore(bytes.NewReader(wrote(t, s)))
	if err != nil {
		t.Fatalf("ReadStore: %v", err)
	}

	tests := []struct {
		name string
		id   string
		want any
	}{
		{"a complete utterance", whole.ID, Utterance{Text: "the disk filled"}},
		{"a fragment", piece.ID, Utterance{Text: "the disk filled", Truncated: true}},
		{"an upvote", up.ID, Vote{dir: Up}},
		{"a downvote", down.ID, Vote{dir: Down}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, ok := back.Get(tt.id)
			if !ok {
				t.Fatalf("the record read back does not hold %s", Short(tt.id))
			}
			if b.Payload != tt.want {
				t.Errorf("payload = %#v, want %#v", b.Payload, tt.want)
			}
		})
	}
}

// A compaction is the payload with something to lose: six blocks, four of them
// collections, and nothing in the aggregates is re-derivable from the store
// once the window that produced them is only a list of addresses.
func TestACompactionKeepsEveryAggregate(t *testing.T) {
	s, _, _ := record(t)

	var scars []Bit
	for _, b := range s.bits {
		if _, cold := b.Payload.(Compaction); cold {
			scars = append(scars, b)
		}
	}
	slices.SortFunc(scars, func(a, b Bit) int { return strings.Compare(a.ID, b.ID) })
	if len(scars) < 2 {
		t.Fatalf("the fixture holds %d scars; it is meant to hold a scar of a scar", len(scars))
	}

	back, err := ReadStore(bytes.NewReader(wrote(t, s)))
	if err != nil {
		t.Fatalf("ReadStore: %v", err)
	}

	for _, scar := range scars {
		got, ok := back.Get(scar.ID)
		if !ok {
			t.Fatalf("the record read back does not hold scar %s", Short(scar.ID))
		}
		was, is := scar.Payload.(Compaction), got.Payload.(Compaction)

		if is.count != was.count {
			t.Errorf("scar %s: count %d, want %d", Short(scar.ID), is.count, was.count)
		}
		if !is.from.Equal(was.from) || !is.to.Equal(was.to) {
			t.Errorf("scar %s: span %s–%s, want %s–%s", Short(scar.ID),
				is.from, is.to, was.from, was.to)
		}
		if !slices.Equal(is.handles, was.handles) {
			t.Errorf("scar %s: handles %v, want %v", Short(scar.ID), is.handles, was.handles)
		}
		if !slices.Equal(is.absorbed, was.absorbed) {
			t.Errorf("scar %s: absorbed %d ids, want %d", Short(scar.ID),
				len(is.absorbed), len(was.absorbed))
		}
		for _, m := range []struct {
			name    string
			is, was map[string]int
		}{
			{"kinds", is.kinds, was.kinds},
			{"bag", is.bag, was.bag},
		} {
			if len(m.is) != len(m.was) {
				t.Errorf("scar %s: %s has %d keys, want %d",
					Short(scar.ID), m.name, len(m.is), len(m.was))
			}
			for k, n := range m.was {
				if m.is[k] != n {
					t.Errorf("scar %s: %s[%q] = %d, want %d",
						Short(scar.ID), m.name, k, m.is[k], n)
				}
			}
		}
	}

	// The outer scar has to be one that actually merged a fragment and words,
	// or the loop above is comparing two empty maps and agreeing.
	outer := scars[0].Payload.(Compaction)
	for _, scar := range scars {
		if c := scar.Payload.(Compaction); c.count > outer.count {
			outer = c
		}
	}
	if outer.kinds["fragment"] == 0 || len(outer.bag) == 0 || len(outer.handles) < 2 {
		t.Errorf("the fixture's largest scar carries kinds %v, %d words, %d handles; "+
			"it is meant to have merged material from both speakers",
			outer.kinds, len(outer.bag), len(outer.handles))
	}
}

// Every single-*bit* corruption is caught, everywhere in the stream.
//
// The sweep flips each of the eight bits of each byte separately rather than
// XORing whole bytes, and the difference is not pedantry: it is what found the
// hole this test used to have. An earlier version XORed 0xff, and against a
// record of 25 bits that mutation can only ever *raise* the bit count, which
// runs the stream out and errors. Lowering it by one — a single bit, 25 → 24 —
// dropped the last bit of the record and returned a shorter one with a nil
// error. The comment above it claimed the catch was "a property of the design
// rather than of these particular bytes", and it was a property of which
// mutation the test happened to apply. A closing tag is what makes the sentence
// true; this sweep is what makes it checkable.
// Swept over the small pinned fixture rather than over record(), because the
// suite is this project's main instrument and cmd/seam runs it upwards of sixty
// times per catalog: exhaustive over 25 bits cost 3.7s, and over framed()'s five
// it costs a fiftieth of that for the same property. Nothing structural is lost
// — the header, the count, the closing tag, a filed address, all three payload
// kinds and a scar are all in framed(). Breadth over payload shapes is the next
// test's job, at one flip per byte.
func TestASingleFlippedBitIsAlwaysCaught(t *testing.T) {
	s, _ := framed()

	var buf bytes.Buffer
	if _, err := s.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	sound := buf.Bytes()

	for i := range sound {
		for bit := range 8 {
			bent := bytes.Clone(sound)
			bent[i] ^= 1 << bit

			got, err := ReadStore(bytes.NewReader(bent))
			if err == nil {
				t.Fatalf("bit %d of byte %d of %d flipped and the record loaded clean, holding %d bits",
					bit, i, len(sound), got.Len())
			}
		}
	}
}

// The same sweep at one flip per byte, over a record with twenty-five bits, two
// generations of folding and every payload the package has. This is the breadth
// half: the test above proves the property exhaustively on a small stream, and
// this one proves it is not an artifact of that stream's shape.
func TestNoByteOfARealRecordCanBeFlippedUnnoticed(t *testing.T) {
	s, _, _ := record(t)
	sound := wrote(t, s)

	for i := range sound {
		bent := bytes.Clone(sound)
		bent[i] ^= 0xff

		got, err := ReadStore(bytes.NewReader(bent))
		if err == nil {
			t.Fatalf("byte %d of %d flipped and the record loaded clean, holding %d bits",
				i, len(sound), got.Len())
		}
	}
}

// The specific corruption that got past the earlier sweep, kept as its own row
// so the regression has a name rather than living inside a loop of thousands.
// A record whose count is read one too low must not come back one bit short.
func TestACountReadOneTooLowIsRefused(t *testing.T) {
	s, _, _ := record(t)
	sound := wrote(t, s)

	// The count is the fourth field: two length-prefixed tags then the version,
	// each number eight bytes big-endian, so its last byte is the low one.
	countAt := 8 + len(magic) + 8 + len(storeMark) + 8 + 7
	if sound[countAt] != byte(s.Len()) {
		t.Fatalf("byte %d is %d, want the record's own count %d", countAt, sound[countAt], s.Len())
	}

	for _, delta := range []int{-1, +1} {
		bent := bytes.Clone(sound)
		bent[countAt] = byte(s.Len() + delta)

		got, err := ReadStore(bytes.NewReader(bent))
		if err == nil {
			t.Fatalf("a count of %d against %d bits loaded clean, holding %d",
				s.Len()+delta, s.Len(), got.Len())
		}
	}
}

// The message has to name both addresses. An error that says only "corrupt"
// leaves an auditor with a record they cannot ask a question about; both
// addresses turn it into two lookups, one of which resolves.
//
// It also pins the half of [ReadStore]'s check that is actually load-bearing:
// that a damaged record comes back as an error rather than as a panic. Delete
// the comparison in ReadStore and [Store.Put] still refuses the bit — from
// inside, by panicking — so this test calling ReadStore in the ordinary way and
// reading its error is the assertion, and a panic here is a red like any other.
func TestACorruptedBitNamesBothAddresses(t *testing.T) {
	s := NewStore()
	b := s.Put(said(0, "tyler", "the deploy failed"))
	sound := wrote(t, s)

	at := bytes.Index(sound, []byte("failed"))
	if at < 0 {
		t.Fatal("the fixture's text is not in its own stream")
	}
	bent := bytes.Clone(sound)
	bent[at] = 'F'

	_, err := ReadStore(bytes.NewReader(bent))
	if err == nil {
		t.Fatal("a bit whose text was edited loaded clean")
	}
	if !strings.Contains(err.Error(), Short(b.ID)) {
		t.Errorf("error %q does not name the address it was filed under, %s", err, Short(b.ID))
	}

	// And the address the edited bytes actually have, which is the other half:
	// one of the two resolves in some record and the other does not, so an
	// auditor holding both knows which question to ask.
	edited := b
	edited.Payload = Utterance{Text: "the deploy Failed"}
	if want := Short(ID(edited)); !strings.Contains(err.Error(), want) {
		t.Errorf("error %q does not name what the bytes address, %s", err, want)
	}
}

// A stream that stops early must say so rather than hand back the bits that did
// arrive. A short record is the failure this whole design exists to refuse: it
// loads clean, it renders, and nothing anywhere says a conversation is missing
// its end.
func TestATruncatedStreamIsRefused(t *testing.T) {
	s, _, _ := record(t)
	sound := wrote(t, s)

	for n := range len(sound) {
		got, err := ReadStore(bytes.NewReader(sound[:n]))
		if err == nil {
			t.Fatalf("a stream cut to %d of %d bytes loaded %d bits", n, len(sound), got.Len())
		}
		if !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatalf("a stream cut to %d of %d bytes failed with %v, want an unexpected EOF",
				n, len(sound), err)
		}
	}
}

// A payload kind this build does not know cannot be skipped past: there is no
// length in front of a payload, so a reader that does not know the kind does
// not know where it ends.
func TestAnUnknownPayloadTagIsRefused(t *testing.T) {
	s := NewStore()
	s.Put(said(0, "tyler", "the deploy failed"))
	sound := wrote(t, s)

	// "utterance" and "quotation" are the same length, so the frame stays
	// intact and the refusal is about the name rather than about the shape.
	bent := bytes.Replace(sound, []byte("utterance"), []byte("quotation"), 1)
	if bytes.Equal(bent, sound) {
		t.Fatal("the fixture's own kind tag is not in its stream")
	}

	_, err := ReadStore(bytes.NewReader(bent))
	if err == nil {
		t.Fatal("a payload named by a tag this build does not know loaded clean")
	}
	if !strings.Contains(err.Error(), "quotation") {
		t.Errorf("error %q does not name the kind it refused", err)
	}
}

func TestAStreamOfTheWrongKindIsRefused(t *testing.T) {
	s, shown, _ := record(t)

	var store, view bytes.Buffer
	if _, err := s.WriteTo(&store); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	if _, err := shown.WriteAgainst(s, &view); err != nil {
		t.Fatalf("WriteAgainst: %v", err)
	}

	bumped := bytes.Clone(store.Bytes())
	// The version is the third field: two length-prefixed tags of 9 and 5
	// bytes, each with an eight-byte length in front, then eight bytes of
	// number. Bumping its last byte is a stream from a build that does not
	// exist yet.
	bumped[8+len(magic)+8+len(storeMark)+7]++

	tests := []struct {
		name, want string
		read       func([]byte) error
	}{
		{"a view read as a record", `not a "` + storeMark + `"`,
			func(b []byte) error { _, err := ReadStore(bytes.NewReader(b)); return err }},
		{"a record read as a view", `not a "` + viewMark + `"`,
			func(b []byte) error { _, err := ReadViewAgainst(s, bytes.NewReader(b)); return err }},
		{"something else entirely", "not a tldreddit stream",
			func(b []byte) error { _, err := ReadStore(bytes.NewReader(b)); return err }},
		{"a newer version", "version",
			func(b []byte) error { _, err := ReadStore(bytes.NewReader(b)); return err }},
	}
	bytesFor := map[string][]byte{
		"a view read as a record": view.Bytes(),
		"a record read as a view": store.Bytes(),
		"something else entirely": []byte("PK\x03\x04this is a zip file, honestly, and then some more of it"),
		"a newer version":         bumped,
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.read(bytesFor[tt.name])
			if err == nil {
				t.Fatal("loaded clean")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error %q does not contain %q", err, tt.want)
			}
		})
	}
}

// A view is what a reader is shown, and what it has let go of is not a function
// of the record, so this is the half that has to be kept rather than re-derived.
func TestAViewSurvivesTheRoundTrip(t *testing.T) {
	s, shown, votes := record(t)

	tests := []struct {
		name string
		view View
	}{
		{"a folded transcript", shown},
		{"a vote view", votes},
		{"empty", View{}},
		{"nil", nil},
		// Nothing stops a view naming one address twice, and its order is its
		// meaning — [Stay.Votes] settles a tie by which comes later.
		{"one address twice", View{shown[0], shown[0], shown[1]}},
		{"the same addresses reversed", func() View {
			v := slices.Clone(shown)
			slices.Reverse(v)
			return v
		}()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			n, err := tt.view.WriteAgainst(s, &buf)
			if err != nil {
				t.Fatalf("WriteAgainst: %v", err)
			}
			if n != int64(buf.Len()) {
				t.Errorf("WriteAgainst reported %d bytes and wrote %d", n, buf.Len())
			}

			back, err := ReadViewAgainst(s, bytes.NewReader(buf.Bytes()))
			if err != nil {
				t.Fatalf("ReadViewAgainst: %v", err)
			}
			if !slices.Equal(back, tt.view) {
				t.Errorf("view came back %v, want %v", back, tt.view)
			}
		})
	}
}

// The amendment's own null hypothesis, run rather than reasoned. [View.Bits]
// panics on an address the store does not hold, so the question is whether it
// already covers a stale view — and it does not, because a stale view names
// nothing that is missing. It names too little.
func TestALargerRecordRendersAStaleViewWithoutComplaint(t *testing.T) {
	s, shown, _ := record(t)
	stale := slices.Clone(shown)

	grown, _ := shown.Add(s, said(99, "tyler", "and then everything after", shown.Head()...))
	if len(grown) == len(stale) {
		t.Fatal("the fixture did not grow")
	}

	// No panic, no error, nothing. The reader is shown a conversation that is
	// missing everything recorded since the view was saved.
	if got := len(stale.Bits(s)); got != len(stale) {
		t.Fatalf("Bits resolved %d of %d addresses", got, len(stale))
	}
}

// A view's own bytes are covered by its seal, and nothing else covers them: a
// View is a list of strings, and no content address reaches a list. The case
// that made this mandatory rather than tidy is the vote view — [Stay].Votes
// decides what a fold keeps, so a silently shortened one lifts holds and the
// next fold takes material somebody voted to keep.
// It asserts *which* error, and that nothing came back, because the version of
// this test that asserted only `err != nil` is what let a real defect through a
// review. [ReadViewAgainst] used to test provenance first and compute the seal
// from the live store, so a bit flipped in the stream's own record address was
// reported as a view from another session and handed out through [StaleView] —
// with its integrity never tested, and the recovery door open onto a damaged
// view. Every assertion below was green throughout.
//
// The property, stated so it can be broken: no single-bit corruption of a valid
// stream may produce a [StaleView]. A StaleView means the bytes are sound and
// the record is not ours, and corruption cannot honestly say that — the seal
// covers the address field, so damaging it damages the seal.
func TestASingleFlippedBitInAViewIsAlwaysCaught(t *testing.T) {
	s, _, votes := record(t)

	var buf bytes.Buffer
	if _, err := votes.WriteAgainst(s, &buf); err != nil {
		t.Fatalf("WriteAgainst: %v", err)
	}
	sound := buf.Bytes()

	for i := range sound {
		for bit := range 8 {
			bent := bytes.Clone(sound)
			bent[i] ^= 1 << bit

			got, err := ReadViewAgainst(s, bytes.NewReader(bent))
			if err == nil {
				t.Fatalf("bit %d of byte %d of %d flipped and the view loaded clean as %v",
					bit, i, len(sound), got)
			}
			if got != nil {
				t.Fatalf("bit %d of byte %d: a damaged view came back anyway: %v", bit, i, got)
			}

			var stale *StaleView
			if errors.As(err, &stale) {
				t.Fatalf("bit %d of byte %d: damage was reported as a view from another record, "+
					"and %d addresses went out through the recovery door unchecked",
					bit, i, len(stale.View))
			}
		}
	}
}

// The specific one, named: a view whose length is read one too low used to come
// back short and silent. For a transcript that is a lost row; for a vote view it
// is a lifted hold.
func TestAViewLengthReadOneTooLowIsRefused(t *testing.T) {
	s, _, votes := record(t)

	var buf bytes.Buffer
	if _, err := votes.WriteAgainst(s, &buf); err != nil {
		t.Fatalf("WriteAgainst: %v", err)
	}
	sound := buf.Bytes()

	// The length is the fifth field: magic, mark, version, the record address
	// and the seal, each string carrying an eight-byte length in front.
	countAt := 8 + len(magic) + 8 + len(viewMark) + 8 + 8 + 64 + 8 + 64 + 7
	if sound[countAt] != byte(len(votes)) {
		t.Fatalf("byte %d is %d, want the view's own length %d", countAt, sound[countAt], len(votes))
	}

	bent := bytes.Clone(sound)
	bent[countAt] = byte(len(votes) - 1)
	got, err := ReadViewAgainst(s, bytes.NewReader(bent))
	if err == nil {
		t.Fatalf("a view of %d addresses declaring %d loaded clean as %v", len(votes), len(votes)-1, got)
	}
	if !strings.Contains(err.Error(), "damaged") {
		t.Errorf("error %q does not say the bytes are damaged", err)
	}
	if got != nil {
		t.Errorf("a damaged view was handed back anyway: %v", got)
	}
}

// A safety check must not become a way to lose data. The arrangement is
// ordinary: a checkpoint writes the record, the process dies before it writes
// the view, and the next session grows the record past the view that survived.
// A view is the one thing here that cannot be re-derived, so refusing to hand it
// back at all would destroy it in the name of protecting it.
func TestAStaleViewIsRefusedAndStillRecoverable(t *testing.T) {
	s, shown, _ := record(t)

	var buf bytes.Buffer
	if _, err := shown.WriteAgainst(s, &buf); err != nil {
		t.Fatalf("WriteAgainst: %v", err)
	}
	was := s.Address()
	shown.Add(s, said(99, "tyler", "and then everything after", shown.Head()...))

	v, err := ReadViewAgainst(s, bytes.NewReader(buf.Bytes()))
	if err == nil {
		t.Fatal("a view taken against an earlier record loaded clean")
	}

	// The default path yields nothing, which is the point of the shape: a
	// caller who ignores the error cannot end up holding another session's
	// view by accident.
	if v != nil {
		t.Errorf("the plain return handed back a stale view: %v", v)
	}

	var stale *StaleView
	if !errors.As(err, &stale) {
		t.Fatalf("error %v is not a *StaleView; there is no way to get the view back", err)
	}
	if !slices.Equal(stale.View, shown) {
		t.Errorf("recovered %v, want the view that was written, %v", stale.View, shown)
	}
	if stale.Against != was || stale.Record != s.Address() {
		t.Errorf("StaleView{Against: %s, Record: %s}, want {%s, %s}",
			Short(stale.Against), Short(stale.Record), Short(was), Short(s.Address()))
	}
}

func TestAViewTakenAgainstAnotherRecordIsRefused(t *testing.T) {
	s, shown, _ := record(t)

	var buf bytes.Buffer
	if _, err := shown.WriteAgainst(s, &buf); err != nil {
		t.Fatalf("WriteAgainst: %v", err)
	}
	was := s.Address()

	// The case [View.Bits] cannot see: the record grew, so every address in the
	// saved view still resolves and rendering it is silent.
	shown.Add(s, said(99, "tyler", "and then everything after", shown.Head()...))
	if s.Address() == was {
		t.Fatal("the record's address did not move when a bit was added")
	}

	_, err := ReadViewAgainst(s, bytes.NewReader(buf.Bytes()))
	if err == nil {
		t.Fatal("a view taken against an earlier record loaded clean")
	}
	for _, want := range []string{Short(was), Short(s.Address())} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %s", err, want)
		}
	}
}

// A record's address is a function of what it holds and of nothing else — not
// of the order things were put in, and not of which process is asking.
func TestARecordsAddressIsItsContents(t *testing.T) {
	s, _, _ := record(t)

	s.mu.RLock()
	bits := make([]Bit, 0, len(s.bits))
	for _, b := range s.bits {
		bits = append(bits, b)
	}
	s.mu.RUnlock()

	other := NewStore()
	slices.Reverse(bits)
	for _, b := range bits {
		other.Put(b)
	}
	if other.Address() != s.Address() {
		t.Errorf("the same bits in another order address %s, want %s",
			Short(other.Address()), Short(s.Address()))
	}

	empty := NewStore()
	if empty.Address() == s.Address() {
		t.Error("an empty record addresses as a full one")
	}
	if NewStore().Address() != empty.Address() {
		t.Error("two empty records address differently")
	}

	// Every bit added has to move it, or a view stamp cannot notice a record
	// that grew.
	seen := map[string]bool{s.Address(): true}
	v := View{}
	for i := range 5 {
		v, _ = v.Add(other, said(100+i, "tyler", "another", v.Head()...))
		if seen[other.Address()] {
			t.Fatalf("adding bit %d left the record's address where it was", i)
		}
		seen[other.Address()] = true
	}
}

// A record and its views are one checkpoint. Each stream says where it ends, so
// they concatenate — which is what makes "one file or several" the caller's
// question rather than this package's.
func TestARecordAndItsViewsShareOneStream(t *testing.T) {
	s, shown, votes := record(t)

	var buf bytes.Buffer
	if _, err := s.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	for _, v := range []View{shown, votes} {
		if _, err := v.WriteAgainst(s, &buf); err != nil {
			t.Fatalf("WriteAgainst: %v", err)
		}
	}

	r := bytes.NewReader(buf.Bytes())
	back, err := ReadStore(r)
	if err != nil {
		t.Fatalf("ReadStore: %v", err)
	}
	for _, want := range []View{shown, votes} {
		got, err := ReadViewAgainst(back, r)
		if err != nil {
			t.Fatalf("ReadViewAgainst: %v", err)
		}
		if !slices.Equal(got, want) {
			t.Errorf("view came back %v, want %v", got, want)
		}
	}
	if r.Len() != 0 {
		t.Errorf("%d bytes left over after three streams", r.Len())
	}

	// And the whole thing still works: the reloaded record answers the same
	// questions the live one did, including the reachability walk D14 is about.
	if got := len(reachable(t, back, shown, votes)); got != back.Len() {
		t.Errorf("the reloaded record reaches %d of its %d bits", got, back.Len())
	}
	if len(Tally(back, votes)) != len(Tally(s, votes)) {
		t.Error("the reloaded record tallies differently")
	}
}

// Replaying a stream is safe, because Put is idempotent: the second pass
// re-addresses every bit to where it already is and files nothing.
func TestReplayingAStreamChangesNothing(t *testing.T) {
	s, _, _ := record(t)
	stream := wrote(t, s)

	back, err := ReadStore(bytes.NewReader(stream))
	if err != nil {
		t.Fatalf("ReadStore: %v", err)
	}
	again, err := ReadStore(bytes.NewReader(stream))
	if err != nil {
		t.Fatalf("ReadStore, second pass: %v", err)
	}

	was := back.Len()
	for _, b := range again.bits {
		back.Put(b)
	}
	if back.Len() != was {
		t.Errorf("replaying grew the record from %d bits to %d", was, back.Len())
	}
	if back.Address() != s.Address() {
		t.Errorf("replaying moved the record's address to %s, want %s",
			Short(back.Address()), Short(s.Address()))
	}
}

// handWritten builds a record stream field by field, so a test can put in one
// what WriteTo would never produce. Byte-splicing a real stream was how this
// was done first, and it broke the moment the frame grew a closing tag —
// building through canon means these cases move with the format instead.
//
// count is passed separately from the entries so a stream can disagree with
// itself, which is the whole point of several of them.
func handWritten(count int64, filed []string, bits []Bit) []byte {
	var buf bytes.Buffer
	c := canon{w: &buf}
	c.tag(magic)
	c.tag(storeMark)
	c.num(version)

	c.num(count)
	for i, b := range bits {
		c.str(filed[i])
		writeBit(&c, b)
	}
	c.tag(endMark)
	return buf.Bytes()
}

// A stream naming one address twice was assembled by something other than
// WriteTo, which walks a map. Refusing keeps the invariant that a record read
// back holds exactly as many bits as its stream declared.
func TestAStreamThatRepeatsAnAddressIsRefused(t *testing.T) {
	b := said(0, "tyler", "the deploy failed")
	bent := handWritten(2, []string{b.ID, b.ID}, []Bit{b, b})

	_, err := ReadStore(bytes.NewReader(bent))
	if err == nil {
		t.Fatal("a stream naming one address twice loaded clean")
	}
	if !strings.Contains(err.Error(), Short(b.ID)) {
		t.Errorf("error %q does not name the repeated address %s", err, Short(b.ID))
	}
}

// The one damage [Store.Put]'s own guard cannot see, because it is gated on a
// non-empty [Bit].ID: a stream filing a bit under the empty address. Without
// the comparison in ReadStore this loads clean and files the bit under whatever
// it really addresses to — a record holding something nothing asked it to hold.
func TestAStreamFilingABitUnderNoAddressIsRefused(t *testing.T) {
	b := said(0, "tyler", "the deploy failed")
	bent := handWritten(1, []string{""}, []Bit{b})

	_, err := ReadStore(bytes.NewReader(bent))
	if err == nil {
		t.Fatal("a bit filed under the empty address loaded clean")
	}
	if !strings.Contains(err.Error(), Short(b.ID)) {
		t.Errorf("error %q does not name what the bytes address, %s", err, Short(b.ID))
	}
}

// A compaction whose aggregates disagree with its count is refused on the way
// in. Nothing that came through [Cool] can fail this, which is why checking it
// cannot orphan a record anything ever wrote; what it stops is a number a
// caller would size work from arriving from somewhere else. `tui.recall` drew
// one row per absorbed bit and sized its slice from Count, and a hand-written
// count of a trillion killed the process in the allocator.
func TestACompactionThatDisagreesWithItsCountIsRefused(t *testing.T) {
	sound := Cool([]Bit{
		said(0, "tyler", "the deploy failed"),
		said(1, "persona", "which deploy"),
	})

	bend := func(edit func(*Compaction)) Bit {
		b := sound
		c := b.Payload.(Compaction)
		c.handles = slices.Clone(c.handles)
		c.kinds, c.bag = maps.Clone(c.kinds), maps.Clone(c.bag)
		c.absorbed = slices.Clone(c.absorbed)
		edit(&c)
		b.Payload = c
		// Re-addressed, so the bit is filed under what it really says and the
		// refusal is about the aggregates rather than about a broken address.
		b.ID = ""
		b.ID = ID(b)
		return b
	}

	tests := []struct {
		name, want string
		bit        Bit
	}{
		{"a count larger than the receipt", "naming 2 ids and counting 1099511627776",
			bend(func(c *Compaction) { c.count = 1 << 40 })},
		{"a receipt longer than the count", "naming 3 ids and counting 2",
			bend(func(c *Compaction) { c.absorbed = append(c.absorbed, "a") })},
		{"kinds that do not sum to the count", "kinds account for 1",
			bend(func(c *Compaction) { c.kinds = map[string]int{"utterance": 1} })},
		// Negatives cancel, so this one sums to the right number and would pass
		// a check that only adds. Cool cannot produce it; a stream can spell it.
		{"a negative count for a kind", `-5 bits of kind "utterance"`,
			bend(func(c *Compaction) { c.kinds = map[string]int{"utterance": -5, "fragment": 7} })},
		// A receipt entry that resolves to nothing. Every other aggregate
		// agrees, so only the entry itself can report this.
		{"a receipt naming nothing", "names nothing at entry 1 of 2",
			bend(func(c *Compaction) { c.absorbed[0] = "" })},
		{"a span running backwards", "runs backwards",
			bend(func(c *Compaction) { c.from, c.to = c.to, c.from })},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bent := handWritten(1, []string{tt.bit.ID}, []Bit{tt.bit})

			got, err := ReadStore(bytes.NewReader(bent))
			if err == nil {
				t.Fatalf("loaded clean, holding %d bits", got.Len())
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error %q does not contain %q", err, tt.want)
			}
		})
	}

	// The control: the same compaction untouched goes in without complaint, so
	// the six rows above are about the aggregates and not about the frame. (This
	// said "the four rows above" while there were six, from the commit that wrote
	// it; corrected rather than deleted, because a count in a comment that no
	// check holds up is the defect this file keeps finding in records.)
	if _, err := ReadStore(bytes.NewReader(handWritten(1, []string{sound.ID}, []Bit{sound}))); err != nil {
		t.Errorf("a compaction straight out of Cool was refused: %v", err)
	}

	// And the boundary the rows above sit next to rather than on: the guard is
	// against a *negative* count for a kind, and zero is not a corruption. Nothing
	// [Cool] produces carries one — it counts up from nothing and never writes a
	// key it did not increment — so this is a record another writer could spell
	// and ours cannot, which is the only kind of record the decoder exists for.
	//
	// Written because `go-gremlins` found `n < 0` could be tightened to `n <= 0`
	// with the suite green: the strict version refuses this stream, and no check
	// in the package had an opinion about it either way.
	zero := bend(func(c *Compaction) { c.kinds = map[string]int{"utterance": 2, "fragment": 0} })
	if _, err := ReadStore(bytes.NewReader(handWritten(1, []string{zero.ID}, []Bit{zero}))); err != nil {
		t.Errorf("a compaction counting zero bits of a kind was refused: %v", err)
	}
}

// The zone is not in the record and the instant is. That was already true of
// the address ([TestIDNormalizesTime]); what is new is that it is now
// observable — a bit read back reads UTC, whatever zone it was captured in.
func TestAReloadedInstantKeepsTheMomentAndLosesTheZone(t *testing.T) {
	tokyo := time.FixedZone("JST", 9*60*60)

	b := said(0, "tyler", "the deploy failed")
	b.At = at(0).In(tokyo)
	b.ID = ID(b)

	s := NewStore()
	s.Put(b)

	back, err := ReadStore(bytes.NewReader(wrote(t, s)))
	if err != nil {
		t.Fatalf("ReadStore: %v", err)
	}
	got, ok := back.Get(b.ID)
	if !ok {
		t.Fatalf("the record read back does not hold %s", Short(b.ID))
	}

	if !got.At.Equal(b.At) {
		t.Errorf("came back at %s, want the instant %s", got.At, b.At)
	}
	if name, _ := got.At.Zone(); name != "UTC" {
		t.Errorf("came back in zone %q; a reloaded bit reads UTC", name)
	}
	if _, offset := b.At.Zone(); offset == 0 {
		t.Fatal("the fixture is wrong: it was not written in an offset zone")
	}
}

// halting fails after n bytes, the way a disk fills or a socket closes
// mid-write.
type halting struct {
	left int
}

func (h *halting) Write(p []byte) (int, error) {
	if len(p) <= h.left {
		h.left -= len(p)
		return len(p), nil
	}
	n := h.left
	h.left = 0
	return n, errors.New("no room")
}

// canon used to write only into a hash, which never fails, so nothing checked.
// A file and a socket both do, and a byte count that kept climbing past the
// failure would be a receipt for bytes that never landed.
func TestAWriteThatFailsPartWayIsReported(t *testing.T) {
	s, shown, _ := record(t)
	full := int64(len(wrote(t, s)))

	for _, stop := range []int{0, 1, 17, 200, int(full) - 1} {
		w := halting{left: stop}
		n, err := s.WriteTo(&w)
		if err == nil {
			t.Errorf("a writer that failed after %d bytes reported no error", stop)
		}
		if n != int64(stop) {
			t.Errorf("a writer that failed after %d bytes reported %d written", stop, n)
		}
	}

	w := halting{left: 3}
	if _, err := shown.WriteAgainst(s, &w); err == nil {
		t.Error("a view written to a failing writer reported no error")
	}
}

func TestAnEmptyRecordAndAnEmptyViewRoundTrip(t *testing.T) {
	s := NewStore()

	back, err := ReadStore(bytes.NewReader(wrote(t, s)))
	if err != nil {
		t.Fatalf("ReadStore: %v", err)
	}
	if back.Len() != 0 {
		t.Errorf("an empty record came back holding %d bits", back.Len())
	}

	var buf bytes.Buffer
	if _, err := View(nil).WriteAgainst(back, &buf); err != nil {
		t.Fatalf("WriteAgainst: %v", err)
	}
	v, err := ReadViewAgainst(back, bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("ReadViewAgainst: %v", err)
	}
	if len(v) != 0 {
		t.Errorf("an empty view came back holding %v", v)
	}
}

// framed is a fixture that must never change, in the way base() and coldBase()
// must never change. It is small and self-contained rather than built from
// record(), because record() is a convenience that will be edited and this is a
// golden: every payload once, a root and a join, and one bit whose channel
// differs, which is enough for the frame around them to be pinned.
func framed() (*Store, View) {
	s := NewStore()
	var v View

	v, first := v.Add(s, Bit{
		At:      at(0),
		From:    Handle{Ref: "tyler", Display: "Tyler"},
		Channel: "tui",
		Payload: Utterance{Text: "the deploy failed"},
	})
	v, piece := v.Add(s, Bit{
		At:      at(1),
		From:    Handle{Ref: "persona", Display: "persona"},
		Channel: "tui",
		Payload: Utterance{Text: "the disk filled because the log", Truncated: true},
		Prev:    []string{first.ID},
	})
	v, _ = v.Add(s, Cool([]Bit{first, piece}))
	s.Put(Cast(at(2), Handle{Ref: "tyler", Display: "Tyler"}, Up, piece))
	s.Put(Cast(at(3), Handle{Ref: "tyler", Display: "Tyler"}, Down, first))
	return s, v
}

// The frame is pinned the way an address is, and for a stronger reason.
//
// A bit's bytes are pinned transitively through the four golden addresses:
// change the encoding and TestIDIsPinned and its three siblings all move. The
// bytes *around* the bits are pinned by nothing at all. magic, the marks, the
// version, the closing tag, the order of the header fields, and the choice to
// write a bit's address before the bit rather than after it — every one of
// those can be edited without moving a single address, and every one of those
// makes every file already on disk permanently unreadable.
//
// The two tests that do header arithmetic are not this. They would be edited in
// the same hand as such a change, which is exactly what a golden must not be.
//
// If this fails, either the stream format changed on purpose — in which case
// every file anyone has written is a different format and that needs saying out
// loud, in the log, with a version bump — or it changed by accident. There is
// no third reading, and updating the constant to make a build go green is the
// one thing to not do.
func TestTheStreamFramingIsPinned(t *testing.T) {
	s, v := framed()

	tests := []struct {
		name, want string
		write      func(io.Writer) (int64, error)
	}{
		{"a record", "014b308342bb071e06662aeaeffefcb3df4253992b68c4c8d11eab8c05ca02f4",
			s.WriteTo},
		{"a view", "ae61dd7373157d522bd0a128a0c40cd4584b51ac91db131ae61ce0bd4ff80c4e",
			func(w io.Writer) (int64, error) { return v.WriteAgainst(s, w) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if _, err := tt.write(&buf); err != nil {
				t.Fatalf("writing: %v", err)
			}
			got := sha256.Sum256(buf.Bytes())
			if hex.EncodeToString(got[:]) != tt.want {
				t.Errorf("the stream hashes to %s, want %s",
					hex.EncodeToString(got[:]), tt.want)
			}
		})
	}
}
