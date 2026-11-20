import type { InstalledPlugin, PluginChanged, WantedPlugin } from "../api";

// What the Plugins page says of a plugin, in words: where it stands,
// where it's from, what it adds, and what its Update would do. Kept apart
// from the page so each can be read, and tested, without drawing it.

export type Tone = "done" | "attention" | "danger" | "quiet";

// stateOf is where a plugin stands: a word for its chip, and the chip's
// tone. Stopped isn't trouble: it's started when it's next needed.
export function stateOf(p: Pick<InstalledPlugin, "state">): { label: string; tone: Tone; title: string } {
  switch (p.state) {
    case "ready":
      return { label: "Running", tone: "done", title: "It's running, and can be asked" };
    case "setup":
      return { label: "Needs setting up", tone: "attention", title: "Something it requires isn't set" };
    case "failed":
      return { label: "Failed", tone: "danger", title: "It didn't start, or keeps stopping" };
    default:
      return { label: "Not running", tone: "quiet", title: "It's started when it's next needed: its view opened, a command of its run" };
  }
}

// home is a path with your home folder as ~, where it says so (a path
// under /Users/<you> or /home/<you>): shorter, and what you'd type.
export function home(path: string): string {
  return path.replace(/^\/(?:Users|home)\/[^/]+(?=\/|$)/, "~");
}

// fromOf is where a plugin's from, as a line: the repo it was installed
// from and which release, the folder it's linked to, or a folder it was
// copied from.
export function fromOf(p: Pick<InstalledPlugin, "from">): string {
  const f = p.from;
  if (f.linked) return `Linked to ${home(f.linked)}`;
  if (!f.repo) return "Copied from a folder on this machine";
  if (f.built) return f.tag ? `${f.repo} ${f.tag}, built here from its source` : `${f.repo}, built here from its source`;
  return `${f.repo} ${f.tag ?? ""}`.trim();
}

// repoLink is a repo's page on GitHub, or a release's.
export function repoLink(repo: string, tag?: string): string {
  return `https://github.com/${repo}${tag ? `/releases/tag/${encodeURIComponent(tag)}` : ""}`;
}

// addsOf is what a plugin adds to the workspace, each in a few words:
// nothing, for one that's never started to say.
export function addsOf(p: Pick<InstalledPlugin, "views" | "commands" | "guide" | "names">): string[] {
  const out: string[] = [];
  const views = p.views ?? [];
  if (views.length === 1) out.push(`${views[0]} in the sidebar`);
  if (views.length > 1) out.push(`${views.join(", ")} in the sidebar`);
  if (p.commands) out.push(`ws ${p.commands}`);
  if (p.guide) out.push("a guide for agents");
  if (p.names?.length) out.push(`names ${p.names.join(", ")}`);
  return out;
}

// updateOf is what a plugin's Update does, and whether there's one known
// to make. One from a folder has none: it's the folder's. One built here
// from source has no saying whether it's changed, so it's fetched and
// built again.
export function updateOf(p: Pick<InstalledPlugin, "from" | "latest" | "newer">): { can: boolean; out: boolean; label: string; title: string } {
  if (!p.from.repo) return { can: false, out: false, label: "", title: "" };
  if (p.newer && p.latest) return { can: true, out: true, label: `Update to ${p.latest}`, title: `${p.latest} is out: fetch it, and start the plugin afresh. Its config and data stay.` };
  if (p.from.built) return { can: true, out: false, label: "Fetch again", title: "It was built here from its source: fetch it and build it again, as there's no saying whether it's changed" };
  return { can: true, out: false, label: "Look for an update", title: "Ask its repo for a later release now, and fetch one if there is" };
}

// changedSaid is what adding or updating a plugin did, for its toast.
export function changedSaid(c: PluginChanged, was: "added" | "updated"): string {
  const needs = c.missing?.length ? `. It needs setting up: ${c.missing.join(", ")}` : "";
  if (was === "added") return `Added ${c.name}${c.version ? ` ${c.version}` : ""}${needs}`;
  if (c.current) return `${c.name} ${c.now ?? ""} is the latest`.replace("  ", " ");
  return `${c.name}: ${c.was ?? "what was here"} → ${c.now ?? "its latest"}${needs}`;
}

// wantedFrom is what adding one of the team's plugins is given: where it
// comes from, at the release the team names if it names one.
export function wantedFrom(w: WantedPlugin): string {
  return w.version && !w.source.includes("@") ? `${w.source}@${w.version}` : w.source;
}

// outCount is how many of the plugins a later release is out for.
export const outCount = (plugins: InstalledPlugin[]) => plugins.filter((p) => p.newer).length;
