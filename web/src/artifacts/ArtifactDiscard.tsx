import { Banner } from "../ui/Banner";
import { Button } from "../ui/Button";
import { Dialog, DialogActions, DialogText } from "../ui/Dialog";
import { Keys } from "../ui/Keys";
import css from "./ArtifactDiscard.module.css";
import { Text } from "../ui/Text";
import { RichTextArea } from "../ui/RichTextArea";

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
        Removed from list, with versions. Folder moves to <Text mono>.ws/artifacts/.discarded</Text>, and is
        deleted after 30 days: until then, <em>Restore</em> puts it back.{" "}
        {home && (
          <>
            Folder in {home.repo} (<Text mono>{home.where}</Text>) is taken away too, commits to {home.repo}'s{" "}
            {branch ?? "default branch"}.{" "}
          </>
        )}
        {id} still says what it was.
      </DialogText>
      <RichTextArea
        marks
        lists
        quote
        code
        refs
        look="field"
        className={css.why}
        autoFocus
        rows={3}
        value={why}
        placeholder="Why? (optional)"
        onChange={onWhy}
        send="mod+enter"
        onSend={onGo}
        aria-label="Reason"
      />
      {error && <Banner>{error}</Banner>}
      <DialogActions>
        <Button variant="ghost" onClick={onClose} disabled={busy} keys="esc">
          Cancel
        </Button>
        <Button variant="danger" onClick={onGo} disabled={busy} keys="mod+⏎">
          {busy ? (home ? "Landing…" : "Discarding…") : home ? `Discard, and land on ${branch ?? "it"}` : "Discard"} <Keys k="mod+⏎" />
        </Button>
      </DialogActions>
    </Dialog>
  );
}
