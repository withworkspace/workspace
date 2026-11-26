package cli

import (
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/store"
)

func TestStandsSaid(t *testing.T) {
	now := time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)
	day := 24 * time.Hour
	found := func(s store.Stands, ended bool, said time.Time) ops.Found {
		return ops.Found{Stands: s, Ended: ended, Said: said}
	}
	for _, c := range []struct {
		f    ops.Found
		want string
	}{
		{found(store.Stands{Standing: store.Passing}, false, now.Add(-19*day)), "said 2 weeks ago"},
		{found(store.Stands{Standing: store.InForce}, false, now.Add(-day)), "said yesterday"},
		{found(store.Stands{}, false, time.Time{}), ""},
		{found(store.Stands{Standing: store.Settled, Since: now.Add(-21 * day)}, true, now.Add(-30*day)), "archived 3 weeks ago"},
		{found(store.Stands{Standing: store.Settled}, false, now.Add(-4*day)), "settled, said 4 days ago"},
		{found(store.Stands{Standing: store.Settled, Since: now}, false, now.Add(-4*day)), "settled today"},
		{found(store.Stands{Standing: store.Gone, Since: now.Add(-day)}, true, now.Add(-30*day)), "no longer so, since yesterday"},
		{found(store.Stands{Standing: store.Gone}, false, now.Add(-90*day)), "no longer so, said 3 months ago"},
		{found(store.Stands{}, true, time.Time{}), "archived"},
	} {
		if got := standsSaid(c.f, now); got != c.want {
			t.Errorf("%+v: %q, want %q", c.f.Stands, got, c.want)
		}
	}
}
