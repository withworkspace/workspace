import { useState } from "react";
import { draftWaiting, pendingSuggestion, type Thread, waitingOnYou, withCrew } from "../api";
import { Markdown } from "../markdown/LazyMarkdown";
import { Button } from "../ui/Button";
import { Chip } from "../ui/Chip";
import { Icon } from "../ui/Icon";
import { Keys } from "../ui/Keys";
import { NoteBody, NoteCard, NoteHead, NoteHint, noteHintActionClass, NoteId, NoteThread, NoteTurn } from "../ui/NoteCard";
import { ReplyBox } from "../ui/ReplyBox";
import { clip } from "../ui/text";
import { AnnotateEdit } from "./AnnotateEdit";
import { AnnotateKindChip } from "./AnnotateKindChip";
import { AnnotateLostQuote } from "./AnnotateLostQuote";
import { AnnotateSettled } from "./AnnotateSettled";
import { AnnotateTyping } from "./AnnotateTyping";
import { DecisionCard } from "./DecisionCard";
import { linesWord } from "./words";

interface CardProps {
  thread: Thread;
  active: boolean;
  hovered?: boolean;
  replying: boolean;
  unplaced: boolean;
  onFocus: () => void;
  onHover: (on: boolean) => void;
  onReply: () => void;
  onSay: (body: string) => void;
  onReplyCancel: () => void;
  onToggle: () => void;
  onSettle: (action: "accept" | "decline") => void;
  settling: boolean;
  onSend: () => void;
  own: boolean; // the document's in no mission: no crew to send to, no decisions
  landing?: string; // accepting its edit lands it on this branch: asked once, a second accept does it
  sending: boolean;
  onPreview: () => void;
  onDecide: (d: { action: "record"; what: string; why: string } | { action: "skip" }) => Promise<void>;
}

// ThreadCard is a thread in the margin: what you said, and what's been
// said since. A resolved one is folded until it's in focus.
export function ThreadCard({ thread, active, hovered, replying, unplaced, onFocus, onHover, onReply, onSay, onReplyCancel, onToggle, onSettle, settling, onSend, sending, own, landing, onDecide, onPreview }: CardProps) {
  const resolved = thread.status === "resolved";
  const pending = !resolved ? pendingSuggestion(thread) : undefined;
  const settled = (thread.suggestions ?? []).filter((s) => s.state === "accepted" || s.state === "declined");
  const drafted = draftWaiting(thread);
  const [deciding, setDeciding] = useState(false); // writing a decision of your own
  const folded = resolved && !active && !unplaced && !drafted;
  const yours = waitingOnYou(thread);
  const crew = !resolved && withCrew(thread) ? thread.sent?.crew : undefined;
  const sendable = !own && !resolved && thread.kind === "change" && !crew && !thread.answering && !pending;
  const [first, ...rest] = thread.turns;
  return (
    <NoteCard
      dense
      cursor={active}
      hovered={hovered}
      resolved={resolved}
      folded={folded}
      yours={yours}
      onClick={onFocus}
      onMouseEnter={() => onHover(true)}
      onMouseLeave={() => onHover(false)}
    >
      <NoteHead
        end={
          <>
            <NoteId>{thread.id}</NoteId>
            <Button variant="ghost" size="small" onClick={(e) => (e.stopPropagation(), onFocus(), onToggle())} title={`${resolved ? "Reopen" : "Resolve"} (x)`}>
              {resolved ? "Reopen" : "Resolve"}
            </Button>
          </>
        }
      >
        <AnnotateKindChip kind={thread.kind} />
        {resolved ? (
          <Chip tone="done">
            <Icon name="check" size={11} /> Resolved
          </Chip>
        ) : crew ? (
          <Chip tone="progress">{crew} is on it</Chip>
        ) : yours ? (
          <Chip tone="attention">Your turn</Chip>
        ) : null}
      </NoteHead>
      {folded ? (
        <NoteBody oneLine className="dim">
          {first?.body}
        </NoteBody>
      ) : (
        <>
          {unplaced && (
            <AnnotateLostQuote title={`opened on version ${thread.anchor.version} of the document, ${linesWord(thread.anchor)}`}>
              {thread.anchor.quote?.exact ? clip(thread.anchor.quote.exact, 240) : `The whole of ${linesWord(thread.anchor)}`}
            </AnnotateLostQuote>
          )}
          {first && <NoteBody>{first.body}</NoteBody>}
          {rest.length > 0 && (
            <NoteThread>
              {rest.map((t) => (
                <NoteTurn key={t.n} who={t.by === "you" ? "You" : t.by === "agent" ? "Agent" : t.by} quiet={t.by === "ws"} said={t.by === "you" ? t.body : <Markdown text={t.body} pinned={t.refs} />}>
                  {t.changed && <span className="dim">changed it</span>}
                  {t.commit && <span className="mono dim">{t.commit.slice(0, 7)}</span>}
                </NoteTurn>
              ))}
            </NoteThread>
          )}
          {thread.answering && <AnnotateTyping>{resolved ? "Drafting what was decided" : "Answering"}</AnnotateTyping>}
          {thread.sent?.finished && !resolved && <AnnotateSettled>{thread.sent.crew} finished without saying it changed the document for this.</AnnotateSettled>}
          {settled.map((s) => (
            <AnnotateSettled key={s.n}>{s.state === "accepted" ? `Edit accepted${s.commit ? `, committed ${s.commit.slice(0, 7)}` : ""}` : "Edit declined"}</AnnotateSettled>
          ))}
          {pending && <AnnotateEdit by={pending.by} from={pending.from} to={pending.to} active={active} onSettle={onSettle} settling={settling} onPreview={onPreview} landing={landing} />}
          {drafted && thread.draft && <DecisionCard key={thread.draft.at} draft={thread.draft} onDecide={onDecide} />}
          {thread.draft?.state === "recorded" && thread.draft.decision && <AnnotateSettled>Recorded as {thread.draft.decision}</AnnotateSettled>}
          {!own && resolved && !thread.draft && !thread.answering && active && !deciding && (
            <NoteHint>
              <Button variant="link" className={noteHintActionClass()} onClick={(e) => (e.stopPropagation(), setDeciding(true))}>
                Record what was decided
              </Button>
            </NoteHint>
          )}
          {deciding && !thread.draft && <DecisionCard onDecide={(d) => onDecide(d).then(() => setDeciding(false))} onCancel={() => setDeciding(false)} />}
          {!replying && active && (
            <NoteHint>
              <Button variant="link" className={noteHintActionClass()} onClick={(e) => (e.stopPropagation(), onReply())}>
                <Icon name="reply" size={12} /> Reply <Keys k="⏎" />
              </Button>
              {sendable && (
                <Button
                  variant="link"
                  disabled={sending}
                  title="Too big for an edit: hand it to crew (the crew that wrote it, or fresh crew in its capsule)"
                  onClick={(e) => (e.stopPropagation(), onSend())}
                >
                  Send to crew
                </Button>
              )}
            </NoteHint>
          )}
          {replying && <ReplyBox onSend={onSay} onCancel={onReplyCancel} />}
        </>
      )}
    </NoteCard>
  );
}
