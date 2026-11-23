import { useEffect, useMemo, useRef, useState } from "react";

// useHoverDelay is what's hovered (shown), a moment after the pointer
// comes (over), so passing over doesn't flash, until a moment after it
// goes (out), so it can move onto the card; stay keeps it while it's on
// the card, and clear hides it at once.
export function useHoverDelay<T>(hideAfter = 120) {
  const [shown, setShown] = useState<T>();
  const timer = useRef<ReturnType<typeof setTimeout>>(undefined);
  useEffect(() => () => clearTimeout(timer.current), []);
  const act = useMemo(
    () => ({
      over(v: T) {
        clearTimeout(timer.current);
        timer.current = setTimeout(() => setShown(() => v), 350);
      },
      out() {
        clearTimeout(timer.current);
        timer.current = setTimeout(() => setShown(undefined), hideAfter);
      },
      stay() {
        clearTimeout(timer.current);
      },
      clear() {
        clearTimeout(timer.current);
        setShown(undefined);
      },
    }),
    [hideAfter],
  );
  return { shown, ...act };
}
