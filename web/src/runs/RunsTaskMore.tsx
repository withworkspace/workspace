import { useState } from "react";
import { getRunPast, type RunDetail } from "../api";
import { useLoaded } from "../load";
import { Copyable } from "../ui/Copyable";
import { Time } from "../ui/Time";
import { RunCame } from "./RunCame";
import { RunOutput } from "./RunOutput";
import { came, commitSaid } from "./runRows";
import { RunsPageFacts } from "./RunsPageFacts";
import { RunsPageMore, runsPageLogClass } from "./RunsPageMore";
import { RunsPagePast, RunsPagePastRun } from "./RunsPagePast";
import { RunsReports } from "./RunsReports";
import { Text } from "../ui/Text";

// RunsTaskMore is a task's run opened: the runs of it that are kept, the
// latest first, one of them picked; that one's facts, and all it printed.
// show, if it names a run of this task (ref#n), picks it.
export function RunsTaskMore({ r, by, show }: { r: RunDetail; by?: string; show: string }) {
  const live = r.state === "running";
  const { value } = useLoaded(() => getRunPast(r.ref), [r.ref, r.n, r.state], { keep: true });
  const all = [r, ...(value?.past ?? [])];
  const [picked, setPicked] = useState(show.includes("#") ? show : (r.nref ?? r.ref));
  const one = all.find((x) => x.nref === picked) ?? r;
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
              {one.dirty && <Text dim>, with changes that weren't committed</Text>}
            </dd>
          </>
        )}
        <dt>Started</dt>
        <dd>
          <Time iso={one.started} />
          {by && <> by {by}</>}
          {one.recipe && (
            <>
              , from its{" "}
              <code>
                [run.{one.type === "task" ? "task" : "service"}.{one.recipe}]
              </code>
            </>
          )}
        </dd>
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
