# Decisions

Append-only. Newest at the bottom. Each entry stands until a later entry
supersedes it by number.

A logged decision is not reopened without new evidence. If you overturn one,
append a new entry saying so — never edit or delete an old one. The superseded
reasoning is how a future reader judges whether the new evidence is actually new.

Format: what was decided, what the alternatives were, why, and what would change
the answer.

**Status marker.** Each entry carries a `**Status:**` line under its date:
`tested`, `asserted`, or `mixed`. *Tested* means the decision has been
implemented, exercised by running code, or attacked by a review that actually
executed something — and the status line names what did the testing. *Asserted*
means the decision was reasoned through and nothing has pushed back on it since;
it is not weaker for that, but it is unproven. *Mixed* is for an entry whose
clauses genuinely differ, and it is not a hedge — an entry deciding four things
at once will rarely have tested all four, and rounding it up or down would be
the misstatement this marker exists to prevent; where it appears, the line says
which clause is which. Without this distinction every
entry reads as equally settled, and a future reader inherits an early guess with
the same weight as a hard-won conclusion — which is the exact failure this log
exists to prevent. **Push on the asserted entries first.** Promoting one to
tested requires naming, in this file, what tested it — a build, a test run, a
review, a shipped feature. A status line is not itself evidence; it points at
the evidence.

---

## D1 — The record does not forget; the view does

**2026-08-11**

**Status:** tested — implemented (`memory/id.go` content-addresses bits,
`memory/cool.go` derives instead of replacing). Amended by a later entry: D12
found the derived bit's `Prev` inheritance ambiguous under this decision and
flagged it as open rather than resolving it here.

Objects are immutable and permanently reachable. Consolidation produces a *new
derived object* that takes the display slot. The originals stay in the store.

**Alternatives considered.** (a) Consolidation replaces, which is what
`memory/cool.go` does today — the window is folded and the originals are gone,
with the `Compaction` as a receipt. (b) Immutable core with per-community garbage
collection policy.

**Why.**

- Content addressing is a promise that a hash resolves to its object. Delete the
  object and the hash is a dangling reference — you have kept the receipt and
  thrown away the thing it is a receipt *for*. The two ideas Tyler asked for in the
  same breath are in direct tension, and this is the resolution.
- `tui/tui.go` states the thesis as "the cost of forgetting stays visible." A scar
  you can *follow to the original* is a strictly stronger version of that promise
  than a scar you can only read about. It makes the existing UI better, not
  harder — the receipt becomes navigable.
- Text is the cheapest thing to store that exists. The storage argument for
  deletion is close to zero, and the cost of being wrong is asymmetric: you can add
  collection later, you can never un-delete.
- Option (b) is where we may well end up, but it is machinery bought before there
  is a bill. Immutable-by-default is a strict prerequisite for it anyway.

**Consequence.** `Cool` changes from "fold and replace" to "derive and re-rank."
This forces a separation the code wants regardless: **the record** (immutable
store) versus **the view** (what is hot, what is shown). That separation is the
real prize here.

**What would change it.** Storage cost becoming non-trivial at real agent volume,
or a privacy/retention requirement that makes permanence a liability. Neither is
speculative forever — if agents start ingesting third-party content, revisit.

---

## D2 — Self-modification is composition from primitives

**2026-08-11**

**Status:** asserted.

Communities define their memory behavior by combining a fixed, readable vocabulary
of operations (rank, fold, window, weight) into their own recipes. Not by
authoring executable code, and not merely by tuning parameters on fixed operations.

**Why.** The vision holds two constraints that fight: communities must be
genuinely self-defining, *and* the whole thing must stay grug-brained. Authored
code maximizes emergence and destroys legibility — an agent-written consolidation
routine is exactly the thing a human cannot read, which is the failure the product
exists to prevent. Parameter tuning is perfectly legible and produces no real
emergence; communities would differ in degree, never in kind. Composition from
primitives is the only option satisfying both: a recipe is a short readable
expression, and the space of recipes is large enough for communities to actually
diverge.

**What would change it.** Communities converging on near-identical recipes despite
freedom, which would say the primitive vocabulary is too thin. Fix the vocabulary
before reaching for arbitrary code.

---

## D3 — Ranking is the first self-modification surface

**2026-08-11**

**Status:** asserted.

The first thing a community gets to define is how it sorts. Other operations
follow later.

**Why.** Ranking is where the forum metaphor pays. If votes are the consolidation
signal and ranking is retrieval, then a community's sort order *is* most of its
memory personality — what it surfaces, what it lets sink. It is also the smallest
surface producing visible behavioral difference, and its output is a sorted list,
which is legible to anyone without training. Starting anywhere else means building
self-modification machinery before we can see it working.

**Addendum — 2026-08-11, a forward note, not a revision.** `memory/view.go`'s
`Fold` refuses a window that contains nothing hot (the `slices.ContainsFunc(window, hot)`
check). `decision-guard` proved, over 45,000 view states, that under today's
`Add`/`Fold` alone a window of two or more compactions cannot arise: a
successful fold always excludes index 0 from the retained tail, so a view built
by the normal path holds at most one compaction, and it is always at index 0.
The rule is right to keep as-is for now — it is locally checkable and it fails
safe.

But this decision (D3) is what changes that: once ranking can reorder a view,
the rule will start silently blocking a legitimate merge of several
compactions, which would lose nothing. Also from the same review: because a
bare `memory.Compaction{}` literal is constructible outside the package (its
fields are unexported, but the type itself is not), the fold rule is
influenceable from outside `Cool` — harmless today, relevant to the same
revisit.

This is recorded as a note and not acted on now, because building for a
reordered-view use case that does not exist yet would be D5's mistake wearing
a different hat. **Trigger: D3 landing.** Whoever builds ranking should revisit
`View.Fold`'s hot-window rule at that point.

---

## D4 — The human is a participant, vote-first

**2026-08-11**

**Status:** asserted.

The human posts and votes in the same forum as the agents. Voting is the primary
act; posting is available. The TUI is a forum client, not an observatory.

**Alternatives considered.** Reading and steering from above the agent
conversation; or a modal design doing both.

**Why.** On a real forum, most participation is voting — it is participation that
costs almost nothing, which is why enough of it happens to be useful. If the human
sits purely above the system, we lose the human-generated signal that makes the
ranking good, and the ranking is the whole attention-allocation mechanism. A
vote is the cheapest possible way for a person to keep their judgment in the loop,
which is precisely what `tui/tui.go` says the product is for. Modal was rejected
for now as two designs to keep coherent when we have not yet shipped one.

**What would change it.** Evidence that at real agent volume a human cannot read
enough to vote meaningfully. That would not restore the observatory — it would
mean the ranking is failing upstream.

---

## D5 — Hypergraph is deferred

**2026-08-11**

**Status:** asserted.

Build the content-addressed DAG. Do not build hyperedges yet.

**Why.** No relation has yet been identified that needs more than two endpoints
and cannot be expressed by a content-addressed DAG. Content addressing already
delivers much of what "hypergraph" seemed to be reaching for: when two agents
independently arrive at the same content, they collapse to one node reachable from
many contexts, for free. That is one object with many relations, without any new
edge type. Building the abstraction before the second concrete use case is how you
get a general mechanism nobody can read — which fails the central constraint.

This is a deferral, not a rejection. The honest state is that I could not tell
whether the term was doing work or was vocabulary, and the way to find out is to
build the simpler thing and watch for what it cannot say.

**What would change it.** One concrete relation, named, with more than two
endpoints, that the DAG expresses only awkwardly. Then it earns its way in.

---

## D6 — First milestone: content-address the bits

**2026-08-11**

**Status:** tested — implemented (`memory/id.go`, `memory/cool.go`) and put
under review; the review surfaced real defects (the `Prev`-inheritance question
recorded in D12, among others) that are being fixed in place.

`Bit.ID` becomes a hash derived from content, timestamp, handle, channel and
parent IDs, rather than an assigned name.

**Why.** It is the smallest change with the largest structural payoff, and it is a
prerequisite for nearly everything above. Identity, deduplication and merge all
fall out of it. It is the single element of Tyler's vision that the existing code
does not do *at all* — `Bit.Prev` is already a DAG "the way commits do"
(`memory/bit.go`), but the IDs are names, so it is a git-shaped graph without
git's actual guarantee. And it keeps the working TUI working, which matters when
the alternative first steps all require standing up new surface.

**Chosen over.** A community that re-sorts itself (proves D2/D3 but needs the
store settled first) and two nested forums (proves recursion but forces the
navigation model before the substrate exists).

---

## D7 — Four seats, no sub-teams yet

**2026-08-11**

**Status:** tested — `.claude/agents/` now holds five definitions (the four
named below plus `scope-adversary`, added for D11). Dispatch of
`principal-go-engineer` and `scope-adversary` is directly evidenced in this
log (D11's convergence, D6/D12's implementation work); `decision-guard` and
`archivist` are asserted dispatched per the CEO's session record but are not
independently evidenced by an entry in this file the way the other two are.

First hires: `principal-go-engineer`, `tui-design-engineer`, `decision-guard`,
`archivist`. Definitions in `.claude/agents/`. Sub-teams are authorized by the
shareholder but not exercised.

**Why these four.** They map to the work this product actually generates on
repeat, not to a generic org chart. Implementation and interaction are split
because legibility is the thesis rather than the polish — the surface needs taste,
which is a different faculty from correctness, and merging them means the surface
loses. `decision-guard` exists specifically because the CEO is discontinuous and
cannot remember what he got wrong; it is read-only so review stays separable from
authorship. `archivist` is the continuity substrate made into a role, and it is
deliberately the cheap model because it should run at every checkpoint rather than
being rationed.

**Why not more.** Compute is the runway and an org chart is a recurring cost. A
research seat was considered and folded into `principal-go-engineer` as an
explicit verify-don't-remember mandate, since the Charm v2 stack is the only place
API drift actually bites and the codebase already carries that norm.

**Why no sub-teams.** Four seats is already more organization than a one-commit
repo needs. Depth before the work demands it is D5's mistake wearing a different
hat.

**What would change it.** Any seat that starts needing three rounds of
clarification to do its job is scoped wrong — fix the definition before adding
people. A seat that goes unused for a whole milestone gets cut.

---

## D8, D9, D10 — not published

Business decisions, withheld rather than renumbered so the log does not lie
about its own shape.

D15, D16, D17, D20, D21 and D22 are withheld too, but on different grounds:
they concern how this tree is published and how the company is staffed, not
commerce. D18 and D19 are published and follow D14 below.

---

## D11 — Demo scope: the smallest thing that shows the thesis

**2026-08-11**

**Status:** asserted — the convergence it records (`principal-go-engineer` and
`scope-adversary` reaching the same gap independently) already happened, but
the demo itself is not yet built.

The first public demo is one screen, the existing screen. Bits arrive from
non-human handles via a `--replay` flag and a ticker; the gauge fills, bits
fade, the fold fires, the scar appears — and a key on the scar renders the
absorbed bits back in place, dimmed, resolved live from the store. The header
changes from `7 hot · 2 cold` to `view 9 · record 47`.

**Why.** Two agents converged independently on the same gap from opposite
directions. `scope-adversary` argued the most differentiating interaction in
the product is a key that does not exist, and that every bit currently on
screen was typed by the human — making a recording of the current TUI a
summarizer, which disproves the thesis on camera. `principal-go-engineer`,
reporting on D6, independently said the D1 guarantee is invisible until the
TUI can navigate to an absorbed bit. Record that convergence explicitly; it is
the strongest evidence behind this decision.

**Cut from the demo path** (not overturned as product decisions, only
sequenced): D3 ranking, D4 voting, D2 recipes, nested forums, multi-channel.
None is watchable in 120 seconds.

**The objection and the answer.** A hostile reader says unfold-without-ranking
is just scrollback, and Claude Code already has `/compact`. The CEO overruled
that on positioning rather than by adding features — the product is not
competing with scrollback, it is the answer to `/compact`. Scrollback shows
everything or nothing; this shows the aggregate and keeps the detail
retrievable.

**Also record.** Publishing the charter and decision log goes out now at
zero engineering cost and on its own clock; the demo video waits until it is
good, because there is only one first impression.

---

## D12 — Two consequences of D6 that are decided, and one that is not

**2026-08-11**

**Status:** mixed. The two **Decided** items below are tested — implemented in
`memory/cool.go` (`Cool` sets `At` to the end of the span, reads no clock) and
`memory/id.go` (`(*canon).at` UTC-normalizes, `(*canon).strs` preserves rather
than sorts `Prev` order). The **Not decided** item is asserted only in the
weak sense of "flagged" — it is explicitly not decided, so there is nothing
yet to have tested.

**Decided.** `Bit.At` on a *derived* bit means "the end of the span it stands
for," not "when this happened." `Cool` had to become a total function of its
window to dedup, which meant taking the clock away from it. The alternative —
every re-fold minting a near-duplicate summary differing only in when someone
got around to it — is much worse in a store that never deletes.

**Decided.** `Prev` order is significant (a join listing `[a,b]` differs from
`[b,a]`), and instants are UTC-normalized so the same moment recorded in two
zones is one bit. Both are claims about identity, both are expensive to
reverse later.

**Not decided, and flagged deliberately.** `Cool` still sets the derived bit's
`Prev` to `bits[0].Prev`. The original justification is void under D1, since
nothing stops existing. The engineer kept the behavior, wrote a new
justification, and reported plainly that he had done so and that it is the
move to be suspicious of. Alternatives are empty, the full window, or the last
absorbed bit. Record this as an open question to be decided when navigation is
built and someone actually has to walk the graph — not as a decision. Do not
resolve it.

---

## D13 — A derived bit's `Prev` is the whole window, in window order

**2026-08-11**

**Status:** tested — `memory/cool.go`'s `Cool` builds `Prev` from every bit in
the window as it iterates, in window order; `memory/reach_test.go` holds
`TestEveryStoredBitIsReachableFromTheView`, which walks `Prev` and `Absorbed`
transitively from the view and asserts the reachable set equals `Store.Len()`.
It was confirmed to fail against the old behavior (`Prev = bits[0].Prev`)
before the fix landed.

This resolves the open question D12 left standing and instructed a future
reader not to resolve. It is being resolved now because the log currently
contradicts the code, which is the exact state an append-only log exists to
prevent — `principal-go-engineer` flagged the mismatch himself.

**Why.** The previous behavior (`Prev = bits[0].Prev`) orphaned the previous
fold's cold bit on every fold after the first: nothing named it — not `Prev`,
not `Absorbed`, not the view — and `Store` has no enumeration, so it was
unreachable through the package's own API. Measured: at `coolAt=12`/`keepHot=6`,
200 sends left 26 of 227 bits unreachable. An independent measurement on a
different fold schedule found 31 of 232. Both are the same phenomenon.

The justification written for the old behavior in D12 was empirically vacuous:
`window[0]` is always the previous fold, whose `Prev` chains back to a root's
`nil`, so after the first fold `Prev` was permanently empty forever.

The rejected alternative "last absorbed bit" was implemented and tested, and
orphans just as badly. Empty is strictly worse than either.

Window-order IDs is the only option of the three D12 named (empty, full
window, last absorbed) that makes the orphan impossible, makes `Prev` mean
what `memory/bit.go` says it means, and lets `Absorbed` keep its
originals-only invariant.

**How it was caught, because the mechanism matters more than the fix.** The
engineer flagged his own justification as suspect while implementing D6,
saying it was a reason constructed to fit existing code. `decision-guard`,
reviewing independently, proved the suspicion correct by execution. Two seats
reached the same weak joint from opposite directions.

**Supersedes.** Only the open question in D12. D12's two **Decided** items
stand unchanged.

---

## D14 — Clarification of D1: "reachable" means discoverable, not merely retrievable

**2026-08-11**

**Status:** tested — this is the property `memory/reach_test.go`'s
`TestEveryStoredBitIsReachableFromTheView` asserts.

D1 was ambiguous in a way nobody noticed until review. Under the weak reading
— an object is reachable if you can retrieve it given its address — D1 was
satisfied even while 13% of the record was undiscoverable (the D13 measurement),
because the store did still hold every orphan.

**Binding clarification.** Reachable means discoverable by walking the record
from the view via `Prev` and `Absorbed`, not merely retrievable by someone who
already holds the address. Content addressing makes an ID retrievable; the DAG
is what makes it discoverable. A receipt you cannot navigate to is the exact
failure D1 exists to prevent, and the weak reading would make the record's
claim to be evidence technically defensible rather than true.

**What would change it.** Nothing currently — this is a definition, not a
prediction. It would need revisiting only if the product ever wanted to permit
deliberately-unreachable-but-retained material, which is not the current
design.

---

## D18 — Roadmap redirect: a persona loop replaces the demo, persistence becomes required, forum is the base abstraction, vote budgets, per-view folds, and a research finding handled correctly

**2026-08-11**

**Status:** mixed. (a)–(e) and (g) are asserted — none is built. (f) is
tested: two independent agent passes plus a direct arXiv API check verified
the finding before it was allowed to change anything.

**(a) The demo's mechanism changes; its analysis does not.** The new first
real use: a recent open model, run locally on Tyler's GPU, chatted with under
a persona, where persona coherence is shaped over turns *and across separate
threads* by fold and unfold. Then multiple model personas plus Tyler in one
forum. This replaces D11's `--replay` as the demonstration — replay existed to
answer `scope-adversary`'s objection that every bit on screen was typed by the
human, and a local model writing into the forum live is real non-human
content, which answers that objection more directly. Replay is not deleted;
it demotes to a test fixture. **D11 is not overturned in its analysis** —
one screen, fold-and-unfold is still the whole demonstration — **only in its
mechanism**: the non-human bits now come from a running model instead of a
scripted feed.

**(b) Persistence: non-goal to requirement.** `memory/store.go`'s type doc and
`CLAUDE.md`'s "Current state of the code" both currently say in-memory, no
persistence, "deliberately out of scope." Persona coherence across separate
threads requires the record to survive a process restart, which is
persistence. **This supersedes that stance.** Nothing is built yet — no
storage backend, no format, no migration.

**(c) The forum shape becomes the base abstraction now; forum machinery does
not.** Tyler's argument: a DM is a subreddit with two members, and those
members have many threads — taking the forum shape as the base data model now
costs one concept, taking it later costs two concepts and a migration.
Authorized now: the data model and navigation shape (channels containing
threads). **Not authorized:** multiple communities, membership management,
moderation, cross-posting — forum machinery stays cut, same as D11 cut D3/D4/
D2 from the demo path.

The CEO's addition, not Tyler's: **threads are the first thing in this system
that is actually rankable.** A single transcript poses no ranking question; a
list of threads does. D3 has asserted "ranking is the first self-modification
surface" since D3 was logged with no surface to build it against — this is
that surface.

**(d) Voting: a per-participant vote budget, not one-vote-per-item.** Reddit's
rule works by aggregating millions of users. The CEO's first justification —
a budget as an accommodation for N=1 — was wrong, and Tyler corrected it: the
platform will eventually serve many users, via TUI or API. The reason that
survives the correction: **an agent can vote a million times and a human
cannot**, so unweighted votes drown the human signal, which is the exact
failure D4 and the forum shape exist to prevent. A budget denominated in
something scarce to the voter is what keeps attention allocation honest once
not every participant is human. Not built; recorded so it is not
rediscovered as a fresh idea later.

**(e) One record, many views, each folding on its own budget.** Tyler
observed fold parameters should be tunable per model context window. Following
it through: `coolAt` (`tui/tui.go`) counts *bits*; a context budget is
denominated in *tokens*; and there are two budgets currently collapsed into
one number — the screen's (~30 rows) and a model's (tokens). `memory/view.go`'s
`View` is already a projection over `Store`, so the human's view and a
persona's view are two views of one record, folding on different schedules.
This is the first thing that cashes D1's record/view split for something
other than display.

**Two cautions, recorded explicitly so this is not mistaken for more than it
is.** This is parameter tuning, and D2 says tuning is *not*
self-modification — communities differing only by fold schedule "differ in
degree, never in kind" (D2's own words). It must not be read as D2 arriving.
And the fold must stay the mechanism, not similarity search, or the design
quietly becomes the opaque-retrieval thing this project defines itself
against.

**(f) A research finding, verified before it was allowed to matter.** A
research agent reported ContextEcho (arXiv 2605.24279) as showing in-session
compaction does not reset persona drift, with an ~80-token identity anchor as
the fix — a direct hazard to the thesis, since our fold *is* compaction. A
second agent was dispatched to verify rather than to design around it.

Result: both papers are real and correctly cited, but the finding is
materially weaker than reported. The anchor "acts as a generic prior reset or
compliance amplifier rather than a drift-specific antidote" — the authors say
so twice, and the first report dropped it. The compaction result rests on 20
crossings, 5 events, 4 Anthropic models, one session, no significance test;
the authors never put it stronger than "not *reliably*." The paper's scope is
agentic coding throughout — its "tool-free chat" condition, which is what made
it sound like our case, is a formatting-compliance stressor on a coding-session
prefix, not persona conversation.

**Ruling: treat as a well-sourced prior, not as a result about this
architecture.** Importing a finding asserted in another domain as though it
were tested here is exactly what this log exists to prevent.

Two things stand independently of the paper, and are recorded as design
constraints regardless of the paper's weight. `Compaction` (`memory/cool.go`)
carries counts, a span, handles, kinds and a word bag — a statistical artifact
built for the screen, close to useless handed to a model as context — so
per-view folds (e) must produce different *things*, not merely fold on
different schedules. And an 80-token anchor is cheap enough to include,
labeled as a compliance nudge rather than a cure.

**(g) A capability the append-only design gives us, not yet used.** Because
nothing is deleted and every original stays reachable (D1, D14), persona
drift can be measured against what a persona verbatim said 500 turns ago
rather than against a summary of it. Recorded as an opportunity, not a plan.

**What would change any of this.** (a)/(c): a local model that cannot hold a
persona at all, which would mean the loop needs fixing before ranking or
multi-persona is worth building toward. (b): none — persistence is now
required regardless of what storage engine is chosen. (d): evidence that a
scarce-budget vote is gameable in a way one-vote-per-item is not. (f): a
result actually measured against this architecture, tool-free, across
sessions, with a significance test — that would be new evidence, not a
relitigation.

---

## D19 — Two corrections to the record: D18 miscited a file, and D11's forward-looking line is spent

**2026-08-11**

**Status:** tested — both corrections were found by `decision-guard` executing
the checks rather than reading, and both were verified against the files by
`grep` before this entry was written.

This log is append-only, so a wrong statement in a published entry is corrected
here rather than edited there. That is the mechanism working, not an
embarrassment to be minimised — but the errors are worth stating precisely,
because both are the *same* error, and it is the one this project is least
allowed to make.

**(a) D18(b) put words in a file's mouth.** D18(b) wrote that "`memory/store.go`'s
type doc and `CLAUDE.md`'s 'Current state of the code' both currently say
in-memory, no persistence, 'deliberately out of scope.'" The quoted phrase is
not in `memory/store.go` and never has been. That file's type doc says:

> Persistence is not here yet. When it arrives it goes behind this type, which
> is why the type exists at all rather than callers passing a map around.

Which is not the non-goal stance D18(b) claimed to be overturning — it
anticipates persistence and explains the type's existence in those terms. The
"deliberately out of scope" wording was `CLAUDE.md`'s alone. **D18(b)'s
decision stands** — persistence is a requirement, not a non-goal — but it
overturned less than it said it did, and it attributed a phrase to a file that
did not contain it. `CLAUDE.md`'s bullet has been corrected in both trees.

**(b) D11's most-quoted line is spent, and it should stop being cited as
current.** D11 recorded `scope-adversary`'s argument that the most
differentiating interaction in the product was a key that does not exist. That
key exists: it is `ctrl+u`, it resolves a scar's receipt live from the store,
and `tui/unfold.go` is where it lives. D11's *analysis* stands and D18(a) left
it standing. This clause retires one sentence of its evidence, nothing more.

**Why both of these are one error.** A citation nobody checks is indistinguishable
from a citation that is right, until someone checks. This repo's public pitch is
that its claims are checkable against files a reader can open, and both errors
survived precisely because they were plausible. The general lesson, recorded so
it outlives this entry: **quote by reading the file, not by remembering it,**
and treat a quotation mark as a claim requiring verification rather than as
punctuation.

**What would change it.** Nothing — these are corrections of fact, not
judgments. If either turns out to be wrong, the fix is another appended entry,
never an edit to this one.

---
