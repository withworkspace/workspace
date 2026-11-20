package cli

import "testing"

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
