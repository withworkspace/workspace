import type { RunDetail } from "../api";
import { Copyable } from "../ui/Copyable";
import { Time } from "../ui/Time";
import { RunOutput } from "./RunOutput";
import { bare } from "./runRows";
import { RunsPageFacts } from "./RunsPageFacts";
import { RunsPageMore, runsPageLogClass } from "./RunsPageMore";
import { Text, textClass } from "../ui/Text";

// RunsServiceMore is a service's run opened: how it was started, and its
// output, followed while it runs.
export function RunsServiceMore({ r, by }: { r: RunDetail; by?: string }) {
  const live = r.state === "running";
  return (
    <RunsPageMore>
      <RunsPageFacts>
        <dt>Command</dt>
        <Text as="dd" mono>
          <Copyable text={r.command}>{r.command}</Copyable>
        </Text>
        {r.dir && (
          <>
            <dt>Folder</dt>
            <Text as="dd" mono>{r.dir}</Text>
          </>
        )}
        <dt>Started</dt>
        <dd>
          <Time iso={r.started} />
          {by && <> by {by}</>}
          {r.recipe && (
            <>
              , from its{" "}
              <code>
                [run.{r.type === "task" ? "task" : "service"}.{r.recipe}]
              </code>
            </>
          )}
        </dd>
        {live && (
          <>
            <dt>Process</dt>
            <dd>
              pid {r.pid}, on port {r.port}
              {r.link !== r.local && (
                <>
                  {" "}
                  (<a href={r.local} target="_blank" rel="noopener noreferrer" className={textClass("mono")}>
                    {bare(r.local)}
                  </a>
                  )
                </>
              )}
            </dd>
          </>
        )}
      </RunsPageFacts>
      <RunOutput runRef={r.ref} live={live} className={runsPageLogClass} />
    </RunsPageMore>
  );
}
