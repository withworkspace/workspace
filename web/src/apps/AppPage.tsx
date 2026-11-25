import { useParams } from "react-router";
import { ErrorCard } from "../ui/ErrorCard";
import { useTitle } from "../ui/useTitle";
import { useWorkspace } from "../workspace/store";
import { AppFrame } from "./AppFrame";
import { AppView } from "./AppView";

// An app of the workspace's own (ws.toml) has a page of its own, with the
// sidebar folded to a rail beside it (room.ts): its bar and card
// (AppFrame), filling the page. A repo's (ws.repo.toml) is a tab of each
// of its worktrees, on the repo's page.
export function AppPage() {
  const workspace = useWorkspace();
  const { name = "" } = useParams();
  const entry = workspace.apps?.find((a) => a.name === name);
  useTitle(entry?.title ?? name);
  if (!entry) {
    return (
      <div className="page">
        <ErrorCard title="No such app" error={`The workspace has no app called ${name}: an app is a service in ws.toml with a page ([run.service.${name}] page = { title = "…" }).`} />
      </div>
    );
  }
  return (
    <AppView>
      <AppFrame name={name} title={entry.title} icon={entry.icon} render={entry.render} />
    </AppView>
  );
}
