import { useState } from "react";
import { type CapsuleDetail, type RunDetail, runRecipe, stopRun } from "../../api";
import type { Reload } from "../../load";
import { Button } from "../../ui/Button";
import { Icon } from "../../ui/Icon";
import { Spinner } from "../../ui/Spinner";
import { humanError } from "../../ui/strings";
import { toast } from "../../ui/toast";
import { MissionWorkHeading } from "./MissionWorkHeading";
import { MissionWorkRecipes } from "./MissionWorkRecipes";
import { MissionWorkRuns } from "./MissionWorkRuns";
import { WorkRunLine } from "./WorkRunLine";

// A capsule's runs (ws run): what's running in it, each with its link and
// when it ends, and its repo's recipes not running, each to run. One run
// from here lasts while someone has the mission open, and 15 minutes
// after; crew's last until their step passes. Nothing shows for a capsule
// with neither.
//
// A task (tests, lint) has no link: it shows what it came to, at which
// commit, and goes to its run on the Runs page, where all it printed and
// its reports are. One run from here is the mission's, and runs to its end.
export function WorkRuns({ slug, c, open, reload }: { slug: string; c: CapsuleDetail; open: boolean; reload: Reload }) {
  const [busy, setBusy] = useState("");
  const runs = c.runs ?? [];
  const running = new Set(runs.filter((r) => r.state === "running").map((r) => r.recipe ?? r.name));
  const recipes = open && c.exists ? (c.recipes ?? []).filter((n) => !running.has(n)) : [];
  const tasks = new Set(c.tasks ?? []);
  // A task's end reaches the page by its mission's topic (its stamp has
  // its capsules' runs): nothing asks after it.
  if (!runs.length && !recipes.length) return null;
  const start = (recipe: string) => {
    setBusy(recipe);
    runRecipe(slug, c.repo, c.name, recipe)
      .then((r) => {
        if (r.type !== "task") toast(r.up ? `${r.name} is up on ${r.port}` : `${r.name} is running on ${r.port}, not up yet`, { tone: "done" });
        return reload();
      })
      .catch((e: Error) => toast(humanError(e.message), { tone: "danger" }))
      .finally(() => setBusy(""));
  };
  const stop = (r: RunDetail) => {
    setBusy(r.ref);
    stopRun(slug, r.ref)
      .then(() => reload())
      .catch((e: Error) => toast(humanError(e.message), { tone: "danger" }))
      .finally(() => setBusy(""));
  };
  return (
    <>
      <MissionWorkHeading fills count={runs.length > 0 ? runs.length : undefined}>Runs</MissionWorkHeading>
      {runs.length > 0 && (
        <MissionWorkRuns>
          {runs.map((r) => (
            <WorkRunLine key={r.ref} r={r} busy={busy === r.ref} stop={() => stop(r)} />
          ))}
        </MissionWorkRuns>
      )}
      {recipes.length > 0 && (
        <MissionWorkRecipes>
          {recipes.map((n) => (
            <Button
              size="small"
              key={n}
              disabled={!!busy}
              onClick={() => start(n)}
              title={tasks.has(n) ? `Run its ${n} task, from its ws.repo.toml: what it comes to is kept` : `Run its ${n} recipe, from its ws.repo.toml, while you have this mission open`}
            >
              {busy === n ? <Spinner size={11} /> : <Icon name="play" size={11} />} {busy === n ? `Starting ${n}…` : `Run ${n}`}
            </Button>
          ))}
        </MissionWorkRecipes>
      )}
    </>
  );
}
