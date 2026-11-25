import { Banner } from "../ui/Banner";
import { Button } from "../ui/Button";
import { Dialog, DialogActions, DialogText } from "../ui/Dialog";
import { Keys } from "../ui/Keys";
import { TextArea } from "../ui/TextArea";
import css from "./ArtifactDiscard.module.css";

type ArtifactDiscardProps = {
  // The artifact's title, and its id.
  title: string;
  id: string;
  // Where it lives in a repo, which it's taken out of too, and the branch
  // that commit lands on (none known: it's said as the default branch).
  home?: { repo: string; where: string };
  branch?: string;
  // Why, as it's typed.
  why: string;
  onWhy: (why: string) => void;
  error?: string;
  busy: boolean;
  onGo: () => void;
  onClose: () => void;
};

// ArtifactDiscard is the dialog that discards an artifact: what that
// does, and a box to say why (which can be left empty).
export function ArtifactDiscard({ title, id, home, branch, why, onWhy, error, busy, onGo, onClose }: ArtifactDiscardProps) {
  return (
    <Dialog title={`Discard “${title}”`} onClose={onClose}>
      <DialogText>
        It's taken off the list, with its versions. Its folder moves to <span className="mono">.ws/artifacts/.discarded</span>, and is deleted for good in 30 days: until then, <em>Restore</em> puts it back.{" "}
        {home && (
          <>
            Its folder in {home.repo} (<span className="mono">{home.where}</span>) is taken away too, in one commit straight to {home.repo}'s {branch ?? "default branch"}.{" "}
          </>
        )}
        {id} still says what it was.
      </DialogText>
      <TextArea
        className={css.why}
        autoFocus
        rows={3}
        value={why}
        placeholder="Why? (you can leave it empty)"
        onChange={(e) => onWhy(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
            e.preventDefault();
            onGo();
          }
        }}
        aria-label="Why it's discarded"
      />
      {error && <Banner>{error}</Banner>}
      <DialogActions>
        <Button variant="ghost" onClick={onClose} disabled={busy}>
          Cancel
        </Button>
        <Button variant="danger" onClick={onGo} disabled={busy}>
          {busy ? (home ? "Landing…" : "Discarding…") : home ? `Discard, and land on ${branch ?? "it"}` : "Discard"} <Keys k="mod+⏎" />
        </Button>
      </DialogActions>
    </Dialog>
  );
}
