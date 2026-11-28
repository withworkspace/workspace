import { memo, useEffect } from "react";
import { Navigate, Route, Routes } from "react-router";
import { useWorkspace } from "../workspace/store";
import type { useNotifications } from "./notify";
import { ToBrief } from "./ToBrief";
import { lazyPage, loadPages } from "./lazyPage";

// Each page is a chunk of its own (lazyPage).
const AppPage = lazyPage(() => import("../apps/AppPage").then((m) => m.AppPage));
const ArtifactView = lazyPage(() => import("../artifacts/ArtifactPage").then((m) => m.ArtifactPage));
const ArtifactsPage = lazyPage(() => import("../artifacts/ArtifactsPage").then((m) => m.ArtifactsPage));
const ComparePage = lazyPage(() => import("../artifacts/ComparePage").then((m) => m.ComparePage));
const FoundArtifactPage = lazyPage(() => import("../artifacts/FoundArtifactPage").then((m) => m.FoundArtifactPage));
const NotFound = lazyPage(() => import("../chrome/NotFound").then((m) => m.NotFound));
const DocsAddress = lazyPage(() => import("../documents/DocumentAt").then((m) => m.DocsAddress));
const DocumentAt = lazyPage(() => import("../documents/DocumentAt").then((m) => m.DocumentAt));
const LocalPageAt = lazyPage(() => import("../documents/DocumentAt").then((m) => m.LocalPageAt));
const OpenAddress = lazyPage(() => import("../documents/DocumentAt").then((m) => m.OpenAddress));
const PageAddress = lazyPage(() => import("../documents/DocumentAt").then((m) => m.PageAddress));
const DocumentsPage = lazyPage(() => import("../documents/Documents").then((m) => m.DocumentsPage));
const SpacePage = lazyPage(() => import("../documents/SpacePage").then((m) => m.SpacePage));
const Explorer = lazyPage(() => import("../files/Explorer").then((m) => m.Explorer));
const FilesPage = lazyPage(() => import("../files/Files").then((m) => m.FilesPage));
const ArtifactPage = lazyPage(() => import("../mission/Artifact/ArtifactPage").then((m) => m.ArtifactPage));
const CrewPage = lazyPage(() => import("../mission/Crew/CrewPage").then((m) => m.CrewPage));
const Missions = lazyPage(() => import("../mission/List/Missions").then((m) => m.Missions));
const MissionPage = lazyPage(() => import("../mission/Mission").then((m) => m.MissionPage));
const NotificationsPage = lazyPage(() => import("../notifications/NotificationsPage").then((m) => m.NotificationsPage));
const OrchestratorPage = lazyPage(() => import("../orchestrator/OrchestratorPage").then((m) => m.OrchestratorPage));
const OrchestratorRunPage = lazyPage(() => import("../orchestrator/OrchestratorRunPage").then((m) => m.OrchestratorRunPage));
const PluginPage = lazyPage(() => import("../plugins/PluginPage").then((m) => m.PluginPage));
const Review = lazyPage(() => import("../review/Review").then((m) => m.Review));
const PRReviewPage = lazyPage(() => import("../reviews/PRReview").then((m) => m.PRReviewPage));
const ReviewsPage = lazyPage(() => import("../reviews/Reviews").then((m) => m.ReviewsPage));
const LessonsPage = lazyPage(() => import("../lessons/LessonsPage").then((m) => m.LessonsPage));
const RunsPage = lazyPage(() => import("../runs/Runs").then((m) => m.RunsPage));
const AutomationOutputPage = lazyPage(() => import("../automations/AutomationOutputPage").then((m) => m.AutomationOutputPage));
const AutomationPage = lazyPage(() => import("../automations/AutomationPage").then((m) => m.AutomationPage));
const AutomationsPage = lazyPage(() => import("../automations/AutomationsPage").then((m) => m.AutomationsPage));
const SettingsPage = lazyPage(() => import("../settings/Settings").then((m) => m.SettingsPage));
const SiloPoint = lazyPage(() => import("../silo/SiloPoint").then((m) => m.SiloPoint));
const TalkPage = lazyPage(() => import("../talk/TalkPage").then((m) => m.TalkPage));
const TalksPage = lazyPage(() => import("../talk/TalksPage").then((m) => m.TalksPage));
const TeamPage = lazyPage(() => import("../team/Team").then((m) => m.TeamPage));

// Pages is the page at the address. It's given nothing of the workspace
// (each page reads what it needs from the store), so it's drawn again on
// going somewhere, and not each time the shell round it is.
export const Pages = memo(function Pages({ notify }: { notify: ReturnType<typeof useNotifications> }) {
  const missions = useWorkspace((w) => w.missions);
  // Shown, with the page at the address, the rest can load.
  useEffect(loadPages, []);
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
      <Route path="/lessons" element={<LessonsPage />} />
      <Route path="/automations" element={<AutomationsPage />} />
      <Route path="/automations/:name" element={<AutomationPage />} />
      <Route path="/automations/:name/output" element={<AutomationOutputPage />} />
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
