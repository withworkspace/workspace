package events

import "testing"

// A Match is read as written, refused where it can't be, and matches the
// events it says: by kind and each field, a pattern or a person included.
func TestMatch(t *testing.T) {
	for _, bad := range []string{"", "pr.mergd pr=api#1", "pr.merged pr", "pr.merged colour=red", "pr.merged pr=["} {
		if _, err := ParseMatch(bad); err == nil {
			t.Errorf("%q was read", bad)
		}
	}
	merged := Event{Kind: PRMerged, Origin: Heard, By: ByPerson("alice-wu"), Repo: "api", PR: "api#212", Title: "Idempotency"}
	issue := Event{Kind: ThingChanged, Origin: Heard, By: ByPlugin("jira"), Thing: "RAM-881", ThingKind: "jira:issue",
		Changes: []FieldChange{{Field: "status", From: "In Review", To: "Done"}}}
	landed := Event{Kind: CommitsLanded, Origin: Heard, By: ByYou, Repo: "records", Paths: []string{"decisions/queue.md", "sketches/q/thing.toml"}}
	ran := Event{Kind: RunEnded, Repo: "api", Run: "deploy.ground.api", Title: "deploy", Status: "failed (exit 1)"}
	for _, c := range []struct {
		match string
		e     Event
		want  bool
	}{
		{"pr.merged pr=api#212", merged, true},
		{"pr.merged pr=api#213", merged, false},
		{"pr.merged repo=api by=alice", merged, true},
		{"pr.merged by=alice-wu origin=heard", merged, true},
		{"pr.opened pr=api#212", merged, false},
		{"thing.changed thing=RAM-881 status=Done", issue, true},
		{"thing.changed thing=RAM-881 status=done", issue, true},
		{"thing.changed kind=jira:issue status=In*", issue, false},
		{"commits.landed repo=records path=decisions/*", landed, true},
		{"commits.landed by=me path=docs/*", landed, false},
		{"run.ended run=deploy status=failed*", ran, true},
		{"run.ended run=deploy status=passed", ran, false},
	} {
		m, err := ParseMatch(c.match)
		if err != nil {
			t.Errorf("%q: %v", c.match, err)
			continue
		}
		if got := m.Matches(c.e); got != c.want {
			t.Errorf("%q matches %s: %v, want %v", c.match, c.e.Kind, got, c.want)
		}
		if m.String() != c.match {
			t.Errorf("%q written back as %q", c.match, m.String())
		}
	}
}
