import type { KeyboardEvent, Ref } from "react";
import { threadKinds, type ThreadKind } from "../api";
import { Banner } from "../ui/Banner";
import { Button } from "../ui/Button";
import { cx } from "../ui/cx";
import { Icon } from "../ui/Icon";
import { Keys } from "../ui/Keys";
import { Segment, Segmented } from "../ui/Segmented";
import { TextArea } from "../ui/TextArea";
import { humanError } from "../ui/text";
import { AnnotateActions } from "./AnnotateActions";
import { kindHints, kindIcon, kindWords } from "./words";
import css from "./AnnotateComposer.module.css";

const kinds: Record<ThreadKind, string> = { ask: css.ask, change: css.change, note: css.note };

// AnnotateComposer is a new comment being written on what's highlighted:
// its kind (ask, change or note), what it's on (quote, or the whole
// diagram), and what's said. edit, when it's there, is what the words
// should read instead: a suggested edit (canSuggest offers one). The
// keys in the boxes are the holder's (onKeys, onEditKeys).
export function AnnotateComposer({
  kind,
  onKind,
  quote,
  text,
  onText,
  onKeys,
  textRef,
  edit,
  onEdit,
  onEditKeys,
  error,
  canSuggest,
  onSuggest,
  canPost,
  onPost,
  postKeys,
  onCancel,
}: {
  kind: ThreadKind;
  onKind: (kind: ThreadKind) => void;
  quote?: string;
  text: string;
  onText: (text: string) => void;
  onKeys?: (e: KeyboardEvent<HTMLTextAreaElement>) => void;
  textRef?: Ref<HTMLTextAreaElement>;
  edit?: string;
  onEdit?: (edit: string) => void;
  onEditKeys?: (e: KeyboardEvent<HTMLTextAreaElement>) => void;
  error?: string;
  canSuggest?: boolean;
  onSuggest?: () => void;
  canPost: boolean;
  onPost: () => void;
  // The keys that post it, as Keys reads them (mod+⏎, or ⏎).
  postKeys: string;
  onCancel: () => void;
}) {
  return (
    <div className={css.composer} onClick={(e) => e.stopPropagation()}>
      <Segmented fill value={kind} onChange={onKind} aria-label="What kind of comment">
        {threadKinds.map((k) => (
          <Segment key={k} value={k} className={cx(css.kind, kinds[k])} title={`${kindWords[k]} (tab cycles)`}>
            <Icon name={kindIcon[k]} size={12} />
            {kindWords[k]}
          </Segment>
        ))}
      </Segmented>
      <blockquote className={css.on}>{quote || "The whole diagram"}</blockquote>
      <TextArea ref={textRef} value={text} rows={2} placeholder={kindHints[kind]} aria-label={kindHints[kind]} onChange={(e) => onText(e.target.value)} onKeyDown={onKeys} />
      {edit !== undefined && (
        <>
          <div className="dim">It should read:</div>
          <TextArea
            className={css.editTo}
            value={edit}
            rows={Math.min(10, Math.max(2, edit.split("\n").length + 1))}
            onChange={(e) => onEdit?.(e.target.value)}
            onKeyDown={onEditKeys}
          />
        </>
      )}
      {error && <Banner>{humanError(error)}</Banner>}
      {canSuggest && edit === undefined && (
        <Button variant="link" className={css.suggest} onClick={onSuggest}>
          <Icon name="pencil" size={12} /> Suggest an edit instead
        </Button>
      )}
      <AnnotateActions
        start={
          <span className={cx("dim", css.tip)}>
            <Keys k="tab" /> kind
          </span>
        }
      >
        <Button variant="ghost" size="small" onClick={onCancel} title="Cancel" keys="esc">
          Cancel
        </Button>
        <Button variant="primary" size="small" onClick={onPost} disabled={!canPost} title="Post it" keys={postKeys}>
          {edit !== undefined ? "Suggest" : "Comment"}
        </Button>
      </AnnotateActions>
    </div>
  );
}
