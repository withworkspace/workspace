package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/workspace"
)

// checkArtifacts is ws doctor's word on where artifacts are kept
// ([artifacts] in ws.toml, and its path in ws.local.toml): that its path
// can be used, the repo it names is cloned and lands directly (a pull
// request a version would bury a sketch in review), its guide is there,
// and, where there's a records repo but no [artifacts], that they're on
// this machine only, and what to add.
func checkArtifacts(ctx *ops.Workspace) workspace.CheckCategory {
	cat := workspace.CheckCategory{Name: "Artifacts"}
	said := ctx.Config().Artifacts
	c := ctx.ArtifactsConfig()
	add := func(name string, st workspace.CheckStatus, detail, hint string) {
		cat.Checks = append(cat.Checks, workspace.CheckResult{Name: name, Status: st, Detail: detail, FixHint: hint})
	}
	for _, p := range c.Problems {
		add("[artifacts]", workspace.CheckFail, p, "path = \"<repo>:artifacts/{date}-{slug}/\" keeps them in a repo; none, or \".ws\", on this machine")
	}
	records := ctx.WS().Records
	switch {
	case c.Repo == "" && said.Path == "" && records != "":
		add("where they're kept", workspace.CheckWarn,
			fmt.Sprintf("on this machine only (.ws/artifacts): there's a records repo, %s, and no [artifacts] to keep them there. To keep them there, for everyone with it, add [artifacts] path = \"%s:artifacts/{date}-{slug}/\" to ws.toml", records, records),
			fmt.Sprintf("to keep them in %s, for everyone with it, add to ws.toml:\n[artifacts]\npath = \"%s:artifacts/{date}-{slug}/\"", records, records))
	case c.Repo == "" && len(c.Problems) == 0:
		add("where they're kept", workspace.CheckOK, "on this machine (.ws/artifacts)", "")
	case c.Repo != "":
		repo := c.Repo
		ground := ctx.WS().MainWorktree(repo)
		where := fmt.Sprintf("in %s (%s)", repo, c.Path)
		if c.Local {
			where += "; yours go on this machine, as ws.local.toml says (path = \".ws\"), and ws reads both"
		}
		if _, err := os.Stat(ground); err != nil {
			add("where they're kept", workspace.CheckFail, fmt.Sprintf("%s, which isn't cloned here: ws reads none of them, and adds none there", where), "ws clone "+repo)
			break
		}
		if !ctx.WS().Direct(repo) {
			add("where they're kept", workspace.CheckFail,
				fmt.Sprintf("%s, which takes pull requests: each version would wait in review, so ws adds none there", where),
				fmt.Sprintf("keep them in a repo with no pull requests (landing = \"direct\" under [repos.%s]), or on this machine (path = \".ws\")", repo))
			break
		}
		add("where they're kept", workspace.CheckOK, where+": each one you add is a commit, landed on its default branch; crew's land with their capsule", "")
	}
	// How long a live one lasts, and the expired in a repo, which wait for
	// you: ws commits nothing to a shared branch on its own.
	if list, err := ctx.Artifacts().List(context.Background()); err == nil {
		now, expired := time.Now(), 0
		var where string
		for _, e := range list {
			if e.InRepo() && c.Expired(e, now) {
				expired++
				where = e.Place
			}
		}
		if expired > 0 {
			add("expiry", workspace.CheckWarn, fmt.Sprintf("%d expired in %s, after %s with no new version: they're listed, dimmed, until you clear them", expired, where, c.ExpiryWords()), "ws artifact clear (or Clear expired, on Artifacts in the web UI); ws artifact promote keeps one")
		} else if c.ExpireAfter <= 0 {
			add("expiry", workspace.CheckOK, "a live one never expires (expire_after = \"never\")", "")
		} else {
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
