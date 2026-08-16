package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
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
// The encoding is in wire.go, and this function is the only difference between
// addressing a bit and sending one: [writeBit] into a hash is an address,
// [writeBit] into a file is persistence. One encoding, so a change that would
// break a stored record breaks the golden addresses on the same run.
//
// The algorithm is part of the format and is not announced in the ID. A
// multihash-style prefix would buy the ability to migrate, and there is nothing
// yet to migrate to; when there is, it is a new field on a new kind of store,
// not a prefix nobody has ever read. A stream is the other case and does
// announce itself, for reasons that do not apply here — see wire.go's header
// constants.
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
	case Utterance, Compaction, Vote:
	default:
		panic(fmt.Sprintf("memory: ID of payload %T; the set is Utterance, Compaction and Vote, by value",
			b.Payload))
	}

	h := sha256.New()
	c := canon{w: h}
	writeBit(&c, b)

	// Unreachable through this function: hash.Hash documents that Write never
	// returns an error. It is checked because [canon] no longer writes only
	// into a hash, and an address taken over a partial write would be a name
	// for something that was never fully said.
	if c.err != nil {
		panic(fmt.Sprintf("memory: hashing a bit: %v", c.err))
	}
	return hex.EncodeToString(h.Sum(nil))
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
