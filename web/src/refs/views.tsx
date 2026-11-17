import type { Repo, Workspace } from "../api";
import type { IconName } from "../ui/Icon";
import { stripGlyph } from "../ui/status";
import type { PageRef } from "./refs";

// How each kind of ref is drawn, said once, here: the tag in what's read
// (RefTag: a conversation, a thread, a log line) and the mark in a page's
// editor (editor/refs.ts) both draw a ref from its Look. To draw a kind
// differently, change its entry; its card is RefCard's.

// RefEnv is what drawing a ref may look at: the workspace (where a
// mission stands, a page's title), and the repos as ws labels them.
export interface RefEnv {
  workspace: Workspace;
  repos: Map<string, Repo>;
}

// Look is how a ref is drawn.
export interface Look {
  // shape: words, marked as somewhere to go; or a pill.
  shape: "words" | "pill";
  icon: IconName;
  // text is shown in place of the ref's name (a repo's label for its name),
  // when the words are the name; words of the writer's own are kept.
  text?: string;
  // accent is a CSS colour for it (a repo's).
  accent?: string;
  // after is what's said after it: where it stands.
  after?: { label: string; tone?: string };
}

const short = (s: string) => (s.length > 44 ? s.slice(0, 43) + "…" : s);

type Views = { [K in PageRef["kind"]]: (r: Extract<PageRef, { kind: K }>, env: RefEnv) => Look };

const views: Views = {
  file: () => ({ shape: "words", icon: "file" }),
  dir: () => ({ shape: "words", icon: "layers" }),
  // A repo: a pill in its colour, by its label.
  repo: (r, env) => {
    const repo = env.repos.get(r.repo);
    return { shape: "pill", icon: "box", text: repo?.label || undefined, accent: repo?.color || undefined };
  },
  pr: () => ({ shape: "words", icon: "pr" }),
  mission: (r, env) => {
    const m = env.workspace.missions.find((x) => x.slug === r.slug);
    if (!m) return { shape: "words", icon: "flag" };
    const tone = { "needs-you": "attention", "in-progress": "progress", waiting: "waiting", "up-next": "quiet", past: "quiet" }[m.headline.group];
    return { shape: "words", icon: "flag", after: { label: stripGlyph(m.headline.label) || m.state, tone } };
  },
  page: (r, env) => {
    const p = env.workspace.pages?.find((x) => x.id === r.id);
    return { shape: "words", icon: "file", after: p ? { label: short(p.title), tone: "quiet" } : undefined };
  },
  talk: (r, env) => {
    const t = env.workspace.conversations.find((x) => x.id === r.id);
    return { shape: "words", icon: "message", after: t ? { label: short(t.title || t.opening), tone: "quiet" } : undefined };
  },
  artifact: (r, env) => {
    const a = env.workspace.artifacts?.find((x) => x.id === r.id);
    return { shape: "words", icon: "artifact", after: a ? { label: short(a.title), tone: "quiet" } : undefined };
  },
};

// lookOf is how a ref is drawn.
export function lookOf(r: PageRef, env: RefEnv): Look {
  return (views[r.kind] as (r: PageRef, env: RefEnv) => Look)(r, env);
}
