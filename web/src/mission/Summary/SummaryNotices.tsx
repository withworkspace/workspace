import type { MissionDetail } from "../../api";
import { usePref } from "../../prefs";
import { Button } from "../../ui/Button";
import { Callout } from "../../ui/Callout";
import { Inline } from "../../ui/Inline";
import { isNotice } from "./words";

// SummaryNotices is a harness's warning the orchestrator passed on (the
// latest), about the whole workspace rather than the mission: shown once,
// until it's dismissed.
export function SummaryNotices({ d }: { d: MissionDetail }) {
  const texts = [...new Set(d.events.filter((e) => isNotice(e.text)).map((e) => e.text.replace(/^orchestrator: /, "")))].slice(-1);
  const [dismissed, setDismissed] = usePref("dismissed");
  const shown = texts.filter((t) => !dismissed.includes(t.slice(0, 80)));
  if (!shown.length) return null;
  const dismiss = (t: string) => setDismissed((was) => [...was.filter((x) => x !== t.slice(0, 80)), t.slice(0, 80)].slice(-50));
  return (
    <>
      {shown.map((t) => {
        const signIn = /sign-in ends/i.test(t);
        return (
          <Callout
            key={t}
            tone="quiet"
            icon="alert"
            title={signIn ? "Agent sign-in expires soon" : "Orchestrator warning"}
            actions={
              <Button size="small" onClick={() => dismiss(t)}>
                Dismiss
              </Button>
            }
          >
            <Inline text={t.replace(/^(warning|notice): (\w)/i, (_, _w: string, c: string) => c.toUpperCase())} />
          </Callout>
        );
      })}
    </>
  );
}
