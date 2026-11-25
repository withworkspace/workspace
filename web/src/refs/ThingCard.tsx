import type { ReactNode } from "react";
import { HoverCardLine } from "../ui/HoverCard";
import { isOutside, type Known, namedOf, type PageRef, refHref } from "./refs";
import { RefCardHead } from "./RefCardHead";
import { RefGo } from "./RefGo";
import { type Surface, thingBlocks } from "./thing";
import { type Pointer, ThingView } from "./ThingView";
import { useThing } from "./useThing";
import { thingIcon } from "./views";

// ThingCard is a plugin's thing (a Jira issue): its fields, as ws has them
// or as its plugin says when asked, drawn as the plugin declared for where
// it's shown (ThingView). Around that is ws's own: that it's being asked
// after, that there's none or the plugin couldn't be asked, and somewhere
// to go. A thing with nothing drawn yet (it's being asked after, there's
// none, or its kind has no fields) is its name, and what one of its kind
// is called.
//
// before is shown in place of ws's own, while there's nothing of the thing
// to draw: Find's pane has what the search found of it already. pointer
// draws a name one of its fields holds as what it names: Find's pane
// does, and a card under a name leaves it as words.
export function ThingCard({
  r,
  known,
  surface = "card",
  before,
  pointer,
}: {
  r: Extract<PageRef, { kind: "thing" }>;
  known?: Known;
  surface?: Surface;
  before?: ReactNode;
  pointer?: Pointer;
}) {
  const named = namedOf(r, known);
  const { thing, error } = useThing(r);
  const what = r.what.replace(/-/g, " ");
  const href = refHref(r, known);
  const drawn = !!thing && thingBlocks(named, thing, surface).length > 0;
  if (!drawn && before) return <>{before}</>;
  return (
    <>
      {drawn ? (
        <ThingView named={named} record={thing} surface={surface} pointer={pointer} />
      ) : (
        <>
          <RefCardHead icon={thingIcon(named)} title={r.id} />
          <HoverCardLine dim>
            {named?.title || `${r.plugin} ${what}`} {r.id}
          </HoverCardLine>
        </>
      )}
      {thing === null && (
        <HoverCardLine dim>
          {r.plugin} has no {what} called {r.id}.
        </HoverCardLine>
      )}
      {error && (
        <HoverCardLine dim>
          {r.plugin} couldn't be asked about it: {error}
        </HoverCardLine>
      )}
      {thing === undefined && !error && <HoverCardLine dim>Asking {r.plugin}…</HoverCardLine>}
      {/* One that isn't there has nowhere to go. */}
      {thing === null ? null : isOutside(href) ? <RefGo href={href}>Open the {what}</RefGo> : <RefGo to={href}>Open the {what}</RefGo>}
    </>
  );
}
