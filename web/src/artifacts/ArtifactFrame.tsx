import { cx } from "../ui/cx";
import css from "./ArtifactFrame.module.css";

type ArtifactFrameProps = {
  // Where the version runs from, and what the frame's called.
  src: string;
  title: string;
  // How wide it's drawn: 0 fits the stage.
  px: number;
  // What it's a frame of, at which run: another is a new frame.
  frameKey: string;
  // The frame itself, for whoever talks to its page (useTweaks).
  frameRef?: (el: HTMLIFrameElement | null) => void;
};

// ArtifactFrame is a version running, in a sandboxed frame: its scripts
// run, in an origin of their own. It fills its stage, or is as wide as
// it's told, with a border outside that width: a width wider than the
// room for it scrolls from its left edge. What's in the frame is the
// artifact's own.
export function ArtifactFrame({ src, title, px, frameKey, frameRef }: ArtifactFrameProps) {
  return (
    <div className={cx(css.box, px ? css.sized : undefined)}>
      <iframe key={frameKey} ref={frameRef} className={css.frame} src={src} sandbox="allow-scripts" referrerPolicy="no-referrer" title={title} style={px ? { width: px } : undefined} />
    </div>
  );
}
