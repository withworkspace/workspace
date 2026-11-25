import { memo } from "react";
import { Navigate, Route, Routes } from "react-router";
import { AppPage } from "../apps/AppPage";
import { ArtifactPage as ArtifactView, ArtifactsPage } from "../artifacts/Artifacts";
import { ComparePage } from "../artifacts/Compare";
import { FoundArtifactPage } from "../artifacts/Found";
import { NotFound } from "../chrome/NotFound";
import { DocsAddress, DocumentAt, LocalPageAt, OpenAddress, PageAddress } from "../documents/DocumentAt";
import { DocumentsPage, SpacePage } from "../documents/Documents";
import { Explorer } from "../files/Explorer";
import { FilesPage } from "../files/Files";
import { ArtifactPage } from "../mission/Artifact/ArtifactPage";
import { CrewPage } from "../mission/Crew/CrewPage";
import { Missions } from "../mission/List/Missions";
import { MissionPage } from "../mission/Mission";
import { NotificationsPage } from "../notifications/NotificationsPage";
import { OrchestratorPage } from "../orchestrator/OrchestratorPage";
import { OrchestratorRunPage } from "../orchestrator/OrchestratorRunPage";
import { PluginPage } from "../plugins/PluginPage";
import { Review } from "../review/Review";
import { PRReviewPage } from "../reviews/PRReview";
import { ReviewsPage } from "../reviews/Reviews";
import { RunsPage } from "../runs/Runs";
import { SettingsPage } from "../settings/Settings";
import { SiloPoint } from "../silo/SiloPoint";
import { TalkPage, TalksPage } from "../talk/Talk";
import { TeamPage } from "../team/Team";
import { useWorkspace } from "../workspace/store";
import type { useNotifications } from "./notify";
import { ToBrief } from "./ToBrief";

// Pages is the page at the address. It's given nothing of the workspace
// (each page reads what it needs from the store), so it's drawn again on
// going somewhere, and not each time the shell round it is.
export const Pages = memo(function Pages({ notify }: { notify: ReturnType<typeof useNotifications> }) {
  const missions = useWorkspace((w) => w.missions);
  return (
    <Routes>
      <Route path="/" element={<Missions missions={missions} />} />
      <Route path="/missions/:slug" element={<MissionPage />} />
      <Route path="/missions/:slug/artifact" element={<ArtifactPage />} />
      <Route path="/missions/:slug/brief" element={<ToBrief />} />
      <Route path="/missions/:slug/crew/:id" element={<CrewPage />} />
      <Route path="/missions/:slug/orchestrator/:id" element={<OrchestratorRunPage />} />
      <Route path="/orchestrator" element={<OrchestratorPage />} />
      <Route path="/runs" element={<RunsPage />} />
      <Route path="/notifications" element={<NotificationsPage />} />
      <Route path="/apps/:name" element={<AppPage />} />
      <Route path="/plugins" element={<Navigate to="/settings/plugins" replace />} />
      <Route path="/settings" element={<SettingsPage notify={notify} />} />
      <Route path="/settings/:section" element={<SettingsPage notify={notify} />} />
      <Route path="/docs" element={<DocsAddress />} />
      <Route path="/files" element={<FilesPage />} />
      <Route path="/files/:repo" element={<FilesPage />} />
      <Route path="/files/:repo/:capsule" element={<FilesPage />} />
      <Route path="/reviews" element={<ReviewsPage />} />
      <Route path="/reviews/:repo/:number" element={<PRReviewPage />} />
      <Route path="/team" element={<TeamPage />} />
      <Route path="/silo/:repo" element={<SiloPoint />} />
      <Route path="/documents" element={<DocumentsPage />} />
      <Route path="/documents/:repo" element={<SpacePage />} />
      <Route path="/documents/local/:id" element={<LocalPageAt />} />
      <Route path="/documents/:repo/*" element={<DocumentAt />} />
      <Route path="/pages" element={<Navigate to="/documents" replace />} />
      <Route path="/pages/open" element={<OpenAddress />} />
      <Route path="/pages/:id" element={<PageAddress />} />
      <Route path="/artifacts" element={<ArtifactsPage />} />
      <Route path="/artifacts/compare" element={<ComparePage />} />
      <Route path="/artifacts/in/*" element={<FoundArtifactPage />} />
      <Route path="/artifacts/:id" element={<ArtifactView />} />
      <Route path="/artifacts/:id/compare" element={<ComparePage />} />
      <Route path="/artifacts/:id/:version" element={<ArtifactView />} />
      <Route path="/conversations" element={<TalksPage />} />
      <Route path="/conversations/:id" element={<TalkPage />} />
      <Route path="/missions/:slug/conversations/:id" element={<TalkPage />} />
      <Route path="/missions/:slug/review/:repo/:name" element={<Review />} />
      <Route path="/missions/:slug/files/:repo/:name" element={<Explorer />} />
      <Route path="/x/:name/*" element={<PluginPage />} />
      <Route path="*" element={<NotFound />} />
    </Routes>
  );
});
