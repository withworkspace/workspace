import type { KnowKind, StrandState, TeamEvent } from "../api";
import type { IconName } from "../ui/Icon";
import type { StatusInfo } from "../ui/Status";

// What the Team page says of things: where a change stands, each kind of
// thing worth knowing, and what happened to it, in words and icons.

export const periods: { since: string; label: string; title: string }[] = [
  { since: "today", label: "Today", title: "Today" },
  { since: "7d", label: "7 days", title: "The last 7 days" },
  { since: "", label: "All", title: "As far back as ws has looked" },
];

export const stateInfo: Record<StrandState, StatusInfo> = {
  "in-progress": { icon: "circle-dashed", tone: "quiet", words: "In progress" },
  "in-review": { icon: "pr", tone: "waiting", words: "In review" },
  landed: { icon: "check-circle", tone: "done", words: "Landed" },
  closed: { icon: "x-circle", tone: "quiet", words: "Closed" },
};

// What each kind of thing worth knowing is called, and drawn with.
export const knowInfo: Record<KnowKind, { label: string; icon: IconName; title: string }> = {
  contract: { label: "Contract", icon: "link", title: "Something other code calls or reads has changed shape" },
  action: { label: "To do", icon: "alert", title: "Something you may have to do" },
  convention: { label: "How we work", icon: "book", title: "How things are done here has changed" },
  removed: { label: "Removed", icon: "minus", title: "Something was taken away or deprecated" },
  behaviour: { label: "Behaviour", icon: "eye", title: "What users or callers see has changed" },
};

// fileHref is a file on its repo's default branch, in Files.
export const fileHref = (repo: string, path: string) => `/files/${encodeURIComponent(repo)}?${new URLSearchParams({ path })}`;

export function eventIcon(e: TeamEvent): IconName {
  switch (e.kind) {
    case "opened":
      return "pr";
    case "ready":
      return "eye";
    case "reviewed":
      return e.verdict === "approved" ? "check" : e.verdict === "changes" ? "alert" : "message";
    case "merged":
      return "check-circle";
    case "closed":
      return "x-circle";
    default:
      return "commit";
  }
}

export function eventWords(e: TeamEvent): string {
  switch (e.kind) {
    case "opened":
      return "opened it";
    case "ready":
      return "made it ready for review";
    case "reviewed":
      return e.verdict === "approved" ? "approved it" : e.verdict === "changes" ? "asked for changes" : "commented";
    case "merged":
      return "landed it";
    case "closed":
      return "Closed without merging";
    default:
      return `landed, straight on ${e.branch || "the default branch"}: `;
  }
}
