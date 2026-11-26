import { useEffect, useMemo, useRef } from "react";
import { useLocation } from "react-router";
import { getRuns, type Mission, type RunsInfo, useEvents } from "../api";
import { useLoaded } from "../load";
import { BelowRuns } from "../runs/BelowRuns";
import { isNew, ofPlace, placeAt, services as servicesOf } from "../runs/runsBelow";
import { useWorkspaceIfRead } from "../workspace/store";
import type { BelowView } from "./Below";
import { belowWhere, seenBelow } from "./chromeState";
import { useChromeStore } from "./store";

// What Below holds, for the app, and what its toggle at the foot says.
// One view so far: Runs, the services that are running (dev servers,
// previews), here or everywhere, to open, go to, stop, or start another.
// Results and a run's terminal are the views still to come.
//
// The runs are the window's: followed (the runs topic) whether Below's
// open or not, so its toggle can say how many run and that one's started
// since you last looked. Looked: Below closed, which is when what was in
// it has been seen.

// Before the workspace is read, it has no missions.
const noMissions: Mission[] = [];

export interface BelowToggle {
  // How many services are running, anywhere.
  count: number;
  // One's started since Below was last closed.
  fresh: boolean;
}

export function useBelow(on: boolean): { views: BelowView[]; toggle: BelowToggle } {
  const { value: info, reload } = useLoaded<RunsInfo>(on ? getRuns : null, [on], { keep: true });
  useEvents("runs", { hello: () => on && reload(), changed: () => on && reload() });
  const below = useChromeStore((s) => s.chrome.below);
  const change = useChromeStore((s) => s.change);
  const missions = useWorkspaceIfRead((w) => w.missions) ?? noMissions;
  const { pathname } = useLocation();
  const place = placeAt(pathname);
  const services = useMemo(() => servicesOf(info?.runs ?? []), [info]);

  // Nothing's new before anything's been seen: from the first time, what's
  // running then is.
  useEffect(() => {
    if (below.seen === undefined) change((s) => seenBelow(s, Date.now()));
  }, [below.seen, change]);
  // Closed, it's been seen, however it was closed.
  const wasOpen = useRef(below.open);
  useEffect(() => {
    if (wasOpen.current && !below.open) change((s) => seenBelow(s, Date.now()));
    wasOpen.current = below.open;
  }, [below.open, change]);

  const here = below.where === "here";
  const shown = here ? (place ? ofPlace(services, place, missions) : []) : services;
  const views: BelowView[] = [
    {
      id: "runs",
      name: "Runs",
      icon: "play",
      count: shown.length || undefined,
      pane: (where) => <BelowRuns where={where} services={services} seen={below.seen} place={place} reload={reload} onEverywhere={() => change((s) => belowWhere(s, "everywhere"))} />,
    },
  ];
  return { views, toggle: { count: services.length, fresh: !below.open && services.some((r) => isNew(r, below.seen)) } };
}
