import { type ReactNode, useLayoutEffect, useRef } from "react";
import type { TalkDetail, TalkProposal } from "../../api";
import { placed, waiting } from "../../ask/ask";
import { Markdown } from "../../markdown/LazyMarkdown";
import { settled } from "../../talk/proposals";
import { ProposalCard, SettledMarker } from "../../talk/Talk";
import { Calls } from "../../ui/Calls";
import { Doing } from "../../ui/Doing";
import { Turn, TurnSaid } from "../../ui/Turn";
import { docLines, type DocLine } from "../briefpage";
import { BriefYourWords } from "./BriefYourWords";
import { missionBriefCallsClass, MissionBriefPlanner } from "./MissionBriefPlanner";
import { MissionBriefProposed } from "./MissionBriefProposed";

// The planner, beside a mission's brief and plan (Brief.tsx): its
// conversation as it's been said, where the brief's page used to keep it
// behind an ask bar.

const isDoc = (p: TalkProposal) => p.kind === "brief" || p.kind === "plan";

// What didn't stand: its name is struck.
const fell = new Set(["superseded", "declined", "withdrawn"]);

// BriefPlanner is the mission's planner's conversation: what you and it
// have said, its steps folded over each reply, and what it proposed under
// the reply it came with. What it proposed for the brief or the plan is a
// line (the documents show it, as changes to read): the one they show
// leads to them. Anything else (crew, a decision, a mission) is its card,
// to confirm here. centre is the conversation in the documents' place,
// while there's nothing drafted to be beside; action is what its head
// offers besides (writing the brief yourself, then).
export function BriefPlanner({
  t,
  slug,
  root,
  shown,
  centre,
  field,
  action,
  onSettle,
  toDocs,
  fold,
}: {
  t: TalkDetail;
  slug: string;
  root: string;
  shown?: number; // the proposal the documents show
  centre?: boolean;
  field: ReactNode;
  action?: ReactNode;
  onSettle: (n: number, action: "accept" | "decline", edits?: object) => Promise<void>;
  toDocs: () => void;
  fold?: () => void;
}) {
  const body = useRef<HTMLDivElement>(null);
  // It keeps to its latest, unless you've scrolled up in it.
  const near = useRef(true);
  useLayoutEffect(() => {
    const el = body.current;
    if (el && near.current) el.scrollTop = el.scrollHeight;
  }, [t]);

  const following = waiting(t);
  const lines = docLines(t, shown);
  // Everything else it proposed, by the reply it came with, as its cards.
  const cards = placed(t, (p) => !isDoc(p));
  const line = (l: DocLine) => <MissionBriefProposed key={l.p.n} icon={l.p.kind === "plan" ? "list-ordered" : "file"} name={l.name} fate={l.fate} shown={l.shown} struck={fell.has(l.p.state)} onGo={toDocs} />;
  const card = (p: TalkProposal, replying?: boolean) => <ProposalCard key={p.n} p={p} slug={slug} replying={replying} onSettle={onSettle} />;
  return (
    <MissionBriefPlanner
      label="The planner's conversation"
      centre={centre}
      talk={{ id: t.id, to: `/missions/${slug}/conversations/${t.id}` }}
      action={action}
      onFold={fold}
      body={body}
      live
      onScroll={(e) => {
        const el = e.currentTarget;
        near.current = el.scrollHeight - el.scrollTop - el.clientHeight < 60;
      }}
      foot={field}
    >
      {t.turns.map((turn) => {
        const m = settled(turn, t.proposals);
        if (m) return <SettledMarker key={turn.n} m={m} t={turn} compact />;
        if (turn.by === "you") {
          return (
            <Turn key={turn.n} by="you" dense landed>
              <TurnSaid written>
                <BriefYourWords body={turn.body} />
              </TurnSaid>
            </Turn>
          );
        }
        return (
          <Turn key={turn.n} by={turn.by === "agent" ? "agent" : "ws"} dense landed>
            {turn.calls && turn.calls.length > 0 && <Calls calls={turn.calls} root={root} className={missionBriefCallsClass()} />}
            <TurnSaid>
              <Markdown text={turn.body} pinned={turn.refs} />
            </TurnSaid>
            {lines.under.get(turn.n)?.map(line)}
            {cards.under.get(turn.n)?.map((p) => card(p))}
          </Turn>
        );
      })}
      {following && (
        <Turn by="agent" dense running>
          {t.run?.said && (
            <TurnSaid className="dim">
              <Markdown text={t.run.said} />
            </TurnSaid>
          )}
          <Doing calls={t.run?.calls ?? []} root={root} />
        </Turn>
      )}
      {lines.after.map(line)}
      {cards.after.map((p) => card(p, following))}
    </MissionBriefPlanner>
  );
}
