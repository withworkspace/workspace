package ops

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/github"
)

// opened is a GitHub that opens pull requests, numbered from 7, and keeps
// what it was asked to open.
type opened struct {
	asked []string // owner/name head→base: title
}

func (o *opened) stub() *github.Stub {
	return &github.Stub{CreatePRFn: func(owner, repo string, n github.NewPR) (*github.PR, error) {
		o.asked = append(o.asked, owner+"/"+repo+" "+n.Head+"→"+n.Base+": "+n.Title+"\n"+n.Body)
		num := 6 + len(o.asked)
		return &github.PR{Number: num, Title: n.Title, HeadRefName: n.Head, URL: "https://github.com/" + owner + "/" + repo + "/pull/" + strconv.Itoa(num)}, nil
	}}
}

func configGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
	return strings.TrimSpace(string(out))
}

// A repo's ws.repo.toml is changed by a pull request, from a capsule
// lifted for it, with the file as ws checked it: the ground stays as it
// was.
func TestEditConfig_RepoFileIsAPullRequest(t *testing.T) {
	w := liftWorld(t)
	gh := &opened{}
	w.opts.GitHub = gh.stub()
	root := w.Root()

	f, err := w.ConfigToEdit("api")
	if err != nil {
		t.Fatal(err)
	}
	if f.Ref != "api:ws.repo.toml" || f.Version != NewFileVersion || f.Text != "" || f.Lands != conversation.LandsPR {
		t.Fatalf("ConfigToEdit(api) = %+v", f)
	}
	if err := CheckConfig(f, "[run.task.test]\ncomand = \"go test ./...\"\n"); err == nil || !strings.Contains(err.Error(), "run.task.test.comand") {
		t.Errorf("a misspelt key: %v", err)
	}
	if err := CheckConfig(f, "[run.task.test\n"); err == nil {
		t.Error("TOML that doesn't parse should be refused")
	}

	text := "[run.task.test]\ncommand = \"go test ./...\"\ndescription = \"Unit tests\"\n"
	made, err := w.EditConfig("api:ws.repo.toml", "", text, "Run api's tests with ws run", "Crew and checks run the tests by name.")
	if err != nil {
		t.Fatal(err)
	}
	if made.PR == nil || made.PR.Number != 7 || made.Branch != "config-run-api-s-tests-with-ws-run" {
		t.Fatalf("made %+v", made)
	}
	if len(gh.asked) != 1 || !strings.HasPrefix(gh.asked[0], "/api config-run-api-s-tests-with-ws-run→main: Run api's tests with ws run\nCrew and checks") {
		t.Errorf("asked GitHub for %q", gh.asked)
	}
	origin := filepath.Join(root, "origin.git")
	if got := configGit(t, origin, "show", made.Branch+":ws.repo.toml"); got != strings.TrimSpace(text) {
		t.Errorf("pushed ws.repo.toml:\n%s", got)
	}
	if got := configGit(t, origin, "log", "-1", "--format=%s", made.Branch); got != "Run api's tests with ws run" {
		t.Errorf("commit: %q", got)
	}
	if _, err := os.Stat(filepath.Join(root, "repos/api/.ground/ws.repo.toml")); err == nil {
		t.Error("the ground was changed in place")
	}
	if _, err := os.Stat(filepath.Join(root, "repos/api", strings.TrimPrefix(made.Capsule, "api/"), "ws.repo.toml")); err != nil {
		t.Errorf("the capsule %s should hold the change: %v", made.Capsule, err)
	}

	// The same again is a branch of its own.
	again, err := w.EditConfig("api:ws.repo.toml", "", text+"\n[capsule]\nafter_create = \"go mod download\"\n", "Run api's tests with ws run", "")
	if err != nil {
		t.Fatal(err)
	}
	if again.Branch == made.Branch {
		t.Errorf("a second change reused %s", made.Branch)
	}
}

// A repo with no pull requests has the change committed in a capsule, for
// you to land: nothing's pushed.
func TestEditConfig_DirectRepoLeftInACapsule(t *testing.T) {
	w := liftWorld(t)
	gh := &opened{}
	w.opts.GitHub = gh.stub()
	w.WS().Landings = map[string]string{"api": "direct"}
	made, err := w.EditConfig("api:ws.repo.toml", "", "landing = \"direct\"\n", "Say api lands direct", "")
	if err != nil {
		t.Fatal(err)
	}
	if made.PR != nil || made.Capsule == "" || len(gh.asked) != 0 {
		t.Errorf("made %+v, asked %v", made, gh.asked)
	}
	if heads := configGit(t, filepath.Join(w.Root(), "origin.git"), "branch", "--list"); strings.Contains(heads, "config-") {
		t.Errorf("pushed to the remote: %s", heads)
	}
	if !strings.Contains(made.Message, "for you to land") {
		t.Errorf("says %q", made.Message)
	}
}

func TestConfigToEdit_Refuses(t *testing.T) {
	w := liftWorld(t)
	_, err := w.ConfigToEdit("ws.local.toml")
	refusedWith(t, err, "yours")
	if _, err := w.ConfigToEdit("api:package.json"); err == nil {
		t.Error("a file that isn't ws's config")
	}
	if _, err := w.ConfigToEdit("nope:ws.repo.toml"); err == nil {
		t.Error("a repo that isn't there")
	}
	// The root here isn't a git repo: no pull request for ws.toml.
	_, err = w.ConfigToEdit("ws.toml")
	refusedWith(t, err, "no-config-repo")
}

// ws.toml is changed by a pull request on the workspace's config repo,
// from a clone of its own: the root, its git put away, stays as it was.
func TestEditConfig_TeamFileIsAPullRequestOnTheConfigRepo(t *testing.T) {
	w, root, remote := sharedWorld(t)
	gh := &opened{}
	w.opts.GitHub = gh.stub()
	gitDir := filepath.Join(root, ".git-disabled")
	_, err := w.ConfigToEdit("ws.toml")
	refusedWith(t, err, "config-not-github") // its remote's a folder

	// On GitHub, as far as ws can tell: git fetches it from the folder.
	shareGit(t, root, "--git-dir="+gitDir, "remote", "set-url", "origin", "https://github.com/acme/config.git")
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "url."+remote+".insteadOf")
	t.Setenv("GIT_CONFIG_VALUE_0", "https://github.com/acme/config.git")

	f, err := w.ConfigToEdit("ws.toml")
	if err != nil {
		t.Fatal(err)
	}
	if f.Where != "a pull request on acme/config" {
		t.Errorf("where: %q", f.Where)
	}
	text := f.Text + "\n[missions]\nrecords = \"api\"\n"
	if err := CheckConfig(f, f.Text+"\n[boarded]\napi = [\".ground\"]\n"); err == nil {
		t.Error("ws.local.toml's [boarded] in ws.toml should be refused")
	}
	made, err := w.EditConfig("ws.toml", f.Text, text, "Keep plans in api", "Plans and decisions go in api.")
	if err != nil {
		t.Fatal(err)
	}
	if made.PR == nil || len(gh.asked) != 1 || !strings.HasPrefix(gh.asked[0], "acme/config config-keep-plans-in-api→main: Keep plans in api") {
		t.Fatalf("made %+v, asked %q", made, gh.asked)
	}
	if got := shareGit(t, remote, "show", made.Branch+":ws.toml"); !strings.Contains(got, "[missions]\nrecords = \"api\"") {
		t.Errorf("pushed ws.toml:\n%s", got)
	}
	if got := shareGit(t, remote, "log", "-1", "--format=%an %s", made.Branch); got != "Ana Keep plans in api" {
		t.Errorf("commit: %q", got)
	}
	if now, _ := os.ReadFile(filepath.Join(root, config.FileName)); string(now) != f.Text {
		t.Errorf("the root's ws.toml changed:\n%s", now)
	}
	if _, err := os.Stat(filepath.Join(root, ".git")); err == nil {
		t.Error("the root's git should stay put away")
	}
	if !strings.Contains(made.Message, "ws sync brings it here") {
		t.Errorf("says %q", made.Message)
	}
}

// What changed in a file since a change was made from it is merged in;
// the same lines changed both ways make nothing.
func TestMergedConfig(t *testing.T) {
	was := "[workspace]\norg = \"acme\"\n\n[repos.api]\n\n[repos.web]\n"
	text := "[workspace]\norg = \"acme\"\n\n[repos.api]\naliases = [\"a\"]\n\n[repos.web]\n"
	got, err := mergedConfig("ws.toml", was, text, []byte(was))
	if err != nil || string(got) != text {
		t.Errorf("unchanged since: %q, %v", got, err)
	}
	now := was + "\n[repos.docs]\n"
	got, err = mergedConfig("ws.toml", was, text, []byte(now))
	if err != nil || !strings.Contains(string(got), "aliases = [\"a\"]") || !strings.Contains(string(got), "[repos.docs]") {
		t.Errorf("merged: %q, %v", got, err)
	}
	clash := strings.Replace(was, "[repos.api]\n", "[repos.api]\naliases = [\"b\"]\n", 1)
	_, err = mergedConfig("ws.toml", was, text, []byte(clash))
	refusedWith(t, err, "changed")
}

// Proposed in a conversation and accepted, a change to a repo's
// ws.repo.toml is a pull request, as you edited it; the agent's told
// where it went.
func TestAcceptConfigProposal(t *testing.T) {
	w := liftWorld(t)
	gh := &opened{}
	w.opts.GitHub = gh.stub()
	store := conversation.ForWorkspace(w.Root())
	c, err := store.OpenTalk(conversation.You, "What should api's ws.repo.toml say?", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	f, _ := w.ConfigToEdit("api")
	p := conversation.Proposal{Kind: conversation.ProposeConfig, By: conversation.Agent, Path: f.Ref, Was: f.Text, Lands: f.Lands, Where: f.Where,
		Text: "[run.task.test]\ncommand = \"go test ./...\"\n", Title: "Run api's tests", Why: "Checks run them by name."}
	if _, err := store.Propose(c.ID, conversation.Proposal{Kind: conversation.ProposeConfig, By: conversation.Agent, Path: f.Ref, Text: "x = 1", Title: ""}); err == nil {
		t.Error("a change with no title should be refused")
	}
	if _, err := store.Propose(c.ID, p); err != nil {
		t.Fatal(err)
	}
	// Proposed again, the same file: it takes the first's place.
	p.Text += "description = \"Unit tests\"\n"
	if _, err := store.Propose(c.ID, p); err != nil {
		t.Fatal(err)
	}
	if c, _ = store.Load(c.ID); c.Proposal(1).State != conversation.ProposalSuperseded {
		t.Errorf("the first: %s", c.Proposal(1).State)
	}
	edited := p.Text + "timeout = \"5m\"\n"
	said, err := w.SettleProposalIn(store, c.ID, 2, true, ProposalEdits{Text: edited})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(said, "Opened api#7") {
		t.Errorf("said %q", said)
	}
	c, _ = store.Load(c.ID)
	got := c.Proposal(2)
	if got.State != conversation.ProposalAccepted || !strings.HasSuffix(got.Result, "/pull/7") {
		t.Errorf("settled %s, %q", got.State, got.Result)
	}
	if len(gh.asked) != 1 || !strings.Contains(gh.asked[0], "Checks run them by name.\n\nProposed in conversation "+c.ID) {
		t.Errorf("the pull request: %q", gh.asked)
	}
	branch := "config-run-api-s-tests"
	if pushed := configGit(t, filepath.Join(w.Root(), "origin.git"), "show", branch+":ws.repo.toml"); !strings.Contains(pushed, "timeout = \"5m\"") {
		t.Errorf("pushed, as you edited it:\n%s", pushed)
	}
	last := c.Turns[len(c.Turns)-1]
	if !strings.Contains(last.Body, "Accepted proposal 2, the change to api:ws.repo.toml “Run api's tests”: opened as a pull request") || !strings.Contains(last.Body, "Edited before accepting") {
		t.Errorf("the agent's told:\n%s", last.Body)
	}
}
