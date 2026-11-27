import { type ReactNode, useEffect, useState } from "react";
import type { Workspace } from "../api";
import { talkName, useTalks } from "../conversation/talks";
import { titleOf } from "../documents/peek";
import { AskPane } from "../panes/AskPane";
import { CommentsPane } from "../panes/CommentsPane";
import { DocumentPane } from "../panes/DocumentPane";
import { EmptyPane } from "../panes/EmptyPane";
import { MissionPane } from "../panes/MissionPane";
import { NewTalkPane } from "../panes/NewTalkPane";
import { PullRequestPane } from "../panes/PullRequestPane";
import { TalkPane } from "../panes/TalkPane";
import { talkStatus } from "../talk/words";
import { everywhereKeys } from "../shell/keys";
import { Button } from "../ui/Button";
import { Icon } from "../ui/Icon";
import { Status } from "../ui/Status";
import { headlineStatus } from "../ui/statusOf";
import { besideWidths, pickBeside, toggleBeside, widenBeside } from "./chromeState";
import { Dock } from "./Dock";
import { DockDivide } from "./DockDivide";
import { DockEdge } from "./DockEdge";
import { DockBody } from "./DockBody";
import { DockHead } from "./DockHead";
import { DockTab } from "./DockTab";
import { DockTabList, type ListedTab } from "./DockTabList";
import { DockTabs } from "./DockTabs";
import { type PaneAt, paneAt } from "./panes";
import { useBeside, useChromeStore } from "./store";
import { useTabNames } from "./tabNames";
import { useWorkspace } from "../workspace/store";
import css from "./Beside.module.css";

type BesideProps = {
  // On a narrow window it's a sheet over the page.
  sheet?: boolean;
};

// Beside is a peek at a second thing, next to the page: a conversation
// about the document you're reading, beside the document. What's open
// there is a tab each, and the one in front is its pane.
//
// First what's about the thing this page is of (what you asked here),
// which is there only on that thing's page: it's hidden as you leave and
// back as you come back. Then what's about nothing, which is there
// anywhere. It can float over the page instead of the page making room
// (its toggle's menu, at the foot).
//
// One of the tabs may be a peek (z, on what the pointer's on): a mission,
// a document, a pull request or a conversation, looked at and not opened.
// The next thing peeked at takes its place. Another may be a conversation
// about to begin (C): nothing's kept of it until something's said, and
// then it's that conversation, in its place. Both are named in italics,
// which is how a tab says it isn't kept yet.
//
// Docked, where the window has room for it next to the page, it stays
// as you go from page to page, with something in it here or not: the
// page's width doesn't change under you. With nothing here it says so
// (EmptyPane). Floating, it goes while it has nothing here, as the page
// keeps its width either way.
//
// What it shows is the window's (store.ts): this draws it, and changes
// it.
export function Beside({ sheet }: BesideProps) {
  // What the workspace lists of its conversations (what a tab calls one,
  // and when one's changed), and where it is.
  const conversations = useWorkspace((w) => w.conversations);
  const missions = useWorkspace((w) => w.missions);
  const root = useWorkspace((w) => w.root);
  const now = useBeside();
  const change = useChromeStore((s) => s.change);
  const close = useChromeStore((s) => s.close);
  const here = useChromeStore((s) => s.here);
  const talks = useTalks((s) => s.talks);
  const names = useTabNames((s) => s.names);
  const peek = useChromeStore((s) => s.chrome.beside.peek);
  const [picks, setPicks] = useState(0);

  // A conversation behind the one in front hasn't been read by a pane:
  // it's read once, for what its tab calls it.
  const unread = now.all.filter((t) => paneAt(t)?.kind === "conversation" && !talks[t]).join("\n");
  useEffect(() => {
    for (const t of unread ? unread.split("\n") : []) void useTalks.getState().load(t);
  }, [unread]);

  const over = now.over && !sheet;
  const front = now.front;
  const at = front ? paneAt(front) : undefined;
  const edge = <DockEdge side="beside" label="Beside's width" size={now.width} {...besideWidths} onSize={(w) => change((s) => widenBeside(s, w))} />;
  if (!front || !at) {
    if (!now.shown) return null;
    return (
      <Dock side="beside" label="Beside" size={now.width} over={over} sheet={sheet} edge={edge}>
        <DockHead>
          <span className={css.nothing}>Beside</span>
        </DockHead>
        <DockBody label="Nothing beside this page">
          <EmptyPane elsewhere={now.elsewhere} />
        </DockBody>
      </Dock>
    );
  }
  const said = (t: string) => tabOf({ conversations, missions }, paneAt(t)!, paneAt(t)!.kind === "conversation" ? talkName(talks[t]?.detail) : names[t]);
  const passing = (t: string) => (paneAt(t)?.kind === "new" ? "not started until you send a message" : t === peek ? "peek, replaced by the next" : undefined);
  const pick = (t: string) => {
    change((s) => pickBeside(s, t, here));
    setPicks((n) => n + 1); // the one that's in front already is brought back into sight
  };
  const tab = (t: string) => <DockTab key={t} {...said(t)} on={t === front} passing={passing(t)} onPick={() => pick(t)} onClose={() => close(t, { yours: true })} />;
  const listed = (t: string): ListedTab => ({ id: t, ...said(t), on: t === front, onPick: () => pick(t) });
  return (
    <Dock side="beside" label="Beside" size={now.width} over={over} sheet={sheet} edge={edge}>
      <DockHead
        // Closing it, and floating it, are its toggle's at the foot
        // (FootBeside). A sheet covers the page, so it has its own way out.
        tools={
          sheet && (
            <Button variant="ghost" size="small" square title="Close Beside" keys={everywhereKeys.beside} onClick={() => change(toggleBeside)}>
              <Icon name="x" size={13} />
            </Button>
          )
        }
      >
        <DockTabs
          label="Open beside"
          shown={picks}
          list={
            <DockTabList
              lots={[
                { name: "About this page", tabs: now.here.map(listed) },
                { name: "Anywhere", tabs: now.anywhere.map(listed) },
              ]}
            />
          }
        >
          {now.here.map(tab)}
          {now.here.length > 0 && now.anywhere.length > 0 && <DockDivide />}
          {now.anywhere.map(tab)}
        </DockTabs>
      </DockHead>
      {at.kind === "conversation" && <TalkPane key={front} to={front} root={root} stamp={at.slug ? undefined : conversations.find((c) => c.id === at.id)?.updated} onGone={() => close(front)} />}
      {at.kind === "ask" && <AskPane key={front} to={front} thing={at.thing} />}
      {at.kind === "new" && <NewTalkPane key={front} to={front} />}
      {at.kind === "mission" && <MissionPane key={front} to={front} slug={at.slug} />}
      {at.kind === "document" && <DocumentPane key={front} to={front} docRef={at.ref} />}
      {at.kind === "pr" && <PullRequestPane key={front} to={front} repo={at.repo} number={at.number} />}
      {at.kind === "comments" && <CommentsPane key={front} of={at.of} />}
    </Dock>
  );
}

// tabOf is what a tab says of its thing: its name, cut short by the tab,
// and how it stands. A conversation or a mission the workspace lists is
// said as its list says it; a conversation nobody's kept (an ask) isn't
// listed, and is called what it was read as. A document and a pull
// request are called what their pane read them as, and until then by
// their address: the file's name, the pull request's number.
function tabOf(listed: Pick<Workspace, "conversations" | "missions">, at: PaneAt, read?: string): { name: string; lead: ReactNode } {
  if (at.kind === "ask") return { name: `Ask about ${at.thing}`, lead: <Icon name="sparkle" size={13} /> };
  if (at.kind === "new") return { name: "New conversation", lead: <Icon name="message" size={13} /> };
  if (at.kind === "mission") {
    const m = listed.missions.find((x) => x.slug === at.slug);
    return { name: m?.title ?? at.slug, lead: m ? <Status s={headlineStatus(m.headline)} iconOnly size={13} /> : <Icon name="flag" size={13} /> };
  }
  if (at.kind === "document") return { name: read ?? titleOf("", at.ref.slice(at.ref.indexOf(":") + 1)), lead: <Icon name="file" size={13} /> };
  if (at.kind === "comments") return { name: "Comments", lead: <Icon name="message" size={13} /> };
  if (at.kind === "pr") return { name: read ?? `${at.repo}#${at.number}`, lead: <Icon name="pr" size={13} /> };
  const t = at.slug ? undefined : listed.conversations.find((c) => c.id === at.id);
  return { name: t?.title || t?.opening || read || "Conversation", lead: t ? <Status s={talkStatus(t)} iconOnly size={13} /> : <Icon name="sparkle" size={13} /> };
}
