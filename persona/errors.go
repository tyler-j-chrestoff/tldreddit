package persona

// Kind says which of the failures this is, for a caller that wants to act
// rather than print — offer to run `ollama pull`, retry, or give up.
//
// The list is short on purpose: a kind exists only where a person would
// plausibly do something different about it. Splitting further would produce
// distinctions the caller cannot use, and every one of them would still have
// to be handled by somebody's switch.
type Kind string

const (
	// Unusable means the call could never have been made: no model named, or a
	// base URL that is not a URL. Nothing was sent.
	Unusable Kind = "unusable"

	// Unreachable means nothing answered at the address. The overwhelmingly
	// common cause is that ollama is not running.
	Unreachable Kind = "unreachable"

	// Timeout means we stopped waiting. Distinct from Unreachable because the
	// server was there, and distinct from Canceled because it was our clock
	// that ran out rather than the caller's decision.
	Timeout Kind = "timeout"

	// Canceled means the caller called it off through the context.
	Canceled Kind = "canceled"

	// NoModel means ollama is running and does not have the model. This is the
	// one failure with a one-line fix, and it is the second thing a new user
	// hits after forgetting to start the server.
	NoModel Kind = "no-model"

	// Rejected means ollama answered, explained why it would not do the thing,
	// and its explanation is in the message.
	Rejected Kind = "rejected"

	// Garbled means something answered but not in a way this package can read:
	// a different server at that address, a version that changed the response
	// shape, or a reply with no reply in it.
	Garbled Kind = "garbled"
)

// Error is a failure written for the person at the keyboard.
//
// Two fields rather than one because the two questions are different and a
// person reading a terminal at speed answers them separately: what went wrong,
// and what do I do now. Fix is empty when there is nothing honest to put
// there — an invented suggestion is worse than none, because the reader will
// spend time on it.
//
// This breaks the Go convention that error strings are lowercase fragments
// without punctuation or newlines, and it does so knowingly. That convention
// exists because error strings get composed into larger ones by libraries;
// these are terminal messages, the last thing anyone does with them is show
// them, and a message that reads like a sentence is the entire point of a
// project whose thesis is legibility. What it costs: an [Error] wrapped by
// somebody's fmt.Errorf("...: %w") will read badly, so wrap the cause, not
// this.
//
// Err carries the underlying failure — a transport error, or ollama's own
// words — so that errors.Is and errors.As still work and nothing is actually
// lost. It is never part of the message: "dial tcp 127.0.0.1:11434: connect:
// connection refused" tells a person nothing they can act on, and printing it
// alongside a sentence that does teaches them to skip the sentence.
type Error struct {
	Kind    Kind
	Problem string
	Fix     string
	Err     error
}

func (e *Error) Error() string {
	if e.Fix == "" {
		return e.Problem
	}
	return e.Problem + "\n" + e.Fix
}

// Unwrap exposes the cause for errors.Is — most usefully
// errors.Is(err, context.Canceled), which stays true through the rewrite.
func (e *Error) Unwrap() error { return e.Err }
