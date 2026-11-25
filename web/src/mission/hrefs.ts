import type { MissionAct, MissionCard, MissionDetail } from "../api";
import { talkHref } from "../talk/words";
import { briefHref } from "./Brief/href";
import { bundleLink } from "./Work/words";

// Where a mission's page leads, in ws.

// docLink is a mission's document's page, to read and comment on.
export const docLink = (slug: string, ref: string) => `/missions/${slug}/artifact?${new URLSearchParams({ ref })}`;

// cameFrom is the conversation a mission came out of: the workspace's (w3),
// or another mission's (cache/t4), on its page.
export const cameFrom = (ref: string) => {
  const [slug, id] = ref.includes("/") ? ref.split("/") : ["", ref];
  return talkHref(slug, id);
};

// actHref is where an act that opens something goes, in ws.
export function actHref(d: Pick<MissionDetail, "slug" | "planner">, c: Pick<MissionCard, "kind">, a: Pick<MissionAct, "do" | "target">): string | undefined {
  const t = a.target ?? "";
  switch (a.do) {
    case "crew":
      return `/missions/${d.slug}/crew/${t}`;
    case "review": {
      const [repo, name] = t.split("/");
      return `/missions/${d.slug}/review/${repo}/${name}`;
    }
    case "verify":
      return bundleLink(d.slug, t);
    case "read":
      return docLink(d.slug, t);
    case "talk":
      // Its planner's conversation is the brief's own page's: what it
      // proposes for the brief and plan is read and confirmed there. What
      // else a conversation proposed is confirmed in the conversation.
      return t && t === d.planner && c.kind !== "proposed" ? briefHref(d.slug) : `/missions/${d.slug}/conversations/${t}`;
    case "brief-page":
      return briefHref(d.slug);
    case "log":
      return `/missions/${d.slug}?tab=log`;
  }
  return undefined;
}
