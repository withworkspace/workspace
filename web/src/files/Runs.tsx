import { useState } from "react";
import { Link, useSearchParams } from "react-router";
import { endRun, type RecipeInfo, type RunDetail, runInWorktree, type WorktreeRunsInfo } from "../api";
import type { Reload } from "../load";
import { ServiceItem, TaskItem } from "../runs/RunItem";
import { RecipePanel } from "../runs/RecipePanel";
import { RunPanel } from "../runs/RunPanel";
import { pickedIn, placeRuns, runId, type ServiceRow } from "../runs/runRows";
import { Button } from "../ui/Button";
import { Empty } from "../ui/Empty";
import { GroupHead } from "../ui/GroupHead";
import { Icon } from "../ui/Icon";
import { Rows } from "../ui/Rows";
import { Spinner } from "../ui/Spinner";
import { humanError } from "../ui/text";
import { toast } from "../ui/toast";
import { useMedia } from "../ui/useMedia";
import { useWorkspace } from "../workspace/store";
import css from "./Runs.module.css";

// A ground's or capsule's Runs tab on the Files page (ws run), laid out as
// its Files are: a list beside the one picked. The list is its services,
// apps apart from tools (a recipe's kind), each recipe a row whether it's
// running or not; then its tasks' runs, what's running first, then the
// last to end, each task's runs that are kept among them, with Run for
// each of its task recipes not running.
//
// What's picked is the address's ?service= (a recipe's or a run's name)
// or ?run= (a task's run, ref#n), else the first worth a look: a service
// that's up, then one running, then the last task to end. A service is
// shown with how and why it was run, and itself in a frame or, where its
// recipe says render = "tab", a way to open it in a tab of its own; one
// that isn't running with Start. A task's run is shown with all it
// printed. With no room for both, the list is shown until one's picked.
//
// One run from here lasts while someone has this ground or capsule open
// here, and 15 minutes after. Its runs are the worktree's tabs' to ask
// for (useWorktreeTabs), as each ask is what says the page is open. A
// task run from here runs to its end, open or not.

type Busy = { id: string; what: "start" | "stop" | "again" | "clear" };

export function WorktreeRuns({ repo, name, value, reload }: { repo: string; name: string; value?: WorktreeRunsInfo; reload: Reload }) {
  const [params, setParams] = useSearchParams();
  const ask = { service: params.get("service") ?? undefined, run: params.get("run") ?? undefined };
  const narrow = useMedia("(max-width: 860px)");
  const missions = useWorkspace((w) => w.missions);
  const talks = useWorkspace((w) => w.conversations);
  const [busy, setBusy] = useState<Busy>();
  if (!value) return null;

  const place = placeRuns(value.runs, value.past, value.recipes);
  const recipes = new Map(value.recipes.map((rc) => [rc.name, rc]));
  const running = new Set(value.runs.filter((r) => r.state === "running").map((r) => r.recipe ?? r.name));
  const taskRecipes = value.recipes.filter((rc) => rc.type === "task");
  const latest = new Set(value.runs.map(runId));
  const picked = pickedIn(place, ask);

  if (!place.apps.length && !place.tools.length && !place.tasks.length && !taskRecipes.length)
    return (
      <div className={css.none}>
        <Empty icon="play" title="Nothing runs here">
          Nothing's run in it lately, and its ws.repo.toml has no recipes: a repo says how it's run as [run.service.&lt;name&gt;] and [run.task.&lt;name&gt;] there, and ws run
          starts one from a terminal.
        </Empty>
      </div>
    );

  // to is this tab's address with one thing picked, or none (the list).
  const to = (pick?: { service: string } | { run: string } | { view: string }) => {
    const next = new URLSearchParams(params);
    next.delete("service");
    next.delete("run");
    if (pick && "service" in pick) next.set("service", pick.service);
    if (pick && "run" in pick) next.set("run", pick.run);
    if (pick && "view" in pick) next.set("view", pick.view);
    return { search: `?${next.toString()}` };
  };
  const show = (pick?: { service: string } | { run: string }) => setParams(to(pick).search.slice(1), { replace: true });

  const start = (recipe: string, b: Busy) => {
    setBusy(b);
    runInWorktree(repo, name, recipe)
      .then((r) => {
        if (r.type !== "task") toast(r.up ? `${r.name} is up on ${r.port}` : `${r.name} is running on ${r.port}, not up yet`, { tone: "done" });
        show(r.type === "task" ? { run: runId(r) } : { service: r.recipe ?? r.name });
        return reload();
      })
      .catch((e: Error) => toast(humanError(e.message), { tone: "danger" }))
      .finally(() => setBusy(undefined));
  };
  const end = (r: RunDetail, what: "stop" | "clear") => {
    setBusy({ id: runId(r), what });
    endRun(r.ref)
      .then(() => {
        toast(what === "stop" ? `Stopped ${r.name}` : `Cleared ${r.name}`, { tone: "done" });
        if (what === "clear" && r.type === "task") show();
        return reload();
      })
      .catch((e: Error) => toast(humanError(e.message), { tone: "danger" }))
      .finally(() => setBusy(undefined));
  };
  // again is how a run that's ended is run again: its recipe, if it's
  // still one of this place's, and isn't running now.
  const again = (r: RunDetail) => (r.state !== "running" && r.recipe && recipes.has(r.recipe) && !running.has(r.recipe) ? () => start(r.recipe!, { id: runId(r), what: "again" }) : undefined);

  const isPicked = (s: ServiceRow | RunDetail) => !narrow && (picked?.service === s || picked?.task === s);
  const services = (title: string, rows: ServiceRow[]) =>
    rows.length > 0 && (
      <section className={css.group}>
        <GroupHead count={rows.length} className={css.groupHead}>
          {title}
        </GroupHead>
        <Rows className={css.rows}>
          {rows.map((s) => (
            <ServiceItem key={s.name} s={s} to={to({ service: s.name })} picked={isPicked(s)} />
          ))}
        </Rows>
      </section>
    );
  const idleTasks = taskRecipes.filter((rc) => !running.has(rc.name));

  const asked = !!(ask.service || ask.run);
  const back = narrow && (
    <Link to={to()} replace className={css.back} aria-label="Its runs" title="Its runs">
      <Icon name="arrow-left" size={14} />
    </Link>
  );
  const panel = (() => {
    if (picked?.task) {
      const r = picked.task;
      return (
        <RunPanel
          key={runId(r)}
          r={r}
          recipe={r.recipe ? recipes.get(r.recipe) : undefined}
          missions={missions}
          talks={talks}
          busy={busy?.id === runId(r) && busy.what !== "start" ? busy.what : undefined}
          onStop={() => end(r, "stop")}
          onRunAgain={again(r)}
          onClear={latest.has(runId(r)) ? () => end(r, "clear") : undefined}
          back={back}
        />
      );
    }
    const s = picked?.service;
    if (s?.run) {
      const r = s.run;
      return (
        <RunPanel
          key={runId(r)}
          r={r}
          recipe={s.recipe}
          missions={missions}
          talks={talks}
          busy={busy?.id === runId(r) && busy.what !== "start" ? busy.what : undefined}
          onStop={() => end(r, "stop")}
          onRunAgain={again(r)}
          onClear={() => end(r, "clear")}
          back={back}
        />
      );
    }
    if (s?.recipe) {
      const rc: RecipeInfo = s.recipe;
      return (
        <RecipePanel
          key={rc.name}
          rc={rc}
          busy={busy?.id === rc.name}
          onStart={() => start(rc.name, { id: rc.name, what: "start" })}
          pageTo={rc.page ? to({ view: rc.name }) : undefined}
          back={back}
        />
      );
    }
    return (
      <div className={css.none}>
        <Empty icon="play" title="Nothing's run here lately">
          Run one of its tasks and it's shown here, with all it printed.
        </Empty>
      </div>
    );
  })();

  return (
    <div className={css.runs}>
      {(!narrow || !asked) && (
        <aside className={css.list} aria-label="Its runs">
          {services("Apps", place.apps)}
          {services("Tools", place.tools)}
          {(place.tasks.length > 0 || taskRecipes.length > 0) && (
            <section className={css.group}>
              <GroupHead count={place.tasks.length || undefined} className={css.groupHead}>
                Tasks
              </GroupHead>
              {idleTasks.length > 0 && (
                <div className={css.run}>
                  {idleTasks.map((rc) => (
                    <Button
                      size="small"
                      key={rc.name}
                      disabled={!!busy}
                      onClick={() => start(rc.name, { id: rc.name, what: "start" })}
                      title={`Run its ${rc.name} task, from its ws.repo.toml: what it comes to is kept`}
                    >
                      {busy?.id === rc.name ? <Spinner size={11} /> : <Icon name="play" size={11} />} {rc.title || rc.name}
                    </Button>
                  ))}
                </div>
              )}
              {place.tasks.length === 0 ? (
                <p className={css.quiet}>None of its tasks has run here lately. What a task comes to is kept a month, with the ten runs of it before.</p>
              ) : (
                <Rows className={css.rows}>
                  {place.tasks.map((r) => (
                    <TaskItem key={runId(r)} r={r} title={r.recipe ? recipes.get(r.recipe)?.title : undefined} to={to({ run: runId(r) })} picked={isPicked(r)} />
                  ))}
                </Rows>
              )}
            </section>
          )}
        </aside>
      )}
      {(!narrow || asked) && panel}
    </div>
  );
}
