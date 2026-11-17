package find

import (
	"encoding/json"
	"flag"
	"math"
	"os"
	"reflect"
	"sort"
	"testing"
)

var update = flag.Bool("update", false, "fill in testdata/fuzzy.json from this matcher")

type fuzzyCase struct {
	Text  string   `json:"text"`
	Query string   `json:"query"`
	Match *bool    `json:"match,omitempty"`
	Score *float64 `json:"score,omitempty"`
	At    []int    `json:"at,omitempty"`
}

type fuzzyOrder struct {
	Query string   `json:"query"`
	Texts []string `json:"texts"`
}

type fuzzyCases struct {
	About  string       `json:"about"`
	Cases  []fuzzyCase  `json:"cases"`
	Orders []fuzzyOrder `json:"orders"`
}

func round(f float64) float64 { return math.Round(f*1000) / 1000 }

// The cases the web UI's matcher is held to as well: the two rank alike.
func TestFuzzyCases(t *testing.T) {
	const path = "testdata/fuzzy.json"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var all fuzzyCases
	if err := json.Unmarshal(data, &all); err != nil {
		t.Fatal(err)
	}
	for i, c := range all.Cases {
		m, ok := NewQuery(c.Query).Match(c.Text, nil)
		score := round(m.Score)
		at := Marks(c.Text, m.At)
		if *update {
			all.Cases[i].Match, all.Cases[i].Score, all.Cases[i].At = &ok, nil, nil
			if ok {
				all.Cases[i].Score, all.Cases[i].At = &score, at
			}
			continue
		}
		if c.Match == nil {
			t.Fatalf("%q in %q: not filled in: run with -update", c.Query, c.Text)
		}
		if ok != *c.Match {
			t.Errorf("%q in %q: match %v, want %v", c.Query, c.Text, ok, *c.Match)
			continue
		}
		if !ok {
			continue
		}
		if score != *c.Score {
			t.Errorf("%q in %q: score %v, want %v", c.Query, c.Text, score, *c.Score)
		}
		if len(at) != len(c.At) || (len(at) > 0 && !reflect.DeepEqual(at, c.At)) {
			t.Errorf("%q in %q: at %v, want %v", c.Query, c.Text, at, c.At)
		}
	}
	for _, o := range all.Orders {
		q := NewQuery(o.Query)
		got := append([]string(nil), o.Texts...)
		sort.SliceStable(got, func(i, j int) bool {
			a, _ := q.Match(got[i], nil)
			b, _ := q.Match(got[j], nil)
			return a.Score > b.Score
		})
		if !reflect.DeepEqual(got, o.Texts) {
			t.Errorf("%q orders %v, want %v", o.Query, got, o.Texts)
		}
		for _, text := range o.Texts {
			if _, ok := q.Match(text, nil); !ok {
				t.Errorf("%q doesn't match %q", o.Query, text)
			}
		}
	}
	if *update {
		out, _ := json.MarshalIndent(all, "", "  ")
		if err := os.WriteFile(path, append(out, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestMask(t *testing.T) {
	q := NewQuery("Main go")
	if !q.Covers(Mask("demo/main.go")) {
		t.Error("demo/main.go should cover main go")
	}
	if q.Covers(Mask("demo/readme.md")) {
		t.Error("demo/readme.md has no n or i")
	}
	if !NewQuery("").Covers(0) {
		t.Error("nothing typed covers everything")
	}
	// A text that covers the mask needn't match; one that doesn't, can't.
	for _, text := range []string{"og.niam", "main.go", "a/b/c", "MAIN.GO"} {
		if _, ok := q.Match(text, nil); ok && !q.Covers(Mask(text)) {
			t.Errorf("%q matches but its mask doesn't cover", text)
		}
	}
}

func TestMarksCountAsABrowserDoes(t *testing.T) {
	text := "docs/café/naïve.md"
	m, ok := NewQuery("naive").Match(text, nil)
	if ok {
		t.Fatalf("naive shouldn't match naïve: %v", m)
	}
	m, ok = NewQuery("cafmd").Match(text, nil)
	if !ok {
		t.Fatal("cafmd should match")
	}
	// é and ï are two bytes each and one place each.
	if got, want := Marks(text, m.At), []int{5, 6, 7, 16, 17}; !reflect.DeepEqual(got, want) {
		t.Errorf("marks %v, want %v (bytes %v)", got, want, m.At)
	}
}

func TestScratchIsReused(t *testing.T) {
	q := NewQuery("main")
	s := &Scratch{}
	a, _ := q.Match("demo/main.go", s)
	first := append([]int(nil), a.At...)
	q.Match("x/y/main_test.go", s)
	b, _ := q.Match("demo/main.go", s)
	if !reflect.DeepEqual(first, b.At) {
		t.Errorf("the same text matched differently: %v then %v", first, b.At)
	}
	if n := testing.AllocsPerRun(100, func() { q.Match("demo/internal/main.go", s) }); n > 0 {
		t.Errorf("a match with scratch allocates %v times", n)
	}
}
