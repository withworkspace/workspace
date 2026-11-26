import { useState } from "react";
import { endRun, getWorktreeRuns, type RunDetail, runInWorktree, runRecipe, type WorktreeRunsInfo } from "../api";
import type { BelowWhere } from "../chrome/chromeState";
import { type Reload, useLoaded } from "../load";
import { useMissionDetail } from "../mission/useMissionDetail";
import { Button } from "../ui/Button";
import { humanError } from "../ui/strings";
import { toast } from "../ui/toast";
import { useWorkspace } from "../workspace/store";
import { BelowRunRow } from "./BelowRunRow";
import { type BelowPlace, isNew, jumpTo, ofPlace, placeSaid } from "./runsBelow";
import { BelowRunsHead } from "./BelowRunsHead";
import { BelowRunsList } from "./BelowRunsList";
import { BelowRunsNone } from "./BelowRunsNone";
import { BelowRunsStart, type StartGroup } from "./BelowRunsStart";
import { bare } from "./runRows";

// BelowRuns is Below's Runs view: the services running here (the
// mission's, or the repo's ground or capsule the page is of) or
// everywhere, the newest first, those started since you last looked
// marked New; each to open where it answers, to go to (its worktree's
// Runs tab, with it picked), and to stop. Here, Start starts another of
// the place's services from its ws.repo.toml, which lasts as one started
// from the page does: while the mission, or the worktree, is open.
//
// The services are the window's (belowViews.tsx), followed whether Below's
// open or not, so its toggle can say what's new; what can be started is
// read here, while it's shown. The parts are BelowRun… (stories:
// Runs/Below).

type BelowRunsProps = {
  where: BelowWhere;
  // The services that are running, everywhere, the newest first.
  services: RunDetail[];
  // When Below was last closed.
  seen?: number;
  // What the page is of, if it's of a place that runs.
  place?: BelowPlace;
  reload: Reload;
  onEverywhere: () => void;
};

export function BelowRuns({ where, services, seen, place, reload, onEverywhere }: BelowRunsProps) {
  const missions = useWorkspace((w) => w.missions);
  const [busy, setBusy] = useState("");
  const here = where === "here";
  if (here && !place) {
    return (
      <BelowRunsNone
        title="No runs on this page"
        action={
          services.length > 0 && (
            <Button size="small" onClick={onEverywhere}>
              Show all {services.length} running
            </Button>
          )
        }
      >
        Runs show here on a mission or repo page.
      </BelowRunsNone>
    );
  }
  const shown = here && place ? ofPlace(services, place, missions) : services;
  const stop = (r: RunDetail) => {
    setBusy(r.ref);
    endRun(r.ref)
      .then(() => {
        toast(`Stopped ${r.name}`, { tone: "done" });
        return reload();
      })
      .catch((e: Error) => toast(humanError(e.message), { tone: "danger" }))
      .finally(() => setBusy(""));
  };
  const started = (r: RunDetail) => {
    toast(r.up ? `${r.name} is up on :${r.port}` : `${r.name} starting on :${r.port}`, { tone: "done" });
    return reload();
  };
  const of = here && place ? placeSaid(place, missions) : undefined;
  const start =
    here && place ? (
      "mission" in place ? (
        <MissionStart slug={place.mission} services={shown} started={started} />
      ) : (
        <WorktreeStart repo={place.repo} worktree={place.worktree} services={shown} started={started} />
      )
    ) : undefined;
  return (
    <>
      <BelowRunsHead of={of} count={shown.length} start={start} />
      {shown.length === 0 ? (
        <BelowRunsNone title={here ? "Nothing running here" : "Nothing running"}>
          {here ? "Start a service above, or with ws run in a terminal." : "Dev servers and previews show here once they start."}
        </BelowRunsNone>
      ) : (
        <BelowRunsList>
          {shown.map((r) => (
            <BelowRunRow
              key={r.ref}
              name={r.name}
              link={r.link || undefined}
              host={r.link ? bare(r.link) : undefined}
              up={!!r.up}
              fresh={isNew(r, seen)}
              place={place && here && "repo" in place ? undefined : runsAt(r)}
              to={jumpTo(r)}
              started={r.started}
              busy={busy === r.ref}
              onStop={() => stop(r)}
            />
          ))}
        </BelowRunsList>
      )}
    </>
  );
}

// runsAt is where a run runs, as its row says it: on one worktree's page
// that goes without saying.
const runsAt = (r: RunDetail) => (r.repo ? `${r.repo} / ${r.worktree}` : "workspace");

type StartProps = { services: RunDetail[]; started: (r: RunDetail) => Promise<void> };

// useStarting is starting one of a place's services, and which is.
function useStarting(run: (id: string) => Promise<RunDetail>, started: (r: RunDetail) => Promise<void>) {
  const [busy, setBusy] = useState<string>();
  const start = (id: string) => {
    setBusy(id);
    run(id)
      .then(started)
      .catch((e: Error) => toast(humanError(e.message), { tone: "danger" }))
      .finally(() => setBusy(undefined));
  };
  return { busy, start };
}

// running is whether a worktree has a recipe's service running.
const running = (services: RunDetail[], repo: string, worktree: string, recipe: string) => services.some((r) => r.repo === repo && r.worktree === worktree && r.recipe === recipe);

// MissionStart is Start on a mission's page: its capsules' services, a
// group each, as its Work tab's Run has them.
function MissionStart({ slug, services, started }: StartProps & { slug: string }) {
  const { value: d } = useMissionDetail(slug);
  const { busy, start } = useStarting((id) => {
    const [repo, capsule, recipe] = id.split("\n");
    return runRecipe(slug, repo, capsule, recipe);
  }, started);
  const groups: StartGroup[] = (d?.capsules ?? [])
    .filter((c) => c.exists)
    .map((c) => {
      const tasks = new Set(c.tasks ?? []);
      return {
        title: `${c.repo} / ${c.name}`,
        recipes: (c.recipes ?? []).filter((n) => !tasks.has(n) && !running(services, c.repo, c.name, n)).map((n) => ({ id: `${c.repo}\n${c.name}\n${n}`, name: n })),
      };
    });
  return <BelowRunsStart groups={groups} busy={busy} onStart={start} />;
}

// WorktreeStart is Start on a repo's page: its ground's or capsule's
// services.
function WorktreeStart({ repo, worktree, services, started }: StartProps & { repo: string; worktree: string }) {
  const { value } = useLoaded<WorktreeRunsInfo>(() => getWorktreeRuns(repo, worktree), [repo, worktree]);
  const { busy, start } = useStarting((recipe) => runInWorktree(repo, worktree, recipe), started);
  const recipes = (value?.recipes ?? []).filter((rc) => rc.type !== "task" && !running(services, repo, worktree, rc.name)).map((rc) => ({ id: rc.name, name: rc.name, title: rc.title }));
  return <BelowRunsStart groups={[{ recipes }]} busy={busy} onStart={start} />;
}
