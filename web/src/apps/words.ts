import type { AppDetail, AppPointer } from "../api";

// pointTitle says what a need is, on its pointer.
export function pointTitle(p: AppPointer): string {
  return p.folder ? `${p.need}: ${p.repo}'s folder, from the worktree picked` : `${p.need}: ${p.repo}'s ${p.recipe}, run in the worktree picked`;
}

// AppStanding is how an app's run stands, in a word: up, coming up,
// ended by itself, or not running.
export type AppStanding = { kind: "up" | "starting" | "exited" | "off"; words: string; title?: string };

// standingOf is how the run stands, with when it ends on hover; nothing
// before it's been asked.
export function standingOf(app: AppDetail | undefined, stopped: boolean): AppStanding | undefined {
  const run = app?.run;
  if (!app) return undefined;
  if (run?.state === "running") return app.ready ? { kind: "up", words: "Up", title: `It ends ${run.ends}.` } : { kind: "starting", words: "Starting", title: `It ends ${run.ends}.` };
  if (run && run.state === "exited") return { kind: "exited", words: `Exited ${run.status}` };
  return { kind: "off", words: stopped ? "Stopped" : "Not running" };
}

// waitingWords is what the card says while there's nothing to frame: the
// run coming up, or why there isn't one.
export function waitingWords(app: AppDetail | undefined, title: string, stopped: boolean): string {
  const run = app?.run;
  if (!app) return `Starting ${title}…`;
  if (run?.state === "running") return `Starting ${title}: it's shown here once it answers.`;
  if (run?.state === "exited") return `${title} ended by itself (exit status ${run.status}). Its last words are below.`;
  if (stopped) return `${title} is stopped. Start it to see it again.`;
  return `${title} isn't running. Start it to see it.`;
}
