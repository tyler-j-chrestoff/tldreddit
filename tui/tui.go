// Package tui is the human surface onto a [memory] record.
//
// The organizing idea: a harness that forgets silently teaches you to stop
// trusting it. So this one shows its own memory working. Bits about to be
// folded away fade before they go, folds leave a visible scar with a receipt,
// and a gauge shows how close the next fold is. The machine still does the
// work — the human is never asked to manage memory by hand — but nothing
// happens behind their back, so their judgement stays in the loop.
package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/tyler-j-chrestoff/tldreddit/memory"
)

const (
	// coolAt is how many hot bits the record holds before folding.
	coolAt = 12

	// keepHot is how many stay hot afterwards. Folding everything would leave
	// nothing legible on screen; a record with no hot tail is a filing
	// cabinet, not a conversation.
	keepHot = 6

	// channel is the only channel this surface speaks on. Cool refuses to fold
	// across channels, and that guard is load-bearing rather than fussy.
	channel = "tui"

	// chrome is the row budget for everything that is not the transcript:
	// header, two rules, composer, footer.
	chrome = 8

	// gaugeWidth is the widest the pressure bar is drawn. It shrinks with the
	// terminal rather than being dropped, because the gauge is the antecedent
	// for a fold that fires on its own, and an automatic operation with no
	// visible antecedent is the thing this surface exists to prevent.
	gaugeWidth = 12
)

// Model is the whole application state.
type Model struct {
	// store is the record and it only grows. It is a pointer, so every copy of
	// this Model shares it — which is safe only because a content-addressed
	// store is append-only: a copy can see bits a sibling added, never a bit
	// that changed under it.
	store *memory.Store

	// shown is the view: which bits are on screen and in what order. It is a
	// value, so Update stays a pure function of the Model it was handed, and
	// folding a copy cannot fold the original.
	shown memory.View

	// unfolded is whether the scars on screen are currently showing what they
	// stand for. It is display state and nothing else: no bit, no address and
	// no order is decided here, so the record and the view read the same either
	// way. If this field could change what m.shown holds it would be an undo
	// button, and there is no undoing a fold — only following it.
	unfolded bool

	viewport viewport.Model
	composer textarea.Model

	width, height int
}

// New returns a Model ready to run.
func New() Model {
	ta := textarea.New()
	ta.Placeholder = "say something"
	ta.Prompt = "› "
	ta.CharLimit = 4000
	ta.ShowLineNumbers = false
	ta.SetVirtualCursor(false)
	ta.SetHeight(3)
	ta.Focus()

	// The cursor line highlight fights the fade, which is the one signal this
	// UI cannot afford to lose.
	s := ta.Styles()
	s.Focused.CursorLine = lipgloss.NewStyle()
	ta.SetStyles(s)

	// Enter sends. A composer that swallows Enter for newlines makes the
	// common case cost two keys to save the rare one.
	ta.KeyMap.InsertNewline.SetEnabled(false)

	// The transcript scrolls on keys the composer cannot want. Left and right
	// belong to the cursor; the bare letters the viewport binds by default
	// (u, d, f, b, j, k, space) belong to whatever the human is typing, and
	// binding them here means a message with a space in it scrolls the record
	// out from under its author. Half-page is dropped rather than rebound:
	// ctrl+u is the key that reaches the record now, and two meanings for one
	// key is one meaning too many.
	vp := viewport.New()
	vp.KeyMap.Left.SetEnabled(false)
	vp.KeyMap.Right.SetEnabled(false)
	vp.KeyMap.HalfPageUp.SetEnabled(false)
	vp.KeyMap.HalfPageDown.SetEnabled(false)
	vp.KeyMap.Up = key.NewBinding(key.WithKeys("up"))
	vp.KeyMap.Down = key.NewBinding(key.WithKeys("down"))
	vp.KeyMap.PageUp = key.NewBinding(key.WithKeys("pgup"))
	vp.KeyMap.PageDown = key.NewBinding(key.WithKeys("pgdown"))

	m := Model{store: memory.NewStore(), composer: ta, viewport: vp, width: 80, height: 24}
	m.layout()
	m.sync()
	return m
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.layout()
		m.sync()

	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "enter":
			m.send()
			return m, nil
		// ctrl+k and ctrl+u are a pair on purpose: adjacent keys for opposite
		// directions on the same object, which is a thing a hand learns once.
		// Both are taken from the composer, where they are readline's kill
		// bindings — a fair trade, since this program's whole subject is what
		// leaves the screen and readline's is what leaves a line.
		//
		// Returning here rather than falling through is what takes them: the
		// composer and the viewport both see every message that reaches the
		// bottom of this function.
		case "ctrl+k":
			m.fold()
			return m, nil
		case "ctrl+u":
			m.unfold()
			return m, nil
		}
	}

	var taCmd, vpCmd tea.Cmd
	m.composer, taCmd = m.composer.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)
	return m, tea.Batch(taCmd, vpCmd)
}

func (m Model) View() tea.View {
	// Two numbers, and the gap between them is the product. The view is what
	// this screen is showing; the record is everything that has ever been put
	// in the store, derived bits included. They start equal and diverge at the
	// first fold, which is the moment worth noticing — and they teach the two
	// words the unfolded block then uses for itself.
	//
	// The count of hot bits moved to the gauge in the footer, where the
	// pressure it belongs to already lives. The count of cold ones was never
	// more than one and is on screen as the scar.
	//
	// Both numbers shrink rather than run off the edge. The header was the last
	// row on this screen still built to a width it had not been told, and at
	// twenty columns it read "view 12 · recor" — a truncation the terminal did,
	// not the program, which is the one kind this surface is not allowed.
	counts := fit(max(m.width-lipgloss.Width("tldr")-1, 1),
		fmt.Sprintf("view %d · record %d", len(m.shown), m.store.Len()),
		fmt.Sprintf("%d · %d", len(m.shown), m.store.Len()),
		"")
	header := m.spread(dim.Render("tldr"), dim.Render(counts))

	above, below := m.offscreen()

	// One exit path, so AltScreen is set once. The renderer diffs each frame
	// against the last, and a branch that forgot it would silently drop out of
	// the alternate screen and scribble over the shell.
	v := tea.NewView(lipgloss.JoinVertical(lipgloss.Left,
		header,
		m.edge("↑", "pgup", above),
		m.viewport.View(),
		m.edge("↓", "pgdn", below),
		m.composer.View(),
		m.footer(),
	))
	v.AltScreen = true
	return v
}

// offscreen is how many transcript rows sit above and below what is drawn.
//
// The transcript has always been taller than its frame — a full hot band plus a
// scar is thirteen rows in the twelve an eighty-by-twenty terminal leaves — and
// until now nothing on screen said so. Unfolding made it unmissable rather than
// new: the block a receipt opens is one row per absorbed bit, so past one fold
// it is taller than any common terminal, and its closing bar sat below the
// margin with the screen looking finished.
func (m Model) offscreen() (above, below int) {
	above = m.viewport.YOffset()
	below = max(m.viewport.TotalLineCount()-above-m.viewport.Height(), 0)
	return above, below
}

// edge draws one of the two rules that bracket the transcript, carrying how
// much is past it in that direction and the key that goes there.
//
// It costs no rows, which is the whole reason it is here rather than as a line
// of its own: at eighty by twenty there are no rows to spare, and an indicator
// that only appears on a big terminal is an indicator for the case that did not
// need it. A plain rule now means something it could not mean before — that
// what is between the two rules is all there is.
//
// The key rides on the indicator for the same reason the scar carries ctrl+u: a
// footer is where a returning user looks and the edge is where someone who has
// just noticed there is more is already looking.
func (m Model) edge(arrow, key string, n int) string {
	w := max(m.width, 1)
	if n <= 0 {
		return rule.Render(strings.Repeat("─", w))
	}

	// The count survives every cut. It is the whole claim: this many rows are
	// past this line, and the screen is not finished.
	tag := fit(w,
		fmt.Sprintf(" %s %d more · %s ─", arrow, n, key),
		fmt.Sprintf(" %s %d more ─", arrow, n),
		fmt.Sprintf(" %s %d ─", arrow, n),
		fmt.Sprintf("%s%d", arrow, n))
	return rule.Render(strings.Repeat("─", max(w-lipgloss.Width(tag), 0)) + tag)
}

// footer is the pressure on the record, and an index of the keys.
//
// When there is not room for both, the gauge wins and the help is cut. The
// gauge is the antecedent for something the machine does without being asked;
// the help is a convenience, and the one key it names that a person cannot
// guess is already printed on the scar it operates.
func (m Model) footer() string {
	g := gauge(len(m.hot()), coolAt, min(gaugeWidth, max(m.width/4, 1)))
	room := m.width - lipgloss.Width(g) - 1

	// Unfold is the last key standing. Folding happens on its own, so ctrl+k is
	// only a shortcut; quitting is ctrl+c everywhere; sending is enter
	// everywhere. Following a receipt into the record is the one thing here
	// nobody can guess, so it is the one thing worth the columns.
	help := fit(room,
		"enter send · ctrl+k fold · ctrl+u unfold · ctrl+c quit",
		"enter send · ctrl+u unfold · ctrl+c quit",
		"ctrl+u unfold · ctrl+c quit",
		"ctrl+u unfold",
		"")
	return m.spread(dim.Render(help), g)
}

// spread puts left and right on one row, pushing right to the margin, and cuts
// the result if the two together will not fit.
func (m Model) spread(left, right string) string {
	gap := max(m.width-lipgloss.Width(left)-lipgloss.Width(right), 1)
	return clip(left+strings.Repeat(" ", gap)+right, m.width)
}

// send records what is in the composer as a new bit under the local handle.
func (m *Model) send() {
	text := strings.TrimSpace(m.composer.Value())
	if text == "" {
		return
	}
	m.composer.Reset()
	m.say(memory.Handle{Ref: "local", Display: "me"}, text)
}

// say records text from h as a new bit, and folds if that pushed the hot band
// over the limit. Folding on write rather than on a timer keeps the program
// free of background state: pressure only ever changes here.
//
// The handle is a parameter rather than a constant because the human is not the
// only speaker for much longer. Every other part of this surface already treats
// the handle as data — the scar merges them, the unfolded block aligns them —
// and this was the last place that assumed there was one.
func (m *Model) say(from memory.Handle, text string) {
	m.shown, _ = m.shown.Add(m.store, memory.Bit{
		At:      time.Now(),
		From:    from,
		Channel: channel,
		Payload: memory.Utterance{Text: text},
		Prev:    m.shown.Head(),
	})

	if len(m.hot()) > coolAt {
		m.fold()
	}
	m.sync()
}

// fold takes everything but the most recent keepHot bits off the screen and
// puts one cold bit in their place. Only the screen changes: the bits it
// absorbed are still in the store, still addressed the same way, so the scar
// the transcript draws has something behind it.
//
// The one new edge is the cold bit's own: it names every bit it absorbed, so
// the way back into what left the screen is on the record and not only on the
// receipt. Nothing existing is repointed. The surviving bits go on naming
// absorbed bits in their Prev, and that is correct rather than dangling — the
// graph the record keeps is the graph as it happened, and shortening it was
// only ever necessary because folding used to delete.
func (m *Model) fold() {
	shown, folded := m.shown.Fold(m.store, keepHot)
	if !folded {
		return
	}
	m.shown = shown

	// A fold closes an open unfold, so the collapse is something you watch
	// happen. Folding while the screen stayed full would be the machine doing
	// the one operation this surface exists to make visible, invisibly. The
	// material is one key away and the key is on the new scar.
	m.unfolded = false
	m.sync()
}

// unfold shows, or stops showing, what the scars on screen stand for.
//
// A toggle, not a mode and not a drill-down. The same key opens and closes, so
// there is no state to be stranded in and nothing to learn beyond "press it
// again." It applies to every scar in the view at once — today at most one —
// because "show me what these lines stand for" is a sentence a person already
// holds, while "show me what the third one stands for" needs a cursor, a
// selection highlight, and a legend explaining both.
//
// Neither the record nor the view changes here. No bit is stored, none is put
// back on screen in the sense of the view holding it again, and m.shown is not
// touched. The transcript resolves the receipt while it draws and drops the
// result on the next press.
func (m *Model) unfold() {
	if m.scars() == 0 {
		// Nothing to follow, so nothing happens. Flipping the flag anyway would
		// arm the next fold to arrive already open, and the fold would then
		// collapse the screen without the screen appearing to collapse.
		return
	}

	m.unfolded = !m.unfolded
	m.draw()

	// The scar sits at the top of the view, so opening it puts the retrieved
	// material off the top of a viewport pinned to the newest line. A key that
	// appears to do nothing has done nothing, as far as the person is
	// concerned.
	if m.unfolded {
		m.viewport.GotoTop()
	} else {
		m.viewport.GotoBottom()
	}
}

// scars counts the folds currently in the view.
func (m Model) scars() int {
	n := 0
	for _, b := range m.shown.Bits(m.store) {
		if _, cold := b.Payload.(memory.Compaction); cold {
			n++
		}
	}
	return n
}

// hot returns the uncompacted tail of the view.
func (m Model) hot() memory.View {
	for i, b := range m.shown.Bits(m.store) {
		if _, cold := b.Payload.(memory.Compaction); !cold {
			return m.shown[i:]
		}
	}
	return nil
}

// fadeBefore is the index before which bits will be absorbed by the next fold.
//
// It has to answer for the fold that is actually coming, not for one happening
// this instant. send appends and then tests the band, so the fold's cut is
// computed against a view one longer than the one last drawn — and the old
// arithmetic, len(shown)-keepHot, therefore missed by exactly one every time.
// One bit per fold went from full brightness to absorbed with no frame in
// between, in a surface whose entire claim is that you watch heat leave before
// it goes.
//
// The fold fires on the send that pushes the hot band to coolAt+1, which is
// coolAt+1-len(hot) sends from here. The view only ever grows at the end, so
// the indices it will cut are the same indices now:
//
//	cut = (len(shown) + coolAt + 1 - len(hot)) - keepHot
//
// Clamped to the view, because early on that reaches past the end: right after
// a fold every row on screen is destined for the next one, and saying so is the
// honest reading rather than an alarming one. The gauge in the footer carries
// how soon; this carries which.
//
// It over-predicts for ctrl+k, which cuts to keepHot immediately and so absorbs
// a subset of what is faded. That direction is the safe one, and it is the
// property worth having in one sentence: nothing is ever absorbed that was not
// drawn cooling first.
func (m Model) fadeBefore() int {
	cut := len(m.shown) + coolAt + 1 - len(m.hot()) - keepHot
	return min(max(cut, 0), len(m.shown))
}

// draw rebuilds the transcript without moving the viewport.
func (m *Model) draw() {
	m.viewport.SetContent(transcript(m.store, m.shown.Bits(m.store), m.fadeBefore(), m.width, m.unfolded))
}

// sync redraws the transcript and returns to the newest material. Every
// mutation ends here, so the viewport can never disagree with the record.
func (m *Model) sync() {
	m.draw()
	m.viewport.GotoBottom()
}

func (m *Model) layout() {
	m.composer.SetWidth(m.width)
	m.viewport.SetWidth(m.width)
	m.viewport.SetHeight(max(m.height-chrome, 1))
}
