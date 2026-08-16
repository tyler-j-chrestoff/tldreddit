package tui

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/tyler-j-chrestoff/tldreddit/memory"
	"github.com/tyler-j-chrestoff/tldreddit/persona"
)

// Nothing in this file may reach a network, and there are two separate reasons
// for that rather than one. Almost every test here leaves the command send
// returns unrun — the runtime is what runs it, and there is no runtime in a
// unit test. Exactly one test runs it deliberately, and that one uses this
// client: persona.Reply checks its own BaseURL with usable() and rejects an
// address with no scheme before it builds a request, so the failure is a
// *persona.Error and not a connection attempt. A suite that needed ollama
// running would pass here and fail on every other machine, which is the same
// defect as a claim nobody re-derived.
func offline() *persona.Client { return &persona.Client{BaseURL: "not-a-url"} }

// The first send records the question and asks. The second, while that is still
// in flight, does neither — and leaves the draft in the composer, because the
// words are the person's and losing them would be a worse answer than holding
// them.
func TestSendAsksAndHoldsTheNext(t *testing.T) {
	m := New()

	m.composer.SetValue("first")
	cmd := m.send()
	if cmd == nil {
		t.Fatal("send returned no command, so nothing was asked")
	}
	if !m.waiting.live {
		t.Fatal("send did not raise the pending state")
	}
	if got := len(m.shown); got != 1 {
		t.Fatalf("view holds %d bits after one send, want 1", got)
	}

	m.composer.SetValue("second, while the first is still out")
	if cmd := m.send(); cmd != nil {
		t.Error("a second send while a reply was in flight asked anyway")
	}
	if got := len(m.shown); got != 1 {
		t.Errorf("view holds %d bits, want 1 — the held send was recorded", got)
	}
	if m.composer.Value() == "" {
		t.Error("the held send emptied the composer, so the person's words are gone")
	}
}

// Enter is the key a person presses to say something, and until this test
// nothing in this package drove it through Update: every send test calls
// m.send() directly. esc was covered and enter was not.
//
// It is also where the Model that comes back is checked. send has a pointer
// receiver and writes to Update's own copy, so the Model returned has to be the
// one it wrote to — "return m, m.send()" left that to an evaluation order the
// spec does not fix, and it happened to work.
func TestEnterSendsThroughUpdate(t *testing.T) {
	m := New()
	m.ollama = offline()
	m.composer.SetValue("what happened to the migration")

	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	after := next.(Model)

	if cmd == nil {
		t.Fatal("enter produced no command, so nothing was asked")
	}
	if got := len(after.shown); got != 1 {
		t.Errorf("view holds %d bits after enter, want 1 — the Model that came back is not the one send wrote to", got)
	}
	if !after.waiting.live {
		t.Error("enter did not raise the pending state")
	}
	if got := after.composer.Value(); got != "" {
		t.Errorf("composer still holds %q after enter, so the send never happened", got)
	}
}

// A pending state that survives one of four exits is a spinner that never
// stops. All four go through settle, so all four are checked here rather than
// the one that is easy to remember.
func TestEveryExitPathClearsTheWait(t *testing.T) {
	answered := func(m *Model) tea.Msg { return replyMsg{epoch: m.epoch, answer: persona.Answer{Text: "sure"}} }
	failed := func(m *Model) tea.Msg {
		return failedMsg{epoch: m.epoch, err: &persona.Error{Kind: persona.Unreachable, Problem: "down"}}
	}
	calledOff := func(m *Model) tea.Msg {
		return failedMsg{epoch: m.epoch, err: &persona.Error{Kind: persona.Canceled, Problem: "called off"}}
	}
	pressedEsc := func(m *Model) tea.Msg { return tea.KeyPressMsg{Code: tea.KeyEscape} }

	for name, exit := range map[string]func(*Model) tea.Msg{
		"answered":   answered,
		"failed":     failed,
		"called off": calledOff,
		"esc":        pressedEsc,
	} {
		t.Run(name, func(t *testing.T) {
			m := New()
			m.composer.SetValue("hello")
			m.send()
			if !m.waiting.live {
				t.Fatal("nothing was pending, so the exit is untested")
			}

			next, _ := m.Update(exit(&m))
			if next.(Model).waiting.live {
				t.Error("the wait is still up after it ended")
			}
		})
	}
}

// Cancelling a context does not stop a request that already succeeded, so an
// answer can arrive after the human pressed esc. Recording it then is the
// machine acting after being told not to.
func TestAnAnswerFromACalledOffRequestIsDropped(t *testing.T) {
	m := New()
	m.composer.SetValue("hello")
	m.send()

	stale := m.epoch
	bits := len(m.shown)

	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = next.(Model)

	next, _ = m.Update(replyMsg{epoch: stale, answer: persona.Answer{Text: "too late"}})
	m = next.(Model)

	if got := len(m.shown); got != bits {
		t.Errorf("view holds %d bits, want %d — a called-off reply was recorded anyway", got, bits)
	}
}

// A wait the human called off is not a failure. They pressed the key and
// watched the line go; a banner about it is a second keystroke for nothing.
func TestCallingItOffRaisesNoNotice(t *testing.T) {
	m := New()
	m.composer.SetValue("hello")
	m.send()

	next, _ := m.Update(failedMsg{
		epoch: m.epoch,
		err:   &persona.Error{Kind: persona.Canceled, Problem: "the reply was called off before it arrived"},
	})
	if next.(Model).trouble.up() {
		t.Error("cancelling raised a failure notice")
	}
}

// The two failures a new user hits first are the reason persona/errors.go
// exists, and each carries a problem and a one-line fix written to be read at
// the keyboard. Reducing them to "error" throws that file away, so both
// sentences have to survive the trip to the screen intact.
func TestBothHalvesOfAFailureReachTheScreen(t *testing.T) {
	// The sentences persona.Client actually writes for the two failures, copied
	// rather than provoked: provoking them needs a server that is down and a
	// model that is missing, and a unit test may not depend on either.
	for _, want := range []*persona.Error{
		{
			Kind:    persona.Unreachable,
			Problem: "ollama is not answering at http://localhost:11434 — it does not appear to be running",
			Fix:     "start it with: ollama serve",
		},
		{
			Kind:    persona.NoModel,
			Problem: `ollama is running, but the model "qwen3.5:latest" is not installed`,
			Fix:     "pull it with: ollama pull qwen3.5:latest",
		},
	} {
		got := explain(want)
		if got.problem != want.Problem {
			t.Errorf("problem = %q, want %q", got.problem, want.Problem)
		}
		if got.fix != want.Fix {
			t.Errorf("fix = %q, want %q", got.fix, want.Fix)
		}

		// And it is on the screen, whole, at a width that can hold it. Wrapped
		// rather than cut: this is the one block here that is prose to act on
		// rather than a receipt to count, and half a sentence is no use.
		m := New()
		m.trouble = got
		block := ansi.Strip(m.troubleBlock())
		flat := strings.Join(strings.Fields(block), " ")
		for _, sentence := range []string{want.Problem, want.Fix} {
			if !strings.Contains(flat, strings.Join(strings.Fields(sentence), " ")) {
				t.Errorf("the block dropped %q:\n%s", sentence, block)
			}
		}
		if !strings.Contains(flat, "nothing was recorded") {
			t.Errorf("the block does not say the failure is absent from the record:\n%s", block)
		}
	}
}

// The two blocks share every row but the first, and the first is the one that
// cannot be shared: a save that failed means everything reached the record and
// the file did not, which is the exact opposite of what the other header says.
//
// Two claims, and the second is the one that would otherwise be unheld. Nothing
// in the suite renders an unsaved notice, so deleting [notice].unsaved's branch
// in [Model.troubleBlock] leaves every other test green while the screen tells a
// person their words were not recorded above a transcript of the words. This is
// a check about a falsehood rather than about how the block looks — the same
// shape as the one in [TestTheUnfinishedLineSaysWhyAndWhatToDo], for the same
// header and the same reason.
//
// The floor is measured rather than derived. At five columns and below both
// headers have been cut to "not …", which is the shared prefix of two true
// sentences and not a claim about either — the terminal is doing the cutting and
// the mark says so. Six is where they part.
func TestASaveThatFailedNeverSaysNothingWasRecorded(t *testing.T) {
	const floor = 6

	// One error and one fix on both, so nothing below the header can account for
	// a difference.
	unsaved := New()
	unsaved.trouble = saveFailed(errors.New("/state/tldr/record: no space left on device"))

	other := New()
	other.trouble = notice{
		problem: "/state/tldr/record: no space left on device",
		fix:     unsaved.trouble.fix,
	}
	other.trouble.unsaved = false

	head := func(m Model, w int) string {
		m.width = w
		row, _, _ := strings.Cut(m.troubleBlock(), "\n")
		return ansi.Strip(row)
	}

	for w := 1; w <= 120; w++ {
		u := unsaved
		u.width = w
		drawn := ansi.Strip(u.troubleBlock())
		for _, lie := range []string{"nothing was recorded", "not recorded"} {
			if strings.Contains(strings.Join(strings.Fields(drawn), " "), lie) {
				t.Fatalf("the save-failure block says %q at width %d, and the words are on the screen above it:\n%s",
					lie, w, drawn)
			}
		}

		a, b := head(unsaved, w), head(other, w)
		switch {
		case w >= floor && a == b:
			t.Errorf("width %d: both blocks head with %q, so one of them is lying about which side of the line the record is on", w, a)
		case w < floor && a != b:
			t.Errorf("width %d: the headers differ (%q, %q) below the measured floor of %d — the floor moved and this comment is now wrong",
				w, a, b, floor)
		}
	}
}

// An error is a fact about this harness, not about the conversation, so it is
// never a bit. The question that provoked it is on the record, which leaves the
// record showing a question with no answer — incomplete, but not a lie.
func TestAFailureIsNotRecorded(t *testing.T) {
	m := New()
	m.composer.SetValue("hello")
	m.send()

	bits, stored := len(m.shown), m.store.Len()
	next, _ := m.Update(failedMsg{
		epoch: m.epoch,
		err:   &persona.Error{Kind: persona.Unreachable, Problem: "down", Fix: "start it"},
	})
	m = next.(Model)

	if got := len(m.shown); got != bits {
		t.Errorf("view holds %d bits after a failure, want %d", got, bits)
	}
	if got := m.store.Len(); got != stored {
		t.Errorf("record holds %d bits after a failure, want %d", got, stored)
	}
	if !m.trouble.up() {
		t.Error("the failure reached neither the record nor the screen")
	}
}

// A reply cut off for room is a sentence that happens to end rather than a
// thought that finished, and the store never forgets — so it is recorded as a
// fragment, which is a bit that says that about itself.
//
// This test asserted the opposite until the screen could draw one. Refusing put
// the fact in a display field and nowhere else: a second truncated reply
// overwrote the first with no trace, and quitting lost both, so the model spoke
// and the permanent record said nothing happened.
//
// The last assertion is the one that makes recording safe rather than merely
// kinder. The flag reaches the content address (D26), so the bit on the record
// is not the bit the same words would have taken had the model finished —
// derived here from the recorded bit itself with one field changed, so nothing
// but the flag can account for the difference. Were it not to reach the address,
// the store would keep whichever landed first and hand back the wrong one under
// the right address, silently.
func TestATruncatedReplyIsRecordedAsAFragment(t *testing.T) {
	const text = "the three steps are, first,"

	m := record(3)
	bits, stored := len(m.shown), m.store.Len()

	m.recordReply(persona.Answer{Text: text, Truncated: true})

	if got := len(m.shown); got != bits+1 {
		t.Fatalf("view holds %d bits, want %d — the fragment was refused", got, bits+1)
	}
	if got := m.store.Len(); got != stored+1 {
		t.Fatalf("record holds %d bits, want %d — the fragment never reached the store", got, stored+1)
	}

	shown := m.shown.Bits(m.store)
	last := shown[len(shown)-1]
	u, ok := last.Payload.(memory.Utterance)
	if !ok {
		t.Fatalf("the reply was recorded as %T, want an utterance", last.Payload)
	}
	if u.Text != text {
		t.Errorf("the record holds %q, want the reply exactly as it arrived", u.Text)
	}
	if !u.Truncated {
		t.Error("the fragment is on the record as a complete utterance, which is a permanent falsehood")
	}
	if last.From != m.persona.Handle() {
		t.Errorf("the fragment was recorded under %+v, want the persona's own handle", last.From)
	}
	if m.trouble.up() {
		t.Errorf("trouble = %+v, want nothing — the record holds this now, so the screen is not the only copy", m.trouble)
	}

	finished := last
	finished.ID = ""
	finished.Payload = memory.Utterance{Text: text}
	if memory.ID(finished) == last.ID {
		t.Errorf("the fragment took address %s, which is the address the same words finished would take",
			memory.Short(last.ID))
	}
}

// A new question supersedes the last failure: it is about to fail the same way
// or to succeed, and either way the old notice is answered by what happens next.
//
// This used to be conditional. One kind of notice could not be superseded by
// anything — a reply discarded for truncation, whose only trace in the whole
// program was the notice itself — so a send checked a flag before clearing.
// That case is gone with the refusal that made it, and the exception went with
// it: a conditional left standing would strand failures on screen with nothing
// to clear them but esc.
func TestASendClearsTheLastFailure(t *testing.T) {
	down := explain(&persona.Error{
		Kind:    persona.Unreachable,
		Problem: "ollama is not answering at http://localhost:11434 — it does not appear to be running",
		Fix:     "start it with: ollama serve",
	})

	t.Run("a send does", func(t *testing.T) {
		m := record(2)
		m.ollama = offline()
		m.trouble = down

		m.composer.SetValue("try that again")
		m.send()

		if m.trouble.up() {
			t.Errorf("trouble = %+v after a send, want the failure superseded by the new question", m.trouble)
		}
	})

	// And esc does, which is the person saying they are done with it rather
	// than a side effect of saying something else. It is the only way out while
	// nothing is being sent.
	t.Run("esc does", func(t *testing.T) {
		m := record(2)
		m.trouble = down

		next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
		if next.(Model).trouble.up() {
			t.Error("esc left the notice up, so there is no way to dismiss it")
		}
	})
}

// A fragment must not reach the persona as a finished answer.
//
// This is a regression the moment fragments are recorded rather than refused,
// and that is the reason it is tested rather than reasoned about. Before, the
// model saw nothing where a truncated reply had been — an absence. Left in its
// speaker's own voice it would now see a sentence that stops, in the assistant
// role, indistinguishable from a thought that finished. turns' own rule for the
// payload nobody planned for is that a gap arrives as something the model is
// told about rather than something it is left to fill, and a known-incomplete
// turn handed over as complete is the worse case: nothing about it invites a
// second look.
//
// Three things, and the middle one is the point. The turn is the system's, the
// words are all there and exactly as they arrived, and no assistant turn
// carries them.
func TestAFragmentReachesThePersonaAsSomethingUnfinished(t *testing.T) {
	const text = "the three steps are, first,"

	m := New()
	m.say(localHandle, "what are the three steps")
	m.utter(m.persona.Handle(), memory.Utterance{Text: text, Truncated: true})

	got := m.turns()
	if len(got) != len(m.shown) {
		t.Fatalf("turns = %d, want one per view entry (%d)", len(got), len(m.shown))
	}

	last := got[len(got)-1]
	if last.Role != persona.RoleSystem {
		t.Errorf("the fragment arrived as role %q, want %q — in its own voice it reads as a finished answer",
			last.Role, persona.RoleSystem)
	}
	if !strings.Contains(last.Content, text) {
		t.Errorf("the fragment's own words did not reach the persona:\n%s", last.Content)
	}
	if !strings.Contains(last.Content, "ran out of room") {
		t.Errorf("the turn does not say the answer was cut off:\n%s", last.Content)
	}
	for i, turn := range got {
		if turn.Role == persona.RoleAssistant && strings.Contains(turn.Content, text) {
			t.Errorf("turn %d carries the fragment as the assistant's own finished words: %q", i, turn.Content)
		}
	}

	// A finished reply with the same words still arrives in the persona's own
	// voice, so the branch above is not satisfied by sending everything as
	// system.
	whole := New()
	whole.say(localHandle, "what are the three steps")
	whole.utter(whole.persona.Handle(), memory.Utterance{Text: text})
	if got := whole.turns(); got[len(got)-1].Role != persona.RoleAssistant {
		t.Errorf("a finished reply arrived as role %q, want %q",
			got[len(got)-1].Role, persona.RoleAssistant)
	}
}

// The remedy has to be somewhere a person meets it. The row above says the
// answer is unfinished and has no room to say why or what to do; this line is
// where both live, and without it the only copy of that sentence in the program
// is in persona/client.go on the path where the model said nothing at all —
// present for the total failure, absent for the common one.
//
// It is derived from the record rather than held in a field, which is what the
// second half checks: nobody has to remember to take it down.
func TestTheUnfinishedLineSaysWhyAndWhatToDo(t *testing.T) {
	m := New()
	m.width = 100
	m.say(localHandle, "what are the three steps")
	if m.note() != "" {
		t.Fatal("a note is up before anything ran out of room")
	}

	m.recordReply(persona.Answer{Text: "the three steps are, first,", Truncated: true})
	got := ansi.Strip(m.note())
	if got == "" {
		t.Fatal("nothing on screen says why the newest answer stops mid-sentence")
	}
	for _, want := range []string{"ran out of room", "ask for less"} {
		if !strings.Contains(got, want) {
			t.Errorf("the line is missing %q: %q", want, got)
		}
	}

	// It must never borrow the trouble block's claim. That header is true of a
	// failure and false of a fragment, and a fragment is on the record.
	if strings.Contains(got, "not recorded") {
		t.Errorf("the line says the fragment was not recorded, which is false: %q", got)
	}

	// Nobody takes it down; the next thing said does.
	m.say(localHandle, "go on then")
	if got := m.note(); got != "" {
		t.Errorf("note = %q after something newer was said, want it gone", ansi.Strip(got))
	}
}

// Roles are decided by Ref, the stable half of a handle, never by Display —
// which is what the actor called itself at the time and may change. A persona
// renamed mid-session must still read its own past words back as its own.
func TestTurnsMapRolesByRef(t *testing.T) {
	m := New()
	it := m.persona.Handle()

	m.say(localHandle, "what broke")
	m.say(it, "the migration")
	m.say(memory.Handle{Ref: it.Ref, Display: "a name it used to have"}, "still me")
	m.say(memory.Handle{Ref: "ollama/other", Display: "coordinator-7"}, "confirmed here too")

	got := m.turns()
	want := []persona.Turn{
		{Role: persona.RoleUser, Content: "what broke"},
		{Role: persona.RoleAssistant, Content: "the migration"},
		{Role: persona.RoleAssistant, Content: "still me"},
		{Role: persona.RoleUser, Content: "coordinator-7: confirmed here too"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("turns =\n%#v\nwant\n%#v", got, want)
	}
}

// The persona sees the view, not the store. That one choice is what makes a
// fold something it experiences rather than a screen effect: the store still
// holds every bit and could be sent whole, and what gets sent shrinks anyway.
func TestFoldingShrinksWhatThePersonaIsSent(t *testing.T) {
	m := record(fixtureBudget * 3)
	if m.scars() == 0 {
		t.Fatal("nothing folded, so the claim is untested")
	}

	got := m.turns()
	if len(got) != len(m.shown) {
		t.Errorf("turns = %d, want one per view entry (%d)", len(got), len(m.shown))
	}
	if len(got) >= m.store.Len() {
		t.Errorf("turns = %d against a record of %d — folding sent no less than the whole store",
			len(got), m.store.Len())
	}

	// Exactly one turn stands where the scar stands, and it is the system's
	// rather than a participant's. Expanding the fold back into its bits would
	// mean folding bought nothing; skipping it would leave a gap.
	folds := 0
	for i, b := range m.shown.Bits(m.store) {
		if _, cold := b.Payload.(memory.Compaction); !cold {
			continue
		}
		folds++
		if got[i].Role != persona.RoleSystem {
			t.Errorf("the fold arrived as role %q, want %q", got[i].Role, persona.RoleSystem)
		}
	}
	if folds != m.scars() {
		t.Errorf("checked %d folds, want %d", folds, m.scars())
	}
}

// What the fold turn has to carry, because a model handed a gap fills it: how
// many went, who was in them, when, and what they were about — followed by the
// two commitments that are the whole point, to ask rather than reconstruct.
//
// What this test is and is not, stated because the note was rewritten and a
// reader will otherwise assume more of it. These are substring checks over
// prose, so they hold the *commitments* and not the wording: any of them passes
// under a paraphrase, and every one of them would pass under a note that had
// been rewritten back into a list of orders. That register is deliberately not
// asserted here — see [standingInstruction]'s comment for why it is the way it
// is, and this seat's craft record for why a test pretending to hold it would be
// a check that cannot fail. What is mechanically held is
// [TestTheFoldNoteCarriesTheIndexAndNoneOfTheContent], below.
func TestFoldNoteSaysWhatWasLostAndWhatToDoAboutIt(t *testing.T) {
	m := New()
	it := m.persona.Handle()
	for i, line := range lines[:fixtureBudget+1] {
		from := localHandle
		if i%2 == 1 {
			from = it
		}
		m.say(from, line)
	}
	c := scar(t, m)

	// The clock comes from the model, not from a fixture: it reads instants in
	// its reference's own zone, and a reference from somewhere else would move
	// every stamp in the note by the offset between them.
	note := foldNote(c, m.frame().clock)
	for _, want := range []string{
		fmt.Sprintf("%d messages went", c.Count()),

		// Both speakers, checked as the clause they appear in rather than as
		// bare names. "me" is a substring of "messages", which is in the note's
		// own first sentence, so a speakers() that dropped the human passed the
		// looser check — provenance is the one thing a summary must not lose,
		// and it was the one thing here not actually being checked.
		fmt.Sprintf(", between %s and %s, from ", localHandle.Display, it.Display),

		c.From().Format("15:04"),
		c.To().Format("15:04"),

		// The two commitments, as the clauses they are. They used to be two
		// imperatives ("Do not invent what they said." / "say so and ask.") and
		// are now a preference between two named moves, which is the whole of the
		// register change: a prohibition leaves a model holding a gap, and naming
		// the alternative is what actually gets asked instead of invented.
		//
		// Deleting them both does fail a bare "ask" check today — executed, not
		// assumed — but only because no other word in the note happens to contain
		// those three letters, which is a fact about the current prose and not
		// about the check. The names above are the cautionary case: "me" is inside
		// "messages" in the note's own first sentence.
		"ask them for it",
		"rather than reconstructing it",
		"something false into a record nobody can edit",
	} {
		if !strings.Contains(note, want) {
			t.Errorf("fold note is missing %q:\n%s", want, note)
		}
	}

	// It says the material survives, and it does not say the persona can go and
	// get it — there are no tools here, and telling a model something is
	// retrievable invites it to narrate a retrieval it never performed.
	if !strings.Contains(note, "None of it is lost") {
		t.Errorf("fold note does not say the material survives:\n%s", note)
	}
	if !strings.Contains(note, "you cannot") {
		t.Errorf("fold note does not say the persona cannot reach it:\n%s", note)
	}

	// The one sentence the note's whole posture rests on: the fold happened to
	// the person as well, in the same instant. It is what makes "there is nothing
	// to cover for" a fact rather than a reassurance, and it is true only because
	// turns() builds from the view rather than the store —
	// TestFoldingShrinksWhatThePersonaIsSent is the warrant, and if that ever
	// stops holding, this sentence becomes a falsehood told to a model.
	if !strings.Contains(note, "went from their screen") {
		t.Errorf("fold note does not say the fold reached the person too:\n%s", note)
	}

	// And the word list is framed as an index rather than left looking like a
	// summary. What is checked is that the sentence is there — not that it
	// works. The live sweep written up in foldNote's comment found a 1B model
	// confabulating 6/6 with this exact wording in front of it, so a test
	// asserting the framing prevents anything would be asserting something
	// nobody executed and the sweep contradicts.
	if !strings.Contains(note, "not a summary") {
		t.Errorf("fold note offers its word count as though it were content:\n%s", note)
	}
}

// jargon is a fixture vocabulary of words that exist nowhere else in this
// program. That is the whole of its design: any one of them found inside a fold
// note came out of the folded messages, because there is no other way for it to
// get there. Real English fixture text cannot do this job — "record", "messages"
// and "answer" are in the note's own sentences.
var jargon = []string{
	"zarquon", "frobnitz", "quibbleth", "vantablack", "wumpus", "snerk",
	"blorptastic", "grindlewald", "flimsature", "yaxley", "murgatroyd", "plinth",
	"cromulent", "widdershins", "bletcherous", "gnurr", "hoopy", "frobozz",
	"skronk", "vorpal", "brillig", "slithy", "mimsy", "borogove", "jubjub",
	"bandersnatch", "manxome", "uffish", "whiffling", "burbled", "galumphing",
	"chortled", "beamish", "frabjous", "callooh", "callay", "outgrabe", "raths",
}

// The one thing about a fold note that is mechanically checkable rather than a
// matter of prose: the index goes over and the content does not.
//
// [foldNote]'s whole claim about the word list is that it is an index — what was
// discussed, never what was said about it. That claim is prose and a test cannot
// hold prose. What it can hold is the arithmetic underneath: every word the note
// carries out of the folded messages is a word [topWords] chose, and every word it
// did not choose is absent. A note that quoted an absorbed bit, or grew a helpful
// "for instance, one of them said…", or simply widened its own slice past what the
// scar shows, fails here and fails nowhere else.
//
// Both directions are checked, and the second is why this is not vacuous: the
// index has to actually arrive. A note that dropped the word list entirely would
// satisfy the first half perfectly.
func TestTheFoldNoteCarriesTheIndexAndNoneOfTheContent(t *testing.T) {
	m := New()
	it := m.persona.Handle()
	for i := range fixtureBudget + 1 {
		from := localHandle
		if i%2 == 1 {
			from = it
		}
		// Four fixture words a line, so that the absorbed window holds
		// comfortably more distinct words than personaWords takes. The filler
		// between them is there to make the bits read as sentences; every filler
		// word here is in tui's own filler set and so is invisible to topWords.
		m.say(from, fmt.Sprintf("the %s and the %s of a %s to the %s",
			jargon[(4*i)%len(jargon)], jargon[(4*i+1)%len(jargon)],
			jargon[(4*i+2)%len(jargon)], jargon[(4*i+3)%len(jargon)]))
	}
	c := scar(t, m)
	note := foldNote(c, m.frame().clock)

	index := topWords(c.Bag(), personaWords)
	if len(index) == 0 {
		t.Fatal("the fixture produced no index, so neither half of this checks anything")
	}

	// Everything the fold absorbed, minus what the index legitimately carries.
	// Filler is dropped because topWords drops it and because those words are
	// ordinary English that the note's own sentences are made of — "the" appearing
	// in both places says nothing about anything.
	held := make(map[string]bool, len(index))
	for _, w := range index {
		held[w] = true
	}
	var withheld []string
	for w := range c.Bag() {
		if !held[w] && !filler[w] {
			withheld = append(withheld, w)
		}
	}
	if len(withheld) == 0 {
		t.Fatalf("the fixture puts %d words in the index and withholds none, so there is nothing here that could leak",
			len(index))
	}
	slices.Sort(withheld)

	// The note, taken apart the way memory/cool.go takes an utterance apart:
	// lower case, split on anything that is not a letter or a number. Comparing
	// whole words rather than substrings, so "plinth" inside "plinths" is a hit
	// and "art" inside "started" is not.
	said := map[string]bool{}
	for _, w := range strings.FieldsFunc(strings.ToLower(note), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	}) {
		said[w] = true
	}

	for _, w := range withheld {
		if said[w] {
			t.Errorf("the fold note carries %q, which the folded messages said and the index did not keep — the note is passing content off as an index:\n%s",
				w, note)
		}
	}
	for _, w := range index {
		if !said[w] {
			t.Errorf("the index chose %q and the note does not carry it:\n%s", w, note)
		}
	}
}

// The word index is a prefix of itself at every length, which is what lets one
// number decide how much of it anybody gets.
//
// This test used to be TestTheScarShowsAPrefixOfWhatThePersonaIsTold and to hold
// a second, larger claim: that the scar's own four words were the first of the
// model's twelve, so the human was never shown a summary the persona did not
// get. That claim is retired — the scar quotes an absorbed bit now, chosen by
// the reader's votes, and the note cannot follow it there without carrying a
// vote to the model. [personaWords] carries the reasoning and
// [TestNoVoteReachesThePersona] holds what replaced it.
//
// What survives here is the property of the function, and it is worth keeping
// even with one caller: topWords sorts by a total order and then slices, so if
// anything ever asks for a different number the two answers agree as far as the
// shorter one goes.
func TestTheWordIndexIsAPrefixOfItselfAtEveryLength(t *testing.T) {
	m := record(fixtureBudget * 3)
	c := scar(t, m)

	seen := topWords(c.Bag(), personaWords)
	if len(seen) < 2 {
		t.Fatalf("the fold's index is %v, which is too short to have a prefix", seen)
	}
	for n := 1; n <= personaWords; n++ {
		got := topWords(c.Bag(), n)
		if want := seen[:min(n, len(seen))]; !slices.Equal(got, want) {
			t.Errorf("topWords(%d) = %v, want the first %d of %v", n, got, n, seen)
		}
	}
}

// A fold that stands for nothing is still a turn, not a skipped entry. Every
// view entry has to reach the model as something, because a gap in a
// conversation is what a model fills.
//
// This test used to be called TestAnUnknownPayloadIsNamedRatherThanSkipped and
// its comment described turns' default: arm, which it does not reach — an empty
// memory.Compaction lands in the Compaction arm and comes out as a fold note.
// Replacing default: with a panic left it passing. The arm cannot be reached
// from this package at all: memory.Payload closes its set with two unexported
// methods, and a payload type declared here fails to compile with "missing
// method canonical" — executed, not reasoned. The arm is still right to keep,
// because it becomes reachable the day the memory package grows a kind, which
// is the day nobody will remember to look.
func TestAViewEntryThatStandsForNothingIsStillATurn(t *testing.T) {
	m := record(2)
	m.shown, _ = m.shown.Add(m.store, memory.Bit{
		At:      time.Now(),
		From:    localHandle,
		Channel: channel,
		Payload: memory.Compaction{}, // a fold with nothing in it: the emptiest payload this package can build
		Prev:    m.shown.Head(),
	})

	got := m.turns()
	if len(got) != len(m.shown) {
		t.Fatalf("turns = %d, want one per view entry (%d) — a payload was skipped", len(got), len(m.shown))
	}
	last := got[len(got)-1]
	if last.Role != persona.RoleSystem {
		t.Errorf("the empty fold arrived as role %q, want %q", last.Role, persona.RoleSystem)
	}
	if last.Content == "" {
		t.Error("the empty fold arrived as an empty turn, which is a gap by another name")
	}
}

// The wait is display state, so a resize must not disturb it. It is also the
// state a resize is most likely to lose, because layout and sync both run on
// the way through.
func TestAResizeDoesNotDisturbTheWait(t *testing.T) {
	m := New()
	m.composer.SetValue("hello")
	m.send()
	m.waiting.elapsed = 9 * time.Second

	next, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 12})
	after := next.(Model)

	if !after.waiting.live {
		t.Fatal("the wait was lost across a resize")
	}
	if after.waiting.epoch != m.epoch || after.waiting.elapsed != 9*time.Second {
		t.Errorf("waiting = %+v after a resize, want the one that was up", after.waiting)
	}
}

// Nothing this surface draws may run past the width it was given, and the two
// blocks here are no exception. The failure block is the one that wraps rather
// than cuts, so it is the one where an off-by-one runs off the edge.
func TestTheNoteNeverRunsPastTheWidthItWasGiven(t *testing.T) {
	pending := New()
	pending.composer.SetValue("hello")
	pending.send()
	pending.waiting.elapsed = 96 * time.Second

	broken := New()
	broken.trouble = notice{
		problem: "ollama is not answering at http://localhost:11434 — it does not appear to be running",
		fix:     "start it with: ollama serve",
	}

	// The third block, and the only one built from the record rather than from
	// display state: it is up exactly while the newest thing on the view is an
	// answer that ran out of room.
	unfinished := New()
	unfinished.recordReply(persona.Answer{Text: "the three steps are, first,", Truncated: true})

	for _, width := range []int{200, 100, 80, 40, 24, 20, 16, 12, 8, 4, 1} {
		for name, m := range map[string]Model{"pending": pending, "failed": broken, "unfinished": unfinished} {
			m.width = width
			for i, row := range strings.Split(m.note(), "\n") {
				if w := lipgloss.Width(row); w > width {
					t.Errorf("%s note at width %d: row %d is %d wide: %q",
						name, width, i+1, w, ansi.Strip(row))
				}
			}
		}
	}
}

// The clock is what survives every cut on the pending line, because a number
// going up is the only thing on screen that tells a slow model apart from a
// wedged one. The name and the keys go first; it goes last, down to the width
// of the number itself.
//
// Below that the line is cut with a mark rather than by the terminal, which is
// the same bargain the scar makes: a claim nobody can see is not a claim kept,
// and a row that ends because it ran out of room must not look like a row that
// happened to end there.
func TestThePendingLineKeepsItsClockDownToTheWidthOfTheClock(t *testing.T) {
	m := New()
	m.composer.SetValue("hello")
	m.send()
	m.waiting.elapsed = 26 * time.Second
	floor := lipgloss.Width("26s")

	for width := floor; width <= 120; width++ {
		m.width = width
		if got := ansi.Strip(m.pendingLine()); !strings.Contains(got, "26s") {
			t.Errorf("pending line at width %d lost the clock: %q", width, got)
		}
	}
	for width := 1; width < floor; width++ {
		m.width = width
		if got := ansi.Strip(m.pendingLine()); !strings.HasSuffix(got, "…") {
			t.Errorf("pending line at width %d was cut with no mark: %q", width, got)
		}
	}
}

// A failure block taller than the frame is shown from its start. Everywhere
// else here the newest row is the one worth pinning; this is the one place it
// is the opposite, because the first row is the claim — that nothing was
// recorded — and the last is a fix for a cause that has scrolled away. At sixty
// by ten the bottom-pinned version showed "→ start it with: ollama serve" and
// nothing else: an instruction with no stated reason.
func TestATallFailureIsShownFromItsStart(t *testing.T) {
	m := record(fixtureBudget)
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 60, Height: 10})
	m = mm.(Model)

	m.trouble = notice{
		problem: "ollama is not answering at http://localhost:11434 — it does not appear to be running",
		fix:     "start it with: ollama serve",
	}
	m.sync()

	rows := strings.Count(m.note(), "\n") + 1
	if rows <= m.viewport.Height() {
		t.Fatalf("the block is %d rows in a %d-row frame, so it is not overflowing and the case is untested",
			rows, m.viewport.Height())
	}
	if got, want := m.viewport.YOffset(), m.viewport.TotalLineCount()-rows; got != want {
		t.Errorf("viewport is at row %d, want %d — the claim is above the top of the frame", got, want)
	}
}

func TestElapsedReadsAsATimeSomebodyIsWaiting(t *testing.T) {
	for d, want := range map[time.Duration]string{
		0:                       "0s",
		1400 * time.Millisecond: "1s",
		26 * time.Second:        "26s",
		59 * time.Second:        "59s",
		60 * time.Second:        "1m00s",
		96 * time.Second:        "1m36s",
		11 * time.Minute:        "11m00s",
	} {
		if got := elapsed(d); got != want {
			t.Errorf("elapsed(%s) = %q, want %q", d, got, want)
		}
	}
}

// A model writes paragraphs. Every column measurement on this surface — the
// width check, the ellipsis, the clip — is arithmetic on a single line, so a
// reply with a newline in it must still be one row: the count on a scar is
// checked by counting rows, and a bit that draws as three of them breaks the
// only proof the receipt has.
//
// The record keeps the newlines. This is the view dropping something and the
// store keeping it, which is the same split the whole program is about.
func TestAReplyWithNewlinesIsStillOneRow(t *testing.T) {
	m := record(2)
	reply := "Three things:\n\n1. backfill first\n2. then drop\n3. verify"
	m.recordReply(persona.Answer{Text: reply})

	// With the caret off the reply, because the caret's row is drawn whole and this
	// is a claim about the row every other bit gets. The claim itself is unchanged
	// and is the one that matters either way: the newlines are in the record and
	// never on the screen. What an expanded row does with them is
	// [TestAnExpandedRowShowsEveryWordAndNotTheLineBreaks], which asserts the same
	// thing from the other side.
	m.move(-1)

	bits := m.shown.Bits(m.store)
	if got := bits[len(bits)-1].Payload.(memory.Utterance).Text; got != reply {
		t.Errorf("the record holds %q, want the reply as it was written", got)
	}

	for _, width := range []int{200, 80, 40, 20} {
		got := strings.Split(shot(m, width, false), "\n")
		if len(got) != len(bits) {
			t.Errorf("transcript at width %d drew %d rows for %d bits", width, len(got), len(bits))
		}
	}
}

// An error that is not a *persona.Error still has to reach the screen. "Every
// path in Reply returns one" is a claim about another package, and this
// repository's characteristic defect is a claim nobody re-derived.
func TestAnUnexpectedErrorStillReachesTheScreen(t *testing.T) {
	got := explain(errors.New("something nobody wrote a sentence for"))
	if !strings.Contains(got.problem, "something nobody wrote a sentence for") {
		t.Errorf("problem = %q, want the error's own words", got.problem)
	}
}

// The standing constraint, made executable: a tea.Cmd runs on its own
// goroutine, and memory.View is a value with no synchronization whose entire
// safety property is the capped append in Add. So the command must capture
// everything it needs before it is returned and touch nothing afterwards. Run
// under -race, this fails if the closure ever reads the view or the model.
//
// What the test has to get right, because the first version of it did not: send
// returns tea.Batch(ask, beat), and a batch runs nothing. Calling it yields a
// tea.BatchMsg — the slice of commands the runtime will each start on its own
// goroutine — so a test that calls the batch and stops there never reaches the
// request closure at all. That version asserted only that the message was
// non-nil, which the BatchMsg satisfied. So the batch is unpacked here and every
// member is run, which is what the runtime does.
//
// Measured, both ways, against an ask deliberately built wrong — capturing
// *Model and calling m.turns() inside the goroutine: the old body passed 40 runs
// out of 40 under -race, and this one failed on the first.
//
// The last assertion is the other half. Running the members is no use if the
// only one that produced anything was the clock, so the request's own message
// has to arrive — that is what says the closure executed on a goroutine while
// the loop below was writing the view. Checked by dropping ask from the batch,
// which this catches and nothing else in the package does.
//
// The client is pointed at an address persona.Reply rejects in its check of the
// client's own BaseURL, before any I/O, so this needs no server and does no
// network. The clock in the batch takes its second, and that second is the
// price of running what the runtime runs.
func TestTheRequestGoroutineTouchesNeitherViewNorModel(t *testing.T) {
	m := record(fixtureBudget)
	m.ollama = offline()

	m.composer.SetValue("what happened to the migration")
	cmd := m.send()
	if cmd == nil {
		t.Fatal("send returned no command")
	}

	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("the command yielded %T, want a tea.BatchMsg — nothing here ran the request", msg)
	}

	msgs := make(chan tea.Msg, len(batch))
	var wg sync.WaitGroup
	for _, c := range batch {
		wg.Go(func() { msgs <- c() })
	}

	// Meanwhile the update loop carries on with its own copy, adding bits and
	// folding — every operation that writes the view.
	for i := range fixtureBudget * 2 {
		m.say(localHandle, fmt.Sprintf("while we wait %d", i))
	}
	wg.Wait()
	close(msgs)

	asked := false
	for msg := range msgs {
		if f, ok := msg.(failedMsg); ok {
			asked = true
			if f.epoch != m.epoch {
				t.Errorf("the request answered for epoch %d, want %d", f.epoch, m.epoch)
			}

			// And it failed on the address rather than on the network, which is
			// this file's no-network rule executed rather than asserted in a
			// comment: persona.Unusable comes from the BaseURL check, before a
			// request is built.
			var e *persona.Error
			if !errors.As(f.err, &e) || e.Kind != persona.Unusable {
				t.Errorf("the request failed with %v, want the address rejected before any I/O", f.err)
			}
		}
	}
	if !asked {
		t.Error("no reply or failure arrived, so the request closure never ran")
	}
}

// What the persona is sent is capped, and the cap is not the screen's.
//
// [Model.budget] is the terminal's height, so without [askCeiling] a tall window
// asks for more context than the model on the other end will read: at 200x80 the
// view reaches 74 bits, and measured against a live ollama 0.17.7 a realistic bit
// costs about 60 tokens against a default window of 4096 — roughly 68 bits, less
// the system prompt. Dragging a window would change the size, the latency and
// eventually the answer of every request.
//
// Asserted at two heights that both exceed the cap, because one would pass
// against any constant at all: the two must send the *same* number of turns while
// their screens hold different numbers of bits. And the newest must be the ones
// that survive, since a reply is to the tail of a conversation.
func TestWhatThePersonaIsSentIsCappedBelowWhatATallScreenHolds(t *testing.T) {
	sent := map[int]int{}
	views := map[int]int{}
	for _, h := range []int{80, 120} {
		m := sized(200, h)
		if m.budget() <= askCeiling {
			t.Fatalf("a terminal %d rows tall budgets %d, which is under the cap of %d — this fixture cannot reach the cap",
				h, m.budget(), askCeiling)
		}
		for i := range m.budget() {
			m.say(localHandle, fmt.Sprintf("bit %d", i))
		}
		views[h], sent[h] = len(m.shown), len(m.turns())

		if got := len(m.turns()); got > askCeiling {
			t.Errorf("a terminal %d rows tall sends %d turns, over the ceiling of %d", h, got, askCeiling)
		}
		// The tail, not the head.
		last := m.shown.Bits(m.store)[len(m.shown)-1].Payload.(memory.Utterance).Text
		if turns := m.turns(); len(turns) == 0 || !strings.Contains(turns[len(turns)-1].Content, last) {
			t.Errorf("a terminal %d rows tall does not end its request with the newest bit (%q)", h, last)
		}
	}

	if views[80] == views[120] {
		t.Fatalf("both terminals held %d bits, so a cap and no cap look the same here", views[80])
	}
	if sent[80] != sent[120] {
		t.Errorf("two screens holding %d and %d bits sent %d and %d turns — the terminal is still deciding how much context the model gets",
			views[80], views[120], sent[80], sent[120])
	}
}
