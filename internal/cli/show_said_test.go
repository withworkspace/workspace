package cli

import (
	"testing"

	"github.com/withworkspace/workspace/internal/ops"
)

func TestSaidLine_FileKeyIsADocument(t *testing.T) {
	for ref, want := range map[string]string{
		"records/adr/0008-how-loud.md": "a document, the workspace's",
		"records:adr/0008-how-loud.md": "a document, the workspace's",
		"demo:README.md":               "a document, the workspace's",
		"demo/token-cache:a.go":        "a document, the workspace's",
		"mission:handoffs/c1.md":       "something, the workspace's",
		"https://example.com/a/b":      "something, the workspace's",
		"demo/adr/":                    "something, the workspace's",
	} {
		if got := saidLine(ops.Said{Ref: ref}); got != want {
			t.Errorf("saidLine(%q) = %q, want %q", ref, got, want)
		}
	}
}
