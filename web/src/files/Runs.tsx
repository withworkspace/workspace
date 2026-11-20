import { useEffect, useState } from "react";
import { endRun, getWorktreeRuns, type RunDetail, runInWorktree, type WorktreeRunsInfo } from "../api";
import { useLoaded } from "../load";
import { RunLine } from "../mission/Runs";
import { Icon } from "../ui/Icon";
import { humanError, toast } from "../ui/kit";

// A ground's or capsule's runs on the Files page (ws run): what's running
// in it, whoever started it, and its ws.repo.toml's recipes not running,
// each to run. One run from here lasts while someone has this ground or
// capsule open here, and 15 minutes after. Nothing shows for one with
// neither.

// lookEvery is how often it asks again while the page is in view, so a
// run coming up or ending shows; hiddenEvery while it isn't. Each ask is
// what says the page is open: the server forgets one that hasn't asked
// for two minutes, and what was started from it ends 15 after.
const lookEvery = 3000;
const hiddenEvery = 30000;

export function WorktreeRuns({ repo, name }: { repo: string; name: string }) {
  const { value, reload } = useLoaded<WorktreeRunsInfo>(() => getWorktreeRuns(repo, name), [repo, name]);
  const [busy, setBusy] = useState("");
  useEffect(() => {
    let last = Date.now();
    const t = setInterval(() => {
      if (Date.now() - last < (document.visibilityState === "visible" ? lookEvery : hiddenEvery)) return;
      last = Date.now();
      reload();
    }, lookEvery);
    return () => clearInterval(t);
  }, [reload]);
  const runs = value?.runs ?? [];
  const running = new Set(runs.filter((r) => r.state === "running").map((r) => r.recipe ?? r.name));
  const recipes = (value?.recipes ?? []).filter((n) => !running.has(n));
  if (!runs.length && !recipes.length) return null;
  const start = (recipe: string) => {
    setBusy(recipe);
    runInWorktree(repo, name, recipe)
      .then((r) => {
        toast(r.up ? `${r.name} is up on ${r.port}` : `${r.name} is running on ${r.port}, not up yet`, { tone: "done" });
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
            <button key={n} className="small" disabled={!!busy} onClick={() => start(n)} title={`Run its ${n} recipe, from its ws.repo.toml, while you have it open here`}>
              <Icon name={busy === n ? "spinner" : "play"} size={11} /> {busy === n ? `Starting ${n}…` : `Run ${n}`}
            </button>
          ))}
        </div>
      )}
    </section>
  );
}
