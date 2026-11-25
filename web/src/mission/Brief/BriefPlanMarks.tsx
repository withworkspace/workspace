import type { Stage } from "../../api";
import { missionBriefDocSelector } from "./MissionBriefDoc";

// BriefPlanMarks marks, in the plan as it reads, where the mission is on
// it: the steps behind it, and the one it's at with who's on it. (A
// revision changes only what's ahead.) They're the plan's own list items,
// marked by where they come in it, so the editor's left to draw them.
export function BriefPlanMarks({ stages, on }: { stages: Stage[]; on: boolean }) {
  if (!on || !stages.length) return null;
  const li = (n: number) => `${missionBriefDocSelector("plan")} .markdown > ol:first-of-type > li:nth-child(${n})`;
  const done = stages.filter((s) => s.state === "accepted").map((s) => li(s.n));
  const now = stages.find((s) => s.state === "current");
  const says = (s: string) => `"${s.replace(/["\\]/g, "")}"`;
  const css = [
    done.length ? `${done.map((x) => `${x}::before`).join(",")}{content:"Done";background:var(--well);color:var(--dim);}` : "",
    done.length ? `${done.join(",")}{color:var(--dim);}` : "",
    now ? `${li(now.n)}::before{content:${says(`Now${now.crew?.length ? ` · ${now.crew.join(", ")}` : ""}`)};}` : "",
    now ? `${li(now.n)}{box-shadow:-3px 0 0 var(--progress);}` : "",
  ].join("");
  return <style>{css}</style>;
}
