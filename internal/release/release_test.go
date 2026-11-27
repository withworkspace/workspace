package release_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/release"
	"github.com/withworkspace/workspace/internal/release/releasetest"
)

// installed is a ws that says it's version, somewhere it can be replaced.
func installed(t *testing.T, version string) string {
	t.Helper()
	exe := filepath.Join(t.TempDir(), "workspace")
	if err := os.WriteFile(exe, releasetest.Script(version), 0o755); err != nil {
		t.Fatal(err)
	}
	return exe
}

// only is what's in a folder, which a fetch that failed leaves as it was.
func only(t *testing.T, dir string, want ...string) {
	t.Helper()
	entries, _ := os.ReadDir(dir)
	var got []string
	for _, e := range entries {
		got = append(got, e.Name())
	}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("%s holds %q, want %q", dir, got, want)
	}
}

// The latest release is the one the releases' "latest" page sends you to,
// and a repo with none says so.
func TestLatest(t *testing.T) {
	r := releasetest.New(t)
	src := release.Source{Base: r.URL}
	if _, err := src.Latest(context.Background()); err == nil || !strings.Contains(err.Error(), "has no release yet") {
		t.Errorf("Latest() of a repo with no release: %v", err)
	}
	r.Release("v1.4.0")
	r.Release("v1.5.0-rc.1") // a prerelease is never the latest
	if got, err := src.Latest(context.Background()); err != nil || got != "v1.4.0" {
		t.Errorf("Latest() = %q, %v: want v1.4.0", got, err)
	}
}

// A release is fetched, checked, run and put in place of the ws that's
// there, which is kept and can be put back.
func TestFetchInstallAndRollback(t *testing.T) {
	r := releasetest.New(t)
	r.Release("v1.4.0")
	exe := installed(t, "v1.3.0")
	src := release.Source{Base: r.URL}

	path, says, err := src.Fetch(context.Background(), "v1.4.0", "linux", "amd64", exe)
	if err != nil || says != "v1.4.0" {
		t.Fatalf("Fetch() = %q, %q, %v", path, says, err)
	}
	if v, _ := release.VersionOf(exe); v != "v1.3.0" {
		t.Errorf("fetching changed the ws in place: it's %s", v)
	}
	if err := release.Install(path, exe); err != nil {
		t.Fatal(err)
	}
	if v, _ := release.VersionOf(exe); v != "v1.4.0" {
		t.Errorf("after the install the ws is %s, want v1.4.0", v)
	}
	only(t, filepath.Dir(exe), ".workspace.prev", "workspace")

	if v, err := release.Rollback(exe); err != nil || v != "v1.3.0" {
		t.Fatalf("Rollback() = %q, %v", v, err)
	}
	if v, _ := release.VersionOf(exe); v != "v1.3.0" {
		t.Errorf("after the rollback the ws is %s, want v1.3.0", v)
	}
	if _, err := release.Rollback(exe); err == nil {
		t.Error("a second rollback put something back")
	}
}

// What can't be trusted or run isn't installed, and nothing's left beside
// the ws that's there.
func TestFetch_Refuses(t *testing.T) {
	r := releasetest.New(t)
	r.Release("v1.4.0")
	// A download that isn't what the release's checksums say.
	r.Release("v1.4.1")
	r.Files["v1.4.1"]["workspace_v1.4.1_linux_amd64.tar.gz"] = releasetest.Archive(releasetest.Script("v6.6.6"))
	// A ws that doesn't run on this machine.
	r.Publish("v1.4.2", []byte("#!/bin/sh\nexit 1\n"))
	// A build missing from the release's checksums.
	r.Release("v1.4.3")
	r.Files["v1.4.3"]["checksums.txt"] = []byte("")

	src := release.Source{Base: r.URL}
	for _, tc := range []struct {
		version, os, arch, want string
	}{
		{"v1.4.1", "linux", "amd64", "does not match the release checksum"},
		{"v1.4.2", "linux", "amd64", "does not run here"},
		{"v1.4.3", "linux", "amd64", "is not in the release's checksums.txt"},
		{"v9.9.9", "linux", "amd64", "no such release"},
		{"v1.4.0", "darwin", "amd64", "no release for Intel Macs"},
		{"v1.4.0", "windows", "amd64", "no release for windows/amd64"},
	} {
		exe := installed(t, "v1.3.0")
		_, _, err := src.Fetch(context.Background(), tc.version, tc.os, tc.arch, exe)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("Fetch(%s, %s/%s) = %v, want %q", tc.version, tc.os, tc.arch, err, tc.want)
		}
		only(t, filepath.Dir(exe), "workspace")
		if v, _ := release.VersionOf(exe); v != "v1.3.0" {
			t.Errorf("Fetch(%s) changed the ws in place: it's %s", tc.version, v)
		}
	}
}

func TestNewer(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want bool
	}{
		{"v1.4.0", "v1.3.9", true},
		{"v1.10.0", "v1.9.0", true},
		{"v1.4.0", "v1.4.0", false},
		{"v1.3.0", "v1.4.0", false},
		{"v1.4.0", "v1.4.0-rc.1", true}, // a release is after its prereleases
		{"v1.4.0-rc.1", "v1.4.0", false},
		{"v1.4.0-rc.2", "v1.4.0-rc.1", true},
		{"v1.4.0", "dev", true}, // what isn't a tag is no later than anything
		{"dev", "v1.4.0", false},
		{"dev", "dev", false},
	} {
		if got := release.Newer(tc.a, tc.b); got != tc.want {
			t.Errorf("Newer(%s, %s) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}
