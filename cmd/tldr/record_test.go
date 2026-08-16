package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/tyler-j-chrestoff/tldreddit/memory"
)

var me = memory.Handle{Ref: "you", Display: "you"}

// fixture is a record with everything in it that the file has to carry: several
// utterances, a fold, and votes on one bit that survived that fold and one that
// did not.
//
// The fold is the part worth insisting on. A record of nothing but utterances
// exercises one payload; a scar puts a [memory.Compaction] on the wire, which is
// the only payload whose decoder re-checks anything, and it makes the two views
// disagree about which bits they name — which is the arrangement the file exists
// to keep straight.
func fixture(t *testing.T) record {
	t.Helper()

	s := memory.NewStore()
	at := time.Date(2026, 8, 13, 9, 0, 0, 0, time.UTC)

	var shown memory.View
	var said []memory.Bit
	for i := range 8 {
		var b memory.Bit
		shown, b = shown.Add(s, memory.Bit{
			At:      at.Add(time.Duration(i) * time.Minute),
			From:    me,
			Channel: "tui",
			Payload: memory.Utterance{Text: fmt.Sprintf("bit %d", i)},
			Prev:    shown.Head(),
		})
		said = append(said, b)
	}

	folded, ok := shown.Fold(s, 3, memory.Stay{})
	if !ok {
		t.Fatal("the fixture did not fold; every test below is then about a simpler file than it says")
	}
	shown = folded

	var votes memory.View
	votes, _ = votes.Add(s, memory.Cast(at.Add(9*time.Minute), me, memory.Up, said[7]))
	votes, _ = votes.Add(s, memory.Cast(at.Add(10*time.Minute), me, memory.Down, said[1]))

	return record{store: s, shown: shown, votes: votes}
}

// bytesOf is the fixture encoded, which several tests then damage.
func bytesOf(t *testing.T, r record) []byte {
	t.Helper()

	var buf bytes.Buffer
	if err := r.encode(&buf); err != nil {
		t.Fatalf("encoding the fixture: %v", err)
	}
	return buf.Bytes()
}

// A record written by one process is the record the next one opens: the same
// bits under the same address, and both views in the order they were in.
//
// Through save and load rather than encode and decode, so the file, its
// directory, its mode and the rename are all in the path being asserted about.
func TestASavedRecordComesBackWhole(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "record")
	before := fixture(t)

	if err := before.save(path); err != nil {
		t.Fatalf("save: %v", err)
	}
	after, err := load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if got, want := after.store.Address(), before.store.Address(); got != want {
		t.Errorf("record address %s, want %s", memory.Short(got), memory.Short(want))
	}
	if got, want := after.store.Len(), before.store.Len(); got != want {
		t.Errorf("record holds %d bits, want %d", got, want)
	}
	if !slices.Equal(after.shown, before.shown) {
		t.Errorf("%s came back as %v, want %v", shownName, after.shown, before.shown)
	}
	if !slices.Equal(after.votes, before.votes) {
		t.Errorf("%s came back as %v, want %v", votesName, after.votes, before.votes)
	}

	// The order is the format — nothing in the file labels which view is which —
	// so a round trip that only compared the two as sets would pass with them
	// swapped. slices.Equal above is ordered; this is the case it rules out,
	// stated separately because it is the failure the format is exposed to.
	if slices.Equal(after.shown, before.votes) {
		t.Error("the two views came back interchangeable, so this test cannot tell them apart")
	}
}

// Two writers over one file, and the one that saves second does not erase what
// the other put on the record.
//
// This is D1 at the smallest scale the product has. A save is the whole file, so
// without [record.absorb] the second writer's record is the only record — and
// this program ships two writers on purpose: a session checkpointing after every
// change, and `tldr say` writing from outside one. Both go through
// [record.save], which is why one test covers both directions; the two rows
// differ in *what* wrote first, and the writer that saves last is the same shape
// in each.
//
// The view is the other half of the claim and it points the opposite way. What
// the other writer said must **not** appear on the second writer's screen: a
// view is a value that session holds, nothing reaches into a running process,
// and merging views would put a row in somebody's transcript without them asking
// and with no honest answer about where it goes. A view is allowed to forget;
// the record is not.
//
// That half used to be asserted against the wrong view, and the wrong view made
// it a specification of a defect (D52(c)'s shape). The prose says *the second
// writer's screen*, and the assertion was on the record reloaded from the file
// afterwards — which is not a screen, it is the next session. So the check
// required the other writer's bit to be in no transcript at all, permanently,
// which is exactly what [record.rejoin] was built to stop. It is now asserted
// where the sentence points, on the view the second writer is still holding, and
// the reload carries the opposite assertion beside it.
//
// What this does not test, because one process cannot produce it: two saves
// genuinely in flight at once. The window [record.save] leaves is between its
// read and its rename, and closing that needs a lock, which that function argues
// against by name.
func TestTheWriterThatSavesSecondKeepsTheOthersBits(t *testing.T) {
	// beside adds one utterance to a record held in memory, the way a surface
	// does — into the store and into the view that names it.
	beside := func(t *testing.T, r *record, by memory.Handle, text string) memory.Bit {
		t.Helper()

		var b memory.Bit
		r.shown, b = r.shown.Add(r.store, memory.Bit{
			At:      time.Date(2026, 8, 13, 10, 0, 0, 0, time.UTC),
			From:    by,
			Channel: "tui",
			Payload: memory.Utterance{Text: text},
			Prev:    r.shown.Head(),
		})
		return b
	}

	tests := []struct {
		name string

		// first writes to the file after the second writer has already loaded it,
		// and returns the address it added.
		first func(t *testing.T, path string) string
	}{
		{
			name: "said from the command line",
			first: func(t *testing.T, path string) string {
				t.Helper()
				out, _, err := ran(t, commands["say"], path, "",
					"-as", "an-agent", "said beside a session that is still open")
				if err != nil {
					t.Fatalf("say: %v", err)
				}
				return strings.TrimSpace(out)
			},
		},
		{
			name: "checkpointed by another session",
			first: func(t *testing.T, path string) string {
				t.Helper()
				other, err := load(path)
				if err != nil {
					t.Fatalf("the other session's load: %v", err)
				}
				b := beside(t, &other, memory.Handle{Ref: "them", Display: "them"}, "said in the other session")
				if err := other.save(path); err != nil {
					t.Fatalf("the other session's checkpoint: %v", err)
				}
				return b.ID
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := saved(t, fixture(t))
			was, err := load(path)
			if err != nil {
				t.Fatal(err)
			}

			// The second writer loads here and saves at the end, so everything the
			// other writer does happens inside its window.
			held, err := load(path)
			if err != nil {
				t.Fatal(err)
			}
			mine := beside(t, &held, me, "and this is what the session itself was saying")

			theirs := tt.first(t, path)
			if _, ok := held.store.Get(theirs); ok {
				t.Fatalf("the second writer already holds %s, so it has nothing to take in and "+
					"this test asserts nothing", memory.Short(theirs))
			}

			if err := held.save(path); err != nil {
				t.Fatalf("the second writer's save: %v", err)
			}

			back, err := load(path)
			if err != nil {
				t.Fatalf("the record does not load after both writers: %v", err)
			}
			for _, want := range []struct{ what, id string }{
				{"what was written beside it", theirs},
				{"what it said itself", mine.ID},
			} {
				if _, ok := back.store.Get(want.id); !ok {
					t.Errorf("%s (%s) is not on the record; the save that came second erased it",
						want.what, memory.Short(want.id))
				}
			}
			if got, want := back.store.Len(), was.store.Len()+2; got != want {
				t.Errorf("the record holds %d bits, want %d — one from each writer on top of %d",
					got, want, was.store.Len())
			}

			if !slices.Contains(back.shown, mine.ID) {
				t.Errorf("%s is not in %s, and it is what the writer that saved last was showing",
					memory.Short(mine.ID), shownName)
			}
			if slices.Contains(held.shown, theirs) {
				t.Errorf("%s reached the second writer's own %s; the record takes in what another "+
					"writer said and the running session's view does not, because nothing here can "+
					"say where in somebody's transcript it goes", memory.Short(theirs), shownName)
			}
			if !slices.Contains(back.shown, theirs) {
				t.Errorf("%s is on the record and in no view of it; the session that opens this file "+
					"next will not draw what the other writer said", memory.Short(theirs))
			}
		})
	}
}

// One command, one outcome. `tldr say` reaches the next session's transcript
// whether or not somebody had a session open while it ran.
//
// This is the property and not the mechanism, which is why it is asserted through
// the file rather than over [record.rejoin]: what a person can observe is that
// they ran a command and then opened the program. Before this held, the two
// branches disagreed permanently — with a session open the bit reached the store
// and no view at all, because that session's checkpoint wrote its own transcript
// over the one `say` had left; with nothing else running the identical command
// put it on the next session's first screen. Neither outcome was announced, and
// the thing selecting between them was whether a terminal happened to be open
// somewhere else on the machine.
//
// The session row asserts the stranding is actually reached, which is the half
// that keeps this from passing on a fixture that never produced the case: the
// session's own view must still not name the bit at the moment it saves. That
// assertion is also the deliberate half of the design — nothing reaches into a
// running process, and this fix did not change that.
func TestSayingBesideAnOpenSessionReachesTheNextOneJustTheSame(t *testing.T) {
	const text = "a handoff note written from outside the surface"

	tests := []struct {
		name string

		// open is whoever else is holding the record while the command runs, and
		// nil is nobody. It loads before `say` and checkpoints after, which is the
		// order that made the two branches disagree.
		open func(t *testing.T, path string) record
	}{
		{name: "with nothing else running"},
		{
			name: "with a session open the whole time",
			open: func(t *testing.T, path string) record {
				t.Helper()
				r, err := load(path)
				if err != nil {
					t.Fatalf("the session's load: %v", err)
				}
				return r
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := saved(t, fixture(t))

			var sess record
			if tt.open != nil {
				sess = tt.open(t, path)
			}

			out, _, err := ran(t, commands["say"], path, "", "-as", "an-agent", text)
			if err != nil {
				t.Fatalf("say: %v", err)
			}
			said := strings.TrimSpace(out)

			if sess.store != nil {
				// The person presses a key. A bit lands and the checkpoint writes
				// the whole file, the session's own views included.
				sess.shown, _ = sess.shown.Add(sess.store, memory.Bit{
					At:      time.Date(2026, 8, 13, 10, 0, 0, 0, time.UTC),
					From:    me,
					Channel: "tui",
					Payload: memory.Utterance{Text: "and this is what the session itself was saying"},
					Prev:    sess.shown.Head(),
				})
				if err := sess.save(path); err != nil {
					t.Fatalf("the session's checkpoint: %v", err)
				}
				if slices.Contains(sess.shown, said) {
					t.Fatalf("%s is in the open session's own %s, so this row never reached the case "+
						"it is about and would pass against a transcript nothing repairs",
						memory.Short(said), shownName)
				}
			}

			next, err := load(path)
			if err != nil {
				t.Fatalf("the next session's load: %v", err)
			}
			if _, ok := next.store.Get(said); !ok {
				t.Fatalf("%s is not on the record at all", memory.Short(said))
			}
			if !slices.Contains(next.shown, said) {
				t.Errorf("%s is on the record and %s does not name it, so the next session opens "+
					"without it in the transcript, in the fold or in what the persona is told — "+
					"which is what running the same command with nothing else open would have given",
					memory.Short(said), shownName)
			}
		})
	}
}

// A load puts back what nothing accounts for, and moves nothing else.
//
// Hand-built rather than driven through the file, because the rows that matter
// are ones no sequence of commands produces: a stray older than the newest row, a
// record with no stray at all, and — the one that would be catastrophic — a
// transcript whose fold this could undo. `rejoin` asks whether a scar in the view
// absorbed the bit, and the tempting simplification is to ask only whether the
// view names it. Under that version every folded bit in every record is a stray,
// and opening the program un-folds the whole conversation. The "a fold is not
// undone" row is the only thing in this repository that would say so.
//
// Every want is written out by hand from the stated rule, not computed through
// the function under test.
func TestALoadPutsAStrayBackWhereItWasSaidAndMovesNothingElse(t *testing.T) {
	at := func(m int) time.Time {
		return time.Date(2026, 8, 16, 9, 0, 0, 0, time.UTC).Add(time.Duration(m) * time.Minute)
	}

	// said files a bit in the store without putting it in any view, which is the
	// state another writer's bit is in by the time this runs.
	said := func(s *memory.Store, m int, text string) memory.Bit {
		return s.Put(memory.Bit{
			At:      at(m),
			From:    me,
			Channel: "tui",
			Payload: memory.Utterance{Text: text},
		})
	}

	tests := []struct {
		name string

		// build returns the record as it came off the file, and the transcript
		// the rule says a load owes back.
		build func(t *testing.T) (record, memory.View)
	}{
		{
			name: "a record nobody wrote beside comes back untouched",
			build: func(t *testing.T) (record, memory.View) {
				t.Helper()
				s := memory.NewStore()
				a, b, c := said(s, 0, "one"), said(s, 1, "two"), said(s, 2, "three")
				shown := memory.View{a.ID, b.ID, c.ID}
				return record{store: s, shown: shown}, memory.View{a.ID, b.ID, c.ID}
			},
		},
		{
			name: "a stray lands between the rows it was said between",
			build: func(t *testing.T) (record, memory.View) {
				t.Helper()
				s := memory.NewStore()
				a, b, c := said(s, 0, "one"), said(s, 2, "three"), said(s, 4, "five")
				stray := said(s, 3, "said beside a session, after three and before five")
				return record{store: s, shown: memory.View{a.ID, b.ID, c.ID}},
					memory.View{a.ID, b.ID, stray.ID, c.ID}
			},
		},
		{
			name: "a stray older than everything shown goes first",
			build: func(t *testing.T) (record, memory.View) {
				t.Helper()
				s := memory.NewStore()
				a, b := said(s, 4, "five"), said(s, 6, "seven")
				stray := said(s, 1, "said before either of them")
				return record{store: s, shown: memory.View{a.ID, b.ID}},
					memory.View{stray.ID, a.ID, b.ID}
			},
		},
		{
			name: "a stray newer than everything shown goes last",
			build: func(t *testing.T) (record, memory.View) {
				t.Helper()
				s := memory.NewStore()
				a, b := said(s, 0, "one"), said(s, 1, "two")
				stray := said(s, 9, "said after the session stopped writing")
				return record{store: s, shown: memory.View{a.ID, b.ID}},
					memory.View{a.ID, b.ID, stray.ID}
			},
		},
		{
			// Four rather than two, because the mutation this row is against is
			// dropping the sort and taking [memory.Store.All]'s address order
			// instead. Address order is a fixed permutation of these particular
			// bits, so with two strays the row is one coin flip away from passing
			// against a version with no sort at all; four leaves it one in
			// twenty-four.
			name: "strays keep the order they were said in",
			build: func(t *testing.T) (record, memory.View) {
				t.Helper()
				s := memory.NewStore()
				a := said(s, 0, "one")
				w, x := said(s, 2, "then this"), said(s, 3, "and then this")
				y, z := said(s, 4, "and this"), said(s, 5, "and finally this")
				return record{store: s, shown: memory.View{a.ID}},
					memory.View{a.ID, w.ID, x.ID, y.ID, z.ID}
			},
		},
		{
			name: "a fold is not undone",
			build: func(t *testing.T) (record, memory.View) {
				t.Helper()
				s := memory.NewStore()

				// Folded twice, so the scar on screen stands for an older scar as
				// well — Absorbed merges across generations and this is the row
				// that depends on it doing so. More bits go in between the two
				// folds because a scar alone is a run of one, which D32's size
				// rule refuses to cool.
				var shown memory.View
				say := func(from, n int) {
					for i := from; i < from+n; i++ {
						shown, _ = shown.Add(s, memory.Bit{
							At:      at(i),
							From:    me,
							Channel: "tui",
							Payload: memory.Utterance{Text: fmt.Sprintf("bit %d", i)},
							Prev:    shown.Head(),
						})
					}
				}
				fold := func() {
					folded, ok := shown.Fold(s, 3, memory.Stay{})
					if !ok {
						t.Fatal("the fixture did not fold, so nothing here is behind a scar")
					}
					shown = folded
				}
				say(0, 8)
				fold()
				say(8, 4)
				fold()

				if len(shown) >= s.Len() {
					t.Fatalf("%d bits are shown out of %d in the store, so no bit is only reachable "+
						"through a scar and this row asserts nothing", len(shown), s.Len())
				}
				return record{store: s, shown: shown}, slices.Clone(shown)
			},
		},
		{
			name: "a stray beside a fold lands without disturbing it",
			build: func(t *testing.T) (record, memory.View) {
				t.Helper()
				s := memory.NewStore()

				var shown memory.View
				for i := range 8 {
					shown, _ = shown.Add(s, memory.Bit{
						At:      at(i),
						From:    me,
						Channel: "tui",
						Payload: memory.Utterance{Text: fmt.Sprintf("bit %d", i)},
						Prev:    shown.Head(),
					})
				}
				folded, ok := shown.Fold(s, 3, memory.Stay{})
				if !ok {
					t.Fatal("the fixture did not fold, so nothing here is behind a scar")
				}
				shown = folded

				stray := said(s, 20, "said long after the fold")
				return record{store: s, shown: shown}, append(slices.Clone(shown), stray.ID)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec, want := tt.build(t)
			was := slices.Clone(rec.shown)

			got := rec.rejoin()
			if !slices.Equal(got.shown, want) {
				t.Errorf("%s came back as %v, want %v", shownName, shorten(got.shown), shorten(want))
			}

			// Insertion and never rearrangement: whatever the view already named
			// is still in the order the session that wrote it arrived at.
			kept := slices.DeleteFunc(slices.Clone(got.shown), func(id string) bool {
				return !slices.Contains(was, id)
			})
			if !slices.Equal(kept, was) {
				t.Errorf("the rows already in %s came back as %v, want %v — a load may insert into "+
					"somebody's transcript and may not rearrange it",
					shownName, shorten(kept), shorten(was))
			}
		})
	}
}

// shorten is a view as a failure message should print it: whole addresses are
// sixty-four characters and a row of them says nothing a reader can compare.
func shorten(v memory.View) []string {
	out := make([]string, 0, len(v))
	for _, id := range v {
		out = append(out, memory.Short(id))
	}
	return out
}

// A file this build cannot read stops the save rather than being replaced by it.
//
// The alternative is worse in the direction that matters: a save is the whole
// file, so overwriting bytes nothing here could parse is the largest possible way
// to forget, and it would happen to a person whose record had been damaged —
// exactly when they need what is left of it. Refusing is loud and recoverable;
// [tui.Save] keeps the session running, so the person moves the bad file aside
// and the next change carries everything.
func TestASaveWillNotReplaceARecordItCannotRead(t *testing.T) {
	path := saved(t, fixture(t))
	held, err := load(path)
	if err != nil {
		t.Fatal(err)
	}

	bad := []byte("this is not a tldreddit stream")
	if err := os.WriteFile(path, bad, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := held.save(path); err == nil {
		t.Error("the save went ahead over a file it could not read")
	} else if !strings.Contains(err.Error(), path) {
		t.Errorf("the refusal does not name the file: %v", err)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, bad) {
		t.Errorf("the unreadable file was replaced anyway: %d bytes before, %d after", len(bad), len(after))
	}
}

// The mode is a claim about who can read the conversation.
func TestASavedRecordIsPrivateToItsOwner(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state", "record")
	if err := fixture(t).save(path); err != nil {
		t.Fatalf("save: %v", err)
	}

	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := fi.Mode().Perm(); got != 0o600 {
		t.Errorf("the record is mode %04o, want 0600", got)
	}
	di, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if got := di.Mode().Perm(); got != 0o700 {
		t.Errorf("the directory is mode %04o, want 0700", got)
	}
}

// No file is a first run, and it is the only absence that is not a failure.
func TestNoFileIsAnEmptyRecordAndNotAnError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "record")

	rec, err := load(path)
	if err != nil {
		t.Fatalf("load of a path that does not exist: %v", err)
	}
	if rec.store == nil {
		t.Fatal("load returned no store, so the surface would have nothing to resolve against")
	}
	if n := rec.store.Len(); n != 0 {
		t.Errorf("a first run starts with %d bits, want 0", n)
	}
	if len(rec.shown) != 0 || len(rec.votes) != 0 {
		t.Errorf("a first run starts with views %v and %v, want both empty", rec.shown, rec.votes)
	}

	// Reading must not create it. A program that files an empty record on every
	// start makes "has this ever been run" unanswerable.
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("load created %s", path)
	}
}

// Every proper prefix of a real record is refused, and the message says which
// file.
//
// A sweep rather than one truncation, because the three streams are
// self-delimiting and each ends at a different kind of boundary: a length, a
// count, a closing tag. A single cut lands in one of them and says nothing about
// the other two.
func TestEveryTruncationIsFatalAndNamesTheFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "record")
	whole := bytesOf(t, fixture(t))

	for cut := range len(whole) {
		if err := os.WriteFile(path, whole[:cut], 0o600); err != nil {
			t.Fatal(err)
		}

		rec, err := load(path)
		if err == nil {
			t.Fatalf("a record cut to %d of %d bytes loaded cleanly, holding %d bits",
				cut, len(whole), rec.store.Len())
		}
		if !strings.Contains(err.Error(), path) {
			t.Fatalf("a record cut to %d bytes failed with %q, which does not name %s", cut, err, path)
		}
		if rec.store != nil {
			t.Fatalf("a record cut to %d bytes came back with a store beside its error", cut)
		}
	}

	// The control: the same assertions against the whole file must pass, or the
	// sweep above is a test that anything at all fails to load.
	if err := os.WriteFile(path, whole, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := load(path); err != nil {
		t.Fatalf("the untruncated record did not load: %v", err)
	}
}

// No single bit of the file can be changed without the record being refused.
//
// Bits and not bytes, because the thing most exposed here is a length or a count,
// and a whole-byte flip cannot lower a small one — 25 stored big-endian becomes
// 230, the stream runs out, and a sweep that looks exhaustive goes green over the
// one direction that produces a silently short result.
//
// Exhaustive and in memory. Which offsets belong to which of the three streams is
// not knowable from here without restating the format, and sampling three of them
// would be a claim about the two-thirds not sampled; every offset costs a parse,
// and the parses are what make this take seconds on disk and a fraction of one
// here. That the path reaches the message is the truncation test above, over every
// prefix of the same file.
func TestNoSingleBitOfTheFileCanBeChangedQuietly(t *testing.T) {
	whole := bytesOf(t, fixture(t))

	for at := range len(whole) * 8 {
		bent := slices.Clone(whole)
		bent[at/8] ^= 1 << (at % 8)

		rec, err := decode(bytes.NewReader(bent))
		if err == nil {
			t.Fatalf("bit %d of byte %d loaded cleanly, holding %d bits and views of %d and %d",
				at%8, at/8, rec.store.Len(), len(rec.shown), len(rec.votes))
		}
		if rec.store != nil {
			t.Fatalf("bit %d of byte %d came back with a store beside its error", at%8, at/8)
		}
	}

	// The control. Every assertion above is about a file that fails, so without
	// this the sweep passes against a decode that refuses everything.
	if _, err := decode(bytes.NewReader(whole)); err != nil {
		t.Fatalf("the unbent record did not load: %v", err)
	}
}

// A view saved against one record and offered to another is refused, and the
// refusal says so in words a person can act on.
//
// The arrangement is ordinary rather than exotic: a checkpoint writes the record,
// the process dies before the views, and the record grows past them. What must
// not happen is the view resolving happily and drawing a conversation with
// everything since missing.
func TestAViewFromAnotherRecordIsFatal(t *testing.T) {
	rec := fixture(t)

	// The views, stamped with the record as it stands now.
	var views bytes.Buffer
	if _, err := rec.shown.WriteAgainst(rec.store, &views); err != nil {
		t.Fatal(err)
	}
	against := rec.store.Address()

	// The record grows. Every address the views name still resolves, which is
	// exactly the case [memory.ReadViewAgainst] exists for: nothing else notices.
	rec.store.Put(memory.Bit{
		At:      time.Date(2026, 8, 13, 10, 0, 0, 0, time.UTC),
		From:    me,
		Channel: "tui",
		Payload: memory.Utterance{Text: "said after the views were written"},
	})
	grown := rec.store.Address()

	var file bytes.Buffer
	if _, err := rec.store.WriteTo(&file); err != nil {
		t.Fatal(err)
	}
	file.Write(views.Bytes())

	got, err := decode(&file)
	if err == nil {
		t.Fatalf("a view from another record loaded cleanly, %d entries", len(got.shown))
	}
	if got.store != nil {
		t.Error("a stale view came back with a record beside its error")
	}

	// Both addresses, whole. Short forms are the wrong length for what somebody
	// does next, which is comparing them against another file.
	for _, want := range []string{against, grown, shownName, "different record"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not mention %q:\n%v", want, err)
		}
	}

	// Not silently dropped, and not silently recovered either: the surface must
	// not be able to reach the view by ignoring the error.
	var stale *memory.StaleView
	if errors.As(err, &stale) {
		t.Error("the stale view is still reachable through errors.As, so a caller could draw it")
	}
}

// A vote view naming something that is not a vote is refused here rather than
// panicking out of memory.Tally on the first frame.
func TestAVoteViewOfSomethingElseIsFatal(t *testing.T) {
	rec := fixture(t)

	// A view is a list of addresses and nothing stops one naming an utterance.
	// Assembling it takes reaching past what the surface can produce, which is the
	// point: this is the file somebody hand-built.
	rec.votes = append(slices.Clone(rec.votes), rec.shown[len(rec.shown)-1])

	var file bytes.Buffer
	if err := rec.encode(&file); err != nil {
		t.Fatal(err)
	}

	got, err := decode(&file)
	if err == nil {
		t.Fatalf("a vote view holding an utterance loaded cleanly, %d entries", len(got.votes))
	}
	if !strings.Contains(err.Error(), "not a vote") {
		t.Errorf("the refusal does not say what is wrong:\n%v", err)
	}
}

// A view naming a bit the record does not hold is refused here, rather than
// panicking out of memory.View.Bits while the surface draws.
func TestAViewNamingAMissingBitIsFatal(t *testing.T) {
	rec := fixture(t)
	rec.shown = append(slices.Clone(rec.shown), strings.Repeat("00", 32))

	var file bytes.Buffer
	if err := rec.encode(&file); err != nil {
		t.Fatal(err)
	}

	got, err := decode(&file)
	if err == nil {
		t.Fatalf("a view naming a bit nothing holds loaded cleanly, %d entries", len(got.shown))
	}
	if !strings.Contains(err.Error(), "does not hold") {
		t.Errorf("the refusal does not say what is wrong:\n%v", err)
	}
}

// full is how many bytes the fixture's file takes, so a test can cut inside it
// and know it did.
func full(t *testing.T) int {
	t.Helper()
	return len(bytesOf(t, fixture(t)))
}

// failAfter passes n bytes through and then behaves like a disk with nothing
// left on it: a short write and an error, which is what os.File does.
type failAfter struct {
	w io.Writer
	n int
}

var errFull = errors.New("no space left on device")

func (f *failAfter) Write(p []byte) (int, error) {
	if f.n <= 0 {
		return 0, errFull
	}
	if len(p) > f.n {
		n, _ := f.w.Write(p[:f.n])
		f.n = 0
		return n, errFull
	}
	n, err := f.w.Write(p)
	f.n -= n
	return n, err
}

// A save that fails partway leaves the previous record exactly as it was.
//
// The last row is the control and it is the reason the other three mean
// anything: with the same assertions and no failure injected, the file must
// change. Without that row this test would pass against an [atomically] that
// never wrote anything at all, which is the shape this project has shipped twice
// (D27, D48).
func TestAFailedSaveLeavesThePreviousRecordIntact(t *testing.T) {
	size := full(t)

	for _, tc := range []struct {
		name     string
		through  int
		wantFail bool
	}{
		{"nothing written at all", 0, true},
		{"stopped inside the record", size / 3, true},
		{"stopped inside the views", size - 20, true},
		{"the control: nothing fails", size * 2, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "record")

			// The previous good record: the fixture, saved the ordinary way.
			old := fixture(t)
			if err := old.save(path); err != nil {
				t.Fatalf("saving the previous record: %v", err)
			}
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}

			// The session that follows it says one more thing, so the file it
			// would write is a different file.
			next := fixture(t)
			next.shown, _ = next.shown.Add(next.store, memory.Bit{
				At:      time.Date(2026, 8, 13, 11, 0, 0, 0, time.UTC),
				From:    me,
				Channel: "tui",
				Payload: memory.Utterance{Text: "one more thing"},
				Prev:    next.shown.Head(),
			})

			err = atomically(path, func(w io.Writer) error {
				return next.encode(&failAfter{w: w, n: tc.through})
			})
			if tc.wantFail && !errors.Is(err, errFull) {
				t.Fatalf("a save that ran out of disk after %d bytes returned %v", tc.through, err)
			}
			if !tc.wantFail && err != nil {
				t.Fatalf("the control save failed: %v", err)
			}

			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			switch {
			case tc.wantFail && !bytes.Equal(before, after):
				t.Errorf("the previous record changed: %d bytes before, %d after", len(before), len(after))
			case !tc.wantFail && bytes.Equal(before, after):
				t.Error("the control save left the file unchanged, so every row above proves nothing")
			}

			// A directory filling with half-saved records is its own way of
			// losing the real one.
			names, err := os.ReadDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			if len(names) != 1 || names[0].Name() != filepath.Base(path) {
				var left []string
				for _, e := range names {
					left = append(left, e.Name())
				}
				t.Errorf("the directory holds %v, want only the record", left)
			}
		})
	}
}

// The record that failed to be written is still loadable, which is the property
// the byte comparison above is standing in for.
func TestTheRecordSurvivesASaveThatFailed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "record")
	old := fixture(t)
	if err := old.save(path); err != nil {
		t.Fatal(err)
	}

	err := atomically(path, func(w io.Writer) error {
		return old.encode(&failAfter{w: w, n: 40})
	})
	if !errors.Is(err, errFull) {
		t.Fatalf("the injected failure did not surface: %v", err)
	}

	back, err := load(path)
	if err != nil {
		t.Fatalf("the previous record no longer loads: %v", err)
	}
	if got, want := back.store.Address(), old.store.Address(); got != want {
		t.Errorf("record address %s, want %s", memory.Short(got), memory.Short(want))
	}
}

// Where the record lives, across the three ways of saying it and the one way of
// failing to.
func TestRecordPath(t *testing.T) {
	for _, tc := range []struct {
		name            string
		tldr, xdg, home string
		want            string
		wantErrMentions string
	}{
		{
			name: "$TLDR_RECORD wins outright",
			tldr: "/scratch/demo", xdg: "/state", home: "/home/me",
			want: "/scratch/demo",
		},
		{
			name: "the XDG state directory when there is no override",
			xdg:  "/state", home: "/home/me",
			want: filepath.Join("/state", "tldreddit", "record"),
		},
		{
			name: "the default under home when neither is set",
			home: "/home/me",
			want: filepath.Join("/home/me", ".local", "state", "tldreddit", "record"),
		},
		{
			// An empty XDG_STATE_HOME is the specification's own "unset", and
			// obeying it literally files the record at the relative path
			// "tldreddit/record" — filepath.Join drops the empty element, so
			// the leading slash a reader assumes is there is not. Measured
			// rather than reasoned: it was written down as "/tldreddit/record"
			// until this row was run against the version that gets it wrong.
			name: "an empty XDG_STATE_HOME is not a relative record",
			xdg:  "", home: "/home/me",
			want: filepath.Join("/home/me", ".local", "state", "tldreddit", "record"),
		},
		{
			name:            "nowhere to put it is an error, not a relative path",
			wantErrMentions: "TLDR_RECORD",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("TLDR_RECORD", tc.tldr)
			t.Setenv("XDG_STATE_HOME", tc.xdg)
			t.Setenv("HOME", tc.home)

			got, err := recordPath()
			if tc.wantErrMentions != "" {
				if err == nil {
					t.Fatalf("no error, and the record would go to %q", got)
				}
				if !strings.Contains(err.Error(), tc.wantErrMentions) {
					t.Errorf("the error does not mention %q: %v", tc.wantErrMentions, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("recordPath: %v", err)
			}
			if got != tc.want {
				t.Errorf("the record goes to %q, want %q", got, tc.want)
			}
		})
	}
}
