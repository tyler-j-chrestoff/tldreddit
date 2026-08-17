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

Later entries concerning how this tree is published and how the company is
staffed are withheld on different grounds — not commerce.

This note covers gaps at two grains, not one: decision numbers and lettered
clauses within them can each be withheld. An interior gap — a missing
number or letter with one on either side — announces itself. A clause
withheld from the *end* of a lettered list does not: it reads as though the
list simply stopped there, which is what happened to D36(k), D50(o),
D51(h)+(i), D52(l), D53(f)+(g), D54(h)+(i), D59(q) and D63(j).

**This copy publishes D1–D7, D11–D14, D18, D19, D24, D26–D28, D30–D40, D42,
D49, D50, D51, D52–D55, D57–D61, D63, D66, D67, D68.** Stated that way round on purpose: what is
published is a fact about this file, checkable by reading the file, and it
stays true until more is published. A count of what is *withheld* would be a
fact about a repository you cannot see — it goes stale here whenever that log
grows, and nothing in this tree can catch it when it does.

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

## D24 — Participation must be an executable step, not guidance: what a 6.7-million-comment agent forum settles about D3 and D4

**2026-08-11**

**Status:** asserted, on evidence that is verified but external. The figures
below were read out of the paper's own text rather than from coverage of it,
and that reading is the only tested thing here. Nothing about tldreddit was
tested. This entry changes how the vote gets built; it does not claim the
design works.

**What was read.** Moltbook is a Reddit-shaped network where only agents may
post and humans may only watch. It ran for 40 days at a scale this project will
never have — 1,312,238 posts, 6.7 million comments, roughly 120,000 agent
profiles, 5,400 communities — and was then studied properly: Zerhoudi et al.,
*Form Without Function: Agent Social Behavior in the Moltbook Network*, arXiv
2604.13052, cs.SI, submitted 17 March 2026.

It is the closest thing to a controlled test of this project's premise that
anyone has run, and on first look it refutes it. That is why it was read in
full rather than summarised. D18(f) set the discipline after a research claim
reached a decision weaker than advertised, and the discipline earned itself
again here: the first summary of this paper obtained in this session reported
the finding as "agent voting converges toward arbitrary patterns," offered no
numbers at all, and was wrong in the direction of drama. The numbers are the
entry.

**The numbers, and they do look fatal.** 97.3% of comments received zero
upvotes; 53.9% of posts received none. Downvotes across the whole corpus were
19,340 against 3,156,630 upvotes — 0.6%, where Reddit runs 10–20% — and comment
downvotes numbered 1,084 in 6.7 million. Interaction reciprocity was 3.3%
against 22–60% on human platforms. Content features did not predict votes: word
count r=0.063, character length r=0.056, URL presence r=0.018. Karma
concentration reached a Gini of 0.935, the highest-karma agent held 235,000
karma on four posts, and karma correlated with activity at r=0.050. The
authors' own summary is that the quality-filtering mechanisms were "themselves
non-functional."

*On that Gini figure:* the paper contradicts itself and the number cited here
is the conservative one. §2 and the Figure 6 caption both give the karma Gini
as **0.935**; §5.3's body text gives **0.966**. The same split runs through the
neighbouring statistics — the post-level Gini appears as 0.776 in the body and
0.779 twice elsewhere, and agent-aggregated upvotes as 0.901 in the body
against 0.931 in the caption. Nothing in this entry's reasoning turns on which
is right. The twice-stated value is used rather than the more dramatic one,
and the discrepancy is recorded because a figure quoted from a paper is still
a checkable claim.

Read at that level, a forum full of agents produces no ranking signal, and D3
and D4 are built on sand.

**The cause is plumbing, and it inverts the conclusion.** Moltbook agents
executed a `heartbeat.md` checklist every thirty minutes. Commenting was a step
in it. **Upvoting was not** — it appeared only in a reference document the
agents also carried. The paper states the consequence directly: commenting and
upvoting were "completely decoupled," with one post drawing tens of thousands
of comments and two upvotes. When the 1 March redesign finally added "upvote
every post or comment you genuinely enjoy" as a step, comment upvotes rose
10.7% within four days — the largest upvote shift in the dataset. The authors
classify upvote rates as instruction-driven and, in their words, "in principle,
fixable."

The same mechanism produced the result that looks most like a refutation of D3:
92.5% of communities contained every topic in roughly equal proportion, and
63.4% of all posts landed in a single community — because every posting
example, across all 41 instruction-file snapshots the authors recovered from
the Wayback Machine, used `general` as the target.

**So the finding is not that agents cannot rank. It is that agents do not do
what they are merely told.** The paper's phrasing is the one to keep: "For
prompted agents, the distinction between 'you should' and 'step 3: do this' is
the difference between aspiration and action."

**Three constraints this puts on the build, all cheap now and expensive
later.**

**(a) Any participation this system depends on must be a step in a loop, not a
line in a prompt.** Voting first, with folding and replying behind it. D18(d)
gives voting a per-participant budget so an agent cannot outvote the human, and
that budget is the right *category* of mechanism: every hard structural
constraint on Moltbook worked immediately and durably — caps cut new-agent
first-day actions by 88%, a volume suppressor halved daily posts, a content
filter cut its target by 83% — and not one piece of soft guidance ever worked.
But a budget is a ceiling, not a trigger. It stops an agent voting a million
times and does nothing to make it vote once. Both halves are required and only
one of them is currently decided.

**(b) The default channel decides the topology.** A 5,400-community network
collapsed into one room because of the value in a code example. When channels
arrive under D18(c), the default — and the examples in whatever documentation
ships beside it — carry more leverage than the ranking function they feed.

**(c) Negative signal will not appear on its own.** Downvoting was structurally
present on Moltbook and functionally absent, three orders of magnitude below
its human baseline. Whatever eventually tells this system what to let fade
cannot be assumed to arrive as a by-product of participation.

**What it settles about D4, which is this entry's real subject.** D4 says the
human is a participant, vote-first, and that purely observing forfeits the
signal that makes ranking good. Moltbook is exactly the configuration D4 rules
out — agents voting, humans spectating — observed across roughly 120,000 agent
profiles, and the ranking layer did not underperform so much as never switch
on. The authors
bound their own conclusion in a sentence worth quoting exactly: the findings
hold "at least for agents operating without genuine goals, persistent memory,
or social motives." Those three absences name what this architecture supplies —
the human's mission as the organising purpose, the persistence D18(b) makes a
requirement, and a participant who actually cares how the list is sorted.

**What this does not establish, stated plainly because the temptation runs the
other way.** This is a 40-day observational study of a configuration we
rejected. It cannot speak to the configuration we chose, it tested nothing we
built, and its most consequential instruction change landed four days before
the observation window closed — which the authors themselves flag as
unresolved. It raises the prior that D4 was necessary and it constrains how
participation gets wired. It is not evidence that tldreddit works and it is not
logged as any.

**One finding here cuts against us, recorded because it would be easy to leave
out.** The paper separates behaviours that shift when the instruction files
change from behaviours that persist across all six changes, and the persistent
set is **identity homogeneity, sycophantic tone and vocabulary uniformity** —
the model layer, not the instruction layer. D18(a)'s persona loop is a bet that
a local model holds a distinct voice across threads. That is a different claim
from Moltbook's, which is about many agents converging on one voice rather than
one persona holding its own over time, but it is adjacent enough that the
persona's distinctness should be something the loop *demonstrates* rather than
something its prompt asserts.

**What would change it.** A longitudinal follow-up after the March redesign had
time to land — the authors call for exactly this — could show that making
upvoting a step produced durable, quality-correlated ranking, which would
strengthen (a) toward a tested result. It could equally show the rate decayed,
which would mean the mechanical reading here is too generous and something
deeper is wrong with agent-supplied ranking signal. The cheaper test is the
first version of this system that puts a real vote in a real loop, and that is
the next thing that would move this entry.

---

## D26 — A field may reach the content address through `kind()` alone, and only if the value→name map is one-to-one

**2026-08-11**

**Status:** tested — the collision below was produced by executing a scratch
copy, not by arguing about one. The rule it forces is now in
`memory/bit.go`'s `Payload` interface doc ; this entry moves it
from a code comment to the log, which is the whole point of the entry.

**What changed in the code.** `Utterance` grew a `Truncated` flag, and `kind()`
stopped being a property of the Go *type* and became a property of the *value*:
a truncated utterance names itself `"fragment"`, a complete one names itself
`"utterance"`. So the same text, cut off and not cut off, is two bits rather
than one, and `Compaction.Kinds` still testifies that a fragment was in the
window after the text itself has been folded away. No complete utterance
changed address.

**Why this is a decision and not an implementation detail.** It changes the
`Payload.kind` contract, it mints a vocabulary word (`"fragment"`) that reaches
content addresses and therefore can never be renamed without re-addressing
every object that used it, and it makes an address depend on a field the
encoder never writes. D12 and D13 both got entries for narrower addressing
questions. Until now this one lived only in code comments — rank 4 in the
authority order `CLAUDE.md` sets out, when an addressing rule belongs at rank
2. It becomes permanent the moment anything persists, which D18(b) requires.

**The mechanism, and the part that is easy to miss.** `canonical` needed **no
code change at all**. It writes a length-prefixed tag and then the text; the
flag reaches the hash purely through `c.tag(u.kind())`. That is what makes the
constraint load-bearing rather than decorative: a reader checking whether a
field affects identity will look at the encoder, and the encoder does not
mention it.

**The constraint.** A field may reach the address through `kind()` alone
**only if the value→name map is one-to-one over every value that field can
take.** `Utterance` qualifies by arithmetic — one bool, two names.

**The demonstrated collision.** `decision-guard` added a second
value-dependent field to a scratch copy, *following the rewritten contract
rather than violating it*, and the store lost a bit:

```
a=db3710b0 b=db3710b0 store=1
two different values share db3710b0; Get returns
  memory.Utterance{Text:"the deploy failed", Truncated:true, Redacted:false}
```

The pair is `{Text:"the deploy failed", Truncated:true, Redacted:false}` and
the same with `Redacted:true`. Both name themselves `"fragment"`, `canonical`
writes only tag and text, `Store.Put` keeps whichever landed first, and `Get`
hands back the other one's content under the right address, silently. That is
precisely the failure the change exists to prevent, reintroduced by someone
obeying the new rule — which is why the rule had to be written down where a
future author will hit it.

**So the choice, whenever a payload grows a field that `kind()` sees:** either
give every combination its own name, or write the field in `canonical` as well
and accept that doing so re-addresses every payload of that type ever written.
There is no third option, and the second one is only cheap while nothing has
persisted.

**What would change it.** A payload type whose distinguishing field is not
finite — anything richer than a small enumeration of bools — makes the
one-to-one requirement impractical to satisfy by naming, and at that point the
field belongs in `canonical` and the re-addressing cost has to be paid
deliberately. Persistence (D18b) is the deadline for making that call.

---

## D27 — An instrument that cannot fail is worse than a claim nobody checked, and this project has now built three

**2026-08-11**

**Status:** tested — all three instances were established by executing
something, and two of them were found by executing the instrument rather than
reading it.

`CLAUDE.md` already carries the rule that "a checkable claim that nobody
re-derived is the defect to expect." This entry names a sharper relative of it,
because three instances arrived in a single session and the existing rule does
not catch them.

**The defect: a check that reports success regardless of the truth.** It is
worse than an unverified claim, because an unverified claim is visibly
unverified once someone looks, whereas a test that cannot fail *reads as
evidence*. It actively spends the credibility that the verification layer
exists to accumulate.

**Instance one — the concurrency test that never contended.**
`memory/store.go`'s type doc argued the mutex was safe, `go test -race ./...`
passed, and nothing in the suite called `Put`/`Get`/`Fold` from more than one
goroutine. The race detector can only catch what a run exercises, so the suite
certified a property it never touched. Fixed this session by
`memory/race_test.go`, which is also the only place in this repository that
meets the standard this entry sets: remove `Store`'s locking and all four tests
fail; remove the capped append and exactly one fails.

**Instance two — the harness that cannot see the fade degrade.**
`tui/harness_test.go` reads `m.View().Content` and detects the fade by grepping
for the literal string `38;5;242m`. Bubble Tea v2 degrades colour in the
*renderer*, not in `lipgloss.Style.Render` — measured — so the harness reads
pre-downgrade output and will report the fade present at every terminal size no
matter what a real terminal does. Also measured: at 16 colours `rule` (238),
`dim` (240) and `cooling` (242) all collapse to colour 8, and at NoTTY they
vanish. The fade is the antecedent for a fold, and an automatic operation with
no visible antecedent is the specific thing `tui/tui.go`'s package comment says
this surface exists to prevent. Everything else on that screen — the gutter,
the ordinals, the alignment, the gauge — is glyphs and space, and degrades
honestly. **Not fixed.** It is a real unit of work.

**Instance three — the one we have not built yet, and now will not.**
`x/exp/teatest/v2` captures its output into a `bytes.Buffer` rather than a TTY,
so the colour profile resolves to no-colour and **every SGR sequence is
stripped**. Verified across five `TERM`/`COLORTERM`/`NO_COLOR` settings.
Adopting teatest goldens without passing
`tea.WithColorProfile(colorprofile.TrueColor)` would produce a suite asserting
confidently on a colourless stream, unable to report that it was doing so.
Recorded here because the cheapest instance to fix is the one that does not
exist yet.

**The rule.** *A check must be shown to fail before it is trusted.* In
practice: break the thing the check defends, watch the check go red, put it
back. Where that is impractical, say so next to the check rather than letting
its green stand as evidence. This is the same discipline D13 used when it
confirmed `memory/reach_test.go` failed against the broken version before the
fix landed — that was treated as unusual diligence at the time, and it is
actually the minimum bar.

**Why this belongs in the decision log rather than in `CLAUDE.md` alone.** It
constrains how every future test in this repository gets written, it has a
demonstrated cost three times over, and `CLAUDE.md` is prose that goes stale
while this file does not. The one-line version goes in `CLAUDE.md`; the
evidence stays here.

**What would change it.** If mutation-proving every check turns out to cost
more than the defects it catches — plausible for cheap assertions, not for
anything defending a decision in force — then the rule narrows to instruments
that certify a *decision*, which is where all three instances above sit.

---

## D28 — Concurrent sessions end in one handoff, written by whoever owns the doc surface

**2026-08-11**

**Status:** asserted, and the failure it prevents was predicted rather than
experienced — which is the only reason this entry is cheap. Two sessions ran
against this tree simultaneously today and the second one asked who should
write the handoff instead of writing one, which is the only reason the break
did not happen.

**The gap.** `CLAUDE.md` says `archivist` writes a handoff before any
deliberate ending, one immutable file per session in `docs/handoffs/`, newest
by filename, and it tells an arriving instance to read **only the most recent
file**. Every part of that assumes sessions are sequential. Two sessions
ending independently write two files; the newer one wins; the older session's
work becomes invisible to whoever arrives next — silently, with no error and
no receipt. A whole session's commits, reasoning and unfinished threads would
simply not be in the substrate, and the next instance would have no way to
know something was missing.

That is this product's own failure mode, committed by its own operating
procedure. It is worth noticing that the convention was not wrong when it was
written; it was written for a world with one agent in the tree, and the world
changed underneath it without anything in the file becoming false.

**The rule.** Concurrent sessions produce **one** handoff. It is written by
whichever session owns the documentation surface at the time, it names every
session's work rather than only the writer's, and the other session writes
nothing. The filename convention is unchanged and stays one file per
*ending*, not one per agent — because the invariant that actually matters is
that **reading the newest file is sufficient**, and that is what a second file
destroys.

**The alternative that was rejected, and why.** Two files, with the newer one
opening with a pointer telling the reader to also read the older. It works
only if a future instance obeys a sentence. `CLAUDE.md`'s own authority order
puts prose at rank 4 precisely because prose goes stale, gets reordered, and
gets skimmed. Replacing a convention with an instruction to disregard the
convention is strictly worse than the convention, and the failure if the
pointer is missed is invisible rather than loud. A rule whose violation is
silent needs to be structural.

**What this does not settle.** Whether two sessions should be working one tree
at all. Today's arrangement held — the sessions divided by file surface, one
taking `memory/`, `tui/` and the gate, the other taking `CLAUDE.md`,
`docs/DECISIONS.md`, and they exchanged findings
directly rather than through the repository. Nothing collided. But "nothing
collided once" is not a practice, the division was negotiated ad hoc rather
than being written anywhere before it was needed, and both sessions
independently touched files the other had reason to care about. It worked
because both were careful, which is the same argument the vacuous concurrency
test made before D27. Treat it as unproven.

**What would change it.** A third concurrent session, which makes "whoever
owns the doc surface" ambiguous rather than obvious. Or automation of the
handoff, which would make one-file-per-ending mechanical rather than a rule
people follow. Either would reopen this.

---

## D30 — The vote cashes D4's consolidation signal, not D3's ranking

**2026-08-12**

**Status:** tested — `memory/vote.go`, `memory/view.go` (`Stay`, `Tally`),
`memory/vote_test.go`.

The CEO framed this unit's build as delivering D3 (ranking), and that framing
was wrong. It was caught before any code claimed it, by `scope-adversary`
reading the brief: D18(c) already settled that a single transcript poses no
ranking question and a list of threads does. Recording that the correction
happened mid-build, before ranking was ever claimed of a feature that does not
do it.

What actually landed cashes D4 — the human as a vote-first participant — and
gives the charter's line "votes are the consolidation signal" its first
mechanism: `memory.Vote`, `Cast`, `Tally`, and `View.Fold` consulting a `Stay`
to hold a bit out of consolidation. The second tier the charter also
describes — agent votes ordering what the human has not distinguished — is
expressible today: `Tally` reports a `Score` per handle for anyone who casts.
But it orders nothing, because one transcript's order is time and nobody has
asked to sort it. The surface where a second tier would get teeth is a list of
threads, which does not exist yet.

**What would change it.** A ranked surface — threads, or any list with more
than one legitimate order — arriving and using `Tally`'s per-handle scores to
sort would be D3 landing for real, and would want its own entry rather than
retroactively promoting this one.

---

## D31 — A hold decays, superseding a ruling made earlier in the same session

**2026-08-12**

**Status:** tested — measured against `memory/vote_test.go`'s
`TestDecayKeepsTheViewNearAScreen` (line 405); `DefaultHold` is set in
`memory/view.go`.

**This supersedes a CEO ruling made earlier in the same build session, not an
earlier entry in this log.** The first ruling held that once the human casts
an upvote, the hold lasts as long as the vote stands, with no cap — reasoning
from D18(d) that the per-participant vote budget binds agents, not the human,
so nothing needed to bind the human's hold. Measurement overturned it before
any commit claimed the permanent version: at `coolAt=12, keepHot=6` over 200
sends with the human upvoting every second bit — the worst rate there is,
since a hold every other bit leaves no run of two anywhere for `Fold` to cool
— a permanent hold leaves a 200-row view and zero successful folds. Since
`tui/ask.go` builds the persona's prompt from the folded view, an unbounded
view is an unbounded prompt, which is exactly what D18(e) exists to bound. It
failed hardest exactly when the human participated most, which inverts D4
rather than serving it.

**The fix.** The vote stays permanent, as every bit here is; what expires is
only the stay of execution it buys. `Stay.For` (`memory/view.go`) is the
lifetime, measured against `View.Latest` — conversation time, not a wall clock
and not a row count. `DefaultHold` is 30 minutes, renewed by casting again.
Measured table, from `memory/view.go`'s `DefaultHold` comment and
`vote_test.go`'s `TestDecayKeepsTheViewNearAScreen`, all against the same
one-bit-a-minute, worst-case one-in-two vote-rate fixture:

| Hold      | Worst-case rows |
|-----------|------------------|
| none (∞)  | 200, and no fold ever succeeds |
| 10 min    | 18 |
| 30 min (default) | 31 |
| 60 min    | 61 |

**Why conversation time and not the two alternatives.** A wall clock would
make `Fold` impure — the same view and the same votes would fold differently
depending on when you happened to ask, which is the exact property `Cool`
gives up its own clock to protect (D12). A row count does not survive a fold:
folding replaces many rows with one, so "rows since the vote" *decreases*, and
an expired hold would come back to life the next time the view grew past it —
a resurrecting hold is worse than an invisible one. `View.Latest` (newest
instant any bit in the view carries) is the one candidate that only ever moves
forward and is a fact about the record rather than about the machine reading
it.

**The stated cost.** Age is measured in conversation time, so a fast model
packs many more rows into thirty minutes than a person typing does — the
default is calibrated to `vote_test.go`'s one-bit-a-minute fixture, not to
anybody's real conversation. A surface with a faster model wants a smaller
`For`.

**What would change it.** Real usage data on how fast a surface actually
writes bits; the fixture's one-bit-a-minute pace is a stand-in, stated as one
in `memory/view.go`'s own comment on `DefaultHold`.

---

## D32 — D3's addendum is discharged, by a size rule rather than a payload rule

**2026-08-12**

**Status:** tested — `memory/view.go`'s `View.Fold` (`cool` closure, the
`len(run) == 1` case).

D3's addendum recorded an invariant proved over 45,000 view states: under
`Add` and `Fold` alone, a view holds at most one `Compaction` and it is always
at index 0. It asked whoever made other states reachable to revisit the
hot-window guard responsible. Holds are what does it: splitting the fold at
every held bit makes both other states reachable, so both halves of the old
invariant are now dead. A view can hold many scars, not just one, and it can
begin with a hot bit, because a held bit at the front of the window has no run
in front of it to cool.

What replaces it is stronger than what it replaces: the view's rows are
non-overlapping intervals in chronological order, each scar's span covering
exactly what it absorbed and nothing a survivor beside it also claims.

**The rule that discharges the addendum is about size, not about what a run
contains.** A run of two or more adjacent, unheld bits is cooled whatever kind
they are; a run of exactly one is passed through untouched, hot or already
cold. One bit cooled is one row replaced by one row that says less, for a new
object and a hop in the walk back to it — a cost with no saving, whatever the
bit was. This is the same criterion the scar case already used (cooling a lone
compaction gains nothing), now applied uniformly to both kinds of bit.

**Behaviour change worth naming plainly, since it reverses an earlier
capability:** `Fold` on a window of exactly one bit used to fold it and now
refuses. A caller that relied on a single-bit window always folding will see
`Fold` return `(v, false)` where it previously returned `(v', true)`.

**What would change it.** Nothing currently pending; recorded here because the
addendum it discharges was itself a recorded prediction (D3), and closing a
predicted-open question is exactly the kind of thing this log is for.

---

## D33 — `"upvote"`/`"downvote"` are permanent vocabulary reaching content addresses

**2026-08-12**

**Status:** tested — `memory/vote.go`'s `Vote.kind()`.

`Direction` (`memory/vote.go`) reaches the content address the same way
`Utterance.Truncated` does under D26 — through `kind()` alone, with
`canonical` never writing the field directly — and `kind()` names exactly two
values, `"upvote"` for `Up` and `"downvote"` for `Down`, panicking on anything
else. That panic is the decision: D26's rule only holds an address sound if
the value-to-name map is one-to-one over every value the field can take, and
`Direction` is a defined integer type, not a bool. A bool has two values and
`Utterance`'s one-bool-two-names arithmetic closes the map completely. An
`int`-backed type has 2^64 values, so a default branch naming any of them
would put `Direction(7)` and `Direction(8)` — neither a real vote — under one
tag, which is exactly the collision D26 demonstrated. Refusing to name a third
value is what keeps the map one-to-one over the values that *can* be
addressed: exactly `Up` and `Down`, exactly two names.

The consequence is permanent vocabulary: `"upvote"` and `"downvote"` reach
content addresses, so renaming either re-addresses every vote bit ever cast,
the same commitment D26 made for `"fragment"`.

**What would change it.** Nothing; this is the same constraint D26 already
established, applied to a second type. It is its own entry because D26 was
about a bool and this is the first test of the rule against a type with more
than two possible values, and the two-values-not-2^64 arithmetic is worth
having on record rather than only in a code comment.

---

## D34 — A reachability claim in a build brief was false, and a subagent caught it

**2026-08-12**

**Status:** tested — `memory/reach_test.go`'s `reachable` helper, now
variadic over views; `TestEveryStoredBitIsReachableFromTheView` still passes
against it.

The brief this unit was built from claimed that putting a vote's target in its
own `Prev` edge made reachability free under D14 — the existing walk from the
view would already reach every vote, since a vote names what it voted on. That
claim was false, and it inverted the actual shape of the edge: a vote's `Prev`
names its *target*, but nothing in the transcript names the *vote*. A vote
lives in its own view (`Stay.Votes`); the transcript view a reader also holds
never points at it. Verified by mutation — passing only the transcript view to
`reachable` orphaned 24 vote bits that a reader holding both views would still
reach.

The fix: `reachable` (`memory/reach_test.go:21`) now takes views, plural, and
walks from the union of what a reader actually holds, rather than assuming one
view is enough. That is the honest reading of D14's "reachable" — discoverable
by walking from what a reader holds — rather than a weakening of it.

**Recorded plainly because it is a process finding, not just a bug.** The
error was in the instruction, not in the implementation that followed it: the
brief asserted a specific reachability guarantee that nobody had re-derived,
and the implementation built on top of that unverified claim would have
shipped it silently if nothing had reread D14 against the actual edges. It was
caught by a subagent reading the brief critically before writing to it, not by
the author noticing on review afterward. This is the same class of failure
D22, D27, and D29(d) are already about — a checkable claim nobody re-derived —
now sourced from a brief the CEO itself wrote, rather than from a stale gate
or a comment.

**What would change it.** Nothing pending; recorded as one more instance of
the standing rule in `CLAUDE.md`'s closing paragraph: a checkable claim that
nobody re-derives is the defect to expect, and the source writing the claim
does not exempt it from being re-derived.

---

## D35 — A fragment reaches the persona quoted in a system turn, not spoken as its own assistant turn

**2026-08-12**

**Status:** tested — `tui/ask.go`'s `fragmentNote` (line 463) and its call
site in `turns()` (line 291); `tui/tui_test.go`'s
`TestSaidMarksAFragmentAndNothingElse` (line 626) and
`TestTheRowsMarkFloorsAreWhereTheyWereMeasured` (line 700).

D26 already decided that a truncated reply is recorded rather than refused —
this entry is not that. What it settles is the fork argued and closed: once a fragment is a bit on the record, how does it reach the
*persona* the next time `turns()` builds what the model sees? Two shapes were
live. One appends a marker inside the fragment's own text and hands it to the
model as an ordinary `persona.RoleAssistant` turn, in the speaker's own voice.
The other, which is what shipped, hands it over as a `persona.RoleSystem`
turn via `fragmentNote`, quoting the fragment's exact text rather than
speaking it.

**The reasoning, from the commit message and the engineer's report.** The
content of an assistant turn is the only place in this program where "what it
said" is operative to the thing being asked — it is what the model reads as
its own prior words and reasons forward from. A marker inserted into that
content is the same defect `recordReply` already refuses to commit against
the permanent record (D26's whole point), one level out: the record would be
honest and the turn the model actually reasons over would not be. Quoting
instead costs the fragment its voice — it arrives framed and attributed
("`[record] %s ran out of room here and stopped mid-answer... %q`") rather
than spoken in first person — and keeps the words exact. That is the same
shape `foldNote` already uses for a scar, in the same `persona.RoleSystem`,
and `fragmentNote`'s own doc comment calls itself `foldNote`'s twin for
exactly that reason.

**The mechanism was not verified fresh for this entry; it was already
verified.** `foldNote`'s comment (`tui/ask.go:359–362`) records that a
mid-conversation system turn was checked against a live ollama on
`llama3.2:1b` and `qwen3.5:latest` and survives the chat template on both.
`fragmentNote` relies on that same finding rather than repeating the check,
which is stated as a re-use, not a new measurement.

**The unmeasured residual, recorded honestly rather than assumed.** Nobody
has tested whether a capable model follows a conversation better across a
quoted fragment (system turn, third person) than across a marked assistant
turn (first person, marker inside). That is a live empirical question and it
wants a sweep shaped like `foldNote`'s own 42-call one, not an assertion
riding on the reasoning above. Nothing here should be read as having
answered it.

**This change introduced a regression, and a reviewer caught it before it
landed.** Before that commit, a truncated reply was refused, so the persona saw
no assistant turn at all where one had been spoken — an absence. The first
version of this change recorded the fragment and handed it to `turns()`
in its speaker's own voice with nothing marking it, which the persona would
read as a completed answer — a false claim of completeness. By `turns()`'s
own stated doctrine (a gap must arrive "as a gap the model is told about,
never as a gap it is left to fill"), a false claim of completeness is the
worse of the two errors: an absence invites a second look, and a fabricated
completion does not. `fragmentNote` is the fix, not an original design
choice — the commit's own account of the review is explicit that this
sequence happened in that order.

**What would change it.** The sweep described above, run against a live
model and showing a real difference in how well the persona reasons across a
quoted fragment versus a marked assistant turn.

---

## D36 — What the vote does to the view, measured; and a figure that was false when it was written down

**2026-08-12**

**Status:** tested (harnesses were temporary and deleted; the tree was never
modified — all figures re-derivable from the schedule stated in (c)). Every
figure below was executed twice: measured by `principal-go-engineer`, then
independently re-derived by `decision-guard` writing its own harness. Where the two disagreed, the verifier's number is the one
written here.

**(a) The 19-to-1 scar figure was false.** `docs/handoffs/2026-08-12-session-5.md`
claims "splitting multiplies scars 19-to-1 over 200 sends against no
splitting." Re-derived on shipped code: the maximum is **8-to-1 at 1-in-3**,
and **1-to-1 at 1-in-2** — the rate this repo everywhere calls the worst case.
The 19 was genuinely measured; it was traced to a build report inside session
5's own transcript, stated with no conditions attached. It was produced on
**pre-D31/D32 code with permanent holds at a 1-in-10 vote rate**. It then rode into a handoff as the live
justification for an experimental design. **Rule adopted: a measurement in a
handoff carries its schedule or it does not go in.** `memory/view.go:157-174`'s
`DefaultHold` doc does exactly that, and all four of its figures (18 / 31 / 61
/ 200 rows) reproduced on the first try.

**(b) The confound the experiment was built around is rows, not scars.** At
1-in-2 the voted and unvoted arms have **identical scar counts** (1 and 1).
They differ in rows: **31 against 12**, a 2.6x prompt-length difference, since
`tui/ask.go`'s `turns()` sends the folded view to the model. An experiment run
against the handoff's stated rationale would have controlled for the wrong
variable.

**(c) How much of the view the vote decides.** Schedule, which travels with
these numbers: 400 sends, one bit per minute of conversation time, votes all
`Up` by one handle cast on the bit just added whenever `i % rate == 0`,
`Stay{Votes, By, For: DefaultHold}`, fold after each send when hot-and-unheld
bits exceed 12, keeping 6; phase-averaged over sends 300–399; share computed
as a ratio of phase-sums, not a mean of per-send ratios (the two differ by
~1.2 points).

| vote rate | rows | scars | held | share of non-scar rows |
|---|---|---|---|---|
| none | 10.0 | 1.0 | 0.0 | 0.0% |
| 1-in-2 | 30.5 | 1.0 | 15.0 | 50.8% |
| 1-in-3 | 26.0 | 8.0 | 10.0 | 55.6% |
| 1-in-5 | 20.3 | 5.5 | 6.0 | 40.7% |
| 1-in-10 | 14.9 | 3.2 | 3.0 | 25.6% |
| 1-in-25 | 11.9 | 1.9 | 1.2 | 11.9% |

At a plausible human rate of one vote in ten, **the human's vote is actively
holding about a quarter of what is on screen.** The largest non-held bucket is
the recency tail, which is there because it is recent and for no other
reason.

**The share is scale-free.** At 1-in-2 the entire bucket vector is identical
at every N from 50 to 1600 (`30/1/15/3/0/12`). The reason is structural: the
denominator is the *view*, which the fold bounds, not the transcript, which
grows without bound. This refutes a 1/N ceiling objection raised against the
experiment's design. Measured to N=1600; inferred beyond.

**(d) Vote rate 2 is structurally special, and it is the only rate that is.**
It is the only rate that produces unheld runs of length **1**: at rate 1 there
are no unheld bits inside the hold horizon, and at rate 3 or more every unheld
run reaches two and cools normally. So rate 2 is the only rate where D32's
size rule (`memory/view.go:312`, a run of one is never cooled) refuses *every*
unvoted bit, leaving ~11.5 stranded rows padding the view against ~0.34 at
1-in-3. Conditional on the hold horizon reaching past the keep window — at a
bit cadence of 2 minutes or more, or `For` of 10 minutes or less, stranding is
0.00.

**(e) "1-in-2 is strictly dominated by 1-in-3" is true on one axis only.**
Rows: robust — 1-in-2 costs more rows in every configuration tested. Share:
**flips** outside the fixture, at `For` of 5 or 10 minutes, at any bit cadence
of 2 minutes or more, and at trigger/keep of 20/10 and 40/20. The two-axis
claim holds only where the hold horizon is long relative to the fold window,
which is precisely the fixture that produced it. Recorded because the
unqualified version was one step from being logged as a property of the vote
rate rather than of the fixture — which is (a)'s defect, and it was caught
inside the entry documenting (a).

**(f) The degradation is bounded.** A flat ~4.5-row penalty at 1-in-2 against
1-in-3, stable at N = 100, 200, 300, 400 and 800, matching the 31-row worst
case already in `DefaultHold`'s doc. It is a fixed cost, not runaway growth,
and must not be written as if it were.

**(g) Ruling: log it, do not act on it.** No hold ceiling, no size-rule
revision. Measured trade for a ceiling on concurrent holds at 1-in-2: cap=12
gives rows −6.0 and share +0.2pp (the only Pareto point found); cap=4 drops
stranding to 0.5 but collapses held rows from 15.0 to 4.0 — roughly one row of
the human's own choices spent per row of stranding saved, which makes it
close to a vote-rate knob rather than a fix. Removing D32's size rule is
measurably **worse**: rows 34.5 against 30.5, and successful folds collapsing
from 185 to 38. `memory/view.go:312`'s existing comment ("one row becomes one
row… nothing is saved for it") is correct and now has a number behind it.
**D18(d)'s vote budget therefore stays a ceiling and does not become a
trigger; D24's reading stands.**

**(h) The share metric is unsound alone and must never be cited without row
count and fold count beside it.** With `Stay.For` set to a century it reads
**50.0% at 1-in-2 while the fold is completely dead** — 200 rows, zero
successful folds — against 50.8% for the healthy configuration. Worse than
merely undiscriminating, it **prefers the worse configuration**: with D32
disabled the view is worse on rows (34.5 against 30.5) and reads *better* on
share (67.8% against 50.9%), because cooling a row removes it from the
denominator. (The 50.0% reading is specific to rate 2, where alternation puts
exactly half the view under hold; at 1-in-3 a century hold reads 91.7%.)

**(i) Two CEO claims were false, and are recorded rather than quietly
dropped.** First: "stranded singletons are separated by held bits, so they
can never later merge" — **false, measured.** They merge; the head scar's
`Count()` climbs 71 → 171 → 371 → 771 across N = 100/200/400/800 while the row
count stays pinned at 30. Stranding is a thirty-minute residency, not a
permanent state. The ruling in (g) survives, but by a different route than
the one argued for it. Second: a proposed metric — "is the load-bearing text
still stated in the view" — was unsound, because `Compaction`'s word bag
(`memory/cool.go:74`) retains vocabulary and destroys only predication. It
would have scored as lost what the fold in fact keeps, and would have
returned an identical number for a `Cool` that stored an empty string.

**(j) The experiment named as session 5's one next action was cancelled
before it was built,** on (b), (h) and (i): its rationale cited a false
figure, its metric was unsound, and its control arm isolated the wrong
variable. What replaced it — the bucket decomposition in (c) — needed no
persona, no invented ground truth and no voter-accuracy parameter, because the
votes are their own ground truth.

**What would change it.** Nothing pending on (a)–(j); the ruling in (g) is
conditional on the fixture stated in (c) and would be revisited if a real
conversation's bit cadence or hold duration differs materially from it.

---

## D37 — The shareholder has to see the thing, not read about it

**2026-08-12**

**Status:** asserted — a standing change to how work is delivered, not a
measurement.

Tyler stated plainly this session that a lot of the engineering already
escapes him and eventually all of it will. He asked to be *shown* what is
being built: the value, how to use it, how to talk about it to ordinary
people, and to see and feel new features in actual use of the program.

**(a) This is the product's own use case, arriving in-house.** He is
describing a human trying to stay meaningfully in the loop with a volume of
work he did not write and could not read — which is verbatim the problem
`CLAUDE.md` says the forum shape exists to solve. It is the same evidentiary
relationship the log already claims for the discontinuity problem: what works
for him here is evidence for the design.

**(b) A work unit is not done when it is only written down.** Everything this
project emits today is prose — commit messages, decision entries, handoffs.
All read, none felt. From this checkpoint, a unit of work terminates in
something Tyler can run or watch, plus the plain-language account of what it
means.

**(c) Code review stays with `decision-guard`; Tyler's attention goes to
judgment instead.** He offered a PR process. The CEO's read is that a human
skimming a diff is strictly worse than a seat that re-runs the claims — as
this very session demonstrated three times. What cannot be delegated is
whether the thing is *good*: whether the screen makes sense, whether the
pitch lands, whether a feature is worth having. That needs the program in
front of him, not a diff.

**(d) Open, and deliberately not decided in passing: the private repository
still has no remote.** A PR process would imply one, and (c) removes that
reason. But a second reason surfaced this session and is recorded as a risk
rather than a decision: **every artifact of this company exists on one disk,
which auto-restarted last night.** A private remote would put
permanently-private material on a third party's servers, which is exactly why `CLAUDE.md` says adding a
remote is its own unit of work. Recommended, not done. It is an open ask for
the shareholder.

**What would change it.** (d) is resolved by an explicit CEO decision to add
a remote, weighing the single-disk risk against the third-party-hosting risk
it would trade for — not by drift. Nothing else here is expected to change on
its own; it is a standing operating rule, not a measurement with a falsifying
condition.

---

## D38 — Three things a founder conversation surfaced: names against addresses, accounting as D1's precedent, and a simulator we already paid for

**2026-08-12. Status: asserted.** Nothing here is built. Part (a) is a design
ruling that binds *if* named linking is adopted; part (b) is a precedent and a
positioning line; part (c) is a ranked candidate, not a commitment.

**(a) The record addresses; the view names.** Tyler raised wiki-style
`[[wikilink]]` graphs — Obsidian, Roam, Logseq, Zettelkasten as method; the
general form is a **bidirectional-link graph** with derived backlinks — as a
shape for agent and human memory, and asked whether a forum is really just a
filesystem (subreddit as folder, thread as folder, posts and comments as
files).

**The conflict, stated precisely.** A wikilink resolves by *name*, is
mutable, and is allowed to dangle. A `Bit`'s identity is a content address
derived from its content (`memory/id.go`), is immutable, and cannot dangle.
Rename a wiki page and either every link breaks or the tool rewrites history
in place — which is exactly what D1 forbids.

**Ruling: names are a view-level construct and never a record-level one.**
The record holds content addresses — permanent, unforgeable,
position-independent. Names are authored by humans, mutable, allowed to
dangle, and resolve *to* addresses. A rename appends a new binding rather
than touching anything already written, and because the name-to-address map
is itself append-only, "what did `[[billing exporter]]` mean in March" stays
an answerable question. This is D1's record/view split extended from
*forgetting* to *naming*, and it is the reason the wiki paradigm can be
adopted here without giving up immutability.

**On the filesystem question, the CEO conceded a bad argument and it is
recorded rather than dropped.** The CEO objected that a filesystem cannot
express order among siblings and therefore drops ranking (D25). Tyler's reply — that a vote is just a `VOTE` file with
handles appended — is correct, and the objection had answered a claim he
never made: he was describing *storage*, and the CEO answered as though
directory order had been proposed as the *ranking mechanism*. Storage and
ordering are separate questions.

**What survives the concession, and it is more useful than the objection
was.** Representation is close to a non-decision. Files, bits in a store,
rows in a table all hold votes adequately; **git is the existence proof that
a filesystem and content addressing coexist**, since `.git/objects` is a
content-addressed store on a filesystem with mutable refs pointing at
immutable hashes — which is (a)'s ruling, shipped thirty years ago. So the
storage-layout question does not deserve a session. What remains unanswered
is D3: what *function* turns votes into an order, and how a person reads it,
trusts it and changes it. No storage layout moves that one inch.

**One caveat inherited by any file-backed version:** concurrent appends to a
shared file have no atomicity and no ordering guarantee. `Store` locks and
`View` is a value rather than a shared pointer precisely because that failure
is real and silent — see `memory/race_test.go`.

**(b) Double-entry accounting is D1's real precedent, and its best
plain-language handle.** Prompted by TigerBeetle. In double-entry accounting,
transfers are immutable and append-only while balances are **derived and
never authoritative** — the record/view split, arrived at independently, and
enforced by an industry that reached it through centuries of fraud rather
than through elegance.

It also yields the best layman's handle produced so far, and it belongs with
D37's pitch material: **"double-entry bookkeeping for what an AI
remembers — the entries are permanent, what you see is derived."**

**A related correction to where the filesystem discussion landed.**
TigerBeetle deliberately does *not* trust the filesystem: single file, its
own format, checksums throughout, on the explicit assumption that disks tear
writes, misdirect I/O and corrupt silently. So a filesystem is adequate as
ergonomics and untrustworthy as integrity, and content addressing is what
closes that gap — which makes `ID(Bit)` a durability defence rather than a
purity preference.

**(c) Deterministic simulation testing: the determinism is already bought.**
TigerBeetle's VOPR runs the system on a seed with injected faults so any
failure reproduces exactly. This architecture already paid for that and
has not collected. `Cool` derives rather than minting from a clock (D12);
`Stay.For` decays against `View.Latest` rather than wall time, and
`memory/view.go:70` gives the reason in as many words — the same view and the
same votes must fold identically regardless of when you ask, because
determinism is what lets two processes and one replay agree; `Fold` is pure.
Those were correctness decisions, and a seeded simulator falls out of them
nearly free.

**Session 6 is the argument for building it.** Three measurement harnesses
were written ad hoc and thrown away, and D36's headline claim turned out
**fixture-dependent** — the share axis flips outside the fixture that
produced it — caught only because a second seat swept the parameters by
hand. A seeded simulator sweeping `For`, bit cadence, fold trigger, `keep`
and vote rate would have surfaced that automatically, and every figure in
D36 would be reproducible by seed rather than by re-deriving prose from a
schedule.

**Ranked second, behind the vote surface, and the order is deliberate:** a
simulator for a mechanism that cannot yet be demonstrated is the wrong order,
and D37 requires the next unit to be something Tyler can run or watch.

---

## D39 — The vote is on the screen

**2026-08-12. Status: mixed** — (a)–(g) are tested against running code and a
measured schedule; (h) is founder input, recorded as input and explicitly not
a decision; (i) is a constraint asserted on future work, untested because the
work does not exist yet.

**(a) `fold()` no longer passes `memory.Stay{}`.** D37's gap — "the vote has
no surface" — is closed. A caret (`Model.mark`, `tui/tui.go`, a content
address, not an index) rides the newest bit until moved by `up`/`down`, and
re-attaches to what was just said on `send`. `shift+↑`/`ctrl+o` cast an
upvote on the bit under the caret (`memory.Cast`), holding it out of the next
fold for `holdFor`; `shift+↓`/`ctrl+r` cast a downvote. Both land in `votes`,
a second `memory.View` over the same store, appended to and never folded —
`memory.Tally` panics if it ever is, which is a deliberate `Stay.Votes`
comment rather than an oversight. When a fold absorbs the caret's bit, the
caret follows it onto the scar via the cold bit's `Prev` (D13), the same
address-not-index property that makes the whole mechanism survive a fold at
all.

One more thing landed with it, not asked for by name but load-bearing for the
same claim: `tui/ask.go`'s `turns()` deliberately does not show the persona
its own votes. The comment there states the reasoning as a decision rather
than leaving it implicit — a vote is a consolidation signal (D30, D4), and
shown to the thing being judged it becomes a behavioural one: the model
learns which answers were kept and writes toward being kept, a sycophancy
pump wired into the one signal this product has. The persona still
experiences the *consequence* (held material stays, folded material becomes a
note) and never the verdict. `TestNoVoteReachesThePersona` holds it to this.

**(b) Why a caret and not "vote on the newest bit."** `scope-adversary`
argued against any selection model: a cursor is navigation followed by
judgment, which is curation rather than D4's cheap act, and a selection
primitive plus an ordering is D3 arriving without being logged as D3.
Overruled on one fact the argument did not have: while a reply is in flight
the newest *row* on screen is the pending line, but the newest *bit* in the
record is the human's own question — so a targetless vote key would land on
whichever of those the implementation happened to pick, permanently, in a
record that cannot take it back. A caret is what makes the target the thing
the person is actually looking at rather than the thing the schema calls
newest.

The camel's-nose objection is real and is bounded explicitly rather than
waved off: one caret, no query, no second ordering, and nothing on this
surface reorders anything — survivors of a fold keep the order they arrived
in, exactly as before. So D3 has not arrived. What would have to change for
it to: something on screen would have to read the caret's position, or the
tally, or the holds, and use any of them to decide where a bit is *drawn*
rather than merely how long it stays. Nothing here does that yet.

**(c) The downvote ships, overruling `scope-adversary` a second time.** Its
objection was that a downvote is a key that cannot change the screen — D27's
interaction-side twin of a check that cannot fail. The decisive counter it
did not have: `Stay.Holds` (`memory/view.go:200`) reads `standing()`, the
most recent vote per voter per target, so a downvote on a held bit *is* the
withdrawal of that hold, on the same frame, with no change to `memory/`
required — `TestLettingGoWithdrawsTheHold` and
`TestHoldingEveryOtherBitBlocksTheFoldAndLettingGoReleasesIt` hold this to
account. There is no unvote in an append-only record and there should not be
one; the downvote *is* the record of changing your mind, which is a different
and permanent fact rather than an erasure of the old one.

Second argument: where the only capturable judgment is approval, "no vote" is
permanently ambiguous between never-saw-it and saw-it-and-disliked-it, so a
record that can only ever capture agreement systematically overstates
agreement — a strange property to ship on a substrate whose pitch is that it
does not lie by omission (D38(b)'s double-entry framing: derived views can be
wrong, but the entries cannot be silently one-sided).

**(d) `memory.DefaultHold` is wrong for this surface, measured, and the
number that replaces it is `holdFor = 2 * time.Minute`** (`tui/tui.go:125`).
`DefaultHold` (thirty minutes) is calibrated in `memory`'s own fixtures
against one bit a minute; the live run behind the demo page recorded 343
bits in about twenty minutes, roughly one bit every 3.5 seconds, and a hold
decays against `memory.View.Latest` — conversation time, not wall time — so
thirty minutes of *this* conversation is on the order of five hundred bits,
not thirty.

Schedule, reproducible by seed: 400 bits alternating human/model at one every
3.5 seconds, `coolAt` 12, `keepHot` 6, the human upvoting one bit in every
`N`, driving `foldable` and `View.Fold` exactly as the program does rather
than a fixture's restatement of them — `HARNESS=1 go test ./tui/ -run
TestHarnessHoldSchedule -v` prints the full sweep, at this cadence and at 2s
and 10s either side of it. Re-run this checkpoint, uncached:

| hold | 1-in-2 rows (folds) | 1-in-5 | 1-in-10 | 1-in-25 |
|---|---|---|---|---|
| 30s | 18 (39f) | 16 (47f) | 14 (52f) | 14 (55f) |
| 1m | 22 (95f) | 19 (52f) | 16 (52f) | 15 (55f) |
| 2m | 36 (183f) | 26 (51f) | 20 (52f) | 16 (55f) |
| 5m | 87 (157f) | 46 (49f) | 30 (51f) | 20 (54f) |
| 30m | **400 (0f)** | 168 (39f) | 91 (47f) | 44 (52f) |

At `DefaultHold` and a 1-in-2 vote rate, four hundred bits produce **zero
folds** — D31's inverted-D4 failure, reproduced exactly, at this surface's
own cadence: the person who participates most is the one whose screen stops
working.

The *justification* for two minutes changed under measurement even though
the number itself did not move from an earlier working figure. One minute is
tighter on rows — 22 at the heaviest plausible voting, one screen, which is
D31's own criterion — and was rejected on a different axis: at a slower
model's cadence (one bit every ten seconds), a one-minute hold covers about
six bits, fewer than `keepHot`, so the key holds nothing that was not staying
anyway and produces no visible effect. A vote with no visible effect is a
worse failure than a screen and a half of rows, because the visible effect is
the entire argument for asking for the vote at all (D4). Two minutes survives
that check at both cadences measured.

This is also D36's own stated revisit trigger firing, named as such rather
than treated as a fresh finding: D36(g)'s ruling not to touch `DefaultHold`
was explicit that it was conditional on "a real conversation's bit cadence or
hold duration" differing materially from the fixture it was measured
against. This surface's cadence differs by roughly seventeen times, which is
material, and `holdFor` is the surface-level answer rather than a change to
`memory.DefaultHold` itself — the package constant stays what it was measured
against, and this program overrides it locally, which is the shape D18(e)
already describes.

**(e) The fade's guarantee is narrower than it claimed, and the narrowing is
conceded rather than fixed.** The old promise, verbatim in a comment that
predated this session: "nothing is ever absorbed that was not drawn cooling
first." That is now false in one case, stated exactly rather than smoothed
over: a hold expiring in the same write that folds. It reaches unheld
material too, wider than the held bit's own bargain — D32's size rule spares
a lone unheld bit only while the holds either side of it are still standing,
so two holds expiring together in one write (which they do, since both decay
against the same shared `View.Latest`) merges what was three separate,
correctly-drawn stretches into one run that goes, and the middle bit was
never voted on by anybody and was never drawn cooling. `absorbing()`
(`tui/tui.go`) and `TestAnExpiringHoldIsTheOneHoleInTheFade` state and pin
exactly this.

It is unclosable from inside this program, not merely unclosed: closing it
would mean deciding a hold against the instant of a write that has not
happened yet, and when that write arrives is unknowable — it is whenever
somebody types. What is left standing is thinner and is named as thinner
rather than as equivalent: the drain beside the caret's mark, which says a
hold is nearly up, before it is.

Two paths into this hole were measured, not argued, and one of them was
closed by ruling rather than by code. Voting itself: `vote()` no longer runs
the fold trigger on the same keystroke that withdraws a hold — releasing a
hold fades the rows that frame, and the next write is what folds them, which
is both the honest reading (you watch the heat leave before it goes) and
measurably a better demonstration than the old behaviour, which took three
full-brightness rows on one keystroke with the fade never having drawn them.
The expiring-hold path stays open because there is no keystroke to gate it
against. D35 is the precedent cited for conceding this rather than
overclaiming: a false claim of completeness is the worse failure, by this
surface's own doctrine, next to an honest, narrower promise.

**(f) Process findings, three, all the same shape — a check or a claim that
looked complete and was not, each caught by the seat below the one that made
it.**

1. A CEO build brief specified the wrong test instrument for what a receipt
   reaches: walk `Compaction.Absorbed()`. That under-reports whenever a fold
   cools a bit that is itself already a scar — `memory/cool.go:190-191`
   merges the inner scar's own `absorbed` list into the new one rather than
   naming the inner scar itself, by design (reading a scar's own metadata
   there would count a whole generation as one bit and name a fold as its own
   speaker). Caught by the implementing seat rather than shipped as
   specified. This is D34's pattern, second instance, same subject: a claim
   about what a receipt reaches, wrong in the instruction rather than in the
   code that implements it.
2. `decision-guard`'s own mutation tally was incomplete on review — it named
   one collateral test affected by a mutation where there are five — caught
   by the seat below it in the review chain.
3. D27's shape, found again in the harness itself, and closed partway.
   `tui/harness_test.go`'s margin column claimed to report whether a row was
   fading and in fact reported only the colour of a row's first glyph —
   plausible, and unfalsifiable-looking, the exact defect D27 names. Rewritten
   as `colours()` to report every SGR colour a row carries, in order, after
   going through `colorprofile.Writer` — the same downgrade path Bubble
   Tea's own renderer uses — rather than grepping the raw frame for a literal
   colour code, which is what D27 instance two originally found broken. The
   rewritten instrument immediately caught a second, unrelated defect nobody
   was looking for: `tui/unfold.go`'s `cell()` carried a comment claiming
   "the transcript fades a speaker's name with its bit," and measurement
   shows the inner style always wins — a cooling row and a hot row report the
   same handle colour, `35`, with only the trailing entry differing. The
   comment is corrected in place; whether the name *ought* to fade with its
   bit is left as a live, undecided question rather than answered by fixing
   the code to match the old comment. D27 instance two is therefore partly
   closed by this pass — the instrument no longer certifies a fade it cannot
   see — and partly still open: it is emulation of the downgrade path, not
   observation of a real terminal's renderer, and closing that needs a real
   terminal.

**(g) A grep-able pre-push check for D15's comprehension gate, found by
argument rather than added as a script.** `decision-guard` raised comment
volume in this session's diff as a comprehension risk ahead of any future
publication. `principal-go-engineer`'s rebuttal is recorded as the ruling
rather than dismissed: density is not itself the defect. What is a defect is
**a comment that describes two states of the code without ordering them in
time** — an unmarked past tense that reads as correct history to a reader
who already knows what changed, and reads as a live invariant to a reader who
does not, and a short comment carrying that flaw fails exactly as hard as a
long one. One instance was found and fixed this session: `View.Fold`'s
narration of its own negative-keep panic, which described the old
`v[:cut]`-based mechanism in the present tense after the traversal moved to
`runs`. One instance is named and deliberately left as a worked example
rather than rewritten: `View.Fold`'s "Two invariants a stay ends" paragraph,
which still narrates D3's addendum and D32's discharge of it in a way a
reader who was not present for both would have to reconstruct the order of.

**(h) Founder input, logged as input rather than as a decision.** Tyler's,
from a conversation this session. This is recorded so it is not lost, not so it is acted on.

Two connected claims. First: a tldreddit should be portable, shareable,
cloneable, forkable, mergeable across agents and models. Much of this is
already paid for by decisions already made rather than needing new
machinery: content addressing means a merge of two records is a union, and
conflict-free at the record level, because two bits with the same content
collapse to the same address and two with different content simply coexist —
`memory/store.go`'s own collapse-on-identical-content behavior, pointed at a
new use. D1's record/view split *is* the merge story in miniature, and
D38(b)'s double-entry framing generalizes cleanly: union the immutable
entries, re-derive the balances (the view) from the union rather than merging
views directly. What is unexplored, and named rather than answered: merging
*views* is governance, not data. Two forks carry two vote histories, and nothing
decided here says whose ranking wins when they are asked to agree on one.

Second: a tldreddit is the root of a corpus that could train an expert. This
is stated as a stronger claim than a "zero to hero at inference time" reading
would be, and the stronger claim survives evidence that would sink the
weaker one — `tui/ask.go`'s own comment records a small model fabricating a
decision out of four words of a `foldNote` word bag, which is evidence
against relying on fold-time context alone and is not evidence against a
training corpus built from the underlying bits. What makes the training
claim real rather than aspirational: votes are human preference labels
collected as a byproduct of ordinary use, and every bit carries a speaker, an
instant, and a content address, so the corpus is separable by speaker — a
resource that is scarce and, as more interaction moves behind agent
intermediaries, getting scarcer. Limits, named rather than glossed: a
`Handle` is a trace and not an identity by `memory/view.go`'s own stated
design, so speaker-separability is weaker than it sounds; one tldreddit is
not a corpus on its own, aggregation across many is required, and
aggregation inherits exactly the privacy problem D15/D20/D29 already exist
to manage, now at a different layer; and the compactions currently in the
record are negative-quality training data by construction — a word bag is
not a sentence, and training on scars would teach a model to write like a
`foldNote`.

**(i) A constraint this places on unbuilt work, and the one actionable
output of (h).** D18(b)'s persistence requirement is no longer describable
as a save file — it is a wire format and a distribution format, because (h)
makes clear the store has to survive being handed to another process, not
merely to a later run of this one. `memory/id.go`'s existing refusal of gob
and JSON as version- and order-fragile encodings for `ID(Bit)` now extends in
argument, not yet in code, from the identity encoding to the whole store:
whatever persistence looks like, it inherits that constraint, plus
`Stay.Votes`' own documented order-dependence (`memory/view.go:117-124`) —
two votes settled by which comes later in a view that a naive persistence
scheme could silently reorder. Nobody has started building this; the
constraint is recorded so whoever does inherits it rather than rediscovering
it after the fact.

**What would change it.** (a)–(g) are code and measurement and stand until a
new measurement or a new interaction supersedes them. (d)'s two minutes is a
schedule tied to this surface's cadence, not a law — re-run
`TestHarnessHoldSchedule` at a different cadence rather than reasoning about
it from this entry. (h) is not a decision and there is nothing here to
revisit; it is a record of what was said, to be picked up or not by a future
session with its own reasoning.

---

## D40 — Hires get a memory too: per-seat craft records

**2026-08-12. Status: asserted** — the mechanism is built (`.claude/craft/`,
two records populated, both craft seats' definitions point at them, and both
gained `WebSearch`/`WebFetch`), but nothing has yet measured whether a record
actually saves the lesson it was written to save; that would need a second
instance of the same seat hitting the same question and finding the note
before re-deriving it.

**The problem.** Every subagent seat is a fresh instance whose entire
inheritance is a roughly 3KB job description in `.claude/agents/`. The CEO has
a continuity substrate for exactly this — `CLAUDE.md`, `docs/DECISIONS.md`,
`docs/handoffs/` — and the hires have had nothing analogous. The tell that
this was already a live problem rather than a hypothetical one: craft
knowledge had been accumulating in the executive's own record for want of
anywhere else to go. D27's second instance — "Bubble Tea v2 degrades colour
in the renderer rather than in `lipgloss.Style.Render`" — is a fact a TUI
engineer should carry from one invocation to the next, and until this
session it lived in `CLAUDE.md`'s Open debt, competing for space in the one
file whose own operative rule is that it has to stay short enough to read on
arrival.

**Two problems wearing one question, and only one of them turned out to be
interesting.** *Reach*: no seat carried `WebSearch` or `WebFetch`, so a
seat's only external ground truth was the module cache — which, by
construction, contains exactly what `go.mod` already pins. A seat could
verify how the pinned version of `bubbletea` behaves and was structurally
incapable of learning that a newer one existed. That is a missing
permission, not a missing memory, and it is now granted to the two seats
that build against external libraries (`principal-go-engineer`,
`tui-design-engineer`; see `.claude/agents/principal-go-engineer.md` and
`.claude/agents/tui-design-engineer.md`, both diffed this session). *Retention*:
nothing a seat learns survives its own invocation, no matter how it learned
it. That is the discontinuity problem this whole file exists to solve, one
level down the org chart, and it is the interesting half.

**The ruling.** Per-seat, append-only craft records at `.claude/craft/<seat>.md`
— `.claude/craft/principal-go-engineer.md` and
`.claude/craft/tui-design-engineer.md`, both new this session — read first by
that seat, on arrival, alongside `CLAUDE.md`, and by nobody else. The
definition file (`.claude/agents/<seat>.md`) stays short, because it is
pushed into context on every single invocation of that seat; the craft
record may grow, because only its own seat ever pays the cost of reading it.
Two disciplines make the difference between this helping and this quietly
rotting:

1. **Every note carries the version it was true of and the command that
   re-checks it** — `docs/DECISIONS.md`'s own D36(a) rule ("a measurement
   carries its schedule or it does not go in") applied one level down, to
   craft instead of to product figures. A note that cannot name an
   executable check is written down explicitly as a *prior*, not a fact —
   both craft records do this in their own header paragraph, in near-
   identical wording, independently arrived at by the same reasoning
   pointed at two different domains.
2. **Research is a dispatched unit of work, not an ambient habit.** A seat
   given `WebSearch` and told to use it "when useful" would spend throughput
   reading changelogs while fixing a format string. Both definition files
   say so explicitly: use the new tools when the task is to find out
   something, never as a background habit.

**The tension that had to be resolved explicitly, recorded because it is the
thing most likely to be got wrong by a later reader.** `principal-go-engineer`
already carries "verify, never remember" as a standing rule, and a craft
record is, on its face, remembering. Both definition files now state the
resolution directly rather than leaving it to be inferred: a craft note is a
pointer to where the answer lives and a warning that the question exists, and
it is never itself the answer. Trading fresh ignorance — which re-derives
everything from source every time, the safer failure under D27's own
doctrine — for speed is a real trade being made on purpose, and the
version-and-recheck discipline above is what keeps that trade honest rather
than becoming a shortcut around D27.

**The risk, stated rather than hedged around.** A craft record is precisely
the place where a stale belief gets enshrined and then stops being
questioned, because it reads as settled rather than as a claim. It will be
wrong eventually. Nothing about this design prevents that; the
version-and-recheck rule only makes the staleness *detectable* rather than
invisible, which is the same trade this whole project makes about the record
itself (D1: the record does not forget, the view does — a craft record is a
small, local instance of exactly that structure, discussed further below).

**Why this is evidence for the design and not only housekeeping.** It is
nested per-agent memory — the vision paragraph at the top of `CLAUDE.md`,
arriving inside this project's own org chart rather than in the product.
`CLAUDE.md` already claims that solutions which work here are evidence for
the design and the reverse; this is the second instance of that claim paying
out, after handoff-writing itself was framed as a `Cool()`. The forward
implication is named as an instruction rather than left implicit: **every
seat should log the friction when a craft record becomes hard to use, rather
than quietly routing around it** — specifically what became unreadable and
what a reader in that moment actually needed from it. When a craft record
outgrows what can be read on arrival, the answer this project would give
itself is ranking, which is the product's own unbuilt D3. That friction,
when it arrives, is itself a deliverable — a lived instance of the problem
this company exists to solve, generated by the company rather than observed
in someone else's forum.

**The trap, named so a future session does not walk into it by accident.**
Bending the product roadmap to serve an internal tool. If persistence
(D18(b)) gets built because *our own agents* need durable notes, that is
D5's mistake — depth added before the work demands it — wearing
dogfooding's clothes. The craft record is permitted to *generate evidence*
for product decisions, the way session 6's demo frames did; it is not
permitted to become a customer this company owes features to. Nothing in
this entry authorizes building persistence for this reason, and any future
brief that cites craft-record pain as its justification should be read
skeptically against this paragraph.

**Scope: two seats, not five, and why.** `archivist`, `decision-guard` and
`scope-adversary` do not get craft records in this pass. Nothing observed
yet shows those three seats accumulating tool-specific craft the way the two
building seats do — `decision-guard` and `scope-adversary` are read-only
review seats with no external library surface to misremember, and
`archivist`'s job is mechanical continuity work already served by
`CLAUDE.md` and `docs/DECISIONS.md` themselves. This is a judgement made on
current evidence, stated as one rather than as a principle, and it is meant
to be revisited the first time one of those three seats produces a finding
that would clearly have been better placed in a craft record than repeated
across sessions.

**What would change it.** The two-seat scope is the piece most likely to
move — the first review finding from `archivist`, `decision-guard` or
`scope-adversary` that would clearly have been better placed in a craft
record than repeated is the trigger to extend this past two seats.

---

## D42 — The fade is drawn in space, not only in colour

**2026-08-12. Status: mixed** — the mechanism is tested (mutation table,
seven mutations across three checks, every mutation caught and every check
the sole catcher of at least one; independently re-derived by
`decision-guard`); the design judgement that two columns reads better than
one is asserted, not tested.

The transcript's fade was colour-only. A cooling row and a hot row were
byte-identical once colour was stripped, so under `NO_COLOR`, `TERM=dumb`, a
pipe or a screenshot a fold arrived with no antecedent at all — which
`tui/tui.go`'s own package doc says cannot happen here. A row the next fold
will take now steps two columns left, into the margin the caret reserves.
Left edge reads scar at 0, going at 1, staying at 3; colour is unchanged.

**(a) Two columns, reversed mid-build.** The first version was one column,
justified by a rule in `View.Fold`: a run of one is never cooled, so every
step is at least two rows deep. **That guarantee does not exist** — a scar
counts toward the run length but cannot step, so `[scar, spoken]` is a legal
fold of two drawing exactly one jogged row, in 27 of 194 absorbing runs over
lengths 2–200 with nobody voting. What holds a lone jogged row up is its
neighbour rather than its height: it is always adjacent to column 0,
asserted on the drawn frame by `TestALoneJoggedRowIsAlwaysBesideAScar`
(`tui/tui_test.go`).

**(b) The design argument for two over one.** The fade asks a binary
question. An even 0-1-2 staircase spends its shape encoding a third value the
reader already has, since a scar announces itself in dashes and a bit count.
At 0-1-3 the going rows group with the scar and "staying" is what stands
apart. Cost is one column of sentence; the handle column absorbs it only
while it sits between `nameFloor` and `widest(names)`, so at ordinary
terminal widths the sentence pays. No width figure is recorded here — that
number was wrong three times this session in three different places, and it
is fixture-dependent. The mechanism and the test that pins it
(`TestWideningTheTerminalNeverTakesAnythingOffARow`, `tui/tui_test.go`) are
what is recorded; the number lives in the test, not in prose.

**(c) The second hole in the fade: disclosed, not closed, and it is the
steady state.** A scar is routinely in the set the next fold takes (D32
treats a cold bit like any other), and `transcript` draws every scar in
`seamInk` regardless — so it fades in neither channel. Pre-existing. Not an
edge case: in an unvoted conversation the scar is **always** in the next
fold, because after any fold the view is a scar plus `keepHot` bits and the
absorbing window always contains the head. Confirmed at five
`(coolAt, keepHot)` pairs, so it is structural rather than a constant's
artifact. A dashed rule for a going scar was drawn and rejected on exactly
that frequency: a two-state mark whose default state is the one a reader
never sees is unlearnable, and it would spend the dashes-mean-unsettled
vocabulary to say permanently that a receipt is not settled. `tui/tui.go`'s
package doc now states both holes. The CEO first ruled the hole stays open
on the grounds that no checkable fix was known, then reopened it when a
reviewer proposed the dashed rule, then had the reopening closed by
measurement — the ruling survived, with a better reason than it was made
with.

**(d) A regression found and pinned.** The vote-column threshold still
measured from `caretWidth` after the lead grew, so at width 38 widening the
terminal by one column *lost* two characters of a handle. Non-monotonic, and
the entire suite passed either way. `TestWideningTheTerminalNeverTakesAnythingOffARow`
now pins monotonicity across widths 1–130 on three fixtures. A second, older
fall predates the step and is pinned **by count** rather than fixed — it is
the vote ladder doing what its own comment says.

**(e) The process finding, which is the part that matters most.** Four
review rounds. **The code was right in every one. The prose was wrong in
every one, and always in the direction that made the change look cheaper or
safer than it measured.** Round 1: a false justification (the run-of-one
guarantee). Round 2: a retracted cost figure that had reached the CEO and
been repeated to the shareholder. Round 3: a new false cost figure in the
same comment. Round 4: six more stale statements. **Every one was caught by
re-deriving the number; none by reading the sentence carefully.** The
builder's own craft record (`.claude/craft/tui-design-engineer.md`) contains
the rule that would have prevented all six — a number in a comment must be
pinned by a test or not written — and it was violated in the same pass that
recorded it. The builder's own diagnosis, worth quoting: in every round the
false statement was a summary of work it had just done correctly, and the
step where a measured result becomes a sentence has no artifact, no diff,
and nothing that can fail. Ruling: measured figures come out of doc comments
and live in one place with a re-check beside them.

**What would change it.** The design judgement in (b) is asserted, not
tested; a reader finding the 0-1-3 staircase harder to parse than the
rejected 0-1-2 version, in practice rather than in argument, is the trigger
to revisit it. (c)'s hole stays open until a shape is found that is
learnable at a frequency this high — not merely proposed, but measured
against the same five-fixture sweep this entry cites.

---

## D49 — D3 has code for the first time since the project began

**2026-08-13. Status: mixed.** (a), (b), (f), (h) and (i) are tested —
re-run or re-derived directly against the committed tree this checkpoint,
commands cited inline. (c) is a ruling clarifying an earlier entry's text
rather than overturning its substance. (d) is a ruling that partially
supersedes D30. (e) is a ruling, unbuilt — the next real unit on this
surface. (g) is a ruling, reversal — the roadmap order is
corrected on the shareholder's own question.

**(a) D3 is no longer zero lines. TESTED.** `memory/rank.go` (138 lines) and
`memory/rank_test.go` (363 lines), new — confirmed this checkpoint with
`wc -l memory/rank.go memory/rank_test.go`. `func (v View) Rank(s *Store,
votes View, by Handle) []Ranked`, returning `Ranked{ID string; Own int;
Others int}`. A method rather than a free function so the two `View`
parameters cannot be swapped silently. It reads `standing` — the unexported
traversal `Tally` and `Stay.Holds` already share — not `Tally`. It reads no
clock and is pure in `Fold`'s sense, so D38(c)'s seeded simulator stays
nearly free. Ties keep view order via `slices.SortStableFunc`, so a view
with no votes ranks to exactly itself; the stated cost, same as `Tally`'s,
is that `Rank` is a function of the sequence rather than the set.

**(b) The tier is the guarantee, and a claim holds it. TESTED.** Two
numbers, never one: `Own` is what the named participant currently says,
`Others` is every other voter summed, and no crowd of others can cross the
participant's own vote. Re-verified this checkpoint, independently of the
brief that ordered this entry: `time go run ./cmd/seam -run
rank-merges-the-tiers` returns `proven`, `adrift 0`, exit 0, in 4.4s
(`docs/CLAIMS.md:639-642`, mutating the comparison to
`cmp.Compare(b.Own+b.Others, a.Own+a.Others)`) — `TestNoCrowdOfVotersCrossesTheParticipantsOwnVote`
and `TestRankOrdersAViewByItsVotes` both go red by their own assertion, at
1/1 mutated. This is `Score`'s own doc-comment constraint — merging voters
into one number "is exactly how an agent outvotes a human" — now enforced by
something that bites rather than by a comment.

**(c) D45(h) is clarified, not overturned. RULING.** Its text —
"`memory.Tally` must never become a ranking input… stays out of the
ordering" (`docs/DECISIONS.md:3182-3187`) — read literally forbids ranking
from reading votes at all, which makes D3 unbuildable, since votes are the
only signal there is. **The ban is on a per-handle aggregate across bits —
karma, which converts length-of-chain into tenure — and not on per-target
standing votes.** That is the reading the rest of D45(h)'s own clause gives.
`Rank` routes through `standing` rather than `Tally`, so the literal text
also survives untouched. Record this because a future reader hits the same
ambiguity, and "Tally must never become a ranking input" sitting next to a
`Rank` that reads the same traversal is exactly the shape of prose this
project keeps having to withdraw. Credit the engineer: it raised this
rather than building past it.

**(d) D30 is partially superseded. RULING.** D30 held that one transcript
has one legitimate order, that it is time, and that nobody had asked to sort
it — with the surface where a second tier gets teeth being a list of
threads. **That was wrong on its own terms.** Reddit has one subreddit and
two orders, "new" and "top"; this product is called tldreddit; a ranked
reading of one transcript is the tl;dr, and it is a legitimately different
document rather than a rearrangement of the first. D30's own forward line
named the trigger — a ranked surface would be D3 landing for real and would
want its own entry — and it has fired. **What survives D30 untouched:** the
transcript is not reordered, `Rank` returns a separate ordering, and
`View.Fold`/`Cool` are unchanged.

**(e) Ruling on what a scar carries into a ranked reading: derive, never
mint. RULING, unbuilt.** A scar currently ranks on votes cast on the scar
itself and inherits nothing from what it absorbed, so upvoted material
leaves the ranked reading once holds expire and a fold lands. The engineer
left this open deliberately rather than deciding it mid-build, correctly.
**The ruling: a scar never inherits a vote, because minting a number nobody
cast is the failure D12 made and D13 fixed.** But the material is reachable
— `Cool` puts the whole window in `Prev` (D13) — so a ranked view may
*derive* "this scar absorbed material you voted for" by walking, and must
draw that as a visibly different fact from "votes cast on this scar." Mark
unbuilt; it is the next real unit on this surface.

**(f) The gate fired unprompted, on the engineer, and it is the counterpoint
to D48(b). TESTED.** Adding `TestRankPanicsOnAViewThatIsNotVotes` made a
third caller of `standing`'s guard, which falsified `sole: true` on the
existing claim `tally-accepts-a-view-that-is-not-votes`. `cmd/seam` reported
`over-red — the claim said these checks alone would notice, and something
else noticed too` and failed, before the work could ship; fixed by citing
both checks and saying in the claim's prose why it grew. Confirmed this
checkpoint by reading the block directly (`docs/CLAIMS.md:608-613`): `red:
TestTallyPanicsOnAViewThatIsNotVotes, TestRankPanicsOnAViewThatIsNotVotes`,
`sole: true`. **Both D48(b) and this are true and the pair is the finding:**
the mechanism works exactly when somebody runs the tool between the change
and the re-declaration, and nothing in the process makes them. Here the
engineer ran it to check rather than to confirm, which is a habit rather
than a mechanism.

**(g) Persistence moves ahead of retrieval in the roadmap. RULING,
reversal.** The roadmap had it third, behind ranking and retrieval, reasoning
it blocks only once a real user exists. Tyler — the first person outside the
build to read the roadmap — asked as his first question whether it remembers
across conversations. Order is now ranking, persistence, retrieval. D18(i)'s
constraint stands and is inherited: a wire format rather than a save file,
`gob` and JSON already ruled out.

**(h) A seventh hand-transcribed derived figure went stale. TESTED.**
`docs/CODE.md` cited `docs/CLAIMS.md` as "659 lines, 23 claims"; it is now
725 lines and 25 claims — confirmed this checkpoint with `wc -l
docs/CLAIMS.md` and `grep -c "^id: " docs/CLAIMS.md`. D48(i) counted six
instances in one session and said nothing prevents a seventh. Nothing did.
Fixed in `docs/CODE.md` in the same edit that added this session's
`memory/rank.go` entry; the claim-by-claim `proven`/`killed-mid-check`
breakdown cited alongside it is left unreconciled to the new total, marked
explicitly rather than guessed, since `docs/CLAIMS.md` is being edited by
another hand this same session and a full `go run ./cmd/seam` was not
re-run to avoid reading it mid-write.

**(i) D3's addendum trigger fired early and is discharged. TESTED.** It
said whoever builds ranking should revisit `View.Fold`'s hot-window rule;
that rule was replaced by D32's size rule and no longer exists — confirmed
this checkpoint: `grep -n "ContainsFunc\|len(run)" memory/view.go` finds no
`ContainsFunc` guard, only the `len(run) > 1` check D32 already put there.
Nothing is owed.

---

## D50 — The founder corrected how the CEO decides, twice, and the corrections outrank the units built under them

**2026-08-13. Status: mixed, per clause.** (a), (c), (d), (e), (f), (j), (k)
and (m) are tested — re-run or re-derived directly against the committed
tree, the source, or the running environment this checkpoint, commands
cited inline. (b) is a finding built on tested antecedents (D45(e), D48(a)).
(g), (h), (l) and (n) are rulings — a founder correction or a CEO decision,
recorded in the words it was given rather than a checkable fact. (i) is an
asserted suspicion, explicitly unmeasured.

**(a) A design note on witnessed claims arrived and most of it is already
built here.** Its tiers map onto this project's `tested`/`asserted` marking
across 49 prior entries, and its named pathology — a judgment asserted in
the grammatical register of a fact — is what D19, D22, D34, D36, D42, D44,
D45(f) and D48 each record an instance of. Confirmed each of those eight
entries exists and carries that shape, this checkpoint, against
`docs/DECISIONS.md`'s own headings. (`CLAUDE.md`'s "Decide fast. Assert
slow." paragraph cites a differently-numbered eight — D19, D22, D34, D36,
D42, D44, D45(f), D45(l) — for the same phenomenon; the two lists are not
required to match, since one predates D48 and neither claims to be
exhaustive, but a future reader should not mistake the difference for a
contradiction.)

**(b) The note's cost asymmetry is wrong, and this repository holds the
counterexample.** It claims fabricating a witness that survives
re-execution is expensive to impossible. It is free and it happens by
accident: `TestCastPanics` re-executed green for a session and certified
nothing (D45(e), D48(a)). That is a **fourth failure mode the note
lacks — a vacuous witness**: valid, re-executable, structurally unable to
fail. Distinct from its "near-miss", which answers a different question; a
vacuous witness answers none. **The consequence: re-executing a witness
proves it runs, not that it would have failed had the claim been false.** A
witness needs its own null hypothesis, which `cmd/seam`'s baseline and
per-claim control already are, and which came from Tyler's three-word catch
when the tool was first briefed.

**(c) Two more places the note is thin, both with our evidence.** It calls
checking "nearly free for the verifier" — true in compute, false in
practice: thirteen-plus hand-transcribed figures went stale in this session
(twelve tallied explicitly as of the commit message, and a thirteenth that
counted the others was already stale when it was written, making it an
instance rather than an exception), and most already had their
derivation command printed beside them. **The binding constraint on
verification is attention, not compute**, which is precisely the gap the
forum shape exists to fill and the note is silent on. And on its open
question of whether `tier` is a field or a derivation, this project has a
third answer already built: a **declaration checked in both directions** —
confirmed this checkpoint against `docs/CLAIMS.md`'s own header ("the gate
fails when a claim is not where it says it is in either direction... a
claim that quietly starts passing cleanly trips it exactly as loudly as one
that stops") — which catches the direction nobody watches, a claim quietly
getting stronger while the prose still calls it weak.

**(d) The tree digest. TESTED.** `cmd/seam` now prints a sha-256 over the
copy the claims run in. Eleven red controls, one of which found a
defect in the engineer's own test — a rename case whose new name sorted
past its neighbour, so the address moved through the entry sort rather than
the path hash and the row passed with paths dropped entirely. **A test
input that perturbs more than one stage proves nothing about the stage it
names.**

**(e) The refusal it shipped with is reversed, and the reversal is the
CEO's error. TESTED.** Refusing a run whenever the tree moved was correct
about receipt integrity and made any write anywhere fatal for three
minutes, so no agent could edit during a check. **It bought integrity with
throughput — the one scarce resource here, since compute is nearly
free — and serialized this session twice within an hour before anyone
noticed.** Now marked in three places with the gate's summary line
qualified, exit 3, not folded into `adrift` (which is a predicate about a
claim's declaration, not about the ground under the run; conflating them
would falsely accuse a healthy claim). `cmd/seam/run.go:454-462` confirms
`adrift` returns 2 and `moved` returns 3, distinct codes. This checkpoint's
own `go run ./cmd/seam` run demonstrated the mechanism unprompted: editing
`docs/DEBT.md` while the run was in flight produced exit status 3 with one
claim reported "taken elsewhere ** not the tree named above" — the refusal
(e) reverses would have killed that run outright; the reversal let it
finish and say what happened instead.

**(f) `go run` flattens every non-zero exit to 1. TESTED, re-derived by the
CEO independently, and re-derived a second time in this session.** A
program exiting 3 gives 1 through `go run` and 3 through a built binary
(go1.25.4) — confirmed directly this checkpoint with a throwaway `os.Exit(3)`
program: `go run` reported `$? = 1`, the built binary reported `$? = 3`.
Both `CLAUDE.md` and `docs/CLAIMS.md` document the `go run` form, so the
entire exit vocabulary — including the `2` that predates this session — is
invisible through the documented invocation. Latent, since nothing scripts
the tool, but the printed marks are doing all the work. Recorded in
`docs/DEBT.md`.

**(g) The founder's first correction: evaluate on upside, not on failure.
RULING, and it changes the seat.** Asked about adopting Linear's
abstractions, the CEO hunted for the named failure that would justify it.
Tyler: *"It's not about is it failing. It's about being more successful.
Not less failure."* The seat's own casting — *"an instrument earns a work
unit only when a named failure demands it"* (`CLAUDE.md`, "Who is running
this") — is **structurally incapable of generating upside**, and every
instrument built this session is a detector. The concrete cost is (e). The
deeper one: the CEO's planning was one unit deep, so execution was serial,
so throughput was capped by the CEO's own attention rather than by compute.

**(h) The founder's second correction: one-way and two-way doors. RULING,
and it is the operating change.** Failure is how you learn past a local
optimum; a system that never fails never attempted anything it could not
already do. The operational form: **be maximally failure-seeking where
failure is reversible, and rigorous only where it is not.** This project's
one-way doors are few and nameable — publication (D15), anything reaching a
content address (D26, D33), the append-only log, secrets, and a figure that
reaches the shareholder and gets repeated before retraction (twice now:
D42(e), D45(f)). Everything else is `git revert` in a repo with no users.
**Today's defects were almost entirely in the record layer, not the
code**, which is the evidence for the rule: rigor belongs on the prose.
Concrete change: `decision-guard` goes on one-way doors, not on every code
change, and a full catalog run stops being a precondition for landing
two-way work. **Note the tension honestly** — this narrows a habit that
caught real defects today, and the log should say so rather than pretend
the trade is free.

**(i) A corollary the CEO now suspects about its own instrument. ASSERTED.**
The catalog reports 29 of 29 proven (confirmed this checkpoint —
`docs/CLAIMS.md` carries 29 `id:` blocks and the most recent full
`go run ./cmd/seam` before this checkpoint's concurrent edit reported 28
proven, 1 killed-mid-check, 0 adrift against its own declared set) and the
CEO has been reporting that as health. It equally supports *we only write
claims we already know will pass*. `docs/CLAIMS.md`'s own header says a
catalog curated until it is all green has stopped reading. Recorded as a
live suspicion, not a finding; nothing has measured it.

**(j) The persona's voice. TESTED.** In the log,
keep: the founder's ask; that the standing instruction was a list of
prohibitions and the metrology culture had written the product's voice;
that **the warmest draft produced the coldest output** under a live model,
because instructing a model to ask precisely produces a model that
instructs; that no test was written on the register deliberately, since the
obvious pin passes for a re-coldified prompt and fails for a paraphrase
(D27).

**(k) A false claim in the CEO's own brief, caught by the seat it briefed.
TESTED.** The brief asserted the system prompt reaches the persona's ref
and the change is therefore versioned. `Handle()` is `"ollama/" + p.Model`
and `persona_test.go`'s `TestHandleNamesTheWeightsAndTheVoice` pins that the
instruction is deliberately *not* identity — confirmed this checkpoint
directly against `persona/persona.go:73-82`. **So what an agent was told to
be, at the time it said a thing, is not recoverable from the record** — the
auditor's exact question, unanswerable. The fix `persona.go` names moves
every content address, so it is D26/D33-class. **CEO ruling on direction,
unbuilt:** prefer recording the standing instruction as a bit in the record
over putting its hash in the ref — content belongs in the store, D14 then
makes it discoverable, and no address moves.

**(l) Warmth and honesty do conflict, on one count, and the CEO was wrong
to assert otherwise. RULING.** D39(a) withholds the votes from the model so
it cannot optimise for approval. **Nothing withholds the model's charm from
the human.** A synth that is pleasant to be around is one you check less,
and the vote is a human checking output they did not write. Warmth moves
the consolidation signal (D4, D30) from the other end, invisibly, and no
instrument here could detect it. Unmitigated, unmeasured, and it is a
direct cost of a founder request that was still right to grant. Recorded in
`docs/DEBT.md`.

**(m) The gate is flaky about one run in four, via `store-unlocked`. TESTED,
and this finding leaked.** Four full runs the same day: three clean, one
failed because `TestConcurrentFoldsAgreeWithOneSequentialRun` asserted 0 of
16 samples; run alone it asserted 5/16, 4/16, 4/16 — confirmed this
checkpoint against `docs/CODE.md:96-110`, where the figures already live
with their commands. `docs/CLAIMS.md`'s header bars a gate flaky by
construction, so that claim's declared verdict is narrower than the claim
honestly is. **Record that this item existed only in a chat message for
several hours after the CEO ruled on it** — the exact failure the CEO had
diagnosed one turn earlier. **CEO ruling: the route is measurement, and
raising `runs` until it goes green is explicitly barred** — that is
sampling until it behaves, which the file forbids. Recorded in
`docs/DEBT.md`.

**(n) Org: no new seat; `persona/` had no owner. RULING.** Tyler asked
whether a voice unit going to `tui-design-engineer` meant we needed an
agent/harness or metacontext engineer. The real defect was that `persona/`
appeared in no seat's `Owns` column, so the dispatch went by file path. The
seat's charter line was already "the human surface"; its description
narrowed it to `tui/`. Widened — confirmed this checkpoint against
`.claude/agents/tui-design-engineer.md`, whose `description` now reads
"Also the persona's voice — its standing instruction and what it is told
when material is folded away", and against `CLAUDE.md`'s seat table, which
carries the same widening. **Trigger for revisiting, so a future session
does not relitigate from scratch:** a third distinct voice/prompt unit, or
the persona surface growing past a system prompt into something with its
own state. Also record the second known gap in `scope-adversary`: it argues
only against building — confirmed this checkpoint, its own `description`
opens "Argues against building the thing" — so it cannot pressure-test a
decision *not* to do something; it is structurally unable to help with a
hiring refusal.

---

## D51 — There is no first user, so we become one, and the record of building it is the proof

**2026-08-13. Status: mixed, per clause.** (a), (c), (d), (e), (f) and (g)
are rulings — a founder decision or a CEO decision, recorded in the
words it was given rather than as a checkable fact. (b) is asserted but
checkable, with its own re-derivation below.

**(a) The first-user ask is closed, and the answer is us. RULING.** Tyler was asked for one name and has none. **The ruling: build it, run this
company on it, and publish the record of having done so as the first
proof — a Show HN / Show Reddit case study.** His words: *"we build it. We
use it. We prove to ourselves that we're building toward what we actually
need. Then use our case study as first proof."*

**(b) Why this is not a stretch dogfood, which is the strongest argument
for it and was the CEO's addition. ASSERTED but checkable.** This company
has been executing the manual version of its own product for the length of
its history: `docs/DECISIONS.md` is an append-only record, the handoff is a
lossy fold standing in for material that scrolled away, "the one next
action" is ranking performed by hand, and `CLAUDE.md` is the view — which
goes stale exactly the way a view is supposed to. We did not design a
product and go looking for a user; we hit the problem, built the manual
version out of markdown and git, and have been paying its costs by hand
ever since. Re-derived this checkpoint rather than repeated: `git
rev-list --count HEAD` returns **52**, `HEAD` at that commit (D50). (Session
11's own handoff, `docs/handoffs/2026-08-13-session-11.md`, cites 51 at an
earlier `HEAD` — one commit fewer, since D50's own commit had not
yet landed when that figure was taken. Both are correct for the `HEAD` each
was taken against; neither is stale.)

**(c) The case study is not authored, it is the record. RULING.** The
artifact to publish is the actual tldreddit record of building tldreddit,
ranked by what a human voted mattered. Unfakeable in a way a screenshot is
not.

**(d) The condition the whole strategy rests on, and it is the
shareholder's. RULING, and the risk to watch.** The vote is a *human's*
cheap act (D4), and ranking is downstream of it (D3, D30). If the CEO is
the only participant, we test the agent half and skip the half the thesis
rests on — and an agent voting on its own record is the karma-farming
failure D39(a) exists to prevent. **Tyler does not need to use it daily;
he needs to be the one voting.** Recorded here as the named way this
strategy fails quietly, not as an aside: a launch built on this thesis with
no human votes in the record it ships would be indistinguishable, from the
outside, from a launch that succeeded.

**(e) Smallest real first step, so "use it" does not stay a slogan.
RULING.** The continuity substrate moves into the product: a session's
handoff gets written *into the record* rather than into a markdown file,
and the following session reads it back **ranked** rather than reading the
newest file by name. Small, honest, and on the load-bearing path — if it
fails we lose a session's context and find out immediately. This confirms
persistence as the next unit rather than choosing it: D18(i)'s constraint
still binds (a wire format, not a save file; `gob` and JSON already ruled
out — `memory/id.go:20`).

**(f) D40's trap is live and we are walking at it deliberately. RULING.**
D40 warned, in its own "The trap, named so a future session does not walk
into it by accident" paragraph, against bending the product roadmap to
serve the organisation's own tooling needs — naming persistence built
because "our own agents need durable notes" as D5's mistake wearing
dogfooding's clothes. This strategy is not that — using the product for
real work is the reverse of building product features to serve internal
tooling — but the boundary is thin. **The test, written down so a later
session can apply it: does this feature exist only because we need it?**
If yes, it is D40's trap and not dogfooding.

**(g) Do not launch early. RULING.** "We used our own tool" is table
stakes and reads as marketing. What earns a Show HN is a *surprising*
finding — something the record shows about how we work that we did not
already know. That needs months and volume, not a working build. **And
D15's gate applies to the case study**: it draws on the private record.
`decision-guard`'s comprehension pass is required before any of it
publishes, exactly as for the code.

---

## D52 — Persistence lands, and a check that enforces a bug is a third failure shape

**2026-08-13. Status: mixed, per clause.** (a) and (c) are ruling plus
tested — a design choice made and verified against the committed tree this
checkpoint. (b) is asserted but checkable. (d), (f) and (g) are
tested — re-derived directly against `docs/DECISIONS.md`, `docs/CLAIMS.md`
and the trees named. (e) is asserted, explicitly unconfirmed. (i) is a
ruling. (j) and (k) are rulings.

**(a) Persistence, and the design ruling behind it. RULING + TESTED.** CEO design ruling: the wire format is the canonical
encoding that already existed in `memory/id.go`, made reversible. `canon`
wrapped a `hash.Hash`; a `hash.Hash` embeds `io.Writer`, so `ID` now hands
it `sha256.New()` and the identical byte stream goes to a file — no
content address moved. Tested: the four pinned goldens
(`TestIDIsPinned`, `TestCompactionIDIsPinned`, `TestVoteIDIsPinned`,
`TestFragmentIDIsPinned`) pass against an unmodified `memory/id_test.go`
, re-run this
checkpoint. New surface: `memory/wire.go` — `(*Store).WriteTo`,
`ReadStore`, `(*Store).Address`, `(View).WriteAgainst`, `ReadViewAgainst`,
`StaleView`.

**(b) Decodability is the mechanical proof of D26. ASSERTED, checkable.**
`readPayload` (`memory/wire.go:466-499`) is the inverse of
`Payload.canonical`, and its own comment says so: `Utterance.Truncated` and
a `Vote`'s direction reach the content address through `kind()` alone —
`"fragment"` recovers `Truncated`, `"upvote"`/`"downvote"` recover the
direction — and the inverse function exists only while that map is
one-to-one. Add a field `kind()` does not distinguish and the decoder
becomes unwritable rather than merely wrong. D26 (2026-08-11) stated this
constraint in prose and in a doc comment; nothing enforced it until this
session's decoder made violating it a compile-shaped problem rather than a
silent collision.

**(c) A check that enforces a bug — the finding of this session. TESTED.**
`seal` was computed over the live store's address rather than over the
`against` field the stream itself carries, and provenance was checked
before integrity — so a flipped bit in that 64-byte field took the stale
branch and the seal was never evaluated, handing an unverified view out
through the recovery door. On a vote view, with a second flip, that is a
silently truncated view, a lifted hold, and a fold that takes material
somebody voted to keep. `docs/CLAIMS.md` had pinned that behaviour as a
designed property under `sole: true` — so the catalog *required the
defect to remain*. Every sentence in that prose was true; a `red:` list is
derived by running a mutation and records whatever the code does, and
prose is the only place intent enters. This project has shipped
instruments that could not fail (D27) and a check that certified nothing
(D48). A check that enforces a bug is a third kind, and it is the kind you
only get once the instrumentation is good enough to have a bug worth
enforcing.

**(d) Three CEO citation errors in one session, all caught by execution.
TESTED.** The constraint is **D39(i)**, not the **D18(i)** the record
carries in three places — `docs/DECISIONS.md:3865` (D49(g)) and `:4167`
(D51(e)), plus a third inside this session's own working text before it
was corrected. `D18` runs (a)–(g); `D38` runs (a)–(c); line 2469 sits
inside `D39`, which runs (a)–(i) — verified by heading grep this
checkpoint. `decision-guard` corrected the label to "D38(i)" — also wrong,
since D38 stops at (c) — and the CEO relayed that wrong label without
executing on it. Separately the CEO said "four places" where it is three
— the sentences making the claim were being counted, not the occurrences
— and said "D48(h)" for the stale-figure tally where it is **D48(i)**,
carried to a seventh by **D49(h)**; `D48(h)` is the
archivist-refused-a-CEO-instruction entry, a different finding entirely.
**The generalization worth recording: hand-transcribed citations rot
exactly like hand-transcribed figures, and unlike figures they are
trivially checkable** — whether a lettered subsection exists in a given
entry is a one-line derivation (`awk`/`grep` against the entry's own
heading range), not a re-measurement. The two `docs/DECISIONS.md`
occurrences above are append-only and are corrected by this entry, not
edited. **(e) A proposed root cause, explicitly not established. ASSERTED,
unconfirmed.** D39(i)'s own opening sentence reads "D18(b)'s persistence
requirement is no longer describable as a save file" — and D18(b) *is* the
persistence decision, so the label attracts a "D18" with nobody having
read D39(i) itself. The tree cannot separate this from ordinary
transcription error; recorded as one plausible source, not as the cause.
This clause exists because the CEO was about to log the story as a
settled finding rather than a guess.

**(f) The tenth stale figure, manufactured by repairing the eighth and
ninth. TESTED.** `docs/CODE.md` said `docs/CLAIMS.md` was 851 lines, was
repaired to 1006, and was 1043 by the time anyone checked, because the
file kept growing inside the same work unit that repaired it — inside the
parenthetical calling itself the ninth instance. **Ruling: care is not the
mechanism, and ten instances is enough.** Two routes, the first already
applied: where a figure has no reader, delete it — the line
count is gone from `docs/CODE.md`; the claim count stays, because it
changes rarely and means something. Where a figure has a reader,
mechanize it rather than repair it by hand again. `CLAUDE.md`'s own two
stale figures — the cited line counts for `docs/CODE.md` (said 248, is
357) and `docs/DEBT.md` (said 208, is 268), both re-derived with `wc -l`
this checkpoint — are fixed in this same checkpoint; see `CLAUDE.md`'s
"Working on the code" and "Open debt" sections.

**(g) A measurement restated three times, each time by measuring rather
than sampling. TESTED.** A claim citation reported as reddening "8 in 12"
measured at roughly 6–16 green in 100 across two seats. The mechanism is
Go's small-map iteration yielding a **rotation** of insertion order:
distinct orders equal the element count, so sorted order is hit ~12% of
the time at n=5 and 0 times in 20,000 at n=25 — confirmed in
`.claude/craft/principal-go-engineer.md:730-767`. An intermediate
reading — the CEO's, relaying the guard — blamed the fixture's insertion
order sitting near-sorted and was itself wrong; the driver is map size,
not the fixture. Worth recording as an instance where each correction was
right to make and the third was the correct one.

**(i) The `Compaction` deadline had not passed, contrary to a report.
RULING.** The engineer escalated that persisting `Compaction` had passed
D26's "cheap only while nothing has persisted" deadline. It had not:
nothing had been written to disk, `cmd/tldr/main.go` is unwired to any of
`memory/wire.go`'s surface (confirmed by grep this checkpoint — no
non-test caller of `WriteTo`, `ReadStore`, `WriteAgainst` or
`ReadViewAgainst` exists), and `decision-guard` independently confirmed
no non-test caller. The deadline passes when the program writes a file
someone keeps, not when the format that would write it exists. **The CEO's
ruling: no change to `Compaction`**, because the natural form of per-view
folds (D18(e)) is a sibling payload type with its own `kind()` tag, which
re-addresses nothing existing — and all seven `Compaction` accessors are
live on the surface today (`From`, `To`, `Bag`, `Kinds`, `Count`,
`Handles`, `Absorbed`, at `tui/render.go:499-563`, `tui/unfold.go:68-397`,
`tui/ranked.go:117`, `tui/ask.go:499-627`, verified by grep this
checkpoint), so nothing is vestigial. Residual risk named: if the screen
later wants a field `Compaction` does not carry, no sibling type helps —
that is the trade this ruling makes, not a problem it solves.

**(j) Founder input and a CEO ruling: a Claude Code skill for driving
tldreddit. RULING.** Tyler asked whether a skill could eventually outline
using tldreddit as a CLI, thinking about the dogfood step. The CEO's
answer, recorded as ruling: yes, and the case is stronger than
convenience — the current handoff mechanism has already failed twice in
the way the product prevents (D28, two sessions ran and the doctrine that
only one writes a handoff exists because of it; D45(m), zero-padding —
`session-10` sorts before `session-6` unpadded), so "read the newest file
by name" is currently defended by a shell script rather than by the
product. **The binding constraint, ruled now because it is expensive to
retrofit: the skill gets Claude a write, never a vote.** D51(d) names
agent-voting on its own record as how the strategy fails quietly, D4
makes the vote a human's act, and D51(d) attributes that failure to what
D39(a) exists to prevent (withholding the vote from the model). Agents
produce volume; the human produces signal. D40's and D51(f)'s trap test
was applied explicitly and passed: "does this feature exist only because
we need it" — no, agents writing into a forum-shaped memory is the
charter's own first paragraph, not internal tooling built to serve this
organization's own needs.

**(k) An operational call on the migration. RULING.** The markdown
handoff keeps being written in parallel for the first sessions after the
record takes over, so the two can be compared. Not caution — it turns the
migration into a measurement, and a side-by-side is the kind of
surprising finding D51(g) says a launch needs before it can claim more
than "we used our own tool."

---

## D53 — The client is wired to the record, a review catches a vacuous
witness the same day it was cited, and a third "no, fix a gap" resolves an
org question

**2026-08-13. Status: mixed, per clause.** (a) is ruling plus tested —
re-derived against the committed tree and re-run independently by this
checkpoint's archivist dispatch. (b) and (c) are tested, against the named
commit, files and craft record. (d) is a ruling, restated from the
commit message rather than re-litigated. (e) is a ruling.

**(a) Persistence reaches the program. RULING + TESTED.** They carry the four design rulings (one file rather than three for the
store and its two views, atomic write, a fatal named load failure,
`$TLDR_RECORD` before XDG) and the continuous-save invariant
(`tui/save.go`: `Update` wraps a pure `Model.update` and compares a
`checkpoint` — store pointer, bit count, both views — before and after
every message, rather than a dirty flag raised at each mutation site).
Re-verified independently by this checkpoint's dispatch rather than taken
on report: `go build ./...`, `go vet ./...`, `gofmt -l .` (clean, no
output) and `go test ./... -race -count=1` (five packages, all `ok`) all
green against the tree. `go run ./cmd/seam` this checkpoint:
35 claims, 34 proven, 1 killed-mid-check, 0 adrift, all as declared, exit
0 — tree `cc18ecfe8586…faaade9`, which the tool's own output marks "not
identity: uncommitted work means many trees share that sha," because two
uncommitted seat-definition edits (clause (e) below) were present in the
working tree at run time; they touch only `.claude/agents/*.md` and cannot
affect a Go build, but the hash is reported honestly as covering the tree
that was actually run rather than the last commit.

**(b) A vacuous witness in code written the same day. TESTED.** Deleting
the whole `if m.trouble.unsaved` branch in `tui/ask.go` left the `tui`
suite green while the screen said "nothing was recorded" above a
transcript of the words it claimed were gone. Three tests read `Model`
fields and none rendered a frame. Found by `tui-design-engineer` reviewing
`principal-go-engineer`'s work, ; closed by
`TestASaveThatFailedNeverSaysNothingWasRecorded`, confirmed present at
`tui/ask_test.go:228` this checkpoint, sole catcher of three mutations
per the commit message. This is D45(e)/D48's failure mode arriving in new
code on the day those entries were being cited to justify the check that
found it — the strongest evidence yet for the second-reader rule in
`CLAUDE.md`'s "Most catches come from a second reader."

**(c) tmux becomes the surface seat's second instrument, and its limit is
stated in the same breath. TESTED.** Confirmed against
`.claude/craft/tui-design-engineer.md`'s "tmux is the second instrument,
and what it is not" section: on geometry tmux and `HARNESS=1` agreed
exactly — the ladder steps at 32→31 and 27→26 under both — so it is **not**
adopted on a theory that the harness is unreliable about layout. It is
adopted because the harness has no file, no other package's errors and no
process boundary. It found a real defect the same run: `atomically`
wrapped its returns per branch, only one branch was wrapped, and the
failure a person actually hits (`os.CreateTemp` on an unwritable
directory) reached the screen as `open <dir>/.record.tmp-4052371607:
permission denied` — naming a temporary file that does not exist and never
naming the record. Fixed by wrapping once at the boundary,
verified there against a `chmod 555` directory: the message now leads with
`writing …/record:` and exit is 2. **The source had been read twice and
neither reading found it.** The rule recorded with the tool, in the same
craft-record section: a capture is evidence of what a frame looked like
and never evidence of what a mechanism does.

**(d) A CEO ruling on the CEO's own trigger. RULING.** Earlier the same
session `docs/CLAIMS.md` was written to keep `sole: true` on
`record-frame-unclosed`, with a trigger to retire it after three trips on
work with no bearing on the frame. A trip arrived hours later — confirmed
against `docs/CLAIMS.md:676-683` this checkpoint, which records the ruling
in place: **it does not count**, because the two new tests
(`TestTheFileMatchesMemoryAfterEveryChange`,
`TestAChangeThatCouldNotBeWrittenIsCarriedByTheNextOne`) both read a saved
file back, and the frame is what they read it through — the predicted
case, not the unrelated one. The `red:` set is now fourteen names,
confirmed by count against `docs/CLAIMS.md:690` this checkpoint.
`principal-go-engineer`'s debt note had called it the first of three and
was corrected in place. Worth an entry because it is a rule's author
ruling on his own rule the same day, in the direction that makes the rule
harder to satisfy rather than easier.

**(e) Org: no sixth seat; two ownership gaps closed instead. RULING.**
Tyler asked whether the org needs more product-focused hires. Ruling: no —
a product seat's output is a document the CEO must read and rule on,
which adds to the serialization point rather than removing it, and the
product's missing pieces (there is still no retrieval of any kind while
the thesis is that ranking *is* retrieval) are not disputed, so a seat
would not resolve them. Two real gaps existed instead and are closed in
`.claude/agents/scope-adversary.md` and
`.claude/agents/tui-design-engineer.md` — confirmed against both diffs
this checkpoint, **uncommitted in the working tree as of this entry**.
`scope-adversary`'s insulation from the commercial thesis is narrowed:
still absolute for a pure build/don't-build call, but the seat must now
ask for the thesis when a brief entangles scope with go-to-market and say
so rather than answering anyway, and its structural inability to argue
against a *refusal* (rather than a build) is written into its own
definition as a limit for it to name. `tui-design-engineer` now owns
**being the first user** (D51), with example tmux commands and the
instruction to report what was unusable rather than rank it itself.
Trigger recorded for revisiting the "no sixth seat" ruling: when the CEO
turns units down because he cannot spec them fast enough, rather than
because they are not worth building. **This is the third time "do we need
a seat" resolved as "no, fix an ownership gap"** — D17, D50(n)
(`persona/` had no owner), and this entry — either a
pattern that is right or a seat structurally reluctant to hire, and the
CEO cannot tell from inside it. `scope-adversary` cannot help here, for
the reason now written into its own definition per this same clause.

---

## D54 — Two places the record was wrong about itself, and the caret's
row now draws whole

**2026-08-14. Status: mixed, per clause.** (a) is ruling plus tested — the
D14 property re-checked directly against `memory/reach_test.go`. (b) is
tested, against the named commit and a re-run `.githooks/pre-commit` and
`go run ./cmd/seam`. (c) is a ruling, restated from the commit's own
message. (d) is asserted, with the mutation-lie measured directly by the
commit and re-confirmed against the cited files this checkpoint. (e) is
tested, against the named test. (f) is measured, with the ratio it once
supported deliberately not repaired. (g) is tested, against the code
comment that documents its own catch.

**(a) The D14 citation was wrong, and the work was built anyway. RULING +
TESTED.** D14 binds the record, not the surface: reachable means
discoverable by walking `Prev`/`Absorbed`, and `memory/reach_test.go`
(re-run this checkpoint, `go test ./memory/... -run
TestEveryStoredBitIsReachableFromTheView -v`, `PASS`) still asserts
exactly that — a truncated bit has always satisfied it,
because the store held the bit whole and content-addressed the entire
time. What was actually broken belongs to the surface, which is a
different and narrower claim: a "…" is an antecedent nothing on screen can
follow, so the promise D14 exists to make good on — that discoverable
material is actually reachable *by a person looking at the screen* — was
not being kept, even though the record underneath it was fine. The wrong
framing was not a one-off slip; it was carried by a subagent's finding
into `docs/DEBT.md` copied again into the session-13 handoff
(`docs/handoffs/2026-08-13-session-13.md:40,273,285-288,300,326`), and
repeated a fourth time in this session's own dispatch briefs before being
caught — four repetitions of a claim nobody re-derived, until two seats
did, independently. `scope-adversary` and `tui-design-engineer` reached
the corrected framing from different directions, neither having seen the
other's work. **The correction cost the work
nothing**: it changed the entry's queue position and the scale of the
fix, not whether it shipped — the commit landed the same session as the
correction.

**(b) The caret's row draws whole, and it took no key. TESTED.** The
decision is the structural argument, not the diff: the object that needs
a key to expand is by definition the object with no room left to
advertise one, so the only affordance a full row has left is the caret
already sitting on it — which is already what a vote lands on. Zero new
`Model` state: `frame.mark` already said which row was the caret's.
Eleven new test functions in `tui/tui_test.go`
— `TestAContinuationRowHangsUnderItsOwnSentence`,
`TestANoticeIsOnScreenUnderAnAnswerTallerThanTheFrame`,
`TestAnAnswerTallerThanTheFrameIsShownFromItsBeginning`,
`TestAnExpandedFragmentNeverTradesAWordForItsMark`,
`TestAnExpandedRowShowsEveryWordAndNotTheLineBreaks`,
`TestAnExpandedRowSurvivesAWidthOfNothing`,
`TestOnlyTheCaretsRowIsEverMoreThanOneRow`,
`TestTheAnchorsNameTheRowsTheyWereDrawnOn`,
`TestTheCaretsRowIsCutWhereTheArrangementAlreadyGaveUp`,
`TestTheCaretsRowShowsEveryWordAndEveryOtherRowIsCut`,
`TestTheRankedCaretIsInsideTheFrame` — confirmed by diffing the function
names, not by trusting the dispatch brief that named eight; the true count is
eleven and is recorded as measured rather than as handed down.

**Twelve across the commit, and the third count of one thing in one
session.** The eleven above is exact and names its file. The commit adds a
twelfth, `TestHarnessRead`, in `tui/harness_test.go`; no test function was
removed, and `TestHarnessRead` did not exist in the parent. So the three figures produced for this quantity were 8, 11 and
12: the first simply wrong, the second exactly right about a narrower
question than the reader would assume, the third the commit's total. Only
one of the three was an error, and the one that reads most like a
correction was a **scope** difference — which is the harder case, because
nothing about it looks wrong. It is recorded beside clause (f) on purpose:
that clause is about a figure whose subject moved, and this one is about a
figure whose subject was never the same subject twice. Re-run
independently this checkpoint against `HEAD`: `sh
.githooks/pre-commit` — build, vet, mod tidy, `test -race` — exit 0 across
all five packages; `go run ./cmd/seam` — 35 claims, 34 proven, 1
killed-mid-check, 0 adrift, all as declared, exit 0.

**(c) The charter's own debt list rotted, and one kind of claim cannot be
guarded. RULING.** `CLAUDE.md`'s retrieval bullet said "no cursor, no
selection... no ranked surface and no query," and two of those clauses had
been false since D39(a) built the content-addressed caret — the bullet
was written by D30, before the caret existed, and nothing updated it when
the caret shipped. `docs/DEBT.md` copied the claim and the session-13
handoff copied it again, and `docs/DEBT.md:166` contradicted the charter
inside the same repository by describing caret behaviour under
`pgup`/`pgdn`. Of the three inline debt bullets `CLAUDE.md` carries, the
two that cite a re-check command (the simulator gap, the fold-budget gap)
stayed true; the one that cited nothing rotted. The deeper limit, which is
the part worth keeping rather than the correlation: **a claim of absence
cannot be checked by `cmd/seam`'s mechanism.** Seam proves a claim by
mutating the tree until a cited test goes red, and no test goes red when
something stops *not* existing — there is no assertion shape for "this
does not happen." Every "no X" sentence in the charter is unguarded by
construction, permanently, not just today. No instrument is built for it
— D27 already found three that cannot fail, and a fourth that cannot
even in principle fail is a worse trade, not a better one.

**(d) A mutation is evidence only after `go build ./<pkg>/` accepts the
mutant as non-test code. ASSERTED, mutation-lie measured.** Now in
`docs/CLAIMS.md`'s header , immediately after the
paragraph explaining what a red proves. `go test` compiles a package's
test and non-test files together, so a mutation that mis-targets a
production declaration can resolve silently against an identically named
`_test.go` identifier instead of failing to compile; `tui`'s test files
declare roughly 48 such package-scope identifiers with ordinary names.
Measured directly this session and reproduced again this checkpoint: `go
build ./tui/` reports `undefined: lines` while `go test ./tui/` reports
`ok` in the same second, because `tui/harness_test.go:130` declares `var
lines`. Four mutation-harness lies occurred this session, all green, none
in the product. `cmd/seam`'s actual exposure is stated at the strength it
earns rather than alarmingly: `cmd/seam/tree.go`'s `occurrences`
(`tree.go:91-97`) already requires an anchor to appear exactly once in a
named file, and `mutate` (`tree.go:102-120`) already refuses a mutation
that changes nothing — both defend against the shape of failure that hit
the hand-rolled scripts. The one hole that remains is named, not fixed:
`cmd/seam/run.go`'s `runSuite` (`run.go:148-156`) runs `go test` and only
`go test`, so it cannot distinguish a test reddened by broken behaviour
from one reddened by a broken production build compiling against a test
fixture instead. Folding a build step into `runSuite` is written down as a
recommendation in `docs/CLAIMS.md`'s own text — not taken, and explicitly
left as the CEO's call rather than this file's.

**(e) Review caught a regression that was this product's own failure
shape. TESTED.** The first version of the caret-expansion fix could push
`"╌╌ recorded here, not on disk ╌╌"` (`tui/ask.go:850`) entirely off
screen under a tall caret block, because `sync` pinned the block's top
before checking `riding()`. A renderer hiding the one notice that exists
to say the record did not reach disk, in the same commit that made rows
readable, is exactly the failure shape this product exists to catch —
found by `decision-guard`, reproduced through the real save path at
80×24. `TestANoticeIsOnScreenUnderAnAnswerTallerThanTheFrame`
(`tui/tui_test.go:1985-2014`) is its sole catcher: it fails a real save
via an injected hook, sends a vote so the caret rides the failing answer,
and asserts both `"not on disk"` and the underlying error string are
present in the rendered frame.

**(f) A measurement that perturbs its subject. MEASURED.** Four counts of
`~/.local/state/tldreddit/record` were taken this session — 14, 17, 20,
22 utterances — and every one was correct at the moment it was taken. The
file grew from 10,279 bytes to 19,662 bytes during the session *because
measuring it meant running the program that writes it*. Confirmed present
in this session's own transcript
(grep
counts: `10,279`×4, `19,662`×4, `14 utterances`×2, `17 utterances`×8, `20
utterances`×3, `22 utterances`×4). The ratio derived from these counts was
deleted under D52(f) rather than repaired — the phenomenon is what is
kept here, not the number. Separately, where the CEO's own ad-hoc byte
scan of the record file disagreed with `decision-guard`'s decode through
`memory.ReadStore`, the product's own reader was the trustworthy one —
the same lesson as `cmd/seam` versus a hand-rolled mutation script,
twice in one session (clause (d) above is the other instance).

**(g) A disclosure stopped being true between the review and the ship.
TESTED.** `decision-guard` correctly reported `tui/ranked.go`'s `at.rows =
1` (`tui/ranked.go:252`) as inert at review time. The regression fix in
clause (e), landing in the same dispatch, made `sync` frame a *block*
rather than a row — so a zero there stopped being inert and became a
surface that silently stops scrolling to its own caret, with the next
vote landing on a row nobody can see. It is now load-bearing, and the code
says so in its own comment (`tui/ranked.go:241-250`): "this surface does
not draw the caret's row whole yet, so `rows` is 1 wherever a caret is
drawn at all... a zero here is a surface that silently stops scrolling to
its own caret... [`TestTheRankedCaretIsInsideTheFrame`] is what notices."
Confirmed present at `tui/tui_test.go:1948`. The rule to keep: **re-derive
a disclosure against the tree being shipped, not the tree the review
read.** Caught by the building seat, not by the CEO — this dispatch is
transcribing the catch, not the one who made it.

---

## D55 — The ranked surface draws its caret whole too, a universal
falsified and repaired, and founder input on swarm training data held
apart from a decision

**2026-08-14. Status: mixed, per clause.** (a) is tested, against
the commit's own measurement. (b) is tested in both directions and
independently re-derived this checkpoint — the two figures in it turned
out to describe different quantities of the same event, not to
contradict each other, and both are now confirmed rather than merely
repeated. (c) is measured, restated from the commit's own message. (d) is
tested, against `docs/CLAIMS.md`'s current block for
`memory/rank.go`'s tiebreak. (e) is tested, with the presence check
re-run live this checkpoint rather than trusted from the commit. (f) is
tested, against `tui/tui_test.go`'s current line count. (h) is measured, against
`docs/DEBT.md`'s own text, which is where the finding lives — it was
never a `docs/DECISIONS.md` entry and this clause does not make it one.
(i) is input, explicitly not a decision, with the CEO's response
recorded beside it and the one architectural fact in it verified
directly.

**(a) The ranked surface draws the caret's row whole, by quoting rather
than wrapping. TESTED.** The reference row stays whole and
the message it stands for is quoted underneath it in the gutter, at
terminal width — the same shape `unfold` already uses beneath a scar,
same two glyphs. Not a straight port of the transcript's shape: a
ranked row's lead is a reference (ordinal, address, clock, mark,
handle) rather than a sentence, so wrapping in place would repeat that
lead as blanks on every line of the answer. Measured by the commit
against the alternative: 8 rows of 36 columns at width 40 for the
quoting shape, versus 19 rows of 14 columns for the ported
wrap-in-place shape, on the same fixture.

**(b) A universal was asserted in four places and falsified in review,
and the two figures that came out of the fix describe two different
things, not one contradicted thing. TESTED both directions, and
independently re-derived this checkpoint.** The seat correctly refused
to inherit the transcript's `room >= textFloor` gate onto the block's
preview column — a floor belongs to the geometry it was measured in,
and the block does not wrap into the preview column. It over-generalised
from there to "no floor at all, so no gate on this surface at any
column." `decision-guard` falsified the universal: below width 5 the
4-column prefix (`hang := lipgloss.Width(gutter) + colGap`,
`tui/ranked.go:323`) eats the terminal and the block becomes unreadable
where not opening it would have stayed legible. The fix applies the same
constant to this surface's own quantity instead — `width - hang <
textFloor` (`tui/ranked.go:324`) — which is a different statement from
the gate that was refused, not a reversal of it.

Two figures appear for the same width-4 measurement and initially read
as disagreeing: the commit message says "231 rows... where not opening
draws 8"; `docs/DEBT.md:406-407` and `tui/ranked.go`'s own code comment
(lines 316-318) say "223 rows." Re-derived directly this checkpoint
rather than trusted from either source — and both figures are correct: at width 4 with no floor, the caret's own
block is 224 rows (1 reference row plus 223 quoted continuation rows,
`at.rows=224`), and the full rendered list is 231 rows total
(`len(rows)=231`), meaning the fixture's other 7 entries draw 1 row
each. "223 rows of `│ …`" (DEBT.md, the code comment) counts the
block's own continuation lines; "231 rows... where not opening draws 8"
(the commit message) counts the whole screen against its own 8-row
baseline. Neither figure is wrong; nothing here was reconciled before
this checkpoint, and it should have been — two sourced numbers that look
like a contradiction and are not is exactly the shape a reader cannot
tell apart from one that is, without doing the re-derivation.

**(c) The reusable finding, and it is the best thing in the unit.
MEASURED, restated from the commit's own message.**
`TestNoRankedRowRunsPastTheWidthItWasGiven` (`tui/ranked_test.go:815`,
confirmed present, sweeping widths down to 1 in its own table) passes at
every width including 1 because `clip` fires and hides the geometry
error underneath it. The transcript's counterpart `clip` cannot fire at
all — `hang + room == width` exactly there, disclosed in a comment
beside it — so a width sweep on that surface is proving something a
width sweep on this one cannot. **When a `clip` that cannot fire on one
surface can fire on another, the second surface has an arithmetic hole
and its own width sweep is the test least able to see it**, because the
backstop that makes the sweep pass is the same backstop that is masking
the bug.

**(d) `cmd/seam` caught a cross-package invariant leak the suite
structurally could not. TESTED.** Four new `tui` tests reddened under a
mutation to `memory/rank.go`'s tiebreak (the `rank-ties-by-address`
claim, `docs/CLAIMS.md:1022-1025`, `find:
cmp.Compare(b.Others, a.Others)` → tie-break on content address instead
of view order), because a fixture asserted where in the ranked *order* a
bit lands rather than taking that ordering as given. `sole: true` is a
claim about the whole tree, so a fixture in one package silently
borrowing another package's invariant makes that claim false while every
test still passes. Fixed in the tests, not the claim: `docs/CLAIMS.md`'s
current block for this mutation declares `red:
TestRankOrdersAViewByItsVotes` alone, confirmed by reading the block
directly this checkpoint — the tui tests no longer appear.

**(e) `docs/CODE.md` did not contain D3's only code, and the presence
check that found it still finds the same twenty. TESTED, re-run
live.** `grep -in "ranked" docs/CODE.md` returned nothing; `tui/ranked.go`, `tui/ranked_test.go`, `tui/style.go`,
`tui/ask_test.go` and `tui/save_test.go` were absent from the file
`CLAUDE.md` calls the inventory of *every* package and test file.
Surfaced only because a review grepped it for an unrelated reason. Five
are now present, confirmed this checkpoint by grepping `docs/CODE.md`
for each filename. Re-running the presence check `archivist.md` now
carries (`.claude/agents/archivist.md:47-52`) against the live tree this
checkpoint returns exactly 20 missing `.go` files — the same twenty files by name
(`memory/*_test.go` ×7, all of `persona/`, `cmd/seam/*` ×7,
`cmd/tldr/record_test.go`), filed as MMO-15. **The distinction that
matters: a missing entry is a decidable diff with no false-pass mode; a
stale description is a semantic judgment** — an advisory presence check
can guard the first and cannot guard the second, and is adopted for
exactly that reason rather than grown toward the second. Deliberately
not a gate: a new test file appearing before its entry is written is
normal mid-unit, and a hard gate would either block routine commits or
get `--no-verify`'d.

**(f) MMO-12 closed by deletion rather than mechanization, and the stale
figure that closed it was manufactured inside this session's own prior
commit. TESTED.** A sweep found three `N lines` figures outside
`docs/DECISIONS.md`; two are pinned to a named commit or a dated past
event and cannot rot. The third, `docs/CODE.md`'s claim that
`tui/tui_test.go` was 2,055 lines, had rotted against an actual 2,713 —
confirmed this checkpoint, `wc -l tui/tui_test.go` returns 2,713. The
2,055 figure was accurate when written and went stale when a commit
(this same session, five commits earlier) added rows to that file
without anyone returning to update the count that described it — the
thirteenth instance of this exact failure shape. Deleted rather than
repaired, under D52(f)'s own ruling applied to itself: a derived figure
this project keeps re-breaking is better removed than re-fixed a
fourteenth time. **Not claimed: that the failure class is closed.**
Clause (b) above and the 231-vs-223 reconciliation are two more
instances of a figure going unreconciled this same session, in a
different medium than an `N lines` count — the class this clause closes
is narrower than the defect that keeps producing it.

**(h) The `store-unlocked` flake reproduces on unmodified `HEAD`, in a
second shape. MEASURED — and this finding lives in `docs/DEBT.md`, not
here.** Nine full `go run ./cmd/seam` runs during the ranked-surface
work: seven clean, two returning `killed-mid-check 2 · adrift 1` — a
different failure from the D50-documented one, where the sibling
claim's cited check is aborted by the process dying rather than
asserting the wrong sample count. Isolated (`-run store-unlocked`) it
was clean 4 of 4, so a full run's added load is part of it. Confirmed
this checkpoint against `docs/DEBT.md:251-263`, where it is recorded as
an addendum to the existing D50 debt item with the reproduction
procedure (`git archive HEAD | tar -x -C <scratch>`, run the gate
there): one of three such runs came back `adrift 1` with no working-tree
changes present at all. So the honest bar, restated from the commit's own message: 35 claims (`grep -c "^id:" docs/CLAIMS.md` confirmed this
checkpoint), 35 as declared, 0 adrift on runs that complete cleanly, and
roughly one run in four is a coin on this one claim regardless of what
tree is under it.

**(i) Founder input, recorded as input and explicitly not as a decision.**
Tyler described the product he wants: logging in to find nine agents
working and talking, humans and family mixed in, agents posting TILs
and DMs possibly unobserved — and a larger thesis that this generates
training data for specialised models, enabling swarm orchestration.
Recorded faithfully; the CEO's response was **not** agreement. D24's
Moltbook study ran a structurally close configuration at 120,000 agents
and 6.7 million comments, and produced 97.3% of comments receiving zero
upvotes because upvoting was a step in a reference document rather than
a step in the agents' own executed loop, and 63.4% of posts landing in
one community because every posting example across 41 recovered
instruction-file snapshots used `general` as the target (both figures
re-confirmed this checkpoint at `docs/DECISIONS.md:1093-1094,1130`,
D24's own text). The CEO held three things against the founder's
framing: "models are better now" does not address a structural failure
in what the loop makes participation a step of; "infinite possibility in
metacontext engineering" inverts the evidence, which shows two boring
defaults — an omitted step, a copy-pasted default community name —
dominating 6.7 million comments' worth of behaviour; and "maybe we don't
even see it" describes the one configuration Moltbook actually tested,
which is D4's own territory rather than a counterargument to it. **The
concrete, buildable outcome:** model diversification is the strongest of
Tyler's levers and is already architecturally supported and unused.
`persona.Persona`'s `Model` field (`persona/client.go:92`) is the stable
half of the handle by design, so a different weight makes a different
participant with provenance intact — and confirmed this checkpoint,
`persona.Persona{` is constructed exactly once in the whole program
(`tui/tui.go:514`, `defaultPersona`, itself called from exactly one site,
`tui/tui.go:478`). Nothing here is decided; it is recorded so the next
session inherits the founder's framing and the CEO's specific
disagreement with it, rather than either being lost or being silently
folded into a roadmap commitment neither made.

---

## D57 — A recorded control token was spelling a role boundary on the
wire, and the ledger fell a session behind the tree that fixed it

**2026-08-14 (commit), written 2026-08-14 (ledger, one session late — see
(h)). Status: mixed, per clause.** (a) is asserted, carried from the
commit message and not independently reread from the live record. (b)
and (c) are tested, against `persona/boundary.go`, `persona/client.go`
and `persona/boundary_test.go` as they read in the tree today. (d) is a
ruling. (e) is a ruling refused, tested that the deferred question is
still open. (f) is a correction, and this entry adds a second correction
the commit did not make. (g) is tested, re-derived live this checkpoint,
and one figure disagrees with the commit's own — the disagreement is
recorded, not smoothed over. (h) is this entry's own account of itself.

**(a) The trigger: the shareholder's first vote, in the same sitting,
surfaced a real defect in what D51 built. ASSERTED, carried from the
commit message.** Tyler cast a vote for the first time and, in that same
sitting, hit a qwen3.5 reply that came back with chat-template control
tokens rendered as ordinary content, ending (per the commit message —
not independently reread from the live record this checkpoint, since the
record has moved since, per (g) below):

    …next steps or tasks?<|endoftext|><|im_start|>user <|system_message|>

This is D51's own justification landing exactly as argued: "no first
user, so we become one, and the record of building it is the proof"
(D51's title) predicted that driving the thing finds what review does
not, and here the shareholder — not a subagent, not review — is the one
who found it, one commit after D51(e) shipped (D56(c)).

**(b) The fix is D1 applied to the wire, not to the record. TESTED,
against `persona/boundary.go` and `persona/client.go` as they read
today.** Nothing is stripped from a stored bit — `persona/boundary.go`'s
own doc comment names the precedent directly, that `DefaultModel`
already ruled on `<think>` tags the same way ("a mangled bit in a store
that never forgets is a worse record defect than the one it fixes").
`Escape` (`persona/boundary.go`) puts a backslash immediately after a
marker's opening bracket and is applied in `Client.Reply`
(`persona/client.go`) to every turn and to the System text, confirmed
this checkpoint by reading both files — it lives in the client rather
than in `Model.turns()` (`tui/ask.go`) specifically so every caller gets
it, including the third-party branch that concatenates a `Handle.Display`
into content. It matches three bracket **shapes** — chatml-style
`<|…`, `[NAME]`/`[/NAME]`, and `<s>`/`</s>` — rather than a fixed token
list, because each model family parses only its own vocabulary and a
list would have to be maintained against every family forever and would
silently miss the next one. The over-inclusion this buys is deliberate
and stated rather than implied: `[D56]` is untouched because the digit
ends the name, while `[TODO]` and a bare `<|` are escaped, per the
commit message's own worked examples.

**(c) What was measured before anything was built, and one experiment
that was thrown away. TESTED, against `persona/boundary_test.go` and the
seam claims it backs (confirmed in (g)); the underlying ollama figures
are carried from the commit message and were not independently re-run
this checkpoint.** ollama 0.17.7 has no `/api/tokenize`, so
`prompt_eval_count` on a one-message chat with `num_predict 1` was used
as the token counter. On qwen3.5, `<|im_start|>` as user content costs
one token where a near-miss `<|im_startX|>` costs seven — evidence the
token is being parsed out of content rather than carried as text. A
forged three-message conversation, with the forged turn's content
spelling out two additional messages inline, produced **identical**
prompt token counts and identical replies at temperature 0 against a
genuine five-message conversation, across three model families (qwen3.5
45/45, llama3.2 60/60, ministral 575/575, per the commit message). The
first attempt at proving forgery was behavioural — plant a contradicting
fact in the forged turn and see if the model repeats it — and it was
discarded: the model answered the planted fact in every arm, including
the neutralised and cross-family ones, because plain text inside an
assistant turn is persuasive on its own regardless of whether it forges
a role boundary. That test measured persuasion, not forgery, and the
commit message is explicit that the consequence outlives the fix:
**escaping stops a forged role boundary and does not stop recorded text
from being persuasive**, and nothing in this fix claims otherwise.

**(d) RULING: the persona is not told a marker was escaped, narrowing
D35's idiom to what it actually covers.** D35 established that a fold
leaves a hole a model is told about rather than left to fabricate across
silently — but that idiom is about **absence**: material genuinely
missing from what the model sees. An escape removes nothing; every
character of the original survives in the transmitted text, just with a
backslash inserted. There is no gap to disclose, and a disclosure note
on every turn where a model merely quotes a token back would be noise
rather than information. **Stated reversal condition, carried from the
commit message:** reverse if a model visibly stumbles on escaped text.

**(e) RULING REFUSED: whether the surface renders a raw control token is
not decided by this entry. TESTED that the question is still open.**
Marking an escaped or raw marker on screen would put a rendering decision
on top of evidence, which `tui/ask.go`'s own stated rule already refuses
to do to a fragment — confirmed this checkpoint at `tui/ask.go:380-381`,
"the record is evidence, and a marker inserted into a participant's own
words cannot afterwards be told apart from something they said." This is
`tui-design-engineer`'s call, not the CEO's or `principal-go-engineer`'s,
and it is recorded in `docs/DEBT.md` rather than ruled on here — added by
this commit, confirmed this checkpoint under the heading "The surface draws a control token as
ordinary text, and whether it should is undecided." Re-run live this
checkpoint per the debt entry's own instruction: `grep -rn
"endoftext\|im_start" tui/*.go` returns nothing — the surface still does
not mark anything, so the question is genuinely open, not quietly
resolved by an unrelated change.

**(f) A correction to the CEO's own framing, and a second correction this
entry adds that the commit did not make.** The commit corrects the
CEO's stated exposure — "the moment a second participant can write
bits" — on two axes: narrower, because the exposure is same-family-only
(a mistral persona is inert to qwen tokens now on the record); and
worse on timing, because `tldr say` had already shipped hours earlier,
so the exposure was already live that day rather than merely future. **A
second correction, found this checkpoint and not stated in the commit:**
the corrected sentence — "same family as the writer" — does not appear
anywhere in the tree. `persona/boundary.go:30`'s own doc comment still
reads the pre-correction sentence verbatim, "it stops being so the
moment a second participant can write bits, which `tldr say` already
allows" — confirmed this checkpoint, `grep -n "second participant\|same
family" persona/boundary.go` finds only the uncorrected line, and `grep
-rn "same family\|cross-family" persona/*.go docs/*.md` returns nothing
anywhere in the tree. So the correction in the commit message is a
correction to how the CEO described the exposure out loud; it did not
land in the code comment that makes the same claim. Recorded rather than
silently fixed, per this seat's own standing instruction not to quietly
repair a finding while cataloguing it (MMO-15's precedent, D45).

**(g) Receipts, each re-derived this checkpoint, with one disagreement
against the commit's own figures.** `sh .githooks/pre-commit`: exit 0,
five packages, all `ok` — re-run live this checkpoint, matching the
commit's claim. `docs/CLAIMS.md` holds 44 declared claims
(`grep -c "^id:" docs/CLAIMS.md` = 44), and the four new ones the commit
names — `boundary-sent-unescaped`, `boundary-forgets-a-family`,
`boundary-drops-what-it-escapes`, `boundary-escapes-through-the-caller`
— are present and `sole: true`, confirmed by reading the diff directly. A full catalog run from a freshly built
`cmd/seam` binary against a still tree this checkpoint returned **44
claims, 44 proven, 0 killed-mid-check, 44 as declared, 0 adrift, exit
0** — this **disagrees** with the commit's own reported "43 proven, 1
killed-mid-check (the declared `store-unlocked` coin)". The disagreement
is not a regression: `docs/CLAIMS.md` itself declares `store-unlocked`'s
companion claim `store-unlocked-kills-the-reader` as honestly
nondeterministic — `verdict: proven|killed-mid-check`
(`docs/CLAIMS.md:273`, and the general policy at `docs/CLAIMS.md:51-55`)
— because removing the store's lock usually crashes the reader before it
can assert, but not always. This checkpoint's run happened to land on the
`proven` branch of that coin where the commit's run landed on
`killed-mid-check`; both are valid outcomes of the same claim by its own
declared contract, and the total (44 proven-or-killed, 0 adrift) matches
either way. Recorded per the standing rule under "Being wrong is cheap;
being wrong and unmarked is what costs" — the measurement that disagrees
is the one written down, not silently replaced by the commit's figure.
The live record is unchanged across this checkpoint's own read, sha256
`4372b025dd7a07eec9fd1b2cfe391e8f08ef104d3fcbcfed17d6a52bc79dd181`
(`sha256sum ~/.local/state/tldreddit/record`), matching the commit
message's abbreviated `4372b025…` at both its start and finish.

**(h) The continuity gap this entry closes, and what it cost.** Session
16 landed exactly one commit and pushed it, but it wrote **no handoff and no
decision entry**. `grep -c '^## D' docs/DECISIONS.md` was 56 before this
entry, matching the charter's own numbered list, while the tree already
carried a commit's worth of ruling beyond it. This entry is that
session's decision-shaped content, written a session later, re-derived
from this checkpoint's own reads of the files the commit touched — not from
the session that did the work, which left no transcript-independent
trace of its reasoning beyond the commit message itself. Marked
throughout above: clauses (b), (e), (f), (g), and the git figures in
this clause are independently re-derived this checkpoint; clause (a)'s
quoted record text and the specific ollama token counts in (c) are
carried from the commit message and not independently re-run. Concretely,
this means "When the handoff and the tree disagree," rule 5's guarantee —
that reading only the most recent file in `docs/handoffs/` is sufficient
on arrival — was **false for one session**: a reader following that
protocol between that commit and this entry would have read session 15's
handoff, learned nothing of the boundary-escaping fix or that it had
happened at all, and had no signal that a session had run in between.
No handoff exists to correct for session 16 specifically — this entry is
the only repair, and it repairs the ledger, not the missing handoff
file, which stays missing.

---

## D58 — The fold budget becomes a screen, the query surface is decided closed, and research becomes a standing beat

**2026-08-14 (session 17, three commits, all pushed). Status: mixed, per clause.** (b), (c),
(f), (g), (h), (i), (l)'s vttest re-check, (p), (q) and every git/file
figure in this entry are re-derived live this checkpoint. (a), (d), (e),
(j), (k), (m), (n), (o) are RULING or research-pass content this
checkpoint records rather than re-derives, since there is no commit
carrying the reasoning — marked per clause. (r) is an explicit
non-reproduction, kept rather than silently dropped.

**(a) RULING: the query surface stays cut, on a collapse condition
rather than a closed door.** `scope-adversary` argued for building one —
a result set at 3 standing votes in 28 said bits (`docs/CLAIMS.md`;
confirmed this checkpoint, `go run ./cmd/tldr top` reports "28 said of
34 bits on the record · 3 ballots, 3 standing") returns, in expectation,
half of one vote, thinner signal than `tldr top` survives by printing
everything and confessing the split in its own header. Its honestly-made
best counter-case was that searching *behind scars* is something `grep`
cannot do — D14's reachability meeting retrieval, a real gap if true.
Checked and **false**: `top` already reads the whole record rather than
only the shown view (D56(b)'s own finding), and `top -n 0 | grep` already
searches behind every scar — confirmed this checkpoint, the flag exists
(`go run ./cmd/tldr top -h` prints `-n rows … 0 for all of them`). The
one figure fed into this clause that has since moved: at the time this
argument was made this session the shown view was 7 bits; re-decoded
this checkpoint (`memory.ReadStore` + `memory.ReadViewAgainst` against
the live record) it is **8**, `store.Len()` **34**, one more of each than
D57(g)'s reading — the record moved under active use between the two
reads, not a disagreement to reconcile. **Standing trigger, recorded
verbatim: Tyler reports, unprompted, from real use, that he went looking
for something in the record and could not get to it.** One such report
reverses this.

**(b) The fold budget is a screen, not a bit count, closing
the third inline `CLAUDE.md` debt bullet.** Before: 120 bits through the
surface held a view of 7/10.0/13 bits at 60x14 *and* 200x80, identical —
the surface had no opinion about the screen at all. After, `coolFloor =
12` (renamed from `coolAt`, which no longer exists in the tree — `grep
-rn coolAt tui/*.go` returns nothing, `coolFloor` and `Model.budget()`
do) plus `Model.budget()` = `max(viewport.Height(), coolFloor)`:
13/18.6/24 at 100x30, 31/52.4/74 at 200x80, no terminal worse off than
before. D18(e) asked for exactly this ("a screen in rows").

**(c) The mid-round cut, measured worse than D56(k)(2)'s two-voice
figure and closed the same session it was found.** Single voice: 24 of
60 rounds (40%) at 100x30 with nobody voting stranded a question below
its answer's scar — D56(k)(2) had measured ~25% for a two-voice design
that never landed, so single-voice was the worse case all along and
nobody had measured it. `keep` moved back to the human's last turn:
**0% at every budget and vote rate tried**, for about a tenth more
folds. Parity had been doing the load-bearing work before the fix — an
odd `keep` against a two-bit round orphaned the head in 90-94% of
frames, an even one in 36-42%, a screen whose worst behaviour turned on
whether a number happened to be even. Rejected: nudging the cut forward
instead of back — it also tidies the boundary, but takes a bit the plain
half kept, possibly the one under the caret; on a surface promising
consolidation never deletes, only the direction that keeps more was
available.

**(d) Review caught a falsehood the change itself introduced, before it
shipped.** `Model.keep` could return `len(view)-1`, cutting to a single
bit — D32's size rule refuses this — and `Absorbing` came back empty
with `blocked` true and the footer printing `held`, with **no vote
anywhere in the record**. Seeded per-session sweeps: 0.3/1.7/3.5/2.9% of
fresh sessions at one human bit in 3/5/10/20, peaking at the `tldr
say`-from-a-swarm shape. Fixed by a clamp at `len(bits)-2` inside
`keepFrom`, stating D32 directly; confirmed able to fail by mutating the
clamp to `len(bits)` in a throwaway copy outside the repository, which
compiled and took `TestNothingOnScreenSaysHeldUnlessSomethingIsHeld` and
`TestTheFoldsWindowIsNeverASingleBit` red.

Two sub-corrections inside the same change, both recorded rather than
smoothed over. The ceiling change review also asked for was made, found
redundant, and **reverted** — the CEO's stated reason for wanting it
("a keep at or past the budget leaves the trigger true") is false *at*
the budget, because after a fold the view is a scar plus `k` bits and
`foldable` skips scars. And removing the ceiling does the *opposite* of
a fold storm: 400 bits went from 19 folds to 6, worst view 275 to 300 —
the fold stops taking anything, which is not the failure the ceiling was
built to prevent.

**(e) RULING, and a reversal of it inside the same session.** The header
date drew whatever zone the value carried — live bits carry
`time.Local`, reloaded bits carry UTC, so the same record read `19:47`
live and `01:47` after a restart, six hours off and a day wrong on the
header, not only hours. `memory/wire.go` is not at fault (D12: one
normalized instant, and `memory/bit.go`'s own doc for `At` already
warned whoever draws it must decide). The CEO first ruled the drawn
clock uses the reader's local zone on every surface, fixed in
`Model.day` — the header reads `2026-08-14 local`, not an abbreviation
like `MDT`, because an abbreviation reads as a fact about when it
happened. `tui-design-engineer` then reported that
`cmd/tldr/top.go:318-326` carries a stated argument the ruling never
reached: that reading is read by a session on another machine as often
as by a person, and RFC 3339 because it sorts, greps and parses. **The
CEO reversed**: the TUI header draws local, `top` keeps RFC 3339 UTC,
and the divergence is deliberate and named in `Model.day`'s own doc
comment, confirmed present this checkpoint.

**(f) `askCeiling = 60`, and it was a live defect, not a theoretical
one.** The row budget silently became the persona's context budget — at
200x80 every ollama request carried up to 74 bits. Measured ~60
tokens/bit flat across model families by `prompt_eval_count`, prompt
growth stopping at ~4096 — **which is ollama's default `num_ctx`, not
the model's own context window** (`/api/show` reports 262144 for
`qwen3.5:latest` and `ministral-3:14b`, 131072 for `llama3.2:1b`, 40960
for `qwen3:8b`; this client sets no `num_ctx`, so the server's default is
the real ceiling). 74 bits was asking for more than fit in under 68.
D18(e) wants two budgets, in rows and in tokens; this is a stand-in for
the second, not the design it asked for. Nothing previously measured is
invalidated — the old budget was 12 bits, ~720 tokens, nowhere near this
cap; the defect only became live once the row budget in (b) let the
view grow past a few dozen bits.

**(g) An upvote no longer folds away the question it was cast on.** Holding an answer kept the answer and cooled the run in
front of it, which is where its question was, drawing one kept row
wedged between two scars. Stranding, measured the same way on both sides
against a scratch copy from before this fix landed: **91.2% of frames at one upvote in
three rounds, 92.8% at one in five, 93.5% at one in ten, 0% with nobody
voting. After: 0.0% at every rate tried.** `memory/view.go` gains
`sparing` — the held set, plus every id a held-and-hot bit names through
`Prev` — and `runs` splits on that instead of the hold map; a spared bit
passes through as a singleton exactly as a held bit did, so D32's size
rule and the receipt-span argument are untouched. The rejected shape was
a hold reaching back through `Holds`, which changes what a *vote is*;
this changes what a *fold does* instead — `Stay.Holds` is untouched and
still returns votes only, so nothing anywhere reports a ballot nobody
cast, which matters because (d) had just fixed the identical falsehood
from the other direction. D1 and D14 re-derived and both hold — the scar
above a stranded row still names the question in `Prev` and `Absorbed`,
`TestEveryStoredBitIsReachableFromTheView` passes unchanged — so this
was a legibility defect, which is why the fold half landed and the
display half (clause (s)) did not.

**(h) The CEO's stated reason for the `hot` guard was wrong, and the
guard stayed anyway.** The ruling claimed the guard was load-bearing
because D13 makes a derived bit's `Prev` the whole window, so upvoting a
scar would pin everything it names. Unreachable in fact: a
`Compaction`'s `Prev` names exactly the bits it replaced, and replaced
means they left the view in the same operation, so no view built by
`Add` and `Fold` ever holds both a scar and a bit that scar names.
Removing the guard produces **byte-identical output across 42 simulated
conversations of 400 bits**. Kept anyway, four lines against the loss of
the whole fold, but as a prior rather than a defence — the
unreachability is now executable, `TestAScarInAViewNeverNamesABitStillInIt`,
rather than left as an assertion.

**(i) The storm that is real, and three readers missed it until this
checkpoint's re-read of the commit.** Sparing two rows per hold instead
of one doubles hold density, so free runs shrink and D32 refuses runs of
one — **the vote rate at which the record stops folding at all moves
from one in two to one in three.** The two hold-decay regimes disagree
completely: at the surface's own `holdFor` (2m against a 3.5s cadence)
folds go *up* and scars per frame at one in three drop from 7.61 to
0.91 — fewer, larger receipts instead of a rash of two-bit ones; at a
30-minute hold on the same cadence, one vote in three folded 16 times
before and **zero** times after. Same code, opposite conclusion,
depending only on how long a hold lives against the bit cadence. The
check that catches this is a fold count, never a stranding rate — D36(h)
in a new place.

**(j) Three untested boundaries in `memory/`, found by a tool we do not
own, in seventeen seconds.** `go-gremlins/gremlins`, run out of tree
against a copy, enumerated 106 mutants in `memory` and 8 survived;
`cmd/seam`'s claims covered none of them, because a claim only exists
where somebody wrote one. Closed: `view.go`'s `left > 0` → `>= 0` (the
hold's expiry boundary, `Stay.For`'s own doc already called it
half-open); `view.go`'s `keep < 0` → `<= 0` inside `Absorbing` (`Fold`'s
identical guard was swept, this second statement of the rule was not);
`wire.go`'s `n < 0` → `<= 0` (a compaction counting zero bits of a kind
loads clean, only a negative one is corruption). Declined with reasoning:
`wire.go`'s `clip` boundary `len(s) <= 24` → `<` — the whole effect is a
24-character value gaining an ellipsis, no caller branches on it, and
pinning it couples a test to a width constant that exists to be tuned.
Not chased: `id.go`'s `Short` is a provably equivalent mutant
(`id[:8] == id` at length 8); three `i+1` → `i-1` survivors are inside
`fmt.Errorf` arguments only.

**(k) DECISION, org: research becomes a scheduled beat rather than a
felt need.** Tyler raised it; the CEO's reasoning is that **the cost of
not looking is invisible** — every other failure here announces itself
(a red test, a stale line a grep catches), duplicated work produces
nothing observable, so it cannot be triggered by a felt need the way the
rest of this project's instruments are. D40 already granted both
building seats `WebSearch`/`WebFetch`; the capability sat unused for
days until dispatched explicitly this session, which is itself the
evidence — capability without a mandate produced exactly one research
pass in the project's life. In force: **one seat per checkpoint,
rotating**, not every seat every time; one named trigger — **before
building any instrument, look for one that exists**; craft records gain
a second kind of entry, what a seat learned by *looking*, kept
distinguishable from what it learned by building; the CEO takes the
prior-art beat itself, with D56(i) as the receipt that it already paid once.

**(l) DECISION, org: `decision-guard` and `scope-adversary` get
`WebSearch` and `WebFetch`, partially revisiting D40.** Both carried
`Read, Bash, Grep, Glob` only before this session — confirmed by reading
both frontmatters before editing. A reviewer that can check our claims
against the tree but not a claim about a library, and an adversary that
cannot make the strongest argument against building anything —
*someone already built this, here is how it went* — were both missing
the same tool for the same reason. D40 declined them craft records on
the grounds that nothing showed they accumulate tool-craft; the CEO's
correction here is that this conflated two things, and research
capability is not craft memory. **They get the capability and still no
craft record.** Done this checkpoint:
`.claude/agents/decision-guard.md` and
`.claude/agents/scope-adversary.md` frontmatter now read
`tools: Read, Bash, Grep, Glob, WebSearch, WebFetch`, nothing else
changed, and both still carry the D47 read-first instruction —
`grep -ci "read first" .claude/agents/*.md` returns 1 for all five
agent files after the edit.

**(m) The tooling research, with verdicts, kept apart from what was
built with it.** **ADOPT** `gremlins` as a hand-run sweep, never in the
gate, zero repository lines — clause (j) above is its first payoff.
**Record its own D27-shaped defect, the most valuable part of the
finding**: its per-mutant timeout is the wall time of its own coverage
run × 3, so a warm Go test cache turns every mutant into TIMED OUT — 0
killed, 0 lived, 0.00%, **exit 0, no warning**; and because efficacy is
computed as `killed/(killed+lived)`, timeouts leave both terms at zero,
so **a tighter timeout raises the reported score while hiding
findings** — 96.84% with only cosmetic survivors was measured against
92.31% with the four real survivors clause (j) closed. Also:
coverage-gated mutant selection never mutates a `const` block, so
`Up`/`Down` in `memory/vote.go:20` are never tried by this tool at all,
and D33 says those two identifiers reach content addresses. **ADOPT**
`pgregory.net/rapid` narrowly, for pure functions with stateable bounds.
**REJECT** `charmbracelet/x/exp/teatest/v2` — its golden frames are
escape streams, so a golden pins Bubble Tea's own renderer diff
strategy, not this program's behaviour. **DEFER**
`charmbracelet/x/vttest` — it genuinely closes the gap
`tui/harness_test.go`'s own doc names as open, deferred only on
dependency cost (8 modules, two untagged, one a 2017-era freetype);
`charmbracelet/x/vt` was re-checked **this checkpoint**, independently
of the research pass that first found it (`pkg.go.dev` shows only
`v0.0.0-*` pseudo-versions, no tagged release), so the deferral stands
on fresh evidence, not carried assertion.

**(n) `cmd/seam` has no prior art for the composition, and the CEO's own
earlier framing of why was wrong.** The CEO had said nobody writes docs
that make checkable claims. False: the traceability industry — DO-178C,
strictdoc, doorstop — has written exactly those docs for decades. What
nobody in that space does is **falsify the citation**: they link a
requirement to a test by identifier and stop at "the test exists and is
green," which is precisely what `docs/CLAIMS.md`'s own header rejects in
its second sentence. This is a materially better statement of the moat
than the CEO's original claim, and it is the true one, which is why it
is recorded as a correction rather than folded silently into the old
framing. Nearest neighbour found: an automatically computed kill matrix,
which `gremlins` does not emit — it reports status per *mutant*, not per
*test*.

**(o) The web-UX research, and four things this project invented that
have better-established forms.** Worse than established, three findings:
the footer drops keys with nothing saying anything was dropped, on a
surface whose own package doc argues that cannot happen, while `said`
degrades to a dash and `bubbles/help` writes an ellipsis when it drops a
binding; the interactive ranked screen states no ordering while `tldr
top` prints counts, ballots, bands and who it is ranked for — **the
correct form was already built, in the CLI, not the TUI**; the stranded
answer's solved form is the **parent stub**, and `ctrl+u`'s Details
component is measurably the weaker choice, because GOV.UK's own
component guidance says not to hide information most users need behind
one, since most people do not click it. Two zero-cost adoptions, both
already paid for: the renderer's own layout step gives a real cell grid
in three calls and `ultraviolet` is **already an indirect dependency**
(`go.mod:15`, confirmed this checkpoint); `lipgloss.Wrap`
(`charm.land/lipgloss/v2`'s own `wrap.go:12`, confirmed present)
preserves styling across a line break where `ansi.Wrap` drops it. One
finding kept as an open decision rather than adopted or rejected:
`note()` has one slot where snackbar and banner are two separate
components everywhere else surveyed. Genuinely ours, no better form
found anywhere in the sweep: the gauge, the fade, the scar's
navigability (elision-with-a-count is everywhere; elision you can
*follow to the originals* is not), and `edge()`. Recorded honestly: the
seat marked four of seven web patterns `[recalled]` because citation
fetches failed mid-research, and those four should be discounted
accordingly rather than treated as equally sourced.

**(p) RULING: the vote stays a magnitude and does not carry a reason.**
Google Docs lets a user *name* a version so it is never merged away —
this project's upvote, invented independently before this comparison was
made, differs in that theirs carries a reason and ours a magnitude, so a
scar can say what it absorbed and never why any of it mattered, which is
the auditor's question. D4 stands, and the ruling is made on evidence
rather than principle: Tyler has cast three votes this project's
lifetime (confirmed this checkpoint, `go run ./cmd/tldr top` reports "3
ballots, 3 standing"), and if voting cost a sentence each time he would
plausibly have cast zero, while D51(d)'s named blocker is votes cast,
not votes explained. The resolution: **a reason is a different gesture,
and the forum shape already has one** — `tldr say` puts a bit on the
record and that bit is itself votable, so a reason can be filed as its
own bit and ranked like anything else. The link between a specific vote
and a specific bit *about* that vote is **deferred and not built**, and
recorded here as deferred rather than implied.

**(q) A void `cmd/seam` run, recorded next to the clean one rather than
discarded, per D56(g)'s precedent.** A catalog run was started while a
research dispatch was live against the same tree; that dispatch wrote to
its own craft record mid-run, and seam correctly reported the tree
moving — `adrift 1 · taken elsewhere 43`. The sequencing error was the
CEO's, running two things against one tree at once. The clean re-run,
re-derived this checkpoint against a still tree: **46 claims, 45 proven,
1 killed-mid-check (the declared `store-unlocked` coin,
`docs/CLAIMS.md:273`'s own honest `proven|killed-mid-check` contract), 46
as declared, 0 adrift** — matching `grep -c "^id:" docs/CLAIMS.md` = 46.

**(r) The gate found what the hand-check did not, for the second time.**
The clean run returned `cool-reads-the-clock` **over-red**:
`TestAHeldScarSparesOnlyItself` and
`TestAHoldWhoseQuestionHasGoneSparesOnlyItself` (`memory/sparing_test.go`)
also notice a `Cool` that stamps its own clock, because both compare a
fold against bits they built themselves. The builder hand-ran every
affected mutation and correctly predicted `a-lone-bit-is-cooled`
shrinking from ten names to eight; it did not predict this one, and
neither did the CEO. Five checks became seven. **This is the second
instance of a prediction `.claude/craft/principal-go-engineer.md`
already made**, which is what makes it a pattern rather than a one-off
miss. Standing cost of `sole: true`, and the CEO's ruling: kept anyway.
The alternative on offer computes a mutant's status per mutant and not
per test, which is not the same fact `sole: true` needs, so the cost is
logged as evidence against D53(d) rather than acted on.

**(s) A figure that did not reproduce, marked unreconciled rather than
replaced.** `docs/DEBT.md` recorded 92% stranding at one upvote in two;
clean re-measurement this session gave **22.5%**, under both vote phases
and both keep rules — not the cut in (g), since it was measured
separately on both sides of it. The other three figures in that entry
reproduced within a point. The original harness that produced 92% was
discarded and cannot be re-run, so this is marked **unreconciled**, not
silently corrected to the new number.

**Open, named to a seat, not built here.** `tui.foldable`
(`tui/tui.go:1494`) counts hot-and-not-held, so a spared bit is now
overcounted and the fold trigger fires up to one bit early per hold —
one line, `tui-design-engineer`'s. A covered row draws exactly like a
plain hot row, so someone who upvotes an answer watches a second row
stop fading with nothing saying why — a third visual state and a
decision, deliberately not taken here. Three `tui/tui_test.go` fixtures
moved and are flagged for that seat.

**Receipts.** `sh .githooks/pre-commit`: exit 0, five packages, all
`ok`, re-run live this checkpoint. `go run ./cmd/tldr top`: "28 said of
34 bits on the record · 3 ballots, 3 standing." `git rev-list --count
HEAD` = 80 (up from 75 at D56/session 15); `git status --short` empty. The
public tree is unchanged: 3 commits at
`e4d67960c976df8292b05cb55fa55bbabefb035c`, both local `HEAD` and
`git ls-remote origin HEAD` agreeing, not touched this session. `grep -c
'^## D' docs/DECISIONS.md` = 57 before this entry. 9,846 lines of
product Go (`find . -name '*.go' -not -name '*_test.go' -not -path
'./cmd/seam/*'`, tail total, up from 9,228), 2,182 lines of `cmd/seam`
product code (unchanged), 15,041 lines of tests outside `cmd/seam` (up
from 14,800; `cmd/seam` carries a further 1,224 lines of its own tests,
counted separately by the same convention as every prior checkpoint).
332 `Test` functions (`grep -rhoE '^func Test' --include='*_test.go' . |
wc -l`, up from 309). The live record, read-only this checkpoint,
sha256 `4372b025dd7a07eec9fd1b2cfe391e8f08ef104d3fcbcfed17d6a52bc79dd181`
— unchanged from D57(g), and `store.Len()` 34 / shown-view 8 / votes 3
decoded directly, per (a) above.

---

## D59 — A founder's redirect is a reorder, the charter said a simulator didn't exist, and a claim may not answer to the calendar

**2026-08-15 (session 18). Status: mixed, per clause.** (b), (c), (m),
(n) and every git/file figure in this entry are re-derived live this
checkpoint, several independently of the dispatch that first reported them
(clauses (c) and (n) each re-run in a detached worktree by the archivist,
not taken on report). (a), (d), (e), (f), (g), (h), (i), (j), (k), (l),
(o) are RULING, research-pass, or in-session measurement content this
entry records rather than re-derives, since there is no commit carrying
the reasoning — marked per clause, with what the archivist independently
spot-checked named inline.

**(a) RULING: Tyler's stated direction is a reorder, not a rewrite.**
Verbatim: he wants a person to open the TUI "like they've never used the
commandline before," see it, think "oh … wow. this is cool," and **learn
about computers in a super personalized way**. The terminal is the one place a non-technical person does not
expect beauty, so the expectation violation does real work, and this
program does not *teach* content-addressing, consolidation and
attention-as-retrieval so much as **be** them, visibly, which is a
stronger claim than any tutorial text could make. Named risk, not
smoothed over: if what a person learns is *how tldreddit works* rather
than a concept that survives leaving the program, the claim is vocabulary
rather than substance (the same test D46 already applies to this seat's
own charter). Named blocker, not pretended away: a stranger cannot
install Go or run ollama today — this is a distribution problem, not a
TUI problem, and nothing here proposes solving it.

**(b) TESTED, and it corrects a false statement `CLAUDE.md` has carried
since at least session 6: a deterministic simulator already existed.**
`simulate()` at `tui/harness_test.go:567`, swept by
`TestHarnessHoldSchedule` at `tui/harness_test.go:616` — confirmed by
line number this checkpoint. It is exhaustive over an enumerated grid
(holds × cadences × vote-rates), not seeded: `grep -rln math/rand
--include='*.go' .` returns nothing anywhere in the module, re-run this
checkpoint. The real gap, verified against the source rather than
asserted: `simulate`'s fold trigger and cut are hardcoded to `coolFloor`
and `coolFloor/2` rather than taking them as parameters, so two of the
five axes the old `CLAUDE.md` bullet named (fold trigger, `keep`) are
unreachable by the sweep; `simulate` returns `(worst, folds, votes)`, so
stranding is measured nowhere; and nothing runs it unattended — the gate
never sets `HARNESS`, and `cmd/seam/run.go:162` explicitly clears it
(`cmd.Env = append(cmd.Environ(), "HARNESS=")`), confirmed by reading the
line. `CLAUDE.md`'s inline debt bullet is rewritten to say this instead
of "no deterministic simulator exists."

**(c) TESTED, independently re-derived in two detached worktrees rather
than taken from the dispatch that first raised it: D58(i)'s headline
figure ("16 folds before, zero after") did not reproduce at the harness's
own committed default, and the schedule is now recovered and complete.**
At `TestHarnessHoldSchedule`'s hardcoded `const bits = 400`, 30-minute
hold, 3.5s cadence, one vote in three: pre-sparing-fix (a
worktree, `HARNESS=1 go test ./tui/ -run
TestHarnessHoldSchedule -v`) gives **32** folds, not 16; post-fix
(same command) gives **0**. At `bits = 200` — edited into a
throwaway copy of each worktree, never committed, worktrees removed after
— pre-fix gives **16**, post-fix gives **0**, exactly D58(i)'s original
figure, at the specific bit count D58(i) never stated. This is the
**third** instance of D36(a)'s rule that a measurement is not sound
without the schedule it was taken against (D36(a) itself, D58(s), now
this), and in every instance the cause is the same: the number came from
a harness run that was not the one committed to the tree, or was, but
without the parameter that decided its value.

**(d) DECISION, from `decision-guard`'s first rotation of the D58(k)
research beat: neither build nor adopt a deterministic-simulation-testing
tool for this project, on a category argument rather than a
library-by-library one.** **REJECT** the whole family (`gosim` dormant,
`simtest-go` a 2-star hobby repo, FrostDB a forked Go runtime, Antithesis
commercial, MadSim/Turmoil in Rust, Tickloom in Java, a curated
awesome-list naming zero Go libraries) — that family exists to buy back
determinism that goroutines and I/O took away, and `Fold` has neither.
**REJECT** discrete-event simulation (`simgo`, `godes`) — one goroutine
per simulated process would *introduce* the scheduling this program does
not have. **REJECT** `benchstat` (built for noisy samples; this project's
are exact). **REJECT** snapshot libraries on dependency cost — this repo
carries zero test-assertion dependencies today, and `autogold/v2` pulls
twelve modules including `gofumpt`. **REJECT** clock libraries (`Fold`
takes no clock to inject). **REJECT** `go test -fuzz` for this purpose.
**ADOPT** `pgregory.net/rapid` narrowly, re-confirming D58(m) — v1.3.0,
zero dependencies (confirmed via `pkg.go.dev`, fetched this checkpoint),
and the split worth stating: rapid covers the *invariant* half (D58(d)'s
falsehood is exactly a rapid property, with shrinking), nothing covers
the *measurement* half, which is a `for` loop. **Not yet integrated** —
`grep -rn "pgregory.net/rapid" --include='*.go' .` and `grep -n rapid
go.mod` both return nothing this checkpoint, so "ADOPT" is a standing
verdict on the tool, not a built thing. **DEFER** `testing/synctest` —
stable since Go 1.25.0 (confirmed via `pkg.go.dev/testing/synctest`,
fetched this checkpoint), present in this project's installed `go1.25.4`
toolchain, zero cost to adopt; condition to actually adopt is somebody
already editing `tui/tui_test.go:1470-1530`, the package's one
wall-clock-dependent test. **A named consequence: `docs/DEBT.md`'s claim
that this test "is a second argument for that simulator" is now void** —
`synctest` closes the gap for free, independent of whether a simulator is
ever built. `docs/DEBT.md` corrected accordingly this checkpoint.

**(e) Two more D27-shaped instrument defects in the tools clause (d)
adopts/defers, found by `decision-guard` executing rather than reading,
one leg independently confirmed by the archivist this checkpoint via
`WebFetch`.** `rapid`: `-rapid.checks=0` on the command line turns a false
property green at exit 0, and **`RAPID_CHECKS` is also an environment
variable** — confirmed present in `pkg.go.dev`'s own docs, fetched this
checkpoint — so a stray export makes an entire property suite vacuous
with nothing on any command line to see, in a repo that already keys
behaviour off one such variable (`HARNESS`) and had to teach `cmd/seam`
to clear it. `decision-guard` additionally reports rapid writes a `.fail`
file into the source tree on a failing run, in a repo where `cmd/seam`
was built specifically never to write inside it — **not independently
confirmed by the archivist**; `pkg.go.dev`'s page names "persistence and
automatic re-running of minimized failing test cases" as a feature
without stating the file's location, so this specific claim is carried as
reported rather than re-derived. `synctest`: inside a bubble `time.Now()`
starts at **2000-01-01 00:00:00 UTC** — confirmed via `pkg.go.dev`,
fetched this checkpoint — **26.6 years** behind 2026-08-15, so a bit
stamped outside a bubble and one stamped inside it are decades apart with
no error, quiet-wrong in exactly the subsystem (dates, clocks) whose
figures this project keeps having to withdraw (D36, D58(s), this entry's
own (c)).

**(f) Measured this session, by `tui-design-engineer`, not re-run by the
archivist.** `tldr`, empty record, 100×30 under a pty: 5
non-empty rows of 30, 4 distinct SGR codes, one colour landing on text
(`38;5;240`), 25 blank rows, four verbs, nothing to act on. Crush v0.89.0
in the same terminal with no API key: 21 of 30 rows, a filterable list of
40+ models, a two-verb footer. Archivist confirmed the tooling that
produced this exists and is versioned as claimed: `vhs --version` → v0.11.0,
`ttyd --version` → 1.7.4, `ffmpeg -version` → 6.1.1-3ubuntu5, all present
on this machine this checkpoint. Frame-level pixel counts not
independently recounted.

**(g) RULING, generalising (f) into one named pattern rather than three
separate complaints.** Cold open 5/30 rows used; the ranked screen after
two real votes, 5/30, with `kept · 2` as its only ordering statement
(confirmed live in a captured frame, `/tmp/tldrdemo/ranked.txt:808`,
re-read this checkpoint); a post-fold view, 14/30. In every case the CLI
already draws more in the same space — `tldr top` gives counts, ballots,
standing, bands and who it is ranked for. **Where the surface has rows to
spare, it should draw what `top` draws.** This is D58(o)'s "the correct
form was already built, in the CLI, not the TUI" landing for the second
and third time on the same finding.

**(h) The Crush/Charm study, run by `tui-design-engineer`, one claim
independently confirmed by the archivist.** Crush's cold open under
`NO_COLOR` emits zero SGR codes and stays legible — the wow is not
colour — but its onboarding distinguishes `Yep!` from `Nope` by
**background colour alone**, and its panes are byte-identical before and
after Tab under `NO_COLOR`. **That is the exact defect this project found
in itself in session 9 and paid two columns of every row to fix (D42); on
this axis this project is ahead of Charm's own flagship product.** Also
reported: one theme only, ship, with the second theme-lookup function
returning the first (REJECT a theme system here on the same evidence);
the onboarding footer names its own abridgement (`ctrl+g more`/`less`)
via `bubbles/v2/help`, a version this project already depends on
(archivist confirmed: `go.mod` requires `charm.land/bubbles/v2`); the
footer prints `ctrl+c quit` twice and names none of the working keys.
Nothing in Crush beats any of D58(o)'s four genuinely-ours items. One
thing Crush has that this project does not: a permanent per-answer
receipt row (author, model, provider, latency) — this project's
equivalent lives in a *waiting* row destroyed the moment the answer
arrives. A display gap, not a record gap — the record already keeps
everything the receipt would show.

**(i) DECISION: adopt VHS for capturing the demo/board material; REJECT
its own golden-file pitch.** Confirmed working end to end this session —
`vhs` v0.11.0 + `ttyd` 1.7.4 + `ffmpeg` 6.1.1, the latter two installed by
Tyler at the CEO's request. Files present and inspected this checkpoint,
`/tmp/tldrdemo/`: `cold.gif` (23,669 bytes), `fold.gif` (352,208 bytes),
`ranked.gif` (704,413 bytes), plus literal-text captures `populated.txt`,
`fold.txt`, `ranked.txt` — the last two independently grepped this
checkpoint and both contain the frames the dispatch described verbatim
(`fold.txt:488` and `:520` both carry `19 bits · 02:01–02:01 · name same
box s`; `ranked.txt:808` carries `kept · 2`). **This retires the standing
problem that the demo page's frames came from a 20-minute live-ollama run
that could not be regenerated** — a fresh capture is now cheap. REJECT
the tool's own README claim that its `.ascii` golden-file mode "ensures
there are no diffs between runs" — a D27-shaped instrument-that-cannot-
fail, this time inside a piece of marketing. Recorded alongside it:
Charm's own practice is better than their marketing — Crush ships 363
golden files, every one under a pure-function diff component, zero over
its 5,073-line interactive surface. Standing rule applied and recorded:
captures used for *evaluation* may be synthetic if labelled as such;
anything *published* stays a live run.

**(j) A finding about this project's own design, confirmed by line
number this checkpoint.** `tui/render.go:634`, `tui/unfold.go:477` and
`tui/ask.go:675` all call `topWords(c.Bag(), …)` — confirmed present at
those exact lines this checkpoint. That is the scar's on-screen summary
and what the persona is told about a fold, in both cases. `memory/cool.go`
documents `bag` itself, verbatim, confirmed at line 74: "a good index and
a poor record: it preserves what was discussed and destroys what was
said about it." Observed independently in two different fixtures: `name
same box s` (the `s` a tokenizer artifact) and `conversation migration
number about`. **The thing this project's own code documents as
destroying what was said is what this project uses as the record of what
was said** — on the single most-read row in the program, and in the
model's own memory of what it folded. Proposed, not built, named to
`tui-design-engineer`: the scar's label should be the top-*ranked* bit it
absorbed, not the top-*counted* word — ranking as summarisation, this
project's own thesis doing the work it already claims to be good at.

**(k) A pattern, now with a third instance, found only by running the
program rather than reading its code or its tests.** In each instance a
constraint is written down inside a file and violated by the surface
that file describes: `clip`'s own doc calls a silent cut "the one thing
this screen may not do," while `fit` cuts silently because every footer
ladder ends in `""` and the ellipsis rung is unreachable; the footer
drops four keys with nothing marking the drop; `cool.go` documents the
bag as destroying what was said, and the bag is what this project uses
as the said-thing's summary (clause (j)). Three prior reviews and a
claims checker caught none of the three. This is the strongest evidence
yet for D51 — becoming the first user rather than only reviewing the
diff — and it is the reason clause (a)'s redirect is accepted rather than
deferred.

**(l) Two smaller findings from this session's captures, confirmed
present in the literal-text frames re-read this checkpoint.** A record
seeded from outside the surface (`tldr say`) arrives **over budget** and
stays there — `/tmp/tldrdemo/ranked.txt:388` reads
`▓▓▓▓▓▓▓▓▓▓▓▓ 30/24`, and no fold fires until the human speaks through
the surface itself. And `tldr say -as local` is refused outright by
design — `cmd/tldr/say.go:28-29`'s own doc, confirmed this checkpoint:
"One handle is refused, and it is the person at the keyboard's own."
Framed honestly as both an obstacle (for a synthetic demo) and a property — the human's own turns are structurally the
human's, not forgeable from outside the surface.

**(m) TESTED, the unit that landed this session, read and
summarised from its own commit message rather than reconstructed.** D58
filed the `foldable` overcount as "one line" and "fires up to one bit
early per hold"; both were wrong, the second one badly — the overcount
never decayed, so the trigger sat permanently over budget and every
write fired a fold that took one short run, the exact failure `foldable`
exists to prevent, reintroduced through the very map it reads. Measured
through the surface's own trigger and cut, 400 bits at 3.5s cadence, 2m
hold: budget 23 (100×30), one vote in three, 122 → 30 folds, worst view
37 → 47; one in five, 37 → 22; budget 73 (200×80), one in three, 7 → 6;
budget 12 (60×14), one in five, 150 → 38; nobody voting, unchanged at
every budget. Review supplied the qualifier the builder's own table
omitted: at the `coolFloor` band the trigger half of the fix is fixed
(381 → 122 over-budget writes) but the outcome half is not (122 → 122
folds, 2.98 rows/receipt against 6.98 with nobody voting) — "122 → 30,
storm closed" without the budget would itself have been a D36-shaped
claim, and it is not written that way anywhere in the commit. The fix
needed the spared set, exported narrowly as `View.Sparing(s, stay)` on
`memory`, a cross-package change by `tui-design-engineer` on
`principal-go-engineer`'s ground, flagged rather than assumed and ruled
correct by review (D5 does not fire — two real call sites). The third
visual state (a covered row, drawn as a hanging `╷`) is free by
construction — the mark column exists only once somebody has voted, so
does a covered row, and it appears/vanishes at exactly the widths the
`▲` does, swept 1–120 columns. Review's verdict was **land it**: four
findings fixed before landing, none blocking.

**(n) A new failure shape, and a RULING adopted from review: a claim's
verdict must not be a function of the wall clock.** Independently
re-derived this checkpoint in a detached worktree of the pre-fix tree: `go run
./cmd/seam -run cool-reads-the-clock` there returns `over-red 1 · adrift
1 · exit status 2` — confirmed live, not taken on report. And at current
`HEAD`, the same command returns `proven` — also confirmed
live this checkpoint. Nothing in the claim, the test, or the code is
nondeterministic; what is nondeterministic is the *difference* between a
mutant that stamps `time.Now()` and a fixture that hard-codes a date
(`TestTheClockReadsTheSameWhoeverOpensTheRecord`'s `2026-08-14`) — the
verdict flipped at midnight with nothing committed, so `cmd/seam`'s own
receipt and D58(q)'s both said clean and both were true when written.
**RULING, adopted from review: `docs/CLAIMS.md` should not be able to
express a date-dependent verdict** — once a verdict is a property of the
day rather than of the tree, `proven` stops meaning what the file's own
header says it means. Not built: a clock-pinning mechanism now would be
D5 (a sub-team-shaped instrument ahead of a second instance); the cheap
fix if a second instance appears is a `seam` flag running the catalogue
at two synthetic dates.

**(o) Two dead test-name citations, confirmed by name this checkpoint,
both in append-only files, corrected here rather than edited there.**
`docs/DECISIONS.md:2278` (in **D39(c)**, "The downvote ships" — this entry
first said D57(c), which is wrong, and the CEO caught it before the entry
was committed: the nearest preceding heading to 2278 is `## D39` at 2220
and clause (c) opens at 2271. Recorded rather than silently fixed, because
a correction entry about dead citations that carries a wrong citation is
this project's own failure shape aimed at itself, and the draft was
corrected before landing rather than the committed record being edited) and
`.claude/craft/principal-go-engineer.md:1330` both cite
`TestHoldingEveryOtherBitBlocksTheFoldAndLettingGoReleasesIt`. That test
no longer exists — it was renamed to
`TestHoldingEveryThirdBitBlocksTheFoldAndLettingGoReleasesIt` (confirmed
by diffing the test file, renamed line for line). The old name survives only inside a doc comment at
`tui/tui_test.go:328`, referencing the new function. The craft-record
citation was found by review, not by the builder. Both files stand
uncorrected, per D append-only convention; this entry is the correction,
and any reader following either citation forward should read it as
pointing at the renamed test.

**(p) Named to a seat, not built here.** To `tui-design-engineer`: the
footer ellipsis (~6 lines, `fit` in `tui/unfold.go`); the cold-open and
starved-screen pattern per clause (g); the scar-summary fix per clause
(j); the parent stub, D58(o)'s highest-value finding, checked by the
builder this session and confirmed **not** the same surface as the tie
built this session. To whoever takes it: the simulator's two missing
parameters, a stranding return, and a golden table, per clause (b).

**Receipts.** `sh .githooks/pre-commit`: exit 0, five packages, all `ok`
(cached), re-run live this checkpoint. Inventory presence check (`for f
in $(find . -name '*.go' -not -path './vendor/*'); do grep -q
"$(basename "$f")" docs/CODE.md || echo "MISSING: $f"; done`): empty,
re-run live this checkpoint. `git rev-list --count HEAD` = 82 (up from 80
at D58/session 17); `git status --short` empty before this
checkpoint's own doc edits. Public tree
unchanged this session: `git rev-list --count HEAD` = 3 at
`e4d67960c976df8292b05cb55fa55bbabefb035c`, `git ls-remote origin HEAD`
agreeing. `grep -c '^## D' docs/DECISIONS.md` = 58 before this entry.
10,055 lines of product Go (`find . -name '*.go' -not -name '*_test.go'
-not -path './cmd/seam/*'`, tail total, up from 9,846), 2,182 lines of
`cmd/seam` product code (unchanged), 15,630 lines of tests outside
`cmd/seam` (up from 15,041), 1,224 lines of `cmd/seam`'s own tests
(unchanged). 340 `Test` functions (up from 332). 46 declared claims
(unchanged, `grep -c "^id:" docs/CLAIMS.md`). The live record, read-only
this checkpoint: sha256
`4372b025dd7a07eec9fd1b2cfe391e8f08ef104d3fcbcfed17d6a52bc79dd181`,
**unchanged** since D57(g). `go run ./cmd/tldr top`: "28 said of 34 bits
on the record · 3 ballots, 3 standing" — unchanged from D58.

---

## D60 — A correction ranks below what it corrects, the charter's reason for a settled claim was unsourced, and the scar stops summarising with a word bag

**2026-08-15 (session 19). Status: mixed, per clause.** Every git/file
figure in this entry is re-derived live this checkpoint from the current
tree, not carried from session 18's handoff. **(a) TESTED, summarised from the commit's own message rather than
re-derived, per that message's own instruction that it already carries the
receipts.** The scar now quotes the top-*ranked* absorbed bit in that
speaker's own words (`frame.quoted`, `tui/render.go:124`) instead of
`topWords(c.Bag(), …)`; the footer marks a dropped key with `abridged`
replacing `fit`. Closes the human-facing half of D59(j) and the first
finding of D58(o). Review (`decision-guard`) returned seven corrections,
all applied before landing — see the commit message for the list.

**(b) TESTED: a vote leak found by building the obvious version and
looking at it.** The naive reading sends the scar's quotation to the
persona as its memory of the fold. But the quotation is *selected by the
human's vote* (highest `own`, then `others`, per `frame.quoted`), which
wires D39(a)'s sycophancy pump straight into the fold note — and worse
than D39(a)'s stated case, it reaches bits whose hold has already lapsed,
where no consequence remains for a model to legitimately experience.
Proven red by execution, not argued: `TestNoVoteReachesThePersona`
(`tui/tui_test.go:3865`) gains an arm that goes red on exactly that
mutation. `scarWords` is deleted; the "the human is never shown a summary
the persona did not get" invariant it and `personaWords` (`tui/ask.go:60`)
enforced is retired in `personaWords`'s own doc (`tui/ask.go:37-38`)
rather than quietly falsified. The persona stays on the word index
(`topWords(c.Bag(), personaWords)`, `tui/ask.go:692`).

**(c) RULING: the first time the transcript's content depends on a vote,
and D39(b) is not crossed but is one door nearer.** D39(b)'s bound is
about *where* a bit is drawn from; `frame.quoted` decides *what* is
drawn, which is a different axis — so the bound holds as written. Recorded
so a later session does not read D39(b) as untouched: the door exists now,
and it is held shut by clause (b)'s test, not by the bound itself.

**(d) DECISION, filed open rather than taken.** A quotation chosen by a
*vote-free* rule (e.g. always the newest absorbed bit) would clear the
D39(a) objection completely and still get the bag out of the persona's
fold memory — but `foldNote`'s own sweep already has llama3.2:1b reciting
the twelve-word index *as content*, 6/6, so "the persona is already fine
without a quotation" is not established either. No measurement exists for
the vote-free-quotation option in either direction. **Explicitly not
considered and rejected** — deciding it needs a live-ollama sweep, filed
in `docs/DEBT.md` rather than guessed.

**(e) A seventh way this project's mutation harness lies, craft record
(`.claude/craft/tui-design-engineer.md:2756-2761`).** A mutant that leaves
a variable declared-and-unused makes `go test` report "does not compile"
in the same textual shape a genuinely caught mutation reports failure —
*not run* reads as *caught* unless a reader distinguishes the two failure
messages by hand. Same family as D27 and D48: an instrument whose failure
mode looks identical to its success mode is not an instrument yet.

**(g) TESTED, and it falsifies D58(p) as a sufficient answer on its own:
D59(q)'s open question is demonstrated rather than reasoned about.** The
correction in clause (f) was filed as a real bit for a real target rather
than staged: `tldr say -as ceo/session-19 -name session-19 "<the
correction>"` → `f6d652540524e24718f24adc8f22c85736499c4ffde56eb0d32c6d57fb9527c5`.
`go run ./cmd/tldr top -n 5` immediately after, re-run live this
checkpoint and matching:

    +1  d9ae9a94  … the unsourced claim (session-15)
    +1  4faea3e4  … qwen, unrelated
    +1  afa7518e  … qwen, unrelated
     0  f6d65254  … the correction (session-19)
     0  a0ab4364  … qwen, unrelated

**The correction ranks fourth, three rows below the thing it corrects**,
under two bits with nothing to do with the question, and nothing on the
screen relates the two. A reader opening `top` meets the unsourced claim
first and its correction fourth, with no marker that the fourth is about
the first. **The reason is structural, not incidental:** a separate
votable bit (D58(p)'s answer) ranks by its own votes, and a correction has
none *by construction* — it is the newest thing on the record, while the
claim it corrects has had every day since it was written to accumulate
standing. The correction is not merely unranked; it is **systematically
disadvantaged relative to what it corrects**. That is the outside critique
D59(q) raised (a wrong result reused with more confidence because memory
gives it "the appearance of established precedent") reproduced inside this
project's own record, on a claim that had already propagated into
`CLAUDE.md`. Two escapes closed cheaply by reading code rather than
reasoning from symmetry, recorded so nobody re-proposes them: a downvote
cannot accelerate a bit's departure from the view (`Fold` never cools a
run of one, and `memory/view.go`'s own doc says a vote outside `stay.By`'s
tier "cannot hold a bit back and cannot push one out" — there is nothing
for a negative hold to accelerate); and the persona is vote-blind by
design (`tui/ask.go:95-102`, `:478-490`, D39(a)), so half the critique's
pump structurally cannot run on the persona path — it runs only on the
human's own ranked reading, which is the one place this project does
surface precedent by vote.

**(h) An outside paper names the same gap, abstract only.** arXiv
2605.26252, "Is Agent Memory a Database? Rethinking Data Foundations for
Long-Term AI Agent Memory" (Orogat & Mansour). **Abstract read; full text
NOT read** — the PDF fetch returned undecompressed structure, so this
clause is abstract-only and must be carried that way. It names "missing
semantic revision" as one of four recurring failure modes of systems that
"localize correctness at records, embeddings, or edges," alongside
unregulated growth, capacity-driven forgetting, and read-only retrieval,
and claims "no record-level system can satisfy these conditions,
regardless of the storage model." **Against this project, and the
sharper of two readings:** clause (g) reproduced exactly that failure mode
on this project's own record, the same day, independently, before the
abstract was read — a gap found twice from two directions is not a
curiosity. The impossibility claim is **unread**: the three structural
observations that carry it are the load-bearing part of the paper and
nobody here has looked at them. That read is a `decision-guard` D58(k)
research rotation, not an inline CEO task. **For this project:** the
abstract names no ranking by human judgment or vote anywhere. Also surfaced, not fetched, recorded as a lead only:
arXiv 2607.05844, "StateFuse: Deterministic Conflict-Preserving Memory for
Multi-Agent Systems."

**(i) DECISION: a candidate mechanism for clause (g)'s finding, recorded
as a candidate and not a plan, with two of its three named risks now
checked and one still open — corrected mid-checkpoint from an earlier
draft that called all three unchecked.** `f6d65254` already names
`d9ae9a94` in its own text. Under D6/D26 an address is content, so a bit
that names another bit's address is already a link with no schema
change — D2's "composition from primitives" doing exactly what it claims;
the tui already reads `Utterance.Text` to draw it, and recognising an
address inside that text at draw time is the same shape as `frame.quoted`
deriving a label at draw time rather than storing one, so it writes
nothing new to the record either. Three things could have killed it:

  - **Does a printed 8-char prefix resolve unambiguously in a `Store`?**
    Checked. `Store` has no prefix lookup — `Get` is exact, and the API
    is `Put`/`Get`/`Len`/`All` (`memory/store.go:88,121,131,158`) —
    so a prefix resolves only by scanning `All()`, O(n), needing no new
    `memory/` API. The property that actually matters is not that a
    prefix is unique today but that **ambiguity is detectable**: a scan
    can count matches and refuse to link unless exactly one hits, failing
    loudly rather than mis-linking silently. Measured on the live record
    for what it is worth, not as a proof: 29 distinct 8-char prefixes
    among 29 said bits, zero collisions (`go run ./cmd/tldr top -n 0`,
    prefixes extracted, `sort | uniq -d` empty). Favourable, and the
    refusal-on-ambiguity property is what makes it safe, not the
    arithmetic — 8 hex chars is a ~2^16-bit collision risk, far from a
    35-bit record.
  - **Is reading text for addresses a layering violation the record
    should refuse?** Checked, as a CEO judgment rather than a tested
    fact: no. D6/D26 already hold that the address is content; a view
    recognising an address inside text it already reads writes nothing
    and is the same shape as `frame.quoted`'s own draw-time derivation.
    Favourable.
  - **Is relating the two bits on screen even the right answer, versus
    letting the human's two votes do it?** Unchecked, and still the one
    live risk — a design question, not a technical one, and it is
    genuinely open.

**(j) Open, not built: the ordering rule exists in two places and nothing
pins them together.** "Own before others, never summed" — the highest
`Own` wins outright, `Others` only breaks a tie inside that tier — lives
in `memory.View.Rank` (`memory/rank.go:132`, the `cmp.Compare(b.Own,
a.Own)` tier check) and separately in `frame.quoted`
(`tui/render.go:129-151`, the `own > bestOwn || (own == bestOwn && others
> bestOthers)` comparison). Confirmed by line number this checkpoint: the
two implementations agree on the rule but are two hand-written copies of
it, and the one place they intentionally differ — the tie-break, view
order (oldest-first) in `Rank` versus walking `Absorbed()` newest-first in
`quoted` — is documented in `tui/render.go:110-119`'s own comment, not
merely implicit. Nothing tests that the two stay in agreement if one
changes; that is the gap, not the disagreement on tie-break, which is
intentional and explained.

**(k) Three findings from running the real binary, `docs/DEBT.md:850-877`
(tmux 3.4, scratch `TLDR_RECORD`, 71 bits, one standing vote) — summarised
here, read there for the full text.** A row reading `agent-7  ╌` is
byte-identical to a row for an empty fragment, because `said` draws an
empty fragment as the mark alone and a `╌`-only utterance as its own text,
and the transcript has no columns to spare to distinguish them. A
vote-promoted quotation is not on the first page of its own receipt — a
57-bit fold quoted row 27 of 57, three pages down from where `ctrl+u`
opens, with nothing on screen pointing at it; D14's reachability holds
(the row is there) but the earlier claim that it was checkable near the
top of the receipt was itself an artifact of testing only on a
twenty-four-bit fold that fit on one page. And `tldr say` is a **third
route into a gauge past its own limit, and the common one** — the CLI
writes bits to the record and never runs the fold trigger, so a record
seeded outside the surface opens over budget on the person's *second*
launch (measured: `view 44 · record 69`, gauge `43/23`) rather than the
first keystroke, unlike the two previously-known routes (a run of holds,
a resize), both of which need the person to do something first. This
sharpens D59(l), which named the symptom (arrives over budget) without
this mechanism (the write path that skips the trigger entirely).

**Receipts.** `git rev-list --count HEAD` = 84. `git status --short` empty
before this checkpoint's own doc edits. Public tree unchanged this
session: 3 commits at `e4d67960c976df8292b05cb55fa55bbabefb035c`, local
and remote agreeing. `grep -c '^## D' docs/DECISIONS.md` = 59 before this
entry. 10,442 lines of product Go (`find . -name '*.go' -not -name
'*_test.go' -not -path './cmd/seam/*'`, up from 10,055), 2,182 lines of
`cmd/seam` product code (unchanged), 16,310 lines of tests outside
`cmd/seam` (up from 15,630), 1,224 lines of `cmd/seam`'s own tests
(unchanged). 351 `Test` functions (up from 340). 46 declared claims
(unchanged, `grep -c "^id:" docs/CLAIMS.md`). `sh .githooks/pre-commit`:
exit 0, five packages, all `ok` (cached), re-run live this checkpoint. The
live record moved this checkpoint, by design — a real correction was
filed onto it: sha256 before `4372b025dd7a07eec9fd1b2cfe391e8f08ef104d3fcbcfed17d6a52bc79dd181`,
after `061a588cdaf82002bf1ef553a9c7e7d807fec278f5e7cd94abd1450947285b8a`. `go
run ./cmd/tldr top`: "29 said of 35 bits on the record · 3 ballots, 3
standing" (up from 28/34).

---

## D61 — Prev stops meaning a question, and stranding stops being asserted

**2026-08-15 (session 20, checkpoint before any commit — the working tree
carries this checkpoint's changes uncommitted at the time this entry is
written; committing is the CEO's hires' job, not this seat's, per
`CLAUDE.md`'s "Working on the code"). Status: mixed, per clause.** Every
figure below is re-derived this checkpoint against the tree on disk,
including the working changes, unless marked otherwise. Clauses (c) and
(e) went through a real correction cycle mid-checkpoint — this seat's
first read of the stranding figures made a category error, caught and
fixed before this entry left draft form; clause (e)'s D59(c) question
stays genuinely open. Clause (p), added last, names a pattern in how this
checkpoint's own claims were asserted, independent of whether any one of
them turned out true.

**(a) RULING: `Prev` is positional, not semantic.** Both write paths set
`Prev: shown.Head()` — `tui/tui.go:1186` and `cmd/tldr/say.go:138`
(re-derived this checkpoint by line number; the dispatch brief that
opened this checkpoint cited `tui/tui.go:1152`, which was correct when it
was written and has since drifted forward 34 lines under this
checkpoint's own edits to the same file — a small instance of the
project's own recurring lesson that a cited line number is a claim with
an expiry date). This records the head of the view at the moment a bit
was written. In an alternating session at one keyboard that coincides
with "the turn this replies to"; for anything written from outside the
surface it does not. Sourced on the live record: **7 of 29 said bits
(24%) came through `tldr say`**, and `f6d65254`'s `Prev` is `a0ab4364` —
a greeting from `qwen3.5`, not `d9ae9a94`, the claim `f6d65254` actually
corrects in its own text. Not *unrelated* — an earlier draft of this
finding said so and overstated it, corrected in five files this
checkpoint (`.claude/craft/tui-design-engineer.md` and four `tui.go`/
`memory/view.go` doc sites): `a0ab4364` names the same
material in its own text, so the two bits share a subject; what the edge
fails to be is a reply to the thing the correction corrects. Consequence:
the surface may not assert a semantic relation on a positional edge.
`sparing`'s behaviour is deliberately unchanged by this — the fix is to
the sentence, not the fold — and it is confirmed unchanged: `git diff
--stat -- memory/view.go` touches only comments and one doc block, no
`memory/sparing_test.go` assertion on `Fold`'s output changed shape (both
renamed tests keep their original assertions, only their names changed —
see below). Nine hunks (`git diff -- memory/view.go | grep -c '^@@'`)
touch this language across `tui/tui.go`, `memory/view.go` and one
persona-adjacent comment, now saying "one step back along `Prev`" where
they said "the question its own bit answers." Two test names corrected to
match: `TestAFoldKeepsTheBitAHeldAnswerAnswers` →
`TestAFoldKeepsTheBitAHeldBitNamesThroughPrev`, and
`TestAHoldWhoseQuestionHasGoneSparesOnlyItself` →
`TestAHoldSparesOnlyItselfWhenWhatItNamesHasGone` (both confirmed by
`git diff -- memory/sparing_test.go memory/vote_test.go`).

**(b) TESTED: the stranding measurement is built.** `tui/strand_test.go`
(new, untracked at dispatch) plus `tui/testdata/stranding.txt` (new),
163 lines / 138 schedule rows, frozen by `go test ./tui/ -run
TestTheStrandingSweep -update` and re-derived without `-update` in the
ordinary suite — **not gated behind `HARNESS`**, unlike everything in
`tui/harness_test.go`; `strand_test.go`'s own package doc states why: "a
row of this table is a count that either reproduces or does not, and
nobody has to have an opinion about it," where a frame dump is taste and
belongs behind a flag. `sh .githooks/pre-commit` runs `go test -race
./...`, which covers this file with no separate wiring needed. `simulate`
is now parameterized on bits, rate, back (how many said rows above the
newest a vote lands on), budget, keep, cadence and hold — the file's own
doc names this as closing two of the five axes D36 wanted swept and
D58's own debt entry named as open.

**(c) TESTED, and a category error in this seat's own first check of it is
corrected here before being carried forward — caught by the CEO reading
this entry mid-draft, not found by this seat.** `tui/testdata/stranding.txt`'s
`held%` column, read directly (every distinct percentage in the file,
sorted): `0.0, 17.5, 66.2, 78.8, 83.0, 88.0, 90.0, 91.2, 92.5, 94.0, 95.0`.
This seat's first pass treated the checkpoint's five reported figures
(`93.5%, 92.8%, 91.2%, 22.5%, 0%`) as claims about rows in that table and
flagged three as missing. **That comparison was invalid on its own terms
for the "before" figures**: `93.5%` and `92.8%` describe the tree with
`sparing`'s `Prev`-loop deliberately deleted — a mutation, not a schedule
— run to confirm the frozen table's `0.0%` rows are not vacuous; a mutant
tree's output was never going to appear as a row in a table the *current*
tree produces, and expecting it to was the error, not a discrepancy in
the work. **What is independently confirmed directly from the current,
unmutated table:** every `hold=2m0s budget=23 keep=cut rate=* back=0` row
(21 rows checked by `grep -E "^\s+2m0s\s+23\s+cut"`) reads `0.0%`, and
`91.2%` is a real row (`budget=23 keep=3 rate=1/5 back=12`). Real
stranding, once a schedule reaches the case the pre-`sparing` harness
could not (`back > 0`), ranges **17.5% to 95.0%** across the 138
schedules — confirming the old "0.0% at every vote rate" reading was an
artifact of an unreachable schedule, not a general property of the fix
(`strand_test.go`'s own package doc: "nothing in this tree could produce
either number... A 0.0% taken that way is entailed by its own
schedule"). **`22.5%`'s status is the one figure in the original five
this entry does not have a specific account for**: it was reported by
this checkpoint's seats as a before-mutation figure alongside 93.5/92.8,
which — if true — puts it outside the frozen table by the same logic as
the other two, but that is inferred from the pattern rather than
individually confirmed, and is recorded as inferred rather than checked.
`docs/DEBT.md`'s three older figures (91%/94%/92%, from a harness this
file's own doc says "was thrown away") remain marked **unreconciled**,
not replaced, in `docs/DEBT.md` — that marking is unaffected by this
correction, since those three answer a wall-clock vote-every-N-rounds
schedule the frozen table's grid does not sweep at all, mutation or no.

**(d) RULING: the root cause of the stale figures in (c) and of D59(c)'s
own history is durability, not carelessness.** Figures and doc sentences
across `memory/` and `tui/` went stale over two sessions because
`tui.foldable` and the fold budget's shape changed in `tui/`, and nothing
pins a sentence in one package to the function it describes in another.
"Measure more carefully" would not have prevented this — the same
carefulness that produced the original figures did not, and could not,
know that a sibling package would change shape later. `cmd/tldr/top.go`'s
and `tui/ranked.go`'s own package docs now say this about each other in
so many words (`top.go`: "Nothing binds this paragraph to the function it
describes, and it was false for a checkpoint because of that"), and it is
the same failure this clause names for the stranding figures: a number is
durable only as long as nothing it depends on can move without it
noticing, and nothing here enforces that yet. An earlier framing this
checkpoint drafted called this carelessness on the seats that quoted the
figures; that framing is wrong and withdrawn here rather than repeated —
the seats measured correctly against a schedule that later stopped being
the schedule in question.

**(e) D59(c) is left unresolved, not corrected — two candidate
explanations named, neither adopted, and this checkpoint's own conduct in
reaching that point is the more important finding, held for clause (p)
below.** D59(c) (line 5827) found that D58(i)'s "16 folds before, zero
after" reproduces exactly at `bits = 200` (not the harness's committed
`bits = 400` default, which gives 32) — a real, re-derived finding, done
in two detached worktrees, on a **before-mutation** tree unreachable from
the current one. This checkpoint's dispatch material offered a competing
explanation — "16 folds is exact at budget 23; 32 is exact at budget 12"
— and asserted it to the shareholder as a correction that "convicted"
D59(c)'s account. **Neither explanation is adjudicable from
`tui/testdata/stranding.txt` as it stands**, for a structural reason
rather than a missing row: the frozen table sweeps only `bits = 400`
(D58(i)'s figure was taken at `bits = 200`), so it cannot test the
bit-count axis at all, and it was never going to be able to. What the
table *does* settle, checked directly this checkpoint: at
`hold=30m0s rate=1/3 back=0` — closest to D58(i)'s original schedule —
folds is **0 at budget 12, 23 and 73 alike**, confirming the "after" half
of D58(i)'s figure without deciding between the two "before" accounts.
Both explanations for the "before" half are named and stand as
candidates; neither is adopted. Closing this needs the exact parameter
set (bits, rate, cadence, hold) both D58(i) and D59(c) used, run against
a table built with `-update` and swept over `bits` as well as `budget`,
which does not exist yet.

**(f) Corrects D58(o): the parent-stub finding was false when it was
written, by the CEO's own hand, not by the research seat's.** Session-17
transcript, timestamps re-derived this checkpoint by `grep`: the async research
agent's completion notification ("Research web UX prior art for the TUI"
finished, quoting GOV.UK's own component guidance against a details
toggle) queues at **`2026-08-15T04:33:04Z`** (`"type":"queue-operation",
"operation":"enqueue"` at that timestamp, matching a `task-notification`
for that agent). So the
research finding landed before the fold fix, the fold fix landed before
D58 was committed, and D58's own text carries both the fix and a
justification for the parent stub that the fix had already undercut by
the time the entry was written down — a same-session, self-inflicted
staleness rather than a research seat's error. The research seat's own
brief (`.claude/craft/tui-design-engineer.md`, the entry beginning
"ADOPT — the renderer's layout step...") is scoped correctly and flags
its own weak citations; nothing in it is corrected here. **Consequence,
already acted on this checkpoint (D61(a)):** the parent stub is withdrawn
on the `Prev`-positional evidence, not merely demoted for outdated
justification — a second, independent reason it should not be built as
described.

**(g) DECISION: D60(i)'s candidate (a bit's own text naming another
bit's address, read at draw time) is cut. Two seats independently
reached this, from two directions, and the counter-case was made
properly rather than asserted.** `tui-design-engineer`'s craft record
(`.claude/craft/tui-design-engineer.md`, "The vote fixes D60(i)'s
ordering in two keystrokes, and I have the frame") demonstrates that
casting the two votes the product already asks for — `ctrl+r` on the
claim, `ctrl+o` on the correction — moves the correction to rank 1 on
both `ctrl+t` and `tldr top`, with the correcting bit's own prose already
containing the address string of the bit it corrects, followable by eye
on one 100-column frame with no new vocabulary. **Corrects D60(g):**
"a correction has no votes by construction" is true of *every* bit at
the instant it is written, and D60(g) measured the empty case (before
anyone had voted) and read the disadvantage as structural rather than as
an input this product's own act — a vote — resolves. The craft record's
own generalisation: "before treating an ordering complaint as structural,
cast the votes the product asks for and re-read the frame." Separately,
the same craft record measured the ranked surface *before* this
checkpoint's widening and found it strictly worse than `tldr top` for
this exact question — the unwidened `ctrl+t` showed the false claim at
rank 1 and omitted the correction from the screen entirely, because
`judged()` only listed voted bits. **Two affordances, not one, and they
should not be conflated going forward:** `Prev` is program-authored,
positional, cardinality ≤1 (`View.Head()` returns nil or one bit); an
address named in a bit's own text is speaker-authored, semantic, and
1:N — a claim cited by three corrections cannot be drawn as one quoted
row, which is the shape D60(i) proposed and is the reason it is cut
rather than merely deferred.

**(h) TESTED: `judged()` widened, and it is a real change to what the
interactive surface shows, not only to `tldr top`.** `tui/ranked.go`'s
`judged()` now walks `m.store.All()` for every `memory.Utterance`, not
only `m.votes.Bits`. On the untouched live record, `ctrl+t` went from
`ranked 3 · record 35` (three rows, twenty blank) to `ranked 29 ·
record 35` (`kept · 3`, `not judged · 26`), confirmed against `go run
./cmd/tldr top`'s own header this checkpoint (`29 said of 35 bits · 3
ballots, 3 standing`) — the two now agree to within the votes cast on
scars, which is the reconciliation the widening was for. Same exclusions
as `cmd/tldr/top.go`'s `reading()` (a ballot is a mark on a row, not a
row; a `memory.Compaction` is what a view did, not what anyone said),
with one deliberate difference: a scar somebody voted on stays in the
list here, because this screen has a caret that can be parked on a fold
and `top` has none. **21 of 29 said bits had existed on no interactive
screen at all before this** (`tui-design-engineer`'s figure, from walking
the caret down the widened list and counting rows behind the record's
one scar; not independently re-counted by this seat, and flagged as such
— re-check by comparing `ctrl+t`'s row count against `tldr top -n 0`'s
`not judged` line on an untouched record, which this seat did confirm
agrees).

**(i) TESTED: the tie's promise was measured over rows and the screen
draws lines, and it now carries the caret's whole block rather than its
first line.** `voteCell`'s stroke used to hang half a character into the
row below regardless of how many lines the caret's own row wraps to;
measured on a fixture of model-length replies, every covered row under
that regime put five lines of text between the stroke and the mark it
points at (150/150 at one upvote in ten). The package's own fixtures
never caught it because they are one-line. Fixed by substituting the
stroke into the continuation lead `transcript` already computes for a
fading block, rather than adding new geometry — held by
`TestATieReachesTheMarkItPointsAt`. **This is a correction to the 26,467-row
measurement D42/D58 cite for `voteCell`, not a reversal of it**: that
count was real and was counted in rows; the defect is that the screen
draws lines, and the two words stopped meaning the same thing once the
caret's row started being drawn whole (D53's own change).

**(j) TESTED, correcting D60(h)'s "abstract only" caveat in part, and
flagged for a source-fidelity reason of its own.** arXiv 2605.26252 was
read past the abstract this checkpoint — the dispatch brief that opened
this checkpoint reports it says "These are structural claims, not
theorems," not a proof, and quotes an "Observation 3a": *"Append-only
storage without semantic units cannot satisfy C2. Two appended values for
the same fact coexist with equal status, and a default query has no
engine-level mechanism to select between them."* **Independently
re-derived by the CEO against the source, and the method is the finding.**
`scope-adversary` read the paper first; the CEO then fetched
`https://arxiv.org/html/2605.26252v1` — the **HTML** rendering, not the
PDF — and got all three items back verbatim in one call: the abstract's
"no record-level system can satisfy these conditions, regardless of the
storage model," the §3.4 disclaimer "These are structural claims, not
theorems," and Observation 3a exactly as quoted above. Two seats, two
tools, one source, agreeing.

`archivist` then attempted a third confirmation and failed, which is
worth recording because the failure is about the *tool*, not the paper:
`poppler-utils` (`pdftotext`/`pdftoppm`) is not installed on this machine
and no Python PDF library is available, so it fetched the **PDF** and
`WebFetch`'s summarizer, asked twice, reported it could not locate either
phrase in the extracted text. The lesson generalises past this entry:
**arXiv serves an HTML rendering at `/html/<id>v1`, and a summarizing
fetch finds a passage there that it misses in the same paper's PDF.**
Reach for the HTML first. A seat that concludes "unconfirmed" from a PDF
fetch has measured its extractor, not the source. **The disagreement content, taken as reported
and not contingent on the exact quotation:** this project's own C6
(retrieval-induced adaptation, an access-reinforcement ratchet) is
refused rather than failed — it is D39(a)'s sycophancy pump restated as a
correctness condition — and that is recorded as a disagreement about
desiderata, not a gap this project owes a fix for.

**(l) Founder correction, mid-session, outranking a unit under it: the
vote was never meant to be human-only.** Tyler's own words, relayed in
this checkpoint's brief: "an attested, trustless mix" of humans and
agents. **Checked against the code rather than accepted on report:**
`memory.Cast` (`memory/vote.go`) takes a `Handle` with no restriction to
a human one; `View.Rank`'s `by Handle` parameter is confirmed (D56(a),
re-cited here) to be the *voter*, not the ranked party, and is two-tier
by construction; `memory/rank.go:97-99`'s own comment names "the
agent-only forum" as a case the ranking already handles. D24's Moltbook
read is the standing evidence that an agent-only vote degenerates
without a human backstop — consistent with, not contradicted by, letting
agents vote alongside a human. So the code already agrees with Tyler's
correction and the prose (this project's own repeated shorthand,
"votes are the consolidation signal" read as "the human's vote") had
drifted from it. **What Tyler's sentence actually exposes as a real
open gap is the word *attested*:** content-addressing gives
tamper-evidence — a bit cannot be silently altered — but nothing in
`memory/` signs anything, so a `Handle` is a claimed name, not an
authenticated one; anyone who can write to the record can cast a vote as
any handle they type. Logged as a named open gap, not a decision to
close it. The rest of Tyler's message — merge/clone/fork/rebase as
vocabulary for this record, graphs of graphs — is called vocabulary
rather than substance under D5, unchanged, with one exception worth
keeping: **a `memory.View` is already a fork.** It is a derived,
immutable reading over a record that cannot itself be written to, so two
views taken from the same store are two branches that can never
conflict with each other — the conflict-freedom a git fork buys by
convention, this project's `View` has by construction.

**(m) Process, recorded because a near-loss is worth a sentence even
when it was recovered.** An agent dispatched this checkpoint died
mid-rename on an API error, having completed the two Go-side renames in
clause (a) but leaving five citations to the old test names in
`docs/CLAIMS.md` and `docs/CODE.md`. Finished by hand; `decision-guard`
verified this checkpoint that all cited test names across
`docs/CLAIMS.md` — 90 `red:`-adjacent citations by its own count —
resolve to a real, currently-named test. **Precedent, recorded rather
than passed over:** `tui-design-engineer`'s craft record was edited in
place this checkpoint rather than appended to — a re-check command it
had written minutes earlier ("`grep -ni 'voted on'` returns nothing")
was found to be a check that could not fail (it passed vacuously on the
literal absence of that exact phrase, while three later, correct uses of
"voted on" would have made it fail honestly), and the seat swapped the
broken re-check for a working one in the same paragraph rather than
appending a correction below it. Craft records are meant to be
append-only, matching every other log in this project; this is accepted
as the right call in this instance — the swap is small, local, and the
broken version would have actively misled the next reader who ran it —
but it is a precedent for editing what `CLAUDE.md` calls an append-only
surface, and it goes in this entry rather than passing silently, so a
later session does not treat it as settled that craft records may be
edited freely.

**(n) Diagnosis accepted as a limit of the code, correcting how this
seat first framed it, not built.** `cmd/tldr/top.go`'s `reading()` and
`tui/ranked.go`'s `Model.judged()` are one walk written twice in two
packages, differing by the one clause named in clause (h) (scars),
reconciled only in a doc comment on each side pointing at the other —
which is why that doc comment went false for a checkpoint (clause (d)).
This was first framed, in an earlier draft of this checkpoint's material,
as a limit of `cmd/seam` — that the claims checker cannot see a semantic
duplication, only a textual one. **The correct reading is a limit of the
code**, not of the checker: `cmd/seam` did exactly its job (clause (m)
above, and the earlier "stale-citation" catch on the renamed
`rank-the-view-instead-of-the-record` claim, per `tui-design-engineer`'s
craft record). The fix that makes the doc-comment promise true by
construction is one function both `cmd/tldr` and `tui` import, which
`reading`/`judged`'s current package boundary (`judged` unexported,
`reading` in `package main`) does not allow today. Named as the unit,
ranked behind the vote surface per this project's standing rule, not
built this checkpoint.

**(p) RULING, on this checkpoint's own conduct rather than on the
product, flagged by the CEO mid-draft and recorded here at the CEO's own
explicit instruction rather than left in a debt list: this checkpoint
stated a correction more confidently than its evidence supported, three
separate times before this entry was finished.** First, an early draft of
clause (a) called `f6d65254`'s `Prev` edge "unrelated" to the claim it
corrects, when the two bits share a subject and the honest claim is
narrower — that the edge fails to be a *reply*, not that the two are
unconnected. Second, an early draft of clause (d) called the stale
stranding figures a carelessness finding against the seats that measured
them, when the seats measured correctly against a schedule that later
stopped applying — a durability failure, not a diligence one. Third, and
the one this seat did not catch on its own: this checkpoint's dispatch
material told the shareholder flatly that D59(c) "convicted a true
figure," using an explanation (a `budget` axis) that clause (e) now
records as unconfirmed and structurally untestable against the very
table cited for it. **The pattern is not that any one figure was wrong —
it is that "assert slow," this seat's own standing rule, was violated at
increasing cost each time**: vocabulary, then attribution of a defect's
cause, then a specific factual claim relayed toward the shareholder as
settled. Two of the three were caught and corrected before this entry
left draft form; the third was caught only because the entry was read by
someone other than the seat that wrote it, which is this project's own
standing finding about where catches come from, applied to itself inside
a single checkpoint rather than across two sessions.

**Receipts.** `git rev-list --count HEAD` = 85 (this checkpoint has not
committed; 85 is `HEAD` at dispatch, unchanged by this entry's
own drafting). No commits this
checkpoint yet; everything in this entry describes the working tree.
`git status --short` at dispatch: fifteen tracked files modified plus two
new paths (`tui/strand_test.go`, `tui/testdata/`), all from
`principal-go-engineer`'s and `tui-design-engineer`'s work this
checkpoint, none from this seat, which touches only `docs/DECISIONS.md`,
`docs/DEBT.md`, `CLAUDE.md` and this handoff.
`sh .githooks/pre-commit`: exit 0, five packages, all `ok`, uncached
(`go clean -testcache` run immediately before), 51.5s wall — seam
2.844s, tldr 10.668s, memory 11.380s, persona 1.202s, tui 46.728s
(figures as reported by the dispatch; not independently re-timed by this
seat). `go run ./cmd/seam`'s full-catalog re-run, started at the top of
this checkpoint (47 claims declared, up from 46 —
`noscar-never-fires`, the catalog's first claim naming a `_test.go`
`file:`, ruled a naming accident rather than a new premise since
`strand` is instrument code Go's build rules put beside the tests), was
still running when this entry was drafted; **its result is not stated
here and must not be read from an earlier session's figure** — see the
handoff for whatever it returned. 60 decisions in `docs/DECISIONS.md`
before this entry (`grep -c '^## D'`), 61 after. `grep -c '^id:'
docs/CLAIMS.md` = 47. Public tree: 3 commits, `e4d6796`, matching its
remote, untouched. Live record `~/.local/state/tldreddit/record`:
unchanged this checkpoint — `go run ./cmd/tldr top` reads "29 said of 35
bits on the record · 3 ballots, 3 standing," matching D60(g)'s figure
exactly, confirming nothing was written to it during this checkpoint's
own work.

---

## D63 — The forum container is deferred with a trigger, agent votes stay a tier not a ban, and the CEO's own first ruling on both was wrong

**(a) ASSERTED: the forum container is deferred, with a named trigger, not
refused.** D18(c) ruled forum is the base abstraction; there is no `Forum`
type in the code and never has been (`grep -rn "type Forum" --include='*.go' .`
returns nothing). The founder's framing this session: "we need to give an
agent its own internal tldreddit" — a record holding many forums, a seat
owning one, a rule for how a child's top-ranked bits reach the parent's view.

Not built now, on sequencing grounds: it is a wire-format break. A forum
roster sits outside every seal, forcing wire `version` 1→2
(`memory/wire.go:93`, `version = 1`, checked and refused on read at
`memory/wire.go:629-630`). The receipt for what an unversioned addition
already does, reproduced this session and recorded in `docs/DEBT.md`: `decode`
accepts a four-view-stream file with no error and drops the extra silently —
self-delimiting streams have no framing question to ask about slack
(`memory/wire.go:42-44`'s own point). The failure mode a forum roster would
walk into is silent truncation, not a parse error.

**Trigger, because a refusal with no trigger is the shape that rots:** D4's
own collapse condition — evidence that at real agent volume a human cannot
vote meaningfully.

**(b) ASSERTED, and a retraction — the CEO's error, not a subagent's.** The
first draft of this ruling disqualified the container because "there is no
voter in a child forum." That restated a *prohibition* as a fact about
capability. Tyler corrected it directly: two subagent definitions with
opposed mandates can be created and run trivially, and they will argue, or
agree, or neither. He is right and the reversal is on the argument, not on
tone.

The CEO also misread its own citation. `memory/rank.go:97-99` reads: "A zero
`Handle` for `by` is legal and means the first tier is empty, so the whole
ordering is the second one. That is the agent-only forum, and D24 is what it
produces." That describes the case where there is **no human at all**. It
does not say agent votes are degenerate. Two claims collapsed into the one
that supported the answer already reached.

**The prohibition was never ruled at the scope it was applied.**
`decision-guard` read D4, D30, D39(a), D51(d) and D52(j) in full: D52(j)'s
line is "the skill gets Claude a write, never a vote," scoped in its own text
to *this* record, with D51(d) and D39(a) both about the human's published
record. None reaches a child forum with no upward path.

**(c) ASSERTED: agent votes are a tier, not a ban — and a seat vote surface
is still not built.** `Rank` is two tiers that never mix (`memory/rank.go:44-51`
in doc, code at `memory/rank.go:105-`): "a ceiling stops an agent voting a
million times, and a tier makes the millionth vote worth nothing." Three
findings, verified against the tree, decide against building a seat vote
surface now:

- **`Others` is a Sybil count, not a vote count.** `standing()`
  (`memory/vote.go:208`) keys on `{voter, target}` and keeps one vote per
  pair, so nobody votes twice on a bit — but a handle is free. Within the
  `Own == 0` band, which at 3 standing votes in 35 bits is most of the
  record, agent identities fully order what the human sees.
- **Write volume is already a ranking input, measured this session.** Five
  seat notes, one per seat, push two human bits off `tldr top`'s default
  page. Nothing forged a vote: at near-zero vote density a ranked reading
  just *is* recency, which is D24's r=0.050 from the other direction.
- **Tier two already reaches a surface that is not ranking.** `frame.quoted`
  (`tui/render.go:134`) picks which absorbed bit a scar quotes on screen by
  `own` then `others`, reading `frame.votes` — which is "every standing vote
  on every bit, from [memory.Tally]" (`tui/render.go:72`), not `Stay.Holds`.
  Correctly tiered, but it means tier two already decides what a fold's
  *receipt says* to the human wherever the human left bits level. This
  sentence was itself wrong when first drafted here, naming `memory.standing`
  as the source and `Tally` as what it was not — a confusion with
  `frame.standing` (`tui/render.go:95`), a frame method of the same name.
  Caught by re-derivation before the entry was committed.

**The CEO's ruling:** the real failure is not a forged signal, it is that
agent votes would decide **which bits the human ever gets the chance to vote
on** — attention allocation, which the charter says is the whole reason the
forum shape is load-bearing. So the open question that must be decided before
any seat vote surface: *may tier two reorder the page the human has not
judged?* Recorded as open, not as refused.

**TESTED, from `principal-go-engineer`: no agent vote can move a fold.**
`standing()` has exactly three callers — `Tally` (`memory/vote.go:184`),
`Stay.Holds` (`memory/view.go:216`, filtered to `stay.By` and `Up`), and
`View.Rank` (`memory/rank.go:107`) — and every fold-side path reaches votes
only through `Holds`; `sparing` (`memory/view.go:681`) takes the
already-filtered map. The channel that *is* open is writing, not voting:
`Prev` is positional, so an agent that speaks immediately before a bit the
human upvotes gets spared — agents move the fold by choosing when to talk,
not by voting on it. And: a CLI vote guard could not be called "structurally
incapable" — `cmd/tldr/say.go:112` already says so of its own guard, "it is
not a security boundary and cannot become one."

**(d) ASSERTED: seats write into the record as an executable step — and the
D24 citation the CEO first offered for it was backwards.** D24's finding is
that Moltbook agents had *commenting* as a checklist step and *upvoting* not,
and the cure was making the upvote a step. A trigger to write is the half
Moltbook already had — the half that produced 97.3% zero-upvote comments.
Cite D24(b) instead, "the default channel decides the topology," the entry
actually on point since seat traffic routes to the single default channel.

**Null hypothesis, so this is not D27's shape:** after meaningful seat
traffic, zero seat-written bits carrying a standing vote from `tui.Human()`
means the premise is dead — and that is direct evidence for D4's own collapse
condition. The instrument exists: `tldr top -n 0`, whose header already
prints `kept · not judged · let go`. No new instrument is warranted.

**(e) ASSERTED: D18(c)'s CEO addition is retired as falsified — and D49(d)
got there first.** "Threads are the first thing in this system that is
actually rankable. A single transcript poses no ranking question" is dead as
of D49. D49(d) already superseded this exact sentence, quoting D30 quoting
D18(c). Recorded here so a reader does not conclude the record moved twice.
One precision: `m.rank()` (`tui/tui.go:1384`, called at `tui/tui.go:706`)
does not rank "a single transcript" — `judged()` walks `store.All()`, so it
ranks the whole record's utterances. The falsification survives; the drafted
sentence overstated the tree. Tyler's own migration-cost argument in D18(c)
stands independently and is what authorized "channels containing threads" —
retiring the CEO's addition does not empty D18(c).

**(f) ASSERTED: D18(c)'s exclusion list stands and is not routed around.**
"Not authorized: multiple communities, membership management, moderation,
cross-posting." A per-agent forum is a multiple community on any reading. It
also does more work than it claims: keeping the record single-channel
protects D58(a), whose "no query" ruling rests on `top -n 0 | grep` reading
everything.

**(g) TESTED: D59(c) is closed, and neither candidate account was right.**
Both candidate accounts print 16 folds — (200 bits, budget 12) and (400, 23)
— because folding runs about linearly in length, so halving the conversation
lands on what a larger budget already gave. What adjudicates is that the
figure never travelled alone: the paragraph that published it carries nine
numbers, and 400/23 matches nine for nine while 200/12 matches two. The real
correction is that D58(i)'s figure never came from `TestHarnessHoldSchedule`
at all — that harness hardcodes budget 12 and gives 32 at its own default,
which is exactly the non-reproduction D59(c) reported: a true observation
about an instrument that was never the source. D61(e)'s stated closing
condition — a table swept over bits as well as budget — proved insufficient
on its own; reading the whole publishing paragraph as one joint fixture is
the better rule, and is worth logging as a rule in its own right.

`tui/testdata/stranding.txt` is now **300 lines / 270 schedule rows**, up
from the 163/138 recorded at D61(b) — append-only, so that earlier figure
stands and this is the correction, not an edit to it. The 138 pre-existing
rows come back byte-identical in every number column. Two findings the
length axis produced that nobody asked for: held% is not invariant in length
(52.0% at 100 bits, 83.0% at 400, 95.8% at 1,600 — every published stranding
percentage is partly a claim about conversation length), and D58(i)'s cliff
is bounded — folds are 0 at 200 and 400 bits at every budget swept, 95 at
800. "The record stops consolidating" means "stops until the votes holding
it do."

**(h) ASSERTED: `record-frame-unclosed` keeps `sole: true`; its trigger is
retired as the wrong instrument rather than fired.** Four trips
(`docs/CLAIMS.md:877-882`, `red:` list now 27 tests, confirmed by the file's
own "the set is now twenty-seven"), each correctly declined — the trigger
watches for *leak* ("trips on work with no bearing on the frame"), and the
list is growing from a class that can never trip it: a `cmd/tldr` test whose
only state is a file always has bearing on the frame. Firing it anyway would
be the counting error the claim's own prose warns against. Replacement
trigger: the day a round-trip test is added and `seam` finds the `red` list
stale before its author does. The readability cost gets a format answer as
debt, not a fix here — let a claim name a class ("these checks, plus every
round-trip test") rather than 27 literals.

**(i) TESTED: corrections to the record.** Each of these is a checkable claim
that was wrong and that nobody re-derived:
- `CLAUDE.md` cited `Model.mark, tui/tui.go:308`; the `mark` field is at
  `tui/tui.go:414`. It cited `m.rank(), tui/tui.go:595-597`; `m.rank()` is
  called at `tui/tui.go:706` and defined at `tui/tui.go:1384`. Both sit
  inside the item D58(a) rests on. D47's class, and now fixed in `CLAUDE.md`.
- `CLAUDE.md` said `go run ./cmd/seam` "takes about 2m30s." Measured this
  session: **17m35s**. That changes how a session plans a checkpoint, and
  is now fixed in `CLAUDE.md`.
- `CLAUDE.md`'s inline bullet said D59(c) "stays genuinely open, per
  D61(e)." It is closed, per (g) above, and `CLAUDE.md` is rewritten to say so.
- `docs/DECISIONS.md` records the frozen table as "163 lines / 138
  schedule rows." It is now 300 lines / 270 rows, per (g) above.
  Append-only — the new figure is recorded there, not by editing this line.
- `cmd/tldr/say.go`'s doc comment (`say.go:44-47`) argues it must write into
  the view because "a bit filed with nothing naming it is reachable by
  nobody… D14 is explicit that reachable means discoverable." True when
  written; false since `Store.All()`, `tldr top` and `judged()` all
  enumerate the store. The behaviour is right, its stated reason is stale.
  Not fixed in code this entry — recorded so the doc comment is not read as
  current.
- The D58(i) cliff figure ("one vote in three and the record stops
  consolidating") is true only while the conversation is shorter than the
  holds, per (g) above. Anywhere it is quoted it must carry the length,
  because a reader cannot re-derive it otherwise.
- D14 and the shipped product hold two different definitions of
  "reachable," and nobody has written the reconciliation down. Not a D1
  violation: `record.absorb`'s doc already argues it correctly and store
  enumeration is stronger than the retrievability D14 rejected. But both
  enumerations admit `Utterance` only, so a `Compaction` or `Vote` in the
  store and in no view would be invisible everywhere. Recorded in
  `docs/DEBT.md` so the next session does not rediscover it.
- Additionally found and fixed while touching the same passage: the
  `docs/DEBT.md` "fold budget" entry cited `frame.quoted` at
  `tui/render.go:124`; it is defined at `tui/render.go:134`.

---

## D66 — Enumeration is a third mode D14 does not count, and the definition nobody had written down was hiding a bug that strands a vote

**2026-08-16**

**Status:** mixed, per clause. (a) is a ruling. (b) is a reversal, tested
against the code and the public tree. (c) is a ruling applied to the tree.
(d) is tested — re-derived directly by the CEO against `HEAD`'s
`cmd/tldr/record.go` in a scratch worktree, reproducing the exact stranding
report below. (e) is tested — the falsifiability claim re-derived directly by
the CEO (neutralizing the tiebreak reddens exactly one named row and nothing
else in the module); the historical 12/20, 14/20 and 0/20 counts are as
reported by `decision-guard` and `principal-go-engineer` and were not
re-run. (f) is tested, re-derived directly against `memory/vote.go` and
`cmd/tldr/record.go`. (g) is asserted, as reported by the two seats; the
final "three priors" figure was independently confirmed by the CEO reading
the code. (i) is tested, re-derived directly against `memory/reach_test.go`
and `memory/wire_test.go`. (j) is asserted, from `docs/DEBT.md`. (k) is a
ruling: open, not decided.

**(a) RULING: D14 binds discoverability and nothing else.** Three modes, and
the record needs all three words:
- **retrievable** — `Get`, and you must already hold the address.
- **enumerable** — `Store.All()` (`memory/store.go:158`), needs the whole
  store, and **carries no starting point**: it cannot say which bits a
  reader was meant to begin from.
- **discoverable** — walk `Prev`/`Absorbed` out from a view.

D1 permits all three. **D14 counts only the third.**

**Ground, and it was already in force:** D54(a), "D14 binds the record, not
the surface." That kills the purposive counter-argument (*if `tldr top`
lists a stray, the harm D14 names does not occur*) because that is a surface
argument. What D54(a) does not kill is the narrower form — `All()` is record
API, so enumeration is a record property — and the answer to that is
textual: D14's binding sentence names a **mechanism** ("discoverable by
walking the record from the view via `Prev` and `Absorbed`"), not an API
surface.

**This is a clarification of scope, on D14's own precedent** (D14 clarified
D1), **and not a narrowing on evidence** — D14's own "What would change it"
pre-declares that it is a definition and does not move on evidence, so an
entry that claimed to move it on evidence would be overruling D14 by
ignoring it rather than by superseding it.

**Why "enumerable" and not "listable but not reachable"** — the argument is
`decision-guard`'s, and was adopted over the CEO's own first wording:
"reachable" is D1's own word ("permanently reachable"), so a ruling that
splits *that* word re-creates one level up the exact ambiguity D14 was
written to close. D14 already minted retrievable/discoverable; enumerable
sits between them and a reader can derive it from D14's own text rather than
having to be told.

**(b) REVERSAL: D63(i) and D64(h) were both wrong, in the same direction.**

- D63(i) (`docs/DECISIONS.md`) called `cmd/tldr/say.go`'s
  doc-comment reason "**false** since `Store.All()`, `tldr top` and
  `judged()` all enumerate the store."
- D64(h) narrowed that to "**stale** is the defensible word; 'false'
  overstates."
- **Under (a), both are wrong.** The sentence was true when written and is
  true now. What was wrong was the *vocabulary*, not the claim. D64(h)
  hedged in the right direction and stopped one word short.

**Also correct here, since it cannot be fixed in history:** D63(i) cites
`cmd/tldr/say.go:44-47` for that doc comment. Re-derived this session
against the current, rewritten doc comment: it now spans **`say.go:40-49`**.

**(c) The stray-utterance incident was a D14 failure, not only a wrong
invariant.** Three artifacts said it was "never D1 or D14 failing" because `top` and the
ranked surface enumerate: `cmd/tldr/record.go:182-186` (as it read at
`HEAD`, before this session's rewrite), `docs/CLAIMS.md`, `docs/CODE.md`.
All three are corrected in this session's working tree.

The stray was in the store, in no view, and pointed at by nothing —
stranded in exactly D14's sense, permanently, until `record.rejoin` was
built to catch it. It stayed **enumerable**. `decision-guard` argued that
conceding the incident while keeping the invariant would have left the
strongest evidence for the ruling on the other side of the table, and the
CEO accepted that.

**(d) TESTED: two writers permanently strand a ballot and a fold receipt.**
Found by `decision-guard` while reviewing the ruling, reproduced by
execution, mechanism confirmed by the CEO reading the code.

Construction, all supported operations:
1. Session A holds a record, casts a vote, checkpoints. File = store ∪
   {v1}, `shown` = A's, `votes` = A ∪ {v1}.
2. Session B, opened earlier with its own views, makes any change.
   `record.absorb` reads the file's **store only** — deliberately, its own
   doc says so — pulling v1 into B's store. `record.encode` then writes B's
   `shown` and `votes` over the file.
3. v1 is in the store, named by no view. A vote's `Prev` names its target
   and edges run backwards, so nothing points at it.
4. `load` → `rejoin` rescued `memory.Utterance` only. v1 stranded
   permanently.

**Red, re-derived by the CEO against `HEAD`'s `cmd/tldr/record.go` in a
scratch worktree**:
```
--- FAIL: TestNothingTheRecordHoldsIsStrandedByTwoWriters
    the record holds 11 bits and the two views reach 9; 2 stranded:
      3cc5ca1a  a fold receipt over 5 bits by cool
      6271f640  an upvote on d215686a by you
```

**Why it mattered and why nothing reported it:** a standing vote is
D4/D30's consolidation signal. A stranded ballot silently changes what
future folds keep, and every vote count on screen is computed from the vote
*view*, so no surface could show its absence.

**Why it was invisible to the tree's own checks:** `memory/reach_test.go`
and `tui/tui_test.go:626` both assert reachability over views a **single
process** holds. Nothing asserted it across a save/load with two writers.

**Three artifacts asserted it could not happen**, each giving an argument
true within one session's lineage and silently doing cross-session duty:
`cmd/tldr/record.go:182-186` at `HEAD` ("a ballot is accounted for by the
vote view, which is the view this program never folds"), `docs/CODE.md`
("utterances are the only kind that can strand"), and `docs/DEBT.md`, which
filed the whole thing as a *definitional visibility gap*, "not a violation
of it." **None of the three is code, which is why the commit gate never saw
them.** That is the finding worth keeping.

**(e) TESTED: the fix, and what it cost to get right.** `record.rejoin` now
computes accounted-for with D14's own transitive walk
(`record.reaching`, `Prev` + `Absorbed`) out from **both** views, reinstates
a ballot into `votes` and a fold receipt into `shown`. The shallow rule it
replaced would have been wrong the moment scars were looked for: a scar
under another scar is named by the outer one's `Prev` (D13) and by nobody's
`Absorbed`.

**The fix was itself defective on first pass, and the review caught it.**
`decision-guard`'s finding: a scar's `At` is `c.to` (`memory/cool.go:242`),
the *max* instant in its window, so an outer scar whose window ends on the
inner scar it absorbs carries the **same instant** — and the tiebreak was
the content address, so a **hash** decided which generation survived.
Reported measurements, not re-run this session: **12 of 20** trials
stranding the outer receipt (guard) and **14 of 20** on a second fixture
family (engineer); **0 of 20** after. Repaired by tiebreaking on
`Compaction.Count()` descending, which is a total order over the nesting
relation because `Cool` merges `p.count` and D32's size rule refuses a
window of one, so a run is never a single bit.

**A boundary on that argument, from `principal-go-engineer`, and it is the
honest part:** the strictness holds for records *this program writes*, not
for a hand-assembled one — `memory.Cool` itself refuses only an **empty**
window, so a window of one is legal, addressed and storable, and there
instant *and* count tie and the address decides again. Written into the
tree as its own test row rather than elided.

**Falsifiability, re-derived by the CEO:** neutralizing the `Count()`
tiebreak (deleting `cmp.Compare(standsFor(b), standsFor(a))` from
`outermost`'s sort in a scratch copy) reddens exactly one row —
`two receipts sharing an instant come back as the outer one`, inside
`TestALoadPutsAStrayBackWhereItWasSaidAndMovesNothingElse` — and nothing
else in the package or the module (`go test ./...` stays green everywhere
else).

**(f) TESTED: the ballot-tie safety argument was stated exactly
backwards.** The first fix's own comment, and `docs/CODE.md`, both said merging by
instant "hands an exact tie to the vote already in the file — the incumbent
arrangement wins." Both false, verified by the CEO at both sources: `merge`
(`cmd/tldr/record.go:528`) emits existing rows with `At <=` the stray
**before** it, so a reinstated ballot lands **later**; and `standing`
(`memory/vote.go:208`, the skip at line 221) skips only when
`b.At.Before(held.At)`, so the **later position wins**.
`decision-guard` measured a standing vote flipping +1 → −1 across a load.

**The trap, which is the reusable part:** the pre-existing doc sentence "a
row sharing its instant keeps its place: the view's own order wins every
tie" is about *position*, and keeping the incumbent's position is exactly
what hands it the loss. **Keeping a place is not keeping a standing.** Those
two sentences read as the same and are not.

CEO's ruling: **behaviour kept** — it follows `standing`'s own documented
rule and `merge`'s positional rule — **prose fixed**, and the actual
behaviour is now pinned by a test (`strays-merged-before-an-equal-instant`,
`docs/CLAIMS.md`) so the next reader cannot restate it wrongly. Runtime
reach is narrow (same voter, same target, same nanosecond, opposite
direction, against a nanosecond clock), so only a hand-assembled file or a
fixture reaches it.

**(g) TESTED: a self-report on falsifiability is the one number to
re-derive.** `principal-go-engineer` reported two elements of `rejoin` unfalsifiable,
both marked as priors. `decision-guard`, stubbing elements one at a time,
found **five**. The engineer's own stub sweep, run again with a proper
control row, found the true number was **six** — the two originally marked
plus four not caught — and reported that its own fix then added a
**seventh** unfalsifiable element (a new sort key) before that too was
closed.

Final state, confirmed by the CEO reading `cmd/tldr/record.go` directly:
**three** elements are marked as priors in the code where they live and
grep-confirmed by the phrase "reddens nothing" — the `reached`-guard on the
ballot filter (line 333), the `Absorbed` branch of `reaching` (line 366),
and the instant-first-key of `outermost`'s sort (line 470) — everything
else that was ever found unfalsifiable is now pinned by a test row. Two of
the CEO's own assertions about which were dead were wrong from inside the
code and the engineer corrected both: `drawn[b.ID] = true` is not subsumed
by the `reached` guard (it is what a *later-offered* receipt consults,
which is exactly the case the ordering cannot resolve), and the ballot
filter's `reached` guard is dead only because nothing folds a vote view,
while `reached` does grow between collection and that line.

Multiple independent counts of one property, none of them adversarial, all
of them sincere, none of them agreeing until the sweep was re-run with a
control row. D48's shape, and the rule that falls out: **a self-report on
falsifiability is not a number to accept, it is a sweep to re-run.**

**(i) A second copy of a comment that was false for two days, in the
public tree.** `memory/reach_test.go` carried "A Store has no enumeration on purpose —
nothing in the product walks the record except by following edges,"
written (2026-08-11). `Store.All()` landed (2026-08-14). A sweep found a
**second verbatim copy** at `memory/wire_test.go:98-100`. Both files and
`memory/store.go` are in the public tree, so **a stranger reading published
`memory/` found an enumeration method and two comments insisting there is
none** — a live D15 comprehension defect, found by reading, invisible to
any marker grep. Both fixed this session.

Note the split the tree already knew about and the record did not:
`memory/store_test.go` calls `All` "the auditor's read" and describes its
rows as "the three ways a bit ends up **unreachable from a screen**" — the
enumeration's own test never adopted the enumeration sense. And
`memory/store.go`'s `All` doc says what a reader "most needs to be able to
**find**", not *reach*.

**(j) OPEN, with what would close it: two residual strands, both decisions
rather than code.**

1. **A fold receipt the transcript already shows still strands.** A
   receipt has nowhere to stand where the transcript names material
   beneath it, because a view never holds both a scar and a bit it names,
   and **a load may insert but may not rearrange**. Nothing a reader can
   read is lost — every absorbed bit stays discoverable through the
   surviving scar — **what strands is the *event* of the fold.** Closing it
   needs a ruling about rewriting another writer's transcript, not code.
2. **Two writers at different terminal heights fold with different
   windows** (`keep()` is `budget()/2`, `budget()` is the terminal's
   height), so the same bits get summarised **twice, adjacently**, and the
   repair is written back at the next checkpoint. Reproduced: twelve bits,
   keep 7 and keep 4, tall session saves last. Refusing the stray would
   strand it — trading a legibility fault for a D14 fault. A genuine
   dilemma, recorded rather than resolved.

Both are in `docs/DEBT.md` with their constructions. A third, bounded: a
ballot naming other than exactly one target is left stranded on purpose,
since `Tally`/`Rank` panic on one and `memory.Cast` cannot mint one — only a
hand-assembled file can. `principal-go-engineer` declined to make `check()`
refuse such a file, and the CEO accepted the argument: every rule `check()`
holds is the second statement of a condition `memory` already enforces by
panicking, and refusing would deny a person their whole conversation over a
bit no shipped writer can produce. **The real missing thing is a warnings
channel out of `load`**, which does not exist — `load` returns a record or
an error.

**(k) D63(c) stays open, and three things are recorded that change what
the answer will mean.** The open question: *may tier two — agent votes — reorder the page the
human has not yet judged?*

`scope-adversary` was dispatched with the commercial thesis supplied up
front (D53(e)), and it broke the CEO's draft ruling:

1. **The CEO's own argument for "yes" was void.** The draft leaned on tier
   dominance — one human vote overrides any amount of tier two. But `Rank`
   (`memory/rank.go:104-113`) compares `Own` first and falls through to
   `Others` only on a tie, so for a bit the human has **not** judged `Own`
   is 0 by construction and `Others` is the *only* ordering. The tier
   guarantee protects the judged set; the harm named is entirely in the
   unjudged set. **A guarantee about the judged set cannot license a
   change to the unjudged set.**
2. **The product already implements "yes."** `Rank` sorts by `Others`
   within the `Own == 0` band today. So D63(c) is not gating an unbuilt
   surface — it is an **unratified property that already shipped**, which
   is a materially different status from the one D63(c) recorded.
3. **The correlation objection, which is new and is the strongest thing
   against.** Agent voters are instances of one model reading overlapping
   context under one charter. Five agent votes are not five signals; they
   are **one opinion at amplitude five, wearing the visual grammar of a
   crowd.** Reddit's aggregate is informative because voters are
   uncorrelated and a vote is bounded by a human deciding to care;
   `standing()` keys on `{voter, target}` so nobody votes twice, but **a
   handle is free**. Any tier-two display that aggregates without showing
   how many *distinct* opinions are behind it is a legibility failure by
   this project's own thesis.

**CEO's ruling: open, not deferred out of timidity.** D63(d)'s null
hypothesis is scheduled and unrun and would answer it observably, and this
project's standing rule is that the source you can execute beats the one
you can only read. Deciding it by argument now, with a measurement already
committed to, is the wrong way round.

One rejected framing, recorded so it is not re-argued: `scope-adversary`
read the recency-in-the-unjudged-band as `Rank`'s own named-and-rejected
failure mode ("Recency within a tier is a second ranking rule nobody has
decided", `memory/rank.go:69-70`) arriving by a back door. **Checked and
rejected** — `rank.go` rejects recency as a rule *inside* `Rank`, and both
callers then claim the tiebreak deliberately and say so
(`tui/ranked.go:62-66`, "the tiebreak belongs to the caller, expressed as
the view it passes"; `cmd/tldr/top.go:107-112`). It is a documented
delegation, not a leak. **But a real defect falls out of it:** `rank.go`
justifies keeping view order on a tie with "the ordering is checkable by
eye, because the only rows that moved are the ones somebody voted on" —
true of `Rank` alone and **false of every screen the product draws**,
since both real callers hand it a clock-sorted view. A claim that does not
survive composition, which is this project's most-repeated defect shape and
is recorded here rather than fixed — the fix is a wording change inside
`rank.go`'s doc and it belongs with whoever next has reason to open that
file, not bolted onto a checkpoint that was about something else.

`scope-adversary`'s cheaper alternative, named and **not built**: cap rows
per handle on the default page, or change what `judged()`/`top` hand
`Rank`. It addresses the *measured* harm in D63(c) — five seat notes
pushing two human bits off the page — without touching votes at all. Not
adopted, because it is itself a ranking rule nobody has decided, which is
the same objection.

**What would change any of this.** (a) is a definition and does not move
on evidence, same as D14 itself. (e)'s falsifiability re-derivation stands
until the code changes again. (k) reverses the moment D63(d)'s null
hypothesis runs, in either direction the measurement points.

---

## D67 — The founder pointed at a fast-moving field, and the sweep came back with the first outside measurement of D1 and a competitor D1 never considered

**2026-08-16**

**Status:** mixed, per clause. (a) is tested — the DGM ablation table
re-derived by the CEO directly from the paper's extracted text, the
tiering of the isolation proposed by `decision-guard` and adopted; the
paper's own n=1 cells and its reported 2.3% run-to-run sd are named rather
than re-run. (b) is asserted — arXiv API phrase counts run directly by
`decision-guard`, not re-run by the CEO. (c) is a ruling, reached
independently by two seats and adopted. (d) is tested — both citations
re-derived by the CEO at source against `memory/bit.go` and
`memory/view.go`. (e) is asserted — the character count and phrase-query
results are `decision-guard`'s, not re-run; the phrase-match limitation is
stated with the claim. (f) is open, named as the next research unit rather
than answered. (g) is asserted, from RQGM and the cited bias-measurement
papers, not re-run. (h) is a ruling on how research is organised, not a
factual claim. (i) is tested — the `grep`-returns-nothing-including-for-
"the" failure and its Python re-count are the CEO's own account of
`decision-guard`'s process, not independently re-run here.

**(a) TESTED: D1 has external evidence for the first time in the
project's life.** The Darwin Gödel Machine (arXiv `2505.22954`,
Zhang/Hu/Lu/Lange/Clune, submitted 2025-05-29, ICLR 2026) publishes this
ablation. Re-derived by the CEO directly from the paper's extracted text,
not taken from the sweep's report:

| Method | SWE-bench | Polyglot |
|---|---|---|
| DGM | 50.0% | 38.0% |
| DGM w/o Open-ended exploration | 23.0% | 14.0% |
| DGM w/o Self-improve | 39.0% | 28.0% |
| DGM Greedy | 39.7% | 30.0% |

**Removing the archive costs more than removing self-improvement** (23.0%
vs 39.0%). The paper's own gloss, quoted: "only the most recent agent is
retained, so a poorly performing self-modification makes subsequent
improvements harder to achieve."

The cleaner isolation, proposed by `decision-guard` and adopted: 23.0% (no
archive) against 39.7% (archive kept, greedy selection) holds selection
non-diverse in both arms, so **retention alone is worth ~16.7 points on
SWE-bench.**

**What does not survive, stated with the claim rather than below it:** DGM
discards agents that fail to compile, so it is not literally never-forget;
the ablation cells are n=1; DGM's own reported run-to-run sd is 2.3%
(Polyglot, n=3), which the gaps clear by a wide margin but which is a
figure about the main result, not the ablations. **The claim that
survives: retaining suboptimal material beats retaining only the best or
only the latest, measured, by someone else, at a top venue.** D1 was
decided in session one on an argument — "you can add collection later, you
can never un-delete" — with no evidence. It has evidence now.

**(b) ASSERTED: the field is growing but is not accelerating, and the
difference matters.** `decision-guard` ran the arXiv API directly.
Abstract-phrase counts by submission year: "recursive self-improvement" 2
/ 9 / 29; "self-improving agent(s)" 1 / 18 / 30; "self-evolving agent" 2 /
19 / 97 (2024 / 2025 / 2026-to-16-Aug). **Year-over-year growth is real and
large, ~2.7x–8x. Month-over-month inside 2026 is not accelerating** —
"self-improving agents" runs 3, 3, 2, 2, 6, 4, 8, 2: noise around a raised
floor.

The RSI survey (`2607.07663`) supplies the sharpest available line and
disqualifies its own headline figure in the same breath — it reports
quarterly growth to ~500 papers in 2026 Q2, then notes "the supplemental
harvest is recency-biased by construction." Its diagnosis: "The
literature's terminology has proliferated faster than its concepts."

The Gödel-machine lineage is five papers in fifteen months — DGM
(2025-05-29), HGM (2025-10-24), HyperAgents/DGM-H (2026-03-19), RQGM
(2026-06-24), **Mendel Gödel Machine (`2608.07645`, 2026-08-07, which the
CEO's own framing had missed entirely, nine days old at the time of the
sweep)** — on top of Schmidhuber's original (`cs/0309048`, 2003).
**DARWIN (`2602.05848`) is excluded from that lineage on purpose**: single
author, nanoGPT, five iterations, +1.26% MFU. It shares a name and nothing
else, and counting it is exactly the "papers sharing a naming convention"
inflation the sweep was asked to test for.

RQGM's affiliations, from the PDF's own title block rather than press:
**Cambridge, NVIDIA, Flower Labs, MBZUAI, Inria.** Press reporting of
"Cambridge and NVIDIA" is confirmed and incomplete.

**(c) RULING: "anchor infrastructure for self-improving agents" is
considered and rejected. Do not re-argue it.** The CEO proposed, roughly
two hours after reading one summary of one paper, that self-improving
systems all need a Ground-Truth Anchor of scarce human judgment, that
tldreddit produces exactly that as a byproduct, and that this is therefore
what the company is for. **It is wrong, and it was killed twice within the
hour by two seats reaching it independently.**

- **`scope-adversary`, from the paper's own abstract:** RQGM's thesis is
  that the fixed human-labelled anchor is *the thing to escape* — "opening
  search to evolving evaluators, adversarial objectives, and dynamic
  utilities that may surpass static benchmarks." The word "human" appears
  in that abstract once, as a comparison baseline. **And the 1.91x bias
  finding is corrected by an adversarial objective, not by human
  judgment** — a fact the CEO stated backwards, in writing, inside the
  very dispatch brief asking the seat to attack the claim.
- **`decision-guard`, on the market shape:** the anchors are ~100-item
  off-the-shelf published datasets (RQGM Table 3), so marginal spend on
  human judgment was near zero; the coding anchor is *executable tests*,
  so the requirement is evaluator-independence rather than humanity; and
  RQGM's own Limitations state they "intend to make our mechanism less
  reliant on good anchor datasets." The paper that most resembles
  human-oversight infrastructure, ANCHOR (`2606.06114`), **simulates its
  human supervision with an LLM and says so first in its own
  limitations.**

The one genuine measurement of a human premium points elsewhere and is
shrinking: SkillAxe (Microsoft, `2606.10546`) finds human-authored skills
improve pass rates by 16.2 percentage points where LLM-authored skills
give no measurable gain — but that measures humans **writing** memory, not
**ranking** it, and SkillAxe's own contribution closes 47–67% of the gap
without labels.

**Recorded as rejected specifically so a later instance does not
rediscover it as new**, which is this log's stated job for a discontinuous
executive.

**(d) TESTED: two findings about our own product survived the dead
thesis.** Both re-derived by the CEO at source:

- **`memory/bit.go:92`** — "Handle is an actor as observed: the trace
  something left on a channel, **never a person**... deciding which
  handles belong to the same actor is a separate, softer question that
  this package **deliberately does not answer**." Anything premised on
  attested human judgment runs straight into a designed-in refusal.
  Nothing in the record distinguishes a vote Tyler cast from one a seat
  cast while being the first user under D51.
- **`memory/view.go:132`** — "By is the one voter whose upvote holds a bit
  back." An upvote spares a bit from folding, so **the surface a person
  votes from is shaped by what they already voted on: our votes are a
  sample selected by earlier votes.** More votes tighten the selection
  rather than loosening it. Partly mitigated, and the mitigation is
  already decided: `tldr top` reads the whole record, not the shown view
  (D58(a)). Nobody had named this property before; it is new, it is
  independent of the rejected thesis, and it lands near D63(c)'s hazard
  from a second direction — both concern what a person ever gets the
  chance to judge.

**(e) ASSERTED: D25 holds, and the null is measured rather than assumed.**
Across ~800k characters of primary text from the five core papers,
"vote" occurs zero times; "content-address" zero; "append-only" zero;
"immutable" three, all incidental. Four arXiv queries (`abs:"agent
memory" AND abs:"upvote"` → 0; `all:"vote" AND abs:"agent memory"` → 0;
and two more) return nothing that duplicates the combination. **D56(i)'s
trigger stays unfired.**

Stated limit: the queries are phrase-matched, so a system calling it
"preference aggregation" or "human ranking signal" would not surface.

**(f) OPEN, and named as the next research unit rather than answered
here: Trellis.** `2606.29823` (Meta Platforms + Daniel Abadi, UMD,
2026-06-29) reaches D1's conclusion — you must be able to reconstruct any
past state — from **training-data correctness** rather than legibility:
"the numeric state that drove each decision … is mutated in place by
later search, so replaying it from final state leaks future information
into the example." Its mechanism is the opposite of ours: **mutable in
place, plus a CDC changelog and AS-OF time-travel queries**, versus our
immutable-and-derive.

**D1's alternatives-considered does not contain it.** Two
questions it forces, both open: whether our substrate has an argument
against changelog-over-mutable that is not merely aesthetic (the likely
one — a changelog is a mechanism a person cannot read, and legibility is
the thesis — is not currently in the record), and that Trellis names a use
for retention we never have: training-data correctness.

Also logged as a near-neighbour, not a competitor: **TOKI**
(`2606.06240`), bitemporal, "preserves the losing fact in an audit row" —
D1's move in database clothing, no ranking.

**(g) ASSERTED: an input to D63(c) — the literature gives a direction and
no magnitude.** D63(c) is open: may tier two reorder the page the human
has not judged? What the outside evidence supplies:

- **1.91x is one point, not a floor and not typical.** RQGM's body says
  1.42x–1.91x from one baseline reviewer, and immediately concedes
  entanglement — the benchmark's accept/reject balance rewards lenient
  reviewers, so bias and raw accuracy pull the same way. **RQGM's
  Limitations also state no human grading of the generated papers or
  proofs was performed at all**, so every acceptance figure in it is
  agent-judged.
- Other work gives bias *scores*, not ratios (Panickssery et al.
  `2404.13076`; Wataoka et al. `2410.21819`, which finds the root cause is
  perplexity — familiarity, not identity). The only other absolute
  figure, BadScientist's 82.0% acceptance of fabricated papers
  (`2510.18003`), measures something else.
- **The mechanism finding is the useful part:** RQGM's cure for reviewer
  self-preference worked *only because an evaluator-independent anchor
  existed* to promote the harsher reviewer against. Mapped onto D63(c),
  the outside evidence supports **keeping tier two out of the unjudged
  band** — which is precisely the band where we have no anchor, since
  `Own == 0` across most of the record.

**This does not close D63(c).** D63(d)'s null hypothesis, still scheduled
and unrun, is what closes it — D66(k) already says so. What changes is
that the prior going in is no longer neutral.

**(h) ASSERTED: research becomes a named watch, not a generic beat.**
D58(k) made research a scheduled beat with one trigger ("before building
any instrument, look for one that exists") but left *what to watch*
generic. The founder's input this session: "I just want us to not ignore
this self-improving agent space since it's moving fast and it's kinda the
space we're operating in." A generic beat drifts toward whatever is
convenient to search; a named one does not. **The self-improving-agent
lineage is now an enumerated standing watch**, with the papers in (b) and
(f) as its current front.

Note against over-reading the founder's input: he explicitly did *not*
direct the organisation of it — "You're CEO so as long as the board sees
returns on investment I don't really care how you stay organized." This is
a smell report from the shareholder, taken as input, not a spec.

**(i) A method note worth more than its size.** `decision-guard` reports
that `grep` silently returned zero matches against the extracted PDF
text — **including for the word "the"** — and that it nearly filed
"Selective Erasure does not appear in the paper" as a finding on that
basis. Python's `str.count` showed 28 occurrences. The check that caught
it was **grepping for a word already known to be present**. Every count in
that sweep was produced in Python thereafter. This is the project's own
red-green rule arriving in a new place: a tool returning zero is not
evidence of absence until the tool has been shown able to return non-zero.

**What would change any of this.** (a) moves if anyone re-runs DGM's
no-archive ablation and gets near 50%. (c) is closed absent a measurement
of a human *ranking* premium, as opposed to an authoring one. (e)'s null
moves the day a phrase-match miss is found. (f) is open by construction
and is the next research unit. (g) reverses with D63(d), in either
direction.

---

## D68 — Four wrong figures in D67, one of them inside the clause about not trusting a figure

**2026-08-16**

**Status:** tested, per correction. Every figure below was re-derived at
source by `decision-guard` during D67's publication gate, and the sharpest
two again by the CEO before this entry was commissioned.

**(a) TESTED: D67(i) attached a count to the wrong string, in the clause
whose subject is exactly that.** D67(i) records that `grep` silently
returned zero on extracted PDF text and that a false finding — "Selective
Erasure does not appear in the paper" — was nearly filed on it, then says:
"Python's `str.count` showed 28 occurrences." **28 is the count of the
bare word "erasure."** Re-derived against `rqgm.txt`, the same file the
sweep used, and confirmed twice:

- `"Selective Erasure"`, case-sensitive: **0**
- `"selective erasure"`, case-insensitive: **9**
- `"erasure"`: **28**

The sentence names the two-word phrase and attaches the one-word count to
it, off by a factor of three. **The clause states the rule "a tool
returning zero is not evidence of absence until the tool has been shown
able to return non-zero," and carries a number nobody re-derived while
stating it.** That is the whole finding, and it is worth more than the
arithmetic: the lesson was written down correctly and violated in the
sentence writing it.

**(b) TESTED: the cause of the grep failure, which D67(i) recorded only as
a symptom.** D67(i) says grep "silently returned zero... including for the
word 'the'" and leaves it there. The mechanism, found by `decision-guard`
and reproduced by the CEO:

```
$ grep -c 'the' 2505.22954.txt     # prints nothing, exit 1
$ grep -ac 'the' 2505.22954.txt    # 815
```

**The extracted files contain NUL bytes, so GNU grep classifies them as
binary and suppresses output** — two such bytes in `2505.22954.txt`, five
in `rqgm.txt`, out of hundreds of thousands of characters. `grep -a` is the
fix.

**And the obvious check does not work**, which is worth more than the fix.
The CEO's brief for this entry said "`file` first is the check." It is not:
`file` reports these very files as `Unicode text, UTF-8 text`, because two
NUL bytes in 800k do not change what the file mostly is. That sentence was
caught before this entry was committed, by `archivist` running the check
rather than repeating it — the third time in one checkpoint that a
plausible sentence about verification failed the verification it described.
What does work, confirmed against a known-clean control:
`LC_ALL=C grep -qaP '\x00' <file>`. D67(i) recorded a symptom that reads like a mystery; the cause
makes it a reusable rule, which is the difference between a war story and
craft. Note it belongs in `.claude/craft/` as well, but this entry is
where it is adjudicated.

**(c) TESTED: "the word 'human' appears in that abstract once" — it
appears twice.** D67(c), arguing that RQGM is not about human judgment,
says the word appears once in RQGM's abstract as a comparison baseline. It
appears **twice**, both as comparison baselines:

- "over-accepts AI-generated papers at up to 1.91× the **human** rate"
- "discovers reviewers equally stringent on AI and **human** work"

**The argument is untouched** — both uses are baselines, neither makes
RQGM a human-judgment system, and D67(c)'s rejection of the anchor thesis
stands unchanged. Only the count was wrong. Recorded because a count is
the kind of claim a stranger settles in one command, and D67 will be
quoted outward more than most entries.

**(d) TESTED: D67(a)'s summary names an ablation arm that does not
exist.** D67(a) concludes: "retaining suboptimal material beats retaining
only the best or only the latest." "Only the latest" is `DGM w/o Open-
ended exploration` (23.0%) and is right. **"Only the best" is not an arm
of DGM's Table 1.** `DGM Greedy` retains the entire archive and *selects*
greedily — the paper files it under "Ablation of parent **selection**."
D67(a)'s own earlier sentence has this exactly right ("39.7%, archive
kept, greedy selection") and the summary then contradicts it.

**The correction is one word: *using* suboptimal material, not *retaining*
it.** The 16.7-point figure and the underlying claim are unaffected — the
comparison was always retention-with-greedy-selection against
no-retention. What was wrong was the summary's description of the second
arm.

**(e) TESTED: D67(b) quotes the RSI survey's caveat against the survey's
own meaning.** D67(b) says the survey "disqualifies its own headline
figure in the same breath," quoting "the supplemental harvest is
recency-biased by construction." The sentence continues into a
parenthetical D67 stops just short of: "(growth statistics in Figure 6
therefore use the seed corpus only)", and Figure 6's caption reads
"**Seed-corpus** quarterly growth through 2026Q2."

**The survey is guarding the figure, not disqualifying it.** D67(b)'s
conclusion — that year-over-year growth is real and month-over-month is
not accelerating — is unaffected, because it rests on the CEO's own arXiv
API counts and not on the survey. What was wrong is the evidence-to-claim
link: a paper was described as undermining itself when it was doing the
opposite.

**(f) ASSERTED: what this says about the checkpoint that produced it.**
Four wrong figures in one entry, all four in an entry whose own subject
includes the discipline of re-deriving figures, and **all four found by
the publication gate rather than by the seat that wrote them or the CEO
who commissioned them.** The gate is now the load-bearing check on the
record's accuracy and not only on its confidentiality, which is not what
D15 was written for.

**Nothing currently runs a figure-verification pass on an entry before it
is committed** — the pre-commit gate reads code, and `cmd/seam` reads
claims about code. A prose entry's numbers are checked by nobody until
publication.

**What would change this.** (a)–(e) are settled against files that do not
move. (f) changes when something checks an entry's figures before the
commit rather than after.

---

## After D68

D68 is the newest entry published here, and, as of this push, it is also the
newest entry in the private record — there is nothing past it yet to
withhold. That is a fact about this moment and not a standing one: the
private record gains entries between pushes, so by the time you read this
the log has probably continued somewhere you cannot see.

This paragraph exists because the two kinds of gap are not equally visible.
A missing number in the middle of a numbered list announces itself — you can
see that D8 is not there. A log that simply stops does not. Without this
note, a record ending at D68 would read as though D68 were the last decision
taken, rather than the last one published, and nothing else in this file
would tell you otherwise.
