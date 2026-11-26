import { cx } from "../ui/cx";
import type { ReactNode } from "react";
import type { Repo } from "../api";
import { Chip } from "../ui/Chip";
import css from "./SettingsRepoRow.module.css";
import { Text } from "../ui/Text";
import { settingsTableQuietClass } from "./SettingsTable";
import { RepoDot } from "../ui/RepoDot";

type SettingsRepoRowProps = {
  repo: Repo;
  // How it lands, in words.
  lands: string;
  // Its group, picked in the row.
  group: ReactNode;
  // Whether the table has a column for what's done to a repo, and what's
  // in this row's.
  acts?: boolean;
  children?: ReactNode;
};

// SettingsRepoRow is a repo in the repos table: its name here (with
// whether it's yours alone, and whether it's cloned), what it's called, on
// GitHub, its aliases, its group, and how it lands.
export function SettingsRepoRow({ repo: r, lands, group, acts, children }: SettingsRepoRowProps) {
  return (
    <tr>
      <Text as="td" mono>
        <span className={css.name}>
          <span className={css.key}>{r.name}</span>
          {r.yours && <Chip title="Only in ws.local.toml">yours</Chip>}
          {r.cloning && <Chip tone="progress">cloning…</Chip>}
          {!r.cloned && !r.cloning && !r.hidden && (
            <Chip tone={r.problem ? "danger" : "quiet"} title={r.problem ? `${r.problem} (ws setup retries)` : "Cloned by ws setup"}>
              not cloned
            </Chip>
          )}
        </span>
      </Text>
      <td>
        <span className={css.called}>
          <RepoDot color={r.color} />
          {r.label}
        </span>
      </td>
      <Text as="td" mono dim className={cx(css.github, settingsTableQuietClass)}>
        {r.github ? (
          <a href={`https://github.com/${r.github}`} target="_blank" rel="noreferrer">
            {r.github}
          </a>
        ) : (
          "—"
        )}
      </Text>
      <Text as="td" mono dim className={settingsTableQuietClass}>{r.aliases?.join(", ") || "—"}</Text>
      <td>{group}</td>
      <Text as="td" dim className={settingsTableQuietClass}>{lands}</Text>
      {acts && <td className={css.acts}>{children}</td>}
    </tr>
  );
}
