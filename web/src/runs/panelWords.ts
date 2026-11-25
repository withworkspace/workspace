import type { Mission, RecipeInfo, RunDetail } from "../api";
import { came, type Said } from "./runRows";

// What a run's panel (RunPanel, RecipePanel) says of it, in words.

// shownAs is what a service's recipe is for, and how it's shown.
export function shownAs(rc: RecipeInfo): string {
  return `${rc.kind === "tool" ? "A tool" : "An app"}, ${rc.render === "tab" ? "opened in a tab of its own" : "framed here"}`;
}

// standing is where a run stands, in words: up on its port, what a task
// came to, how a service ended.
export function standing(r: RunDetail): string {
  if (r.type === "task") return came(r);
  if (r.state === "running") return r.up ? `up on :${r.port}` : `starting on :${r.port}`;
  return r.state === "exited" ? `exited ${r.status ?? 0}` : "gone";
}

// checkSaid is the check of a mission's step a task was run for, going to
// the mission.
export function checkSaid(r: RunDetail, missions: Mission[]): Said {
  const c = r.check!;
  const m = missions.find((x) => x.slug === c.mission);
  return { text: `check ${c.n} of step ${c.step} of ${m?.title || c.mission}`, to: `/missions/${c.mission}` };
}
