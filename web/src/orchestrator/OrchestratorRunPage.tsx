import { useParams } from "react-router";
import { getOrchestratorRun, type OrchestratorRunDetail } from "../api";
import { useLoaded } from "../load";
import { Markdown } from "../markdown/LazyMarkdown";
import { Callout } from "../ui/Callout";
import { Calls } from "../ui/Calls";
import { Crumbs } from "../ui/Crumbs";
import { ErrorCard } from "../ui/ErrorCard";
import { GroupHead } from "../ui/GroupHead";
import { Inline } from "../ui/Inline";
import { PageHead } from "../ui/PageHead";
import { Skeleton } from "../ui/Skeleton";
import { Status } from "../ui/Status";
import { useTitle } from "../ui/useTitle";
import { duration } from "../ui/when";
import { useWorkspace } from "../workspace/store";
import { OrchestratorNone } from "./OrchestratorNone";
import { OrchestratorPrompt } from "./OrchestratorPrompt";
import { OrchestratorRunMeta } from "./OrchestratorRunMeta";
import { agentOf, lookedAt, outcome } from "./words";
import { Prose } from "../ui/Prose";

// A run of the orchestrator's: what it was told, what it said and every
// call it made, refused ones marked. This is what the page knows.
export function OrchestratorRunPage() {
  const { slug = "", id = "" } = useParams();
  const m = useWorkspace((w) => w.missions.find((m) => m.slug === slug));
  const root = useWorkspace((w) => w.root);
  useTitle(`Orchestrator run${m ? ` · ${m.title}` : ""}`);
  const { value: d, error, reload: load } = useLoaded<OrchestratorRunDetail>(() => getOrchestratorRun(slug, id), [slug, id]);
  return (
    <div className="page narrow">
      <Crumbs
        steps={[
          { to: `/missions/${slug}`, label: m?.title ?? slug },
          { to: "/orchestrator", label: "All runs" },
        ]}
      />
      <PageHead title="Orchestrator run">{d && <Status s={outcome(d)} />}</PageHead>
      {error && <ErrorCard error={error} retry={load} />}
      {!d && !error && <Skeleton rows={6} />}
      {d && (
        <>
          <OrchestratorRunMeta on={lookedAt(d)} started={d.started} took={duration(d.started, d.ended)} agent={agentOf(d)} />
          {d.retry && (
            <Callout tone="quiet" icon="rotate" title="A second try">
              The run before it ran no <code>ws mission</code> commands, so it was run again and told its shell works.
            </Callout>
          )}
          {d.error && (
            <Callout tone="danger" title="It failed">
              <Inline text={d.error} />
            </Callout>
          )}
          <section>
            <GroupHead>What it said</GroupHead>
            {d.output ? (
              <Prose>
                <Markdown text={d.output} />
              </Prose>
            ) : (
              <OrchestratorNone>Nothing.</OrchestratorNone>
            )}
          </section>
          <section>
            <GroupHead count={d.log.length}>What it ran</GroupHead>
            {d.log.length === 0 ? <OrchestratorNone>No calls recorded: only Claude Code reports them.</OrchestratorNone> : <Calls calls={d.log} root={root} open />}
          </section>
          <section>
            <OrchestratorPrompt text={d.prompt} />
          </section>
        </>
      )}
    </div>
  );
}
