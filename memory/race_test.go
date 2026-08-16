package memory

import (
	"slices"
	"sync"
	"testing"
)

// The race detector only reports what a run actually did. Every other test in
// this package runs on one goroutine, so before this file the mutex in [Store]
// had never been contended by anything and `go test -race` was affirming a
// claim nobody had made it check. The tests here exist to make that claim
// falsifiable: each one is written so that removing the locking it depends on
// makes it fail rather than merely makes it lucky.
//
// They are in their own file because they are shaped differently from the rest.
// The others state a property of one call; these state a property of an
// interleaving, and an interleaving has to be built.

// contend runs body in n goroutines released together.
//
// The closed channel is the whole point. Goroutines started in a loop begin as
// they are spawned, and under a scheduler with work to do the first is often
// finished before the last exists — which reads on the page as a concurrency
// test and executes as a sequence. The barrier makes them overlap.
func contend(n int, body func(g int)) {
	start := make(chan struct{})

	var wg sync.WaitGroup
	wg.Add(n)
	for g := range n {
		go func() {
			defer wg.Done()
			<-start
			body(g)
		}()
	}

	close(start)
	wg.Wait()
}

// shorten abbreviates a view for a failure message. Display only — Short is
// never what anything compares on.
func shorten(v View) []string {
	out := make([]string, 0, len(v))
	for _, id := range v {
		out = append(out, Short(id))
	}
	return out
}

// Concurrent Put of identical content is where a check-then-act would show.
// Put reads the map, finds the address absent, and writes it; between those two
// steps another goroutine can do the same thing with the same address. The mild
// failure that leaves is a count that depends on scheduling, and a record whose
// size depends on timing is not evidence of anything. The real one is a torn
// map, which Go reports by killing the process.
//
// Every goroutine puts the same slice in the same order from a standing start,
// so the collision is on one address at one instant rather than spread thinly
// across the map. Spreading it would still be concurrent and would contend far
// less, which is the failure this file is here to avoid.
func TestConcurrentPutCollapsesIdenticalContent(t *testing.T) {
	const goroutines, distinct = 12, 40

	contents := make([]Bit, 0, distinct)
	for i := range distinct {
		contents = append(contents, said(i, "tyler", "the deploy failed"))
	}

	s := NewStore()
	ids := make([][]string, goroutines)
	contend(goroutines, func(g int) {
		got := make([]string, len(contents))
		for i, b := range contents {
			got[i] = s.Put(b).ID
		}
		// Its own index of a slice sized before the goroutines started. Nothing
		// here is shared but the store, which is the thing under test.
		ids[g] = got
	})

	if got := s.Len(); got != distinct {
		t.Errorf("store holds %d bits after %d puts of %d distinct contents, want %d",
			got, goroutines*distinct, distinct, distinct)
	}

	for g, got := range ids {
		for i, id := range got {
			if want := ID(contents[i]); id != want {
				t.Fatalf("goroutine %d filed content %d as %s, want %s",
					g, i, Short(id), Short(want))
			}
			if _, held := s.Get(id); !held {
				t.Fatalf("goroutine %d was told it put %s, which the store does not hold",
					g, Short(id))
			}
		}
	}
}

// Readers run against a store growing under them. [Store]'s argument for why
// this is safe is a claim about what a reader sees — that because the store only
// grows, a key can appear but can never change value — so a reader has to be the
// one to check it, and there are two ways for it to be false. Content that does
// not hash to the address it was asked for is the map handing back something
// filed under a different name. An address that resolves and then later misses
// is the store having lost something, which it is never allowed to do.
func TestConcurrentGetSeesSettledBitsOnly(t *testing.T) {
	const writers, readers, each, passes = 4, 8, 60, 4

	bits := make([]Bit, 0, writers*each)
	for i := range writers * each {
		bits = append(bits, said(i, "tyler", "the deploy failed"))
	}

	s := NewStore()
	contend(writers+readers, func(g int) {
		if g < writers {
			for _, b := range bits[g*each : (g+1)*each] {
				s.Put(b)
			}
			return
		}

		// Several passes, because "resolves once, resolves forever" needs a
		// second look at an address to have anything to catch.
		seen := map[string]bool{}
		for range passes {
			for _, b := range bits {
				got, ok := s.Get(b.ID)
				if !ok {
					if seen[b.ID] {
						t.Errorf("%s resolved earlier and now misses", Short(b.ID))
					}
					continue
				}
				seen[b.ID] = true
				if addr := ID(got); addr != b.ID {
					t.Errorf("Get(%s) returned content addressing to %s",
						Short(b.ID), Short(addr))
				}
			}
		}
	})

	if got := s.Len(); got != len(bits) {
		t.Errorf("store holds %d bits, want %d", got, len(bits))
	}
}

// One record, many views, which is the arrangement D18(e) is heading for and
// the only one where Fold contends anything.
//
// Be precise about what is shared, because a View is a value and it would be
// easy to write a version of this that looks concurrent and is not. Each
// goroutine below holds its own view; the [Store] pointer is the shared state,
// and a fold is the heaviest thing that touches it — it reads a window back out
// with Get, cools it, and puts the result, so it is a read-modify-write across
// the same map every other goroutine is writing.
//
// It contends something less obvious too, and this is the half worth keeping.
// Get releases its lock while handing back a [Compaction] that still points at
// the very maps and slices the store holds, so several goroutines folding the
// same cold bit read one set of maps at once — and the writes that filled those
// maps happened in whichever goroutine ran [Cool]. The lock is what publishes
// them. Run with the locking removed, this test reports that as a race between
// Cool building a bag and another goroutine walking it, which is a hazard the
// store's own "it only ever grows" argument does not reach and never claimed to.
//
// The assertion is that concurrency changes nothing at all. Content addressing
// makes the script deterministic — the same bits in the same order, and Cool
// reads no clock — so however the goroutines interleave, the store must end up
// holding exactly what a single sequential run leaves in it and every goroutine
// must end on the same view.
func TestConcurrentFoldsAgreeWithOneSequentialRun(t *testing.T) {
	const goroutines, sends, coolAt, keepHot = 8, 60, 12, 6

	script := func(s *Store) View {
		var v View
		for i := range sends {
			v, _ = v.Add(s, said(i, "tyler", "the deploy failed", v.Head()...))
			if len(v) > coolAt {
				v, _ = v.Fold(s, keepHot, Stay{})
			}
		}
		return v
	}

	alone := NewStore()
	want := script(alone)

	together := NewStore()
	views := make([]View, goroutines)
	contend(goroutines, func(g int) { views[g] = script(together) })

	if got := together.Len(); got != alone.Len() {
		t.Errorf("%d goroutines running one script left %d bits; one run leaves %d",
			goroutines, got, alone.Len())
	}
	for g, got := range views {
		if !slices.Equal(got, want) {
			t.Errorf("goroutine %d ended on view %v, want %v", g, shorten(got), shorten(want))
		}
	}

	// D1, under contention: what the views name, the record still holds.
	for g, v := range views {
		for _, id := range v {
			if _, held := together.Get(id); !held {
				t.Errorf("goroutine %d's view names %s, which the store does not hold",
					g, Short(id))
			}
		}
	}
}

// The one piece of state a [View] can share, and the reason [View.Add] uses a
// full slice expression. Two goroutines holding "the same" view hold two slice
// headers, and two slice headers can name one backing array — so an append into
// shared spare capacity is two goroutines writing one word and both believing
// they got it.
//
// The view is built by hand for the spare capacity it guarantees, not because
// the hazard needs a hand-built view. Add caps what it appends *into* —
// append(v[:len(v):len(v)], …) — and then returns whatever the runtime sized the
// new array at, which is not len: measured, the third Add returns len 3 cap 4
// and the eighth returns len 8 cap 14. So three ordinary Adds are enough to
// reproduce this, and the full slice expression is protecting the package's own
// path rather than only views that callers built elsewhere. What make() buys is
// that the spare word is there by construction instead of by whatever growslice
// decided that release, which is not a thing a test should rest on.
func TestConcurrentAddDoesNotShareAViewsSpareCapacity(t *testing.T) {
	const goroutines = 12

	store := NewStore()
	root := store.Put(said(0, "tyler", "the deploy failed"))

	shared := make(View, 1, goroutines+1)
	shared[0] = root.ID

	grown := make([]View, goroutines)
	contend(goroutines, func(g int) {
		grown[g], _ = shared.Add(store, said(g+1, "tyler", "the deploy failed", root.ID))
	})

	for g, v := range grown {
		want := ID(said(g+1, "tyler", "the deploy failed", root.ID))
		if len(v) != 2 || v[0] != root.ID || v[1] != want {
			t.Errorf("goroutine %d grew the view to %v, want [%s %s]",
				g, shorten(v), Short(root.ID), Short(want))
		}
	}
	if len(shared) != 1 || shared[0] != root.ID {
		t.Errorf("the shared view is now %v, want [%s]", shorten(shared), Short(root.ID))
	}
	if got := store.Len(); got != goroutines+1 {
		t.Errorf("store holds %d bits, want %d", got, goroutines+1)
	}
}
