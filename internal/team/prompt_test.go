package team

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var update = flag.Bool("update", false, "rewrite the golden files from what the prompts say now")

// golden compares a prompt with its golden file, so a change to what an
// agent's told shows in a diff: -update rewrites them.
func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	if *update {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (go test ./internal/team -update writes it)", err)
	}
	if string(want) != got {
		t.Errorf("%s isn't what %s says: go test ./internal/team -update, and look at the diff\n%s", name, path, got)
	}
}

var named = func(repo string) string { return map[string]string{"api": "Bundles API", "web": "Web"}[repo] }

var gistAsk = Ask{
	Subject: Subject{Strand: "api#12", Kind: "pr", Repo: "api", PR: 12, Title: "Bundles say what they hold", Body: "<!-- template -->\nAdds `type` and `contents`.", Of: "alice"},
	Material: Material{
		Files: []FileChange{{Path: "api/bundles.tsp", Additions: 4, Deletions: 1}, {Path: "src/bundle/wire.py", Additions: 10}},
		Diff:  "--- a/api/bundles.tsp\n+++ b/api/bundles.tsp\n@@ -1 +1,4 @@\n-model Bundle {}\n+model Bundle {\n+  type: string;\n+  contents: string[];\n+}\n",
	},
	Touches: []Touch{{Name: "contracts", Title: "API contracts", Files: []string{"api/bundles.tsp"}}},
}

var summaryAsk = SummaryAsk{
	Since: time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC),
	Changes: []Change{
		{Number: 1, Strand: "api#12", Name: "Bundles say what they hold", Repos: []string{"api"}, People: []string{"Alice"}, State: InReview, Gist: "Bundles say what they contain.",
			Know: []Know{{Kind: KnowContract, File: "api/bundles.tsp", Text: "Clients built before this ignore them."}}},
		{Number: 2, Strand: "web#3", Name: "Show bundle contents", Repos: []string{"web"}, People: []string{"Bob"}, State: Done, Gist: "The bundle page lists what's in it."},
	},
}

// What an agent writing a gist, or a period's account, is told, pinned.
func TestPrompts_Golden(t *testing.T) {
	golden(t, "gist", GistPrompt(gistAsk, named))
	golden(t, "summary", SummaryPrompt(summaryAsk, named))
}

// Each prompt's form is read by the parser beside it.
func TestPrompts_FormsRead(t *testing.T) {
	p := GistPrompt(gistAsk, named)
	if !strings.Contains(p, "gist: ") || !strings.Contains(p, "know: ") {
		t.Fatalf("a gist's prompt asks for no gist: and know: lines:\n%s", p)
	}
	gist, know, err := ParseGist("gist: Bundles say what they contain.\nknow: contract | api/bundles.tsp | Clients built before this ignore them.\n", gistAsk.Material.Files)
	if err != nil || gist != "Bundles say what they contain." || len(know) != 1 || know[0].File != "api/bundles.tsp" {
		t.Errorf("a gist: %q %+v %v", gist, know, err)
	}

	s := SummaryPrompt(summaryAsk, named)
	if !strings.Contains(s, "topic: <name> | <its changes' numbers, with commas between> | <the sentences>") {
		t.Fatalf("an account's prompt asks for no topic: lines:\n%s", s)
	}
	topics, err := ParseSummary("topic: Bundle contents | 1, 2 | The API's half is in review; the page's has landed.\n", summaryAsk.Changes)
	if err != nil || len(topics) != 1 || topics[0].Name != "Bundle contents" || len(topics[0].Strands) != 2 {
		t.Errorf("an account: %+v %v", topics, err)
	}
}
