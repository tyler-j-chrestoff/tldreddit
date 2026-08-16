# Claims

Everything this repository says is guarded, and the exact break that proves the
guard is real.

A claim here is one sentence: *if this behaviour were broken, these checks would
fail.* That sentence is checkable, so it is checked — `cmd/seam` re-breaks the
behaviour in a copy of the working tree and runs the suite against it. A cited
check that goes on passing was never holding the claim up; it was only present.

```
go run ./cmd/seam            # every claim
go run ./cmd/seam -list      # the catalog, without running it
go run ./cmd/seam -run <id>  # one claim
```

**This file is the catalog.** The tool parses the `seam` blocks below; there is
no second copy of them in the source, because two statements of one thing drift
and the drift is the failure this whole exercise is about. Edit a claim here and
the tool runs the edited one.

**What is in a block.** `find` is an exact substring of `file` — `occ` picks
which occurrence when it is not unique — and `after` replaces it. `red` names the
checks that must fail, which is the claim itself. `sole` says nothing outside
`red` may fail; `among` narrows that comparison to a named set, for a claim that
compares checks against each other rather than against the suite. `race` runs
under the detector. `runs` is a sample size — see below. `isolate` runs each
cited check alone, in its own package, for the one shape of claim a whole run
cannot see: a mutation whose damage kills the process takes every check after it
off the run, so the second and third cited check never get to speak however many
times it runs. Alone *and* in its own package, because a check run by itself
against the whole module still builds and runs every other package beside it, and
that load is enough to change which of two racers finishes first. What isolation
gives up is printed on the claim rather than left implied — a check that only
reddens with nothing else running is a weaker fact than one that reddens in the
suite. `verdict` is what the claim expects of itself — see below. `\n` and `\t`
are the escapes, so a multi-line anchor is expressible.

**A claim declares its own verdict, and the gate is an equality against it.**
`verdict` defaults to `proven` and is absent from nearly every block. Where a
claim honestly cannot do better — the second one below cannot assert, because the
process dies first — it says so in the block, beside the prose explaining why.
That is not a suppression and not a list of known failures somewhere nobody
looks: the gate fails when a claim is not where it says it is **in either
direction**, so a claim that quietly starts passing cleanly trips it exactly as
loudly as one that stops. A claim whose own account of itself has gone out of date
is the defect, whichever way it moved. Exit 0 means every claim is where it says
it is; it does not mean every claim is proven.

**A gate may not be flaky by construction, and the declaration is what handles
that — not an exemption from it.** Some claims are honestly nondeterministic: the
store's unlocked reader is one, where the runtime's throw usually kills the check
before its own assertion runs and occasionally does not, so the honest answer is
two verdicts and neither is a surprise. Such a block declares the set —
`verdict: proven|killed-mid-check` — and the gate on the set is deterministic
where a gate on either member alone would flap.

**A set is not an opt-out, and an earlier attempt at this was.** The first version
of this paragraph described a `gated: false` key that let a claim stand down from
the exit status entirely. That key is deleted. It was wrong for a reason worth
keeping: an ungated claim's declaration was unfalsifiable, so it opted out not only
of the gate but of the staleness check the declaration exists to be — and if the
cited check had been weakened until the mutation stopped reddening it at all,
nothing would have said so. That is this instrument's own subject matter, inside
the instrument. Widening the mechanism already here was the correct move and the
new one was not needed.

So: every verdict outside a claim's declared set fails, in either direction. A set
must be **exhaustive**, and the block must show the measurement justifying every
member — a two-member set is a claim about nondeterminism and carries the same
burden of proof as any other claim in this file. The bar is not "this claim is
inconvenient" but "these are the outcomes this claim can honestly have, and here
is the measurement".

**A red is not evidence without a green control, and there are three of them.**
Before any mutant runs, the unmutated tree runs whole; a working tree that is not
green is a refusal rather than a run with a caveat, because every verdict here is
a difference between two runs and there is nothing to subtract from a red one.
That run also decides which checks *exist and ran* — a cited check that was
renamed away, or that skipped, is exactly as much evidence as one that was
deleted, and gets `stale-citation` rather than a green it never earned. And per
claim, the cited checks are sampled `runs` times on the unmutated tree as well as
on the mutated one, in the same shape and the same environment. A claim is
`proven` only where the unmutated rate is 0 out of N and the mutated rate is at
least 1; where the control reddens even once the claim is `unattributable`, and
both rates are printed for every claim whatever the verdict. The measured control
in this catalog is zero out of N on every claim in it, which is what a control is
supposed to look like and is exactly why it has to be printed rather than assumed.
The answer to an unattributable claim is never to sample it until it behaves.

*A previous draft of this paragraph cited "five runs in ten" from
`memory/store.go` as a noise floor. That figure is measured with the locking
removed — it is a mutant-side rate, not a control-side one — and it was cited
without its command in the file whose own rule is that a measurement of a race is
a measurement of a command. Withdrawn here rather than deleted, because the shape
of the error is the same one this file exists to catch.*

The cost of all that is real: the control doubles the work, nothing is
short-circuited once a check has reddened, and the whole catalog takes minutes
rather than seconds. That is the price of the second rate, and it is not
negotiable down.

**A red proves nothing until the mutated copy also builds outside its test
package, and this tool does not check that.** `go test` compiles a package's
non-test and test files together into one binary, so a mutation that deletes
or mis-targets a production declaration can resolve silently against an
identically named identifier in a `_test.go` file instead of failing to
compile at all. `go test` still reports `ok`, a claim built on the mutation
can still come back `proven` or `vacuous`, and neither verdict is about the
behaviour the claim names — it is about which symbol the linker happened to
find. `tui`'s test files are the exposed case, found rather than reasoned to:
they declare roughly 48 package-scope identifiers with ordinary names —
`lines`, `rows`, `block`, `first`, `page`, `scar`, `record`, `screen`,
`split`, `flat` among them (`var lines = []string{…}` at
`tui/harness_test.go:130`) — and a mutation aimed at a same-named production
symbol can land on one of these instead. Measured directly, by hand, outside
this tool: `go build ./tui/` reported `undefined: lines` while `go test
./tui/` reported `ok` in the same second, against a mutation that a cited
check should have caught and came back caught-by-nothing.

`cmd/seam/tree.go`'s `occurrences` (`tree.go:91-97`) and `mutate`
(`tree.go:102-120`) already narrow this a long way — an anchor has to appear
exactly once in the file it names, and a mutation that changes nothing is
refused outright — so a mis-target this blunt is far harder to write here
than it was in the hand-rolled script that hit it. What is left is narrower
than that sounds: this tool cannot tell a check that reddened because the
claimed behaviour broke from one that reddened, or stayed green, because the
production build broke while the test build silently compiled anyway.
`cmd/seam/run.go`'s `runSuite` runs `go test` and only `go test`
(`run.go:148-156`); nothing here ever runs `go build` against a mutated copy.

**So: a mutation is evidence only after `go build ./<pkg>/` has agreed the
mutant compiles as non-test code.** Run that by hand against the same
mutated copy before trusting what this tool reports, until something in this
catalog does it for you. That last clause is a recommendation and not a
decision this paragraph is making: folding the build into `runSuite` would
close the gap for good, but D27 is the standing reason not to reach for a new
instrument the moment a gap is named, and whether this one clears that bar is
the CEO's call, not this file's.

**A vacuous verdict is not repaired by whoever found it.** It is reported, and
somebody who is not mid-build decides what to do about it — because a builder who
strengthens his own vacuous check is the loop this tool exists to break, and a
catalog curated until it is all green is a catalog that has stopped reading. One
has been through that cycle already; it is the last claim in this file, and it
says what it was and who ruled.

**What it cannot see.** Only what is cited. A claim nobody wrote down here is
exactly as unchecked as it was before this file existed, and a behaviour with no
check at all cannot be distinguished from one whose check is perfect. There is no
score, deliberately: a percentage would go up when a claim is deleted.

---

## The store's lock, and what depends on it

`memory/store.go`'s own doc comment says the locking is "exercised rather than
argued: race_test.go contends this lock on purpose, and taking the locking out
fails those tests while every other test in the package goes on passing green
under -race". `docs/CODE.md` repeats it as "removing `Store`'s locking fails all
four" — it was `CLAUDE.md` that repeated it until D47's cut moved the
file-by-file inventory out, and this paragraph and the two below it went on
naming the charter after the sentence had left it. Both were measured by hand,
once.

Taking out one `Lock` leaves an unbalanced `Unlock`, so the mutation swaps the
mutex for a type with the same four methods and no behaviour — the store still
compiles, still has a `mu`, and no longer excludes anything.

**`isolate` is here because the suite cannot see this claim whole**, and finding
that out is what the first run of this catalog was worth. Unlocked, the store
throws `fatal error: concurrent map read and map write`, which is unrecoverable
and ends the test binary wherever it lands — so the first of these checks reddens
and the rest never run at all. `memory/store.go`'s own comment already knows this
("0 in 6 with the whole package, where a test earlier in the file throws first")
and prescribes one test at a time. `docs/CODE.md`'s summary, "removing `Store`'s
locking fails all four", is true, and it is not re-derivable from a run of the
suite.

Run alone, each of the four fails. Three of them fail by their own assertion —
one nearly every sample, one rarely — so `runs` here is a sample size, and the
report prints the rate on both sides rather than a yes. Every sample is taken on
both sides even after a check has reddened, because two rates counted differently
are two measurements and one conclusion.

The rates this claim prints are of one command, which `isolate` picks: `go test
-race -count=1 -run '^<check>$' <the check's own package>`. A rate measured any
other way is a different experiment — measured 2026-08-12,
`TestConcurrentFoldsAgreeWithOneSequentialRun` asserted 4 of 12 samples against
`./memory/` and 1 of 9 against `./...`.

The fourth is slower to assert than these three by an order of magnitude, and it
gets its own entry below with its own rate and its own declaration.

```seam
id: store-unlocked
file: memory/store.go
find: \tmu   sync.RWMutex\n\tbits map[string]Bit\n}
after: \tmu   nolock\n\tbits map[string]Bit\n}\n\ntype nolock struct{}\n\nfunc (nolock) Lock()    {}\nfunc (nolock) Unlock()  {}\nfunc (nolock) RLock()   {}\nfunc (nolock) RUnlock() {}\n\nvar _ sync.Locker = (*sync.Mutex)(nil)
red: TestConcurrentPutCollapsesIdenticalContent, TestConcurrentFoldsAgreeWithOneSequentialRun, TestConcurrentAddDoesNotShareAViewsSpareCapacity
race: true
isolate: true
runs: 16
```

## The unlocked store usually kills the reader before it can assert

Same mutation as above, cited against the fourth of the four checks. Unlocked, the
interleaving `TestConcurrentGetSeesSettledBitsOnly` builds trips the runtime's own
concurrent map detection, which is a throw rather than a failure a test can
report: the process dies, and a check that dies has not shown it is watching
anything, since a check with no assertions in it would die exactly as loudly.

**Usually, and not always. The version of this section that said otherwise was
reported to the CEO and relayed onward before it was withdrawn, and the withdrawal
is the part of this entry worth reading.** What was reported: that the four checks
`docs/CODE.md` says the lock is held up by fail in two categorically different
ways —
three by asserting, this one by crashing. That sentence was built on a figure of
twenty samples and no assertion, and the figure was measured with a command this
row does not run.

The rule that caught it was written the same day, by the same hand that then broke
it: *a measurement of a race is a measurement of a command.* It was ruled a
standing rule on a Tuesday afternoon and falsified its own author's headline claim
within the hour. A file that recorded only the final number would have kept the
number and lost that, which is the more useful half.

The withdrawn figure read "twenty samples, twenty throws, no assertion". It was
true of

```
go test -race -count=1 -run '^TestConcurrentGetSeesSettledBitsOnly$' ./memory/
```

and is not the command this row runs. The command this row runs is the one
`isolate` builds, and it is a different experiment:

```
go test -json -race -count=1 -timeout=120s \
  -run '^TestConcurrentGetSeesSettledBitsOnly$' \
  github.com/tyler-j-chrestoff/tldreddit/memory
```

Twenty samples of *that*, measured 2026-08-12: **one assertion, nineteen throws.**
So the check does get its own failure in, and rarely. The categorical sentence is
withdrawn: this is the slowest of the four by a long way, and that is a difference
of degree rather than of kind.

**So this claim declares two verdicts, and the set is exhaustive rather than
convenient.** `runs` is 1, so the row reports one sample, and one sample of a
check that asserts about once in twenty can come back either way. Both ways mean
the same thing — *the check noticed* — and they are the only two available: the
other verdicts cannot arrive here by chance. `vacuous` would mean the unlocked
store stopped reddening this check at all; `unattributable` would mean it reddens
with the lock in place; `stale-anchor` and `broken-build` are properties of the
mutation, which is the same one the claim above uses and is checked there. Each of
those would be a real regression and each still fails the gate.

That is what makes the set deterministic where a single verdict flapped, and why
it is not an exemption: the two members are measured (one assertion, nineteen
throws, by the command above), the rest are excluded by argument, and anything
outside is still caught.

Narrowing it later is one edit: find a form of this check whose verdict is stable
at a sample size somebody can afford, and drop the member that stops arriving.

```seam
id: store-unlocked-kills-the-reader
file: memory/store.go
find: \tmu   sync.RWMutex\n\tbits map[string]Bit\n}
after: \tmu   nolock\n\tbits map[string]Bit\n}\n\ntype nolock struct{}\n\nfunc (nolock) Lock()    {}\nfunc (nolock) Unlock()  {}\nfunc (nolock) RLock()   {}\nfunc (nolock) RUnlock() {}\n\nvar _ sync.Locker = (*sync.Mutex)(nil)
red: TestConcurrentGetSeesSettledBitsOnly
race: true
isolate: true
runs: 1
verdict: proven|killed-mid-check
```

## The view's capped append

`View` is a value and carries no synchronization; its whole safety is the full
slice expression in `Add`, which forces every append to allocate so two holders
cannot grow into each other's spare capacity. Stated in `memory/view.go`, in
`docs/DEBT.md`'s list — `CLAUDE.md` until D47's cut moved it, the same stale
attribution as the store's lock above — and in this seat's craft record, which
is the only one of the three that also says removing the cap fails *exactly one*
test. That extra half-sentence is the part nothing else held up, and it is why
this block exists.

```seam
id: view-add-uncapped
file: memory/view.go
find: v[:len(v):len(v)]
after: v
red: TestConcurrentAddDoesNotShareAViewsSpareCapacity
sole: true
race: true
runs: 3
```

## The fade is drawn in space, not only in colour

Seven mutations across three checks, hand-run once and written into
`.claude/craft/tui-design-engineer.md` as a table, with the claim that every
mutation was caught and every check was the sole catcher of at least one. That
second half is what `sole` makes checkable.

`among` narrows each row to the three checks the table compares. The table is an
argument about those three and says nothing about the rest of the suite — a
mutation to `step` moves measured floors elsewhere, and reporting that as the
table being wrong would be reporting a claim it never made.

```seam
id: fade-step-removed
file: tui/render.go
find: step = 2
after: step = 0
red: TestTheFadeIsDrawnInSpaceAndNotOnlyInColour
sole: true
among: TestTheFadeIsDrawnInSpaceAndNotOnlyInColour, TestALoneJoggedRowIsAlwaysBesideAScar, TestWideningTheTerminalNeverTakesAnythingOffARow
```

```seam
id: fade-step-one
file: tui/render.go
find: step = 2
after: step = 1
red: TestTheFadeIsDrawnInSpaceAndNotOnlyInColour
sole: true
among: TestTheFadeIsDrawnInSpaceAndNotOnlyInColour, TestALoneJoggedRowIsAlwaysBesideAScar, TestWideningTheTerminalNeverTakesAnythingOffARow
```

```seam
id: fade-everything-going
file: tui/render.go
find: going := f.absorbing[b.ID]
after: going := true
red: TestTheFadeIsDrawnInSpaceAndNotOnlyInColour
sole: true
among: TestTheFadeIsDrawnInSpaceAndNotOnlyInColour, TestALoneJoggedRowIsAlwaysBesideAScar, TestWideningTheTerminalNeverTakesAnythingOffARow
```

```seam
id: fade-caret-row-does-not-step
file: tui/render.go
find: \t\t\t\treturn hot.Render(caret)\n
after: \t\t\t\treturn hot.Render(caret) + strings.Repeat(" ", step)\n
red: TestTheFadeIsDrawnInSpaceAndNotOnlyInColour
sole: true
among: TestTheFadeIsDrawnInSpaceAndNotOnlyInColour, TestALoneJoggedRowIsAlwaysBesideAScar, TestWideningTheTerminalNeverTakesAnythingOffARow
```

A going scar stepping into the caret's lane is the row that earns two checks
their place: it moves the scar *and* stops it being identical to its twin, so
both notice. Moving the scar's rule off column 0 without otherwise changing it is
the row only the lone-jogged-row check sees, and the row that was missing for a
review round.

```seam
id: fade-going-scar-steps
file: tui/render.go
find: \tif !scar {
after: \tif !scar || going {
red: TestTheFadeIsDrawnInSpaceAndNotOnlyInColour, TestALoneJoggedRowIsAlwaysBesideAScar
sole: true
among: TestTheFadeIsDrawnInSpaceAndNotOnlyInColour, TestALoneJoggedRowIsAlwaysBesideAScar, TestWideningTheTerminalNeverTakesAnythingOffARow
```

```seam
id: fade-scar-off-column-zero
file: tui/render.go
find: \treturn seamInk.Render("──")
after: \treturn " " + seamInk.Render("─")
red: TestALoneJoggedRowIsAlwaysBesideAScar
sole: true
among: TestTheFadeIsDrawnInSpaceAndNotOnlyInColour, TestALoneJoggedRowIsAlwaysBesideAScar, TestWideningTheTerminalNeverTakesAnythingOffARow
```

The last row restores the stale margin the vote column's threshold used to read,
which made the sentence column non-monotonic in the terminal's width.

```seam
id: vote-threshold-stale-margin
file: tui/render.go
find: width-caretColumn-step-vote-widest(names)-colGap
after: width-caretWidth-vote-widest(names)-colGap
red: TestWideningTheTerminalNeverTakesAnythingOffARow
sole: true
among: TestTheFadeIsDrawnInSpaceAndNotOnlyInColour, TestALoneJoggedRowIsAlwaysBesideAScar, TestWideningTheTerminalNeverTakesAnythingOffARow
```

## A derived bit's Prev is every bit in the window

The answer this replaced orphaned the previous fold's cold bit on every fold
after the first — measured at the time as 26 of 227 bits, none of them reachable
by walking out from anything on screen. The old behaviour was exactly
`Prev = bits[0].Prev`, and that is the mutation, rather than a plausible-looking
break, because the point is to re-run the failure this was settled on.

Not `sole`: `Prev` reaches the content address, so every pinned compaction hash
moves with it. Those reds are correct and the report lists them.

```seam
id: prev-is-the-previous-folds-prev
file: memory/cool.go
find: \t\tPrev:    prev,
after: \t\tPrev:    bits[0].Prev,
red: TestEveryStoredBitIsReachableFromTheView
```

**What this check does not hold up, found by writing the mutation loosely
first.** A weaker break — `Prev` naming only the window's first bit — leaves
`TestEveryStoredBitIsReachableFromTheView` green, because `Absorbed` names every
original and the walk follows it, so `Prev` is holding up exactly one edge:
the scar beneath this one. Re-derive by changing this block's `after` to
`\t\tif len(prev) == 0 {\n\t\t\tprev = append(prev, b.ID)\n\t\t}` against the
`prev = append(prev, b.ID)` anchor and running the claim; it comes back vacuous.
Left out of the catalog as a standing entry because nothing in this repository
claims it — the finding is about the reach of the check, not about a promise
anybody made.

## A fragment takes its own address

A field reaches a bit's content address through `kind()` alone only if the
value→name map is one-to-one. `Utterance.Truncated` qualifies by arithmetic —
one bool, two names — and the collapse this mutation performs is exactly the
failure that rule exists to prevent: every fragment takes the address of its
complete twin,
and the store keeps whichever landed there first.

```seam
id: fragment-collapses-onto-its-twin
file: memory/bit.go
find: \tif u.Truncated {\n\t\treturn "fragment"\n\t}\n
after:
red: TestAFragmentAddressesApartFromACompleteUtterance, TestAStoreFilesAFragmentApartFromItsCompleteTwin, TestAFoldTalliesFragmentsSeparately
```

## A direction that is not one cannot be addressed

The same mechanism over a type with 2^64 values rather than two. `Vote.kind`
refuses to name a third direction, which is what keeps the address sound: a
default branch returning any tag at all would file `Direction(7)` and
`Direction(8)` under one name. The mutation gives them one.

```seam
id: a-third-direction-gets-a-name
file: memory/vote.go
find: \tpanic(fmt.Sprintf("memory: vote direction %d is neither Up (%d) nor Down (%d)",\n\t\tv.dir, Up, Down))
after: \treturn "vote"
red: TestCastPanics
```

## A run of one is never cooled

The size rule, written in exactly one place — `View.runs` — and consumed by both
`Fold` and `Absorbing`, so the two cannot state it differently. That sharing is
also what makes their agreement test blind to it: break the rule and both move
together. `memory/absorbing_test.go`'s header says so, and says the rule is held
up instead by a column of indices written out by hand from the rule as stated.

That distinction is inside one test function, so it is not expressible at test
granularity here — this claim checks that the rule bites, not which half of that
table caught it.

What it does make executable is the tally the header carries: besides the table,
the mutation "also fails `TestAbsorbingNamesAScarItWouldCoolAgain` below, and four
tests elsewhere in the package that reach the size rule through a fold". Those are
named in `red` below, together with the ones on the surface that read the same rule
through a screen, and `sole` is what holds the sentence to exactly that set — no
count restated here, because the set in the block is the statement and this
sentence would be a second copy of it. The note this corrects said "and nothing
else" and was wrong by four, which is why the tally is worth executing rather than
retelling.

*An earlier version of this paragraph said "the three on the surface" and there
are four. A restated count, wrong, inside the paragraph arguing that restated
counts are the defect — and `sole` holds the set rather than the count, so nothing
in the tool could have caught it. That is the finding: a number in prose beside a
mechanism that does not hold it is unheld however good the mechanism is.*

*The set moved again when a hold learned to cover the bit it answers, and it
moved in both directions: three of the four checks on the surface stopped
reddening under this mutation, because with the pair spared there is no longer a
lone row for a broken size rule to take, and one new check in `memory` started.
Re-derived by running the mutation and reading the FAIL names rather than by
reasoning about them — ten cited names became eight. Both directions matter: a
citation that stops biting is exactly the silent green this catalog exists to
find, and it is only visible because `sole` fails on a check outside the set as
loudly as on one inside it that stayed green.*

*The set gained an eleventh name when the stranding sweep landed
(`tui/testdata/stranding.txt`). It is not a witness in the sense the ten above
are: it reddens because a size rule that cools a run of one changes what two
hundred and seventy simulated conversations fold, which is true of every
mutation in this file that touches the fold. It is cited because `sole` is a set
assertion and an uncited red is a gate failure, not because it says anything
about lone runs.*

*And then two of those three came back, on the same mechanism seen from the
other end. `tui`'s trigger stopped counting covered bits, which made two of its
fixtures stop folding at the vote rates they were written at; both were moved
one rate sparser, and at the sparser rate a lone row exists in that view again.
Eight became ten and they are the same two names as before. **The general form,
which is worth more than the count: a vote rate in a `tui` fixture is a
parameter of this `memory` claim**, because how densely a view is held decides
whether a run of one occurs in it at all. Nothing in either file says so, and
the gate is the only thing that connects them.*

```seam
id: a-lone-bit-is-cooled
file: memory/view.go
find: if !yield(run, len(run) > 1) {
after: if !yield(run, len(run) > 0) {
red: TestAbsorbingIsExactlyWhatTheFoldThenAbsorbs, TestAbsorbingNamesAScarItWouldCoolAgain, TestFoldMergesARunOfSeveralScarsButNotOne, TestDecayKeepsTheViewNearAScreen, TestFoldRefusesAWindowWithNothingHot, TestFoldRefusesWhenThereIsNothingToAbsorb, TestALoneJoggedRowIsAlwaysBesideAScar, TestAHoldSparesOnlyItselfWhenWhatItNamesHasGone, TestNothingIsAbsorbedWithoutFadingFirst, TestHoldingEveryThirdBitBlocksTheFoldAndLettingGoReleasesIt, TestTheStrandingSweepReproducesItsFrozenTable
sole: true
```

## The window is where the fold says it is

The other mutation `memory/absorbing_test.go`'s header names: move the boundary
by one and three rows of that table notice, while the rows whose holds fall
either side of the moved boundary predict the same set anyway. That second half
is why the table carries two rows with no hold in the middle of the window.

The stranding sweep is cited beside it because a window off by one is exactly
what its worst, mean and fold columns are counting, across a hundred and
thirty-eight schedules. No `sole` here, so this is a second witness rather than
an exhaustiveness claim.

```seam
id: fold-window-off-by-one
file: memory/view.go
find: cut := len(bits) - keep
after: cut := len(bits) - keep - 1
red: TestAbsorbingIsExactlyWhatTheFoldThenAbsorbs, TestTheStrandingSweepReproducesItsFrozenTable
```

## A hold covers the row above, which is not the same as the question it answers

A vote lands where a person is reading, nobody votes on the row above, and a
fold that cooled that row left the kept one standing between two scars stripped
of its context — measured at 400 bits through the surface's own trigger, in
93.5% of frames at one upvote in ten. `sparing` is the rule that closed it: a
hold covers what its bit names through `Prev`, one step and no further.

**This paragraph used to say "the question it answers" and that is a claim the
field does not carry.** `Prev` is the head of the view when a bit was written, so
in an alternating session it coincides with the turn being replied to and outside
one it does not: 24% of said bits on this project's own record came from
`tldr say`. Corrected here because a claim's prose is where intent enters, and
read as a specification this one was asserting a relation nothing records.

The mutation is the rule's whole reach — the hold map alone, which is what
`View.runs` read before this. Seven checks redden; `sole` is deliberately absent,
because the surface reads this rule through `View.Absorbing` and a fixture added
to `tui` could join the set without anything about the rule having changed. To
see the current set rather than trust this paragraph: apply the block's own
`after` in a scratch copy and run
`go test ./... -count=1 | grep '^--- FAIL'`.

The seventh is `TestTheStrandingSweepReproducesItsFrozenTable`, and it is the one
citation here that is about the claim rather than incidental to it: the frozen
table's `held` column *is* the measurement quoted above, on both sides of this
mutation, at a vote rate the earlier figure could not reach.

```seam
id: hold-covers-nothing
file: memory/view.go
find: \t\tfor _, id := range b.Prev {\n\t\t\tout[id] = true\n\t\t}
after:
red: TestAFoldKeepsTheBitAHeldBitNamesThroughPrev, TestABitACoverSparesIsNotHeld, TestAnUpvotedBitSurvivesAFoldAndSplitsIt, TestAbsorbingIsExactlyWhatTheFoldThenAbsorbs, TestAFoldWithAVoteAbsorbsDifferentMaterialThanOneWithout, TestTwoViewsOverOneRecordFoldOnDifferentRules, TestTheStrandingSweepReproducesItsFrozenTable
```

## A cover reaches back from a hot bit and never from a scar

The guard on the rule above, and it is the half with the larger downside. A
`Compaction`'s `Prev` is every bit in the window it absorbed (D13), so a cover
that reached back from a cold bit would spare a whole generation for one upvote
on one receipt, and the fold would stop taking anything at all.

The arrangement that shows it is built by hand, because no view produced by
`View.Add` and `View.Fold` holds both a scar and a bit that scar names — the bits
a scar names are the bits it replaced. That makes this guard a prior rather than
a live protection, and the prior is itself executed:
`TestAScarInAViewNeverNamesABitStillInIt` walks 120 bits of folding and fails the
day it stops being true. Measured, and it is why the guard is worth four lines:
removing it changes nothing at all across 42 simulated conversations of 400 bits
each, and turns the hand-built view from a fold into a refusal.

*Re-derived on wider evidence rather than re-asserted: this mutation also leaves
`tui/testdata/stranding.txt` byte-identical across the 270 schedules that table
sweeps, which reach budgets, keeps, hold regimes, vote positions and now
conversation lengths the original 42 did not — including the six at 800 bits,
where the holds do lapse and the folding this guard is about actually resumes.
The prior is now held up by a check that runs in the commit gate and would print
a diff the day it stops holding.*

```seam
id: a-cover-reaches-back-from-a-scar
file: memory/view.go
find: \t\tif !hot(b) {\n\t\t\tcontinue\n\t\t}\n
after:
red: TestAHeldScarSparesOnlyItself
sole: true
```

## Only the human's vote holds a bit back

`Stay.Holds` reads strictly one voter and strictly upvotes. Everyone else's votes
are tallied and cannot move the cut. That is a per-participant budget expressed
as priority rather than as a count: an agent never outvotes the human because it
is never voting in the same tier, and a ceiling would only stop it voting a
million times. The mutation lets anyone's upvote hold.

```seam
id: any-voter-holds
file: memory/view.go
find: if who.voter != stay.By || vote.Payload.(Vote).Dir() != Up {
after: if vote.Payload.(Vote).Dir() != Up {
red: TestOnlyTheHumansVoteMovesTheCut
```

## A tie between two votes goes to the later position in the view

`Tally` settles two votes carrying the same instant by which comes later in the
vote view, which is what a view's order already means. The stated cost is that
`Tally` is a function of the sequence and not of the set. The mutation keeps the
first instead.

```seam
id: tally-tie-keeps-the-earlier
file: memory/vote.go
find: if held, seen := out[who]; seen && b.At.Before(held.At) {
after: if held, seen := out[who]; seen && !b.At.After(held.At) {
red: TestTallyBreaksAnInstantTieByViewOrder
sole: true
```

## Every string in the canonical encoding carries its length

`memory/wire.go` says that without a length in front of every string,
`{Ref: "ab", Display: "c"}` and `{Ref: "a", Display: "bc"}` would hash alike.
`TestIDDependsOnEveryField` carries three rows that exist for exactly that, and
`TestIDIsPinned` fails alongside them because the encoding is the address.

The block's `file` moved from `memory/id.go` to `memory/wire.go` and its anchor
changed from `c.h.Write` to `c.write` when the canonical encoding became the
wire format — one encoding, written into a hash to address a bit and into a
stream to send one.

`red` still names the two checks the claim is *about*, and no `sole`, because
the mutation now reddens a great deal more than that: the wire round-trip
checks read the same damage, since they are round trips through this function.
Which ones was measured rather than reasoned — apply this block's `after` by
hand and run `go test ./memory/ -count=1 | grep '^--- FAIL'`; a first attempt to
write the list out from the reasoning got it wrong in both directions, naming
`TestATruncatedStreamIsRefused`, which does not redden, and missing most of
those that do.

```seam
id: canonical-drops-the-length-prefix
file: memory/wire.go
find: \tc.num(int64(len(s)))\n\tc.write([]byte(s))
after: \tc.write([]byte(s))
red: TestIDDependsOnEveryField, TestIDIsPinned
```

## One record is one file, whoever wrote it

`Store.WriteTo` walks the record in address order because a `Store` is a map and
Go randomizes map iteration — an unsorted walk gives one record a different file
on every write, which is the order fragility `gob` and JSON were refused for
(D39(i)), arriving through the back door. Sorted, two
processes holding the same bits write the same bytes, and a round trip is a
fixed point. The mutation walks the map as it comes.

*The citation above is D39(i) and was wrong three times before it was right.
It was written "D18(i)" first, a label the append-only record still carries in
several files; a review then offered "D38(i)", and D38 has no (i) either. The
third wrong answer was the fix for the first two: this paragraph used to settle
the question with a line offset,
`awk 'NR<=2469 && …' docs/DECISIONS.md`, which is a claim about how long a file
is rather than about what it says. Run against a tree whose `docs/DECISIONS.md`
carries a different subset of the record, that command names whichever entry
happens to sit at that offset — no error, no empty result, a confident wrong
answer, inside the one paragraph whose whole subject is a citation that keeps
being wrong. It does not even settle on one wrong answer: measured against the
public tree twice on 2026-08-15, hours apart while that file was being written,
it said `## D19` and then `## D52`. What settles it is the entry's own text:*

```
awk '/^## D[0-9]+ —/ {h=$0} /A constraint this places on unbuilt work/ {print h}' docs/DECISIONS.md
```

*One line out is the answer. Two lines would mean the anchor sentence has been
quoted somewhere else in the record and this citation needs a narrower one,
which is why it prints every match rather than stopping at the first; no lines
means the tree does not carry the entry, which is the honest answer to give in
a tree that does not. Why the label drifted is **not** established: D39(i)'s
own opening line cites D18(b), and D18(b) simply is the persistence decision,
so either could attract a D18 label and the tree cannot separate them.*

`TestTheStreamFramingIsPinned` is deliberately **not** cited, and this is the
whole reason `sole` is absent. It is a hash over a stream `WriteTo` produced, so
an unsorted walk usually moves it — usually. It was cited here for one edit on
the strength of a single hand-run mutation, and the catalog caught it as
`vacuous` on the very next full run.

**Why it is flaky, measured directly rather than inferred from the end-to-end
rate.** Go does not hand back an arbitrary permutation. Over 20,000 walks of one
map, the number of distinct orders equals the number of *elements* — 5 orders
for 5 bits, 6 for 6, 25 for 25 — and the sorted order is one of them:

| map | distinct orders | sorted hit |
|---|---|---|
| `framed()`, 5 bits | 5 | 2,532 / 20,000 = **12.66%** |
| `framed()` plus one, 6 bits | 6 | 2,470 / 20,000 = **12.35%** |
| `record(t)`, 25 bits | 25 | **0 / 20,000** |

So the flakiness is a property of **how small the map is**, not of this
fixture's particular addresses — adding a bit changes every address and the rate
holds. That is the reverse of the first guess, which was that `framed()` sat
unluckily close to sorted; the probe is what settled it, and reasoning about
which map "happens" to be near sorted is not a thing to do by argument.

The two checks below bite because they run against the 25-bit record, where
sorted was not hit once in twenty thousand, and one of them writes it twenty
times per run.

*Three end-to-end rates were quoted for this before the probe existed: 8 red in
12, 75 in 80, and 84 in 100 — the first from this seat and under-powered enough
to be five standard deviations from the second. The mechanism figure above is
the one with the sample size behind it, and the lesson is that a rate needs the
sample and the command, which the craft record already said and this violated
anyway. Re-check: 20,000 walks of `slices.Collect(maps.Keys(s.bits))` compared
against `slices.Sorted`, roughly fifteen lines in a throwaway test.*

```seam
id: record-written-unsorted
file: memory/wire.go
find: \tids := slices.Sorted(maps.Keys(s.bits))\n\tbits := make
after: \tids := slices.Collect(maps.Keys(s.bits))\n\tbits := make
red: TestARecordSurvivesBeingHandedToAnotherProcess, TestOneRecordIsOneFile
```

## A view refuses to load against a record it was not taken against

A persisted `View` carries the address of the `Store` it was taken against, and
`ReadViewAgainst` refuses a record whose address differs. Without it a view
saved by an earlier session loads clean over a record that has since grown:
every address in it still resolves, so `View.Bits` renders it in silence, and a
reader is shown a conversation missing everything recorded since.
`TestALargerRecordRendersAStaleViewWithoutComplaint` is that null hypothesis run
rather than argued — it is the reason this check is not the redundant one it
looks like, since `View.Bits`' existing panic only ever fires on a view naming
*too much*, and a stale view names too little.

What this does not claim, and the block cannot: that a view is a legitimate
consequence of folding this record. A view that dropped or reordered a bit still
stamps and still loads. See `View.WriteAgainst`.

**This block once required a defect to stay in the code, and that is a third
kind of broken instrument worth naming.** An earlier version of it cited
`TestASingleFlippedBitInAViewIsAlwaysCaught` and explained the citation like
this: *"the seal is computed from the store's real address and the decoded view,
so it never reads the field that was damaged and matches anyway."* Every word of
that is accurate, and it is a description of a bug — `seal` took the `*Store`
and hashed the live answer, so a flipped bit in the stream's own record address
was reported as a view from another session and handed out through `StaleView`
with its integrity never tested. Under `sole: true`, the catalog was
**enforcing** that: fix the ordering and the claim goes `over-red` and the gate
fails.

D27 was an instrument that could not fail. D48 was a check that certified
nothing. This is a check that certified a defect, arrived at honestly, by
writing down what a mutation did instead of what the code should do. The
defence is not more care — it is that a claim's prose has to survive being read
as a specification. *"The seal never reads the field that was damaged"* does not.

`seal` now takes the address as a value and `ReadViewAgainst` tests it before
provenance, so a flipped address byte is an integrity failure and this claim
cites two checks rather than three. The stream is byte-identical either way,
which is why `TestTheStreamFramingIsPinned` never moved.

*The block's anchor has also been stale twice, once silently.* `ReadViewAgainst`
grew a line and the `if now := s.Address(); …` form it cited stopped existing,
so a hand-run mutation matched nothing, reddened nothing, and looked exactly
like a passing control. `cmd/seam` reports it; a hand run does not. Check
`grep -cF` on the anchor before believing a silent mutation.

*`TestAViewFromAnotherRecordIsFatal` joined `red` when `cmd/tldr` started
reading records: the command turns this refusal into a message with a filename
in it, so removing the refusal takes the command's check with it. Same reasoning
as `record-frame-unclosed` below, where the growth is larger and the trade is
argued in full.*

```seam
id: view-loads-against-any-record
file: memory/wire.go
find: \tif now := s.Address(); now != against {
after: \tif now := s.Address(); false && now != against {
red: TestAViewTakenAgainstAnotherRecordIsRefused, TestAStaleViewIsRefusedAndStillRecoverable, TestAViewFromAnotherRecordIsFatal
sole: true
```

## A record whose bit count is one too low comes back one bit short

Content addressing secures a bit's contents and says nothing about the container
around it. The bit count is the one field no address covers, so lowering it by a
single bit used to drop the last bit of the record and return a shorter one with
a nil error — a receipt for a conversation with its end removed, inside the
storage of a product whose whole claim is that this cannot happen. A closing tag
after the last bit is what notices; a leftover-bytes check cannot, because these
streams are self-delimiting on purpose so a record and its views may share a
file, which makes "the count was wrong" and "a view follows" the same bytes.

Found by review, executed rather than argued, and the comment above the sweep
that was supposed to catch it claimed a generalization its own mutation could
not reach: it XORed whole bytes, and against a 25-bit record that can only
*raise* the count, which is the one direction that errors. The sweep now flips
one bit at a time.

*This set went from five to twelve when `cmd/tldr` became the first non-test
caller of the wire format, and the seven that joined are all in that command.
They belong here: a program that reads a record off disk depends on the frame
exactly as the package's own checks do, so the mutation reddening them is the
claim being true of more surface than it was, not the claim leaking. The CEO
kept `sole: true` against a recommendation to drop it, and the cost is named
here rather than discovered later: **a future `cmd/tldr` test that round-trips a
file will trip this claim, and the repair is to add its name to `red`.** That is
a minute of work, and paying it is cheaper than retiring an exhaustiveness
assertion because it became inconvenient — which is the exact shape D50(i)
suspects this catalog of. Trigger for revisiting: if this trips three times on
work with no bearing on the frame, drop `sole` then, and cite those three.*

*Trip one arrived the same day, from continuous saving: `TestTheFileMatchesMemoryAfterEveryChange`
and `TestAChangeThatCouldNotBeWrittenIsCarriedByTheNextOne`, both of which read
a saved file back. **It does not count toward the three.** Both are round-trip
tests and the frame is what they round-trip through, so this is the predicted
case rather than the unrelated one — the trigger is for a trip with no bearing
on the frame, and reading a trip that confirms the prediction as evidence
against it would be the counting error the trigger exists to avoid. The set is
now fourteen.*

*Trip two, from `tldr say` and `tldr top` — the non-interactive verbs (D51(e)).
**It does not count toward the three either, and for the same reason:** every one
of the nine names below opens a record file this program wrote, so the frame is
what they read it through. Nine at once, because a command with no state but the
file has no test that does not touch the file. The set is now twenty-three, and
the cost the CEO accepted when he kept `sole` is now visible at its real size: it
is not "a minute of work per trip", it is a minute of work per trip **plus a
`red` list nobody can read**. Recorded as an argument for revisiting `sole` on
this block, not as a decision to — the trigger the prose already set is about
trips with no bearing on the frame, and this is not one of those.*

*Trip three, three names, from the same unit after review: the save that takes in
what is already on the file (`record-a-save-that-does-not-erase` below) and the
row that names its speaker's ref. Same reason as trips one and two and it does
not count toward the three. The set is now twenty-six. Measured rather than
reasoned — `go build ./... && go test ./... -count=1 | grep '^--- FAIL'` in a
copy with this block's own `after` applied — which is the only way any of these
lists has ever been right.*

*Trip four, one name, from the load that puts a stray utterance back in the
transcript: `TestSayingBesideAnOpenSessionReachesTheNextOneJustTheSame` runs the
`say` verb and then opens the file twice, so the frame is what it reads the
record through. Same reason as trips one to three and it does not count toward
the three. The set is now twenty-seven. Its sibling
`TestALoadPutsAStrayBackWhereItWasSaidAndMovesNothingElse` is **not** here, and
that is the general escape rather than luck: it builds its records in memory and
never touches a file, which is what keeps a unit test of a rule off the wire
format's claims. Measured the same way.*

```seam
id: record-frame-unclosed
file: memory/wire.go
find: \tsc.want(endMark)\n\tif sc.err != nil {
after: \tif sc.err != nil {
red: TestACountReadOneTooLowIsRefused, TestASingleFlippedBitIsAlwaysCaught, TestNoByteOfARealRecordCanBeFlippedUnnoticed, TestATruncatedStreamIsRefused, TestARecordAndItsViewsShareOneStream, TestASavedRecordComesBackWhole, TestAViewFromAnotherRecordIsFatal, TestAViewNamingAMissingBitIsFatal, TestAVoteViewOfSomethingElseIsFatal, TestEveryTruncationIsFatalAndNamesTheFile, TestNoSingleBitOfTheFileCanBeChangedQuietly, TestTheRecordSurvivesASaveThatFailed, TestAChangeThatCouldNotBeWrittenIsCarriedByTheNextOne, TestTheFileMatchesMemoryAfterEveryChange, TestABitSaidFromTheCommandLineFoldsLikeAnyOther, TestAReadingOfAnEmptyRecordSaysSo, TestNoCommandOnThisSurfaceCanCastAVote, TestSayPutsAnOrdinaryBitOnTheRecord, TestTheHeaderSaysHowMuchOfTheOrderAPersonDecided, TestTheMarkOnARowIsTheVoteThePersonCast, TestTheReadingKeepsTheShapeOfWhatWasSaid, TestTheRowLimitPrintsThatManyAndSaysWhenItCut, TestTopReadsTheRecordAndNotTheTranscript, TestARowNamesTheHandleTheRecordKeysOn, TestASaveWillNotReplaceARecordItCannotRead, TestTheWriterThatSavesSecondKeepsTheOthersBits, TestSayingBesideAnOpenSessionReachesTheNextOneJustTheSame
sole: true
```

## A view's own bytes are checked, not just where it came from

A `View` is a list of strings and no content address reaches a list, so the
record's address stamped on a view stream proves provenance and nothing about
integrity. Unsealed, a view whose length was read one too low loads clean and
loses its tail. For a transcript that is a lost row. For a vote view it is a
lifted hold: `memory/view.go`'s `Stay.Votes` says its order and contents decide
what a fold keeps, which is exactly why it is persisted rather than rebuilt, so
a silently shortened one lets the next fold take material somebody voted to
keep.

*`TestNoSingleBitOfTheFileCanBeChangedQuietly` joined `red` with `cmd/tldr`: it
sweeps every bit of a whole saved file — record and both views in one stream —
so the seal is one of the things standing between it and a quiet corruption.*

```seam
id: view-seal-unchecked
file: memory/wire.go
find: \tif want := seal(against, v); sealed != want {
after: \tif want := seal(against, v); false && sealed != want {
red: TestASingleFlippedBitInAViewIsAlwaysCaught, TestAViewLengthReadOneTooLowIsRefused, TestNoSingleBitOfTheFileCanBeChangedQuietly
sole: true
```

## A compaction's count has to agree with its receipt before a caller sizes work from it

`Cool` calls these invariants rather than input validation and enforces them
where a compaction is made. The decoder re-checks them, which sounds like the
"a rule added later must not orphan an existing record" mistake and is not:
every populated `Compaction` in existence came through `Cool` and satisfies them
already, and the only one that did not is the bare literal, which satisfies them
trivially. What it stops is a number arriving from another process. Executed
before the check existed: a hand-written record advertising `Count()` of a
trillion was admitted, and `tui.recall` sized a slice from it and died with
`fatal error: out of memory` — no panic, no defers, no receipt. `recall` no
longer preallocates from `Count` either; both fixes are wanted, because each
holds whatever the other does.

```seam
id: compaction-count-unchecked-on-read
file: memory/wire.go
find: \tif len(p.absorbed) != p.count {
after: \tif false && len(p.absorbed) != p.count {
red: TestACompactionThatDisagreesWithItsCountIsRefused
sole: true
```

## The stream's frame is pinned, and nothing else pins it

A bit's bytes are pinned transitively through four golden addresses — change the
encoding and `TestIDIsPinned` and its three siblings all move. The bytes
*around* the bits are pinned by nothing: the magic, the marks, the version, the
closing tag, the order of the header fields and the choice to write a bit's
address before the bit. Every one of those can be edited without moving a single
address, every one makes every file already written permanently unreadable, and
the two tests that do header arithmetic would be edited in the same hand as such
a change, which is what disqualifies them as goldens.

The mutation is the sharpest available demonstration: it renames the closing tag
in the one place the constant is declared, so writer and reader move together
and **every round-trip check in the package still passes**. One test notices.

```seam
id: stream-framing-unpinned
file: memory/wire.go
find: \tendMark   = "end"
after: \tendMark   = "fin"
red: TestTheStreamFramingIsPinned
sole: true
```

## The mark floors are where they were measured

Below nine columns in the transcript and thirteen in the receipt, a fragment's
row and a screen-cut row are the same bytes. That is conceded rather than fixed,
and the four numbers live in exactly one place — `tui/tui_test.go`'s
`TestTheRowsMarkFloorsAreWhereTheyWereMeasured` — after the doc comment that used
to restate them went stale by two generations. Shortening the ladder of marks
moves a floor, which is the whole thing that test is for.

```seam
id: floors-lose-the-narrow-mark
file: tui/unfold.go
find: \t"╌ unfinished ╌",\n\t"╌ unfinished",
after: \t"╌ unfinished ╌",
red: TestTheRowsMarkFloorsAreWhereTheyWereMeasured
```

## No vote reaches the persona

`tui/ask.go` says it takes one line to put the votes in and that the line must
not be written: a vote shown to the thing being judged stops being a
consolidation signal and becomes a behavioural one. The mutation is that line.

`occ` is spelled out because that anchor occurs twice in the file — in `turns()`
and again in `endsUnfinished()` — and the tool refuses an anchor that matches more
than once unless the block says which. Riding the default silently was safe today
and would become a false accusation against a healthy check the first time a
refactor moved those two functions past each other.

```seam
id: votes-reach-the-persona
file: tui/ask.go
occ: 1
find: \tbits := m.shown.Bits(m.store)
after: \tbits := append(m.shown.Bits(m.store), m.votes.Bits(m.store)...)
red: TestNoVoteReachesThePersona
sole: true
```

## Nor does the standing instruction mention that one exists

The claim above guards the back door. This one guards the front: the persona's
system prompt is sent ahead of every request and does not change when anybody
votes, so byte equality across a vote cannot see it. A sentence in there saying
that the person keeps some answers and lets others go produces the whole
sycophancy effect without a single verdict crossing — a model does not need the
score to write toward the score, it needs only to know one is being kept.

The mutation is that sentence, written the way somebody explaining the product to
its own participant would actually write it. The check reads the constructed
persona's `System` rather than the constant, so a `defaultPersona()` that stopped
using it does not go quietly unchecked.

```seam
id: the-standing-instruction-explains-the-vote
file: tui/ask.go
find: Each message is drawn as one row on that screen
after: They can upvote a message to hold it out of the next fold, so some of what you say will outlast the rest. Each message is drawn as one row on that screen
red: TestNoVoteReachesThePersona
sole: true
```

## A fold note is an index of what was folded, never a sample of it

The word list a fold hands the persona is the one place folded material crosses
into a live prompt, and everything `tui/ask.go` says about it rests on its being a
count rather than a quotation: it points at what was discussed and cannot say what
was said about it. That is prose, and prose is not checkable. The arithmetic under
it is: every word the note carries out of the folded messages is a word `topWords`
chose, and every word it did not choose is absent.

The mutation is the honest-looking one — the note widening its own slice, which is
what somebody tuning a prompt would write — rather than a quotation nobody would.

Nothing else in the tree watches that width, and the reason is worth stating
because it changed. The note used to be one of a pair: `scarWords` drew the scar's
label from the same bag, `personaWords` drew this one, and an invariant held the
human's account no smaller than the model's. `scarWords` is deleted — the scar
quotes a bit somebody actually said now — so the pair is gone and this claim is
the only thing standing over the note's width. What remains beside it is
`TestTheWordIndexIsAPrefixOfItselfAtEveryLength`, and that is a property of
`topWords` alone: it asserts a shorter index is a prefix of a longer one, never
reads `foldNote`, and stays green through this mutation by construction. Measured
— the whole suite was run under this mutation and exactly one check reddened.

The fixture is a vocabulary of invented words that exist nowhere else in the
program, which is what makes the check possible at all: any of them found in the
note came out of the folded messages, because there is no other way in. English
fixture text cannot do it — "record", "messages" and "answer" are in the note's own
sentences.

```seam
id: fold-note-widens-past-the-index
file: tui/ask.go
find: topWords(c.Bag(), personaWords)
after: topWords(c.Bag(), personaWords*3)
red: TestTheFoldNoteCarriesTheIndexAndNoneOfTheContent
sole: true
```

## Cool reads no clock

Folding the same window twice has to produce the same object, or the store fills
with near-duplicate summaries that differ only in when somebody got around to
making them. So a cold bit's instant is the end of the span it stands for, and
`Cool` takes no clock reading of its own — which is also what makes the whole
fold path pure — which is the property a seeded simulator would be built on, if
one is ever built.

Seven checks notice, which is the useful part of the claim rather than a detail:
one about `Cool` itself, one about folding the same window twice, one about a view
staying in the order things happened, one about eight goroutines agreeing with
a single sequential run, — since persistence — one about the byte-for-byte
shape of a written stream, because a fold whose instant moves writes a different
file, and — since the cover — two about which bits a hold spares. Purity is not a
property of the function alone, and this is the set that says so.

The fifth arrived when `TestTheStreamFramingIsPinned` was written and turned this
claim `over-red` in the gate, exactly as
`.claude/craft/principal-go-engineer.md` says a new caller of an existing
guard will. Deterministic, measured 5 of 5: apply this block's `after` and run
`go test ./memory/ -run TestTheStreamFramingIsPinned -count=1` five times.

**The sixth and seventh arrived the same way and are the second instance of that
prediction, which is what makes it a pattern rather than an anecdote.**
`TestAHeldScarSparesOnlyItself` and `TestAHoldSparesOnlyItselfWhenWhatItNamesHasGone`
came with `sparing` (`memory/view.go`), and both compare a fold against bits they
built themselves, so a `Cool` that stamped its own clock makes their expected
addresses wrong. The builder hand-ran the affected mutations across the tree and
predicted a different claim's list moving; it did not predict this one, and the
gate is again how we found out. That is the standing cost of `sole: true` — a
test added in one part of a package restates a claim three sections away — and it
is recorded here rather than argued about, because the alternative on offer
computes a mutant's status per *mutant* and not per *test*, which is not the same
fact.

**The ninth arrived the same way as the sixth and seventh, one session later,
and that is now three instances of the same prediction.**
`TestSparingAnswersAboutTheViewItIsAsked` came with `View.Sparing`, folds a view
and compares the result against bits it built itself, so a `Cool` that stamps its
own clock makes its expected addresses wrong. Nobody predicted it; the gate did.
The pattern is stable enough to state as a rule: **any new check in `memory/`
that folds and then names an address is a citation on this claim**, and the
cheapest way to find out is to run this one block after adding one.

**The eighth was invisible for exactly one day, and the day it was invisible is
the day this claim was last declared clean.** `TestTheClockReadsTheSameWhoeverOpensTheRecord`
(`tui`) folds and opens a receipt, then requires the header to carry its
fixture's own date, `2026-08-14`. Under this mutation the scar is stamped
`time.Now()` and becomes the newest instant in the view, so the header draws
*today* — which on 2026-08-14 was the same string, and on every day since is
not. So the check does not merely fail to bite on one day: **whether this claim
is `proven` or `over-red` was a function of the wall clock**, and it flipped at
midnight with nothing committed. Recorded rather than smoothed over, because
D58(q) and the commit receipt from that day both say this claim ran clean, and
both were true when they were written.

**The tenth and eleventh are the stranding sweep, and they are the strongest
statement yet of the rule two paragraphs up.** A `Cool` that stamps `time.Now()`
makes the newest instant in the view the moment the fold ran, so every hold in
the sweep decays against a clock instead of against the conversation — the frozen
table moves, and `TestTheStrandingSweepCanReportEitherAnswer` goes red as well,
because at least one schedule stops stranding anything a hold is holding. Two
names for one mutation and they are not a second witness of the same fact: one
says the numbers moved, the other says the sweep stopped being able to measure.

Kept as a citation rather than repaired in the fixture, and the direction of
that choice is the point: making the fixture's date relative to now would make
the check *stop* noticing a `Cool` that reads a clock, which is weakening a
guard to satisfy a catalog. A fixed date in the past reddens stably from here
on. The residual, stated because nothing holds it: a date literal in a fixture
is a fact about when the fixture was written, and any *other* check comparing a
mutant's clock against one can have the same one-day hole.

```seam
id: cool-reads-the-clock
file: memory/cool.go
find: \t\tAt:      c.to,
after: \t\tAt:      time.Now(),
red: TestCoolIsDeterministic, TestFoldOfTheSameWindowCollapses, TestASplitFoldLeavesTheViewInTheOrderThingsHappened, TestConcurrentFoldsAgreeWithOneSequentialRun, TestTheStreamFramingIsPinned, TestAHeldScarSparesOnlyItself, TestAHoldSparesOnlyItselfWhenWhatItNamesHasGone, TestTheClockReadsTheSameWhoeverOpensTheRecord, TestSparingAnswersAboutTheViewItIsAsked, TestTheStrandingSweepReproducesItsFrozenTable, TestTheStrandingSweepCanReportEitherAnswer
sole: true
```

## A folded vote view is refused rather than reported as no votes

`Cool` will fold a window of votes without complaining, and `memory/cool.go` says
plainly not to: the votes leave the vote view, `Tally` over that view then reports
nothing, and every stay it was holding lifts at once. Nothing about that looks
like an error from the outside — the folds simply start taking material the human
had held back. `Tally` panics instead, which is the enforcement behind the limit.

**This claim came back `vacuous` the first time it was run, and that is why it is
worth reading.** With the guard removed, `TestTallyPanicsOnAViewThatIsNotVotes`
went on passing: it asserted that *something* panicked and nothing more, and with
the guard gone `Tally`'s own type assertion panics a few lines later as an
interface conversion. `recover() != nil` cannot tell those apart.

What was at stake is not the refusal but the explanation, and in this program the
explanation is what a guard is for. The guard names the offending bit by address
and says what it carries instead; the runtime's version names a line the caller
did not know it was on, about a bit it cannot identify, in a record of several
hundred.

The check now asserts the message says both. Found by this tool, reported rather
than repaired by the hand that found it, and repaired on a ruling — the sequence
matters, because a builder who quietly strengthens his own vacuous check has
removed the only evidence that it was ever weak.

**A second check joined this claim when `View.Rank` landed, and the gate is how
we found out.** The guard lives in `standing`, which `Tally`, `Stay.Holds` and
now `Rank` all read, so a second entry point is a second way to reach it —
`TestRankPanicsOnAViewThatIsNotVotes` asserts the same message through `Rank`,
and this mutation reddens it too. Run before that check was cited here, the
claim came back `over-red` with `also red: TestRankPanicsOnAViewThatIsNotVotes`
rather than passing quietly, which is the whole of what `sole` is for: a claim
that a mutation is noticed by exactly these checks goes stale the moment
somebody adds a caller, and nothing else in the repository would have said so.

```seam
id: tally-accepts-a-view-that-is-not-votes
file: memory/vote.go
find: \t\tif _, ok := b.Payload.(Vote); !ok {
after: \t\tif _, ok := b.Payload.(Vote); !ok && false {
red: TestTallyPanicsOnAViewThatIsNotVotes, TestRankPanicsOnAViewThatIsNotVotes
sole: true
```

## An agent's votes cannot cross the participant's own

`View.Rank` is D3's first surface, and the only thing that makes it a ranking
rather than a popularity count is that it has two tiers. The participant the
caller names decides first — upvoted above untouched above downvoted, whatever
anybody else said — and everyone else's votes arrange only what that participant
left level. `memory/vote.go`'s doc comment on `Score` states the failure this
avoids in as many words: merging voters into one number "is exactly how an agent
outvotes a human", and the per-voter map is "a priority order, which is what
D18(d)'s per-participant budget looks like when it is expressed as rank instead
of as a count".

So the mutation is that merge, written as the one line somebody refactoring for
brevity would actually write: add the two numbers and sort on the total.

`TestNoCrowdOfVotersCrossesTheParticipantsOwnVote` sweeps 1 to 50 agents rather
than fixing one crowd, and the sweep is load-bearing rather than thorough: under
the merged score the tier is crossed by arithmetic, so it holds at one agent and
fails from two on. Measured — the one-agent row passes green under this exact
mutation. A single fixture picks one count and can sit on the safe side of that
boundary by luck.

```seam
id: rank-merges-the-tiers
file: memory/rank.go
find: \t\tif tier := cmp.Compare(b.Own, a.Own); tier != 0 {\n\t\t\treturn tier\n\t\t}\n\t\treturn cmp.Compare(b.Others, a.Others)
after: \t\treturn cmp.Compare(b.Own+b.Others, a.Own+a.Others)
red: TestNoCrowdOfVotersCrossesTheParticipantsOwnVote, TestRankOrdersAViewByItsVotes
sole: true
```

## A ranked reading leaves everything nobody voted on where it was

`View.Rank`'s tie-break is the view's own order, which is what makes the whole
ordering checkable by eye: with no votes at all it hands back exactly the
transcript, so the only rows that moved are the ones somebody voted on. The
alternative this refuses is named in the doc comment and is the one `Tally`
already refused for its own tie — settling "which of these did the room stand
behind" with "which hash sorts higher" is not an answer anyone can give the voter
it demoted.

The mutation is therefore that rejected design rather than a plausible-looking
break: a final comparison on the content address, reached only where the votes
have said everything they have to say. Nine of the eleven rows in
`TestRankOrdersAViewByItsVotes` notice, including the row with no votes in it at
all.

```seam
id: rank-ties-by-address
file: memory/rank.go
find: \t\treturn cmp.Compare(b.Others, a.Others)
after: \t\tif rest := cmp.Compare(b.Others, a.Others); rest != 0 {\n\t\t\treturn rest\n\t\t}\n\t\treturn cmp.Compare(a.ID, b.ID)
red: TestRankOrdersAViewByItsVotes
sole: true
```

## A refused direction says which direction it was

The claim above shows that the refusal bites. This one asks the narrower question
the repaired tally guard made worth asking everywhere: does anything hold up what
the refusal *says*? The panic names the direction it was given and both directions
it was not, which is what tells a reader whether they are looking at an
uninitialised `Vote{}` or a number somebody computed. The mutation keeps the panic
and empties the message.

**This block was headed "A refused direction says which direction it was —
VACUOUS, open" for one session, and the history is the evidence.** When it was
first run, `TestCastPanics` asserted `recover() != nil` and nothing further —
the same shape as the tally guard's check before it was strengthened. It was
reported rather than repaired by the hand that found it, deliberately: the
obvious fix was the one already applied one file away, and applying it in the
same breath as discovering it is the loop this catalog exists to break, because
the builder who fixes his own vacuous check removes the only evidence it was
ever weak (D45(e)). The heading is recorded here because `docs/DECISIONS.md`
quotes it and is append-only; a reader following that citation lands nowhere
otherwise.

**Closed one session later — and the gate did not fire, which is the finding.**
`TestCastPanics` now names, per row, the substrings its panic has to contain, so
this claim's mutation reddens it by its own assertion. The block above says a
claim returning `proven` against a `vacuous` declaration fails the gate and
sends somebody back here to say so. **That did not happen.** The repair and the
deletion of `verdict: vacuous` were made by one hand three seconds apart, on a
CEO instruction that ordered both in the same dispatch, and the tool first ran
ten seconds after that — reporting `proven`, `as declared`, exit 0. It never saw
the disagreement, because nothing runs it between the repair and the
re-declaration.

So the gate is a tripwire only if something trips it, and nothing does: closing a
declared finding quietly requires no evasion, just two edits and no run in
between. The one time this claim ever failed the gate was four seconds after this
block was first authored — inside the session that built it, by the hand that
built it. **What is not silent here is that a person wrote this paragraph**, and
the block should not credit a mechanism for that. The declaration's real,
narrower guarantee is the one the evidence supports: it survived a session
boundary, and a later run confirmed the claim was still where it said it was.

What the repair does **not** hold is stated where it is missing rather than here:
the two rows covering `Cast`'s own unaddressed-target guard assert the fault and
not the identity, because that guard prints only the *target's* `Handle.Ref` and
both rows therefore panic with byte-identical text. The check is honest about it,
the message is what is weak, and nothing in this catalog watches that guard yet.

```seam
id: a-refused-direction-names-itself
file: memory/vote.go
find: \tpanic(fmt.Sprintf("memory: vote direction %d is neither Up (%d) nor Down (%d)",\n\t\tv.dir, Up, Down))
after: \tpanic("memory: bad vote")
red: TestCastPanics
```

## The ranked view is over the record, so a fold does not move it

`tui/tui.go`'s package doc says the second surface "ranks over the *record* and
not over the view, which is the difference between retrieval and theatre", and
that the consequence is the one thing this screen demonstrates: **the list does
not move when a fold fires.** `Model.judged` walks the *store* and takes every
utterance, plus anything else somebody voted on, whether or not the transcript
still shows it — so a fold changes what is on the transcript and changes nothing
here.

It walked the vote view instead until this checkpoint, taking each vote's target
alone. That was over the record too and the claim held, but the filter hid the
material this screen exists to reach: on the live record it drew three rows out
of twenty-nine said bits, with the correction to the top-ranked claim absent
entirely. Widening it does not move this claim; it moves what the mutation has to
be, because the line that used to hold the property is a different line.

The mutation is the other design, and it is the one somebody would write while
tidying: keep only the bits the transcript is still showing. It is not a
strawman — it is what "rank the view" means, it compiles, it looks right on a
short conversation, and it is wrong in a way that only appears after a fold.

Two checks notice and they say different things.
`TestTheRankedListDoesNotMoveWhenAFoldFires` presses ctrl+t, lets the
conversation carry on until a fold takes rows off the transcript, and asserts
that every address on the list before is still on it after, in the same order and
the same band; under the mutation they are gone, which is what a person would
have watched happen. (It compared the drawn rows byte for byte until the list
widened; a widened list also gains the rows the triggering conversation wrote, so
the ordinals move and byte equality stops being the true statement of the same
property.) `TestTheRankedListIsEverythingSaidPlusAnythingElseJudged` says it
about the set rather than the frame, and fails immediately rather than only after
a fold — worth having both, because a mutation that emptied the list *without* a
fold would slip past a check that only ever looks after one.

`sole` is deliberately absent. The mutation drops rows from a list several other
checks read, and which of them notice is a fact about the fixtures rather than
about this claim; asserting the whole set here would make every future ranked
test a maintenance event on this block.

```seam
id: rank-the-view-instead-of-the-record
file: tui/ranked.go
find: \t\tsaid[b.ID] = true
after: \t\tsaid[b.ID] = true\n\t\tif !slices.Contains(m.shown, b.ID) {\n\t\t\tcontinue\n\t\t}
red: TestTheRankedListDoesNotMoveWhenAFoldFires, TestTheRankedListIsEverythingSaidPlusAnythingElseJudged
```

## Two trees that hold different code do not share an address

Every verdict this tool prints is now printed under the address of the tree it
was taken against, because a verdict with no tree named is a sentence with no
subject — `22 proven` was true of some tree and there was no way left to find
out which. That binding is worth exactly what the address is worth. An address
that did not cover the bytes of the files would give a repaired tree and a
broken one the same name, and a receipt that agrees with everything is worse
than no receipt: it reads as a check having been made.

The mutation is the file case's content digest, dropped. Paths, the execute bit
and symlink targets stay in, so the tree is still addressed — just not by what
is in it.

This is the first claim in the catalog whose `file` is the tool's own, and the
circularity is apparent rather than real. `cmd/seam` builds itself once from the
unmutated tree when `go run` starts, and that binary computes the address this
run reports; the mutation is compiled only into the *copy's* test binary, which
is what `TestWhatTheTreesAddressDistinguishes` runs under. The judge and the
judged are two builds.

`sole` is absent and the reason is worth stating rather than leaving to a
reader's guess: nothing else in the suite reads a tree address today, so
soleness would be true and would say nothing, and the first check that does read
one would turn a true claim into a maintenance event on this block.

```seam
id: tree-address-drops-contents
file: cmd/seam/digest.go
find: \t\t\ta.num(b2i(e.exec))\n\t\t\ta.str(e.detail)
after: \t\t\ta.num(b2i(e.exec))
red: TestWhatTheTreesAddressDistinguishes
```

## Nothing outside the surface can cast a vote

`tldr say` and `tldr top` let a participant who is not at a keyboard write to the
record and read it back, and neither of them can vote. That asymmetry is the
whole of what makes the ranking worth anything: a vote is a human's cheapest act
and the only signal this product has (D4, D30), so an agent that can cast one can
manufacture the signal it will later be measured by. D51(d) names it as the way
the dogfood strategy fails quietly — a launch pitched on "ranked by a human's
votes", shipping a record full of votes no human cast, looks from outside exactly
like one that worked. D52(j) ruled the constraint before either command existed:
the skill gets Claude a write, never a vote.

An absence is the hard thing to hold, because there is no line of code to break.
The mutation therefore adds the feature rather than removing one — `say`, having
recorded what it was told, also upvotes it — which is the shape the failure would
actually take: not a `vote` verb somebody argues about in review, but a write path
that quietly casts one on the way past.

`TestNoCommandOnThisSurfaceCanCastAVote` walks the command table rather than
naming the two verbs, so it also covers a third the day it is added; measured
separately, adding a `vote` command to that table reddens it whether or not the
test's own coverage list is updated, because an uncovered command fails the
coverage check outright.

The second cited check is not a second witness and is named so it does not read
as one. `TestTheWriterThatSavesSecondKeepsTheOthersBits` asserts an exact bit
count over two writers, so a write path that quietly files an extra bit reddens
it whatever that bit is. It is here because `sole` is an exhaustiveness claim
and this is what exhaustive currently means, not because it knows anything about
votes.

```seam
id: a-write-path-that-also-votes
file: cmd/tldr/say.go
find: \tif err := rec.save(path); err != nil {
after: \trec.votes, _ = rec.votes.Add(rec.store, memory.Cast(time.Now(), b.From, memory.Up, b))\n\tif err := rec.save(path); err != nil {
red: TestNoCommandOnThisSurfaceCanCastAVote, TestTheWriterThatSavesSecondKeepsTheOthersBits
sole: true
```

## The one handle this program will not write under is the human's own

`tldr say` will spell nearly any handle it is given and refuses exactly one: the
ref `tui.Human()` returns, which is the handle the surface writes for the person
at the keyboard. That is the claim above one step removed rather than a second
rule. This program already says a machine may produce material and may not
produce the human's judgment; an utterance minted under the human's ref produces
that judgment by another door, because it is the ref `View.Rank` takes as `by` —
so it decides the order of every ranked reading of this record — and it is the
handle an audit of the record would be *about*. A record where any local process
can speak as the person it is evidence for is not a provable record, whatever
else it is.

The seam is new rather than pre-existing, which is why it is closed here and was
not before. Until these verbs existed the only writers were the surface's own
handle and the persona's, and neither was choosable; `say` is what made a handle
a flag.

**The scope, stated because overclaiming this would be worse than not having
it.** The refusal is an equality, so a `-as local2` or a borrowed display name is
recorded like anything else and a reader still has to read — the residue is
handled on the reading side, by printing the ref rather than judging it. A
similarity test would be a guess about intent, and the refs this program most
wants are the ones such a guess would refuse. It is also not authentication and
cannot become it: anyone who can run this command can write the record file
directly. What is settled is what *this program* will do on its own behalf, which
is exactly the standing the absent vote verb has and is worth exactly as much.
`TestSayPutsAnOrdinaryBitOnTheRecord`'s near-miss row is that scope written where
somebody adding a similarity check would have to delete it deliberately.

The mutation removes the check and leaves the message, so what is measured is the
refusal rather than a compile error. Two cited checks and they hold two different
things. `TestSayWillNotSpeakAsThePersonAtTheKeyboard` requires the refusal to
arrive before anything is read — no record opened, the reader undrained — and to
name a ref a person could use instead, because a message that only says no leaves
somebody guessing at a rule nobody wrote down. `TestSayRefusesAndLeavesTheRecordAlone`
puts this beside every other way the verb refuses and holds the one thing true of
all of them: the file did not move. Measured against the whole suite: those two
and nothing else.

```seam
id: say-speaking-as-the-human
file: cmd/tldr/say.go
find: \tif me := tui.Human(); *as == me.Ref {
after: \tif me := tui.Human(); false && *as == me.Ref {
red: TestSayWillNotSpeakAsThePersonAtTheKeyboard, TestSayRefusesAndLeavesTheRecordAlone
sole: true
```

## Nothing draws under the human's display name without its ref beside it

`tldr top` is the only reading of this record with no screen, and its speaker
column is where the refusal above stops being absolute: a near-miss ref is
recorded, so the column has to be readable enough that a person notices. It
prints `display (ref)` when the two differ and the one name when they agree —
except that a handle displaying under the name the person at the keyboard uses
always shows its ref.

The exception is not decoration. `say -as me` drew a bare `me` while the human's
own row drew `me (local)`: *less* said about the row that had never been near a
keyboard, so it read as the more human of the two. Printing the ref on every row
instead would be the simpler rule and is worse — `tldr say -as session-15` with
no `-name` records the ref as the display, so the ordinary row would become
`session-15 (session-15)`, the same word twice on nearly every line, to
disambiguate a case that mostly does not arise.

The mutation is the simpler rule in the other direction: drop the exception and
keep the rest. `me (me)` is odd to read and is what it should be, and a later
tidy-up that mistakes it for a bug is exactly what this claim is here to stop.

```seam
id: speaker-drops-the-humans-name
file: cmd/tldr/top.go
find: \tif h.Display == "" || (h.Display == h.Ref && h.Display != tui.Human().Display) {
after: \tif h.Display == "" || h.Display == h.Ref {
red: TestARowNamesTheHandleTheRecordKeysOn
sole: true
```

## A walk of the whole record is in an order two processes agree on

`Store.All` is how anything outside `memory` reads a record it did not write —
`tldr top` ranks over it, and what is in a record that no view names is exactly
what an auditor most needs to find. A `Store` is a map, and Go randomizes map
iteration, so an unsorted walk hands back a different sequence every call: two
readings of one record disagree, and a caller sorting by something else inherits
a different tiebreak each time. It is the fragility `memory/id.go` refuses gob and
JSON for, arriving through the reader instead of the writer.

The fixture is twenty-five bits and the number is the claim's own load-bearing
part. Go hands back a *rotation* of insertion order, so the distinct orders equal
the element count: at five bits the sorted order comes up about one time in eight
and this check would pass over a real defect that often. At twenty-five it did not
come up once in twenty thousand walks. Measured 10 of 10 red under this mutation.

```seam
id: record-walked-unsorted
file: memory/store.go
find: \tids := slices.Sorted(maps.Keys(s.bits))\n\tbits := make([]Bit, 0, len(ids))
after: \tids := slices.Collect(maps.Keys(s.bits))\n\tbits := make([]Bit, 0, len(ids))
red: TestAllIsInAddressOrderAndSaysTheSameThingTwice
sole: true
```

## A save takes in what is already on the file before replacing it

`cmd/tldr` has two writers over one path: a session saving the whole record after
every change, and `tldr say` writing a bit from outside one. A save is the whole
file, so whichever finishes second used to be the only record that survived — not
a row missing from a screen, a bit **absent from the store**, with no error and no
receipt. That is D1 failing inside the binary whose thesis it is, and it was
disclosed in a doc comment as an accepted lost update before review pointed out
that the argument for accepting it was a false choice.

There is nothing to reconcile. The store is content-addressed and only grows, so
two writers cannot produce contents that disagree: the union is `Store.Put` in a
loop, identical bits collapse by construction, and both views are sealed against
the merged store because `WriteAgainst` reads the store's address at write time.
No lock, no second format.

Two checks, because the mutation removes two guarantees at once and both are the
claim. `TestTheWriterThatSavesSecondKeepsTheOthersBits` runs the real `say` verb
and a second session against one file in both orders and requires every bit to be
on the record afterwards — and requires the other writer's bit **not** to be in
the second writer's *running* view, which is the half that keeps the fix from
being "merge the views too". `TestASaveWillNotReplaceARecordItCannotRead` holds
the other consequence of reading before writing: a file this build cannot parse
stops the save instead of being overwritten by it.

*That first check used to make the "not in the view" assertion against the record
**reloaded from the file** rather than against the view the second writer was
still holding, and this block described it that way. Read as a specification, it
said the other writer's bit must be in no transcript at all, ever — which is the
defect `record.rejoin` was built to fix, written down here as a requirement
(D52(c)'s shape, third instance). The assertion moved to the live view, where the
sentence above always pointed, and the reload now carries the opposite assertion
beside it.*

*A third name joined with that repair, and it is not a third witness: without the
merge, `TestSayingBesideAnOpenSessionReachesTheNextOneJustTheSame` never gets past
"is it on the record at all", which is the same failure the first check reports in
its own words. It is here because `sole: true` is a set assertion and it is in the
set, not because it holds anything up that was not already held.*

What neither buys, and the code says so where it happens: this is not a lock. The
window between the read and the rename is milliseconds rather than a session, and
two saves genuinely in flight can still lose one. One process cannot produce that
interleaving, so nothing here tests it.

```seam
id: record-a-save-that-does-not-erase
file: cmd/tldr/record.go
find: \tif err := r.absorb(path); err != nil {\n\t\treturn fmt.Errorf("reading %s before replacing it: %w", path, err)\n\t}\n\treturn atomically(path, r.encode)
after: \treturn atomically(path, r.encode)
red: TestTheWriterThatSavesSecondKeepsTheOthersBits, TestASaveWillNotReplaceARecordItCannotRead, TestSayingBesideAnOpenSessionReachesTheNextOneJustTheSame
sole: true
```

## The transcript accounts for every utterance the record holds

`tldr say` writes a bit and puts it in the transcript on the file. A session that
is open holds its own transcript for the life of the process and writes it over
that file at the next change, so the bit ended up in the store and in **no view**
— not until the next session, permanently. With nothing else running the identical
command put it on the next session's first screen, in the fold window and in what
the persona is told. One command, two outcomes, selected by whether a terminal
happened to be open somewhere else on the machine.

It was never D1 or D14 failing: `tldr top` and the ranked surface both enumerate
`Store.All` rather than a view, so the bit stayed reachable throughout. What was
wrong is that the *transcript* disagreed with itself about the same act, and said
so nowhere.

`record.rejoin` is the repair, at load rather than on the save path — a save may
not write a view the surface does not hold, or `tui.Save`'s whole sentence stops
being true. The mutation makes it a no-op, which is the state the program shipped
in. Note which row of the cited check reddens: *with a session open* fails and
*with nothing else running* passes, so the two-outcome shape is what the check is
measuring rather than something about `say` in general.

`TestALoadPutsAStrayBackWhereItWasSaidAndMovesNothingElse` is the rule's own
table, hand-built rather than driven through a file, and its "a fold is not
undone" row is the one worth naming: `rejoin` asks whether a scar in the view
absorbed a bit, and the obvious simplification is to ask only whether the view
names it. Under that version every folded bit in every record is a stray and
opening the program un-folds the conversation. Measured — dropping the `Absorbed`
walk reddens that row plus `TestASavedRecordComesBackWhole`,
`TestSayPutsAnOrdinaryBitOnTheRecord` and `TestTheFileMatchesMemoryAfterEveryChange`,
which is why no `sole` is declared here: this mutation's blast radius is the whole
transcript.

```seam
id: a-transcript-that-drops-a-stray
file: cmd/tldr/record.go
find: func (r record) rejoin() record {\n\theld :=
after: func (r record) rejoin() record {\n\tif true {\n\t\treturn r\n\t}\n\theld :=
red: TestSayingBesideAnOpenSessionReachesTheNextOneJustTheSame, TestALoadPutsAStrayBackWhereItWasSaidAndMovesNothingElse, TestTheWriterThatSavesSecondKeepsTheOthersBits
```

## A recorded message cannot draw a role boundary

The record keeps what was said, control tokens and all — that is D1, and
`persona.DefaultModel`'s doc already ruled that cutting text out of a bit because
of a few particular characters is the worse defect. What goes on the wire is
derived. `persona.Escape` breaks any control marker on the way out, and this
claim is that it is actually *on* the outbound path rather than merely present in
the package.

The defect is real and was found by the shareholder using the program: a qwen3.5
reply ended `?<|endoftext|><|im_start|>user\n<|system_message|>` and went onto the
live record as ordinary content, correctly. Measured against ollama 0.17.7 by
`prompt_eval_count` — a genuine five-message conversation and a three-message one
whose middle assistant turn spells the other two out inside its content produce
the *same* prompt token count and the same reply at temperature 0. A forged turn
is not similar to a real one; it is the same bytes.

The check asserts on what the server received rather than on `Escape`'s return
value, which is the whole point: the function working and the function being
called are different facts.

```seam
id: boundary-sent-unescaped
file: persona/client.go
find: \t\tsafe, _ := Escape(m.Content)
after: \t\tsafe := m.Content
red: TestTheWireCarriesNoBoundaryTheRecordHeld
sole: true
```

## Each model family spells its own boundary, so the rule is not one family's

Measured the same way: `<|eot_id|>` is one token to `llama3.2:1b` and eight to
`ministral-3:14b`; `[INST]` is one to ministral and three to llama3.2. A model
parses its own vocabulary and nobody else's, so a rule that knows only chatml
protects only the personas that happen to run on qwen or llama — and D56(k) names
`ministral-3:14b` as the second-voice candidate, which is the family this
mutation blinds it to.

The mutation is the shape a later simplification would actually take: keep the
`<|` case, drop the other two as apparently redundant. Two checks redden and both
are the claim — the rule's own table, and the wire, because a message sent with
`[INST]` still in it is the defect whatever the table says.

```seam
id: boundary-forgets-a-family
file: persona/boundary.go
find: \tcase strings.HasPrefix(s, "</s>"):\n\t\treturn 4\n\tcase strings.HasPrefix(s, "<s>"):\n\t\treturn 3\n\tcase len(s) > 0 && s[0] == '[':\n\t\treturn bracketed(s)
after: \tcase false:\n\t\treturn bracketed(s)
red: TestEscapeBreaksEveryMarkerShapeItKnows, TestTheWireCarriesNoBoundaryTheRecordHeld
sole: true
```

## The escape keeps every character it escapes

A neutralisation that deleted the marker would be just as safe and would be a
silent transformation: a reader of a captured request could no longer tell what
the record held, and the two accounts of one event would quietly disagree —
which is the failure this project is a bet against. So the rule inserts a
backslash and drops nothing, and removing the backslashes returns the original
exactly.

The mutation drops the tail of every marker instead of writing it, which is the
cheap-looking version of the same function. Three checks redden and the
distinguishing one is `TestEscapeKeepsEveryCharacterOfTheOriginal`: the other two
would go on passing under a neutralisation that was merely *different*, and this
one is about the text still being there. Predicted two and measured three — the
standing-instruction check reddens because its want is written out by hand rather
than computed through `Escape`, which is why it is worth having.

```seam
id: boundary-drops-what-it-escapes
file: persona/boundary.go
find: b.WriteString(s[i+1 : i+w])
after: _ = w
red: TestEscapeBreaksEveryMarkerShapeItKnows, TestEscapeKeepsEveryCharacterOfTheOriginal, TestTheStandingInstructionIsEscapedToo
sole: true
```

## Deriving the wire does not rewrite the record's own turns

The turns handed to `Reply` came from bits. Escaping through the caller's slice
would make the derived form the thing the surface holds and saves, and the record
would end up carrying what was transmitted instead of what happened — D1 inverted,
arriving as an aliasing bug rather than as a decision anybody took.

The mutation is what an optimisation would look like: reuse the slice rather than
build a second one. It is guarding a property no current path can break, which is
the honest description — `Reply` builds `wire` fresh today and nothing writes back.

```seam
id: boundary-escapes-through-the-caller
file: persona/client.go
find: \tfor _, m := range turns {\n\t\tsafe, _ := Escape(m.Content)
after: \tfor i, m := range turns {\n\t\tsafe, _ := Escape(m.Content)\n\t\tturns[i].Content = safe
red: TestReplyDoesNotChangeTheTurnsItWasGiven
sole: true
```

## The stranding sweep's third column can report a number other than zero

`noscar` counts the stranded rows that have *no* scar directly above them — the
question of whether a fold is silently unreachable (D1, D14) rather than merely
illegible. It reads zero in all 270 rows of `tui/testdata/stranding.txt`, and
that zero is the evidence D58(g)'s ruling rests on. It stayed zero when the
sweep gained the length axis, at 200 and 800 bits as well as 400 — which is
worth a sentence rather than silence, because a column that is constant is
exactly the one a wider grid could have broken without anybody looking.

A frozen table cannot hold that zero up on its own, and the asymmetry is the
reason this block exists rather than a scruple about it. Measured both ways
against the golden: **inverting** the condition reddens 16+ rows, because an
inversion puts numbers where the column had none. **Deleting** the counter
leaves the table green — a dead counter's output is indistinguishable from the
constant the column already carries. So the golden pins this column in the
direction it already has values in and pins nothing in the direction of its own
constant, which is D27's shape stated generally.

This is the catalog's first block whose `file:` is a `_test.go`, and that is a
naming accident rather than a new premise: `strand` is instrument code, and it
lives beside the tests because Go's build rules put it there. The block mutates
an instrument and asserts a cited check catches it, exactly as the other
forty-six do. An instrument this project built to answer "can it fail" is the
last thing that should be exempt from the question.

```seam
id: noscar-never-fires
file: tui/strand_test.go
find: \t\tif i == 0 {\n\t\t\tnoscar++\n\t\t\tcontinue\n\t\t}
after: \t\tif i == 0 {\n\t\t\tcontinue\n\t\t}
red: TestAHeldStrandWithNoFoldAboveItIsCounted
sole: true
```
