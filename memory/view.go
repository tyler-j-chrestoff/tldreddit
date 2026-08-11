package memory

import (
	"fmt"
	"slices"
)

// View is what a reader is currently shown: an ordered window of content
// addresses into a [Store].
//
// A View is where forgetting is allowed to happen. It drops bits, replaces
// runs of them with a summary, and will one day reorder them — all of which is
// safe only because the Store it points into never drops anything, so every ID
// a View lets go of still resolves.
//
// It is a slice of IDs and not a slice of bits on purpose. A view that held
// bits would be a second copy of the record, and the moment two copies exist
// one of them starts being the real one. Holding addresses keeps it honest:
// the view can only ever show what the store already has.
type View []string

// Add files b in the store and appends it to the view, returning both the
// extended view and the stored bit with its ID set.
//
// The two steps are one method because separating them is how the record loses
// something: a bit shown but never stored is a bit that disappears at the next
// fold with nothing left to resolve.
func (v View) Add(s *Store, b Bit) (View, Bit) {
	b = s.Put(b)

	// Capped so the append always allocates. Without the cap, adding to a view
	// could write into spare capacity another copy of that view is still
	// reading, and views get copied constantly — every Bubble Tea update
	// passes one by value.
	return append(v[:len(v):len(v)], b.ID), b
}

// Head is the Prev for the next bit written after this view: the last bit
// shown, or nothing at all if the view is empty, which is how a first bit
// becomes a root.
func (v View) Head() []string {
	if len(v) == 0 {
		return nil
	}
	return []string{v[len(v)-1]}
}

// Bits resolves the view against the store, in view order.
//
// It panics on an ID the store does not hold. That is an invariant, not input
// validation: a view built by [View.Add] and [View.Fold] can only name bits
// that were stored first, so an unresolvable ID means the view and the store
// came from different records, and rendering it as a gap would hide that.
func (v View) Bits(s *Store) []Bit {
	out := make([]Bit, 0, len(v))
	for _, id := range v {
		b, ok := s.Get(id)
		if !ok {
			panic(fmt.Sprintf("memory: view names %s, which the store does not hold", Short(id)))
		}
		out = append(out, b)
	}
	return out
}

// Fold cools everything but the last keep bits into one cold bit, and returns
// the view that shows that bit in their place. The second return is false when
// there is nothing worth folding — fewer than one bit would be absorbed, or the
// window holds nothing that is not already a fold — and the view comes back
// unchanged.
//
// Nothing is removed by this. The cold bit goes into the store alongside the
// bits it absorbed, all of which stay addressable, so the scar the surface
// draws is one a reader can follow back to what was folded rather than one
// they can only read about.
//
// Fold panics on a negative keep. Keeping fewer than no bits has no meaning,
// and the alternative to failing here is worse than a bad answer: cut runs past
// the end of the view, v[:cut] reads the spare capacity behind it rather than
// refusing, and the empty IDs that picks up surface later as [View.Bits]
// reporting that the store does not hold something. That message is this
// package's alarm for a record that lost a bit, and spending it on a caller's
// arithmetic sends the next debugger hunting a reachability bug that never
// happened.
func (v View) Fold(s *Store, keep int) (View, bool) {
	if keep < 0 {
		panic(fmt.Sprintf("memory: Fold keeping %d bits", keep))
	}

	cut := len(v) - keep
	if cut < 1 {
		return v, false
	}

	// A window with nothing hot in it has already been folded. Cooling it again
	// merges the same totals into the same answer, so the screen does not
	// change — but it mints a new object and adds a link to the chain a reader
	// has to walk to reach the originals. Refusing is what makes a second press
	// of the fold key cost nothing.
	window := v[:cut].Bits(s)
	if !slices.ContainsFunc(window, hot) {
		return v, false
	}

	cold := s.Put(Cool(window))
	return append(View{cold.ID}, v[cut:]...), true
}

// hot reports whether b is an original rather than a fold of other bits.
func hot(b Bit) bool {
	_, cold := b.Payload.(Compaction)
	return !cold
}
