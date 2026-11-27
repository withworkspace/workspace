import { filesKeys } from "../keys";
import { Button } from "../../ui/Button";
import { Copyable } from "../../ui/Copyable";
import { FileIcon } from "../../ui/FileIcon";
import css from "./FilesFilePath.module.css";

// FilesFilePath is where the open file is: each folder it's in, which
// shows that folder in the tree (onReveal), then its own name, and its
// path to copy.
export function FilesFilePath({ path, onReveal }: { path: string; onReveal: (folder: string) => void }) {
  const parts = path.split("/");
  return (
    <div className={css.path} title={path}>
      {parts.map((p, i) =>
        i < parts.length - 1 ? (
          <span key={i}>
            <Button variant="link" className={css.crumb} onClick={() => onReveal(parts.slice(0, i + 1).join("/"))}>
              {p}
            </Button>
            <span className={css.sep}>/</span>
          </span>
        ) : (
          <strong key={i} className={css.name}>
            <FileIcon path={path} />
            {p}
          </strong>
        ),
      )}
      <Copyable text={path} className={css.copy} icon="inline" title="Copy path" keys={filesKeys.copyPath}>
        {""}
      </Copyable>
    </div>
  );
}
