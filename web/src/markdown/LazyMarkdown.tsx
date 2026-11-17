import { lazy, Suspense } from "react";
import type { Pinned } from "../refs/refs";
import type { Place } from "./links";

// The markdown renderer loads with the first page that shows markdown, not
// with every page.
const Inner = lazy(() => import("./Markdown").then((m) => ({ default: m.Markdown })));

export function Markdown({ text, lines, at, pinned, headingLink }: { text: string; lines?: boolean; at?: Place; pinned?: Pinned[]; headingLink?: (anchor: string) => void }) {
  return (
    <Suspense fallback={<div className="markdown markdown-loading">{text}</div>}>
      <Inner text={text} lines={lines} at={at} pinned={pinned} headingLink={headingLink} />
    </Suspense>
  );
}
