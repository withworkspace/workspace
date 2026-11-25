import type { PageInfo } from "../api";
import { HoverCardLine } from "../ui/HoverCard";
import { type PageRef, refHref } from "./refs";
import { RefCardHead } from "./RefCardHead";
import { RefGo } from "./RefGo";

// RefPageCard is a page's card: its title, its cells and what's open or
// out of date in it.
export function RefPageCard({ r, page: p }: { r: Extract<PageRef, { kind: "page" }>; page?: PageInfo }) {
  return (
    <>
      <RefCardHead icon="file" title={p?.title ?? r.id} />
      <HoverCardLine dim>
        Page {r.id}
        {p?.cells ? `, ${p.cells} ${p.cells === 1 ? "cell" : "cells"}` : ""}
        {p?.findings ? `, ${p.findings} open` : ""}
        {p?.stale ? `, ${p.stale} out of date` : ""}
      </HoverCardLine>
      <RefGo to={refHref(r)}>Open the page</RefGo>
    </>
  );
}
