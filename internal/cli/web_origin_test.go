package cli

import (
	"testing"

	"github.com/withworkspace/workspace/internal/web"
)

func TestLinkOrigin_LeavesOutTheSignIn(t *testing.T) {
	for link, want := range map[string]string{
		"http://shop.localhost/auth?t=secret&next=%2F":      "http://shop.localhost",
		"http://shop.localhost:7080/auth?t=secret&next=%2F": "http://shop.localhost:7080",
		"http://127.0.0.1:41234/auth?t=secret&next=%2Fteam": "http://127.0.0.1:41234",
	} {
		if got := linkOrigin(link); got != want {
			t.Errorf("linkOrigin(%q) = %q, want %q", link, got, want)
		}
	}
}

func TestWebOrigin_WithoutNamesIsThePort(t *testing.T) {
	t.Setenv("WS_ROUTES", t.TempDir())
	info := web.Info{Port: 41234}
	if got := webOrigin(t.TempDir(), info); got != "http://127.0.0.1:41234" {
		t.Errorf("webOrigin = %q, want the web UI's port", got)
	}
}
