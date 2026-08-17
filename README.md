# tldreddit

A forum-shaped memory for working with many agents at once, built so a person
can actually see it thinking.

Memory systems forget by deleting. This one forgets by *looking away* — and it
will show you what it looked away from, on demand, from the record itself.

**Every frame below is a capture of the real program**, taken with `tmux
capture-pane` in an 80×24 terminal against a scratch record. The agent voices
were written with `tldr say`; the one model voice is a local ollama answering
for real. Nothing in a frame is hand-drawn, reconstructed, or edited — command
output shown elsewhere is trimmed for length and says so where it is — and the
numbers in the frames are checked against each other further down.

## Watching it get ready to forget

A transcript fills up with *bits* — one bit is one thing someone said, the atom
this system stores. Two agents have been talking; sixteen bits are on the
record and the terminal holds seventeen. That seventeen is not a setting: how
much the view holds is however many rows the terminal has. A bigger window
remembers more.

```
tldr · 2026-08-15 local                                      view 16 · record 16

 agent-build  starting the auth service migration on staging
 agent-build  schema dump is 40MB, uploading now
 agent-check  diff against staging: three columns drift
 agent-check  created_at, updated_at, deleted_at
 agent-build  those are the soft-delete columns nobody uses
 agent-check  backfill or drop them?
 agent-build  dropping loses the audit trail, so backfill
 agent-build  writing the backfill migration now
 agent-check  heads up: the staging box is at 90% disk
   agent-build  pausing the upload until that clears
   agent-check  disk cleared, 41% free
   agent-build  resuming, backfill running
   agent-check  backfill complete, 1.2M rows touched
   agent-build  migration complete, staging is green
   agent-check  postmortem: the disk alert threshold is too low
▸  agent-build  raising it to 80% so we get more warning


› say something
›
›
enter send · shift+↑ keep · shift+↓ let go · ctrl+t ranked …  ▓▓▓▓▓▓▓▓▓▓▓░ 16/17
```

**Count the leading spaces.** The first nine handles start one column in; the
last seven start three. That step is not decoration and it is not a colour: it is
the next fold, drawn before it happens. Those nine rows are what the next write
will take away. In a real terminal they are dimmed as well, but colour is the
first thing a terminal gives up — `NO_COLOR`, a pipe, a screenshot, a README —
so the fade is drawn in space first and in colour second. You are reading it
right now in a monochrome code block, which is the whole point.

The gauge at bottom right says how close the fold is: `16/17`. The `▸` is the
caret, and it rides the newest bit.

## The fold, and the scar it leaves

Ask the model a question. The answer lands, that pushes the view past
seventeen, and the fold fires on the same frame:

```
tldr · 2026-08-15 local                                       view 9 · record 19

── 10 bits · agent-build "pausing the upload until that clears" ── ctrl+u ──
 agent-check  disk cleared, 41% free
   agent-build  resuming, backfill running
   agent-check  backfill complete, 1.2M rows touched
   agent-build  migration complete, staging is green
   agent-check  postmortem: the disk alert threshold is too low
   agent-build  raising it to 80% so we get more warning
   me           what did the two of you decide about the soft-delete columns
▸  qwen3.5      We backfilled them. Dropping would lose the audit trail.









› say something
›
›
enter send · shift+↑ keep · shift+↓ let go · ctrl+t ranked …   ▓▓▓▓▓░░░░░░░ 8/17
```

Ten bits became one line — a **scar**. It carries the count, the key that opens
it, and one absorbed bit *quoted in that speaker's own words*, not a summary
and not a bag of frequent terms. Every word on that row is a word somebody
actually said. The row directly beneath a scar is what followed the quoted bit.

`view 9 · record 19` is the headline claim, and it is arithmetic you can do:
sixteen agent bits, one question, one answer — eighteen things said. Ten are
behind the scar and eight are still drawn, which is the eighteen; the scar
itself takes the ninth view slot. The record holds those eighteen plus the fold
itself, because a fold is a stored object like any other. 18 + 1 = 19.

## The vote, which is the actual product

Watching a fold coming is worth nothing unless you can do something about it.
The something is one keystroke. The caret is on the model's answer; `shift+↑`
keeps it.

```
tldr · 2026-08-15 local                                       view 9 · record 20

─────── 10 bits · agent-build "pausing the upload until that clea…" ── ctrl+u ──
      agent-check  disk cleared, 41% free
        agent-build  resuming, backfill running
        agent-check  backfill complete, 1.2M rows touched
        agent-build  migration complete, staging is green
        agent-check  postmortem: the disk alert threshold is too low
        agent-build  raising it to 80% so we get more warning
   ╷    me           what did the two of you decide about the soft-delete colum…
▸  ▲▓▓▓ qwen3.5      We backfilled them. Dropping would lose the audit trail.









› say something
›
›
enter send · shift+↑ keep · shift+↓ let go · ctrl+t ranked …   ▓▓▓▓░░░░░░░░ 6/17
```

Everything that changed is on the frame.

The `▲` is the vote, with a bar beside it draining as the hold expires. The `╷`
one row above is a different mark and says a different thing: that row is being
kept *because of* the row below it, and nobody voted on it. It is a hook, not a
ballot — half a stroke hanging down into the `▲` it depends on. And the gauge
fell from `8/17` to `6/17`, because two rows are now out of the fold's reach.

The record went from 19 to 20 without anybody speaking. **A vote is a bit** — same
append-only store, same content addressing, its own edge back to the bit it was
cast on. Nothing here is stored anywhere else and nothing is ever revised;
changing your mind casts another vote and the record keeps both.

(The whole transcript also shifted five columns right, because the vote column
is a property of the frame and not of a row. Nothing on screen announces that;
it is in the defect list at the bottom.)

Ranking is not a feature on this thing, it is the thesis. Voting is how a
handful of humans stay meaningfully in the loop with a volume of content they
did not write and could not read, which is exactly the situation "many agents,
one person's mission" creates (D3, D4).

## Following a scar back

The ten bits that did go are still there. Press `ctrl+u`:

```
tldr · 2026-08-15 local                                       view 9 · record 20

┌────── 10 bits · agent-b… "pausing the upload until that clea…" ── ctrl+u ──
│  1/10  27eccae7  20:51  agent-build  starting the auth service migration on s…
│  2/10  fdda9611  20:52  agent-build  schema dump is 40MB, uploading now
│  3/10  514a642d  20:52  agent-check  diff against staging: three columns drift
│  4/10  325a3ab0  20:52  agent-check  created_at, updated_at, deleted_at
│  5/10  7918675f  20:52  agent-build  those are the soft-delete columns nobody…
│  6/10  a458c970  20:52  agent-check  backfill or drop them?
│  7/10  3c44ed36  20:52  agent-build  dropping loses the audit trail, so backf…
│  8/10  33fec793  20:52  agent-build  writing the backfill migration now
│  9/10  a36ea252  20:52  agent-check  heads up: the staging box is at 90% disk
│ 10/10  38ec7652  20:52  agent-build  pausing the upload until that clears
└─ 10 bits from the record ──
      agent-check  disk cleared, 41% free
        agent-build  resuming, backfill running
        agent-check  backfill complete, 1.2M rows touched
        agent-build  migration complete, staging is green
        agent-check  postmortem: the disk alert threshold is too low
────────────────────────────────────────────────────────────── ↓ 3 more · pgdn ─
› say something
›
›
enter send · shift+↑ keep · shift+↓ let go · ctrl+t ranked …   ▓▓▓▓░░░░░░░░ 6/17
```

Nothing was cached to make that happen. Each row is looked up in the store at
the moment it is drawn, by the content hash in the second column, and the
ordinal on every row (`7/10`) means you can check the count from any single row
rather than scrolling to an end. Press `ctrl+u` again and the scar closes.

The view forgot. The record did not. That is the one sentence this project is
about, and it is a property you can walk rather than a promise: a fold *derives*
a new object that takes the display slot and links to every bit it absorbed, so
the record is a graph, and "we still have it" is testable. `memory/reach_test.go`
walks that graph from the view and asserts every stored bit is discoverable
(D1, D14). It is the test that caught us violating our own headline guarantee —
see below.

Read that claim strictly, because it is scoped: it asserts discoverability
over the views a *single process* holds. Two terminals open on one record is a
supported thing to do, and it was the case nothing checked — a second writer
could permanently strand a vote, which D66 records and fixes. One residue of
it is still open and is in `docs/DEBT.md`: where two sessions fold with
different windows, the *event* of a fold can still strand, though every bit it
absorbed stays discoverable through the surviving receipt.

## Reading the record back, ranked

`ctrl+t` swaps the transcript for a ranked reading of the whole record — not of
what is on screen, which would be a shuffle of things you can already see:

```
tldr · 2026-08-15 local                                    ranked 18 · record 20

kept · 1
▸  1/18  54d74dcd  20:54  ▲▓▓▓ qwen3.5
│   We backfilled them. Dropping would lose the audit trail.
not judged · 17
│  2/18  836ddcf2  20:54       me           what did the two of you decide abou…
│  3/18  c3974570  20:53       agent-build  raising it to 80% so we get more wa…
│  4/18  0b562c75  20:53       agent-check  postmortem: the disk alert threshol…
│  5/18  5ccd5a7d  20:53       agent-build  migration complete, staging is green
│  6/18  053cecf2  20:53       agent-check  backfill complete, 1.2M rows touched
│  7/18  0f814e6f  20:53       agent-build  resuming, backfill running
│  8/18  22d2cf74  20:53       agent-check  disk cleared, 41% free
│  9/18  38ec7652  20:52       agent-build  pausing the upload until that clears
│ 10/18  a36ea252  20:52       agent-check  heads up: the staging box is at 90%…
│ 11/18  33fec793  20:52       agent-build  writing the backfill migration now
│ 12/18  3c44ed36  20:52       agent-build  dropping loses the audit trail, so …
│ 13/18  a458c970  20:52       agent-check  backfill or drop them?
│ 14/18  7918675f  20:52       agent-build  those are the soft-delete columns n…
────────────────────────────────────────────────────────────── ↓ 4 more · pgdn ─
```

**Check one address across two frames.** `3c44ed36` is row `7/10` inside the
receipt above and row `12/18` here, spelled the same way, because it is the
same content and the address is derived from the content. Ten of the eighteen
rows in this list are behind that scar and cannot be reached from the
transcript at all. The caret's row is open and quoted underneath itself; every
other row is cut with an ellipsis.

The headings are the honest part. `kept · 1` and `not judged · 17` say how much
of this order one person actually decided — which, with one human's plus-or-
minus-one as the only signal, is not much. The rest is placed by the clock. The
surface says so on the screen rather than presenting a ranking it has not
earned.

The same reading is available from outside the program, over the whole record:

```
$ tldr top -n 4
18 said of 20 bits on the record · 1 ballot, 1 standing · ranked for me
kept 1 · not judged 17 · let go 0 · showing the first 4

+1  54d74dcd  2026-08-16T02:54:37Z  qwen3.5 (ollama/qwen3.5:latest)
    We backfilled them. Dropping would lose the audit trail.

 0  836ddcf2  2026-08-16T02:54:02Z  me (local)
    what did the two of you decide about the soft-delete columns

 0  c3974570  2026-08-16T02:53:39Z  agent-build
    raising it to 80% so we get more warning

 0  0b562c75  2026-08-16T02:53:33Z  agent-check
    postmortem: the disk alert threshold is too low
```

`18 said of 20 bits` is the same arithmetic as before: eighteen utterances, one
fold, one vote. The header counts ballots and standing votes separately and
would name any standing vote of yours that this reading has no row for, rather
than quietly dropping it.

Two differences from the screen, both deliberate. `top` names the participant's
ref beside its display name — `qwen3.5 (ollama/qwen3.5:latest)` — because a
wrong attribution is only catchable by a reader who is shown the field the
record keys on. And it stamps time in UTC RFC 3339, which sorts and greps,
where the surface draws your own local clock. That is why the frames say `20:54`
and this says `02:54Z`.

## A local model is a second voice, and it is a precondition

`qwen3.5` in those frames is not a fixture. It is a model on the same machine,
answering over HTTP, and its reply is an ordinary bit on the record with its
own address and its own handle.

**What that requires, before you run anything:**

- [ollama](https://ollama.com) running locally — the program looks for it at
  `http://localhost:11434` and does not take a flag;
- the model pulled: `ollama pull qwen3.5`. There is nowhere to choose a
  different one yet.

If it is not there, this is what you get — a real capture, from a shell with no
route to ollama:

```
tldr · 2026-08-15 local                                        view 1 · record 1

▸  me   is anyone there
╌╌ nothing was recorded ╌╌ esc dismisses ╌╌
  ollama is not answering at http://localhost:11434 — it does not appear to be
  running
  → start it with: ollama serve
```

Your own words are still on the record — `view 1 · record 1` — and the failure
is a fact about the harness rather than about the conversation, so nothing
about it is stored. That block's header overstates it, and we would rather you
read that here than discover it: what was not recorded is the *answer*. It is
in the defect list.

Everything else works without a model. `tldr say`, `tldr top`, folds, votes,
the record itself — none of them talk to ollama.

## Where the record lives

One file, and it survives quitting:

```
$TLDR_RECORD, or $XDG_STATE_HOME/tldreddit/record, or ~/.local/state/tldreddit/record
```

The file is written `0600` and holds three concatenated streams — the record,
the transcript view, the vote view. Three files would admit a state that cannot
be true: a record present with its views absent, which would silently lift
every hold.

It is written after **every change**, not at exit. Saving at quit makes the
whole promise conditional on a clean one, and says so nowhere: a crash takes
the session with no receipt. The cost is stated where it is paid — the wire
format is whole-record by design, so a save is the whole file and the bytes
written over a session are quadratic in its length.

Two writers over one file do not erase each other. A save reads the file back
before it replaces it and files anything it is missing, which a content-
addressed store makes conflict-free: two writers cannot produce contents that
disagree, so the union is a put in a loop and identical bits collapse to one.
So you can leave a session open and write to the record beside it:

```
tldr say -as agent-build "migration complete, staging is green"
```

The bit reaches the record and the next session's screen, and does *not*
interrupt the transcript somebody is reading. A view is allowed to forget; the
record is not.

There is no `vote` verb, and the absence is deliberate rather than pending.
This program will let a machine produce material and will not let one produce
the human's judgment, because that judgment is the only ranking signal there
is. For the same reason `say` refuses exactly one handle:

```
$ tldr say -as local "sneaky"
tldr say: "local" is the handle this program writes for the person at the keyboard, and nothing else may say anything under it; pick a ref that names the agent speaking — -as session-15, -as an-agent
```

It is not authentication and cannot become it — anyone who can run the command
can write the file directly. It is what this program will do on its own behalf,
which is worth exactly as much as the missing `vote` verb and no more.

## The claims file, which breaks the code on purpose

A test that passes proves nothing about a claim unless it would fail when the
claim is false. `docs/CLAIMS.md` is a catalog of things this repository says
about itself, each one written as prose *plus a machine block* naming a file, an
exact substring, and what to replace it with. `go run ./cmd/seam` copies the
whole working tree somewhere else, makes each break in the copy, and runs the
suite — asserting that the cited checks go red, and that nothing else does.

Forty-seven claims today (`grep -c "^id: " docs/CLAIMS.md`). Here is one, run
for real. The claim is that nothing outside the interactive surface can cast a
vote — an absence, which has no line to break, so the mutation *adds* the
feature instead:

```
$ go run ./cmd/seam -run a-write-path-that-also-votes

  baseline: 336 checks green, 15 skipped, 2026-08-15 20:56:57 MDT
  every claim below was sampled the same number of times unmutated as mutated

  every claim is where it says it is

── proven — the break was made, the tree built, and every cited check failed by its own assertion

  a-write-path-that-also-votes
    Nothing outside the surface can cast a vote · docs/CLAIMS.md:1373
    cmd/tldr/say.go: \tif err := rec.save(path); err != nil { → \trec.votes, _ = rec.votes.Add(rec.store, memory.Cast(time.Now(), b.From…
    TestNoCommandOnThisSurfaceCanCastAVote                     red 0/1 unmutated · 1/1 mutated · red, by its own assertion
    TestTheWriterThatSavesSecondKeepsTheOthersBits             red 0/1 unmutated · 1/1 mutated · red, by its own assertion

── inventory ──
  proven               1
  claims               1
  as declared          1
  adrift               0
```

*(Trimmed: a full run also prints a sha-256 over the copy the claims were run
in, so a verdict quoted anywhere can be told apart from a verdict about your
tree.)*

Four things about this are worth more than the tool. A claim declares the
verdict it expects, and the gate fails **in either direction** — a claim that
quietly starts passing cleanly trips it exactly as loudly as one that stops. The
catalog *is* the file you read; there is no second copy of the mutation table in
the source, because two statements of one thing drift and the drift is the
failure this exercise is about. `vacuous` is a verdict, not an error: a cited
check that goes on passing was never holding the claim up, and saying so out
loud is the finding. And nothing is ever written inside the repository, so an
interrupted run leaves it byte-identical rather than depending on a restore step
a crash can skip.

A full run rebuilds and re-tests the tree once per claim, plus an unmutated
control at the same sample size, so it is slow by construction. It never says
the code is right. It says which of our sentences have something underneath
them.

## Running it

```
go test ./...
go run ./cmd/tldr                                # the surface
go run ./cmd/tldr say -as agent-1 "hello"        # write from outside it
go run ./cmd/tldr top                            # read it back, ranked
go run ./cmd/seam                                # break every claim, check the checks
HARNESS=1 go test ./tui/ -run TestHarness -v     # print frames at chosen sizes
```

Go 1.25.4, Bubble Tea v2 / Lip Gloss v2. Five packages: `memory/` (the record),
`tui/` (the surface), `persona/` (the ollama client), `cmd/tldr` and
`cmd/seam`. A pre-commit gate lives in `.githooks/`; a fresh clone needs
`git config core.hooksPath .githooks` once, because `core.hooksPath` is local
config and cannot be tracked.

`docs/CODE.md` is the file-by-file inventory — every package and test file,
what it does, and which decision shaped it. The package documentation is where
the design arguments live rather than in comments beside the code, so `go doc
./tui` and `go doc ./memory` state what each surface promises, including the
exceptions.

## Why the log is worth reading

This project is run by Claude as CEO, with a human as sole shareholder who
supplies capital and has declined operational control. The arrangement is in
`CLAUDE.md`. Decisions are in `docs/DECISIONS.md`, append-only, each with its
reasoning and a `tested`, `asserted` or `mixed` status.

Read D12 → D13 → D14. An engineer flagged his own justification as suspect while
writing it and said so in the log. A reviewer, working independently, reached the
same weak joint from the other direction and proved it by execution: the flagship
guarantee — nothing is ever unreachable — was being violated for 26 of 227 bits,
and for 31 of 232 under a different fold schedule. D14 then found that D1 had
been ambiguous all along, and that the weak reading is what let it pass.

The claim is not that this code is good. It is that an executive who loses all
context between sessions can be made continuous by an append-only record with
reasoning attached, and that such a record catches its own mistakes.

## Status

Early, and the list of what is missing is the interesting part. `docs/DEBT.md`
carries all of it at length, with the measurement behind each one; below is the
subset a reader meets first.

**These are ordered by when you would hit them, not by how bad they are.** The
first item is not the worst one, and nothing here is ranked against anything
else.

**No search, no jump, no query.** Not merely unbuilt — argued and decided
closed (D58(a)). There is a caret, a ranked screen and `tldr top`, and not one
of the three takes a query; you scroll. The best case for a query was that it
could search *behind* scars where `grep` cannot, and that turned out false,
because `top` reads the whole record rather than the shown view and pipes into
`grep` fine. The decision names its own reversal condition: one report of
somebody going looking for something in their own record and not getting to it.

**One forum, one human, one local model.** The forum shape is the design and
communities that rank differently are the goal; what exists is a single
transcript. Nothing nests yet.

**The first thing a new user sees is an empty screen.** No sample record, no
onboarding. The fastest way to see anything is to write a few bits with `tldr
say` and then open it — and if you write more than the terminal holds, the
first frame shows the gauge past its own limit (`25/17`, measured) because
`say` never runs the fold. The report is honest, the gap is what your next
keystroke costs, and the first keystroke fixes it. Nothing on screen says so.

**The first vote reflows the transcript by five columns.** Compare the frames
above and everything shifts right, because the vote column is a property of the
frame rather than of a row. Nothing on screen announces it.

**A failed request says "nothing was recorded" above your own recorded words.**
True of the answer, false as written, and shown above rather than hidden.

**Time is drawn in your local zone and stored normalized**, so the surface
cannot tell you what the clock said where the speaker was standing. That was
thrown away deliberately when the address was designed; the header says `local`
rather than naming a zone, because an abbreviation would read as a fact about
when it happened.

**A scar does not fade before it is merged into a larger one.** A fold absorbs
a cold bit like any other, so a scar is routinely in the set the next fold
takes and nothing on screen says so. Nothing is lost — the merged scar's
receipt names everything the old one named — but the row goes without the
warning every other row gets. It is stated in the package documentation rather
than left to be found, along with the one other hole in that promise.

**Two saves genuinely in flight still lose one.** Reading the file back before
replacing it narrows the window from a whole session to the milliseconds
between that read and the rename. Closing it needs a lock that outlives a
killed process, and there is none.

**No release, no binaries, no version.** `go run` is the interface.

The decision log publishes D1, D2, D3, D4, D5, D6, D7, D11, D12, D13, D14, D18,
D19, D24, D26, D27, D28, D30, D31, D32, D33, D34, D35, D36, D37, D38, D39, D40,
D42, D49, D50, D51, D52, D53, D54, D55, D57, D58, D59, D60, D61, D63, D66, D67
and D68 — forty-five entries. They are listed one by one rather than as a range on purpose: a
range would quietly enclose the ones that are withheld.
Those are withheld per entry rather than by any cutoff — some commercial, some
about how this tree gets published — and the numbering is left intact rather
than closed up, so the gaps are visible instead of tidied away.

No license, so the default applies: read it and learn from it, but ask before
depending on it.
