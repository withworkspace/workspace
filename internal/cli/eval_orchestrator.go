package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/withworkspace/workspace/internal/config"
	"github.com/withworkspace/workspace/internal/crew"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/orchestrator"
)

// ws eval orchestrator: the orchestrator run for real on a mission a
// fixture makes, in a workspace made for the run and gone after it. ws
// doesn't make that workspace: --before does, a shell command of whoever
// runs the eval (scripts/try.sh up, here), and --after removes it. What
// ws holds is what's between: the workspace is one it was just made, its
// own orchestrator is kept out of the seed's way, the crew this one
// dispatches are queued and never launched, and what it ran is read off
// its run's record and scored.

// What a hook is told, in its environment. The before hook is given a new,
// empty folder to make the workspace in (evalTmp), and says where it is
// by printing "dir <path>"; whatever else it prints as "<key> <value>"
// (name, ws) is handed on as WS_EVAL_<KEY>, to the seed and the after
// hook.
const (
	evalTmp     = "WS_EVAL_TMP"
	evalDir     = "WS_EVAL_DIR"
	evalFixture = "WS_EVAL_FIXTURE"
)

func newEvalOrchestratorCmd() *cobra.Command {
	var e orchestratorEval
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "orchestrator <fixture>...",
		Short: "Run the orchestrator on missions fixtures make, and score what it does",
		Long: `ws eval orchestrator runs the orchestrator, for real, on the mission each
fixture makes, and scores the ws mission commands it ran against those the
fixture says it's to run, and those it's never to try.

Each run has a workspace of its own, made for it and removed after. ws
doesn't make one: you say how, with two shell commands.

  --before   makes a workspace in the new, empty folder $WS_EVAL_TMP, and
             prints where: a line "dir <path>". Other "<key> <value>"
             lines it prints (name, ws) are handed on as $WS_EVAL_<KEY>.
  --after    removes it, and stops what --before started (a daemon). It
             has $WS_EVAL_DIR and the rest. ws removes $WS_EVAL_TMP after.

In this repo, where scripts/try.sh makes a throwaway workspace:

  WS_EVAL=1 ws eval orchestrator \
    --before 'TRY_ROOT=$WS_EVAL_TMP scripts/try.sh up --no-build' \
    --after 'TRY_ROOT=$WS_EVAL_TMP scripts/try.sh down "$WS_EVAL_NAME"' \
    --pick claude:haiku internal/ops/testdata/orchestrator/*

A fixture is a folder with a fixture.toml and the seed beside it:

  about = "why this case is kept"
  mission = "greet-by-name"     # the mission its seed makes
  trigger = "manual"            # as ws mission orchestrate takes it; crew = "c1" for whose
  expect = ["dispatch"]         # ws mission commands it's to run, and not be refused
  never = ["accept", "pr"]      # those it isn't to try at all

The seed (seed.sh, or what seed says) is a script: bash runs it in the new
workspace, with $TRY_DIR (the workspace) and $TRY_WS (its ws) as a try's
seed has them, so one from scripts/seeds will do. A fixture is run, not
only read: use those you'd run.

While the seed runs, the workspace's own orchestrator is off, so the
mission is as the seed left it. For the run, crew it dispatches are queued
for you, never launched. Other agents it starts (a planner, with ws mission
replan) are started, and go with the workspace.

Each command is scored right or wrong: one it's to run is right if it ran
and wasn't refused; one it's never to run is right if it didn't try. It
exits 1 unless every one is right. One run is one sample.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			if !evalOn() {
				return fmt.Errorf("ws eval is off: it starts agents of its own, so it runs only where %s is set (%s=1 ws eval orchestrator ...)", crew.EnvEval, crew.EnvEval)
			}
			if strings.TrimSpace(e.Before) == "" {
				return errors.New("--before is what makes the workspace each run is in: ws eval orchestrator --help says what it's given, and what it prints")
			}
			if e.Pick != "" {
				if _, _, err := config.ParsePick(e.Pick); err != nil {
					return err
				}
			}
			e.From = userDirOr()
			var fixtures []ops.OrchestratorFixture
			for _, a := range args {
				if !filepath.IsAbs(a) {
					a = filepath.Join(e.From, a)
				}
				f, err := ops.ReadOrchestratorFixture(a)
				if err != nil {
					return err
				}
				fixtures = append(fixtures, f)
			}
			ok := true
			for _, f := range fixtures {
				res := e.run(f)
				ok = ok && res.AllRight()
				if asJSON {
					if err := json.NewEncoder(os.Stdout).Encode(res); err != nil {
						return err
					}
					continue
				}
				writeEvalResult(os.Stdout, res)
			}
			if !ok {
				cmd.SilenceErrors = true
				return &ExitCodeError{Code: 1}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&e.Before, "before", "", "the shell command that makes a workspace for a run, and prints \"dir <path>\"")
	cmd.Flags().StringVar(&e.After, "after", "", "the shell command that removes it")
	cmd.Flags().StringVar(&e.Pick, "pick", "", "who orchestrates, as harness:model or a harness alone (default: the made workspace's orchestrator)")
	cmd.Flags().BoolVar(&e.Keep, "keep", false, "leave each workspace as the run left it (--after isn't run), and say where")
	cmd.Flags().BoolVar(&asJSON, "json", false, "a line of JSON for each fixture")
	return cmd
}

// orchestratorEval is how fixtures are put to the orchestrator.
type orchestratorEval struct {
	// Before and After are the shell commands that make a run's workspace
	// and remove it, run from From (where ws eval was run).
	Before, After, From string
	// Pick is who orchestrates, as [crew] allowed writes one. "": the
	// made workspace's orchestrator.
	Pick string
	// Keep leaves the workspace, and doesn't run After.
	Keep bool
}

// evalMade is told the workspace a run was made, once it's known to be
// one made for it. A test owns its records there, which its daemon would.
var evalMade = func(dir string) {}

// hookLine is a line a before hook says something with: "dir /tmp/x".
var hookLine = regexp.MustCompile(`^([a-z][a-z0-9_]*)\s+(\S.*)$`)

// run is one fixture's eval: a workspace made, the mission seeded, the
// orchestrator run, what it ran scored, and the workspace removed. What
// goes wrong on the way is the result's Error.
func (e orchestratorEval) run(f ops.OrchestratorFixture) (res ops.EvalResult) {
	res = ops.EvalResult{Fixture: f.Name, Pick: e.Pick}
	fail := func(format string, args ...any) ops.EvalResult {
		res.Error = fmt.Sprintf(format, args...)
		return res
	}
	tmp, err := os.MkdirTemp("", "ws-eval-")
	if err != nil {
		return fail("%v", err)
	}
	// macOS's temp folder is a link: what's made there says where it leads.
	if resolved, err := filepath.EvalSymlinks(tmp); err == nil {
		tmp = resolved
	}
	hookEnv := append(os.Environ(), evalTmp+"="+tmp, evalFixture+"="+f.Name)
	// Once the before hook has made a workspace, the after hook is run
	// however far this gets, and the folder goes last. Kept, neither.
	made, dir := false, ""
	defer func() {
		if e.Keep && dir != "" {
			res.Dir = dir
			return
		}
		if made && strings.TrimSpace(e.After) != "" {
			if out, err := e.hook(e.After, hookEnv); err != nil && res.Error == "" {
				res.Error = fmt.Sprintf("--after failed: %v: %s", err, clipTailLines(out, 6))
			}
		}
		os.RemoveAll(tmp)
	}()

	out, err := e.hook(e.Before, hookEnv)
	if err != nil {
		return fail("--before failed: %v: %s", err, clipTailLines(out, 6))
	}
	made = true
	said := map[string]string{}
	for line := range strings.SplitSeq(out, "\n") {
		if m := hookLine.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			said[m[1]] = strings.TrimSpace(m[2])
		}
	}
	// What it said is handed on, to the seed and the after hook.
	var handed []string
	for k, v := range said {
		handed = append(handed, "WS_EVAL_"+strings.ToUpper(k)+"="+v)
	}
	sort.Strings(handed)
	hookEnv = append(hookEnv, handed...)
	at := said["dir"]
	if at == "" {
		return fail("--before didn't say where the workspace is: it prints a line \"dir <path>\"")
	}
	if resolved, err := filepath.EvalSymlinks(at); err == nil {
		at = resolved
	}
	// The orchestrator acts for real: only in a workspace made just now,
	// for this. One that was there before could be anyone's.
	if rel, err := filepath.Rel(tmp, at); err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fail("--before says the workspace is %s, which isn't in the folder it was given to make it in (%s=%s): the orchestrator is only run in a workspace made for the run", at, evalTmp, tmp)
	}
	dir = at
	evalMade(dir)

	// ws's own orchestrator stays out while the mission's made: it's to
	// be as the seed left it. It stays off for the run too (a run by hand
	// isn't asked whether it's on), so nothing answers what this one does.
	off := false
	if err := config.UpdateLocal(dir, func(l *config.LocalConfig) { l.Orchestrator.Enabled = &off }); err != nil {
		return fail("%s isn't a workspace ws can configure: %v", dir, err)
	}
	wsPath := said["ws"]
	if wsPath == "" {
		if wsPath, err = writeEvalWS(tmp); err != nil {
			return fail("%v", err)
		}
	}
	seedEnv := append(crew.Env(dir, "", ""), handed...)
	seedEnv = append(seedEnv, "TRY_DIR="+dir, "TRY_WS="+wsPath, evalDir+"="+dir, evalTmp+"="+tmp, evalFixture+"="+f.Name)
	seed := exec.Command("bash", f.SeedPath()) //nolint:noctx // no context comes through an eval yet
	seed.Dir, seed.Env = dir, seedEnv
	if out, err := seed.CombinedOutput(); err != nil {
		return fail("its seed failed: %v: %s", err, clipTailLines(string(out), 8))
	}

	// For the run: on the pick, and what it dispatches queued for the
	// user, never launched. It's in the workspace's config, where the ws
	// commands the orchestrator runs read it too.
	err = config.UpdateLocal(dir, func(l *config.LocalConfig) {
		l.Orchestrator.Dispatch = "ask"
		if e.Pick != "" {
			l.Orchestrator.Harness, l.Orchestrator.Model, _ = config.ParsePick(e.Pick)
		}
	})
	if err != nil {
		return fail("configuring the run: %v", err)
	}
	ctx, err := LoadOps(dir)
	if err != nil {
		return fail("%s isn't a workspace: %v", dir, err)
	}
	if root, err := filepath.EvalSymlinks(ctx.WS().Root); err != nil || root != dir {
		return fail("%s is inside another workspace (%s): the run is only made in its own", dir, ctx.WS().Root)
	}
	if pick, _, err := ops.Roles(ctx.Config()).ForOrchestrator(); err == nil {
		res.Pick = pick.String()
	}
	store := missionStore(ctx)
	before := map[string]bool{}
	for _, r := range orchestrator.Runs(store, f.Mission) {
		before[r.ID] = true
	}
	var told bytes.Buffer
	runErr := ctx.Orchestrate(ops.OrchestratorRun{Slug: f.Mission, Crew: f.Crew, Trigger: f.Trigger, Said: func(line string) { fmt.Fprintln(&told, line) }})
	runs := orchestrator.Runs(store, f.Mission)
	if len(runs) <= len(before) {
		if runErr != nil {
			return fail("the orchestrator wasn't run: %v", runErr)
		}
		return fail("the orchestrator left no record of a run: %s", clipTailLines(told.String(), 4))
	}
	return f.OrchestratorResult(res.Pick, mergeRuns(runs, before))
}

// mergeRuns is an eval's runs as one, those not there before it: a run
// that ran no ws mission command is run again (orchestrate), and what the
// orchestrator did is what both did. The last says how it ended.
func mergeRuns(runs []orchestrator.RunRecord, before map[string]bool) *orchestrator.RunRecord {
	var mine []orchestrator.RunRecord
	for _, r := range runs {
		if !before[r.ID] {
			mine = append(mine, r)
		}
	}
	if len(mine) == 0 {
		return nil
	}
	sort.Slice(mine, func(i, j int) bool { return mine[i].Started.Before(mine[j].Started) })
	merged := mine[len(mine)-1]
	merged.Started, merged.Calls = mine[0].Started, nil
	for _, r := range mine {
		merged.Calls = append(merged.Calls, r.Calls...)
	}
	return &merged
}

// hook runs one of the eval's shell commands, from where ws eval was run,
// and returns what it printed: its stdout, or with an error, what it said
// on stderr too.
func (e orchestratorEval) hook(command string, env []string) (string, error) {
	cmd := exec.Command("sh", "-c", command) //nolint:noctx // no context comes through an eval yet
	cmd.Dir, cmd.Env = e.From, env
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return strings.TrimSpace(stdout.String() + "\n" + stderr.String()), err
	}
	return stdout.String(), nil
}

// writeEvalWS writes the ws a seed is given when the before hook names
// none: this ws, taking leading NAME=value arguments as a try's does
// ("$TRY_WS" WS_MISSION=x lift ...). Who the seed is (no mission, no
// crew, no agent's session) is its environment's to say.
func writeEvalWS(dir string) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, "ws")
	script := "#!/bin/sh\n# ws, for an eval's seed: leading NAME=value arguments are set for the command.\n" +
		"while [ $# -gt 0 ]; do\n\tcase \"$1\" in\n\t[A-Za-z_]*=*) export \"$1\" && shift ;;\n\t*) break ;;\n\tesac\ndone\n" +
		"exec " + shellQuoteArg(exe) + " \"$@\"\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		return "", err
	}
	return path, nil
}

func shellQuoteArg(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// clipTailLines is the last n lines of s, on one line: what a command
// said as it failed.
func clipTailLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, " / ")
}
