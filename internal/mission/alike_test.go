package mission

import (
	"encoding/json"
	"os"
	"testing"
)

// testdata/alike.json is every mission the dogfood workspace had raised by
// 2026-11-16, its title and what it was raised as, with the duplicates
// marked by what they're the same as: six crew raising the same crew
// environment leak, two gofmt drift, two the same commit signing failure.
// Alike finds each duplicate pair, and no other.
func TestAlike(t *testing.T) {
	data, err := os.ReadFile("testdata/alike.json")
	if err != nil {
		t.Fatal(err)
	}
	var missions []struct{ Slug, Title, Request, Same string }
	if err := json.Unmarshal(data, &missions); err != nil {
		t.Fatal(err)
	}
	for i, a := range missions {
		for _, b := range missions[i+1:] {
			m1, m2 := &Mission{Title: a.Title, Request: a.Request}, &Mission{Title: b.Title, Request: b.Request}
			same := a.Same != "" && a.Same == b.Same
			if got := Alike(m1.Says(), m2.Says()); got != same {
				t.Errorf("Alike(%s, %s) = %v, want %v", a.Slug, b.Slug, got, same)
			}
		}
	}
}

// Plain words don't make two alike, however many they share; a name from
// the code and enough of the wording do.
func TestAlikeNeedsNames(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{
		{"Launch crew: pick harness and model", "Resumed crew lose their model: launch picks no harness", false},
		{"internal/ops and internal/mission tests are slow", "internal/ops and internal/mission need a doc", false},
		{"TestLinkNames signs its commits through 1Password", "TestLinkNames waits on 1Password to sign a commit", true},
		{"TestLinkNames fails: commits are signed", "TestLinkNames: signed commits fail it", true},
		{"TestLinkNames is slow", "The reader's contents rail scrolls badly", false},
	} {
		if got := Alike(c.a, c.b); got != c.want {
			t.Errorf("Alike(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}
