package artifact

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// [artifacts] path is a repo, a folder in it, and an artifact's folder's
// name, with {date} and {slug} (or {number}) in that last part only, and
// ending in / because it's a folder.
func TestParsePath(t *testing.T) {
	for _, c := range []struct{ path, repo, dir, name, err string }{
		{path: "records:artifacts/{date}-{slug}/", repo: "records", dir: "artifacts", name: "{date}-{slug}"},
		{path: "records:design/sketches/{number}-{slug}/", repo: "records", dir: "design/sketches", name: "{number}-{slug}"},
		{path: "records:{slug}/", repo: "records", dir: "", name: "{slug}"},
		{path: "artifacts/{date}-{slug}/", err: "doesn't say which repo"},
		{path: "records:artifacts/{date}-{slug}", err: "ends in /"},
		{path: "records:artifacts/{date}/", err: "neither {slug} nor {number}"},
		{path: "records:{date}/x-{slug}/", err: "only an artifact's own folder"},
		{path: "records:../x/{slug}/", err: "leads out"},
		{path: "records:a/{title}/", err: "{title} isn't something"},
		{path: "records:a/{slug}-{slug}/", err: "{slug} twice"},
	} {
		repo, dir, name, err := ParsePath(c.path)
		if c.err != "" {
			if err == nil || !strings.Contains(err.Error(), c.err) {
				t.Errorf("%s: %v, want %q", c.path, err, c.err)
			}
			continue
		}
		if err != nil || repo != c.repo || dir != c.dir || name != c.name {
			t.Errorf("%s: %q %q %q %v", c.path, repo, dir, name, err)
		}
	}
}

// What [artifacts] says, read: the team's repo, ws.local.toml's ".ws"
// keeping new ones here while the repo's still read, the messages and
// expiry ws has where it says none, and what it can't use said and left
// out.
func TestReadConfig(t *testing.T) {
	repos := func(name string) (string, bool) {
		switch name {
		case "records", "rec":
			return "records", true
		}
		return "", false
	}
	c := ReadConfig(Said{}, repos)
	if c.Repo != "" || c.InRepo() || c.Place() != Local || c.ExpireAfter != DefaultExpiry || c.Messages != DefaultMessages || len(c.Problems) != 0 {
		t.Errorf("none: %+v", c)
	}
	c = ReadConfig(Said{Path: "rec:artifacts/{date}-{slug}/", Add: " Sketch:  {title} ", ExpireAfter: "never"}, repos)
	if c.Repo != "records" || c.Dir != "artifacts" || c.Name != "{date}-{slug}" || !c.InRepo() || c.Place() != "records" || c.Messages.Add != "Sketch: {title}" || c.Messages.Revise != DefaultMessages.Revise || c.ExpireAfter != 0 {
		t.Errorf("in records: %+v", c)
	}
	c = ReadConfig(Said{Path: "records:artifacts/{date}-{slug}/", Local: ".ws"}, repos)
	if c.Repo != "records" || !c.Local || c.InRepo() || c.Place() != Local {
		t.Errorf("ws.local.toml's .ws: %+v", c)
	}
	c = ReadConfig(Said{Path: "elsewhere:artifacts/{slug}/", ExpireAfter: "4 weeks"}, repos)
	if said := strings.Join(c.Problems, "; "); c.Repo != "" || len(c.Problems) != 2 || !strings.Contains(said, "elsewhere isn't one of the workspace's repos") || !strings.Contains(said, "4 weeks\": it's a number of days") {
		t.Errorf("what it can't use: %+v", c)
	}
	if d, err := ParseExpiry("14d"); err != nil || d != 14*24*time.Hour {
		t.Errorf("14d: %v %v", d, err)
	}
}

// A folder's name is made by the workspace's shape for them, -2 past a
// name that's taken, and a {number} one past the highest there; its
// messages are filled in from what its name says.
func TestNameFolder(t *testing.T) {
	made := time.Date(2026, 11, 16, 12, 0, 0, 0, time.Local)
	taken := map[string]bool{"2026-11-16-horizon-glow": true}
	name, err := NameFolder("{date}-{slug}", made, "Horizon glow", nil, func(n string) bool { return taken[n] })
	if err != nil || name != "2026-11-16-horizon-glow-2" {
		t.Errorf("past a taken name: %q %v", name, err)
	}
	name, _ = NameFolder("{number}-{slug}", made, "Horizon glow", []string{"0003-other", "0010-more", "notes"}, func(string) bool { return false })
	if name != "0011-horizon-glow" {
		t.Errorf("numbered: %q", name)
	}
	if f, ok := MatchName("{number}-{slug}", "0011-horizon-glow"); !ok || f.Number != "0011" || f.Slug != "horizon-glow" {
		t.Errorf("read back: %+v %v", f, ok)
	}
	if s := SlugOf("{date}-{slug}", "artifacts/2026-11-16-horizon-glow-2"); s != "horizon-glow-2" {
		t.Errorf("its slug: %q", s)
	}
	f := Fields{Slug: "horizon-glow", Title: "Horizon glow", Version: 3}
	if got := f.Subject(DefaultMessages.Revise); got != "Revise artifact horizon-glow: v3" {
		t.Errorf("revise: %q", got)
	}
	if got := f.Subject(DefaultMessages.Add); got != "Add artifact: Horizon glow" {
		t.Errorf("add: %q", got)
	}
}

// A capsule's versions that the other hasn't are added in flight: each
// numbered one past the latest where its number's been taken, based on
// what the one it revises is numbered now. One that's there already,
// under any number, is the same version.
func TestAddFlying(t *testing.T) {
	at := func(m int) time.Time { return day.Add(time.Duration(m) * time.Minute) }
	ground := &Artifact{Title: "Glow", Made: at(0), Versions: []Version{
		{N: 1, At: at(0), By: "you", Note: "one"},
		{N: 2, BasedOn: 1, At: at(1), By: "you", Note: "two"},
		{N: 3, BasedOn: 2, At: at(5), By: "you", Note: "theirs"},
	}}
	capsule := &Artifact{Title: "Glow", Made: at(0), Versions: []Version{
		{N: 1, At: at(0), By: "you", Note: "one"},
		{N: 2, BasedOn: 1, At: at(1), By: "you", Note: "two"},
		{N: 3, BasedOn: 2, At: at(3), By: "c1", Note: "crew's"},
		{N: 4, BasedOn: 3, At: at(4), By: "c1", Note: "crew's again"},
	}}
	got := AddFlying(ground, capsule, "records/glow")
	if got[3] != 4 || got[4] != 5 || len(ground.Versions) != 5 {
		t.Fatalf("renumbered %v: %+v", got, ground.Versions)
	}
	v4, _ := ground.Version(4)
	v5, _ := ground.Version(5)
	if v4.Note != "crew's" || v4.Was != 3 || v4.BasedOn != 2 || v4.Capsule != "records/glow" || v5.BasedOn != 4 || v5.Was != 4 {
		t.Errorf("v4 %+v, v5 %+v", v4, v5)
	}
	// Once landed as v4 and v5, they're the same versions.
	again := AddFlying(ground, capsule, "records/glow")
	if len(ground.Versions) != 5 || again[3] != 4 || again[4] != 5 {
		t.Errorf("again: %v, %d versions", again, len(ground.Versions))
	}
}

// Kept in a repo, artifacts are read from its ground, and from the
// mission capsules of it, where crew's versions are in flight: listed
// with the capsule, and under the number they'll land as.
func TestRepoGroundAndCapsules(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	ground, capsule := filepath.Join(t.TempDir(), ".ground"), filepath.Join(t.TempDir(), "glow")
	s.Repo = &Repo{Name: "records", Ground: ground, Dir: "artifacts", Capsules: []Capsule{{Ref: "records/glow", Path: capsule, Mission: "glow"}}}

	at := day.UTC()
	folder, a, err := MakeIn(filepath.Join(ground, "artifacts"), "{date}-{slug}", New{Title: "Glow", HTML: []byte(page), At: at, Version: Version{By: "you", Note: "one"}}, nil)
	if err != nil || folder != "2026-11-12-glow" || a.Versions[0].N != 1 {
		t.Fatalf("made %q %+v %v", folder, a, err)
	}
	list, err := s.List(ctx)
	if err != nil || len(list) != 1 || list[0].ID != "a1" || list[0].Place != "records" || list[0].Folder != "artifacts/2026-11-12-glow" || list[0].Where() != "records:artifacts/2026-11-12-glow/" || list[0].InFlight() != "" {
		t.Fatalf("listed %+v %v", list, err)
	}

	// Crew's capsule has it, and a v2 of theirs; and a new one, in flight.
	copyTree(t, filepath.Join(ground, "artifacts"), filepath.Join(capsule, "artifacts"))
	if _, n, err := ReviseIn(filepath.Join(capsule, "artifacts", folder), "a1", Revision{HTML: []byte(page + "<p>2</p>"), At: at.Add(time.Hour), Version: Version{By: "c1", From: "glow", Note: "crew's"}}, 1); err != nil || n != 2 {
		t.Fatalf("crew's v2: %d %v", n, err)
	}
	if _, _, err := MakeIn(filepath.Join(capsule, "artifacts"), "{date}-{slug}", New{Title: "Sketch", HTML: []byte(page), At: at, Version: Version{By: "c1", From: "glow", Note: "new"}}, nil); err != nil {
		t.Fatal(err)
	}
	e, err := s.Get(ctx, "a1")
	if err != nil || len(e.Versions) != 2 || e.Versions[1].Capsule != "records/glow" || len(e.Flying()) != 1 || e.InFlight() != "" {
		t.Fatalf("a1 with crew's v2: %+v %v", e, err)
	}
	if got, _ := os.ReadFile(s.VersionPath(e, 2)); string(got) != page+"<p>2</p>" {
		t.Errorf("v2 is read from the capsule: %q", got)
	}
	sketch, err := s.Get(ctx, "a2")
	if err != nil || sketch.InFlight() != "records/glow" || sketch.Where() != "records/glow:artifacts/2026-11-12-sketch/" {
		t.Fatalf("a2, in flight: %+v %v", sketch, err)
	}

	// Someone else's v2 lands first: crew's is listed as v3.
	if _, _, err := ReviseIn(filepath.Join(ground, "artifacts", folder), "a1", Revision{HTML: []byte(page + "<p>theirs</p>"), At: at.Add(2 * time.Hour), Version: Version{By: "you", Note: "theirs"}}, 0); err != nil {
		t.Fatal(err)
	}
	e, _ = s.Get(ctx, "a1")
	v3, ok := e.Version(3)
	if !ok || v3.Note != "crew's" || v3.Was != 2 || v3.BasedOn != 1 || v3.Capsule != "records/glow" {
		t.Fatalf("crew's, renumbered: %+v", e.Versions)
	}
	if got, _ := os.ReadFile(s.VersionPath(e, 3)); string(got) != page+"<p>2</p>" {
		t.Errorf("v3 is crew's v2.html: %q", got)
	}

	// The capsule's gone without landing: the new one with it.
	os.RemoveAll(capsule)
	s.Repo.Capsules = nil
	if sketch, _ = s.Get(ctx, "a2"); !sketch.Gone {
		t.Errorf("a2's capsule is gone, and it isn't: %+v", sketch)
	}
	if e, _ = s.Get(ctx, "a1"); len(e.Versions) != 2 || len(e.Flying()) != 0 {
		t.Errorf("a1 without the capsule: %+v", e.Versions)
	}
	ids := s.IDs()
	if !slices.Equal(ids, []string{"a1", "a2"}) {
		t.Errorf("ids: %v", ids)
	}
}

func copyTree(t *testing.T, from, to string) {
	t.Helper()
	filepath.WalkDir(from, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(from, p)
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(to, rel), 0o755)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(to, rel), data, 0o644)
	})
}
