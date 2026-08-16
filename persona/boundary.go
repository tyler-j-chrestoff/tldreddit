package persona

import "strings"

// Escape rewrites text so that it cannot spell a role boundary, and reports how
// many markers it changed.
//
// This is D1's own split applied to the wire: **the record does not forget, and
// what is transmitted is derived.** A bit is evidence and keeps whatever was
// said, control tokens included — the same call [DefaultModel] makes about
// `<think>` tags, where cutting text out of a message because it contains a few
// particular characters was ruled the worse record defect. Nothing here touches
// a bit. This runs once, on the way out, in [Client.Reply], and the record it
// was derived from is still sitting there to derive it again.
//
// What it defends against, measured rather than assumed. ollama renders the
// message list into the model's own chat template and tokenizes the result with
// special tokens parsed wherever they occur, including inside content a caller
// supplied; /api/chat has no field that says "read this as text" (ollama 0.17.7,
// checked against its own API reference and by asking a running one). Measured
// by prompt_eval_count against a live ollama 0.17.7: on qwen3.5:latest
// `<|im_start|>` in a user message costs **one** token where `<|im_startX|>`
// costs seven — and a genuine five-message conversation, against a three-message
// one whose middle assistant turn carries the other two spelled out inside its
// content, produce the *same prompt token count and the same reply at
// temperature 0*. A forged turn is not similar to a real one; it is the same
// bytes. This record already holds the ingredients: a qwen3.5 reply ended
// `?<|endoftext|><|im_start|>user\n<|system_message|>` and went onto the record
// as ordinary content, which is correct and stays. The exposure is narrower and
// nearer than "a second participant can write bits": it is **same family as the
// writer**, and it is already today. Narrower, because a family parses only its
// own vocabulary — the qwen tokens now on this record are inert to a mistral
// persona, so a forged boundary needs a reader of the same family as whoever
// wrote it. Nearer, because `tldr say` shipped the day before this was written,
// so a second writer is not a future condition. Corrected here rather than only
// in a commit message, which is where the correction first landed and where a
// reader of this comment would never find it (D57(f)).
//
// The rule is one sentence: **a backslash goes in immediately after a marker's
// opening bracket.** `<|im_start|>` becomes `<\|im_start|>`, `[INST]` becomes
// `[\INST]`, `</s>` becomes `<\/s>`. Every character of the original survives in
// order, so a person reading a captured request sees both what was recorded and
// that something escaped it, and the transformation is a pure function of the
// text, so anyone holding the record can re-derive exactly what was sent.
// Deleting the marker would leave a reader unable to tell what had been there,
// and a zero-width character would be invisible — a silent transformation is a
// second record quietly disagreeing with the first, which is the failure this
// project is a bet against.
//
// Which markers, and the honest limit. Each model parses its own family's
// vocabulary and nobody else's — measured the same way: `<|eot_id|>` is one
// token to llama3.2:1b and eight to ministral-3:14b, `[INST]` is one to
// ministral and three to llama3.2. So this cannot be a list of tokens, because
// the list is per-model and this client does not know the vocabulary. It is a
// list of bracket *shapes*, which covers every name inside a shape it knows,
// including names nobody has minted yet:
//
//	<|…       chatml, qwen, llama3, gpt-oss — breaking the opener is enough
//	[NAME]    mistral: [INST], [/INST], [SYSTEM_PROMPT], [AVAILABLE_TOOLS]
//	<s> </s>  sentencepiece's beginning and end of sequence
//
// A family whose boundary is spelled some other way is not covered, and the way
// to find out is the measurement, not this comment: ask a running ollama for
// prompt_eval_count with the marker alone in a user message, and again with one
// character of it changed. One token means it is a boundary.
//
// It escapes text that was never dangerous, and that is the side to err on.
// `[TODO]` in somebody's note becomes `[\TODO]`, because the shape is right and
// this client cannot know that no model has such a token. The cost of that error
// is one visible backslash in a message. The cost of the other is a turn the
// conversation never had, asserted to the model as something a participant said.
//
// What this deliberately does not do is tell the persona it happened. A gap
// reaches a persona as a note in a system turn (D35), and this is the same
// shape — but choosing what the persona is allowed to see belongs to the caller
// holding the view, as this package's own doc says, not to the client that makes
// the request. The count comes back so that a caller who wants to say so can.
func Escape(s string) (string, int) {
	var b strings.Builder
	n, last := 0, 0
	// Scanned by byte rather than by rune: every marker here is ASCII, and no
	// byte of a multi-byte character is ever below 0x80, so this cannot land
	// inside one and mistake part of it for a bracket.
	for i := 0; i < len(s); {
		w := marker(s[i:])
		if w == 0 {
			i++
			continue
		}
		if n == 0 {
			b.Grow(len(s) + 16)
		}
		b.WriteString(s[last:i])
		b.WriteByte(s[i])
		b.WriteByte('\\')
		b.WriteString(s[i+1 : i+w])
		last, i = i+w, i+w
		n++
	}
	if n == 0 {
		// The overwhelmingly common case, and it hands back the caller's own
		// string rather than a copy of it.
		return s, 0
	}
	b.WriteString(s[last:])
	return b.String(), n
}

// marker reports the length of the control marker at the start of s, or zero if
// there is none.
//
// Only the opening bracket has to be recognized. A special token is matched by
// its exact whole string, so a marker whose opener is broken cannot match one
// however it ends — which is also why `<|` needs no closing `|>` to be worth
// escaping: an unterminated one costs a backslash and settles nothing else.
func marker(s string) int {
	switch {
	case strings.HasPrefix(s, "<|"):
		return 2
	case strings.HasPrefix(s, "</s>"):
		return 4
	case strings.HasPrefix(s, "<s>"):
		return 3
	case len(s) > 0 && s[0] == '[':
		return bracketed(s)
	}
	return 0
}

// bracketed reports the length of a mistral-shaped [NAME] or [/NAME] marker at
// the start of s, or zero.
//
// Upper case and underscore only. That is how every marker in the family is
// spelled ([INST], [/INST], [SYSTEM_PROMPT], [TOOL_CALLS]), and it is also what
// keeps a decision reference out of it: in `[D56]` the digit ends the name
// before the closing bracket, so there is no match and nothing is escaped.
func bracketed(s string) int {
	i := 1
	if i < len(s) && s[i] == '/' {
		i++
	}
	name := i
	for i < len(s) && (s[i] >= 'A' && s[i] <= 'Z' || s[i] == '_') {
		i++
	}
	if i == name || i == len(s) || s[i] != ']' {
		return 0
	}
	return i + 1
}
