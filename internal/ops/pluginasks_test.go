package ops

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/withworkspace/plugin/protocol"
	"github.com/withworkspace/workspace/internal/acp"
)

// ask is AnswerPlugin, its answer as a plugin reads it: as JSON, into the
// protocol's types.
func ask[T any](t *testing.T, w *Workspace, method string, params any) (T, error) {
	t.Helper()
	var out T
	data, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	res, err := w.AnswerPlugin(context.Background(), "jira", method, data)
	if err != nil {
		return out, err
	}
	data, err = json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	return out, nil
}

func rpcCode(err error) int {
	var e *acp.RPCError
	if errors.As(err, &e) {
		return e.Code
	}
	return 0
}

func TestPluginAsksForMissions(t *testing.T) {
	w, _ := fixture(t, "## Goal\nCache it.\n", "cache-a")
	m, err := w.Missions.Create("Old")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Archive(m.Slug, false, AsYou); err != nil {
		t.Fatal(err)
	}

	got, err := ask[protocol.MissionListResult](t, w, protocol.MethodMissionList, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Missions) != 1 {
		t.Fatalf("in play: %+v", got.Missions)
	}
	c := got.Missions[0]
	if c.Slug != "cache" || c.Title != "Cache" || c.Path != "/missions/cache" || c.Created.IsZero() || c.Updated.IsZero() {
		t.Errorf("mission = %+v", c)
	}
	if c.State == "" || c.Headline == nil || c.Headline.Court == "" || c.Headline.Text == "" {
		t.Errorf("where it stands: state %q, headline %+v", c.State, c.Headline)
	}
	if len(c.Capsules) != 1 || c.Capsules[0].Repo != "frontend" || c.Capsules[0].Name != "cache-a" || c.Capsules[0].Path == "" || c.Capsules[0].PullRequest != nil {
		t.Errorf("capsules = %+v", c.Capsules)
	}

	all, err := ask[protocol.MissionListResult](t, w, protocol.MethodMissionList, protocol.MissionListParams{Archived: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(all.Missions) != 2 || all.Missions[1].Slug != m.Slug || all.Missions[1].State != protocol.MissionArchived || all.Missions[1].Headline != nil {
		t.Errorf("archived too: %+v", all.Missions)
	}
}

func TestPluginAsksForAMission(t *testing.T) {
	w, _ := fixture(t, "## Goal\nCache it.\n", "cache-a")
	d, err := ask[protocol.MissionDetail](t, w, protocol.MethodMissionShow, protocol.MissionShowParams{Slug: "cache"})
	if err != nil {
		t.Fatal(err)
	}
	if d.Slug != "cache" || d.Brief != "## Goal\nCache it.\n" || len(d.Capsules) != 1 || d.Headline == nil {
		t.Errorf("mission = %+v", d)
	}

	_, err = ask[protocol.MissionDetail](t, w, protocol.MethodMissionShow, protocol.MissionShowParams{Slug: "nope"})
	if rpcCode(err) != protocol.CodeNotFound {
		t.Errorf("no such mission: %v", err)
	}
	_, err = ask[protocol.MissionDetail](t, w, protocol.MethodMissionShow, nil)
	if rpcCode(err) != protocol.CodeInvalidParams {
		t.Errorf("no slug: %v", err)
	}
}

func TestPluginAsksForRepos(t *testing.T) {
	w, _ := fixture(t, "", "cache-a")
	ground := filepath.Join(w.WS().RepoDir("frontend"), ".ground")
	for _, dir := range []string{ground, filepath.Join(w.WS().RepoDir("frontend"), "loose")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	got, err := ask[protocol.RepoListResult](t, w, protocol.MethodRepoList, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Repos) != 1 {
		t.Fatalf("repos = %+v", got.Repos)
	}
	r := got.Repos[0]
	if r.Name != "frontend" || r.Title != "frontend" || !r.Cloned || r.Path != w.WS().RepoDir("frontend") {
		t.Errorf("repo = %+v", r)
	}
	byName := map[string]protocol.Capsule{}
	for _, c := range r.Capsules {
		byName[c.Name] = c
	}
	if len(r.Capsules) != 3 || r.Capsules[0].Name != ".ground" || !r.Capsules[0].Ground || r.Capsules[0].Path != ground {
		t.Errorf("capsules = %+v", r.Capsules)
	}
	if byName["cache-a"].Mission != "cache" || byName["loose"].Mission != "" {
		t.Errorf("missions of capsules = %+v", r.Capsules)
	}
}

func TestPluginAsksWhatWSHasNot(t *testing.T) {
	w, _ := fixture(t, "")
	if _, err := w.AnswerPlugin(context.Background(), "jira", "link/put", nil); rpcCode(err) != acp.CodeMethodNotFound {
		t.Errorf("a method ws hasn't: %v", err)
	}
}
