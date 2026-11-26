import { cx } from "../ui/cx";
import { Button } from "../ui/Button";
import { Segment, Segmented } from "../ui/Segmented";
import css from "./SettingsGroupRow.module.css";
import { Text } from "../ui/Text";
import { settingsTableQuietClass } from "./SettingsTable";

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
            {label} <Text mono dim className={settingsTableQuietClass}>{name}</Text>
          </span>
        ) : (
          <Text mono>{name}</Text>
        )}
      </td>
      <Text as="td" dim className={settingsTableQuietClass}>{repos}</Text>
      <td>
        <Segmented value={hidden ? "hidden" : "shown"} onChange={(v) => onSet(v === "hidden")} disabled={busy} aria-label={`${name} visibility`}>
          <Segment value="shown">Shown</Segment>
          <Segment value="hidden">Hidden</Segment>
        </Segmented>
      </td>
      <Text as="td" dim className={cx(css.team, settingsTableQuietClass)}>
        {chosen ? (
          <>
            the team's: {teamHidden ? "hidden" : "shown"}
            <Button variant="link" className={css.reset} disabled={busy} onClick={onReset}>
              Reset
            </Button>
          </>
        ) : (
          "team default"
        )}
      </Text>
    </tr>
  );
}
