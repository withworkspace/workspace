import { Button } from "../ui/Button";
import css from "./SettingsRepoShare.module.css";

type SettingsRepoShareProps = {
  repo: string;
  // Asked: it's about to be given, and says what that does.
  asking: boolean;
  busy?: boolean;
  onAsk: () => void;
  onShare: () => void;
  onCancel: () => void;
};

// SettingsRepoShare gives a repo of yours to the team, asked first: it's
// a commit on the workspace's config repo, pushed.
export function SettingsRepoShare({ repo, asking, busy, onAsk, onShare, onCancel }: SettingsRepoShareProps) {
  if (!asking)
    return (
      <Button variant="ghost" size="small" onClick={onAsk}>
        Share with the team
      </Button>
    );
  return (
    <span className={css.ask}>
      <Button variant="primary" size="small" disabled={busy} onClick={onShare} title={`Adds ${repo} to ws.toml, commits it on the workspace's config repo and pushes it`}>
        {busy ? "Pushing…" : "Commit and push"}
      </Button>
      <Button variant="ghost" size="small" disabled={busy} onClick={onCancel}>
        Cancel
      </Button>
    </span>
  );
}
