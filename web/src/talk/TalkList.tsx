import { useEffect, useMemo, useRef, useState } from "react";
import { useNavigate, useSearchParams } from "react-router";
import { startTalk, type Talk } from "../api";
import { useKeys } from "../keys";
import { useThingMenu } from "../shell/ThingMenu";
import { Button } from "../ui/Button";
import { Empty } from "../ui/Empty";
import { Icon } from "../ui/Icon";
import { MoreLink } from "../ui/MoreLink";
import { Rows } from "../ui/Rows";
import { sendKeys, useSendKey } from "../ui/sendKey";
import { humanError } from "../ui/strings";
import { toast } from "../ui/toast";
import { useDraft } from "../ui/useDraft";
import { TalkNew } from "./TalkNew";
import { TalkRow } from "./TalkRow";
import { docName, talkHref, talkName, talkStatus } from "./words";

// TalkList starts a conversation and lists them: the workspace's, or a
// mission's on its Talk tab (where the page's own keys apply, not these).
// docs: the workspace has kinds of document, so its agent can propose one.
// planner is the mission's planner's conversation, which opens on the
// brief's own page (where what it proposes is read and confirmed).
export function TalkList({
  slug,
  talks: all,
  showResolved,
  onToggleResolved,
  keys,
  docs,
  planner,
}: {
  slug: string;
  talks: Talk[];
  showResolved: boolean;
  onToggleResolved?: () => void;
  keys?: boolean;
  docs?: boolean;
  planner?: string;
}) {
  const menu = useThingMenu();
  const hrefOf = (id: string) => (slug && id === planner ? `/missions/${slug}/brief` : talkHref(slug, id));
  const navigate = useNavigate();
  const [params, setParams] = useSearchParams();
  const fresh = params.get("new") === "1";
  const talks = useMemo(() => all.filter((t) => showResolved || t.status === "open"), [all, showResolved]);
  const [index, setIndex] = useState(0);
  const [text, setText] = useDraft(`new:${slug}`);
  const [starting, setStarting] = useState(false);
  const box = useRef<HTMLTextAreaElement>(null);
  // From "New mission": straight into words.
  useEffect(() => {
    if (fresh) box.current?.focus();
  }, [fresh]);
  const open = () => talks[index] && navigate(hrefOf(talks[index].id));
  const move = (i: number) => {
    const n = Math.min(Math.max(i, 0), talks.length - 1);
    setIndex(n);
    document.getElementById(`talk-${talks[n]?.id}`)?.scrollIntoView({ block: "nearest" });
  };
  useKeys(
    keys
      ? {
          n: () => box.current?.focus(),
          j: () => move(index + 1),
          ArrowDown: () => move(index + 1),
          k: () => move(index - 1),
          ArrowUp: () => move(index - 1),
          Enter: open,
          o: open,
          A: () => onToggleResolved?.(),
        }
      : {},
  );
  const sendKey = useSendKey();
  const start = () => {
    if (!text.trim() || starting) return;
    setStarting(true);
    startTalk(slug, text)
      .then((d) => {
        setText("");
        navigate(talkHref(slug, d.id));
      })
      .catch((e: Error) => toast(humanError(e.message), { tone: "danger" }))
      .finally(() => setStarting(false));
  };
  const placeholder = slug
    ? "Think about this mission: is the plan still right, what should happen next, what about…"
    : fresh
      ? "Describe the mission: what, and why. The agent reads the code and proposes a brief."
      : "What are you thinking about? An idea, a problem, a question about the code…";
  return (
    <>
      <TalkNew
        fresh={fresh}
        field={box}
        value={text}
        onChange={setText}
        onSend={start}
        rows={2}
        tallest={280}
        aria-label="Start a conversation"
        placeholder={placeholder}
        onBlur={() => fresh && !text && setParams({}, { replace: true })}
        hint={
          <>
            <Icon name="eye" size={12} />
            {slug
              ? "Read-only. The agent can propose a decision, a brief, crew, a replan or a follow-up."
              : `Read-only. The agent can propose a mission${docs ? ", a decision or a document" : " or a decision"}.`}
          </>
        }
      >
        <Button variant="primary" disabled={!text.trim() || starting} onClick={start} keys={sendKeys(sendKey)}>
          {starting ? "Starting…" : "Start"}
        </Button>
      </TalkNew>
      {talks.length === 0 ? (
        all.length ? (
          <Empty icon="check-circle" title="Nothing open">
            All conversations resolved.{" "}
            {onToggleResolved && (
              <MoreLink onClick={onToggleResolved} keys={keys ? "⇧A" : undefined}>
                Show them
              </MoreLink>
            )}
          </Empty>
        ) : (
          <Empty icon="message" title="No conversations yet">
            {slug
              ? "Talk the mission through with an agent: whether the plan's still right, what's next, what was decided."
              : "Think out loud with an agent that's read the code: an idea, a problem, how something works. When it's ready, it proposes a mission."}
          </Empty>
        )
      ) : (
        <Rows role="listbox" aria-label="Conversations">
          {talks.map((t, i) => (
            <TalkRow
              key={t.id}
              href={hrefOf(t.id)}
              id={`talk-${t.id}`}
              selected={keys && i === index}
              status={talkStatus(t)}
              title={talkName(t)}
              yourTurn={t.waiting && t.status === "open"}
              proposed={t.proposed}
              pr={t.pr}
              doc={t.doc ? { name: docName(t.doc), title: `The document it's about: ${t.doc}${t.section ? `, § ${t.section}` : ""}` } : undefined}
              became={t.became}
              summary={t.summary}
              at={t.updated || t.at}
              onPoint={() => i !== index && setIndex(i)}
              onContextMenu={menu(hrefOf(t.id), t.title || t.opening)}
            />
          ))}
        </Rows>
      )}
      {!keys && all.some((t) => t.status !== "open") && onToggleResolved && talks.length > 0 && (
        <MoreLink onClick={onToggleResolved}>
          {showResolved ? "Hide" : "Show"} resolved
        </MoreLink>
      )}
    </>
  );
}
