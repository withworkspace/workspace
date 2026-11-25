import { useEffect, useRef } from "react";
import { unview, type Viewing, view } from "../../ui/Viewer";
import type { UiProps } from "./uiProps";

// PictureUi shows a picture a plugin's page asked for in ws's viewer,
// over the whole window, as one in ws's own pages opens: it's the
// viewer's, and draws nothing of its own. It's done as the person closes
// it; the page taking it back takes it down. ui.ts has checked its
// address is one ws shows a picture from.
export function PictureUi({ asked, done }: UiProps) {
  const finish = useRef(done);
  finish.current = done;
  const { src, title } = asked.with as { src: string; title?: string };
  useEffect(() => {
    const shown: Viewing = { kind: "image", src, ...(title ? { title } : {}) };
    // Taken down here (the page took it back, or the page went): the page
    // isn't told. A render made twice (StrictMode) shows it again.
    let leaving = false;
    view(shown, () => {
      if (!leaving) finish.current();
    });
    return () => {
      leaving = true;
      unview(shown);
    };
  }, [src, title]);
  return null;
}
