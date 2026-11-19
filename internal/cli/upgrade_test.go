package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/release"
	"github.com/withworkspace/workspace/internal/release/releasetest"
)

// installedWS is a release's build of ws at version, somewhere it can be
// replaced, with r as where its releases are.
func installedWS(t *testing.T, r *releasetest.Releases, version string) upgrader {
	t.Helper()
	exe := filepath.Join(t.TempDir(), "workspace")
	if err := os.WriteFile(exe, releasetest.Script(version), 0o755); err != nil {
		t.Fatal(err)
	}
	return upgrader{Exe: exe, Version: version, Release: true, Source: release.Source{Base: r.URL}, OS: "linux", Arch: "amd64"}
}

func versionRun(t *testing.T, u upgrader) string {
	t.Helper()
	v, err := release.VersionOf(u.Exe)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// ws upgrade puts the latest release in place of the ws that's running,
// keeping that one.
func TestUpgrade_InstallsTheLatest(t *testing.T) {
	r := releasetest.New(t)
	r.Release("v1.4.0")
	u := installedWS(t, r, "v1.3.0")
	got, err := u.upgrade(context.Background(), "", false)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Installed || !got.Kept || got.From != "v1.3.0" || got.To != "v1.4.0" {
		t.Errorf("upgrade() = %+v", got)
	}
	if v := versionRun(t, u); v != "v1.4.0" {
		t.Errorf("the ws in place is %s, want v1.4.0", v)
	}
}

// It changes nothing when there's nothing later to take, or when it's
// only asked to look.
func TestUpgrade_LeavesTheWSAsItIs(t *testing.T) {
	r := releasetest.New(t)
	r.Release("v1.4.0")
	for _, tc := range []struct {
		name, running, version string
		check                  bool
		want                   upgraded
	}{
		{"it's the latest", "v1.4.0", "", false, upgraded{From: "v1.4.0", To: "v1.4.0", Current: true}},
		{"it's the one named", "v1.4.0", "v1.4.0", false, upgraded{From: "v1.4.0", To: "v1.4.0", Current: true}},
		{"a prerelease ahead of the latest", "v1.5.0-rc.1", "", false, upgraded{From: "v1.5.0-rc.1", To: "v1.4.0", Older: true}},
		{"only looking", "v1.3.0", "", true, upgraded{From: "v1.3.0", To: "v1.4.0"}},
	} {
		u := installedWS(t, r, tc.running)
		got, err := u.upgrade(context.Background(), tc.version, tc.check)
		if err != nil || got != tc.want {
			t.Errorf("%s: upgrade() = %+v, %v: want %+v", tc.name, got, err, tc.want)
		}
		if v := versionRun(t, u); v != tc.running {
			t.Errorf("%s: the ws in place is %s, want %s still", tc.name, v, tc.running)
		}
	}
}

// A release that's named is installed whether it's later or not: an older
// one, or a prerelease.
func TestUpgrade_ToAReleaseThatsNamed(t *testing.T) {
	r := releasetest.New(t)
	r.Release("v1.2.0")
	r.Release("v1.4.0")
	r.Release("v1.5.0-rc.1")
	for _, version := range []string{"v1.2.0", "v1.5.0-rc.1"} {
		u := installedWS(t, r, "v1.4.0")
		got, err := u.upgrade(context.Background(), version, false)
		if err != nil || !got.Installed || got.To != version {
			t.Errorf("upgrade(%s) = %+v, %v", version, got, err)
		}
		if v := versionRun(t, u); v != version {
			t.Errorf("the ws in place is %s, want %s", v, version)
		}
	}
}

// What ws upgrade won't do, it says, and leaves the ws as it is.
func TestUpgrade_Refuses(t *testing.T) {
	r := releasetest.New(t)
	r.Release("v1.4.0")
	for _, tc := range []struct {
		name    string
		change  func(*testing.T, *upgrader)
		version string
		want    string
	}{
		{"a ws built from a clone", func(_ *testing.T, u *upgrader) { u.Release = false }, "", "isn't a release's build"},
		{"what isn't a tag", func(*testing.T, *upgrader) {}, "latest", "isn't a release's tag"},
		{"a release that isn't there", func(*testing.T, *upgrader) {}, "v9.9.9", "is there such a release?"},
		{"a machine with no builds", func(_ *testing.T, u *upgrader) { u.OS, u.Arch = "darwin", "amd64" }, "", "no release of ws for Intel Macs"},
		{"a folder it can't write to", func(t *testing.T, u *upgrader) {
			if os.Getuid() == 0 {
				t.Skip("root writes anywhere")
			}
			dir := filepath.Dir(u.Exe)
			os.Chmod(dir, 0o555)
			t.Cleanup(func() { os.Chmod(dir, 0o755) })
		}, "", "ws upgrade replaces"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			u := installedWS(t, r, "v1.3.0")
			tc.change(t, &u)
			got, err := u.upgrade(context.Background(), tc.version, false)
			if err == nil || !strings.Contains(err.Error(), tc.want) || got.Installed {
				t.Errorf("upgrade() = %+v, %v: want an error with %q", got, err, tc.want)
			}
			if v := versionRun(t, u); v != "v1.3.0" {
				t.Errorf("the ws in place is %s, want v1.3.0 still", v)
			}
		})
	}
}
