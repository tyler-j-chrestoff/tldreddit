// Package persona gives a model a name and a standing instruction, and asks it
// things.
//
// A persona is not a model. Two personas can sit on one set of weights and be
// two different participants in the record, so the thing the record names has
// to be the persona, and the weights are only where it happens to run. That is
// what [Persona.Handle] encodes: the model is the ref, stable within the
// channel it was seen on, and the persona's name is what it called itself at
// the time — [memory.Handle]'s own split, used as intended.
//
// Nothing here remembers anything, and that is the point. A [Client] is a way
// to ask; the turns handed to it come from the caller's view of the record. A
// chat client that kept its own transcript alongside the store would be a
// second record, invisible, unaddressed, and free to disagree with the first —
// which is the failure this whole project is a bet against.
//
// What it does do to a message on the way out is derive rather than copy. The
// record keeps what was said, control tokens and all; the wire carries a form of
// it that cannot draw a role boundary the conversation never had. [Escape] is
// the whole of that, and it is the only difference between the two.
//
// The wire format is ollama's `/api/chat` with streaming off, verified against
// ollama 0.17.7 by calling a running one, not remembered. Streaming is a later
// decision: a reply that arrives all at once is one bit, and a reply that
// arrives in pieces raises the question of what gets recorded and when, which
// deserves its own answer rather than being settled by an implementation
// detail.
//
// Errors here are written to be read by the person at the keyboard. See
// [Error] for why, and for what that costs.
package persona

import "github.com/tyler-j-chrestoff/tldreddit/memory"

// DefaultModel is what the demo reaches for when nothing else is chosen.
//
// It is a name on someone's disk, not a claim about quality: it is a recent
// open model that this machine actually has pulled. Nothing in this package
// depends on it — a Persona names its own model, and this constant exists so
// that the choice is written down in one place instead of being retyped at
// each call site.
//
// It is a *thinking* model, and that is why [Persona.Think] exists and why it is
// off unless somebody asks. Left to itself this model writes out its monologue
// before answering, which a person waiting at the keyboard pays for on every
// turn: roughly 8–66s against a flat 0.22–0.25s for the same one-line question,
// eight samples on one machine against ollama 0.17.7. Swapping this default for
// a model that does not reason would hide any regression in that, so the tests
// keep a reasoning reply in the table regardless of what is set here.
//
// Where the client's guard stops, stated because the comments around it used to
// imply otherwise: whether reasoning arrives in a field of its own is the
// server's and the chat template's doing, and "think": false is a request rather
// than a guarantee. A model that writes `<think>…</think>` into its content
// hands the reasoning over as ordinary text, and it is recorded as something the
// persona said. Stripping the tags would mean cutting text out of a message
// because it contains five particular characters, and a mangled bit in a store
// that never forgets is a worse record defect than the one it fixes.
// TestReplyKeepsInlineReasoningInTheAnswer pins the behavior we do have.
//
// Corrected here rather than left standing, and the correction is kept because
// the shape of the error is worth more than the fact. This paragraph used to
// argue that no "think" setting could be sent at all — that doing so "would
// break every non-reasoning persona." What had actually been measured was one
// value: ollama 0.17.7 answers "think": true with 400 `"llama3.2:1b" does not
// support thinking` on a model that cannot reason. That is still true. What was
// inferred from it, and written down as though measured, was a claim about the
// whole field.
//
// The other two values were then measured against the same running server, and
// neither behaves like true. "think": false is accepted by llama3.2:1b — 200,
// an ordinary reply, indistinguishable from omitting the key. Omitting the key
// is *not* the same as false on a model that can reason: qwen3.5:latest answers
// an absent "think" with its monologue on, which is why [chatRequest] sends the
// field with no omitempty. Three values, three behaviours, and a measurement of
// one of them is not a measurement of the field.
//
// All of that is about the *record*, and what gets *sent* is a separate question
// with a different answer — see [Escape]. Reasoning left in content is a bit
// that is honest about what was said; a control marker left in content is a
// forged turn the next time that bit is handed to a model. Recording keeps the
// text; transmitting derives from it.
const DefaultModel = "qwen3.5:latest"

// DefaultWindow is how much room a persona gets when it does not say, in
// tokens.
//
// It exists so that "nobody chose" and "the server's own fallback" are
// different states of this program. ollama's fallback is 4,096 and it is not a
// statement about the model — qwen3.5:latest and ministral-3:14b advertise
// 262,144, llama3.2:1b 131,072, qwen3:8b 40,960, all read from /api/show on
// this machine — so a client that sends nothing gets a sixtieth of what the
// default model can hold, and gets it silently.
//
// 32,768 rather than the model's own ceiling, and the reason is somebody else's
// hardware. The window is a key-value cache the server allocates: measured here
// on qwen3.5:latest, 32,768 costs about 1.2 GB more resident than 4,096, and
// the 262,144 that model advertises would want roughly 11 GB on top of the
// weights. A default that helps itself to the maximum would be this program
// spending a stranger's memory on a conversation it has not had yet.
//
// It is a floor to argue with, not a measurement of what is needed. What is
// measured is that it is enough for the conversations this program has now: an
// exchange of 401 turns off a shape like this project's own record came to
// 26,605 tokens and fit inside it whole. A persona that needs more says so.
const DefaultWindow = 32768

// Persona is a participant that happens to be a model.
type Persona struct {
	// Name is what it calls itself, and it lands in the record as the display
	// half of a [memory.Handle] — point-in-time, never rewritten. Renaming a
	// persona therefore does not rename what it already said, which is correct:
	// the old bits record what was true when they were written.
	Name string

	// Model is the ollama model name, tag included ("qwen3.5:latest"). It is
	// also the stable half of the handle, so changing it makes a different
	// participant even under the same name — deliberately. The whole reason to
	// keep provenance is to be able to ask later which weights said this.
	Model string

	// System is the standing instruction, sent ahead of every conversation.
	// It is not part of the handle: two personas differing only here are the
	// same participant as far as the record is concerned, which is a claim
	// worth noticing. If that turns out to be wrong, the fix is to put the
	// system prompt's hash in the ref, and it will be a deliberate change.
	//
	// Empty means send no system message at all, rather than sending an empty
	// one — the model's own default template is a more honest fallback than a
	// blank instruction.
	System string

	// Temperature goes on the wire exactly as written, including zero, which
	// ollama reads as greedy decoding rather than as "unset". This package
	// does not substitute a default: a persona whose voice depends on a number
	// nobody wrote down cannot be reproduced, and reproducibility is the only
	// reason the record is worth keeping. State it.
	Temperature float64

	// Think lets a reasoning model write out its monologue before answering.
	//
	// It sits here rather than on [Client] for the reason Temperature does,
	// and the placement is worth defending because latency is what makes
	// somebody want to move it. Thinking is not a property of the connection:
	// it changes what the model says, those replies become bits in a store
	// that never forgets, and two personas sharing one server can reasonably
	// disagree about it — which a field on the connection cannot express. A
	// setting that shapes the voice belongs to the participant.
	//
	// False is the zero value and sends "think": false, deliberately rather
	// than by omission. A reasoning model left to itself writes out its
	// monologue before answering, and the person at the keyboard pays for it
	// on every turn. Measured against ollama 0.17.7 and qwen3.5:latest at
	// temperature 0, one one-line question: off is 0.22–0.25s and 8 evaluated
	// tokens every time; on is roughly 8–66s and 636–5,120 tokens, of which
	// 2,350–17,833 characters are monologue that gets discarded. The spread is
	// as much of the finding as the middle — the cost is not merely large, it
	// is unpredictable, so quote a range and never a single sample.
	//
	// Set it to true to compare. What was measured is latency, not quality:
	// whether a persona answers *better* with the monologue in front of it is
	// open, and it is a question somebody should be able to run rather than
	// argue. Two things whoever runs it should know before reading a result.
	// True on a model that cannot reason is a hard failure and not a no-op —
	// ollama 0.17.7 answers 400 `"llama3.2:1b" does not support thinking`,
	// which arrives as a Rejected [Error]. And a model asked to think can
	// return thousands of characters of monologue and *no answer at all*,
	// which [Client.Reply] surfaces as a Garbled "thought for N characters and
	// then said nothing"; observed on qwen3.5:latest at 17,833 characters.
	// That is the thinking arm behaving normally, not a regression in it.
	Think bool

	// Window is how much of the conversation the model is given room to read,
	// in tokens — ollama's num_ctx, stated on every request rather than left
	// to the server.
	//
	// Zero means [DefaultWindow], and the one thing it never means is "let the
	// server decide." Omitting num_ctx is not a neutral act: ollama 0.17.7
	// falls back to 4096 whatever the model can hold, drops the oldest turns
	// to make a longer conversation fit, and returns an ordinary reply with
	// nothing in it to say that it did. Measured through this client against a
	// live 0.17.7, one 401-turn conversation of 26,605 tokens sent twice: with
	// no num_ctx on the wire the server read 4,045 of them in 125.1s, and with
	// num_ctx 32768 it read all 26,605 in 13.5s. Same bytes, six and a half
	// times as much of them reaching the model, nine times faster — cutting a
	// conversation down is work — and no field of either reply distinguishing
	// the two. A record that keeps the reply has to be able to say what
	// produced it, so this number is written down for the same reason
	// Temperature is.
	//
	// It sits here rather than on [Client] on the argument Think's placement
	// settles: a setting that changes what the model says belongs to the
	// participant, not to the connection. Two personas on one server can
	// reasonably want different windows — one summarising a long stretch, one
	// answering a question — which a field on the connection cannot express.
	// What that placement costs is real and is worth knowing: ollama loads one
	// instance per model *and window*, so two personas alternating on the same
	// model with different windows make the server reload between turns, about
	// four seconds against a tenth of a second for a repeat at the same window.
	//
	// Asking for more than the model can hold is not an error and not honoured
	// either: 0.17.7 clamps, silently. Measured — qwen3:8b advertises 40,960
	// and a request for 45,000 came back as an ordinary reply with the instance
	// loaded at 40,960. [Client.WindowFor] is how a caller finds out what the
	// ceiling is; nothing here checks it, because that would put a second HTTP
	// round trip in front of every turn to re-learn a fact about a file on
	// disk.
	//
	// The cost of a large window is memory on whoever's machine ollama is,
	// which is why this is a number a person states rather than a maximum the
	// program helps itself to. Measured on this machine with qwen3.5:latest,
	// resident size as ollama's own /api/ps reports it: 8.65 GB at 4,096,
	// 9.15 GB at 16,384, 9.85 GB at 32,768, 11.33 GB at 65,536 — about 41 KB
	// a token. At the 262,144 that model advertises, the same arithmetic wants
	// roughly 11 GB of key-value cache on top of the weights, which is why
	// [DefaultWindow] is nowhere near it.
	//
	// Negative is a caller mistake and [Client.Reply] refuses it rather than
	// quietly reading it as zero.
	Window int
}

// window is the number that goes on the wire — never zero, because zero is
// what makes the server choose.
func (p Persona) window() int {
	if p.Window == 0 {
		return DefaultWindow
	}
	return p.Window
}

// Handle is how this persona appears in the record.
//
// The shape — "ollama/<model>" as the ref — says where the actor was observed
// and which weights answered, in one field that is unique within a channel.
// It is not a URL and must not become one: the machine ollama happens to run
// on is deployment, not identity, and putting a host in here would make the
// same persona a different participant after a move.
func (p Persona) Handle() memory.Handle {
	return memory.Handle{Ref: "ollama/" + p.Model, Display: p.Name}
}

// Role is who a turn came from, in the vocabulary the chat API understands.
//
// It is a string rather than an int because it is sent verbatim and read back
// verbatim; a numbered enum would need a table in both directions and would
// print as a number in the one place it matters, which is a person reading an
// error.
type Role string

const (
	// RoleSystem is the standing instruction. [Client.Reply] sends the
	// persona's own System under this role, first; a caller may also pass
	// turns with it, and they are sent where they sit.
	RoleSystem Role = "system"

	// RoleUser is the human, or whoever is asking.
	RoleUser Role = "user"

	// RoleAssistant is the persona's own past turns, handed back so it can see
	// what it already said. This is where fold and unfold will eventually show
	// up: what the caller chooses to include here *is* the persona's memory.
	RoleAssistant Role = "assistant"
)

// Turn is one exchange in a conversation, as the model will read it.
//
// It is deliberately not a [memory.Bit]. A bit is a recorded occurrence with
// provenance and an address; a turn is what gets handed to a model this time.
// Choosing which bits become turns is a decision about what the persona is
// allowed to see, and it belongs to the caller holding the view — not to the
// client that makes the HTTP request.
//
// Not named Msg, though the wire calls these messages: in a Bubble Tea program
// a Msg is a thing that arrives at Update, and the code that wires a persona
// into the interface will hold both kinds at once.
type Turn struct {
	Role    Role
	Content string
}
