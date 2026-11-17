package web

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/withworkspace/workspace/internal/nav"
	"github.com/withworkspace/workspace/internal/review"
)

func TestNavNames_ACapsulesFileAtACommit(t *testing.T) {
	s, _ := reviewServer(t)
	var d review.Diff
	if code := getInto(t, s, "/api/missions/m/capsules/api/fix/diff", &d); code != 200 {
		t.Fatalf("diff: %d", code)
	}
	q := url.Values{"rev": {d.Head}, "path": {"a.go"}}
	if res := do(t, s, "GET", "/api/missions/m/capsules/api/fix/nav/names?"+q.Encode(), true, nil); res.StatusCode != http.StatusNotFound {
		t.Errorf("no navigation here: %d", res.StatusCode)
	}

	s.Nav = nav.New(t.TempDir())
	res := do(t, s, "GET", "/api/missions/m/capsules/api/fix/nav/names?"+q.Encode(), true, nil)
	var n struct {
		Lang  string
		Lines map[string][][]any
	}
	if err := json.NewDecoder(res.Body).Decode(&n); err != nil || res.StatusCode != 200 {
		t.Fatalf("names: %d %v", res.StatusCode, err)
	}
	// var x = 2: x is defined at columns 4 to 5 of line 3.
	if got := n.Lines["3"]; n.Lang != "go" || len(got) != 1 || got[0][0] != 4.0 || got[0][1] != 5.0 || got[0][2] != "d" {
		t.Errorf("names = %+v", n)
	}
	for _, bad := range []url.Values{
		{"rev": {"HEAD"}, "path": {"a.go"}},
		{"rev": {d.Head}, "path": {"--help"}},
		{"rev": {d.Head}},
	} {
		if res := do(t, s, "GET", "/api/missions/m/capsules/api/fix/nav/names?"+bad.Encode(), true, nil); res.StatusCode != http.StatusBadRequest {
			t.Errorf("names %v: %d", bad, res.StatusCode)
		}
	}
}

func TestNavNames_APullRequestsFileFromItsClone(t *testing.T) {
	w := newReviewsWorld(t)
	w.s.Nav = nav.New("")
	var diff struct{ Head string }
	if code := getInto(t, w.s, "/api/prs/api/5/diff", &diff); code != 200 {
		t.Fatalf("diff: %d", code)
	}
	var n nav.Names
	if code := getInto(t, w.s, "/api/prs/api/5/nav/names?rev="+diff.Head+"&path=a.go", &n); code != 200 {
		t.Fatalf("names: %d", code)
	}
	if n.Lang != "go" || len(n.Lines) == 0 {
		t.Errorf("names = %+v", n)
	}
}

func TestNavDefinition(t *testing.T) {
	s, _ := reviewServer(t)
	s.Nav = nav.New("")
	var d review.Diff
	if code := getInto(t, s, "/api/missions/m/capsules/api/fix/diff", &d); code != 200 {
		t.Fatalf("diff: %d", code)
	}
	base := "/api/missions/m/capsules/api/fix/nav/definition?rev=" + d.Head + "&path=a.go"
	var a nav.Answer
	if code := getInto(t, s, base+"&line=3&col=4", &a); code != 200 {
		t.Fatalf("definition: %d", code)
	}
	if a.Name != "x" || a.Answer != "resolved" || a.How != "defined here" || len(a.Targets) != 1 || a.Targets[0].Line != 3 || a.Targets[0].Col != 4 {
		t.Errorf("definition = %+v", a)
	}
	for q, want := range map[string]int{
		"&line=3&col=0":  http.StatusNotFound, // var: a keyword, not a name
		"&line=0&col=4":  http.StatusBadRequest,
		"&line=3":        http.StatusBadRequest,
		"&line=3&col=-1": http.StatusBadRequest,
		"&line=99&col=0": http.StatusNotFound,
	} {
		if res := do(t, s, "GET", base+q, true, nil); res.StatusCode != want {
			t.Errorf("%s: %d, want %d", q, res.StatusCode, want)
		}
	}
}

func TestNavReferences(t *testing.T) {
	s, _ := reviewServer(t)
	s.Nav = nav.New("")
	var d review.Diff
	if code := getInto(t, s, "/api/missions/m/capsules/api/fix/diff", &d); code != 200 {
		t.Fatalf("diff: %d", code)
	}
	var refs nav.References
	if code := getInto(t, s, "/api/missions/m/capsules/api/fix/nav/references?rev="+d.Head+"&path=a.go&line=3&col=4", &refs); code != 200 {
		t.Fatalf("references: %d", code)
	}
	if refs.Name != "x" || refs.Def == nil || refs.Def.Line != 3 || len(refs.Uses) != 0 {
		t.Errorf("references = %+v", refs)
	}
}
