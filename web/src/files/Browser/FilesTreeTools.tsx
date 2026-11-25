import { Button } from "../../ui/Button";
import { Count } from "../../ui/Count";
import { cx } from "../../ui/cx";
import { Icon } from "../../ui/Icon";
import { Segment, Segmented } from "../../ui/Segmented";
import css from "./FilesTreeTools.module.css";

type FilesTreeToolsProps = {
  // Every file, and what's changed of them since the branch point.
  all: number;
  changed: number;
  // Nothing to compare with (a repo's ground): its files are only counted.
  counted?: boolean;
  changedOnly: boolean;
  // There's a branch point to say what's changed since.
  canChange: boolean;
  // Where it left its base, said on hover.
  title?: string;
  onShow: (changedOnly: boolean) => void;
  onFind: () => void;
  // Search what the files say: a capsule's, whole.
  onSearch?: () => void;
};

// FilesTreeTools is the tree's head: what it shows (only what's changed,
// or every file), and finding a file or what the files say.
export function FilesTreeTools({ all, changed, counted, changedOnly, canChange, title, onShow, onFind, onSearch }: FilesTreeToolsProps) {
  return (
    <div className={css.tools} title={title}>
      {counted ? (
        <span className={cx("dim", css.count)}>
          {all} {all === 1 ? "file" : "files"}
        </span>
      ) : (
        <Segmented fill className={css.show} value={changedOnly ? "changed" : "all"} onChange={(v) => onShow(v === "changed")} aria-label="Show">
          <Segment value="changed" title="Only what's changed since the branch point (c)" disabled={!canChange}>
            Changed <Count>{changed}</Count>
          </Segment>
          <Segment value="all" title="Every file (c)">
            All <Count>{all}</Count>
          </Segment>
        </Segmented>
      )}
      <Button variant="ghost" size="small" square onClick={onFind} title="Find a file (t)" aria-label="Find a file">
        <Icon name="search" size={14} />
      </Button>
      {onSearch && (
        <Button variant="ghost" size="small" square onClick={onSearch} title="Search what the files say (⌘⇧F)" aria-label="Search what the files say">
          <Icon name="code" size={14} />
        </Button>
      )}
    </div>
  );
}
