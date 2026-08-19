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

handoff=$(find docs/handoffs -name '*.md' -type f 2>/dev/null | sort | tail -1)

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
newest_mtime=$(find docs/handoffs -name '*.md' -type f -printf '%T@ %p\n' 2>/dev/null | sort -n | tail -1 | cut -d' ' -f2-)
if [ -n "$handoff" ] && [ -n "$newest_mtime" ] && [ "$handoff" != "$newest_mtime" ]; then
	handoff_warning="WARNING: the newest handoff by filename ($handoff) is not the
newest by modification time ($newest_mtime). One of them is not the last ending.
Filename order is the convention D28 relies on; check both before trusting either.

"
fi

if [ -z "$handoff" ]; then
	body="NO HANDOFF FOUND under docs/handoffs/.

Either this is the first session, or the handoff convention broke. Do not assume
continuity — say so to Tyler rather than guessing where work stopped."
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
