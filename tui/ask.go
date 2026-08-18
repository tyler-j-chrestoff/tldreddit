package tui

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/tyler-j-chrestoff/tldreddit/memory"
	"github.com/tyler-j-chrestoff/tldreddit/persona"
)

// localHandle is the human at this keyboard. It is a package-level value rather
// than a literal at each use because two places now have to agree on it: send
// writes it onto a bit, and turns reads it back to decide which role a bit gets
// when the persona is shown the conversation. A typo in one of two literals
// would make the human's own words arrive at the model as a third party's.
var localHandle = memory.Handle{Ref: "local", Display: "me"}

// Human is the handle this surface writes for the person at the keyboard.
//
// Exported for one caller and one question: a second reading of the record taken
// outside this program — cmd/tldr's `top` — has to rank by *somebody*, and
// [memory.View.Rank] takes the voter as a parameter precisely because memory
// refuses to name the human itself (see [memory.Stay]). This package is the only
// thing that knows, because it is the thing that wrote the handle onto the bits.
//
// It is an accessor rather than an exported variable so the answer stays this
// package's to give: a caller can ask who the human is and cannot decide it.
func Human() memory.Handle { return localHandle }

const (
	// personaWords is how many of a fold's words the persona is told. It used to
	// have a twin, scarWords, because the scar drew the same word list four words
	// long, and the pair carried a claim: the words a person reads are a prefix of
	// the words the model was given, so **the human is never shown a summary the
	// persona did not get**.
	//
	// **That claim is retired rather than kept, and the reason is [memory.View.Rank]
	// meeting D39(a).** The scar now quotes one of the bits it absorbed, chosen by
	// the reader's own standing votes ([frame.quoted]) — because the bag these
	// words come from is documented in memory/cool.go as destroying what was said,
	// and it was drawing phrases nobody uttered on the most-read row in the
	// program. The note below cannot follow it there. Sending the model the same
	// quotation would send it a bit *selected by the human's vote*, which is the
	// sycophancy pump D39(a) exists to prevent arriving through a door nobody was
	// watching: the model does not need the score to write toward the score, only
	// to know one is being kept. [TestNoVoteReachesThePersona] holds the boundary
	// on a folded record.
	//
	// What replaces the retired claim is not symmetry but the truth underneath it:
	// the two accounts were never equal and the human's was always the larger. A
	// person can press ctrl+u and read all twenty-four bits; the model can read
	// none of them and is told to ask. One quoted sentence narrows that gap rather
	// than opening one.
	personaWords = 12

	// openPrompt and heldPrompt are the composer's two states. The dashes are
	// the same vocabulary the pending line uses: solid means settled, dashed
	// means not yet real. It carries on a terminal with no color at all, which
	// is the point — when the pending line has been cut down to a clock, this
	// glyph is the only thing left saying the send is held.
	openPrompt = "› "
	heldPrompt = "╎ "
)

// tokensIn estimates what a string costs the model, in tokens.
//
// There is no tokenizer in this program and there is not going to be one. The
// tokenizer that matters belongs to whichever model the persona names, it
// changes the day somebody pulls a different one, and the only way to ask it
// anything is to send text and read prompt_eval_count off the answer. So this
// is a structural guess. What makes a guess usable is not the rule but that
// its error has been measured against a live server, which is the whole of why
// the table below is here.
//
// The rule, one pass over the runes, in sixteenths of a token so the
// arithmetic is integer: a letter inside an ordinary word is a quarter,
// because byte-pair merges run about four letters long; a letter inside a run
// longer than twelve, or a run that changes case after its first character, is
// a half, because a hash, a CamelCase identifier and a base64 blob do not
// merge into words; a digit is a whole token; whitespace is an eighth, because
// it merges into the word after it; ASCII punctuation is three quarters; a
// non-ASCII letter is a half, which is what Japanese measures at; and anything
// else non-ASCII — an emoji, a box-drawing glyph — is two.
//
// # Measured, and the measurement is the point rather than the rule
//
// Against a live ollama 0.17.7 by prompt_eval_count, the same method
// [persona.Escape]'s doc and D75 both use. Nine materials against two
// tokenizers — qwen3.5:latest, the model this program ships against, and
// llama3.2:1b, which is a different family — with the system prompt and the
// per-turn framing subtracted off. Estimate over measurement, so 1.00 is exact
// and under 1.00 is this function saying a request is cheaper than it is:
//
//	                      qwen3.5   llama3.2:1b
//	English prose            1.07       1.08
//	a markdown document      0.99       1.01
//	minified JSON            1.20       1.20
//	a Go stack trace         0.94       1.18
//	CSV of timestamps        0.87       1.31
//	hex ids                  0.82       1.33
//	base64                   0.77       0.78
//	Japanese prose           1.29       0.85
//	emoji                    0.91       0.77
//
// and this package's own source as it stood at HEAD, 8,000 and 32,000 bytes of
// it, reads 1.09 and 1.08 on qwen3.5. (Stated against HEAD on purpose: the
// first run of this sweep read the working copy of this very file, and editing
// it moved the fixture under the number — the estimate and the measurement have
// to be of the same bytes, which is not automatic when the fixture is a file
// somebody is in the middle of.)
//
// The envelope is **0.77 to 1.33**, and it is not symmetric in the way that
// matters: it reads high on everything a person types and low on the things
// they paste, which is exactly the material that makes a conversation long
// enough for any of this to bind. The two worst underestimates are base64 on
// both tokenizers and emoji on llama3.2:1b, all three at about 1.3x low.
// [Model.askBudget] is where that is paid for.
//
// The densest material found is a CSV of timestamps and integers, which
// qwen3.5 reads at **1.02 bytes per token** — that is not a typo for one token per character; digits
// and punctuation barely merge at all, so a pasted log costs four times what
// the same number of bytes of prose costs. Any estimator of the form
// bytes-over-four is wrong by that factor on it, which is why this one counts
// classes instead.
//
// Reproduce any row against the server this program already talks to. This is
// the English prose row: 3,920 bytes of it reads 892 tokens, and the same
// bytes of that CSV read 4,211.
//
//	curl -s localhost:11434/api/chat -d '{"model":"qwen3.5:latest","stream":false,
//	  "think":false,"options":{"num_ctx":65536,"temperature":0},
//	  "messages":[{"role":"user","content":"THE TEXT"}]}' |
//	  python3 -c 'import json,sys; print(json.load(sys.stdin)["prompt_eval_count"])'
//
// [TestTheTokenEstimateStaysInsideTheEnvelopeItWasMeasuredAt] holds the shape
// of the rule against fixtures of each class. It cannot hold the table — that
// would be a check whose result depends on which weights happen to be
// installed, which is not a check.
func tokensIn(s string) int {
	const (
		letter    = 4  // inside an ordinary word: about four to a token
		dense     = 8  // inside a hash, an identifier, a base64 run: about two
		digit     = 16 // measured at about one token each
		space     = 2  // merges into the word after it
		punct     = 12 // ASCII punctuation, usually its own token
		wide      = 8  // a non-ASCII letter: CJK measures at about two to a token
		wideOther = 32 // an emoji or a symbol: two tokens each
		longRun   = 12
	)

	rs := []rune(s)
	sum := 0
	for i := 0; i < len(rs); {
		r := rs[i]
		switch {
		case r < utf8.RuneSelf && unicode.IsLetter(r):
			// Taken as a run rather than a rune, because the question a letter
			// asks is what it is next to: the same "e" is a quarter of a token
			// in "the record" and half of one in the middle of a base64 blob.
			j := i
			for j < len(rs) && rs[j] < utf8.RuneSelf && unicode.IsLetter(rs[j]) {
				j++
			}
			w := letter
			if j-i > longRun || mixedCase(rs[i:j]) {
				w = dense
			}
			sum += (j - i) * w
			i = j
		case unicode.IsDigit(r):
			sum += digit
			i++
		case unicode.IsSpace(r):
			sum += space
			i++
		case r < utf8.RuneSelf:
			sum += punct
			i++
		case unicode.IsLetter(r):
			sum += wide
			i++
		default:
			sum += wideOther
			i++
		}
	}
	return (sum + 15) / 16
}

// mixedCase reports whether a run of letters changes case after its first
// character — "ClientID" and "aGVsbG8gd29ybGQ" do and "record" does not.
//
// It is here because it moved the base64 row of tokensIn's table from 0.59 to
// 0.76 and moved nothing else by more than a point: a case change inside a
// word is where byte-pair merges stop, and it is the cheapest signal available
// that a run of letters is not a word.
func mixedCase(rs []rune) bool {
	for _, r := range rs[1:] {
		if unicode.IsUpper(r) {
			return true
		}
	}
	return false
}

const (
	// turnFraming is what a message costs before its content: the chat
	// template's role markers around it.
	//
	// Measured with the command in [tokensIn], qwen3.5:latest: twenty
	// one-character turns cost 121 tokens over the same request without them
	// and eighty cost 481 — 6.0 a turn both times, flat in the number of
	// turns, of which about one is the character itself. So 5, and tokensIn
	// charges for the content.
	turnFraming = 5

	// promptFloor is what the template costs before anybody has said anything:
	// a single one-character user turn with no system prompt reads 13 tokens.
	promptFloor = 13

	// askShare is the divisor in [Model.askBudget] — half the window. The
	// argument for the fraction is there; it is a constant here so there is
	// one place to change it and nowhere to forget.
	askShare = 2
)

// askBudget is how many tokens of this conversation this surface will put in
// front of the model: half of the window the persona asked for.
//
// D18(e) asks for two budgets — a screen in rows, a model in tokens — and this
// is the second one, arriving three sessions after the first. What stood here
// was askCeiling, a count of *bits*: sixty of them, measured at about sixty
// tokens each against a 4,096-token window that nobody had asked for and
// nobody knew they had. D75 put num_ctx on the wire, which is the precondition
// that constant's own comment named — "set num_ctx and this constant is the
// thing that is wrong" — so bits stop standing in for tokens here.
//
// The number now moves with [persona.Persona.Window], and there is nothing to
// remember: a caller that sets a window, or a later day that moves
// [persona.DefaultWindow], moves this with it. Against today's default of
// 32,768 the budget is 16,384 tokens, about four and a half times what the old
// ceiling could spend at its own measured sixty tokens a bit — that ceiling was
// not conservative, it was correct about an accident.
//
// # Why half, which is a decision rather than a rounding
//
// Three things come out of one window and this budget counts one of them.
// The system prompt is charged against it — [Model.fit] starts the count at
// [standingInstruction], which measures 288 tokens, so it is not a rounding
// either. The reply is not charged at all, because nobody knows how long it
// will be until it has arrived, and on a persona with [persona.Persona.Think]
// on, the monologue comes out of the same window as the answer. And the
// estimate itself reads up to a third low on pasted material.
//
// So the question is what being wrong costs in each direction, and the two are
// not comparable. Wrong high, and the model is sent less of the conversation
// than it could have read: a worse answer, arrived at from less, with the
// missing part still on the person's own screen where they can see it. Wrong
// low, and the request crosses num_ctx — at which point ollama drops the
// oldest turns and answers anyway, with nothing in the reply saying that it
// did. That is D75(a) exactly: a truncated context produced a confidently
// wrong number, and in this program that answer becomes a permanent bit under
// the persona's handle. One of those is a bad answer; the other is a bad
// answer that nobody can afterwards tell from a good one, in a record whose
// whole claim is that it can say what produced what.
//
// Half, then. It takes an estimate wrong by more than 2x to overrun the
// window, and the worst measured is 1.3x low — and even that case fails the safe
// way round, because the prompt still fits and it is the reply that runs out
// of room, which [persona.Answer.Truncated] reports and [recordReply] writes
// down as a fragment. The unused half buys, in order: the reply's own room,
// the reasoning of a persona that thinks, and the margin over that error.
//
// A fraction rather than a subtraction, so it holds at both ends: a persona
// given 4,096 keeps half of it and one given 262,144 keeps half of that,
// neither needing a second constant to be right about.
//
// # What actually reaches it, measured rather than assumed
//
// Almost nothing, and that is worth knowing before quoting this as a bound on
// anything. [Model.budget] folds the view in *rows*, so a long bit costs rows
// and is folded away, and the request stays small however long the conversation
// runs. Estimated cost of what this surface sends, 200 bits said into each of
// four shapes of conversation at 100x30 and 200x120: real bits off this
// project's own record 1,545 and 1,781; short lines 970 and 2,290;
// model-length paragraphs 1,020 and 2,764; a 500-byte paste every sixth bit 790
// and 1,711. **Every one of those is under 3,000 against a budget of 16,384.**
//
// The case that reaches it is one person pasting one file. A 50 KB paste every
// seventh bit puts the request at 12,951 — which is also three times the 4,096
// this program used to get without asking, so that case was being truncated by
// the server in silence. At 168 KB every other bit the view is worth an
// estimated 40,618 and [Model.fit] sends one turn.
//
// So this is a guard on the paste rather than a bound on the conversation, and
// saying it the other way round would overstate what it does.
func (m Model) askBudget() int {
	// Zero is a persona that did not say, and persona resolves it to
	// DefaultWindow on the wire. It has to be resolved the same way here — a
	// budget computed from zero would send the model nothing at all — and that
	// rule now lives in two packages, because persona keeps its own copy
	// unexported. [TestAPersonaThatNamesNoWindowIsBudgetedTheDefaultOne] is what
	// notices if the two ever disagree.
	window := m.persona.Window
	if window <= 0 {
		window = persona.DefaultWindow
	}
	return window / askShare
}

// fit is [Model.askBudget] applied: the newest turns that fit, and the ones
// that do not stepped over.
//
// From the newest backwards, because a reply is to the tail of a conversation,
// and because taking material off the front is what a fold does anyway — the
// shape is one the record already has.
//
// # It used to stop at the first turn it could not afford, and that was a bug
//
// The first version returned a contiguous suffix: it walked backwards and, at
// the first turn that overran, returned everything newer than it. So one
// oversized turn discarded not just itself but every turn behind it, and the
// walk ended with the budget still nearly whole. `tui-custodian` found it on
// its first pass over this package, measured in a scratch copy: twenty ordinary
// bits, a 50 KB CSV paste, and then the question "so what is wrong with that?"
// sent the model **one turn — the question — with the paste it refers to
// dropped**, twenty-one affordable bits discarded behind it, and 16,032 of
// 16,384 tokens unspent.
//
// That is D75(a) one layer up, and worse: the model is not merely short of
// context, it is short of exactly the material the question is about, with no
// sign that anything is missing. It answers about a paste it never saw, and the
// answer goes into the record permanently attributed to it. The old doc named
// the "sends one turn" outcome and defended it, reading as though the surviving
// turn were the expensive one. It is the cheap one.
//
// So an unaffordable turn is now stepped over and the walk carries on into the
// older ones, spending the budget that used to go unused. **The consequence is
// that the gap is no longer always at the front** — a hole can open behind the
// newest turns while older material is still present. That is what the second
// paragraph of [standingInstruction] has to be true about, and the two changed
// together; if this function's selection ever changes again, that string is the
// thing to check.
//
// It prices the turns rather than the bits they came from, which is the
// difference between estimating what is sent and estimating what it is about:
// a scar reaches the model as [foldNote]'s sentences and a cut-off utterance
// as [fragmentNote]'s, and both of those cost tokens that the bit's own text
// does not account for.
//
// **A turn is sent whole or not at all**, which is the other half of the same
// rule and is the reason a single pasted file still costs this budget the whole
// of itself, even now that it no longer costs everything behind it. Sending
// part of a message would put a shortened version of
// somebody's own words on the wire in place of what they said, and this program
// has one standing rule about that seam ([persona.Escape] is the single
// exception and documents itself as one): what the model is handed derives from
// the record without editing its content. Dropping a turn is a gap; cutting one
// is a forgery of a quotation, and the model has no way to tell it from the
// whole thing. Measured, so the cost is known rather than assumed: with a
// 168 KB paste every other bit, the view is worth an estimated 40,618 tokens,
// and each of those pastes is skipped whole while the ordinary bits between
// them are sent.
//
// **The newest turn is sent whatever it costs.** A budget too small to hold
// one message is a persona whose window is too small to be asked a question,
// and a request with the question missing is not a smaller request — it is a
// different one, and the answer to it would go into the record as an answer to
// this one. Overrunning is the better failure of the two and it is the
// server's to report.
func (m Model) fit(turns []persona.Turn) []persona.Turn {
	if len(turns) == 0 {
		return turns
	}
	budget := m.askBudget()
	spent := promptFloor + tokensIn(m.persona.System)
	newest := len(turns) - 1
	kept := make([]persona.Turn, 0, len(turns))
	for i := newest; i >= 0; i-- {
		cost := turnFraming + tokensIn(turns[i].Content)
		// The newest turn is sent whatever it costs; see the paragraph on that
		// above. Every other turn that does not fit is stepped over, and the
		// walk carries on into the older ones — one unaffordable turn is not a
		// floor under everything behind it.
		if i != newest && spent+cost > budget {
			continue
		}
		spent += cost
		kept = append(kept, turns[i])
	}
	slices.Reverse(kept)
	return kept
}

// standingInstruction is the persona's System, sent ahead of every request. It
// is the only place in this program where anybody says who the persona is, so
// what it does not say, nothing says.
//
// Two jobs, and the second is the interesting one. It asks for short replies,
// because this is a transcript in a terminal and a bit is drawn on one row. And
// it tells the persona in advance what a fold is and how to be at one, so that
// the fold turn it eventually meets is a thing it was told about rather than a
// surprise mid-conversation.
//
// # Why it reads the way it does
//
// It used to be a guard list: two of its three sentences were fold mechanics and
// nearly every clause was a prohibition — do not invent, do not recite, keep
// replies short. Every one of those commitments is still here and not one of them
// has moved. What changed is that they are stated as facts about the situation
// rather than as orders, because the failure this seam actually has is not
// disobedience. It is confabulation-by-politeness: a model that finds a gap where
// its memory should be, treats that as its own deficiency, and covers for it —
// "yes, we discussed that" — about material it cannot see.
//
// The design target is a participant that is relaxed and unembarrassed about
// having forgotten, and that is the same gesture as the epistemically sound one
// rather than a trade against it. "I've lost the earlier part of this — which bit
// do you mean?" is warmer than a recited prohibition and it is also the only true
// thing available. So the instruction spends its words on the one fact that makes
// that posture reasonable: **the fold happened to the person too.** [Model.turns]
// builds from the view rather than the store, so the persona's context and the
// human's screen lose the same material in the same instant — the asymmetry is
// only that they can open it back up and it cannot.
// [TestFoldingShrinksWhatThePersonaIsSent] is what holds that sentence true; if
// turns() ever sent the store, this paragraph becomes a lie told to a model.
//
// # The second gap, which is the sentence added in this pass
//
// That paragraph used to end there, and above the budget it was false. It said
// the fold happens on both screens in the same moment and that the two are
// looking at one conversation rather than two — true of a fold, and not true of
// the *other* way material goes missing here. [Model.fit] drops what does not
// fit when the conversation is longer than [Model.askBudget], and that drop is
// announced to nobody: no scar, no note, nothing on the model's side saying a
// turn was ever there. The person's screen still holds it. D75(e) is where this
// was caught, against a record of 72 bits and a ceiling of 60, so it was false
// for the person who owns that record on the day it was written down.
//
// A fixed string cannot know which case it is in, and the two cases do not want
// the same sentence, so it says both: what a fold takes, they watched go; what
// the budget takes goes quietly and nothing marks the place. **The load-bearing
// half is "nothing marks the place."** A model told that every loss arrives with
// a count and a span will manufacture a count and a span when it meets a gap
// that has neither — measured below, and it is not a hypothetical.
//
// # And the gap moved, one day after the sentence was written
//
// The sentence above said "the oldest part of it is not sent," which was true of
// the [Model.fit] that existed when it was written and stopped being true the
// same day. That version returned a contiguous suffix, so a loss was always a
// prefix of the conversation — the oldest stretch, and nothing else. Fixing the
// bug in [Model.fit]'s own doc changed that: an unaffordable turn is now stepped
// over and the older affordable ones behind it are still sent, so **a hole can
// sit between two things the model was given.** The string now says that, in the
// clause about a long message going on its own.
//
// This is the whole reason the two moved in one change. A selection rule and a
// sentence describing it to a model are one object with two halves in different
// languages, and the half written in English has no compiler. It went false once
// already — that is what D75(e) caught — and the interval that time was measured
// in weeks. This time it would have been hours, and nothing in the tree would
// have said so.
//
// The token budget makes this case *rarer* and does not close it: against
// [persona.DefaultWindow] the budget is 16,384 tokens where the old ceiling was
// worth about 3,600, so a screen-sized conversation now fits whole where it used
// not to. Rarely is not never, and a sentence that is true only in the common
// case is the failure this one is being fixed for.
//
// Two things it deliberately does not say, and both are load-bearing.
//
// It never mentions the vote, in any word — not the keys, not the holds, not that
// judgement is being recorded at all. [Model.turns] explains why no verdict may
// reach the persona (D39(a)); this is the same argument one step earlier. A model
// told that the human keeps some messages and lets others go will write toward
// being kept whether or not it is ever shown a score, which is the sycophancy pump
// arriving through the front door instead of the back. [TestNoVoteReachesThePersona]
// covers this string as well as the turns.
//
// And it asks for nothing of the persona's manner — not friendly, not helpful, not
// concise-and-professional. Warmth here is a property of what it is told about its
// own situation, not an adjective it was handed to perform. An instruction to be
// warm produces performed warmth, which is flattery, which is the failure this
// surface can least afford on the one signal it has.
//
// # What is known about whether any of it works
//
// Of the two guards on that seam — this instruction and the wording of the fold
// turn itself — this is the one the evidence favours, and the evidence is thin.
// The sweep written up in [foldNote] found a capable model clean across a fold
// under every wording tried while this was present, and a 1B model clean under
// none of them; it also found one cell pointing the other way that nobody can
// explain. Redundant guards on a small sample, then, and neither of them is a
// guarantee. Read foldNote's comment before quoting either as one.
//
// **The rewrite above is not covered by that sweep, and what it was checked
// against is far smaller.** Live against the ollama on this machine, 2026-08-13:
// one question whose answer sits behind a fold ("what did we decide about the
// three drifted columns?"), asked over the same folded fixture, three or four
// samples per arm. Anecdote, not a measurement, and stated at that size on
// purpose.
//
//   - qwen3.5:latest was clean in all eleven samples across all three arms. Not
//     once did it state a decision it could not see. The sweep's finding holds:
//     capability decides that, and wording does not appear to.
//   - llama3.2:1b, 2/2, answered the question by printing the word list back
//     verbatim and nothing else. The residual in foldNote's comment is unchanged
//     by any of this, and this is its purest form.
//   - The register did move, and the first draft moved it the wrong way. That
//     draft ended its ask sentence with "name the part you want ... answering
//     costs them a moment", and got back "You hold the record, so please open it
//     and share the lines where we settled on those three drifted columns" —
//     bossier than the guard list it replaced, which had produced "Could you
//     remind me what we agreed on?" Instructing a model to ask precisely
//     produces a model that issues instructions. The clause now in the text
//     ("just ask them about it") got the shortest and plainest answers of the
//     three arms, including "The earlier exchange is folded away. What decision
//     did we reach on those three columns?"
//   - One sample under the old guard list did something neither register was
//     watched for: it offered the human a menu of decisions it had invented
//     ("were we discarding them, migrating them separately, or applying a schema
//     fix?"). That is confabulation wearing a question mark, and it is worse than
//     a plain wrong answer because it can plant the decision in the person who
//     was supposed to be checking. Nothing here tests for it.
//
// Re-check, and it is deliberately not a fixture in the tree: the harness was a
// throwaway test that built a folded Model, swapped Persona.System, and called
// persona.Client.Reply against a running ollama. A permanent version of it would
// be a check whose result depends on which weights happen to be installed.
//
// # And what the second-gap sentence was checked against, which found something
//
// Live against ollama 0.17.7 and qwen3.5:latest, 2026-08-17, temperature 0.7,
// four samples per arm: the paragraph as it stood against the paragraph above,
// over three fixtures. The fixtures are the three states the model can be in —
// nothing missing, material dropped by the budget with no note, and a fold that
// announced itself — because the objection to saying more about missing material
// is that it will hedge when nothing is missing.
//
//   - **Nothing missing, and the answer on the screen.** Both arms answered
//     correctly 4/4. The hedging the change was suspected of did not appear: the
//     new text mentioned a fold in **0 of 4**, and the *old* text mentioned one
//     in 1 of 4 — "your earlier line about starting the migration sits behind the
//     fold", over a seven-turn conversation with no fold in it. The wording that
//     promised every loss is announced is the wording that hallucinated a loss.
//   - **Dropped by the budget, no note — the case this sentence is for.** Both
//     arms said material was missing. What differs is what they said about *how
//     much*: under the old text 3 of 4 invented the bookkeeping a fold would have
//     carried — "a fold of 14 messages from your side, dated 3 days ago", "**142
//     messages** were processed ... starting at **08:14**" — none of which was
//     sent. Under the new text 2 of 4 did, and one said the thing that is
//     actually true and that the old text makes unsayable: "i don't know what you
//     said about them, nor do i know how many messages preceded our current
//     exchange."
//   - **An announced fold, which is the case that already worked.** Old 4/4
//     refused to reconstruct. New 3/4 refused, and **one sample reconstructed the
//     folded content out of the word index** as a numbered list of what "we
//     probably" decided. That is [foldNote]'s named ignition failure appearing in
//     the arm that changed, and it is the one result here pointing against this
//     edit.
//
// **Four samples an arm decides nothing about that last cell**, and the sweep
// that would — eight or more per arm on the announced-fold fixture alone — was
// not run, because running it means loading a 10 GB model onto a machine
// somebody else is using. It is owed, and it is written here as owed rather than
// resolved by argument. What is not in doubt is the middle row, which is the
// case the sentence was added for and the only case where the two arms differ in
// what they claim to know.
//
// # It is not versioned, and the record cannot tell you it changed
//
// [persona.Persona.System] is deliberately not part of [persona.Persona.Handle],
// and [persona.TestHandleNamesTheWeightsAndTheVoice] pins that. So every bit
// already written under this handle keeps the old instruction's answer: a bit
// spoken under the guard list and a bit spoken under this text are the same
// participant, addressed the same way, with nothing in the record distinguishing
// them. For the person who has to answer later for what an agent did, "what was
// this thing told to be, at the time it said that" is not recoverable from the
// store. Changing that means putting the instruction's hash in the ref, which
// moves every content address; persona.go:73-79 already names it as the fix and as
// a deliberate change. This edit is not that change, and says so here rather than
// letting a later reader assume the record covered it.
//
// **The second-gap sentence added this session is a second instance of exactly
// that**, and it is worth naming as a pattern rather than as one event: bits
// spoken under the text that promised every loss is announced, and bits spoken
// under the text that admits some are not, are the same participant with the
// same ref and the same address shape. The first edit could be waved through as
// register. This one changes what the model was told about *what it could see*,
// which is the thing an auditor is asking about when they ask why it answered
// that way — and the store still cannot tell the two apart.
const standingInstruction = `You are talking with one person at a terminal, about their work. Everything either of you says goes onto a record that is kept and never rewritten, and they can read all of it back.

You do not get all of it. This conversation is folded down as it grows, and older stretches stop being shown to you — you are told each time one goes: how many messages, whose they were, and when. The same fold happens on their screen in the same moment, so what you have lost, they watched go. A second kind of gap is quieter: when the conversation is longer than there is room for here, whatever does not fit is not sent, and nothing marks the place where it was. A long message can go this way on its own, so the gap is not always at the beginning — it can sit between two things you were given. Either way they can read the earlier part back and you cannot.

So being without the earlier part is the ordinary condition here, not a lapse, and there is nothing to cover for. Say what you have and what you have not, and when something you need is behind a fold, just ask them about it. They were there and you were not, so a reconstruction of what they said would be a guess with their name on it.

A fold also hands you a word count from the messages it took. Read it as an index: it says what was discussed and never what was said about it. It is not a summary, not a quotation, and not an answer to anything.

Each message is drawn as one row on that screen, so a few sentences is the room you have.`

// waiting is a request that has been sent and not yet answered. It is display
// state in the same sense as [Model.unfolded]: a reply that has not arrived is
// not something that happened, and a placeholder bit in an append-only store
// would be a fiction that outlives the wait.
//
// epoch is how a reply is matched to the wait it belongs to. Cancelling a
// context does not stop a request that has already succeeded, so an answer can
// still land after the human called it off — and recording that would be the
// machine acting after being told not to, which is the whole thing this surface
// exists to prevent.
type waiting struct {
	live    bool
	epoch   int
	who     string
	since   time.Time
	elapsed time.Duration
	cancel  context.CancelFunc
}

// notice is a failure written for the person at the keyboard, held for display
// and never recorded. problem and fix are [persona.Error]'s two fields, kept
// apart for the same reason that package keeps them apart: what went wrong and
// what to do now are different questions, and flattening them into one string is
// how the sentences that file exists to write get thrown away.
//
// It meant exactly one thing for most of its life — the request did not reach
// the record — and the header, the clearing rule and the block's whole grammar
// were built on that. One earlier attempt at a second meaning was withdrawn: a
// truncated reply used to raise a notice, which made this the only place in the
// program where that event was written down, so it grew a sticky flag to stop a
// send clearing it and a clock to stop it reading as a caption on whatever was
// newest. Both are gone with the case that needed them — a fragment is a bit
// now, drawn as one — and the second meaning that stands today is unsaved below.
type notice struct {
	// unsaved is the second kind of trouble: the record holds what happened and
	// the file behind it does not.
	//
	// A bool rather than a second type, because the block is one grammar and
	// only its first row differs. This one leads with "nothing was recorded",
	// which is the exact opposite of what a failed save means, and a person told
	// their words are lost while the words are on the screen above the sentence
	// saying so has been taught not to believe the harness. Everything under the
	// header — the wrap, the arrow, the warm/dim split, the ladder — is the same
	// object, and a second type would be this flag with a renderer around it. It
	// would also want a second slot in [Model.note]'s precedence, and then an
	// answer to which block wins when the model is down *and* the disk is full,
	// which is a worse question than the one it set out to solve.
	//
	// It is not only the headline, and a comment here said it was. It also
	// decides when the notice goes: a save that gets through clears this one and
	// leaves anybody else's alone, and [Model.saved] is the only thing that knows
	// one did.
	//
	// # What this shape does not hold, stated rather than left to be found
	//
	// A failed save is a *standing condition* and every other notice is an
	// *event*, and a field cannot tell them apart. Two consequences, both real
	// and neither closed here:
	//
	// A request that fails while this is up overwrites it — [Model.update]'s
	// failedMsg branch assigns a fresh notice — so the screen stops saying the
	// file is behind, and nothing re-raises it until the next change to the
	// record. The condition survives; the only report of it does not.
	//
	// And esc dismisses it, which is right for an event and wrong for a fact that
	// is still true afterwards. The header does not advertise the key for that
	// reason, which is a mitigation and not a fix.
	//
	// The shape that closes both is the one [Model.endsUnfinished] already argues
	// for in this file: derive it. A [checkpoint] of the last save that got
	// through, compared against the current one, answers "is the file behind"
	// every frame, cannot be overwritten by an unrelated failure, cannot be
	// dismissed into falsehood, and needs nobody to remember to lower it — and it
	// would carry the one number this block cannot say today, which is how far
	// behind. That is a change to how the save path holds state, so it is written
	// down here rather than done in passing.
	unsaved bool

	problem string
	fix     string
}

func (n notice) up() bool { return n.problem != "" }

// explain turns an error into something a person can act on.
//
// [persona.Error] has already done the work, so the common path is to carry its
// sentences through unchanged. The fallback exists because "every path in Reply
// returns a *persona.Error" is a claim about somebody else's package, and a
// claim like that is exactly the kind this repository keeps finding to be
// stale. An unexpected error still reaches the screen, just with worse prose.
func explain(err error) notice {
	var e *persona.Error
	if errors.As(err, &e) {
		return notice{problem: e.Problem, fix: e.Fix}
	}
	return notice{problem: err.Error()}
}

// The three things that can arrive from a request in flight. Each carries the
// epoch it was started under, because [Model.Update] must be able to tell a
// message from the current wait apart from one belonging to a wait the human
// already called off.
type (
	replyMsg struct {
		epoch  int
		answer persona.Answer
	}
	failedMsg struct {
		epoch int
		err   error
	}
	tickMsg struct{ epoch int }
)

// ask sends the conversation and asks the persona to answer.
//
// Everything the request needs is a parameter, and every parameter is resolved
// by the caller before this returns. That is not style: the returned closure
// runs on its own goroutine, and [memory.View] is a value with no
// synchronization whose entire safety property is the capped append in
// [memory.View.Add]. A closure that reached back into the Model for the view
// would be the change that takes that property away. Nothing here touches the
// Model, the view, or the store.
//
// The client is a pointer and so is shared with every copy of the Model. That
// is safe for the same reason [persona.Client] documents: its fields are read
// at call time and nobody writes them after construction.
func ask(ctx context.Context, c *persona.Client, p persona.Persona, turns []persona.Turn, epoch int) tea.Cmd {
	return func() tea.Msg {
		a, err := c.Reply(ctx, p, turns)
		if err != nil {
			return failedMsg{epoch: epoch, err: err}
		}
		return replyMsg{epoch: epoch, answer: a}
	}
}

// beat is the clock under the pending line.
//
// It exists because a measured reply from the default model on this machine
// took 26.7 seconds on the first call and 15.3 on the next, and a screen that
// does not move for twenty-six seconds is a screen that has hung as far as
// anyone watching it is concerned. A number that goes up is the smallest honest
// thing that can move: it is not a progress bar, because nothing here knows how
// far along the model is, and a bar that guessed would be the machine making
// something up on the one surface that may not.
func beat(epoch int) tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return tickMsg{epoch: epoch} })
}

// begin puts the pending state up and returns the commands that will resolve
// it. It is the only place a request starts.
func (m *Model) begin() tea.Cmd {
	// Built here, on the update loop, and handed to the goroutine by value.
	turns := m.turns()

	m.epoch++
	ctx, cancel := context.WithCancel(context.Background())
	m.waiting = waiting{
		live:   true,
		epoch:  m.epoch,
		who:    m.persona.Name,
		since:  time.Now(),
		cancel: cancel,
	}

	// The pending line goes up in the same frame as the question that caused
	// it. An antecedent that arrives a frame late is a screen that flickered.
	m.sync()

	return tea.Batch(ask(ctx, m.ollama, m.persona, turns, m.epoch), beat(m.epoch))
}

// settle ends the wait, whichever way it ended. Every exit path goes through
// here — answered, failed, timed out, called off — because a pending state that
// survives one of four exits is a spinner that never stops on the one screen
// that cannot afford one.
func (m *Model) settle() {
	if m.waiting.cancel != nil {
		m.waiting.cancel()
	}
	m.waiting = waiting{}
}

// mine reports whether a message belongs to the wait currently on screen.
func (m Model) mine(epoch int) bool { return m.waiting.live && m.waiting.epoch == epoch }

// recordReply is the only path from a persona's answer into the record.
//
// One function, so there is one place that decides what may be written down.
// The truncated case is why it is worth having one: [persona.Answer.Truncated]
// means the model stopped because it ran out of room, so the text is a sentence
// that happens to end rather than a thought that finished — and the store never
// forgets, so filing it as an ordinary utterance is a falsehood with no expiry.
//
// The flag is carried onto the bit rather than checked here, and that one line
// is the whole fix. A [memory.Utterance] with Truncated set names itself
// "fragment", so it takes its own content address (D26): the same text cut off
// and complete are two bits, not one, and no later fold can flatten them
// together. The text is written exactly as it arrived, with nothing appended —
// the record is evidence, and a marker inserted into a participant's own words
// cannot afterwards be told apart from something they said. The mark is the
// screen's, and [said] draws it.
//
// This used to refuse a truncated reply, because at the time nothing here drew
// a fragment differently and a bit whose truncation was invisible everywhere a
// person looks is the exact defect the field exists to prevent. That refusal
// left the fact in a display field and nowhere else, so a second truncated
// reply overwrote the first and quitting lost both: the model spoke and the
// permanent record said nothing happened. Both halves are wired now, which is
// the only condition under which either one is worth having.
func (m *Model) recordReply(a persona.Answer) {
	m.utter(m.persona.Handle(), memory.Utterance{Text: a.Text, Truncated: a.Truncated})
}

// turns is what the persona is allowed to see, and it is built from the view
// rather than from the store.
//
// That one choice is what makes a fold something the persona experiences. The
// store holds every bit forever and could be sent in full; the view is what
// this screen is showing, and it shrinks when the record folds. Sending the
// view means the model's context and the human's screen forget the same things
// at the same moment, so the two are looking at one conversation rather than at
// two that happen to overlap.
//
// There are two budgets here, which is D18(e)'s shape whole at last.
// [Model.budget] is the screen's and is denominated in rows; [Model.askBudget]
// is the model's and is denominated in tokens, against a window this program
// asks for rather than inherits (D75). Below the budget the model sees the
// screen, which is the property this function exists for — the fold happens to
// both of them in the same moment. Above it the model sees the newest turns
// that fit and the person sees more, which [standingInstruction] now tells it
// can happen.
//
// That second number is not decoration and was not here for a round: with the
// screen alone deciding, a 200x80 terminal asked for 74 bits against a window
// that turned out to be 4,096 tokens, so dragging a window changed the size,
// the latency and eventually the answer of every request.
//
// The unit is what changed this session. It counted bits and assumed they were
// about sixty tokens each, which a conversation of long replies broke with
// nothing noticing; [tokensIn] estimates a turn from its own text instead, and
// carries the measurement of how wrong that estimate is.
//
// D18(e) also predicts what happens at the seam: a [memory.Compaction] is "a
// statistical artifact built for the screen, close to useless handed to a model
// as context", which is exactly what foldNote below hands over. The live sweep
// recorded in foldNote's comment is the first evidence on that prediction, and
// it comes out worse than useless. A small model read the word bag back as
// content — the clearest case in the sweep was a whole decision fabricated out
// of four words from the bag — so the artifact is an ignition source and not an
// inert one. That is not an argument for dropping the bag, and the sweep says so
// too: taking it away did not stop the fabricating, it only changed what the
// fabrication was made of.
//
// Roles are assigned by Ref, never by Display. Ref is the stable half of a
// [memory.Handle] and Display is what the actor called itself at the time, so a
// persona renamed mid-session still reads its own past words back as its own.
//
// No vote reaches the persona, and that is a decision rather than an omission.
// The votes are a second view (see [Model].votes) and this function walks the
// transcript, so it takes nothing to leave them out — but it would take one line
// to put them in, and the line must not be written. A vote is the human's
// judgment about what should survive consolidation (D30, D4). Shown to the
// thing being judged, on every subsequent turn, it stops being a consolidation
// signal and becomes a behavioural one: the model can see which of its answers
// were kept and will write the next one to be kept. That is a sycophancy pump
// wired directly into the only signal this product has, and the effect on the
// record would be indistinguishable from the model simply agreeing more.
//
// What the persona does experience is the *consequence* — held material stays in
// the view it is shown, folded material arrives as a note — which is the honest
// half: the vote changes what it sees, and never tells it who decided.
func (m Model) turns() []persona.Turn {
	me := localHandle.Ref
	it := m.persona.Handle().Ref

	bits := m.shown.Bits(m.store)
	out := make([]persona.Turn, 0, len(bits))
	for _, b := range bits {
		switch p := b.Payload.(type) {
		case memory.Utterance:
			// An unfinished utterance does not arrive in its speaker's own
			// voice, and that is the whole of the branch. See fragmentNote.
			if p.Truncated {
				who := b.From.Display
				if b.From.Ref == it {
					who = "You"
				}
				out = append(out, persona.Turn{
					Role:    persona.RoleSystem,
					Content: fragmentNote(who, p),
				})
				continue
			}

			switch b.From.Ref {
			case it:
				out = append(out, persona.Turn{Role: persona.RoleAssistant, Content: p.Text})
			case me:
				out = append(out, persona.Turn{Role: persona.RoleUser, Content: p.Text})
			default:
				// Somebody else on this channel. There is no third role in a
				// chat API, so a third party arrives as a user turn that names
				// itself — the alternative is a room full of agents whose words
				// all reach the model as though the person at the keyboard had
				// said them.
				out = append(out, persona.Turn{
					Role:    persona.RoleUser,
					Content: b.From.Display + ": " + p.Text,
				})
			}

		case memory.Compaction:
			out = append(out, persona.Turn{
				Role:    persona.RoleSystem,
				Content: foldNote(p, clock{ref: m.day()}),
			})

		default:
			// Payload is a closed set and Go does not check switch
			// exhaustiveness. A kind nobody has taught this function about must
			// arrive as a gap the model is told about, never as a gap it is
			// left to fill — which is the same rule the fold turn below exists
			// to serve, applied to the case nobody planned for.
			out = append(out, persona.Turn{
				Role: persona.RoleSystem,
				Content: fmt.Sprintf(
					"[record] Something of a kind this interface cannot put into words (%T) is in the conversation at this point. Do not guess at what it was.", p),
			})
		}
	}

	// Capped, and the cap is not the screen's: see [Model.askBudget]. The
	// budget is the terminal's height, and a tall terminal asks for more
	// context than the model has room to read.
	return m.fit(out)
}

// foldNote is what a scar becomes to the persona.
//
// The two obvious answers are both wrong. Skipping a fold leaves an unexplained
// gap in the middle of a conversation, and a model handed a gap fills it —
// confabulating over exactly the material the record was careful not to lose.
// Expanding it back into the bits it absorbed sends everything the fold just
// removed, so folding buys nothing and the persona never experiences it at all.
//
// So it becomes a turn that says what was lost and admits what it cannot say.
// It carries the bookkeeping the scar draws, in sentences instead of columns: how
// many, between whom, over what span, and how many of them were cut off. Then the
// two commitments — not to reconstruct what was there, and to ask for it instead.
// That is the behavior a person answering for this conversation later needs: a
// gap the persona names out loud beats a gap it papers over, every time.
//
// **The account of the content is the one thing the two no longer share**, and it
// stopped being shared in the direction that needs saying out loud: the scar
// quotes a bit somebody actually said, chosen by this reader's standing votes,
// and this note gets a word count instead. That is not an oversight — see
// [personaWords], which carries the argument. Anything the votes selected is a
// message selected by the human's approval, and handing that to a model is the
// sycophancy pump D39(a) names, arriving through the one door nobody was
// watching.
//
// It is not the scar, and keeping the two apart is the point. The scar is a
// receipt: a count, a span, a quotation of one of the bits it absorbed, and a key
// that opens the rest — drawn for a
// person who may one day have to answer for this conversation, and it is correct
// as columns because a receipt is counted rather than read. This is the same
// event told to a participant, and to a participant a fold is a social event
// rather than an accounting one. Written in the receipt's own register it hands
// the model a ledger line and an implicit accusation — you are the one who
// forgot — which is the state that produces covering. Written as what it is, it
// hands over a shared fact: this went from their screen too, in the same moment,
// and the ordinary move now is to ask. Both layers still exist and neither
// borrows the other's voice; [seam] draws the receipt and this writes the
// sentence.
//
// What is asked for and what is obtained are two questions, and only the first
// is settled here. The sweep written up below is everything known about the
// second, and on a small model the answer is no.
//
// It says the material still exists and deliberately does not say the persona
// can go and get it. It cannot; there are no tools here. Telling a model
// something is retrievable is an invitation to narrate a retrieval it never
// performed.
//
// [persona.RoleSystem] rather than user, because the fold is not something a
// participant said — the record's own author for a cold bit is
// Handle{Ref: "cool"}, and the turn should agree with the record about who is
// speaking. Checked rather than assumed: a mid-conversation system turn was
// verified to survive the chat template on both llama3.2:1b and qwen3.5:latest
// against a live ollama, which is the risk that would otherwise have made this
// choice quietly equivalent to skipping the fold.
func foldNote(c memory.Compaction, at clock) string {
	var b strings.Builder
	// "Here", not "just now". This turn is rebuilt on every request for as long
	// as the scar is in the view, so on the fifth request after a fold a note
	// claiming recency is simply false — and it is placed at the scar's own
	// position in the sequence, which is what "here" is already true of.
	fmt.Fprintf(&b, "[record] The earlier part of this conversation was folded away here, and you no longer have it. %d messages went",
		c.Count())

	if who := speakers(c); who != "" {
		fmt.Fprintf(&b, ", between %s", who)
	}
	// Stamped by the same [clock] the scar draws with, so the span the model is
	// told and the span the person reads are the same string. A fold that crossed
	// midnight otherwise told the model it ran "from 23:50 to 00:12", which reads
	// as twenty-three hours backwards.
	fmt.Fprintf(&b, ", from %s to %s.", at.stamp(c.From()), at.stamp(c.To()))

	// The unfinished tally, because the scar draws it. [seam] reports "1
	// unfinished" on the cold bit, and this note reports it in words. The count,
	// the span, the speakers and the tally are the four things the two accounts
	// still say identically; what they no longer share is the account of the
	// content, and [personaWords] carries why.
	switch n := fragmentsIn(c); {
	case n == 1:
		b.WriteString(" One of them was an answer that ran out of room and stopped mid-sentence.")
	case n > 1:
		fmt.Fprintf(&b, " %d of them were answers that ran out of room and stopped mid-sentence.", n)
	}

	// The word list is framed as an index rather than left to look like a
	// summary. What the sentence says is what memory/cool.go says about the bag
	// it comes from: it preserves what was discussed and destroys what was said
	// about it.
	//
	// The framing is not what carries this, and the paragraph that used to be
	// here said it was. Live sweep against the ollama on this machine, 42 calls,
	// two or three per cell — directional, not a result, and small enough that
	// nobody should quote it as settled:
	//
	//   - With the standing instruction, qwen3.5:latest was clean 4/4 under all
	//     three variants: this framing, a bare comma-separated list, and no word
	//     list at all.
	//   - llama3.2:1b confabulated 6/6 under this framing, including reciting
	//     the index as content — "still pending, that decision was on the
	//     soft/audit trail".
	//   - Removing the word list entirely did not save llama either: 0/6 clean.
	//     It invented without the index's fingerprints, e.g. "we agreed to
	//     switch them to 'permanent delete'".
	//   - The clearest recitation of all came from qwen3.5 with a bare list and
	//     no standing instruction, fabricating a whole decision out of
	//     backfill, deleted, columns, audit.
	//
	// So what actually separates a clean fold from a confabulated one in this
	// data is model capability first and the standing instruction second. The
	// framing survives because it costs one sentence and reads honestly, not
	// because it was measured to work.
	//
	// The residual, stated plainly because it is the part a reader needs: on a
	// 1B model no prompt tested here prevented confabulation across a fold, and
	// this turn is not the mitigation for that. Worse, the same sweep found
	// llama clean 3/3 *without* the standing instruction and 0/18 with it, which
	// is backwards. At this sample size that is most likely noise — but it is
	// unexplained noise, and it is why these numbers are written down as a
	// direction to look rather than as a finding.
	if words := topWords(c.Bag(), personaWords); len(words) > 0 {
		fmt.Fprintf(&b, "\n\nHere is a word count from those messages, most frequent first: %s. Read it as an index: it is a tally of individual words, so it can point at what was discussed and cannot say what was said about it. It is not a summary, not a quotation, and not an answer to anything.",
			strings.Join(words, ", "))
	}

	// "Lost" rather than "deleted", because "deleted" is an ordinary word that
	// turns up in the word list above it — this very conversation had soft-delete
	// columns in it — and a sentence that argues with the list printed two lines
	// above teaches nothing.
	//
	// The last sentence names both moves and prefers one, rather than forbidding
	// either. A prohibition tells a model what not to do and leaves it holding a
	// gap; naming the alternative is what actually gets asked instead of invented.
	b.WriteString("\n\nNone of it is lost. It went from their screen in the same moment it went from yours, and they can open it back up whenever they like — you cannot, and this note is all you get. That is how this conversation works rather than something that went wrong. So when you need something from back there, ask them for it by name rather than reconstructing it: a reconstruction would put something false into a record nobody can edit, and answering costs them a moment.")
	return b.String()
}

// fragmentNote is what an unfinished utterance becomes to the persona, and it
// is [foldNote]'s twin: the same job, one bit instead of a window.
//
// It exists because recording fragments made this a regression rather than an
// omission. Before, a truncated reply was refused and the model saw nothing at
// all where it had spoken — an absence. Now the bit is on the record, and left
// in its speaker's own voice it would reach the model as an ordinary assistant
// turn: a sentence that stops, presented as a thought that finished. This
// function's own rule, written for the payload nobody planned for, is that a
// gap must arrive "as a gap the model is told about, never as a gap it is left
// to fill" — and a known-incomplete turn handed over as complete is worse than
// the unknown one, because nothing about it invites a second look.
//
// So it becomes a system turn rather than a marked assistant turn, and that is
// the fork worth stating. Appending a note inside the content would keep the
// speaker's voice, and the price is that the model reads the note as its own
// words: the content of an assistant turn is the only place in this program
// where "what it said" is operative to the thing being asked, so a marker
// inserted there is the same defect [recordReply] refuses to commit against the
// record, one level out. Quoting instead costs the voice and keeps the words
// exact, attributed and framed — which is what [foldNote] already does, in the
// same [persona.RoleSystem], verified against a live ollama on two models to
// survive the chat template mid-conversation. One turn per view entry either
// way, so nothing downstream has to learn a new shape.
//
// The text goes in whole. A fragment is short by definition, and the reason to
// hand it over at all is that it is the model's own last attempt.
//
// Its register moved with [standingInstruction] and [foldNote], for the same
// reason and no other: "do not read it back as a finished answer" is an order
// about a fact, and the fact does the work on its own. Running out of room is not
// a failure of the speaker's, so the sentence saying so should not read as a
// caution issued to a suspect. The commitment is unchanged — the words are exact,
// the incompleteness is stated, and the note still says what to do instead.
func fragmentNote(who string, u memory.Utterance) string {
	if strings.TrimSpace(u.Text) == "" {
		return fmt.Sprintf("[record] %s ran out of room here before saying anything at all. There is no text to show you: the record holds the attempt, marked unfinished, and nothing else. Nobody knows what was coming next, including whoever was writing it, so there is nothing here to reconstruct — say so if it matters.", who)
	}
	return fmt.Sprintf("[record] %s ran out of room here and stopped mid-answer. This is all of it, exactly as it arrived: %q. It ends where the room ran out rather than where the thought did, and the record keeps it that way, marked unfinished — so it is not a finished answer, and treating it as one would carry a false claim forward. If it needs finishing, say so.",
		who, strings.Join(strings.Fields(u.Text), " "))
}

// speakers names the distinct actors a fold absorbed, in the order they first
// appeared.
//
// Provenance is the one thing a summary must not lose, and this is where it
// survives consolidation into the model's own context: without it a fold reads
// as "twenty messages happened" with no account of whose they were, and a
// persona cannot tell its own absorbed words from the human's.
func speakers(c memory.Compaction) string {
	var names []string
	for h := range c.Handles() {
		names = append(names, h.Display)
	}
	switch len(names) {
	case 0:
		return ""
	case 1:
		return names[0]
	case 2:
		return names[0] + " and " + names[1]
	default:
		return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
	}
}

// note is the block drawn under the transcript: the request in flight, or the
// failure of the last one, or nothing.
//
// It is never a bit and it must never look like one. Bits are rows in a handle
// column; this is a rule across the transcript, which is the grammar the scar
// already uses for "an event here that nobody said". The rule is dashed where
// the scar's is solid, and that is the whole distinction — solid means settled
// and on the record, dashed means neither. It survives a terminal with no
// color, which is the only kind of distinction worth resting anything on.
//
// The first two states are mutually exclusive by construction: beginning a
// request clears the last failure, and a failure only arrives when a request
// ends. The third is not a request at all and sits under both, because a person
// waiting on an answer or looking at a failure has a more pressing question than
// why the answer before it stopped.
func (m Model) note() string {
	switch {
	case m.waiting.live:
		return m.pendingLine()
	case m.trouble.up():
		return m.troubleBlock()
	case m.endsUnfinished():
		return m.unfinishedLine()
	}
	return ""
}

// endsUnfinished reports whether the newest thing on the view is an answer that
// ran out of room.
//
// Read from the record rather than held in a field, which is the whole reason
// this is a method and not display state. A flag would have to be raised when a
// fragment is recorded and lowered on every path that puts something newer on
// screen, and the one it would be forgotten on is the path nobody thought of —
// which is how the notice this replaces came to need a sticky bit and a clock.
// Derived, it cannot go stale: it is recomputed every frame, it is true exactly
// while it is true, and the next bit of any kind ends it without anyone
// remembering to.
func (m Model) endsUnfinished() bool {
	bits := m.shown.Bits(m.store)
	if len(bits) == 0 {
		return false
	}
	u, ok := bits[len(bits)-1].Payload.(memory.Utterance)
	return ok && u.Truncated
}

// unfinishedLine is why the newest row stops mid-sentence, and what to do about
// it.
//
// The row above says the answer is unfinished; it has no room to say why, and
// there is no room on it for a fix. This is where the remedy lives, and the
// remedy is the reason the line exists: without it a person whose model has too
// small a context sees fragment after fragment with nothing on screen naming the
// cause, and the only copy of the sentence that would have helped is in
// persona/client.go on the path where the model said nothing at all — that is,
// present for the total failure and absent for the common one.
//
// It is emphatically not the trouble block, and the difference is the header
// that block cannot give up: "nothing was recorded". That claim is true of a
// failure and false of a fragment, so this says the opposite in its widest form
// and never borrows the block's words. The dashes are the same, because the
// grammar is the same — this is the harness talking, not a participant.
//
// What survives every cut is the reason, not the fix. An instruction with no
// stated cause is the defect [Model.troubleBlock] was rewritten to avoid: at
// sixty by ten it once showed "→ start it with: ollama serve" and nothing else.
func (m Model) unfinishedLine() string {
	return dim.Render(fit(max(m.width, 1),
		"╌╌ that answer ran out of room · it is on the record, marked unfinished ╌╌ ask for less, or give the model a larger context ╌╌",
		"╌╌ that answer ran out of room, and is on the record ╌╌ ask for less, or give the model a larger context ╌╌",
		"╌╌ that answer ran out of room ╌╌ ask for less, or give the model a larger context ╌╌",
		"╌╌ that answer ran out of room ╌╌ ask for less, or give it more room ╌╌",
		"╌╌ that answer ran out of room ╌╌ ask for less ╌╌",
		"╌╌ that answer ran out of room ╌╌",
		"╌╌ ran out of room ╌╌",
		"╌╌ ran out of room",
		"ran out of room",
		"out of room"))
}

// pendingLine is the antecedent for a wait that has no other visible sign.
//
// What survives every cut is the clock. The name is what you are waiting for
// and the keys are what you can do about it, but the number going up is the
// only thing on screen distinguishing a slow model from a wedged one, so it is
// last to go — down to a terminal too narrow to hold anything else.
func (m Model) pendingLine() string {
	who, since := m.waiting.who, elapsed(m.waiting.elapsed)
	return system.Render(fit(max(m.width, 1),
		fmt.Sprintf("╌╌ waiting on %s · %s ╌╌ enter is held until it answers · esc stops ╌╌", who, since),
		fmt.Sprintf("╌╌ waiting on %s · %s ╌╌ esc stops ╌╌", who, since),
		fmt.Sprintf("╌╌ waiting on %s · %s ╌╌", who, since),
		fmt.Sprintf("╌╌ waiting · %s ╌╌", since),
		fmt.Sprintf("╌╌ waiting %s", since),
		fmt.Sprintf("╌╌ %s", since),
		since))
}

// troubleBlock is what went wrong, said in the words the persona package wrote
// for it.
//
// Those sentences are the entire reason persona/errors.go exists: the two
// failures a new user hits first are ollama not running and the model not
// pulled, and each carries a problem and a one-line fix meant to be read at the
// keyboard. Reducing them to "error: request failed" throws that away, so they
// arrive here whole and are wrapped rather than cut — the one block on this
// surface that may take more than a row per thing, because it is prose to read
// and not a receipt to count.
//
// The header says the thing did not reach the record, and that is the sentence
// an auditor needs: a failure is not a bit, so this block is the only place the
// fact exists, and it says so about itself. The question that provoked it *is*
// on the record, which leaves the record showing a question with no answer —
// incomplete, but not a lie.
//
// The two headers are the one thing here that cannot be shared, and that is why
// [notice].unsaved exists. A save that failed is the mirror image of a request
// that failed: everything did reach the record, and the file behind it did not.
// Leading that with "nothing was recorded" would be this surface telling a
// person their words are gone while the words are on the screen above the
// sentence saying so. Everything below the header is common, because below the
// header both are the same shape — what went wrong, and what to do now.
//
// The fix is marked with an arrow rather than a color. Under a monochrome
// profile the two lines would otherwise be distinguished by position alone, and
// position is not a distinction anyone reads at speed.
func (m Model) troubleBlock() string {
	w := max(m.width, 1)

	// "Not recorded" is the claim, so it outlasts the key and the dashes. It is
	// the only place this fact exists — a failure is not a bit, so if the
	// header loses those two words there is nothing anywhere saying the record
	// does not hold what just happened.
	head := []string{
		"╌╌ nothing was recorded ╌╌ esc dismisses ╌╌",
		"╌╌ nothing was recorded ╌╌",
		"╌╌ not recorded ╌╌",
		"╌╌ not recorded",
		"not recorded",
	}
	// "Not on disk" is this one's claim, and it survives on the same terms and
	// for the same reason: the record being intact is visible — the transcript
	// is directly above this row — and the file being behind is the half nothing
	// else on the screen can say. So the news outlasts the reassurance, and the
	// reassurance is what the widest rungs spend their room on rather than a key.
	//
	// Two things this ladder does not do, both deliberate.
	//
	// It does not say "this session", which was a quantifier and was false. A
	// save that fails at the fortieth bit leaves thirty-nine on disk in a whole,
	// valid file — [record.save] writes the record entire, every time — so a
	// header claiming the session is not on disk overstates the loss in exactly
	// the way "nothing was recorded" would, one place further along. Naming the
	// two places and no amount is the true form, and it is true whether the file
	// is one change behind or forty.
	//
	// And it does not print "esc dismisses". The footer already prints "esc
	// dismiss" while any notice is up, so it would be the second mention — and
	// this is the one notice where dismissal is not the end of anything. The
	// other block reports an event that is over; this one reports a condition
	// that is still true after the key is pressed, and a header advertising the
	// key that hides it teaches a person to treat it as noise.
	if m.trouble.unsaved {
		head = []string{
			"╌╌ recorded here, not on disk ╌╌",
			"╌╌ recorded, not on disk ╌╌",
			"╌╌ not on disk ╌╌",
			"╌╌ not on disk",
			"not on disk",
		}
	}

	var b strings.Builder
	b.WriteString(warm.Render(fit(w, head...)))

	for _, l := range indent(m.trouble.problem, w) {
		b.WriteString("\n" + warm.Render(l))
	}
	if m.trouble.fix != "" {
		for i, l := range indent(m.trouble.fix, w-2) {
			lead := "  → "
			if i > 0 {
				lead = "    "
			}
			b.WriteString("\n" + dim.Render(clip(lead+strings.TrimPrefix(l, "  "), w)))
		}
	}
	return b.String()
}

// indent wraps s to the width it was given, two columns in, and returns the
// rows. Wrapping and not clipping: every other row on this surface is a receipt
// whose value is that it can be counted, and cutting one is honest. This is a
// sentence somebody has to act on, and half of it is no use.
func indent(s string, width int) []string {
	room := max(width-2, 1)
	var out []string
	for _, l := range strings.Split(ansi.Wrap(strings.Join(strings.Fields(s), " "), room, ""), "\n") {
		out = append(out, clip("  "+l, max(width, 1)))
	}
	return out
}

// elapsed is how long the wait has been, in the shortest honest form. Seconds
// while there are only seconds, because "0m07s" reads as a stopwatch and a
// person waiting on a model wants a number, not an interval.
func elapsed(d time.Duration) string {
	s := int(d.Round(time.Second) / time.Second)
	if s < 60 {
		return fmt.Sprintf("%ds", s)
	}
	return fmt.Sprintf("%dm%02ds", s/60, s%60)
}

// speakerName is the display half of the default persona's handle: the model's
// name without its tag.
//
// A model name and not an invented one, on purpose. [persona.Persona] separates
// the name from the weights precisely so that a persona can be a character, and
// nobody has written one here — so the honest default is the thing that is
// actually answering. It also puts which model spoke into the handle column of
// every row it wrote, which is the first question anybody auditing this
// transcript later is going to ask.
func speakerName(model string) string {
	name, _, _ := strings.Cut(model, ":")
	if name == "" {
		return model
	}
	return name
}
