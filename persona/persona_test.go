package persona

import (
	"testing"

	"github.com/tyler-j-chrestoff/tldreddit/memory"
)

// The handle is what lands in the record, so it is the one thing here that
// outlives the process. The ref names the weights that answered, the display
// names what the persona called itself at the time, and the split is
// [memory.Handle]'s: a ref is stable within a channel, a display never is.
func TestHandleNamesTheWeightsAndTheVoice(t *testing.T) {
	tests := []struct {
		name string
		p    Persona
		want memory.Handle
	}{
		{
			"a persona",
			Persona{Name: "nikola", Model: "qwen3.5:latest"},
			memory.Handle{Ref: "ollama/qwen3.5:latest", Display: "nikola"},
		},
		{
			// Two voices on one set of weights are two participants. If this
			// collapsed to one handle, the record could not say which of them
			// spoke.
			"a second voice on the same weights",
			Persona{Name: "ada", Model: "qwen3.5:latest"},
			memory.Handle{Ref: "ollama/qwen3.5:latest", Display: "ada"},
		},
		{
			// The instruction is not part of identity. That is a real claim
			// and it is worth failing loudly if someone changes it by
			// accident: every bit already written under this handle would
			// keep the old answer.
			"the same persona under a different instruction",
			Persona{Name: "nikola", Model: "qwen3.5:latest", System: "be terse", Temperature: 0.2},
			memory.Handle{Ref: "ollama/qwen3.5:latest", Display: "nikola"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.p.Handle(); got != tt.want {
				t.Errorf("Handle = %+v, want %+v", got, tt.want)
			}
		})
	}
}
