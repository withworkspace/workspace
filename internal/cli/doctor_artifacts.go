package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/withworkspace/workspace/internal/artifact"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/workspace"
)

// checkArtifacts is ws doctor's word on where a promoted artifact lives
// ([artifacts] in ws.toml, and its path in ws.local.toml): that its path
// can be used, the repo it names is cloned and lands directly (a pull
// request would bury a promoted sketch in review), its guide is there,
// and, where there's a records repo but no [artifacts], that promoted
// ones stay on this machine, and what to add. It brings onto this
// machine any that were kept in a repo whole, from before they all lived
// here, and names what's left of those there.
func checkArtifacts(ctx *ops.Workspace) workspace.CheckCategory {
	cat := workspace.CheckCategory{Name: "Artifacts"}
	said := ctx.Config().Artifacts
	c := ctx.ArtifactsConfig()
	add := func(name string, st workspace.CheckStatus, detail, hint string) {
		cat.Checks = append(cat.Checks, workspace.CheckResult{Name: name, Status: st, Detail: detail, FixHint: hint})
	}
	for _, p := range c.Problems {
		add("[artifacts]", workspace.CheckFail, p, "path = \"<repo>:artifacts/{date}-{slug}/\" is where a promoted one lives in a repo; none, or \".ws\", and it stays on this machine")
	}
	records := ctx.WS().Records
	switch {
	case c.Repo == "" && said.Path == "" && records != "":
		add("where a promoted one lives", workspace.CheckWarn,
			fmt.Sprintf("on this machine only (.ws/artifacts): there's a records repo, %s, and no [artifacts] to say they live there. For one you promote to be put there, for everyone with it, add [artifacts] path = \"%s:artifacts/{date}-{slug}/\" to ws.toml", records, records),
			fmt.Sprintf("for a promoted one to live in %s, for everyone with it, add to ws.toml:\n[artifacts]\npath = \"%s:artifacts/{date}-{slug}/\"", records, records))
	case c.Repo == "" && len(c.Problems) == 0:
		add("where a promoted one lives", workspace.CheckOK, "on this machine (.ws/artifacts), with every other", "")
	case c.Repo != "":
		repo := c.Repo
		ground := ctx.WS().MainWorktree(repo)
		where := fmt.Sprintf("in %s (%s)", repo, c.Path)
		if c.Local {
			where += "; yours stay on this machine, as ws.local.toml says (path = \".ws\"), unless you name a home for one"
		}
		if _, err := os.Stat(ground); err != nil {
			add("where a promoted one lives", workspace.CheckFail, fmt.Sprintf("%s, which isn't cloned here: ws can put none there", where), "ws clone "+repo)
			break
		}
		if !ctx.WS().Direct(repo) {
			add("where a promoted one lives", workspace.CheckFail,
				fmt.Sprintf("%s, which takes pull requests: a promoted one would wait in review, so ws puts none there", where),
				fmt.Sprintf("name a repo with no pull requests (landing = \"direct\" under [repos.%s]), or keep them on this machine (path = \".ws\")", repo))
			break
		}
		add("where a promoted one lives", workspace.CheckOK, where+": the version chosen, in one commit landed on its default branch each time it's promoted. The rest of its versions, and every artifact that isn't promoted, are on this machine", "")
	}
	// Those from before they all lived here, brought now; and what's left
	// in the repo of the ones that weren't promoted, which is yours to
	// remove.
	if brought, err := ctx.BringArtifactsHome(); err != nil {
		add("from a repo", workspace.CheckWarn, fmt.Sprintf("couldn't bring onto this machine the artifacts that were kept in a repo: %v", err), "")
	} else if len(brought) > 0 {
		var ids string
		for i, e := range brought {
			if i > 0 {
				ids += ", "
			}
			ids += e.ID
		}
		add("from a repo", workspace.CheckOK, fmt.Sprintf("brought %s onto this machine: artifacts live here now, and a promoted one in a repo too", ids), "")
	}
	if c.Repo != "" {
		if left := oldArtifactFolders(ctx, c); len(left) > 0 {
			add("left in "+c.Repo, workspace.CheckWarn,
				fmt.Sprintf("%d %s in %s hold every version of an artifact (v1.html, …), from when artifacts were kept there whole: %s. ws reads none of them now",
					len(left), map[bool]string{true: "folder", false: "folders"}[len(left) == 1], c.Repo, joinSome(left, 4)),
				"remove each from "+c.Repo+", or, for one worth keeping, promote the artifact (ws artifact promote), which puts the version chosen there in its place")
		}
	}
	if list, err := ctx.Artifacts().List(context.Background()); err == nil {
		now, expired := time.Now(), 0
		for _, e := range list {
			if c.Expired(e, now) {
				expired++
			}
		}
		switch {
		case expired > 0:
			add("expiry", workspace.CheckOK, fmt.Sprintf("%d expired, after %s with no new version: the daemon discards them within the day (ws artifact clear does now, and ws artifact promote keeps one)", expired, c.ExpiryWords()), "")
		case c.ExpireAfter <= 0:
			add("expiry", workspace.CheckOK, "a live one never expires (expire_after = \"never\")", "")
		default:
			add("expiry", workspace.CheckOK, fmt.Sprintf("a live one expires %s after its last version", c.ExpiryWords()), "")
		}
	}
	if c.Guide != "" {
		if _, err := os.Stat(ctx.ArtifactGuidePath(c.Guide)); err != nil {
			add("its guide", workspace.CheckWarn, fmt.Sprintf("%s isn't there: a conversation's agent proposing one is shown ws's own", c.Guide), "")
		} else {
			add("its guide", workspace.CheckOK, c.Guide, "")
		}
	}
	return cat
}

// oldArtifactFolders are the folders in the repo [artifacts] names, on
// its ground, that hold an artifact whole, as they did before artifacts
// lived on this machine: a record beside numbered versions (v1.html). A
// promoted one's home has one file, index.html, and isn't one of them.
func oldArtifactFolders(ctx *ops.Workspace, c artifact.Config) []string {
	parent := filepath.Join(ctx.WS().MainWorktree(c.Repo), filepath.FromSlash(c.Dir))
	entries, err := os.ReadDir(parent)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(parent, e.Name())
		if _, err := os.Stat(filepath.Join(dir, artifact.RecordFile)); err != nil {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, artifact.VersionFile(1))); err == nil {
			out = append(out, e.Name())
		}
	}
	return out
}

// joinSome lists the first few of some names, and says how many more.
func joinSome(names []string, most int) string {
	s := ""
	for i, n := range names {
		if i == most {
			return fmt.Sprintf("%s and %d more", s, len(names)-most)
		}
		if i > 0 {
			s += ", "
		}
		s += n
	}
	return s
}
