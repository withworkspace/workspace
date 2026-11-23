import type { ReactNode } from "react";
import { Button } from "../ui/Button";
import { Icon, type IconName } from "../ui/Icon";
import { Segment, Segmented } from "../ui/Segmented";
import { type BelowWhere, belowHeights, belowWhere, heightenBelow, pickBelow, toggleBelow } from "./chromeState";
import { Dock } from "./Dock";
import { DockBody } from "./DockBody";
import { DockEdge } from "./DockEdge";
import { DockHead } from "./DockHead";
import { DockTab } from "./DockTab";
import { DockTabs } from "./DockTabs";
import { useChromeStore } from "./store";

// BelowView is one of the things Below shows, a tab each: a view of
// what's going on (what's running, what it came to), not a thing you
// opened, unless it can be closed (a run's terminal).
export type BelowView = {
  id: string;
  // What its tab says, and leads with.
  name: string;
  icon: IconName;
  // How many it holds, where that's worth saying.
  count?: number;
  // Closing it, for one you opened. One without is Below's own, and
  // stays.
  onClose?: () => void;
  // Set in from the dock's edges: for what's read. Rows, or a terminal,
  // run to the edges without.
  pad?: boolean;
  // What it shows: of here (what this page is of), or of everywhere.
  pane: (where: BelowWhere) => ReactNode;
};

// belowView is the view that's showing: the one last picked, or the first
// where that's gone or none was.
export const belowView = (views: BelowView[], tab?: string): BelowView | undefined => views.find((v) => v.id === tab) ?? views[0];

type BelowProps = {
  // What it shows, in the order of its tabs. With none it isn't drawn.
  views: BelowView[];
  // On a narrow window it's a sheet over the page's lower part.
  sheet?: boolean;
};

// Below is what's going on while you're on the page, under it: the
// sidebar's card along the bottom. The page makes room, and what's
// beside stays next to the two of them. Its tabs are views, which
// whoever draws it gives; its head says which of each is shown, here's
// or everywhere's, and closes it.
//
// Whether it's open, how tall, which view and which of it are the
// window's (store.ts): this draws them, and changes them.
export function Below({ views, sheet }: BelowProps) {
  const below = useChromeStore((s) => s.chrome.below);
  const change = useChromeStore((s) => s.change);
  const now = belowView(views, below.tab);
  if (!now) return null;
  return (
    <Dock side="below" label="Below" size={below.height} sheet={sheet} edge={<DockEdge side="below" label="Below's height" size={below.height} {...belowHeights} onSize={(h) => change((s) => heightenBelow(s, h))} />}>
      <DockHead
        tools={
          <>
            <Segmented aria-label="Which of it" size="small" value={below.where} onChange={(w: BelowWhere) => change((s) => belowWhere(s, w))}>
              <Segment value="here" title="Of what this page is of">
                Here
              </Segment>
              <Segment value="everywhere" title="The workspace's">
                Everywhere
              </Segment>
            </Segmented>
            <Button variant="ghost" size="small" square title="Close Below" onClick={() => change(toggleBelow)}>
              <Icon name="x" size={13} />
            </Button>
          </>
        }
      >
        <DockTabs label="Below">
          {views.map((v) => (
            <DockTab key={v.id} name={v.name} lead={<Icon name={v.icon} size={13} />} count={v.count} on={v.id === now.id} onPick={() => change((s) => pickBelow(s, v.id))} onClose={v.onClose} />
          ))}
        </DockTabs>
      </DockHead>
      <DockBody key={now.id} label={now.name} pad={now.pad}>
        {now.pane(below.where)}
      </DockBody>
    </Dock>
  );
}
