package conversation

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseGuide(t *testing.T) {
	in := map[string]bool{"internal/render/render.go": true, "internal/render/render_test.go": true, "cmd/demo/main.go": true, "docs/README.md": true, "go.sum": true}
	g := ParseGuide("Moves the greeting into a package, and colours it.\n\nIt also moves the README.\n\n"+
		"## differs\n- The description doesn't say the output is now green.\n- `main.go` now takes a name.\n\n"+
		"## read: The greeting itself\nStart here: it's what the rest follows from.\n- internal/render/render.go: the greeting, moved, now through colour\n- `internal/render/render_test.go`: its test\n- not/there.go: nothing\n\n"+
		"## Read: **What calls it**\n- cmd/demo/main.go - the command, which now takes a name\n- internal/render/render.go: placed twice\n\n"+
		"## skim: moves\nNothing in these changes but where they are.\n- docs/README.md: renamed\n- go.sum\n\n"+
		"## read: Empty\n- nope.go: not in it\n\n"+
		"## notes\nSomething else.\n", func(p string) bool { return in[p] })
	want := &ReviewGuide{
		Does:    "Moves the greeting into a package, and colours it.\n\nIt also moves the README.",
		Differs: []string{"The description doesn't say the output is now green.", "`main.go` now takes a name."},
		Groups: []GuideGroup{
			{Name: "The greeting itself", Why: "Start here: it's what the rest follows from.", Files: []GuideFile{
				{Path: "internal/render/render.go", Note: "the greeting, moved, now through colour"}, {Path: "internal/render/render_test.go", Note: "its test"}}},
			{Name: "What calls it", Files: []GuideFile{{Path: "cmd/demo/main.go", Note: "the command, which now takes a name"}}},
			{Name: "Moves", Why: "Nothing in these changes but where they are.", Skim: true, Files: []GuideFile{{Path: "docs/README.md", Note: "renamed"}, {Path: "go.sum"}}},
		},
	}
	if !reflect.DeepEqual(g, want) {
		t.Errorf("guide:\n got %+v\nwant %+v", g, want)
	}
	// Nothing differs, and it says so.
	if g := ParseGuide("Fixes a typo.\n\n## differs\n- Nothing.\n\n## read: It\n- docs/README.md: the typo\n", func(p string) bool { return in[p] }); len(g.Differs) != 0 || len(g.Groups) != 1 {
		t.Errorf("nothing differs: %+v", g)
	}
}

// Read as it's written: what it does so far, word by word; a heading
// still being written isn't taken for more of that; a group shows once a
// file's placed in it, and a file once its path is whole.
func TestParseGuideSoFar(t *testing.T) {
	in := func(p string) bool { return p == "a.go" || p == "a_test.go" }
	reply := "Makes x two, and tests it.\n\n## differs\n- Nothing.\n\n## read: The change\nWhere it starts.\n- a.go: one becomes two\n- a_test.go: its test\n"
	for _, c := range []struct {
		upTo, does string
		groups     int
		files      []string
	}{
		{"Makes x", "Makes x", 0, nil},
		{"Makes x two, and tests it.\n\n#", "Makes x two, and tests it.", 0, nil},
		{"Makes x two, and tests it.\n\n## diff", "Makes x two, and tests it.", 0, nil},
		{"## read: The cha", "Makes x two, and tests it.", 0, nil},
		{"- a.g", "Makes x two, and tests it.", 0, nil},
		{"- a.go: one bec", "Makes x two, and tests it.", 1, []string{"a.go: one bec"}},
		{"- a_test.go: its test\n", "Makes x two, and tests it.", 1, []string{"a.go: one becomes two", "a_test.go: its test"}},
	} {
		i := strings.Index(reply, c.upTo)
		if i < 0 {
			t.Fatalf("%q isn't in the reply", c.upTo)
		}
		g := ParseGuideSoFar(reply[:i+len(c.upTo)], in)
		var files []string
		for _, gr := range g.Groups {
			for _, f := range gr.Files {
				files = append(files, f.Path+": "+f.Note)
			}
		}
		if g.Does != c.does || len(g.Groups) != c.groups || !reflect.DeepEqual(files, c.files) || len(g.Differs) != 0 {
			t.Errorf("up to %q: %+v", c.upTo, g)
		}
	}
	if g := ParseGuideSoFar("", in); g.Does != "" || len(g.Groups) != 0 {
		t.Errorf("nothing yet: %+v", g)
	}
}
