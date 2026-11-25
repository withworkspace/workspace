import type { Ref } from "react";
import type { TalkProposal, TalkTurn } from "../api";
import { Markdown } from "../markdown/LazyMarkdown";
import { Calls } from "../ui/Calls";
import { Turn, TurnSaid } from "../ui/Turn";
import { settled } from "./proposals";
import { SettledMarker } from "./SettledMarker";
import { Who } from "./Who";

// TurnView is a turn: yours as a bubble, the agent's (its steps folded
// above it) or ws's as text; one ws wrote for a card you settled, as its
// marker. said is given its words' box (the conversation's latest reply:
// where the page rests); landed is one that was the run going until now,
// which doesn't come in again.
export function TurnView({ t, root, proposals, said, landed }: { t: TalkTurn; root: string; proposals: TalkProposal[]; said?: Ref<HTMLDivElement>; landed?: boolean }) {
  const m = settled(t, proposals);
  if (m) return <SettledMarker m={m} t={t} />;
  const who = t.by === "you" || t.by === "agent" || t.by === "ws" ? t.by : "crew";
  return (
    <Turn by={who} landed={landed} head={<Who by={t.by} at={t.at} />}>
      {t.calls && t.calls.length > 0 && <Calls calls={t.calls} root={root} />}
      <TurnSaid ref={said} written={t.by === "you"}>
        {t.by === "you" ? t.body : <Markdown text={t.body} pinned={t.refs} />}
      </TurnSaid>
    </Turn>
  );
}
