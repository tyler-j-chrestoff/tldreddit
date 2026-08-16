package memory

import (
	"fmt"
	"iter"
	"maps"
	"slices"
	"sync"
)

// Store is the record: every bit ever put in it, addressed by content, forever.
//
// It only grows. There is no delete and there will not be one, because a
// content address is a promise that a hash resolves to its object — keeping the
// receipt and dropping the thing it is a receipt for is the one failure this
// design exists to rule out. Consolidation happens in a [View] instead, which
// is free to drop anything precisely because the Store never does.
//
// Identical content collapses to one entry. That is the payoff of deriving
// identity rather than assigning it: two agents that reach the same conclusion
// separately land on one object reachable from both their histories, with no
// merge step and no new kind of edge.
//
// What comes out of a Store cannot be used to change what is in it. That is the
// load-bearing invariant: a stored bit whose contents can be edited
// through a returned copy no longer hashes to the address it is filed under,
// and the record stops being evidence of anything. Three things hold it up. A
// [Bit] is a value. Its payloads carry no writable state — [Utterance] is a
// string and a [Compaction] hands out iterators over unexported fields it never
// alters after construction, while [ID] rejects the pointer payloads that would
// smuggle a reference back in. And [Bit].Prev, the one writable field left, is
// copied on every crossing of this boundary: into Put, back out of Put, and out
// of Get. All three, because a bit going in and the same bit coming straight
// back out is one round trip with two chances to hand out the store's array.
//
// Persistence is here, and it is behind this type, which is why the type
// exists at all rather than callers passing a map around: [Store.WriteTo] and
// [ReadStore]. A record read back is not trusted — every bit is re-addressed as
// it lands — so nothing above changes when a store came off a disk rather than
// out of this process. What is not here is a storage *engine*: this writes a
// stream and reads one, and where that stream lives, how it is rotated and
// whether a write is atomic are all the caller's.
type Store struct {
	// mu guards bits. A Store is the one thing several goroutines will
	// plausibly share — Bubble Tea runs commands off the update loop — and
	// because the store only grows, holding it briefly is enough: a reader can
	// never observe a key change value, only appear.
	//
	// That argument is true and it is not the whole reason this is safe. Get
	// releases the lock while handing back a bit whose [Compaction] still
	// points at the maps and slices filed here, so the Unlock/RLock pair is
	// also the edge that publishes them. Without it, a reader walking a bag
	// races the [Cool] that filled it — and no amount of never mutating a
	// Compaction afterwards repairs an unpublished write.
	//
	// Both halves are exercised rather than argued: race_test.go contends this
	// lock on purpose, and taking the locking out fails those tests while every
	// other test in the package goes on passing green under -race, which is the
	// state that file exists to end.
	//
	// Seeing the second half takes a more specific instruction than the first,
	// so it is written down rather than left to be rediscovered. Remove the
	// locking and run `go test -race -run TestConcurrentFolds ./memory`, more
	// than once. Two failures are racing each other there: the detector reports
	// the map race, and the runtime throws `fatal error: concurrent map read and
	// map write`, which ends the run wherever it lands — so the publication race
	// is named only on the runs where it lands later. Measured here at 5 runs in
	// 10 with that test alone, and 0 in 6 with the whole package, where a test
	// earlier in the file throws first. GORACE=halt_on_error=0 does not change
	// that: it governs how many reports the detector makes, not the throw.
	mu   sync.RWMutex
	bits map[string]Bit
}

// NewStore returns an empty Store.
func NewStore() *Store {
	return &Store{bits: map[string]Bit{}}
}

// Put files a bit under its content address and returns it with [Bit].ID set.
// Putting the same content twice is a no-op that returns the same bit, which is
// what makes Put safe to call without checking first.
//
// Put panics if the bit already carries an ID that does not match its content.
// That means the bit was edited after it was stored, and an edited bit is a
// different bit; re-addressing it quietly would leave the caller holding an
// object it believes is the one in the store.
func (s *Store) Put(b Bit) Bit {
	id := ID(b)
	if b.ID != "" && b.ID != id {
		panic(fmt.Sprintf("memory: Put bit labelled %s but addressed %s", Short(b.ID), Short(id)))
	}
	b.ID = id

	// Three arrays where the obvious version has one. The caller may still be
	// holding what it passed in, and it goes on holding what comes back out —
	// so the filed bit gets a slice that neither of them names. Sharing with
	// either one lets a write land in the store after the address is fixed, and
	// then the store holds a bit under the name of what it used to say. Prev is
	// bounded by a fold window, so the second copy costs nothing worth saving.
	filed := b
	filed.Prev = slices.Clone(b.Prev)
	b.Prev = slices.Clone(b.Prev)

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.bits[id]; !ok {
		s.bits[id] = filed
	}
	return b
}

// Get resolves a content address. The second return is false if nothing under
// that ID has been put here — which under content addressing means it was
// written somewhere else, never that it was removed.
//
// The bit that comes back is the caller's to keep and cannot be used to reach
// what the Store holds. Prev is copied per call: it is bounded by the size of a
// fold window, so this stays cheap enough for [View.Bits] to run on every
// frame, which nothing that walked a compaction's aggregates would be.
func (s *Store) Get(id string) (Bit, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, ok := s.bits[id]
	b.Prev = slices.Clone(b.Prev)
	return b, ok
}

// Len is how many distinct bits the store holds. It counts objects, not writes:
// after two Puts of the same content it is 1.
func (s *Store) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.bits)
}

// All hands out every bit in the record, in address order.
//
// A record that can only be read through the [View] that was written beside it is
// a record only its own author can audit, and the audit is the point: what is in
// here that no view names is exactly what a reader most needs to be able to find.
// [View.Bits] answers "what am I being shown"; this answers "what is there".
//
// Address order, which is the same order [Store.WriteTo] uses and for the same
// reason — a map's iteration order is not stable, so two processes holding one
// record must be given some order they agree on. It is emphatically **not a
// reading order**: an address is a hash, so this arrives shuffled with respect to
// time, to speaker and to anything a person cares about. A caller building a
// reading has to sort it by something it can name and defend, and saying which
// order it chose is part of what it owes its reader.
//
// The record is snapshotted under the lock and yielded outside it, so a caller may
// [Store.Put] while iterating — what it gets is the record as of the call, and the
// new bit is not in it. Holding the lock across the yield would deadlock on
// exactly that, and a sequence that cannot be walked while the surface is writing
// is one no [View.Fold] could ever be built on. Prev is copied per bit for
// [Store.Get]'s reason: what comes out of a Store must not reach what is in it.
func (s *Store) All() iter.Seq[Bit] {
	s.mu.RLock()
	ids := slices.Sorted(maps.Keys(s.bits))
	bits := make([]Bit, 0, len(ids))
	for _, id := range ids {
		bits = append(bits, s.bits[id])
	}
	s.mu.RUnlock()

	return func(yield func(Bit) bool) {
		for _, b := range bits {
			b.Prev = slices.Clone(b.Prev)
			if !yield(b) {
				return
			}
		}
	}
}
