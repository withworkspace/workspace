import { useLayoutEffect, useState } from "react";
import { Below } from "./Below";
import { sampleViews } from "./belowSamples";
import { resting } from "./chromeState";
import { useChromeStore } from "./store";

// useBelowStarted puts the window's store as a story starts with it:
// nobody's arranged it, and Below is open or not, on Results. True once
// it's so: until then there's nothing to draw, or Below would be seen to
// arrive.
export function useBelowStarted(open: boolean): boolean {
  const [started, setStarted] = useState(false);
  useLayoutEffect(() => {
    useChromeStore.setState({ root: "", chrome: { ...resting, below: { ...resting.below, open, tab: "results" } } });
    setStarted(true);
  }, [open]);
  return started;
}

// SampleBelow is Below as the stories draw it: the app's own (Below.tsx),
// which reads the window's store, over hand-written views
// (belowSamples.tsx). The run's terminal is a tab you can close.
export function SampleBelow({ sheet }: { sheet?: boolean }) {
  const where = useChromeStore((s) => s.chrome.below.where);
  const [terminal, setTerminal] = useState(true);
  return <Below views={sampleViews(where, terminal ? () => setTerminal(false) : undefined)} sheet={sheet} />;
}
