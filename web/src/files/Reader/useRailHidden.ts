import { useCallback, useState } from "react";

// Hidden with t, the rail stays hidden from one document to the next until
// it's shown again or the app's reloaded.
let railHiddenNow = false;

export function useRailHidden(): [boolean, () => void] {
  const [hidden, setHidden] = useState(railHiddenNow);
  const toggle = useCallback(() => setHidden((h) => (railHiddenNow = !h)), []);
  return [hidden, toggle];
}
