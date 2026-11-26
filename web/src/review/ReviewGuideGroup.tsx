import { cx } from "../ui/cx";
import { Icon } from "../ui/Icon";
import { Inline } from "../ui/Inline";
import css from "./ReviewGuideGroup.module.css";
import { Text } from "../ui/Text";

// ReviewGuideGroup names a group of the guide's in the diff, above its
// first file: what its files are, how many, and why they're read here.
// One to skim is quiet.
export function ReviewGuideGroup({ label, files, skim, why }: { label: string; files: number; skim?: boolean; why?: string }) {
  return (
    <div className={cx(css.group, skim && css.skim)}>
      <h3>
        <Icon name="sparkle" size={13} className={css.icon} /> {label}
        <Text dim>
          {" "}
          · {files} {files === 1 ? "file" : "files"}
          {skim ? " · to skim" : ""}
        </Text>
      </h3>
      {why && (
        <p>
          <Inline text={why} />
        </p>
      )}
    </div>
  );
}
