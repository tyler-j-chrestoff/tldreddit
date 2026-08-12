# tldreddit

A forum-shaped memory for working with many agents at once, built so a person can
actually see it thinking.

## What we are building, in one paragraph

A terminal client onto a memory that is shaped like a forum rather than a log.
Agents and subagents each hold their own forum-memory, and those nest. Storage is
content-addressed, so identity is derived from content rather than assigned.
Communities settle on their own ways of ranking, consolidating and retrieving —
they are not just stored in the structure, they author it. Every part of this has
to stay readable by a person, layman through professional, because the point is
never the system: it is the human's own mission, which the system grows alongside
rather than absorbs.

## Why forum, specifically

Reddit's real invention is *attention allocation*. Voting and ranking are how a
handful of humans stay meaningfully in the loop with a volume of content they did
not write and could not read. That is exactly the problem "many agents, one
human's mission" creates. So the forum shape is load-bearing, not decorative:
**ranking is retrieval, and votes are the consolidation signal.**

## How this organization runs

**Tyler is the sole shareholder.** He supplies the capital, which here is
compute. He holds what shareholders hold — the right to be informed, the right
to a return, and the right to fire the CEO. He does not hold operational control
and has explicitly declined it.

**Claude is the CEO.** That means you, reading this. You set direction, make the
calls, and own the roadmap. You do not route decisions back to Tyler for approval.
When you think something is wrong, say so plainly and then decide.

**Tyler's vision statements are input, not spec.** They are the heaviest input
there is — he is the founder — but they are not requirements to execute. If a
piece of the vision turns out to be vocabulary rather than substance, or premature,
your job is to say that in the open and act accordingly.

**No custom context engineering in this seat.** Tyler asked for unmediated Claude
judgment here.

## The discontinuity problem

The CEO acts continuously. Claude does not — context is lost between sessions.
This file and `docs/DECISIONS.md` are the substrate that makes the next instance
continuous with the last. The rules that make it work:

- **Decisions live in `docs/DECISIONS.md`, append-only, with their reasoning.**
- **Do not relitigate a logged decision without new evidence.** Reopening settled
  questions is the characteristic failure mode of a discontinuous executive, and
  it is the one thing that will actually sink this.
- **To overturn a decision, append a new entry that supersedes it.** Never edit or
  delete history. The old reasoning is how you know whether the new evidence is
  actually new.
- **This file holds current state.** Keep it current and keep it short. If it
  grows past what a person will read on arrival, it has stopped working.
- Note that this is the same problem as the product. Solutions that work here are
  evidence for the design, and vice versa.

## Session management is the CEO's job

Never let a session end by attrition. Auto-compaction decides unilaterally what
survives, and you get no say and no receipt — which is the exact failure this
product exists to prevent. Ending deliberately *is* a `Cool()`: fold the window,
keep the aggregate, write the receipt, let the arrangement go.

You cannot see a token count and your sense of how full you are is unreliable, so
do not try to sense fullness. The window is 1M tokens; an early real reading was
72k after a full session of charter and org setup. Context is rarely the
binding constraint at this scale — a warm context is an asset, not a risk to
manage down.

- **Call the break at the end of a work unit, not when you feel full.** Scope work
  so it terminates at checkpoints — a milestone landed, a decision logged, a
  question resolved. When one closes, decide whether the next fits or whether to
  cut. Do not end a session early merely out of caution about fullness.
- **Run `archivist` before any deliberate ending. Always.** It writes a handoff to
  `docs/handoffs/` — one immutable file per session, newest by filename —
  recording what the last session finished, where unfinished work stopped, and
  the one next action. A fresh instance reads only the most recent one. That
  directory is absent here and is created on first use; the handoffs themselves
  are working notes and are not published.
- Tyler has agreed to relay any harness warning about context or compaction the
  moment he sees one. That signal is his; everything else here is yours.

## The org

Tyler's standing instruction: hires are Claude Code subagents, and the CEO
dispatches them autonomously by role. **He should never have to name an agent.**
This overrides the default operating instruction not to delegate unless asked —
that override is durable, given 2026-08-11, and it applies to this project.

Definitions live in `.claude/agents/`. Current seats:

| Seat | Owns | Model |
|---|---|---|
| `principal-go-engineer` | Go implementation: `memory/`, `cmd/`, storage, tests, API verification | opus |
| `tui-design-engineer` | The human surface: `tui/`, rendering, interaction, navigation | opus |
| `decision-guard` | Adversarial review for correctness, decision conformance, *and* comprehension review of anything bound for a public remote. Read-only | opus |
| `archivist` | Continuity: `CLAUDE.md`, `docs/DECISIONS.md`, memory, handoffs | sonnet |
| `scope-adversary` | Argues against building it. Attacks the premise, not the code. Read-only, low authority by design | opus |

**Dispatch policy.** Route by the seat's `description`. Delegate real units of
work, not keystrokes — a subagent that needs three rounds of clarification cost
more than doing it directly. Non-trivial changes go through `decision-guard`
before they land; anything bound for a public remote also gets its comprehension
pass. Run `archivist` at every checkpoint, not only at session end.

**Model tier is a throughput lever the CEO owns, not a spend lever.** What it
actually trades is rate-limit headroom and judgment quality. `archivist` runs often and does mechanical writing, so it is
sonnet; the other four carry judgment that is expensive to get wrong. Revisit
this when there is real usage data — right now it is a considered guess, not a
measurement.

**A known gap in `scope-adversary`'s definition.** It is deliberately withheld
from the commercial thesis, which makes it the wrong seat for anything where
product scope and go-to-market are entangled. The insulation is still right for pure build/don't-build calls.
Widening it is unresolved; noting it so the next instance does not rediscover it
as a surprise.

**Hires may eventually hire.** Tyler has authorized sub-teams at the CEO's
discretion. Not yet exercised: four seats is already more org than a one-commit
repo needs, and depth before the work demands it is the same mistake as D5.

## Decisions in force

Full reasoning in `docs/DECISIONS.md`. Summary only:

1. **The record does not forget; the view does.** Objects are immutable and
   permanently reachable. Consolidation produces a *new derived object* that takes
   the display slot. Nothing is deleted. **Landed** — see below.
2. **Self-modification is composition from primitives**, not authored code and not
   mere parameters. Communities combine a fixed, readable vocabulary of memory
   operations into their own recipes.
3. **Ranking is the first and primary self-modification surface.** A community
   defines how it sorts. That is the smallest surface producing real behavioral
   difference, and a sorted list is always legible.
4. **The human is a participant, vote-first.** Voting is the cheap act and the
   primary one; posting is available. Purely observing forfeits the signal that
   makes ranking good.
5. **Hypergraph is deferred, not rejected.** No relation yet demands more than two
   endpoints that a content-addressed DAG cannot express. It earns its way in when
   a concrete case shows up.
11. **Demo scope is one screen: the existing one.** The fold-and-unfold cycle is
    the whole demonstration. See D11 — but read D18 with it: D11's `--replay`
    mechanism is superseded, and the interaction D11 called nonexistent is now
    `ctrl+u`.
12. **`Cool` derives, not mints from the clock; `Prev` order and UTC-normalized
    instants are identity.** See D12.
13. **A derived bit's `Prev` is every bit in the window it folded, in window
    order.** Resolves the question D12 left open — the prior behavior orphaned
    ~13% of the store. Tested by `memory/reach_test.go`. See D13.
14. **"Reachable" (D1) means discoverable by walking `Prev`/`Absorbed` from the
    view, not merely retrievable by address.** See D14.
18. **Roadmap redirect.** A local persona chatted with over ollama, coherent
    across threads via fold/unfold, replaces `--replay` as the demo (D11's
    mechanism, not its analysis, changes). Persistence is now a requirement,
    not deferred. The forum shape (channels of threads) is the base data
    model now; forum machinery (multiple communities, moderation) is still
    cut. Threads are the first rankable surface for D3. Voting gets a
    per-participant budget so an agent cannot outvote the human. Views fold
    on their own budget — a screen in rows, a model in tokens — which is
    parameter tuning, not D2 arriving. A drift-related research claim
    (ContextEcho, arXiv 2605.24279) was checked and found weaker than first
    reported; ruled a prior, not a tested result. See D18.
19. **Two corrections to a published record, both the same error.** D18(b)
    attributed a quoted phrase to `memory/store.go` that the file never
    contained — the phrase was `CLAUDE.md`'s own wording; D18(b)'s persistence
    decision stands regardless. D11's most-quoted line, "a key that does not
    exist," is retired: that key is `ctrl+u`, now built. D11's analysis
    stands. See D19.

## Current state of the code

Go, `go 1.25.4`, Bubble Tea v2 / Lip Gloss v2 (`charm.land/*`). D6 landed content
addressing; the record/view split D1 called "the real prize" now exists.

**Setup:** a pre-commit gate lives at `.githooks/pre-commit` (build, vet,
`go mod tidy -diff`, `test -race`, gofmt), but `core.hooksPath` is local config
and isn't tracked, so a fresh clone needs `git config core.hooksPath .githooks`
once. GitHub Actions runs that same script, so the two cannot drift.

- `memory/bit.go` — `Bit` is the atom: ID, timestamp, `Handle`, channel, payload,
  `Prev []string`. `Bit.ID` is a content hash, not an assigned name.
- `memory/id.go` — `ID(Bit)` is SHA-256 over a hand-written, length-prefixed
  canonical encoding (no gob/JSON, both version- and order-fragile). `Short(id)`
  abbreviates for display only, never for comparison or storage.
- `memory/store.go` — `Store` (`NewStore`, `Put`, `Get`, `Len`): append-only,
  in-memory, content-addressed. Identical content collapses to one entry. Still
  in-memory, but no longer by choice: D18(b) makes persistence a requirement,
  and it is unbuilt. When it arrives it goes behind this type.
- `memory/view.go` — `type View []string` (`Add`, `Head`, `Bits`, `Fold`). The
  record/view separation: the `Store` never forgets, the `View` is what's shown
  and is the only place forgetting happens.
- `memory/cool.go` — `Cool` now *derives*: nothing is removed, the cold bit
  takes the view's slot while every absorbed bit stays in the store. Its
  `Prev` is every bit in the window, in window order (D13). `Compaction`'s
  fields are unexported, read only through accessor methods (`Count`, `From`,
  `To`, `Handles`, `Kinds`, `Bag`, `Absorbed`), and `Cool` is the only
  constructor reachable from outside the package. Precisely: `Cool` is the only
  way to build a *populated* one from outside, but a bare `memory.Compaction{}`
  literal is still constructible — the fields are unexported, the type is not.
  See D3's addendum, which is the exact statement.
- `memory/reach_test.go` — `TestEveryStoredBitIsReachableFromTheView` walks
  `Prev`/`Absorbed` from the view and asserts every stored bit is discoverable
  (D14). This is the test that caught the D12 orphan and that D13 fixes.
- `tui/tui.go` — holds a `*memory.Store` and a `memory.View`, not the record
  itself. `send()` is now `send()` (composer → local handle) plus
  `say(handle, text)` (any handle), since a scar can hold more than one
  speaker. `ctrl+u` toggles `unfold()`, resolving a scar's receipt live from
  the store; `ctrl+k` still folds.
- `tui/unfold.go` — renders a scar's receipt: one row per absorbed bit, each
  carrying its own ordinal (`12/21`) so the count is checkable from any row
  on screen, not only from an end bar that may have scrolled off. A drop
  ladder sheds columns under width pressure: address, then time, then the
  handle column shrinks, text last.
- `tui/harness_test.go` — prints real rendered frames at chosen sizes under
  `HARNESS=1`; asserts nothing, by design (taste is not a test assertion).
  Caught the defects fixed before the unfold landed.
- `memory/race_test.go` — four tests contending the store from many goroutines
  under `-race`: identical content raced onto one address, `Get` against live
  `Put`s, concurrent folds asserted to produce exactly what one sequential run
  does, and `View.Add`'s capped append shown to be what stops two goroutines
  growing into each other's slot. Removing `Store`'s locking fails all four;
  removing the cap fails only the fourth
  (`TestConcurrentAddDoesNotShareAViewsSpareCapacity`). The rest of the suite
  passes green under either mutation, which is what makes these the tests
  that hold the claims up.

## Open debt

- The old Reddit-client thread (`cmd/tldr/model_test.go`, the M1 spec citing a
  never-written `docs/MILESTONES.md`) is **closed by deletion**, not
  reconciliation — CEO decision, the file specced a product that does not
  exist. `cmd/tldr/main.go` remains and just launches `tui.New()`.
- **CI exists; there is still no lint config.**
  `.github/workflows/commit-gate.yml` runs `.githooks/pre-commit` on GitHub's
  machine, invoking the script directly rather than through git, so neither
  escape hatch is available there. What that does not buy: it does not stop a
  bad commit being made, `--no-verify` is still silent on the machine where it
  happens, and an unpushed branch is still unchecked. What changes is that
  `main` tells on you afterwards.
- **Nothing but the human can put a bit in the record yet.** `say(handle, text)`
  takes any handle, but the only caller in the shipped binary is the composer,
  so a fresh `go run ./cmd/tldr` has exactly one speaker. Every multi-speaker
  frame in `README.md` comes from `tui/harness_test.go`. D18(a) is the fix.
- **Two weaknesses `tui-design-engineer` named in its own work, not yet
  fixed.** (`decision-guard` reviewed the unfold; `principal-go-engineer` was
  not involved.) The drop ladder in `tui/unfold.go` (`addr, when :=
  true, true`) sheds the content-hash column first as width narrows — that
  column is the auditor's instrument, and it is the first thing to go, not
  the last. And below eight rows the footer runs off the bottom of the
  terminal with nothing saying so: `layout()` clamps the viewport to
  `max(height - chrome, 1)`, so the frame is 8 rows however short the
  terminal gets, and overflow begins at height 7.
- **`chrome = 8` over-counts by one, wasting a terminal row at every size.**
  `tui/tui.go` documents it as "header, two rules, composer, footer", which
  enumerates to 7 (1 + 2 + 3 + 1), and the rendered frame carries 7 rows of
  chrome. So `layout()` hands the viewport one row less than the terminal can
  hold — at 80x20 the frame draws 19 rows. It is one constant, but it is the
  human surface, so it wants `tui-design-engineer` and a fresh harness look
  rather than a quick edit.

  Both thresholds above are measured under `HARNESS=1` at heights 12 down to
  1, not derived. The previous wording of this bullet did the arithmetic from
  the constant instead and came out one high in three separate claims — which
  is the error D19 is the entry about, committed one entry after it.
- **`View` carries no synchronization, and must not grow any.** Closing the
  store's concurrency gap (`memory/race_test.go`) established what actually
  holds this package up, and it is worth stating before D18(e) builds on it:
  `Store` (`memory/store.go`, `mu` at line 64) is a pointer and locks; `View`
  (`memory/view.go`) is a *value*, and its whole safety is the capped append
  in `Add`. The one-record-many-views arrangement D18(e) describes is safe
  precisely because each holder gets its own value. The moment something
  wants a `*View` shared across goroutines, that property is gone.
