import { useEffect, useState } from "react";
import { cx } from "../ui/cx";
import { useTheme } from "../ui/theme";
import { CodeBlock, codeBlockClass } from "./CodeBlock";
import css from "./Mermaid.module.css";

// A ```mermaid block drawn as its diagram: by the reader (Markdown.tsx)
// and by the editor (editor/views.ts), the same way, so a diagram reads
// alike in both, and one that can't be drawn says so alike.

type MermaidAPI = typeof import("mermaid").default;
let mermaidLoading: Promise<MermaidAPI> | undefined;

// loadMermaid loads Mermaid the first time a diagram needs it (it's large).
function loadMermaid(): Promise<MermaidAPI> {
  mermaidLoading ??= import("mermaid").then(({ default: mermaid }) => mermaid);
  return mermaidLoading;
}

// Drawn is a diagram as Mermaid drew it, or why it couldn't be.
export type Drawn = { svg: string } | { error: string };

let drawings = 0;

// drawMermaid draws a diagram in the theme's colours. Each drawing has an
// id of its own: the diagram's styles are scoped by it, and two the same
// on a page (the reader's and the editor's, say) would style each other.
export async function drawMermaid(code: string, dark: boolean): Promise<Drawn> {
  const id = `mermaid-${++drawings}`;
  try {
    const mermaid = await loadMermaid();
    // strict: no scripts or click handlers from the diagram's text.
    mermaid.initialize({ startOnLoad: false, securityLevel: "strict", theme: dark ? "dark" : "default", fontFamily: "inherit" });
    const { svg } = await mermaid.render(id, code);
    return { svg };
  } catch (e) {
    return { error: (e as Error)?.message ?? String(e) };
  } finally {
    document.getElementById("d" + id)?.remove(); // what a failed render leaves behind
  }
}

type MermaidProps = {
  code: string;
  data?: Record<string, unknown>;
  viewable?: boolean;
  // Where it's drawn: className is what's drawn, whichever it is (the
  // diagram, the code while it loads, or why it can't be drawn);
  // drawnClass, the diagram once it's drawn; codeClass, the code shown as
  // written, in either of the others.
  className?: string;
  drawnClass?: string;
  codeClass?: string;
};

// Mermaid is a diagram drawn: as written while Mermaid loads, drawn again
// in the other theme's colours when it changes, and as written, with the
// first line of what's wrong, when it can't be drawn. data is the block's
// data- attributes (which lines it's from), kept on what draws it.
//
// A click opens it in the viewer (ui/Viewer.tsx), to move around in,
// unless viewable is false (the editor's, where a click selects it).
export function Mermaid({ code, data = {}, viewable = true, className, drawnClass, codeClass }: MermaidProps) {
  const [drawn, setDrawn] = useState<Drawn>();
  const { dark } = useTheme();
  useEffect(() => {
    let live = true;
    drawMermaid(code, dark).then((d) => live && setDrawn(d));
    return () => {
      live = false;
    };
  }, [code, dark]);
  if (drawn && "error" in drawn) {
    return (
      <div className={cx(css.error, className)} {...data} data-skip="">
        <div>This diagram couldn't be drawn: {drawn.error.split("\n")[0]}</div>
        <CodeBlock className={codeClass}>{code}</CodeBlock>
      </div>
    );
  }
  if (!drawn)
    return (
      <pre className={cx(codeBlockClass, css.loading, codeClass, className)} {...data} data-skip="">
        {code}
      </pre>
    );
  const open = viewable ? { "data-viewable": "Diagram", title: "Open it larger" } : {};
  return <div className={cx(css.mermaid, drawnClass, className)} {...data} {...open} data-skip="" dangerouslySetInnerHTML={{ __html: drawn.svg }} />;
}
