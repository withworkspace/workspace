import { useEffect, useRef, useState } from "react";
import { Link } from "react-router";
import { checkUpdates, getInstalledPlugins, type PluginsInstalled } from "../../api";
import { useLoaded } from "../../load";
import { Empty } from "../../ui/Empty";
import { ErrorCard } from "../../ui/ErrorCard";
import { GroupHead } from "../../ui/GroupHead";
import { Skeleton } from "../../ui/Skeleton";
import { humanError } from "../../ui/text";
import { toast } from "../../ui/toast";
import { usePlugins } from "../../workspace/store";
import { outCount } from "../installed";
import { AddPlugin } from "./AddPlugin";
import type { Failed } from "./failed";
import { InstalledRow } from "./InstalledRow";
import { PluginsFailed } from "./PluginsFailed";
import { PluginsList } from "./PluginsList";
import { PluginsNote } from "./PluginsNote";
import { PluginsUpdates } from "./PluginsUpdates";
import { WantedRow } from "./WantedRow";

// Plugins, a section of Settings: the workspace's plugins, each with where it's from,
// where it stands and what it adds; adding one, from the repo it's
// published at or a folder on this machine; updating one to its repo's
// latest release; and removing one. It's what ws plugin add, update and
// remove do. The daemon looks once a day for later releases, and what it
// found is said on each row; Check for updates looks now.

export function PluginsSection() {
  const live = usePlugins();
  const { value: info, error, reload } = useLoaded<PluginsInstalled>(getInstalledPlugins, [], { keep: true });
  // What the sidebar follows says when a plugin starts, stops, comes or
  // goes: the list is read again then.
  const stamp = (live ?? []).map((p) => `${p.name}:${p.state}:${p.starts}`).join(" ");
  const first = useRef(true);
  useEffect(() => {
    if (first.current) first.current = false;
    else void reload();
  }, [stamp, reload]);

  const [problem, setProblem] = useState<Failed>();
  const [looking, setLooking] = useState(false);
  const look = () => {
    setLooking(true);
    checkUpdates()
      .then(async (up) => {
        await reload();
        toast(up.plugins > 0 ? `${up.plugins} ${up.plugins === 1 ? "plugin has" : "plugins have"} a later release` : "No plugin has a later release", { tone: "done" });
      })
      .catch((e: Error) => toast(`Couldn't look: ${humanError(e.message)}`, { tone: "danger", ms: 8000 }))
      .finally(() => setLooking(false));
  };
  const installed = info?.plugins ?? [];
  const out = outCount(installed);
  const fromRepo = installed.some((p) => p.from.repo);

  return (
    <div>
      <AddPlugin reload={reload} onFailed={setProblem} />
      {info && fromRepo && <PluginsUpdates out={out} checked={info.checked} looking={looking} onLook={look} />}
      {problem && <PluginsFailed what={problem.what} error={problem.error} output={problem.output} onDismiss={() => setProblem(undefined)} />}
      {error && !info && <ErrorCard error={error} retry={reload} />}
      {!info && !error && <Skeleton rows={4} />}
      {info && (
        <>
          <section>
            <GroupHead count={installed.length}>
              Installed
            </GroupHead>
            {installed.length === 0 ? (
              <Empty icon="box" title="No plugins yet">
                Add one above: <code>withworkspace/plugin-plugin-dev</code> writes a plugin of your own, and runs it here as you change it.
              </Empty>
            ) : (
              <PluginsList>
                {installed.map((p) => (
                  <InstalledRow key={p.name} p={p} icon={live?.find((l) => l.name === p.name)?.views[0]?.icon} reload={reload} onFailed={setProblem} />
                ))}
              </PluginsList>
            )}
          </section>
          {info.wanted.length > 0 && (
            <section>
              <GroupHead count={info.wanted.length}>
                The team's, not installed here
              </GroupHead>
              <PluginsNote>
                In <code>ws.toml</code>, under <code>[plugins]</code>.
              </PluginsNote>
              <PluginsList>
                {info.wanted.map((w) => (
                  <WantedRow key={w.name} w={w} reload={reload} onFailed={setProblem} />
                ))}
              </PluginsList>
            </section>
          )}
          {info.checks === false && fromRepo && (
            <PluginsNote>
              The daemon isn't looking for updates by itself: <Link to="/settings/about#updates">Look for updates</Link> is off.
            </PluginsNote>
          )}
        </>
      )}
    </div>
  );
}

