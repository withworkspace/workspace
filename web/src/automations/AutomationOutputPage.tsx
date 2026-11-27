import { useState } from "react";
import { useParams } from "react-router";
import { type AutomationOutputInfo, getAutomationOutput, useEvents } from "../api";
import { useLoaded } from "../load";
import { Crumbs } from "../ui/Crumbs";
import { Empty } from "../ui/Empty";
import { ErrorCard } from "../ui/ErrorCard";
import { PageHead } from "../ui/PageHead";
import { Skeleton } from "../ui/Skeleton";
import { useTitle } from "../ui/useTitle";
import { AutomationOutputView } from "./AutomationOutputView";
import { AutomationsFrame } from "./AutomationsFrame";

// An automation's output on a page of its own (/automations/<name>/output):
// what its notifications open with open = "output", for pinning and
// finding. The latest unless an older version's asked for; read again as
// a run writes a new one.
export function AutomationOutputPage() {
  const { name = "" } = useParams();
  useTitle(name);
  const [version, setVersion] = useState<number>();
  const { value: info, error, reload } = useLoaded<AutomationOutputInfo>(() => getAutomationOutput(name, version), [name, version], { keep: true });
  useEvents("automations", { hello: reload, changed: () => version === undefined && reload() });
  return (
    <AutomationsFrame>
      <Crumbs steps={[{ to: `/automations/${encodeURIComponent(name)}`, label: name, keys: "esc" }]} />
      <PageHead title={`${name}'s page`} />
      {error && !info && <ErrorCard error={error} retry={reload} />}
      {!info && !error && <Skeleton rows={6} />}
      {info &&
        (info.output ? (
          <AutomationOutputView name={name} output={info.output} versions={info.versions.map((v) => v.version)} onVersion={setVersion} />
        ) : (
          <Empty icon="file" title="Nothing written yet">
            It writes its page each time it runs: this is where it shows.
          </Empty>
        ))}
    </AutomationsFrame>
  );
}
