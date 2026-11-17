package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// handWritten is a ws.local.toml as a person writes one: comments,
// indentation, a trailing comment, an array over lines, a key ws doesn't
// read, and a section ws doesn't know.
const handWritten = `# My settings for this workspace.
git = "ssh"

# Agents I use here.
[crew]
  harness = "claude"   # Claude Code for now
  model = "opus"

[conversation]
  # Codex, to try it.
  harness = "codex"

[boarded]
  api = [
    "feat",
    "fix",
  ]

[future]
  setting = true
`

func writeLocal(t *testing.T, text string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, LocalFileName), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func readLocal(t *testing.T, root string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, LocalFileName))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// A change is made where the key is, and nothing else in the file moves.
func TestEditLocalKeepsTheFile(t *testing.T) {
	for _, c := range []struct {
		what string
		fn   func(*LocalConfig)
		want string
	}{
		{"a value changed in place, its comment kept", func(l *LocalConfig) { l.Crew.Harness = "cursor" },
			strings.Replace(handWritten, `harness = "claude"   # Claude Code for now`, `harness = "cursor"   # Claude Code for now`, 1)},
		{"a key added after its table's last, indented as they are", func(l *LocalConfig) { l.Conversation.Model = "gpt-6.1-sol[medium]" },
			strings.Replace(handWritten, "  harness = \"codex\"\n", "  harness = \"codex\"\n  model = \"gpt-6.1-sol[medium]\"\n", 1)},
		{"a key removed", func(l *LocalConfig) { l.Crew.Model = "" },
			strings.Replace(handWritten, "  model = \"opus\"\n", "", 1)},
		{"a table emptied of keys but not comments keeps its header", func(l *LocalConfig) { l.Conversation.Harness = "" },
			strings.Replace(handWritten, "  harness = \"codex\"\n", "", 1)},
		{"a table emptied of everything goes", func(l *LocalConfig) { l.Crew = CrewConfig{} },
			strings.Replace(handWritten, "\n# Agents I use here.\n[crew]\n  harness = \"claude\"   # Claude Code for now\n  model = \"opus\"\n", "\n# Agents I use here.\n", 1)},
		{"an array over lines replaced whole", func(l *LocalConfig) { l.Boarded["api"] = []string{"feat"} },
			strings.Replace(handWritten, "  api = [\n    \"feat\",\n    \"fix\",\n  ]\n", "  api = [\"feat\"]\n", 1)},
		{"a table in the middle emptied: one blank line left between its neighbours", func(l *LocalConfig) { l.Conversation = AgentConfig{} },
			strings.Replace(handWritten, "[conversation]\n  # Codex, to try it.\n  harness = \"codex\"\n\n", "[conversation]\n  # Codex, to try it.\n\n", 1)},
		{"a table that loses a key as it gains one stays put", func(l *LocalConfig) { l.Crew.Model = ""; l.Crew.In = "web" },
			strings.Replace(handWritten, "  model = \"opus\"\n", "  in = \"web\"\n", 1)},
		{"a new table at the end", func(l *LocalConfig) { l.Orchestrator.Dispatch = "ask" },
			handWritten + "\n[orchestrator]\ndispatch = \"ask\"\n"},
		{"a pointer to false written", func(l *LocalConfig) { off := false; l.Orchestrator.Enabled = &off },
			handWritten + "\n[orchestrator]\nenabled = false\n"},
		{"a top-level key", func(l *LocalConfig) { l.Git = "https" },
			strings.Replace(handWritten, `git = "ssh"`, `git = "https"`, 1)},
		{"a quoted table", func(l *LocalConfig) {
			l.Repos = map[string]RepoConfig{"my.repo": {Color: "#ff0000"}}
		}, handWritten + "\n[repos.\"my.repo\"]\ncolor = \"#ff0000\"\n"},
	} {
		root := writeLocal(t, handWritten)
		if err := EditLocal(root, c.fn); err != nil {
			t.Errorf("%s: %v", c.what, err)
			continue
		}
		if got := readLocal(t, root); got != c.want {
			t.Errorf("%s:\n--- got\n%s--- want\n%s", c.what, got, c.want)
		}
	}
}

// ws's own writes (a lift boards a capsule) keep your comments now.
func TestSaveBoardedKeepsComments(t *testing.T) {
	root := writeLocal(t, handWritten)
	if err := SaveBoarded(root, map[string][]string{"api": {"feat", "fix"}, "web": {"redesign"}}); err != nil {
		t.Fatal(err)
	}
	got := readLocal(t, root)
	want := strings.Replace(handWritten, "  ]\n", "  ]\n  web = [\"redesign\"]\n", 1)
	if got != want {
		t.Errorf("--- got\n%s--- want\n%s", got, want)
	}
	cfg, _, err := Load(writeWorkspace(t, root))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Crew.Harness != "claude" || len(cfg.Boarded["web"]) != 1 {
		t.Errorf("read back: %+v", cfg)
	}
}

func writeWorkspace(t *testing.T, root string) string {
	t.Helper()
	os.WriteFile(filepath.Join(root, FileName), []byte("[workspace]\norg = \"o\"\n\n[repos.api]\n[repos.web]\n"), 0o644)
	return root
}

// A file ws can't edit key by key: a setting asked for is refused and the
// file left alone; ws's own write rewrites it, as it did before.
func TestEditLocalByHandOnly(t *testing.T) {
	for _, text := range []string{
		"crew.harness = \"claude\"\n",         // a dotted key at the top
		"crew = { harness = \"claude\" }\n",   // an inline table
		"[team]\npeople.ana.name = \"Ana\"\n", // a dotted key inside a table
	} {
		root := writeLocal(t, text)
		err := EditLocal(root, func(l *LocalConfig) {
			l.Crew.Model = "opus"
			if l.Team.People != nil {
				l.Team.People["bo"] = TeamPerson{Name: "Bo"}
			}
		})
		if !errors.Is(err, ErrHandEdit) {
			t.Errorf("%q: EditLocal = %v, want ErrHandEdit", text, err)
		}
		if got := readLocal(t, root); got != text {
			t.Errorf("%q: the file changed to %q", text, got)
		}
		if err := UpdateLocal(root, func(l *LocalConfig) { l.Crew.Model = "opus" }); err != nil {
			t.Errorf("%q: UpdateLocal = %v", text, err)
		}
		cfg, _, err := Load(writeWorkspace(t, root))
		if err != nil || cfg.Crew.Model != "opus" {
			t.Errorf("%q: rewritten, it reads %+v, %v", text, cfg.Crew, err)
		}
	}
}

// What ws doesn't read stays, and a change that changes nothing doesn't
// touch the file.
func TestEditLocalLeavesTheRest(t *testing.T) {
	root := writeLocal(t, handWritten)
	if err := EditLocal(root, func(l *LocalConfig) { l.Crew.Harness = "claude" }); err != nil {
		t.Fatal(err)
	}
	if got := readLocal(t, root); got != handWritten {
		t.Errorf("nothing changed, but the file did:\n%s", got)
	}
	empty := t.TempDir()
	if err := EditLocal(empty, func(l *LocalConfig) { l.Conversation.Harness = "codex" }); err != nil {
		t.Fatal(err)
	}
	if got := readLocal(t, empty); got != "[conversation]\nharness = \"codex\"\n" {
		t.Errorf("a new file says %q", got)
	}
}

// Writers at once (a lift and the settings page) don't lose each other's
// changes.
func TestUpdateLocalLocked(t *testing.T) {
	root := writeLocal(t, handWritten)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := UpdateLocal(root, func(l *LocalConfig) {
				l.Boarded[fmt.Sprintf("repo%d", i)] = []string{"c"}
			}); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	got := readLocal(t, root)
	for i := 0; i < 8; i++ {
		if !strings.Contains(got, fmt.Sprintf("repo%d = [\"c\"]", i)) {
			t.Errorf("repo%d lost:\n%s", i, got)
		}
	}
	if !strings.Contains(got, "# Claude Code for now") {
		t.Error("a comment lost")
	}
}
