import type { ReactNode } from "react";
import { HoverCardHead, HoverCardLine, HoverCardTitle } from "../../ui/HoverCard";

// FindItemCard says, in Find's pane, of a row with no card of its own (a
// place, something to do) what the row says: its mark, what it's called,
// what kind of thing it is, and what more the row says of it.
export function FindItemCard({ mark, title, kind, hint }: { mark: ReactNode; title: string; kind: string; hint?: string }) {
  return (
    <>
      <HoverCardHead>
        {mark}
        <HoverCardTitle>{title}</HoverCardTitle>
      </HoverCardHead>
      <HoverCardLine dim>{kind}</HoverCardLine>
      {hint && <HoverCardLine dim>{hint}</HoverCardLine>}
    </>
  );
}
