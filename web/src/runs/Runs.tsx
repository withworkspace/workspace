import { useSearchParams } from "react-router";
import { getRuns, type RunsInfo, useEvents } from "../api";
import { useLoaded } from "../load";
import { Chip } from "../ui/Chip";
import { Empty } from "../ui/Empty";
import { ErrorCard } from "../ui/ErrorCard";
import { GroupHead } from "../ui/GroupHead";
import { PageHead } from "../ui/PageHead";
import { Skeleton } from "../ui/Skeleton";
import { useTitle } from "../ui/useTitle";
import { split } from "./runRows";
import { RunsNames } from "./RunsNames";
import { RunsPageFrame } from "./RunsPageFrame";
import { RunsPageLede } from "./RunsPageLede";
import { RunsPageList } from "./RunsPageList";
import { RunsPageNote } from "./RunsPageNote";
import { RunsRow } from "./RunsRow";
import { RunsSiloRow } from "./RunsSiloRow";

// The Runs page: everything ws is running for someone in this workspace
// (ws run): dev servers, previews, compose stacks, whether you, crew or a
// conversation started them. Each with its link, who it's for (it ends
// when they go), its output, and Stop. Above them, the names it's reached
// by under .localhost (ws router), or how to turn them on.
//
// Those are services. A task is the other type of run: tests, lint, a
// build, which ends by itself. One that's running is with what's
// running; once it has ended it's a result, kept with all it printed,
// the commit it ran at, its reports, and the runs of it before.
// /runs?show=<ref> opens one, as a step's check links to its run.
//
// This is what the page knows: the runs and silos, read again as the
// server says they've moved, and which one the address asks for. What
// each is drawn as is RunsRow's and RunsSiloRow's; the parts are
// RunsPage… (stories: Runs/RunsPage).

export function RunsPage() {
  useTitle("Runs");
  const { value: info, error, reload } = useLoaded<RunsInfo>(getRuns, [], { keep: true });
  // Read again when a run starts, comes up, ends or is cleared away, or a
  // silo moves: the server says so (the runs topic).
  useEvents("runs", { hello: reload, changed: reload });
  const { running, results, ended } = split(info?.runs ?? []);
  const up = new Set(running.map((r) => r.ref));
  const [params] = useSearchParams();
  const show = params.get("show") ?? "";
  return (
    <RunsPageFrame>
      <PageHead title="Runs">
        {running.length > 0 && <Chip tone="done">{running.length} running</Chip>}
      </PageHead>
      <RunsPageLede>
        Dev servers, tests and builds run for you and your agents. Services get a free port and stop when no
        longer needed. Tasks end by themselves, and their results are kept.
      </RunsPageLede>
      {error && !info && <ErrorCard error={error} retry={reload} />}
      {!info && !error && <Skeleton rows={4} />}
      {info && (
        <>
          <RunsNames names={info.names} />
          <section>
            <GroupHead count={running.length}>
              Running
            </GroupHead>
            {running.length === 0 ? (
              <Empty icon="play" title="Nothing's running">
                Dev servers and tests started with <code>ws run</code> show here. Start one from a terminal,
                or with Run on a capsule in a mission's Work tab.
              </Empty>
            ) : (
              <RunsPageList>
                {running.map((r) => (
                  <RunsRow key={r.ref} r={r} up={up} reload={reload} show={show} />
                ))}
              </RunsPageList>
            )}
          </section>
          {results.length > 0 && (
            <section>
              <GroupHead count={results.length}>Results</GroupHead>
              <RunsPageNote>Latest result of each task, with output, reports and earlier runs. Kept until the capsule is gone.</RunsPageNote>
              <RunsPageList>
                {results.map((r) => (
                  <RunsRow key={r.ref} r={r} up={up} reload={reload} show={show} />
                ))}
              </RunsPageList>
            </section>
          )}
          {/* The repos' silos, as the daemon keeping them has them: what
              each is pointed at, its last sync, and its last hook. */}
          {info.silos.length > 0 && (
            <section>
              <GroupHead count={info.silos.length}>
                Silos
              </GroupHead>
              <RunsPageNote>
                Each mirrors a capsule, kept in sync as files change. <code>ws silo point</code> points one
                elsewhere.
              </RunsPageNote>
              <RunsPageList>
                {info.silos.map((st) => (
                  <RunsSiloRow key={st.repo} st={st} reload={reload} />
                ))}
              </RunsPageList>
            </section>
          )}
          {ended.length > 0 && (
            <section>
              <GroupHead count={ended.length}>
                Ended
              </GroupHead>
              <RunsPageNote>Services that exited on their own. Kept for an hour, with output.</RunsPageNote>
              <RunsPageList>
                {ended.map((r) => (
                  <RunsRow key={r.ref} r={r} up={up} reload={reload} show={show} />
                ))}
              </RunsPageList>
            </section>
          )}
        </>
      )}
    </RunsPageFrame>
  );
}
