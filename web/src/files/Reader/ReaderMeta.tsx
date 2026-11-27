import { docKeys } from "../keys";
import { keyOf, useKeymap } from "../../keymap/keymap";
import type { ReactNode, Ref } from "react";
import { Link } from "react-router";
import { Button, buttonClass } from "../../ui/Button";
import { Chip, chipClass } from "../../ui/Chip";
import { Copyable } from "../../ui/Copyable";
import { Icon } from "../../ui/Icon";
import { keyProps } from "../../ui/keyHint";
import { Keys } from "../../ui/Keys";
import css from "./ReaderMeta.module.css";
import { textClass } from "../../ui/Text";

type ReaderMetaProps = {
  // What kind of document it is (an ADR, a proposal), and what that
  // means; a plain Document without one.
  kind?: { title: string; hint?: string };
  repo: string;
  // Which of its repo's worktrees it's read from: the ground, or a capsule.
  worktree: string;
  ground: boolean;
  // The page it's edited in, and where that is.
  page?: { name: string; to: string };
  // The capsules it's changed in: read it as it is there.
  flights: { capsule: string; pr?: number; to: string }[];
  docRef: string;
  // Pin it (one on the default branch).
  pin?: ReactNode;
  // Its contents, where they're folded into a menu: the button that opens
  // them, and whether they're open.
  contents?: { open: boolean; ref?: Ref<HTMLButtonElement>; onToggle: () => void };
  onAsk: () => void;
  onEdit: () => void;
  // Why it can't be edited here, if it can't.
  editUnavailable?: string;
  filesTo: string;
};

// ReaderMeta is what's said of a document under its title: its kind,
// where it's read from, where it's edited and changed, its ref, and what's
// done with it (its contents, ask, edit, open in Files).
export function ReaderMeta({ kind, repo, worktree, ground, page, flights, docRef, pin, contents, onAsk, onEdit, editUnavailable, filesTo }: ReaderMetaProps) {
  const map = useKeymap();
  return (
    <div className={css.meta}>
      <span className={css.kind} title={kind?.hint}>
        {kind ? kind.title : "Document"}
      </span>
      <Chip tone={ground ? "done" : "quiet"} title={ground ? "Default branch version" : `Version in capsule ${repo}/${worktree}`}>
        <Icon name={ground ? "layers" : "box"} size={11} /> {ground ? "On the default branch" : `In ${worktree}`}
      </Chip>
      {page && (
        <Link to={page.to} className={textClass("dim")} title="Open page">
          Edited in {page.name}
        </Link>
      )}
      {flights.map((f) => (
        <Link key={f.capsule} to={f.to} className={chipClass("progress")} title={`Changed in capsule ${f.capsule}${f.pr ? `, in pull request #${f.pr}` : ""}: open that version`}>
          <Icon name="box" size={11} /> In flight in {f.capsule}
          {f.pr ? ` · #${f.pr}` : ""}
        </Link>
      ))}
      <Copyable text={docRef} className={css.ref} title="Copy ref">
        {docRef.replace(":", " · ")}
      </Copyable>
      <span className={css.actions}>
        {pin}
        {contents && (
          <Button size="small" ref={contents.ref} data-contents-toggle aria-expanded={contents.open} onClick={contents.onToggle} title="Contents" keys={docKeys.contents}>
            <Icon name="list" size={12} /> Contents <Keys k={docKeys.contents} />
          </Button>
        )}
        <Button size="small" className={css.ask} onMouseDown={(e) => e.preventDefault()} onClick={onAsk} title="Ask about document or selection" keys={docKeys.ask}>
          <Icon name="sparkle" size={12} /> Ask <Keys k={docKeys.ask} />
        </Button>
        <Button size="small" onClick={onEdit} unavailable={editUnavailable} title="Edit as page" keys={docKeys.edit}>
          <Icon name="pencil" size={12} /> Edit <Keys k={docKeys.edit} />
        </Button>
        <Link to={filesTo} className={buttonClass({ size: "small" })} {...keyProps(keyOf(docKeys.files, map), "Open in Files")}>
          <Icon name="file" size={12} /> Files <Keys k={docKeys.files} />
        </Link>
      </span>
    </div>
  );
}
