import { Link } from "react-router";
import { getProcess, type ProcessInfo } from "../api";
import { Markdown } from "../markdown/LazyMarkdown";
import { ErrorCard, Skeleton } from "../ui/kit";
import { useLoaded } from "../load";

// The workspace's process, on the Settings page: PROCESS.md as it's
// written (how work goes here, in prose: the team's, in the config repo),
// and whether it's as committed. The planner and the orchestrator read it
// as it is; what ws holds itself, whatever they read, is a guarantee, with
// the missions' settings.

// ProcessAbout is what the process is, under the section's name.
export function ProcessAbout() {
  return (
    <>
      How work goes here, in prose: the steps work takes, whose turn each is, what done means, and when it's yours. It's the team's, in the config repo, changed by commit
      like the rest of it. The planner reads it as it's written to plan a mission, and the orchestrator to know what to do when something happens. What ws holds whatever
      they read is under{" "}
      <Link to="/settings/missions#missions" className="link">
        Guarantees
      </Link>
      .
    </>
  );
}

export function ProcessFile() {
  const { value: p, error, reload } = useLoaded<ProcessInfo>(getProcess, [], { keep: true });
  return (
    <section className="settings-block process-section">
      {error && <ErrorCard error={error} retry={reload} />}
      {!p && !error && <Skeleton rows={4} />}
      {p && <Process p={p} />}
    </section>
  );
}

function Process({ p }: { p: ProcessInfo }) {
  if (!p.text.trim())
    return (
      <div className="process-none dim">
        There's no process yet: write how work goes here in <code>{p.name}</code>, beside <code>ws.toml</code> (or name another file with <code>[agents] process</code>).
      </div>
    );
  return (
    <>
      <div className="process-bar">
        <span className="process-where">
          <code>{p.name}</code>
          <span className="dim">
            {p.commit && !p.uncommitted && <> · as committed in {p.commit}</>}
            {p.commit && p.uncommitted && <> · changed since {p.commit}, not committed</>}
            {!p.commit && <> · not committed</>}
          </span>
        </span>
      </div>
      <div className="process-file">
        <div className="prose">
          <Markdown text={p.text} />
        </div>
      </div>
    </>
  );
}
