package mission

import "testing"

func TestPRChange(t *testing.T) {
	open := SeenPR{Number: 88, Review: "REVIEW_REQUIRED", Checks: "pending"}
	with := func(f func(p *SeenPR)) SeenPR { p := open; f(&p); return p }
	cases := []struct {
		name string
		prev *SeenPR
		now  SeenPR
		text string
		act  bool
	}{
		{"first sighting", nil, open, "", false},
		{"nothing changed", &open, open, "", false},
		{"checks go red", &open, with(func(p *SeenPR) { p.Checks = "failure" }), "api#88's checks are failing", true},
		{"changes requested", &open, with(func(p *SeenPR) { p.Review = "CHANGES_REQUESTED" }), "api#88: a reviewer asked for changes", true},
		{"merged", &open, with(func(p *SeenPR) { p.Merged = true }), "api#88 merged", true},
		{"approved", &open, with(func(p *SeenPR) { p.Review = "APPROVED" }), "api#88 was approved", false},
		{"checks pass", &open, with(func(p *SeenPR) { p.Checks = "success" }), "api#88's checks pass", false},
		{"a different PR on the branch", &open, SeenPR{Number: 90, Checks: "failure"}, "", false},
	}
	for _, c := range cases {
		text, act := PRChange("api#88", c.prev, c.now)
		if text != c.text || act != c.act {
			t.Errorf("%s: %q %v, want %q %v", c.name, text, act, c.text, c.act)
		}
	}
}

func TestStore_SeePR(t *testing.T) {
	s := owned(t)
	m, _ := s.Create("Cache tokens")
	s.Update(m.Slug, func(m *Mission) error { m.AddCapsule("api", "fix"); return nil })
	first := SeenPR{Number: 88, Checks: "pending"}
	if prev, changed, err := s.SeePR(m.Slug, "api/fix", first); err != nil || prev != nil || !changed {
		t.Fatalf("first: %v %v %v", prev, changed, err)
	}
	if _, changed, _ := s.SeePR(m.Slug, "api/fix", first); changed {
		t.Error("seeing the same again is a change")
	}
	red := SeenPR{Number: 88, Checks: "failure"}
	if prev, changed, _ := s.SeePR(m.Slug, "api/fix", red); !changed || prev == nil || *prev != first {
		t.Errorf("red: prev %v, changed %v", prev, changed)
	}
	if got, _ := s.Load(m.Slug); got.Capsules[0].SeenPR == nil || *got.Capsules[0].SeenPR != red {
		t.Errorf("recorded %+v", got.Capsules[0].SeenPR)
	}
}
