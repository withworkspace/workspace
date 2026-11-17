package harness

import (
	"os"
	"strings"
	"testing"
)

// Screens drawn as Claude Code draws its menus (colours included): a
// permission to run a command, and a question with its own options.
const permissionScreen = "\x1b[1m Bash command\x1b[0m\n" +
	"\n" +
	"   rm -rf build\n" +
	"   Remove the build folder\n" +
	"\n" +
	" Do you want to proceed?\n" +
	" \x1b[36m❯ 1. Yes\x1b[0m\n" +
	"   2. Yes, and don't ask again for rm commands in /w/repos/api/fix\n" +
	"   3. No, and tell Claude what to do differently (esc)\n"

const questionScreen = "╭──────────────────────────────╮\n" +
	"│ Which cache key?             │\n" +
	"│                              │\n" +
	"│   1. Per tenant              │\n" +
	"│      Keys carry the tenant   │\n" +
	"│ ❯ 2. Global                  │\n" +
	"│   3. Type something.         │\n" +
	"╰──────────────────────────────╯\n" +
	"\n" +
	" Enter to select · Esc to cancel\n"

func TestReadPermissionPrompt(t *testing.T) {
	p, ok := ReadPrompt(permissionScreen)
	if !ok || len(p.Options) != 3 || p.Selected != 0 || !p.IsPermission() {
		t.Fatalf("%+v %v", p, ok)
	}
	if p.Title != "Do you want to proceed?" {
		t.Errorf("title %q", p.Title)
	}
	if p.PermissionChoice("allow") != 0 || p.PermissionChoice("always") != 1 || p.PermissionChoice("deny") != 2 {
		t.Errorf("choices: allow %d always %d deny %d", p.PermissionChoice("allow"), p.PermissionChoice("always"), p.PermissionChoice("deny"))
	}
	if p.Moves(2) != 2 {
		t.Errorf("moves to 3: %d", p.Moves(2))
	}
}

func TestReadQuestionPrompt(t *testing.T) {
	p, ok := ReadPrompt(questionScreen)
	if !ok || len(p.Options) != 3 || p.Selected != 1 || p.IsPermission() {
		t.Fatalf("%+v %v", p, ok)
	}
	if p.Title != "Which cache key?" {
		t.Errorf("title %q", p.Title)
	}
	if p.Option("Per tenant") != 0 || p.Option("global") != 1 || p.FreeText() != 2 || p.Option("Nope") != -1 {
		t.Errorf("options %+v", p.Options)
	}
	if p.Moves(0) != -1 {
		t.Errorf("moves up: %d", p.Moves(0))
	}
}

func TestReadNoPrompt(t *testing.T) {
	for _, screen := range []string{
		"> Working on it…\n  ✻ Thinking\n",
		"Steps:\n1. Read the brief\nThen some prose.\n", // one numbered line isn't a menu
		"",
	} {
		if p, ok := ReadPrompt(screen); ok {
			t.Errorf("read a prompt in %q: %+v", screen, p)
		}
	}
}

// A real screen (claude 2.1.x, as the daemon keeps it, colours and
// all): a Bash command waiting for permission, its "always" option wrapped
// over two lines, its last line drawn over by the status bar.
func TestReadRealPermissionScreen(t *testing.T) {
	data, err := os.ReadFile("testdata/claude-permission-screen.txt")
	if err != nil {
		t.Fatal(err)
	}
	p, ok := ReadPrompt(string(data))
	if !ok || !p.IsPermission() || len(p.Options) != 4 || p.Selected != 0 {
		t.Fatalf("%+v %v", p, ok)
	}
	if p.PermissionChoice("allow") != 0 || p.PermissionChoice("always") != 1 || p.PermissionChoice("deny") != 3 {
		t.Errorf("allow %d always %d deny %d in %q", p.PermissionChoice("allow"), p.PermissionChoice("always"), p.PermissionChoice("deny"), p.Options)
	}
	if ctx := strings.Join(p.Context, " | "); !strings.Contains(ctx, "Bash command") || !strings.Contains(ctx, "touch vet-ran.txt") {
		t.Errorf("context %q", ctx)
	}
}
