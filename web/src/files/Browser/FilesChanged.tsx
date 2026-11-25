import { Button } from "../../ui/Button";
import { Chip } from "../../ui/Chip";
import { FileIcon } from "../../ui/FileIcon";
import { GroupHead } from "../../ui/GroupHead";
import { FilesMark } from "./FilesMark";
import css from "./FilesChanged.module.css";
import type { FileStatus } from "./words";

export interface ChangedFile {
  path: string;
  status?: FileStatus;
  uncommitted?: boolean;
}

// FilesChanged is the files changed since the branch point, under their
// count (of all of them), a click each to open.
export function FilesChanged({ files, count, onOpen }: { files: ChangedFile[]; count: number; onOpen: (path: string) => void }) {
  return (
    <section>
      <GroupHead count={count}>Changed</GroupHead>
      <ul className={css.list}>
        {files.map((f) => (
          <li key={f.path}>
            <Button variant="link" className={css.file} onClick={() => onOpen(f.path)}>
              <FilesMark status={f.status} uncommitted={f.uncommitted} />
              <FileIcon path={f.path} />
              <span className="mono">{f.path}</span>
              {f.uncommitted && <Chip tone="attention">Not committed</Chip>}
            </Button>
          </li>
        ))}
      </ul>
    </section>
  );
}
