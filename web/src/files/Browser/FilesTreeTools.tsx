import { filesKeys } from "../keys";
import { everywhereKeys } from "../../shell/keys";
import { Button } from "../../ui/Button";
import { Count } from "../../ui/Count";
import { Icon } from "../../ui/Icon";
import { Segment, Segmented } from "../../ui/Segmented";
import css from "./FilesTreeTools.module.css";
import { Text } from "../../ui/Text";

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
        <Text dim className={css.count}>
          {all} {all === 1 ? "file" : "files"}
        </Text>
      ) : (
        <Segmented fill className={css.show} value={changedOnly ? "changed" : "all"} onChange={(v) => onShow(v === "changed")} aria-label="Show">
          <Segment value="changed" title="Changed files only" keys={filesKeys.changed} disabled={!canChange}>
            Changed <Count>{changed}</Count>
          </Segment>
          <Segment value="all" title="All files" keys={filesKeys.changed}>
            All <Count>{all}</Count>
          </Segment>
        </Segmented>
      )}
      <Button variant="ghost" size="small" square onClick={onFind} title="Find a file" keys={filesKeys.find} aria-label="Find a file">
        <Icon name="search" size={14} />
      </Button>
      {onSearch && (
        <Button variant="ghost" size="small" square onClick={onSearch} title="Search in files" keys={everywhereKeys.findCode} aria-label="Search in files">
          <Icon name="code" size={14} />
        </Button>
      )}
    </div>
  );
}
