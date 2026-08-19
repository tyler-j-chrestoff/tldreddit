#!/bin/sh
# SessionStart hook. Injects what a new instance cannot get right by reasoning:
# what the last session actually finished, and where the tree actually is.
#
# CLAUDE.md already *tells* the next instance to read the handoff. That is an
# instruction, and instructions are heuristic — they compete with whatever the
# user opens the session by asking about. This is not an instruction. It is
# state, already in context before the first prompt is read.
#
# It must fail loudly. A continuity hook that silently injects nothing is worse
# than no hook, because the session that needed it will not know it was missing.
# Every failure path below therefore still emits context, saying what broke.
set -eu

# Static, pre-escaped so it needs no JSON encoder to report the encoder missing.
if ! command -v python3 >/dev/null 2>&1; then
	printf '%s' '{"hookSpecificOutput":{"hookEventName":"SessionStart","additionalContext":"SESSION START HOOK FAILED: python3 not found, so no state could be injected. Read CLAUDE.md and the newest file in docs/handoffs/ before assuming anything about where work stopped."}}'
	exit 0
fi

emit() {
	python3 -c 'import json,sys; print(json.dumps({"hookSpecificOutput":{"hookEventName":"SessionStart","additionalContext":sys.stdin.read()}}))'
}

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$root" || exit 0

if [ ! -f CLAUDE.md ]; then
	printf 'SESSION START HOOK ran outside the project (resolved to %s) and injected no state. Read CLAUDE.md and docs/handoffs/ manually.' "$root" | emit
	exit 0
fi

# Handoffs do not live in this repository. D81 split the trees by content: the
# work publishes from here, and the handoffs — which name competitor reads and
# burn figures — stay in the context repository beside docs/PRIVATE.md. So this
# looks there first and falls back to a local docs/handoffs/ if one ever exists,
# which keeps a clone of this repo alone from silently reporting no handoffs as
# though the convention had broken. Override the location with TLDR_CONTEXT.
context=${TLDR_CONTEXT:-$root/../tldreddit-context}
if [ -d "$context/docs/handoffs" ]; then
	handoffs="$context/docs/handoffs"
elif [ -d docs/handoffs ]; then
	handoffs=docs/handoffs
else
	handoffs=""
fi

# Guarded by an `if` rather than `[ -n "$handoffs" ] &&`, because under `set -e`
# an assignment whose command substitution exits non-zero kills the script — which
# it did, silently, for every case where no handoffs directory was found. That made
# the NO HANDOFF FOUND branch below unreachable: the one path this hook exists to
# report was the one path that emitted nothing at all.
handoff=""
if [ -n "$handoffs" ]; then
	handoff=$(find "$handoffs" -name '*.md' -type f 2>/dev/null | sort | tail -1)
fi

# D28's invariant is that reading the newest file is *sufficient*, and this line
# is the only thing that decides which file that is. It sorts by name, so the
# convention only holds while filename order equals time order — which it stopped
# doing at session 10, where `-session-10.md` sorts before `-session-6.md` and the
# hook silently served a handoff one session stale. Names are zero-padded now, and
# this is the check that says so out loud if it ever drifts again: newest-by-name
# and newest-by-mtime must be the same file. A disagreement is not fatal — mtime
# moves for reasons that are not authorship — so it is reported beside the handoff
# rather than swallowed or thrown, and the reader decides.
handoff_warning=""
newest_mtime=""
if [ -n "$handoffs" ]; then
	newest_mtime=$(find "$handoffs" -name '*.md' -type f -printf '%T@ %p\n' 2>/dev/null | sort -n | tail -1 | cut -d' ' -f2-)
fi
if [ -n "$handoff" ] && [ -n "$newest_mtime" ] && [ "$handoff" != "$newest_mtime" ]; then
	handoff_warning="WARNING: the newest handoff by filename ($handoff) is not the
newest by modification time ($newest_mtime). One of them is not the last ending.
Filename order is the convention D28 relies on; check both before trusting either.

"
fi

if [ -z "$handoff" ]; then
	body="NO HANDOFF FOUND. Looked in ${handoffs:-<no handoffs directory found>};
the context repository is expected at \$TLDR_CONTEXT or ../tldreddit-context (D81).

Either this is the first session, the context repository is not checked out
beside this one, or the handoff convention broke. Do not assume continuity —
say so to Tyler rather than guessing where work stopped."
else
	body="$handoff_warning=== MOST RECENT HANDOFF: $handoff ===

$(cat "$handoff")"
fi

remotes=$(git remote -v 2>/dev/null | head -1)
[ -z "$remotes" ] && remotes="none — nothing has been published. Read D15 before changing that."

state="branch:       $(git branch --show-current 2>/dev/null || echo '?')
HEAD:         $(git log -1 --format='%h %s' 2>/dev/null || echo '?')
uncommitted:  $(git status --porcelain 2>/dev/null | wc -l | tr -d ' ') file(s)
remote:       $remotes"

printf '%s

=== LIVE GIT STATE AT SESSION START ===
%s

Injected by .claude/session-start.sh — read from disk, not remembered. The git
state is current as of this moment; the handoff is current as of when it was
written. Where they disagree, the git state is right.' "$body" "$state" | emit
