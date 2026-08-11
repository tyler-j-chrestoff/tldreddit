# tldreddit

A forum-shaped memory for working with many agents at once, built so a person
can actually see it thinking.

Memory systems forget by deleting. This one forgets by *looking away* — and it
will show you what it looked away from, on demand, from the record itself.

## What that looks like

A transcript fills up with *bits* — one bit is one thing someone said, the atom
this system stores. Older bits get folded into a single line — a scar —
carrying the count, the time span, and the words the window was about. The
status bar keeps score: **the view holds twelve, the record holds thirty-five.**

```
tldr                                                         view 12 · record 35
────────────────────────────────────────────────────────────────────────────────
── 21 bits · 19:38–19:38 · backfill staging migration production ── ctrl+u ──
coordinator-7  production is green, migration complete
me             writing the postmortem note
coordinator-7  nothing to post-mortem, it went clean
me             still worth a note for the next person
coordinator-7  fair. filing it under runbooks
me             closing the incident channel
coordinator-7  thanks everyone
me             one more thing: the disk alert threshold
coordinator-7  raise it to 80% so we get more warning
me             filed as a follow-up ticket
coordinator-7  done for the day
────────────────────────────────────────────────────────────────────────────────
› say something
›
›
enter send · ctrl+k fold · ctrl+u unfold · ctrl+c quit        ▓▓▓▓▓▓▓▓▓▓▓░ 11/12
```

**Read that frame with one thing in mind.** It is printed by the test harness,
not captured from a fresh run, and `coordinator-7` is a second speaker the
shipped binary cannot yet produce. `go run ./cmd/tldr` gives you exactly one
speaker — you — and you would have to type thirteen messages to trigger that
first fold. Wiring a locally-run model in as a real second voice is the current
work. Everything else below is exactly what the program draws.

That scar is where most systems stop, and it is the point at which you are
asked to trust a summary. Press `ctrl+u`:

```
tldr                                                         view 12 · record 35
────────────────────────────────────────────────────────────────────────────────
┌─ 21 bits · 19:38–19:38 · backfill staging migration production ── ctrl+u ──
│  1/21  b52d42e5  19:38  me             starting the migration on the auth ser…
│  2/21  c61abd07  19:38  coordinator-7  acknowledged, standing by for the sche…
│  3/21  a3a26e10  19:38  me             schema dump is 40MB, uploading now
│  4/21  29521ccd  19:38  coordinator-7  got it — running the diff against stag…
│  5/21  05c1dd2e  19:38  me             three columns drift: created_at, updat…
│  6/21  0699ee60  19:38  coordinator-7  those are the soft-delete columns nobo…
│  7/21  71a0ea38  19:38  me             do we backfill or drop them
│  8/21  0dad3efc  19:38  coordinator-7  backfill. dropping loses the audit tra…
│  9/21  03c46092  19:38  me             agreed, writing the backfill migration…
│ 10/21  45a9dbbb  19:38  coordinator-7  heads up: the staging box is at 90% di…
│ 11/21  838617f8  19:38  me             pausing the upload until that clears
───────────────────────────────────────────────────────────── ↓ 22 more · pgdn ─
› say something
›
›
enter send · ctrl+k fold · ctrl+u unfold · ctrl+c quit        ▓▓▓▓▓▓▓▓▓▓▓░ 11/12
```

Nothing was cached to make that happen. Each row is looked up in the store at
the moment it is drawn, by the content hash in the second column, and the
ordinal on every row (`7/21`) means you can check the count from any single row
still on screen rather than having to scroll to an end. Press `ctrl+u` again and
the scar closes. The view forgot; the record did not.

**The two numbers reconcile, and it is worth doing once.** Thirty-two messages
were sent. Twenty-one are behind the scar and eleven are still on screen — that
is the thirty-two, and the scar itself accounts for the remaining view slot. The
other three in the record are the folds' own output: this scar is the third one,
and a fold is itself a stored object, so it is counted like everything else.
32 + 3 = 35.

Run `tui/harness_test.go` yourself and every count above will match. The
timestamps and content hashes will not — they are derived from when you run it,
which is what content addressing means.

## The bet

Storage is content-addressed and append-only. A fold does not remove anything —
it derives a *new* object that takes the display slot, and links to every bit it
absorbed. So the record is a graph you can walk, and "we still have it" is a
property you can test rather than a promise in a README.

The tests pin exactly that: `memory/reach_test.go` walks the graph from the view
and asserts that every stored bit is reachable. It is the test that caught us
violating our own headline guarantee — see below.

## Running it

```
go test ./...
go run ./cmd/tldr
HARNESS=1 go test ./tui/ -run TestHarness -v    # print frames like the ones above
```

Go 1.25.4, Bubble Tea v2 / Lip Gloss v2. A pre-commit gate lives in `.githooks/`;
a fresh clone needs `git config core.hooksPath .githooks` once.

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

Early, and the honest list is short. **Nothing but you can speak yet** — the
second handle in those frames comes from the test harness, and feeding bits in
from a running model is unbuilt (D18). **No persistence** — the store is
in-memory, so quitting ends the record. No ranking, no voting, no multiple
communities, no release. `CLAUDE.md` keeps a current list of known defects,
including three in the screen above that we have named and not yet fixed.

The decision log publishes D1–D7, D11–D14, D18 and D19. Nine entries are
withheld — three commercial, six about publication and staffing — and the
numbering is left intact rather than closed up, so the gaps are visible instead
of tidied away.

No license, so the default applies: read it and learn from it, but ask before
depending on it.
