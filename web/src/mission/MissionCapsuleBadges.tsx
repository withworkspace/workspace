import type { CapsuleDetail } from "../api";
import { Chip, chipClass } from "../ui/Chip";
import { Icon } from "../ui/Icon";
import css from "./MissionCapsuleBadges.module.css";

// MissionCapsuleBadges is where a capsule stands, as chips in a row:
// landed or burned, what's to land where its repo has no pull requests,
// uncommitted changes, ahead and behind, its pull request and its checks.
export function MissionCapsuleBadges({ c }: { c: CapsuleDetail }) {
  return (
    <span className={css.badges}>
      {c.landed ? (
        <Chip tone="done" title="Pushed straight to default branch: repo has no pull requests">
          Landed
        </Chip>
      ) : (
        !c.exists && <Chip tone="quiet">Burned</Chip>
      )}
      {c.direct && c.exists && !c.landed && (
        <Chip tone={c.toLand ? "waiting" : "quiet"} title="Repo has no pull requests: commits land straight on the default branch when you land them (from review, or ws land)">
          {c.toLand ? `${c.toLand} to land` : "No pull request"}
        </Chip>
      )}
      {c.dirty && (
        <Chip tone="attention" title="Uncommitted changes">
          Uncommitted changes
        </Chip>
      )}
      {!!c.ahead && <Chip tone="quiet" title={`${c.ahead} commits not on the default branch`}>{c.ahead} ahead</Chip>}
      {!!c.behind && <Chip tone="quiet" title={`${c.behind} commits behind the default branch`}>{c.behind} behind</Chip>}
      {c.pr && (
        <a href={c.pr.url} target="_blank" rel="noopener noreferrer" className={chipClass(c.pr.merged ? "done" : "waiting")} onClick={(e) => e.stopPropagation()} title={c.pr.title}>
          <Icon name="pr" size={11} />#{c.pr.number}
          {c.pr.merged ? " merged" : ""}
        </a>
      )}
      {c.pr?.checks && (
        <Chip tone={/fail/i.test(c.pr.checks) ? "danger" : /pass|success/i.test(c.pr.checks) ? "done" : "waiting"}>
          checks {c.pr.checks.toLowerCase()}
        </Chip>
      )}
    </span>
  );
}
