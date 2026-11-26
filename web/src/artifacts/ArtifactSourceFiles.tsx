import type { ReactNode } from "react";
import { Icon } from "../ui/Icon";
import css from "./ArtifactSourceFiles.module.css";
import { Text } from "../ui/Text";

type ArtifactSourceFilesProps = {
  // What the list is called: "v2's files".
  label: string;
  // How many there are, and how much they come to: "240 KB".
  count: number;
  size: string;
  // The files: an ArtifactSourceFile each.
  children: ReactNode;
};

// ArtifactSourceFiles is the list of a version's files, beside the one
// being read: it stays in sight as the file scrolls. On a narrow window
// it's a row over the file, which scrolls sideways.
export function ArtifactSourceFiles({ label, count, size, children }: ArtifactSourceFilesProps) {
  return (
    <nav className={css.files} aria-label={label}>
      <Text as="div" dim className={css.head}>
        {count} files, {size}
      </Text>
      {children}
    </nav>
  );
}

type ArtifactSourceFileProps = {
  path: string;
  // How big it is, said: "12 KB".
  size: string;
  // The one being read.
  current?: boolean;
  // The version's page (index.html), which has the artifact's icon.
  page?: boolean;
  onPick: () => void;
};

// ArtifactSourceFile is one of them: its path, cut short from its start
// so its name stays in sight, and its size.
export function ArtifactSourceFile({ path, size, current, page, onPick }: ArtifactSourceFileProps) {
  return (
    <button
      // The one being read is kept in sight: on a narrow window the list is a row that scrolls.
      ref={current ? (el) => el?.scrollIntoView({ block: "nearest", inline: "nearest" }) : undefined}
      className={css.file}
      aria-current={current ? "true" : undefined}
      onClick={onPick}
      title={`${path}, ${size}`}
    >
      <Icon name={page ? "artifact" : "file"} size={12} />
      <span className={css.path}>{path}</span>
      <Text dim className={css.size}>{size}</Text>
    </button>
  );
}
