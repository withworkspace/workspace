import { artifactKeys as artifactViewKeys, artifactsKeys, compareKeys, foundArtifactKeys } from "../artifacts/keys";
import { documentsKeys, spaceKeys } from "../documents/Documents";
import { docKeys } from "../files/DocPage";
import { explorerKeys } from "../files/Explorer";
import { filesKeys } from "../files/Files";
import { artifactKeys } from "../mission/Artifact/words";
import { crewKeys } from "../mission/Crew/CrewPage";
import { listKeys } from "../mission/List/words";
import { missionKeys } from "../mission/Mission";
import { pageKeys } from "../pages/Pages";
import { reviewKeys } from "../review/Review";
import { prReviewKeys } from "../reviews/PRReview";
import { reviewsKeys } from "../reviews/Reviews";
import { settingsKeys } from "../settings/Settings";
import { talkKeys, talksKeys } from "../talk/words";
import { teamKeys } from "../team/Team";

// helpKeys are the keys for the page at path.
export function helpKeys(path: string): [string, string][] {
  if (path === "/files" || path.startsWith("/files/")) return filesKeys;
  if (path === "/documents") return documentsKeys;
  if (path === "/artifacts") return artifactsKeys;
  if (path === "/artifacts/compare" || /^\/artifacts\/[^/]+\/compare$/.test(path)) return compareKeys;
  if (path.startsWith("/artifacts/in/")) return foundArtifactKeys;
  if (path.startsWith("/artifacts/")) return artifactViewKeys;
  if (path.startsWith("/documents/local/")) return pageKeys;
  if (/^\/documents\/[^/]+\/.+/.test(path)) return new URLSearchParams(window.location.search).has("edit") ? pageKeys : docKeys;
  if (path.startsWith("/documents/")) return spaceKeys;
  if (path === "/reviews") return reviewsKeys;
  if (path === "/team") return teamKeys;
  if (path === "/settings" || path.startsWith("/settings/")) return settingsKeys;
  if (path.startsWith("/reviews/")) return [...prReviewKeys, ...reviewKeys.filter(([k]) => k !== "esc"), ["esc", "Clear the selection, or back to Reviews"]];
  if (path.includes("/review/")) return [...reviewKeys.filter(([k]) => k !== "esc"), ["L", "Land it on the default branch (a repo with no pull requests)"], ...reviewKeys.filter(([k]) => k === "esc")];
  if (path.includes("/files/")) return explorerKeys;
  if (path.endsWith("/artifact")) return artifactKeys;
  if (path.includes("/crew/")) return crewKeys;
  if (path.includes("/conversations/")) return talkKeys;
  if (path === "/conversations") return talksKeys;
  if (path.startsWith("/pages/")) return pageKeys;
  if (path.startsWith("/missions/")) return missionKeys;
  if (path === "/") return listKeys;
  return [];
}

// globalKeys are the keys everywhere, the Keys dialog's second group.
export const globalKeys: [string, string][] = [
  ["mod+K", "Find anything: a mission, a file, a pull request, an address"],
  ["mod+P", "Find a file"],
  ["mod+shift+F", "Find text in the files: the repo you're in (⌫ for everywhere), or a capsule's own"],
  ["g m", "Go to Missions"],
  ["g c", "Go to Conversations"],
  ["g d / g p", "Go to Documents"],
  ["g a", "Go to Artifacts"],
  ["g r", "Go to Reviews"],
  ["g t", "Go to Team"],
  ["g f", "Go to Files"],
  ["g o", "Go to the Orchestrator"],
  [".", "Pin what the page is to your Focus, or unpin it"],
  ["z", "Peek at what the pointer's on (or the row the cursor's on) beside the page: a mission, a document, a pull request, a conversation. Again closes it"],
  ["C", "Begin a conversation beside the page: it's one once you've said something"],
  ["\\", "Open or close what's beside the page: what you asked about it, what you peeked at"],
  ["?", "These keys"],
  ["esc", "Close, or go back"],
];
