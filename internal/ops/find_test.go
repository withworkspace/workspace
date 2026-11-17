package ops

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/review"
	"github.com/withworkspace/workspace/internal/store/storetest"
	"github.com/withworkspace/workspace/internal/workspace"
)

// records is a workspace whose records are testdata/find: a real
// workspace's (ws-test's: five missions, one archived, their decisions,
// threads and reviews, and four of the workspace's talks) as ws.db, and
// its missions' briefs and handoffs as files (records/). The file's layout
// is brought forward as the daemon brings it, and every store says its
// words again, so what's ranked is what this ws says of them.
func records(t *testing.T) *Workspace {
	t.Helper()
	root := t.TempDir()
	db, err := os.ReadFile(filepath.Join("testdata", "find", "ws.db"))
	if err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Join(root, ".ws"), 0o755)
	if err := os.WriteFile(filepath.Join(root, ".ws", "ws.db"), db, 0o644); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join("testdata", "find", "records")
	if err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		to := filepath.Join(root, ".ws", rel)
		os.MkdirAll(filepath.Dir(to), 0o755)
		return os.WriteFile(to, data, 0o644)
	}); err != nil {
		t.Fatal(err)
	}
	storetest.Own(t, root)
	w := New(&config.Config{}, &workspace.Workspace{Root: root}, nil, Options{})
	if _, err := w.SaidWords(context.Background()); err != nil {
		t.Fatal(err)
	}
	return w
}

// TestFind_Ranking holds the ranking of what ws records, as
// internal/space/testdata/ranking.json holds the documents': questions a
// person or an agent would ask of real records, and what should come first.
func TestFind_Ranking(t *testing.T) {
	w := records(t)
	data, err := os.ReadFile(filepath.Join("testdata", "find", "ranking.json"))
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Cases []struct {
			Q      string  `json:"q"`
			Want   string  `json:"want"` // scope/ref, as found: house-sorting-cli/d1, /w1, /house-sorting-cli (a mission)
			At     *string `json:"at"`   // where in it, when said: a turn's number, a section's anchor ("" for the whole)
			Within int     `json:"within"`
			Weak   string  `json:"weak"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatal(err)
	}
	for _, c := range file.Cases {
		found, _, err := w.Find(context.Background(), FindQuery{Words: c.Q, Limit: 5})
		if err != nil {
			t.Errorf("%q: %v", c.Q, err)
			continue
		}
		var got []string
		at := -1
		for i, f := range found {
			name := f.Thing.Key.Scope + "/" + f.Thing.Key.Name
			got = append(got, fmt.Sprintf("%s@%s (%.1f)", name, f.At, f.Score))
			if name == c.Want && at < 0 && (c.At == nil || *c.At == f.At) {
				at = i
			}
		}
		switch {
		case c.Weak != "":
			t.Logf("%q (weak: %s): %s", c.Q, c.Weak, strings.Join(got, ", "))
		case at < 0 || at >= c.Within:
			t.Errorf("%q: want %s within %d, got %s", c.Q, c.Want, c.Within, strings.Join(got, ", "))
		default:
			t.Logf("%q: %s", c.Q, strings.Join(got, ", "))
		}
	}
}

// TestFind_BriefsAndFilesAreReadAgainWhenTheyMove: a brief edited outside
// ws, a handoff written, and one taken away, as a search next finds them.
func TestFind_BriefsAndFilesAreReadAgainWhenTheyMove(t *testing.T) {
	w := records(t)
	ctx := context.Background()
	refs := func(q string) []string {
		t.Helper()
		found, _, err := w.Find(ctx, FindQuery{Words: q})
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, f := range found {
			out = append(out, f.Thing.Key.Scope+"/"+f.Thing.Key.Name+"@"+f.At)
		}
		return out
	}
	if got := refs("quidditch"); len(got) != 0 {
		t.Fatalf("quidditch, before it's said: %q", got)
	}
	slug := "house-sorting-cli"
	dir := filepath.Join(w.Root(), ".ws", "missions", slug)
	brief, _ := os.ReadFile(filepath.Join(dir, "brief.md"))
	os.WriteFile(filepath.Join(dir, "brief.md"), append(brief, "\n## Later\nSort quidditch teams too.\n"...), 0o644)
	os.WriteFile(filepath.Join(dir, "files", "notes.md"), []byte("# Notes\n\nQuidditch can wait.\n"), 0o644)
	got := refs("quidditch")
	slices.Sort(got)
	if got, want := strings.Join(got, " "), "/house-sorting-cli@later house-sorting-cli/mission:notes.md@"; got != want {
		t.Errorf("after the brief's edited and a note's written: %q, want %q", got, want)
	}
	os.Remove(filepath.Join(dir, "files", "notes.md"))
	if got := strings.Join(refs("quidditch"), " "); got != "/house-sorting-cli@later" {
		t.Errorf("after the note's taken away: %q", got)
	}
	// Renamed, it's read again under its new name.
	if err := w.Missions.Rename(slug, "sorting-hat"); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(refs("quidditch"), " "); got != "/sorting-hat@later" {
		t.Errorf("after it's renamed: %q", got)
	}
	// A mission's title is how it's found, as it's changed.
	if _, err := w.Missions.Update("sorting-hat", func(m *mission.Mission) error { m.Title = "The sorting hat"; return nil }); err != nil {
		t.Fatal(err)
	}
	if got := refs("sorting hat"); len(got) == 0 || got[0] != "/sorting-hat@" {
		t.Errorf("by its new title: %q", got)
	}
}

// TestFind_ADecisionAsItsMadeAndWithdrawn: what's written is found at
// once; what's withdrawn, only when asked for.
func TestFind_ADecisionAsItsMadeAndWithdrawn(t *testing.T) {
	w := records(t)
	ctx := context.Background()
	slug := "house-sorting-cli"
	d, err := w.Missions.Decide(slug, mission.Decision{What: "Houses are told in Welsh as well", Why: "The brief's readers are in Cardiff", By: "you", At: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	found, _, _ := w.Find(ctx, FindQuery{Words: "welsh"})
	if len(found) != 1 || found[0].Ref != d.ID || found[0].Show != "ws show "+d.ID+" --mission "+slug {
		t.Fatalf("the decision, made: %+v", found)
	}
	if err := w.Missions.Withdraw(slug, d.ID, "you", "No one asked"); err != nil {
		t.Fatal(err)
	}
	if found, _, _ = w.Find(ctx, FindQuery{Words: "welsh"}); len(found) != 0 {
		t.Errorf("withdrawn, it's still found: %+v", found)
	}
	if found, _, _ = w.Find(ctx, FindQuery{Words: "welsh", Gone: true}); len(found) != 1 {
		t.Errorf("withdrawn, and asked for: %+v", found)
	}
}

// A mission written out is the files it was kept in before the records:
// testdata/find/before holds what ws-test's missions were as files, which
// is what the records were made from, and what writing them out gives
// back. Each line of a log, of decisions and of a conversation is as it
// was; a record and a review say what they said.
func TestExportMission_TheFilesTheyWereKeptIn(t *testing.T) {
	w := records(t)
	before := filepath.Join("testdata", "find", "before", "missions")
	slugs, err := os.ReadDir(before)
	if err != nil {
		t.Fatal(err)
	}
	lines := func(path string) []string {
		data, _ := os.ReadFile(path)
		var out []string
		for _, l := range strings.Split(string(data), "\n") {
			if l = strings.TrimSpace(l); l != "" {
				out = append(out, l)
			}
		}
		return out
	}
	walked, compared := 0, 0
	for _, e := range slugs {
		slug := e.Name()
		dir := filepath.Join(t.TempDir(), slug)
		out, err := w.ExportMission(slug, dir)
		if err != nil {
			t.Fatalf("%s: %v", slug, err)
		}
		if _, err := os.Stat(filepath.Join(dir, "brief.md")); err != nil || out.Files == 0 {
			t.Errorf("%s: its folder wasn't copied: %d files, %v", slug, out.Files, err)
		}
		was := filepath.Join(before, slug)
		filepath.WalkDir(was, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			rel, _ := filepath.Rel(was, p)
			now := filepath.Join(dir, rel)
			walked++
			if _, err := os.Stat(now); err != nil {
				t.Errorf("%s/%s wasn't written", slug, rel)
				return nil
			}
			compared++
			switch {
			case strings.HasSuffix(rel, ".jsonl"):
				if got, want := lines(now), lines(p); !slices.Equal(got, want) {
					t.Errorf("%s/%s: %d lines written, %d were; first differing %v", slug, rel, len(got), len(want), firstDiff(got, want))
				}
			case rel == "mission.toml":
				var a, b mission.Mission
				_, errA := toml.DecodeFile(p, &a)
				_, errB := toml.DecodeFile(now, &b)
				// Kept before confirmations, a mission's records have one now
				// (as it was made, or none, to brief), and no briefing state.
				if a.State == mission.Briefing {
					a.State = ""
				} else if a.Confirmed == nil {
					a.Confirmed = &a.Created
				}
				if errA != nil || errB != nil || !sameJSON(a, b) {
					t.Errorf("%s: mission.toml says something else (%v, %v)", slug, errA, errB)
				}
			case strings.HasPrefix(rel, "reviews"+string(filepath.Separator)):
				var a, b review.Review
				dataA, _ := os.ReadFile(p)
				dataB, errB := os.ReadFile(now)
				json.Unmarshal(dataA, &a)
				json.Unmarshal(dataB, &b)
				a.ID = strings.TrimSuffix(filepath.Base(rel), ".json") // as its file's name said
				if errB != nil || !sameJSON(a, b) {
					t.Errorf("%s/%s says something else (%v)", slug, rel, errB)
				}
			}
			return nil
		})
	}
	if walked < 15 || compared != walked {
		t.Errorf("compared %d of %d files", compared, walked)
	}
	if _, err := w.ExportMission("house-sorting-cli", filepath.Join(t.TempDir())); err != nil {
		t.Errorf("into an empty folder: %v", err)
	}
	full := t.TempDir()
	os.WriteFile(filepath.Join(full, "x"), nil, 0o644)
	if _, err := w.ExportMission("house-sorting-cli", full); err == nil {
		t.Error("written into a folder with something in it")
	}
}

func firstDiff(a, b []string) string {
	for i := range min(len(a), len(b)) {
		if a[i] != b[i] {
			return fmt.Sprintf("line %d:\n  %s\n  %s", i+1, a[i], b[i])
		}
	}
	return "none"
}

func sameJSON(a, b any) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}
