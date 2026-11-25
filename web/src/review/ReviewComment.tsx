import type { ReviewNote } from "../api";
import { Button } from "../ui/Button";
import { cx } from "../ui/cx";
import { Excerpt } from "../ui/Excerpt";
import { Icon } from "../ui/Icon";
import { Inline } from "../ui/Inline";
import { Keys } from "../ui/Keys";
import { NoteBody, NoteCard, NoteHead, NoteHint, NoteId, NoteStatus, NoteThread, NoteTurn } from "../ui/NoteCard";
import { ReplyBox } from "../ui/ReplyBox";
import { capital } from "../ui/text";
import { followWords, linesWord } from "./findings";
import { statusWords } from "./words";
import css from "./ReviewComment.module.css";
import { Suggestion } from "./Suggestion";

// ReviewComment is a comment in a review: a draft's (yours to edit, ⏎;
// D deletes it), or a submitted one, with what crew and you have said
// since, resolving it (x) and replying (⏎). Resolved, it's its head alone
// unless the cursor's on it. One apart from the lines (on a file the
// diff doesn't have) shows the lines it's on; one made on a commit the
// diff doesn't show says so (elsewhere). A follow-up's word on it says
// whether what's been pushed since deals with it.
export function ReviewComment({
  id,
  note,
  draft,
  apart,
  elsewhere,
  cursor,
  replying,
  followup,
  onClick,
  onToggle,
  onReply,
  onReplyCancel,
}: {
  id: string;
  note: ReviewNote;
  draft: boolean;
  apart?: boolean;
  elsewhere?: boolean;
  cursor: boolean;
  replying: boolean;
  followup?: { kind: string; text: string; blocks?: boolean };
  onClick: () => void;
  onToggle: () => void; // resolve it, or reopen it
  onReply: (body: string) => void;
  onReplyCancel: () => void;
}) {
  const resolved = note.status === "resolved";
  return (
    <NoteCard id={id} className={cx(css.card, apart && css.apart)} cursor={cursor} draft={draft} resolved={resolved} onClick={onClick}>
      <NoteHead
        end={
          <>
            {!draft &&
              (note.url ? (
                <NoteId href={note.url} title="Open it on GitHub">
                  GitHub
                </NoteId>
              ) : (
                <NoteId>{note.id}</NoteId>
              ))}
            {cursor && draft && (
              <span className={`dim ${css.keys}`}>
                <Keys k="⏎" /> edit <Keys k="D" /> delete
              </span>
            )}
            {!draft && (
              <Button variant="ghost" size="small" onClick={(e) => (e.stopPropagation(), onToggle())} title={resolved ? "Reopen" : "Resolve"} keys="x">
                {resolved ? "Reopen" : "Resolve"}
              </Button>
            )}
          </>
        }
      >
        {note.kind ? <span className={`kind k-${note.kind}`}>{capital(note.kind)}</span> : <span className="kind k-none">Comment</span>}
        {note.by && (
          <span className={css.by} title={note.by === "agent" ? "An agent proposed it, and you added it to your review: it's yours to edit" : `${note.by} on GitHub`}>
            <Icon name={note.by === "agent" ? "sparkle" : "user"} size={11} /> {note.by === "agent" ? "Agent's" : note.by}
          </span>
        )}
        {draft ? <NoteStatus>Draft</NoteStatus> : <NoteStatus status={note.status}>{statusWords[note.status] ?? note.status}</NoteStatus>}
        {note.sending && (
          <span className="dim" title="GitHub's being told">
            Sending…
          </span>
        )}
        <span className={`dim ${css.where}`}>
          {note.side === "old" ? "removed " : ""}
          {linesWord(note)}
          {elsewhere && ` · made on ${note.commit.slice(0, 7)}, not in this diff`}
        </span>
      </NoteHead>
      {apart && note.excerpt?.length > 0 && !(resolved && !cursor) && <Excerpt lines={note.excerpt} from={note.line} title="The lines it's on, not showing above" />}
      {followup && !resolved && (
        <div className={cx(css.followup, followup.kind === "addressed" && css.addressed, followup.blocks && css.blocks)} title="What the Follow-up pass said of this comment: whether what's been pushed since deals with it">
          <span className={css.pass}>
            <Icon name="sparkle" size={11} /> Follow-up
          </span>
          <strong>{followWords[followup.kind] ?? followup.kind}</strong>
          <Inline text={followup.text} className={css.followSaid} />
        </div>
      )}
      {!(resolved && !cursor) && (
        <>
          {note.body && <NoteBody>{note.body}</NoteBody>}
          {note.suggestion != null && <Suggestion from={note.excerpt} to={note.suggestion} />}
          {note.thread.length > 0 && (
            <NoteThread>
              {note.thread.map((t, i) => (
                <NoteTurn key={i} who={t.by === "you" ? "You" : t.by} said={t.body}>
                  {t.sending && (
                    <span className="dim" title="It's on its way to GitHub">
                      Sending…
                    </span>
                  )}
                  {t.status && <NoteStatus status={t.status === "open" ? "reopened" : t.status}>{t.status === "open" ? "Reopened" : statusWords[t.status]}</NoteStatus>}
                  {t.commit && <span className="mono dim">{t.commit.slice(0, 7)}</span>}
                </NoteTurn>
              ))}
            </NoteThread>
          )}
          {!draft && !replying && cursor && (
            <NoteHint>
              <Keys k="⏎" /> reply
            </NoteHint>
          )}
          {replying && <ReplyBox onSend={onReply} onCancel={onReplyCancel} />}
        </>
      )}
    </NoteCard>
  );
}
