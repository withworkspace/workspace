import { Link } from "react-router";
import type { CapsuleDetail, MissionDetail } from "../../api";
import { Button, buttonClass } from "../../ui/Button";
import { Icon } from "../../ui/Icon";
import { Time } from "../../ui/Time";
import type { WorkCapsule } from "../workRows";
import { MissionWorkNow } from "./MissionWorkNow";
import { bundleLink } from "./words";

// WorkNow is the step the mission's on, when it's in this capsule: whose
// it is, in a line, and what moves it on.
export function WorkNow({ d, x, review }: { d: MissionDetail; x: WorkCapsule; review: (c: CapsuleDetail) => void }) {
  const s = x.now!;
  const live = x.live && s.crew?.includes(x.live.id) ? x.live : undefined;
  // The bundle a step of yours looks at.
  const bundle = x.bundles.find((b) => b.asked && !b.verdict && !b.replacedBy);
  const tone = s.who === "you" || (s.opensPR && !x.c.pr) ? "attention" : "progress";
  return (
    <MissionWorkNow
      tone={tone}
      actions={
        <>
          {s.who === "you" && s.type === "Review" && x.c.exists && (
            <Button variant="primary" size="small" onClick={() => review(x.c)} keys="v">
              <Icon name="eye" /> Review changes
            </Button>
          )}
          {bundle && (
            <Link to={bundleLink(d.slug, bundle.id)} className={buttonClass({ size: "small" })}>
              <Icon name="list-checks" size={12} /> {bundle.id} in Verify
            </Link>
          )}
          {live && (
            <Link to={`/missions/${d.slug}/crew/${live.id}`} className={buttonClass({ size: "small" })}>
              <Icon name="terminal" size={12} /> Watch {live.id}
            </Link>
          )}
        </>
      }
    >
      <strong>
        Step {s.n} · {s.title}
      </strong>{" "}
      {s.who === "you" ? "is yours." : live ? <>has {live.id} on it, since <Time iso={live.started} as="clock" />.</> : s.who === "ws" ? "is Workspace's." : s.who === "crew" ? "waits for crew." : `is ${s.whoSays ?? s.who ?? "next"}.`}
    </MissionWorkNow>
  );
}
