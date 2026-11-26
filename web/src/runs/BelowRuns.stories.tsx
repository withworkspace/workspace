import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { Below, type BelowView } from "../chrome/Below";
import type { BelowWhere } from "../chrome/chromeState";
import { useBelowStarted } from "../chrome/SampleBelow";
import { useChromeStore } from "../chrome/store";
import { Button } from "../ui/Button";
import { BelowRunRow } from "./BelowRunRow";
import { BelowRunsHead } from "./BelowRunsHead";
import { BelowRunsList } from "./BelowRunsList";
import { BelowRunsNone } from "./BelowRunsNone";
import { BelowRunsStart, type StartGroup } from "./BelowRunsStart";

// Below's Runs, in the dock as the app draws it: Sorrel's services, the
// newest first, one just started (marked), one not up yet; here (a
// mission's capsules, with Start for what isn't running) or everywhere.
// Then the ways it's empty: nothing running here, and a page of no place.

const meta: Meta = { title: "Runs/Below" };
export default meta;

const ago = (min: number) => new Date(Date.now() - min * 60_000).toISOString();

type Sample = { name: string; host?: string; up: boolean; fresh?: boolean; place: string; started: string; here?: boolean };

const sorrel: Sample[] = [
  { name: "storybook", host: "storybook.dark-mode.web.sorrel.localhost", up: false, fresh: true, place: "web / dark-mode", started: ago(0.3), here: true },
  { name: "dev", host: "dev.dark-mode.web.sorrel.localhost", up: true, fresh: true, place: "web / dark-mode", started: ago(2), here: true },
  { name: "api", host: "api.dark-mode.api.sorrel.localhost", up: true, place: "api / dark-mode", started: ago(14), here: true },
  { name: "dev", host: "dev.reminders.web.sorrel.localhost", up: true, place: "web / reminders", started: ago(48) },
  { name: "docs", host: "docs.sorrel.localhost", up: true, place: "workspace", started: ago(190) },
];

const start: StartGroup[] = [
  { title: "web / dark-mode", recipes: [{ id: "web\npreview", name: "preview" }] },
  { title: "api / dark-mode", recipes: [{ id: "api\nworker", name: "worker" }, { id: "api\nmail", name: "mail", title: "Mail catcher" }] },
];

function Services({ where }: { where: BelowWhere }) {
  const [busy, setBusy] = useState<string>();
  const [gone, setGone] = useState<string[]>([]);
  const here = where === "here";
  const shown = sorrel.filter((s) => (!here || s.here) && !gone.includes(s.name + s.place));
  return (
    <>
      <BelowRunsHead
        of={here ? "Dark mode" : undefined}
        count={shown.length}
        start={here && <BelowRunsStart groups={start} busy={busy} onStart={(id) => (setBusy(id), setTimeout(() => setBusy(undefined), 1500))} />}
      />
      <BelowRunsList>
        {shown.map((s) => (
          <BelowRunRow
            key={s.name + s.place}
            name={s.name}
            link={s.host && `http://${s.host}`}
            host={s.host}
            up={s.up}
            fresh={s.fresh}
            place={s.place}
            to="/files/web/dark-mode?view=runs"
            started={s.started}
            onStop={() => setGone((g) => [...g, s.name + s.place])}
          />
        ))}
      </BelowRunsList>
    </>
  );
}

function InDock({ pane, count }: { pane: BelowView["pane"]; count?: number }) {
  const started = useBelowStarted(true);
  if (!started) return null;
  return (
    <div style={{ display: "flex", flexDirection: "column", justifyContent: "flex-end", height: "100vh", padding: 12, boxSizing: "border-box" }}>
      <Below views={[{ id: "runs", name: "Runs", icon: "play", count, pane }]} />
    </div>
  );
}

export const Running: StoryObj = { render: () => <InDock count={3} pane={(w) => <Services where={w} />} /> };

export const NothingHere: StoryObj = {
  render: () => (
    <InDock
      pane={() => (
        <>
          <BelowRunsHead of="web / ground" count={0} start={<BelowRunsStart groups={[{ recipes: [{ id: "dev", name: "dev" }, { id: "storybook", name: "storybook" }] }]} onStart={() => {}} />} />
          <BelowRunsNone title="Nothing running here">Start a service above, or with ws run in a terminal.</BelowRunsNone>
        </>
      )}
    />
  ),
};

function NoPlacePane() {
  const change = useChromeStore((s) => s.change);
  return (
    <BelowRunsNone
      title="No runs on this page"
      action={
        <Button size="small" onClick={() => change((s) => ({ ...s, below: { ...s.below, where: "everywhere" } }))}>
          Show all 5 running
        </Button>
      }
    >
      Runs show here on a mission or repo page.
    </BelowRunsNone>
  );
}

export const NoPlace: StoryObj = { render: () => <InDock pane={(w) => (w === "here" ? <NoPlacePane /> : <Services where={w} />)} /> };
