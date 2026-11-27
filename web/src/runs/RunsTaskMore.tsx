import { useState } from "react";
import { getRunPast, runInWorktree, type RunDetail } from "../api";
import { useLoaded } from "../load";
import { Copyable } from "../ui/Copyable";
import { Time } from "../ui/Time";
import { RunCame } from "./RunCame";
import { RunOutput } from "./RunOutput";
import { came, commitSaid, took } from "./runRows";
import { RunsPageCache } from "./RunsPageCache";
import { RunsPageFacts } from "./RunsPageFacts";
import { RunsPageMore, runsPageLogClass } from "./RunsPageMore";
import { RunsPagePast, RunsPagePastRun } from "./RunsPagePast";
import { RunsReports } from "./RunsReports";
import { Text } from "../ui/Text";

// RunsTaskMore is a task's run opened: the runs of it that are kept, the
// latest first, one of them picked; that one's facts, and all it printed.
// show, if it names a run of this task (ref#n), picks it. A cached one
// says what the cache made of it, and its latest, where it was reused,
// can be run fresh.
export function RunsTaskMore({ r, by, show }: { r: RunDetail; by?: string; show: string }) {
  const live = r.state === "running";
  const { value } = useLoaded(() => getRunPast(r.ref), [r.ref, r.n, r.state], { keep: true });
  const all = [r, ...(value?.past ?? [])];
  const [picked, setPicked] = useState(show.includes("#") ? show : (r.nref ?? r.ref));
  const one = all.find((x) => x.nref === picked) ?? r;
  const [busy, setBusy] = useState(false);
  // Run fresh is asked of its worktree, as the Files page asks: a silo's
  // and the workspace's own runs aren't asked for there.
  const worktree = r.worktree === "ground" ? ".ground" : r.worktree;
  const canFresh = one.nref === r.nref && !!one.reused && !!r.recipe && !!r.repo && r.worktree !== "silo" && !live;
  const fresh = () => {
    setBusy(true);
    runInWorktree(r.repo, worktree, r.recipe!, true)
      .then((started) => setPicked(started.nref ?? started.ref))
      .finally(() => setBusy(false));
  };
  return (
    <RunsPageMore>
      {all.length > 1 && (
        <RunsPagePast label={`Runs of ${r.name}`}>
          {all.map((x) => (
            <RunsPagePastRun
              key={x.nref}
              n={x.n}
              result={x.result}
              at={x.ended ?? x.started}
              on={x.nref === one.nref}
              onPick={() => setPicked(x.nref ?? x.ref)}
              title={`Run ${x.n}: ${came(x)}${x.commit ? `, at ${commitSaid(x)}` : ""}`}
            />
          ))}
        </RunsPagePast>
      )}
      <RunsPageFacts>
        <dt>Result</dt>
        <RunCame as="dd" result={one.result}>
          {came(one)}
          {one.ended && (
            <Text dim>
              , ended <Time iso={one.ended} />
            </Text>
          )}
        </RunCame>
        <dt>Command</dt>
        <Text as="dd" mono>
          <Copyable text={one.command}>{one.command}</Copyable>
        </Text>
        {one.dir && (
          <>
            <dt>Folder</dt>
            <Text as="dd" mono>{one.dir}</Text>
          </>
        )}
        {one.commit && (
          <>
            <dt>Commit</dt>
            <dd>
              <Text mono>{one.commit.slice(0, 12)}</Text>
              {one.dirty && <Text dim>, with uncommitted changes</Text>}
            </dd>
          </>
        )}
        <dt>Started</dt>
        <dd>
          <Time iso={one.started} />
          {by && <> by {by}</>}
          {one.recipe && (
            <>
              , from{" "}
              <code>
                [run.{one.type === "task" ? "task" : "service"}.{one.recipe}]
              </code>
            </>
          )}
        </dd>
        {(one.after?.length ?? 0) > 0 && (
          <>
            <dt>After</dt>
            <Text as="dd" mono>
              {one.after!.join(", ")}
            </Text>
          </>
        )}
        {one.cached && (
          <RunsPageCache
            reused={one.reused && { from: one.reused.from, took: took(one.reused.took) || "under a second" }}
            kept={one.result === "passed" && !one.reused && !one.keepFailed}
            keepFailed={one.keepFailed}
            undeclared={one.undeclared}
            onFresh={canFresh ? fresh : undefined}
            busy={busy}
          />
        )}
        {(one.reports?.length ?? 0) > 0 && (
          <>
            <dt>Reports</dt>
            <dd>
              <RunsReports r={one} />
            </dd>
          </>
        )}
      </RunsPageFacts>
      <RunOutput runRef={one.nref ?? one.ref} live={live && one.nref === r.nref} className={runsPageLogClass} />
    </RunsPageMore>
  );
}
