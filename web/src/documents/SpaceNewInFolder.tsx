import { useState } from "react";
import { Button } from "../ui/Button";
import { Dialog, DialogActions } from "../ui/Dialog";
import { Input } from "../ui/Input";
import { Keys } from "../ui/Keys";
import css from "./SpaceNewInFolder.module.css";
import { Text } from "../ui/Text";

type SpaceNewInFolderProps = {
  repo: string;
  label: string; // the space's
  root: string; // the space's own folder
  folders: string[]; // its folders, below root
  busy: boolean;
  onStart: (title: string, to: string) => void; // to: repo:folder/
  onClose: () => void;
};

// SpaceNewInFolder starts a page to be filed in one of a space's folders
// (a space with no kinds of document): a folder it has, or a new one, and
// a title its file's named from. File as… files it there.
export function SpaceNewInFolder({ repo, label, root, folders, busy, onStart, onClose }: SpaceNewInFolderProps) {
  const [folder, setFolder] = useState(folders.find((f) => f !== "") ?? "");
  const [title, setTitle] = useState("");
  const clean = folder
    .trim()
    .replace(/^\/+/, "")
    .replace(/\/*$/, folder.trim() ? "/" : "");
  const go = () => title.trim() && onStart(title.trim(), `${repo}:${root}${clean}`);
  const name =
    title
      .toLowerCase()
      .split(/[^a-z0-9]+/)
      .filter(Boolean)
      .join("-") || "its-title";
  return (
    <Dialog title={`New in ${label}`} onClose={onClose} wide>
      <div className={css.fields}>
        <label>
          <span>Folder</span>
          <Input list="space-folders" value={folder} placeholder="(top level)" aria-label="Folder" onChange={(e) => setFolder(e.target.value)} />
          <datalist id="space-folders">
            {folders.filter(Boolean).map((f) => (
              <option key={f} value={f} />
            ))}
          </datalist>
        </label>
        <label>
          <span>Title</span>
          <Input
            autoFocus
            value={title}
            placeholder="What's this about?"
            aria-label="Title"
            onChange={(e) => setTitle(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                e.preventDefault();
                go();
              }
            }}
          />
        </label>
      </div>
      <Text as="p" dim className={css.what}>
        When ready, use <em>File as…</em> to file it at{" "}
        <Text mono>{`${repo} · ${root}${clean}${name}.md`}</Text>.
      </Text>
      <DialogActions>
        <Button variant="ghost" onClick={onClose}>
          Cancel
        </Button>
        <Button variant="primary" onClick={go} disabled={busy || !title.trim()} title="Create page" keys="⏎">
          Create <Keys k="⏎" />
        </Button>
      </DialogActions>
    </Dialog>
  );
}
