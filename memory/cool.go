package memory

import (
	"fmt"
	"iter"
	"maps"
	"slices"
	"strings"
	"time"
	"unicode"
)

// Compaction is the resting form of a window of bits.
//
// The material survives in aggregate; the arrangement does not. Word counts,
// participants, payload kinds and the span are kept exactly, because they are
// cheap to keep exactly. Everything else — order, sentence boundaries, who said
// which word — is gone, and gone for good.
//
// A Compaction is itself a [Payload], so cold bits can be cooled again. Every
// aggregate is therefore mergeable: cooling a Compaction folds its totals in
// rather than counting it as one bit. Without that, each generation would
// quietly discard the one beneath it.
//
// Nothing here is writable from outside. The fields are unexported and the
// collections are handed out as iterators, so a caller holding a Compaction
// that came from a [Store] cannot alter what the Store has. A stored bit that
// can be edited in place stops hashing to the address it is filed under, and a
// record that can be edited after the fact is an assertion rather than
// evidence.
//
// The same closure means [Cool] is the only way to make one that says anything.
// A bare Compaction{} is still a legal literal anywhere, since writing no
// fields sets no unexported ones — but it is the exact case that cannot lie:
// it stands for nothing, absorbed nothing and spans a single instant, so its
// totals agree trivially. Everything with content in it came through Cool.
//
// Nothing writes to a Compaction after Cool returns it, and two guarantees
// elsewhere now rest on that rather than on any copying. The accessors below
// hand out iterators over the very slices and maps this holds, and those may be
// run long after the call that returned them; [Store.Get] releases its lock
// while handing back a bit that still points at them. A mutating method added
// here would break the store's escape guarantee and its lock discipline at
// once, without touching a line of either — so anything that needs to change a
// Compaction returns a new one.
type Compaction struct {
	// count is how many original (uncompacted) bits this stands for, however
	// many generations back.
	count int

	// from and to are the span the absorbed bits covered.
	from, to time.Time

	// handles are the distinct actors present, in first-seen order. A re-fold
	// merges these rather than reading the cold bit's own handle, which is the
	// fold itself and was never in the conversation.
	handles []Handle

	// kinds counts *original* payloads by kind, so it sums to count. In a
	// transcript the keys are "utterance" and "fragment" — an [Utterance] names
	// itself the second when its speaker was cut off — and this is what lets a
	// cold bit say, holding nothing but its own tally, that a fragment was in
	// the window it absorbed. The fragments themselves are not gone: absorbed
	// names them,
	// they are still in the store, and a reader with both can read what the
	// speaker got as far as saying. This is the summary, not a replacement for
	// them. "compaction" is never a key: a cold bit contributes the tally it
	// already holds instead of itself. The names are this package's own and
	// outlive any Go type name they happen to resemble, because they reach
	// content addresses and so cannot move when a type does.
	kinds map[string]int

	// bag counts words across every utterance absorbed, at any depth. It is a
	// good index and a poor record: it preserves what was discussed and
	// destroys what was said about it.
	bag map[string]int

	// absorbed lists the content addresses of the original bits, so the drop
	// stays visible. It is a receipt rather than topology — the graph edges
	// live in Bit.Prev — but because the store keeps what it names, it is a
	// receipt you can follow: every ID here resolves to the bit it stands for.
	absorbed []string
}

func (Compaction) kind() string { return "compaction" }

// Count is how many original bits this stands for, however many generations
// back. It is not how many bits were in the window that produced it.
func (p Compaction) Count() int { return p.count }

// From and To are the span the absorbed bits covered.
func (p Compaction) From() time.Time { return p.from }
func (p Compaction) To() time.Time   { return p.to }

// Handles yields the distinct actors present, in first-seen order.
func (p Compaction) Handles() iter.Seq[Handle] { return slices.Values(p.handles) }

// Kinds yields original payload kinds and their counts, which sum to [Compaction.Count].
// Map order, so sort it before showing it to anyone.
func (p Compaction) Kinds() iter.Seq2[string, int] { return maps.All(p.kinds) }

// Bag yields words and their counts across everything absorbed. Map order, so
// sort it before showing it to anyone.
func (p Compaction) Bag() iter.Seq2[string, int] { return maps.All(p.bag) }

// Absorbed yields the content addresses of the original bits, in the order they
// occurred. Every one of them resolves in the store the compaction came from.
func (p Compaction) Absorbed() iter.Seq[string] { return slices.Values(p.absorbed) }

// Cool derives a single cold bit standing for a window of bits. It removes
// nothing: the window is untouched and every bit in it stays in the store. What
// the cold bit takes is the window's slot in a [View], which is the only place
// forgetting is allowed to happen.
//
// The cold bit's Prev is every bit in the window, in window order, because that
// is what it was derived from. It is also the only choice that leaves the
// record walkable. Absorbed carries originals only, so when a fold absorbs the
// fold before it, Prev is the sole edge that names that earlier fold; anything
// shorter strands a generation — still filed in the store, reachable by nobody
// — which is exactly the loss D1 exists to prevent.
//
// Cool is a pure function of its input. It reads no clock and takes no ID: the
// returned bit is already addressed, and At is the end of the span it covers
// rather than the moment of folding. Both follow from the same requirement —
// folding the same window twice must produce the same object, or the store
// fills with near-duplicate summaries that differ only in when someone got
// around to making them.
//
// Cool is for a window of things said, and it will fold a window of votes
// without complaining. Do not — a vote view is not folded, ever. Nothing is
// destroyed by it, as ever: the cold bit's Prev names every vote it absorbed, so
// an auditor with the store still walks to each one and reads what it voted on.
// What goes is the view. Kinds would say three upvotes and one downvote and
// could not say what any of them were about, because a vote's whole content is
// its target and a target is an edge rather than a payload — and, worse, those
// votes have left the vote view, so [Tally] over that view now reports nothing
// and every stay it was holding lifts at once. Tally panics rather than let that
// pass quietly, but the fold that caused it happened here.
//
// Cool panics if bits is empty, if any bit is unaddressed, or if the window
// spans more than one channel. Channels have no ordering, so there is no honest
// answer for what channel a mixed compaction belongs to — and a compaction that
// quietly mixed an internal channel with a public one would launder private
// material into a bit nothing marks as private. Cool within a channel; join
// later if ever.
func Cool(bits []Bit) Bit {
	if len(bits) == 0 {
		panic("memory: Cool on an empty window")
	}

	channel := bits[0].Channel
	c := Compaction{kinds: map[string]int{}, bag: map[string]int{}}
	prev := make([]string, 0, len(bits))

	// first, rather than testing c.from for the zero time. The zero time is a
	// legal instant, so reading it as "no span yet" conflates an unset
	// accumulator with a bit that genuinely carries it, and the next bit
	// overwrites that bit's start — a receipt naming material its own span
	// excludes.
	first := true

	seen := map[Handle]bool{}
	note := func(h Handle) {
		if !seen[h] {
			seen[h] = true
			c.handles = append(c.handles, h)
		}
	}

	for _, b := range bits {
		if b.Channel != channel {
			panic(fmt.Sprintf("memory: Cool across channels %q and %q", channel, b.Channel))
		}

		// Both Prev and Absorbed name bits by address, and both promise to
		// resolve. An unaddressed bit would put an empty string in each — a
		// dangling edge and a receipt for nothing, neither of which reports
		// itself later.
		if b.ID == "" {
			panic(fmt.Sprintf("memory: Cool on an unaddressed bit from %q; store it first", b.From.Ref))
		}
		prev = append(prev, b.ID)

		from, to := b.At, b.At
		if p, ok := b.Payload.(Compaction); ok {
			// A cold bit stands for everything under it, so every total merges.
			// Reading its own metadata instead would count it as one bit, name
			// the fold as the speaker, and record its kind as "compaction" —
			// three different ways to lose the generation beneath it.
			c.count += p.count
			c.absorbed = append(c.absorbed, p.absorbed...)
			for _, h := range p.handles {
				note(h)
			}
			for k, n := range p.kinds {
				c.kinds[k] += n
			}
			for w, n := range p.bag {
				// Add n, not 1: a compaction has already done the counting.
				c.bag[w] += n
			}
			from, to = p.from, p.to
		} else {
			// A hot bit stands for itself.
			c.count++
			c.absorbed = append(c.absorbed, b.ID)
			note(b.From)
			c.kinds[b.Payload.kind()]++
			if u, text := b.Payload.(Utterance); text {
				words(c.bag, u.Text)
			}
		}

		if first || from.Before(c.from) {
			c.from = from
		}
		if first || to.After(c.to) {
			c.to = to
		}
		first = false
	}

	// Invariants, not input validation. Each one says an aggregate agrees with
	// the count it is supposed to agree with; if any is false the fold is wrong
	// and every later generation inherits the error, so fail where it happened
	// rather than where it shows.
	if len(c.absorbed) != c.count {
		panic(fmt.Sprintf("memory: cooled %d ids but counted %d", len(c.absorbed), c.count))
	}
	kinds := 0
	for _, n := range c.kinds {
		kinds += n
	}
	if kinds != c.count {
		panic(fmt.Sprintf("memory: cooled %d bits but kinds account for %d", c.count, kinds))
	}
	if c.from.After(c.to) {
		panic(fmt.Sprintf("memory: cooled span runs backwards: %s to %s", c.from, c.to))
	}

	cold := Bit{
		At:      c.to,
		From:    Handle{Ref: "cool", Display: "system"},
		Channel: channel,
		Payload: c,
		Prev:    prev,
	}
	cold.ID = ID(cold)
	return cold
}

// isSep splits on anything that is not a letter or a number, so trailing
// punctuation does not mint a separate word.
func isSep(r rune) bool {
	return !unicode.IsLetter(r) && !unicode.IsNumber(r)
}

// words adds the words of text to counts, folded to lower case.
//
// It takes the map rather than returning one so that every aggregate in [Cool]
// is merged by the same loop over the same window. Splitting the word count out
// into its own pass is how Handles and Kinds drifted out of agreement with
// Count in the first place: two loops, one of them updated.
func words(counts map[string]int, text string) {
	for _, w := range strings.FieldsFunc(strings.ToLower(text), isSep) {
		counts[w]++
	}
}
