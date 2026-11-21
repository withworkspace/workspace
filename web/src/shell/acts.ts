import { type ActVerb, prAPI, type Workspace } from "../api";
import type { IconName } from "../ui/Icon";
import { onGitHub, remoteOf } from "../refs/refs";
import { crewStatus } from "../ui/status";
import { type Item, missionDo } from "./find";

// What can be done to one of ws's own things, from Find: on the thing's
// own page (Here), or from its row anywhere (⌘K on it). Never in the list
// at large: a conversation's Resolve is under that conversation, not one
// row per conversation among everything else.

// Subject is a thing of ws's own that has things to do to it, by the id
// its row has in Find (mission:slug, talk:id, artifact:id, pr:repo#n).
export type Subject =
  | { kind: "mission"; slug: string }
  | { kind: "talk"; id: string }
  | { kind: "artifact"; id: string }
  | { kind: "pr"; repo: string; number: number }
  | { kind: "settings"; section: string };

// subjectOf is the thing a row of Find is, if it has things to do to it.
export function subjectOf(id: string | undefined): Subject | undefined {
  if (!id) return undefined;
  const i = id.indexOf(":");
  const kind = id.slice(0, i);
  const rest = id.slice(i + 1);
  if (!rest) return undefined;
  switch (kind) {
    case "mission":
      return { kind: "mission", slug: rest };
    case "talk":
      return { kind: "talk", id: rest };
    case "artifact":
      return { kind: "artifact", id: rest };
    case "settings":
      return { kind: "settings", section: rest };
    case "pr": {
      const m = /^([\w.-]+)#(\d+)$/.exec(rest);
      return m ? { kind: "pr", repo: m[1], number: Number(m[2]) } : undefined;
    }
  }
  return undefined;
}

// WsDoing is something ws does to one of its things, from Find, that isn't
// a mission's act: a conversation resolved or reopened, an artifact
// restored, a pull request docked.
export type WsDoing = { talk: string; status: "open" | "resolved" } | { restore: string } | { dock: string };

const row = (id: string, title: string, icon: IconName, more: Partial<Item>): Item => ({ id: `act:${id}`, group: "do", kind: "Do", title, icon, score: 0, ...more });

// actsFor is what can be done to a thing of ws's own, as it stands now:
// the first what it asks of you, if it asks; then what changes it; then
// where else it's read. Nothing, for a thing the workspace doesn't hold.
export function actsFor(s: Subject, w: Workspace): Item[] {
  switch (s.kind) {
    case "settings":
      // A section of Settings is a place: what's done there, from Find,
      // is what the section itself offers.
      return s.section === "repos" ? [row("add-repo", "Add a repo from GitHub", "plus", { hint: "yours, or the team's", to: "/settings/repos?add=1" })] : [];
    case "mission": {
      const m = w.missions.find((x) => x.slug === s.slug);
      if (!m) return [];
      const out: Item[] = [];
      const asked = missionDo(m);
      if (asked) out.push({ ...asked, id: `act:headline:${m.slug}` });
      const has = (v: ActVerb) => asked?.doing && "act" in asked.doing && asked.doing.act === v;
      const verb = (v: ActVerb, title: string, icon: IconName, hint?: string) => !has(v) && out.push(row(`${v}:${m.slug}`, title, icon, { hint, doing: { act: v, slug: m.slug } }));
      if (m.state === "parked") verb("unpark", "Unpark it", "play", "back to work");
      else if (m.state !== "archived" && m.state !== "briefing") verb("park", "Park it", "pause", "put aside, out of the way");
      for (const c of m.crew.filter((c) => crewStatus(c).live))
        out.push(row(`stop:${m.slug}/${c.id}`, `Stop ${c.id}`, "stop", { hint: [c.harness, c.capsule].filter(Boolean).join(" · "), doing: { act: "stop", slug: m.slug, crew: c.id } }));
      if (m.state === "archived") verb("unarchive", "Unarchive it", "rotate", "bring it back");
      else verb("archive", "Archive it", "archive", "burns its capsules");
      out.push(row(`brief:${m.slug}`, "Its brief", "flag", { hint: "plan", to: `/missions/${encodeURIComponent(m.slug)}?tab=brief` }));
      return out;
    }
    case "talk": {
      const t = w.conversations.find((x) => x.id === s.id);
      if (!t) return [];
      const out: Item[] = [];
      if (t.status === "resolved") out.push(row(`reopen:${t.id}`, "Reopen it", "rotate", { doing: { ws: { talk: t.id, status: "open" } } }));
      else out.push(row(`resolve:${t.id}`, "Resolve it", "check-circle", { hint: "close done", doing: { ws: { talk: t.id, status: "resolved" } } }));
      if (t.became) out.push(row(`became:${t.id}`, "The mission it became", "flag", { to: `/missions/${encodeURIComponent(t.became)}` }));
      const pr = t.pr && /^([\w.-]+)#(\d+)$/.exec(t.pr);
      if (pr) out.push(row(`pr:${t.id}`, `Its pull request, ${t.pr}`, "pr", { to: `/reviews/${encodeURIComponent(pr[1])}/${pr[2]}` }));
      return out;
    }
    case "artifact": {
      const a = w.artifacts?.find((x) => x.id === s.id);
      if (!a) return [];
      const out: Item[] = [];
      if (a.status === "discarded") out.push(row(`restore:${a.id}`, "Restore it", "rotate", { doing: { ws: { restore: a.id } } }));
      if (a.versions > 1) out.push(row(`compare:${a.id}`, "Compare its versions", "layers", { to: `/artifacts/${encodeURIComponent(a.id)}/compare` }));
      return out;
    }
    case "pr": {
      const out: Item[] = [row(`dock:${s.repo}#${s.number}`, "Dock it here", "branch", { hint: "check out a capsule, as ws dock does", doing: { ws: { dock: prAPI(s.repo, s.number) } } })];
      if (onGitHub(w, s.repo)) out.push(row(`github:${s.repo}#${s.number}`, "Open it on GitHub", "external", { to: `https://github.com/${remoteOf(w, s.repo)}/pull/${s.number}` }));
      return out;
    }
  }
}
