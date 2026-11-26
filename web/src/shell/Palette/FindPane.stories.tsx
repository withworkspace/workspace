import type { Meta, StoryObj } from "@storybook/react-vite";
import type { ReactNode } from "react";
import { finderBox } from "../../ui/Finder";
import { HoverCardLine } from "../../ui/HoverCard";
import { Icon } from "../../ui/Icon";
import { FindAct } from "./FindAct";
import { FindActs } from "./FindActs";
import { FindCodeCard } from "./FindCodeCard";
import { FindItemCard } from "./FindItemCard";
import { FindPane } from "./FindPane";
import { FindPullCard } from "./FindPullCard";
import { FindSaidCard } from "./FindSaidCard";
import { FindThing } from "./FindThing";
import { hits, pulls } from "./samples";
import { Text, textClass } from "../../ui/Text";

// Find's pane, with each card it shows for what the cursor's on: a line
// of a file, a pull request (as Find was told of it, and one it would
// have to ask GitHub about), something found by what it says, a row with
// no card of its own, a plugin's thing with what can be done to it under
// it (long enough to scroll), and nothing. The pane is as wide whatever
// it holds.

const meta: Meta = { title: "Find/FindPane" };
export default meta;

// A pane, in a box as tall as Find's, beside where its list would be.
function Pane({ says, scrolls, children }: { says: string; scrolls?: boolean; children: ReactNode }) {
  return (
    <div style={{ display: "grid", gap: 6 }}>
      <Text dim>{says}</Text>
      <div className={finderBox} style={{ display: "flex", width: "auto", height: 260 }}>
        <FindPane scrolls={scrolls}>{children}</FindPane>
      </div>
    </div>
  );
}

const lantern =
  "It hangs from the hook by the door, and has done since the shop opened. Its glass is cracked on the side that faces the wall, which nobody sees. It wants dusting each month, and its wick trimming twice a year, in spring and before the nights draw in. ";

export const Cards: StoryObj = {
  render: () => {
    const h = hits[0]!;
    return (
      <div className="page" style={{ display: "flex", flexWrap: "wrap", gap: 24, alignItems: "start" }}>
        <Pane says="A line of a file">
          <FindCodeCard path={h.path} line={h.line} repo={h.repo} worktree={h.worktree} />
        </Pane>
        <Pane says="A file with a long path, on the default branch">
          <FindCodeCard path={hits[3]!.path} line={hits[3]!.line} repo="api" worktree=".ground" />
        </Pane>
        <Pane says="A pull request, its checks passing, in a stack">
          <FindPullCard pull={pulls[1]!.pull!} />
        </Pane>
        <Pane says="A draft whose checks fail">
          <FindPullCard pull={pulls[2]!.pull!} />
        </Pane>
        <Pane says="A pull request Find wasn't told of">
          <FindPullCard pull={{ repo: "api", number: 12, title: "", asks: true }} />
        </Pane>
        <Pane says="Found by what it says">
          <FindSaidCard
            icon="message"
            title="Where do we round: per line, or on the total, and does it matter to anyone but the auditors?"
            about={["Conversation", "w3", "on Invoice totals off by a penny", "archived"]}
            snippet={[{ text: "…we agreed to keep " }, { text: "rounding", mark: true }, { text: " per line, since that's what the PDF shows, and to say so in ADR 0001 so the " }, { text: "rounding", mark: true }, { text: " isn't argued again." }]}
          />
        </Pane>
        <Pane says="A row with no card of its own">
          <FindItemCard mark={<Icon name="play" size={13} className={textClass("dim")} />} title="Resume" kind="Do" hint="Invoices in more than one currency is parked" />
        </Pane>
        <Pane says="A plugin's thing, and what can be done to it" scrolls>
          <FindThing>
            <FindItemCard mark={<Icon name="chevron-right" size={13} className={textClass("dim")} />} title="THING-2: the lantern" kind="A fixture thing" />
            <HoverCardLine wrap>{lantern.repeat(3)}</HoverCardLine>
          </FindThing>
          <FindActs>
            <FindAct icon="play" onClick={() => {}}>
              Start a mission
            </FindAct>
            <FindAct icon="check" onClick={() => {}}>
              Dust it
            </FindAct>
            <FindAct icon="x" disabled onClick={() => {}}>
              Shelve it
            </FindAct>
          </FindActs>
        </Pane>
        <Pane says="The cursor on nothing">
          <HoverCardLine dim>What the cursor's on shows here.</HoverCardLine>
        </Pane>
      </div>
    );
  },
};
