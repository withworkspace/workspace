import { Button } from "../ui/Button";
import { cx } from "../ui/cx";
import { Segment, Segmented } from "../ui/Segmented";
import css from "./SettingsGroupRow.module.css";

type SettingsGroupRowProps = {
  name: string;
  // What it's called, where that isn't its name.
  label?: string;
  // Its repos, as they're called, in a line.
  repos: string;
  hidden?: boolean;
  // Shown or hidden is your choice, not the team's: what the team says is
  // beside it, with the way back.
  chosen?: boolean;
  teamHidden?: boolean;
  busy?: boolean;
  onSet: (hidden: boolean) => void;
  onReset: () => void;
};

// SettingsGroupRow is a group in the groups table: its name, its repos,
// and shown or hidden for you.
export function SettingsGroupRow({ name, label, repos, hidden, chosen, teamHidden, busy, onSet, onReset }: SettingsGroupRowProps) {
  return (
    <tr>
      <td>
        {label && label !== name ? (
          <span className={css.named}>
            {label} <span className="mono dim">{name}</span>
          </span>
        ) : (
          <span className="mono">{name}</span>
        )}
      </td>
      <td className="dim">{repos}</td>
      <td>
        <Segmented value={hidden ? "hidden" : "shown"} onChange={(v) => onSet(v === "hidden")} disabled={busy} aria-label={`${name}: shown or hidden`}>
          <Segment value="shown">Shown</Segment>
          <Segment value="hidden">Hidden</Segment>
        </Segmented>
      </td>
      <td className={cx("dim", css.team)}>
        {chosen ? (
          <>
            the team's: {teamHidden ? "hidden" : "shown"}
            <Button variant="link" className={css.reset} disabled={busy} onClick={onReset}>
              Reset
            </Button>
          </>
        ) : (
          "as the team has it"
        )}
      </td>
    </tr>
  );
}
