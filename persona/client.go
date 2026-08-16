package persona

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	// DefaultBaseURL is where ollama listens when nobody has said otherwise.
	DefaultBaseURL = "http://localhost:11434"

	// DefaultTimeout is a backstop, not a service level, and it is long on
	// purpose. A two-sentence reply from a mid-sized model on a developer's GPU
	// measured about 14 seconds here, and the first call after a cold start
	// also pays for loading the weights, which can dwarf that. Anything under a
	// minute cuts off replies that were going to arrive.
	//
	// It exists at all so that a wedged connection eventually reports itself
	// instead of leaving a spinner up forever. The real control is the context:
	// a caller that knows how long it is willing to wait should say so there.
	DefaultTimeout = 5 * time.Minute

	// maxBody caps what we will read back. It guards one specific mistake —
	// a base URL pointed at something that is not ollama and streams
	// indefinitely — and is far above any real reply: the largest models here
	// cannot emit a megabyte of text in one non-streamed response, because
	// their context window will not hold it.
	maxBody = 4 << 20

	// peek is how much of an unrecognized body [because] will look at to quote
	// it back. Far more than the two hundred runes that can survive into a
	// message, and three orders of magnitude below maxBody — which is the point:
	// that fallback exists for a short complaint from another server, and
	// reading four megabytes of somebody's HTML to print two lines of it is work
	// done for nobody.
	peek = 4 << 10
)

// Client talks to one ollama server.
//
// The zero value works and points at [DefaultBaseURL], because the common case
// is a local ollama on its default port and making the caller construct
// something to say so is ceremony. Both fields are read at call time rather
// than resolved in a constructor, so a Client that has been copied or built as
// a struct literal behaves the same as one that has not.
//
// It is safe for concurrent use exactly as far as [http.Client] is, which is to
// say: fine, as long as nobody writes the fields while a call is in flight.
type Client struct {
	// BaseURL is the server's root, with no trailing path — the "/api/chat"
	// is this package's business. Empty means DefaultBaseURL.
	BaseURL string

	// HTTP is the client to send with. Nil means one with DefaultTimeout. Set
	// it to control timeouts, proxies, or — in tests — to stand in for the
	// network entirely.
	HTTP *http.Client
}

func (c *Client) base() string {
	if c.BaseURL == "" {
		return DefaultBaseURL
	}
	return c.BaseURL
}

func (c *Client) httpClient() *http.Client {
	if c.HTTP == nil {
		return &http.Client{Timeout: DefaultTimeout}
	}
	return c.HTTP
}

// chatRequest and chatReply are this package's own, rather than json tags on
// [Turn] and a return type. The wire format belongs to ollama and can change
// under us; Turn belongs to us. Keeping them apart means a field added to Turn
// for our own purposes cannot silently start being sent to a model, and a
// field ollama adds cannot silently become part of our vocabulary.
//
// Verified against ollama 0.17.7 by calling a live server.
type chatRequest struct {
	Model    string    `json:"model"`
	Messages []wireMsg `json:"messages"`
	Stream   bool      `json:"stream"`
	Options  options   `json:"options"`
}

type wireMsg struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

type options struct {
	Temperature float64 `json:"temperature"`
}

type chatReply struct {
	Message struct {
		Role    Role   `json:"role"`
		Content string `json:"content"`

		// Thinking is a reasoning model's scratchpad, in the cases where ollama
		// returns it beside the answer rather than inside it. It is read here
		// for exactly one purpose: so that a reply which is all reasoning and no
		// answer can say so instead of looking like silence.
		//
		// It must never be concatenated onto Content. A persona's chain of
		// thought is not something it said; appending it would put hundreds of
		// words the participant never uttered into the record, where — the
		// store being append-only — they would stay forever and get folded into
		// aggregates that nothing can undo.
		//
		// Whether the split happens at all is the server's and the chat
		// template's business, not this client's: no "think" setting is sent, so
		// a model that writes `<think>…</think>` into its content hands that
		// over as ordinary text and it is recorded as something the persona
		// said. See [DefaultModel] for why that is left alone rather than
		// forced or stripped.
		Thinking string `json:"thinking"`
	} `json:"message"`

	// Done is whether this is the whole reply.
	//
	// A pointer because absent and false are different answers. A server too old
	// to send the field is saying nothing, and silence is not a claim; one that
	// sends false is saying outright that more was coming — and handing that
	// back as a finished answer would put a fragment into a store that never
	// forgets, with nothing on it to say it is one.
	Done *bool `json:"done"`

	// DoneReason is why generation stopped: "stop" for a model that finished,
	// "length" for one that ran out of room. Verified against ollama 0.17.7,
	// which sends it on every non-streamed reply.
	DoneReason string `json:"done_reason"`

	// Error is ollama's own explanation. It arrives beside a non-2xx status,
	// and is read on a 2xx as well because a server that says it failed while
	// claiming success should be believed about the failure.
	Error string `json:"error"`
}

// Answer is what a persona said, and whether it got to finish saying it.
//
// The second field exists because the first one lies without it. A reply cut
// off by the context window is a well-formed sentence that simply stops, and
// it goes into an append-only store as though it were the whole thought — the
// same class of defect as letting a model's scratchpad through, and worse in
// one way: reasoning in the record is obvious on sight, and a truncation is
// invisible forever.
type Answer struct {
	// Text is the reply, trimmed of the whitespace chat templates add and
	// otherwise exactly as it came.
	Text string

	// Truncated is true when ollama stopped because it ran out of room
	// ("done_reason":"length") rather than because the model was finished.
	//
	// A bool a caller can ignore is a weak guard, and it is knowingly the
	// smaller half of the fix. The real one is making truncation visible in
	// the record — a bit that says it is a fragment — and that belongs to the
	// code that writes bits, not to the code that fetches text. Whoever wires
	// this into the store inherits that obligation here: storing a truncated
	// Answer as an ordinary utterance is a silent, permanent falsehood about
	// what the persona said.
	Truncated bool
}

// Reply asks the persona to answer, given the conversation so far, and returns
// what it said.
//
// The persona's System is sent first, ahead of everything in turns, and turns go
// in the order given: a chat model reads position as time, so reordering turns
// is changing what happened. Nothing is dropped or summarized here. What the
// persona is allowed to see is the caller's decision, made against its view of
// the record, and that decision is the interesting one — this function only
// carries it.
//
// One thing does change between the record and the wire, and it is the only
// thing: a control marker in the text is escaped, so that content cannot draw a
// role boundary that this client did not draw. See [Escape], which says what
// that is defending against and what it measurably costs.
//
// The context is the real control over how long this takes. Cancelling it
// aborts the request in flight.
//
// It returns an [Answer] rather than a string so that a reply which ran out of
// room can say so. A caller is free to ignore that and store the text anyway;
// what it is not free to do is claim it never had the chance to know.
func (c *Client) Reply(ctx context.Context, p Persona, turns []Turn) (Answer, error) {
	// An error rather than a panic, unlike the caller mistakes memory.Cool and
	// memory.Store.Put refuse. Those guard the integrity of the record, where a
	// wrong answer is worse than no program. This is ordinary bad input, and a
	// panic here would land inside a tea.Cmd goroutine, killing the program over
	// a raw terminal and taking the session's unwritten work with it. Do not
	// "restore consistency" by turning it back into a panic.
	if p.Model == "" {
		return Answer{}, &Error{
			Kind:    Unusable,
			Problem: fmt.Sprintf("the persona %q names no model, so there is nothing to ask", p.Name),
			Fix:     "give it a model — for example: " + DefaultModel,
		}
	}

	// Checked here rather than left to the transport. An address with no
	// scheme, or one the http package will not speak, fails at Do with an
	// error that looks exactly like a server being down — and "start ollama"
	// is advice that cannot work, so the reader follows it and learns nothing.
	if err := usable(c.base()); err != nil {
		return Answer{}, err
	}

	// What goes on the wire is derived from what was recorded rather than being
	// the same bytes: [Escape] breaks any control marker in the text so that a
	// recorded message cannot spell a role boundary the conversation never had.
	// The turns themselves are left exactly as the caller handed them over —
	// they came from bits, and a bit is evidence.
	wire := make([]wireMsg, 0, len(turns)+1)
	if p.System != "" {
		safe, _ := Escape(p.System)
		wire = append(wire, wireMsg{Role: RoleSystem, Content: safe})
	}
	for _, m := range turns {
		safe, _ := Escape(m.Content)
		wire = append(wire, wireMsg{Role: m.Role, Content: safe})
	}

	// Marshal cannot fail for these types — no channels, no functions, no
	// cycles. Checked anyway, because the alternative is a discarded error that
	// a future field could quietly start filling.
	body, err := json.Marshal(chatRequest{
		Model:    p.Model,
		Messages: wire,
		Stream:   false,
		Options:  options{Temperature: p.Temperature},
	})
	if err != nil {
		return Answer{}, &Error{
			Kind:    Unusable,
			Problem: "this conversation could not be encoded to send",
			Err:     err,
		}
	}

	endpoint := strings.TrimSuffix(c.base(), "/") + "/api/chat"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return Answer{}, unusableAddress(c.base(), err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return Answer{}, c.sendFailed(ctx, p, err)
	}
	defer resp.Body.Close()

	// Read before switching on the status: every branch below wants the body,
	// and ollama explains itself in it on the failures as well as the success.
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		// A read that fails after the headers arrived is usually the server, but
		// it is not always: a cancelled context and an expired [http.Client]
		// timeout both surface from here rather than from Do, and the persona
		// loop cancels routinely. Reporting those as the server stopping sends a
		// person who pressed ctrl-c to somebody else's log to find out why.
		if stopped := c.calledOff(ctx, p, err); stopped != nil {
			return Answer{}, stopped
		}
		return Answer{}, &Error{
			Kind:    Garbled,
			Problem: fmt.Sprintf("ollama at %s stopped part way through its answer", c.base()),
			Fix:     "try again — if it keeps happening, check the ollama server's own log",
			Err:     err,
		}
	}
	if len(raw) > maxBody {
		return Answer{}, &Error{
			Kind:    Garbled,
			Problem: fmt.Sprintf("whatever is at %s sent more than a reply could be", c.base()),
			Fix:     "check that the address points at ollama and not at another server",
		}
	}

	// A 404 is ambiguous and only the body resolves it. This client cannot know
	// that its request reached /api/chat at all — a mistyped base URL, or a
	// different local server at that port, answers a route it does not have with
	// the same status. So the claim "the model is not installed" is made only
	// when the server's own sentence supports it, because the advice attached to
	// it is `ollama pull`, and sending someone to pull a model they already have
	// tells them the opposite of the truth in the one failure that has a one-line
	// fix.
	if resp.StatusCode == http.StatusNotFound {
		said := ollamaSays(raw)
		if aboutTheModel(said, p.Model) {
			return Answer{}, &Error{
				Kind:    NoModel,
				Problem: fmt.Sprintf("ollama is running, but the model %q is not installed", p.Model),
				Fix:     "pull it with: ollama pull " + p.Model,
				Err:     errors.New(said),
			}
		}
		// The other server's own words are kept as the cause rather than shown:
		// "no route matched" is a fact about somebody else's router, and it
		// belongs where a debugger will find it, not in the sentence.
		var cause error
		if said != "" {
			cause = errors.New(said)
		}
		return Answer{}, &Error{
			Kind:    Garbled,
			Problem: fmt.Sprintf("something is listening at %s, but it does not answer like ollama", c.base()),
			Fix:     "check that the address points at ollama and not at another server",
			Err:     cause,
		}
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		// Err carries ollama's own sentence when there is one, and nothing when
		// there is not. The alternative — keeping the raw body — puts up to
		// maxBody of somebody else's HTML inside an error value that will be
		// printed by whoever catches it.
		var cause error
		if said := ollamaSays(raw); said != "" {
			cause = errors.New(said)
		}
		return Answer{}, &Error{
			Kind: Rejected,
			Problem: fmt.Sprintf("ollama refused the request to %q: %s",
				p.Model, because(raw, resp.StatusCode)),
			Err: cause,
		}
	}

	var reply chatReply
	if err := json.Unmarshal(raw, &reply); err != nil {
		return Answer{}, &Error{
			Kind:    Garbled,
			Problem: fmt.Sprintf("ollama at %s answered, but not in a shape this client can read", c.base()),
			Fix:     "check the ollama version — this client speaks /api/chat with streaming off",
			Err:     err,
		}
	}
	if reply.Error != "" {
		return Answer{}, &Error{
			Kind:    Rejected,
			Problem: fmt.Sprintf("ollama refused the request to %q: %s", p.Model, reply.Error),
			Err:     errors.New(reply.Error),
		}
	}

	// The server said in the body that this is not the whole thing. ollama 0.17.7
	// answers a request with streaming off with done:true — checked against a
	// running one — which is exactly why this is worth reading: the reachable
	// cause is something else at this address, the same reader maxBody and the
	// 404 body-sniffing are written for.
	//
	// Garbled and not Truncated, though both mean "there was more". Truncated is
	// a claim about the model — it ran out of room — and the caller acts on it by
	// shortening the conversation, which would do nothing here. This is a claim
	// about the protocol: we asked for one reply and got a piece of a stream, so
	// the text is not a short answer, it is the first slice of one.
	if reply.Done != nil && !*reply.Done {
		return Answer{}, &Error{
			Kind:    Garbled,
			Problem: fmt.Sprintf("whatever is at %s sent a piece of a reply rather than the whole one", c.base()),
			Fix:     "check that the address points at ollama — this client asks for streaming off",
		}
	}

	// Trimmed because chat templates leave leading and trailing newlines that
	// the persona did not say. Everything inside the reply is left exactly as
	// it came: it is about to become a bit, and a bit is evidence.
	text := strings.TrimSpace(reply.Message.Content)
	truncated := reply.DoneReason == "length"
	if text == "" {
		// Cut off before saying anything is a different problem from refusing
		// to answer, and it has a different fix: the room ran out, so ask for
		// less or give the model more of it.
		if truncated {
			return Answer{}, &Error{
				Kind:    Garbled,
				Problem: fmt.Sprintf("the model %q ran out of room before it said anything", p.Model),
				Fix:     "shorten the conversation being sent, or give the model a larger context",
			}
		}
		if reply.Message.Thinking != "" {
			return Answer{}, &Error{
				Kind: Garbled,
				Problem: fmt.Sprintf("the model %q thought for %d characters and then said nothing",
					p.Model, len(reply.Message.Thinking)),
				Fix: "reasoning is not an answer and is not recorded — try again, or use a model that answers directly",
			}
		}
		return Answer{}, &Error{
			Kind:    Garbled,
			Problem: fmt.Sprintf("the model %q answered with nothing at all", p.Model),
			Fix:     "try again, or check the persona's instructions for something that forbids answering",
		}
	}
	return Answer{Text: text, Truncated: truncated}, nil
}

// calledOff reports the failures that are somebody's decision rather than the
// server's doing, and nil for one that is neither.
//
// It is its own function because a request can die in two places — the send and
// the read of the body — and from the reader's side those are one event: they
// pressed ctrl-c, or they ran out of patience. The classification has to be the
// same in both, or which sentence they get depends on how far the bytes had got.
func (c *Client) calledOff(ctx context.Context, p Persona, err error) *Error {
	// The caller's context first: when a deadline expires, the failure is also
	// a timeout and also a transport error, and the caller's own decision is
	// the truest account of it.
	if ctxErr := ctx.Err(); ctxErr != nil {
		problem := fmt.Sprintf("the reply from %q was called off before it arrived", p.Model)
		if errors.Is(ctxErr, context.DeadlineExceeded) {
			problem = fmt.Sprintf("the reply from %q did not arrive before the deadline", p.Model)
		}
		return &Error{Kind: Canceled, Problem: problem, Err: ctxErr}
	}

	// Matched by behavior rather than by type. A send that times out arrives
	// wrapped in a [url.Error]; the same [http.Client] timeout firing during the
	// body read arrives bare, as net/http's own error type, which is unexported
	// and can only be recognized by the one method it exists to answer.
	var expired interface{ Timeout() bool }
	if errors.As(err, &expired) && expired.Timeout() {
		waited := "in time"
		if t := c.httpClient().Timeout; t > 0 {
			waited = "within " + t.String()
		}
		return &Error{
			Kind:    Timeout,
			Problem: fmt.Sprintf("ollama at %s did not answer %s", c.base(), waited),
			Fix:     "a first reply is slow while the model loads — try again, or allow more time",
			Err:     err,
		}
	}
	return nil
}

// sendFailed turns a transport error into something a person can act on. This
// is the path a new user hits first and it has one overwhelmingly likely cause,
// so it says that cause plainly rather than reciting what the socket reported.
func (c *Client) sendFailed(ctx context.Context, p Persona, err error) error {
	if stopped := c.calledOff(ctx, p, err); stopped != nil {
		return stopped
	}

	// A name that does not resolve is a different mistake from a server that
	// is not up, and "start ollama" would be the wrong advice for it. Detected
	// by type rather than by string, since the message is the platform's.
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return &Error{
			Kind:    Unreachable,
			Problem: fmt.Sprintf("no machine called %q could be found", dnsErr.Name),
			Fix:     "check the address ollama was given: " + c.base(),
			Err:     err,
		}
	}

	return &Error{
		Kind:    Unreachable,
		Problem: fmt.Sprintf("ollama is not answering at %s — it does not appear to be running", c.base()),
		Fix:     "start it with: ollama serve",
		Err:     err,
	}
}

// usable rejects a base URL that cannot address an HTTP server, and returns
// nil for one that can. It says nothing about whether anything is listening —
// that is the network's answer, and it comes later.
func usable(base string) error {
	u, err := url.Parse(base)
	if err != nil {
		return unusableAddress(base, err)
	}
	if u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return unusableAddress(base, nil)
	}
	return nil
}

func unusableAddress(base string, err error) *Error {
	return &Error{
		Kind:    Unusable,
		Problem: fmt.Sprintf("%q is not a usable address for ollama", base),
		Fix:     "it should look like " + DefaultBaseURL,
		Err:     err,
	}
}

// ollamaSays returns the explanation a response body carries, or "" if it
// carries none — either because it is not JSON at all, or because the object has
// no error in it.
//
// It reports the words and not what they mean. A JSON error string is not by
// itself evidence of which server sent it, which is why the 404 path asks
// [aboutTheModel] a second question before acting on one.
func ollamaSays(raw []byte) string {
	var said struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(raw, &said); err != nil {
		return ""
	}
	return strings.TrimSpace(said.Error)
}

// aboutTheModel reports whether a 404's own explanation is about the model
// rather than about the address it was sent to.
//
// ollama says "model 'nope:1b' not found" — verified against 0.17.7 by asking a
// running one for a model it does not have. Two tests rather than one: the
// model's own name in the sentence is the strong signal and survives any
// rewording, while "not found" catches a phrasing that names the missing thing
// without quoting it back.
//
// What is left out is deliberate. ollama answers a route it does not have with
// text/plain "404 page not found" — checked the same way — so a mistyped base
// URL never reaches this at all, and ollama alone would not need the test. It is
// here for the other server: this package's audience runs several things on
// localhost, and one of them answering an unknown route with a JSON error string
// is a 404 this package has no business explaining.
func aboutTheModel(said, model string) bool {
	if said == "" {
		return false
	}
	said = strings.ToLower(said)
	return strings.Contains(said, strings.ToLower(model)) || strings.Contains(said, "not found")
}

// because renders why a request was refused. It prefers ollama's sentence,
// falls back to a bounded one-line slice of whatever did come back, and to the
// status when even that is empty — because "refused the request" with no reason
// at all sends the reader to a packet capture.
//
// The fallback is skipped for a body that is itself a reply, and that is the
// only reason [isReply] exists. A generation arriving on an error status is not
// an explanation of the error, and it carries the model's scratchpad in a field
// beside the answer — so flattening it whole is the one place in this package
// where reasoning gets out. It reaches a terminal rather than the record, which
// makes it the smaller leak; it is still the only one that rests on nobody
// looking rather than on the code.
func because(raw []byte, status int) string {
	if said := ollamaSays(raw); said != "" {
		return said
	}
	if isReply(raw) {
		return fmt.Sprintf("HTTP %d, with a reply where an explanation should be", status)
	}

	// Cut the bytes before flattening, not after. Fields and Join each copy
	// everything handed to them and raw is bounded only by maxBody, so a hostile
	// four megabytes would be split, rejoined and widened to runes — some five
	// times its own size in allocation — to print two hundred characters of it.
	// An explanation buried past peek is not explaining itself.
	if len(raw) > peek {
		raw = raw[:peek]
		// A cut at a fixed offset lands mid-character eventually. Drop the
		// fragment rather than let it print as a replacement glyph; a U+FFFD
		// that was really in the body decodes at more than one byte and stays.
		for len(raw) > 0 {
			if r, n := utf8.DecodeLastRune(raw); r != utf8.RuneError || n > 1 {
				break
			}
			raw = raw[:len(raw)-1]
		}
	}

	flat := strings.Join(strings.Fields(string(raw)), " ")
	if flat == "" {
		return fmt.Sprintf("HTTP %d, with no explanation", status)
	}
	// Cut by runes. A body that is not ollama's is not necessarily ASCII, and
	// slicing bytes out of the middle of a character would put a replacement
	// glyph in a message whose whole job is to be readable.
	if r := []rune(flat); len(r) > 200 {
		flat = string(r[:200]) + "…"
	}
	return fmt.Sprintf("HTTP %d: %s", status, flat)
}

// isReply reports whether a body is a reply rather than something written about
// a failure. It asks for a field only a generation has — the answer, the
// scratchpad, why it stopped, or whether it finished — because an empty
// [chatReply] is also what any other JSON object decodes to, and treating
// `{"detail":"forbidden"}` as a reply would throw away the only sentence there.
func isReply(raw []byte) bool {
	var r chatReply
	if err := json.Unmarshal(raw, &r); err != nil {
		return false
	}
	return r.Message.Content != "" || r.Message.Thinking != "" || r.DoneReason != "" || r.Done != nil
}
