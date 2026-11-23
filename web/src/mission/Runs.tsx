import { useEffect, useState } from "react";
import { Link } from "react-router";
import { type CapsuleDetail, type RunDetail, runRecipe, stopRun } from "../api";
import type { Reload } from "../load";
import { came, commitSaid } from "../runs/runRows";
import { Icon } from "../ui/Icon";
import { Spinner } from "../ui/Spinner";
import { humanError, toast } from "../ui/kit";
import { Time } from "../ui/time";
import { Chip } from "../ui/Chip";
import { Count } from "../ui/Count";
import { Button } from "../ui/Button";

// A capsule's runs (ws run): what's running in it, each with its link and
// when it ends, and its repo's recipes not running, each to run. One run
// from here lasts while someone has the mission open, and 15 minutes
// after; crew's last until their step passes. Nothing shows for a capsule
// with neither.
//
// A task (tests, lint) has no link: it shows what it came to, at which
// commit, and goes to its run on the Runs page, where all it printed and
// its reports are. One run from here is the mission's, and runs to its end.

export function RunsHere({ slug, c, open, reload }: { slug: string; c: CapsuleDetail; open: boolean; reload: Reload }) {
  const [busy, setBusy] = useState("");
  const runs = c.runs ?? [];
  const running = new Set(runs.filter((r) => r.state === "running").map((r) => r.recipe ?? r.name));
  const recipes = open && c.exists ? (c.recipes ?? []).filter((n) => !running.has(n)) : [];
  const tasks = new Set(c.tasks ?? []);
  // A task that's running is asked after until it ends: nothing else
  // tells the page it has.
  const tasking = runs.some((r) => r.type === "task" && r.state === "running");
  useEffect(() => {
    if (!tasking) return;
    const t = setInterval(() => {
      if (document.visibilityState === "visible") reload();
    }, 2000);
    return () => clearInterval(t);
  }, [tasking, reload]);
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
      <h3>
        Runs {runs.length > 0 && <Count>{runs.length}</Count>}
      </h3>
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
              title={tasks.has(n) ? `Run its ${n} task, from its ws.repo.toml: what it comes to is kept` : `Run its ${n} recipe, from its ws.repo.toml, while you have this mission open`}
            >
              {busy === n ? <Spinner size={11} /> : <Icon name="play" size={11} />} {busy === n ? `Starting ${n}…` : `Run ${n}`}
            </Button>
          ))}
        </div>
      )}
    </>
  );
}

// TaskLine is a task in a capsule: what it came to, at which commit, going
// to its run on the Runs page.
function TaskLine({ r, busy, stop }: { r: RunDetail; busy: boolean; stop: () => void }) {
  const live = r.state === "running";
  const icon = r.result === "passed" ? "check-circle" : r.result === "failed" ? "x-circle" : "circle-dashed";
  return (
    <li className="work-run">
      {live ? (
        <span className="work-run-mark up" aria-hidden>
          <span className="work-live-dot" />
        </span>
      ) : (
        <span className={`work-run-mark task-mark ${r.result}`} role="img" aria-label={r.result}>
          <Icon name={icon} size={13} />
        </span>
      )}
      <span className="work-run-main">
        <span className="work-run-top">
          <Link to={`/runs?show=${encodeURIComponent(r.nref ?? r.ref)}`} className="mono work-run-name" title={`${r.command}: its output and reports`}>
            {r.name}
          </Link>
          <span className={`task-came ${r.result}`}>
            {came(r)}
            {!live && r.ended && (
              <span className="dim">
                , <Time iso={r.ended} as="relative" />
              </span>
            )}
          </span>
        </span>
        {(r.commit || r.check) && (
          <span className="work-run-ends">
            {r.commit && <span title={r.commit}>At {commitSaid(r)}</span>}
            {r.commit && r.check && " · "}
            {r.check && (
              <>
                check {r.check.n} of step {r.check.step}
              </>
            )}
          </span>
        )}
      </span>
      {live && (
        <Button variant="ghost" size="small" disabled={busy} onClick={stop} title="Stop it now: it's kept as stopped">
          {busy ? <Spinner size={11} /> : <Icon name="stop" size={11} />} Stop
        </Button>
      )}
    </li>
  );
}

export function RunLine({ r, busy, stop }: { r: RunDetail; busy: boolean; stop: () => void }) {
  const live = r.state === "running";
  if (r.type === "task") return <TaskLine r={r} busy={busy} stop={stop} />;
  return (
    <li className={`work-run${live ? "" : " ended"}`}>
      <span className={`work-run-mark${live && r.up ? " up" : ""}`} aria-hidden>
        {live ? <span className="work-live-dot" /> : <Icon name="circle" size={8} />}
      </span>
      <span className="work-run-main">
        <span className="work-run-top">
          <span className="mono work-run-name" title={r.command}>
            {r.name}
          </span>
          {live ? (
            <a href={r.link} target="_blank" rel="noopener noreferrer" className="mono work-run-link" title={`${r.up ? "Open it" : "Nothing's listening on its port yet"}${r.link !== r.local ? `: ${r.local.replace(/^https?:\/\//, "")}` : ""}`}>
              {r.link.replace(/^https?:\/\//, "").replace(/\/$/, "")}
              <Icon name="arrow-up-right" size={11} />
            </a>
          ) : (
            <span className="dim">
              {r.state === "exited" ? `exited ${r.status ?? 0}` : "gone"}
              {r.ended && (
                <>
                  , <Time iso={r.ended} as="relative" />
                </>
              )}
            </span>
          )}
          {live && !r.up && <Chip tone="quiet">not up</Chip>}
        </span>
        {live && (r.needs?.length ?? 0) > 0 && (
          <span className="work-run-ends">
            Needs{" "}
            {r.needs?.map((n, i) => (
              <span key={n.name}>
                {i > 0 && ", "}
                {n.name} (
                {n.folder ? (
                  // A repo's folder: the worktree that meets it.
                  <span className="mono" title={n.dir}>
                    {n.repo} / {n.worktree}
                  </span>
                ) : (
                  <a href={n.address} target="_blank" rel="noopener noreferrer" className="mono" title={n.address}>
                    {n.run ?? n.address.replace(/^https?:\/\//, "")}
                  </a>
                )}
                {n.mount && r.link !== r.local ? `, at ${n.mount}` : ""})
              </span>
            ))}
          </span>
        )}
        {live && r.ends && <span className="work-run-ends">Ends {r.ends}</span>}
      </span>
      {live && (
        <Button variant="ghost" size="small" disabled={busy} onClick={stop} title="End it now">
          {busy ? <Spinner size={11} /> : <Icon name="stop" size={11} />} Stop
        </Button>
      )}
    </li>
  );
}
