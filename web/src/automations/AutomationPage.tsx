import { useState } from "react";
import { useNavigate, useParams } from "react-router";
import {
  type AutomationDryRun,
  type AutomationPage as AutomationInfo,
  dryRunAutomation,
  getAutomation,
  removeAutomation,
  setAutomationOn,
  useEvents,
} from "../api";
import { useLoaded } from "../load";
import { Button } from "../ui/Button";
import { Callout } from "../ui/Callout";
import { confirm } from "../ui/Confirm";
import { Crumbs } from "../ui/Crumbs";
import { Empty } from "../ui/Empty";
import { ErrorCard } from "../ui/ErrorCard";
import { GroupHead } from "../ui/GroupHead";
import { Icon } from "../ui/Icon";
import { PageHead } from "../ui/PageHead";
import { Skeleton } from "../ui/Skeleton";
import { Switch } from "../ui/Switch";
import { Text } from "../ui/Text";
import { Time } from "../ui/Time";
import { toast } from "../ui/toast";
import { useTitle } from "../ui/useTitle";
import { AutomationFacts } from "./AutomationFacts";
import { AutomationRunItem } from "./AutomationRunItem";
import { AutomationRuns } from "./AutomationRuns";
import { AutomationScript } from "./AutomationScript";
import { AutomationsFrame, AutomationsSection } from "./AutomationsFrame";
import { standing, tallyWords } from "./automationWords";
import { DryRunShown } from "./DryRunShown";

// An automation's page (/automations/<name>): your words for it, when it
// runs and what it may do, on or off; its latest runs, each with what it
// did; what it would have done over the last 30 days, dry-run on asking;
// its script; and Remove. Changing it is a conversation's: it proposes
// the change, checked and dry-run, as it proposed it.
export function AutomationPage() {
  const { name = "" } = useParams();
  useTitle(name);
  const navigate = useNavigate();
  const { value: info, error, reload, set } = useLoaded<AutomationInfo>(() => getAutomation(name), [name], { keep: true });
  useEvents("automations", { hello: reload, changed: reload });
  const [busy, setBusy] = useState(false);
  const [dry, setDry] = useState<AutomationDryRun>();
  const [dryError, setDryError] = useState("");
  const [drying, setDrying] = useState(false);
  const gone = error && !info;

  const turn = (on: boolean) => {
    setBusy(true);
    set((was) => was && { ...was, automation: { ...was.automation, on } });
    setAutomationOn(name, on)
      .then(() => toast(on ? `${name} is on: it runs from the next event` : `${name} is off`))
      .catch((e: Error) => {
        toast(`Couldn't turn ${name} ${on ? "on" : "off"}: ${e.message}`, { tone: "danger" });
        void reload();
      })
      .finally(() => setBusy(false));
  };
  const dryRun = () => {
    setDrying(true);
    setDryError("");
    dryRunAutomation(name)
      .then(setDry, (e: Error) => setDryError(e.message))
      .finally(() => setDrying(false));
  };
  const remove = async () => {
    const yes = await confirm({
      title: `Remove ${name}?`,
      body: "Its script and its runs go. What it did stays: its notifications, the conversations it started.",
      confirm: "Remove it",
      danger: true,
    });
    if (!yes) return;
    setBusy(true);
    removeAutomation(name)
      .then(() => {
        toast(`Removed ${name}`);
        navigate("/automations");
      })
      .catch((e: Error) => {
        toast(`Couldn't remove ${name}: ${e.message}`, { tone: "danger" });
        setBusy(false);
      });
  };

  const a = info?.automation;
  const wrong = a && standing(a);
  const tally = a ? tallyWords(a.tally) : [];
  return (
    <AutomationsFrame>
      <Crumbs steps={[{ to: "/automations", label: "Automations", keys: "esc" }]} />
      <PageHead
        title={name}
        actions={
          a && (
            <>
              <Switch on={a.on} onChange={turn} disabled={busy || !!a.broken} aria-label={`${name} on`} />
              <Text dim>{a.on ? "On" : "Off"}</Text>
              <Button variant="ghost" onClick={remove} disabled={busy}>
                <Icon name="x" size={12} /> Remove
              </Button>
            </>
          )
        }
      />
      {gone && (error.includes("no automation") || error.includes("not found") ? (
        <Empty icon="bolt" title={`There's no automation called ${name}`}>
          It may have been removed. Your automations are on the Automations page.
        </Empty>
      ) : (
        <ErrorCard error={error} retry={reload} />
      ))}
      {!info && !error && <Skeleton rows={4} />}
      {info && a && (
        <>
          {a.about && <p>{a.about}</p>}
          {wrong && <Callout tone="danger">{wrong.words}</Callout>}
          <AutomationFacts when={a.when} may={a.may}>
            <dt>Added</dt>
            <dd>
              <Time iso={a.added} />
            </dd>
          </AutomationFacts>
          <AutomationsSection>
            <GroupHead count={tally.length > 0 ? tally.join(" · ") : undefined}>Runs</GroupHead>
            {info.runs.length === 0 ? (
              <Text as="p" dim>
                It hasn't run yet: it runs on the next event it's for{a.on ? "" : ", once it's on"}.
              </Text>
            ) : (
              <AutomationRuns>
                {info.runs.map((r) => (
                  <AutomationRunItem key={r.n} r={r} />
                ))}
              </AutomationRuns>
            )}
          </AutomationsSection>
          <AutomationsSection>
            <GroupHead>Dry run</GroupHead>
            {dry ? (
              <DryRunShown dr={dry} />
            ) : (
              <Text as="p" dim>
                What it would have done over the last 30 days, if it had been on, with nothing sent anywhere.
              </Text>
            )}
            {dryError && <Callout tone="danger">{dryError}</Callout>}
            <div>
              <Button onClick={dryRun} disabled={drying || !!a.broken} aria-busy={drying}>
                <Icon name="play" size={11} /> {dry ? "Dry run again" : "Dry run"}
              </Button>
            </div>
          </AutomationsSection>
          <AutomationsSection>
            <AutomationScript source={info.source} open={!!a.broken} />
            <Text as="p" dim>
              To change it, say what you want in a conversation: it proposes the change, with a dry run, for you to
              turn on.
            </Text>
          </AutomationsSection>
        </>
      )}
    </AutomationsFrame>
  );
}
