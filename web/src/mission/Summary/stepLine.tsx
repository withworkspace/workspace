import type { ReactNode } from "react";
import type { MissionDetail } from "../../api";
import { Chip } from "../../ui/Chip";
import { waitHow } from "../planRows";
import { Text } from "../../ui/Text";

// stepLine is the step the mission's on, as its lead says it over what's
// happening: which it is, and for a Wait, whether anything's watching.
export function stepLine(d: MissionDetail): ReactNode {
  const now = d.stages.find((s) => s.state === "current");
  return (
    now && (
      <>
        <span title={now.text}>
          {d.plan ? "Step" : "Stage"} {now.n} of {d.stages.length}: <strong>{now.title}</strong>
        </span>
        {now.checkIn && <Text dim> · check in after</Text>}
        {now.who === "you" && !waitHow(now) && <Text dim> · yours: only you pass it</Text>}
        {waitHow(now) && (
          <span>
            {" "}
            · <Chip tone={waitHow(now)!.auto ? "progress" : "attention"}>{waitHow(now)!.word}</Chip> <Text dim>{waitHow(now)!.says}</Text>
          </span>
        )}
      </>
    )
  );
}
