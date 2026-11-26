import type { ReactNode } from "react";
import css from "./MissionVerifyShot.module.css";

type MissionVerifyShotProps = {
  kind: "image" | "video" | "text";
  url: string;
  // What it's of, for whoever can't see it.
  what: string;
  // Over it: Before, on ground; After, in the capsule.
  label?: string;
  // A text file's words: read by whoever draws it.
  children?: ReactNode;
};

// MissionVerifyShot is one file that shows a check, as its kind: an image
// (which opens larger), a video to play, or text.
export function MissionVerifyShot({ kind, url, what, label, children }: MissionVerifyShotProps) {
  return (
    <div className={css.shot}>
      {label && <span className={css.label}>{label}</span>}
      {kind === "video" ? (
        <video src={url} controls preload="metadata" aria-label={what} />
      ) : kind === "image" ? (
        <img src={url} alt={label ? `${label}: ${what}` : what} data-viewable={label ? `${label}: ${what}` : what} title="Open larger" />
      ) : (
        children
      )}
    </div>
  );
}
