# Publishing

**Read this from the private tree's chair.** It is the procedure that produces
the public tree out of the private one, and it publishes into the tree it
describes — so throughout, "this repository" means the *private* one, and every
command here is written to be run there. A reader who found this file in the
public tree is holding the recipe rather than the kitchen: the commands will
not resolve, because the paths they compare against are the ones that stay
behind. It ships anyway, because a publication rule nobody outside can read is
the thing D78 was written against.

The public tree is `/home/tyler/code/tldreddit-public` — a separate
repository, own history, no shared objects with this one. Chosen over an
orphan branch because an orphan branch shares an object database and one
`git push --all` mails the private history to a public host.

## The rule

**`docs/PRIVATE.md` never leaves. Everything else publishes, every
checkpoint.**

That is all of it. There is no per-entry ruling, no manifest of withheld
numbers, no backlog of entries awaiting a decision. If something belongs in
`docs/PRIVATE.md`, put it there when you write it — a decision entry that
would mix private and public content gets split by the person who knows why,
once, instead of by a reviewer re-deriving the boundary forever.

**Four things do not reach the public tree, and none of them is a per-file
ruling — each is the `docs/PRIVATE.md` test or the hash sweep, applied.** `docs/board.html`
carries burn and revenue figures, so it is `docs/PRIVATE.md` material by
definition; `.claude/craft/tui-design-engineer.md` carries competitor reads,
same test, and the craft records stay together rather than being split
record-by-record. `docs/handoffs/` fails the private-hash sweep in bulk — of the 150 distinct
hex tokens in that directory, **109 resolve as commits here and not in the
public repository, naming 96 distinct commits** once short and long forms are
collapsed (measured 2026-08-18; the command is `grep -rohE '\b[0-9a-f]{7,40}\b'
docs/handoffs | sort -u`, each token tested with `git cat-file -e "$h^{commit}"`
in both trees) — and
stripping them every checkpoint forever would cost more than the files are
worth to a reader who has the decisions and the code. And
`docs/archive/PUBLICATION-2026-08.md` is the record of *what was withheld and
why*, which is an index of private material and stays with it.

Everything else goes, including the things it is easy to forget are files:
this document, `.claude/settings.json` and `.claude/session-start.sh` (whose
hook looks for `docs/handoffs/`, does not find it in the public tree, and
says so in the context it injects rather than failing silently — it exits 0),
and the whole of `.claude/agents/`. The public tree also carries a `README.md` that
this one does not, which is the one divergence that runs the other way.

The list above is the whole of it, and it is closed by a tripwire rather than
by good intentions: **when a fifth path wants to join it, that is the signal
the apparatus is regrowing**, and the answer is to fix the cause — move the
private material to `docs/PRIVATE.md`, or strip the hashes — rather than
lengthen the paragraph. Re-derive the list at any time with
`comm -23 <(git ls-files | sort) <(git -C ../tldreddit-public ls-files | sort)`;
anything it prints that is not named here is drift, not policy. (Run before
the push that first publishes this file, it also prints `docs/PUBLICATION.md`,
`.claude/settings.json` and `.claude/session-start.sh` — the three the
paragraph above adds to the public tree. That resolves with the push and is
not an exception.)

## Before a push

1. **Sync code one way**, private → public, then `diff -r` the packages and
   confirm they are identical. Code is never edited in the public tree.
2. **Sweep for private commit hashes.** Any 7–40 character hex that resolves
   in this repository and not in the public one comes out.
3. **Sweep whole files, not diffs, wherever a file has drifted.** A file that
   has not been synced for weeks is not a small change; it is a large one
   arriving at once, and every check that compares *what changed* is blind to
   what was already there. This step exists because it was skipped: three
   private hashes shipped inside `docs/CODE.md` on the day the rule above
   replaced the old apparatus.
4. **Read the produced tree, not the plan.** Cuts leave aftermath — a heading
   promising a paragraph that is gone, a "what would change this" naming a
   clause that no longer exists, a tail note that outlives its own clause.
   None of that is visible in the plan for the cut.

## Append-only

The public history is append-only from its root. A mistake already pushed is
fixed by a new commit, never an amend or a force-push — including a leak,
because rewriting public history to remove a string costs more than the
string does. A problem found in a local commit that has not been pushed is
fixed by amending it; the point of gating before a push is that nothing is
permanent until it is.

## Entries not published

**Forward, from D78: an entry is written so that it can publish whole.** If it
would carry `docs/PRIVATE.md` material, the author splits it at the moment of
writing — figure to the private file, reasoning to the entry. There is no
per-entry gate and no ruling to make later; the split already happened.

**Backward: twenty-four entries were withheld before that rule existed, and
they stay withheld as history rather than as a backlog (D79(e)).** Eleven of
them were screened and hold nothing secret. They still do not ship, because
publishing them verbatim would put five sentences into the public tree that
are false *in that tree* — two of them instructions saying the entry the
reader is reading is private — plus about twenty-five `file:line` citations
that resolve there to unrelated prose. Correcting that means editing history,
which this record does not do. **What reopens one is a person asking for it**,
and then it publishes with a correction appended.

Twelve of the twenty-four are held on content, and the grounds, corrected at
D79(f), are: **D8, D9, D10, D15, D25 and D56** carry the commercial mandate,
the business position or a competitor read; **D62** quotes the burn and revenue position verbatim;
**D65 and D70** are itemized indexes of what was cut from which entry, the
same ground the archive is held on; **D41** carries a third party's
safety-critical material, which is a higher class than business material and
is why it is named separately; **D43 and D44** survey the founder's own
machine. **D64** has no ground of its own left — D78 dissolved the one it
had — so it is held only by the backward rule above. The other eleven are the
ones D79(e) screened clean.

The gaps in the public log's numbering announce themselves, and a note at the
end of that file says so. An entry that decided two things where only one is
publishable ships with the other conjunct dropped from its title, and the
public `CLAUDE.md` list says that is what happened — a legacy shape from
before the split-at-write-time rule, not something to do again.

## What this replaced

A 1,161-line manifest holding a per-entry ruling record, a withheld backlog
nobody had drained, and four decision entries that were themselves withheld
for describing the mechanism. It is kept at
`docs/archive/PUBLICATION-2026-08.md` — not as an operating document, but
because the rulings in it are the record of what was withheld and why, and
losing that is how something gets published twice by accident. **It covers
D1 through D68 and no further** — `for n in $(seq 69 79); do printf '%s %s\n' \
"$n" "$(grep -c "D$n" docs/archive/PUBLICATION-2026-08.md)"; done` returns
zero for every number except D70, whose sixteen hits are D70's own corrections
written *into* the older rulings rather than a ruling on D70. So the eleven
most recent decisions, which are exactly the ones a next push touches, have no
ruling written there. Do not read the archive as covering the present. Full
reasoning: D78, and D79 for what the cut left behind.
