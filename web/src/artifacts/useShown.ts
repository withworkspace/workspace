import { useEffect, useState } from "react";

// useShown is what a control shows: what was last put in it, at once, and
// the tweak's value whenever the frame says another (a preset, the page
// itself). The frame's answer is a message away, which a slider being
// dragged or a word being typed can't wait for.
export function useShown<T>(value: T): [T, (v: T) => void] {
  const [shown, setShown] = useState(value);
  useEffect(() => setShown(value), [value]);
  return [shown, setShown];
}
