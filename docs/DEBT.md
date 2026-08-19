# Open debt

- **A fold can still open the view on an answer whose question it took, once a
  message can be half a screen — and closing it needs a ruling in `memory/`.**
  The budget counts rows now (`Model.rows`), which is what the unit mismatch
  needed; the residual is that D58's move of the cut back to the last thing the
  human said cannot always happen any more. The cause is structural rather than a
  bug: a record with documents in it is large in *rows* and small in *bits*, D32
  requires the fold's window to be at least two bits, and those two together
  leave `keepFrom` no range to move the cut in. Measured at 100x30 with a fenced
  36-row answer every sixth bit: **18 of 38 folds** open on an answer whose
  question has gone, against 0 before; at every other bit, 58 of 116. It is a
  legibility defect and not a reachability one — the scar directly above is the
  receipt and `ctrl+u` opens it, which is D58(g)'s own ruling on the same shape.

  **What would close it is D32's size rule, which is not this seat's.** D32
  refuses to cool a run of one on the grounds that folding a single bit into a
  scar standing for one bit buys nothing. That stops being true when the one bit
  is half a screen: replacing eleven rows with one is exactly what the fold is
  for. Allowing a run of one *when the run costs more than a row* would give
  `keepFrom` its range back. It reaches a content address and belongs to
  `principal-go-engineer`.

  Two figures that are not residuals and are recorded so nobody re-derives them:
  the fold rate rises with the share of the conversation that is documents — 22
  folds in 300 writes with none, 53 at one in eleven, 296 at one in two — and the
  scars merge rather than pile up, so a high rate produces one receipt standing
  for many bits rather than many small ones (measured: 37 bits behind one scar
  after 40 writes at one document in two).
- **The segmenter cannot reach the fold's word bag, so a program's tokens are
  still counted into it.** D72(a) says a fenced region is segmented for "layout,
  budget, quoting and the word index"; three of those are `tui/`'s and the
  fourth is not. `Compaction.Bag` is built by `memory/cool.go` and reaches
  `ID(cold)`, so excluding fenced words at the source re-addresses every scar on
  disk. What shipped instead is `topWords` skipping tokens of fewer than three
  characters (ruled this session), which removes the loud half — measured on a
  40-bit fold at 100x30, one fenced reply put `j`, `s` and `1` in the three most
  prominent of the persona's twelve slots — and leaves the quiet half:
  `func`, `int`, `len`, `err` are words by that rule and are not what the window
  was about. Re-check: build a ten-bit window with and without a fenced reply
  and print `topWords(c.Bag(), personaWords)` for each.
- **A quotation can still contain a quotation mark, when the mark is in prose.**
  `opening` refuses a message that starts with a fence, which is where the
  problem was acute — `import "fmt"` closed the quotation four words early — but
  an ordinary paragraph with a quoted phrase in it still puts one between the
  marks. Left open on the grounds that it reads as nested speech rather than as
  a broken assertion, and that no rule which removed it could leave the words
  verbatim, which is the whole contract of that row.

- **`scope-adversary`'s insulation from the commercial thesis is notional,
  and three entries reason from it as though it holds** (D72(b)). D53(e)
  narrowed the insulation to "must ask when a brief entangles scope and
  go-to-market"; D47 requires every brief to tell the seat to read `CLAUDE.md`
  from disk, and `CLAUDE.md` carries the Business section. So the seat has held
  the thesis on every compliant dispatch since D47. It disclosed this itself
  rather than using it quietly. Three options, none chosen: a redacted charter
  for this seat, moving Business out of `CLAUDE.md`, or retiring the insulation
  as unenforceable. **Until one is chosen, do not cite D17, D36(k) or D53(e)
  for the claim that this seat cannot see the thesis.** Re-check:
  `grep -n 'Business' CLAUDE.md` and `grep -c 'read.*from disk'
  .claude/agents/scope-adversary.md`.
- **The twenty-entry backlog this bullet used to track no longer exists to
  drain.** D78(c) replaced the per-entry ruling record with the one-file rule
  (`docs/PRIVATE.md` never leaves, everything else publishes) and dissolved
  the backlog rather than working through it — there is no longer a set of
  decision entries "awaiting a withholding ruling." The historical gaps in
  the public log's numbering are unchanged and remain announced in
  `docs/DECISIONS.md`'s "After D81" note (`docs/PUBLICATION.md`, where this
  used to live, is deleted per D81(c)). D70(g)'s rule —
  that a withheld entry needs an independent adversarial re-derivation, not
  just a read — still holds, but now applies only to whatever is still
  withheld under the one-file rule, which as of D78 is `docs/PRIVATE.md`
  content and nothing else.
- **`decode` accepts a file with a fourth view stream, drops it, and says
  nothing** (`cmd/tldr/record.go`). The three streams are self-delimiting so a
  reader that has taken its three simply stops; there is no framing question it
  could ask, which is `memory/wire.go:42-44`'s own point about a self-delimiting
  format not being able to detect slack. Reproduced: a record plus three views
  loads with a nil error, `store 12, shown 4, votes 2`, and the fourth stream is
  gone. Untouched by `rejoin()` and not fixed by it — but `rejoin()` did move the
  *symptom*, which is why this is written down now: a bit that only the dropped
  stream named used to be stranded in the store, and is now silently in the
  transcript instead. Neither state is an error and nothing notices either. The
  fix is a count or a closing tag over the whole file, which is a format change
  and a decision rather than a repair. Re-check: encode a fixture, append a
  fourth `View.WriteAgainst` to the same buffer, `decode` it, and read the error.
- The old Reddit-client thread (`cmd/tldr/model_test.go`, the M1 spec citing a
  never-written `docs/MILESTONES.md`) is **closed by deletion**, not
  reconciliation — CEO decision, the file specced a product that does not
  exist. `cmd/tldr/main.go` remains and just launches `tui.New()`.
- **CI exists; there is still no lint config.** `.githooks/pre-commit` has
  two local escape hatches — `git commit --no-verify` skips it outright, and
  `core.hooksPath` is untracked local config, so a fresh clone never runs it
  at all. `.github/workflows/commit-gate.yml` closes neither: it invokes
  `sh .githooks/pre-commit` directly rather than through git, on GitHub's own
  machine, so neither hatch exists there. What it still does not buy: it
  does not stop a bad commit being made, and it only fires on
  `push`/`pull_request` against `main` — a pushed feature branch with no
  open PR against `main` is exactly as unchecked as one never pushed at all.
  What changes is that `main` itself now tells on you.
- **A fragment is indistinguishable from a screen-cut row below 9 columns
  (transcript) and 13 columns (receipt).** `tui/unfold.go`'s `said()` marks a
  truncated utterance with a word ("unfinished"), degrading to a bare dash
  as width narrows; below those two floors the row a speaker's own cut
  produces and the row a terminal's own ellipsis produces are the same
  bytes. Measured under `HARNESS=1` at every width from 1 and pinned by
  `tui/tui_test.go`'s `TestTheRowsMarkFloorsAreWhereTheyWereMeasured`;
  conceded rather than fixed. Re-check:
  `HARNESS=1 go test ./tui/ -run TestHarnessFloors -v`. The floor was 8
  before this session and, before that, a stale 6 that never matched the
  code (the session-5 handoff carries that older figure and is immutable —
  left alone).
- **The unfinished mark is forgeable.** Nothing stops a participant typing
  `╌ unfinished ╌` into a message, and at some widths that row is
  byte-identical to a real fragment's marked row. Low realism as an attack,
  but it is the first place this surface's own harness vocabulary shares a
  row with participant content rather than owning a row of its own. The
  tests no longer depend on the string (`tui/tui_test.go`'s
  `TestSaidMarksAFragmentAndNothingElse` compares a bit against its own
  finished twin instead), so the forgery is a display ambiguity now, not a
  test hole.
- **D27 instance two, partly closed.** `tui/harness_test.go` used to grep
  `m.View().Content` for the literal `38;5;242m`, reading pre-downgrade
  output and reporting the fade present at every terminal size regardless of
  what a real terminal shows. Now goes through `colorprofile.Writer` at a
  chosen profile before inspecting a frame, the same downgrade path Bubble
  Tea's own renderer uses (D39). Closed for **emulation**; still open for
  **observation** — this still runs the downgrade a terminal *would* run,
  never a real terminal's own renderer. Closing that needs an actual
  terminal, which remains unbuilt.
- **Of two weaknesses `tui-design-engineer` named in its own work, one is
  fixed this session and one still stands.** Fixed: the drop ladder in
  `tui/unfold.go` shed the content-hash column first as width narrowed — the
  auditor's own instrument, first to go rather than last — and now sheds
  time first instead (D39). Still open: below eight rows the footer runs off
  the bottom of the terminal with nothing saying so — `layout()` clamps the
  viewport to `max(height - chrome, 1)`, so the frame is 8 rows however
  short the terminal gets, and overflow begins at height 7, confirmed this
  session under `HARNESS=1`'s `TestHarnessFits`.
- **`View` carries no synchronization, and must not grow any.** Closing the
  store's concurrency gap (`memory/race_test.go`) established what actually
  holds this package up, and it is worth stating before D18(e) builds on it:
  `Store` (`memory/store.go`, `mu` at line 64) is a pointer and locks; `View`
  (`memory/view.go`) is a *value*, and its whole safety is the capped append
  in `Add`. The one-record-many-views arrangement D18(e) describes is safe
  precisely because each holder gets its own value. The moment something
  wants a `*View` shared across goroutines, that property is gone.
- **The pre-commit gate (`.githooks/pre-commit`) runs against the working
  tree, not the index.** A partial `git add` can commit a snapshot that does
  not build while the gate — reading files on disk, not what is staged —
  reports green. Met and worked around this session by verifying the
  committed snapshot separately (`git archive HEAD` into a temp dir, then
  build and test there), but nothing in the hook itself enforces that; a
  future commit can still reintroduce the gap.
- **Six sites in `memory/*_test.go` share the vacuous `recover() == nil`
  shape, and only one of them (`vote_test.go`'s `TestCastPanics`) is closed.**
  `grep -rn "recover() == nil" memory/*_test.go` returns
  `cool_test.go:267`, `id_test.go:266`, `id_test.go:279`, `store_test.go:87`,
  `store_test.go:143`, `vote_test.go:133` — none of the six test names
  appears anywhere in `docs/CLAIMS.md` (checked by name, this checkpoint),
  so none is currently measured by a claim. `TestCoolPanics`
  (`cool_test.go:267`) is the cheapest next one: `memory/cool.go:180` carries
  a near-identical guard sentence to `memory/vote.go:126` (see the next
  item), so the same repair shape applies. Re-check:
  `grep -rn "recover() == nil" memory/*_test.go`. D48(g).
- **`Cast`'s and `Cool`'s unaddressed-target guards print only the target's
  `Handle.Ref`, which is empty exactly when the guard fires, so they name
  nothing.** `memory/vote.go:126` and `memory/cool.go:180` panic with the
  identical shape — `"memory: Cast/Cool on an unaddressed bit from %q; store
  it first"` — so both of `TestCastPanics`'s failing rows (the zero `Bit` and
  a built-but-unstored one) panic with byte-identical text. Proposed fix
  (CEO-ruled yes, not yet built): add `at %s carrying %T` to distinguish the
  zero `Bit` (`<nil>`) from a built-but-unstored `memory.Utterance`, plus a
  claim in `docs/CLAIMS.md` to hold it. D48(j).

The following are from a `tui-design-engineer` capture and read of seven real
100×30 frames from the live run behind the demo page (see above); judgement
on real frames, not measurements, and not yet fixed:

- **Immediately after a fold, every transcript row is drawn cooling**, in the
  captured demo frames — the old `fadeBefore` computation (since replaced by
  `absorbing()`, D39) equalled the view length in that frame — so the money
  frame had no bright/dim contrast anywhere. Not re-verified against the new
  computation.
- **`view N · record M` sits at the maximum possible distance from the scar
  it explains** — top-right corner, grey, while the scar is at the left edge.
  One idea rendered at opposite corners.
- ~~**The scar's word summary degrades at scale.**~~ **Closed for the scar,
  still open for the persona (D59(j)).** Over 7 bits it read "backfill before
  read deleted"; over 294 bits of the same migration it read **"understood s
  before migration"** — "understood" is the model's verbal tic opening dozens
  of replies, and "s" is a crumb from splitting `let's` on punctuation. The
  scar now quotes one absorbed bit instead (`frame.quoted`, `tui/render.go`),
  ranked by the reader's own standing votes and tie-broken by recency, so
  every word on that row is a word somebody said. **What is unchanged is
  `topWords` itself and the one caller left**: `foldNote` still sends the
  persona a twelve-word index off the same bag, with the same tics and the
  same crumbs — framed in the note as an index rather than a summary, which
  is the whole of the mitigation. `filler` still knows English function words
  and not assistant tics. Re-check: `grep -n "topWords(" tui/*.go | grep -v
  _test` returns two lines, the definition and its one caller. (The command
  written here first was `grep -n topWords tui/*.go`, which returns eighteen —
  the glob takes the test files with it. The claim was right and the command
  did not demonstrate it, which on this project is the same defect as a wrong
  claim.)
- **The two headline numbers do not reconcile on screen.** A steady-state
  frame reads `view 8 · record 343` over a scar claiming 294 bits and seven
  hot rows — 294 + 7 = 301, not 343. The missing 42 are compaction bits, 41
  of them themselves absorbed. Not false, but uncheckable from the screen, on
  a product whose claim is that its numbers can be checked.
- **Hot rows carry no time and no address; only folded rows do.** The most
  recent material has the least provenance on screen and only acquires an
  address and a clock after being folded away, which is backwards for an
  auditor. Putting both on every transcript row costs roughly fifteen columns
  and drops the text column from ~91 to ~65 at width 100 — a column-budget
  conflict between the developer's frame and the auditor's, unresolved, and
  probably a toggle.
- **Nothing shows what the model was given.** `turns()` replaces an absorbed
  window with a `foldNote` — a word bag and two instructions. That
  substitution is the most consequential thing this program does to an
  agent's behaviour and has no surface at all: the human sees the receipt,
  the model sees the bag, and the screen shows only the human's view.
  `tui/ask.go`'s own comment records a sweep where a small model fabricated a
  decision out of four words from such a bag.
- **An unfold receipt loses its header after one page.** The `┌─` line
  scrolls off, so on a 294-bit receipt thirteen of fourteen pages carry
  nothing saying which scar they belong to; the ordinal denominator is the
  only thread back.
- **Which weights answered is not recorded precisely.** The handle says
  `qwen3.5` while the persona holds `qwen3.5:latest`, a mutable tag. No
  digest, temperature or standing instruction reaches the screen or the bit.

**"The fade is colour-only" is closed for the transcript (D42).** A row the
next fold will take now steps two columns left into the margin the caret
reserves, so a cooling row is no longer byte-identical to a hot one once
colour is stripped. **What is not closed: a scar.** `transcript` draws every
scar in `seamInk` whether it is about to be absorbed by the next fold or
not, in either channel — and this is the steady state for a reader who
never votes, not an edge case: after any fold the view is a scar plus
`keepHot` bits, and the absorbing window always contains the head, so a
scar is *routinely* — measured at five `(coolAt, keepHot)` pairs, *always*
— part of what the next fold takes. A dashed-rule shape for a going scar
was drawn and rejected on that same measurement: a mark whose default state
is the one a reader never sees is unlearnable. `tui/tui.go`'s package doc
states both holes. See D42(c).

From wiring the vote in (D39), still open:

- **Conversation-time units are unstated on screen.** The drain beside a held
  bit's mark (`drain()`, `tui/render.go`) draws a bar fraction of `holdFor`
  and never states the unit or the number — a person can see a hold
  draining and cannot read off "45 seconds" or "2 minutes" from anything on
  screen.
- **The first vote reflows the transcript by five columns.** `f.voted()`
  reserves `markWidth+drainWidth+1 = 5` columns the instant any bit in the
  frame carries a vote, and every row shifts to make room — a side effect of
  casting the very first vote that nothing on screen announces.
- **An agent's votes are invisible.** `frame.standing` reads only the local
  handle's entry from `memory.Tally`; an agent's vote is tallied and never
  drawn, on a surface whose central object is now a vote mark.
- **`pgup`/`pgdn` can scroll the caret off screen.** They move the viewport
  in rows, independent of the caret; the next vote keystroke scrolls back to
  the caret before drawing, so no vote is ever cast on a row the person
  cannot see, but between those two moments the caret's own row can be
  entirely off screen with nothing saying so.
- **A `shift+arrow` failure is undetectable.** Where a terminal does not
  report the modified key it silently delivers a bare arrow instead, moving
  the caret rather than casting a vote — the aliases (`ctrl+o`/`ctrl+r`)
  exist because of this, but the program itself cannot tell which case it is
  in and says nothing.
- **`ctrl+u` anchors on the first scar in the view, unmeasured against a view
  holding several.** D32 ended the one-scar invariant, so a view can hold
  many; `unfold()` scrolls to `anchors.scar`, the first one drawn, which is a
  design choice rather than something checked against a multi-scar frame.
- **`TestAnExpiringHoldIsTheOneHoleInTheFade` is the package's only
  wall-clock-dependent test** (a 150ms sleep past a 100ms margin, because
  holds are stamped with `time.Now`). **D59(d) voids the argument this
  entry used to make** — the framing that this "wants the same injectable
  clock D38(c)'s simulator would also want" treated a real simulator
  (D38(c)'s determinism argument) and a real deterministic test harness
  (`tui/harness_test.go`'s `simulate`, D59(b)) as the same unbuilt thing;
  they are not, and the harness already exists. `testing/synctest`
  (stable since Go 1.25.0, in this project's installed toolchain, zero
  new dependency) closes this specific gap for free, independent of
  whether a simulator is ever built — deferred rather than adopted only
  because nobody is editing this file's test yet (D59(d)).
- **A new steady state: vote-then-walk-away leaves the gauge over its limit
  with rows cooling and no fold.** `blocked()` names it — D32's size rule
  meeting a run of holds, so `pressured()` is true, the gauge reads full or
  past it, and nothing on screen is actually going anywhere until a hold
  lapses or the human releases one.

New this checkpoint (D42):

- **A scar fades in neither channel — the steady state, not an edge case.**
  Disclosed rather than closed: see the retirement note above for the
  mechanism. For a reader who never votes it is the *common* case, not a
  corner one, and no learnable on-screen shape for it has survived
  measurement yet.
- **An older sentence-column fall, pinned by count rather than fixed.** The
  vote column widens by three columns while the terminal gains one at the
  width where a drain first fits, so the sentence column gives up two —
  reproduced at `step = 1`, so it predates D42's two-column step and is not
  its regression. `TestWideningTheTerminalNeverTakesAnythingOffARow`
  (`tui/tui_test.go`) pins it to exactly one such fall per fixture; a
  second would be a failure.
- **The caret sits hard against the vote mark on a marked going row, where a
  staying row has a space.** Named in `tui/harness_test.go`'s
  `TestHarnessFade` comment as one of two things to look at and neither is
  assertable; not yet given a fix or a test.

New this checkpoint (the persona's voice):

- **The persona's standing instruction is not in the record, so an auditor
  cannot ask what the thing was told to be.** `persona.Persona.System` is
  deliberately outside `Handle()` (`persona/persona.go:73-79`,
  `TestHandleNamesTheWeightsAndTheVoice` pins it), so a bit spoken under the
  old guard-list instruction and one spoken under the rewritten text are the
  same participant with the same ref, and nothing in the store distinguishes
  them. The devtool reader loses nothing by this; the reader who has to
  answer for what an agent did loses the whole question "what was it told,
  at the time it said that". persona.go already names the fix — the
  instruction's hash in the ref — and it moves every content address, so it
  is a D26/D33-class decision rather than a tidy-up. Recorded, not decided.
  D50(k).
- **Nothing anywhere tests for confabulation-as-a-question.** One live sample
  under the *old* instruction answered a question about folded material by
  offering the human a menu of decisions it had invented ("were we
  discarding them, migrating them separately, or applying a schema fix?").
  It states nothing false and is worse than a wrong answer, because it can
  plant the decision in the person who was supposed to be checking. Both
  the fold note and the standing instruction are written against
  *asserting* what was folded; neither says anything about proposing it.
  No check for this exists and none is proposed here.

New this checkpoint (D50):

- **The gate is flaky about one run in four, via `store-unlocked`.** Four
  full `go run ./cmd/seam` runs the same day: three came back clean and one
  failed because `TestConcurrentFoldsAgreeWithOneSequentialRun` asserted 0
  of its 16 samples where the claim declares `proven`. Run alone —
  `go run ./cmd/seam -run store-unlocked` — the same check asserted 5/16,
  4/16, 4/16 across three further runs: never 0 there, and never higher than
  5 anywhere. `docs/CLAIMS.md`'s own header bars a gate flaky by
  construction, so this claim's declared verdict (`proven`, unconditionally)
  is narrower than the claim honestly is; full account already at
  `docs/CODE.md:96-114`. CEO ruling on the route (D50(m)): fix the
  declaration or the check; raising `runs` until it comes back green is
  explicitly barred, since that is sampling until the gate behaves rather
  than fixing what it measures. Re-check: `go run ./cmd/seam -run
  store-unlocked` (isolated) and `go run ./cmd/seam` (full).

  **Second shape of the same flake, and a confirmation it is not any
  particular tree's fault, measured 2026-08-14.** Nine full runs during
  the ranked-surface work: seven clean, two reporting `killed-mid-check 2 ·
  adrift 1` — a *different* failure from the one above, where the sibling
  claim's cited check is aborted by the process dying rather than
  asserting the wrong number. Isolated (`-run store-unlocked`) it was
  clean 4 of 4, so the extra load of a full run is part of it. **It
  reproduces on an unmodified `HEAD`**: `git archive HEAD | tar -x -C
  <scratch>` and run the gate there — one of three runs came back `adrift
  1` with no working-tree changes present at all. So a seat that sees the
  gate go red on this claim should re-run it against a clean copy before
  believing it is theirs, which is two minutes and is the difference
  between a real finding and a coin.
- **`go run` flattens every non-zero exit code to 1.** `cmd/seam/main.go:145`
  calls `os.Exit(code)` with `code` 0/2/3 from `status()`
  (`cmd/seam/run.go:454-462`: 2 for `adrift`, 3 for `moved`), but a program
  exiting 3 gives `$?` = 1 through `go run ./cmd/seam` and only gives 3
  through a built binary — measured directly, go1.25.4. Both `CLAUDE.md` and
  `docs/CLAIMS.md` document the `go run` invocation, so the exit vocabulary
  is currently invisible through the form this project tells people to use.
  Latent, since nothing scripts the tool today, but the printed marks
  (identity block, per-row `[moved]`, inventory) are doing all the work the
  exit code was supposed to also do. D50(f).
- **Warmth may be moving the only signal this product has, unmeasured.**
  D39(a) withholds the vote from the model so it cannot optimise for
  approval, but nothing withholds the model's charm from the human — a
  synth that is pleasant to be around is one a person checks less, and the
  vote (D4, D30) is exactly a human checking output they did not write. The
  rewritten `standingInstruction` (`tui/ask.go`) was chosen for register on a
  three-arm live sample (the commit that landed the rewrite carries the counts in its message), never
  for its effect on vote rate, and no instrument here could currently tell
  the two apart. No check exists and none is proposed here. D50(l).

New this checkpoint (D52):

- **`writestring` lint warnings, pre-existing and surfaced by the LSP this
  session, not caused by it.** `tui/ask.go:780,788` and
  `tui/harness_test.go:112,118` build `strings.Builder` output with
  `b.WriteString(fmt.Sprintf(...))`/similar rather than `fmt.Fprintf(&b,
  ...)` directly. `tui/ask.go` has been untouched since the commit that rewrote `standingInstruction`, so this
  predates this session; noted rather than fixed, since fixing it was not
  this checkpoint's unit.
- **The stale-figure mechanization route from D52(f), not yet built.**
  Where a hand-transcribed figure has a real reader, the fix this session
  applied is deletion (`docs/CODE.md`'s line count for `docs/CLAIMS.md`).
  Mechanizing the ones that do have readers — generating the figure at
  read time or at commit time rather than hand-repairing it — is the
  route that would close the disease rather than the tenth instance of it;
  nothing does this yet.
- **`TestCoolPanics` (`memory/cool_test.go:247`) is still open and still
  uncited by any claim** — confirmed by name-grep against `docs/CLAIMS.md`
  this checkpoint. D48(j) ruled the guard-message rework and the panic
  test as a separate commit; neither is built. See also the two existing
  bullets above citing `cool_test.go:267` and `vote.go:126`/`cool.go:180`
  — those line numbers are from an earlier checkpoint and have since
  drifted with file growth; `:247` above is this checkpoint's own
  re-check and is the one to trust.
- **The `err != nil` blind spot in `memory/` is at four instances, not
  three** — `.claude/craft/principal-go-engineer.md:769-781`.
  `TestASingleFlippedBitInAViewIsAlwaysCaught` swept 8,000 single-bit
  corruptions and asserted only that *something* non-nil came back,
  staying green across a defect that returned the wrong error and leaked
  an unverified view through the error type. Re-check: `go test ./memory/
  -run TestASingleFlippedBitInAViewIsAlwaysCaught` after swapping the two
  comparisons in `ReadViewAgainst` — it must fail.

New this checkpoint (continuous persistence — no decision number yet):

- **A save is the whole file and there is one per change, so the bytes
  written over a session are quadratic in its length.** Accepted, named,
  and deliberately not fixed: the wire format is whole-record by design
  (`memory/wire.go` sorts by address so one record is one file byte for
  byte, and a view is sealed against the record's address as a whole), so
  an append mode would be a second format — which is the thing that
  package exists to refuse. Measured 2026-08-13 on this machine, over a
  fixture of plain utterances with no votes and no folds, so the byte
  figures are a floor for a real session: 12 bits → 4,421 bytes, 1.3 ms;
  100 → 34,253, 2.0 ms; 343 → 116,873, 3.8 ms; 1,000 → 340,253, 8.6 ms.
  About 1.3 ms of every save is fixed cost — create, fsync, rename — and
  the rest is roughly 22 ns a byte. Against the live run behind the demo
  page, which averaged one bit every 3.5 s, a save is a tenth of a
  percent of the gap between two bits. The full argument is the doc
  comment on `cmd/tldr`'s `record.checkpoint`. Re-check: a throwaway test
  that saves a record of n bits twenty times and divides; the standing
  cheap version is `wc -c` on the record beside `store.Len()`.
- **`record-frame-unclosed` went `over-red` and has been repaired** — the
  two names are in its `red:` list and the set is now fourteen:
  `TestAChangeThatCouldNotBeWrittenIsCarriedByTheNextOne` and
  `TestTheFileMatchesMemoryAfterEveryChange`, both new in
  `cmd/tldr/save_test.go`, both found by
  `go run ./cmd/seam -run record-frame-unclosed`. Kept here as the record
  of a prediction that came true within the day it was written. **The
  engineer's note called this the first of the three trips that would
  retire `sole: true`; the CEO ruled it is not one of them, and the ruling
  is in the claim's own prose.** The trigger is for a trip with *no
  bearing on the frame*; both of these read a saved file back, and the
  frame is what they read it through. Counting a confirmation of the
  prediction as evidence against it is the error the trigger exists to
  avoid.
- **The save hook makes `tui.Update` do I/O**, which is the one place in
  that package that is not a pure function of the Model and the message.
  `Model.update` — the switch, and everything the tests drive — is
  unchanged and still pure; a nil `tui.Save` is a session that does
  nothing, and that is what every test in the package runs. Noted rather
  than treated as a problem, because the alternative the CEO ruled
  against (a `tea.Cmd`) buys asynchrony and costs two writes in flight
  over one file. Re-check: `grep -n "m.save" tui/*.go` — three lines, all
  in `save.go`.

New after the D53 checkpoint (found by a subagent, verified by the CEO):

- **A bit longer than the terminal was unreadable in full — closed for the
  transcript, still open everywhere else.** The row the caret is on is now
  drawn whole and wrapped (`tui/render.go`, `saidWhole` in
  `tui/unfold.go`); every other row is still one line cut with
  `ansi.Truncate(text, width, "…")`. No key was taken: the caret rides the
  newest bit and `Load` puts it on the newest bit of a record read back,
  so the answer that just arrived is whole without a keystroke, and
  anything older is one arrow away. Re-check:
  `HARNESS=1 go test ./tui/ -run TestHarnessRead -v`.

  **Two corrections to what this item used to say, both worth keeping.**

  It called this "a D14 violation on the surface", and that is right in
  spirit and wrong in letter — D14 binds the *record*, and
  `memory/reach_test.go` was passing the whole time. What was broken is
  the promise D14 makes about the record: a `…` is an antecedent with no
  receipt, the one cut on this surface that is visible and unfollowable,
  where a fold leaves a scar with a key printed on it.

  It also said "nothing anywhere reads the whole `memory.Utterance`
  back", which was false, and the true version is the better argument:
  `Model.turns` (`tui/ask.go`) sends `p.Text` whole to the model on every
  request, and `oneLine` returns it whole. Nothing ever *drew* it whole.
  The other participant in the conversation could read more of the record
  than the person at the keyboard.

  **What is still open**, none of it a keybinding either:

  - ~~**The ranked view cuts every row**~~ — **closed.** The caret's row
    there shows its whole message too, in a different shape: the row stays
    a reference and keeps every column it had, and the message is quoted
    underneath it in the gutter at the width of the terminal. That shape
    rather than the transcript's because the columns differ — measured, a
    ranked row's lead is 44 columns of a 100-column frame and 60% of a
    40-column one, so wrapping in place would repeat those blanks on every
    line of an answer. **The transcript's gate is deliberately not
    inherited and the block has a floor of its own**, which is a
    distinction the first version of this got wrong in both directions.
    That gate is a floor on the column a transcript row wraps into, and
    the column matching it here is the preview, which the block does not
    wrap into — so porting it would cut this surface where it reads
    perfectly. The block wraps into the terminal, and *that* width has a
    floor: below twenty columns the row falls back to being cut. Measured
    without one, on the shipped code: at ten columns the block broke
    ordinary words across rows, and at four it drew **223 rows of `│  …`
    with every character clipped away**, against eight rows and a visible
    ellipsis for not opening at all. Re-check:
    `HARNESS=1 go test ./tui/ -run TestHarnessRead -v` for the frames, and
    `TestARankedBlockRefusesToOpenIntoAColumnTooNarrowForIt` for the floor
    in both directions.
  - **A voted scar no longer opens, and it is still the one row here that
    cannot be followed.** *Half of this changed with D59(j): `said` and
    `saidWhole` now route a fold to `scarLine`, which draws the same
    quotation the transcript's scar draws, already cut to the row it is
    on — so `saidWhole` returns one line for a fold and the block never
    opens. Stated in `saidWhole`'s own doc: this block exists to show a
    message whole, a fold has no message of its own, and quoting one
    absorbed bit at greater length here would be the surface that cannot
    open the receipt making the larger claim about the fold.* What is
    unchanged is the older gap underneath it: `ctrl+u` does nothing on
    this surface, so the one row whose whole subject is standing for
    absent material is the one row here that cannot be followed to it.
    Held by `TestAScarQuotesTheSameBitOnBothSurfaces`.
  - **A block and a fold can want the screen at the same time, and that
    frame has not been looked at.** The ranked list is stable across a
    fold by design, so this is a scrolling question rather than an
    ordering one, and `Model.revealBlock` handles the two claimants in a
    stated order. What is untested and unseen is a tall block open while
    the transcript underneath it collapses. Named rather than fixed.
  - **A bit behind a scar** is still one cut line inside its receipt
    unless somebody voted on it and it can be reached through `ctrl+t`.
    The caret walks the view; a receipt's rows are not in the view.
  - **Line breaks are dropped — closed on the caret's row.** It read: a
    numbered list comes back as a run-on sentence. `markdown` closed it
    for a message written as a document and `wrapped` closes the rest —
    the caret's block now keeps every line and every leading indent the
    record holds and wraps only a line the terminal cannot fit, with the
    continuation carrying that line's own indentation. Pinned by
    `TestAnExpandedRowKeepsTheWordsAndTheLineBreaksTheRecordHolds`
    (the renamed pin, now asserting the opposite),
    `TestAWrappedLineCarriesItsOwnIndentAndLosesNoWords`,
    `TestADeeplyIndentedLineKeepsItsOwnCharacters` and
    `TestNoDrawnRowCarriesATab`.
    **What stays open is every row that is not the caret's**: `said`,
    the ranked previews and a receipt's rows have one row each, so a
    break there is collapsed like any other cut, and a wrapped
    continuation is told from a written line by the words rather than by
    the shape.

    The cost of the residual, while it stood, was measured before it was
    accepted and **the figure is deliberately
    not written here**, under D52(f): four counts of
    `~/.local/state/tldreddit/record` taken the same day came back 14, 17,
    20 and 22 utterances, and every one was right when it was taken.

    That is worth a reader's time, because it is not sloppiness — **the
    record is the file `tldr` writes, so measuring it by using the program
    changes it.** Three seats ran the real binary against the real record
    while counting it, and the file went from 10,279 bytes to 19,662 in
    twelve minutes. Any figure quoted from that path is true at a
    timestamp and false shortly after, so it is quoted with one or not at
    all. What the counts agree on is the part the decision rested on: a
    message with a line break in it is a minority of the record at every
    count, and normalizing them is a display decision this unit did not
    need to make.

    Re-check, expecting a different number: decode with
    `memory.ReadStore` — the product's own reader — and not with a scan
    for length-prefixed tags. Where the two disagreed here, the ad-hoc
    scan was the one that was wrong, which is the same lesson as
    `cmd/seam` versus a hand-rolled mutation script and is now twice in
    one session.
  - **Below 24 columns the caret's row is cut like the rest**, because
    the fixed columns have already outrun the terminal — the same regime
    `clip` and `nameColumn` concede. Measured on a fixture nobody has
    voted in, and pinned in both directions by
    `TestTheCaretsRowIsCutWhereTheArrangementAlreadyGaveUp`; a fixture
    with a vote column has a higher floor, because a floor belongs to its
    lead.
  - **The `clip` on a continuation row cannot fire.** `hang + room` is
    exactly the width whenever a row is drawn whole, so that backstop is
    unreachable and no mutation catches its removal. Disclosed in
    `tui/render.go` beside the line rather than left to be found.

New this checkpoint (the non-interactive verbs, D51(e) — no decision number
yet):

- **Two writers over one file are not serialised, and what is left is a race
  of milliseconds.** *Fixed in the large after review: this bullet used to say
  `tldr say` beside an open session was an accepted lost update, and the
  argument it gave — that the only fix was a lock file — was a false choice.
  `cmd/tldr/record.go`'s `absorb()` now reads the file back before replacing
  it and files anything the writer lacks, which the content-addressed store
  makes conflict-free. Held by `TestTheWriterThatSavesSecondKeepsTheOthersBits`
  and claim `record-a-save-that-does-not-erase`.* What remains is genuinely a
  lock's job: the window between that read and the rename in `place()`. Two
  saves in flight inside it still lose one, and nothing detects it. Not tested,
  because one process cannot produce the interleaving. Re-check on the part
  that *is* fixed: run `tldr`, `tldr say -as x hello` beside it, press a key in
  the session, then `tldr top` — the bit is on the record, and correctly not on
  that session's screen.
- **A save now refuses over a record it cannot parse, which is new behaviour
  with teeth.** `absorb()` reads before writing, so a file damaged mid-session
  stops every subsequent save instead of being replaced by a good one. That is
  the deliberate direction — overwriting bytes this build cannot read is the
  largest possible way to forget — but the person's route out is undocumented
  anywhere they will see it: the notice says the save failed and names the
  file, and does not say "move it aside and the next change writes everything".
  Held by `TestASaveWillNotReplaceARecordItCannotRead`.
- **A vote cast on a scar has no row in `tldr top`.** The reading admits
  utterances and fragments only (`reading`, `cmd/tldr/top.go`), because a
  `memory.Compaction` is what a view did rather than what anybody said and
  every bit it stands for is in the reading on its own account. The surface's
  ranked screen *does* let a person vote on a scar (D49), so that vote is
  visible on one surface and absent from the other. Disclosed in the doc
  comment; no fix proposed.
- **`tldr top` cannot follow anything.** It prints what was said, ranked; there
  is no way from it to a scar's receipt, to a bit's neighbours, or to a query —
  the same "no search, no jump" hole `CLAUDE.md` names for the surface, now on
  a second surface. The addresses it prints are abbreviated for a person
  (`memory.Short`), and nothing takes one back as input.
- **`say` writes a wall clock, and it is the only clock on this program's write
  path.** `memory.Cool` derives its instant and `memory.Stay` decays against
  `View.Latest` precisely so folding stays pure; a bit said from the command
  line has no such source and takes `time.Now()`. A machine whose clock is
  behind the record's newest bit therefore files a bit older than what it
  follows, which nothing refuses and which sorts wrongly in `top`'s own
  newest-first tiebreak. Not reachable through the surface, where every bit is
  stamped by the same process in order.
- **The guarantee that an agent cannot vote does not extend to identity.**
  `say -as local` writes as the human. That is deliberate — refusing it would
  block a person leaving themselves a note, and the signal being protected is
  the vote — but it means the record's attribution is only as good as whoever
  can run the binary. Stated in `cmd/tldr/cli.go`; the honest boundary is that
  an utterance attributed wrongly is a lie a reader can catch by reading it,
  and a vote nobody cast is a lie nothing here can check.
- **This unit created a publication obligation, and it should not be discovered
  at push time.** Every Go file currently in the public tree cites only D1, D13
  and D18, and the public `docs/DECISIONS.md` stops at D19 (both measured, with
  the command below). The three new files cite D1, D4, D14, D30, D39(a), D40,
  D51(d)/(e)/(f), D52(j) and D54(b) — nine of those eleven unpublished — and
  `cmd/tldr/cli.go`'s header paraphrases D51(d)'s launch positioning in prose
  and quotes D52(j) verbatim. `cmd/tldr/main.go` adds D50. *`decision-guard`'s
  report also listed D5, D49 and D53(a) for this unit. Re-derived: D5 is in
  `tui/tui.go` and predates this change, and **D49 and D53 appear in no Go file
  in this repository at all** — `grep -rn "D49\|D53" --include=*.go .` returns
  nothing. Corrected here rather than repeated; the list is shorter than the
  report said and the obligation is not.* So publishing `cmd/tldr`'s
  verbs means either publishing a large block of decisions that have never
  cleared the D15 gate, or shipping source whose comments cite a record a
  reader cannot open. Found by `decision-guard` during review of this unit.
  **CLOSED by D81, and the closure is worth stating because the obligation
  was real for weeks:** there is no longer a separate tree to publish into.
  This repository *is* the public one, `cmd/tldr`'s verbs and the decisions
  their comments cite are in it, and the entries those comments name that a
  reader cannot open are accounted for in "After D81" rather than being an
  undischarged obligation. What follows is the history of the item, kept
  because the reasoning is how a future reader judges whether the closure is
  sound. **No action taken at the time on purpose, and the mechanism it
  described changed twice:** at the time, `docs/PUBLICATION.md` held a per-entry
  ruling and the withholding call (D20(a)) was made entry by entry; D78
  replaced both with the one-file rule (`docs/PRIVATE.md` never leaves,
  everything else publishes), so the live question for these decisions is no
  longer "does each one clear a per-entry gate" but whether any of them mixes
  in `docs/PRIVATE.md`-class content that needs splitting out before
  publishing. Re-check the premise rather than trusting this bullet:
  `grep -rho "D[0-9]\+" . --include=*.go | sort -u` over this tree, against
  `grep -o '^## D[0-9]*' docs/DECISIONS.md` — the difference is the set of
  entries cited by code that a reader of this repository cannot open. There is
  no second tree to compare with any more.
- **The surface draws a control token as ordinary text, and whether it should is
  undecided.** `persona.Escape` neutralises a control marker on the way *out* to
  a model; nothing touches what is drawn. So the qwen3.5 reply that ends
  `?<|endoftext|><|im_start|>user` is on the live record and renders on screen as
  those characters, which is honest and is also the one place a reader could
  mistake the record's own vocabulary for the model's. **Not this seat's call and
  deliberately untouched** — the display of a bit is `tui-design-engineer`'s, and
  the argument runs both ways: marking it makes the row legible and puts a
  rendering decision on top of evidence, which is the thing `tui/ask.go` already
  refuses to do to a fragment ("a marker inserted into a participant's own words
  cannot afterwards be told apart from something they said"). Re-check that it is
  still unmarked: `grep -rn "endoftext\|im_start" tui/*.go` returns nothing today.
- **The escape covers three bracket shapes and cannot cover a family nobody has
  measured.** `persona/boundary.go` breaks `<|…`, `[NAME]`/`[/NAME]` and
  `<s>`/`</s>`, which is chatml, llama3, mistral and sentencepiece — verified by
  token count against ollama 0.17.7 on `qwen3.5:latest`, `llama3.2:1b` and
  `ministral-3:14b`. A model whose boundary is spelled some other way is
  unprotected and nothing would say so. This is a list, and lists go stale; the
  check is in `Escape`'s own doc comment and takes about a minute — ask a running
  ollama for `prompt_eval_count` with the marker alone in a user message, then
  again with one character of it changed. One token means it is a boundary.

New this checkpoint (the fold budget in rows — no decision number yet):

- **An upvote folds the question out from under the answer it was cast on, and
  the fix is in `memory/`.** *The `memory/` half landed: `View.Fold` now spares
  the bit a hot held bit names through `Prev` (`sparing`, `memory/view.go`), and
  the stranding rate below re-derived on the same schedule is **0.0% at every
  vote rate**, with folds over 400 bits going 28 → 37 at one upvote in ten and
  23 → 122 at one in three. What is still open is the legibility half named at
  the bottom of this entry, and one consequence on this surface: `foldable`
  (`tui/tui.go:1494`) counts hot-and-unheld bits and so now overcounts what a
  fold can take by up to one bit per hold — the trigger fires slightly early,
  which is the failure `memory.Stay.Holds`'s own doc warns about, and
  `View.Absorbing` is the exact count. Everything below is the state before that
  landed, kept because the measurement is what the fix is judged against.*
  `View.Fold` splits its window at every held bit
  (`memory/view.go`'s `runs`), so a hold keeps the answer and cools the run in
  front of it — which contains the question. Measured on this surface at 400
  bits, one bit every 3.5 seconds, `holdFor` 2m, budget 23: **at one upvote
  every ten rounds, an answer somebody kept is on screen without its question in
  about 91% of frames**; at one in five, 94%; at one in two, 92%. With nobody
  voting it is 0%. **These three figures (91/94/92%) are marked unreconciled
  against `tui/testdata/stranding.txt` (D61(b)/(c)), not replaced by it.** The
  frozen table answers a different schedule — a parameterized `back` (how far
  above the newest a vote lands) rather than a wall-clock vote-every-N-rounds
  — and the harness that produced 91/94/92 no longer exists to be re-run
  beside it. *Re-checked against the widened grid, which now sweeps the
  conversation's length as well: they are still not reached and stay
  unreconciled, but one explanation is now eliminated rather than merely
  unlisted. Length is not the missing parameter. At the 23-row budget this
  entry's own text names, the pre-`sparing` table reads 93.5% at one vote in
  ten, 92.8% at one in five and **22.5%** at one in two; and no length rescues
  the third, because `held%` is monotone in length and at that budget one vote
  in two runs 16.0% at 100 bits to 24.7% at 3,200. Nor is any other cell a
  match: 91/94/92 puts one-in-two (92%) above one-in-ten (91%), and in every
  budget and length swept one-in-two is either a fifth of its neighbours (23-row
  budget) or the lowest of the three (12-row). The gap is a difference in kind,
  not in scale.* What the frozen table does confirm independently: real
  stranding, once a vote reaches beyond the kept tail, is large — it ranges
  17.5% to 95.0% across 270 schedules (the range is unchanged by the widening;
  only the count moved) — and the "0.0% at every vote rate"
  reading published just below, after the `sparing` fix landed, is real but
  entailed by the schedule that measured it (it only ever voted on the
  just-added bit, whose `Prev` is by construction still in view), not a
  general property of the fix. It is not a D1 or D14 failure — the scar directly above the
  stranded row is the receipt and `ctrl+u` opens it — but the row a person
  pressed keep on is the half of an exchange that means least alone, and this is
  the surface's central act making its own screen worse.

  **No fold-side fix is available from `tui/`**, and that is the part that is
  absolute: `Stay` carries votes, a voter and a lifetime, and `Holds` resolves
  those against the store, so a synthetic hold needs a vote bit in the store —
  this program forging a ballot in the human's name, which is worse than the
  defect. `keep` is a tail count and cannot reach a split `runs` makes
  mid-window, and declining to fold is unbounded growth. The shape that would
  close it is a hold that reaches one step back along `Prev`, or a run rule that
  will not cool a bit a held bit names — both are decisions in `memory/`, both
  move what a vote means, and neither is proposed here.

  **A legibility fix is a different question and it does belong to this seat.**
  This is a defect about what a row *says*, not about what the fold *takes*, so
  the fold being out of reach does not put the whole item out of reach: the scar
  above a stranded row could name what it absorbed, or the stranded answer could
  draw its own `Prev`. Neither touches `memory/` or the fold. Deliberately not
  built — the CEO is ruling on the `memory/` shape first and two fixes racing at
  one symptom is how a surface ends up with two vocabularies for it. This entry
  said "nothing in `tui/` can reach it", which foreclosed both, and only the
  first should have been foreclosed. *Both halves are now settled and neither
  the way this paragraph expected. The scar above a stranded row does name what
  it absorbed — it quotes it (D59(j)/D60) — and **"the stranded answer could
  draw its own `Prev`" is withdrawn**: `Prev` is the head of the view at the
  moment of writing, not the turn being answered, measured at 24% of said bits
  on the live record, so drawing it would put a false relation on a whole row.
  See the entry at the bottom of this file.* Re-check: the mechanism is visible in
  `HARNESS=1 go test ./tui/ -run TestHarnessVote -v`, in the frame captioned
  "one row kept, between two scars".
- **A taller terminal does not un-fold what a shorter one folded.** The budget
  grows the view forward and never backward, so a session dragged small, given
  one message, and then dragged large again sits at seven bits in a screen that
  would hold thirty-three. Seen in a real terminal, not derived: 100x30 → 100x18
  → one message → thirty-one bits into one scar → reopened at 100x40, gauge
  `6/33`. Correct under D1 (a view derives forward and a fold never deletes) and
  still a screen that is emptier than the record it is over. Rebuilding a view
  from the store at a larger budget is a feature nobody has asked for; named so
  it is not rediscovered as a bug.
- **A bigger budget means a bigger fold, so every scar's receipt is longer.**
  The 31-bit scar above opens into a block of three pages at 100x18, which makes
  the older "an unfold receipt loses its header after one page" item bite harder
  rather than differently. Unchanged in kind, worse in degree, and no fix
  proposed here.
- **A resize can leave the gauge reading past its own limit for an unbounded
  time.** Seen at `23/12` after 100x30 was dragged to 100x18. It is the honest
  report — the gap between the two numbers is what the next keystroke will cost,
  and every row above the keep is drawn cooling meanwhile — and it is a second
  route into a state that previously needed a run of holds. Stated in `gauge`'s
  own doc comment; the residual is that a person who never types again sees a
  full bar and no fold, with no word on screen (`blocked()` is false, because a
  fold *could* fire).
- **The persona's window is capped in bits, and bits are a proxy for tokens.**
  *Closed in the large: `tui/ask.go`'s `askCeiling` caps `Model.turns` at 60 bits
  so the terminal's height no longer decides how much context a model is sent.
  That was a live defect and not a theoretical one — measured against a live
  ollama 0.17.7 by `prompt_eval_count`, a realistic bit costs about 60 tokens and
  the prompt stops growing at ~4096, so a 200x80 terminal was asking for 74 bits
  against room for fewer than 68 and the server was truncating in silence. Held
  by `TestWhatThePersonaIsSentIsCappedBelowWhatATallScreenHolds`.*

  What is still open is the unit. D18(e) wants the model's budget in **tokens**;
  this one counts bits and assumes each is about 60 tokens, which a conversation
  of long replies breaks with nothing noticing. And **the 4096 is ollama's
  default `num_ctx`, not the model's context length** — `/api/show` reports
  262144 for `qwen3.5:latest` and `ministral-3:14b`, 131072 for `llama3.2:1b`,
  40960 for `qwen3:8b`. This client sets no `num_ctx`, so the real window is the
  server's default; anyone who sets one makes this constant wrong in the
  conservative direction by up to sixty times. Re-check: `askCeiling`'s doc
  carries the method, and `curl -s localhost:11434/api/show -d '{"model":"…"}'`
  the declared figures.

  **Closed 2026-08-17, and `askCeiling` no longer exists.** D75 put `num_ctx` on
  the wire, which met the precondition that constant's own comment named, and
  the budget is now `Model.askBudget` — **half of `persona.Persona.Window`,
  denominated in tokens**, so 16,384 against today's default rather than 60 bits
  against an accident. What prices a turn is `tokensIn`, a per-rune structural
  estimate whose error was measured against a live ollama 0.17.7 by
  `prompt_eval_count` over nine materials and two tokenizers: **estimate over
  measurement lands between 0.77 and 1.33**, reading high on typed prose and low
  on pasted density. The margin is the half that is not spent, and the reason it
  is a half rather than a subtraction is in `askBudget`'s doc. Held by
  `TestWhatThePersonaIsSentIsCappedByTheWindowAndNotByTheScreen`,
  `TestAPersonaThatNamesNoWindowIsBudgetedTheDefaultOne`,
  `TestTheNewestTurnIsSentEvenWhenItAloneOverrunsTheBudget` and
  `TestTheTokenEstimateChargesDenseMaterialMoreThanProse`; the old test named
  above is gone, and it went red against the new behaviour before it was
  replaced.

  *What is open in its place, and it is smaller:* the zero-window rule now lives
  in two packages, because `persona.Persona.window()` is unexported — `tui`
  restates "zero means `DefaultWindow`" and one test is all that keeps the two
  copies honest. Exporting it in `persona/` would delete the duplication.
- **A record showed one clock live and another after a restart — closed, and it
  was this seat's bug rather than `memory/`'s.** Found by driving the real
  binary: the same bits read `19:47` in the session that wrote them and `01:47`
  (next day) in the session that reopened them, on a machine six hours behind
  UTC — wrong by a day on the header, not only by hours.

  *This entry first filed it against `memory/wire.go` and said "not this seat's
  package". Both halves were wrong and the correction is worth more than the
  fix.* `wire.go:166-170` and `:449-450` are carrying out **D12**: one normalized
  instant, so the same moment recorded in two zones is one bit. `memory/bit.go`'s
  own doc for `At` already said the consequence in as many words — "anything
  drawing this to a person is drawing UTC after a restart and has to decide for
  itself whose local time to show." The package warned its caller and the caller
  had not decided.

  **CEO ruling: the drawn row uses the reader's local zone, on every surface.**
  Done in one place, `Model.day` (`tui/tui.go`), because `clock.stamp` reads every
  row in that reference's location and the header's date comes from it too. It
  deliberately does not answer "what time was it where the speaker was standing"
  — D12 threw that away and nothing here can invent it — which is why the header
  says `2026-08-14 local` rather than naming a zone: an abbreviation would read as
  a fact about when it happened. Held by
  `TestTheClockReadsTheSameWhoeverOpensTheRecord`, which drives the same bits in
  three zones and requires identical frames, the local reading of the instant, and
  the word on screen.

  **`cmd/tldr top` deliberately does not follow, and that is a live disagreement
  between two surfaces.** `stamp` (`cmd/tldr/top.go:318-326`) forces UTC with a
  stated reason the ruling's premise does not reach: that reading "is read by a
  session on another machine as often as by a person", and RFC 3339 because it
  "sorts, greps and parses". The ruling was made for one person at one keyboard.
  Left as it is, reported, and named in `Model.day`'s doc so the difference is a
  decision somebody can find rather than a discrepancy somebody trips over.
- **The budget is in foldable bits and the view is drawn in rows, and they
  differ by the scars and held bits in it.** `Model.foldable` excludes both;
  both still take a row. Measured with nobody voting, and the answer depends on
  which side of the floor the terminal is on: **where the budget is the screen**
  (viewport 12 and up, so height 19 and up) the view sits exactly one row past
  the frame — 24 at a viewport of 23, 44 at 43, 74 at 73, the extra being the
  scar at the head. **Where `coolFloor` is the budget** the two are unrelated
  and the gap is whatever the terminal is short by: **6 rows at 60x14** (viewport
  7, worst view 13) and 11 at a height of 9. That is the behaviour small
  terminals always had, and it is not "one row at every size", which is what this
  entry said until a review measured the other end. Counting rows
  instead was measured and refused: `len(view) > budget` as the trigger folds
  302 times over 400 bits at one upvote in five, against 47, because held bits
  keep the view permanently over budget and every write takes one run. Stated in
  `Model.budget`'s doc comment; the residual is that the word "budget" is one row
  looser than it sounds.

New this session (session 18, the covered row):

- **At one upvote in two there is no pressure at all, and the screen says
  nothing beyond a gauge reading nearly nothing.** A hold spares the bit its
  own bit answers, so holding every other row leaves every row in the view
  either held or covered: `Model.foldable` is 0, `pressured()` is false, and
  `blocked()` — which needs pressure *with nowhere to go* — is correctly
  false too. So the view grows past the frame with the gauge reading `1/23`
  and no word anywhere. Measured: 26 bits at 100x30, one vote in two,
  `foldable 1 of 23 · blocked false · covered 12 rows · view 26 rows`
  (`HARNESS=1 go test ./tui/ -run TestHarnessVote -v`, the frame captioned
  "every row kept or covered"). **Argued as honest rather than fixed**:
  there is genuinely nothing a fold could take, `Model.edge` says how much
  is off screen, and every row carries a `▲` or a `╷` saying why it is
  staying — which is a better account than the old behaviour, where the same
  arrangement printed a full gauge and the word `held` on pressure that did
  not exist. Recorded because it is a state a person can reach in a dozen
  keystrokes and nothing names it.
- **The covered mark is transcript-only, and the ranked surface says nothing
  about why a row is being kept.** The tie is positional — it points at the
  row below it — and that is only true in view order, so `Model.frame`
  leaves the covered set empty when the ranked surface is up. Today the
  guard cannot fire (`Model.judged` lists only bits somebody voted on, and a
  covered bit is by definition one nobody voted on), and
  `TestTheRankedSurfaceDrawsNoTie` asserts that intersection rather than the
  guard. It becomes live the moment that list widens, which D58(o) argues it
  should. No non-positional shape for "kept because of another row" has been
  drawn or looked at.
- **A covered scar is unreachable and the guard against one is inert.**
  `voteCell` refuses to draw a tie inside a scar's rule, and nothing can
  currently produce that state: `Model.utter` takes `Prev` from the view's
  last entry, a fold always leaves a kept tail after the scar, and
  `Model.keep` never returns less than half a budget, so no bit is ever
  written while a scar is the newest thing in the view. Measured: **zero, over
  499,508 view-rows** — nine sweeps of 400 bits through `Model.say` at 100x30,
  200x80 and 60x14 by one upvote in 3, 5 and 10, counting every row of every
  frame. `TestNoScarIsEverCovered` is
  the executable form; the guard is kept as a backstop, in D58(h)'s shape.
- **The covered state has no equivalent for a downvote and that is not
  symmetric.** A downvote holds nothing, so it covers nothing, and there is
  no row anywhere that reads "this is going *because* of something you did".
  Letting go of a hold does free rows, and the frame after the keystroke
  draws them cooling — so the antecedent exists, it just is not attached to
  the vote that caused it the way the tie is. Nothing measured, no shape
  drawn.

New this session (session 19, the scar's quotation and the footer's mark):

- **One quoted bit stands for a whole window, and nothing on the row says
  which of the two reasons put it there.** The scar quotes the absorbed bit
  the reader's own votes rank highest, tie-broken by recency — so a row
  carrying a vote-promoted quotation and a row carrying the plain
  most-recent one are byte-identical. The vote's own vocabulary cannot mark
  it: `▲` is already on that row in the vote column, meaning a vote cast on
  the *scar*, and a second `▲` mid-row would be one glyph with two
  meanings. Considered and not built: the absorbed bit's ordinal (`9/24`),
  which is navigable — it names the row `ctrl+u` opens — and reads as a
  second count beside "24 bits". Nothing measured on how often the two
  cases occur.
- **The count and the quotation are two claims of different sizes on one
  row, and only the count is checkable from the screen.** `24 bits · me
  "…"` is honest about both, and a hurried reader can take the sentence as
  the summary of twenty-four. What holds it up is that the count sits to
  its left and `ctrl+u` sits to its right; no wording was measured against
  a reader.
- **A narrow scar says nothing about its content at all.** Below
  `quoteFloor` columns of surviving words the row falls back to count, span
  and key. The old word list degraded further, one word at a
  time, and that was the behaviour being removed rather than a capability
  being lost — a single word off a frequency count is the noise D59(j)
  named. The speaker's column gives way before the words
  do — truncated with an ellipsis, floored at `nameFloor`, the handle
  column's own rule — so a long handle costs the quotation columns rather
  than costing it its existence; that was the first arrangement and it took
  the whole quotation off a 60x14 frame with a vote cast. What is left is
  the ordinary width floor. Re-check: `HARNESS=1 go test ./tui/ -run
  TestHarnessScar -v`.
- **The footer's mark says the index is incomplete and not by how much.**
  `bubbles/help`'s form, adopted deliberately: a count would be a number
  the screen cannot be checked against, and `Model.edge`'s "12 more" names
  rows a keystroke brings into view where no keystroke brings a dropped
  binding back. So a person learns that keys exist and not which. There is
  no help screen, and one was not proposed here.
- **The mark is off only on terminals 104 columns or wider.** Measured off
  drawn footers rather than counted out of the strings: the widest rung of
  the transcript ladder is 86 columns, and the complete index first appears
  at **width 104** on a fixture whose gauge reads `8/17` — the gauge's own
  width moves with the digits in its number, so that figure belongs to that
  frame. The two shorter ladders — a request in flight, a notice up — do fit
  at 100, so the contrast between a marked and an unmarked index is
  available in one session rather than only across terminals. Named because
  a mark that is on in every frame anybody sees is wallpaper, and this one
  is close to that. Re-check: `HARNESS=1 go test ./tui/ -run TestHarnessScar
  -v`, whose footer sweep crosses it.
- **The transcript's *content* now depends on a vote for the first time, and
  D39(b)'s bound is stated in a way that does not cover it.** D39(b) bounded
  the vote's reach at *where* things are drawn — the surface may not reorder
  itself under a keystroke. `frame.quoted` reads `frame.votes` to decide
  *what* is drawn on a scar's row, which is not that, so the bound is not
  crossed; but until D59(j) nothing on the transcript changed its text
  because somebody voted, and a later session reading D39(b) alone would not
  learn that the line moved. Stated here so it is not rediscovered as a
  violation. What holds the actual boundary is `TestNoVoteReachesThePersona`:
  the same quotation may not go to the model, because a message the human's
  approval selected is the sycophancy pump D39(a) names.
- **Open question, no measurement either way: whether the persona's account
  of a fold should be a quotation rather than a word count.** `foldNote`
  still sends a twelve-word index (`personaWords`), and the argument that
  kept it there is about *votes*, not about word lists — a quotation chosen
  by the reader's standing votes may not cross to the model. A quotation
  chosen by a **vote-free** rule (the newest absorbed bit, say) carries no
  vote information at all and would clear that objection completely. Nobody
  has ruled on it and no measurement exists. **For:** a verbatim sentence
  cannot be recited as content that nobody said, which is exactly what
  `foldNote`'s own sweep caught llama3.2:1b doing 6/6 under the present
  framing — "reciting the index as content". **Against:** an index reads as a
  tally and cannot be mistaken for a summary, while one real sentence out of
  twenty-four may anchor a model on the single exchange it happens to name,
  and the note's whole register is *this is a pointer, not a memory*.
  Deciding it needs a live-ollama sweep comparing one verbatim sentence
  against the twelve-word index over the same folded fixture — the same
  measurement `foldNote` already ran once and got noisy results from, which
  is why guessing is worse than waiting. **Not built, not rejected.**

Found in a real terminal after the review, using the binary rather than the
harness (tmux 3.4, scratch `TLDR_RECORD`, 71 bits, one standing vote):

- **A bit whose text is literally `╌` is byte-identical on the transcript to a
  fragment that got nothing out.** `unmarked` closed this for the *quotation* —
  a speaker's own glyph now stays inside their quotation marks — and it does
  not close it for the row, because `said` draws an empty fragment as the mark
  alone and a `╌` utterance as its own text. Seen on screen: a row reading
  `agent-7  ╌` that is a complete message. The transcript has no columns to
  spare for a distinction, U+254C is not on anybody's keyboard, and the row
  above the ambiguity is the one place it could be said. Named, not fixed.
- **The vote-promoted quotation is not on the first page of its own receipt.**
  A 57-bit fold quoted `note 27`, which is row 27 of 57 in the block `ctrl+u`
  opens — three pages down, with nothing on screen pointing at it. D14's
  reachability is real (the row is there, spelled the same way) and it costs a
  scroll through a receipt that has already lost its header. Under the recency
  tie the quoted bit is the *last* row, which is equally off the first page.
  The earlier tmux note that found the quotation at row 24/24 was on a
  twenty-four-bit fold, where the whole receipt fits. Nothing proposed; the
  obvious shapes (open the receipt scrolled to the quoted row, or mark that
  row) both spend the receipt's own ordering, which is what makes it checkable.
- **A third route into a gauge past its own limit, and it is the common one:
  `tldr say`.** The CLI writes bits to the record and never runs the fold
  trigger, so a record seeded from outside opens in the surface over budget —
  measured at first launch, `view 44 · record 69` with the gauge reading
  `43/23`. The two known routes were a run of holds and a resize; this one
  needs neither, and it is what a person who uses `say` for real work meets on
  their *second* launch. The first keystroke folds it correctly. Same family as
  the resize item above and no worse in kind — the report is honest, the gap is
  what the next keystroke costs — but the state now arrives without the person
  having done anything to cause it.

New this checkpoint (the ranked list widens, and the tie stops implying a
relation it cannot know):

**Reconciliation note, D61(h)/(n).** The widening below closed the gap
between `ctrl+t` and `tldr top`, and 21 of 29 said bits on the live
record went from existing on no interactive screen at all to existing on
one, in the `not judged` band. **That band is honest and it is not
retrieval** — it is every remaining row, placed by the clock, with a
heading saying so; nothing in it lets a person ask for a bit rather than
scroll to it. `CLAUDE.md`'s standing "no search, no jump, no query"
closure (D58(a)) names its own collapse condition as Tyler reporting he
went looking for something and could not get to it. This checkpoint does
not fire that condition — everything is now reachable by scrolling,
which was already true of `tldr top -n 0` — but it moves the condition
closer in one specific way: a 29-row unordered-by-anything-but-time band
is a worse place to go looking than a 3-row voted list was, precisely
because it is now honest about being large. The collapse condition is
unchanged; the cost of not having a query, on this widened surface, is
not.

- **`Prev` is positional, and every affordance built on it inherits that.**
  Both write paths set `Prev: shown.Head()` (`tui/tui.go`'s `utter`,
  `cmd/tldr/say.go`), so it records the head of the view when a bit was
  written. In an alternating session at this keyboard that coincides with "the
  turn this replies to"; for anything written from outside the surface it does
  not. Measured on the live record at `~/.local/state/tldreddit/record`
  (35 bits): **7 of 29 said bits, 24%, came in through `tldr say`**, and
  `f6d65254`'s `Prev` is `a0ab4364`, a greeting, rather than `d9ae9a94`, the
  claim it corrects. Not *unrelated* — an earlier draft of this entry said so
  and overstated it: `a0ab4364` is about the same subject, naming the DeltaDB
  and Zed material in its own text. What the edge fails to be is a reply to
  the thing the correction corrects, which is the whole of the finding and
  does not need the stronger word. The screen's response is a *narrowing of
  what the tie is allowed to mean* — "the row below is keeping this row out of
  the next fold", true on every frame — written into `tui/tui.go`'s package
  doc, `Model.covered` and `voteCell`. **What stays open:** nothing on the
  surface distinguishes a `Prev` edge that is a reply from one that is an
  accident of ordering, and nothing can, because the record does not carry the
  difference. Closing it is a `memory/` decision about what a write records,
  not a rendering one. The parent stub named as a legibility fix further up
  this file is **withdrawn on this evidence**: it draws the same edge at a
  whole row, so it would be the same falsehood at ten times the size.
- **The ranked list is now everything said, and nothing states its ordering on
  a scrolled frame.** `Model.judged()` (`tui/ranked.go`) listed only the voted
  bits until this checkpoint; on the live record that drew `ranked 3 · record
  35` — three rows, twenty blank ones, a claim later found unsourced at rank 1
  and the correction to it absent from the screen entirely. It now lists every
  utterance plus any voted-on fold, which on the same record draws 29 rows with
  `kept · 3` and `not judged · 26` as headings. The residual: the band headings
  scroll off, and a person whose caret is twenty rows down sees a wall of rows
  with no line saying what the list is ordered by. `Model.edge` says how much is
  above and below, and the clock column happens to make the recency tiebreak
  visible, but neither is a statement of the ordering. The established shape
  ("showing X of Y, sorted by Z") is already built in this repo, in
  `cmd/tldr/top.go`'s two header lines, and is **not** built here — one dim row
  under the header on a `fit` ladder. Named, ranked, not built.
- **`judged()` walks the whole store on every frame the ranked surface draws.**
  It was a walk of the vote view, which is small; it is now a walk of the record
  plus a sort, and `Model.frame` calls it on every keystroke while the surface
  is up. At 35 bits this is invisible and nothing has measured where it stops
  being. `cmd/tldr top` does the same walk once per process and does not care.
  No measurement, no fix, named so it is not discovered as a mystery.
- **A record nobody has voted in now draws a full ranked screen, and the first
  thing a new user meets there is 100% unjudged rows.** That is the honest
  state and it is what `band()`'s middle heading says. What is untested by
  anything is whether it *reads* as a working ranking to somebody who has never
  voted — the rows are placed by the clock, and the only thing saying so is a
  heading that scrolls. Same fix as the item two above; no wording measured
  against a reader.
- **The screen narrowed what the tie *means*; the fold still spends a row of
  budget on it, and off the surface that row can be anything.** `sparing`
  (`memory/view.go`) keeps the bit a hot held bit names through `Prev`, so an
  upvote holds two rows out of the next fold rather than one. Where the write
  came from this keyboard that second row is the turn being answered and it is
  well spent. Where it came from `tldr say` it is whatever the head was: upvote
  `f6d65254` on the live record today and the cover falls on `a0ab4364`, a
  model's greeting — a row of a screen-sized budget, held for as long as the
  vote holds. The screen's response to this (the first item in this section)
  fixed the *sentence*, and it does not reach the arithmetic: `Model.covered`
  now says only what is true of every frame, while the fold goes on treating
  both kinds of edge identically because the record gives it nothing else to
  read. The aggregate cost is measured and is not a runaway — mean view length
  22.7 → 25.1 rows at one upvote in ten on the published schedule, with the
  whole distribution in `memory/view.go`'s `sparing` and every figure a row of
  `tui/testdata/stranding.txt`. **What is unmeasured is the split**: nothing
  counts how many covers land on a reply and how many on an adjacency, on this
  record or any other. Until something does, "a hold covers local context" is an
  average being quoted as if it described each case. No fix proposed — a rule
  that spared only real replies would have to read a distinction the record does
  not carry (see the `Prev` item above), so this is a `memory/` question about
  what a write records, and the cheap thing available first is the count.
- **The request-failure block can print "nothing was recorded" directly above
  the person's own recorded bit.** `tui/ask.go`'s `troubleBlock()` carries two
  header ladders sharing one function: a request failure (ollama down, model
  not pulled) heads with "nothing was recorded" (`tui/ask.go:910-914`), and a
  save failure heads differently precisely because that claim would be false
  there — the person's words *did* reach the record, only the file behind it
  did not (see the package doc at `tui/ask.go:891-897`, and
  `TestASaveThatFailedNeverSaysNothingWasRecorded`,
  `tui/ask_test.go:228-268`). That test guards only the save-failure sibling
  against the string; nothing guards the request-failure block itself. On a
  request failure the person's own question is on the record and the header
  above it — `record 1`, or whatever `m.store.Len()` reads (`tui/tui.go:815`)
  — says so, one row above a block insisting nothing was. Found by
  `tui-design-engineer` while being the first user (D51). Proposed fix:
  `no answer was recorded`, which is true without qualification; the ladder
  widths (`tui/ask.go:909-915`) would need re-measuring against the new
  string's length, not copied from the old one. Re-check:
  `grep -n '"nothing was recorded"' tui/ask.go`.
- **CLOSED by D81 — this item's failure mode no longer has a mechanism.**
  There are no synced files and no second copy of the code, so a citation
  can no longer go wrong by someone editing a copy instead of the source; the
  re-check command it named (`diff -r` between two trees) has nothing left to
  compare. The general hazard it identified is not closed and is covered by
  D76(h) instead — cite by identifier, not by line number, because the number
  rots whenever anything above it moves. The original item follows.
  **CLOSED by D81: nine `file:line` citations in synced files resolved
  correctly only because the code was byte-identical across the two trees.**
  There is one tree now and nothing syncs, so the invariant those citations
  silently depended on is not merely held, it is gone — a citation can no
  longer be right in one tree and wrong in another. The history is kept
  because the failure mode generalises: a citation that resolves for a reason
  other than its own content is a citation waiting to break. A tenth, the one this
  checkpoint fixed (`docs/CLAIMS.md:676`, an `awk 'NR<=N'` line-offset
  settler), answered `## D39` in the private tree and `## D19` in the public
  one on the same command — found because D62's own citation history was the
  subject of the sentence it sat in. `principal-go-engineer` swept the rest:
  twelve `file:line` citations total in files that sync whole, nine survive
  naming a Go file at a specific line, and every one of the nine is correct
  in both trees for the same reason — the `.go` files they cite are
  byte-identical, not because the citation is content-anchored. If a future
  D15 gate finding is ever fixed by editing the *public* copy of a `.go` file
  rather than the private one and re-syncing, all nine become wrong in one
  tree silently, with no paragraph anywhere warning a reader that a citation
  crossed a boundary it depends on. The standing protection is the
  byte-identical invariant itself (`docs/PUBLICATION.md`'s "code syncs one
  way" rule), not anything in the nine citations — which is exactly why a
  gate finding must never be repaired by editing public code directly.
  Enumerated in `docs/CLAIMS.md` and `docs/CODE.md`; not individually listed
  here because the risk is the class, not the instances. The re-check this
  item used to carry was a `diff -r` between the two trees; there is no second
  tree to diff against, so what replaces it is reading the citation itself:
  a `file:line` that names a line rather than a symbol goes stale on the next
  edit above it, one tree or two.
- **Roughly twenty-one citations to `.claude/craft/*` now point at files the
  public tree does not carry.** D62(h) narrowed D40's craft-record
  publication default to a fresh per-checkpoint sweep rather than
  publish-by-default, and neither craft record shipped this checkpoint. The
  citations were not written with that in mind: `.claude/agents/
  principal-go-engineer.md` and `.claude/agents/tui-design-engineer.md` —
  both published — open by telling a reader "Your craft record:
  `.claude/craft/<seat>.md`. Read it first," and `docs/CODE.md` and
  `docs/DEBT.md` (also published) add bare path citations, four of them
  carrying line ranges into a file that, in the public tree, does not exist
  at all. `decision-guard`'s second D15 pass counted them directly
  (`grep -rn '\.claude/craft'`) and ruled the class comprehension, not
  confidentiality — a dangling reference discloses nothing, it just resolves
  to nothing for a stranger — and accepted the cost rather than editing five
  files to patch it. Recorded here so the next session does not rediscover
  it as new. **Still live after D81**, which kept `.claude/craft/` in the
  context repository for the same reason (D81(e)) — the cost is unchanged,
  only the path is: the records are at `$TLDR_CONTEXT/.claude/craft/`, else
  `../tldreddit-context/.claude/craft/`. Re-check:
  `grep -rn '\.claude/craft' docs/ .claude/agents/` over this tree and count
  how many resolve to nothing in it.
- **D18 ruled that forum is the base abstraction. There is no forum in the
  code, and there never has been.** Verified:
  `grep -rn "forum" --include='*.go' .` returns **five** hits, not six —
  and only four of the five are inside comments (`cmd/tldr/say.go:24,26`,
  `memory/rank.go:39,98`); the fifth, `cmd/tldr/cli.go:139`, is a string
  literal in the CLI's own help text (`"tldr — a forum-shaped memory you
  can watch think"`), which is prose about the product, not documentation
  of an abstraction the type system has. No `Forum` type exists anywhere
  (`grep -rn "type Forum" --include='*.go' .` returns nothing). One
  `Store` per process, instantiated at `cmd/tldr/record.go:122` and
  `tui/tui.go:483` — both confirmed at those exact lines — and nothing in
  the tree holds more than one (`grep -rn '\[\]Store\|map\[.*\]Store'
  --include='*.go' .` returns nothing outside `memory/store.go`'s own
  type definition). Nothing nests. This is the same class as the charter
  saying four packages and then listing five (D47's own cut) — a decision
  in force that the tree does not implement, unnoticed because nobody
  re-derived it. Related, and worth reading beside it: `CLAUDE.md`'s
  founding paragraph says "agents and subagents each hold their own
  forum-memory, and those nest." That is the one line of the founding
  paragraph with no code behind it. **Ruled on at D63(a): deferred, not
  refused**, on the sequencing ground that a forum roster is a wire-format
  break (`version` 1→2, `memory/wire.go:93`) — with a named trigger (D4's
  collapse condition) rather than left open indefinitely. D63(c) rules
  separately that an agent vote surface, if built, stays tiered rather than
  banned, and leaves one question open: whether tier-two votes may reorder
  a page the human has not yet judged. Neither is built.
- **`record-frame-unclosed`'s `red:` list is 28 test-name literals and grows
  every trip.** (It was 27 the checkpoint before, and the line number that used
  to be printed here is deleted rather than repaired: `docs/CLAIMS.md` is
  hand-maintained and a line into it answers a different question in each tree.
  `grep -n '^id: record-frame-unclosed' docs/CLAIMS.md` finds it in either.) D63(h) retires the
  trigger meant to catch it (a trip with no bearing on the frame) as the
  wrong instrument — nothing in the class that keeps tripping it can ever
  satisfy that trigger, since a `cmd/tldr` test whose only state is a file
  always has bearing on the frame. Left open: the readability cost of a
  27-and-growing literal list. A format fix would let the claim name a
  class ("these checks, plus every round-trip test") rather than enumerate
  it; not built, and no new trigger fires until a round-trip test is added
  and `seam` finds the list stale before its author does.
- **D14 and the shipped product disagree about what "reachable" means —
  reconciled, and this item is now the residue rather than the question.**
  The ruling: D14 binds *discoverability* and nothing else. `Store.All()`
  makes the record **enumerable**, a third mode that D1 permits and D14 does
  not count — more than *retrievable* (no address has to be known first),
  less than *discoverable* (it comes with no starting point, so it cannot say
  which bits a reader was meant to begin from). Recorded at D63(i); the
  ruling itself is D66(a), and its own reasoning is what "reconciled" above
  refers to. `memory/reach_test.go`'s header carries the three words.

  *Two things this entry asserted are false and are corrected rather than
  edited away.* It said `Store.All()`, `tldr top` and `judged()` "all
  enumerate ... `memory.Utterance` only". `Store.All()` admits **everything**
  — `TestAllHandsOutEveryBitIncludingWhatNoViewNames` proves it for a vote and
  a scar — and only the two callers filter. And `judged()` is at
  `tui/ranked.go:72`, not `tui/tui.go`; the callers that filter are
  `cmd/tldr/top.go:120-125` and `tui/ranked.go:80-86`.

  What is left open, and it is narrower than the entry above: **a fold receipt
  the transcript already shows can still strand.** `record.rejoin` now walks
  D14's own edges from both views and puts back what neither reaches — an
  utterance and a receipt into the transcript, a ballot into the vote view —
  but a receipt has nowhere to stand where the transcript already names the
  material it summarises, because a view never holds both a scar and a bit it
  names and a load may insert without rearranging. The construction, which is
  a row of `TestALoadPutsAStrayBackWhereItWasSaidAndMovesNothingElse`: one
  session folds and checkpoints while another that never folded saves after
  it, so the file's transcript shows the originals and the receipt over them
  is named by nothing. Nothing is lost that a reader can read — the material
  is all there — what strands is the *event* of the fold. Closing it needs a
  ruling about rewriting somebody else's transcript, not code.

  Also still open and unmeasured: a ballot naming other than exactly one
  target is left stranded on purpose, since `memory.Tally` and
  `memory.View.Rank` panic on one. `memory.Cast` cannot mint one, so the only
  source is a hand-assembled file. **Review asked whether `record.check()`
  should refuse such a file outright rather than strand the bit quietly, on the
  pattern the same file already uses ("a person moving a bad file aside
  deliberately is the better outcome"). Refused, and the line is worth
  stating:** every rule `check()` holds is the second statement of a condition
  `memory` already enforces by panicking — `View.Bits` on a view naming a
  missing bit, `Tally` on a non-vote and on a wrong arity *in the vote view* —
  so each refusal fires exactly where the surface could not be drawn at all. A
  wrong-arity ballot sitting in the store and in no view draws fine, costs one
  bit of the vote view, and refusing on it would deny a person their whole
  conversation over a bit no shipped writer can produce. What it actually wants
  is a way to say so and carry on, and this program has no channel for that:
  `load` returns a record or an error and nothing in between. **A warnings
  channel out of `load` is the unbuilt thing here**, not a stricter `check()`.

- **Two receipts can come back summarising the same material, adjacently, and
  it is permanent.** Not the refused-receipt case above — this one is
  reinstated rather than stranded, and it needs no hand-assembled file. Two
  sessions at different terminal heights fold with different windows
  (`Model.budget()` is the terminal height and `keep()` is half of it,
  `tui/tui.go`), so one mints a scar over five bits and the other a scar over
  eight of the same bits. Whichever saves last writes its own transcript over
  the other's; the loser's scar is then in the store and in no view, and
  `summarises()` only asks whether the transcript *names* something beneath the
  stray — never whether another *receipt* already covers the same material. So
  it is put back beside the wider one.

  Measured 2026-08-16 on a twelve-bit transcript folded at keep 7 and keep 4,
  the tall session saving last: the load returns
  `RECEIPT span 09:00..09:04` immediately above `RECEIPT span 09:00..09:07`,
  and the next checkpoint writes both. Nothing is lost and nothing is
  unreachable — it is a legibility fault, and the reader meets the same five
  bits summarised twice in two different aggregations.

  Not fixed, because the fix is a dilemma rather than a line: refusing the
  narrower receipt strands it, which trades a legibility fault for the D14
  fault this whole mechanism exists to close. Deciding it needs a rule about
  which of two overlapping receipts a transcript should carry, which is a
  ruling and not a build. `outermost()`'s instant key is the half of it that is
  already chosen — the day one of the pair is refused, the survivor should be
  the newer receipt and not the higher hash.

- **A receipt over a window of one cannot be ordered against the receipt
  beneath it, and the outer one strands.** `outermost()` orders nesting by
  `Compaction.Count()`, which grows strictly with nesting *because D32's size
  rule refuses a window of one* — so a `memory.Cool` called by hand over a
  single bit ties on the instant and on the count, and the content address
  decides. No fold builds one; the source is a hand-assembled file. The cost is
  a receipt in the wrong place, never two receipts standing for one thing —
  that half has no exceptions and is a row of
  `TestALoadPutsAStrayBackWhereItWasSaidAndMovesNothingElse`.

- **An exact tie in the vote view goes to the vote being put back, and the
  alternative was considered and refused.** `merge()` emits every row not later
  than a stray before the stray, and `standing()` (`memory/vote.go`) keeps the
  later position, so a reinstated ballot beats an incumbent it ties with to the
  nanosecond. The alternative — place strays *before* an equal instant, so the
  incumbent wins — was refused because `merge`'s rule is positional and both
  views take the same one: making the vote view merge by a second rule is a
  harder thing to explain than either outcome. Reachable only from a
  hand-assembled file or a fixture, since `memory.Cast` reads the clock; pinned
  as a row of the same test. The prose in `cmd/tldr/record.go` and in
  `docs/CODE.md` asserted the opposite for a checkpoint, on the reading that a
  row "keeping its place" keeps its standing — it keeps the *earlier* position,
  which is the losing one.

- **`docs/CODE.md` describes a file that was never committed, and a guard
  flag that appears nowhere in the tree.** `docs/CODE.md:906-908` describes
  `zz_probe_test.go` as "a separate, explicitly throwaway probe (guarded by
  `PROBE=1`) over the same rows" in `tui/`. Neither exists: `git log --all
  --diff-filter=A -- '*zz_probe*'` returns nothing (the file was never added
  in any commit, on any branch), `find . -iname '*zz_probe*'` finds nothing
  on disk, and `grep -rnw PROBE --include='*.go' .` matches nothing in the
  module. The inventory names a mechanism that has never existed, in either
  sense — not deleted-and-stale, invented and never built.

  This is the **first confirmed instance** of the `docs/CODE.md` ↔ code
  drift `CLAUDE.md`'s "Working on the code" section has been predicting in
  the abstract ("goes stale the moment code changes without a matching edit
  there — nothing enforces that yet"). It was found by reading past the
  noise of a one-command check with an 11:1 false-positive rate (it flagged
  stdlib symbols and missed `const`-block declarations), not by the check
  itself catching it directly.

  **Why the standing presence check (`CLAUDE.md`'s inventory sweep, also run
  in `archivist`'s checkpoint routine) cannot see this class at all:** that
  check only asserts every `.go` file currently in the tree is *named
  somewhere* in `docs/CODE.md` — a one-directional existence test. It has no
  inverse: nothing asserts that a file `docs/CODE.md` names, or a mechanism
  it describes, actually exists in the tree. A description of a nonexistent
  file passes the same sweep a true description does. Not proposing a
  checker for this — D69 just ruled on when an instrument earns a work unit,
  and the demand here is a single found instance, not a named recurring
  failure.

- **Assistant markdown is rendered in the caret's block only, and four things
  about it are open.** `tui/markdown.go` (new this session) draws a bit whose
  text carries a block-level mark — a fence, an ATX heading, or a list item —
  as a document: heading rows, bullets, an indented and syntax-coloured code
  block. Everything else takes `wrapped`, which keeps the message's own
  lines and indentation — it was the collapsed wrap until the pass after
  D73, and see the closed line-breaks item above. Each item below is a real
  cost, all measured, none of them fixed:

  - *(Closed.)* **A wrapped line inside a fenced block used to lose that
    block's indent.** Glamour indents a code block and then wraps at the full
    width, so a continuation landed at column zero, level with the prose —
    measured at 52 columns on the real binary. Closed by `prewrapped`, which
    breaks the long lines *before* glamour sees them and opens each
    continuation with `… ` at the code column. The break is at the column
    rather than at a word, so the rows concatenate back to exactly what the
    speaker wrote (`ansi.Wrap` eats the space it breaks at);
    `TestAWrappedCodeLineStaysInsideItsBlock` and
    `TestAWrappedCodeLineIsExactlyReversible` hold the two halves. **One
    residual, not fixed:** a continuation is lexed as a fresh line, so a
    broken comment loses its comment colour on the continuation. Structure is
    unaffected and it is invisible without colour.

  - **The depth of a heading is not drawn.** Headings are capitals with a
    blank row under them and no hashes, which is character-carried and so
    survives the fade — but capitals cannot carry a count of hashes. Indenting
    each level's body was drawn and rejected: it spends two columns of every
    prose line on this surface's scarcest dimension, and a two-column relative
    indent is already the fade's step, so it would put two meanings in one
    channel one row apart. The record keeps the depth.

  - **The one-row form of a document is markdown source.** `said`/`oneLine`
    are untouched by design — `oneLine` answers "what does the record hold"
    and the reachability tests read it — so every transcript row the caret is
    *not* on, every ranked-row preview, and every receipt row shows
    `### Reversing a slice in place Two things matter here: - it swaps from
    **both en…`. That is honest and it is also most of what a person sees.
    The shape that would close it is a display-only lede (the bit's first
    paragraph, skipping a leading fence) used by `said` alone, which is the
    same object as the scar-quotation item below.

  - **A scar quoting a code-bearing bit will read as garbage, and it is
    unfixed on purpose.** `frame.quoted` picks the top-ranked absorbed bit
    and `frame.quotation` renders it through `said`, so a fold whose winner
    is a fenced reply quotes ```` ```go package main import "fmt" … ````
    inside quotation marks — including the `"` from `import "fmt"`, which
    breaks the one assertion those two characters make. Held rather than
    fixed: the CEO ruled mid-session that this probably dissolves if a reply
    is segmented into several bits, because a scar could then quote the prose
    bit and leave the code bit alone. If segmentation does not happen, the
    fix is `opening(b)` — the bit's first paragraph, refusing when that
    paragraph is a fence opener — used by `frame.quotation` only, which
    changes nothing for any bit without a blank line in it.

  - *(Closed, by something that was not about it.)* **Glamour's padding trim
    had no mutation.** It was provably inert when written — both callers ask
    for a room that is the width minus their own lead, so a padded line landed
    flush, and removing it produced a byte-identical frame. `prewrapped`, added
    for the indent defect above, made it live: a code line broken to fit is
    only honest if the rows reassemble into what the speaker wrote, and the
    padding sits between them. Removing the trim now reddens
    `TestAWrappedCodeLineIsExactlyReversible`.

- **A code block in a folded window takes over the persona's word index.**
  `memory/cool.go`'s `words` splits on anything that is not a letter or a
  number, so identifiers and loop variables enter the bag with the frequency
  code gives them. Measured over a ten-bit fold window with one fenced Go
  reply in it against the same window without: distinct words 49 → 77,
  tokens of two characters or fewer 9 → 18, and the twelve words the persona
  is told go from `backfill, migration, columns, schema, dump, now,
  acknowledged, backfilled, uploading, dropping, standing, starting` to
  **`j, s, 1, t, migration, backfill, columns, schema, dump, now, 0,
  acknowledged`** — the four most prominent slots spent on a loop index, a
  slice, a literal and a type parameter.

  This is D60's model-facing half degrading, and it degrades further the more
  code a record carries. **Two fixes, on opposite sides of a one-way door.**
  In `memory/`: change `isSep` or `words`, which moves `Compaction.Bag`,
  which reaches `ID(cold)` — every scar already on disk re-addresses, so it
  is a D26/D33-class decision and not a tidy-up. In `tui/`: `topWords`
  already skips `filler`, and skipping single- and double-character tokens
  there costs one line, moves no address, and fixes the persona's index and
  the display alike. The second is inside `tui-design-engineer`'s ownership
  and is *not* built — it changes what the persona is told, which is D39(a)
  and D60 territory, and the CEO should rank it rather than have it arrive
  in a rendering unit. Re-check as a procedure: build a ten-bit window with
  and without a fenced reply in it, fold, and print
  `topWords(c.Bag(), personaWords)` for each.

- **The module's `go` line moved from 1.25.4 to 1.25.8, because glamour
  requires it.** `charm.land/glamour/v2 v2.0.1` declares `go 1.25.8`, so the
  whole module bumps; `.github/workflows/commit-gate.yml` reads
  `go-version-file: go.mod`, so CI follows automatically. And the binary
  grew: `go build ./cmd/tldr` is **12,170,076 bytes before and 22,266,246
  after, +10,096,170 (+83%)**, almost all of it chroma's lexer registry,
  which glamour imports wholesale and which nothing here can trim without
  forking. Named because it is the largest single dependency cost this
  project has taken and nobody would find it otherwise.

- **D79(d)'s clean-cut counter: how many consecutive cuts have step 4 run
  against them (a read of the produced tree, by a seat that did not write
  the cut) and come back with no cross-file aftermath.** Count stands at
  **0 consecutive clean**, because the only cut checked so far was not
  clean: D78's cut produced four cross-file defects, all four named in
  D79(a) — plus two unsourced figures, D79(b) and (c), which are a different
  class and are not counted here. The next cut checked that comes back with no aftermath
  makes it 1; a second in a row makes it 2, which reverses D79(d) and
  retires the extra pass as ceremony. A dirty result at any point resets
  the count to 0.

- **`go run ./cmd/cite`'s source-side check has not actually run on this
  machine this checkpoint.** `$TLDR_SOURCES` is unset and
  `~/.cache/tldreddit/sources` does not exist here, so every citation
  reports `evidence-missing`: `go run ./cmd/cite` → `13 citations, 13
  evidence-missing`, exit status 2. This is the tool failing loud as
  designed (D69) rather than a defect in the citations themselves — the
  record-side test the commit hook actually runs
  (`TestEveryShippedCitationResolvesIntoTheRecord`) needs no cache and is
  unaffected. Recorded so a future session does not read a past "measured"
  cite figure and assume the cache is filled here: it never has been on
  this machine. Filling it means fetching and extracting three PDFs per
  the manifest in `docs/CITATIONS.md`'s header.
