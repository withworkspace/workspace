import { useState } from "react";
import { shareRepo } from "../api";
import { toast } from "../ui/toast";
import { SettingsRepoShare } from "./SettingsRepoShare";

// RepoShare gives a repo of yours to the team, asked first: it's a commit
// on the workspace's config repo, pushed, which everyone's next sync
// brings.
export function RepoShare({ repo, done }: { repo: string; done: () => void }) {
  const [asking, setAsking] = useState(false);
  const [busy, setBusy] = useState(false);
  const share = () => {
    setBusy(true);
    shareRepo(repo)
      .then((r) => {
        toast(r.message);
        done();
      })
      .catch((e: Error) => toast(e.message, { tone: "danger", ms: 8000 }))
      .finally(() => {
        setBusy(false);
        setAsking(false);
      });
  };
  return <SettingsRepoShare repo={repo} asking={asking} busy={busy} onAsk={() => setAsking(true)} onShare={share} onCancel={() => setAsking(false)} />;
}
