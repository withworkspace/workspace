import type { Headline } from "../api";

// shownHeadlines is the headlines a mission's page shows, in the order
// they came: every one of yours and of outside's, and of the agents', one:
// the first. However many agents are on it (crew at work, the planner
// answering, the orchestrator looking), one banner says it's in hand.
//
// Which that is isn't worked out here: ws does (mission.Resolve keeps the
// most pressing, for ws mission show too), so given what ws gives, this
// changes nothing. It holds the page to one banner whatever it's given.
export function shownHeadlines(hs: Headline[]): Headline[] {
  let agent = false;
  return hs.filter((h) => {
    if (h.court !== "agent") return true;
    if (agent) return false;
    agent = true;
    return true;
  });
}
