package cli

import "testing"

// --set is a tweak's state as the frame's address says it, t.name=value,
// a value with spaces and = in it as it is; one with no name is refused.
func TestTweakState(t *testing.T) {
	q, err := tweakState([]string{"density=compact", "accent=#3a7", "label=Two takes=both", "density=airy"})
	if err != nil {
		t.Fatal(err)
	}
	if got := q.Encode(); got != "t.accent=%233a7&t.density=airy&t.label=Two+takes%3Dboth" {
		t.Errorf("%q", got)
	}
	for _, bad := range []string{"density", "=compact", " =x"} {
		if _, err := tweakState([]string{bad}); err == nil {
			t.Errorf("%q: no error", bad)
		}
	}
}
