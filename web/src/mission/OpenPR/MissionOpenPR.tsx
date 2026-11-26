import { Banner } from "../../ui/Banner";
import { Button } from "../../ui/Button";
import { Checkbox } from "../../ui/Checkbox";
import { cx } from "../../ui/cx";
import { Dialog, DialogActions } from "../../ui/Dialog";
import { Icon } from "../../ui/Icon";
import { Input } from "../../ui/Input";
import { Spinner } from "../../ui/Spinner";
import { humanError } from "../../ui/strings";
import { TextArea } from "../../ui/TextArea";
import css from "./MissionOpenPR.module.css";

type MissionOpenPRProps = {
  // The capsule it's opened from, repo/name.
  name: string;
  // The step of the plan that opens it, if one does.
  step?: number;
  title: string;
  onTitle: (title: string) => void;
  body: string;
  onBody: (body: string) => void;
  draft: boolean;
  onDraft: (draft: boolean) => void;
  // The PR writer writing its title and description.
  writing?: boolean;
  onWrite: () => void;
  busy?: boolean;
  error?: string;
  onClose: () => void;
  onOpen: () => void;
};

// MissionOpenPR asks what a pull request opened from a capsule says: its
// title and description (yours, or the PR writer's, written here or once
// it's opened), and whether it's a draft. ws pushes the branch and opens
// it, holding the process's rules about pull requests.
export function MissionOpenPR({ name, step, title, onTitle, body, onBody, draft, onDraft, writing, onWrite, busy, error, onClose, onOpen }: MissionOpenPRProps) {
  return (
    <Dialog title={`Open a pull request from ${name}`} onClose={onClose} className={css.dialog} wide>
      <p>
        ws pushes its branch and opens the pull request onto the default branch, holding the process's rules about pull requests: one that wants your
        review first refuses it until you've approved the capsule as it is.
        {step !== undefined && ` Step ${step} of the plan opens it.`} Its title and description are the PR writer's to write, from the step it's for and the workspace's PR.md, unless you give them.
      </p>
      <div className={css.form}>
        <div className={css.title}>
          <Input
            value={title}
            autoFocus
            aria-label="Title"
            placeholder="Its title, or leave it to the PR writer"
            disabled={writing}
            onChange={(e) => onTitle(e.target.value)}
          />
          <Button variant="ghost" size="small" onClick={onWrite} disabled={writing || busy} title="Have the PR writer write its title and description, from the step it's for and PR.md">
            {writing ? <Spinner size={12} /> : <Icon name="pencil" size={12} />} {writing ? "Writing…" : "Write it"}
          </Button>
        </div>
        <TextArea
          value={body}
          rows={8}
          aria-label="Description"
          placeholder="What it does, and why"
          disabled={writing}
          onChange={(e) => onBody(e.target.value)}
        />
      </div>
      <Checkbox className={cx(css.toggle, css.draft)} checked={draft} onChange={onDraft}>
        Open it as a draft
      </Checkbox>
      {error && <Banner>{humanError(error)}</Banner>}
      <DialogActions>
        <Button variant="ghost" onClick={onClose}>
          Cancel
        </Button>
        <Button
          variant="primary"
          onClick={onOpen}
          disabled={busy || writing}
          title={title.trim() ? undefined : "The PR writer writes its title and description, and ws opens it once they're written"}
        >
          {title.trim() ? "Open it" : "Write and open it"}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
