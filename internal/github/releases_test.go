package github

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// asAnyone has GitHub asked as it is with no gh logged in, of a stand-in
// that answers as GitHub's pages do.
func asAnyone(t *testing.T, mux *http.ServeMux) {
	t.Helper()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	ghOnce.Do(func() {})
	wasReady, wasHost := ghReady, githubHost
	ghReady, githubHost = false, srv.URL
	t.Cleanup(func() { ghReady, githubHost = wasReady, wasHost })
}

func TestReleaseAsAnyone(t *testing.T) {
	mux := http.NewServeMux()
	// A repo with a release: /releases/latest sends you to it.
	mux.HandleFunc("GET /acme/plugin-jira/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/acme/plugin-jira/releases/tag/v0.3.0", http.StatusFound)
	})
	mux.HandleFunc("GET /acme/plugin-jira/releases/download/v0.3.0/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("aa  jira_v0.3.0_linux_arm64.tar.gz\nbb *jira_v0.3.0_darwin_arm64.tar.gz\n"))
	})
	mux.HandleFunc("GET /acme/plugin-jira/releases/download/v0.3.0/jira_v0.3.0_linux_arm64.tar.gz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("the build"))
	})
	// One with none: it sends you to its list of them.
	mux.HandleFunc("GET /acme/plugin-new/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/acme/plugin-new/releases", http.StatusFound)
	})
	asAnyone(t, mux)

	tag, files, ok, err := Releases{}.Release("acme/plugin-jira", "")
	if err != nil || !ok || tag != "v0.3.0" {
		t.Fatalf("the latest: %q, %v, %v", tag, ok, err)
	}
	want := []string{"checksums.txt", "jira_v0.3.0_linux_arm64.tar.gz", "jira_v0.3.0_darwin_arm64.tar.gz"}
	if len(files) != len(want) {
		t.Fatalf("its files: %q", files)
	}
	for i := range want {
		if files[i] != want[i] {
			t.Errorf("its files: %q", files)
		}
	}
	// By its tag.
	if tag, _, ok, err := (Releases{}).Release("acme/plugin-jira", "v0.3.0"); err != nil || !ok || tag != "v0.3.0" {
		t.Errorf("by its tag: %q, %v, %v", tag, ok, err)
	}
	// A tag there's no release of, a repo with none, a repo that isn't there.
	for _, c := range [][2]string{{"acme/plugin-jira", "v9"}, {"acme/plugin-new", ""}, {"acme/nothing", ""}} {
		if _, _, ok, err := (Releases{}).Release(c[0], c[1]); ok || err != nil {
			t.Errorf("%s@%s: ok %v, err %v", c[0], c[1], ok, err)
		}
	}

	dir := t.TempDir()
	if err := (Releases{}).Download("acme/plugin-jira", "v0.3.0", []string{"jira_v0.3.0_linux_arm64.tar.gz", "checksums.txt"}, dir); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "jira_v0.3.0_linux_arm64.tar.gz")); string(data) != "the build" {
		t.Errorf("what was downloaded: %q", data)
	}
	if err := (Releases{}).Download("acme/plugin-jira", "v0.3.0", []string{"nope.tar.gz"}, dir); err == nil {
		t.Error("a file the release hasn't got was downloaded")
	}
}
