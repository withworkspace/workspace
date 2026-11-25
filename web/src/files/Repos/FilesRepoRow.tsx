import { Link } from "react-router";
import { cx } from "../../ui/cx";
import { RepoDot } from "../../ui/RepoDot";
import css from "./FilesRepoRow.module.css";

type FilesRepoRowProps = {
  to: string;
  // Its name to ws (for how it's set up), and as ws labels it.
  name: string;
  label: string;
  color?: string;
  capsules: number;
  // It's the one open.
  on?: boolean;
  // It's not been cloned yet: ws setup does that.
  cloned?: boolean;
};

// FilesRepoRow is a repo in the Files page's column: its colour, its
// label, and how many capsules it has.
export function FilesRepoRow({ to, name, label, color, capsules, on, cloned }: FilesRepoRowProps) {
  return (
    <Link to={to} className={cx(css.row, on && css.on, !cloned && css.uncloned)} title={cloned ? undefined : `Not cloned (ws setup ${name})`} aria-current={on ? "page" : undefined}>
      <RepoDot color={color} />
      <span className={css.label}>{label}</span>
      {capsules > 0 && (
        <span className={css.capsules} title={`${capsules} ${capsules === 1 ? "capsule" : "capsules"}`}>
          {capsules}
        </span>
      )}
    </Link>
  );
}
