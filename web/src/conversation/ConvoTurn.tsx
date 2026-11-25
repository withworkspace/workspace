import type { ReactNode } from "react";
import type { TalkDetail, TalkTurn } from "../api";
import { Markdown } from "../markdown/LazyMarkdown";
import { SettledMarker } from "../talk/SettledMarker";
import { settled } from "../talk/proposals";
import { Calls } from "../ui/Calls";
import { Turn, TurnSaid } from "../ui/Turn";
import { ConversationGiven } from "./ConversationGiven";
import css from "./ConvoTurn.module.css";

// ConvoTurn is a turn of a conversation beside the page: what you said
// (with what the page gave the agent with it, folded), the agent's answer
// with its steps above it and what it proposed with it below (children),
// ws saying it couldn't, or the marker of a card you settled.
export function ConvoTurn({
  t,
  talk,
  root,
  children,
}: {
  t: TalkTurn;
  talk: TalkDetail;
  root?: string;
  children?: ReactNode;
}) {
  const m = settled(t, talk.proposals);
  if (m) return <SettledMarker m={m} t={t} compact />;
  if (t.by === "you") {
    return (
      <Turn by="you" dense landed>
        <TurnSaid written>{t.body}</TurnSaid>
        {t.given && <ConversationGiven from={t.given.from} parts={givenParts(t.given)} />}
      </Turn>
    );
  }
  if (t.by !== "agent")
    return (
      <Turn by="ws" dense landed>
        <TurnSaid>{t.body}</TurnSaid>
      </Turn>
    );
  return (
    <Turn by="agent" dense landed>
      {t.calls && t.calls.length > 0 && <Calls calls={t.calls} root={root} />}
      <TurnSaid className={css.answer}>
        <Markdown text={t.body} pinned={t.refs} />
      </TurnSaid>
      {children}
    </Turn>
  );
}

// givenParts is what a page gave the agent with something you said, each
// under its label.
function givenParts(g: NonNullable<TalkTurn["given"]>): [string, string][] {
  const parts: [string, string][] = [];
  if (g.holding || g.holdingText) parts.push([`Selected${g.holding ? ` (${g.holding})` : ""}`, g.holdingText ?? ""]);
  if (g.context) parts.push(["With this message", g.context]);
  if (g.page) parts.push(["The page", g.page]);
  return parts;
}
