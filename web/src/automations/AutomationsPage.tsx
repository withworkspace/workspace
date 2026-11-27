import { useState } from "react";
import { Link } from "react-router";
import { type AutomationsInfo, getAutomations, setAutomationOn, useEvents } from "../api";
import { useLoaded } from "../load";
import { buttonClass } from "../ui/Button";
import { Empty } from "../ui/Empty";
import { ErrorCard } from "../ui/ErrorCard";
import { Icon } from "../ui/Icon";
import { PageHead } from "../ui/PageHead";
import { Rows } from "../ui/Rows";
import { Skeleton } from "../ui/Skeleton";
import { toast } from "../ui/toast";
import { useTitle } from "../ui/useTitle";
import { AutomationRow } from "./AutomationRow";
import { AutomationsFrame, AutomationsLede } from "./AutomationsFrame";

// The Automations page: what ws does by itself when something happens
// (internal/automation), each of yours with how it's going, on or off.
// One's page (AutomationPage) has its runs, its script, a dry run, and
// Remove. Writing one is a conversation's: say what you want, and it
// proposes one you turn on from its card. Read again as the server says
// they've moved (the automations topic): a run, one turned on or off.
export function AutomationsPage() {
  useTitle("Automations");
  const { value: info, error, reload, set } = useLoaded<AutomationsInfo>(getAutomations, [], { keep: true });
  useEvents("automations", { hello: reload, changed: reload });
  const [busy, setBusy] = useState("");
  const turn = (name: string, on: boolean) => {
    setBusy(name);
    // Shown at once; the server's answer (and its next telling) says so.
    set((was) => was && { automations: was.automations.map((a) => (a.name === name ? { ...a, on } : a)) });
    setAutomationOn(name, on)
      .then(() => toast(on ? `${name} is on: it runs from the next event` : `${name} is off`))
      .catch((e: Error) => {
        toast(`Couldn't turn ${name} ${on ? "on" : "off"}: ${e.message}`, { tone: "danger" });
        void reload();
      })
      .finally(() => setBusy(""));
  };
  const list = info?.automations ?? [];
  const newOne = (
    <Link to="/conversations?new=1" className={buttonClass({ variant: "primary" })}>
      <Icon name="plus" size={12} /> New automation
    </Link>
  );
  return (
    <AutomationsFrame>
      <PageHead title="Automations" actions={list.length > 0 && newOne} />
      <AutomationsLede>
        What ws does by itself when something happens: tell you, or start a conversation to look into it. Say what
        you want in a conversation, and it proposes one for you to turn on.
      </AutomationsLede>
      {error && !info && <ErrorCard error={error} retry={reload} />}
      {!info && !error && <Skeleton rows={3} />}
      {info &&
        (list.length === 0 ? (
          <Empty icon="bolt" title="No automations yet" action={newOne}>
            Ask for one in a conversation, in your own words: “Tell me when Brendan pushes a new drawing”, “Start a
            conversation when CI fails on main”.
          </Empty>
        ) : (
          <Rows>
            {list.map((a) => (
              <AutomationRow key={a.name} a={a} onOn={(on) => turn(a.name, on)} busy={busy === a.name} />
            ))}
          </Rows>
        ))}
    </AutomationsFrame>
  );
}
