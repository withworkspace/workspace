import type { ArtifactFound, ArtifactInfo, ArtifactPromotion, ArtifactVersion } from "../api";
import type { PageRef } from "../refs/refs";
import type { StatusInfo } from "../ui/Status";

// Artifacts, apart from how they're drawn: which the list shows, where a
// version came from, and stepping through versions.

// Shown is the list's filter by where an artifact stands. "all" is the
// list: live and promoted. A discarded one is gone from the list, and
// shows only when it's asked for.
export type Shown = "all" | "live" | "promoted" | "discarded";

export const shownLabels: Record<Shown, string> = { all: "All", live: "Live", promoted: "Promoted", discarded: "Discarded" };

// The list's filter by where an artifact came from: anywhere, by hand
// (no version from a conversation or a mission), or one of those.
export const ANYWHERE = "";
export const BY_HAND = "-";

// listed is what the list shows: by where it stands and where it came
// from, the latest version's newest first (as the server has them).
export function listed(all: ArtifactInfo[], shown: Shown, from: string): ArtifactInfo[] {
  return all.filter((a) => {
    const standing = shown === "all" ? a.status !== "discarded" : a.status === shown;
    if (!standing) return false;
    if (from === ANYWHERE) return true;
    if (from === BY_HAND) return !a.from?.length;
    return !!a.from?.some((f) => f.from === from);
  });
}

// Source is somewhere versions came from: a conversation or a mission,
// with its title as the version had it.
export interface Source {
  from: string;
  title?: string;
}

// sources are where the artifacts' versions came from, each once, in the
// order the list has them: for the filter.
export function sources(all: ArtifactInfo[]): Source[] {
  const seen = new Map<string, Source>();
  for (const a of all) for (const f of a.from ?? []) if (!seen.has(f.from)) seen.set(f.from, { from: f.from, title: f.title });
  return [...seen.values()];
}

// isConversation is whether a version's from is a conversation (w9);
// otherwise it's a mission's slug, or a mission's conversation.
export const isConversation = (from: string) => /^w[1-9]\d*$/.test(from);

// missionTalk reads a version's from as a mission's conversation (glow/t3,
// confirmed there): its mission's slug, and the conversation.
export function missionTalk(from: string): { slug: string; id: string } | undefined {
  const m = /^([^/\s]+)\/(t[1-9]\d*)$/.exec(from);
  return m ? { slug: m[1], id: m[2] } : undefined;
}

// fromRef is where a version came from, as a ref: a conversation, or a
// mission (a mission's conversation is drawn as its mission).
export function fromRef(from: string): PageRef {
  if (isConversation(from)) return { kind: "talk", id: from };
  return { kind: "mission", slug: missionTalk(from)?.slug ?? from };
}

// byWords is who added a version: you, or crew (c1).
export const byWords = (v: Pick<ArtifactVersion, "by">) => (v.by === "you" ? "by you" : `by ${v.by}`);

// An artifact is on this machine. One that's promoted may live in a repo
// too, for everyone with it (its home): the version chosen, in a folder
// there.

// keptWords is where an artifact is kept, as the list says it: this
// machine, or the repo it lives in, with the version that's there.
export const keptWords = (a: Pick<ArtifactInfo, "home">) => (a.home ? `${a.home.repo} · v${a.home.version}` : "This machine");

// keptIcon is the icon beside where it's kept.
export const keptIcon = (a: Pick<ArtifactInfo, "home">) => (a.home ? "branch" : "monitor");

// keptTitle says where it's kept, in full.
export function keptTitle(a: Pick<ArtifactInfo, "where" | "home">): string {
  if (!a.home) return `On this machine only: ${a.where}`;
  return `v${a.home.version} in ${a.home.where}, shared via ${a.home.repo}. All versions on this machine: ${a.where}`;
}

// behind is how many versions an artifact has that are newer than the one
// in the repo it lives in: 0 for one that lives in none, or whose latest
// is there.
export const behind = (a: Pick<ArtifactInfo, "home" | "latest">) => (a.home && a.latest ? Math.max(0, a.latest.n - a.home.version) : 0);

// artifactStatus is where an artifact stands, as a status draws it.
export function artifactStatus(a: Pick<ArtifactInfo, "status" | "gone"> & Partial<Pick<ArtifactInfo, "expired">>): StatusInfo {
  if (a.gone) return { icon: "alert", tone: "danger", words: "Gone" };
  if (a.status === "live" && a.expired) return { icon: "clock", tone: "quiet", words: "Expired" };
  switch (a.status) {
    case "promoted":
      return { icon: "check-circle", tone: "done", words: "Promoted" };
    case "discarded":
      return { icon: "ban", tone: "quiet", words: "Discarded" };
  }
  return { icon: "circle-dot", tone: "progress", words: "Live" };
}

// stepVersion is the version before (-1) or after (+1) n, in the order
// they're listed; n itself at either end.
export function stepVersion(versions: Pick<ArtifactVersion, "n">[], n: number, by: -1 | 1): number {
  const ns = versions.map((v) => v.n).sort((a, b) => a - b);
  const i = ns.indexOf(n);
  if (i < 0) return ns[ns.length - 1] ?? n;
  return ns[Math.min(Math.max(i + by, 0), ns.length - 1)];
}

// versionOf reads a version from the address: v2 (or 2); 0 for none.
export function versionOf(s: string | undefined): number {
  const m = /^v?([1-9]\d{0,4})$/.exec(s ?? "");
  return m ? Number(m[1]) : 0;
}

// Width is how wide a version's frame is drawn: fitting the page, or as a
// desktop, a tablet or a phone would show it.
export interface Width {
  key: string;
  px: number; // 0 fits the page
  label: string;
  title: string;
}

export const widths: Width[] = [
  { key: "fit", px: 0, label: "Fit", title: "Page width" },
  { key: "1280", px: 1280, label: "1280", title: "Desktop, 1280px" },
  { key: "768", px: 768, label: "768", title: "Tablet, 768px" },
  { key: "390", px: 390, label: "390", title: "Phone, 390px" },
];

// filesWords says how much a version is: "3 files, 240 KB", or its size
// alone for one that's its page only (artifact.FilesWords).
export function filesWords(files: number | undefined, size: number): string {
  return files && files > 1 ? `${files} files, ${sizeWords(size)}` : sizeWords(size);
}

// sizeWords says a file's size as a person would (artifact.Size).
export function sizeWords(n: number): string {
  if (n >= 1 << 20) return `${(n / (1 << 20)).toFixed(1).replace(/\.0$/, "")} MB`;
  if (n >= 1 << 10) return `${Math.round(n / 1024)} KB`;
  return `${n} bytes`;
}

// --- Lifetime: expiry, promoting, discarding

const DAY = 24 * 60 * 60 * 1000;

// WARN is how long before it expires a live artifact is listed as
// expiring: its last week (artifact.Warning).
export const WARN = 7 * DAY;

// KEEP_DISCARDED is how long a folder discarded on this machine is kept, to
// restore it (artifact.KeepDiscarded).
export const KEEP_DISCARDED = 30 * DAY;

// Expiry is when a live artifact expires, as the list says it.
export interface Expiry {
  words: string; // "expires in 4 days", "expires today", "expired"
  soon: boolean; // in its last week: the list warns, with Promote beside it
  expired: boolean;
}

// expiryOf is when a live one expires, said as artifact.ExpiresWords says
// it; none for one that doesn't.
export function expiryOf(a: Pick<ArtifactInfo, "expires">, now: number): Expiry | undefined {
  if (!a.expires) return undefined;
  const left = Date.parse(a.expires) - now;
  if (Number.isNaN(left)) return undefined;
  if (left <= 0) return { words: "expired", soon: false, expired: true };
  const days = Math.floor(left / DAY);
  const words = days === 0 ? "expires today" : days === 1 ? "expires tomorrow" : `expires in ${days} days`;
  return { words, soon: left <= WARN, expired: false };
}

// clearable are the expired artifacts Clear expired takes: those the
// daemon hasn't swept yet.
export const clearable = (all: ArtifactInfo[]) => all.filter((a) => a.status === "live" && a.expired && !a.gone);

// promoteVersions are the versions an artifact can be promoted at, newest
// first.
export function promoteVersions(versions: Pick<ArtifactVersion, "n">[]): number[] {
  return versions.map((v) => v.n).sort((a, b) => b - a);
}

// Into is what a promoted artifact went into, as the Promote dialog asks
// it: nothing, a mission that's there, a mission started from it, or a
// document.
export type IntoKind = "nothing" | "mission" | "start" | "document";

// Lives is where a promoted artifact is to live, as the Promote dialog
// asks it the first time: the folder the workspace offers it in a repo,
// this machine only, or another folder (named as repo:folder/). "home" is
// where it lives already, which isn't asked.
export type Lives = "home" | "offered" | "here" | "other";

// livesAt is what the Promote dialog opens on, for where it's to live:
// where it lives; else, unless it was promoted to stay here before, the
// folder the workspace offers it; else this machine.
export function livesAt(a: Pick<ArtifactInfo, "home" | "here">, offered?: string): Lives {
  if (a.home) return "home";
  return offered && !a.here ? "offered" : "here";
}

// promotion is what the Promote dialog sends, from what's in it: the
// version, what it went into, and, for one that lives in no repo yet,
// where it's to live.
export function promotion(version: number, kind: IntoKind, text: string, lives: Lives = "home", folder = ""): ArtifactPromotion {
  const t = text.trim();
  const out: ArtifactPromotion = { version };
  if ((kind === "mission" || kind === "document") && t) out.into = t;
  if (kind === "start" && t) out.start = t;
  if (lives === "here") out.here = true;
  if ((lives === "offered" || lives === "other") && folder.trim()) out.home = folder.trim();
  return out;
}

// promoteBlocked says why the Promote dialog can't promote yet, as it's
// filled in: "" when it can.
export function promoteBlocked(kind: IntoKind, text: string, lives: Lives = "home", folder = ""): string {
  if (kind !== "nothing" && !text.trim()) return kind === "start" ? "Describe the mission." : kind === "mission" ? "Choose the mission." : "Name the document, as repo:path.";
  if (lives === "other" && !/^[^:\s]+:[^:]+$/.test(folder.trim())) return "Name the folder, as repo:folder/.";
  if (kind === "document" && lives === "here") return "A document can only cite it from a repo: choose a folder in one.";
  return "";
}

// A folder of the artifacts ws found in the workspace's repos
// (Found.tsx), as the list's tree has them: a repo, at the top, or a
// folder its artifacts are grouped in.
export interface FoundFolder {
  // What it's called: the repo as ws calls it, at the top, or the
  // folder's own name.
  name: string;
  // The repo it's in, and where it is under where promoted artifacts live
  // there ("" for the repo itself, boards, 2026/q4).
  repo: string;
  path: string;
  // The artifacts straight in it, and the folders in it.
  found: ArtifactFound[];
  folders: FoundFolder[];
  // How many artifacts are in it, and in the folders in it.
  count: number;
}

// foundTree lays them out as they're organised in their repos: a tree for
// each repo, in order of what ws calls it, of the folders its artifacts
// are grouped in, by name. A folder that holds only folders is in it too.
// The artifacts in each keep the order they came in.
export function foundTree(found: ArtifactFound[]): FoundFolder[] {
  const repos = new Map<string, FoundFolder>();
  // The folder of that name in one, made where there's none yet.
  const within = (at: FoundFolder, part: string): FoundFolder => {
    const there = at.folders.find((f) => f.name === part);
    if (there) return there;
    const made: FoundFolder = { name: part, repo: at.repo, path: at.path ? `${at.path}/${part}` : part, found: [], folders: [], count: 0 };
    at.folders.push(made);
    return made;
  };
  for (const a of found) {
    let at: FoundFolder = repos.get(a.repo) ?? { name: a.label || a.repo, repo: a.repo, path: "", found: [], folders: [], count: 0 };
    repos.set(a.repo, at);
    at.count++;
    for (const part of (a.group ?? "").split("/").filter(Boolean)) {
      at = within(at, part);
      at.count++;
    }
    at.found.push(a);
  }
  const sorted = (folders: FoundFolder[]): FoundFolder[] => {
    for (const f of folders) sorted(f.folders);
    return folders.sort((a, b) => a.name.localeCompare(b.name) || a.repo.localeCompare(b.repo));
  };
  return sorted([...repos.values()]);
}
