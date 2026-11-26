import { HoverCardHead, HoverCardLine, HoverCardTitle } from "../ui/HoverCard";
import { Markdown } from "../markdown/LazyMarkdown";
import { Icon } from "../ui/Icon";
import { exact, stamp } from "../ui/when";
import { NoRefs } from "./known";
import type { ReactNode } from "react";
import type { Named } from "./refs";
import { type Drawn, type Surface, thingBlocks, type ThingRecord } from "./thing";
import { thingIcon } from "./views";
import { Chip } from "../ui/Chip";
import { cx } from "../ui/cx";
import { RefChip } from "./RefChip";
import css from "./ThingView.module.css";
import { textClass } from "../ui/Text";

// A plugin's thing, drawn from its fields as its plugin declared for where
// it's shown: the card under its name, the pane beside the list in Find.
// Which blocks, and what each says, is refs/thing.ts's; this is how each
// looks. What ws says around it (asking, none there, somewhere to go) is
// whoever shows it's.

// said is a block's words: a time as ws says times, anything else as it
// is.
function said(d: Drawn) {
  if (!d.time || Number.isNaN(Date.parse(d.value))) return d.value;
  return (
    <time dateTime={d.value} title={exact(d.value)}>
      {stamp(d.value)}
    </time>
  );
}

// Pointer draws one name a thing's field holds (a field that names other
// things): as what it names, where whoever shows the thing has room for
// that. Without one a name is its words, as under a name it always is: a
// card opens no cards.
export type Pointer = (name: string) => ReactNode;

// ThingView draws a thing's view for a surface: its blocks in order, a
// badge on the line of the title or subtitle before it.
export function ThingView({ named, record, surface, pointer }: { named?: Named; record: ThingRecord; surface: Surface; pointer?: Pointer }) {
  const drawn = thingBlocks(named, record, surface);
  // A line is a block and the badges that sit on it.
  const lines: { of: Drawn; badges: Drawn[] }[] = [];
  for (const d of drawn) {
    if (d.inline && lines.length) lines[lines.length - 1].badges.push(d);
    else lines.push({ of: d, badges: [] });
  }
  if (!lines.length) return null;
  // In a pane there's room for a name said whole, and long words break.
  const pane = surface === "pane";
  return (
    <span className={css.view}>
      {lines.map(({ of: d, badges }, i) => {
        // On a line, a badge keeps a little from the words before it; in
        // the head, the head's own gap does.
        const badged = (onLine: boolean) =>
          badges.map((b, j) =>
            onLine ? (
              <RefChip tone="quiet" key={j}>
                {said(b)}
              </RefChip>
            ) : (
              <Chip tone="quiet" key={j}>
                {said(b)}
              </Chip>
            ),
          );
        const chips = badged(true);
        switch (d.block) {
          case "title":
            return (
              <HoverCardHead key={i} top={pane}>
                {d.icon && <Icon name={thingIcon(named)} size={13} className={textClass("dim")} />}
                <HoverCardTitle wrap={pane}>{said(d)}</HoverCardTitle>
                {badged(false)}
              </HoverCardHead>
            );
          case "subtitle":
            return (
              <HoverCardLine key={i} dim wrap={pane}>
                {said(d)}
                {chips}
              </HoverCardLine>
            );
          case "badge":
            return (
              <HoverCardLine key={i} dim wrap={pane}>
                {/* The first on its line: nothing before it to keep from. */}
                <Chip tone="quiet">{said(d)}</Chip>
                {chips}
              </HoverCardLine>
            );
          case "field":
            return (
              <HoverCardLine key={i} dim wrap={pane}>
                {d.label}:{" "}
                {d.names
                  ? d.names.map((name, j) => (
                      <span key={j}>
                        {j > 0 && ", "}
                        {pointer ? pointer(name) : name}
                      </span>
                    ))
                  : said(d)}
              </HoverCardLine>
            );
          case "markdown":
            // What it says is its plugin's words: the names in them are
            // only words, and it's kept to the lines its block says.
            return (
              <div key={i} className={cx(css.md, !!d.lines && css.kept)} style={d.lines ? ({ "--lines": d.lines } as React.CSSProperties) : undefined}>
                <NoRefs>
                  <Markdown text={d.value} />
                </NoRefs>
              </div>
            );
        }
      })}
    </span>
  );
}
