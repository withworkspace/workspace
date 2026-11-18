package ref

import (
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// The cases are shared with the web UI's finder (web/src/refs), which is
// tested against the same file.

type knownJSON struct {
	Repos     map[string]*[]string `json:"repos"`
	Aliases   map[string]string    `json:"aliases"`
	Capsules  map[string][]string  `json:"capsules"`
	Root      string               `json:"root"`
	Missions  []string             `json:"missions"`
	Pages     []string             `json:"pages"`
	Talks     []string             `json:"talks"`
	Artifacts []string             `json:"artifacts"`
	Things    []struct {
		Plugin   string   `json:"plugin"`
		What     string   `json:"what"`
		Prefixes []string `json:"prefixes"`
		Tail     string   `json:"tail"`
		Address  string   `json:"address"`
	} `json:"things"`
	Org           string                       `json:"org"`
	DefaultBranch string                       `json:"defaultBranch"`
	Branches      map[string]map[string]string `json:"branches"`
}

type cases struct {
	Finds []struct {
		About string    `json:"about"`
		Known knownJSON `json:"known"`
		Cases []struct {
			Text  string            `json:"text"`
			Found []json.RawMessage `json:"found"`
		} `json:"cases"`
	} `json:"finds"`
	Names []struct {
		Ref  Ref    `json:"ref"`
		Name string `json:"name"`
	} `json:"names"`
	Addresses []struct {
		Ref     Ref    `json:"ref"`
		Where   Where  `json:"where"`
		Address string `json:"address"`
	} `json:"addresses"`
	Read struct {
		Known knownJSON `json:"known"`
		Cases []struct {
			Address string `json:"address"`
			Ref     *Ref   `json:"ref"`
		} `json:"cases"`
	} `json:"read"`
	// Parse is names read by their form alone, and how each is written
	// back: a document's stored ref is written as the file it is.
	Parse []struct {
		About   string `json:"about"`
		Name    string `json:"name"`
		Ref     *Ref   `json:"ref"`
		Written string `json:"written"`
	} `json:"parse"`
}

// Paths is a Tree of a list of paths, as a test has it.
type Paths []string

func (p Paths) Has(path string) bool {
	i := sort.SearchStrings(p, path)
	return i < len(p) && p[i] == path
}

func (p Paths) HasDir(path string) bool {
	path = strings.TrimSuffix(path, "/") + "/"
	i := sort.SearchStrings(p, path)
	return i < len(p) && strings.HasPrefix(p[i], path)
}

func (j knownJSON) known() *Known {
	k := &Known{Repos: map[string]Tree{}, Aliases: j.Aliases, Capsules: j.Capsules, Root: j.Root, Missions: j.Missions, Pages: j.Pages, Talks: j.Talks, Artifacts: j.Artifacts, Org: j.Org, DefaultBranch: j.DefaultBranch, Branches: j.Branches}
	for _, n := range j.Things {
		k.Things = append(k.Things, Named{Plugin: n.Plugin, What: n.What, Prefixes: n.Prefixes, Tail: n.Tail, Address: n.Address})
	}
	for r, files := range j.Repos {
		if files == nil {
			k.Repos[r] = nil
			continue
		}
		ps := Paths(append([]string(nil), *files...))
		sort.Strings(ps)
		k.Repos[r] = ps
	}
	return k
}

func load(t *testing.T) cases {
	t.Helper()
	b, err := os.ReadFile("testdata/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var c cases
	if err := json.Unmarshal(b, &c); err != nil {
		t.Fatal(err)
	}
	return c
}

// The Known's Ws and Ds want trimming the way JSON doesn't care about.
type found struct {
	Words string
	Ref   Ref
}

func TestFind(t *testing.T) {
	for _, g := range load(t).Finds {
		k := g.Known.known()
		for _, c := range g.Cases {
			var want []found
			for _, raw := range c.Found {
				var pair [2]json.RawMessage
				if err := json.Unmarshal(raw, &pair); err != nil {
					t.Fatal(err)
				}
				var f found
				json.Unmarshal(pair[0], &f.Words)
				json.Unmarshal(pair[1], &f.Ref)
				want = append(want, f)
			}
			var got []found
			for _, f := range k.Find(c.Text) {
				got = append(got, found{c.Text[f.From:f.To], f.Ref})
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("%s: Find(%q)\n got  %+v\n want %+v", g.About, c.Text, got, want)
			}
		}
	}
}

func TestNames(t *testing.T) {
	for _, c := range load(t).Names {
		if got := c.Ref.Name(); got != c.Name {
			t.Errorf("Name(%+v) = %q, want %q", c.Ref, got, c.Name)
		}
		// A name is read back as what it names (a repo's alone is read as
		// a mission's, without knowing which repos there are).
		// A plugin's thing is read with how its plugin names things.
		if c.Ref.Kind == Repo || c.Ref.Kind == Thing {
			continue
		}
		if got, ok := Parse(c.Name); !ok || !reflect.DeepEqual(got, c.Ref) {
			t.Errorf("Parse(%q) = %+v, %v; want %+v", c.Name, got, ok, c.Ref)
		}
	}
}

func TestAddresses(t *testing.T) {
	c := load(t)
	for _, a := range c.Addresses {
		if got := a.Ref.Address(a.Where); got != a.Address {
			t.Errorf("Address(%+v) = %q, want %q", a.Ref, got, a.Address)
		}
	}
	k := c.Read.Known.known()
	for _, a := range c.Read.Cases {
		got, ok := ParseAddress(a.Address, k)
		if a.Ref == nil {
			if ok {
				t.Errorf("ParseAddress(%q) = %+v, want nothing", a.Address, got)
			}
			continue
		}
		if !ok || !reflect.DeepEqual(got, *a.Ref) {
			t.Errorf("ParseAddress(%q) = %+v, %v; want %+v", a.Address, got, ok, *a.Ref)
		}
	}
}

func TestParse(t *testing.T) {
	for _, c := range load(t).Parse {
		got, ok := Parse(c.Name)
		if c.Ref == nil {
			if ok {
				t.Errorf("%s: Parse(%q) = %+v, want nothing", c.About, c.Name, got)
			}
			continue
		}
		if !ok || got != *c.Ref {
			t.Errorf("%s: Parse(%q) = %+v, %v; want %+v", c.About, c.Name, got, ok, *c.Ref)
		}
		if w := got.Name(); w != c.Written {
			t.Errorf("%s: Parse(%q).Name() = %q, want %q", c.About, c.Name, w, c.Written)
		}
	}
	for in, want := range map[string]Ref{
		"demo/main.go:5":      {Kind: File, Repo: "demo", Worktree: Ground, Path: "main.go", Line: 5},
		"demo/token-cache:a/": {Kind: Dir, Repo: "demo", Worktree: "token-cache", Path: "a"},
	} {
		if got, ok := Parse(in); !ok || got != want {
			t.Errorf("Parse(%q) = %+v, %v; want %+v", in, got, ok, want)
		}
	}
	for _, in := range []string{"", "a b", "/x", "Demo Thing"} {
		if got, ok := Parse(in); ok {
			t.Errorf("Parse(%q) = %+v, want nothing", in, got)
		}
	}
}

func TestLink(t *testing.T) {
	k := (knownJSON{Repos: map[string]*[]string{"demo": {"main.go", "README.md"}}, Missions: []string{"fix-token-refresh"}}).known()
	where := Where{Org: "acme", DefaultBranch: "main"}
	address := func(f Found, said string) (string, bool) {
		r := f.Ref
		if r.Kind == File && r.Line > 0 {
			r.Commit = "4f2a9c1"
		}
		return r.Address(where), true
	}
	in := "# demo/main.go\n\nIt's in demo/main.go:5, and `demo/README.md`; see [demo](https://x/demo/main.go) and `cat demo/main.go`.\n" +
		"<!-- ws:cell w1 v1 | demo/main.go -->\n```\ndemo/main.go\n```\nFor fix-token-refresh, https://github.com/acme/demo/blob/main/main.go stays.\n"
	want := "# demo/main.go\n\nIt's in [demo/main.go:5](https://github.com/acme/demo/blob/4f2a9c1/main.go#L5), and [`demo/README.md`](https://github.com/acme/demo/blob/main/README.md); see [demo](https://x/demo/main.go) and `cat demo/main.go`.\n" +
		"<!-- ws:cell w1 v1 | demo/main.go -->\n```\ndemo/main.go\n```\nFor [fix-token-refresh](ws:fix-token-refresh), https://github.com/acme/demo/blob/main/main.go stays.\n"
	if got := k.Link(in, address); got != want {
		t.Errorf("Link:\n got  %q\n want %q", got, want)
	}
}
