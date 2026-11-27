package cli

import (
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/config"
)

// A run's words after -- are run as they were said: several are each a
// word, one is a command line.
func TestCommandOf(t *testing.T) {
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"pnpm", "dev", "--port", "{port}"}, "pnpm dev --port {port}"},
		{[]string{"sh", "-c", "echo building; exit 3"}, "sh -c 'echo building; exit 3'"},
		{[]string{"echo", "it's", ""}, `echo 'it'\''s' ''`},
		{[]string{"pnpm build && pnpm preview"}, "pnpm build && pnpm preview"},
	} {
		if got := commandOf(c.args); got != c.want {
			t.Errorf("commandOf(%q) = %s, want %s", c.args, got, c.want)
		}
	}
}

// Words after a recipe's name, with no -- before them, are refused: they'd
// be a command of the recipe's name, run as a service in its place.
func TestWordsAfterARecipeAreRefused(t *testing.T) {
	recipes := map[string]config.RunRecipe{"build": {Type: config.RunTask, Command: "make"}}
	err := wordsAfterRecipe(recipes, []string{"build", "--fresh"}, -1)
	if err == nil || !strings.Contains(err.Error(), "flags go before it (ws run --fresh build)") || !strings.Contains(err.Error(), "(ws run -- build --fresh)") {
		t.Errorf("build --fresh: %v", err)
	}
	for _, ok := range []struct {
		args []string
		dash int
	}{
		{[]string{"build"}, -1},
		{[]string{"build", "--fresh"}, 0},
		{[]string{"pnpm", "dev"}, -1},
	} {
		if err := wordsAfterRecipe(recipes, ok.args, ok.dash); err != nil {
			t.Errorf("%v (-- at %d): %v", ok.args, ok.dash, err)
		}
	}
}
