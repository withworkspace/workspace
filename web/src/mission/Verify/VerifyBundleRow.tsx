import type { Bundle } from "../../api";
import { Button } from "../../ui/Button";
import { Icon } from "../../ui/Icon";
import { checkRows, queued, standing, stillLeft } from "../verifyRows";
import { MissionVerifyBundle } from "./MissionVerifyBundle";
import { MissionVerifyBundles } from "./MissionVerifyBundles";
import { MissionVerifyStanding } from "./MissionVerifyStanding";
import { VerifyBundleMenu } from "./VerifyBundleMenu";
import { standingTone, stepTitle } from "./words";

// VerifyBundleRow is the mission's bundles in a line: those yours to
// approve, each with how many of its checks are left for you; the one
// that's open, when it isn't one of those (with a way to close it when
// there are none); and the rest behind a button.
export function VerifyBundleRow({ bundles, open, pick }: { bundles: Bundle[]; open: Bundle; pick: (id?: string) => void }) {
  const groups = queued(bundles);
  const todo = groups.find((g) => g.group === "todo")?.bundles ?? [];
  const rest = groups.filter((g) => g.group !== "todo");
  const others = rest.reduce((n, g) => n + g.bundles.length, 0);
  const chip = (b: Bundle, other = false) => (
    <MissionVerifyBundle
      key={b.id}
      id={b.id}
      title={stepTitle(b)}
      standing={<MissionVerifyStanding tone={standingTone(b)}>{standing(b)}</MissionVerifyStanding>}
      left={!b.verdict && !b.replacedBy ? stillLeft(checkRows(b)).length : 0}
      selected={b.id === open.id}
      other={other}
      onPick={() => pick(b.id)}
    />
  );
  return (
    <MissionVerifyBundles label={todo.length ? "To approve" : "Nothing to approve"} attention={todo.length > 0}>
      {todo.map((b) => chip(b))}
      {!todo.includes(open) && chip(open, true)}
      {!todo.length && (
        <Button variant="ghost" size="small" square type="button" onClick={() => pick()} title={`Close ${open.id}`} keys="esc" aria-label={`Close ${open.id}`}>
          <Icon name="x" size={13} />
        </Button>
      )}
      {others > 0 && <VerifyBundleMenu groups={rest} label={`${others} ${todo.length ? "other " : ""}${others === 1 ? "bundle" : "bundles"}`} pick={pick} end />}
    </MissionVerifyBundles>
  );
}
