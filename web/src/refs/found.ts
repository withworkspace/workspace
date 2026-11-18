import type { Repo, Workspace } from "../api";
import { findRefs, goneCapsule, GROUND, type Known, type PageRef, refHref, refKey } from "./refs";
import { lookOf } from "./views";

// Refs found for somewhere ws doesn't draw them itself: a plugin's app, in
// its frame, sends ws the words it shows and draws what ws finds in them
// (plugins/frame.ts). The finding is the same as for ws's own pages, and
// so is where each goes and how it looks (RefTag): the app is told, and
// holds no grammar and no list of its own.

// FoundRef is a ref in a run of text, as whoever draws it is told: where
// it is, ws's word for it (refKey: what's said back to ws of it), its
// kind, where it goes, and how its kind is drawn (views.tsx), as far as
// that's told: a pill, and an accent colour.
export interface FoundRef {
  from: number;
  to: number;
  ref: string;
  kind: PageRef["kind"];
  href: string;
  shape?: "pill";
  accent?: string;
}

// FoundEnv is what finding and drawing look at: what there is to name,
// the repos as ws labels them, and the workspace.
export interface FoundEnv {
  known: Known;
  repos: Map<string, Repo>;
  workspace: Workspace;
}

// refsFound is the refs in a run of text, in order, none over another,
// each as it's drawn where it's read: a file whose capsule has gone goes
// to the default branch, where it most likely is now.
export function refsFound(text: string, env: FoundEnv): FoundRef[] {
  return findRefs(text, env.known).map((f) => {
    const r = f.ref;
    const look = lookOf(r, { workspace: env.workspace, repos: env.repos, things: env.known.things });
    const to: PageRef = (r.kind === "file" || r.kind === "dir") && goneCapsule(r, env.known) ? { ...r, worktree: GROUND } : r;
    return {
      from: f.from,
      to: f.to,
      ref: refKey(r),
      kind: r.kind,
      href: refHref(to, env.known),
      ...(look.shape === "pill" ? { shape: "pill" as const } : {}),
      ...(look.accent ? { accent: look.accent } : {}),
    };
  });
}
