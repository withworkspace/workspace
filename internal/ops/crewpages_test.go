package ops

import (
	"testing"

	uv "github.com/charmbracelet/ultraviolet"
)

func TestBrowserKey(t *testing.T) {
	cases := []struct {
		in   TermKey
		want uv.KeyPressEvent
	}{
		{TermKey{Key: "a"}, uv.KeyPressEvent{Code: 'a', Text: "a"}},
		{TermKey{Key: "A", Shift: true}, uv.KeyPressEvent{Code: 'a', ShiftedCode: 'A', Text: "A", Mod: uv.ModShift}},
		{TermKey{Key: "c", Ctrl: true}, uv.KeyPressEvent{Code: 'c', Mod: uv.ModCtrl}},
		{TermKey{Key: " "}, uv.KeyPressEvent{Code: uv.KeySpace, Text: " "}},
		{TermKey{Key: "Enter"}, uv.KeyPressEvent{Code: uv.KeyEnter}},
		{TermKey{Key: "Tab", Shift: true}, uv.KeyPressEvent{Code: uv.KeyTab, Mod: uv.ModShift}},
		{TermKey{Key: "ArrowUp"}, uv.KeyPressEvent{Code: uv.KeyUp}},
		{TermKey{Key: "F5"}, uv.KeyPressEvent{Code: uv.KeyF5}},
		{TermKey{Key: "é"}, uv.KeyPressEvent{Code: 'é', Text: "é"}},
	}
	for _, c := range cases {
		if got, ok := browserKey(c.in); !ok || got != c.want {
			t.Errorf("%+v: %+v, want %+v", c.in, got, c.want)
		}
	}
	for _, k := range []string{"Shift", "Unidentified", "Dead", "\x07"} {
		if _, ok := browserKey(TermKey{Key: k}); ok {
			t.Errorf("%q is a key", k)
		}
	}
}
