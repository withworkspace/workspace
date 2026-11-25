import { usePref } from "../prefs";

// The list's width on a space's page, as dragged (kept in this browser);
// none, as it was laid out: five parts to the preview's seven.
export const LIST_MIN = 280;
export const PREVIEW_MIN = 380;

export function useListWidth(): [number | undefined, (w: number | undefined) => void] {
  const [w, set] = usePref("spaceListWidth");
  return [w !== undefined && w >= LIST_MIN ? w : undefined, (n) => set(n || undefined)];
}
