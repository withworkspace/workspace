import { useState } from "react";
import { type CapsuleDetail, type RunDetail, runRecipe, stopRun } from "../api";
import type { Reload } from "../load";
import { Icon } from "../ui/Icon";
import { humanError, toast } from "../ui/kit";
import { Time } from "../ui/time";

// A capsule's runs (ws run): what's running in it, each with its link and
// when it ends, and its repo's recipes not running, each to run. One run
// from here lasts while someone has the mission open, and 15 minutes
// after; crew's last until their step passes. Nothing shows for a capsule
// with neither.

export function RunsHere({ slug, c, open, reload }: { slug: string; c: CapsuleDetail; open: boolean; reload: Reload }) {
  const [busy, setBusy] = useState("");
  const runs = c.runs ?? [];
  const running = new Set(runs.filter((r) => r.state === "running").map((r) => r.recipe ?? r.name));
  const recipes = open && c.exists ? (c.recipes ?? []).filter((n) => !running.has(n)) : [];
  if (!runs.length && !recipes.length) return null;
  const start = (recipe: string) => {
    setBusy(recipe);
    runRecipe(slug, c.repo, c.name, recipe)
      .then((r) => {
        toast(r.up ? `${r.name} is up on ${r.port}` : `${r.name} is running on ${r.port}, not up yet`, { tone: "done" });
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
        Runs {runs.length > 0 && <span className="count">{runs.length}</span>}
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
            <button key={n} className="small" disabled={!!busy} onClick={() => start(n)} title={`Run its ${n} recipe, from its ws.repo.toml, while you have this mission open`}>
              <Icon name={busy === n ? "spinner" : "play"} size={11} /> {busy === n ? `Starting ${n}…` : `Run ${n}`}
            </button>
          ))}
        </div>
      )}
    </>
  );
}

export function RunLine({ r, busy, stop }: { r: RunDetail; busy: boolean; stop: () => void }) {
  const live = r.state === "running";
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
          {live && !r.up && <span className="chip tone-quiet">not up</span>}
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
        <button className="ghost small" disabled={busy} onClick={stop} title="End it now">
          <Icon name={busy ? "spinner" : "stop"} size={11} /> Stop
        </button>
      )}
    </li>
  );
}
