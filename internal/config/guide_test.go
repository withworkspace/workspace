package config

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// guideKey is a key's line in the guide's Every key: "- `run.task.<name>.command`: …".
var guideKey = regexp.MustCompile("^- `([a-z_<>.]+)`:")

// guideKeys are the keys the guide's Every key lists.
func guideKeys(t *testing.T) []string {
	t.Helper()
	_, ref, ok := strings.Cut(Guide(), "\n## Every key\n")
	if !ok {
		t.Fatal("the guide has no Every key section")
	}
	var out []string
	for _, line := range strings.Split(ref, "\n") {
		if m := guideKey.FindStringSubmatch(line); m != nil {
			out = append(out, m[1])
		}
	}
	return out
}

// Every key ws reads is in the guide, and nothing's there that ws doesn't
// read: a key added to a file's struct fails here until it's described.
func TestGuideHasEveryKey(t *testing.T) {
	listed := guideKeys(t)
	keys := Keys()
	for _, k := range SortedKeys() {
		if !slices.Contains(listed, k) {
			t.Errorf("the guide (internal/config/guide.md) doesn't list %s (%s): add it under Every key", k, keys[k])
		}
	}
	seen := map[string]bool{}
	for _, k := range listed {
		if _, ok := keys[k]; !ok {
			t.Errorf("the guide lists %s, which ws doesn't read", k)
		}
		if seen[k] {
			t.Errorf("the guide lists %s twice", k)
		}
		seen[k] = true
	}
}

func TestKeysWalksEmbeddedMapsAndPointers(t *testing.T) {
	keys := Keys()
	for k, want := range map[string][]KeyFile{
		"conversation.turn_minutes":          {InTeam, InLocal}, // beside AgentConfig's, embedded
		"conversation.model":                 {InTeam, InLocal},
		"run.service.<name>.page.title":      {InTeam, InRepo}, // a pointer to a table
		"run.task.<name>.needs.<name>.mount": {InTeam, InRepo},
		"repos.<name>.documents":             {InTeam, InLocal}, // read by UnmarshalTOML: a key, not a table
		"documents":                          {InRepo},
		"plugin":                             {InTeam, InLocal}, // the plugin's keys are its own
		"router.enabled":                     {InLocal},
	} {
		if got := keys[k]; !slices.Equal(got, want) {
			t.Errorf("Keys()[%q] = %v, want %v", k, got, want)
		}
	}
	for _, k := range []string{"github.fixture", "boarded", "git"} {
		if slices.Contains(keys[k], InTeam) {
			t.Errorf("%s is ws.local.toml's alone (toml:\"-\" in Config), but Keys has it in ws.toml", k)
		}
	}
}

func TestSaidAndTops(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ws.repo.toml")
	if got, err := Said(path); err != nil || got != nil {
		t.Fatalf("Said(missing) = %v, %v; want nil, nil", got, err)
	}
	os.WriteFile(path, []byte("landing = \"direct\"\n[capsule]\nafter_create = \"pnpm i\"\n[run.task.test]\ncommand = \"pnpm test\"\n"), 0o644)
	said, err := Said(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"landing", "capsule.after_create", "run.task.test.command"} {
		if !slices.Contains(said, k) {
			t.Errorf("Said = %v, missing %s", said, k)
		}
	}
	if got, want := Tops(said), []string{"landing", "capsule", "run"}; !slices.Equal(got, want) {
		t.Errorf("Tops = %v, want %v", got, want)
	}
}
