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
// It is a *thinking* model, which makes one thing in [Client.Reply] load-bearing
// rather than defensive: asked with no "think" setting at all, ollama 0.17.7
// returns this model's reasoning in a field beside the answer, and the client
// drops it. Verified by asking a running one. Swapping this default for a model
// that does not reason would hide any regression in that, so the tests keep a
// reasoning reply in the table regardless of what is set here.
//
// Where that guard stops, stated because the comments around it used to imply
// otherwise: the split is the server's and the chat template's doing. A model
// that writes `<think>…</think>` into its content instead hands the reasoning
// over as ordinary text, and it is recorded as something the persona said.
// Neither available fix is worth its cost. Sending "think": true would make the
// split structural, but ollama 0.17.7 answers it with 400 `"llama3.2:1b" does
// not support thinking` on a model that cannot — verified the same way — so it
// would break every non-reasoning persona to protect against a leak in some.
// Stripping the tags out of content would mean cutting text from a message
// because it contains five particular characters, and a mangled bit in a store
// that never forgets is a worse record defect than the one it fixes.
// TestReplyKeepsInlineReasoningInTheAnswer pins the behavior we do have.
//
// All of that is about the *record*, and what gets *sent* is a separate question
// with a different answer — see [Escape]. Reasoning left in content is a bit
// that is honest about what was said; a control marker left in content is a
// forged turn the next time that bit is handed to a model. Recording keeps the
// text; transmitting derives from it.
const DefaultModel = "qwen3.5:latest"

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
