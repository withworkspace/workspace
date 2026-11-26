package mission

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// A plan or brief read while it's being written is the one before or the
// one after, never a part of either: the mission's state is worked out of
// them on every load.
func TestWritesAreWhole(t *testing.T) {
	s := owned(t)
	m, err := s.Create("Whole")
	if err != nil {
		t.Fatal(err)
	}
	plans := []string{strings.Repeat("1. Cache the tokens.\n", 2000), strings.Repeat("1. Evict the tokens.\n", 3000)}
	if err := s.WritePlan(m.Slug, plans[0]); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteBrief(m.Slug, plans[1]); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	stop := make(chan struct{})
	wg.Go(func() {
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			s.WritePlan(m.Slug, plans[i%2])
			s.WriteBrief(m.Slug, plans[(i+1)%2])
		}
	})
	for range 500 {
		for _, read := range []func(string) (string, error){s.ReadPlan, s.ReadBrief} {
			if got, err := read(m.Slug); err != nil || (got != plans[0] && got != plans[1]) {
				close(stop)
				wg.Wait()
				t.Fatalf("read %d bytes mid-write (%v)", len(got), err)
			}
		}
	}
	close(stop)
	wg.Wait()
	left, _ := filepath.Glob(filepath.Join(filepath.Dir(s.PlanPath(m.Slug)), ".*"))
	if len(left) > 0 {
		t.Errorf("left behind: %v", left)
	}
	if _, err := os.Stat(s.BriefPath(m.Slug)); err != nil {
		t.Error(err)
	}
}
