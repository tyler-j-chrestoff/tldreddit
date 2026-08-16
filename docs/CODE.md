# Current state of the code

Go, `go 1.25.4`, Bubble Tea v2 / Lip Gloss v2 (`charm.land/*`). D6 landed content
addressing; the record/view split D1 called "the real prize" now exists.

**Setup:** a pre-commit gate lives at `.githooks/pre-commit` (build, vet,
`go mod tidy -diff`, `test -race`, gofmt), but `core.hooksPath` is local
config and isn't tracked, so a fresh clone needs
`git config core.hooksPath .githooks` once.

- `memory/bit.go` — `Bit` is the atom: ID, timestamp, `Handle`, channel, payload,
  `Prev []string`. `Bit.ID` is a content hash, not an assigned name. `Payload`
  is implemented by `Utterance`, `Compaction`, and `Vote`. `Utterance` grew
  `Truncated`; `kind()` is now value-dependent rather than type-dependent, and
  a truncated utterance names itself `"fragment"` so it takes its own address
  (D26).
- `memory/id.go` — `ID(Bit)` is SHA-256 over a hand-written, length-prefixed
  canonical encoding (no gob/JSON, both version- and order-fragile). `Short(id)`
  abbreviates for display only, never for comparison or storage. The encoding
  itself moved to `memory/wire.go`; `ID` is now just "hash the bytes a stream
  would send".
- `memory/id_test.go` — `ID`'s tests, pinned rather than merely self-consistent:
  `TestIDIsPinned` and `TestCompactionIDIsPinned` hash a fixed `Bit` and a fixed
  `Compaction` against a golden value written down once, so a silent reorder of
  the encoding is caught even though a self-comparison would not see it.
  `base()` and `coldBase()` are the two canonical fixtures this file builds —
  a fully populated utterance bit and a fully populated compaction bit — and
  every other test in the package that needs one reaches for these rather than
  building its own. `TestIDDependsOnEveryField` and
  `TestCompactionIDDependsOnEveryField` are differential tables asserting every
  field, including three rows that exist only because a length-prefixed
  encoding has to keep field boundaries from running together (`"boundary
  shifted right"` etc.). `TestIDIgnoresLabelAndBuildOrder`,
  `TestIDNormalizesTime` (a zone and a monotonic reading are not content) and
  `TestIDIsStableAcrossMapOrder` (a `Compaction` holds two maps, and Go
  randomizes their iteration) each pin one more thing that must not reach the
  hash. `TestIDRejectsAPointerPayload` and `TestIDPanicsWithoutPayload` cover
  the two ways `ID` refuses rather than addresses something. `TestShort`
  covers the display-only abbreviation directly.
- `memory/wire.go` — persistence, and the encoding both it and `ID` share.
  `canon` writes into an `io.Writer` rather than a `hash.Hash`, so the byte
  stream that produces a content address *is* the byte stream on the wire —
  D39(i)'s wire-format-not-a-save-file constraint (`docs/DECISIONS.md:2469`)
  satisfied by having one encoding rather than two that can drift. *That entry
  is cited as "D18(i)" three times in the committed tree (`git grep -c
  "D18(i)" HEAD`) and as "D38(i)" in one review; neither D18 nor D38 has an
  (i). The line number is what settles it. Why the label drifted is not
  established — D39(i) opens by citing D18(b), and D18(b) is itself the
  persistence decision, so either is a plausible source and the tree cannot
  tell them apart.* `scan`
  is its exact inverse, paired method by method in the same file for that
  reason, and `readPayload` is the inverse of `Payload.kind`: it is writable
  only while D26's value→name map is one-to-one, which makes D26 mechanical
  rather than an argument. `Store.WriteTo` / `ReadStore` round-trip the record,
  every bit re-addressed on arrival and compared against the address it was
  filed under; `View.WriteAgainst` / `ReadViewAgainst` round-trip a view
  carrying two fields — `Store.Address`, the record's own address over its
  sorted bit IDs, for provenance, and `seal`, a hash over that address and the
  view together, for integrity. **`seal` takes the address as a value, and
  integrity is tested before provenance; both were the other way for one review
  cycle and that was a real defect** — the seal hashed the live store, so the
  one field a reader most needs checked, the address written *into* the stream,
  was the field the seal never read, and a bit flipped there came back as
  `StaleView` carrying a view nothing had verified. The test that missed it
  asserted only `err != nil`; it now asserts which error and that no view
  escaped. Streams are self-delimiting, so a record and its
  views concatenate into one file or sit in several — the caller's choice.
  **What content addressing does not cover is the container**, and review found
  two holes there, both now closed and both claimed: a record's bit count
  (`endMark` closes the frame; a leftover-bytes check cannot work, since
  trailing data is legitimate by the concatenation design) and a view's own
  bytes (`seal`). `readCompaction` re-checks five things `Cool` guarantees —
  its three written invariants, plus a non-empty receipt entry and a
  non-negative kind count, the latter because negatives cancel and
  `{"utterance": -5, "fragment": 7}` sums correctly — which orphans no record
  ever written and stops a `Count()` from another process sizing a caller's
  allocation. A sixth, a non-negative word count in `Bag`, was considered and
  not taken: nothing sums it and nothing sizes from it. `StaleView` is
  an error type carrying the decoded view, so a view refused on provenance can
  still be recovered deliberately — a safety check must not become a way to
  lose the one thing here that cannot be re-derived. Two things stated in the
  doc comments rather than left to be discovered: the address comparison on
  load mostly buys an *error* rather than a panic, since `Store.Put` refuses a
  mislabelled bit anyway — except under the empty address, which Put's guard is
  gated past; and neither view field proves a view is a legitimate fold of its
  record, only that it belongs to it and arrived intact. Claims
  `record-written-unsorted`, `view-loads-against-any-record`,
  `record-frame-unclosed`, `view-seal-unchecked`,
  `compaction-count-unchecked-on-read` and `stream-framing-unpinned` in
  `docs/CLAIMS.md`; the last is the one-way-door protection, since the frame is
  the only part of the format no address covers and
  `TestTheStreamFramingIsPinned` is the only check that notices it moving.
- `memory/wire_test.go` — the round-trip suite behind every claim `wire.go`'s
  own entry above lists. `record(t)` builds a fixture holding every payload
  the package can address — a fragment sitting mid-conversation rather than
  at the end, on purpose, so a scar's tally has to carry `"fragment"` rather
  than dodging it by folding a clean prefix — folded twice so the record
  holds a scar that absorbed a scar, the only arrangement that exercises all
  four of a compaction's merged collections. `TestARecordSurvivesBeingHandedToAnotherProcess`
  is the strong form of a round trip: not merely equal by some comparison
  written here, but re-addressing identically bit for bit and re-encoding to
  the same bytes, so a defect in the encoder, the decoder or the sort alike
  would move the second file. `TestOneRecordIsOneFile` is the map-order
  determinism guarantee gob/JSON were refused for in the first place, checked
  by writing the same store twenty times and by reversing the put order.
  `TestEveryPayloadSurvivesTheRoundTrip` and `TestTheDecoderRecoversWhatOnlyTheTagCarries`
  are D26 exercised directly: the two payloads whose value reaches the
  address through `kind()` alone are the ones a decoder can only be written
  for at all if that map stays one-to-one. `TestACompactionKeepsEveryAggregate`
  checks a scar's four collections survive, and its own last check — that the
  fixture's largest scar actually merged a fragment and words rather than
  comparing two empty maps and agreeing — is there because an earlier
  version of this kind of test did exactly that.
  `TestASingleFlippedBitIsAlwaysCaught` flips every bit of every byte, not
  every byte whole, over the small pinned fixture `framed()`; its own comment
  says why the distinction matters rather than being pedantry: XORing whole
  bytes on an earlier version of this test could only *raise* a stream's
  declared bit count and never lower it, so a mutation that dropped the last
  bit of the record and returned a shorter one with a nil error went
  uncaught — "a property of which mutation the test happened to apply," not
  of the design, until the sweep changed. `TestNoByteOfARealRecordCanBeFlippedUnnoticed`
  is the same property's breadth half, over a full 25-bit two-generation
  record at one flip per byte rather than eight, because `cmd/seam` runs this
  suite upwards of sixty times per catalog and the exhaustive form costs
  seventy times as long for no structural gain. `TestACountReadOneTooLowIsRefused`
  and `TestAViewLengthReadOneTooLowIsRefused` are the specific corruption the
  sweep above found, named rather than left inside a loop of thousands.
  `TestACorruptedBitNamesBothAddresses` pins that `ReadStore`'s own address
  comparison — and not only `Store.Put`'s internal panic — is what a caller
  actually sees, by calling `ReadStore` the ordinary way and reading its
  error rather than recovering a panic. `TestATruncatedStreamIsRefused` and
  `TestAnUnknownPayloadTagIsRefused` cover a stream cut short and a payload
  kind this build does not know, the latter unskippable because nothing
  precedes a payload with its own length. `TestAStreamOfTheWrongKindIsRefused`
  covers a view read as a record, a record read as a view, something that is
  neither, and a version bump, each by its own message. `TestAViewSurvivesTheRoundTrip`
  covers a view naming one address twice and one reversed, since a view's
  order is its meaning (`Stay.Votes` settles a tie by which comes later).
  `TestALargerRecordRendersAStaleViewWithoutComplaint` is the amendment's own
  null hypothesis, run rather than reasoned: `View.Bits` already panics on an
  address the store lacks, so the open question was whether that already
  covers a stale view, and it does not — a stale view names too little, not
  something missing. `TestASingleFlippedBitInAViewIsAlwaysCaught` is where a
  real defect was caught by a review rather than by this test's own earlier
  version, which asserted only `err != nil`: `ReadViewAgainst` used to check
  provenance before integrity and compute the seal from the live store, so a
  bit flipped in the stream's own record-address field came back as a
  `*StaleView` with its integrity never checked and the recovery door open
  onto a damaged view; every assertion in the old test stayed green through
  it. `TestAStaleViewIsRefusedAndStillRecoverable` and
  `TestAViewTakenAgainstAnotherRecordIsRefused` cover the two directions a
  saved view can go stale — the record grew past it, or it belongs to a
  different record altogether — with the first proving the recovery door
  (`*StaleView`) actually returns the view rather than destroying it in the
  name of protecting it. `TestARecordsAddressIsItsContents` and
  `TestARecordAndItsViewsShareOneStream` cover order-independence and the
  concatenation design — three streams sharing one file because each is
  self-delimiting — including a reachability and a `Tally` check against the
  reloaded record, not only a byte comparison. `TestReplayingAStreamChangesNothing`
  checks `Put`'s idempotence at the level of a whole replayed stream.
  `handWritten` builds a stream field by field rather than by splicing real
  bytes, because byte-splicing broke the moment the frame grew a closing tag;
  `TestAStreamThatRepeatsAnAddressIsRefused`, `TestAStreamFilingABitUnderNoAddressIsRefused`
  and `TestACompactionThatDisagreesWithItsCountIsRefused` are all built on it,
  the last covering the specific case negatives can produce that a
  sum-only check would miss (`{"utterance": -5, "fragment": 7}` sums
  correctly to 2). `TestAReloadedInstantKeepsTheMomentAndLosesTheZone` checks
  what `TestIDNormalizesTime` already pins about the address is also true of
  what a caller reads back. `TestAWriteThatFailsPartWayIsReported` (via the
  `halting` writer) is what made `canon` writing into an `io.Writer` rather
  than only a `hash.Hash` worth doing — a byte count that kept climbing past
  a failure would be a receipt for bytes that never landed.
  `TestTheStreamFramingIsPinned` is the frame's own golden, over the small
  fixture `framed()`, and its comment draws the line this file's other pins
  do not: a bit's *bytes* are pinned transitively through the four golden
  addresses in `memory/id_test.go`, but the bytes *around* the bits — magic,
  marks, version, the closing tag, field order — are pinned by nothing else
  at all, and changing any of them without bumping the version makes every
  file already on disk permanently unreadable.
- `memory/store.go` — `Store` (`NewStore`, `Put`, `Get`, `Len`, `All`, `WriteTo`,
  `Address`): append-only, content-addressed. Identical content collapses to one
  entry. D18(b)'s persistence requirement is discharged and it went behind this
  type, which is what the type was for; what is still absent is a storage
  *engine* — file layout, rotation and atomic writes are the caller's. `All`
  (`iter.Seq[Bit]`, added for `cmd/tldr top`) is the auditor's read: until it
  existed, a record could only be read through the `View` written beside it, so
  the bits no view names — everything behind a scar, every vote, anything stored
  and never shown — were reachable from outside the package only by walking edges
  backwards from a view. It hands back address order, which is deliberately *not*
  a reading order (a caller building one has to name and defend its own), and it
  snapshots under the lock and yields outside it so a surface can go on writing
  mid-walk. Claim `record-walked-unsorted` holds the sort; the fixture is
  twenty-five bits because Go's map iteration is a rotation of insertion order, so
  a five-bit fixture would pass over the defect about one time in eight.
- `memory/store_test.go` — `Store` and `View.Add`/`View.Fold`'s own tests, most
  of them about one invariant: what a caller gets back out of the store cannot
  be used to change what is in it. `TestPutCollapsesIdenticalContent` is the
  dedup guarantee stated as a table; `TestPutRejectsAMismatchedLabel` is the
  other half, an edited bit re-`Put` under its old address panicking rather
  than silently re-addressing. `TestViewAddDoesNotDisturbTheOldView` guards
  against aliased backing arrays, since Bubble Tea passes models by value on
  every keystroke. `TestFoldLeavesEveryAbsorbedBitResolvable` is D1 itself —
  folding may take bits off the screen and must never take them out of the
  store — swept over how much of a window survives. `TestFoldRefusesWhenThereIsNothingToAbsorb`
  includes the one-bit-past-the-line row a mutation run pins to exactly this
  test and nothing else in the package. `TestFoldOfTheSameWindowCollapses` and
  `TestFoldsNestWithoutLosingTheFirstOne` cover re-folding being free and a
  scar of a scar keeping the earlier receipt reachable.
  `TestFoldRefusesAWindowWithNothingHot` is pressing the fold key twice for
  free. `TestGetCannotAlterTheStore` and `TestPutDoesNotShareTheCallersPrev`
  are the load-bearing invariant from both directions — a returned copy edited
  cannot reach the store, and a caller's own slice cannot reach into it either —
  and `TestPutDoesNotHandBackTheStoredPrev` is the third direction between
  them, since duplicate and first-`Put` used to disagree about which slice a
  caller got back. `TestFoldPanicsOnANegativeKeep` asserts the panic names
  `Fold`, so a caller's own arithmetic mistake is not mistaken for a hole in
  the record.
- `memory/view.go` — `type View []string` (`Add`, `Head`, `Bits`, `Latest`,
  `Fold`, `Absorbing`). The record/view separation: the `Store` never
  forgets, the `View` is what's shown and is the only place forgetting
  happens. `Stay` (`Votes`, `By`, `For`) is the right to be held out of a
  fold; `Fold` now takes a `Stay` and cools every run of two or more
  adjacent, unheld bits, splitting at each held bit rather than treating the
  window as one run (D30, D31, D32). `DefaultHold` is 30 minutes.
  `Absorbing` is what a fold *would* take without taking it — a screen can
  draw the coming cut before it happens. It and `Fold` now share one
  unexported traversal, `runs`, so the two cannot state the rule differently
  and drift; both panic on a stay carrying votes with no `For` (D39).
  `View.Sparing` is the exported form of that set, narrowed to the view, for a
  caller that has to count what a fold can take or draw why a row is bright;
  it is *not* the complement of `Absorbing`, since a bit can be in neither.
  `sparing` is what `runs` splits on: every held bit, **and the bit each hot
  hold names through `Prev`** — the question under an answer somebody kept,
  which the fold used to cool out from under it in 93.5% of frames at one
  upvote in ten. A covered bit is *not* held and `Stay.Holds` never says it
  is; the two categories are kept apart on purpose, since everything a screen
  draws about a vote comes from `Holds`. A cover reaches back only from a hot
  bit, because a `Compaction`'s `Prev` is the whole window it absorbed (D13).
- `memory/absorbing_test.go` — `View.Absorbing`'s tests, and the file's own
  doc comment states the tautology risk before trusting any of it: `Absorbing`
  and `Fold` share `runs`, so comparing their answers cannot catch an error in
  the rule the two share — only a divergence between what each *caller* does
  with the stretches `runs` hands out. `cooledBy` is the helper that resolves
  that risk for the fold side, and it reads `Prev` rather than a scar's
  `Absorbed`, on purpose: `Absorbed` names only originals, so a fold that
  absorbs an earlier scar never names the scar itself, which is exactly the
  bit being predicted. `TestAbsorbingIsExactlyWhatTheFoldThenAbsorbs` is the
  table this file is built around, sweeping every shape a hold can cut a
  window into, and its own comment carries the mutation catalog — which rows
  each of four specific mutations moves, and a correction to an earlier count
  in that same catalog, framed as D22's own shape (a checkable claim nobody
  had re-derived). `TestAbsorbingNamesAScarItWouldCoolAgain` is D32 asked
  through this function rather than through `Fold` directly: a scar alone in
  a window is not going anywhere, and the receipt cannot be asked this
  question since it never names the scar. `TestAbsorbingFilesNothing` checks
  the store's length is unmoved by five straight calls, live rather than
  vacuous, since the same fixture folded is asserted to move it by exactly
  what the fold costs. `TestAbsorbingPanicsOnANegativeKeep` names `Absorbing`
  rather than `Fold` in its panic message, for the same reason
  `TestFoldPanicsOnANegativeKeep` names `Fold` — the caller holding a screen
  did not call the fold. `TestRunsHandOutTheWholeViewInOrder` sweeps view
  length as well as keep and is written up as the check that found a real
  gap: the empty-stretch half of the invariant was asserted against a fixture
  that could not produce one, while the code could on any empty view with a
  keep above zero.
- `memory/sparing_test.go` — what a hold covers and what it does not, in
  pairs, because the rule fails in two directions and only one of them is the
  defect it was written for. `TestAFoldKeepsTheBitAHeldBitNamesThroughPrev` is
  the headline and asserts the edge as well as the rule — the bit *before* the
  named one is still cooled, so the cover reaches one step and not
  transitively. `TestABitACoverSparesIsNotHeld` is the one that keeps a cover
  from becoming a vote nobody cast: absent from `Stay.Holds`, absent from
  `Absorbing`, present after the fold. `TestAHeldScarSparesOnlyItself` builds
  by hand the arrangement the `hot` guard exists for — a scar beside the bits
  it names — and `TestAScarInAViewNeverNamesABitStillInIt` is the claim that
  no view produced by `Add` and `Fold` can reach that arrangement today,
  written as a check rather than a sentence so the day it stops being true is
  a red rather than a comment nobody re-derived.
- `memory/vote.go` — `Vote` is one participant's judgment on one other bit,
  addressed via its single `Prev` edge; its `dir Direction` field is
  unexported, so `Cast` is the only way to spell one that names a direction.
  `Direction` (`Up`/`Down`) has two values and `kind()` refuses to name any
  other (D33). `Tally` reads a vote view and reports a `Score` per handle;
  `standing` resolves one voter's current vote per target. Never a ranking
  input (D45(h)) — `Tally` weights D4's consolidation signal and stays out
  of any future ordering.
- `memory/vote_test.go` — `TestTallyPanicsOnAViewThatIsNotVotes` now
  asserts the panic message itself, not just that something panicked: it
  must name the offending bit's address and what it actually carries.
  Strengthened this session after `cmd/seam` reported the old version
  `vacuous` — it asserted `recover() != nil` and could not tell the guard's
  own message from `Tally`'s unrelated interface-conversion panic two lines
  later (D45(e)). Its sibling, `TestCastPanics`, had the identical shape,
  was reported rather than repaired by the same hand that found it, and was
  closed a session later: it now names, per row, the substrings its panic
  has to contain. Claim `a-refused-direction-names-itself` in
  `docs/CLAIMS.md` returns `proven`; re-check with
  `go run ./cmd/seam -run a-refused-direction-names-itself`.
- `memory/rank.go` — `View.Rank(s *Store, votes View, by Handle) []Ranked` is
  D3's first surface: a second reading of a view, ordered by votes rather
  than time, returned as a `[]Ranked` rather than a `View` so it cannot be
  mistaken for the transcript (D30, partially superseded by D49(d)).
  `Ranked{ID, Own, Others}` never sums the two: `Own` is `by`'s own standing
  vote, `Others` is everyone else's summed, and the sort compares `Own`
  first, so no crowd of `Others` votes crosses a participant's own —
  `docs/CLAIMS.md`'s `rank-merges-the-tiers` claim holds it; re-check with
  `go run ./cmd/seam -run rank-merges-the-tiers`. Ties keep view order via
  `slices.SortStableFunc`, held by the `rank-ties-by-address` claim. It reads
  `standing` — the unexported traversal `Tally` and `Stay.Holds` already
  share — not `Tally`: a per-handle aggregate across bits is karma, which
  D45(h) bars from ranking, and D49(c) clarifies that bar as reaching karma
  specifically, not the per-target standing vote `standing` returns. Reads no
  clock, so it stays pure in `Fold`'s sense (D38(c)'s simulator stays free); a
  scar ranks only on votes cast on the scar itself and inherits nothing from
  what it absorbed, a ruling left unbuilt (D49(e)). See D3, D49.
- `memory/rank_test.go` — `View.Rank`'s tests, every expectation in them
  written by hand from the rule in `Rank`'s own doc comment rather than read
  off a run or compared against `Tally` — since `Rank` and `Tally` share one
  traversal, an agreement check could not catch an error in the traversal
  itself. `rankFixture`/`order` are the shared fixture and the helper that
  turns a `[]Ranked` back into transcript positions so a hand-written table
  stays readable. `TestRankOrdersAViewByItsVotes` is that table: the
  participant's vote sorts into three bands, an agent orders only what the
  participant left level, and votes it is silent about keep their arrival
  order. `TestNoCrowdOfVotersCrossesTheParticipantsOwnVote` sweeps 1 through
  50 agents rather than fixing a crowd size, since the failure it guards
  against is arithmetic — a merged score crossing the tier at some particular
  count — and one fixture could sit on either side of that count by luck.
  `TestRankLeavesTheViewItRanked` is pinned by a mutation run recorded in its
  own comment as catching two distinct faults rather than one, a correction
  to an earlier version of the same comment that claimed only one.
  `TestRankIsTheSameOrderEveryTimeItIsAsked` samples 50 calls over 20 voters
  because `standing`'s map iteration is deliberately unstable and one call
  cannot tell a stable answer from a lucky one. `TestRankedCarriesTheTwoNumbersSeparately`
  pins `Own` and `Others` never being summed. `TestRankPanicsOnAViewThatIsNotVotes`
  asserts the panic message rather than merely that one occurred, inheriting
  the same shape `TestTallyPanicsOnAViewThatIsNotVotes` and `TestCastPanics`
  were repaired into, and is stated as *not* a seventh instance of the six
  D48(g) already counted in this package.
- `cmd/seam/` and `docs/CLAIMS.md` (D45) — the seam-check: a claims file of
  human prose plus fenced ` ```seam ` blocks (`docs/CLAIMS.md`, 35 claims —
  `grep -c "^id: " docs/CLAIMS.md`, run 2026-08-13. **A line count used to
  sit here and has been deleted rather than corrected.** It said 851, then
  1006; it was 1043 by the time anyone checked, because the file kept growing
  inside the same work unit that repaired it. That is the tenth instance of
  the hand-transcribed-figure defect D48(i) tallied and D49(h) carried to a
  seventh — and the tenth was manufactured by the act of fixing the eighth
  and ninth. The claim count stays because it changes rarely and means
  something; nobody ever needed the line count, so removing it removes the
  failure instead of resetting its clock)
  that `cmd/seam` parses and re-breaks in a copy of the working
  tree, never in the repository itself. The claims file *is* the catalog —
  no second copy of the mutation table exists anywhere else, which is the
  property `words/tools/mutate.mjs` (the prior art this replaces) lacked. A
  claim declares its own `verdict:` (default `proven`, or a set like
  `proven|killed-mid-check` for an honestly nondeterministic check); the
  gate fails on any claim outside its declared set, in either direction. A
  baseline run refuses to proceed against a non-green tree, and a per-claim
  control is sampled at the same N as the mutant, so a control that
  reddens even once makes the claim `unattributable` rather than proven.
  Every verdict is now printed under the address of the tree it was taken
  against (`cmd/seam/digest.go`): a sha-256 over the copy the claims run in,
  paths and file bytes and the execute bit and symlink targets, `.git` out
  and permissions beyond execute out. Before that, a run was stamped with a
  wall clock, which names the moment and not the subject — so a figure like
  the one below stopped being checkable the instant anything committed, which
  is what happened twice to this file. Two things that address does not do:
  it addresses a *working directory* and not a commit, so a built tree and a
  fresh clone of the same source differ; and it makes staleness detectable
  rather than impossible, since nothing stops a figure being quoted without
  it. `go run ./cmd/seam -list` prints the address in about a second and runs
  nothing.
  Re-check: `go run ./cmd/seam`, and this is the form a transcribed figure
  has to take from here — the tree it was taken against, named. Four full
  runs 2026-08-13, `200 checks green, 22 skipped (-race)`, 27 claims, 2m49s
  each. Three came back 26 proven, 1 killed-mid-check, 0 vacuous, 0 adrift
  (trees `8800841d…`, `22b2f12d…`, `85eb6a2a…`). The fourth, against tree
  `27083637b962bf75d672f5ff014002c3352bba393fdbe5ee05c58212f95d7b2f`, failed
  the gate: `store-unlocked` came back `killed-mid-check` against a declared
  `proven`, because one of its three cited checks,
  `TestConcurrentFoldsAgreeWithOneSequentialRun`, asserted 0 of its 16
  samples instead of aborting into some other check's red. Under
  `go run ./cmd/seam -run store-unlocked` the same check asserted 5, 4 and 4
  of 16 across three runs, so **that claim's declaration is narrower than the
  claim honestly is, and the gate is flaky by construction today** — the
  reading `docs/CLAIMS.md`'s own header bars. Left for a hand that is not
  mid-build; two commands, two rates, both above.
  The addresses above are of the trees that produced the figures and
  therefore never of the tree holding this sentence: writing a run's address
  into the tree it describes moves the tree, the same way a commit cannot
  contain its own hash. So an address here always names a parent, and what it
  buys is that a reader can tell whether it names *theirs*.
- `cmd/seam/claims.go` — `parseClaims`/`parseBlock`, the catalog parser: one
  `claim` per fenced ` ```seam ` block, every field required and unknown keys
  refused rather than defaulted, because a catalog that parses loosely is one
  whose entries can stop meaning what they say without anything failing.
  `id`/`title`/`line`/`file`/`find`/`occ`/`after`/`red`/`sole`/`among`/`race`/
  `declared`/`isolate`/`runs` are the fields a block can set; a duplicate id
  across the whole file is refused at the end of the parse rather than left
  to `-run` to disambiguate. `unescape` reads exactly three escapes
  (`\n`, `\t`, `\\`) so a multi-line anchor can be spelled — an escape it
  does not know is an error rather than a literal backslash, since a value
  here is spliced straight into source.
- `cmd/seam/tree.go` — `copyTree` (the whole working tree, minus `.git`,
  copied into a fresh temp directory — symlinks copied as links rather than
  followed, so a link pointing outside the tree cannot smuggle material into
  the copy) is the entire safety story this tool rests on: nothing ever
  opens a file inside the repository for writing, so an interrupted run
  leaves the repository byte-identical rather than depending on a restore
  step a crash can skip. `occurrences`/`mutate`/`nth` count an anchor before
  touching it, because two of the three possible counts are findings in
  their own right — zero means the catalog has gone stale against the code,
  and more than one in a block that never chose an `occ` means nobody chose
  and the tool must not choose for them — and `mutate` refuses a no-op edit
  (`after` that spells `find` back unchanged) rather than silently declaring
  a claim proven against a tree nothing actually moved in.
- `cmd/seam/run.go` — the execution and verdict half. `runSuite`/`readEvents`
  drive `go test -json` (`-count=1`, so a second sample is not served from the
  test cache) and turn its events into an `outcome` per `check` — green,
  `assertRed`, `crashRed`, `abortedRun`, `skipped` or `absent`, kept apart
  rather than collapsed into pass/fail, because a check that reddens through
  a panic has not shown what it claims to catch and a check that never
  started has shown nothing at all. `crashy` distinguishes the runtime dying
  from a check's own printed output by column: everything the testing
  package prints on a check's behalf is indented, so an unindented panic
  banner at column zero is the process talking rather than the test. The
  `verdict` enum (`proven`, `vacuous`, `overRed`, `crashProof`, `killed`,
  `neverRan`, `staleAnchor`, `ambiguousAnchor`, `staleCitation`,
  `unattributable`, `brokenBuild`) is eleven different facts rather than a
  pass/fail, and `judge` is where two rate tables — the same checks sampled
  the same number of times on the mutated tree and on the unmutated one —
  become one verdict: a red on the mutant alone is not evidence until the
  same check stays clean on the control throughout, and a control that
  reddens even once makes the claim `unattributable` rather than proven,
  never repaired by sampling until it behaves. `resolve` refuses to guess
  which of two packages a check name shared between them was meant to cite,
  since guessing wrong would mutate the wrong package and print a verdict
  about a check that never ran. `status` is the exit code: 2 when a claim
  is not where it declares itself to be, 3 when the tree moved mid-run so
  the printed address does not cover every verdict, 2 winning when both are
  true because drift weakens a claim rather than voiding it.
- `cmd/seam/report.go` — `printCatalog`/`printReport`/`printJSON`, the human
  and machine forms of one inventory, deliberately never a score: a
  percentage of claims proven would rise when a claim worth having — one
  likely to come back vacuous — is deleted, which is exactly backwards.
  `printIdent` is shared by both forms so a `-list` run and a real report
  print the tree identity in the same words and are comparable by eye; it
  also carries the "tree moved mid-run" warning at the one place a reader
  who only reads the top of a report would still see it. `oneLine` renders a
  multi-line anchor back through the same escapes the catalog is written in,
  so what a report shows can be pasted back into `docs/CLAIMS.md`.
- `cmd/seam/digest_test.go` — `digestTree`'s tests (the function itself is
  in `cmd/seam/digest.go`), organized entirely around what the address
  distinguishes and what it deliberately does not. `TestWhatTheTreesAddressDistinguishes`
  is a table where most rows expect the address to move — one byte of one
  file, two files' contents swapped, a byte-identical rename, the execute
  bit, a symlink's target, an empty directory — and three are held apart
  because they say the address does *not* move: doing nothing to the tree,
  a file's mtime, and a non-execute permission bit, the last two because a
  checkout's own umask and clone time would otherwise give two identical
  trees two different addresses. The comment above the test names two exact
  stub edits inside `digestTree` and states which rows each one reddens, so
  the claim that this table can fail is re-derivable rather than taken on
  faith — one of those two is carried into `docs/CLAIMS.md` as
  `tree-address-drops-contents`. `TestCopyingATreeDoesNotChangeItsAddress`
  checks `copyTree` preserves everything the address reads.
  `TestATreeThatMovedMidRunIsMarkedRatherThanRefused` exercises `checkOne`'s
  own drift-handling directly, with a fixture module built by hand rather
  than `seed`'s, because this row has to actually run a check rather than
  only address a tree.
- `cmd/seam/fixture_test.go` — the tool's self-test, and its own comment
  states why it exists: a tool that reports which checks cannot fail has to
  be able to fail itself, and saying so is not the same as showing it.
  `TestTheToolCanReportEveryVerdictItHas` runs the real pipeline
  (`runCatalog`, the same baseline, the same mutation machinery) against a
  disposable fixture module built by `fixture` — never against `memory/` or
  `tui/`, since mutating the product here would be a second, silent catalog
  nobody reads — and asserts every verdict shape this tool can produce is
  actually reachable, including the gate itself: `fixture-declared` and
  `fixture-set` make the identical break and differ only in what they
  declare, landing on opposite sides of `adrift`. The comment above the test
  also names the two stubs (in `judge` and in `checkOne`) that would have to
  be applied together to prove this test can fail, since either stub alone
  leaves half the table unexamined — one settles some rows before `judge` is
  ever called. `TestAFlakyCheckIsUnattributableRatherThanProven` needs a
  check that passes once and then fails, and counts its own runs in a file
  beside itself rather than by a clock, specifically so this test is not the
  flake it exists to catch.
- `cmd/seam/seam_test.go` — the parser and pipeline's own unit tests, apart
  from the self-test in `fixture_test.go`. `TestABlockParsesIntoExactlyWhatItSays`
  pins every field of one hand-written block, including the title and line
  number a stale anchor's report would need. `TestAClaimDeclaresItsOwnVerdict`,
  `TestADeclaredSetStillFailsOnEverythingOutsideIt` and
  `TestADeclarationFailsInEitherDirection` are the verdict-set guarantee
  from three sides: a block with no `verdict` key expects `proven`, a set is
  not an opt-out (every verdict outside it still fails, including a claim
  that quietly starts coming back clean), and the gate fails in either
  direction — a claim that stops reddening trips it as loudly as one that
  starts. `TestTheExitStatusSaysWhichKindOfFindingItIs` pins `status`'s
  precedence: a claim outside its declaration (2) beats the tree having
  moved mid-run (3) when both are true. `TestAnAnchorCanSpanLines` is the
  reason the three escapes exist — the only way to unlock a store and go on
  compiling is to replace a field declaration and the lines around it.
  `TestAMalformedBlockIsRefused` is the whole table of catalog mistakes that
  must be refused rather than defaulted (an unknown key, a missing required
  key, a key given twice, an occurrence that is not one, an unknown escape,
  `among` without `sole`, a verdict — or a member of a verdict set — nobody
  reports). `TestADuplicateIdIsRefused` and `TestAnUnclosedBlockIsRefused`
  round out the parser's refusals. `TestTheMutationLandsInTheCopyAndNeverInTheTree`
  is the safety invariant stated as a test rather than only as a doc
  comment: `.git` is absent from the copy, the copy is mutated, and the real
  tree is untouched. `TestAnAssertionAndACrashAreNotTheSameRed`,
  `TestADetectedRaceIsTheCheckAsserting`, `TestASkipIsNotAPass` and
  `TestABuildFailureIsReadBeforeAnyRed` all replay real `go test -json`
  event sequences recorded from a live go1.25.4 run rather than composed by
  hand, because the distinction the whole tool rests on — a check reporting
  its own failure versus the process dying under it — is one only the
  toolchain can settle. `TestTwoPackagesWithOneTestNameDoNotShareASlot` and
  `TestAnAmbiguousCitationIsRefused` are the keying bug this file exists to
  keep closed: two packages carrying one test name give a bare-name map one
  slot, so a red check in one package could be silently overwritten by a
  green one in another, and a citation of such a name is refused rather than
  resolved to whichever package happened to win the race. `TestJudge` is the
  large table behind `judge`'s verdicts, including soleness narrowed to an
  `among` set. `TestBothRatesAreCounted` and `TestTheRunAProofLandedInIsReported`
  pin that both the control and mutant rates reach a result, and which run a
  nondeterministic proof first landed in. `TestAStrandedCheckIsNeitherRedNorIgnored`
  checks a check outside the cited set that never finished is reported apart
  from one that failed.
- `memory/cool.go` — `Cool` now *derives*: nothing is removed, the cold bit
  takes the view's slot while every absorbed bit stays in the store. Its
  `Prev` is every bit in the window, in window order (D13). `Compaction`'s
  fields are unexported, read only through accessor methods (`Count`, `From`,
  `To`, `Handles`, `Kinds`, `Bag`, `Absorbed`), and `Cool` is the only
  constructor reachable from outside the package. Precisely: `Cool` is the only
  way to build a *populated* one from outside, but a bare `memory.Compaction{}`
  literal is still constructible — the fields are unexported, the type is not.
  D3's addendum (a view held at most one `Compaction`, always at index 0) is
  discharged, not merely tracked here anymore: `View.Fold` can now produce
  many scars and a view that starts with a hot bit. See D32.
- `memory/cool_test.go` — `Cool`'s tests, and the file that defines the
  package's other two shared fixtures: `t0`/`at()` (a fixed origin, so a
  failure here means the code changed rather than that time passed) and
  `said()`, the addressed-utterance builder every other test file in the
  package reaches for. `TestCoolAbsorbsWindow` pins the shape of a fold's
  `Prev` — the whole window, in window order, never the root the window hangs
  off of (D13). `TestCoolNamesTheFoldItAbsorbed` and
  `TestCoolIsClosedUnderItself` are the leak this whole design exists to
  refuse: cooling a fold of a fold must not quietly drop a generation's
  count, bag or receipt. `TestCoolSpansABitWithNoInstant` covers the zero
  time as a legal instant rather than an empty-accumulator sentinel — a case
  the surface cannot currently reach but the span accumulator could get
  wrong. `TestCoolMergesHandlesAndKindsAcrossGenerations` and
  `TestCoolDedupesHandlesInOrder` cover the aggregates that would render
  identically either way if they merged wrong, which is what makes the bug
  worth a test rather than an eyeballing. `TestCompactionAccessorsAgreeWithTheFold`
  checks the read-only accessors agree with what `Cool` built, without
  re-testing that a holder can't reach back through them —
  `TestGetCannotAlterTheStore` is where that would bite. `TestCoolPanics`
  covers an empty window, a window crossing channels, and an unaddressed
  bit, the last because `Prev` and `Absorbed` would both take an empty
  string and silently promise it resolves.
- `memory/fragment_test.go` — a fragment's tests across the three places its
  truncation could quietly stop meaning anything: the address, the store, and
  the fold. Deliberately does not re-check that a complete utterance still
  addresses as before fragments existed — `TestIDIsPinned` already is that
  check, and it stayed green through this change. `cutOff` is `said`'s
  counterpart for a speaker who ran out of room, built to the same shape on
  purpose so every paired test's only difference is the truncation.
  `TestAFragmentAddressesApartFromACompleteUtterance` sweeps text that spells
  the kind tags themselves (`"fragment"`, `"utterance"`), which is the case a
  length-prefixed encoding without them would let a speaker forge.
  `TestFragmentIDIsPinned` is derived from the encoding by hand rather than
  copied out of a failing run — "a pin taken from the output it is supposed
  to check is not a pin." `TestAStoreFilesAFragmentApartFromItsCompleteTwin`
  is the same guarantee at the store, where identical content collapsing by
  design is exactly why the distinction has to live in the content rather
  than beside it. `TestAFoldTalliesFragmentsSeparately` and
  `TestAFoldOfAFoldKeepsTheFragmentTally` check `Kinds` sums a `"fragment"`
  entry apart from `"utterance"` while still summing to `Count`, and that a
  fragment's words still reach the bag — dropping them would be the opposite
  error to the one this change exists to fix. `TestAFragmentStaysReachableThroughAFold`
  is D14 asked about a fragment specifically: the scar's tally says one
  fragment, and this is the reader being able to go and read it.
- `memory/reach_test.go` — `TestEveryStoredBitIsReachableFromTheView` walks
  `Prev`/`Absorbed` from the view and asserts every stored bit is discoverable
  (D14). This is the test that caught the D12 orphan and that D13 fixes.
  `reachable` is now variadic over views: a vote view names nothing that
  points back at the vote, so a reader must hold both the transcript and the
  vote view for reachability to hold (D34).
- `tui/tui.go` — holds a `*memory.Store`, a `memory.View` (`shown`) and now a
  second, never-folded `memory.View` (`votes`; `memory.Tally` panics if it
  ever is folded). `send()` delegates to `utter(handle, memory.Utterance)`,
  the one place a bit is written. `ctrl+u` toggles `unfold()`; `ctrl+k`
  still folds. The persona is wired in: `enter` asks a live ollama persona
  and records the reply, built from the **folded** view so a `Compaction`
  reaches it as a system turn — and never from `votes` (D39(a)): showing an
  agent its own votes turns a consolidation signal into a behavioural one.
  **The vote (D39).** `Model.mark` is a content address, not an index: the
  caret rides the newest bit (`riding()`) until `up`/`down` move it, and
  follows its bit onto the scar that absorbs it via the cold bit's `Prev`
  (D13). `shift+↑`/`ctrl+o` upvote (`vote(memory.Up)`) and hold the bit for
  `holdFor` — 2 minutes, measured against this surface's own bit cadence
  rather than `memory.DefaultHold`'s 30; `shift+↓`/`ctrl+r` downvote, which
  withdraws a hold on the same frame without running the fold trigger.
  `foldable()` (hot bits no hold is sparing) replaced `hot()`, which wrongly
  assumed scars are a prefix; it reads `memory.View.Sparing` rather than
  `Stay.Holds`, because a hold now spares the bit its answer answers as well
  and a trigger counting the covered bit sat permanently over its budget —
  measured at 100x30, one upvote in three, 122 folds against 30, a scar for
  every third thing said. `pressured()` is the fold trigger and `blocked()`
  names the state where D32's size rule meets a run of holds and nothing can
  fold; at one vote in *two* every row is held or covered, so there is no
  pressure at all and `blocked` is correctly false — a different state, in
  `docs/DEBT.md`. `covered(holds)` is `Sparing` minus the holds: the rows a
  vote is keeping that nobody voted on. `absorbing()` asks `memory.View.Absorbing` rather than re-deriving
  the cut, and states its own narrowed guarantee: a hold expiring in the
  same write that folds is the one case where something goes without being
  drawn cooling first. `day()` is the view's newest instant, printed once in
  the header so a row's own clock drops the date until it differs. `chrome`
  is 7, not 8 — measured under `HARNESS=1`, not derived from the doc
  comment; overflow starts at height 7. **The fade in space (D42).** The
  package doc now states two holes rather than one: a hold expiring in the
  same write that folds (pre-existing), and a scar — a fold absorbs a cold
  bit like any other, so a scar routinely (measured: always, in an unvoted
  conversation) sits in the set the next fold takes, and nothing on screen
  says so in either channel. Both are named explicitly rather than left to
  be found, with the test that holds each one to its size. Two accessors are
  exported for `cmd/tldr`'s non-interactive verbs and for nothing else:
  `tui.Channel()` (the channel a bit written into this surface's view has to
  carry, since `memory.Cool` panics across two) and `tui.Human()` in `ask.go`
  (the handle this surface writes for the person, which is who a ranked reading
  is *for* — `memory` refuses to name the human itself, see `memory.Stay`).
  Accessors rather than exported variables, so the answer stays this package's to
  give: a caller may ask who the human is and may not decide it.
- `tui/style.go` — the palette: `dim`, `rule`, `speaker`, `system`, `warm`,
  `hot` (the terminal's own foreground, left unstyled on purpose — recent
  material should look like the terminal rather than a theme) and `cooling`
  (242, what fades toward a fold). `seamInk` (244) carries D42's finding in
  its own doc comment rather than restating it here: a scar drawn in `rule`
  (238) was measurably the darkest object on screen while standing for the
  one thing on it that is absent, and it is a style of its own rather than a
  brighter `rule`, because `rule` has two other jobs in a receipt that break
  at 244. Every call site in this package reaches a colour through one of
  these names rather than a raw ANSI code, which is what keeps a colour
  decision made once here instead of re-argued at each row.
- `tui/ask.go` — the request/reply cycle. `waiting`/`trouble`/`epoch` are
  display state only, never bits: a reply not yet arrived did not happen,
  and a failure is a fact about this harness rather than the conversation.
  `turns()` builds what the persona sees from the view, assigning roles by
  `Handle.Ref` rather than `Display`; a `memory.Compaction` becomes a
  `persona.RoleSystem` turn via `foldNote`. `recordReply` is the only path
  from an answer into the record and now writes a truncated reply as
  `Truncated: true` rather than refusing it. `turns()` never hands a
  fragment to the persona as a finished assistant turn: it becomes a
  `persona.RoleSystem` turn via `fragmentNote`, quoting the exact text
  rather than speaking it in the fragment's own voice (D35). `notice.sticky`
  and `notice.at` are gone — `endsUnfinished()` derives the remedy line from
  the view instead of a stored flag. `turns()` never shows the persona its
  own votes (D39(a)), and `foldNote` now stamps its span with the same
  `clock` `seam()` draws with, so a fold crossing midnight is not told it
  ran backwards. `standingInstruction` is the persona's whole personality
  and is the only place anything says who it is; it was a guard list of
  prohibitions and is now written as facts about the situation, with every
  epistemic commitment intact — the register moved, the doctrine did not.
  It never names the vote, in any word, which
  `TestNoVoteReachesThePersona` now covers alongside the turns: a model
  told its answers are kept or let go writes toward being kept without ever
  seeing a score. **`scarWords` is gone and `personaWords` no longer has a
  twin (D59(j)).** The two carried a claim — the words a person reads on a
  scar are a prefix of the words the model was given — and that claim is
  retired rather than kept: the scar quotes a bit the reader's votes chose,
  and sending the model the same quotation would send it a message
  *selected by the human's approval*, which is D39(a)'s pump arriving
  through a door nobody was watching. `foldNote` keeps the word index and is
  now its only caller. `TestNoVoteReachesThePersona` gained a second arm for
  it — byte equality of `turns()` over a record that has actually folded,
  with and without a vote inside the fold; the first arm's six-bit fixture
  has no scar in it and could not see this at all. It is **not versioned** — `persona.Persona.System` is
  deliberately outside `Handle()`, so no bit in the store distinguishes
  what was said under the old instruction from the new.
- `tui/ask_test.go` — the request/reply cycle's tests, driven through
  `Update` and `Model.send`/`turns`/`recordReply` wherever a key or a
  message can reach them, rather than by calling internals directly.
  `offline()` is the only client any test in this file is allowed to hold —
  a `*persona.Client` pointed at an unusable address, so a request fails on
  `persona.Unusable` before any I/O runs, and this suite cannot pass here
  and fail on a machine without ollama, which is the defect a networked
  test would be. `TestEveryExitPathClearsTheWait` drives all four ways a
  pending request ends — an answer, a failure, a call-off, and `esc` —
  through the same exit path, because a pending state that survives one of
  the four is a spinner that never stops.
  `TestAFragmentReachesThePersonaAsSomethingUnfinished` and
  `TestTheUnfinishedLineSaysWhyAndWhatToDo` cover D35 from both ends: a
  truncated reply reaches the persona as a system turn quoting its own
  words rather than as a finished assistant turn, and the screen names why
  it stopped and what to do about it, taken down by whatever is said next
  rather than left sticky. `TestTurnsMapRolesByRef` asserts a persona
  renamed mid-session still reads its own past words back as its own,
  because roles are assigned by `Handle.Ref` and never by `Display`.
  `TestFoldingShrinksWhatThePersonaIsSent` checks that the fold turn stands
  exactly where the scar stands, one turn for the whole run, arriving as
  `persona.RoleSystem` rather than as the expanded bits or a gap.
  `TestTheRequestGoroutineTouchesNeitherViewNorModel` races the update loop
  — bits landing and folds firing on the caller's own copy — against the
  closures a batched command hands back, reading the answer only off the
  channel the runtime would use, which is what makes a data race here a
  compile-time impossibility rather than a `-race` finding to hope for.
- `tui/render.go` — `transcript(frame)` draws every row but the caret's row
  through `said()`, shared with the receipt so the two cannot drift. The
  caret's own row is drawn whole instead, by `saidWhole()` (`render.go:262`),
  wrapped across as many rows as its sentence needs rather than cut at the
  margin — the one variable-height row this surface has, which is why a
  view's drawn height is the bit count plus one bit's worth of wrapping and
  not a multiple of it. `said` and `saidWhole` both read through
  `unfold.go`'s `oneLine`, so the two cannot state a bit's text differently,
  only how much of it a row shows. Both now take a `frame`
  (built once per draw by `Model.frame()`): one resolution of the store, the
  absorbing set, the current holds and the vote tally, so no two rows on one
  screen read the record at different moments. `caretCell`/`voteCell` draw
  the caret and the vote mark (`▲` held, `△` expired, `▼` downvoted, with a
  draining gauge beside a live hold) in the margin reserved ahead of the
  handle. `voteCell` has a fourth state that is not a vote: a **covered**
  row — one a hold spares because the row below it was kept — draws `tie`
  (`╷`), half a stroke in the mark column hanging into the `▲` beneath it,
  and never a ballot glyph, since nobody cast one on it. It costs no columns
  (the vote column exists only once somebody has voted, which is the only way
  a covered row exists) and it is suppressed on the ranked surface, where the
  row below a covered one is not its holder — a guard that was unreachable
  until `judged()` widened and is now the live thing
  `TestTheRankedSurfaceDrawsNoTie` holds. **The tie says a position, not a
  relation.** `Prev` is the head of the view when a bit was written, which
  coincides with "the turn this replies to" only in an alternating session
  at the keyboard; measured on this project's own record, 7 of 29 said bits
  came in through `tldr say`, one of them a correction whose `Prev` is a
  greeting rather than the claim it corrects. So the mark means *the row below
  is keeping this row
  out of the next fold*, which is true on every frame, and the surface says
  nothing about why the two were adjacent — the limit any later feature
  built on that edge inherits. The stroke also carries down the caret's own
  block (`transcript`), because the caret's row is the one row drawn whole
  and drawing the tie on its first line alone left it five lines above the
  mark it points at on a real record (`TestATieReachesTheMarkItPointsAt`). `seam(frame, memory.Compaction, width)` draws the scar in `seamInk` (244), no longer `rule`
  (238) — the scar used to be dimmer than the faded material it explains.
  **What it says about the content is a quotation, not a word count
  (D59(j)).** `frame.quoted` walks a fold's `Absorbed()` backwards through
  the store and returns the absorbed bit this reader's own standing votes
  rank highest — `memory.View.Rank`'s two tiers, own before others — with
  the tie going to the newest, so an unvoted scar quotes the last bit it
  took and the row directly beneath it is what followed. `frame.quotation`
  draws it as `who "words"`, cut by `said` like any other row, with the
  unfinished mark placed *outside* the closing quote (`unmarked(row, marked)`,
  `unfold.go`) so this surface's vocabulary is never inside an assertion
  that these are somebody's exact words. `marked` comes from
  `Utterance.Truncated` rather than from the row, which is what makes the
  split exact: matching a suffix alone took a participant's own `╌` off them
  and re-attributed it to the program, so `all done ╌ unfinished ╌` drew as
  `me "all done" ╌ unfinished ╌` — the surface asserting somebody stopped when
  they finished (`TestASpeakersOwnMarkStaysInsideTheirQuotation`). The speaker is never dropped and
  truncates instead, floored at `nameFloor` and marked with an ellipsis — the
  handle column's own rule (`cell`, `nameColumn`) rather than a new one.
  Dropping it was measured to make a terminal one column wider show three
  fewer characters of what somebody said; keeping it whole was measured to
  cost the whole quotation at sixty columns with a vote cast, where
  `coordinator-7` is thirteen of a twenty-four-column allowance. The span is taken only once
  the whole quotation already fits beside it, for the same monotonicity
  reason. Below `quoteFloor` columns of surviving words the row falls back to
  count, span and key rather than shedding words one at a time — a single
  word off a frequency count is the noise this replaced. It also refuses
  outright any quotation wider than the room it was given: `nameFloor` and
  `said`'s clamp of any width below one back up to one meant a one-character
  bit — its own whole sentence, so `quoteFloor` does not refuse it — drew a
  fifteen-column row in a one-column slot, which the caller's `clip` then cut
  through the closing quotation mark (`TestNoScarRunsPastTheWidthItWasGiven`).
  `clock` stamps a time as bare `15:04` on the day the header already
  named, and grows a date only on a row that differs from it. **The fade in
  space (D42).** `caretCell(marked, scar, open, going bool)` gained a fourth
  argument: a row the next fold will take (`going`) now begins at
  `caretColumn` rather than `caretColumn+step`, so the left edge reads scar
  at column 0, going at 1, staying at 3 (`caretWidth`/`caretColumn`/`step`
  constants). A lone jogged row is possible — a scar counts toward
  `View.Fold`'s run length but cannot step — and is always beside a scar,
  pinned by `TestALoneJoggedRowIsAlwaysBesideAScar` (`tui/tui_test.go`). The
  sentence column (`room`) is measured once per frame from a *staying* row
  so nothing reflows when a bit starts or stops cooling; the vote-column
  threshold now reads `caretColumn+step` rather than the stale `caretWidth`,
  fixing a non-monotonic regression pinned by
  `TestWideningTheTerminalNeverTakesAnythingOffARow`. No width figure is
  written in the doc comment on this function — three were, in three review
  rounds, and all three were wrong; the numbers live only in that test.
- `tui/unfold.go` — renders a scar's receipt: one row per absorbed bit, each
  carrying its own ordinal (`12/21`). A drop ladder sheds columns under
  width pressure: time, then address, then the handle shrinks, text last —
  reversed from address-then-time, because the address is the auditor's own
  checkable instrument and a time is recoverable from the scar's own span. A
  vote column (mark only, no gauge — nothing under a live hold can reach a
  receipt) appears only once something in the receipt carries a vote, and is
  never dropped after that. `said(frame, b, width)` is the row definition the
  receipt always uses — one row per absorbed bit, never wrapped — and the
  one `render.go`'s transcript uses for every row except the caret's; the
  caret's own row there is drawn instead by `saidWhole(frame, b, width)`, wrapped
  across as many rows as it needs. Both take the frame because a **fold**
  drawn as a row is not a fact about that bit alone: they route a
  `memory.Compaction` to `scarLine`, which gives the ranked surface the same
  quotation the transcript's scar carries, so one object has one account of
  itself on both screens (`TestAScarQuotesTheSameBitOnBothSurfaces`).
  `saidWhole` returns exactly one line for a fold, so a scar's row on the
  ranked surface no longer opens a block. `oneLine`'s Compaction branch is
  now the count alone — the four bag words it used to carry are gone, and
  nothing that draws reaches that branch; its witness is
  `TestOneLineSaysWhatAFoldHoldsAndNothingItStandsFor`. `abridged(width,
  ladder...)` is `fit` for an index of keys: it marks a rung that dropped a
  binding with `…` and never returns the empty string, closing D58(o). The two share `oneLine`, so a bit's text
  cannot read differently depending on which one draws it, only how much of
  it fits. Below the width `said` can hold, an unfinished bit's mark
  degrades word → bare dash — see Open debt. The four
  floor numbers this doc comment used to restate (D42) went stale by two
  generations without anything noticing; they now live in exactly one place,
  `TestTheRowsMarkFloorsAreWhereTheyWereMeasured` (`tui/tui_test.go`), and
  the comment points at the test instead of repeating the numbers.
  `recall` no longer preallocates from `memory.Compaction.Count()`. Once a
  compaction can arrive from a persisted record, `Count` is a number in a file
  this program did not write, and sizing a `make` from it turned a damaged
  record into `fatal error: out of memory` — a death with no panic, no defers
  and no receipt. Found by review, executed. `memory`'s decoder now refuses a
  compaction whose count disagrees with its receipt, which closes the same hole
  from the other side; both are wanted, because each holds whatever the other
  does. No test reaches this from `tui/` — with the decoder's check in place an
  adversarial `Count` is unconstructible from outside `memory`, so this half is
  defence in depth and is deliberately unpinned rather than pinned by a test
  that reaches into another package's internals to fabricate one.
- `tui/ranked.go` — D3's surface on the TUI side: `ctrl+t` swaps the
  transcript for a second reading of the record, ordered by what has been
  voted on rather than by time. `Model.judged()` builds the view it ranks
  over — **everything anybody said, named once, newest first, plus anything
  else somebody voted on** — read over the *record* rather than the
  transcript, which is the whole difference between retrieval and theatre:
  most rows here are behind a scar and unreachable from the transcript at
  all. It listed only the voted bits until this checkpoint; on this
  project's own record that drew three rows out of twenty-nine said bits,
  with a claim later found unsourced at rank one and the correction to it
  absent from the screen, while `cmd/tldr top` over the same record printed
  the correction and said `kept 3 · not judged 26`. Two exclusions remain
  and are the same two `top`'s `reading()` makes — a ballot is not a row, a
  fold is not a row — except that a fold somebody *voted on* keeps its row,
  because the caret can be parked on one. `Model.ranking()` hands that view
  to `memory.View.Rank` (D30); the file's own doc comment states the honest
  limit rather than letting the surface imply more than it does — with one
  human's ±1 as the only signal this ranks what somebody has already read
  and marked and merely lists the rest, which `band()`'s middle heading
  (`not judged · 26`, unreachable before the widening) says on the frame.
  It does not discharge D3 by itself. `stands()` is the caret's rescue at
  draw time: an address the ranked view no longer holds directly is
  resolved onto whatever scar absorbed it, walking both `Prev` (D13) and a
  compaction's flattened `Absorbed` set, so a caret parked here when a fold
  fires on the transcript still has somewhere to be drawn once the surfaces
  swap back. The draw function itself lays out a gutter and an ordinal on
  every row — the receipt's row vocabulary rather than the transcript's —
  headed by `band()` headings that group rows by the reader's own standing
  vote, kept above let go, with `gutterCell()` drawing the caret and the
  scar's own rule in the margin the transcript spends on the fade instead.
  Built (D49), extended so the caret's own row
  draws whole rather than cut at the margin, the same shape `render.go`'s
  `saidWhole` gives the transcript (D53(g), D54(b)). **This file is under
  active work this checkpoint** — its column arithmetic and exact widths
  are deliberately not restated here; the doc comment beside each function
  is the current source, not this entry.
- `tui/ranked_test.go` — the ranked surface's tests, reached through the
  same `press()` helper `ask_test.go` and `tui_test.go` use, so what is
  exercised is the key rather than the method behind it.
  `TestTheRankedListIsEverythingSaidPlusAnythingElseJudged` records in its
  own comment that its first version asked `Model.list()` on the wrong
  surface and reported rows nobody had voted on; it was
  `TestTheRankedListIsExactlyWhatWasJudged` until `judged()` widened.
  `TestAnUnvotedRankedListSpendsNothingOnTheVoteColumn` pins the case the
  widening created — a record with no votes in it used to draw an empty
  list, and now draws every row, so a vote column that says nothing on any
  row would cost columns on every one. It pins measured handle columns at
  two widths rather than comparing the voted and unvoted frames, because
  the two ladders shed the address at different widths and the comparison
  is false across that step.
  `TestTheRankedListDoesNotMoveWhenAFoldFires` and
  `TestTheCaretComesBackFromTheRankedViewOntoTheScarThatAbsorbedItsBit` are
  this surface's half of the two-screen, one-caret problem `stands()`
  solves: a fold firing while the ranked view is up moves nothing on this
  screen, and the caret re-attaches onto the absorbing scar once the
  transcript is back. `TestAVotedScarIsARowRatherThanAnUnrenderedPayload`
  is the regression this surface introduced: before it existed nothing drew
  a bit by address, so a fold's default rendering branch had never run in
  anger and would have printed a bare Go type. `readingRanked` is the
  shared fixture for the caret's-row-drawn-whole tests
  (`TestTheRankedCaretsRowShowsEveryWordAndEveryOtherRowIsCut`,
  `TestARankedBlockIsQuotedInTheGutterAndCountsForNothing`,
  `TestARankedRowOpensExactlyWhenItsOwnRowCannotHoldTheMessage`), and its
  own comment explains why it does not assert where in the list the answer
  lands: that ordering is `memory.View.Rank`'s claim, held by memory's own
  tests, and a fixture that leaned on it would make these tests a second,
  weaker witness for someone else's property — which is exactly what
  happened once, reported by `cmd/seam` as four ranked tests going red
  under a mutation to `memory/rank.go`. `zz_probe_test.go`, in the same
  package, is a separate, explicitly throwaway probe (guarded by
  `PROBE=1`) over the same rows and is not part of this suite.
- `tui/tui_test.go` — the transcript's tests. Added under D42:
  `TestALoneJoggedRowIsAlwaysBesideAScar` sweeps fold lengths 2–60 asserting
  a single-row jog is always drawn against a scar at column 0, not on the
  run-of-one tautology the first version of this work wrongly relied on;
  `TestWideningTheTerminalNeverTakesAnythingOffARow` sweeps widths 1–130 on
  three fixtures asserting the handle column never falls and the sentence
  column falls at most a pinned number of times (0 for an unvoted fixture, 1
  for a voted one — the older, pre-existing fall, left standing and pinned
  by count). Both were built against a mutation table now in
  `.claude/craft/tui-design-engineer.md` and re-derivable via
  `cmd/seam` (D45): seven mutations across three checks — this file's two
  plus `TestTheFadeIsDrawnInSpaceAndNotOnlyInColour`, which is defined at
  `tui/tui_test.go:471`, not in `tui/harness_test.go` as an earlier version
  of this line said (`harness_test.go` only references it in comments) —
  every mutation caught, every check the sole catcher of at least one.
  D45's own catalog cites by test *name*, so it catches a rename;
  it structurally cannot catch a wrong file path in prose, which is what
  this correction was.
- `tui/harness_test.go` — prints real rendered frames at chosen sizes under
  `HARNESS=1`; most tests here assert nothing, by design. `screen`/`profiled`
  now push every frame through `colorprofile.Writer` at a chosen
  `colorprofile.Profile` before printing — the same downgrade path Bubble
  Tea's own renderer uses — replacing a literal grep for `38;5;242m` that
  read pre-downgrade output and could not fail (D27 instance two, closed
  partway by D39(f)). The margin column (`colours()`) reports every SGR
  colour a row carries, in order, not only the first; rewriting it caught a
  stale comment in `tui/unfold.go`'s `cell()` claiming a fade it never
  actually performed. `TestHarnessFloors` gained a `"transcript, cooling"`
  row (D42) — a going row begins `step` columns further left, so its dash
  floor measures lower (7) than a staying transcript row's (9);
  `TestHarnessFade` prints the frame the fade design was decided on, in
  colour and through `colorprofile.NoTTY`, and names in comment two things a
  person has to look at rather than a test can assert — including the caret
  sitting hard against the vote mark on a going row. `TestHarnessHoldSchedule`
  prints a hold-vs-vote-rate table at three cadences for `const bits = 400`,
  and reads the simulator that now lives next door in `tui/strand_test.go` —
  this file prints and that one asserts, off one simulator, because two would
  be D36's own defect.
- `tui/strand_test.go` — **the deterministic simulator and the frozen sweep it
  produces**, and the answer to the debt the entry above used to carry.
  `simulate` takes a `schedule` value rather than four positional arguments, so
  the budget, the cut, *where the upvote lands* and — added when the table was
  asked to settle an argument about a fold count and could not — *how long the
  conversation ran* are all parameters; `strand` counts, per frame, a said bit
  still on screen naming a `Prev` the view has let go, three ways (anyone's
  vote, the human's held ones, and the held ones with no scar above them to walk
  back through). `TestTheStrandingSweepReproducesItsFrozenTable` compares 270
  schedules against `tui/testdata/stranding.txt` and rewrites it under
  `-update`; `TestTheStrandingSweepCanReportEitherAnswer` is the null
  hypothesis, and each of its branches was stubbed red before the table's green
  was believed — the four original ones, and the two that say the length axis
  changes an answer, which were shown red three ways. **Not behind `HARNESS`**,
  unlike everything in `harness_test.go`: a frame dump's output is taste and a
  golden count is not, and the three published stranding figures that nothing
  could re-derive are what an ungated instrument costs. Runs the grid
  concurrently and once per binary (`sync.OnceValue`): 9.6s under `-race` for
  the two tests that read it, and the whole `-race` suite went 52.7s to 53.4s
  when the length axis doubled the grid, because the sweep overlaps the rest of
  the package rather than adding to it.
- `tui/testdata/stranding.txt` — the frozen sweep. Its header names every
  column and says which cells are deliberately blank. It is the only place in
  this repository where a stranding figure is re-derivable rather than quoted,
  and `memory/view.go`'s `sparing` now points at it instead of restating it.
- `memory/race_test.go` — four tests contending the store from many goroutines
  under `-race`: identical content raced onto one address, `Get` against live
  `Put`s, concurrent folds asserted to produce exactly what one sequential run
  does, and `View.Add`'s capped append shown to be what stops two goroutines
  growing into each other's slot. Removing `Store`'s locking fails all four;
  removing the cap fails only the fourth
  (`TestConcurrentAddDoesNotShareAViewsSpareCapacity`). The rest of the suite
  passes green under either mutation, which is what makes these the tests
  that hold the claims up.
- `cmd/tldr/record.go` — the program's side of persistence, and it was missing
  from this file for a whole unit: that commit landed it and did not add an
  entry here, which is the staleness D52(f) is about, committed by the same
  hand that wrote D52(f) down. One file holds three concatenated streams — the
  record, the transcript view, the vote view, in that order — because three
  files would admit a state one cannot: a record present and a view absent,
  which silently lifts every hold. `recordPath()` is `$TLDR_RECORD`, then
  `$XDG_STATE_HOME`, then `~/.local/state`; an *empty* env var is treated as
  unset, because `filepath.Join` drops the empty element and would file the
  record at a relative path. `atomically()` writes a temp file in the target's
  own directory and renames, so a save that fails leaves the previous record
  untouched. `load()` treats only a missing file as a first run and refuses
  everything else by name, and `check()` restates two rules `memory` enforces
  by panicking so a bad file is an error with a path rather than a panic on
  the first frame.
- `cmd/tldr/record.go`'s `absorb()` — **a save reads the file before it replaces
  it**, and files any bit the writer lacks. Without it the program's two writers
  — a session checkpointing after every change, and `tldr say` from outside one —
  erase each other outright: a bit *absent from the store*, not merely off a
  screen, with no error and no receipt, which is D1 failing inside the binary
  whose thesis it is. It costs one read per save and no new format, because the
  store is content-addressed: two writers cannot produce contents that disagree,
  so the union is `Put` in a loop and identical bits collapse. The store only —
  **a view is allowed to forget; the record is not** — so a bit said beside an
  open session reaches the record and never interrupts the transcript somebody is
  reading. Two things stated where they happen: this is not a lock (the window
  narrows from a whole session to the milliseconds between the read and the
  rename, and closing it needs a lock file that outlives a killed process), and a
  file this build cannot parse stops the save rather than being overwritten by it.
  Claim `record-a-save-that-does-not-erase`, three cited checks, `sole`. This was
  disclosed as an accepted lost update in `say.go` before review named the
  argument for accepting it a false choice. *This entry, and that doc comment,
  claimed the bit also reached the next session's screen; it did not, and
  `rejoin()` below is what makes it true.*
- `cmd/tldr/record.go`'s `rejoin()` — **a load puts back into the transcript
  every utterance the record holds that the transcript does not account for**, and
  it closes a defect that was one command with two outcomes. `absorb()` merges the
  store and never the views, correctly; but the session's checkpoint then writes
  its own `shown` over the file, so a bit written by `tldr say` beside an open
  session landed in the store and in *no view at all* — permanently, not until the
  next session. With nothing else running the identical command put it in the next
  session's transcript, fold window and persona context. The selector was whether a
  terminal happened to be open elsewhere on the machine, which is invisible to
  everybody. Not a D1 or D14 failure — `tldr top` and `ctrl+t` both walk
  `Store.All()`, so it stayed reachable — but the transcript disagreed with itself
  about one act and said so nowhere. Accounted-for means *named by the view, or
  absorbed by a scar in it*; `Compaction.Absorbed()` merges across generations
  (`memory/cool.go`), so a bit folded three times over is still covered, and
  utterances are the only kind that can strand (a ballot is in the vote view, a
  compaction is minted straight into a view) — the same two exclusions
  `tui/ranked.go`'s `judged` and `top.go`'s `reading` make. Strays are **merged by
  instant, not appended**: appending would put an hour-old note below everything
  said since, and `tui.Load` lands the caret on the last row. A bit the view
  already named is never moved. At load rather than in `absorb()` because a save
  may not write a view the surface does not hold, or `tui/save.go`'s sentence stops
  being true — and because this is a property of a record rather than an agreement
  between the two writers this program happens to ship. Claim
  `a-transcript-that-drops-a-stray`, no `sole`: dropping the `Absorbed` walk
  un-folds every conversation and reddens four checks well outside this rule.
- `cmd/tldr/record.go`'s `checkpoint()` — **the file is now level with memory
  continuously, not at quit.** It returns a `tui.Save` bound to one path, and
  `tui.Load` takes it. Saving at exit made the whole promise conditional on a
  clean one, and said so nowhere: a crash or a `kill -9` took the session with
  no receipt, while `atomically` guarded the *previous* record and nothing
  guarded the current one. The doc comment carries the measured cost — whole
  file per change, so quadratic in a session's length; 4,421 bytes and 1.3 ms
  at 12 bits, 116,873 and 3.8 ms at 343 — and says why the fix is not an
  append-only format. `main.go`'s exit 2 changed meaning with it: a panic no
  longer means the session is gone, only that the file may be one change
  short.
- `cmd/tldr/record_test.go` — the file format's own tests, one layer below
  `save_test.go`'s. `fixture(t)` builds a `record` with a fold and votes on
  both a bit that survived it and one that didn't, because a record of bare
  utterances exercises one payload and the arrangement this file exists to
  keep straight — two views disagreeing about which bits they name — needs a
  scar. `TestASavedRecordComesBackWhole` asserts the two views with
  `slices.Equal` rather than as sets, and states explicitly why: nothing in
  the file labels which view is which, so a swap would pass a set comparison
  and this is the test that rules it out. `TestASavedRecordIsPrivateToItsOwner`
  pins the file at 0600 and its directory at 0700.
  `TestNoFileIsAnEmptyRecordAndNotAnError` checks a first run twice over —
  that it starts empty, and that reading it does not create the file, since a
  program that files an empty record on every start makes "has this ever run
  before" unanswerable. `TestEveryTruncationIsFatalAndNamesTheFile` sweeps
  every prefix of a real encoded record rather than one truncation, because
  the three streams are self-delimiting and each ends at a different kind of
  boundary — a length, a count, a closing tag — so one cut only ever tests
  one of them. `TestNoSingleBitOfTheFileCanBeChangedQuietly` is the same
  exhaustive-bit-flip discipline `memory/wire_test.go` uses, done in memory
  rather than on disk because every offset costs a parse. `TestAViewFromAnotherRecordIsFatal`,
  `TestAVoteViewOfSomethingElseIsFatal` and `TestAViewNamingAMissingBitIsFatal`
  are the three ways a hand-assembled file can lie about what a view names,
  each refused here — rather than inside `memory` panicking on the first
  frame the surface draws — with the first also checking the stale view is
  not reachable through `errors.As`, so the surface cannot recover it by
  accident. `failAfter` behaves like a disk with nothing left on it — a short
  write and an error, the way `os.File` actually fails — and
  `TestAFailedSaveLeavesThePreviousRecordIntact` cuts at three different
  points inside the stream plus a control row where nothing fails, because
  without that control row the test would pass against an `atomically` that
  never wrote anything at all — named in its own comment as a shape this
  project has shipped twice before (D27, D48).
  `TestTheRecordSurvivesASaveThatFailed` is the load-back half of the same
  guarantee. `TestRecordPath` covers the three-way precedence
  (`$TLDR_RECORD`, then `$XDG_STATE_HOME`, then the default under home) and
  the case measured rather than assumed: an empty `$XDG_STATE_HOME` is not a
  relative path, because `filepath.Join` silently drops the empty element —
  the test's own comment says this was written down wrong until it was run
  against the version that gets it right.
- `cmd/tldr/cli.go`, `say.go`, `top.go` — the record's second mouth, and it is
  not a screen (D51(e)). `tldr` with no arguments is the surface, exactly as
  before; any argument at all is a non-interactive verb and never opens a
  terminal, which is what keeps `tui`'s `defaultPersona` argument standing (there
  is still nowhere on the path that opens a session to choose a model). `say`
  appends one ordinary bit — same channel, same view, same edge back to what it
  follows as anything typed at the keyboard — and prints its address on stdout,
  everything else on stderr. `top` prints a ranked reading over the *whole
  record* rather than over the transcript, ordered by `memory.View.Rank` for
  `tui.Human()`. **Writing is open here and voting is not**: there is no `vote`
  verb, the reason is D4/D30/D39(a)/D51(d) with D52(j)'s ruling in those words
  ("the skill gets Claude a write, never a vote"), and the absence is held by
  `TestNoCommandOnThisSurfaceCanCastAVote`, which walks the command table rather
  than naming the verbs — plus claim `a-write-path-that-also-votes`, whose
  mutation *adds* the feature, because an absence has no line to break. Three
  things stated in the code rather than left to be found: the channel is
  `tui.Channel()` and not a name of its own, since `memory.Cool` panics on a
  window spanning two channels and the bit goes into the view the surface folds;
  the guarantee reaches identity in exactly one place, the human's own ref (below);
  and a bit said beside an open session reaches the
  record and the *next* session's screen but never that session's, which is
  `absorb()` and `rejoin()` above and D1's own division. `say` prompts on stderr when standard input is a terminal and it was
  given no text, because otherwise the likeliest first invocation anybody types
  looks like a hung program. **`say` refuses a `-as` naming the person at the
  keyboard** (`tui.Human().Ref`, never a copied literal) — the write-yes/vote-no
  line once more rather than a second rule, since that ref is what `View.Rank`
  takes as `by` and what an audit of the record would be about. It stops the exact
  handle and no lookalike, and it is not authentication: anyone who can run the
  command can write the file. Claim `say-speaking-as-the-human`; *this entry said
  the opposite until the refusal was ruled, that `say` would spell any handle
  including the human's.* `top`'s speaker column is `display (ref)` when those
  differ, and always when the display is the human's own — "a wrong attribution is
  a lie a reader can catch by reading it" is only true where the reader is shown
  the field the record keys on, and a bare `me` said less about itself than the
  human's `me (local)` did (claim `speaker-drops-the-humans-name`). Its header counts **ballots and standing votes separately**,
  and names any standing vote of the reader's own that landed on something the
  reading has no row for — a vote on a scar. It printed ballots alone, beside
  bands counting standing votes on rows, and the two disagreed in both directions
  that flatter us.
- `cmd/tldr/cli_test.go`, `top_test.go` — the verbs' tests, run against buffers
  through the `streams` value the commands take rather than against os.Stdout.
  `TestABitSaidFromTheCommandLineFoldsLikeAnyOther` is the one that cannot be
  checked by reading the record back: it drives the real surface until a fold
  fires and looks for the command-line bit inside the scar, which is the channel
  agreement stated as behaviour instead of as a constant. `top_test.go`
  deliberately asserts **no ordering** — that is `memory.View.Rank`'s claim, held
  by memory's own tests, and restating it here would make these a second, weaker
  witness for somebody else's property (the mistake `tui/ranked_test.go` records
  making once). What it asserts instead is what this command owns: which bits are
  in the reading, the mark each row carries, and the header's three band counts.
  Measured: the two rank claims in `docs/CLAIMS.md` stay unaffected by this
  package, and `record-frame-unclosed` gained nine names from this unit, then
  three more from the review that followed it — twelve of its twenty-six, as its
  own prose predicted.
  `TestTheWriterThatSavesSecondKeepsTheOthersBits` and
  `TestASaveWillNotReplaceARecordItCannotRead` live in `record_test.go` and hold
  `absorb()`: two writers over one file in both orders, every bit still on the
  record, and the other writer's bit deliberately **not** in the second writer's
  *running* view, which is the assertion that keeps the fix from growing into
  "merge the views too". *That assertion used to be made against the record
  reloaded from the file instead, where it read as a requirement that the other
  writer's bit be in no transcript ever — a check enforcing a defect (D52(c)), and
  the third instance of that shape. It now asserts on the live view, and the
  reload asserts the opposite beside it.*
  `TestSayingBesideAnOpenSessionReachesTheNextOneJustTheSame` is the property
  `rejoin()` exists for, stated as behaviour rather than as a rule: the same `say`
  reaches the next session's transcript whether or not one was open while it ran.
  Its session row asserts the stranding is actually reached before asserting it is
  repaired, so the check cannot pass on a fixture that never produced the case.
  `TestALoadPutsAStrayBackWhereItWasSaidAndMovesNothingElse` is `rejoin()`'s own
  table, hand-built in memory — which is also what keeps it off
  `record-frame-unclosed`, since a test that opens no file cannot trip the wire
  format's claims.
- `cmd/tldr/save_test.go` — drives the real surface with the real save path
  against a real file, asserting after every message that the file *read back*
  is the record the Model holds; a bit landing, a vote and a fold each get a
  step, and the fold is asserted to have happened so a change to `coolFloor`
  (D58(b) — `coolAt` no longer exists) fails loudly instead of quietly
  weakening the sequence. Also: a change whose save
  failed is carried by the next one that works (through `atomically` and a disk
  that stops taking bytes, not a stub), and `TestHarnessAKilledSessionKeepsItsBits`,
  which builds the binary, runs it under a pty via `script -qe`, sends
  `SIGKILL` mid-session and reopens the record. That last one is `HARNESS`-only
  — it wants a terminal and util-linux — and it is the check the whole unit
  exists for.
- `tui/save.go` — the invariant, stated once: *after any change to the store or
  to either view, the file matches memory.* `Update` wraps `Model.update` and
  compares a `checkpoint` — the store pointer, its bit count, and the two views
  — taken before and after the message. A dirty flag at each mutation site was
  the alternative and is wrong the day a sixth site appears without knowing the
  flag exists; comparing state has no list to fall off. The bit count stands in
  for the record's contents only because `memory.Store` never shrinks, which is
  the one sentence to re-read before adding a delete. A failed save raises
  `notice.unsaved` and does not end the session, so a full disk costs the file
  and not the conversation; a save that gets through clears it, because a
  warning that has stopped being true should not need dismissing.
- `tui/save_test.go` — the continuous-save invariant's tests, one layer up
  from `cmd/tldr/save_test.go`: this file drives `Model.Update` against an
  injected `Save` hook (`saver`) rather than a real file, so it can reach
  states the real key sequence cannot — a change with no key bound to it
  yet, a swapped record — without needing a disk.
  `TestACheckpointNoticesEachThingASaveWrites` hand-builds `checkpoint`
  values row by row, one term at a time, because a test that only pressed
  keys would leave the unreachable terms unfalsifiable — exactly the state
  where a term gets deleted as dead weight and the invariant quietly
  narrows; its last row is the control, asserting two identical checkpoints
  compare equal, since without it every row above would pass against a
  `same()` that always answers false.
  `TestEveryChangeToTheRecordIsSavedAndNothingElseIs` drives a full
  sequence of keys and messages — resize, typing, the caret moving, both
  surfaces swapping, votes, a fold, a reply, a failed request — asserting
  the hook fires on exactly the ones that touch the store or either view,
  and that what it is handed is what the model then holds rather than a
  stale copy from before the change. `TestAChangeNoBranchMadeIsStillWritten`
  states the mechanism's honest limit as a test: it proves a mutation the
  save path has never heard of is still noticed, not that every future
  caller outside this package saves, since `Model.update` is unexported and
  `Model.Update` is the only way in.
  `TestAFailedSaveKeepsTheSessionAndSaysSo` and
  `TestASaveThatGetsThroughClearsTheNoticeTheLastOneRaised` cover the
  failure surface: a save that fails keeps the bits, keeps the surface up,
  and raises `notice.unsaved` rather than ending the session, and a save
  that later succeeds takes that notice down again — while
  `TestASaveDoesNotClearSomebodyElsesFailure` checks the two notices
  sharing one field do not paper over each other.
- `persona/persona.go` — the package doc states the load-bearing claim: a
  persona is not a model, since two personas can sit on one set of weights
  and be two different participants in the record, so `Persona.Handle()`
  names the persona and not the weights alone (`"ollama/"+Model` as the ref,
  the persona's own `Name` as the display — `memory.Handle`'s own split,
  used as intended). `System` and `Temperature` are deliberately *not* part
  of that handle: two personas differing only in instruction are one
  participant as far as the record is concerned, a claim the package doc
  flags as worth noticing if it turns out wrong. `DefaultModel` is
  `qwen3.5:latest`, and its doc comment is where the reasoning-model leak
  this package accepts rather than fixes is argued — see `client.go`.
  `Role`/`Turn` are the wire-facing vocabulary, deliberately not
  `memory.Bit`: a bit is a recorded occurrence with an address, a turn is
  what gets sent this time, and which bits become turns is the caller's
  decision against its own view of the record, never this package's.
- `persona/client.go` — `Client.Reply`, the only thing that talks to ollama,
  non-streaming. `chatRequest`/`chatReply`/`wireMsg` are kept apart from
  `Turn` on purpose, so a field added to `Turn` cannot silently start being
  sent and a field ollama adds cannot silently become part of this
  package's vocabulary. `Answer{Text, Truncated}` exists because
  `done_reason: "length"` has to reach the caller — a reply cut off by the
  context window is a well-formed sentence that simply stops, and storing it
  as a finished thought would be a silent, permanent falsehood in an
  append-only record. `reply.Message.Thinking` is read and never
  concatenated onto the answer, by design stated in its own doc comment: a
  model's scratchpad is not something it said. `usable()` checks the base
  URL can address an HTTP server before anything is sent, so a bad address
  fails with an actionable message rather than one indistinguishable from a
  stopped server. `calledOff()` tells a caller's own cancel apart from the
  caller's own deadline, matched by behaviour rather than by type where it
  has to be (`interface{ Timeout() bool }`, since one `http.Client` timeout
  arrives wrapped in a `*url.Error` and the other bare). `sendFailed()`
  gives a DNS failure its own message rather than "start ollama," which
  would be the wrong advice for it. `ollamaSays()`/`aboutTheModel()`
  disambiguate a 404 that means "the model is not pulled" from one that
  means "this is not ollama," by reading the body's own words rather than
  trusting the status code alone — verified against a running ollama 0.17.7
  rather than assumed. `because()`/`isReply()`/`peek` bound and flatten an
  unrecognized refusal body for the terminal, cutting by rune rather than by
  byte, and refuse to apply that fallback to a body that is itself a
  generation — the one place in this package a reasoning model's scratchpad
  can still reach a person, stated in the doc comment as the smaller leak
  rather than a closed one.
- `persona/boundary.go` — `Escape`, and D1's split applied to the wire: the
  record keeps what was said, control tokens and all, and what is transmitted
  is derived from it. ollama renders a message list into the model's own chat
  template and tokenizes the result with special tokens parsed wherever they
  sit, content included, and `/api/chat` has no field that says otherwise — so
  a recorded `<|im_start|>` spells a role boundary the conversation never had.
  Measured, not assumed: on ollama 0.17.7 a genuine five-message conversation
  and a three-message one carrying the other two inside an assistant turn's
  content produce the same prompt token count and the same reply at temperature
  0. The rule is a backslash after a marker's opening bracket — every character
  survives, so the escape is legible rather than silent — over three bracket
  *shapes* rather than a token list, because each family parses only its own
  vocabulary (`marker`, `bracketed`). Applied once, in `Client.Reply`, to every
  turn and to the persona's `System`; never on the way in, and never to a bit.
- `persona/errors.go` — `Error{Kind, Problem, Fix, Err}` and the `Kind` enum
  (`Unusable`, `Unreachable`, `Timeout`, `Canceled`, `NoModel`, `Rejected`,
  `Garbled`) — one member for every failure a caller could plausibly act on
  differently, and no finer than that. Deliberately breaks the Go convention
  of lowercase, unpunctuated error strings, on the stated grounds that these
  are terminal messages rather than components some library will wrap —
  argued in the doc comment as the project's own legibility thesis applied
  to an error type. `Err` carries the underlying cause for
  `errors.Is`/`errors.As` but is never folded into the printed message.
- `persona/persona_test.go` — `TestHandleNamesTheWeightsAndTheVoice`: two
  personas on one model produce two distinct handles, and the same persona
  under a different `System` produces the same handle, pinning that the
  instruction is not part of identity.
- `persona/client_test.go` — the client's tests, built against `httptest`
  servers so nothing here reaches beyond loopback and no test depends on a
  real ollama. `TestReplyReturnsWhatTheModelSaid` covers a plain answer,
  chat-template padding, a reasoning model's scratchpad correctly dropped,
  and a truncated reply correctly flagged. `TestReplySendsTheConversation`
  pins what actually goes on the wire — the system prompt first, `stream`
  sent as an explicit `false` (a missing key defaults to ollama's own
  streaming), and turns in order, since a chat model reads position as time.
  `TestReplyFailures` is the large table behind every `Kind` in
  `errors.go`, including the two 404 cases that must read differently
  (missing model vs. wrong route) and a `"done": false` body read as a piece
  of a stream rather than a short answer. `TestReplyKeepsInlineReasoningInTheAnswer`
  pins the accepted leak `DefaultModel`'s doc comment argues for, and
  `TestARefusalDoesNotQuoteTheModelsReasoning` checks the one place that leak
  is closed. `TestARefusalQuotesAStrangeBodyBoundedAndWhole` checks the
  `peek` cut lands on a rune boundary rather than mid-character.
  `TestReplyWhenOllamaIsNotRunning` and `TestReplyWhenTheModelIsNotPulled`
  assert the exact strings a new user hits first, on the grounds that the
  wording is this package's real product surface.
  `TestReplyStopsWhenTheContextDoes`/`...MidBody` cover cancellation before
  and after the response headers arrive, the latter because the persona
  loop cancels routinely and an earlier version of this code reported that
  case as the server having failed. `TestReplyGivesUpOnASilentServer` and
  `TestReplyWhenTheHostDoesNotResolve` give a client timeout and a DNS
  failure their own distinct messages. `TestReplyRefusesWhatItCannotSend`
  checks a missing model and a bad base URL are caught before any request
  goes out. `TestZeroClientPointsAtALocalOllama` pins the zero-value
  defaults.
- `persona/boundary_test.go` — the escape's tests. `recorded` is the real
  fixture, the tail of the qwen3.5 reply that put control tokens on the live
  record on 2026-08-14, read back out of it rather than imagined.
  `TestEscapeBreaksEveryMarkerShapeItKnows` is the table — chatml, llama3,
  mistral and sentencepiece forgeries broken, and the shapes that must be left
  alone (`[D56]`, `[record]`, `[Client]`, `</div>`) untouched.
  `TestEscapeKeepsEveryCharacterOfTheOriginal` asserts the other half of the
  design over the same corpus: removing the inserted backslashes returns the
  input exactly, which is what makes the transformation legible instead of
  merely safe. `TestTheWireCarriesNoBoundaryTheRecordHeld` asserts on the bytes
  the server received, because the function working and the function being on
  the outbound path are different facts.
  `TestTheStandingInstructionIsEscapedToo` covers the `System`, and
  `TestReplyDoesNotChangeTheTurnsItWasGiven` covers the direction that would
  invert D1 — the derived form must never be written back through the caller's
  slice into what the surface holds and saves.
- `.github/workflows/commit-gate.yml` — runs `.githooks/pre-commit` verbatim
  on GitHub's own machine, on every push and pull request against `main`, so
  the checks are stated once and CI cannot drift from what the local hook
  enforces.
