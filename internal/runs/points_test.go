package runs

import (
	"fmt"
	"sync"
	"testing"
)

// Needs pointed at once, from as many writers, are all kept.
func TestPoint_AllKeptPointedAtOnce(t *testing.T) {
	s := Store{Root: t.TempDir()}
	var wg sync.WaitGroup
	for i := range 16 {
		wg.Go(func() {
			if err := s.Point("web", fmt.Sprintf("need-%d", i), "feat"); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if got := s.Points("web"); len(got) != 16 {
		t.Errorf("%d kept, want 16: %v", len(got), got)
	}
	if err := s.Point("web", "need-3", ""); err != nil {
		t.Fatal(err)
	}
	if got := s.Points("web"); len(got) != 15 || got["need-3"] != "" {
		t.Errorf("after pointing need-3 at the nearest again: %v", got)
	}
}
