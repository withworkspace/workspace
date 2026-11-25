import type { ReviewPass } from "../api";
import { PassMark } from "./PassMark";
import { standingWords } from "./passWords";

// PassMarks are how each pass that's been run stands, a mark each: beside
// Passes in the list down the side, so go or no go shows from any view.
export function PassMarks({ passes }: { passes?: ReviewPass[] }) {
  const ran = (passes ?? []).filter((p) => p.standing.status !== "none" || p.running);
  if (!ran.length) return null;
  return (
    <>
      {ran.map((p) => {
        const running = !!p.running || p.standing.status === "running";
        return <PassMark key={p.name} standing={p.standing} running={running} title={`${p.title}: ${running ? "running" : standingWords(p.standing).toLowerCase()}`} />;
      })}
    </>
  );
}
