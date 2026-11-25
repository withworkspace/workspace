import { getProcess, type ProcessInfo } from "../api";
import { useLoaded } from "../load";
import { Markdown } from "../markdown/LazyMarkdown";
import { ErrorCard } from "../ui/ErrorCard";
import { Skeleton } from "../ui/Skeleton";
import { SettingsBlock } from "./SettingsBlock";
import { SettingsProcess } from "./SettingsProcess";
import { SettingsProcessNone } from "./SettingsProcessNone";

// The workspace's process, on the Settings page: PROCESS.md as it's
// written (how work goes here, in prose: the team's, in the config repo),
// and whether it's as committed. The planner and the orchestrator read it
// as it is; what ws holds itself, whatever they read, is a guarantee, with
// the missions' settings.
export function ProcessFile() {
  const { value: p, error, reload } = useLoaded<ProcessInfo>(getProcess, [], { keep: true });
  return (
    <SettingsBlock>
      {error && <ErrorCard error={error} retry={reload} />}
      {!p && !error && <Skeleton rows={4} />}
      {p &&
        (p.text.trim() ? (
          <SettingsProcess name={p.name} commit={p.commit} uncommitted={p.uncommitted}>
            <Markdown text={p.text} />
          </SettingsProcess>
        ) : (
          <SettingsProcessNone name={p.name} />
        ))}
    </SettingsBlock>
  );
}
