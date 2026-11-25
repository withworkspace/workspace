import type { ReactNode } from "react";
import type { Repo } from "../api";
import { Chip } from "../ui/Chip";
import { cx } from "../ui/cx";
import css from "./SettingsRepoRow.module.css";

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
      <td className="mono">
        <span className={css.name}>
          <span className={css.key}>{r.name}</span>
          {r.yours && <Chip title="In ws.local.toml alone">yours</Chip>}
          {r.cloning && <Chip tone="progress">cloning…</Chip>}
          {!r.cloned && !r.cloning && !r.hidden && (
            <Chip tone={r.problem ? "danger" : "quiet"} title={r.problem ? `${r.problem} (ws setup tries again)` : "ws setup clones it"}>
              not cloned
            </Chip>
          )}
        </span>
      </td>
      <td>
        <span className={css.called}>
          <span className="repo-dot" style={{ background: r.color }} />
          {r.label}
        </span>
      </td>
      <td className={cx("mono", "dim", css.github)}>
        {r.github ? (
          <a href={`https://github.com/${r.github}`} target="_blank" rel="noreferrer">
            {r.github}
          </a>
        ) : (
          "—"
        )}
      </td>
      <td className="mono dim">{r.aliases?.join(", ") || "—"}</td>
      <td>{group}</td>
      <td className="dim">{lands}</td>
      {acts && <td className={css.acts}>{children}</td>}
    </tr>
  );
}
