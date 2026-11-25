import { Change } from "../ui/Change";
import css from "./Suggestion.module.css";

// Suggestion is the code a comment (or a finding, or a comment an agent
// proposes) would put in place of the lines it's on: those lines, then
// its own; with nothing of its own, it takes them away.
export function Suggestion({ from, to }: { from?: string[]; to: string }) {
  return <Change className={css.suggested} title="Suggested change" signs was={from ?? []} now={to === "" ? [] : to.split("\n")} gone="(delete these lines)" />;
}
