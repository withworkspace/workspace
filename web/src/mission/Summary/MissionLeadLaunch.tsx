import type { CrewPick, CrewPicks } from "../../api";
import { Button } from "../../ui/Button";
import { Chip } from "../../ui/Chip";
import { cx } from "../../ui/cx";
import { Icon } from "../../ui/Icon";
import { Menu, MenuItem } from "../../ui/Menu";
import { pickName } from "./words";
import css from "./MissionLeadLaunch.module.css";
import { Text } from "../../ui/Text";

// MissionLeadLaunch is the Launch crew button: it starts crew on crew's
// default agent and model. Where [crew] allows others, it's a split
// button, whose menu starts them on one of those instead (or the default).
// again, it's crew after the first: Launch new crew.
export function MissionLeadLaunch({
  picks,
  primary,
  again,
  busy,
  launch,
}: {
  picks?: CrewPicks;
  primary: boolean;
  again: boolean;
  busy: boolean;
  launch: (p?: CrewPick) => void;
}) {
  const others = picks?.allowed ?? [];
  const main = (
    <Button variant={primary ? "primary" : undefined} disabled={busy} onClick={() => launch()} title={picks && others.length ? `On ${pickName(picks.default)}` : undefined}>
      <Icon name="play" /> Launch {again ? "new " : ""}crew
    </Button>
  );
  if (!picks || !others.length) return main;
  const all = [picks.default, ...others];
  // data-field: keys typed in the menu are its own, not the page's (keys.ts).
  return (
    <div className={css.split}>
      {main}
      <Menu
        placement="bottom end"
        aria-label="Launch crew on"
        onAction={(k) => launch(k === "0" ? undefined : all[Number(k)])}
        trigger={
          <Button variant={primary ? "primary" : undefined} className={cx(css.toggle, primary && css.primary)} disabled={busy} aria-label="Launch crew on another agent or model" data-field>
            <Icon name="chevron-down" size={13} />
          </Button>
        }
      >
        {all.map((p, i) => (
          <MenuItem key={pickName(p)} id={String(i)} textValue={pickName(p)}>
            <Text mono className={css.pick}>
              {p.harness}
              {p.model ? ` · ${p.model}` : ""}
            </Text>
            {!p.model && <Text dim>its own model</Text>}
            {i === 0 && <Chip>default</Chip>}
          </MenuItem>
        ))}
      </Menu>
    </div>
  );
}
