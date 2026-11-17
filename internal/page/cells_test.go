package page

import (
	"slices"
	"testing"
)

func TestForPage(t *testing.T) {
	defs := Passes([]PassDef{{Name: "adr", Looks: "x"}, {Name: "risk", Looks: "x"}, {Name: "tone", Looks: "x"}})
	kinds := map[string][]string{"decision": {"risk", "adr"}, "note": {"tone"}}
	names := func(ds []PassDef) (out []string) {
		for _, d := range ds {
			if !d.Built() {
				out = append(out, d.Name)
			}
		}
		return out
	}
	if got := names(ForPage(defs, kinds, "")); len(got) != 0 {
		t.Errorf("a page of no kind has its kinds' passes: %v", got)
	}
	if got := names(ForPage(defs, kinds, "decision")); !slices.Equal(got, []string{"risk", "adr"}) {
		t.Errorf("a decision's own passes = %v, want its kind's, in its order", got)
	}
	if got := len(ForPage(defs, kinds, "decision")) - 2; got != len(Builtin) {
		t.Errorf("built-in passes = %d, want %d", got, len(Builtin))
	}
	if got := names(ForPage(defs, map[string][]string{}, "")); !slices.Equal(got, []string{"adr", "risk", "tone"}) {
		t.Errorf("passes no kind names = %v, want every one", got)
	}
}

func TestHasCell(t *testing.T) {
	text := []byte("Intro\n\n<!-- ws:cell w3 -->\nheld\n<!-- /ws:cell -->\n")
	if !HasCell(text, "w3") || HasCell(text, "w30") || HasCell(text, "w4") {
		t.Error("HasCell")
	}
}
