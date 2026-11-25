import { useState } from "react";
import { type RunDetail, runRecipe } from "../../api";
import { humanError } from "../../ui/text";
import { toast } from "../../ui/toast";
import { MissionVerifyStart, type MissionVerifyStartState } from "./MissionVerifyStart";
import type { TryRuns } from "./VerifyTryRuns";

// VerifyTryStart is a recipe a bundle says to try (ws run web), started
// in the bundle's capsule from the page: as the mission's Run buttons do,
// it lasts while someone has the mission open, and 15 minutes after; what
// it needs is started with it. Once it's running, it's its link; a task,
// once it's ended, what it came to.
//
// The capsule's runs reach the page by the mission's topic, which can come
// after a short task has ended: the run it started is kept here until the
// capsule's runs have it.
export function VerifyTryStart({ recipe, at }: { recipe: string; at: TryRuns & { capsule: NonNullable<TryRuns["capsule"]> } }) {
  const [asking, setAsking] = useState(false);
  const [started, setStarted] = useState<RunDetail>();
  const c = at.capsule;
  const ofIt = (c.runs ?? []).filter((r) => (r.recipe ?? r.name) === recipe);
  const listed = started && ofIt.find((r) => r.ref === started.ref && (r.nref ?? "") === (started.nref ?? ""));
  // What it stands at: the run it started until the capsule's runs have
  // it, then theirs.
  const run = listed ?? started ?? ofIt.find((r) => r.state === "running");
  const live = run?.state === "running";
  const task = (c.tasks ?? []).includes(recipe) || run?.type === "task";
  const ended = ofIt.find((r) => r.type === "task" && r.state !== "running");
  const state: MissionVerifyStartState = asking ? "starting" : live ? (task || run?.up ? "up" : "running") : "idle";
  const start = () => {
    setAsking(true);
    runRecipe(at.slug, c.repo, c.name, recipe)
      .then((r) => {
        setStarted(r);
        if (r.type !== "task" && !r.up) toast(`${r.name} is running on ${r.port}, not up yet`);
        return at.reload();
      })
      .catch((e: Error) => toast(humanError(e.message), { tone: "danger" }))
      .finally(() => setAsking(false));
  };
  return (
    <MissionVerifyStart
      recipe={recipe}
      task={task}
      state={state}
      link={run?.link}
      came={task && !live ? ended?.result : undefined}
      where={`${c.repo}/${c.name}`}
      note={at.moved ? "The capsule has moved on since this bundle: it starts as it is now" : undefined}
      start={start}
    />
  );
}
