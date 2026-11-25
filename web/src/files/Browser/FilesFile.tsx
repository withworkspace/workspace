import type { ReactNode } from "react";
import css from "./FilesFile.module.css";

// The class of what's said in place of a file's text (couldn't open it,
// opening it, deleted, a binary file): placed under its head.
export const filesFileNoteClass = css.note;

// FilesFile is a file that's open: its head and what's said of it, over
// its text (or its picture, or what's said in place of them).
export function FilesFile({ children }: { children: ReactNode }) {
  return <div className={css.file}>{children}</div>;
}
