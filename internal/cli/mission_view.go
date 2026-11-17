package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/withworkspace/workspace/internal/github"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/workspace"
)

func missionStore(ctx *ops.Workspace) mission.Store {
	return mission.Store{Root: ctx.WS().Root}
}

// resolveMission picks the mission a command applies to: an explicit slug,
// then $WS_MISSION, then the mission owning the capsule the user stands in.
func resolveMission(ctx *ops.Workspace, slug string) (*mission.Mission, error) {
	store := missionStore(ctx)
	if slug == "" {
		slug = os.Getenv(mission.EnvMission)
	}
	if slug != "" {
		m, err := store.Load(slug)
		if errors.Is(err, mission.ErrNotFound) {
			return nil, fmt.Errorf("no mission %q (see ws mission list)", slug)
		}
		return m, err
	}
	cwd, _ := userDir()
	if repo, capsule, ok := workspace.DetectRepo(ctx.WS().Root, cwd); ok {
		if m, ok := store.FindByCapsule(repo, capsule); ok {
			return m, nil
		}
	}
	return nil, errors.New("no mission given: pass a mission slug, set " + mission.EnvMission + ", or run from a mission's capsule")
}

// envMission returns the mission named by $WS_MISSION, or nil when it is
// unset. A set-but-missing mission is reported as an error so callers can warn.
func envMission(ctx *ops.Workspace) (*mission.Mission, error) {
	slug := os.Getenv(mission.EnvMission)
	if slug == "" {
		return nil, nil
	}
	m, err := missionStore(ctx).Load(slug)
	if errors.Is(err, mission.ErrNotFound) {
		return nil, fmt.Errorf("%s=%s does not exist", mission.EnvMission, slug)
	}
	return m, err
}

// missionLLMHeader is printed above `ws status -f llm` output when crew run it
// with $WS_MISSION set. It is empty otherwise, leaving status unchanged.
func missionLLMHeader(ws *workspace.Workspace, gh github.Client) string {
	slug := os.Getenv(mission.EnvMission)
	if slug == "" {
		return ""
	}
	m, err := mission.Store{Root: ws.Root}.Load(slug)
	if err != nil {
		return fmt.Sprintf("Mission: %s (not found — %s points at a mission that doesn't exist)\n\n", slug, mission.EnvMission)
	}
	v := ops.BuildMissionViews(ws, gh, []*mission.Mission{m})[0]
	var b strings.Builder
	fmt.Fprintf(&b, "Mission: %s — %s [%s]\n", m.Slug, m.Title, v.State)
	if len(v.Capsules) == 0 {
		b.WriteString("Mission capsules: none yet\n")
	} else {
		names := make([]string, len(v.Capsules))
		for i, c := range v.Capsules {
			names[i] = c.Repo + "/" + c.Name
			if !c.Exists {
				names[i] += " (burned)"
			}
		}
		fmt.Fprintf(&b, "Mission capsules: %s\n", strings.Join(names, ", "))
	}
	b.WriteString("You are crew on this mission. Run `ws show -f llm` for the brief, and `ws show -f llm <ref>` for anything in it.\n\n")
	return b.String()
}
