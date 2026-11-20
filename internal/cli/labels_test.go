package cli

import (
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/withworkspace/workspace/internal/web"
)

// Every command an agent might run has words for what it's doing, so a
// new one can't show in a conversation as the command typed.
func TestEveryCommandHasALabel(t *testing.T) {
	seen := map[string]bool{}
	var walk func(c *cobra.Command, path []string)
	walk = func(c *cobra.Command, path []string) {
		for _, k := range c.Commands() {
			if k.Hidden || k.Name() == "help" || k.Name() == "completion" {
				continue
			}
			p := append(path[:len(path):len(path)], k.Name())
			key := strings.Join(p, " ")
			seen[key] = true
			if labels[key] == "" {
				t.Errorf("ws %s has no label", key)
			}
			walk(k, p)
		}
	}
	walk(newBuiltinRootCmd(""), nil)
	for key := range labels {
		if key != "" && !seen[key] {
			t.Errorf("label for ws %s, which isn't a command", key)
		}
	}
}

func TestCommandLabelsHaveAliasesAndValueFlags(t *testing.T) {
	got := commandLabels(newBuiltinRootCmd(""))
	find := func(words ...string) *web.CommandLabel {
		for i := range got {
			if slices.Equal(got[i].Words, words) {
				return &got[i]
			}
		}
		return nil
	}
	root := find()
	if root == nil || root.Label != labels["status"] {
		t.Errorf("ws on its own: %+v", root)
	}
	for _, words := range [][]string{{"mission", "list"}, {"missions", "ls"}, {"mission", "ls"}, {"missions", "list"}} {
		if l := find(words...); l == nil || l.Label != "Listing missions" {
			t.Errorf("%v: %+v", words, l)
		}
	}
	show := find("show")
	if show == nil || show.Label != "Looking up {1}" {
		t.Fatalf("show: %+v", show)
	}
	if !slices.Contains(show.Options, "--format") || !slices.Contains(show.Options, "-f") {
		t.Errorf("show's options miss -f: %v", show.Options)
	}
	for _, o := range show.Options {
		if o == "--help" || o == "-h" {
			t.Errorf("show's options have a flag with no value: %v", show.Options)
		}
	}
}
