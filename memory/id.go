package memory

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash"
	"maps"
	"slices"
	"time"
)

// ID is the content address of a bit: the SHA-256 of everything the bit says,
// as lowercase hex. It ignores [Bit].ID itself, so it is safe to call on a bit
// that already carries one.
//
// Equal content gives an equal ID in every process, on every machine, forever.
// That promise is the whole point, so the encoding underneath is written by
// hand rather than borrowed: gob, JSON and fmt all have shapes that depend on
// Go's version, on struct field names, or on map iteration order, and any of
// those drifting would silently split one object into two.
//
// The algorithm is part of the format and is not announced in the ID. A
// multihash-style prefix would buy the ability to migrate, and there is nothing
// yet to migrate to; when there is, it is a new field on a new kind of store,
// not a prefix nobody has ever read.
//
// ID panics on a bit with no payload. A bit that carries nothing has no
// content, so it has no content address; minting one anyway would hand every
// empty bit the same identity. It panics for the same reason on a payload
// outside the closed set — see the switch below for what gets in that way.
func ID(b Bit) string {
	if b.Payload == nil {
		panic("memory: ID of a bit with no payload")
	}

	// [Payload] is closed by unexported methods, but Go puts a value method in
	// the pointer type's method set too, so *Utterance and *Compaction satisfy
	// it as well. Refuse them here, which is enough: an unaddressed bit cannot
	// be stored. A pointer payload would be shared mutable state inside an
	// object whose entire claim is that it does not change, and every type
	// switch in this program would miss it and fall through to its default.
	switch b.Payload.(type) {
	case Utterance, Compaction:
	default:
		panic(fmt.Sprintf("memory: ID of payload %T; the set is Utterance and Compaction, by value",
			b.Payload))
	}

	var c canon
	c.h = sha256.New()

	c.tag("bit")
	c.at(b.At)
	c.tag("from")
	c.str(b.From.Ref)
	c.str(b.From.Display)
	c.str(b.Channel)
	b.Payload.canonical(&c)
	c.tag("prev")
	c.strs(b.Prev)

	return hex.EncodeToString(c.h.Sum(nil))
}

// Short abbreviates an ID for display, git-style. The full ID is the identity;
// this is only ever a label, so never store what Short returns or compare with
// it.
func Short(id string) string {
	const n = 8
	if len(id) <= n {
		return id
	}
	return id[:n]
}

// canon writes a canonical byte encoding into a hash.
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
// Nothing here checks the error from Write. hash.Hash documents that it never
// returns one, and threading an error nobody can produce through every method
// would cost more legibility than it buys.
type canon struct{ h hash.Hash }

// tag marks what comes next. It is length-prefixed like any other string, so a
// tag can never be forged by a value that happens to spell it.
func (c *canon) tag(name string) { c.str(name) }

func (c *canon) str(s string) {
	c.num(int64(len(s)))
	c.h.Write([]byte(s))
}

// num writes a fixed eight bytes, big-endian. Fixed width over varint because
// there is nothing to save here and one less encoding rule to get wrong.
func (c *canon) num(n int64) {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(n))
	c.h.Write(buf[:])
}

// at encodes an instant as seconds and nanoseconds since the epoch in UTC.
//
// Normalizing to UTC is a deliberate claim: identity is the instant, not the
// zone it was displayed in, so the same moment recorded in Tokyo and in London
// is one bit. It also drops the monotonic clock reading that time.Now attaches,
// which is process-local and would otherwise make every ID unrepeatable.
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

func (u Utterance) canonical(c *canon) {
	c.tag(u.kind())
	c.str(u.Text)
}

func (p Compaction) canonical(c *canon) {
	c.tag(p.kind())
	c.num(int64(p.count))
	c.at(p.from)
	c.at(p.to)

	c.tag("handles")
	c.num(int64(len(p.handles)))
	for _, h := range p.handles {
		c.str(h.Ref)
		c.str(h.Display)
	}

	c.tag("kinds")
	c.counts(p.kinds)
	c.tag("bag")
	c.counts(p.bag)
	c.tag("absorbed")
	c.strs(p.absorbed)
}
