import type { Artifact } from "../api";
import { capital } from "../ui/text";

// Outputs (a mission's recorded artifacts: its documents, pull requests
// and artifacts): a short title, what the agent said of it under, and its
// kind and state as marks (MissionOutput). An artifact (a3) opens on its
// own page, /artifacts/a3.

const kindPlural: Record<string, string> = {
  proposal: "Proposals",
  adr: "Decision records",
  note: "Notes",
  doc: "Documents",
  handoff: "Handoffs",
  review: "Reviews",
  plan: "Plans",
};

export const kindTitle = (kind: string) => kindPlural[kind] ?? capital(kind) + "s";

// artifactTitle is a document's name, short: its label when it's a name,
// else made from its kind and who made it, with the label to describe it.
export function artifactTitle(a: Pick<Artifact, "kind" | "label" | "ref" | "by">): { title: string; desc?: string } {
  const label = a.label?.trim() ?? "";
  const review = /^Review (r\d+) of (.+?): (.+)$/.exec(label);
  if (review) return { title: `Review ${review[1]} of ${review[2]}`, desc: review[3][0].toUpperCase() + review[3].slice(1) };
  if (a.kind === "handoff") return { title: a.by ? `${a.by}'s handoff` : "Handoff", desc: label || undefined };
  if (label && label.length <= 64) return { title: label };
  const base = a.ref.split(/[:/]/).pop() ?? a.ref;
  return { title: label ? `${kindTitle(a.kind).replace(/s$/, "")}${a.by ? ` by ${a.by}` : ""}` : base, desc: label || undefined };
}

// isArtifact is whether a mission's output is an artifact (a3): opened on
// its own page, with its versions.
export const isArtifact = (a: Pick<Artifact, "kind" | "ref">) => a.kind === "artifact" && /^a[1-9]\d*$/.test(a.ref);

// shortPath shows a capsule's path from its repo down: the workspace's
// own folders are the same for every capsule.
export const shortPath = (p: string) => p.replace(/^.*\/repos\//, "repos/");
