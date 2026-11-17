package review

import (
	"strings"
	"testing"
)

func TestNumbered(t *testing.T) {
	files := []File{
		{Path: "cache.go", Status: "modified", Hunks: []Hunk{
			{Lines: []Line{{Kind: "c", Old: 3, New: 3, Text: "func key() {"}, {Kind: "d", Old: 4, Text: "\treturn id"}, {Kind: "a", New: 4, Text: "\treturn tenant + id"}}},
			{Lines: []Line{{Kind: "a", New: 40, Text: "// end"}}},
		}},
		{Path: "go.sum", Status: "modified", Skim: SkimLockfile, Hunks: []Hunk{{Lines: []Line{{Kind: "a", New: 1, Text: "x"}}}}},
		{Path: "new.go", Status: "renamed", OldPath: "old.go", Hunks: []Hunk{{Lines: []Line{{Kind: "a", New: 1, Text: strings.Repeat("y", 400)}}}}},
	}
	text, left := Numbered(files, 200)
	want := "=== cache.go (modified)\n3       func key() {\no4     -\treturn id\n4      +\treturn tenant + id\n...\n40     +// end"
	if text != want {
		t.Errorf("text:\n%q\nwant\n%q", text, want)
	}
	if len(left) != 2 || left[0] != (Left{Path: "go.sum", Why: SkimLockfile}) || left[1].String() != "new.go (no room)" {
		t.Errorf("left = %q", left)
	}
	if text, left := Numbered(files, 4000); !strings.Contains(text, "=== new.go (renamed from old.go)") || len(left) != 1 {
		t.Errorf("with room: left = %q, text %q", left, text[:80])
	}
}
