import { useState } from "react";
import { endRun, type RunDetail, runInWorktree, type WorktreeRunsInfo } from "../api";
import type { Reload } from "../load";
import { RunLine } from "../mission/Runs";
import { Icon } from "../ui/Icon";
import { Spinner } from "../ui/Spinner";
import { toast } from "../ui/kit";
import { Empty } from "../ui/Empty";
import { humanError } from "../ui/text";
import { Button } from "../ui/Button";

// A ground's or capsule's Runs tab on the Files page (ws run): what's
// running in it, whoever started it, and its ws.repo.toml's recipes not
// running, each to run. One run from here lasts while someone has this
// ground or capsule open here, and 15 minutes after. Its runs are the
// worktree's tabs' to ask for (useWorktreeTabs), as each ask is what says
// the page is open.
//
// A task (tests, lint) run from here runs to its end, open or not, and
// shows what it came to, going to its run on the Runs page.

export function WorktreeRuns({ repo, name, value, reload }: { repo: string; name: string; value?: WorktreeRunsInfo; reload: Reload }) {
  const [busy, setBusy] = useState("");
  const runs = value?.runs ?? [];
  const running = new Set(runs.filter((r) => r.state === "running").map((r) => r.recipe ?? r.name));
  const recipes = (value?.recipes ?? []).filter((n) => !running.has(n));
  const tasks = new Set(value?.tasks ?? []);
  if (!value) return null;
  if (!runs.length && !recipes.length)
    return (
      <Empty icon="play" title="Nothing runs here">
        Nothing's running in it, and its ws.repo.toml has no recipes: a repo says how it's run as [run.service.&lt;name&gt;] and [run.task.&lt;name&gt;] there, and ws run starts one from a terminal.
      </Empty>
    );
  const start = (recipe: string) => {
    setBusy(recipe);
    runInWorktree(repo, name, recipe)
      .then((r) => {
        if (r.type !== "task") toast(r.up ? `${r.name} is up on ${r.port}` : `${r.name} is running on ${r.port}, not up yet`, { tone: "done" });
        return reload();
      })
      .catch((e: Error) => toast(humanError(e.message), { tone: "danger" }))
      .finally(() => setBusy(""));
  };
  const stop = (r: RunDetail) => {
    setBusy(r.ref);
    endRun(r.ref)
      .then(() => reload())
      .catch((e: Error) => toast(humanError(e.message), { tone: "danger" }))
      .finally(() => setBusy(""));
  };
  return (
    <section className="files-runs" aria-label="Runs">
      {runs.length > 0 && (
        <ul className="work-runs">
          {runs.map((r) => (
            <RunLine key={r.ref} r={r} busy={busy === r.ref} stop={() => stop(r)} />
          ))}
        </ul>
      )}
      {recipes.length > 0 && (
        <div className="work-run-recipes">
          {recipes.map((n) => (
            <Button
              size="small"
              key={n}
              disabled={!!busy}
              onClick={() => start(n)}
              title={tasks.has(n) ? `Run its ${n} task, from its ws.repo.toml: what it comes to is kept` : `Run its ${n} recipe, from its ws.repo.toml, while you have it open here`}
            >
              {busy === n ? <Spinner size={11} /> : <Icon name="play" size={11} />} {busy === n ? `Starting ${n}…` : `Run ${n}`}
            </Button>
          ))}
        </div>
      )}
    </section>
  );
}
