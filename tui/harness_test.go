// This file asserts nothing. It prints frames.
//
// Taste does not belong in a test, and an assertion about how a screen looks is
// taste wearing a lab coat: it locks in whatever the last person thought looked
// right and makes the next person argue with a diff instead of with a picture.
// So everything here is skipped unless HARNESS is set, and everything here
// writes to stdout rather than to t.
//
//	HARNESS=1 go test ./tui/ -run TestHarness -v
//
// What it is for is the other half of the job: looking. The defects this exists
// to catch — a handle silently shortened to collide with another, a block that
// runs off the bottom with the screen looking finished, a row built to a width
// nobody told it — are all invisible in a passing test suite and obvious in
// forty lines of rendered output. Every real property they turned out to stand
// for is asserted in tui_test.go, where it belongs.
package tui

import (
	"fmt"
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/tyler-j-chrestoff/tldreddit/memory"
)

// screen renders m at its current size the way a terminal would: ANSI stripped,
// every row clipped to width, boxed so the right margin is visible. A "~" in
// the right margin marks a row drawn in the cooling style.
func screen(m Model, label string) string {
	rows := strings.Split(m.View().Content, "\n")

	var b strings.Builder
	fmt.Fprintf(&b, "── %s · %dx%d %s\n", label, m.width, m.height, strings.Repeat("─", 16))
	b.WriteString("┌" + strings.Repeat("─", m.width) + "┐\n")
	for _, r := range rows {
		plain := ansi.Truncate(ansi.Strip(r), m.width, "")
		fade := " "
		if strings.Contains(r, "38;5;242m") {
			fade = "~"
		}
		fmt.Fprintf(&b, "│%s%s│%s\n", plain,
			strings.Repeat(" ", max(m.width-ansi.StringWidth(plain), 0)), fade)
	}
	b.WriteString("└" + strings.Repeat("─", m.width) + "┘\n")
	return b.String()
}

func sized(w, h int) Model {
	m := New()
	mm, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return mm.(Model)
}

var lines = []string{
	"starting the migration on the auth service",
	"acknowledged, standing by for the schema dump",
	"schema dump is 40MB, uploading now",
	"got it — running the diff against staging",
	"three columns drift: created_at, updated_at, deleted_at",
	"those are the soft-delete columns nobody backfilled",
	"do we backfill or drop them",
	"backfill. dropping loses the audit trail",
	"agreed, writing the backfill migration now",
	"heads up: the staging box is at 90% disk",
	"pausing the upload until that clears",
	"cleared, 40% free after the log rotation",
	"resuming, ETA four minutes",
	"backfill migration is up for review",
	"reviewing — the null default worries me",
	"switching it to an explicit epoch timestamp",
	"that reads better, approving",
	"merged and deploying to staging",
	"staging is green across the board",
	"promoting to production in ten minutes",
	"production deploy started",
	"production is green, migration complete",
	"writing the postmortem note",
	"nothing to post-mortem, it went clean",
	"still worth a note for the next person",
	"fair. filing it under runbooks",
	"closing the incident channel",
	"thanks everyone",
	"one more thing: the disk alert threshold",
	"raise it to 80% so we get more warning",
	"filed as a follow-up ticket",
	"done for the day",
}

// talk sends n bits alternating between a human and a model's persona handle.
func talk(m Model, n int) Model {
	handles := []memory.Handle{
		{Ref: "local", Display: "me"},
		{Ref: "ollama/llama3", Display: "coordinator-7"},
	}
	for i := range n {
		m.say(handles[i%len(handles)], lines[i%len(lines)])
	}
	return m
}

func page(m Model, n int) Model {
	for range n {
		mm, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
		m = mm.(Model)
	}
	return m
}

func TestHarness(t *testing.T) {
	if os.Getenv("HARNESS") == "" {
		t.Skip("set HARNESS=1")
	}
	out := os.Stdout

	for _, size := range [][2]int{{80, 20}, {100, 30}} {
		m := talk(sized(size[0], size[1]), 32)
		fmt.Fprint(out, screen(m, "closed"))
		m.unfold()
		fmt.Fprint(out, screen(m, "unfolded, at the top"))
		fmt.Fprint(out, screen(page(m, 1), "unfolded, one page down"))
		fmt.Fprint(out, screen(page(m, 2), "unfolded, two pages down"))
	}
}

func TestHarnessNarrow(t *testing.T) {
	if os.Getenv("HARNESS") == "" {
		t.Skip("set HARNESS=1")
	}
	out := os.Stdout

	m := talk(sized(80, 20), 32)
	for _, w := range []int{40, 24, 16, 8, 1} {
		fmt.Fprintf(out, "── transcript at width %d ──\n%s\n%s\n", w, strings.Repeat("·", w),
			ansi.Strip(transcript(m.store, m.shown.Bits(m.store), m.fadeBefore(), w, true)))
	}

	c := cooled(t, "the deploy failed", "deploy again", "and again")
	for _, w := range []int{80, 40, 30, 20, 16} {
		fmt.Fprintf(out, "── unresolvable receipt at width %d ──\n%s\n%s\n", w,
			strings.Repeat("·", w), ansi.Strip(unfold(memory.NewStore(), c, w)))
	}
}

func TestHarnessShort(t *testing.T) {
	if os.Getenv("HARNESS") == "" {
		t.Skip("set HARNESS=1")
	}
	for _, h := range []int{12, 10, 9, 8, 1} {
		m := talk(sized(60, h), 32)
		m.unfold()
		fmt.Fprint(os.Stdout, screen(m, "unfolded"))
	}
}
