package ops

import (
	"slices"
	"testing"
)

func TestANeedIsAnnouncedOnce(t *testing.T) {
	root := t.TempDir()
	first, err := Announce(root, []string{"a", "b"}, []string{"a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(first, []string{"a", "b"}) {
		t.Fatalf("the first to ask is told %v are new, want both", first)
	}
	// A second page, or the first reloaded, asks of the same needs and a
	// new one.
	again, err := Announce(root, []string{"a", "b", "c"}, []string{"a", "b", "c"})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(again, []string{"c"}) {
		t.Fatalf("asked again, %v are new: want only the one that wasn't asked of before", again)
	}
}

func TestANeedThatWentAndCameBackIsNews(t *testing.T) {
	root := t.TempDir()
	if _, err := Announce(root, []string{"a"}, []string{"a"}); err != nil {
		t.Fatal(err)
	}
	// It stopped holding: a claim made without it forgets it.
	if _, err := Announce(root, []string{"b"}, []string{"b"}); err != nil {
		t.Fatal(err)
	}
	back, err := Announce(root, []string{"a"}, []string{"a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(back, []string{"a"}) {
		t.Fatalf("a need that came back is told as %v, want it new again", back)
	}
}

func TestAskingOfNothingAnnouncesNothing(t *testing.T) {
	fresh, err := Announce(t.TempDir(), nil, nil)
	if err != nil || fresh == nil || len(fresh) != 0 {
		t.Fatalf("Announce(nothing) = %v, %v: want an empty list", fresh, err)
	}
}
