import type { ReactNode } from "react";
import { Chip } from "../../ui/Chip";
import css from "./FilesFileMeta.module.css";
import { type FileStatus, statusWords } from "./words";

type FilesFileMetaProps = {
  // How it's changed since the branch it left (base), if it has.
  status?: FileStatus;
  base: string;
  // Where it was, renamed.
  oldPath?: string;
  uncommitted?: boolean;
  // What its marks beside the line numbers mean (a FilesChangeLegend).
  legend?: ReactNode;
};

// FilesFileMeta is what's said of the open file under its head: how it's
// changed since the branch point, and what its marks mean.
export function FilesFileMeta({ status, base, oldPath, uncommitted, legend }: FilesFileMetaProps) {
  return (
    <div className={css.meta}>
      {status ? (
        <Chip tone={status === "deleted" ? "danger" : status === "added" ? "done" : "attention"}>
          {statusWords[status]} since {base}
        </Chip>
      ) : (
        base && <Chip tone="quiet">Unchanged since {base}</Chip>
      )}
      {oldPath && <span className="dim">from {oldPath}</span>}
      {uncommitted && <Chip tone="attention">Not committed</Chip>}
      {legend}
    </div>
  );
}
