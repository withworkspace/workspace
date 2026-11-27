import { docAddress, type Talk } from "../api";
import { slugger } from "../markdown/links";
import type { IconName } from "../ui/Icon";
import type { StatusInfo } from "../ui/Status";

// What a conversation's pages say of it: its name, where it is, how it
// stands, its keys, and the icon each kind of proposal has.

export const talksKeys: [string, string][] = [
  ["n", "New conversation"],
  ["j / k", "Next / previous"],
  ["⏎ / o", "Open"],
  ["⇧A", "Show or hide resolved"],
];

export const talkKeys: [string, string][] = [
  ["mod+⏎", "Send"],
  ["r", "Reply"],
  ["x", "Resolve or reopen"],
  ["esc", "Back to conversations"],
];

export const talkName = (t: { title?: string; opening?: string }) => t.title || t.opening || "New conversation";

// docName is a document's file name, from its ref (records:notes/x.md).
export const docName = (ref: string) => ref.slice(ref.lastIndexOf("/") + 1).replace(/^.*:/, "");

// docAt is where a conversation about a document leads: the document, at
// the section it was asked of.
export const docAt = (ref: string, section?: string) => docAddress(ref) + (section ? `#${slugger()(section)}` : "");

// talkHref is a conversation's page: the workspace's, or a mission's.
export const talkHref = (slug: string, id: string) => (slug ? `/missions/${slug}/conversations/${id}` : `/conversations/${id}`);

export function talkStatus(t: Pick<Talk, "status" | "answering" | "waiting">): StatusInfo {
  if (t.status === "resolved") return { icon: "check-circle", tone: "quiet", words: "Resolved" };
  if (t.answering) return { icon: "spinner", tone: "progress", words: "Answering", live: true };
  if (t.waiting) return { icon: "reply", tone: "attention", words: "Your turn" };
  return { icon: "message", tone: "quiet", words: "Open" };
}

export const proposalIcons: Record<string, IconName> = { decision: "gavel", mission: "flag", brief: "pencil", dispatch: "play", replan: "rotate", comment: "message", plan: "list-ordered", document: "file", artifact: "artifact", automation: "bolt", config: "sliders" };

export const whoName = (by: string) => ({ you: "You", agent: "Agent", ws: "ws" })[by] ?? by;
