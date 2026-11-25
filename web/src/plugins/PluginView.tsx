import type { ReactNode } from "react";
import { Banner } from "../ui/Banner";
import css from "./PluginView.module.css";

type PluginViewProps = {
  // What ws said to an app it has no frame protocol version in common
  // with, above the frame.
  refused?: ReactNode;
  // The frame, and what's held round it.
  children: ReactNode;
  // What ws draws over the frame (FrameUiLayer).
  over?: ReactNode;
};

// PluginView is a plugin's view: its own pages, in a frame that fills the
// page.
export function PluginView({ refused, children, over }: PluginViewProps) {
  return (
    <div className={css.view}>
      {refused && <Banner className={css.refused}>{refused}</Banner>}
      {children}
      {over}
    </div>
  );
}
