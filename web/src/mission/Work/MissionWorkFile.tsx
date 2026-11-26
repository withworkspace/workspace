import { Link, type To } from "react-router";
import { cx } from "../../ui/cx";
import css from "./MissionWorkFile.module.css";
import { MissionWorkLines } from "./MissionWorkLines";
import { Text } from "../../ui/Text";

const letters: Record<string, string> = { added: "A", deleted: "D", modified: "M", renamed: "R", copied: "C", type: "T" };
const statuses: Record<string, string> = { added: css.added, deleted: css.deleted };

type MissionWorkFileProps = {
  // Where it opens: nothing, for a file that's gone.
  to?: To;
  // How it changed: added, deleted, modified, renamed, copied, type.
  status: string;
  // Its path under its group's folder: the folders it's in (dir, with
  // the slash that ends it), and its name.
  dir: string;
  name: string;
  additions: number;
  deletions: number;
  // It has changes that aren't committed, where others of the capsule are.
  loose?: boolean;
  // One to skim (generated, a lock file): it's quieter.
  skim?: boolean;
  title?: string;
};

// MissionWorkFile is one of a capsule's changed files, in a line: a
// letter for how it changed, its path (cut short from the left, so it
// keeps its name), and its lines. It opens the file in the capsule.
export function MissionWorkFile({ to, status, dir, name, additions, deletions, loose, skim, title }: MissionWorkFileProps) {
  const body = (
    <>
      <span className={cx(css.st, statuses[status])}>{letters[status] ?? "M"}</span>
      <span className={css.path}>
        <bdi>
          <Text dim>{dir}</Text>
          <span className={css.name}>{name}</span>
        </bdi>
      </span>
      {loose && <span className={css.loose} title="Not committed" />}
      <MissionWorkLines additions={additions} deletions={deletions} tally />
    </>
  );
  const cls = cx(css.file, status === "deleted" && css.gone, skim && css.skim);
  return to ? (
    <Link to={to} className={cls} title={title}>
      {body}
    </Link>
  ) : (
    <div className={cls} title={title}>
      {body}
    </div>
  );
}
