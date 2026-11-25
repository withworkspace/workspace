import type { Anchor } from "../../ui/FloatingCard";
import type { Asked } from "../ui";

// UiProps is what each of ws's own UIs a plugin's page asked for (ui.ts)
// is drawn with, over the page's frame (FrameUiLayer).
export interface UiProps {
  // What the page asked for: what it's given is as its kind takes it.
  asked: Asked;
  // The plugin whose page asked.
  plugin: string;
  // Where it was asked from, in ws's window, if the page said.
  anchor?: Anchor;
  // ws is done with it: it goes, and the page is told what came of it.
  done(result?: unknown): void;
  // On hover: the pointer's on it, and has left it.
  stay(): void;
  leave(): void;
}
