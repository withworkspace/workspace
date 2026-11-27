import type { KeyboardEvent, Ref } from "react";
import { threadKinds, type ThreadKind } from "../api";
import { Banner } from "../ui/Banner";
import { Button } from "../ui/Button";
import { cx } from "../ui/cx";
import { Icon } from "../ui/Icon";
import { Keys } from "../ui/Keys";
import { type KeyPress, RichTextArea } from "../ui/RichTextArea";
import { Segment, Segmented } from "../ui/Segmented";
import { TextArea } from "../ui/TextArea";
import { humanError } from "../ui/strings";
import { AnnotateActions } from "./AnnotateActions";
import { kindHints, kindIcon, kindWords } from "./words";
import css from "./AnnotateComposer.module.css";
import { Text } from "../ui/Text";

const kinds: Record<ThreadKind, string> = { ask: css.ask, change: css.change, note: css.note };

// AnnotateComposer is a new comment being written on what's highlighted:
// its kind (ask, change or note), what it's on (quote, or the whole
// diagram), and what's said. edit, when it's there, is what the words
// should read instead: a suggested edit (canSuggest offers one). What's
// said is markdown, drawn as it reads (RichTextArea); what it should read
// is the document's own words, as they are. The key you've chosen posts
// what's said (onPost), Escape is onCancel, and the holder has the other
// keys first (onKeys: Tab for the kind) and the edit's (onEditKeys).
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
  onKeys?: (e: KeyPress) => boolean;
  textRef?: Ref<HTMLElement>;
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
      <Segmented fill value={kind} onChange={onKind} aria-label="Comment kind">
        {threadKinds.map((k) => (
          <Segment key={k} value={k} className={cx(css.kind, kinds[k])} title={kindWords[k]} keys="tab">
            <Icon name={kindIcon[k]} size={12} />
            {kindWords[k]}
          </Segment>
        ))}
      </Segmented>
      <blockquote className={css.on}>{quote || "The whole diagram"}</blockquote>
      <RichTextArea
        marks
        lists
        quote
        code
        refs
        look="field"
        className={css.said}
        field={textRef}
        value={text}
        onChange={onText}
        rows={2}
        tallest={260}
        placeholder={kindHints[kind]}
        aria-label={kindHints[kind]}
        onSend={onPost}
        onEscape={onCancel}
        onKeyDown={onKeys}
      />
      {edit !== undefined && (
        <>
          <Text as="div" dim>Should read:</Text>
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
          <Icon name="pencil" size={12} /> Suggest edit
        </Button>
      )}
      <AnnotateActions
        start={
          <Text dim className={css.tip}>
            <Keys k="tab" /> kind
          </Text>
        }
      >
        <Button variant="ghost" size="small" onClick={onCancel} title="Cancel" keys="esc">
          Cancel
        </Button>
        <Button variant="primary" size="small" onClick={onPost} disabled={!canPost} title="Post" keys={postKeys}>
          {edit !== undefined ? "Suggest" : "Comment"}
        </Button>
      </AnnotateActions>
    </div>
  );
}
