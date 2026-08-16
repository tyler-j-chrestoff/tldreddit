package memory

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"
	"time"
)

// The wire format is the canonical encoding, and that is the whole design.
//
// [ID] hashes a bit by writing it through [canon]; a stream writes the same
// bits through the same [canon] into a file or a socket instead of into a hash.
// There is one encoding in this package, not two, so a change to the format
// moves every golden address in id_test.go, vote_test.go and fragment_test.go
// on the same run — a persistence format that drifted from the addressing
// format is a format that cannot be caught drifting, which is what gob and
// JSON were refused for in the first place ([ID]'s own doc).
//
// What that buys, and it is the reason to prefer this over anything else:
// **every bit is self-verifying.** [Bit].ID is deliberately not part of the
// encoding, so a reader decodes a bit, re-runs [ID] on what it reconstructed,
// and compares the answer against the address the record filed it under. A
// drifted encoder and a flipped byte inside a bit both fail there.
//
// Say exactly that much, because there is a gap and it was found by executing
// rather than by reading. The addresses cover the *bits*; they cover nothing
// around them. A record's bit count is not inside any bit, so lowering it by
// one used to drop the last bit and return a shorter record with a nil error —
// a receipt for a conversation with its end removed, which is this product's
// own failure mode inside its own storage. The frame is what closes that: a
// closing tag after the last bit (see [endMark]), so a count that does not
// match what follows it is refused rather than obeyed. A [View] is not covered
// by any of this either and carries its own hash, for the same reason found the
// same way — see [View.WriteAgainst].
//
// The general lesson, worth more than the fix: content addressing secures the
// contents and says nothing about the container. Every field this format adds
// outside a bit needs its own answer to "what notices when this changes".
//
// The second thing it buys is a proof rather than an argument. D26 permits a
// field to reach the content address through [Payload.kind] alone, on the
// condition that the value→name map is one-to-one — [Utterance].Truncated and
// [Vote]'s direction both do. [readPayload] is the inverse of that map, written
// out as a function that refuses a name it does not know. It can only exist
// while the condition holds: add a payload field that kind does not fully
// distinguish and the decoder becomes unwritable, which is the constraint
// making itself felt at compile time rather than as a lost bit two months
// later.
//
// Encoder and decoder live in one file, paired method by method, because an
// edit to one that is not mirrored in the other is the defect this format is
// most exposed to. Splitting them across files is how the two statements of one
// rule that [View.runs] exists to prevent would come back.

// magic marks a tldreddit stream, the mark says which of the two it is, and
// version is the encoding's own generation.
//
// [ID] argues against announcing the algorithm inside an address, and that
// argument still holds there: an address is read by a program that already
// knows what it is holding. A stream is not. It is opened by a program that did
// not write it, possibly a different program on a different machine, and the
// two streams here are structurally alike enough to be confused — a store's bit
// count would read as a view's length and hand back a plausible view of
// nonsense. So the mark earns its bytes by saying which stream this is, not
// merely that it is ours.
//
// version is a separate field rather than folded into the magic so a reader can
// tell "not ours" from "ours and newer than this build". There is still nothing
// to migrate to, and nothing here reads version except to refuse one it does
// not know.
//
// endMark closes a record, and it is the only thing standing between a damaged
// bit count and a silently short record. Nothing else can do that job here: a
// leftover-bytes check is unavailable by design, because these streams are
// self-delimiting so that a record and its views may sit in one file, which
// makes "the count said 24 and 25 entries are present" indistinguishable from
// "a record followed by a view". A tag can tell them apart, since a closing tag
// is three bytes of literal where a filed address would be sixty-four bytes of
// hex.
//
// None of these five strings may ever change. They are not addresses and no
// hash covers them, so an edit here is silent in every test that only checks a
// bit — see TestTheStreamFramingIsPinned, which exists precisely because the
// bits are pinned hard and the frame around them was pinned by nothing.
const (
	magic   = "tldreddit"
	version = 1

	storeMark = "store"
	viewMark  = "view"
	endMark   = "end"
)

// canon writes a canonical byte encoding.
//
// Two rules make it unambiguous, and both matter. Every variable-length piece
// is written with its length first, so no arrangement of field values can be
// mistaken for a different arrangement — without that, {Ref: "ab", Display:
// "c"} and {Ref: "a", Display: "bc"} would hash alike. And every composite is
// preceded by a literal tag naming what follows, so a payload can never be
// confused with a different payload that happens to encode to the same bytes.
//
// Tags come from Payload.kind, which is a hand-written literal rather than
// %T, because a type's printed name carries the package name with it: renaming
// this package would otherwise re-address every object in every store.
//
// Errors are sticky rather than returned per call, and the reason changed under
// this type. It used to write only into a hash.Hash, which documents that it
// never fails, so nothing checked. A file fills up and a socket closes, so that
// reasoning does not survive the format leaving the process. Sticky over
// returned because the alternative is an error check between every field of
// every payload, and that is where a missed one hides; here the first failure
// stops every later write and surfaces once, at the boundary, next to the byte
// count that says how far it got.
type canon struct {
	w   io.Writer
	n   int64
	err error
}

func (c *canon) write(p []byte) {
	if c.err != nil {
		return
	}
	n, err := c.w.Write(p)
	c.n += int64(n)
	if err != nil {
		c.err = err
	}
}

// tag marks what comes next. It is length-prefixed like any other string, so a
// tag can never be forged by a value that happens to spell it.
func (c *canon) tag(name string) { c.str(name) }

func (c *canon) str(s string) {
	c.num(int64(len(s)))
	c.write([]byte(s))
}

// num writes a fixed eight bytes, big-endian. Fixed width over varint because
// there is nothing to save here and one less encoding rule to get wrong.
func (c *canon) num(n int64) {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(n))
	c.write(buf[:])
}

// at encodes an instant as seconds and nanoseconds since the epoch in UTC.
//
// Normalizing to UTC is a deliberate claim: identity is the instant, not the
// zone it was displayed in, so the same moment recorded in Tokyo and in London
// is one bit. It also drops the monotonic clock reading that time.Now attaches,
// which is process-local and would otherwise make every ID unrepeatable.
//
// Now that the encoding is reversible, this is no longer only a claim about
// identity: the zone is not merely excluded from the address, it is not in the
// record at all, so a bit read back from a stream carries the instant and reads
// UTC. See [Bit].At, where a caller drawing a timestamp will meet it.
func (c *canon) at(t time.Time) {
	u := t.UTC()
	c.num(u.Unix())
	c.num(int64(u.Nanosecond()))
}

// strs writes a sequence, count first. Order is preserved rather than sorted:
// the caller's order is part of the value.
func (c *canon) strs(ss []string) {
	c.num(int64(len(ss)))
	for _, s := range ss {
		c.str(s)
	}
}

// counts writes a map of counts in sorted key order, because Go randomizes map
// iteration and an unordered walk would give the same map a different address
// on every run.
func (c *canon) counts(m map[string]int) {
	c.num(int64(len(m)))
	for _, k := range slices.Sorted(maps.Keys(m)) {
		c.str(k)
		c.num(int64(m[k]))
	}
}

// handles writes a sequence of actors, count first, each as its two strings.
// Not strs over a flattened pair: a Handle is two fields and flattening it
// would let a two-handle sequence and a four-string one encode alike.
func (c *canon) handles(hs []Handle) {
	c.num(int64(len(hs)))
	for _, h := range hs {
		c.str(h.Ref)
		c.str(h.Display)
	}
}

// Truncation is encoded through the tag and nowhere else: [Utterance.kind] is
// already "fragment" when the speaker did not finish, and a tag is
// length-prefixed like every other string, so no text can spell one and the two
// encodings cannot meet. Writing the flag a second time as a field of its own
// would add nothing the tag does not already say and would re-address every
// utterance ever written — a cost worth paying for information, not for
// reassurance.
func (u Utterance) canonical(c *canon) {
	c.tag(u.kind())
	c.str(u.Text)
}

// A vote encodes to its tag and nothing else, which is the whole payload: the
// direction is in the tag, because [Vote.kind] names each direction, and the
// target is not in the payload at all — it is the bit's own Prev, which [ID]
// writes after this returns. So two votes differ in the hash exactly where they
// differ in fact: direction here, target in the prev block, voter in the from
// block.
//
// This is the second payload whose value reaches its address through kind
// alone, and it obeys D26's rule by refusal rather than by arithmetic: kind
// panics on a direction it cannot name, so the two names are one-to-one over
// every direction that can be addressed at all. See [Vote.kind].
func (v Vote) canonical(c *canon) {
	c.tag(v.kind())
}

func (p Compaction) canonical(c *canon) {
	c.tag(p.kind())
	c.num(int64(p.count))
	c.at(p.from)
	c.at(p.to)

	c.tag("handles")
	c.handles(p.handles)
	c.tag("kinds")
	c.counts(p.kinds)
	c.tag("bag")
	c.counts(p.bag)
	c.tag("absorbed")
	c.strs(p.absorbed)
}

// writeBit writes the bytes that are the bit's identity. [ID] hashes exactly
// this and nothing else, which is why [Bit].ID does not appear here: an address
// cannot contain itself, and leaving it out is what lets a reader re-derive it.
func writeBit(c *canon, b Bit) {
	c.tag("bit")
	c.at(b.At)
	c.tag("from")
	c.str(b.From.Ref)
	c.str(b.From.Display)
	c.str(b.Channel)
	b.Payload.canonical(c)
	c.tag("prev")
	c.strs(b.Prev)
}

// scan reads back what [canon] wrote, and is its exact inverse field for field.
//
// Errors are sticky here for canon's reason and one more: a decoder reads
// lengths that decide later reads, so the first bad length makes every read
// after it meaningless. Stopping at the first failure and carrying it to the
// boundary is what keeps a corrupt stream from being interpreted at length.
//
// Everything this reads came out of another process, so nothing off the wire is
// trusted to size an allocation — see [scan.str] and [scan.strs] for the shape
// that takes.
//
// Where the line is drawn on *meaning*, and it moved once already. A rule added
// after a record was written must not be able to make that record unloadable,
// because a record its own reader may decide to reject is not a record. That
// argument is about rules added later and it does not reach the three
// agreements [Cool] already enforces at construction: every populated
// [Compaction] in existence came through Cool, and the only one that did not is
// the bare literal, which satisfies them trivially. So checking those three on
// read cannot orphan anything ever written, and [readCompaction] checks them.
// The first version of this comment used the general argument to decline the
// specific case, and the cost was measured rather than imagined: a hand-written
// record advertising a count of a trillion was admitted, and a caller one
// package away sized a slice from it and died in the allocator.
//
// Nothing beyond those three is judged. The address check is what protects the
// reader from everything else — the bytes say what they say, and they say it
// under a name that has to match.
type scan struct {
	r   io.Reader
	err error
}

// fail records the first failure and keeps it. A later one is a consequence of
// this one and would only bury it.
func (sc *scan) fail(format string, a ...any) {
	if sc.err == nil {
		sc.err = fmt.Errorf(format, a...)
	}
}

// read fills p, and turns a clean end of stream into an unexpected one. Every
// read here was promised by a length or a count already on the wire, so there
// is no position in a well-formed stream where running out is anything but
// truncation — and a caller checking errors.Is(err, io.ErrUnexpectedEOF) should
// not have to know which byte the file happened to stop on.
func (sc *scan) read(p []byte) bool {
	if sc.err != nil {
		return false
	}
	if _, err := io.ReadFull(sc.r, p); err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		sc.fail("memory: reading %d bytes: %w", len(p), err)
		return false
	}
	return true
}

func (sc *scan) num() int64 {
	var buf [8]byte
	if !sc.read(buf[:]) {
		return 0
	}
	return int64(binary.BigEndian.Uint64(buf[:]))
}

// str reads a length-prefixed string.
//
// Through io.CopyN rather than make([]byte, n) because n came off the wire: a
// flipped bit in a length prefix asks for up to eight exabytes, and make would
// answer by dying in the allocator with nothing said about the stream. CopyN
// grows the builder as bytes actually arrive, so a bogus length fails as
// truncation at the end of the stream, which is what it is.
func (sc *scan) str() string {
	n := sc.num()
	if sc.err != nil {
		return ""
	}
	if n < 0 {
		sc.fail("memory: a string of length %d", n)
		return ""
	}

	var b strings.Builder
	if _, err := io.CopyN(&b, sc.r, n); err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		sc.fail("memory: reading a string of %d bytes: %w", n, err)
		return ""
	}
	return b.String()
}

// want reads a tag and refuses any other. The tags are the encoding's frame:
// finding "prev" where "from" belongs means the reader and the writer disagree
// about the shape, and every byte after it would be read against the wrong
// rule.
func (sc *scan) want(name string) {
	got := sc.str()
	if sc.err != nil {
		return
	}
	if got != name {
		sc.fail("memory: expected the tag %q here, found %q", name, clip(got))
	}
}

// strs reads a sequence, and preallocates nothing from the count for
// [scan.str]'s reason: a corrupt count is a request to reserve memory for
// elements that are not there. Appending as they arrive costs a few growths on
// an honest stream and fails at the first missing element on a corrupt one.
func (sc *scan) strs() []string {
	n := sc.num()
	if sc.err != nil {
		return nil
	}
	if n < 0 {
		sc.fail("memory: a sequence of %d strings", n)
		return nil
	}

	var out []string
	for range n {
		s := sc.str()
		if sc.err != nil {
			return nil
		}
		out = append(out, s)
	}
	return out
}

func (sc *scan) counts() map[string]int {
	n := sc.num()
	if sc.err != nil {
		return nil
	}
	if n < 0 {
		sc.fail("memory: a map of %d counts", n)
		return nil
	}
	if n == 0 {
		return nil
	}

	out := map[string]int{}
	for range n {
		k := sc.str()
		v := sc.num()
		if sc.err != nil {
			return nil
		}
		out[k] = int(v)
	}
	return out
}

func (sc *scan) handles() []Handle {
	n := sc.num()
	if sc.err != nil {
		return nil
	}
	if n < 0 {
		sc.fail("memory: a sequence of %d handles", n)
		return nil
	}

	var out []Handle
	for range n {
		h := Handle{Ref: sc.str(), Display: sc.str()}
		if sc.err != nil {
			return nil
		}
		out = append(out, h)
	}
	return out
}

// at reads an instant back as UTC. Nothing validates the nanoseconds: out of
// range they normalize into the seconds, which changes the instant, which
// changes the address, which is where it is caught. That is the general shape
// of this decoder — it reconstructs and lets the address judge, rather than
// growing a second opinion about what a legal value is.
func (sc *scan) at() time.Time {
	sec := sc.num()
	nsec := sc.num()
	if sc.err != nil {
		return time.Time{}
	}
	return time.Unix(sec, nsec).UTC()
}

// clip bounds a value read off the wire before it goes into an error message.
// It came from another process and a corrupt length can make it as long as the
// stream; an error nobody can read is a second failure on top of the first.
func clip(s string) string {
	const n = 24
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// readPayload is the inverse of [Payload.canonical], and it is the mechanical
// proof of D26 rather than an argument for it.
//
// Every case here reads a name and reconstructs a value. That is only possible
// while the value→name map is one-to-one: "fragment" recovers Truncated and
// "upvote" recovers Up because nothing else is named either of those things. A
// payload field that kind does not fully distinguish would make one of these
// cases unwritable — there would be no way to know which value the name stood
// for — and the collision D26 exists about would show up here, in a compiler's
// worth of trouble, instead of as a bit the store silently kept the wrong
// version of.
//
// An unknown name is refused rather than skipped. There is no length in front
// of a payload, so a reader that does not know a kind does not know how many
// bytes it occupies; carrying on would decode the next field out of the middle
// of this one.
func readPayload(sc *scan) Payload {
	kind := sc.str()
	if sc.err != nil {
		return nil
	}

	switch kind {
	case "utterance":
		return Utterance{Text: sc.str()}
	case "fragment":
		return Utterance{Text: sc.str(), Truncated: true}
	case "upvote":
		return Vote{dir: Up}
	case "downvote":
		return Vote{dir: Down}
	case "compaction":
		return readCompaction(sc)
	}

	sc.fail("memory: payload kind %q is not one this build can read", clip(kind))
	return nil
}

// readCompaction rebuilds the fields directly rather than through [Cool],
// which is the only constructor this package has for a populated one. Cool
// derives a compaction from the window of bits it folded, and a stream carries
// the aggregates instead — the originals it names are in the store, but the
// window it was folded from is not recoverable from them, since Absorbed lists
// originals at any depth and never the scars in between.
//
// An empty collection comes back nil rather than empty. The encoding cannot
// tell the two apart — both write a count of zero — so nil is what a decoder
// can honestly produce, and it is what the accessors already treat as empty.
//
// Five things [Cool] guarantees are checked again here, and this is the one
// place the decoder judges what it read. [Compaction].Count is the number a
// caller sizes work from — a receipt is drawn one row per absorbed bit — so a
// count that does not agree with the receipt beside it is not a difference of
// opinion, it is a number nothing in the record supports. See [scan] for why
// re-checking what Cool cannot produce orphans no record that has ever been
// written.
//
// A sixth was considered and not taken: a word count in [Compaction].Bag is
// positive by construction too, and nothing sums it and nothing sizes work from
// it, so a negative one is a wrong number on a screen rather than a number that
// can hurt. Named so the omission reads as a decision.
func readCompaction(sc *scan) Compaction {
	var p Compaction
	// count is read as a value rather than as a length: nothing is allocated
	// from it here. What it can do is escape into a caller who does allocate
	// from it, which is what the agreement below actually protects against.
	p.count = int(sc.num())
	p.from = sc.at()
	p.to = sc.at()

	sc.want("handles")
	p.handles = sc.handles()
	sc.want("kinds")
	p.kinds = sc.counts()
	sc.want("bag")
	p.bag = sc.counts()
	sc.want("absorbed")
	p.absorbed = sc.strs()
	if sc.err != nil {
		return p
	}

	// Cool's own words for the first three: invariants, not input validation.
	// Each says an aggregate agrees with the count it is supposed to agree
	// with, and a fold that fails one is wrong in a way every later generation
	// inherits. The last two are things Cool guarantees about its result rather
	// than checks it writes down, and they are here for the same reason: every
	// populated compaction in existence came through Cool, so re-checking what
	// Cool cannot produce orphans nothing that was ever written.
	if len(p.absorbed) != p.count {
		sc.fail("memory: a compaction naming %d ids and counting %d", len(p.absorbed), p.count)
		return p
	}
	kinds := 0
	for k, n := range p.kinds {
		// Before the sum, because negatives cancel: {"utterance": -5,
		// "fragment": 7} totals 2 and passes a check that only adds. A count
		// of minus five then reaches whatever draws a scar's tally.
		if n < 0 {
			sc.fail("memory: a compaction counting %d bits of kind %q", n, clip(k))
			return p
		}
		kinds += n
	}
	if kinds != p.count {
		sc.fail("memory: a compaction counting %d bits whose kinds account for %d", p.count, kinds)
		return p
	}
	for i, id := range p.absorbed {
		// Cool panics on an unaddressed bit rather than put an empty string in
		// a receipt, because a receipt entry that resolves to nothing is a
		// dangling edge that reports itself only as a row on a screen saying
		// the store does not hold something. A stream can spell one directly.
		if id == "" {
			sc.fail("memory: a compaction whose receipt names nothing at entry %d of %d",
				i+1, len(p.absorbed))
			return p
		}
	}
	if p.from.After(p.to) {
		sc.fail("memory: a compaction whose span runs backwards: %s to %s", p.from, p.to)
	}
	return p
}

func readBit(sc *scan) Bit {
	sc.want("bit")

	var b Bit
	b.At = sc.at()
	sc.want("from")
	b.From.Ref = sc.str()
	b.From.Display = sc.str()
	b.Channel = sc.str()
	b.Payload = readPayload(sc)
	sc.want("prev")
	b.Prev = sc.strs()
	return b
}

// readHeader reads the three fields every stream opens with and says which of
// the three failed, because "this is not a tldreddit file", "this is the other
// kind of tldreddit file" and "this is a tldreddit file from a newer build" are
// three different things for the person holding it.
func readHeader(sc *scan, mark string) {
	got := sc.str()
	if sc.err != nil {
		// The value is not printed. Nothing has established yet that these
		// bytes mean anything, so whatever the length prefix produced is not
		// worth quoting back.
		sc.err = fmt.Errorf("memory: not a tldreddit stream: %w", sc.err)
		return
	}
	if got != magic {
		sc.fail("memory: not a tldreddit stream: it begins %q", clip(got))
		return
	}

	if got := sc.str(); sc.err == nil && got != mark {
		sc.fail("memory: this is a tldreddit %q stream, not a %q stream", clip(got), mark)
		return
	}
	if v := sc.num(); sc.err == nil && v != version {
		sc.fail("memory: tldreddit stream version %d; this build reads version %d", v, version)
	}
}

// WriteTo writes the whole record as a stream another process can read.
//
// Every bit is written as the address it is filed under followed by the bytes
// that address is the hash of. The redundancy is the point: [ReadStore] hashes
// what it decoded and compares, so the file carries its own check on every
// object in it rather than one checksum over the whole thing. The address goes
// out as the hex [Bit].ID rather than as thirty-two raw bytes for the same
// reason tags are literals — one encoding rule in this package, and the thing
// on the wire is the thing the rest of the program calls an address.
//
// The count and [endMark] frame those bits, and they are the two fields no
// address covers. The count says how many to expect and the closing tag says
// there were no more, so neither a count read too low nor a stream stopping
// early can produce a record that is merely shorter than the one written.
//
// Bits go out in address order. A [Store] is a map and Go randomizes map
// iteration, so an unordered walk would give one record a different file on
// every write, which is precisely the version-and-order fragility gob and JSON
// were refused for. Sorted, one record is one file, byte for byte, from any
// process. The order is not topological and does not need to be: [Bit].Prev
// names parents by address and [Store.Put] files each bit independently, so a
// child may land before its parent with nothing to resolve at load time.
//
// The record is snapshotted under the lock and written outside it. Holding a
// read lock across a write to a file or a socket would stall every [Store.Put]
// for as long as the far end takes, and the far end is not this program's to
// bound — the surface writes bits from a Bubble Tea update loop.
//
// The signature is io.WriterTo's, which buys a familiar shape and a byte count
// and not standard-library integration: io.Copy reaches for WriterTo on its
// *source*, and a Store is not an io.Reader, so nothing in the standard library
// will ever call this.
func (s *Store) WriteTo(w io.Writer) (int64, error) {
	s.mu.RLock()
	ids := slices.Sorted(maps.Keys(s.bits))
	bits := make([]Bit, 0, len(ids))
	for _, id := range ids {
		// Read-only copies. Unlike [Store.Get] these share the store's Prev
		// arrays and a compaction's maps, which is safe for exactly as long as
		// nothing here writes to them — and this only encodes.
		bits = append(bits, s.bits[id])
	}
	s.mu.RUnlock()

	c := canon{w: w}
	c.tag(magic)
	c.tag(storeMark)
	c.num(version)

	c.num(int64(len(bits)))
	for _, b := range bits {
		c.str(b.ID)
		writeBit(&c, b)
	}
	c.tag(endMark)
	return c.n, c.err
}

// ReadStore rebuilds a record from a stream, re-deriving every address as it
// goes.
//
// Nothing is taken on trust. Each bit is decoded, hashed by [ID], and compared
// against the address the stream filed it under; a disagreement is an error
// naming both, and the record is refused rather than partly loaded. That covers
// a corrupted byte, a truncated file and an encoder that has drifted from this
// one, without any of them needing a separate check — which is this project's
// own rule about re-derivation, applied to its own storage.
//
// Be exact about what that comparison adds, because it is less than it sounds
// and both halves were measured rather than assumed. The decoded bit is handed
// to [Store.Put] carrying the address it was filed under, and Put panics on a
// bit whose label and content disagree — so for most damage the comparison
// below is the second statement of one rule, and deleting it turns a named
// error into a panic out of Put rather than into a silent load. Two things it
// does that Put cannot. Put's guard is gated on a non-empty [Bit].ID, so a
// stream filing a bit under the *empty* address walks straight past it and gets
// filed under whatever it really addresses to; that stream is refused here, by
// name. And an error is a thing a caller can report, where a panic out of a
// library reading a file is not. The order is load-bearing either way:
// assigning the address before Put is what leaves Put's invariant standing
// behind this one.
//
// A stream naming one address twice is refused too, though [Store.Put] would
// have collapsed it harmlessly. WriteTo walks a map and cannot produce one, so
// a repeat means the stream was assembled by something else, and refusing keeps
// the invariant worth having: a record read back holds exactly the number of
// bits its stream declared.
//
// [endMark] must follow the last bit, and nothing is required after that. The
// stream stays self-delimiting — the header, the count and the closing tag say
// where it ends — so a store stream and any number of view streams concatenate
// into one file and read back in order, and whether they do is the caller's
// filesystem question rather than this package's. That is also why the frame
// cannot be checked by looking for leftover bytes: trailing data is legitimate
// here, so only a tag can tell "the count was wrong" from "a view follows".
func ReadStore(r io.Reader) (*Store, error) {
	sc := scan{r: r}
	readHeader(&sc, storeMark)

	n := sc.num()
	if sc.err != nil {
		return nil, sc.err
	}
	if n < 0 {
		return nil, fmt.Errorf("memory: a record of %d bits", n)
	}

	s := NewStore()
	for i := range n {
		filed := sc.str()
		b := readBit(&sc)
		if sc.err != nil {
			return nil, fmt.Errorf("memory: bit %d of %d: %w", i+1, n, sc.err)
		}

		if got := ID(b); got != filed {
			return nil, fmt.Errorf("memory: bit %d of %d is filed as %s but its bytes address %s",
				i+1, n, Short(filed), Short(got))
		}
		if _, repeat := s.Get(filed); repeat {
			return nil, fmt.Errorf("memory: bit %d of %d repeats the address %s", i+1, n, Short(filed))
		}

		b.ID = filed
		s.Put(b)
	}

	// The frame closes here, and this is the only thing that notices a count
	// that was read one too low: without it the stream simply stops early and
	// hands back a record missing its tail, with a nil error.
	sc.want(endMark)
	if sc.err != nil {
		return nil, fmt.Errorf("memory: after %d bits: %w", n, sc.err)
	}
	return s, nil
}

// Address is the record's own content address: a hash over every address it
// holds, in sorted order.
//
// Sorted rather than in insertion order, because a record is a set — two
// processes that arrived at the same bits by different routes hold the same
// record and must say so. It covers contents transitively without hashing them
// again: every ID in it is already a hash of its bit.
//
// What it is for is [View.WriteAgainst]. A view is a list of addresses with
// nothing in it that says which record they are addresses *into*, so a view
// saved by one session and loaded against another's record would resolve
// happily and show the wrong thing. This is what makes that checkable.
func (s *Store) Address() string {
	s.mu.RLock()
	ids := slices.Sorted(maps.Keys(s.bits))
	s.mu.RUnlock()

	h := sha256.New()
	c := canon{w: h}
	// Tagged like every other composite, so a record of one bit cannot address
	// as the bit itself, and a store address cannot be mistaken for one.
	c.tag(storeMark)
	c.strs(ids)
	if c.err != nil {
		panic(fmt.Sprintf("memory: addressing a record: %v", c.err))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// WriteAgainst writes a view as a stream, stamped with the address of the
// record it was taken against.
//
// A view is where forgetting happens, so it is the one thing here that cannot
// be re-derived: which bits were dropped and which runs were folded is not a
// function of the record, and a record reloaded without its view is a record
// nobody has read yet. That is why it is persisted at all.
//
// It is stamped because verifying the record and then trusting the view would
// check the wrong half. The record is the part that re-derives itself bit by
// bit; the view is the part a person actually looks at, and unstamped it loads
// clean from any session, any record, or a text editor.
//
// Two fields do that, answering two different questions, and the first version
// of this wrote only one of them. The record's address answers *whose view is
// this* — provenance. A hash over the record's address and the view together
// answers *are these the bytes that were written* — integrity, which nothing
// else here provides: a [View] is a list of strings and no address covers it,
// so a view whose length was read one too low used to load clean and lose its
// tail in silence. That is not a hypothetical for a vote view. [Stay].Votes
// says its order and contents decide what a fold keeps, which is the reason it
// is persisted rather than rebuilt; a silently shortened one lifts holds with
// nothing said, and the fold that follows takes material a person had voted to
// keep.
//
// Both are written rather than only the hash, because the two failures want
// different sentences. "This is last session's view" sends a reader to the
// right file; "these bytes are damaged" sends them to a backup. One hash
// covering both would refuse correctly and be unable to say which.
//
// Be exact about what neither field buys, because the gap is easy to overclaim.
// They buy: this view belongs to this record, and these are the bytes that were
// written. They do not buy: *this view is a legitimate consequence of folding
// this record*. A view that dropped a bit or reordered two at the moment it was
// written still stamps, still hashes, and still loads. Proving that would mean
// recording the fold operations themselves as bits, which is a larger design
// and is not built — named here as open so nobody reads the stamp as more than
// it is.
//
// The stamp is exact equality, so a record that has grown by one bit
// invalidates every view saved against it. That is deliberate rather than
// tolerated: a subset check would pass for the stale view, and a stale view is
// the failure worth catching. What it asks of a caller is that the record and
// its views are written in one checkpoint, together — and when that fails
// halfway, [StaleView] is how the surviving view is got back rather than lost.
func (v View) WriteAgainst(s *Store, w io.Writer) (int64, error) {
	c := canon{w: w}
	c.tag(magic)
	c.tag(viewMark)
	c.num(version)

	address := s.Address()
	c.str(address)
	c.str(seal(address, v))
	c.strs(v)
	return c.n, c.err
}

// seal is a view's integrity hash: the two fields the stream carries ahead of
// it, through the same encoding as everything else here.
//
// It takes the record address as a *value* and not as a [Store], and that is
// the whole of what makes it an integrity check rather than a decoration. The
// first version took the store and hashed the live answer — so the one field a
// reader most needs checked, the address written into the stream, was the one
// field the seal never read. A flipped bit there produced a seal that matched
// perfectly and a provenance mismatch that sent the reader hunting for last
// session's file. Sealing what the stream says, and comparing that against what
// the store says separately, are two different questions and this answers only
// the first.
func seal(against string, v View) string {
	h := sha256.New()
	c := canon{w: h}
	c.tag(viewMark)
	c.str(against)
	c.strs(v)
	if c.err != nil {
		panic(fmt.Sprintf("memory: sealing a view: %v", c.err))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// StaleView is a view that read cleanly and belongs to a different record.
//
// It exists so that a safety check cannot become a way to lose data. A view is
// the one thing in this package that cannot be re-derived, and the arrangement
// that produces a stale one is ordinary rather than exotic: a checkpoint writes
// the record, the process dies before it writes the view, and the next session
// grows the record past the view that survived. Refusing to hand that view back
// would leave a caller holding bytes with no exported way to look at them.
//
// It is an error type rather than a second return value for a reason worth
// stating, because returning a usable value beside a non-nil error is the
// unusual shape here. A caller who writes `v, _ :=` gets nil, and has to say
// [errors.As] out loud to get the stale view — so recovering one is deliberate,
// while ignoring the error leaves you with nothing rather than with a view from
// another session. The whole point of the check is that a stale view must not
// be reachable by accident.
//
// Only provenance produces one. A view whose bytes did not survive is not
// returned at all: there is nothing to recover from an integrity failure, and
// handing back a damaged view is how damage propagates.
//
// That sentence is guaranteed by exactly one thing — [ReadViewAgainst] testing
// the seal before it tests provenance — and it was written here while the code
// did the reverse, so it was false for a review cycle: a bit flipped in the
// stream's record address arrived as a StaleView carrying a view nothing had
// checked. Read it as a claim on the order of two comparisons, and if that
// order ever moves, this type is a hole rather than a door.
type StaleView struct {
	// View is what the stream held, decoded and intact. Every address in it may
	// or may not resolve in the record that refused it — check before drawing.
	View View

	// Against is the address of the record the view was written against, and
	// Record is the address of the one it was offered to.
	Against, Record string
}

func (e *StaleView) Error() string {
	return fmt.Sprintf("memory: this view was taken against record %s; this record is %s",
		Short(e.Against), Short(e.Record))
}

// ReadViewAgainst reads a view back, refusing one taken against a different
// record and one whose bytes did not survive.
//
// The store is a parameter and not an option because the check is the reason
// this exists. A view whose addresses the store does not hold already fails
// loudly at [View.Bits], which panics — but that is not this case and does not
// cover it. The case this covers is a view every one of whose addresses
// resolves: a view saved earlier against a record that has since grown, loaded
// against the larger one. [View.Bits] renders it without complaint, showing a
// conversation that is missing everything recorded since, and nothing anywhere
// says a word. Measured rather than argued — see
// TestALargerRecordRendersAStaleViewWithoutComplaint.
//
// A provenance failure comes back as a [StaleView], which carries the decoded
// view so a caller can recover it deliberately. Everything else comes back as a
// plain error and no view.
func ReadViewAgainst(s *Store, r io.Reader) (View, error) {
	sc := scan{r: r}
	readHeader(&sc, viewMark)

	against := sc.str()
	sealed := sc.str()
	v := sc.strs()
	if sc.err != nil {
		return nil, sc.err
	}

	// Integrity first, and this order is the correctness of the function rather
	// than a preference about error messages. The two checks ask different
	// questions — are these the bytes that were written, and is this our
	// record — and only the first one has any standing to say what the other is
	// even looking at. Provenance first reads a field that has not been checked
	// yet, so a flipped bit in the address is reported as somebody else's view
	// and handed out through [StaleView] with its integrity never tested. That
	// was the code here for one review cycle; it is what the ordering exists to
	// prevent, and it is why the seal must cover `against` rather than be
	// computed from the store.
	if want := seal(against, v); sealed != want {
		return nil, fmt.Errorf("memory: this view seals as %s and its stream says %s; its bytes are damaged",
			Short(want), Short(sealed))
	}
	if now := s.Address(); now != against {
		return nil, &StaleView{View: v, Against: against, Record: now}
	}
	return v, nil
}
