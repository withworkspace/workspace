import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import css from "./ArtifactNote.module.css";

type ArtifactNoteProps = {
  // dim is one that matters less than the version's own note.
  dim?: boolean;
  // expiry is one that says when it expires: soon, in the colour of what
  // wants attention, or expired, which is quiet again.
  expiry?: "soon" | "expired";
  children: ReactNode;
};

// ArtifactNote is a line or two said of an artifact in its head, at a
// reading measure: a version's note, where it stands, when it expires.
export function ArtifactNote({ dim, expiry, children }: ArtifactNoteProps) {
  return <div className={cx(css.note, expiry && css.expiry, expiry === "expired" && css.expired, dim && "dim")}>{children}</div>;
}
