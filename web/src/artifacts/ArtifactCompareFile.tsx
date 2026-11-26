import type { ReactNode } from "react";
import { Chip } from "../ui/Chip";
import { Icon } from "../ui/Icon";
import css from "./ArtifactCompareFile.module.css";
import { diffStatusWords } from "./words";
import { Text } from "../ui/Text";

type ArtifactCompareFileProps = {
  // Its path, said where there's more than the page to tell apart.
  path?: string;
  // added, deleted or modified, as a diff's file says it.
  status: string;
  // It isn't text: it says only that it differs.
  binary?: boolean;
  // How many lines went and came, and in which version each side is.
  deletions: number;
  additions: number;
  a: string;
  b: string;
  // Its diff's lines.
  children?: ReactNode;
};

// ArtifactCompareFile is one file's diff between two versions: a head
// that stays in sight (its path, and what each side lost and gained),
// over its lines.
export function ArtifactCompareFile({ path, status, binary, deletions, additions, a, b, children }: ArtifactCompareFileProps) {
  return (
    <div className={css.file}>
      <div className={css.head}>
        {path !== undefined && (
          <span className={css.path}>
            <Icon name="file" size={12} /> {path}
            {status !== "modified" && <Chip tone={status === "added" ? "done" : "quiet"}>{diffStatusWords[status] ?? status}</Chip>}
          </span>
        )}
        {binary ? (
          <Text dim>Not text: {status === "modified" ? "differs" : `was ${diffStatusWords[status] ?? status}`}.</Text>
        ) : (
          <>
            <span>
              <Text minus>−{deletions}</Text> in {a}
            </span>
            <span>
              <Text plus>+{additions}</Text> in {b}
            </span>
          </>
        )}
      </div>
      {children}
    </div>
  );
}
