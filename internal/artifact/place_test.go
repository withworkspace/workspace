package artifact

import (
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

// What [artifacts] says, read: the team's repo, where promoted ones
// live; ws.local.toml's ".ws", keeping yours here; the messages and
// expiry ws has where it says none; and what it can't use, said and left
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
	if c.Repo != "" || c.Homes() || c.ExpireAfter != DefaultExpiry || c.Messages != DefaultMessages || len(c.Problems) != 0 {
		t.Errorf("none: %+v", c)
	}
	c = ReadConfig(Said{Path: "rec:artifacts/{date}-{slug}/", Promote: " Keep:  {title} ", ExpireAfter: "never"}, repos)
	if c.Repo != "records" || c.Dir != "artifacts" || c.Name != "{date}-{slug}" || !c.Homes() || c.Messages.Promote != "Keep: {title}" || c.Messages.Discard != DefaultMessages.Discard || c.ExpireAfter != 0 {
		t.Errorf("in records: %+v", c)
	}
	c = ReadConfig(Said{Path: "records:artifacts/{date}-{slug}/", Local: ".ws"}, repos)
	if c.Repo != "records" || !c.Local || c.Homes() {
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
	if got := f.Subject(DefaultMessages.Promote); got != "Promote artifact horizon-glow: v3" {
		t.Errorf("promote: %q", got)
	}
	if got := f.Subject(DefaultMessages.Discard); got != "Discard artifact horizon-glow" {
		t.Errorf("discard: %q", got)
	}
}
