import { Button } from "../ui/Button";
import { cx } from "../ui/cx";
import { Icon } from "../ui/Icon";
import { Menu, MenuItem, MenuText } from "../ui/Menu";
import css from "./MissionPlanLevel.module.css";

// A Work step's level: how hard the planner judged it, which picks the
// model its crew run on ([crew.levels]).
export type Level = "rote" | "normal" | "hard";

export const levels: Level[] = ["rote", "normal", "hard"];

const says: Record<Level, string> = {
  rote: "Rote: mechanical, a command shows it's done. Unmet, it goes again a level up",
  normal: "Normal",
  hard: "Hard: needs the most thought",
};

// MissionLevelBars is a level as three bars, as many filled as it's high.
export function MissionLevelBars({ level }: { level: Level }) {
  const n = levels.indexOf(level) + 1;
  return (
    <span className={css.bars} aria-hidden>
      {levels.map((l, i) => (
        <span key={l} className={cx(css.bar, i < n && css.on)} />
      ))}
    </span>
  );
}

// MissionPlanLevel is a Work step's level at a row's end: its bars, and
// its name where it isn't normal. With onChoose it's a menu of the
// levels, each with what crew run on at it (on), the step's own ticked:
// choosing one puts the step there.
export function MissionPlanLevel({ level, on, onChoose, busy }: { level: Level; on?: Partial<Record<Level, string>>; onChoose?: (l: Level) => void; busy?: boolean }) {
  const face = (
    <>
      <MissionLevelBars level={level} />
      {level !== "normal" && <span className={css.word}>{level}</span>}
    </>
  );
  const title = on?.[level] ? `${says[level]}. On ${on[level]}` : says[level];
  if (!onChoose) {
    return (
      <span className={cx(css.level, level === "hard" && css.hard)} title={title}>
        {face}
      </span>
    );
  }
  return (
    <Menu
      aria-label="The step's level"
      placement="bottom end"
      onAction={(id) => id !== level && onChoose(id as Level)}
      trigger={
        <Button variant="bare" className={cx(css.level, css.pick, level === "hard" && css.hard)} aria-label={`Level: ${level}. Change it`} aria-busy={busy} title={title}>
          {face}
        </Button>
      }
    >
      {levels.map((l) => (
        <MenuItem key={l} id={l} textValue={l}>
          <MissionLevelBars level={l} />
          <MenuText>
            <span className={css.name}>{l}</span>
            {on?.[l] && <span className={css.model}> on {on[l]}</span>}
          </MenuText>
          {l === level && <Icon name="check" size={12} />}
        </MenuItem>
      ))}
    </Menu>
  );
}
