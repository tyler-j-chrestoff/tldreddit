package persona

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// nikola is the persona under test everywhere below. Its model name is the one
// the demo defaults to, because the two error messages this package exists to
// get right both quote it back to the reader.
func nikola() Persona {
	return Persona{
		Name:        "nikola",
		Model:       DefaultModel,
		System:      "You keep answers to one sentence.",
		Temperature: 0.7,
	}
}

// serve stands up an ollama-shaped server and points a Client at it. The
// server's own client is used, so nothing in these tests reaches beyond
// loopback, and no test depends on an ollama being installed.
func serve(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return &Client{BaseURL: srv.URL, HTTP: srv.Client()}
}

// says writes the success body ollama sends, verified against 0.17.7: the
// reply is nested under "message", a reasoning model puts its scratchpad in a
// sibling field rather than in the content, and done_reason says whether the
// model finished or hit the end of its room.
func says(w http.ResponseWriter, content, thinking string) {
	saysAndStopped(w, content, thinking, "stop")
}

func saysAndStopped(w http.ResponseWriter, content, thinking, reason string) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"model":"m","created_at":"2026-08-12T01:41:59Z",`+
		`"message":{"role":"assistant","content":%s,"thinking":%s},`+
		`"done":true,"done_reason":%s}`, quote(content), quote(thinking), quote(reason))
}

// drain reads the request body to EOF, and it is not tidiness: net/http only
// starts watching a connection for the client hanging up once the body has been
// read, so a handler that blocks without draining never learns that its caller
// left, and the test server's Close waits on it until the test binary times out.
//
// It is a separate step from [hangUp] because of the order it has to happen in.
// A handler that writes first and drains afterwards is draining a connection its
// own reader may already have cancelled, and this read is what fails instead.
// Drain at the top of the handler, always.
func drain(t *testing.T, r *http.Request) {
	t.Helper()
	if _, err := io.Copy(io.Discard, r.Body); err != nil {
		t.Errorf("draining the request body: %v", err)
	}
}

// hangUp waits for the client to give up, which is what a model that is still
// loading looks like from here. Call [drain] first — see there for why.
//
// The ceiling is there so a regression fails as an assertion rather than as a
// hung build; it is longer than any wait a caller here uses, so it never fires
// first.
func hangUp(t *testing.T, r *http.Request) {
	t.Helper()
	select {
	case <-r.Context().Done():
	case <-time.After(30 * time.Second):
		t.Error("the client never gave up, and never hung up")
	}
}

func quote(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func TestReplyReturnsWhatTheModelSaid(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		thinking string
		reason   string
		want     Answer
	}{
		{
			name:    "a plain answer",
			content: "Purge the stale logs.",
			reason:  "stop",
			want:    Answer{Text: "Purge the stale logs.", Window: DefaultWindow},
		},
		{
			name:    "padding the chat template added",
			content: "\n\n  Purge the stale logs.  \n",
			reason:  "stop",
			want:    Answer{Text: "Purge the stale logs.", Window: DefaultWindow},
		},
		{
			// The failure this rules out is not cosmetic. Reasoning that
			// reached the caller would be stored as something the persona
			// said, and the store never forgets, so there is no later pass
			// that takes it back out.
			name:     "a reasoning model, whose scratchpad is not an answer",
			content:  "Purge the stale logs.",
			thinking: "Thinking Process:\n1. **Analyze the Request** — the user wants...",
			reason:   "stop",
			want:     Answer{Text: "Purge the stale logs.", Window: DefaultWindow},
		},
		{
			// The text here is perfectly well-formed and is not what the
			// persona meant to say. Nothing downstream can tell that from the
			// string alone, which is the whole reason the flag exists.
			name:    "an answer that ran out of room",
			content: "Purge the stale logs, then rotate the",
			reason:  "length",
			want:    Answer{Text: "Purge the stale logs, then rotate the", Truncated: true, Window: DefaultWindow},
		},
		{
			// An older ollama, or a shape change, leaves the reason empty. A
			// missing reason is not a claim of truncation — silence must fall
			// on the side that does not accuse.
			name:    "a reply with no reason given",
			content: "Purge the stale logs.",
			reason:  "",
			want:    Answer{Text: "Purge the stale logs.", Window: DefaultWindow},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := serve(t, func(w http.ResponseWriter, r *http.Request) {
				saysAndStopped(w, tt.content, tt.thinking, tt.reason)
			})

			got, err := c.Reply(context.Background(), nikola(), []Turn{{RoleUser, "what now?"}})
			if err != nil {
				t.Fatalf("Reply: %v", err)
			}
			if got != tt.want {
				t.Errorf("Reply = %+v, want %+v", got, tt.want)
			}
			if tt.thinking != "" && strings.Contains(got.Text, "Analyze the Request") {
				t.Errorf("the model's reasoning reached the caller: %q", got.Text)
			}
		})
	}
}

// What goes on the wire is the persona: its model, its instruction ahead of
// everything else, and the turns in the order they happened. A chat model reads
// position as time, so a reordering here is a different conversation.
func TestReplySendsTheConversation(t *testing.T) {
	type sent struct {
		Model    string `json:"model"`
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
		// Pointers, because absent and false are different answers: ollama
		// streams by default, so a missing "stream" would leave this client
		// parsing the first chunk of a stream as the whole reply.
		Stream  *bool `json:"stream"`
		Options *struct {
			Temperature *float64 `json:"temperature"`
		} `json:"options"`
	}

	var got sent
	var method, path, contentType string
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		method, path, contentType = r.Method, r.URL.Path, r.Header.Get("Content-Type")
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("decoding the request body: %v", err)
		}
		says(w, "ok", "")
	})

	turns := []Turn{
		{RoleUser, "first"},
		{RoleAssistant, "second"},
		{RoleUser, "third"},
	}
	if _, err := c.Reply(context.Background(), nikola(), turns); err != nil {
		t.Fatalf("Reply: %v", err)
	}

	if method != http.MethodPost || path != "/api/chat" {
		t.Errorf("sent %s %s, want POST /api/chat", method, path)
	}
	if contentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", contentType)
	}
	if got.Model != DefaultModel {
		t.Errorf("model = %q, want %q", got.Model, DefaultModel)
	}
	if got.Stream == nil || *got.Stream {
		t.Errorf("stream = %v, want an explicit false", got.Stream)
	}
	if got.Options == nil || got.Options.Temperature == nil || *got.Options.Temperature != 0.7 {
		t.Errorf("options = %+v, want temperature 0.7 sent as written", got.Options)
	}

	want := []struct{ role, content string }{
		{"system", "You keep answers to one sentence."},
		{"user", "first"},
		{"assistant", "second"},
		{"user", "third"},
	}
	if len(got.Messages) != len(want) {
		t.Fatalf("sent %d messages, want %d: %+v", len(got.Messages), len(want), got.Messages)
	}
	for i, w := range want {
		if got.Messages[i].Role != w.role || got.Messages[i].Content != w.content {
			t.Errorf("message %d = %s/%q, want %s/%q",
				i, got.Messages[i].Role, got.Messages[i].Content, w.role, w.content)
		}
	}
}

// The "think" key is on every request, with a value, and a Client nobody
// configured sends false.
//
// The pointer is the whole test. Absent and false are different answers here in
// the way they are for "stream", and the difference is not cosmetic: measured
// against ollama 0.17.7 and qwen3.5:latest, the same one-line question sent
// without the key took 8.21s and returned 2,350 characters of monologue, and
// sent with an explicit false took 0.22s and returned none. An omitempty here —
// or a Persona that only sets it when true — would leave [Persona.Think] reading
// like a control while the server went on doing whatever the model felt like,
// which is a setting that cannot fail rather than a setting.
func TestReplySendsAnExplicitThinkSetting(t *testing.T) {
	for _, want := range []bool{false, true} {
		t.Run(fmt.Sprintf("think %v", want), func(t *testing.T) {
			var got struct {
				Think *bool `json:"think"`
			}
			c := serve(t, func(w http.ResponseWriter, r *http.Request) {
				if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
					t.Errorf("decoding the request body: %v", err)
				}
				says(w, "ok", "")
			})
			p := nikola()
			p.Think = want

			if _, err := c.Reply(context.Background(), p, []Turn{{RoleUser, "hello"}}); err != nil {
				t.Fatalf("Reply: %v", err)
			}
			if got.Think == nil {
				t.Fatalf("no \"think\" key was sent; ollama reads that as the model's own default")
			}
			if *got.Think != want {
				t.Errorf("think = %v, want %v", *got.Think, want)
			}
		})
	}
}

// A persona that says nothing about thinking gets the fast answer, and it has
// to get it through a zero field rather than through a default somewhere in the
// client. That is not a taste about struct literals: any caller that builds a
// Persona without mentioning Think must still get the fast answer, and nobody
// should have to know this setting exists to get a program that answers in a
// quarter of a second. tui's own defaultPersona does name it, deliberately and
// redundantly, for the reproducibility reason its comment gives — this check is
// what holds the guarantee for every caller that does not.
//
// Written against a Persona built the way a caller builds one, and a Client with
// no HTTP set, because the pair of zero values is the thing being claimed.
func TestAPersonaThatSaysNothingDoesNotAskForThinking(t *testing.T) {
	var got struct {
		Think *bool `json:"think"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("decoding the request body: %v", err)
		}
		says(w, "ok", "")
	}))
	t.Cleanup(srv.Close)

	c := Client{BaseURL: srv.URL}
	p := Persona{Name: "nikola", Model: DefaultModel, Temperature: 0.7}
	if _, err := c.Reply(context.Background(), p, []Turn{{RoleUser, "hello"}}); err != nil {
		t.Fatalf("Reply: %v", err)
	}
	if got.Think == nil || *got.Think {
		t.Errorf("a persona that set no Think sent think = %v, want an explicit false", got.Think)
	}
}

// An empty System sends no system message rather than an empty one: a blank
// instruction is a real instruction to a chat template, and not the one anyone
// meant.
func TestReplyOmitsAnEmptySystemPrompt(t *testing.T) {
	var roles []string
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []struct {
				Role string `json:"role"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decoding the request body: %v", err)
		}
		for _, m := range body.Messages {
			roles = append(roles, m.Role)
		}
		says(w, "ok", "")
	})

	p := nikola()
	p.System = ""
	if _, err := c.Reply(context.Background(), p, []Turn{{RoleUser, "hello"}}); err != nil {
		t.Fatalf("Reply: %v", err)
	}
	if len(roles) != 1 || roles[0] != "user" {
		t.Errorf("sent roles %v, want just [user]", roles)
	}
}

// A base URL with a trailing slash is what someone copying out of a browser
// will paste. Doubling the slash gives a 404, which this package would then
// report as a missing model — the wrong sentence entirely.
func TestReplyToleratesATrailingSlash(t *testing.T) {
	var path string
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		says(w, "ok", "")
	})
	c.BaseURL += "/"

	if _, err := c.Reply(context.Background(), nikola(), nil); err != nil {
		t.Fatalf("Reply: %v", err)
	}
	if path != "/api/chat" {
		t.Errorf("requested %q, want /api/chat", path)
	}
}

// Every way a running server can fail to produce a reply, and what the person
// reading it is told. The assertions are on [Kind] plus the load-bearing
// fragment of the sentence — the phrasing is free to improve, the model name
// and the command to run are not.
func TestReplyFailures(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
		want    Kind
		says    []string
	}{
		{
			name: "the model is not pulled",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprintf(w, `{"error":"model '%s' not found"}`, DefaultModel)
			},
			want: NoModel,
			says: []string{DefaultModel, "ollama pull " + DefaultModel},
		},
		{
			// ollama's exact wording is not a promise, so the model's own name
			// in the sentence is enough on its own.
			name: "a 404 that names the model without saying not found",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprintf(w, `{"error":"pull model '%s' before using it"}`, DefaultModel)
			},
			want: NoModel,
			says: []string{"ollama pull " + DefaultModel},
		},
		{
			// The same status as a missing model, and a completely different
			// problem: an ordinary web server at the wrong address. Only the
			// body tells them apart, so the body is what this package reads.
			name: "a 404 from something that is not ollama",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.NotFound(w, r)
			},
			want: Garbled,
			says: []string{"does not answer like ollama"},
		},
		{
			// A 404 with a JSON error in it, from a server that is not ollama —
			// an OpenAI-shaped local one answering a route it does not have,
			// which is the address a reader of this package is most likely to
			// have mistyped it into. Reading the JSON alone as proof of a
			// missing model sends them to `ollama pull`, which succeeds and
			// changes nothing, having told them the opposite of the truth.
			name: "a 404 about the route rather than the model",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprint(w, `{"error":"no route matched"}`)
			},
			want: Garbled,
			says: []string{"does not answer like ollama"},
		},
		{
			// "done":false with streaming off is a server saying outright that
			// more was coming. ollama 0.17.7 cannot say it; something else at
			// the address can, and the text beside it is the first slice of an
			// answer rather than a short one.
			name: "a piece of a reply, marked as a piece",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"model":"m","message":{"role":"assistant",`+
					`"content":"Purge the stale"},"done":false}`)
			},
			want: Garbled,
			says: []string{"a piece of a reply"},
		},
		{
			name: "a page where the reply should be",
			handler: func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, "<html><body>hello from something else</body></html>")
			},
			want: Garbled,
			says: []string{"not in a shape this client can read"},
		},
		{
			name: "a reply that stops mid-object",
			handler: func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, `{"message":{"role":"assistant","content":"half a th`)
			},
			want: Garbled,
			says: []string{"not in a shape this client can read"},
		},
		{
			name: "refused, and explained",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprint(w, `{"error":"model is required"}`)
			},
			want: Rejected,
			says: []string{"model is required", DefaultModel},
		},
		{
			name: "refused, with nothing but a page",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprint(w, "boom")
			},
			want: Rejected,
			says: []string{"HTTP 500", "boom"},
		},
		{
			name: "refused, with no explanation at all",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusServiceUnavailable)
			},
			want: Rejected,
			says: []string{"HTTP 503", "no explanation"},
		},
		{
			// A server that says it succeeded and then says it failed should
			// be believed about the failure.
			name: "an error carried on a 200",
			handler: func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, `{"error":"unexpected server status: llm server loading"}`)
			},
			want: Rejected,
			says: []string{"llm server loading"},
		},
		{
			name: "an answer with nothing in it",
			handler: func(w http.ResponseWriter, r *http.Request) {
				says(w, "   \n ", "")
			},
			want: Garbled,
			says: []string{"nothing at all"},
		},
		{
			name: "all reasoning and no answer",
			handler: func(w http.ResponseWriter, r *http.Request) {
				says(w, "", "Thinking Process: the user wants...")
			},
			want: Garbled,
			says: []string{"thought for", "said nothing"},
		},
		{
			// Empty and truncated is a different problem from empty and
			// finished, and the difference is what the reader should do next.
			name: "cut off before it said anything",
			handler: func(w http.ResponseWriter, r *http.Request) {
				saysAndStopped(w, "", "Thinking Process: the user wants...", "length")
			},
			want: Garbled,
			says: []string{"ran out of room", "shorten the conversation"},
		},
		{
			// Guards one specific mistake: an address pointed at something
			// that answers forever. Without the cap this is memory, not an
			// error.
			name: "more body than a reply could be",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Write(make([]byte, maxBody+1))
			},
			want: Garbled,
			says: []string{"more than a reply could be"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := serve(t, tt.handler)

			got, err := c.Reply(context.Background(), nikola(), []Turn{{RoleUser, "hello"}})
			if got != (Answer{}) {
				t.Errorf("Reply returned %+v alongside a failure", got)
			}

			var e *Error
			if !errors.As(err, &e) {
				t.Fatalf("Reply returned %T (%v), want a *persona.Error", err, err)
			}
			if e.Kind != tt.want {
				t.Errorf("Kind = %q, want %q — the whole message was: %v", e.Kind, tt.want, e)
			}
			for _, want := range tt.says {
				if !strings.Contains(e.Error(), want) {
					t.Errorf("the message does not mention %q:\n%v", want, e)
				}
			}
		})
	}
}

// A server too old to send "done" says nothing about whether it finished, and
// silence is not a denial. The pointer behind that field is the whole mechanism
// keeping absent apart from false, and this is the half that would go unnoticed:
// reading a missing field as false would refuse every reply such a server sends.
func TestReplyAcceptsAServerThatOmitsDone(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"model":"m","message":{"role":"assistant","content":"Purge the stale logs."}}`)
	})

	got, err := c.Reply(context.Background(), nikola(), []Turn{{RoleUser, "what now?"}})
	if err != nil {
		t.Fatalf("Reply: %v", err)
	}
	if want := (Answer{Text: "Purge the stale logs.", Window: DefaultWindow}); got != want {
		t.Errorf("Reply = %+v, want %+v", got, want)
	}
}

// A model that writes its reasoning inside content rather than beside it hands
// it over as ordinary text, and this client passes it through untouched. Pinned
// rather than fixed: both available fixes cost more than the leak, and
// [DefaultModel] carries the argument. This test is where anyone who reverses
// that call has to say so out loud.
func TestReplyKeepsInlineReasoningInTheAnswer(t *testing.T) {
	const said = "<think>The user wants the logs gone.</think>Purge the stale logs."

	c := serve(t, func(w http.ResponseWriter, r *http.Request) { says(w, said, "") })

	got, err := c.Reply(context.Background(), nikola(), []Turn{{RoleUser, "what now?"}})
	if err != nil {
		t.Fatalf("Reply: %v", err)
	}
	if got.Text != said {
		t.Errorf("Reply text = %q, want it exactly as it came:\n%q", got.Text, said)
	}
}

// The one place this package's claim about reasoning is not held up by the
// code's shape. A body that is a generation, arriving on a status that says the
// request was refused, reaches the fallback that flattens whatever came back —
// and a reasoning model's scratchpad is in there. It would print to a terminal
// rather than land in the record, which makes it the smaller leak, not a small
// one.
func TestARefusalDoesNotQuoteTheModelsReasoning(t *testing.T) {
	const scratchpad = "Thinking Process:\n1. **Analyze the Request** — the user wants..."

	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"model":"m","message":{"role":"assistant",`+
			`"content":"Purge the stale logs.","thinking":%s},"done":true,"done_reason":"stop"}`,
			quote(scratchpad))
	})

	_, err := c.Reply(context.Background(), nikola(), []Turn{{RoleUser, "hello"}})

	var e *Error
	if !errors.As(err, &e) {
		t.Fatalf("Reply returned %T (%v), want a *persona.Error", err, err)
	}
	if e.Kind != Rejected {
		t.Errorf("Kind = %q, want %q", e.Kind, Rejected)
	}
	if !strings.Contains(e.Error(), "HTTP 500") {
		t.Errorf("the message does not say what the status was:\n%v", e)
	}
	if strings.Contains(e.Error(), "Analyze the Request") {
		t.Errorf("the model's reasoning reached the message:\n%v", e)
	}
}

// An unrecognized body is quoted back bounded and cut at a character, never
// inside one. The bound keeps a megabyte of somebody else's HTML out of a
// terminal message; the boundary keeps the part that does get shown readable.
//
// Both halves of the body are load-bearing. The character is three bytes wide
// and peek is not a multiple of three, so the cut lands one byte into one of
// them; the padding is whitespace, which flattening drops, so what is left of
// that fragment sits inside the two hundred runes that survive — the only place
// it could show.
func TestARefusalQuotesAStrangeBodyBoundedAndWhole(t *testing.T) {
	body := strings.Repeat(" ", peek-10) + strings.Repeat("…", 1000)

	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, body)
	})

	_, err := c.Reply(context.Background(), nikola(), []Turn{{RoleUser, "hello"}})

	var e *Error
	if !errors.As(err, &e) {
		t.Fatalf("Reply returned %T (%v), want a *persona.Error", err, err)
	}
	if strings.ContainsRune(e.Error(), utf8.RuneError) {
		t.Errorf("the body was cut inside a character:\n%v", e)
	}
	if got := utf8.RuneCountInString(e.Error()); got > 400 {
		t.Errorf("the message is %d characters long; a body this size is quoted, not printed", got)
	}
}

// The first failure a new user hits, and the one this package's error handling
// exists for. The exact string is asserted because it is the product surface
// here — if it changes, that should be somebody's deliberate edit and not a
// side effect.
func TestReplyWhenOllamaIsNotRunning(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	c := &Client{BaseURL: srv.URL, HTTP: srv.Client()}
	srv.Close()

	_, err := c.Reply(context.Background(), nikola(), []Turn{{RoleUser, "hello"}})

	var e *Error
	if !errors.As(err, &e) {
		t.Fatalf("Reply returned %T (%v), want a *persona.Error", err, err)
	}
	if e.Kind != Unreachable {
		t.Errorf("Kind = %q, want %q", e.Kind, Unreachable)
	}
	want := fmt.Sprintf("ollama is not answering at %s — it does not appear to be running\n"+
		"start it with: ollama serve", srv.URL)
	if e.Error() != want {
		t.Errorf("the person reads:\n%v\n\nwant:\n%s", e, want)
	}
	if strings.Contains(e.Error(), "dial tcp") {
		t.Errorf("the socket's own words reached the message:\n%v", e)
	}
	if e.Unwrap() == nil {
		t.Error("the cause was dropped; it is the only thing a debugger has")
	}
}

// The second failure a new user hits: ollama is up, the model was never
// pulled. Exact string, for the same reason as above.
func TestReplyWhenTheModelIsNotPulled(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, `{"error":"model '%s' not found"}`, DefaultModel)
	})

	_, err := c.Reply(context.Background(), nikola(), []Turn{{RoleUser, "hello"}})

	var e *Error
	if !errors.As(err, &e) {
		t.Fatalf("Reply returned %T (%v), want a *persona.Error", err, err)
	}
	want := fmt.Sprintf("ollama is running, but the model %q is not installed\n"+
		"pull it with: ollama pull %s", DefaultModel, DefaultModel)
	if e.Error() != want {
		t.Errorf("the person reads:\n%v\n\nwant:\n%s", e, want)
	}
	if e.Unwrap() == nil || !strings.Contains(e.Unwrap().Error(), "not found") {
		t.Errorf("ollama's own words were dropped rather than kept out of sight: %v", e.Unwrap())
	}
}

// Cancelling has to abort the request, not merely stop waiting for it: a
// generation left running on a local GPU holds the machine's only accelerator
// for as long as it takes.
func TestReplyStopsWhenTheContextDoes(t *testing.T) {
	tests := []struct {
		name string
		ctx  func() (context.Context, context.CancelFunc)
		// stopOnce the request is under way, rather than by a clock. The
		// deadline case must be left alone to expire, or it reports the
		// caller's cancel and proves nothing about deadlines.
		stopOnStart bool
		is          error
		says        string
	}{
		{
			name:        "cancelled by the caller",
			ctx:         func() (context.Context, context.CancelFunc) { return context.WithCancel(context.Background()) },
			stopOnStart: true,
			is:          context.Canceled,
			says:        "called off",
		},
		{
			name: "the caller's deadline",
			ctx: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), 50*time.Millisecond)
			},
			is:   context.DeadlineExceeded,
			says: "before the deadline",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			aborted := make(chan struct{}, 1)
			started := make(chan struct{}, 1)
			c := serve(t, func(w http.ResponseWriter, r *http.Request) {
				drain(t, r)
				started <- struct{}{}
				hangUp(t, r)
				aborted <- struct{}{}
			})

			ctx, stop := tt.ctx()
			defer stop()
			if tt.stopOnStart {
				go func() {
					<-started
					stop()
				}()
			}

			_, err := c.Reply(ctx, nikola(), []Turn{{RoleUser, "hello"}})

			var e *Error
			if !errors.As(err, &e) {
				t.Fatalf("Reply returned %T (%v), want a *persona.Error", err, err)
			}
			if e.Kind != Canceled {
				t.Errorf("Kind = %q, want %q", e.Kind, Canceled)
			}
			if !errors.Is(err, tt.is) {
				t.Errorf("errors.Is(err, %v) is false; the cause did not survive the rewrite", tt.is)
			}
			if !strings.Contains(e.Error(), tt.says) {
				t.Errorf("the message does not say %q:\n%v", tt.says, e)
			}

			select {
			case <-aborted:
			case <-time.After(5 * time.Second):
				t.Error("the server never saw the request end; it was not aborted")
			}
		})
	}
}

// Cancelling after the headers have arrived kills the request in the body read,
// which is a different line of code from the one Do fails at — and was, before
// this test, a different sentence: the person who pressed ctrl-c was told the
// server had stopped part way through, and sent to its log to find out why. The
// persona loop will cancel these routinely, so this is not the rare path.
func TestReplyStopsWhenTheContextDoesMidBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		drain(t, r)
		w.Header().Set("Content-Type", "application/json")
		// The first slice of a real reply, flushed, so that by the time anything
		// is cancelled the send has already succeeded and the read is the only
		// thing left that can fail.
		fmt.Fprint(w, `{"model":"m","message":{"role":"assistant","content":"Purge`)
		w.(http.Flusher).Flush()
		hangUp(t, r)
	}))
	t.Cleanup(srv.Close)

	ctx, stop := context.WithCancel(context.Background())
	defer stop()

	// Cancelling from inside the body's own first Read is what makes this test
	// about the read rather than about which goroutine wins a footrace.
	c := &Client{
		BaseURL: srv.URL,
		HTTP: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			resp, err := srv.Client().Transport.RoundTrip(r)
			if err != nil {
				return nil, err
			}
			resp.Body = &stopOnRead{ReadCloser: resp.Body, stop: stop}
			return resp, nil
		})},
	}

	_, err := c.Reply(ctx, nikola(), []Turn{{RoleUser, "hello"}})

	var e *Error
	if !errors.As(err, &e) {
		t.Fatalf("Reply returned %T (%v), want a *persona.Error", err, err)
	}
	if e.Kind != Canceled {
		t.Errorf("Kind = %q, want %q — the whole message was: %v", e.Kind, Canceled, e)
	}
	if !errors.Is(err, context.Canceled) {
		t.Error("errors.Is(err, context.Canceled) is false; the cause did not survive the rewrite")
	}
	if !strings.Contains(e.Error(), "called off") {
		t.Errorf("the caller's own cancel was reported as something else:\n%v", e)
	}
	if strings.Contains(e.Error(), "ollama server's own log") {
		t.Errorf("a cancel was blamed on the server, and sent the reader to its log:\n%v", e)
	}
}

// stopOnRead cancels once, after the first read has handed back whatever had
// already arrived. Nothing guards the flag because [io.ReadAll] is the only
// reader of a body and it reads on one goroutine.
type stopOnRead struct {
	io.ReadCloser
	stop    func()
	stopped bool
}

func (s *stopOnRead) Read(p []byte) (int, error) {
	n, err := s.ReadCloser.Read(p)
	if !s.stopped {
		s.stopped = true
		s.stop()
	}
	return n, err
}

// Our own clock running out is a different sentence from the caller's: it means
// wait longer, not that anything is wrong.
func TestReplyGivesUpOnASilentServer(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		drain(t, r)
		hangUp(t, r)
	})
	c.HTTP = &http.Client{Timeout: 50 * time.Millisecond}

	_, err := c.Reply(context.Background(), nikola(), []Turn{{RoleUser, "hello"}})

	var e *Error
	if !errors.As(err, &e) {
		t.Fatalf("Reply returned %T (%v), want a *persona.Error", err, err)
	}
	if e.Kind != Timeout {
		t.Errorf("Kind = %q, want %q — the message was: %v", e.Kind, Timeout, e)
	}
	if !strings.Contains(e.Error(), "within 50ms") {
		t.Errorf("the message does not name how long it waited:\n%v", e)
	}
}

// A hostname that does not resolve gets its own sentence, because "start
// ollama" is the wrong advice for it and following wrong advice costs more
// than being told nothing.
func TestReplyWhenTheHostDoesNotResolve(t *testing.T) {
	c := &Client{
		BaseURL: "http://ollama.invalid:11434",
		HTTP: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return nil, &net.DNSError{Err: "no such host", Name: "ollama.invalid", IsNotFound: true}
		})},
	}

	_, err := c.Reply(context.Background(), nikola(), []Turn{{RoleUser, "hello"}})

	var e *Error
	if !errors.As(err, &e) {
		t.Fatalf("Reply returned %T (%v), want a *persona.Error", err, err)
	}
	if e.Kind != Unreachable {
		t.Errorf("Kind = %q, want %q", e.Kind, Unreachable)
	}
	if !strings.Contains(e.Error(), "ollama.invalid") {
		t.Errorf("the message does not name the host that was not found:\n%v", e)
	}
	if strings.Contains(e.Error(), "ollama serve") {
		t.Errorf("a name that does not resolve was blamed on a stopped server:\n%v", e)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// Mistakes that can be caught without asking anything of the network are
// caught there, so a wedged or absent server never gets blamed for them.
func TestReplyRefusesWhatItCannotSend(t *testing.T) {
	tests := []struct {
		name    string
		persona func(Persona) Persona
		baseURL string
		says    string
	}{
		{
			name:    "a persona with no model",
			persona: func(p Persona) Persona { p.Model = ""; return p },
			says:    "names no model",
		},
		{
			name:    "an address that is not one",
			baseURL: "not-a-url::11434",
			says:    "is not a usable address",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asked := false
			c := serve(t, func(w http.ResponseWriter, r *http.Request) {
				asked = true
				says(w, "ok", "")
			})
			if tt.baseURL != "" {
				c.BaseURL = tt.baseURL
			}
			p := nikola()
			if tt.persona != nil {
				p = tt.persona(p)
			}

			_, err := c.Reply(context.Background(), p, []Turn{{RoleUser, "hello"}})

			var e *Error
			if !errors.As(err, &e) {
				t.Fatalf("Reply returned %T (%v), want a *persona.Error", err, err)
			}
			if e.Kind != Unusable {
				t.Errorf("Kind = %q, want %q — the message was: %v", e.Kind, Unusable, e)
			}
			if !strings.Contains(e.Error(), tt.says) {
				t.Errorf("the message does not say %q:\n%v", tt.says, e)
			}
			if asked {
				t.Error("a request went out that could never have worked")
			}
		})
	}
}

// The zero Client is the local ollama on its default port. Nothing is sent
// here: the point is only that the defaults are the ones documented, and the
// address is what every error message quotes back.
func TestZeroClientPointsAtALocalOllama(t *testing.T) {
	var c Client
	if got := c.base(); got != DefaultBaseURL {
		t.Errorf("base = %q, want %q", got, DefaultBaseURL)
	}
	if got := c.httpClient().Timeout; got != DefaultTimeout {
		t.Errorf("timeout = %v, want %v — a short one truncates real replies", got, DefaultTimeout)
	}
}

// The window is on every request, with a value, and a persona that says nothing
// gets [DefaultWindow] rather than whatever the server would have picked.
//
// The pointer is the test, as it is for "think" and "stream". Absent is a third
// value here and it is the expensive one: measured against ollama 0.17.7, a
// 400-message conversation sent with no num_ctx had 4,086 of its tokens read,
// and the same conversation with num_ctx 32768 had 26,578 read — the server
// dropping the oldest turns to fit its own 4096 default, answering normally,
// and saying nothing about it in any field of the reply. A request that omits
// this key is not asking for less; it is asking without knowing.
func TestReplySendsTheWindowItAsksFor(t *testing.T) {
	tests := []struct {
		name string
		set  int
		want int
	}{
		{"a persona that says nothing", 0, DefaultWindow},
		{"a persona that asks for less", 8192, 8192},
		{"a persona that asks for the default in so many words", DefaultWindow, DefaultWindow},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got struct {
				Options *struct {
					NumCtx *int `json:"num_ctx"`
				} `json:"options"`
			}
			c := serve(t, func(w http.ResponseWriter, r *http.Request) {
				if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
					t.Errorf("decoding the request body: %v", err)
				}
				says(w, "ok", "")
			})
			p := nikola()
			p.Window = tt.set

			ans, err := c.Reply(context.Background(), p, []Turn{{RoleUser, "hello"}})
			if err != nil {
				t.Fatalf("Reply: %v", err)
			}
			if got.Options == nil || got.Options.NumCtx == nil {
				t.Fatalf("no \"num_ctx\" key was sent; ollama reads that as 4096 whatever the model can hold")
			}
			if *got.Options.NumCtx != tt.want {
				t.Errorf("num_ctx = %d, want %d", *got.Options.NumCtx, tt.want)
			}
			// The same number comes back on the answer, because a caller that
			// wants to say what the model read needs to know what it was given
			// room to read.
			if ans.Window != tt.want {
				t.Errorf("Answer.Window = %d, want %d", ans.Window, tt.want)
			}
		})
	}
}

// A zero num_ctx still goes on the wire as a zero.
//
// Nothing in this package sends one — [Persona.Window]'s zero becomes
// [DefaultWindow] before the request is built — so this is a check on the tag
// rather than on a reachable path, and it is written against the encoding
// directly because there is no request that would exercise it. What it holds up
// is the guard: an omitempty added here in a tidying pass would be invisible
// today and would turn the first future caller that reaches this with a zero
// into one that silently inherits the server's 4096.
func TestTheWireKeepsAZeroWindowVisible(t *testing.T) {
	body, err := json.Marshal(chatRequest{Model: DefaultModel})
	if err != nil {
		t.Fatalf("marshalling a chatRequest: %v", err)
	}
	if !strings.Contains(string(body), `"num_ctx":0`) {
		t.Errorf("a zero window marshalled to %s, want a visible \"num_ctx\":0", body)
	}
}

// A negative window is a mistake at the call site, and it is refused before
// anything is sent — the same treatment, in the same place, as a persona that
// names no model. Sent, it would be ollama's to interpret, and the reply would
// come back looking ordinary.
func TestReplyRefusesAWindowNobodyCouldHaveMeant(t *testing.T) {
	asked := false
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		asked = true
		says(w, "ok", "")
	})
	p := nikola()
	p.Window = -1

	_, err := c.Reply(context.Background(), p, []Turn{{RoleUser, "hello"}})
	var e *Error
	if !errors.As(err, &e) || e.Kind != Unusable {
		t.Fatalf("Reply = %v, want an Unusable Error", err)
	}
	if !strings.Contains(e.Error(), "-1") {
		t.Errorf("the message does not quote the window back: %q", e.Error())
	}
	if asked {
		t.Error("a request went out that could never have worked")
	}
}

// How much of the conversation the model read comes back on the answer, and a
// server that does not say leaves a zero rather than a claim.
//
// prompt_eval_count is the only thing ollama 0.17.7 says about what went in —
// verified by dumping every key of a reply to a conversation that was cut and
// one that fit, which differ in nothing else. Reading it is what lets a caller
// notice, across turns, that the count has stopped growing while the record
// has not. Losing it here would take that away with every test still green,
// because nothing else in this package looks at the number.
func TestReplyReportsHowMuchOfTheConversationWasRead(t *testing.T) {
	tests := []struct {
		name string
		body string
		want int
	}{
		{
			name: "a server that counts the prompt",
			body: `{"message":{"role":"assistant","content":"ok"},"done":true,"done_reason":"stop","prompt_eval_count":4086}`,
			want: 4086,
		},
		{
			// An older ollama, or another server answering in ollama's shape.
			// Zero is "it did not say" and not "it read none of it": a reply
			// that exists read at least one token, so there is nothing for the
			// two readings to be confused about.
			name: "a server that says nothing about the prompt",
			body: `{"message":{"role":"assistant","content":"ok"},"done":true,"done_reason":"stop"}`,
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := serve(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, tt.body)
			})

			got, err := c.Reply(context.Background(), nikola(), []Turn{{RoleUser, "hello"}})
			if err != nil {
				t.Fatalf("Reply: %v", err)
			}
			if got.PromptTokens != tt.want {
				t.Errorf("PromptTokens = %d, want %d", got.PromptTokens, tt.want)
			}
		})
	}
}

// shows writes the shape /api/show answers with, cut down to the one field this
// package reads. The real body also carries the licence, the Modelfile, the
// chat template and a tensor list — 42 KB for llama3.2:1b and 83 KB for
// qwen3.5:latest, measured — none of which anything here looks at.
func shows(w http.ResponseWriter, info string) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"details":{"family":"qwen35"},"model_info":%s}`, info)
}

// What the model can hold is read off /api/show, under a key whose prefix is
// the architecture's rather than a fixed name.
//
// Verified against ollama 0.17.7: qwen3.5:latest reports 262144 under
// "qwen35.context_length" and llama3.2:1b reports 131072 under
// "llama.context_length", so a client that matched a spelling would answer for
// one model and not the other. The near neighbours in the table are the ones a
// real model_info carries — qwen3.5:latest has an embedding length, an
// attention key length and a vision embedding length beside the one that
// matters.
func TestWindowForReadsWhatTheModelCanHold(t *testing.T) {
	var method, path string
	var sent struct {
		Model string `json:"model"`
	}
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&sent); err != nil {
			t.Errorf("decoding the request body: %v", err)
		}
		shows(w, `{"qwen35.context_length":262144,"qwen35.embedding_length":4096,
			"qwen35.attention.key_length":256,"qwen35.vision.embedding_length":1152,
			"general.architecture":"qwen35"}`)
	})

	got, err := c.WindowFor(context.Background(), DefaultModel)
	if err != nil {
		t.Fatalf("WindowFor: %v", err)
	}
	if got != 262144 {
		t.Errorf("WindowFor = %d, want 262144", got)
	}
	if method != http.MethodPost || path != "/api/show" {
		t.Errorf("sent %s %s, want POST /api/show", method, path)
	}
	if sent.Model != DefaultModel {
		t.Errorf("asked about %q, want %q", sent.Model, DefaultModel)
	}
}

func TestWindowForReadsAnyArchitecturesKey(t *testing.T) {
	tests := []struct {
		name string
		info string
		want int
	}{
		{"llama", `{"llama.context_length":131072,"llama.embedding_length":2048}`, 131072},
		{"qwen3", `{"qwen3.context_length":40960}`, 40960},
		{
			// Not a shape ollama 0.17.7 produces, and read anyway: a bare key
			// is unambiguous, and refusing it would be this client insisting on
			// a prefix for its own sake.
			name: "no architecture prefix at all",
			info: `{"context_length":8192}`,
			want: 8192,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := serve(t, func(w http.ResponseWriter, r *http.Request) { shows(w, tt.info) })
			got, err := c.WindowFor(context.Background(), "a-model")
			if err != nil {
				t.Fatalf("WindowFor: %v", err)
			}
			if got != tt.want {
				t.Errorf("WindowFor = %d, want %d", got, tt.want)
			}
		})
	}
}

// Every way the answer can fail to be a window, refused rather than guessed.
//
// The two-keys row is the one worth defending. Picking the first match would
// return a definite-looking number for a question this client cannot answer,
// and the number would be whichever key sorted first — a wrong window is not a
// visible failure, it is a conversation quietly cut somewhere else.
func TestWindowForRefusesAnAnswerItCannotRead(t *testing.T) {
	tests := []struct {
		name string
		info string
		says []string
	}{
		{
			name: "no context length anywhere",
			info: `{"llama.embedding_length":2048}`,
			says: []string{"does not say"},
		},
		{
			name: "two of them",
			info: `{"llama.context_length":131072,"clip.context_length":77}`,
			says: []string{"more than one", "clip.context_length", "llama.context_length"},
		},
		{
			name: "one that is not a number",
			info: `{"llama.context_length":"lots"}`,
			says: []string{"not a number"},
		},
		{
			name: "one that is not a length",
			info: `{"llama.context_length":0}`,
			says: []string{"cannot be a window"},
		},
		{
			name: "one no window could be",
			info: `{"llama.context_length":9007199254740993}`,
			says: []string{"cannot be a window"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := serve(t, func(w http.ResponseWriter, r *http.Request) { shows(w, tt.info) })
			got, err := c.WindowFor(context.Background(), "a-model")
			var e *Error
			if !errors.As(err, &e) || e.Kind != Garbled {
				t.Fatalf("WindowFor = %d, %v, want a Garbled Error", got, err)
			}
			if got != 0 {
				t.Errorf("WindowFor = %d beside an error, want 0", got)
			}
			for _, want := range tt.says {
				if !strings.Contains(e.Error(), want) {
					t.Errorf("the message does not mention %q: %q", want, e.Error())
				}
			}
		})
	}
}

// The two failures a person actually hits, told apart the same way [Reply]
// tells them apart: ollama's own 404 about a model it does not have carries the
// one fix worth printing, and any other 404 at that address is somebody else's
// server.
func TestWindowForWhenTheModelIsNotPulled(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"error":"model 'nope:1b' not found"}`)
	})

	_, err := c.WindowFor(context.Background(), "nope:1b")
	var e *Error
	if !errors.As(err, &e) || e.Kind != NoModel {
		t.Fatalf("WindowFor = %v, want a NoModel Error", err)
	}
	if !strings.Contains(e.Fix, "ollama pull nope:1b") {
		t.Errorf("the fix does not name the pull: %q", e.Fix)
	}
}

func TestWindowForRefusesWhatItCannotAsk(t *testing.T) {
	asked := false
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		asked = true
		shows(w, `{"llama.context_length":131072}`)
	})

	_, err := c.WindowFor(context.Background(), "")
	var e *Error
	if !errors.As(err, &e) || e.Kind != Unusable {
		t.Fatalf("WindowFor = %v, want an Unusable Error", err)
	}
	if asked {
		t.Error("a request went out with no model in it")
	}

	bad := &Client{BaseURL: "not-a-url"}
	if _, err := bad.WindowFor(context.Background(), DefaultModel); !errors.As(err, &e) || e.Kind != Unusable {
		t.Fatalf("WindowFor against a bad address = %v, want an Unusable Error", err)
	}
}
