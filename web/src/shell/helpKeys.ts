import { artifactKeys as artifactViewKeys, artifactsKeys, compareKeys, foundArtifactKeys } from "../artifacts/keys";
import { documentsKeys, spaceKeys } from "../documents/keys";
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
  if (path.startsWith("/reviews/")) return [...prReviewKeys, ...reviewKeys.filter(([k]) => k !== "esc"), ["esc", "Clear selection, or back to Reviews"]];
  if (path.includes("/review/")) return [...reviewKeys.filter(([k]) => k !== "esc"), ["L", "Land on default branch (repos without pull requests)"], ...reviewKeys.filter(([k]) => k === "esc")];
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
  ["mod+K", "Find anything"],
  ["mod+P", "Find a file"],
  ["mod+shift+F", "Search in files (⌫ for all repos)"],
  ["g m", "Go to Missions"],
  ["g c", "Go to Conversations"],
  ["g d / g p", "Go to Documents"],
  ["g a", "Go to Artifacts"],
  ["g r", "Go to Reviews"],
  ["g t", "Go to Team"],
  ["g f", "Go to Files"],
  ["g o", "Go to Orchestrator"],
  [".", "Pin or unpin in Focus"],
  ["z", "Peek at what's under the pointer; again to close"],
  ["C", "Start a conversation beside the page"],
  ["\\", "Open or close side panel"],
  ["?", "Keyboard shortcuts"],
  ["esc", "Close, or go back"],
];
