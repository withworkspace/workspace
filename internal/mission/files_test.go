package mission

import (
	"errors"
	"sync"
	"testing"
)

// A brief and plan written only if they're as they were: both or
// neither, refused with the one that's changed.
func TestWriteFilesIf(t *testing.T) {
	s := owned(t)
	m, err := s.Create("Cache")
	if err != nil {
		t.Fatal(err)
	}
	brief, _ := s.ReadBrief(m.Slug)
	s.WritePlan(m.Slug, "1. Work: build it.\n")
	plan, _ := s.ReadPlan(m.Slug)

	err = s.WriteFilesIf(m.Slug, FileWrite{Was: brief, Now: "new brief"}, FileWrite{Plan: true, Was: "not it", Now: "new plan"})
	var ch *ChangedError
	if !errors.As(err, &ch) || ch.File != "plan" || !errors.Is(err, ErrChanged) {
		t.Fatalf("err = %v, want the plan changed", err)
	}
	if now, _ := s.ReadBrief(m.Slug); now != brief {
		t.Errorf("the brief was written though the plan had changed: %q", now)
	}
	if err := s.WriteFilesIf(m.Slug, FileWrite{Was: brief, Now: "new brief"}, FileWrite{Plan: true, Was: plan, Now: "new plan"}); err != nil {
		t.Fatal(err)
	}
	if b, _ := s.ReadBrief(m.Slug); b != "new brief\n" {
		t.Errorf("brief = %q", b)
	}
	if p, _ := s.ReadPlan(m.Slug); p != "new plan\n" {
		t.Errorf("plan = %q", p)
	}
}

// Of writers racing from the same brief, one writes it: the rest find it
// changed.
func TestWriteFilesIf_OneOfThoseRacing(t *testing.T) {
	s := owned(t)
	m, _ := s.Create("Cache")
	was, _ := s.ReadBrief(m.Slug)
	var wg sync.WaitGroup
	var mu sync.Mutex
	wrote := 0
	for i := range 8 {
		wg.Go(func() {
			if err := s.WriteFilesIf(m.Slug, FileWrite{Was: was, Now: string(rune('a' + i))}); err == nil {
				mu.Lock()
				wrote++
				mu.Unlock()
			} else if !errors.Is(err, ErrChanged) {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if wrote != 1 {
		t.Errorf("%d wrote it, want 1", wrote)
	}
}
