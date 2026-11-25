import { useEffect, useLayoutEffect } from "react";
import { useLocation } from "react-router";
import { useChromeStore, useFrameHere } from "../chrome/store";
import { useRefs } from "../refs/known";
import type { Known } from "../refs/refs";
import { useWorkspace } from "../workspace/store";
import { hereAt } from "./here";

// SaysHere says what the page that's open is of (Find's Here), once, for
// what's beside it (chrome/store.ts): what was opened about a thing is
// there only on that thing's page. A plugin's page is of what its app
// says it shows, which needs what there is to name; any other page's
// address is enough.
const nothingKnown: Known = { repos: new Map(), missions: [], pages: [], talks: [] };

export function SaysHere() {
  const workspace = useWorkspace();
  const location = useLocation();
  const known = useRefs()?.known;
  const frame = useFrameHere();
  const setHere = useChromeStore((c) => c.setHere);
  const id = hereAt(location.pathname, location.search, workspace, known ?? nothingKnown, frame)?.id;
  useLayoutEffect(() => setHere(id), [setHere, id]);
  useEffect(() => () => setHere(undefined), [setHere]);
  return null;
}
