import type { RunDetail } from "../../api";
import { RunCame } from "../../runs/RunCame";
import { RunEnds } from "../../runs/RunEnds";
import { RunMark } from "../../runs/RunMark";
import { RunName } from "../../runs/RunName";
import { came, commitSaid } from "../../runs/runRows";
import { Button } from "../../ui/Button";
import { Icon } from "../../ui/Icon";
import { Spinner } from "../../ui/Spinner";
import { Time } from "../../ui/Time";
import { MissionWorkRun } from "./MissionWorkRun";
import { Text } from "../../ui/Text";

// WorkTaskLine is a task in a capsule: what it came to, at which commit,
// going to its run on the Runs page.
export function WorkTaskLine({ r, busy, stop }: { r: RunDetail; busy: boolean; stop: () => void }) {
  const live = r.state === "running";
  return (
    <MissionWorkRun
      mark={live ? <RunMark live up /> : <RunMark task={r.result} />}
      under={
        (r.commit || r.check) && (
          <RunEnds>
            {r.commit && <span title={r.commit}>At {commitSaid(r)}</span>}
            {r.commit && r.check && " · "}
            {r.check && (
              <>
                check {r.check.n} of step {r.check.step}
              </>
            )}
          </RunEnds>
        )
      }
      action={
        live && (
          <Button variant="ghost" size="small" disabled={busy} onClick={stop} title="Stop now">
            {busy ? <Spinner size={11} /> : <Icon name="stop" size={11} />} Stop
          </Button>
        )
      }
    >
      <RunName to={`/runs?show=${encodeURIComponent(r.nref ?? r.ref)}`} title={`${r.command}: output and reports`}>
        {r.name}
      </RunName>
      <RunCame result={r.result}>
        {came(r)}
        {!live && r.ended && (
          <Text dim>
            , <Time iso={r.ended} as="relative" />
          </Text>
        )}
      </RunCame>
    </MissionWorkRun>
  );
}
